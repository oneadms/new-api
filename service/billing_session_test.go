package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBillingSessionUsesWalletForSubscriptionDisabledGroup(t *testing.T) {
	truncate(t)
	seedUser(t, 2401, 1_000_000)
	ctx, _ := gin.CreateTestContext(nil)
	common.SetContextKey(ctx, constant.ContextKeySubscriptionDisabledGroups, map[string]struct{}{"vip": {}})

	info := &relaycommon.RelayInfo{
		UserId:       2401,
		UsingGroup:   "vip",
		IsPlayground: true,
		UserSetting: dto.UserSetting{
			BillingPreference: "subscription_first",
		},
	}

	session, apiErr := NewBillingSession(ctx, info, 100)

	require.Nil(t, apiErr)
	require.NotNil(t, session)
	assert.Equal(t, BillingSourceWallet, info.BillingSource)
}

func TestNewBillingSessionUsesWalletForGloballyDisabledGroup(t *testing.T) {
	original := setting.SubscriptionDisabledGroups2JSONString()
	require.NoError(t, setting.UpdateSubscriptionDisabledGroupsByJSONString(`["vip"]`))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateSubscriptionDisabledGroupsByJSONString(original))
	})

	truncate(t)
	seedUser(t, 2403, 1_000_000)
	seedSubscription(t, 3403, 2403, 10_000, 0)
	ctx, _ := gin.CreateTestContext(nil)
	info := &relaycommon.RelayInfo{
		UserId:       2403,
		UsingGroup:   "vip",
		IsPlayground: true,
		UserSetting: dto.UserSetting{
			BillingPreference: "subscription_first",
		},
	}

	session, apiErr := NewBillingSession(ctx, info, 100)

	require.Nil(t, apiErr)
	require.NotNil(t, session)
	assert.Equal(t, BillingSourceWallet, info.BillingSource)

	var user model.User
	require.NoError(t, model.DB.First(&user, 2403).Error)
	assert.Equal(t, 999_900, user.Quota)
	var subscription model.UserSubscription
	require.NoError(t, model.DB.First(&subscription, 3403).Error)
	assert.Zero(t, subscription.AmountUsed)
}

func TestNewBillingSessionUsesWalletWhenAutoGroupIsGloballyDisabled(t *testing.T) {
	original := setting.SubscriptionDisabledGroups2JSONString()
	require.NoError(t, setting.UpdateSubscriptionDisabledGroupsByJSONString(`["auto"]`))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateSubscriptionDisabledGroupsByJSONString(original))
	})

	truncate(t)
	seedUser(t, 2404, 1_000_000)
	ctx, _ := gin.CreateTestContext(nil)
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "auto")
	info := &relaycommon.RelayInfo{
		UserId:       2404,
		UsingGroup:   "vip",
		IsPlayground: true,
		UserSetting: dto.UserSetting{
			BillingPreference: "subscription_first",
		},
	}

	session, apiErr := NewBillingSession(ctx, info, 100)

	require.Nil(t, apiErr)
	require.NotNil(t, session)
	assert.Equal(t, BillingSourceWallet, info.BillingSource)
}

func TestNewBillingSessionUsesWalletWhenAutoGroupIsSubscriptionDisabled(t *testing.T) {
	truncate(t)
	seedUser(t, 2402, 1_000_000)
	ctx, _ := gin.CreateTestContext(nil)
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "auto")
	common.SetContextKey(ctx, constant.ContextKeySubscriptionDisabledGroups, map[string]struct{}{"auto": {}})

	info := &relaycommon.RelayInfo{
		UserId:       2402,
		UsingGroup:   "vip",
		IsPlayground: true,
		UserSetting: dto.UserSetting{
			BillingPreference: "subscription_first",
		},
	}

	session, apiErr := NewBillingSession(ctx, info, 100)

	require.Nil(t, apiErr)
	require.NotNil(t, session)
	assert.Equal(t, BillingSourceWallet, info.BillingSource)
}

func TestNewBillingSessionRejectsSubscriptionOnlyForDisabledGroup(t *testing.T) {
	ctx, _ := gin.CreateTestContext(nil)
	common.SetContextKey(ctx, constant.ContextKeySubscriptionDisabledGroups, map[string]struct{}{"vip": {}})
	info := &relaycommon.RelayInfo{
		UsingGroup: "vip",
		UserSetting: dto.UserSetting{
			BillingPreference: "subscription_only",
		},
	}

	session, apiErr := NewBillingSession(ctx, info, 100)

	require.Nil(t, session)
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
}
