package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// Codex2APIPolicyBinding binds one outbound Codex2API key and one upstream
// target to a signing secret.  The raw upstream key is never stored in this
// structure; callers configure only its SHA-256 fingerprint.
type Codex2APIPolicyBinding struct {
	PlatformID          string `json:"platform_id"`
	Target              string `json:"target"`
	CodexKeyFingerprint string `json:"codex_key_fingerprint"`
	Secret              string `json:"secret"`
	Enabled             bool   `json:"enabled"`
}

// Codex2APIPolicyConfig controls the optional NewAPI <-> Codex2API adapter.
// Every enforcement switch defaults to false.  Identity forwarding is also
// opt-in so an accidental environment variable cannot leak user metadata to a
// non-Codex upstream.
type Codex2APIPolicyConfig struct {
	Enabled                bool
	IdentityForwardEnabled bool
	AuditEnabled           bool
	StrikeEnabled          bool
	AccountBanEnabled      bool
	IPBlockEnabled         bool
	BanAfter               int
	WindowSeconds          int64
	Bindings               []Codex2APIPolicyBinding
}

const (
	policyHeaderUserID                  = "X-NewAPI-User-ID"
	policyHeaderClientIP                = "X-NewAPI-Client-IP"
	policyHeaderRequestID               = "X-NewAPI-Request-ID"
	policyHeaderTimestamp               = "X-NewAPI-Timestamp"
	policyHeaderMethod                  = "X-NewAPI-Method"
	policyHeaderPath                    = "X-NewAPI-Path"
	policyHeaderBodySHA256              = "X-NewAPI-Body-SHA256"
	policyHeaderSignatureVersion        = "X-NewAPI-Signature-Version"
	policyHeaderSignature               = "X-NewAPI-Signature"
	policyHeaderMeta                    = "X-NewAPI-Policy-Meta"
	policyHeaderMetaSignature           = "X-NewAPI-Policy-Meta-Signature"
	policyResponseViolation             = "X-Codex2API-Policy-Violation"
	policyResponseRequestID             = "X-Codex2API-Policy-Request-ID"
	policyResponseReason                = "X-Codex2API-Policy-Reason"
	policyResponseAction                = "X-Codex2API-Policy-Action"
	policyResponseDecisionID            = "X-Codex2API-Policy-Decision-ID"
	policyResponseEventID               = "X-Codex2API-Policy-Event-ID"
	policyResponseEventSignatureVersion = "X-Codex2API-Policy-Event-Signature-Version"
	policyResponseEventSignature        = "X-Codex2API-Policy-Event-Signature"
	policyResponseProfile               = "X-Codex2API-Policy-Profile"
	policyResponseRuleVersion           = "X-Codex2API-Policy-Rule-Version"
	policyResponseStrikeEligible        = "X-Codex2API-Policy-Strike-Eligible"
	policyResponseEvidenceSHA256        = "X-Codex2API-Policy-Evidence-SHA256"
	policyResponseSeverity              = "X-Codex2API-Policy-Severity"
	policyResponseSignatureVersion      = "X-Codex2API-Policy-Signature-Version"
	policyResponseSignature             = "X-Codex2API-Policy-Response-Signature"
	policySignatureVersionV1            = "1"
	policyCanonicalVersionV1            = "v1"
	policyDecisionSignatureVersionV1    = "v1"
	policyEventSignatureVersionV1       = "v1"
	policyMetaSignatureVersionV1        = "policy-meta-v1"
	policyDecisionSignaturePrefix       = "policy-decision-v1"
	policyEventSignaturePrefix          = "policy-event-v1"
	maxPolicyBodyBuffer                 = 256 << 20
	maxPolicyEventBuffer                = 4 << 20
	maxPolicyJSONProbe                  = 1 << 20
	policyStateContextKey               = "codex2api_policy_state"
	policyTrackerContextKey             = "codex2api_policy_tracker"
)

// Codex2APIPolicyViolationContextKey is set on the current Gin request after
// a signed upstream decision is verified. Controllers use it to prevent an
// authoritative policy block from being retried on another channel.
const Codex2APIPolicyViolationContextKey = "codex2api_policy_violation"

const policyViolationContextKey = Codex2APIPolicyViolationContextKey

type codex2APIPolicyRequestState struct {
	RequestID      string
	UserID         string
	ClientIP       string
	BodySHA256     string
	Secret         string
	PlatformID     string
	KeyFingerprint string
	Target         string
}

// codex2APIPolicyDecisionEnvelope is the compact decision object embedded in
// a Responses `response.failed`/`error` event when HTTP headers have already
// been committed (for example, an SSE stream).  Its field names intentionally
// mirror Codex2API's public wire format.
type codex2APIPolicyDecisionEnvelope struct {
	RequestID             string `json:"request_id"`
	DecisionID            string `json:"decision_id"`
	EventID               string `json:"event_id,omitempty"`
	Action                string `json:"action"`
	Profile               string `json:"profile"`
	ReasonCode            string `json:"reason_code"`
	Severity              string `json:"severity"`
	StrikeEligible        bool   `json:"strike_eligible"`
	RuleVersion           string `json:"rule_version"`
	EvidenceSHA256        string `json:"evidence_sha256"`
	SignatureVersion      string `json:"signature_version"`
	ResponseSignature     string `json:"response_signature"`
	EventSignatureVersion string `json:"event_signature_version,omitempty"`
	EventSignature        string `json:"event_signature,omitempty"`
}

// codex2APIPolicyTracker is request-scoped state shared by the HTTP response
// reader and (for realtime/WebSocket relays) the target-reader goroutine.  A
// decision can be visible both in response headers and in an SSE/event body;
// the tracker makes processing idempotent without relying on a database row.
type codex2APIPolicyTracker struct {
	mu        sync.Mutex
	state     codex2APIPolicyRequestState
	processed map[string]struct{}
}

func newCodex2APIPolicyTracker(state codex2APIPolicyRequestState) *codex2APIPolicyTracker {
	return &codex2APIPolicyTracker{state: state, processed: make(map[string]struct{})}
}

