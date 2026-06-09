package model

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetBattleTestTables(t *testing.T) {
	t.Helper()

	clear := func() {
		require.NoError(t, DB.Exec("DELETE FROM battle_records").Error)
		require.NoError(t, DB.Exec("DELETE FROM logs").Error)
		require.NoError(t, DB.Exec("DELETE FROM users").Error)
	}
	clear()
	t.Cleanup(clear)
}

func createBattleTestUser(t *testing.T, id int, quota int) {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id:       id,
		Username: fmt.Sprintf("battle-user-%d", id),
		Password: "battle-test-password",
		Status:   common.UserStatusEnabled,
		Quota:    quota,
	}).Error)
}

func battleTestUserQuota(t *testing.T, id int) int {
	t.Helper()
	var quota int
	require.NoError(t, DB.Model(&User{}).Where("id = ?", id).Select("quota").Scan(&quota).Error)
	return quota
}

func TestDebitBattleQuota(t *testing.T) {
	resetBattleTestTables(t)
	createBattleTestUser(t, 1, 1000)

	record, err := DebitBattleQuota(BattleQuotaMutationParams{
		RoomId:  "room-a",
		EventId: "debit-a",
		UserId:  1,
		Quota:   250,
		Reason:  "drop",
	})
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.Equal(t, 750, battleTestUserQuota(t, 1))
	assert.Equal(t, 1, record.FromUserId)
	assert.Zero(t, record.ToUserId)
	assert.Equal(t, 250, record.Quota)

	var recordCount int64
	require.NoError(t, DB.Model(&BattleRecord{}).Count(&recordCount).Error)
	assert.Equal(t, int64(1), recordCount)

	var logCount int64
	require.NoError(t, LOG_DB.Model(&Log{}).Where("user_id = ?", 1).Count(&logCount).Error)
	assert.Equal(t, int64(1), logCount)
}

func TestDebitBattleQuotaInsufficientBalanceRollsBack(t *testing.T) {
	resetBattleTestTables(t)
	createBattleTestUser(t, 1, 100)

	_, err := DebitBattleQuota(BattleQuotaMutationParams{
		RoomId:  "room-a",
		EventId: "debit-too-large",
		UserId:  1,
		Quota:   101,
		Reason:  "drop",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrBattleQuotaInsufficient))
	assert.Equal(t, 100, battleTestUserQuota(t, 1))

	var recordCount int64
	require.NoError(t, DB.Model(&BattleRecord{}).Count(&recordCount).Error)
	assert.Zero(t, recordCount)
}

func TestCreditBattleQuota(t *testing.T) {
	resetBattleTestTables(t)
	createBattleTestUser(t, 2, 100)

	record, err := CreditBattleQuota(BattleQuotaMutationParams{
		RoomId:  "room-a",
		EventId: "credit-a",
		UserId:  2,
		Quota:   75,
		Reason:  "pickup",
	})
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.Equal(t, 175, battleTestUserQuota(t, 2))
	assert.Zero(t, record.FromUserId)
	assert.Equal(t, 2, record.ToUserId)
	assert.Equal(t, 75, record.Quota)
}

func TestCreditBattleQuotaMissingUserRollsBack(t *testing.T) {
	resetBattleTestTables(t)

	_, err := CreditBattleQuota(BattleQuotaMutationParams{
		RoomId:  "room-a",
		EventId: "credit-missing",
		UserId:  99,
		Quota:   75,
		Reason:  "pickup",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrBattleUserNotFound))

	var recordCount int64
	require.NoError(t, DB.Model(&BattleRecord{}).Count(&recordCount).Error)
	assert.Zero(t, recordCount)
}

