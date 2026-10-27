package channel

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	common2 "github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/pkg/ssetrace"
	"github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ApplyUpstreamBodyMetadata restores metadata that net/http cannot infer from
// a ReplayableBody. Callers must pass the original body because NewRequest
// hides its dynamic type behind req.Body's io.ReadCloser wrapper.
func ApplyUpstreamBodyMetadata(req *http.Request, body io.Reader) {
	replayable, ok := body.(common2.ReplayableBody)
	if !ok {
		return
	}

	// BodyStorage structurally satisfies ReplayableBody, but it also exposes
	// io.Closer. If a caller passes the storage directly instead of using
	// NewReplayableBodyReader, hide Close before the transport takes ownership
	// of req.Body so the shared replay source remains available to GetBody.
	if _, rawStorage := body.(common2.BodyStorage); rawStorage {
		req.Body = io.NopCloser(body)
	}

	req.ContentLength = replayable.Size()
	if req.GetBody == nil {
		req.GetBody = replayable.NewReader
	}
}

// bodySHA256Provider is implemented by common.ReaderOnly when its wrapped
// storage is seekable.  http.NewRequest intentionally type-erases an
// io.Reader into io.NopCloser, which would otherwise hide that method and make
// the policy signer buffer the complete body in heap memory just to hash it.
type bodySHA256Provider interface {
	BodySHA256() (string, error)
}

// policyBodyReadCloser keeps the digest provider visible after
// http.NewRequest's io.NopCloser wrapping while preserving the request body's
// existing Close behavior.  The embedded closer is normally the transport's
// no-op closer around ReaderOnly; the request-scoped BodyStorage cleanup still
// owns the actual storage lifetime.
type policyBodyReadCloser struct {
	io.ReadCloser
	provider bodySHA256Provider
}

func (r *policyBodyReadCloser) BodySHA256() (string, error) {
	if r == nil || r.provider == nil {
		return "", fmt.Errorf("body digest provider is nil")
	}
	return r.provider.BodySHA256()
}

func attachBodySHA256Provider(req *http.Request, source io.Reader) {
	if req == nil || req.Body == nil || source == nil {
		return
	}
	provider, ok := source.(bodySHA256Provider)
	if !ok || provider == nil {
		return
	}
	if _, alreadyAttached := req.Body.(bodySHA256Provider); alreadyAttached {
		return
	}
	req.Body = &policyBodyReadCloser{ReadCloser: req.Body, provider: provider}
}

func SetupApiRequestHeader(info *common.RelayInfo, c *gin.Context, req *http.Header) {
	if info.RelayMode == constant.RelayModeAudioTranscription || info.RelayMode == constant.RelayModeAudioTranslation {
		// multipart/form-data
	} else if info.RelayMode == constant.RelayModeRealtime {
		// websocket
	} else {
		req.Set("Content-Type", c.Request.Header.Get("Content-Type"))
		req.Set("Accept", c.Request.Header.Get("Accept"))
		if info.IsStream && c.Request.Header.Get("Accept") == "" {
			req.Set("Accept", "text/event-stream")
		}
	}
}

const clientHeaderPlaceholderPrefix = "{client_header:"

const (
	headerPassthroughAllKey        = "*"
	headerPassthroughRegexPrefix   = "re:"
	headerPassthroughRegexPrefixV2 = "regex:"
)

