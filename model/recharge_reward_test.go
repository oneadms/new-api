package model

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func resetRechargeRewardTestData(t *testing.T, userIds ...int) {
	t.Helper()
	require.NoError(t, DB.Exec("DELETE FROM recharge_lottery_draws").Error)
	require.NoError(t, DB.Exec("DELETE FROM recharge_reward_events").Error)
	require.NoError(t, DB.Exec("DELETE FROM user_group_passes").Error)
	require.NoError(t, DB.Exec("DELETE FROM recharge_reward_configs").Error)
	for _, userId := range userIds {
		require.NoError(t, DB.Where("user_id = ?", userId).Delete(&TopUp{}).Error)
		require.NoError(t, DB.Unscoped().Delete(&User{}, userId).Error)
	}
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"speed":0.2}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	})
}

func validRechargeRewardSettings() RechargeRewardSettings {
	return RechargeRewardSettings{
		GroupPassEnabled:        true,
		LotteryEnabled:          true,
		LotteryMinRechargeQuota: 500_000,
		LotteryDrawsPerRecharge: 2,
		GroupPassTemplates: []GroupPassTemplate{
			{Id: "speed-hour", Name: "Speed hour", GroupName: "speed", DurationMinutes: 60, ValidDays: 30, Enabled: true},
		},
		RechargeRewardRules: []RechargeRewardRule{
			{Id: "recharge-1", Name: "Recharge reward", MinRechargeQuota: 500_000, TemplateId: "speed-hour", Quantity: 1, Enabled: true},
		},
		LotteryPrizes: []RechargeLotteryPrize{
			{Id: "quota-1", Name: "Fixed quota", Type: LotteryPrizeTypeQuota, ProbabilityBps: 5000, MinQuota: 1000, MaxQuota: 2000, Enabled: true},
			{Id: "pass-1", Name: "Speed pass", Type: LotteryPrizeTypeGroupPass, ProbabilityBps: 2500, TemplateId: "speed-hour", Quantity: 1, Enabled: true},
		},
	}
}

func TestSaveRechargeRewardSettingsRejectsUnsafeConfigurationAndStaleWrites(t *testing.T) {
	resetRechargeRewardTestData(t)
	settings := validRechargeRewardSettings()

	saved, err := SaveRechargeRewardSettings(settings)
	require.NoError(t, err)
	assert.Equal(t, int64(1), saved.Version)

	_, err = SaveRechargeRewardSettings(settings)
	assert.ErrorIs(t, err, ErrRechargeRewardConfigConflict)

	overweight := saved
	overweight.LotteryPrizes[0].ProbabilityBps = 8000
	overweight.LotteryPrizes[1].ProbabilityBps = 3000
	_, err = SaveRechargeRewardSettings(overweight)
	assert.EqualError(t, err, "启用奖品的总概率不能超过 100%")

	invalidQuota := saved
	invalidQuota.LotteryPrizes[0].ProbabilityBps = 5000
	invalidQuota.LotteryPrizes[1].ProbabilityBps = 2500
	invalidQuota.LotteryPrizes[0].MinQuota = 2000
	invalidQuota.LotteryPrizes[0].MaxQuota = 1000
	_, err = SaveRechargeRewardSettings(invalidQuota)
	assert.EqualError(t, err, "额度奖品范围无效")

	disabledTemplate := validRechargeRewardSettings()
	disabledTemplate.GroupPassTemplates[0].Enabled = false
	_, err = SaveRechargeRewardSettings(disabledTemplate)
	assert.EqualError(t, err, `启用的充值奖励规则引用了未启用的模板 "speed-hour"`)

	disabledTemplate.RechargeRewardRules[0].Enabled = false
	_, err = SaveRechargeRewardSettings(disabledTemplate)
	assert.EqualError(t, err, `启用的抽奖奖品引用了未启用的模板 "speed-hour"`)
}

