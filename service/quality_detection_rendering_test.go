package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualityPNGArtifactsEnforceTwoMiBLimitOnWriteAndRead(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("QUALITY_ARTIFACT_DIR", directory)
	validPNG := qualityRendererPNGFixture(t, 1200, 800)
	// The extra ancillary chunk keeps the oversized fixture a valid PNG,
	// making rejection an artifact-size contract rather than a parse failure.
	padding := make([]byte, (2<<20)+1-len(validPNG))
	binary.BigEndian.PutUint32(padding[:4], uint32(len(padding)-12))
	copy(padding[4:8], "tEXt")
	copy(padding[8:], "Comment\x00")
	binary.BigEndian.PutUint32(padding[len(padding)-4:], crc32.ChecksumIEEE(padding[4:len(padding)-4]))
	oversized := append(bytes.Clone(validPNG[:len(validPNG)-12]), padding...)
	oversized = append(oversized, validPNG[len(validPNG)-12:]...)

	_, err := writeQualityArtifact("png", oversized)
	assert.ErrorIs(t, err, ErrQualityArtifactUnavailable)
	digest := sha256.Sum256(oversized)
	legacyKey := common.GetUUID() + "-" + hex.EncodeToString(digest[:]) + ".png"
	require.NoError(t, os.WriteFile(filepath.Join(directory, legacyKey), oversized, 0600))
	_, err = ReadQualityArtifact(&model.QualityProbeRun{HasThumbnail: true, ThumbnailKey: legacyKey}, "thumbnail")
	assert.ErrorIs(t, err, ErrQualityArtifactUnavailable)

	key, err := writeQualityArtifact("png", validPNG)
	require.NoError(t, err)
	data, err := ReadQualityArtifact(&model.QualityProbeRun{HasThumbnail: true, ThumbnailKey: key}, "thumbnail")
	require.NoError(t, err)
	assert.Equal(t, validPNG, data)
}

func TestQualityRenderedRunPersistsBundledRendererIdentifier(t *testing.T) {
	qualityServiceFixture(t, func(_ context.Context, _ QualityExecutionRequest, emit func(string) error) (QualityExecutionResult, error) {
		return QualityExecutionResult{Started: true, Complete: true}, emit("<html><body>rendered</body></html>")
	})
	pngBytes := qualityRendererPNGFixture(t, 1200, 800)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()
	t.Setenv("QUALITY_RENDERER_URL", server.URL)
	t.Setenv("QUALITY_RENDERER_TOKEN", "test-renderer-token")
	qualityServiceChannel(t, 1, "standard")
	batch, err := CreateQualityBatch(QualityBatchRequest{}, "renderer-identifier", 1)
	require.NoError(t, err)
	QualityDetectionHandler{}.Run(context.Background(), qualityServiceClaim(t, batch), "quality-test-owner")
	_, runs, err := model.GetQualityBatch(batch.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, "completed", runs[0].RenderStatus)
	assert.Equal(t, "new-api-quality-v1-pw1.64.1", runs[0].RendererVersion)
	data, err := ReadQualityArtifact(&runs[0], "thumbnail")
	require.NoError(t, err)
	assert.Equal(t, pngBytes, data)
}
