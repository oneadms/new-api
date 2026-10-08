package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCheckinControllerTest(t *testing.T, regularEnabled, luckyEnabled bool) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedis := common.RedisEnabled
	regular, lucky := operation_setting.GetCheckinSetting(), operation_setting.GetLuckyCheckinSetting()
	previousRegular, previousLucky := *regular, *lucky

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	*regular = operation_setting.CheckinSetting{Enabled: regularEnabled, MinQuota: 50, MaxQuota: 50}
	// 固定成功概率，测试只验证开关与额度变动，不依赖随机结果。
	*lucky = operation_setting.LuckyCheckinSetting{Enabled: luckyEnabled, MinStakeQuota: 100, MaxStakeQuota: 200}
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.RedisEnabled = previousRedis
		*regular, *lucky = previousRegular, previousLucky
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Checkin{}, &model.Log{}))
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "checkin-user", AffCode: "checkin-user", Quota: 1000}).Error)
	return db
}

func TestCheckinStatusReportsIndependentModes(t *testing.T) {
	for _, test := range []struct {
		name    string
		regular bool
		lucky   bool
	}{
		{name: "both disabled"},
		{name: "regular only", regular: true},
		{name: "lucky only", lucky: true},
		{name: "both enabled", regular: true, lucky: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupCheckinControllerTest(t, test.regular, test.lucky)
			statusRecorder := httptest.NewRecorder()
			statusContext, _ := gin.CreateTestContext(statusRecorder)
			statusContext.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)
			GetStatus(statusContext)
			var status struct {
				Data struct {
					Regular bool `json:"checkin_enabled"`
					Lucky   bool `json:"lucky_checkin_enabled"`
				} `json:"data"`
			}
			require.NoError(t, common.Unmarshal(statusRecorder.Body.Bytes(), &status))
			assert.Equal(t, test.regular, status.Data.Regular)
			assert.Equal(t, test.lucky, status.Data.Lucky)

			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Set("id", 1)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/checkin", nil)
			GetCheckinStatus(ctx)
			var response struct {
				Success bool `json:"success"`
				Data    struct {
					Enabled bool `json:"enabled"`
					Lucky   struct {
						Enabled bool `json:"enabled"`
					} `json:"lucky"`
				} `json:"data"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			assert.Equal(t, test.regular || test.lucky, response.Success)
			if response.Success {
				assert.Equal(t, test.regular, response.Data.Enabled)
				assert.Equal(t, test.lucky, response.Data.Lucky.Enabled)
			}
		})
	}
}

func TestDoCheckinEnforcesEachModeIndependently(t *testing.T) {
	for _, test := range []struct {
		name         string
		regular      bool
		lucky        bool
		body         string
		award        int
		failureBps   int
		errorMessage string
	}{
		{name: "lucky without regular", lucky: true, body: `{"stake_quota":100}`, award: 100},
		{name: "regular without lucky", regular: true, body: `{}`, award: 50},
		{name: "regular disabled", lucky: true, body: `{}`, errorMessage: "签到功能未启用"},
		{name: "lucky disabled", regular: true, body: `{"stake_quota":100}`, errorMessage: "运气签到功能未启用"},
		{name: "lucky loss without regular", lucky: true, body: `{"stake_quota":100}`, failureBps: 10000, award: -100},
		{name: "stake below minimum", lucky: true, body: `{"stake_quota":0}`, errorMessage: "押注额度必须在 100 到 200 之间"},
		{name: "stake above maximum", lucky: true, body: `{"stake_quota":201}`, errorMessage: "押注额度必须在 100 到 200 之间"},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := setupCheckinControllerTest(t, test.regular, test.lucky)
			lucky := operation_setting.GetLuckyCheckinSetting()
			lucky.MinFailureBps, lucky.MaxFailureBps = test.failureBps, test.failureBps
			lucky.ActualMinFailureBps, lucky.ActualMaxFailureBps = test.failureBps, test.failureBps
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Set("id", 1)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/checkin", strings.NewReader(test.body))
			DoCheckin(ctx)
			var response struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
				Data    struct {
					QuotaAwarded int `json:"quota_awarded"`
				} `json:"data"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.Equal(t, test.errorMessage == "", response.Success, response.Message)
			if test.errorMessage != "" {
				assert.Equal(t, test.errorMessage, response.Message)
			} else {
				assert.Equal(t, test.award, response.Data.QuotaAwarded)
				// 两种签到继续共用每日一次的限制，切换模式不能重复领取。
				operation_setting.GetCheckinSetting().Enabled = true
				_, err := model.UserCheckin(1, nil)
				assert.ErrorIs(t, err, model.ErrAlreadyCheckedInToday)
			}
			var user model.User
			require.NoError(t, db.First(&user, 1).Error)
			assert.Equal(t, 1000+test.award, user.Quota)
			var count int64
			require.NoError(t, db.Model(&model.Checkin{}).Count(&count).Error)
			if test.errorMessage == "" {
				assert.EqualValues(t, 1, count)
			} else {
				assert.Zero(t, count)
			}
		})
	}
}