var passthroughSkipHeaderNamesLower = map[string]struct{}{
	// RFC 7230 hop-by-hop headers.
	"connection":          {},
	"keep-alive":          {},
	"proxy-authenticate":  {},
	"proxy-authorization": {},
	"te":                  {},
	"trailer":             {},
	"transfer-encoding":   {},
	"upgrade":             {},

	"cookie": {},

	// Additional headers that should not be forwarded by name-matching passthrough rules.
	"host":            {},
	"content-length":  {},
	"accept-encoding": {},

	// Do not passthrough credentials by wildcard/regex.
	"authorization":  {},
	"x-api-key":      {},
	"x-goog-api-key": {},
	// Alternate provider credential headers must not be copied by wildcard
	// passthrough; channel setup remains the sole source of upstream auth.
	"anthropic-auth-token": {},
	"api-key":              {},
	"mj-api-secret":        {},

	// NewAPI/Codex2API identity headers are generated from authenticated
	// gateway context.  Never copy caller-supplied values through passthrough.
	"x-newapi-user-id":               {},
	"x-newapi-client-ip":             {},
	"x-newapi-request-id":            {},
	"x-newapi-timestamp":             {},
	"x-newapi-method":                {},
	"x-newapi-path":                  {},
	"x-newapi-body-sha256":           {},
	"x-newapi-signature-version":     {},
	"x-newapi-signature":             {},
	"x-newapi-policy-meta":           {},
	"x-newapi-policy-meta-signature": {},

	// WebSocket handshake headers are generated by the client/dialer.
	"sec-websocket-key":        {},
	"sec-websocket-version":    {},
	"sec-websocket-extensions": {},
}

var headerPassthroughRegexCache sync.Map // map[string]*regexp.Regexp

func getHeaderPassthroughRegex(pattern string) (*regexp.Regexp, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, errors.New("empty regex pattern")
	}
	if v, ok := headerPassthroughRegexCache.Load(pattern); ok {
		if re, ok := v.(*regexp.Regexp); ok {
			return re, nil
		}
		headerPassthroughRegexCache.Delete(pattern)
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	actual, _ := headerPassthroughRegexCache.LoadOrStore(pattern, compiled)
	if re, ok := actual.(*regexp.Regexp); ok {
		return re, nil
	}
	return compiled, nil
}

func IsHeaderPassthroughRuleKey(key string) bool {
	return isHeaderPassthroughRuleKey(key)
}
func isHeaderPassthroughRuleKey(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	if key == headerPassthroughAllKey {
		return true
	}
	lower := strings.ToLower(key)
	return strings.HasPrefix(lower, headerPassthroughRegexPrefix) || strings.HasPrefix(lower, headerPassthroughRegexPrefixV2)
}

func shouldSkipPassthroughHeader(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return true
	}
	lower := strings.ToLower(name)
	// Keep the complete reserved namespaces private, including future protocol
	// extensions that are not yet listed in the exact-name map below.
	if strings.HasPrefix(lower, "x-newapi-") || strings.HasPrefix(lower, "x-codex2api-policy-") {
		return true
	}
	if _, ok := passthroughSkipHeaderNamesLower[lower]; ok {
		return true
	}
	return false
}

func applyHeaderOverridePlaceholders(template string, c *gin.Context, apiKey string) (string, bool, error) {
	trimmed := strings.TrimSpace(template)
	if strings.HasPrefix(trimmed, clientHeaderPlaceholderPrefix) {
		afterPrefix := trimmed[len(clientHeaderPlaceholderPrefix):]
		end := strings.Index(afterPrefix, "}")
		if end < 0 || end != len(afterPrefix)-1 {
			return "", false, fmt.Errorf("client_header placeholder must be the full value: %q", template)
		}

		name := strings.TrimSpace(afterPrefix[:end])
		if name == "" {
			return "", false, fmt.Errorf("client_header placeholder name is empty: %q", template)
		}
		if c == nil || c.Request == nil {
			return "", false, fmt.Errorf("missing request context for client_header placeholder")
		}
		clientHeaderValue := c.Request.Header.Get(name)
		if strings.TrimSpace(clientHeaderValue) == "" {
			return "", false, nil
		}
		// Do not interpolate {api_key} inside client-supplied content.
		return clientHeaderValue, true, nil
	}

	if strings.Contains(template, "{api_key}") {
		template = strings.ReplaceAll(template, "{api_key}", apiKey)
	}
	if strings.TrimSpace(template) == "" {
		return "", false, nil
	}
	return template, true, nil
}

