package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateTopUpCreditAmountSupportsBigintWalletCredit(t *testing.T) {
	generalSetting := operation_setting.GetGeneralSetting()
	originalDisplayType := generalSetting.QuotaDisplayType
	generalSetting.QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	t.Cleanup(func() {
		generalSetting.QuotaDisplayType = originalDisplayType
	})

	quota, err := calculateTopUpCreditAmount(10_000)
	require.NoError(t, err)
	assert.Equal(t, int64(5_000_000_000), quota)
}
