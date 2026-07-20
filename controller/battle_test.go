package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	battlesvc "github.com/QuantumNous/new-api/service/battle"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type battleWebSocketTestMessage struct {
	Type      string                     `json:"type"`
	RoomId    string                     `json:"room_id"`
	Me        int                        `json:"me"`
	MapWidth  int                        `json:"map_width"`
	MapHeight int                        `json:"map_height"`
	Players   []battlesvc.PlayerSnapshot `json:"players"`
	Events    []battlesvc.BattleEvent    `json:"events"`
}

func readBattleWebSocketTestMessage(t *testing.T, conn *websocket.Conn, messageType string) battleWebSocketTestMessage {
	t.Helper()
	for {
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
		_, data, err := conn.ReadMessage()
		require.NoError(t, err)

		var message battleWebSocketTestMessage
		require.NoError(t, common.Unmarshal(data, &message))
		if message.Type == messageType {
			return message
		}
	}
}

func battleWebSocketTestPlayer(t *testing.T, message battleWebSocketTestMessage, userId int) battlesvc.PlayerSnapshot {
	t.Helper()
	for _, player := range message.Players {
		if player.UserId == userId {
			return player
		}
	}
	t.Fatalf("player %d not found in snapshot", userId)
	return battlesvc.PlayerSnapshot{}
}

func setupBattleControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db

	require.NoError(t, db.AutoMigrate(&model.User{}))

	t.Cleanup(func() {
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func battleWebSocketTestAuth(userId int, username string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("id", userId)
		c.Set("username", username)
		c.Next()
	}
}

func createBattleControllerTestUser(t *testing.T, id int, quota int) {
	t.Helper()
	username := fmt.Sprintf("battle-controller-user-%d", id)
	affCode := fmt.Sprintf("battle-controller-aff-%d", id)
	cleanup := func() {
		require.NoError(t, model.DB.Unscoped().Where("id = ? OR username = ? OR aff_code = ?", id, username, affCode).Delete(&model.User{}).Error)
	}
	cleanup()
	t.Cleanup(cleanup)

	require.NoError(t, model.DB.Create(&model.User{
		Id:       id,
		Username: username,
		Password: "battle-test-password",
		Status:   common.UserStatusEnabled,
		Quota:    quota,
		AffCode:  affCode,
	}).Error)
}

func TestBattleWebSocketJoinsRoomAndAppliesServerMovement(t *testing.T) {
	setupBattleControllerTestDB(t)

	setting := operation_setting.GetBattleSetting()
	originalSetting := *setting
	*setting = operation_setting.BattleSetting{
		Enabled:            true,
		MinDropQuota:       100,
		MaxDropQuota:       100,
		MaxRoundLossQuota:  1000,
		MaxRoundGainQuota:  1000,
		MaxDailyLossQuota:  1000,
		MaxDailyGainQuota:  1000,
		MaxPlayersPerRoom:  8,
		TickRate:           30,
		MapWidth:           800,
		MapHeight:          600,
		PlayerSpeed:        240,
		BulletSpeed:        700,
		BulletDamage:       34,
		FireCooldownMs:     220,
		RespawnSeconds:     3,
		DropPickupRadius:   38,
		DropExpireSeconds:  18,
		IdleRoomTTLSeconds: 5,
	}
	t.Cleanup(func() {
		*setting = originalSetting
	})

	originalManager := battlesvc.DefaultManager
	battlesvc.DefaultManager = battlesvc.NewManager()
	t.Cleanup(func() {
		battlesvc.DefaultManager = originalManager
	})
	createBattleControllerTestUser(t, 7, 1000)

	router := gin.New()
	router.Use(battleWebSocketTestAuth(7, "battle-test-user"))
	router.GET("/api/battle/ws", BattleWebSocket)

	server := httptest.NewServer(router)
	defer server.Close()

	headers := http.Header{}
	headers.Set("Origin", server.URL)

	wsUrl := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/battle/ws?room=ws-test"
	conn, _, err := websocket.DefaultDialer.Dial(wsUrl, headers)
	require.NoError(t, err)
	defer conn.Close()

	joined := readBattleWebSocketTestMessage(t, conn, "joined")
	assert.Equal(t, "ws-test", joined.RoomId)

	initial := readBattleWebSocketTestMessage(t, conn, "snapshot")
	assert.Equal(t, 7, initial.Me)
	assert.Equal(t, 800, initial.MapWidth)
	assert.Equal(t, 600, initial.MapHeight)
	assert.NotNil(t, initial.Events)
	initialPlayer := battleWebSocketTestPlayer(t, initial, 7)

	input := battlesvc.PlayerInput{AimX: 1}
	expectIncrease := initialPlayer.X < float64(initial.MapWidth)/2
	if expectIncrease {
		input.Right = true
	} else {
		input.Left = true
	}
	data, err := common.Marshal(battlesvc.ClientMessage{
		Type:  "input",
		Input: input,
	})
	require.NoError(t, err)
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, data))

	moved := false
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		snapshot := readBattleWebSocketTestMessage(t, conn, "snapshot")
		player := battleWebSocketTestPlayer(t, snapshot, 7)
		if expectIncrease && player.X > initialPlayer.X {
			moved = true
			break
		}
		if !expectIncrease && player.X < initialPlayer.X {
			moved = true
			break
		}
	}
	assert.True(t, moved, "server snapshot should apply the submitted movement input")
}

