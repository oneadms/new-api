package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

// Codex 客户端的模型清单。
//
// Codex CLI 刷新模型选单时请求 GET /v1/models?client_version=<版本>，期望的是 Codex
// manifest 形状（{"models":[{"slug":...}]}），而不是 OpenAI 的 {"data":[{"id":...}]}。
// 拿到后者它解析不了，只能静默冻结在本地缓存（选单不全、模型能力未知）。
//
// 这里按当前 Key/分组可见的模型合成一份 manifest，并把「该模型支持哪些内置工具」
// 显式声明出来——客户端正是靠 experimental_supported_tools 决定要不要在会话里暴露
// 内置 image_gen 等工具。
//
// 内置图片工具名单默认取实测支持 hosted image_generation 的模型（用扁平
// {"type":"image_generation"} 打 /v1/responses 能拿到 image_generation_call）；
// 可用环境变量 CODEX_MANIFEST_IMAGE_TOOL_MODELS 覆盖（逗号分隔，置为 none 关闭）。

const codexManifestImageToolModelsEnv = "CODEX_MANIFEST_IMAGE_TOOL_MODELS"

// defaultCodexImageToolModels 是实测支持 hosted image_generation 的模型。
var defaultCodexImageToolModels = []string{
	"gpt-6-astra",
	"gpt-5.6-sol",
	"gpt-5.6-terra",
}

// codexManifestLiteModels 是走 Responses Lite 的模型（上游模型清单 use_responses_lite
// 为 true 的集合；与网关侧的内置种子一致）。
var codexManifestLiteModels = map[string]bool{
	"gpt-5.6-sol":       true,
	"gpt-5.6-terra":     true,
	"gpt-5.6-luna":      true,
	"codex-auto-review": true,
}

type codexManifestReasoningLevel struct {
	Effort      string `json:"effort"`
	Description string `json:"description,omitempty"`
}

type codexManifestModel struct {
	Slug                       string                        `json:"slug"`
	DisplayName                string                        `json:"display_name"`
	Hidden                     bool                          `json:"hidden"`
	Availability               string                        `json:"availability"`
	SupportedInAPI             bool                          `json:"supported_in_api"`
	PreferWebsockets           bool                          `json:"prefer_websockets"`
	UseResponsesLite           bool                          `json:"use_responses_lite"`
	InputModalities            []string                      `json:"input_modalities,omitempty"`
	SupportedReasoningLevels   []codexManifestReasoningLevel `json:"supported_reasoning_levels"`
	DefaultReasoningLevel      string                        `json:"default_reasoning_level"`
	Description                string                        `json:"description"`
	ShellType                  string                        `json:"shell_type"`
	Visibility                 string                        `json:"visibility"`
	BaseInstructions           string                        `json:"base_instructions"`
	SupportsReasoningSummaries bool                          `json:"supports_reasoning_summaries"`
	SupportVerbosity           bool                          `json:"support_verbosity"`
	SupportsParallelToolCalls  bool                          `json:"supports_parallel_tool_calls"`
	ExperimentalSupportedTools []string                      `json:"experimental_supported_tools"`
	TruncationPolicy           map[string]any                `json:"truncation_policy"`
}

// isCodexManifestRequest 判定请求是否来自 Codex 客户端的模型清单刷新。
// Codex CLI 会带上 client_version 查询参数，普通 OpenAI 客户端不会。
func isCodexManifestRequest(c *gin.Context) bool {
	if c == nil {
		return false
	}
	return strings.TrimSpace(c.Query("client_version")) != ""
}

// codexManifestImageToolModels 返回声明支持内置图片工具的模型集合。
// 环境变量覆盖时以它为准；值为 none/off 时返回空集合（不声明任何模型支持）。
func codexManifestImageToolModels() map[string]bool {
	names := defaultCodexImageToolModels
	if raw, ok := os.LookupEnv(codexManifestImageToolModelsEnv); ok {
		trimmed := strings.TrimSpace(raw)
		if strings.EqualFold(trimmed, "none") || strings.EqualFold(trimmed, "off") {
			return map[string]bool{}
		}
		names = strings.Split(trimmed, ",")
	}
	result := make(map[string]bool, len(names))
	for _, name := range names {
		if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
			result[name] = true
		}
	}
	return result
}

