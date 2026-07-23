package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDistributorRejectsPlaygroundGroupRestrictedBySubscription(t *testing.T) {
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.SubscriptionPlan{},
		&model.UserSubscription{},
		&model.RechargeRewardConfig{},
		&model.UserGroupPass{},
	))
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
	})

	originalUsableGroups := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	require.NoError(t, i18n.Init())

	plan := model.SubscriptionPlan{
		Title:            "playground restriction plan",
		DurationUnit:     model.SubscriptionDurationMonth,
		DurationValue:    1,
		RestrictedGroups: []string{"vip"},
	}
	require.NoError(t, db.Create(&plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	now := common.GetTimestamp()
	require.NoError(t, db.Create(&model.UserSubscription{
		UserId: 2401, PlanId: plan.Id, Status: "active", StartTime: now - 60, EndTime: now + 3600,
	}).Error)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/pg/chat/completions",
		strings.NewReader(`{"model":"gpt-test","group":"vip"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 2401)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "default")

	Distribute()(ctx)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "No permission to access this group")
}

func TestDistributorAllowsPlaygroundOverrideWhenCurrentGroupIsRestricted(t *testing.T) {
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.SubscriptionPlan{},
		&model.UserSubscription{},
		&model.RechargeRewardConfig{},
		&model.UserGroupPass{},
	))
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
	})

	originalUsableGroups := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	require.NoError(t, i18n.Init())

	plan := model.SubscriptionPlan{
		Title:            "current group restriction plan",
		DurationUnit:     model.SubscriptionDurationMonth,
		DurationValue:    1,
		RestrictedGroups: []string{"default"},
	}
	require.NoError(t, db.Create(&plan).Error)
	now := common.GetTimestamp()
	require.NoError(t, db.Create(&model.UserSubscription{
		UserId: 2402, PlanId: plan.Id, Status: "active", StartTime: now - 60, EndTime: now + 3600,
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/pg/chat/completions",
		strings.NewReader(`{"model":"gpt-test","group":"vip"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 2402)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "default")

	Distribute()(ctx)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "No permission to access this group")
}
