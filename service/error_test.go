package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestResetStatusCode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		statusCode       int
		statusCodeConfig string
		expectedCode     int
	}{
		{
			name:             "map string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"503"}`,
			expectedCode:     503,
		},
		{
			name:             "map int value",
			statusCode:       429,
			statusCodeConfig: `{"429":503}`,
			expectedCode:     503,
		},
		{
			name:             "skip invalid string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"bad-code"}`,
			expectedCode:     429,
		},
		{
			name:             "skip status code 200",
			statusCode:       200,
			statusCodeConfig: `{"200":503}`,
			expectedCode:     200,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			newAPIError := &types.NewAPIError{
				StatusCode: tc.statusCode,
			}
			ResetStatusCode(newAPIError, tc.statusCodeConfig)
			require.Equal(t, tc.expectedCode, newAPIError.StatusCode)
		})
	}
}

func TestRelayErrorHandlerTruncatesInvalidJSONBodyInLog(t *testing.T) {
	withDebugEnabled(t, false)

	body := strings.Repeat("b", common.LocalLogContentLimit+256)
	var logBuffer bytes.Buffer

	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, "bad response status code 500", newAPIError.Error())
	require.Contains(t, logBuffer.String(), "[truncated")
	require.Contains(t, logBuffer.String(), fmt.Sprintf("original_length=%d", len(body)))
	require.NotContains(t, logBuffer.String(), strings.Repeat("b", common.LocalLogContentLimit+1))
}

func TestRelayErrorHandlerKeepsStructuredErrorMessage(t *testing.T) {
	message := strings.Repeat("c", common.LocalLogContentLimit+256)
	body := `{"message":"` + message + `"}`
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, message, newAPIError.Error())
}

func TestRelayErrorHandlerKeepsOpenAIErrorMessage(t *testing.T) {
	message := strings.Repeat("d", common.LocalLogContentLimit+256)
	body := `{"error":{"message":"` + message + `","type":"server_error","code":"server_error"}}`
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, message, newAPIError.Error())
}

func TestRelayErrorHandlerKeepsInvalidJSONBodyInDebugLog(t *testing.T) {
	withDebugEnabled(t, true)

	body := strings.Repeat("e", common.LocalLogContentLimit+256)
	var logBuffer bytes.Buffer

	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.NotContains(t, logBuffer.String(), "[truncated")
	require.Contains(t, logBuffer.String(), body)
}

func TestHideUpstreamErrorMessageMasksPublicResponseOnly(t *testing.T) {
	body := `{"error":{"message":"公益站暂时不可用","type":"server_error","code":"server_error"}}`
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)
	HideUpstreamErrorMessage(newAPIError, true)

	require.NotNil(t, newAPIError)
	require.Equal(t, "公益站暂时不可用", newAPIError.Error())
	require.Equal(t, upstreamUnavailablePublicMessage, newAPIError.ToOpenAIError().Message)
	require.Equal(t, upstreamUnavailablePublicMessage, newAPIError.ToClaudeError().Message)

	PrepareRelayErrorForResponse(newAPIError, "req_public_mask", true)
	require.Equal(t, "公益站暂时不可用 (request id: req_public_mask)", newAPIError.Error())
	require.Equal(t, upstreamUnavailablePublicMessage+" (request id: req_public_mask)", newAPIError.ToOpenAIError().Message)
	require.Equal(t, upstreamUnavailablePublicMessage+" (request id: req_public_mask)", newAPIError.ToClaudeError().Message)
}

