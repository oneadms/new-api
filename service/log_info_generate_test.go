package service

import (
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
	other := map[string]interface{}{}

	appendRelayTimingInfo(info, other)

	assertTimingField(t, other, "pre_upstream_ms", int64(30_000))
	assertTimingField(t, other, "upstream_header_ms", int64(1_000))
	assertTimingField(t, other, "upstream_frt_ms", int64(15_700))
}

func TestAppendRelayTimingInfoOmitsUnavailableCheckpoints(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	other := map[string]interface{}{}

	appendRelayTimingInfo(&relaycommon.RelayInfo{StartTime: start}, other)

	if len(other) != 0 {
		t.Fatalf("unexpected timing fields: %#v", other)
	}
}

func assertTimingField(t *testing.T, other map[string]interface{}, key string, want int64) {
	t.Helper()
	got, ok := other[key]
	if !ok {
		t.Fatalf("missing %s in %#v", key, other)
	}
	if got != want {
		t.Fatalf("%s = %#v, want %d", key, got, want)
	}
}
