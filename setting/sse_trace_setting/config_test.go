package sse_trace_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateSnapshotNormalizesUnsafeValues(t *testing.T) {
	oldSetting := sseTraceSetting
	t.Cleanup(func() {
		sseTraceSetting = oldSetting
		UpdateSnapshot()
	})

	sseTraceSetting = SSETraceSetting{
		Enabled:        true,
		MaxRequestMB:   0,
		QueueMaxMB:     2,
		RetentionHours: 900,
		StoragePath:    "  ",
	}
	UpdateSnapshot()
	setting := GetSetting()

	require.True(t, setting.Enabled)
	require.Equal(t, defaultMaxRequestMB, setting.MaxRequestMB)
	require.Equal(t, defaultQueueMaxMB, setting.QueueMaxMB)
	require.Equal(t, defaultRetentionHours, setting.RetentionHours)
	require.Equal(t, defaultStoragePath, setting.StoragePath)
}