func TestBattleWebSocketRejectsNonPositiveBalance(t *testing.T) {
	for _, tc := range []struct {
		name   string
		userId int
		quota  int
	}{
		{name: "zero", userId: 8, quota: 0},
		{name: "negative", userId: 9, quota: -100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupBattleControllerTestDB(t)

			setting := operation_setting.GetBattleSetting()
			originalSetting := *setting
			*setting = operation_setting.BattleSetting{Enabled: true}
			t.Cleanup(func() {
				*setting = originalSetting
			})
			createBattleControllerTestUser(t, tc.userId, tc.quota)

			router := gin.New()
			router.Use(battleWebSocketTestAuth(tc.userId, fmt.Sprintf("battle-controller-user-%d", tc.userId)))
			router.GET("/api/battle/ws", BattleWebSocket)

			server := httptest.NewServer(router)
			defer server.Close()

			headers := http.Header{}
			headers.Set("Origin", server.URL)

			wsUrl := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/battle/ws?room=ws-test"
			conn, upgradeResponse, err := websocket.DefaultDialer.Dial(wsUrl, headers)
			if conn != nil {
				_ = conn.Close()
			}
			require.Error(t, err)
			require.NotNil(t, upgradeResponse)
			require.NoError(t, upgradeResponse.Body.Close())
			assert.Equal(t, http.StatusPaymentRequired, upgradeResponse.StatusCode)
		})
	}
}

func TestBattleWebSocketRejectsBalanceBelowMatchDeposit(t *testing.T) {
	setupBattleControllerTestDB(t)

	setting := operation_setting.GetBattleSetting()
	originalSetting := *setting
	*setting = operation_setting.BattleSetting{
		Enabled:          true,
		MatchModeEnabled: true,
		MatchEntryQuota:  500,
	}
	t.Cleanup(func() {
		*setting = originalSetting
	})
	createBattleControllerTestUser(t, 10, 100)

	router := gin.New()
	router.Use(battleWebSocketTestAuth(10, "battle-controller-user-10"))
	router.GET("/api/battle/ws", BattleWebSocket)

	server := httptest.NewServer(router)
	defer server.Close()

	headers := http.Header{}
	headers.Set("Origin", server.URL)

	wsUrl := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/battle/ws?room=ws-test"
	conn, upgradeResponse, err := websocket.DefaultDialer.Dial(wsUrl, headers)
	if conn != nil {
		_ = conn.Close()
	}
	require.Error(t, err)
	require.NotNil(t, upgradeResponse)
	require.NoError(t, upgradeResponse.Body.Close())
	assert.Equal(t, http.StatusPaymentRequired, upgradeResponse.StatusCode)
}