// processHeaderOverride applies channel header overrides, with placeholder substitution.
// Supported placeholders:
//   - {api_key}: resolved to the channel API key
//   - {client_header:<name>}: resolved to the incoming request header value
//
// Header passthrough rules (keys only; values are ignored):
//   - "*": passthrough all incoming headers by name (excluding unsafe headers)
//   - "re:<regex>" / "regex:<regex>": passthrough headers whose names match the regex (Go regexp)
//
// Passthrough rules are applied first, then normal overrides are applied, so explicit overrides win.
func processHeaderOverride(info *common.RelayInfo, c *gin.Context) (map[string]string, error) {
	headerOverride := make(map[string]string)
	if info == nil {
		return headerOverride, nil
	}

	headerOverrideSource := common.GetEffectiveHeaderOverride(info)
	// Background polling has no client request from which to copy headers.
	hasClientRequest := !info.IsChannelTest && c != nil && c.Request != nil

	passAll := false
	var passthroughRegex []*regexp.Regexp
	if hasClientRequest {
		for k := range headerOverrideSource {
			key := strings.TrimSpace(strings.ToLower(k))
			if key == "" {
				continue
			}
			if key == headerPassthroughAllKey {
				passAll = true
				continue
			}

			var pattern string
			switch {
			case strings.HasPrefix(key, headerPassthroughRegexPrefix):
				pattern = strings.TrimSpace(key[len(headerPassthroughRegexPrefix):])
			case strings.HasPrefix(key, headerPassthroughRegexPrefixV2):
				pattern = strings.TrimSpace(key[len(headerPassthroughRegexPrefixV2):])
			default:
				continue
			}

			if pattern == "" {
				return nil, types.NewError(fmt.Errorf("header passthrough regex pattern is empty: %q", k), types.ErrorCodeChannelHeaderOverrideInvalid)
			}
			compiled, err := getHeaderPassthroughRegex(pattern)
			if err != nil {
				return nil, types.NewError(err, types.ErrorCodeChannelHeaderOverrideInvalid)
			}
			passthroughRegex = append(passthroughRegex, compiled)
		}
	}

	if passAll || len(passthroughRegex) > 0 {
		for name := range c.Request.Header {
			if shouldSkipPassthroughHeader(name) {
				continue
			}
			if !passAll {
				matched := false
				for _, re := range passthroughRegex {
					if re.MatchString(name) {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
			value := strings.TrimSpace(c.Request.Header.Get(name))
			if value == "" {
				continue
			}
			headerOverride[strings.ToLower(strings.TrimSpace(name))] = value
		}
	}

	for k, v := range headerOverrideSource {
		if isHeaderPassthroughRuleKey(k) {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(k))
		if key == "" {
			continue
		}

		str, ok := v.(string)
		if !ok {
			return nil, types.NewError(nil, types.ErrorCodeChannelHeaderOverrideInvalid)
		}
		if !hasClientRequest && strings.HasPrefix(strings.TrimSpace(str), clientHeaderPlaceholderPrefix) {
			continue
		}

		value, include, err := applyHeaderOverridePlaceholders(str, c, info.ApiKey)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeChannelHeaderOverrideInvalid)
		}
		if !include {
			continue
		}

		headerOverride[key] = value
	}
	return headerOverride, nil
}

func ResolveHeaderOverride(info *common.RelayInfo, c *gin.Context) (map[string]string, error) {
	return processHeaderOverride(info, c)
}

// ApplyHeaderOverride applies channel overrides after the adaptor's headers.
// Without a client request, only static values and {api_key} are resolved.
func ApplyHeaderOverride(info *common.RelayInfo, c *gin.Context, req *http.Request) error {
	headers, err := processHeaderOverride(info, c)
	if err != nil {
		return err
	}
	applyHeaderOverrideToRequest(req, headers)
	return nil
}

func applyHeaderOverrideToRequest(req *http.Request, headerOverride map[string]string) {
	if req == nil {
		return
	}
	for key, value := range headerOverride {
		req.Header.Set(key, value)
		// set Host in req
		if strings.EqualFold(key, "Host") {
			req.Host = value
		}
	}
}

func DoApiRequest(a Adaptor, c *gin.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	fullRequestURL, err := a.GetRequestURL(info)
	if err != nil {
		return nil, fmt.Errorf("get request url failed: %w", err)
	}
	logger.LogDebug(c, "fullRequestURL: %s", common.SanitizeURLForLog(fullRequestURL))
	req, err := http.NewRequest(c.Request.Method, fullRequestURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("new request failed: %w", err)
	}
	attachBodySHA256Provider(req, requestBody)
	ApplyUpstreamBodyMetadata(req, requestBody)
	headers := req.Header
	err = a.SetupRequestHeader(c, &headers, info)
	if err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}
	// 在 SetupRequestHeader 之后应用 Header Override，确保用户设置优先级最高
	// 这样可以覆盖默认的 Authorization header 设置
	if err = ApplyHeaderOverride(info, c, req); err != nil {
		return nil, err
	}
	resp, err := doRequest(c, req, info)
	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}
	return resp, nil
}

