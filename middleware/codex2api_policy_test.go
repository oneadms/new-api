package middleware

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsRelayPolicyPathScopesRelayEntrypoints(t *testing.T) {
	tests := []struct {
		path  string
		match bool
	}{
		{path: "/v1/responses", match: true},
		{path: "/v1beta/models/gpt-5:generateContent", match: true},
		{path: "/v1/models", match: true},
		{path: "/pg/chat/completions", match: true},
		{path: "/mj/submit/imagine", match: true},
		{path: "/fast/mj/submit/imagine", match: true},
		{path: "/kling/v1/videos/text2video", match: true},
		{path: "/v1/videos", match: true},
		{path: "/v1/dashboard/billing/usage", match: false},
		{path: "/v1/dashboard", match: false},
		{path: "/v1/dashboard/health", match: false},
		{path: "/api/user/login", match: false},
		{path: "/api/status", match: false},
		{path: "/dashboard", match: false},
		{path: "/v1foo/not-relay", match: false},
	}
	for _, test := range tests {
		require.Equal(t, test.match, IsRelayPolicyPath(test.path), "path=%q", test.path)
	}
}
