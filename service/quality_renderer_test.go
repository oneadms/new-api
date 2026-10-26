package service

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func qualityRendererPNGFixture(t *testing.T, width, height int) []byte {
	t.Helper()
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, width, height))))
	return encoded.Bytes()
}

func TestQualityRendererSuccessfulPNG(t *testing.T) {
	imageBytes := qualityRendererPNGFixture(t, 1200, 800)
	html := "<!doctype html><p>new-api / QuantumNous 中文 & script</p>"
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/render", r.URL.Path)
		assert.Equal(t, "Bearer renderer-test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "image/png", r.Header.Get("Accept"))
		var payload struct {
			HTML string `json:"html"`
		}
		if assert.NoError(t, common.DecodeJson(r.Body, &payload)) {
			assert.Equal(t, html, payload.HTML)
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	defer server.Close()
	t.Setenv("QUALITY_RENDERER_URL", server.URL+"/")
	t.Setenv("QUALITY_RENDERER_TOKEN", "renderer-test-token")

	assert.True(t, QualityRendererAvailable())
	result, err := RenderQualityHTML(context.Background(), []byte(html))
	require.NoError(t, err)
	assert.Equal(t, imageBytes, result)
	assert.EqualValues(t, 1, calls.Load())
}

func TestQualityRendererUnavailableConfiguration(t *testing.T) {
	for _, test := range []struct {
		name, url, token string
	}{
		{name: "neither configured"},
		{name: "missing token", url: "http://renderer:3000"},
		{name: "missing URL", token: "token"},
		{name: "whitespace token", url: "http://renderer:3000", token: " "},
		{name: "header injection token", url: "http://renderer:3000", token: "token\r\nx: y"},
		{name: "file URL", url: "file:///tmp/page", token: "token"},
		{name: "userinfo", url: "https://secret:password@renderer", token: "token"},
		{name: "query", url: "https://renderer?token=secret", token: "token"},
		{name: "fragment", url: "https://renderer/#secret", token: "token"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("QUALITY_RENDERER_URL", test.url)
			t.Setenv("QUALITY_RENDERER_TOKEN", test.token)
			assert.False(t, QualityRendererAvailable())
			result, err := RenderQualityHTML(context.Background(), []byte("<p>completed generation</p>"))
			require.ErrorIs(t, err, ErrQualityRendererUnavailable)
			assert.Nil(t, result)
			assert.NotContains(t, err.Error(), "secret")
		})
	}
}

func TestQualityRendererRejectsInvalidHTMLBeforeRequest(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	t.Setenv("QUALITY_RENDERER_URL", server.URL)
	t.Setenv("QUALITY_RENDERER_TOKEN", "test-token")
	for _, html := range [][]byte{nil, bytes.Repeat([]byte("x"), (1<<20)+1), {0xff, 0xfe}} {
		result, err := RenderQualityHTML(context.Background(), html)
		require.Error(t, err)
		assert.Nil(t, result)
	}
	assert.Zero(t, calls.Load())
}

func TestQualityRendererRejectsInvalidResponsesWithoutRetry(t *testing.T) {
	validPNG := qualityRendererPNGFixture(t, 1200, 800)
	corruptPNG := bytes.Clone(validPNG)
	corruptPNG[len(corruptPNG)-1] ^= 1
	for _, test := range []struct {
		name, contentType, encoding string
		status                      int
		body                        []byte
	}{
		{name: "HTTP error", status: 503, contentType: "text/plain", body: []byte("upstream-secret")},
		{name: "HTML response", status: 200, contentType: "text/html", body: []byte("<p>not a PNG</p>")},
		{name: "invalid bytes", status: 200, contentType: "image/png", body: []byte("not a PNG")},
		{name: "wrong dimensions", status: 200, contentType: "image/png", body: qualityRendererPNGFixture(t, 1, 1)},
		{name: "truncated PNG", status: 200, contentType: "image/png", body: validPNG[:len(validPNG)-12]},
		{name: "corrupt checksum", status: 200, contentType: "image/png", body: corruptPNG},
		{name: "trailing data", status: 200, contentType: "image/png", body: append(bytes.Clone(validPNG), 'x')},
		{name: "encoded response", status: 200, contentType: "image/png", encoding: "gzip", body: validPNG},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", test.contentType)
				w.Header().Set("Content-Encoding", test.encoding)
				w.WriteHeader(test.status)
				_, _ = w.Write(test.body)
			}))
			defer server.Close()
			t.Setenv("QUALITY_RENDERER_URL", server.URL)
			t.Setenv("QUALITY_RENDERER_TOKEN", "test-token")
			result, err := RenderQualityHTML(context.Background(), []byte("<p>quality</p>"))
			require.Error(t, err)
			assert.Nil(t, result)
			assert.NotContains(t, err.Error(), "upstream-secret")
			assert.EqualValues(t, 1, calls.Load())
		})
	}
}