func DoFormRequest(a Adaptor, c *gin.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	fullRequestURL, err := a.GetRequestURL(info)
	if err != nil {
		return nil, fmt.Errorf("get request url failed: %w", err)
	}
	logger.LogDebug(c, "fullRequestURL: %s", common.SanitizeURLForLog(fullRequestURL))
	req, err := http.NewRequest(c.Request.Method, fullRequestURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("new request failed: %w", err)
	}
	attachBodySHA256Provider(req, requestBody)
	ApplyUpstreamBodyMetadata(req, requestBody)
	// set form data
	req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	headers := req.Header
	err = a.SetupRequestHeader(c, &headers, info)
	if err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}
	// 在 SetupRequestHeader 之后应用 Header Override，确保用户设置优先级最高
	// 这样可以覆盖默认的 Authorization header 设置
	if err = ApplyHeaderOverride(info, c, req); err != nil {
		return nil, err
	}
	resp, err := doRequest(c, req, info)
	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}
	return resp, nil
}

// DoWssRequest dials the adaptor's upstream over WebSocket for the realtime
// and Responses WebSocket relays. It honors the channel proxy, is bound to the
// request context, and reports a rejected handshake as a *types.NewAPIError
// carrying the upstream status code (types.NewError preserves it).
func DoWssRequest(a Adaptor, c *gin.Context, info *common.RelayInfo, requestBody io.Reader) (*websocket.Conn, error) {
	fullRequestURL, err := a.GetRequestURL(info)
	if err != nil {
		return nil, fmt.Errorf("get request url failed: %w", err)
	}
	fullRequestURL = toWebSocketURL(fullRequestURL)
	if targetURL, parseErr := url.Parse(fullRequestURL); parseErr == nil {
		common2.SetContextKey(c, appconstant.ContextKeyUpstreamRequestOrigin, targetURL.Scheme+"://"+targetURL.Host)
	}
	targetHeader := http.Header{}
	err = a.SetupRequestHeader(c, &targetHeader, info)
	if err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}
	// 在 SetupRequestHeader 之后应用 Header Override，确保用户设置优先级最高
	// 这样可以覆盖默认的 Authorization header 设置
	headerOverride, err := processHeaderOverride(info, c)
	if err != nil {
		return nil, err
	}
	for key, value := range headerOverride {
		targetHeader.Set(key, value)
	}
	targetHeader.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	dialer := *websocket.DefaultDialer
	if info.ChannelSetting.Proxy != "" {
		proxyURL, _, proxyErr := common2.ParseProxyURLRuntime(info.ChannelSetting.Proxy)
		if proxyErr != nil {
			return nil, proxyErr
		}
		dialer.Proxy = http.ProxyURL(proxyURL)
	}
	if info.ChannelSetting.TLSInsecureSkipVerify {
		dialer.TLSClientConfig = common2.InsecureTLSConfig
	}
	// WebSocket handshakes have no request body, but use the same signed
	// identity envelope as HTTP requests.  Keep the synthetic request around so
	// a policy response returned during the handshake can be verified.
	// Gorilla always performs a GET during the WebSocket upgrade, regardless of
	// the inbound relay method.  Sign the actual handshake method so Codex2API's
	// verifier can compare the header with the request it receives.
	signRequest, err := http.NewRequest(http.MethodGet, fullRequestURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("new websocket signing request failed: %w", err)
	}
	signRequest.Header = targetHeader.Clone()
	if err := service.SignCodex2APIRequest(c, info, signRequest); err != nil {
		return nil, types.NewError(err, types.ErrorCodeDoRequestFailed, types.ErrOptionWithHideErrMsg("upstream policy signing failed"))
	}
	targetHeader = signRequest.Header
	targetConn, handshakeResp, err := dialer.DialContext(c.Request.Context(), fullRequestURL, targetHeader)
	if handshakeResp != nil {
		// Gorilla may leave Request nil on failed handshakes.  The synthetic
		// request carries the exact signing context in either case.
		handshakeResp.Request = signRequest
		service.ProcessCodex2APIResponse(c, handshakeResp)
		service.StripCodex2APIPolicyResponseHeaders(handshakeResp)
		if err != nil && handshakeResp.Body != nil {
			_ = handshakeResp.Body.Close()
		}
	}
	if err != nil {
		statusCode := http.StatusInternalServerError
		if handshakeResp != nil {
			statusCode = handshakeResp.StatusCode
		}
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("dial failed to %s: %w", common.SanitizeURLForLog(fullRequestURL), err), types.ErrorCodeDoRequestFailed, statusCode)
	}
	return targetConn, nil
}

