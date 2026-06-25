package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"
)

func MidjourneyErrorWrapper(code int, desc string) *dto.MidjourneyResponse {
	return &dto.MidjourneyResponse{
		Code:        code,
		Description: desc,
	}
}

func MidjourneyErrorWithStatusCodeWrapper(code int, desc string, statusCode int) *dto.MidjourneyResponseWithStatusCode {
	return &dto.MidjourneyResponseWithStatusCode{
		StatusCode: statusCode,
		Response:   *MidjourneyErrorWrapper(code, desc),
	}
}

//// OpenAIErrorWrapper wraps an error into an OpenAIErrorWithStatusCode
//func OpenAIErrorWrapper(err error, code string, statusCode int) *dto.OpenAIErrorWithStatusCode {
//	text := err.Error()
//	lowerText := strings.ToLower(text)
//	if !strings.HasPrefix(lowerText, "get file base64 from url") && !strings.HasPrefix(lowerText, "mime type is not supported") {
//		if strings.Contains(lowerText, "post") || strings.Contains(lowerText, "dial") || strings.Contains(lowerText, "http") {
//			common.SysLog(fmt.Sprintf("error: %s", text))
//			text = "请求上游地址失败"
//		}
//	}
//	openAIError := dto.OpenAIError{
//		Message: text,
//		Type:    "new_api_error",
//		Code:    code,
//	}
//	return &dto.OpenAIErrorWithStatusCode{
//		Error:      openAIError,
//		StatusCode: statusCode,
//	}
//}
//
//func OpenAIErrorWrapperLocal(err error, code string, statusCode int) *dto.OpenAIErrorWithStatusCode {
//	openaiErr := OpenAIErrorWrapper(err, code, statusCode)
//	openaiErr.LocalError = true
//	return openaiErr
//}

func ClaudeErrorWrapper(err error, code string, statusCode int) *dto.ClaudeErrorWithStatusCode {
	text := err.Error()
	lowerText := strings.ToLower(text)
	if !strings.HasPrefix(lowerText, "get file base64 from url") {
		if strings.Contains(lowerText, "post") || strings.Contains(lowerText, "dial") || strings.Contains(lowerText, "http") {
			common.SysLog(fmt.Sprintf("error: %s", text))
			text = "请求上游地址失败"
		}
	}
	claudeError := types.ClaudeError{
		Message: text,
		Type:    "new_api_error",
	}
	return &dto.ClaudeErrorWithStatusCode{
		Error:      claudeError,
		StatusCode: statusCode,
	}
}

func ClaudeErrorWrapperLocal(err error, code string, statusCode int) *dto.ClaudeErrorWithStatusCode {
	claudeErr := ClaudeErrorWrapper(err, code, statusCode)
	claudeErr.LocalError = true
	return claudeErr
}

const (
	upstreamBadRequestPublicMessage   = "上游拒绝了该请求，请检查请求参数"
	upstreamAuthErrorPublicMessage    = "上游认证失败，请联系管理员"
	upstreamBillingErrorPublicMessage = "上游账户状态异常，请联系管理员"
	upstreamNotFoundPublicMessage     = "上游模型或接口不可用，请联系管理员"
	upstreamTimeoutPublicMessage      = "请求上游服务超时，请稍后重试"
	upstreamRateLimitPublicMessage    = "上游服务繁忙，请稍后重试"
	upstreamUnavailablePublicMessage  = "上游服务暂时不可用，请稍后重试"
	upstreamGenericErrorPublicMessage = "上游请求失败，请稍后重试"
)

var taskErrorHideCodes = map[string]struct{}{
	"do_request_failed":              {},
	"read_response_body_failed":      {},
	"unmarshal_response_body_failed": {},
	"unmarshal_response_failed":      {},
	"fail_to_fetch_task":             {},
	"ali_api_error":                  {},
	"task_failed":                    {},
	"invalid_response":               {},
}