func TestApplyRechargeRewardsIsIdempotentPerTopUp(t *testing.T) {
	const userId = 31001
	resetRechargeRewardTestData(t, userId)
	_, err := SaveRechargeRewardSettings(validRechargeRewardSettings())
	require.NoError(t, err)
	require.NoError(t, DB.Create(&User{Id: userId, Username: "reward-user", AffCode: "reward-aff-31001", Quota: 0, Status: common.UserStatusEnabled}).Error)
	topUp := TopUp{UserId: userId, Amount: 2, Money: 2, TradeNo: "reward-idempotency", Status: common.TopUpStatusSuccess}
	require.NoError(t, DB.Create(&topUp).Error)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		if err := applyRechargeRewardsTx(tx, &topUp, 1_000_000); err != nil {
			return err
		}
		return applyRechargeRewardsTx(tx, &topUp, 1_000_000)
	}))

	var eventCount int64
	require.NoError(t, DB.Model(&RechargeRewardEvent{}).Where("top_up_id = ?", topUp.Id).Count(&eventCount).Error)
	assert.Equal(t, int64(1), eventCount)
	var passCount int64
	require.NoError(t, DB.Model(&UserGroupPass{}).Where("user_id = ?", userId).Count(&passCount).Error)
	assert.Equal(t, int64(1), passCount)
	var event RechargeRewardEvent
	require.NoError(t, DB.Where("top_up_id = ?", topUp.Id).First(&event).Error)
	assert.Equal(t, 2, event.LotteryDraws)
	assert.Equal(t, 1, event.GrantedCards)
}

func TestRechargeEpayCommitsQuotaAndRewardsExactlyOnce(t *testing.T) {
	const userId = 31002
	resetRechargeRewardTestData(t, userId)
	settings := validRechargeRewardSettings()
	settings.LotteryDrawsPerRecharge = 1
	_, err := SaveRechargeRewardSettings(settings)
	require.NoError(t, err)
	require.NoError(t, DB.Create(&User{Id: userId, Username: "epay-reward-user", AffCode: "reward-aff-31002", Quota: 100, Status: common.UserStatusEnabled}).Error)
	topUp := TopUp{
		UserId: userId, Amount: 2, Money: 2, TradeNo: "epay-reward-order",
		PaymentMethod: "alipay", PaymentProvider: PaymentProviderEpay,
		CreateTime: common.GetTimestamp(), Status: common.TopUpStatusPending,
	}
	require.NoError(t, DB.Create(&topUp).Error)
	expectedQuota := common.QuotaFromDecimal(decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)))

	require.NoError(t, RechargeEpay(topUp.TradeNo, "wechat", "127.0.0.1"))
	require.NoError(t, RechargeEpay(topUp.TradeNo, "wechat", "127.0.0.1"))

	var user User
	require.NoError(t, DB.First(&user, userId).Error)
	assert.Equal(t, 100+expectedQuota, user.Quota)
	var stored TopUp
	require.NoError(t, DB.First(&stored, topUp.Id).Error)
	assert.Equal(t, common.TopUpStatusSuccess, stored.Status)
	assert.Equal(t, "wechat", stored.PaymentMethod)
	var eventCount int64
	require.NoError(t, DB.Model(&RechargeRewardEvent{}).Where("top_up_id = ?", topUp.Id).Count(&eventCount).Error)
	assert.Equal(t, int64(1), eventCount)
	var passCount int64
	require.NoError(t, DB.Model(&UserGroupPass{}).Where("user_id = ?", userId).Count(&passCount).Error)
	assert.Equal(t, int64(1), passCount)
}

func TestRechargeEpayRollsBackWhenWalletCreditWouldOverflow(t *testing.T) {
	const userId = 31006
	resetRechargeRewardTestData(t, userId)
	_, err := SaveRechargeRewardSettings(validRechargeRewardSettings())
	require.NoError(t, err)
	require.NoError(t, DB.Create(&User{
		Id: userId, Username: "epay-limit-user", AffCode: "reward-aff-31006",
		Quota: common.MaxQuota - 100, Status: common.UserStatusEnabled,
	}).Error)
	topUp := TopUp{
		UserId: userId, Amount: 1, Money: 1, TradeNo: "epay-limit-order",
		PaymentMethod: "alipay", PaymentProvider: PaymentProviderEpay,
		CreateTime: common.GetTimestamp(), Status: common.TopUpStatusPending,
	}
	require.NoError(t, DB.Create(&topUp).Error)

	err = RechargeEpay(topUp.TradeNo, "wechat", "127.0.0.1")
	assert.ErrorIs(t, err, ErrUserQuotaLimitExceeded)

	var user User
	require.NoError(t, DB.First(&user, userId).Error)
	assert.Equal(t, common.MaxQuota-100, user.Quota)
	var stored TopUp
	require.NoError(t, DB.First(&stored, topUp.Id).Error)
	assert.Equal(t, common.TopUpStatusPending, stored.Status)
	assert.Equal(t, "alipay", stored.PaymentMethod)
	var eventCount int64
	require.NoError(t, DB.Model(&RechargeRewardEvent{}).Where("top_up_id = ?", topUp.Id).Count(&eventCount).Error)
	assert.Zero(t, eventCount)
}

