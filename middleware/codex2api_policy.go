package middleware

import (
	"strings"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// Codex2APIPolicyIPBlock delegates the optional shared IP deny-list check to
// the policy service.  Keeping it as a middleware makes the check run before
// TokenAuth and prevents token rotation from bypassing an active block.
func Codex2APIPolicyIPBlock() gin.HandlerFunc {
	check := service.Codex2APIPolicyIPBlockMiddleware()
	return func(c *gin.Context) {
		if c == nil {
			return
		}
		if c.Request == nil || c.Request.URL == nil || !IsRelayPolicyPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		check(c)
	}
}

// IsRelayPolicyPath keeps the deny-list scoped to relay/task entry points.
// Dashboard, authentication, and health routes must remain reachable so an
// administrator can inspect or clear a block without being locked out by a
// user's flagged address.
func IsRelayPolicyPath(path string) bool {
	path = "/" + strings.TrimPrefix(strings.TrimSpace(path), "/")
	// SetRelayRouter installs the middleware on the engine before registering
	// the later relay routes, while the dashboard also has a few legacy
	// endpoints under /v1. Keep those administrative endpoints reachable even
	// when the source address is on the relay deny-list.
	if path == "/v1/dashboard" || strings.HasPrefix(path, "/v1/dashboard/") {
		return false
	}
	if path == "/pg" || strings.HasPrefix(path, "/pg/") ||
		path == "/v1" || strings.HasPrefix(path, "/v1/") ||
		path == "/v1beta" || strings.HasPrefix(path, "/v1beta/") ||
		path == "/mj" || strings.HasPrefix(path, "/mj/") ||
		path == "/suno" || strings.HasPrefix(path, "/suno/") ||
		path == "/kling" || strings.HasPrefix(path, "/kling/") ||
		path == "/jimeng" || strings.HasPrefix(path, "/jimeng/") {
		return true
	}
	// Midjourney mode aliases use /<mode>/mj/... (for example /fast/mj).
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) >= 2 && parts[1] == "mj"
}
