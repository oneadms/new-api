package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestActiveSubscriptionRestrictedGroupsUseActivePlanUnion(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()

	plans := []SubscriptionPlan{
		{
			Title:                      "first restricted plan",
			DurationUnit:               SubscriptionDurationMonth,
			DurationValue:              1,
			RestrictedGroups:           []string{"vip", "shared"},
			SubscriptionDisabledGroups: []string{"wallet-only", "shared"},
		},
		{
			Title:                      "second restricted plan",
			DurationUnit:               SubscriptionDurationMonth,
			DurationValue:              1,
			Enabled:                    false,
			RestrictedGroups:           []string{"premium", "shared"},
			SubscriptionDisabledGroups: []string{"premium-wallet"},
		},
		{
			Title:            "inactive restricted plan",
			DurationUnit:     SubscriptionDurationMonth,
			DurationValue:    1,
			RestrictedGroups: []string{"expired-only"},
		},
		{
			Title:         "legacy unrestricted plan",
			DurationUnit:  SubscriptionDurationMonth,
			DurationValue: 1,
		},
	}
	require.NoError(t, DB.Create(&plans).Error)
	for _, plan := range plans {
		InvalidateSubscriptionPlanCache(plan.Id)
	}
	require.NoError(t, DB.Model(&plans[1]).Update("enabled", false).Error)
	require.NoError(t, DB.Model(&plans[3]).Update("restricted_groups", nil).Error)

	subscriptions := []UserSubscription{
		{UserId: 2101, PlanId: plans[0].Id, Status: "active", StartTime: now - 60, EndTime: now + 3600},
		{UserId: 2101, PlanId: plans[1].Id, Status: "active", StartTime: now - 60, EndTime: now + 7200},
		{UserId: 2101, PlanId: plans[2].Id, Status: "active", StartTime: now - 7200, EndTime: now - 1},
		{UserId: 2101, PlanId: plans[2].Id, Status: "cancelled", StartTime: now - 60, EndTime: now + 7200},
		{UserId: 2101, PlanId: plans[3].Id, Status: "active", StartTime: now - 60, EndTime: now + 7200},
		{UserId: 2102, PlanId: plans[2].Id, Status: "active", StartTime: now - 7200, EndTime: now - 1},
		{UserId: 2102, PlanId: plans[2].Id, Status: "cancelled", StartTime: now - 60, EndTime: now + 7200},
	}
	require.NoError(t, DB.Create(&subscriptions).Error)

	access, err := GetActiveSubscriptionGroupAccess(2101)
	require.NoError(t, err)
	assert.True(t, access.HasActiveSubscription)
	assert.Equal(t, map[string]struct{}{
		"premium": {},
		"shared":  {},
		"vip":     {},
	}, access.RestrictedGroups)
	assert.Equal(t, map[string]struct{}{
		"premium-wallet": {},
		"shared":         {},
		"wallet-only":    {},
	}, access.SubscriptionDisabledGroups)

	access, err = GetActiveSubscriptionGroupAccess(2102)
	require.NoError(t, err)
	assert.False(t, access.HasActiveSubscription)
	assert.Empty(t, access.RestrictedGroups)
	assert.Empty(t, access.SubscriptionDisabledGroups)
}

func TestActiveSubscriptionRestrictedGroupsFollowCurrentPlan(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	plan := SubscriptionPlan{
		Title:                      "historical subscription plan",
		DurationUnit:               SubscriptionDurationMonth,
		DurationValue:              1,
		RestrictedGroups:           []string{"before"},
		SubscriptionDisabledGroups: []string{"wallet-before"},
	}
	require.NoError(t, DB.Create(&plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	require.NoError(t, DB.Create(&UserSubscription{
		UserId:    2201,
		PlanId:    plan.Id,
		Status:    "active",
		StartTime: now - 3600,
		EndTime:   now + 3600,
	}).Error)

	access, err := GetActiveSubscriptionGroupAccess(2201)
	require.NoError(t, err)
	assert.Equal(t, map[string]struct{}{"before": {}}, access.RestrictedGroups)
	assert.Equal(t, map[string]struct{}{"wallet-before": {}}, access.SubscriptionDisabledGroups)

	plan.RestrictedGroups = []string{"after"}
	plan.SubscriptionDisabledGroups = []string{"wallet-after"}
	require.NoError(t, DB.Save(&plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)

	access, err = GetActiveSubscriptionGroupAccess(2201)
	require.NoError(t, err)
	assert.Equal(t, map[string]struct{}{"after": {}}, access.RestrictedGroups)
	assert.Equal(t, map[string]struct{}{"wallet-after": {}}, access.SubscriptionDisabledGroups)
}

func TestEnsureSubscriptionPlanTableSQLiteAddsRestrictedGroups(t *testing.T) {
	previousDB := DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
	})
	require.NoError(t, db.Exec("CREATE TABLE subscription_plans (id integer PRIMARY KEY)").Error)

	require.NoError(t, ensureSubscriptionPlanTableSQLite())

	assert.True(t, db.Migrator().HasColumn("subscription_plans", "restricted_groups"))
	assert.True(t, db.Migrator().HasColumn("subscription_plans", "subscription_disabled_groups"))
}
