package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func qualityRouterFixture(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	apiLimit, criticalLimit := common.GlobalApiRateLimitEnable, common.CriticalRateLimitEnable
	common.GlobalApiRateLimitEnable, common.CriticalRateLimitEnable = false, false
	t.Cleanup(func() {
		common.GlobalApiRateLimitEnable, common.CriticalRateLimitEnable = apiLimit, criticalLimit
	})
	engine := gin.New()
	SetApiRouter(engine)
	return engine
}

func TestQualityRoutesRejectAnonymousAndOrdinaryUsersBeforeHandlers(t *testing.T) {
	engine := qualityRouterFixture(t)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	previousDB, previousRedis := model.DB, common.RedisEnabled
	previousType := common.MainDatabaseType()
	model.DB, common.RedisEnabled = db, false
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB, common.RedisEnabled = previousDB, previousRedis
		common.SetMainDatabaseType(previousType)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Option{}))
	require.NoError(t, model.EnsureLegacyAccessTokenRetireAt(common.GetTimestamp()))
	token := "quality-route-ordinary-user"
	require.NoError(t, db.Create(&model.User{Id: 78001, Username: "quality-reader", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AccessToken: &token, AffCode: "quality-reader", AuthVersion: 1}).Error)

	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/settings"}, {http.MethodPut, "/settings"},
		{http.MethodGet, "/channels"}, {http.MethodPut, "/channels/1"},
		{http.MethodGet, "/prompts"}, {http.MethodPost, "/prompts"}, {http.MethodPost, "/prompts/1/archive"},
		{http.MethodGet, "/batches"}, {http.MethodPost, "/batches"}, {http.MethodGet, "/batches/1"}, {http.MethodPost, "/batches/1/cancel"},
		{http.MethodGet, "/runs/1"}, {http.MethodGet, "/runs/1/output"}, {http.MethodGet, "/runs/1/thumbnail"},
		{http.MethodPost, "/runs/1/review"}, {http.MethodPost, "/runs/1/baseline"},
	} {
		t.Run(route.method+route.path, func(t *testing.T) {
			for _, credential := range []struct {
				token  string
				status int
			}{{"", http.StatusUnauthorized}, {token, http.StatusForbidden}} {
				request := httptest.NewRequest(route.method, "/api/quality-detection"+route.path, strings.NewReader("{}"))
				if credential.token != "" {
					request.Header.Set("Authorization", "Bearer "+credential.token)
				}
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, request)
				assert.Equal(t, credential.status, response.Code)
			}
		})
	}
	// The quality tables intentionally do not exist: authorization must run
	// before configuration, artifacts or paid generation handlers are reached.
}

func TestStatusRoutesEnforceNavigationVisibilityAndAuthentication(t *testing.T) {
	engine := qualityRouterFixture(t)
	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})
	for _, config := range []struct {
		name, value string
		status      int
	}{
		{"disabled module", `{"status":{"enabled":false}}`, http.StatusForbidden},
		{"login required", `{"status":{"enabled":true,"requireAuth":true}}`, http.StatusUnauthorized},
	} {
		t.Run(config.name, func(t *testing.T) {
			common.OptionMapRWMutex.Lock()
			common.OptionMap["HeaderNavModules"] = config.value
			common.OptionMapRWMutex.Unlock()
			for _, path := range []string{"/api/status-page", "/api/status-page/groups/public", "/api/status-page/artifacts/public/thumbnail"} {
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
				assert.Equal(t, config.status, response.Code, path)
			}
		})
	}
}

func TestRetiredUptimeRouteReturnsGoneWithoutExternalDependency(t *testing.T) {
	engine := qualityRouterFixture(t)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/uptime/status", nil))
	assert.Equal(t, http.StatusGone, response.Code)
	assert.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	assert.Contains(t, response.Body.String(), "/api/status-page")
}
