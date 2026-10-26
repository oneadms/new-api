package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"image/png"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
)

const (
	qualityRendererHTMLLimit = 1 << 20
	qualityRendererPNGLimit  = 2 << 20
	qualityRendererTimeout   = 20 * time.Second
	// Identifies the bundled renderer build; the HTTP response does not attest
	// the version running at the configured renderer endpoint.
	qualityRendererVersion = "new-api-quality-v1-pw1.64.1"
)

// ErrQualityRendererUnavailable means the optional renderer has no usable
// deployment configuration. It must not turn a completed model generation into
// a failed generation; callers should record a separate unavailable render state.
var ErrQualityRendererUnavailable = errors.New("quality renderer unavailable")

// This client is deliberately independent of relay clients, proxies, wallet
// state, and global TLS bypass options. The destination is operator-controlled
// deployment configuration, including private container addresses, never input
// from a user/model. Redirects and environment proxies must not forward its token.
var qualityRendererHTTPClient = &http.Client{
	Timeout: qualityRendererTimeout,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:    3 * time.Second,
		ResponseHeaderTimeout:  qualityRendererTimeout,
		MaxResponseHeaderBytes: 16 << 10,
		DisableCompression:     true,
		// A fresh connection per job also avoids transparent transport retries on
		// stale pooled connections. No application-level retry is performed.
		DisableKeepAlives: true,
	},
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func qualityRendererConfiguration() (string, string, error) {
	rawURL := strings.TrimSpace(os.Getenv("QUALITY_RENDERER_URL"))
	token := os.Getenv("QUALITY_RENDERER_TOKEN")
	if rawURL == "" || token == "" {
		return "", "", ErrQualityRendererUnavailable
	}
	if len(token) > 4096 || strings.IndexFunc(token, func(r rune) bool { return r < 33 || r > 126 }) >= 0 {
		return "", "", fmt.Errorf("%w: invalid token configuration", ErrQualityRendererUnavailable)
	}
	u, err := url.Parse(rawURL)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") ||
		u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return "", "", fmt.Errorf("%w: invalid URL configuration", ErrQualityRendererUnavailable)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/render"
	u.RawPath = ""
	return u.String(), token, nil
}

// QualityRendererAvailable reports usable configuration, not network health.
// It neither contacts the renderer nor exposes its URL or authentication token.
func QualityRendererAvailable() bool {
	_, _, err := qualityRendererConfiguration()
	return err == nil
}

// RenderQualityHTML renders one bounded HTML document through the optional,
// isolated renderer and returns only a validated 1200x800 PNG. There is no retry,
// URL rendering, local-browser fallback, ordinary relay call, or wallet mutation.
func RenderQualityHTML(ctx context.Context, html []byte) ([]byte, error) {
	endpoint, token, err := qualityRendererConfiguration()
	if err != nil {
		return nil, err
	}
	if len(html) == 0 || len(html) > qualityRendererHTMLLimit || !utf8.Valid(html) {
		return nil, errors.New("quality renderer requires nonempty UTF-8 HTML of at most 1 MiB")
	}
	ctx, cancel := context.WithTimeout(ctx, qualityRendererTimeout)
	defer cancel()

	body, err := common.Marshal(struct {
		HTML string `json:"html"`
	}{HTML: string(html)})
	if err != nil {
		return nil, errors.New("quality renderer request encoding failed")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("quality renderer request configuration is invalid")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "image/png")
	resp, err := qualityRendererHTTPClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("quality renderer request interrupted: %w", ctx.Err())
		}
		// Do not expose the deployment URL, credentials, or a remote error body.
		return nil, errors.New("quality renderer request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("quality renderer returned HTTP %d", resp.StatusCode)
	}
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || mediaType != "image/png" {
		return nil, errors.New("quality renderer returned a non-PNG content type")
	}
	if encoding := resp.Header.Get("Content-Encoding"); encoding != "" && encoding != "identity" {
		return nil, errors.New("quality renderer returned an unsupported content encoding")
	}
	if resp.ContentLength > qualityRendererPNGLimit {
		return nil, errors.New("quality renderer PNG exceeds 2 MiB")
	}
	imageBytes, err := io.ReadAll(io.LimitReader(resp.Body, qualityRendererPNGLimit+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("quality renderer response interrupted: %w", ctx.Err())
		}
		return nil, errors.New("quality renderer PNG read failed")
	}
	if len(imageBytes) > qualityRendererPNGLimit {
		return nil, errors.New("quality renderer PNG exceeds 2 MiB")
	}
	config, err := png.DecodeConfig(bytes.NewReader(imageBytes))
	if err != nil || config.Width != 1200 || config.Height != 800 {
		return nil, errors.New("quality renderer returned an invalid PNG or viewport")
	}
	// DecodeConfig alone accepts truncated images. Check the complete stream,
	// including chunk checksums and end marker, after bounding allocation size.
	reader := bytes.NewReader(imageBytes)
	if _, err = png.Decode(reader); err != nil || reader.Len() != 0 {
		return nil, errors.New("quality renderer returned a corrupt PNG")
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("quality renderer interrupted: %w", ctx.Err())
	}
	return imageBytes, nil
}
