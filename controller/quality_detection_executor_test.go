package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestQualityStreamRequiresProtocolCompletionAndKeepsUsageUnknownWhenMissing(t *testing.T) {
	cases := []struct {
		name, protocol, stream, output string
		complete                       bool
		input, tokens                  *int64
	}{
		{name: "chat completes with finish reason and terminal marker", protocol: "chat", stream: "data: {\"model\":\"reported\",\"choices\":[{\"delta\":{\"content\":\"<html>ok</html>\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", output: "<html>ok</html>", complete: true},
		{name: "chat disconnect after content is incomplete", protocol: "chat", stream: "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n", output: "partial"},
		{name: "chat terminal marker without finish is incomplete", protocol: "chat", stream: "data: [DONE]\n\n"},
		{name: "chat truncated finish is incomplete", protocol: "chat", stream: "data: {\"choices\":[{\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n"},
		{name: "responses completes and settles final usage", protocol: "responses", stream: "data: {\"type\":\"response.output_text.delta\",\"delta\":\"<html>ok</html>\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":12,\"output_tokens\":9}}}\n\n", output: "<html>ok</html>", complete: true, input: int64Pointer(12), tokens: int64Pointer(9)},
		{name: "responses incomplete event keeps partial output", protocol: "responses", stream: "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\ndata: {\"type\":\"response.incomplete\"}\n\n", output: "partial"},
		{name: "responses completed event must confirm status", protocol: "responses", stream: "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"failed\"}}\n\n"},
		{name: "Claude includes cached input and final output", protocol: "claude", stream: "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":2,\"cache_read_input_tokens\":10,\"cache_creation_input_tokens\":3}}}\n\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"html\"}}\n\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":4}}\n\ndata: {\"type\":\"message_stop\"}\n\n", output: "html", complete: true, input: int64Pointer(15), tokens: int64Pointer(4)},
		{name: "Claude negative input cannot be offset by cache usage", protocol: "claude", stream: "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":-1,\"cache_read_input_tokens\":10}}}\n\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"html\"}}\n\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":4}}\n\ndata: {\"type\":\"message_stop\"}\n\n", output: "html", complete: true, tokens: int64Pointer(4)},
		{name: "Claude stop without end turn is incomplete", protocol: "claude", stream: "data: {\"type\":\"message_stop\"}\n\n"},
		{name: "invalid JSON cannot become a successful request", protocol: "chat", stream: "data: {invalid}\n\n"},
		{name: "upstream error content is not exposed", protocol: "responses", stream: "data: {\"type\":\"error\",\"error\":{\"message\":\"private-secret\"}}\n\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var result service.QualityExecutionResult
			var output strings.Builder
			collector := qualityStreamCollector{protocol: tc.protocol, started: time.Now(), result: &result, emit: func(s string) error { _, err := output.WriteString(s); return err }}
			err := collector.consume(strings.NewReader(tc.stream))
			if tc.complete {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.NotContains(t, err.Error(), "private-secret")
			}
			assert.Equal(t, tc.complete, result.Complete)
			assert.Equal(t, tc.output, output.String())
			assert.Equal(t, tc.input, result.InputTokens)
			assert.Equal(t, tc.tokens, result.OutputTokens)
		})
	}
}

func int64Pointer(value int64) *int64 { return &value }

func TestQualityStreamRejectsOutputLimitAndCallbackFailure(t *testing.T) {
	for _, tc := range []struct {
		name          string
		text          string
		callbackError error
		code          string
	}{
		{"oversized visible output stops collection", strings.Repeat("x", service.QualityOutputLimit+1), nil, "output_limit"},
		{"failed output consumer stops collection", "hello", errors.New("private-storage-error"), "output_write_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := common.Marshal(map[string]any{"type": "response.output_text.delta", "delta": tc.text})
			require.NoError(t, err)
			result := service.QualityExecutionResult{}
			collector := qualityStreamCollector{protocol: "responses", result: &result, started: time.Now(), emit: func(string) error { return tc.callbackError }}
			err = collector.consume(bytes.NewReader(append(append([]byte("data: "), body...), '\n', '\n')))
			require.Error(t, err)
			assert.Equal(t, tc.code, result.ErrorCode)
			assert.False(t, result.Complete)
			assert.NotContains(t, err.Error(), "private-storage-error")
		})
	}
}

