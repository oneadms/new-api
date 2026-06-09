package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	battlesvc "github.com/QuantumNous/new-api/service/battle"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestBattleWebSocketJoinsRoomAndAppliesServerMovement(t *testing.T) {
	gin.SetMode(gin.TestMode)

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

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("battle-test-secret"))))
	router.GET("/test-session", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", 7)
		session.Set("username", "battle-test-user")
		session.Set("status", common.UserStatusEnabled)
		if err := session.Save(); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.GET("/api/battle/ws", BattleWebSocket)

	server := httptest.NewServer(router)
	defer server.Close()

	response, err := http.Get(server.URL + "/test-session")
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusNoContent, response.StatusCode)
	require.NotEmpty(t, response.Cookies())

	headers := http.Header{}
	headers.Set("Origin", server.URL)
	for _, sessionCookie := range response.Cookies() {
		headers.Add("Cookie", sessionCookie.String())
	}

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
