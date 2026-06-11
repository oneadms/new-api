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
	require.NotContains(t, other, "actual_model_ratio")
	require.NotContains(t, other, "stream_status")
	require.Equal(t, 2.0, other["model_ratio"])
}
