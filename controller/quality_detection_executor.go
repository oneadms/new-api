package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// channelQualityExecutor shares channel credentials, mappings, request conversion
// and HTTP transport with normal relay, but not its retry, routing or billing loop.
type channelQualityExecutor struct{}

func (channelQualityExecutor) Supports(channelType int, protocol string) bool {
	apiType, supported := common.ChannelType2APIType(channelType)
	if !supported && channelType != constant.ChannelTypeAzure {
		return false
	}
	switch apiType {
	case constant.APITypeOpenAI:
		return protocol == "chat" || protocol == "responses"
	case constant.APITypeAnthropic:
		return protocol == "claude"
	// Native Codex strips max_output_tokens. It cannot honor the reserved
	// generation bound, so it is not admitted until bounded execution exists.
	default:
		return false
	}
}

func (executor channelQualityExecutor) Execute(ctx context.Context, input service.QualityExecutionRequest, emit func(string) error) (result service.QualityExecutionResult, err error) {
	started := time.Now()
	result.ErrorCode = "configuration_conflict"
	defer func() {
		result.GenerationMS = time.Since(started).Milliseconds()
		if err == nil {
			result.ErrorCode = ""
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result.ErrorCode = "timeout"
		} else if ctx.Err() != nil {
			result.ErrorCode = "cancelled"
		}
	}()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if input.ChannelID <= 0 || input.Model == "" || strings.TrimSpace(input.Prompt) == "" || len(input.Prompt) > 16000+len(service.QualityHTMLInstruction) || input.MaxOutputTokens < 1 || uint64(input.MaxOutputTokens) > uint64(helper.MaxTokensLimit) || emit == nil {
		return result, errors.New("invalid quality detection request")
	}
	channel, err := model.GetChannelById(input.ChannelID, true)
	if err != nil {
		result.ErrorCode = "channel_disabled"
		return result, errors.New("detection channel is unavailable")
	}
	if channel.Status != common.ChannelStatusEnabled {
		result.ErrorCode = "channel_disabled"
		return result, errors.New("detection channel is no longer enabled")
	}
	if !slices.Contains(channel.GetModels(), input.Model) {
		result.ErrorCode = "model_unavailable"
		return result, errors.New("detection model is no longer enabled")
	}
	if !executor.Supports(channel.Type, input.Protocol) {
		result.ErrorCode = "unsupported_protocol"
		return result, errors.New("channel does not support this detection protocol")
	}
	request, format, path, err := buildQualityRequest(input)
	if err != nil {
		return result, err
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequestWithContext(ctx, http.MethodPost, path, nil)
	c.Request.Header.Set("Content-Type", "application/json")
	// This is a representative internal channel request, not an impersonated
	// customer. It never enters wallet, subscription or consume-log handlers.
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "__quality_detection__")
	common.SetContextKey(c, constant.ContextKeyUserGroup, "__quality_detection__")
	if setupErr := middleware.SetupContextForSelectedChannel(c, channel, input.Model); setupErr != nil {
		result.ErrorCode = "credential_unavailable"
		return result, errors.New("channel credentials are unavailable")
	}
	info, err := relaycommon.GenRelayInfo(c, format, request, nil)
	if err != nil {
		return result, errors.New("cannot prepare detection request")
	}
	info.IsChannelTest = true
	info.IsQualityDetection = true
	info.DisablePing = true
	info.InitChannelMeta(c)
	if info.ChannelSetting.SystemPrompt != "" {
		result.ErrorCode = "configuration_conflict"
		return result, errors.New("channel system prompt conflicts with the frozen detection prompt")
	}
	if err := helper.ModelMappedHelper(c, info, request); err != nil {
		return result, errors.New("invalid channel model mapping")
	}
	request.SetModelName(info.UpstreamModelName)
	adaptor := relay.GetAdaptor(info.ApiType)
	if adaptor == nil {
		return result, errors.New("channel adapter is unavailable")
	}
	adaptor.Init(info)
	var converted any
	switch req := request.(type) {
	case *dto.GeneralOpenAIRequest:
		if info.SupportStreamOptions {
			req.StreamOptions = &dto.StreamOptions{IncludeUsage: true}
		}
		converted, err = adaptor.ConvertOpenAIRequest(c, info, req)
	case *dto.OpenAIResponsesRequest:
		converted, err = adaptor.ConvertOpenAIResponsesRequest(c, info, *req)
	case *dto.ClaudeRequest:
		converted, err = adaptor.ConvertClaudeRequest(c, info, req)
	}
	if err != nil || converted == nil {
		return result, errors.New("cannot convert detection request")
	}
	payload, err := common.Marshal(converted)
	if err != nil {
		return result, errors.New("cannot encode detection request")
	}
	if len(info.ParamOverride) > 0 {
		original := payload
		payload, err = relaycommon.ApplyParamOverrideWithRelayInfo(payload, info)
		if err != nil || validateQualityOverrides(original, payload) != nil {
			result.ErrorCode = "configuration_conflict"
			return result, errors.New("channel overrides change the detection prompt, model or generation limits")
		}
	}
	var actual struct {
		Model string `json:"model"`
	}
	if err := common.Unmarshal(payload, &actual); err != nil || actual.Model == "" {
		return result, errors.New("converted detection model is unavailable")
	}
	result.MappedModel = actual.Model
	result.RequestFingerprint = fmt.Sprintf("%x", sha256.Sum256(payload))
	if err := ctx.Err(); err != nil {
		return result, err
	}
	// Mark conservatively before dispatch. A network error cannot prove that the
	// upstream never received the request, so admission must not auto-replay it.
	result.Started = true
	result.ErrorCode = "upstream_error"
	c.Request.Body = io.NopCloser(bytes.NewReader(payload))
	response, err := adaptor.DoRequest(c, info, bytes.NewBuffer(payload))
	if err != nil {
		return result, errors.New("upstream request failed")
	}
	resp, ok := response.(*http.Response)
	if !ok || resp == nil || resp.Body == nil {
		return result, errors.New("upstream returned no response")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		result.ErrorCode = "upstream_rejected"
		if resp.StatusCode >= http.StatusInternalServerError {
			result.ErrorCode = "upstream_unavailable"
		}
		return result, errors.New("upstream rejected the detection request")
	}
	if !strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		result.ErrorCode = "invalid_stream"
		return result, errors.New("upstream did not return an event stream")
	}
	collector := qualityStreamCollector{protocol: input.Protocol, emit: emit, started: started, result: &result}
	err = collector.consume(resp.Body)
	return result, err
}

