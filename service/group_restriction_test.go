package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionRestrictionsFilterSelectableAndAutomaticGroups(t *testing.T) {
	truncate(t)
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalAutoGroups := setting.AutoGroups2JsonString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"auto":"Automatic","default":"Default","vip":"VIP"}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["vip","default"]`))

	now := common.GetTimestamp()
	plan := model.SubscriptionPlan{
		Title:            "restricted auto plan",
		DurationUnit:     model.SubscriptionDurationMonth,
		DurationValue:    1,
		RestrictedGroups: []string{"vip"},
	}
	require.NoError(t, model.DB.Create(&plan).Error)
	require.NoError(t, model.DB.Create(&model.UserSubscription{
		UserId:    2301,
		PlanId:    plan.Id,
		Status:    "active",
		StartTime: now - 60,
		EndTime:   now + 3600,
	}).Error)

	ctx, _ := gin.CreateTestContext(nil)
	ctx.Set("id", 2301)
	groups, err := ResolveUserUsableGroups(ctx, "default")
	require.NoError(t, err)
	assert.Contains(t, groups, "default")
	assert.Contains(t, groups, "auto")
	assert.NotContains(t, groups, "vip")
	assert.Equal(t, []string{"default"}, GetUserAutoGroupFromUsableGroups(groups))
}

func TestSubscriptionRestrictionsCanDisableAutoSelection(t *testing.T) {
	truncate(t)
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"auto":"Automatic","default":"Default"}`))

	plan := model.SubscriptionPlan{
		Title:            "disable automatic selection plan",
		DurationUnit:     model.SubscriptionDurationMonth,
		DurationValue:    1,
		RestrictedGroups: []string{"auto"},
	}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	require.NoError(t, model.DB.Create(&model.UserSubscription{
		UserId: 2302, PlanId: plan.Id, Status: "active", StartTime: now - 60, EndTime: now + 3600,
	}).Error)

	ctx, _ := gin.CreateTestContext(nil)
	ctx.Set("id", 2302)
	groups, err := ResolveUserUsableGroups(ctx, "default")
	require.NoError(t, err)
	assert.Contains(t, groups, "default")
	assert.NotContains(t, groups, "auto")
}

func TestActiveGroupPassTemporarilyRestoresRestrictedGroup(t *testing.T) {
	truncate(t)
	require.NoError(t, model.DB.Exec("DELETE FROM recharge_reward_configs").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM user_group_passes").Error)
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, model.DB.Exec("DELETE FROM recharge_reward_configs").Error)
		require.NoError(t, model.DB.Exec("DELETE FROM user_group_passes").Error)
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	_, err := model.SaveRechargeRewardSettings(model.RechargeRewardSettings{
		GroupPassEnabled: true,
		GroupPassTemplates: []model.GroupPassTemplate{
			{Id: "vip-hour", Name: "VIP hour", GroupName: "vip", DurationMinutes: 60, ValidDays: 30, Enabled: true},
		},
		RechargeRewardRules: []model.RechargeRewardRule{},
		LotteryPrizes:       []model.RechargeLotteryPrize{},
	})
	require.NoError(t, err)

	now := common.GetTimestamp()
	plan := model.SubscriptionPlan{
		Title: "pass restricted plan", DurationUnit: model.SubscriptionDurationMonth,
		DurationValue: 1, RestrictedGroups: []string{"vip"},
	}
	require.NoError(t, model.DB.Create(&plan).Error)
	require.NoError(t, model.DB.Create(&model.UserSubscription{
		UserId: 2303, PlanId: plan.Id, Status: "active", StartTime: now - 60, EndTime: now + 3600,
	}).Error)
	require.NoError(t, model.DB.Create(&model.UserGroupPass{
		UserId: 2303, TemplateId: "vip-hour", Name: "VIP hour", GroupName: "vip",
		DurationMinutes: 60, Status: model.GroupPassStatusActive, ExpiresAt: now + 86400,
		ActivatedAt: now - 60, ActiveUntil: now + 3540, SourceType: "test", SourceId: "service-group-pass", CreatedAt: now - 60,
	}).Error)

	ctx, _ := gin.CreateTestContext(nil)
	ctx.Set("id", 2303)
	groups, err := ResolveUserUsableGroups(ctx, "default")
	require.NoError(t, err)
	assert.Contains(t, groups, "vip")
	assert.Equal(t, "vip", common.GetContextKeyString(ctx, constant.ContextKeyGroupPassGroup))
	assert.Equal(t, "vip", ApplyActiveGroupPass(ctx, "default"))

	require.NoError(t, model.DB.Model(&model.UserGroupPass{}).
		Where("user_id = ?", 2303).
		Update("active_until", now-1).Error)
	expiredCtx, _ := gin.CreateTestContext(nil)
	expiredCtx.Set("id", 2303)
	groups, err = ResolveUserUsableGroups(expiredCtx, "default")
	require.NoError(t, err)
	assert.NotContains(t, groups, "vip")
	assert.Empty(t, common.GetContextKeyString(expiredCtx, constant.ContextKeyGroupPassGroup))
	assert.Equal(t, "default", ApplyActiveGroupPass(expiredCtx, "default"))
}
