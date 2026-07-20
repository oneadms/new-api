package sse_trace_setting

import (
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/setting/config"
)

const (
	defaultStoragePath    = "data/sse-traces"
	defaultMaxRequestMB   = 1
	defaultQueueMaxMB     = 256
	defaultRetentionHours = 24
)

type SSETraceSetting struct {
	Enabled        bool   `json:"enabled"`
	MaxRequestMB   int    `json:"max_request_mb"`
	QueueMaxMB     int    `json:"queue_max_mb"`
	RetentionHours int    `json:"retention_hours"`
	StoragePath    string `json:"storage_path"`
}

var sseTraceSetting = SSETraceSetting{
	Enabled:        false,
	MaxRequestMB:   defaultMaxRequestMB,
	QueueMaxMB:     defaultQueueMaxMB,
	RetentionHours: defaultRetentionHours,
	StoragePath:    defaultStoragePath,
}

var currentSetting atomic.Value

func init() {
	config.GlobalConfig.Register("sse_trace_setting", &sseTraceSetting)
	UpdateSnapshot()
}

func GetSetting() SSETraceSetting {
	return currentSetting.Load().(SSETraceSetting)
}

func UpdateSnapshot() {
	setting := sseTraceSetting
	setting.MaxRequestMB = clamp(setting.MaxRequestMB, 1, 64, defaultMaxRequestMB)
	setting.QueueMaxMB = clamp(setting.QueueMaxMB, 16, 4096, defaultQueueMaxMB)
	setting.RetentionHours = clamp(setting.RetentionHours, 1, 24*30, defaultRetentionHours)
	setting.StoragePath = strings.TrimSpace(setting.StoragePath)
	if setting.StoragePath == "" {
		setting.StoragePath = defaultStoragePath
	}
	setting.StoragePath = filepath.Clean(setting.StoragePath)
	currentSetting.Store(setting)
}

func clamp(value, minValue, maxValue, fallback int) int {
	if value < minValue || value > maxValue {
		return fallback
	}
	return value
}
