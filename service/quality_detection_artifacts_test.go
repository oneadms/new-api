package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualityHTMLExtractionPreservesWrittenCodeWithoutRepair(t *testing.T) {
	cases := []struct{ name, raw, html string }{
		{"complete document", "<!doctype html><html><body>original</body></html>", "<!doctype html><html><body>original</body></html>"},
		{"markdown fence", "```html\n<html><body>original</body></html>\n```", "<html><body>original</body></html>"},
		{"fragment stays fragment", "<svg><circle /></svg>", ""},
		{"missing closing tag remains invalid", "<html><body>unfinished", ""},
		{"similar tag is not a document", "<html-example>data</html>", ""},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) { assert.Equal(t, tt.html, string(ExtractQualityHTML([]byte(tt.raw)))) })
	}
}

func TestQualityArtifactsStayBoundedConfinedAndContentAddressed(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("QUALITY_ARTIFACT_DIR", directory)
	raw := []byte("<html><body>untrusted source</body></html>")
	key, err := writeQualityArtifact("raw", raw)
	require.NoError(t, err)
	run := &model.QualityProbeRun{HasOutput: true, RawKey: key}
	got, err := ReadQualityArtifact(run, "raw")
	require.NoError(t, err)
	assert.Equal(t, raw, got)
	_, err = ReadQualityArtifact(run, "html")
	require.ErrorIs(t, err, ErrQualityArtifactUnavailable)
	_, err = writeQualityArtifact("raw", []byte(strings.Repeat("x", QualityOutputLimit+1)))
	require.ErrorIs(t, err, ErrQualityArtifactUnavailable)
	for _, unsafeKey := range []string{"../secret.txt", filepath.Join(t.TempDir(), "secret.txt"), "nested/" + key} {
		run.RawKey = unsafeKey
		_, err = ReadQualityArtifact(run, "raw")
		require.ErrorIs(t, err, ErrQualityArtifactUnavailable)
	}
	// Changed storage bytes cannot be substituted for the immutable digest.
	require.NoError(t, os.WriteFile(filepath.Join(directory, key), []byte("substituted bytes"), 0600))
	run.RawKey = key
	_, err = ReadQualityArtifact(run, "raw")
	require.ErrorIs(t, err, ErrQualityArtifactUnavailable)
}

func TestQualityArtifactReferenceDoesNotExposeDirectoryOrSourceInMetadata(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("QUALITY_ARTIFACT_DIR", directory)
	key, err := writeQualityArtifact("html", []byte("<html>private output</html>"))
	require.NoError(t, err)
	assert.NotContains(t, key, directory)
	assert.NotContains(t, key, "private output")
	// The administrative reader returns bytes, never an executable HTTP URL.
	data, err := ReadQualityArtifact(&model.QualityProbeRun{HasHTML: true, HTMLKey: key}, "html")
	require.NoError(t, err)
	assert.Equal(t, "<html>private output</html>", string(data))
}
