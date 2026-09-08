package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBuildCodexModelsManifestDeclaresImageTool(t *testing.T) {
	items := buildCodexModelsManifest([]string{"gpt-6-astra", "gpt-5.6-sol", "gpt-image-2", "codex-auto-review"})
	require.Len(t, items, 4)

	bySlug := make(map[string]codexManifestModel, len(items))
	for _, item := range items {
		bySlug[item.Slug] = item
	}

	astra := bySlug["gpt-6-astra"]
	require.Equal(t, []string{"image_generation"}, astra.ExperimentalSupportedTools)
	require.False(t, astra.Hidden)
	require.False(t, astra.UseResponsesLite)
	require.Equal(t, "available", astra.Availability)
	require.True(t, astra.SupportedInAPI)

	// gpt-5.6 全系走 Responses Lite，并且实测支持 hosted 生图。
	sol := bySlug["gpt-5.6-sol"]
	require.Equal(t, []string{"image_generation"}, sol.ExperimentalSupportedTools)
	require.True(t, sol.UseResponsesLite)
	require.Equal(t, "xhigh", sol.SupportedReasoningLevels[len(sol.SupportedReasoningLevels)-1].Effort)

	// 纯生图模型不是对话模型：隐藏且不声明内置工具。
	imageOnly := bySlug["gpt-image-2"]
	require.True(t, imageOnly.Hidden)
	require.Empty(t, imageOnly.ExperimentalSupportedTools)

	// 审核模型不声明图片工具。
	require.Empty(t, bySlug["codex-auto-review"].ExperimentalSupportedTools)
}

func TestBuildCodexModelsManifestDeduplicatesAndSorts(t *testing.T) {
	items := buildCodexModelsManifest([]string{"zeta", "alpha", " zeta ", "", "beta"})
	slugs := make([]string, 0, len(items))
	for _, item := range items {
		slugs = append(slugs, item.Slug)
	}
	require.Equal(t, []string{"alpha", "beta", "zeta"}, slugs)
}

func TestBuildCodexModelsManifestImageToolEnvOverride(t *testing.T) {
	t.Setenv(codexManifestImageToolModelsEnv, "gpt-5.6-sol, custom-model")
	items := buildCodexModelsManifest([]string{"gpt-6-astra", "gpt-5.6-sol", "custom-model"})
	bySlug := make(map[string]codexManifestModel, len(items))
	for _, item := range items {
		bySlug[item.Slug] = item
	}
	require.Empty(t, bySlug["gpt-6-astra"].ExperimentalSupportedTools)
	require.Equal(t, []string{"image_generation"}, bySlug["gpt-5.6-sol"].ExperimentalSupportedTools)
	require.Equal(t, []string{"image_generation"}, bySlug["custom-model"].ExperimentalSupportedTools)

	t.Setenv(codexManifestImageToolModelsEnv, "none")
	for _, item := range buildCodexModelsManifest([]string{"gpt-6-astra", "gpt-5.6-sol"}) {
		require.Empty(t, item.ExperimentalSupportedTools, "none 应关闭所有图片工具声明")
	}
}

func TestIsCodexManifestRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	require.False(t, isCodexManifestRequest(c))

	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models?client_version=0.153.4", nil)
	require.True(t, isCodexManifestRequest(c))
}

func TestListCodexModelsManifestWritesManifestShapeAndETag(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 走真实 gin 引擎：直接调 handler 不会 flush 状态码，304 这类无响应体的分支
	// 在裸 httptest.Recorder 上会误读成 200。
	router := gin.New()
	router.GET("/v1/models", func(c *gin.Context) {
		ListCodexModelsManifest(c, []string{"gpt-6-astra"})
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/models?client_version=0.153.4", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	etag := recorder.Header().Get("ETag")
	require.NotEmpty(t, etag)

	var payload struct {
		Models []codexManifestModel `json:"models"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Len(t, payload.Models, 1)
	require.Equal(t, "gpt-6-astra", payload.Models[0].Slug)
	// Codex 客户端只认 {"models":[...]}，不能出现 OpenAI 的 data 字段。
	require.False(t, strings.Contains(recorder.Body.String(), `"data"`))

	// 同一份清单必须字节稳定，客户端带 If-None-Match 时应拿到 304。
	first := recorder.Body.String()
	req := httptest.NewRequest(http.MethodGet, "/v1/models?client_version=0.153.4", nil)
	req.Header.Set("If-None-Match", etag)
	recorder2 := httptest.NewRecorder()
	router.ServeHTTP(recorder2, req)
	require.Equal(t, http.StatusNotModified, recorder2.Code)
	require.Empty(t, recorder2.Body.String())

	recorder3 := httptest.NewRecorder()
	router.ServeHTTP(recorder3, httptest.NewRequest(http.MethodGet, "/v1/models?client_version=0.153.4", nil))
	require.Equal(t, first, recorder3.Body.String())
}