func TestDrawRechargeLotteryConsumesOneChanceAndAwardsBoundedQuota(t *testing.T) {
	const userId = 31003
	resetRechargeRewardTestData(t, userId)
	settings := validRechargeRewardSettings()
	settings.RechargeRewardRules = []RechargeRewardRule{}
	settings.LotteryPrizes = []RechargeLotteryPrize{
		{Id: "certain", Name: "Certain quota", Type: LotteryPrizeTypeQuota, ProbabilityBps: 10000, MinQuota: 1234, MaxQuota: 1234, Enabled: true},
	}
	saved, err := SaveRechargeRewardSettings(settings)
	require.NoError(t, err)
	require.NoError(t, DB.Create(&User{Id: userId, Username: "lottery-user", AffCode: "reward-aff-31003", Quota: 100, Status: common.UserStatusEnabled}).Error)
	require.NoError(t, DB.Create(&RechargeRewardEvent{
		TopUpId: 70001, UserId: userId, RechargeQuota: 1_000_000, LotteryDraws: 1,
		ConfigVersion: saved.Version, CreatedAt: common.GetTimestamp(),
	}).Error)

	result, err := DrawRechargeLottery(userId)
	require.NoError(t, err)
	assert.Equal(t, int64(1234), result.Draw.QuotaAwarded)
	assert.Equal(t, LotteryPrizeTypeQuota, result.Draw.PrizeType)
	var user User
	require.NoError(t, DB.First(&user, userId).Error)
	assert.Equal(t, 1334, user.Quota)
	_, err = DrawRechargeLottery(userId)
	assert.ErrorIs(t, err, ErrNoLotteryChance)
}

func TestDrawRechargeLotteryPreservesChanceWhenWalletCreditWouldOverflow(t *testing.T) {
	const userId = 31007
	resetRechargeRewardTestData(t, userId)
	settings := validRechargeRewardSettings()
	settings.RechargeRewardRules = []RechargeRewardRule{}
	settings.LotteryPrizes = []RechargeLotteryPrize{
		{Id: "certain-limit", Name: "Certain quota", Type: LotteryPrizeTypeQuota, ProbabilityBps: 10000, MinQuota: 1234, MaxQuota: 1234, Enabled: true},
	}
	saved, err := SaveRechargeRewardSettings(settings)
	require.NoError(t, err)
	require.NoError(t, DB.Create(&User{
		Id: userId, Username: "lottery-limit-user", AffCode: "reward-aff-31007",
		Quota: common.MaxQuota - 100, Status: common.UserStatusEnabled,
	}).Error)
	event := RechargeRewardEvent{
		TopUpId: 70002, UserId: userId, RechargeQuota: 1_000_000, LotteryDraws: 1,
		ConfigVersion: saved.Version, CreatedAt: common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(&event).Error)

	_, err = DrawRechargeLottery(userId)
	assert.ErrorIs(t, err, ErrUserQuotaLimitExceeded)

	var user User
	require.NoError(t, DB.First(&user, userId).Error)
	assert.Equal(t, common.MaxQuota-100, user.Quota)
	require.NoError(t, DB.First(&event, event.Id).Error)
	assert.Zero(t, event.UsedDraws)
	var drawCount int64
	require.NoError(t, DB.Model(&RechargeLotteryDraw{}).Where("reward_event_id = ?", event.Id).Count(&drawCount).Error)
	assert.Zero(t, drawCount)

	require.NoError(t, DB.Model(&User{}).Where("id = ?", userId).Update("quota", 0).Error)
	result, err := DrawRechargeLottery(userId)
	require.NoError(t, err)
	assert.Equal(t, int64(1234), result.Draw.QuotaAwarded)
}