// toWebSocketURL maps an http(s) endpoint to ws(s). Realtime adaptors already
// return ws(s) URLs, which pass through unchanged.
func toWebSocketURL(raw string) string {
	switch {
	case strings.HasPrefix(raw, "https://"):
		return "wss://" + strings.TrimPrefix(raw, "https://")
	case strings.HasPrefix(raw, "http://"):
		return "ws://" + strings.TrimPrefix(raw, "http://")
	default:
		return raw
	}
}

func DoRequest(c *gin.Context, req *http.Request, info *common.RelayInfo) (*http.Response, error) {
	return doRequest(c, req, info)
}

// keepUpstreamRedirectResponse stops net/http from following redirects while
// returning the upstream 3xx response to the relay without an extra error.
func keepUpstreamRedirectResponse(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

func doRequest(c *gin.Context, req *http.Request, info *common.RelayInfo) (*http.Response, error) {
	if info.IsQualityDetection {
		// Detection is owned by a leased background task. Cancellation must stop
		// the actual upstream request, and its body must never be replayed.
		req = req.WithContext(c.Request.Context())
		req.GetBody = nil
	}
	client, err := service.GetHttpClientWithProxySettings(info.ChannelSetting.Proxy, info.ChannelSetting)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	// Clients are cached and shared across channels, so override redirect
	// behavior on a shallow copy instead of mutating the cached client. This
	// still reuses its transport and connection pools, including HTTP/2's
	// transparent stream retries.
	relayClient := *client
	relayClient.CheckRedirect = keepUpstreamRedirectResponse
	if common2.DebugEnabled && req != nil && req.URL != nil {
		policy := service.NormalizeHTTPTransportPolicy(info.ChannelSetting)
		logger.LogDebug(c, "http transport select: host=%s protocol=%s shards=%d policy=%s",
			req.URL.Host,
			policy.Protocol,
			policy.Shards,
			policy.String())
	}

	if info.IsStream {
		helper.SetEventStreamHeaders(c)
		// 处理流式请求的 ping 保活
		generalSettings := operation_setting.GetGeneralSetting()
		if generalSettings.PingIntervalEnabled && !info.DisablePing {
			pingInterval := time.Duration(generalSettings.PingIntervalSeconds) * time.Second
			stopPinger := helper.StartStreamKeepAlive(c, pingInterval)
			defer stopPinger()
		}
	}

	// Refresh the generic handler's checkpoint at the actual HTTP dispatch.
	// Custom/SDK adaptors still use the outer checkpoint as a fallback.
	info.SetUpstreamStartTime()
	// Inject the trusted NewAPI identity envelope only after all adaptor and
	// channel header overrides have been applied.  The signer overwrites any
	// client-supplied values for its reserved header names and binds the
	// signature to the exact final request body.
	if err := service.SignCodex2APIRequest(c, info, req); err != nil {
		return nil, types.NewError(err, types.ErrorCodeDoRequestFailed, types.ErrOptionWithHideErrMsg("upstream policy signing failed"))
	}
	if service.IsCodex2APIPolicyRequest(req) {
		// A redirect would change the signed method/path/body and may forward
		// trusted identity headers to a host outside the binding. Use a per-call
		// client copy so ordinary provider requests retain their normal redirect
		// behavior.
		clientCopy := relayClient
		clientCopy.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return errors.New("redirects are disabled for signed Codex2API requests")
		}
		relayClient = clientCopy
	}
	common2.SetContextKey(c, appconstant.ContextKeyUpstreamRequestOrigin, req.URL.Scheme+"://"+req.URL.Host)
	resp, err := relayClient.Do(req)
	if err != nil {
		logger.LogError(c, common2.LogText("do request failed: %s", err.Error()))
		return nil, types.NewError(err, types.ErrorCodeDoRequestFailed, types.ErrOptionWithHideErrMsg("upstream error: do request failed"))
	}
	info.SetUpstreamHeadersTime()
	if resp == nil {
		return nil, errors.New("resp is nil")
	}
	if resp.Request == nil || (service.IsCodex2APIPolicyRequest(req) && !service.IsCodex2APIPolicyRequest(resp.Request)) {
		// A custom RoundTripper may omit Response.Request or return a cloned
		// request without the context value. Preserve the signed request context
		// so policy response verification and stream-event parsing still have the
		// binding state they need.
		resp.Request = req
	}
	// Policy headers are consumed before any response handler reads or streams
	// the body.  Invalid signatures are ignored by the service and never add a
	// strike.
	service.ProcessCodex2APIResponse(c, resp)
	service.WrapCodex2APIPolicyResponseBody(c, resp)
	service.StripCodex2APIPolicyResponseHeaders(resp)
	isEventStream := strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream")
	if (info.IsStream || isEventStream) && resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		resp.Body = ssetrace.WrapReadCloser(resp.Body, info.RequestId)
	}
	if common2.DebugEnabled {
		policy := service.NormalizeHTTPTransportPolicy(info.ChannelSetting)
		logger.LogDebug(c, "http transport negotiated: host=%s protocol=%s shards=%d policy=%s negotiated=%s",
			req.URL.Host,
			policy.Protocol,
			policy.Shards,
			policy.String(),
			resp.Proto)
	}

	if upID := resp.Header.Get(common2.RequestIdKey); upID != "" {
		c.Set(common2.UpstreamRequestIdKey, upID)
	}

	if req.Body != nil {
		_ = req.Body.Close()
	}
	if c != nil && c.Request != nil && c.Request.Body != nil {
		_ = c.Request.Body.Close()
	}
	return resp, nil
}

