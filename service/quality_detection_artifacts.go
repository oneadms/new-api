package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

var qualityArtifactKeyPattern = regexp.MustCompile(`^[a-f0-9]{32}-([a-f0-9]{64})\.(raw|html|png)$`)
var ErrQualityArtifactUnavailable = errors.New("quality artifact is unavailable")

func qualityArtifactRoot(create bool) (*os.Root, error) {
	directory := os.Getenv("QUALITY_ARTIFACT_DIR")
	if directory == "" {
		directory = filepath.Join("data", "quality")
	}
	if create {
		if err := os.MkdirAll(directory, 0750); err != nil {
			return nil, ErrQualityArtifactUnavailable
		}
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, ErrQualityArtifactUnavailable
	}
	return root, nil
}

// writeQualityArtifact writes an unguessable immutable object before any DB
// reference is published. OpenRoot confines symlinks as well as relative paths.
func writeQualityArtifact(kind string, data []byte) (string, error) {
	limit := QualityOutputLimit
	if kind == "png" {
		limit = qualityRendererPNGLimit
	}
	if (kind != "raw" && kind != "html" && kind != "png") || len(data) == 0 || len(data) > limit {
		return "", ErrQualityArtifactUnavailable
	}
	digest := sha256.Sum256(data)
	key := common.GetUUID() + "-" + hex.EncodeToString(digest[:]) + "." + kind
	root, err := qualityArtifactRoot(true)
	if err != nil {
		return "", err
	}
	defer root.Close()
	file, err := root.OpenFile(key, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", ErrQualityArtifactUnavailable
	}
	_, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		_ = root.Remove(key)
		return "", ErrQualityArtifactUnavailable
	}
	return key, nil
}

// ReadQualityArtifact returns bounded bytes only, never a storage path. Callers
// must authorize the run first; public callers may request thumbnail only.
func ReadQualityArtifact(run *model.QualityProbeRun, kind string) ([]byte, error) {
	if run == nil {
		return nil, ErrQualityArtifactUnavailable
	}
	var key, extension string
	limit := QualityOutputLimit
	switch kind {
	case "raw":
		if run.HasOutput {
			key = run.RawKey
		}
		extension = "raw"
	case "html":
		if run.HasHTML {
			key = run.HTMLKey
		}
		extension = "html"
	case "thumbnail", "png":
		if run.HasThumbnail {
			key = run.ThumbnailKey
		}
		extension = "png"
		limit = qualityRendererPNGLimit
	default:
		return nil, &QualityValidationError{"Invalid artifact kind"}
	}
	match := qualityArtifactKeyPattern.FindStringSubmatch(key)
	if len(match) != 3 || match[2] != extension {
		return nil, ErrQualityArtifactUnavailable
	}
	root, err := qualityArtifactRoot(false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.Open(key)
	if err != nil {
		return nil, ErrQualityArtifactUnavailable
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > int64(limit) {
		return nil, ErrQualityArtifactUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil || len(data) > limit {
		return nil, ErrQualityArtifactUnavailable
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != match[1] {
		return nil, ErrQualityArtifactUnavailable
	}
	if extension == "png" {
		config, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 4096 || config.Height > 4096 {
			return nil, ErrQualityArtifactUnavailable
		}
	}
	return data, nil
}

// ExtractQualityHTML only selects model-written bytes. It never repairs,
// wraps fragments, supplies missing tags, or invents generated code.
func ExtractQualityHTML(raw []byte) []byte {
	if len(raw) == 0 || len(raw) > QualityOutputLimit {
		return nil
	}
	lower := bytes.ToLower(raw)
	start := bytes.Index(lower, []byte("<html"))
	if start < 0 {
		return nil
	}
	if start+5 >= len(lower) || !strings.ContainsRune(" \t\r\n>", rune(lower[start+5])) {
		return nil
	}
	if doctype := bytes.Index(lower[:start], []byte("<!doctype html")); doctype >= 0 {
		start = doctype
	}
	end := bytes.LastIndex(lower, []byte("</html>"))
	if end < start {
		return nil
	}
	return bytes.TrimSpace(raw[start : end+len("</html>")])
}

func renderQualityRun(ctx context.Context, run *model.QualityProbeRun, owner string, html []byte) {
	pngBytes, err := RenderQualityHTML(ctx, html)
	status, key := "failed", ""
	if err == nil && len(pngBytes) <= qualityRendererPNGLimit {
		config, imageErr := png.DecodeConfig(bytes.NewReader(pngBytes))
		if imageErr == nil && config.Width > 0 && config.Height > 0 && config.Width <= 4096 && config.Height <= 4096 {
			key, err = writeQualityArtifact("png", pngBytes)
			if err == nil {
				status = "completed"
			}
		}
	}
	if ctx.Err() != nil {
		status, key = "interrupted", ""
	}
	// Errors are intentionally not logged: renderer responses are untrusted.
	_ = model.SaveQualityThumbnail(run.ID, owner, key, status, qualityRendererVersion)
}

// CleanupQualityArtifacts retains current baselines and the latest sample for
// every channel. Metadata is unlinked transactionally before objects disappear.
func CleanupQualityArtifacts(ctx context.Context, retentionDays int) error {
	if retentionDays < 1 || retentionDays > 90 {
		return &QualityValidationError{"Invalid retention period"}
	}
	keys, err := model.PruneQualityArtifacts(ctx, time.Now().Add(-time.Duration(retentionDays)*24*time.Hour).Unix(), time.Now().Add(-90*24*time.Hour).Unix())
	if err != nil {
		return err
	}
	root, err := qualityArtifactRoot(false)
	if err != nil {
		return nil
	} // No artifacts is a valid installation state.
	defer root.Close()
	for _, key := range keys {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if qualityArtifactKeyPattern.MatchString(key) {
			_ = root.Remove(key)
		}
	}
	// Grace exceeds the maximum generation+render duration. Newly written
	// objects have time to become referenced before orphan collection.
	directory, err := root.Open(".")
	if err != nil {
		return ErrQualityArtifactUnavailable
	}
	defer directory.Close()
	for {
		entries, readErr := directory.ReadDir(100)
		for _, entry := range entries {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if !qualityArtifactKeyPattern.MatchString(entry.Name()) {
				continue
			}
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() || info.ModTime().After(time.Now().Add(-24*time.Hour)) {
				continue
			}
			referenced, err := model.QualityArtifactReferenced(entry.Name())
			if err != nil {
				return err
			}
			if !referenced {
				_ = root.Remove(entry.Name())
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return ErrQualityArtifactUnavailable
		}
	}
}