func TestGroupPassActivationControlsTemporaryGroupAccess(t *testing.T) {
	const userId = 31004
	resetRechargeRewardTestData(t, userId)
	settings := validRechargeRewardSettings()
	settings.LotteryEnabled = false
	settings.LotteryDrawsPerRecharge = 0
	settings.LotteryPrizes = []RechargeLotteryPrize{}
	_, err := SaveRechargeRewardSettings(settings)
	require.NoError(t, err)
	require.NoError(t, DB.Create(&User{Id: userId, Username: "pass-user", AffCode: "reward-aff-31004", Status: common.UserStatusEnabled}).Error)

	passes, err := GrantUserGroupPasses(GroupPassGrantRequest{UserId: userId, TemplateId: "speed-hour", Quantity: 2})
	require.NoError(t, err)
	require.Len(t, passes, 2)
	activated, err := ActivateUserGroupPass(userId, passes[0].Id)
	require.NoError(t, err)
	assert.Equal(t, GroupPassStatusActive, activated.Status)
	assert.Equal(t, int64(60*60), activated.ActiveUntil-activated.ActivatedAt)

	access, err := GetActiveGroupPassAccess(userId)
	require.NoError(t, err)
	assert.Greater(t, access["speed"], common.GetTimestamp())
	_, err = ActivateUserGroupPass(userId, passes[1].Id)
	assert.ErrorIs(t, err, ErrGroupPassAlreadyActive)

	require.NoError(t, DB.Model(&UserGroupPass{}).Where("id = ?", activated.Id).Update("active_until", common.GetTimestamp()-1).Error)
	_, err = ActivateUserGroupPass(userId, passes[1].Id)
	require.NoError(t, err)
}

func TestSelectLotteryPrizeUsesAbsoluteBasisPointRanges(t *testing.T) {
	prizes := []RechargeLotteryPrize{
		{Id: "first", Enabled: true, ProbabilityBps: 1000},
		{Id: "disabled", Enabled: false, ProbabilityBps: 8000},
		{Id: "second", Enabled: true, ProbabilityBps: 2000},
	}
	testCases := []struct {
		roll int
		want string
	}{
		{roll: 0, want: "first"},
		{roll: 999, want: "first"},
		{roll: 1000, want: "second"},
		{roll: 2999, want: "second"},
		{roll: 3000, want: ""},
		{roll: 9999, want: ""},
	}
	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("roll_%d", testCase.roll), func(t *testing.T) {
			selected := selectLotteryPrize(prizes, testCase.roll)
			if testCase.want == "" {
				assert.Nil(t, selected)
				return
			}
			require.NotNil(t, selected)
			assert.Equal(t, testCase.want, selected.Id)
		})
	}
}

func TestExpiredGroupPassCannotBeActivated(t *testing.T) {
	const userId = 31005
	resetRechargeRewardTestData(t, userId)
	settings := validRechargeRewardSettings()
	settings.LotteryEnabled = false
	settings.LotteryDrawsPerRecharge = 0
	settings.LotteryPrizes = []RechargeLotteryPrize{}
	_, err := SaveRechargeRewardSettings(settings)
	require.NoError(t, err)
	require.NoError(t, DB.Create(&User{Id: userId, Username: "expired-pass-user", AffCode: "reward-aff-31005", Status: common.UserStatusEnabled}).Error)
	pass := UserGroupPass{
		UserId: userId, TemplateId: "speed-hour", Name: "Expired", GroupName: "speed",
		DurationMinutes: 60, Status: GroupPassStatusUnused, ExpiresAt: common.GetTimestamp() - 1,
		SourceType: "test", SourceId: "expired-test", CreatedAt: common.GetTimestamp() - 100,
	}
	require.NoError(t, DB.Create(&pass).Error)

	_, err = ActivateUserGroupPass(userId, pass.Id)
	assert.ErrorIs(t, err, ErrGroupPassUnavailable)
	assert.Equal(t, "expired", expireGroupPassStatus(pass, time.Now()))
	assert.False(t, errors.Is(err, gorm.ErrRecordNotFound))
	rewards, err := GetUserRechargeRewards(userId)
	require.NoError(t, err)
	require.Len(t, rewards.GroupPasses, 1)
	assert.Equal(t, GroupPassStatusExpired, rewards.GroupPasses[0].Status)
}