func TestHideUpstreamErrorMessageSkipsLocalSkipRetryError(t *testing.T) {
	newAPIError := types.NewErrorWithStatusCode(
		fmt.Errorf("local validation failed"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusInternalServerError,
		types.ErrOptionWithSkipRetry(),
	)

	HideUpstreamErrorMessage(newAPIError, true)

	require.Equal(t, "local validation failed", newAPIError.ToOpenAIError().Message)
}

func TestPrepareRelayErrorForResponseKeepsRawMessageWhenDisabled(t *testing.T) {
	newAPIError := types.WithOpenAIError(types.OpenAIError{
		Message: "upstream raw failure",
		Type:    "server_error",
		Code:    "server_error",
	}, http.StatusInternalServerError)

	PrepareRelayErrorForResponse(newAPIError, "req_raw", false)

	require.Equal(t, "upstream raw failure (request id: req_raw)", newAPIError.Error())
	require.Equal(t, "upstream raw failure (request id: req_raw)", newAPIError.ToOpenAIError().Message)
	require.Equal(t, "upstream raw failure (request id: req_raw)", newAPIError.ToClaudeError().Message)
}

func TestPrepareRelayErrorForResponseOmitsEmptyRequestID(t *testing.T) {
	newAPIError := types.WithOpenAIError(types.OpenAIError{
		Message: "upstream raw failure",
		Type:    "server_error",
		Code:    "server_error",
	}, http.StatusInternalServerError)

	PrepareRelayErrorForResponse(newAPIError, "", false)

	require.Equal(t, "upstream raw failure", newAPIError.Error())
	require.Equal(t, "upstream raw failure", newAPIError.ToOpenAIError().Message)
}

func TestPrepareTaskErrorForResponseMasksUpstreamMessages(t *testing.T) {
	taskErr := &dto.TaskError{
		Code:       "task_failed",
		Message:    "task upstream error",
		StatusCode: http.StatusBadRequest,
	}

	PrepareTaskErrorForResponse(taskErr, true)

	require.Equal(t, upstreamBadRequestPublicMessage, taskErr.Message)
}

func TestPrepareTaskErrorForResponseKeepsLocalErrors(t *testing.T) {
	taskErr := &dto.TaskError{
		Code:       "build_request_failed",
		Message:    "local build request failed",
		StatusCode: http.StatusInternalServerError,
		LocalError: true,
	}

	PrepareTaskErrorForResponse(taskErr, true)

	require.Equal(t, "local build request failed", taskErr.Message)
}

func TestPrepareMidjourneyResponseForResponseMasksUpstreamMessages(t *testing.T) {
	resp := &dto.MidjourneyResponse{
		Code:        23,
		Description: "队列已满，请稍后尝试",
		Properties:  map[string]any{"discordInstanceId": "111"},
		Result:      "task-1",
	}

	statusCode := PrepareMidjourneyResponseForResponse(resp, http.StatusOK, true)

	require.Equal(t, http.StatusTooManyRequests, statusCode)
	require.Equal(t, "上游队列已满，请稍后重试", resp.Description)
	require.Empty(t, resp.Result)
	require.Nil(t, resp.Properties)
}

func TestPrepareMidjourneyResponseForResponseKeepsResultForLocalLoadSignal(t *testing.T) {
	resp := &dto.MidjourneyResponse{
		Code:        30,
		Description: "当前分组负载已饱和，请稍后再试",
		Result:      "keep-this",
	}

	statusCode := PrepareMidjourneyResponseForResponse(resp, http.StatusOK, true)

	require.Equal(t, http.StatusTooManyRequests, statusCode)
	require.Equal(t, "当前分组上游负载已饱和，请稍后再试", resp.Description)
	require.Equal(t, "keep-this", resp.Result)
}

func TestPrepareMidjourneyTaskForResponseByStoredTextMasksPublicText(t *testing.T) {
	task := &dto.MidjourneyDto{
		Description: "公益站暂时不可用",
		FailReason:  "No available account instance",
		Properties:  &dto.Properties{FinalPrompt: "keep"},
	}

	PrepareMidjourneyTaskForResponseByStoredText(task, true)

	require.Equal(t, upstreamUnavailablePublicMessage, task.Description)
	require.Equal(t, upstreamUnavailablePublicMessage, task.FailReason)
	require.Nil(t, task.Properties)
}

func TestPrepareMidjourneyModelTaskForResponseMasksPublicText(t *testing.T) {
	task := &model.Midjourney{
		Description: "队列已满，请稍后再试",
		FailReason:  "余额不足",
		Properties:  "keep",
	}

	PrepareMidjourneyModelTaskForResponse(task, true)

	require.Equal(t, "上游队列已满，请稍后重试", task.Description)
	require.Equal(t, upstreamBillingErrorPublicMessage, task.FailReason)
	require.Empty(t, task.Properties)
}

func TestMidjourneyHideUpstreamErrorFallsBackToDB(t *testing.T) {
	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	defer func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	}()

	db, err := gorm.Open(sqlite.Open("file:midjourney-hide-upstream-error?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer func() { _ = sqlDB.Close() }()

	model.DB = db
	common.MemoryCacheEnabled = false
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))

	hideUpstreamError := true
	settingsBytes, marshalErr := common.Marshal(dto.ChannelSettings{HideUpstreamError: hideUpstreamError})
	require.NoError(t, marshalErr)
	channel := &model.Channel{
		Id:      10001,
		Name:    "hide-upstream-error-channel",
		Key:     "sk-test",
		Status:  common.ChannelStatusEnabled,
		Setting: common.GetPointer(string(settingsBytes)),
	}
	require.NoError(t, db.Create(channel).Error)

	require.True(t, MidjourneyHideUpstreamError(channel.Id))
}

func TestUpstreamPublicMessageForStatus(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		expected   string
	}{
		{name: "bad request", statusCode: http.StatusBadRequest, expected: upstreamBadRequestPublicMessage},
		{name: "unprocessable entity", statusCode: http.StatusUnprocessableEntity, expected: upstreamBadRequestPublicMessage},
		{name: "unauthorized", statusCode: http.StatusUnauthorized, expected: upstreamAuthErrorPublicMessage},
		{name: "forbidden", statusCode: http.StatusForbidden, expected: upstreamAuthErrorPublicMessage},
		{name: "payment required", statusCode: http.StatusPaymentRequired, expected: upstreamBillingErrorPublicMessage},
		{name: "not found", statusCode: http.StatusNotFound, expected: upstreamNotFoundPublicMessage},
		{name: "timeout", statusCode: http.StatusRequestTimeout, expected: upstreamTimeoutPublicMessage},
		{name: "gateway timeout", statusCode: http.StatusGatewayTimeout, expected: upstreamTimeoutPublicMessage},
		{name: "rate limited", statusCode: http.StatusTooManyRequests, expected: upstreamRateLimitPublicMessage},
		{name: "server error", statusCode: http.StatusInternalServerError, expected: upstreamUnavailablePublicMessage},
		{name: "bad gateway", statusCode: http.StatusBadGateway, expected: upstreamUnavailablePublicMessage},
		{name: "service unavailable", statusCode: http.StatusServiceUnavailable, expected: upstreamUnavailablePublicMessage},
		{name: "other upstream 5xx", statusCode: 599, expected: upstreamUnavailablePublicMessage},
		{name: "other upstream 4xx", statusCode: http.StatusConflict, expected: upstreamGenericErrorPublicMessage},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.expected, upstreamPublicMessageForStatus(tc.statusCode))
		})
	}
}

func withDebugEnabled(t *testing.T, enabled bool) {
	t.Helper()

	oldDebug := common.DebugEnabled
	common.DebugEnabled = enabled
	t.Cleanup(func() {
		common.DebugEnabled = oldDebug
	})
}