var taskErrorNeverHideCodes = map[string]struct{}{
	"build_request_failed":           {},
	"channel_no_available_key":       {},
	"channel_not_found":              {},
	"convert_to_openai_video_failed": {},
	"copy_response_body_failed":      {},
	"get_channel_failed":             {},
	"get_origin_task_failed":         {},
	"get_task_failed":                {},
	"get_task_request_failed":        {},
	"get_tasks_failed":               {},
	"gen_relay_info_failed":          {},
	"invalid_api_platform":           {},
	"invalid_channel_id":             {},
	"invalid_relay_mode":             {},
	"invalid_request":                {},
	"marshal_response_failed":        {},
	"model_mapping_failed":           {},
	"model_price_error":              {},
	"not_implemented":                {},
	"pre_consume_token_quota_failed": {},
	"query_data_error":               {},
	"read_request_body_failed":       {},
	"setup_locked_channel_failed":    {},
	"task_channel_disable":           {},
	"task_not_exist":                 {},
	"update_data_error":              {},
	"insufficient_user_quota":        {},
}

func PrepareRelayErrorForResponse(newApiErr *types.NewAPIError, requestId string, hideUpstreamError bool) {
	if newApiErr == nil {
		return
	}
	HideUpstreamErrorMessage(newApiErr, hideUpstreamError)
	publicMessage := newApiErr.PublicMessage()
	newApiErr.SetMessage(common.MessageWithRequestId(newApiErr.Error(), requestId))
	if publicMessage != "" {
		newApiErr.SetPublicMessage(common.MessageWithRequestId(publicMessage, requestId))
	}
}

func PrepareTaskErrorForResponse(taskErr *dto.TaskError, hideUpstreamError bool) {
	if taskErr == nil {
		return
	}
	if taskErr.StatusCode == http.StatusTooManyRequests {
		taskErr.Message = "当前分组上游负载已饱和，请稍后再试"
		return
	}
	if !hideUpstreamError || !shouldHideUpstreamTaskError(taskErr) {
		return
	}
	taskErr.Message = upstreamPublicMessageForStatus(taskErr.StatusCode)
}

func HideUpstreamErrorMessage(newApiErr *types.NewAPIError, enabled bool) {
	if !enabled || !shouldHideUpstreamErrorMessage(newApiErr) {
		return
	}
	newApiErr.SetPublicMessage(upstreamPublicMessageForStatus(newApiErr.StatusCode))
}

func shouldHideUpstreamErrorMessage(newApiErr *types.NewAPIError) bool {
	if newApiErr == nil {
		return false
	}
	if types.IsSkipRetryError(newApiErr) {
		return false
	}

	switch newApiErr.GetErrorType() {
	case types.ErrorTypeOpenAIError,
		types.ErrorTypeClaudeError,
		types.ErrorTypeGeminiError,
		types.ErrorTypeRerankError,
		types.ErrorTypeUpstreamError:
		return true
	}

	switch newApiErr.GetErrorCode() {
	case types.ErrorCodeDoRequestFailed,
		types.ErrorCodeReadResponseBodyFailed,
		types.ErrorCodeBadResponseStatusCode,
		types.ErrorCodeBadResponse,
		types.ErrorCodeBadResponseBody,
		types.ErrorCodeEmptyResponse,
		types.ErrorCodeAwsInvokeError,
		types.ErrorCodeChannelResponseTimeExceeded:
		return true
	}

	return false
}

func shouldHideUpstreamTaskError(taskErr *dto.TaskError) bool {
	if taskErr == nil {
		return false
	}
	if taskErr.LocalError {
		return false
	}
	if _, ok := taskErrorNeverHideCodes[taskErr.Code]; ok {
		return false
	}
	_, ok := taskErrorHideCodes[taskErr.Code]
	if ok {
		return true
	}
	switch taskErr.StatusCode {
	case http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusRequestTimeout,
		http.StatusUnprocessableEntity,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return taskErr.StatusCode >= http.StatusInternalServerError
	}
}

func upstreamPublicMessageForStatus(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return upstreamBadRequestPublicMessage
	case http.StatusUnauthorized, http.StatusForbidden:
		return upstreamAuthErrorPublicMessage
	case http.StatusPaymentRequired:
		return upstreamBillingErrorPublicMessage
	case http.StatusNotFound:
		return upstreamNotFoundPublicMessage
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return upstreamTimeoutPublicMessage
	case http.StatusTooManyRequests:
		return upstreamRateLimitPublicMessage
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return upstreamUnavailablePublicMessage
	default:
		if statusCode >= http.StatusInternalServerError {
			return upstreamUnavailablePublicMessage
		}
		return upstreamGenericErrorPublicMessage
	}
}

