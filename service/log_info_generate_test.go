package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestAppendRelayTimingInfoSplitsGatewayAndUpstreamLatency(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	info := &relaycommon.RelayInfo{
		StartTime:           start,
		UpstreamStartTime:   start.Add(30 * time.Second),
		UpstreamHeadersTime: start.Add(31 * time.Second),
		FirstResponseTime:   start.Add(45*time.Second + 700*time.Millisecond),
	}
	other := model.NewLogOther()

	appendRelayTimingInfo(info, other)

	parsed, err := common.StrToMap(other.JSONString())
	require.NoError(t, err)
	assert.Equal(t, float64(30_000), parsed["pre_upstream_ms"])
	assert.Equal(t, float64(1_000), parsed["upstream_header_ms"])
	assert.Equal(t, float64(15_700), parsed["upstream_frt_ms"])
}

func TestAppendRelayTimingInfoOmitsUnavailableCheckpoints(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	other := model.NewLogOther()

	appendRelayTimingInfo(&relaycommon.RelayInfo{StartTime: start}, other)

	parsed, err := common.StrToMap(other.JSONString())
	require.NoError(t, err)
	assert.Empty(t, parsed)
}