func (t *codex2APIPolicyTracker) markProcessed(decisionID string) bool {
	if t == nil || strings.TrimSpace(decisionID) == "" {
		return true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.processed[decisionID]; exists {
		return false
	}
	t.processed[decisionID] = struct{}{}
	return true
}

func (t *codex2APIPolicyTracker) snapshotState() codex2APIPolicyRequestState {
	if t == nil {
		return codex2APIPolicyRequestState{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state
}

type codex2APIPolicyContextKey struct{}

var policyContextKey codex2APIPolicyContextKey

var codex2APIPolicyRequestHeaderNames = []string{
	policyHeaderUserID, policyHeaderClientIP, policyHeaderRequestID, policyHeaderTimestamp,
	policyHeaderMethod, policyHeaderPath, policyHeaderBodySHA256, policyHeaderSignatureVersion,
	policyHeaderSignature, policyHeaderMeta, policyHeaderMetaSignature,
}

// policyMeta is intentionally a fixed-policy envelope.  Codex2API accepts
// these fields for audit context, but NewAPI never derives enforcement rules
// from caller-provided metadata.
type codex2APIPolicyMeta struct {
	PlatformID         string `json:"platform_id,omitempty"`
	UserName           string `json:"user_name,omitempty"`
	UserEmail          string `json:"user_email,omitempty"`
	UserGroup          string `json:"user_group,omitempty"`
	Profile            string `json:"profile"`
	Mode               string `json:"mode"`
	Provider           string `json:"provider"`
	Protocol           string `json:"protocol"`
	OriginalEndpoint   string `json:"original_endpoint,omitempty"`
	OriginalProtocol   string `json:"original_protocol,omitempty"`
	RequestedModel     string `json:"requested_model,omitempty"`
	UpstreamModel      string `json:"upstream_model,omitempty"`
	ChannelID          int    `json:"channel_id,omitempty"`
	SessionFingerprint string `json:"session_fingerprint,omitempty"`
}

type codex2APIPolicyDecision struct {
	RequestID      string
	DecisionID     string
	EventID        string
	Action         string
	Profile        string
	ReasonCode     string
	Severity       string
	StrikeEligible bool
	RuleVersion    string
	EvidenceSHA256 string
}

var (
	policyConfigMu   sync.RWMutex
	policyConfigRaw  string
	policyConfig     Codex2APIPolicyConfig
	policyConfigInit bool
)

// GetCodex2APIPolicyConfig reads and validates the adapter configuration.
// Environment variables are re-read when their combined value changes, which
// keeps tests and operator-triggered process configuration reloads predictable
// without exposing a mutable global config to request handlers.
func GetCodex2APIPolicyConfig() Codex2APIPolicyConfig {
	raw := strings.Join([]string{
		os.Getenv("CODEX2API_POLICY_ENABLED"),
		os.Getenv("CODEX2API_POLICY_IDENTITY_FORWARD_ENABLED"),
		os.Getenv("CODEX2API_POLICY_AUDIT_ENABLED"),
		os.Getenv("CODEX2API_POLICY_STRIKE_ENABLED"),
		os.Getenv("CODEX2API_POLICY_ACCOUNT_BAN_ENABLED"),
		os.Getenv("CODEX2API_POLICY_IP_BLOCK_ENABLED"),
		os.Getenv("CODEX2API_POLICY_BAN_AFTER"),
		os.Getenv("CODEX2API_POLICY_WINDOW_SECONDS"),
		os.Getenv("CODEX2API_POLICY_BINDINGS"),
	}, "\x00")

	policyConfigMu.RLock()
	if policyConfigInit && policyConfigRaw == raw {
		cfg := policyConfig
		cfg.Bindings = append([]Codex2APIPolicyBinding(nil), policyConfig.Bindings...)
		policyConfigMu.RUnlock()
		return cfg
	}
	policyConfigMu.RUnlock()

	cfg := Codex2APIPolicyConfig{
		Enabled:                common.GetEnvOrDefaultBool("CODEX2API_POLICY_ENABLED", false),
		IdentityForwardEnabled: common.GetEnvOrDefaultBool("CODEX2API_POLICY_IDENTITY_FORWARD_ENABLED", false),
		AuditEnabled:           common.GetEnvOrDefaultBool("CODEX2API_POLICY_AUDIT_ENABLED", false),
		StrikeEnabled:          common.GetEnvOrDefaultBool("CODEX2API_POLICY_STRIKE_ENABLED", false),
		AccountBanEnabled:      common.GetEnvOrDefaultBool("CODEX2API_POLICY_ACCOUNT_BAN_ENABLED", false),
		IPBlockEnabled:         common.GetEnvOrDefaultBool("CODEX2API_POLICY_IP_BLOCK_ENABLED", false),
		BanAfter:               common.GetEnvOrDefault("CODEX2API_POLICY_BAN_AFTER", 2),
		WindowSeconds:          int64(common.GetEnvOrDefault("CODEX2API_POLICY_WINDOW_SECONDS", 86400)),
	}
	if cfg.BanAfter < 1 {
		cfg.BanAfter = 1
	}
	if cfg.BanAfter > 1000 {
		cfg.BanAfter = 1000
	}
	if cfg.WindowSeconds < 60 {
		cfg.WindowSeconds = 60
	}
	if cfg.WindowSeconds > 90*24*60*60 {
		cfg.WindowSeconds = 90 * 24 * 60 * 60
	}
	cfg.Bindings = parseCodex2APIPolicyBindings(os.Getenv("CODEX2API_POLICY_BINDINGS"))

	policyConfigMu.Lock()
	policyConfigRaw = raw
	cfg.Bindings = append([]Codex2APIPolicyBinding(nil), cfg.Bindings...)
	policyConfig = cfg
	policyConfigInit = true
	policyConfigMu.Unlock()
	return cfg
}

func parseCodex2APIPolicyBindings(raw string) []Codex2APIPolicyBinding {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var entries []Codex2APIPolicyBinding
	if err := common.Unmarshal([]byte(raw), &entries); err != nil {
		common.SysError("invalid CODEX2API_POLICY_BINDINGS: " + err.Error())
		return nil
	}
	valid := make([]Codex2APIPolicyBinding, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		entry.PlatformID = strings.ToLower(strings.TrimSpace(entry.PlatformID))
		entry.Target = strings.TrimSpace(entry.Target)
		entry.CodexKeyFingerprint = strings.ToLower(strings.TrimSpace(entry.CodexKeyFingerprint))
		entry.Secret = strings.TrimSpace(entry.Secret)
		if !validCodex2APIPlatformID(entry.PlatformID) || entry.Target == "" || entry.Secret == "" || len([]byte(entry.Secret)) < 32 || !isHexDigest(entry.CodexKeyFingerprint, 64) {
			continue
		}
		if !validCodex2APITarget(entry.Target) {
			continue
		}
		// A key/target pair has one authoritative platform binding. Reject
		// duplicate pairs even when their platform labels differ; otherwise the
		// selected secret would depend on JSON order and could diverge from
		// Codex2API's one-key/one-platform binding.
		key := entry.Target + "\x00" + entry.CodexKeyFingerprint
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		valid = append(valid, entry)
	}
	return valid
}

// Codex2API stores platform codes using ^[a-z0-9][a-z0-9_-]{0,31}$.
// Validate the same compact form here so a malformed local binding cannot
// silently produce a request that the receiver will reject.
func validCodex2APIPlatformID(value string) bool {
	if len(value) < 1 || len(value) > 32 {
		return false
	}
	for index, char := range []byte(value) {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || (index > 0 && (char == '_' || char == '-')) {
			continue
		}
		return false
	}
	return true
}

// Codex2APIKeyFingerprint returns the exact SHA-256 fingerprint expected in a
// binding.  It hashes the configured upstream key byte-for-byte, including any
// provider-specific JSON or prefix.
func Codex2APIKeyFingerprint(key string) string {
	digest := sha256.Sum256([]byte(key))
	return hex.EncodeToString(digest[:])
}

func isHexDigest(value string, length int) bool {
	if len(value) != length {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validCodex2APITarget(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "ws" && parsed.Scheme != "wss") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	return true
}

func policySchemesMatch(configured, actual string) bool {
	configured = strings.ToLower(strings.TrimSpace(configured))
	actual = strings.ToLower(strings.TrimSpace(actual))
	if configured == actual {
		return true
	}
	return (configured == "http" && actual == "ws") ||
		(configured == "ws" && actual == "http") ||
		(configured == "https" && actual == "wss") ||
		(configured == "wss" && actual == "https")
}

func codex2APITargetMatches(configured, actual string) bool {
	if !validCodex2APITarget(configured) {
		return false
	}
	want, err := url.Parse(strings.TrimSpace(configured))
	if err != nil {
		return false
	}
	have, err := url.Parse(strings.TrimSpace(actual))
	if err != nil || have == nil || have.User != nil {
		return false
	}
	if !policySchemesMatch(want.Scheme, have.Scheme) || !strings.EqualFold(want.Host, have.Host) {
		return false
	}
	prefix := strings.TrimSuffix(want.EscapedPath(), "/")
	if prefix == "" {
		return true
	}
	path := have.EscapedPath()
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// outboundCodex2APIKey returns the credential that the final outbound
// request will actually present to Codex2API. Header overrides are applied
// before this function runs, so using the selected channel key alone could
// accidentally bind a signature to a different credential (or leak a valid
// identity alongside an overridden credential).
func outboundCodex2APIKey(info *relaycommon.RelayInfo, req *http.Request) (string, bool) {
	if req != nil && req.Header != nil {
		if auth := strings.TrimSpace(req.Header.Get("Authorization")); auth != "" {
			// Match Codex2API's authentication parser exactly: it strips the
			// case-sensitive `Bearer ` prefix and otherwise treats the complete
			// Authorization value as the key.
			if strings.HasPrefix(auth, "Bearer ") {
				return strings.TrimSpace(auth[len("Bearer "):]), true
			}
			// Codex2API treats a non-Bearer Authorization value as the raw key;
			// preserve that behavior for exact fingerprint matching.
			return auth, true
		}
		for _, headerName := range []string{"x-api-key", "anthropic-auth-token"} {
			if value := strings.TrimSpace(req.Header.Get(headerName)); value != "" {
				return value, true
			}
		}
		// OpenAI's realtime client can put its key in the subprotocol list.
		for _, item := range strings.Split(req.Header.Get("Sec-WebSocket-Protocol"), ",") {
			item = strings.TrimSpace(item)
			const prefix = "openai-insecure-api-key."
			if strings.HasPrefix(item, prefix) {
				return strings.TrimSpace(strings.TrimPrefix(item, prefix)), true
			}
		}
	}
	if info == nil || info.ChannelMeta == nil {
		return "", false
	}
	// When no explicit credential header exists, the selected channel key is
	// the credential used by the standard adaptors. Do not normalize it before
	// fingerprinting: bindings intentionally identify the exact bytes.
	key := strings.TrimSpace(info.ApiKey)
	return key, key != ""
}

func resolveCodex2APIPolicyBinding(info *relaycommon.RelayInfo, req *http.Request) (Codex2APIPolicyBinding, bool, string) {
	key, ok := outboundCodex2APIKey(info, req)
	if !ok {
		return Codex2APIPolicyBinding{}, false, ""
	}
	fingerprint := Codex2APIKeyFingerprint(key)
	target := ""
	if req != nil && req.URL != nil {
		target = req.URL.String()
	}
	for _, binding := range GetCodex2APIPolicyConfig().Bindings {
		if !binding.Enabled || !strings.EqualFold(binding.CodexKeyFingerprint, fingerprint) || !codex2APITargetMatches(binding.Target, target) {
			continue
		}
		return binding, true, fingerprint
	}
	return Codex2APIPolicyBinding{}, false, ""
}

func policyRequestID(info *relaycommon.RelayInfo, contexts ...*gin.Context) string {
	base := ""
	if info != nil {
		base = strings.TrimSpace(info.RequestId)
		if base != "" && info.RetryIndex > 0 {
			return fmt.Sprintf("%s-r%d", base, info.RetryIndex)
		}
	}
	var c *gin.Context
	if len(contexts) > 0 {
		c = contexts[0]
	}
	if base == "" && c != nil {
		base = strings.TrimSpace(c.GetString(common.RequestIdKey))
	}
	if base == "" {
		return common.NewRequestId()
	}
	return base
}

func policyPath(req *http.Request) string {
	if req == nil || req.URL == nil {
		return "/"
	}
	path := req.URL.EscapedPath()
	if path == "" {
		path = req.URL.Path
	}
	if path == "" {
		return "/"
	}
	return path
}

func bodySHA256FromRequest(req *http.Request) (string, error) {
	empty := sha256.Sum256(nil)
	if req == nil || req.Body == nil || req.Body == http.NoBody {
		return hex.EncodeToString(empty[:]), nil
	}
	if provider, ok := req.Body.(interface{ BodySHA256() (string, error) }); ok {
		// A wrapper may expose BodySHA256 even when its underlying reader is not
		// seekable. In that case fall through to the bounded buffering path rather
		// than rejecting an otherwise valid request outright.
		if digest, err := provider.BodySHA256(); err == nil && isHexDigest(strings.ToLower(strings.TrimSpace(digest)), 64) {
			return strings.ToLower(strings.TrimSpace(digest)), nil
		}
	}
	// Prefer the request's replay reader when available.  net/http supplies
	// GetBody for bytes.Buffer/bytes.Reader/strings.Reader bodies; hashing that
	// independent reader avoids consuming the live body and avoids a second
	// full-size heap buffer for large but replayable custom requests.
	if req.GetBody != nil {
		clone, err := req.GetBody()
		if err != nil {
			return "", err
		}
		hasher := sha256.New()
		_, copyErr := io.Copy(hasher, clone)
		closeErr := clone.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		return hex.EncodeToString(hasher.Sum(nil)), nil
	}
	if seeker, ok := req.Body.(interface {
		io.Reader
		io.Seeker
	}); ok {
		current, err := seeker.Seek(0, io.SeekCurrent)
		if err != nil {
			return "", err
		}
		if _, err = seeker.Seek(0, io.SeekStart); err != nil {
			return "", err
		}
		hasher := sha256.New()
		_, copyErr := io.Copy(hasher, seeker)
		_, restoreErr := seeker.Seek(current, io.SeekStart)
		if copyErr != nil {
			return "", copyErr
		}
		if restoreErr != nil {
			return "", restoreErr
		}
		return hex.EncodeToString(hasher.Sum(nil)), nil
	}
	// Last-resort readers are buffered only for the signed request.  The limit
	// prevents a broken custom adaptor from turning signing into an unbounded
	// allocation; normal NewAPI body limits are lower than this value.
	data, err := io.ReadAll(io.LimitReader(req.Body, maxPolicyBodyBuffer+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxPolicyBodyBuffer {
		return "", fmt.Errorf("request body exceeds signed body limit")
	}
	_ = req.Body.Close()
	replacement := io.NopCloser(bytes.NewReader(data))
	req.Body = replacement
	req.ContentLength = int64(len(data))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func requestMeta(info *relaycommon.RelayInfo, c *gin.Context, binding Codex2APIPolicyBinding) codex2APIPolicyMeta {
	meta := codex2APIPolicyMeta{
		PlatformID: binding.PlatformID,
		Profile:    "balanced",
		Mode:       "enforce",
		Provider:   "codex2api",
		Protocol:   "unknown",
	}
	if c != nil {
		meta.UserName = c.GetString(string(constant.ContextKeyUserName))
		meta.UserEmail = c.GetString(string(constant.ContextKeyUserEmail))
		meta.UserGroup = c.GetString(string(constant.ContextKeyUserGroup))
	}
	if info == nil {
		return meta
	}
	if info.RelayFormat != "" {
		meta.Protocol = string(info.RelayFormat)
		meta.OriginalProtocol = string(info.RelayFormat)
	}
	meta.OriginalEndpoint = policyOriginalEndpoint(info.RequestURLPath)
	meta.RequestedModel = strings.TrimSpace(info.OriginModelName)
	if info.ChannelMeta != nil {
		meta.UpstreamModel = strings.TrimSpace(info.UpstreamModelName)
		meta.ChannelID = info.ChannelId
	}
	if info.TokenKey != "" {
		digest := sha256.Sum256([]byte(info.TokenKey))
		meta.SessionFingerprint = hex.EncodeToString(digest[:16])
	}
	return meta
}

func policyOriginalEndpoint(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if parsed, err := url.Parse(raw); err == nil {
		path := parsed.EscapedPath()
		if path != "" {
			return path
		}
	}
	if index := strings.IndexAny(raw, "?#"); index >= 0 {
		raw = raw[:index]
	}
	if !strings.HasPrefix(raw, "/") {
		return ""
	}
	return raw
}

func setPolicyRequestHeaders(c *gin.Context, info *relaycommon.RelayInfo, req *http.Request, binding Codex2APIPolicyBinding, keyFingerprint string, bodyDigest string) error {
	if req == nil || req.URL == nil {
		return errors.New("missing outbound request URL")
	}
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	bodyDigest = strings.ToLower(strings.TrimSpace(bodyDigest))
	if bodyDigest == "" || !isHexDigest(bodyDigest, 64) {
		return errors.New("invalid outbound body digest")
	}
	keyFingerprint = strings.ToLower(strings.TrimSpace(keyFingerprint))
	if !isHexDigest(keyFingerprint, 64) {
		return errors.New("invalid Codex2API key fingerprint")
	}
	requestID := policyRequestID(info, c)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodPost
	}
	path := policyPath(req)
	userID := "0"
	clientIP := ""
	if c != nil {
		if id := c.GetInt(string(constant.ContextKeyUserId)); id != 0 {
			userID = strconv.Itoa(id)
		} else if id := c.GetInt("id"); id != 0 {
			userID = strconv.Itoa(id)
		}
		if c.Request != nil {
			if clientIPValue := c.ClientIP(); clientIPValue != "" {
				clientIP = strings.TrimSpace(clientIPValue)
			}
		}
	}
	if userID == "0" && info != nil && info.UserId > 0 {
		userID = strconv.Itoa(info.UserId)
	}
	if net.ParseIP(clientIP) == nil {
		// c.ClientIP can be empty in synthetic tests.  Keep the signed value
		// explicit rather than allowing a client-provided header to fill it.
		clientIP = "0.0.0.0"
	}
	canonical := strings.Join([]string{policyCanonicalVersionV1, timestamp, requestID, userID, clientIP, method, path, bodyDigest}, "\n")
	signature := signPolicyValue(binding.Secret, canonical)
	if signature == "" {
		return errors.New("empty Codex2API policy signature")
	}

	// These names are security-sensitive.  Set (rather than Add) deliberately
	// overwrites values copied from the client or a channel header override.
	req.Header.Set(policyHeaderUserID, userID)
	req.Header.Set(policyHeaderClientIP, clientIP)
	req.Header.Set(policyHeaderRequestID, requestID)
	req.Header.Set(policyHeaderTimestamp, timestamp)
	req.Header.Set(policyHeaderMethod, method)
	req.Header.Set(policyHeaderPath, path)
	req.Header.Set(policyHeaderBodySHA256, bodyDigest)
	req.Header.Set(policyHeaderSignatureVersion, policySignatureVersionV1)
	req.Header.Set(policyHeaderSignature, signature)

	metaBytes, err := common.Marshal(requestMeta(info, c, binding))
	if err != nil {
		return fmt.Errorf("marshal policy metadata: %w", err)
	}
	if len(metaBytes) > 3072 {
		return errors.New("Codex2API policy metadata exceeds 3072 bytes")
	}
	encodedMeta := base64.RawURLEncoding.EncodeToString(metaBytes)
	if len(encodedMeta) > 4096 {
		return errors.New("Codex2API policy metadata header exceeds 4096 bytes")
	}
	metaCanonical := strings.Join([]string{policyMetaSignatureVersionV1, requestID, bodyDigest, encodedMeta}, "\n")
	req.Header.Set(policyHeaderMeta, encodedMeta)
	req.Header.Set(policyHeaderMetaSignature, signPolicyValue(binding.Secret, metaCanonical))

	state := codex2APIPolicyRequestState{
		RequestID: requestID, UserID: userID, ClientIP: clientIP, BodySHA256: bodyDigest, Secret: binding.Secret,
		PlatformID: binding.PlatformID, KeyFingerprint: keyFingerprint, Target: req.URL.String(),
	}
	tracker := newCodex2APIPolicyTracker(state)
	ctx := context.WithValue(req.Context(), policyContextKey, state)
	ctx = context.WithValue(ctx, policyTrackerContextKey, tracker)
	*req = *req.WithContext(ctx)
	if c != nil {
		// The latest attempt state is used by long-lived WebSocket relays, where
		// policy decisions arrive as individual JSON frames rather than HTTP
		// responses. A typed tracker keeps duplicate processing idempotent.
		c.Set(policyStateContextKey, state)
		c.Set(policyTrackerContextKey, tracker)
	}
	return nil
}

func signPolicyValue(secret, canonical string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

// SignCodex2APIRequest adds the trusted identity envelope immediately before
// dispatch.  It is called by the shared HTTP request path, so JSON, form,
// task, and custom adaptor requests all use the same implementation.
func SignCodex2APIRequest(c *gin.Context, info *relaycommon.RelayInfo, req *http.Request) error {
	// A request object can be reused by a custom adaptor or redirect callback.
	// Clear state from an earlier target before deciding whether this target is
	// bound; otherwise a stale secret could verify a later response.
	if req != nil {
		ctx := context.WithValue(req.Context(), policyContextKey, codex2APIPolicyRequestState{})
		ctx = context.WithValue(ctx, policyTrackerContextKey, (*codex2APIPolicyTracker)(nil))
		*req = *req.WithContext(ctx)
	}
	if c != nil {
		c.Set(policyStateContextKey, nil)
		c.Set(policyTrackerContextKey, (*codex2APIPolicyTracker)(nil))
		c.Set(policyViolationContextKey, false)
		c.Set("codex2api_policy_decision", nil)
	}
	// Remove any values copied from the caller before evaluating the binding.
	// This is important when a channel uses wildcard header passthrough: an
	// unmatched target must never receive a client-forged NewAPI identity.
	SanitizeCodex2APIPolicyRequestHeaders(req)
	cfg := GetCodex2APIPolicyConfig()
	if !cfg.Enabled || !cfg.IdentityForwardEnabled || req == nil || req.URL == nil {
		return nil
	}
	binding, matched, keyFingerprint := resolveCodex2APIPolicyBinding(info, req)
	if !matched {
		return nil
	}
	digest, err := bodySHA256FromRequest(req)
	if err != nil {
		return fmt.Errorf("hash outbound request body: %w", err)
	}
	return setPolicyRequestHeaders(c, info, req, binding, keyFingerprint, digest)
}

// SanitizeCodex2APIPolicyRequestHeaders removes all reserved identity and
// metadata headers from an outbound request.  Callers may use it even when
// the adapter is disabled to ensure a client cannot smuggle trusted-looking
// headers through a generic channel override.
func SanitizeCodex2APIPolicyRequestHeaders(req *http.Request) {
	if req == nil || req.Header == nil {
		return
	}
	reserved := make(map[string]struct{}, len(codex2APIPolicyRequestHeaderNames))
	for _, name := range codex2APIPolicyRequestHeaderNames {
		reserved[strings.ToLower(name)] = struct{}{}
	}
	// Header.Del canonicalizes its argument, but custom adaptors can construct a
	// map with non-canonical keys directly. Iterate the actual map so reserved
	// values cannot survive merely by changing their casing.
	for name := range req.Header {
		lowerName := strings.ToLower(strings.TrimSpace(name))
		// Treat the namespaces as reserved too.  Exact-name matching alone would
		// let a future X-NewAPI-* extension or a response-policy header survive
		// through a custom adaptor that copies headers verbatim.
		if strings.HasPrefix(lowerName, "x-newapi-") || strings.HasPrefix(lowerName, "x-codex2api-policy-") {
			delete(req.Header, name)
			continue
		}
		if _, ok := reserved[lowerName]; ok {
			delete(req.Header, name)
		}
	}
}

func policyStateFromResponse(resp *http.Response) (codex2APIPolicyRequestState, bool) {
	if resp == nil || resp.Request == nil {
		return codex2APIPolicyRequestState{}, false
	}
	state, ok := resp.Request.Context().Value(policyContextKey).(codex2APIPolicyRequestState)
	return state, ok && state.RequestID != "" && state.Secret != ""
}

// IsCodex2APIPolicyRequest reports whether req carries a request-scoped
// signing state. Signed requests must not transparently follow redirects:
// the redirected method/path/body would no longer match the original HMAC,
// and forwarding the identity envelope to an unexpected host could leak PII.
func IsCodex2APIPolicyRequest(req *http.Request) bool {
	if req == nil {
		return false
	}
	state, ok := req.Context().Value(policyContextKey).(codex2APIPolicyRequestState)
	return ok && state.RequestID != "" && state.Secret != ""
}

func policyStateFromContext(c *gin.Context) (codex2APIPolicyRequestState, bool) {
	if c == nil {
		return codex2APIPolicyRequestState{}, false
	}
	if value, exists := c.Get(policyStateContextKey); exists {
		if state, ok := value.(codex2APIPolicyRequestState); ok && state.RequestID != "" && state.Secret != "" {
			return state, true
		}
	}
	if value, exists := c.Get(policyTrackerContextKey); exists {
		if tracker, ok := value.(*codex2APIPolicyTracker); ok {
			state := tracker.snapshotState()
			return state, state.RequestID != "" && state.Secret != ""
		}
	}
	return codex2APIPolicyRequestState{}, false
}

func policyTrackerFromResponse(resp *http.Response) *codex2APIPolicyTracker {
	if resp == nil || resp.Request == nil {
		return nil
	}
	tracker, _ := resp.Request.Context().Value(policyTrackerContextKey).(*codex2APIPolicyTracker)
	return tracker
}

func policyTrackerFromContext(c *gin.Context) *codex2APIPolicyTracker {
	if c == nil {
		return nil
	}
	tracker, _ := c.Get(policyTrackerContextKey)
	result, _ := tracker.(*codex2APIPolicyTracker)
	return result
}

func decisionFromEnvelope(envelope codex2APIPolicyDecisionEnvelope) codex2APIPolicyDecision {
	return codex2APIPolicyDecision{
		RequestID:      strings.TrimSpace(envelope.RequestID),
		DecisionID:     strings.TrimSpace(envelope.DecisionID),
		EventID:        strings.TrimSpace(envelope.EventID),
		Action:         strings.ToLower(strings.TrimSpace(envelope.Action)),
		Profile:        strings.TrimSpace(envelope.Profile),
		ReasonCode:     strings.TrimSpace(envelope.ReasonCode),
		Severity:       strings.TrimSpace(envelope.Severity),
		StrikeEligible: envelope.StrikeEligible,
		RuleVersion:    strings.TrimSpace(envelope.RuleVersion),
		EvidenceSHA256: strings.ToLower(strings.TrimSpace(envelope.EvidenceSHA256)),
	}
}

func validPolicyDecision(decision codex2APIPolicyDecision, state codex2APIPolicyRequestState) bool {
	return state.RequestID != "" && state.Secret != "" &&
		decision.RequestID != "" && decision.RequestID == state.RequestID &&
		decision.DecisionID != "" && decision.Action != "" && decision.Profile != "" &&
		decision.ReasonCode != "" && decision.Severity != "" && decision.RuleVersion != "" &&
		isHexDigest(decision.EvidenceSHA256, 64)
}

func verifyPolicyDecisionSignatures(state codex2APIPolicyRequestState, decision codex2APIPolicyDecision, signatureVersion, responseSignature, eventSignatureVersion, eventSignature string) bool {
	if !validPolicyDecision(decision, state) || strings.TrimSpace(signatureVersion) != policyDecisionSignatureVersionV1 {
		return false
	}
	canonical := strings.Join([]string{
		policyDecisionSignaturePrefix,
		decision.RequestID,
		decision.DecisionID,
		decision.Action,
		decision.Profile,
		decision.ReasonCode,
		decision.Severity,
		strconv.FormatBool(decision.StrikeEligible),
		decision.RuleVersion,
		decision.EvidenceSHA256,
	}, "\n")
	expected := signPolicyValue(state.Secret, canonical)
	if expected == "" || !hmac.Equal([]byte(expected), []byte(strings.ToLower(strings.TrimSpace(responseSignature)))) {
		return false
	}
	if decision.EventID == "" {
		return true
	}
	if strings.TrimSpace(eventSignatureVersion) != policyEventSignatureVersionV1 {
		return false
	}
	eventCanonical := strings.Join([]string{
		policyEventSignaturePrefix,
		decision.RequestID,
		decision.DecisionID,
		decision.EventID,
		decision.Action,
		decision.Profile,
		decision.ReasonCode,
		decision.Severity,
		strconv.FormatBool(decision.StrikeEligible),
		decision.RuleVersion,
		decision.EvidenceSHA256,
	}, "\n")
	expectedEvent := signPolicyValue(state.Secret, eventCanonical)
	return expectedEvent != "" && hmac.Equal([]byte(expectedEvent), []byte(strings.ToLower(strings.TrimSpace(eventSignature))))
}

func parsePolicyDecisionHeaders(resp *http.Response, state codex2APIPolicyRequestState) (codex2APIPolicyDecision, bool) {
	if resp == nil || resp.Header == nil || !strings.EqualFold(strings.TrimSpace(resp.Header.Get(policyResponseViolation)), "true") {
		return codex2APIPolicyDecision{}, false
	}
	h := resp.Header
	decision := codex2APIPolicyDecision{
		RequestID:      strings.TrimSpace(h.Get(policyResponseRequestID)),
		DecisionID:     strings.TrimSpace(h.Get(policyResponseDecisionID)),
		EventID:        strings.TrimSpace(h.Get(policyResponseEventID)),
		Action:         strings.ToLower(strings.TrimSpace(h.Get(policyResponseAction))),
		Profile:        strings.TrimSpace(h.Get(policyResponseProfile)),
		ReasonCode:     strings.TrimSpace(h.Get(policyResponseReason)),
		Severity:       strings.TrimSpace(h.Get(policyResponseSeverity)),
		RuleVersion:    strings.TrimSpace(h.Get(policyResponseRuleVersion)),
		EvidenceSHA256: strings.ToLower(strings.TrimSpace(h.Get(policyResponseEvidenceSHA256))),
	}
	strikeEligible, err := strconv.ParseBool(strings.TrimSpace(h.Get(policyResponseStrikeEligible)))
	if err != nil {
		return codex2APIPolicyDecision{}, false
	}
	decision.StrikeEligible = strikeEligible
	if !verifyPolicyDecisionSignatures(state, decision, h.Get(policyResponseSignatureVersion), h.Get(policyResponseSignature), h.Get(policyResponseEventSignatureVersion), h.Get(policyResponseEventSignature)) {
		return codex2APIPolicyDecision{}, false
	}
	return decision, true
}

func parsePolicyDecisionEnvelope(payload []byte, state codex2APIPolicyRequestState) (codex2APIPolicyDecision, bool) {
	if len(payload) == 0 || state.RequestID == "" || state.Secret == "" {
		return codex2APIPolicyDecision{}, false
	}
	paths := []string{
		"response.error.details.codex2api_policy",
		"error.details.codex2api_policy",
		"response.details.codex2api_policy",
		"details.codex2api_policy",
		"codex2api_policy",
	}
	for _, path := range paths {
		value := gjson.GetBytes(payload, path)
		if !value.Exists() || !value.IsObject() {
			continue
		}
		var envelope codex2APIPolicyDecisionEnvelope
		if err := common.Unmarshal([]byte(value.Raw), &envelope); err != nil {
			continue
		}
		decision := decisionFromEnvelope(envelope)
		if verifyPolicyDecisionSignatures(state, decision, envelope.SignatureVersion, envelope.ResponseSignature, envelope.EventSignatureVersion, envelope.EventSignature) {
			return decision, true
		}
	}
	// A few compatible adaptors place the policy object at the event root
	// rather than under response.error/details. Accept that compact form too;
	// the same signature and request-ID checks still apply.
	var envelope codex2APIPolicyDecisionEnvelope
	if err := common.Unmarshal(payload, &envelope); err == nil {
		decision := decisionFromEnvelope(envelope)
		if verifyPolicyDecisionSignatures(state, decision, envelope.SignatureVersion, envelope.ResponseSignature, envelope.EventSignatureVersion, envelope.EventSignature) {
			return decision, true
		}
	}
	return codex2APIPolicyDecision{}, false
}

// processCodex2APIPolicyDecision applies one already-verified decision.  The
// signature check is deliberately performed by the caller before this method
// is reached; an invalid or unsigned payload can therefore never create a
// strike or disable an account.
func processCodex2APIPolicyDecision(c *gin.Context, state codex2APIPolicyRequestState, tracker *codex2APIPolicyTracker, decision codex2APIPolicyDecision) {
	if !validPolicyDecision(decision, state) {
		return
	}
	if tracker != nil && !tracker.markProcessed(decision.DecisionID) {
		return
	}
	cfg := GetCodex2APIPolicyConfig()
	if !cfg.Enabled || !cfg.IdentityForwardEnabled {
		return
	}
	if c != nil {
		c.Set("codex2api_policy_decision", decision)
		// A signed policy decision is authoritative for this logical request;
		// automatic channel retries would both bypass the decision and count the
		// same user action multiple times.  The relay controller checks this flag
		// before selecting another channel.
		c.Set(policyViolationContextKey, true)
	}
	if model.DB == nil || (!cfg.AuditEnabled && !cfg.StrikeEnabled && !cfg.AccountBanEnabled && !cfg.IPBlockEnabled) {
		return
	}
	userID, tokenID := 0, 0
	if c != nil {
		userID = c.GetInt("id")
		tokenID = c.GetInt("token_id")
	}
	if userID == 0 {
		userID, _ = strconv.Atoi(strings.TrimSpace(state.UserID))
	}
	clientIP := state.ClientIP
	if (net.ParseIP(clientIP) == nil || clientIP == "0.0.0.0") && c != nil {
		clientIP = strings.TrimSpace(c.ClientIP())
	}
	// 0.0.0.0 is the explicit wire-level placeholder used when a synthetic
	// context has no peer address. Never let it become a shared deny-list key.
	if clientIP == "0.0.0.0" {
		clientIP = ""
	}
	record := &model.Codex2APIPolicyStrike{
		UserId: userID, TokenId: tokenID, ClientIP: clientIP,
		RequestID: decision.RequestID, DecisionID: decision.DecisionID, EventID: decision.EventID,
		Action: decision.Action, ReasonCode: decision.ReasonCode, Severity: decision.Severity,
		StrikeEligible: decision.StrikeEligible, RuleVersion: decision.RuleVersion,
		EvidenceSHA256: decision.EvidenceSHA256, Platform: state.PlatformID, CreatedAt: time.Now().Unix(),
	}
	if !persistCodex2APIPolicyStrike(record) {
		return
	}
	// Audit, accumulation, and each punishment switch are independent. A
	// deployment may deliberately leave StrikeEnabled off while enabling only
	// account or IP enforcement; the verified decision still has to be counted
	// for that selected enforcement path.
	if !decision.StrikeEligible || !strings.EqualFold(decision.Action, "block") || (!cfg.StrikeEnabled && !cfg.AccountBanEnabled && !cfg.IPBlockEnabled) {
		return
	}
	applyCodex2APIPolicyEnforcement(cfg, record)
}

// ProcessCodex2APIResponse verifies policy response headers.  Stream bodies
// may carry the same signed decision after the HTTP headers have already been
// committed; those bodies are handled by wrapCodex2APIPolicyResponseBody.
func ProcessCodex2APIResponse(c *gin.Context, resp *http.Response) {
	cfg := GetCodex2APIPolicyConfig()
	if !cfg.Enabled || !cfg.IdentityForwardEnabled {
		return
	}
	state, ok := policyStateFromResponse(resp)
	if !ok {
		return
	}
	decision, valid := parsePolicyDecisionHeaders(resp, state)
	if !valid {
		if resp != nil && resp.Header != nil && strings.EqualFold(strings.TrimSpace(resp.Header.Get(policyResponseViolation)), "true") {
			logger.LogWarn(c, "ignored unsigned or malformed Codex2API policy response")
		}
		return
	}
	processCodex2APIPolicyDecision(c, state, policyTrackerFromResponse(resp), decision)
}

// ProcessCodex2APIPolicyEvent verifies a decision embedded in a JSON event.
// It is used by long-lived WebSocket relays, whose policy errors arrive as
// frames rather than HTTP responses. The function returns true only when a
// valid signed decision was found and applied.
func ProcessCodex2APIPolicyEvent(c *gin.Context, payload []byte) bool {
	cfg := GetCodex2APIPolicyConfig()
	if !cfg.Enabled || !cfg.IdentityForwardEnabled || len(payload) == 0 {
		return false
	}
	state, ok := policyStateFromContext(c)
	if !ok {
		return false
	}
	decision, valid := parsePolicyDecisionEnvelope(payload, state)
	if !valid {
		return false
	}
	processCodex2APIPolicyDecision(c, state, policyTrackerFromContext(c), decision)
	return true
}

// codex2APIPolicyResponseBody is a transparent response-body wrapper. It
// parses complete SSE events and bounded JSON responses while returning every
// byte unchanged to the existing relay handlers.
type codex2APIPolicyResponseBody struct {
	source      io.ReadCloser
	c           *gin.Context
	state       codex2APIPolicyRequestState
	tracker     *codex2APIPolicyTracker
	inspectSSE  bool
	inspectJSON bool
	line        []byte
	data        bytes.Buffer
	eventName   string
	jsonProbe   bytes.Buffer
	finished    bool
	finishOnce  sync.Once
}

func (r *codex2APIPolicyResponseBody) Read(p []byte) (int, error) {
	if r == nil || r.source == nil {
		return 0, io.EOF
	}
	n, err := r.source.Read(p)
	if n > 0 {
		r.feed(p[:n])
	}
	if err != nil {
		r.finish()
	}
	return n, err
}

func (r *codex2APIPolicyResponseBody) Close() error {
	if r == nil {
		return nil
	}
	r.finish()
	if r.source == nil {
		return nil
	}
	return r.source.Close()
}

func (r *codex2APIPolicyResponseBody) feed(chunk []byte) {
	if r == nil || len(chunk) == 0 || r.finished {
		return
	}
	if r.inspectJSON && r.jsonProbe.Len() < maxPolicyJSONProbe {
		probeChunk := chunk
		remaining := maxPolicyJSONProbe - r.jsonProbe.Len()
		if len(probeChunk) > remaining {
			probeChunk = probeChunk[:remaining]
		}
		_, _ = r.jsonProbe.Write(probeChunk)
		if bytes.Contains(r.jsonProbe.Bytes(), []byte("codex2api_policy")) {
			r.processPayload(r.jsonProbe.Bytes())
		}
	}
	if !r.inspectSSE {
		return
	}
	r.line = append(r.line, chunk...)
	if len(r.line) > maxPolicyEventBuffer {
		// A malformed/oversized event is ignored, but the transparent body stream
		// continues so ordinary large responses are not disrupted.
		r.line = r.line[len(r.line)-maxPolicyEventBuffer:]
	}
	for {
		index := bytes.IndexByte(r.line, '\n')
		if index < 0 {
			return
		}
		// Consume the line before compacting r.line. processSSELine copies data
		// fields into r.data and does not retain the slice, so this avoids a
		// per-line allocation while preventing compaction from corrupting it.
		line := bytes.TrimSuffix(r.line[:index], []byte{'\r'})
		r.processSSELine(line)
		r.line = append(r.line[:0], r.line[index+1:]...)
	}
}

func (r *codex2APIPolicyResponseBody) processSSELine(line []byte) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		r.flushSSEEvent()
		return
	}
	if bytes.HasPrefix(trimmed, []byte("event:")) {
		r.eventName = strings.TrimSpace(string(trimmed[len("event:"):]))
		return
	}
	if bytes.HasPrefix(trimmed, []byte("data:")) {
		value := bytes.TrimSpace(trimmed[len("data:"):])
		if len(value) == 0 || bytes.Equal(value, []byte("[DONE]")) {
			return
		}
		if r.data.Len()+len(value)+1 <= maxPolicyEventBuffer {
			if r.data.Len() > 0 {
				_ = r.data.WriteByte('\n')
			}
			_, _ = r.data.Write(value)
		}
	}
}

func (r *codex2APIPolicyResponseBody) flushSSEEvent() {
	if r == nil {
		return
	}
	if r.data.Len() > 0 {
		r.processPayload(r.data.Bytes())
	}
	r.data.Reset()
	r.eventName = ""
}

func (r *codex2APIPolicyResponseBody) processPayload(payload []byte) {
	if r == nil || len(payload) == 0 {
		return
	}
	decision, valid := parsePolicyDecisionEnvelope(payload, r.state)
	if !valid {
		return
	}
	processCodex2APIPolicyDecision(r.c, r.state, r.tracker, decision)
}

func (r *codex2APIPolicyResponseBody) finish() {
	if r == nil {
		return
	}
	r.finishOnce.Do(func() {
		r.finished = true
		if r.inspectSSE {
			if len(r.line) > 0 {
				r.processSSELine(bytes.TrimSuffix(r.line, []byte{'\r'}))
			}
			r.flushSSEEvent()
		}
		if r.inspectJSON && r.jsonProbe.Len() > 0 && bytes.Contains(r.jsonProbe.Bytes(), []byte("codex2api_policy")) {
			r.processPayload(r.jsonProbe.Bytes())
		}
	})
}

func isPolicyInspectableResponse(resp *http.Response) (inspectSSE, inspectJSON bool) {
	if resp == nil {
		return false, false
	}
	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	inspectSSE = strings.HasPrefix(contentType, "text/event-stream")
	inspectJSON = contentType == "" || strings.Contains(contentType, "application/json") || strings.Contains(contentType, "+json")
	return inspectSSE, inspectJSON
}

// WrapCodex2APIPolicyResponseBody installs the transparent stream/body parser
// when a signed request can produce an embedded policy decision. Custom
// adaptors that bypass the shared channel helper may call this exported hook.
func WrapCodex2APIPolicyResponseBody(c *gin.Context, resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	state, ok := policyStateFromResponse(resp)
	if !ok {
		return
	}
	inspectSSE, inspectJSON := isPolicyInspectableResponse(resp)
	if !inspectSSE && !inspectJSON {
		return
	}
	if _, alreadyWrapped := resp.Body.(*codex2APIPolicyResponseBody); alreadyWrapped {
		return
	}
	resp.Body = &codex2APIPolicyResponseBody{
		source: resp.Body, c: c, state: state, tracker: policyTrackerFromResponse(resp),
		inspectSSE: inspectSSE, inspectJSON: inspectJSON,
	}
}

// StripCodex2APIPolicyResponseHeaders removes internal policy metadata before
// any relay handler copies upstream headers to the public client response.
// The verified decision is retained in Gin context/database instead.
func StripCodex2APIPolicyResponseHeaders(resp *http.Response) {
	if resp == nil || resp.Header == nil {
		return
	}
	for name := range resp.Header {
		if strings.HasPrefix(strings.ToLower(name), "x-codex2api-policy-") {
			// Header.Del canonicalizes its argument. A custom RoundTripper can
			// nevertheless return a map with a non-canonical key, so delete the
			// actual entry while ranging to ensure internal decision metadata is
			// never copied to the public response.
			delete(resp.Header, name)
		}
	}
}

func persistCodex2APIPolicyStrike(record *model.Codex2APIPolicyStrike) bool {
	if record == nil || model.DB == nil || record.DecisionID == "" {
		return false
	}
	var existing model.Codex2APIPolicyStrike
	lookup := model.DB.Where("decision_id = ?", record.DecisionID).Limit(1).Find(&existing)
	if lookup.Error != nil {
		// A database error must not be interpreted as a new strike.  Returning
		// false keeps enforcement fail-closed with respect to punishment.
		common.SysError("failed to inspect Codex2API policy decision: " + lookup.Error.Error())
		return false
	}
	if lookup.RowsAffected > 0 {
		return false
	}
	if err := model.DB.Create(record).Error; err != nil {
		if isDuplicateDBError(err) {
			return false
		}
		common.SysError("failed to persist Codex2API policy decision: " + err.Error())
		return false
	}
	return true
}

func isDuplicateDBError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "unique") || strings.Contains(lower, "duplicate") || strings.Contains(lower, "constraint")
}

func applyCodex2APIPolicyEnforcement(cfg Codex2APIPolicyConfig, record *model.Codex2APIPolicyStrike) {
	if record == nil || model.DB == nil || (!cfg.AccountBanEnabled && !cfg.IPBlockEnabled) {
		return
	}
	cutoff := time.Now().Unix() - cfg.WindowSeconds
	var userCount, ipCount int64
	if record.UserId > 0 {
		_ = model.DB.Model(&model.Codex2APIPolicyStrike{}).
			Where("user_id = ? AND action = ? AND strike_eligible = ? AND created_at >= ?", record.UserId, "block", true, cutoff).
			Count(&userCount).Error
	}
	if net.ParseIP(record.ClientIP) != nil {
		_ = model.DB.Model(&model.Codex2APIPolicyStrike{}).
			Where("client_ip = ? AND action = ? AND strike_eligible = ? AND created_at >= ?", record.ClientIP, "block", true, cutoff).
			Count(&ipCount).Error
	}
	if userCount < int64(cfg.BanAfter) && ipCount < int64(cfg.BanAfter) {
		return
	}
	if cfg.AccountBanEnabled && userCount >= int64(cfg.BanAfter) {
		disableCodex2APIPolicyUser(record.UserId)
	}
	if cfg.IPBlockEnabled && ipCount >= int64(cfg.BanAfter) {
		blockCodex2APIPolicyIP(record.ClientIP, cfg.WindowSeconds, record.ReasonCode)
	}
}

func disableCodex2APIPolicyUser(userID int) {
	if userID <= 0 || model.DB == nil {
		return
	}
	var user model.User
	if err := model.DB.First(&user, userID).Error; err != nil {
		return
	}
	// Never let an upstream policy response disable an administrator account.
	if user.Role >= common.RoleAdminUser || user.Status == common.UserStatusDisabled {
		return
	}
	// Update only the status column. Calling User.Update with a stale snapshot
	// could overwrite quota/accounting fields that another request changed
	// concurrently. The conditional role/status predicates also make the
	// operation idempotent across multiple policy responses.
	result := model.DB.Model(&model.User{}).
		Where("id = ? AND role < ? AND status <> ?", userID, common.RoleAdminUser, common.UserStatusDisabled).
		Update("status", common.UserStatusDisabled)
	if result.Error != nil {
		common.SysError(fmt.Sprintf("failed to disable user %d after Codex2API policy strikes: %v", userID, result.Error))
		return
	}
	if result.RowsAffected == 0 {
		return
	}
	if err := model.DB.Model(&model.Token{}).Where("user_id = ?", userID).Update("status", common.TokenStatusDisabled).Error; err != nil {
		common.SysError(fmt.Sprintf("failed to disable tokens for user %d after Codex2API policy strikes: %v", userID, err))
	}
	if err := model.InvalidateUserCache(userID); err != nil {
		common.SysError(fmt.Sprintf("failed to invalidate user cache for %d after Codex2API policy strikes: %v", userID, err))
	}
	if err := model.InvalidateUserTokensCache(userID); err != nil {
		common.SysError(fmt.Sprintf("failed to invalidate tokens for user %d after Codex2API policy strikes: %v", userID, err))
	}
}

func blockCodex2APIPolicyIP(ip string, windowSeconds int64, reason string) {
	if model.DB == nil || net.ParseIP(ip) == nil {
		return
	}
	now := time.Now().Unix()
	expires := now + windowSeconds
	var existing model.Codex2APIPolicyIPBlock
	lookup := model.DB.Where("ip = ?", ip).Limit(1).Find(&existing)
	if lookup.Error != nil {
		return
	}
	if lookup.RowsAffected > 0 {
		updates := map[string]interface{}{"reason": strings.TrimSpace(reason), "expires_at": expires}
		_ = model.DB.Model(&existing).Updates(updates).Error
		return
	}
	_ = model.DB.Create(&model.Codex2APIPolicyIPBlock{IP: ip, Reason: strings.TrimSpace(reason), ExpiresAt: expires, CreatedAt: now}).Error
}

// IsCodex2APIPolicyIPBlocked is used by the relay ingress middleware.  It is a
// no-op unless IP enforcement is explicitly enabled.
func IsCodex2APIPolicyIPBlocked(ip string) bool {
	cfg := GetCodex2APIPolicyConfig()
	if !cfg.Enabled || !cfg.IPBlockEnabled || model.DB == nil || net.ParseIP(strings.TrimSpace(ip)) == nil {
		return false
	}
	var block model.Codex2APIPolicyIPBlock
	if err := model.DB.Where("ip = ? AND expires_at > ?", strings.TrimSpace(ip), time.Now().Unix()).First(&block).Error; err != nil {
		return false
	}
	return true
}

// Codex2APIPolicyIPBlockMiddleware rejects IPs that were independently
// blocked by a verified Codex2API event.  It is intentionally separate from
// TokenAuth so a blocked address cannot rotate tokens to evade the restriction.
func Codex2APIPolicyIPBlockMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c != nil && c.Request != nil && IsCodex2APIPolicyIPBlocked(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"message": "请求来源地址已被暂时限制",
					"type":    "new_api_error",
					"code":    "ip_policy_blocked",
				},
			})
			return
		}
		c.Next()
	}
}

// VerifyCodex2APIPolicyResponse is exported for integration tests and custom
// adaptors that receive an HTTP response outside the shared channel helper.
func VerifyCodex2APIPolicyResponse(resp *http.Response) bool {
	state, ok := policyStateFromResponse(resp)
	if !ok {
		return false
	}
	_, ok = parsePolicyDecisionHeaders(resp, state)
	return ok
}