func RelayErrorHandler(ctx context.Context, resp *http.Response, showBodyWhenFail bool) (newApiErr *types.NewAPIError) {
	newApiErr = types.InitOpenAIError(types.ErrorCodeBadResponseStatusCode, resp.StatusCode)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	CloseResponseBodyGracefully(resp)
	var errResponse dto.GeneralErrorResponse
	responseBodyText := string(responseBody)
	responseBodyPreview := common.LocalLogPreview(responseBodyText)
	buildErrWithBody := func(message string) error {
		if message == "" {
			return fmt.Errorf("bad response status code %d, body: %s", resp.StatusCode, responseBodyText)
		}
		return fmt.Errorf("bad response status code %d, message: %s, body: %s", resp.StatusCode, message, responseBodyText)
	}

	err = common.Unmarshal(responseBody, &errResponse)
	if err != nil {
		if showBodyWhenFail {
			newApiErr.Err = buildErrWithBody("")
		} else {
			logger.LogError(ctx, fmt.Sprintf("bad response status code %d, body: %s", resp.StatusCode, responseBodyPreview))
			newApiErr.Err = fmt.Errorf("bad response status code %d", resp.StatusCode)
		}
		return
	}

	if common.GetJsonType(errResponse.Error) == "object" {
		// General format error (OpenAI, Anthropic, Gemini, etc.)
		oaiError := errResponse.TryToOpenAIError()
		if oaiError != nil {
			newApiErr = types.WithOpenAIError(*oaiError, resp.StatusCode)
			if showBodyWhenFail {
				newApiErr.Err = buildErrWithBody(newApiErr.Error())
			}
			return
		}
	}
	newApiErr = types.NewOpenAIError(errors.New(errResponse.ToMessage()), types.ErrorCodeBadResponseStatusCode, resp.StatusCode)
	if showBodyWhenFail {
		newApiErr.Err = buildErrWithBody(newApiErr.Error())
	}
	return
}

func ResetStatusCode(newApiErr *types.NewAPIError, statusCodeMappingStr string) {
	if newApiErr == nil {
		return
	}
	if statusCodeMappingStr == "" || statusCodeMappingStr == "{}" {
		return
	}
	statusCodeMapping := make(map[string]any)
	err := common.Unmarshal([]byte(statusCodeMappingStr), &statusCodeMapping)
	if err != nil {
		return
	}
	if newApiErr.StatusCode == http.StatusOK {
		return
	}
	codeStr := strconv.Itoa(newApiErr.StatusCode)
	if value, ok := statusCodeMapping[codeStr]; ok {
		intCode, ok := parseStatusCodeMappingValue(value)
		if !ok {
			return
		}
		newApiErr.StatusCode = intCode
	}
}

func parseStatusCodeMappingValue(value any) (int, bool) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return 0, false
		}
		statusCode, err := strconv.Atoi(v)
		if err != nil {
			return 0, false
		}
		return statusCode, true
	case float64:
		if v != math.Trunc(v) {
			return 0, false
		}
		return int(v), true
	case int:
		return v, true
	case json.Number:
		statusCode, err := strconv.Atoi(v.String())
		if err != nil {
			return 0, false
		}
		return statusCode, true
	default:
		return 0, false
	}
}

func TaskErrorWrapperLocal(err error, code string, statusCode int) *dto.TaskError {
	openaiErr := TaskErrorWrapper(err, code, statusCode)
	openaiErr.LocalError = true
	return openaiErr
}

func TaskErrorWrapper(err error, code string, statusCode int) *dto.TaskError {
	text := err.Error()
	lowerText := strings.ToLower(text)
	if strings.Contains(lowerText, "post") || strings.Contains(lowerText, "dial") || strings.Contains(lowerText, "http") {
		common.SysLog(fmt.Sprintf("error: %s", text))
		//text = "请求上游地址失败"
		text = common.MaskSensitiveInfo(text)
	}
	//避免暴露内部错误
	taskError := &dto.TaskError{
		Code:       code,
		Message:    text,
		StatusCode: statusCode,
		Error:      err,
	}

	return taskError
}

// TaskErrorFromAPIError 将 PreConsumeBilling 返回的 NewAPIError 转换为 TaskError。
func TaskErrorFromAPIError(apiErr *types.NewAPIError) *dto.TaskError {
	if apiErr == nil {
		return nil
	}
	return &dto.TaskError{
		Code:       string(apiErr.GetErrorCode()),
		Message:    apiErr.Err.Error(),
		StatusCode: apiErr.StatusCode,
		LocalError: true,
		Error:      apiErr.Err,
	}
}