func TestQualityRendererPNGSizeBoundary(t *testing.T) {
	validPNG := qualityRendererPNGFixture(t, 1200, 800)
	// A valid ancillary chunk makes the complete PNG exactly the public limit.
	padding := make([]byte, (2<<20)-len(validPNG))
	binary.BigEndian.PutUint32(padding[:4], uint32(len(padding)-12))
	copy(padding[4:8], "tEXt")
	copy(padding[8:], "Comment\x00")
	binary.BigEndian.PutUint32(padding[len(padding)-4:], crc32.ChecksumIEEE(padding[4:len(padding)-4]))
	boundedPNG := append(bytes.Clone(validPNG[:len(validPNG)-12]), padding...)
	boundedPNG = append(boundedPNG, validPNG[len(validPNG)-12:]...)
	require.Len(t, boundedPNG, 2<<20)

	for _, overLimit := range []bool{false, true} {
		t.Run(map[bool]string{false: "exactly 2 MiB accepted", true: "stream beyond 2 MiB rejected"}[overLimit], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "image/png")
				w.WriteHeader(http.StatusOK)
				w.(http.Flusher).Flush() // No Content-Length: streamed reads must also be bounded.
				_, _ = w.Write(boundedPNG)
				if overLimit {
					_, _ = w.Write([]byte{'x'})
				}
			}))
			defer server.Close()
			t.Setenv("QUALITY_RENDERER_URL", server.URL)
			t.Setenv("QUALITY_RENDERER_TOKEN", "test-token")
			result, err := RenderQualityHTML(context.Background(), []byte("<p>quality</p>"))
			if overLimit {
				require.ErrorContains(t, err, "exceeds 2 MiB")
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, boundedPNG, result)
		})
	}
}

func TestQualityRendererDoesNotFollowRedirect(t *testing.T) {
	var forwarded atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		forwarded.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	t.Setenv("QUALITY_RENDERER_URL", server.URL)
	t.Setenv("QUALITY_RENDERER_TOKEN", "test-token")
	_, err := RenderQualityHTML(context.Background(), []byte("<p>quality</p>"))
	require.ErrorContains(t, err, "HTTP 307")
	assert.Zero(t, forwarded.Load())
}

func TestQualityRendererCancellationInterruptsResponse(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	t.Setenv("QUALITY_RENDERER_URL", server.URL)
	t.Setenv("QUALITY_RENDERER_TOKEN", "test-token")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		_, err := RenderQualityHTML(ctx, []byte("<p>quality</p>"))
		finished <- err
	}()
	<-started
	cancel()
	require.ErrorIs(t, <-finished, context.Canceled)
}

func TestQualityRendererRespectsExpiredCallerDeadline(t *testing.T) {
	t.Setenv("QUALITY_RENDERER_URL", "http://127.0.0.1:1")
	t.Setenv("QUALITY_RENDERER_TOKEN", "test-token")
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()
	result, err := RenderQualityHTML(ctx, []byte(strings.Repeat("x", 1<<20)))
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Nil(t, result)
}