func qualityExecutorFixture(t *testing.T, endpoint string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	savedDB := model.DB
	mainType, logType := common.MainDatabaseType(), common.LogDatabaseType()
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB = savedDB
		common.SetDatabaseTypes(mainType, logType)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	mapping := `{"representative":"mapped-model"}`
	header := `{"X-Quality-Fixture":"configured"}`
	require.NoError(t, db.Create(&model.Channel{Id: 1, Type: constant.ChannelTypeOpenAI, Key: "test-private-key", Name: "internal-name",
		Status: common.ChannelStatusEnabled, Models: "representative", Group: "standard,member", BaseURL: &endpoint, ModelMapping: &mapping, HeaderOverride: &header}).Error)
	service.InitHttpClient()
}

func TestQualityExecutorUsesSelectedChannelOnceWithoutWalletOrRoutingWrites(t *testing.T) {
	var calls atomic.Int32
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-private-key", r.Header.Get("Authorization"))
		assert.Equal(t, "configured", r.Header.Get("X-Quality-Fixture"))
		require.NoError(t, common.DecodeJson(r.Body, &payload))
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"<html>ok</html>\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	qualityExecutorFixture(t, server.URL)
	var output strings.Builder
	result, err := (channelQualityExecutor{}).Execute(context.Background(), service.QualityExecutionRequest{ChannelID: 1, Model: "representative", Protocol: "chat", Prompt: service.QualityHTMLInstruction + "draw a bicycle", MaxOutputTokens: 128}, func(s string) error { _, err := output.WriteString(s); return err })
	require.NoError(t, err)
	assert.Equal(t, int32(1), calls.Load())
	assert.True(t, result.Started)
	assert.True(t, result.Complete)
	assert.Equal(t, "mapped-model", payload["model"])
	assert.Equal(t, float64(128), payload["max_completion_tokens"])
	assert.Equal(t, []any{map[string]any{"role": "user", "content": service.QualityHTMLInstruction + "draw a bicycle"}}, payload["messages"])
	assert.Equal(t, "mapped-model", result.MappedModel)
	assert.Len(t, result.RequestFingerprint, 64)
	assert.Equal(t, "<html>ok</html>", output.String())
	channel, err := model.GetChannelById(1, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusEnabled, channel.Status)
	assert.Zero(t, channel.UsedQuota)
	// No user/wallet/log tables exist in this fixture; execution cannot depend
	// on customer identity or use the ordinary consume/settlement handlers.
}

func TestQualityExecutorRejectsOverridesAndBoundsBeforeSending(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	qualityExecutorFixture(t, server.URL)
	for _, override := range []string{`{"model":"another"}`, `{"max_completion_tokens":0}`, `{"messages":[]}`, `{"tools":[{"type":"web_search"}]}`} {
		require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 1).Update("param_override", override).Error)
		result, err := (channelQualityExecutor{}).Execute(context.Background(), service.QualityExecutionRequest{ChannelID: 1, Model: "representative", Protocol: "chat", Prompt: "draw", MaxOutputTokens: 128}, func(string) error { return nil })
		require.Error(t, err)
		assert.False(t, result.Started)
		assert.Equal(t, "configuration_conflict", result.ErrorCode)
	}
	for _, limit := range []int{0, -1, helper.MaxTokensLimit + 1} {
		result, err := (channelQualityExecutor{}).Execute(context.Background(), service.QualityExecutionRequest{ChannelID: 1, Model: "representative", Protocol: "chat", Prompt: "draw", MaxOutputTokens: limit}, func(string) error { return nil })
		require.Error(t, err)
		assert.False(t, result.Started)
	}
	assert.Zero(t, calls.Load())
}

