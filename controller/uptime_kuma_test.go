package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchGroupDataLimitsHeartbeatsToMostRecent(t *testing.T) {
	statusPayload := `{"publicGroupList":[{"id":1,"name":"Core","monitorList":[{"id":12,"name":"API"}]}]}`
	heartbeats := make([]string, maxHeartbeatSamples+1)
	for i := range heartbeats {
		status := "3"
		if i == 0 {
			status = "2"
		}
		heartbeats[i] = `{"status":` + status + `}`
	}
	heartbeatPayload := `{"heartbeatList":{"12":[` + strings.Join(heartbeats, ",") + `]},"uptimeList":{"12_24":1,"12_720":0.99}}`

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == apiStatusPath+"main" {
			_, _ = w.Write([]byte(statusPayload))
			return
		}
		_, _ = w.Write([]byte(heartbeatPayload))
	}))
	t.Cleanup(upstream.Close)

	result := fetchGroupData(
		t.Context(),
		upstream.Client(),
		map[string]any{"url": upstream.URL, "slug": "main", "categoryName": "Core"},
	)

	require.Len(t, result.Monitors, 1)
	monitor := result.Monitors[0]
	require.Len(t, monitor.Heartbeats, maxHeartbeatSamples)
	assert.Equal(t, 3, monitor.Heartbeats[0].Status)
	assert.Equal(t, 3, monitor.Heartbeats[len(monitor.Heartbeats)-1].Status)
	assert.Equal(t, 2, monitor.Status)
	require.NotNil(t, monitor.Uptime24)
	assert.Equal(t, float64(1), *monitor.Uptime24)
	require.NotNil(t, monitor.Uptime30)
	assert.Equal(t, float64(0.99), *monitor.Uptime30)
}

func TestGetUptimeKumaStatusReturnsEmptyGroupWhenUpstreamFails(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(upstream.Close)

	settings := console_setting.GetConsoleSetting()
	previousGroups := settings.UptimeKumaGroups
	settings.UptimeKumaGroups = `[{"url":"` + upstream.URL + `","slug":"main","categoryName":"Core"}]`
	t.Cleanup(func() { settings.UptimeKumaGroups = previousGroups })

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/uptime/status", nil)

	GetUptimeKumaStatus(context)

	assert.Equal(t, http.StatusOK, response.Code)
	var payload struct {
		Success bool                `json:"success"`
		Data    []UptimeGroupResult `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	assert.True(t, payload.Success)
	require.Len(t, payload.Data, 1)
	assert.Empty(t, payload.Data[0].Monitors)
}
