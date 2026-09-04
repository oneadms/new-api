package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const testCodex2APIPolicySecret = "0123456789abcdef0123456789abcdef-test"

func testCodex2APIPolicyContext(t *testing.T, body string) (*gin.Context, *relaycommon.RelayInfo, *http.Request) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set(string(constant.ContextKeyUserId), 42)
	c.Set(string(constant.ContextKeyUserName), "alice")
	c.Set(string(constant.ContextKeyUserEmail), "alice@example.com")
	c.Set(string(constant.ContextKeyUserGroup), "default")
	info := &relaycommon.RelayInfo{
		RequestId:       "req-policy-test",
		RetryIndex:      0,
		RequestURLPath:  "/v1/chat/completions",
		OriginModelName: "gpt-5.5",
		RelayFormat:     "openai",
		TokenKey:        "caller-token",
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:    "codex2api-key-test",
			ChannelId: 7,
		},
	}
	return c, info, req
}

func setTestCodex2APIPolicyEnv(t *testing.T, target string, enabled bool) {
	t.Helper()
	fingerprint := Codex2APIKeyFingerprint("codex2api-key-test")
	bindings := `[{"platform_id":"primary-newapi","target":"` + target + `","codex_key_fingerprint":"` + fingerprint + `","secret":"` + testCodex2APIPolicySecret + `","enabled":true}]`
	t.Setenv("CODEX2API_POLICY_ENABLED", strconv.FormatBool(enabled))
	t.Setenv("CODEX2API_POLICY_IDENTITY_FORWARD_ENABLED", "true")
	t.Setenv("CODEX2API_POLICY_BINDINGS", bindings)
	t.Setenv("CODEX2API_POLICY_AUDIT_ENABLED", "false")
	t.Setenv("CODEX2API_POLICY_STRIKE_ENABLED", "false")
	t.Setenv("CODEX2API_POLICY_ACCOUNT_BAN_ENABLED", "false")
	t.Setenv("CODEX2API_POLICY_IP_BLOCK_ENABLED", "false")
	t.Setenv("CODEX2API_POLICY_BAN_AFTER", "2")
	t.Setenv("CODEX2API_POLICY_WINDOW_SECONDS", "86400")
}