func TestQualityExecutorSkipsConflictingChannelConfigurationWithoutDispatch(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		changes    map[string]any
	}{
		{"channel prompt changes frozen input", "configuration_conflict", map[string]any{"setting": `{"system_prompt":"Replace the task"}`}},
		{"channel disabled after snapshot", "channel_disabled", map[string]any{"status": common.ChannelStatusManuallyDisabled}},
		{"native Codex cannot enforce generation cap", "unsupported_protocol", map[string]any{"type": constant.ChannelTypeCodex}},
		{"model removed after snapshot", "model_unavailable", map[string]any{"models": "other-model"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()
			qualityExecutorFixture(t, server.URL)
			require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 1).Updates(tc.changes).Error)
			result, err := (channelQualityExecutor{}).Execute(context.Background(), service.QualityExecutionRequest{ChannelID: 1, Model: "representative", Protocol: "chat", Prompt: "draw", MaxOutputTokens: 128}, func(string) error { return nil })
			require.Error(t, err)
			assert.Equal(t, tc.code, result.ErrorCode)
			assert.False(t, result.Started)
			assert.Zero(t, calls.Load())
		})
	}
}

func TestQualityExecutorDoesNotFollowRedirectOrRetryHTTPFailure(t *testing.T) {
	for _, code := range []int{http.StatusTemporaryRedirect, http.StatusServiceUnavailable} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", "/replayed")
				w.WriteHeader(code)
			}))
			defer server.Close()
			qualityExecutorFixture(t, server.URL)
			result, err := (channelQualityExecutor{}).Execute(context.Background(), service.QualityExecutionRequest{ChannelID: 1, Model: "representative", Protocol: "chat", Prompt: "draw", MaxOutputTokens: 128}, func(string) error { return nil })
			require.Error(t, err)
			assert.True(t, result.Started)
			assert.False(t, result.Complete)
			assert.Equal(t, int32(1), calls.Load())
		})
	}
}

func TestQualityExecutorKeepsProtocolCapsAndActualMappedModel(t *testing.T) {
	for _, tc := range []struct {
		name, protocol, path, model, capField, authHeader, authValue, events string
		channelType                                                          int
	}{
		{"Responses model suffix conversion", "responses", "/v1/responses", "o3", "max_output_tokens", "Authorization", "Bearer test-private-key", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"<html>ok</html>\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n", constant.ChannelTypeOpenAI},
		{"Claude native request", "claude", "/v1/messages", "o3-high", "max_tokens", "x-api-key", "test-private-key", "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"<html>ok</html>\"}}\n\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}\n\ndata: {\"type\":\"message_stop\"}\n\n", constant.ChannelTypeAnthropic},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			var payload map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				assert.Equal(t, tc.path, r.URL.Path)
				assert.Equal(t, tc.authValue, r.Header.Get(tc.authHeader))
				require.NoError(t, common.DecodeJson(r.Body, &payload))
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, tc.events)
			}))
			defer server.Close()
			qualityExecutorFixture(t, server.URL)
			require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 1).Updates(map[string]any{
				"type": tc.channelType, "model_mapping": `{"representative":"o3-high"}`,
			}).Error)
			result, err := (channelQualityExecutor{}).Execute(context.Background(), service.QualityExecutionRequest{ChannelID: 1, Model: "representative", Protocol: tc.protocol,
				Prompt: service.QualityHTMLInstruction + "draw", MaxOutputTokens: 128}, func(string) error { return nil })
			require.NoError(t, err)
			assert.Equal(t, int32(1), calls.Load())
			assert.True(t, result.Complete)
			assert.Equal(t, tc.model, result.MappedModel)
			assert.Equal(t, tc.model, payload["model"])
			assert.Equal(t, float64(128), payload[tc.capField])
			assert.Equal(t, true, payload["stream"])
		})
	}
}

func TestQualityExecutorCancellationReachesTheUpstreamRequest(t *testing.T) {
	started, stopped := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
		close(stopped)
	}))
	defer server.Close()
	qualityExecutorFixture(t, server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan service.QualityExecutionResult, 1)
	go func() {
		result, _ := (channelQualityExecutor{}).Execute(ctx, service.QualityExecutionRequest{ChannelID: 1, Model: "representative", Protocol: "chat", Prompt: "draw", MaxOutputTokens: 128}, func(string) error { return nil })
		done <- result
	}()
	select {
	case <-started:
		cancel()
	case <-time.After(5 * time.Second):
		t.Fatal("local upstream did not receive request")
	}
	select {
	case result := <-done:
		assert.Equal(t, "cancelled", result.ErrorCode)
		assert.False(t, result.Complete)
	case <-time.After(5 * time.Second):
		t.Fatal("request cancellation was not propagated")
	}
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("upstream request context was not cancelled")
	}
}