func buildQualityRequest(input service.QualityExecutionRequest) (dto.Request, types.RelayFormat, string, error) {
	stream := true
	limit := uint(input.MaxOutputTokens)
	switch input.Protocol {
	case "chat":
		return &dto.GeneralOpenAIRequest{
			Model: input.Model, Stream: &stream, MaxCompletionTokens: &limit, ReasoningEffort: input.ReasoningEffort,
			Messages: []dto.Message{{Role: "user", Content: input.Prompt}},
		}, types.RelayFormatOpenAI, "/v1/chat/completions", nil
	case "responses":
		text, err := common.Marshal([]map[string]any{{"role": "user", "content": []map[string]string{{"type": "input_text", "text": input.Prompt}}}})
		if err != nil {
			return nil, "", "", err
		}
		req := &dto.OpenAIResponsesRequest{Model: input.Model, Input: text, Stream: &stream, MaxOutputTokens: &limit}
		if input.ReasoningEffort != "" {
			req.Reasoning = &dto.Reasoning{Effort: input.ReasoningEffort}
		}
		return req, types.RelayFormatOpenAIResponses, "/v1/responses", nil
	case "claude":
		req := &dto.ClaudeRequest{Model: input.Model, Stream: &stream, MaxTokens: &limit, Messages: []dto.ClaudeMessage{{Role: "user", Content: input.Prompt}}}
		if input.ReasoningEffort != "" {
			req.Thinking = &dto.Thinking{Type: "adaptive"}
			config, err := common.Marshal(map[string]string{"effort": input.ReasoningEffort})
			if err != nil {
				return nil, "", "", err
			}
			req.OutputConfig = config
		}
		return req, types.RelayFormatClaude, "/v1/messages", nil
	default:
		return nil, "", "", errors.New("unsupported detection protocol")
	}
}

// Parameter overrides may tune a representative request, but cannot replace its
// subject, create extra choices/tool work, or remove the reserved output bound.
func validateQualityOverrides(before, after []byte) error {
	var original, overridden map[string]any
	if err := common.Unmarshal(before, &original); err != nil {
		return err
	}
	if err := common.Unmarshal(after, &overridden); err != nil {
		return err
	}
	for _, key := range []string{"model", "messages", "input", "instructions", "system", "stream", "n", "max_tokens", "max_completion_tokens", "max_output_tokens", "tools", "tool_choice", "parallel_tool_calls", "background", "previous_response_id", "conversation", "prompt", "mcp_servers"} {
		if !reflect.DeepEqual(original[key], overridden[key]) {
			return errors.New("protected detection parameter changed")
		}
	}
	return nil
}
