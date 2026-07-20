package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

func TestFormatUserLogsHidesAdminOnlyOtherFields(t *testing.T) {
	logs := []*Log{
		{
			Id:          42,
			ChannelName: "channel-a",
			Other: common.MapToJsonStr(map[string]interface{}{
				"admin_info":         map[string]interface{}{"use_channel": []string{"1"}},
				"audit_info":         map[string]interface{}{"operator": "root"},
				"actual_model_ratio": 1.25,
				"model_ratio":        2.0,
				"stream_status":      map[string]interface{}{"status": "ok"},
			}),
		},
	}

	formatUserLogs(logs, 10)

	require.Equal(t, "", logs[0].ChannelName)
	require.Equal(t, 11, logs[0].Id)

	other, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.NotContains(t, other, "admin_info")
	require.NotContains(t, other, "audit_info")
	require.NotContains(t, other, "actual_model_ratio")
	require.NotContains(t, other, "stream_status")
	require.Equal(t, 2.0, other["model_ratio"])
}

// TestFormatUserLogsStripsQuotaSaturation verifies the admin-only quota
// saturation marker (nested under other.admin_info) is removed for non-admin
// log views, since formatUserLogs strips the whole admin_info object.
func TestFormatUserLogsStripsQuotaSaturation(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"model_price": 0.004,
		"admin_info": map[string]interface{}{
			"quota_saturation": map[string]interface{}{
				"op":      "QuotaFromDecimal",
				"kind":    "overflow",
				"clamped": common.MaxQuota,
			},
		},
	})
	logs := []*Log{{Other: other}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	_, hasAdminInfo := parsed["admin_info"]
	require.False(t, hasAdminInfo, "admin_info (and nested quota_saturation) must be stripped for non-admin views")
	// Non-admin billing fields remain visible.
	require.Contains(t, parsed, "model_price")
}