// codexManifestReasoningLevels 给出模型支持的思考强度与默认值。
// gpt-5.6 / gpt-6 起的模型放行 xhigh（与网关侧的强度钳位规则一致），其余只到 high。
func codexManifestReasoningLevels(model string) ([]codexManifestReasoningLevel, string) {
	lower := strings.ToLower(model)
	levels := []string{"low", "medium", "high"}
	if strings.HasPrefix(lower, "gpt-5.6") || strings.HasPrefix(lower, "gpt-6") {
		levels = append(levels, "xhigh")
	}
	items := make([]codexManifestReasoningLevel, 0, len(levels))
	for _, level := range levels {
		items = append(items, codexManifestReasoningLevel{Effort: level})
	}
	return items, "medium"
}

// buildCodexModelsManifest 按可见模型合成 manifest 条目。顺序按 slug 排序，
// 保证同样的模型集合恒定产出同样的字节，ETag 才有意义。
func buildCodexModelsManifest(modelNames []string) []codexManifestModel {
	imageToolModels := codexManifestImageToolModels()
	seen := make(map[string]bool, len(modelNames))
	items := make([]codexManifestModel, 0, len(modelNames))
	for _, name := range modelNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true

		levels, defaultLevel := codexManifestReasoningLevels(name)
		// 纯生图模型（gpt-image-*）不是对话模型：留在清单里但隐藏，
		// 也不声明任何内置工具。
		imageOnly := strings.Contains(key, "image")
		tools := []string{}
		if !imageOnly && imageToolModels[key] {
			tools = append(tools, "image_generation")
		}
		items = append(items, codexManifestModel{
			Slug:                       name,
			DisplayName:                name,
			Hidden:                     imageOnly,
			Availability:               "available",
			SupportedInAPI:             true,
			PreferWebsockets:           false,
			UseResponsesLite:           codexManifestLiteModels[key],
			InputModalities:            []string{"text"},
			SupportedReasoningLevels:   levels,
			DefaultReasoningLevel:      defaultLevel,
			Description:                "Gateway model: " + name,
			ShellType:                  "shell_command",
			Visibility:                 "list",
			BaseInstructions:           "You are a helpful coding assistant.",
			SupportsReasoningSummaries: false,
			SupportVerbosity:           false,
			SupportsParallelToolCalls:  false,
			ExperimentalSupportedTools: tools,
			TruncationPolicy:           map[string]any{"mode": "tokens", "limit": 10000},
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Slug) < strings.ToLower(items[j].Slug)
	})
	return items
}

// buildCodexModelsManifestJSON 输出 Codex 客户端期望的 {"models":[...]} 载荷。
func buildCodexModelsManifestJSON(modelNames []string) ([]byte, error) {
	return json.Marshal(struct {
		Models []codexManifestModel `json:"models"`
	}{Models: buildCodexModelsManifest(modelNames)})
}

// ListCodexModelsManifest 向 Codex 客户端返回模型清单。
func ListCodexModelsManifest(c *gin.Context, modelNames []string) {
	body, err := buildCodexModelsManifestJSON(modelNames)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"message": "failed to build codex models manifest: " + err.Error(), "type": "server_error"},
		})
		return
	}
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`
	c.Header("ETag", etag)
	if etagHeaderMatches(c.GetHeader("If-None-Match"), etag) {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// etagHeaderMatches 判断 If-None-Match 是否命中当前 ETag。
func etagHeaderMatches(header, current string) bool {
	current = strings.TrimSpace(current)
	if current == "" {
		return false
	}
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" ||
			candidate == current ||
			strings.TrimPrefix(candidate, "W/") == strings.TrimPrefix(current, "W/") {
			return true
		}
	}
	return false
}