func DoTaskApiRequest(a TaskAdaptor, c *gin.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	fullRequestURL, err := a.BuildRequestURL(info)
	if err != nil {
		return nil, err
	}
	req, err := newTaskAPIRequest(c, fullRequestURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("new request failed: %w", err)
	}
	attachBodySHA256Provider(req, requestBody)
	ApplyUpstreamBodyMetadata(req, requestBody)
	// Do NOT wrap requestBody in a GetBody closure here: returning the same
	// (already consumed) reader would make any transport-level retry silently
	// replay an empty body. http.NewRequest already derives a correct,
	// snapshot-based GetBody for *bytes.Reader/Buffer/strings.Reader bodies
	// (which most task adaptors pass in); ApplyUpstreamBodyMetadata wires the
	// same contract for bodies that explicitly implement ReplayableBody.
	// Otherwise GetBody stays nil so the transport fails the retry instead of
	// sending a corrupted request.

	err = a.BuildRequestHeader(c, req, info)
	if err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}
	if err = ApplyHeaderOverride(info, c, req); err != nil {
		return nil, err
	}
	resp, err := doRequest(c, req, info)
	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}
	return resp, nil
}

func newTaskAPIRequest(c *gin.Context, fullRequestURL string, requestBody io.Reader) (*http.Request, error) {
	if c == nil || c.Request == nil {
		return nil, errors.New("task client request is missing")
	}
	return http.NewRequestWithContext(c.Request.Context(), c.Request.Method, fullRequestURL, requestBody)
}