func TestBattleQuotaDuplicateEventDoesNotDoubleDebit(t *testing.T) {
	resetBattleTestTables(t)
	createBattleTestUser(t, 1, 1000)

	params := BattleQuotaMutationParams{
		RoomId:  "room-a",
		EventId: "same-debit-event",
		UserId:  1,
		Quota:   200,
		Reason:  "drop",
	}
	_, err := DebitBattleQuota(params)
	require.NoError(t, err)
	_, err = DebitBattleQuota(params)
	require.Error(t, err)

	assert.Equal(t, 800, battleTestUserQuota(t, 1))
	var recordCount int64
	require.NoError(t, DB.Model(&BattleRecord{}).Where("event_id = ?", params.EventId).Count(&recordCount).Error)
	assert.Equal(t, int64(1), recordCount)
}

func TestBattleQuotaLimitIsEnforcedInsideTransaction(t *testing.T) {
	resetBattleTestTables(t)
	createBattleTestUser(t, 1, 1000)
	start := BattleDailyUsageStart()

	_, err := DebitBattleQuota(BattleQuotaMutationParams{
		RoomId:  "room-a",
		EventId: "limited-debit-a",
		UserId:  1,
		Quota:   80,
		Reason:  "drop",
		UsageLimit: &BattleQuotaLimit{
			Since: start,
			Max:   100,
		},
	})
	require.NoError(t, err)

	_, err = DebitBattleQuota(BattleQuotaMutationParams{
		RoomId:  "room-b",
		EventId: "limited-debit-b",
		UserId:  1,
		Quota:   30,
		Reason:  "drop",
		UsageLimit: &BattleQuotaLimit{
			Since: start,
			Max:   100,
		},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrBattleQuotaLimitExceeded))
	assert.Equal(t, 920, battleTestUserQuota(t, 1))

	usage, err := GetBattleQuotaUsageSince(1, start)
	require.NoError(t, err)
	assert.Equal(t, BattleQuotaUsage{Lost: 80, Won: 0}, usage)
}

func TestBattleCreditLimitUsesWonQuota(t *testing.T) {
	resetBattleTestTables(t)
	createBattleTestUser(t, 2, 100)
	start := BattleDailyUsageStart()

	_, err := CreditBattleQuota(BattleQuotaMutationParams{
		RoomId:  "room-a",
		EventId: "limited-credit-a",
		UserId:  2,
		Quota:   80,
		Reason:  "pickup",
		UsageLimit: &BattleQuotaLimit{
			Since: start,
			Max:   100,
		},
	})
	require.NoError(t, err)

	_, err = CreditBattleQuota(BattleQuotaMutationParams{
		RoomId:  "room-b",
		EventId: "limited-credit-b",
		UserId:  2,
		Quota:   30,
		Reason:  "pickup",
		UsageLimit: &BattleQuotaLimit{
			Since: start,
			Max:   100,
		},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrBattleQuotaLimitExceeded))
	assert.Equal(t, 180, battleTestUserQuota(t, 2))

	usage, err := GetBattleQuotaUsageSince(2, start)
	require.NoError(t, err)
	assert.Equal(t, BattleQuotaUsage{Lost: 0, Won: 80}, usage)
}

func TestGetBattleDailyQuotaUsage(t *testing.T) {
	resetBattleTestTables(t)
	now := common.GetTimestamp()
	old := time.Now().Add(-48 * time.Hour).Unix()

	require.NoError(t, DB.Create([]BattleRecord{
		{CreatedAt: now, RoomId: "room-a", EventId: "today-loss", FromUserId: 1, Quota: 70},
		{CreatedAt: now, RoomId: "room-a", EventId: "today-win", ToUserId: 1, Quota: 30},
		{CreatedAt: old, RoomId: "room-a", EventId: "old-loss", FromUserId: 1, Quota: 900},
		{CreatedAt: old, RoomId: "room-a", EventId: "old-win", ToUserId: 1, Quota: 800},
	}).Error)

	usage, err := GetBattleDailyQuotaUsage(1)
	require.NoError(t, err)
	assert.Equal(t, BattleQuotaUsage{Lost: 70, Won: 30}, usage)
}
