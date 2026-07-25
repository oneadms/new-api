package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAdminUpdateSubscriptionPlanHandlesRestrictedGroups(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.SubscriptionPlan{}))
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.RedisEnabled = previousRedisEnabled
	})

	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1}`))

	plan := model.SubscriptionPlan{
		Title:         "restricted plan",
		Currency:      "USD",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
	}
	require.NoError(t, db.Create(&plan).Error)
	planPayload := gin.H{
		"title":                        plan.Title,
		"currency":                     "USD",
		"duration_unit":                model.SubscriptionDurationMonth,
		"duration_value":               1,
		"enabled":                      true,
		"restricted_groups":            []string{" vip ", "default", "vip"},
		"subscription_disabled_groups": []string{" vip ", "default", "vip"},
		"quota_reset_period":           model.SubscriptionResetNever,
		"allow_balance_pay":            true,
		"allow_wallet_overflow":        true,
	}
	payload, err := common.Marshal(gin.H{"plan": planPayload})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/subscription/admin/plans/"+strconv.Itoa(plan.Id), bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(plan.Id)}}

	AdminUpdateSubscriptionPlan(ctx)

	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success, response.Message)
	var updated model.SubscriptionPlan
	require.NoError(t, db.First(&updated, plan.Id).Error)
	assert.Equal(t, []string{"default", "vip"}, updated.RestrictedGroups)
	assert.Equal(t, []string{"default", "vip"}, updated.SubscriptionDisabledGroups)

	delete(planPayload, "restricted_groups")
	planPayload["title"] = "legacy client update"
	payload, err = common.Marshal(gin.H{"plan": planPayload})
	require.NoError(t, err)
	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/subscription/admin/plans/"+strconv.Itoa(plan.Id), bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(plan.Id)}}

	AdminUpdateSubscriptionPlan(ctx)

	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success, response.Message)
	require.NoError(t, db.First(&updated, plan.Id).Error)
	assert.Equal(t, []string{"default", "vip"}, updated.RestrictedGroups)
	assert.Equal(t, []string{"default", "vip"}, updated.SubscriptionDisabledGroups)

	planPayload["restricted_groups"] = []string{}
	planPayload["subscription_disabled_groups"] = []string{}
	payload, err = common.Marshal(gin.H{"plan": planPayload})
	require.NoError(t, err)
	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/subscription/admin/plans/"+strconv.Itoa(plan.Id), bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(plan.Id)}}

	AdminUpdateSubscriptionPlan(ctx)

	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success, response.Message)
	require.NoError(t, db.First(&updated, plan.Id).Error)
	assert.Empty(t, updated.RestrictedGroups)
	assert.Empty(t, updated.SubscriptionDisabledGroups)
}

func TestNormalizeSubscriptionRestrictedGroupsRejectsUnknownGroup(t *testing.T) {
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))

	groups, err := normalizeSubscriptionRestrictedGroups([]string{"missing"})

	assert.Nil(t, groups)
	assert.EqualError(t, err, "限制分组 missing 不存在")
}

func TestNormalizeSubscriptionRestrictedGroupsAllowsAuto(t *testing.T) {
	groups, err := normalizeSubscriptionRestrictedGroups([]string{" auto ", "auto"})

	require.NoError(t, err)
	assert.Equal(t, []string{"auto"}, groups)
}
