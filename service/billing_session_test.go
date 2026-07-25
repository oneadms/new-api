package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
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
