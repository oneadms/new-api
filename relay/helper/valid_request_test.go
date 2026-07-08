package helper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newJSONTestContext(body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v1/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	return ctx
}

func TestGetAndValidateTextRequestRejectsHugeMaxCompletionTokens(t *testing.T) {
	ctx := newJSONTestContext(`{"model":"gpt-test","messages":[{"role":"user","content":"hi"}],"max_completion_tokens":2147483648}`)

	_, err := GetAndValidateTextRequest(ctx, relayconstant.RelayModeChatCompletions)

	require.Error(t, err)
	require.Contains(t, err.Error(), "max_completion_tokens is invalid")
}

func TestGetAndValidateResponsesRequestRejectsHugeMaxOutputTokens(t *testing.T) {
	ctx := newJSONTestContext(`{"model":"gpt-test","input":"hi","max_output_tokens":2147483648}`)

	_, err := GetAndValidateResponsesRequest(ctx)

	require.Error(t, err)
	require.Contains(t, err.Error(), "max_output_tokens is invalid")
}

func TestGetAndValidOpenAIImageRequestRejectsHugeN(t *testing.T) {
	ctx := newJSONTestContext(`{"model":"gpt-image-1","prompt":"hi","n":2147483648}`)

	_, err := GetAndValidOpenAIImageRequest(ctx, relayconstant.RelayModeImagesGenerations)

	require.Error(t, err)
	require.Contains(t, err.Error(), "n is invalid")
}

func TestGetAndValidateGeminiRequestRejectsHugeMaxOutputTokens(t *testing.T) {
	ctx := newJSONTestContext(`{"contents":[{"parts":[{"text":"hi"}]}],"generationConfig":{"maxOutputTokens":2147483648}}`)

	_, err := GetAndValidateGeminiRequest(ctx)

	require.Error(t, err)
	require.Contains(t, err.Error(), "max_output_tokens is invalid")
}