func TestSignCodex2APIRequestBindsIdentityAndExactBody(t *testing.T) {
	setTestCodex2APIPolicyEnv(t, "http://127.0.0.1:18095", true)
	c, info, _ := testCodex2APIPolicyContext(t, `{"model":"gpt-5.5","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18095/v1/chat/completions", io.NopCloser(bytes.NewBufferString(`{"model":"gpt-5.5"}`)))
	require.NoError(t, SignCodex2APIRequest(c, info, req))

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	digest := sha256.Sum256(body)
	digestHex := hex.EncodeToString(digest[:])
	require.Equal(t, digestHex, req.Header.Get(policyHeaderBodySHA256))
	require.Equal(t, "42", req.Header.Get(policyHeaderUserID))
	require.Equal(t, "192.0.2.1", req.Header.Get(policyHeaderClientIP))
	require.Equal(t, "/v1/chat/completions", req.Header.Get(policyHeaderPath))
	require.Equal(t, "1", req.Header.Get(policyHeaderSignatureVersion))

	canonical := strings.Join([]string{
		"v1", req.Header.Get(policyHeaderTimestamp), req.Header.Get(policyHeaderRequestID),
		"42", "192.0.2.1", "POST", "/v1/chat/completions", digestHex,
	}, "\n")
	require.Equal(t, signPolicyValue(testCodex2APIPolicySecret, canonical), req.Header.Get(policyHeaderSignature))

	encodedMeta := req.Header.Get(policyHeaderMeta)
	metaBytes, err := base64.RawURLEncoding.DecodeString(encodedMeta)
	require.NoError(t, err)
	require.Contains(t, string(metaBytes), `"platform_id":"primary-newapi"`)
	metaCanonical := strings.Join([]string{policyMetaSignatureVersionV1, req.Header.Get(policyHeaderRequestID), digestHex, encodedMeta}, "\n")
	require.Equal(t, signPolicyValue(testCodex2APIPolicySecret, metaCanonical), req.Header.Get(policyHeaderMetaSignature))
}

func TestSignCodex2APIRequestOverwritesClientHeaders(t *testing.T) {
	setTestCodex2APIPolicyEnv(t, "http://127.0.0.1:18095", true)
	c, info, _ := testCodex2APIPolicyContext(t, `{}`)
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18095/v1/responses", strings.NewReader(`{}`))
	for name, value := range map[string]string{
		policyHeaderUserID:     "attacker",
		policyHeaderClientIP:   "198.51.100.99",
		policyHeaderRequestID:  "forged",
		policyHeaderSignature:  "forged",
		policyHeaderBodySHA256: "forged",
	} {
		req.Header.Set(name, value)
	}
	require.NoError(t, SignCodex2APIRequest(c, info, req))
	require.Equal(t, "42", req.Header.Get(policyHeaderUserID))
	require.Equal(t, "192.0.2.1", req.Header.Get(policyHeaderClientIP))
	require.NotEqual(t, "forged", req.Header.Get(policyHeaderSignature))
	require.NotEqual(t, "forged", req.Header.Get(policyHeaderBodySHA256))
}

func TestSignCodex2APIRequestBindsFinalAuthorizationKey(t *testing.T) {
	const overriddenKey = "codex2api-overridden-key"
	overriddenFingerprint := Codex2APIKeyFingerprint(overriddenKey)
	t.Setenv("CODEX2API_POLICY_ENABLED", "true")
	t.Setenv("CODEX2API_POLICY_IDENTITY_FORWARD_ENABLED", "true")
	t.Setenv("CODEX2API_POLICY_BINDINGS", fmt.Sprintf(`[{
		"platform_id":"primary-newapi","target":"http://127.0.0.1:18095",
		"codex_key_fingerprint":"%s","secret":"%s","enabled":true
	}]`, overriddenFingerprint, testCodex2APIPolicySecret))
	c, info, _ := testCodex2APIPolicyContext(t, `{}`)
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18095/v1/responses", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+overriddenKey)
	require.NoError(t, SignCodex2APIRequest(c, info, req))
	state, ok := req.Context().Value(policyContextKey).(codex2APIPolicyRequestState)
	require.True(t, ok)
	require.Equal(t, overriddenFingerprint, state.KeyFingerprint)
	require.NotEmpty(t, req.Header.Get(policyHeaderSignature))
}

func TestSignCodex2APIRequestDoesNotFallbackAfterExplicitCredentialOverride(t *testing.T) {
	setTestCodex2APIPolicyEnv(t, "http://127.0.0.1:18095", true)
	c, info, _ := testCodex2APIPolicyContext(t, `{}`)
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18095/v1/responses", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer an-unbound-overridden-key")
	req.Header.Set(policyHeaderUserID, "forged")
	require.NoError(t, SignCodex2APIRequest(c, info, req))
	require.Empty(t, req.Header.Get(policyHeaderSignature))
	require.Empty(t, req.Header.Get(policyHeaderUserID))
}

func TestSignCodex2APIRequestClearsStateWhenTargetBecomesUnbound(t *testing.T) {
	setTestCodex2APIPolicyEnv(t, "http://127.0.0.1:18095", true)
	c, info, _ := testCodex2APIPolicyContext(t, `{}`)
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18095/v1/responses", strings.NewReader(`{}`))
	require.NoError(t, SignCodex2APIRequest(c, info, req))
	require.True(t, IsCodex2APIPolicyRequest(req))
	unboundReq, err := http.NewRequest(http.MethodPost, "https://unbound.example.test/v1/responses", nil)
	require.NoError(t, err)
	req.URL = unboundReq.URL
	require.NoError(t, SignCodex2APIRequest(c, info, req))
	require.False(t, IsCodex2APIPolicyRequest(req))
	require.Empty(t, req.Header.Get(policyHeaderSignature))
}

func TestSignCodex2APIRequestClearsStateWhenCredentialChanges(t *testing.T) {
	setTestCodex2APIPolicyEnv(t, "http://127.0.0.1:18095", true)
	c, info, _ := testCodex2APIPolicyContext(t, `{}`)
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18095/v1/responses", strings.NewReader(`{}`))
	require.NoError(t, SignCodex2APIRequest(c, info, req))
	require.True(t, IsCodex2APIPolicyRequest(req))
	c.Set(policyViolationContextKey, true)

	// A later channel/header override can replace the actual credential while
	// reusing the same request object. The previous binding state and reserved
	// headers must be removed rather than being paired with the new key.
	req.Header.Set("Authorization", "Bearer credential-not-bound")
	require.NoError(t, SignCodex2APIRequest(c, info, req))
	require.False(t, IsCodex2APIPolicyRequest(req))
	require.False(t, c.GetBool(policyViolationContextKey))
	require.Empty(t, req.Header.Get(policyHeaderUserID))
	require.Empty(t, req.Header.Get(policyHeaderSignature))
}

func TestSignCodex2APIRequestDoesNotLeakHeadersToUnmatchedTarget(t *testing.T) {
	setTestCodex2APIPolicyEnv(t, "http://127.0.0.1:18095", true)
	c, info, _ := testCodex2APIPolicyContext(t, `{}`)
	req := httptest.NewRequest(http.MethodPost, "https://api.example.test/v1/responses", strings.NewReader(`{}`))
	require.NoError(t, SignCodex2APIRequest(c, info, req))
	require.Empty(t, req.Header.Get(policyHeaderUserID))
	require.Empty(t, req.Header.Get(policyHeaderSignature))
}

func TestSanitizeCodex2APIPolicyRequestHeadersRemovesReservedNamespaces(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://example.com/v1/responses", nil)
	req.Header["x-newapi-future"] = []string{"forged"}
	req.Header["X-Codex2API-Policy-Future"] = []string{"forged"}
	req.Header["X-NewAPI-User-ID"] = []string{"forged"}
	req.Header.Set("X-Trace-ID", "keep")
	SanitizeCodex2APIPolicyRequestHeaders(req)
	for name := range req.Header {
		lower := strings.ToLower(name)
		require.False(t, strings.HasPrefix(lower, "x-newapi-"))
		require.False(t, strings.HasPrefix(lower, "x-codex2api-policy-"))
	}
	require.Equal(t, "keep", req.Header.Get("X-Trace-ID"))
}

func TestVerifyCodex2APIPolicyResponseUsesSignedRequestState(t *testing.T) {
	setTestCodex2APIPolicyEnv(t, "http://127.0.0.1:18095", true)
	c, info, _ := testCodex2APIPolicyContext(t, `{}`)
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18095/v1/responses", strings.NewReader(`{}`))
	require.NoError(t, SignCodex2APIRequest(c, info, req))

	evidence := sha256.Sum256([]byte("policy evidence"))
	decision := codex2APIPolicyDecision{
		RequestID: req.Header.Get(policyHeaderRequestID), DecisionID: "dec_test_1", Action: "block",
		Profile: "balanced", ReasonCode: "prompt_policy", Severity: "critical", StrikeEligible: true,
		RuleVersion: "rule-1", EvidenceSHA256: hex.EncodeToString(evidence[:]),
	}
	resp := &http.Response{StatusCode: http.StatusBadRequest, Header: make(http.Header), Request: req}
	resp.Header.Set(policyResponseViolation, "true")
	resp.Header.Set(policyResponseRequestID, decision.RequestID)
	resp.Header.Set(policyResponseDecisionID, decision.DecisionID)
	resp.Header.Set(policyResponseAction, decision.Action)
	resp.Header.Set(policyResponseProfile, decision.Profile)
	resp.Header.Set(policyResponseReason, decision.ReasonCode)
	resp.Header.Set(policyResponseSeverity, decision.Severity)
	resp.Header.Set(policyResponseRuleVersion, decision.RuleVersion)
	resp.Header.Set(policyResponseStrikeEligible, "true")
	resp.Header.Set(policyResponseEvidenceSHA256, decision.EvidenceSHA256)
	resp.Header.Set(policyResponseSignatureVersion, policyDecisionSignatureVersionV1)
	canonical := strings.Join([]string{policyDecisionSignaturePrefix, decision.RequestID, decision.DecisionID, decision.Action, decision.Profile, decision.ReasonCode, decision.Severity, "true", decision.RuleVersion, decision.EvidenceSHA256}, "\n")
	resp.Header.Set(policyResponseSignature, signPolicyValue(testCodex2APIPolicySecret, canonical))
	require.True(t, VerifyCodex2APIPolicyResponse(resp))

	resp.Header.Set(policyResponseDecisionID, "tampered")
	require.False(t, VerifyCodex2APIPolicyResponse(resp))
}

func TestStripCodex2APIPolicyResponseHeadersRemovesNonCanonicalKeys(t *testing.T) {
	resp := &http.Response{Header: http.Header{
		"x-codex2api-policy-violation":   []string{"true"},
		"X-Codex2API-Policy-Decision-ID": []string{"decision"},
		"Content-Type":                   []string{"application/json"},
	}}
	StripCodex2APIPolicyResponseHeaders(resp)
	require.Empty(t, resp.Header.Get("X-Codex2API-Policy-Violation"))
	require.Empty(t, resp.Header.Get("X-Codex2API-Policy-Decision-ID"))
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	for name := range resp.Header {
		require.NotEqual(t, "x-codex2api-policy-violation", strings.ToLower(name))
		require.NotEqual(t, "x-codex2api-policy-decision-id", strings.ToLower(name))
	}
}

func signedPolicyEnvelopeForTest(t *testing.T, req *http.Request, eventID string) (codex2APIPolicyDecision, []byte) {
	t.Helper()
	evidence := sha256.Sum256([]byte("stream policy evidence"))
	decision := codex2APIPolicyDecision{
		RequestID: req.Header.Get(policyHeaderRequestID), DecisionID: "dec_stream_test", EventID: eventID,
		Action: "block", Profile: "balanced", ReasonCode: "upstream_cyber_policy", Severity: "critical",
		StrikeEligible: true, RuleVersion: "rule-stream-1", EvidenceSHA256: hex.EncodeToString(evidence[:]),
	}
	envelope := codex2APIPolicyDecisionEnvelope{
		RequestID: decision.RequestID, DecisionID: decision.DecisionID, EventID: decision.EventID,
		Action: decision.Action, Profile: decision.Profile, ReasonCode: decision.ReasonCode,
		Severity: decision.Severity, StrikeEligible: decision.StrikeEligible,
		RuleVersion: decision.RuleVersion, EvidenceSHA256: decision.EvidenceSHA256,
		SignatureVersion: policyDecisionSignatureVersionV1,
	}
	canonical := strings.Join([]string{policyDecisionSignaturePrefix, decision.RequestID, decision.DecisionID, decision.Action, decision.Profile, decision.ReasonCode, decision.Severity, "true", decision.RuleVersion, decision.EvidenceSHA256}, "\n")
	envelope.ResponseSignature = signPolicyValue(testCodex2APIPolicySecret, canonical)
	if eventID != "" {
		envelope.EventSignatureVersion = policyEventSignatureVersionV1
		eventCanonical := strings.Join([]string{policyEventSignaturePrefix, decision.RequestID, decision.DecisionID, decision.EventID, decision.Action, decision.Profile, decision.ReasonCode, decision.Severity, "true", decision.RuleVersion, decision.EvidenceSHA256}, "\n")
		envelope.EventSignature = signPolicyValue(testCodex2APIPolicySecret, eventCanonical)
	}
	envelopeBytes, err := common.Marshal(envelope)
	require.NoError(t, err)
	return decision, envelopeBytes
}

func signedPolicyResponseForTest(t *testing.T, req *http.Request, decisionID string) *http.Response {
	t.Helper()
	evidence := sha256.Sum256([]byte("database policy evidence " + decisionID))
	decision := codex2APIPolicyDecision{
		RequestID: req.Header.Get(policyHeaderRequestID), DecisionID: decisionID, Action: "block",
		Profile: "balanced", ReasonCode: "upstream_cyber_policy", Severity: "critical", StrikeEligible: true,
		RuleVersion: "rule-db-1", EvidenceSHA256: hex.EncodeToString(evidence[:]),
	}
	canonical := strings.Join([]string{policyDecisionSignaturePrefix, decision.RequestID, decision.DecisionID, decision.Action, decision.Profile, decision.ReasonCode, decision.Severity, "true", decision.RuleVersion, decision.EvidenceSHA256}, "\n")
	resp := &http.Response{StatusCode: http.StatusBadRequest, Header: make(http.Header), Request: req, Body: io.NopCloser(strings.NewReader(`{"error":{"code":"cyber_policy"}}`))}
	resp.Header.Set(policyResponseViolation, "true")
	resp.Header.Set(policyResponseRequestID, decision.RequestID)
	resp.Header.Set(policyResponseDecisionID, decision.DecisionID)
	resp.Header.Set(policyResponseAction, decision.Action)
	resp.Header.Set(policyResponseProfile, decision.Profile)
	resp.Header.Set(policyResponseReason, decision.ReasonCode)
	resp.Header.Set(policyResponseSeverity, decision.Severity)
	resp.Header.Set(policyResponseRuleVersion, decision.RuleVersion)
	resp.Header.Set(policyResponseStrikeEligible, "true")
	resp.Header.Set(policyResponseEvidenceSHA256, decision.EvidenceSHA256)
	resp.Header.Set(policyResponseSignatureVersion, policyDecisionSignatureVersionV1)
	resp.Header.Set(policyResponseSignature, signPolicyValue(testCodex2APIPolicySecret, canonical))
	return resp
}

func TestCodex2APIPolicySSEDecisionIsProcessedAfterHeadersCommit(t *testing.T) {
	setTestCodex2APIPolicyEnv(t, "http://127.0.0.1:18095", true)
	c, info, _ := testCodex2APIPolicyContext(t, `{}`)
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18095/v1/responses", strings.NewReader(`{}`))
	require.NoError(t, SignCodex2APIRequest(c, info, req))
	decision, envelope := signedPolicyEnvelopeForTest(t, req, "responses:1")
	body := append([]byte("event: response.failed\ndata: "), envelope...)
	body = append(body, []byte("\n\n")...)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Request:    req,
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
	WrapCodex2APIPolicyResponseBody(c, resp)
	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, body, got, "policy parser must be transparent to downstream stream handlers")
	require.True(t, c.GetBool(policyViolationContextKey))
	stored, exists := c.Get("codex2api_policy_decision")
	require.True(t, exists)
	require.Equal(t, decision.DecisionID, stored.(codex2APIPolicyDecision).DecisionID)
}

func TestCodex2APIPolicyInvalidSSEDecisionIsIgnored(t *testing.T) {
	setTestCodex2APIPolicyEnv(t, "http://127.0.0.1:18095", true)
	c, info, _ := testCodex2APIPolicyContext(t, `{}`)
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18095/v1/responses", strings.NewReader(`{}`))
	require.NoError(t, SignCodex2APIRequest(c, info, req))
	_, envelope := signedPolicyEnvelopeForTest(t, req, "responses:2")
	// Tamper with the signed decision while retaining a valid JSON envelope.
	var decoded map[string]any
	require.NoError(t, common.Unmarshal(envelope, &decoded))
	decoded["decision_id"] = "tampered"
	tampered, err := common.Marshal(decoded)
	require.NoError(t, err)
	body := append([]byte("data: "), tampered...)
	body = append(body, []byte("\n\n")...)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Request:    req,
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
	WrapCodex2APIPolicyResponseBody(c, resp)
	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.False(t, c.GetBool(policyViolationContextKey))
}

func TestCodex2APIPolicyJSONDecisionIsProcessedTransparently(t *testing.T) {
	setTestCodex2APIPolicyEnv(t, "http://127.0.0.1:18095", true)
	c, info, _ := testCodex2APIPolicyContext(t, `{}`)
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18095/v1/responses", strings.NewReader(`{}`))
	require.NoError(t, SignCodex2APIRequest(c, info, req))
	decision, envelope := signedPolicyEnvelopeForTest(t, req, "")
	body := []byte(`{"error":{"details":{"codex2api_policy":` + string(envelope) + `}}}`)
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Request:    req,
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
	WrapCodex2APIPolicyResponseBody(c, resp)
	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, body, got)
	require.True(t, c.GetBool(policyViolationContextKey))
	stored, exists := c.Get("codex2api_policy_decision")
	require.True(t, exists)
	require.Equal(t, decision.DecisionID, stored.(codex2APIPolicyDecision).DecisionID)
}

func TestCodex2APIPolicyTargetMatchesWebSocketScheme(t *testing.T) {
	setTestCodex2APIPolicyEnv(t, "https://127.0.0.1:18095", true)
	c, info, _ := testCodex2APIPolicyContext(t, `{}`)
	req := httptest.NewRequest(http.MethodGet, "wss://127.0.0.1:18095/v1/responses", http.NoBody)
	require.NoError(t, SignCodex2APIRequest(c, info, req))
	require.Equal(t, "1", req.Header.Get(policyHeaderSignatureVersion))
}

func TestCodex2APIPolicyWebSocketHandshakeUsesGET(t *testing.T) {
	setTestCodex2APIPolicyEnv(t, "https://127.0.0.1:18095", true)
	c, info, _ := testCodex2APIPolicyContext(t, `{}`)
	// The incoming relay request may be POST-shaped, but the outbound upgrade
	// request that Codex2API verifies is always a GET handshake.
	req := httptest.NewRequest(http.MethodGet, "wss://127.0.0.1:18095/v1/realtime", http.NoBody)
	require.NoError(t, SignCodex2APIRequest(c, info, req))
	require.Equal(t, http.MethodGet, req.Header.Get(policyHeaderMethod))
}

func TestCodex2APIPolicyVerifiedStrikesDisableUserAndBlockIP(t *testing.T) {
	require.NotNil(t, model.DB)
	require.NoError(t, model.DB.AutoMigrate(&model.Codex2APIPolicyStrike{}, &model.Codex2APIPolicyIPBlock{}, &model.User{}, &model.Token{}))
	const userID = 98001
	const tokenID = 98002
	const clientIP = "203.0.113.77"
	// Keep this fixture isolated from other service tests and from any rows left
	// by an interrupted local run.
	model.DB.Unscoped().Where("id = ?", tokenID).Delete(&model.Token{})
	model.DB.Unscoped().Where("id = ?", userID).Delete(&model.User{})
	model.DB.Where("user_id = ? OR client_ip = ?", userID, clientIP).Delete(&model.Codex2APIPolicyStrike{})
	model.DB.Where("ip = ?", clientIP).Delete(&model.Codex2APIPolicyIPBlock{})
	user := &model.User{Id: userID, Username: "policy-test-user", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "policy-test-aff"}
	require.NoError(t, model.DB.Create(user).Error)
	token := &model.Token{Id: tokenID, UserId: userID, Key: "policy-test-token", Status: common.TokenStatusEnabled, Name: "policy-test-token", ExpiredTime: -1}
	require.NoError(t, model.DB.Create(token).Error)
	t.Cleanup(func() {
		model.DB.Unscoped().Where("user_id = ?", userID).Delete(&model.Token{})
		model.DB.Unscoped().Where("id = ?", userID).Delete(&model.User{})
		model.DB.Where("user_id = ? OR client_ip = ?", userID, clientIP).Delete(&model.Codex2APIPolicyStrike{})
		model.DB.Where("ip = ?", clientIP).Delete(&model.Codex2APIPolicyIPBlock{})
	})

	t.Setenv("CODEX2API_POLICY_ENABLED", "true")
	t.Setenv("CODEX2API_POLICY_IDENTITY_FORWARD_ENABLED", "true")
	t.Setenv("CODEX2API_POLICY_AUDIT_ENABLED", "true")
	// Account/IP enforcement must remain independent from the optional strike
	// flag: verified eligible decisions still reach the selected punishment path
	// when strike accumulation itself is disabled.
	t.Setenv("CODEX2API_POLICY_STRIKE_ENABLED", "false")
	t.Setenv("CODEX2API_POLICY_ACCOUNT_BAN_ENABLED", "true")
	t.Setenv("CODEX2API_POLICY_IP_BLOCK_ENABLED", "true")
	t.Setenv("CODEX2API_POLICY_BAN_AFTER", "2")
	t.Setenv("CODEX2API_POLICY_WINDOW_SECONDS", "3600")
	fingerprint := Codex2APIKeyFingerprint("codex2api-key-test")
	t.Setenv("CODEX2API_POLICY_BINDINGS", fmt.Sprintf(`[{"platform_id":"primary-newapi","target":"http://127.0.0.1:18095","codex_key_fingerprint":"%s","secret":"%s","enabled":true}]`, fingerprint, testCodex2APIPolicySecret))

	for index := 1; index <= 2; index++ {
		c, info, _ := testCodex2APIPolicyContext(t, `{}`)
		c.Set("id", userID)
		c.Set("token_id", tokenID)
		c.Request.RemoteAddr = clientIP + ":4567"
		info.RequestId = fmt.Sprintf("db-policy-request-%d", index)
		req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18095/v1/responses", strings.NewReader(`{}`))
		require.NoError(t, SignCodex2APIRequest(c, info, req))
		resp := signedPolicyResponseForTest(t, req, fmt.Sprintf("dec_db_%d", index))
		ProcessCodex2APIResponse(c, resp)
	}

	var gotUser model.User
	require.NoError(t, model.DB.First(&gotUser, userID).Error)
	require.Equal(t, common.UserStatusDisabled, gotUser.Status)
	var gotToken model.Token
	require.NoError(t, model.DB.First(&gotToken, tokenID).Error)
	require.Equal(t, common.TokenStatusDisabled, gotToken.Status)
	var block model.Codex2APIPolicyIPBlock
	require.NoError(t, model.DB.Where("ip = ?", clientIP).First(&block).Error)
	require.Greater(t, block.ExpiresAt, time.Now().Unix())
	var strikeCount int64
	require.NoError(t, model.DB.Model(&model.Codex2APIPolicyStrike{}).Where("user_id = ?", userID).Count(&strikeCount).Error)
	require.Equal(t, int64(2), strikeCount)
}

func TestBodySHA256ReaderOnlyRestoresPosition(t *testing.T) {
	storage, err := common.CreateBodyStorage([]byte("body"))
	require.NoError(t, err)
	defer storage.Close()
	reader := common.ReaderOnly(storage)
	provider, ok := reader.(interface{ BodySHA256() (string, error) })
	require.True(t, ok)
	_, exposesCloser := reader.(io.Closer)
	require.False(t, exposesCloser, "ReaderOnly must not expose the underlying closer to net/http")
	digest, err := provider.BodySHA256()
	require.NoError(t, err)
	want := sha256.Sum256([]byte("body"))
	require.Equal(t, hex.EncodeToString(want[:]), digest)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, "body", string(data))
}

func TestBodySHA256UsesReplayReaderWithoutConsumingLiveBody(t *testing.T) {
	payload := strings.Repeat("x", 64*1024)
	req, err := http.NewRequest(http.MethodPost, "https://example.com/v1/responses", strings.NewReader(payload))
	require.NoError(t, err)
	digest, err := bodySHA256FromRequest(req)
	require.NoError(t, err)
	want := sha256.Sum256([]byte(payload))
	require.Equal(t, hex.EncodeToString(want[:]), digest)
	remaining, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, payload, string(remaining))
}
