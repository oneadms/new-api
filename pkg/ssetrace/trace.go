package ssetrace

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/sse_trace_setting"
)

const (
	jobQueueCapacity = 4096
	writerCount      = 2
	cleanupBatchSize = 500
	readHardLimit    = 64 << 20
)

var (
	ErrTraceNotFound = errors.New("sse trace not found")
	ErrTraceExpired  = errors.New("sse trace expired")
	ErrTraceTooLarge = errors.New("sse trace exceeds read limit")

	requestIDSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	globalManager      = newManager()
)

type writeJob struct {
	requestID     string
	data          []byte
	originalSize  int64
	truncated     bool
	storagePath   string
	retentionHour int
}

type Manager struct {
	jobs        chan writeJob
	queuedBytes atomic.Int64
	dropped     atomic.Int64
	written     atomic.Int64
	writeErrors atomic.Int64
	once        sync.Once
}

type Stats struct {
	Enabled        bool   `json:"enabled"`
	StoragePath    string `json:"storage_path"`
	MaxRequestMB   int    `json:"max_request_mb"`
	QueueMaxMB     int    `json:"queue_max_mb"`
	RetentionHours int    `json:"retention_hours"`
	QueuedBytes    int64  `json:"queued_bytes"`
	Dropped        int64  `json:"dropped"`
	Written        int64  `json:"written"`
	WriteErrors    int64  `json:"write_errors"`
	FileCount      int64  `json:"file_count"`
	TotalSize      int64  `json:"total_size"`
}

type TraceContent struct {
	Metadata model.SSETrace `json:"metadata"`
	Content  string         `json:"content"`
}

type capture struct {
	manager       *Manager
	requestID     string
	maxBytes      int
	queueMaxBytes int64
	storagePath   string
	retentionHour int

	mutex        sync.Mutex
	data         []byte
	originalSize int64
	truncated    bool
	finished     bool
}

type captureReadCloser struct {
	source  io.ReadCloser
	capture *capture
	once    sync.Once
}

func newManager() *Manager {
	return &Manager{jobs: make(chan writeJob, jobQueueCapacity)}
}

func Init() {
	globalManager.once.Do(func() {
		for range writerCount {
			go globalManager.writerLoop()
		}
		go globalManager.cleanupLoop()
		go func() {
			if err := CleanupExpired(); err != nil {
				common.SysError("failed to clean expired SSE traces: " + err.Error())
			}
		}()
	})
}

// WrapReadCloser transparently captures the bytes read from an upstream SSE
// response. When tracing is disabled it returns source unchanged.
func WrapReadCloser(source io.ReadCloser, requestID string) io.ReadCloser {
	if source == nil || strings.TrimSpace(requestID) == "" {
		return source
	}
	setting := sse_trace_setting.GetSetting()
	if !setting.Enabled {
		return source
	}
	Init()
	maxBytes := setting.MaxRequestMB << 20
	return &captureReadCloser{
		source: source,
		capture: &capture{
			manager:       globalManager,
			requestID:     requestID,
			maxBytes:      maxBytes,
			queueMaxBytes: int64(setting.QueueMaxMB) << 20,
			storagePath:   setting.StoragePath,
			retentionHour: setting.RetentionHours,
			data:          make([]byte, 0, min(maxBytes, 64<<10)),
		},
	}
}

func (r *captureReadCloser) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	if n > 0 {
		r.capture.append(p[:n])
	}
	if errors.Is(err, io.EOF) {
		r.finish()
	}
	return n, err
}

func (r *captureReadCloser) Close() error {
	err := r.source.Close()
	r.finish()
	return err
}

func (r *captureReadCloser) finish() {
	r.once.Do(r.capture.finish)
}

func (c *capture) append(data []byte) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.finished {
		return
	}
	c.originalSize += int64(len(data))
	remaining := c.maxBytes - len(c.data)
	if remaining <= 0 {
		c.truncated = true
		return
	}
	if len(data) > remaining {
		c.data = append(c.data, data[:remaining]...)
		c.truncated = true
		return
	}
	c.data = append(c.data, data...)
}

func (c *capture) finish() {
	c.mutex.Lock()
	if c.finished {
		c.mutex.Unlock()
		return
	}
	c.finished = true
	job := writeJob{
		requestID:     c.requestID,
		data:          c.data,
		originalSize:  c.originalSize,
		truncated:     c.truncated,
		storagePath:   c.storagePath,
		retentionHour: c.retentionHour,
	}
	c.data = nil
	c.mutex.Unlock()

	if len(job.data) == 0 {
		return
	}
	c.manager.enqueue(job, c.queueMaxBytes)
}

func (m *Manager) enqueue(job writeJob, maxQueueBytes int64) {
	size := int64(len(job.data))
	for {
		current := m.queuedBytes.Load()
		if current+size > maxQueueBytes {
			m.dropped.Add(1)
			return
		}
		if m.queuedBytes.CompareAndSwap(current, current+size) {
			break
		}
	}

	select {
	case m.jobs <- job:
	default:
		m.queuedBytes.Add(-size)
		m.dropped.Add(1)
	}
}

func (m *Manager) writerLoop() {
	for job := range m.jobs {
		m.queuedBytes.Add(-int64(len(job.data)))
		if err := writeTrace(job); err != nil {
			m.writeErrors.Add(1)
			common.SysError("failed to persist SSE trace: " + err.Error())
			continue
		}
		m.written.Add(1)
	}
}

func writeTrace(job writeJob) error {
	root, err := filepath.Abs(job.storagePath)
	if err != nil {
		return fmt.Errorf("resolve storage path: %w", err)
	}
	now := time.Now()
	dir := filepath.Join(root, now.Format("2006"), now.Format("01"), now.Format("02"))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create trace directory: %w", err)
	}

	tempFile, err := os.CreateTemp(dir, ".sse-trace-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary trace file: %w", err)
	}
	tempName := tempFile.Name()
	cleanupTemp := true
	defer func() {
		_ = tempFile.Close()
		if cleanupTemp {
			_ = os.Remove(tempName)
		}
	}()
	if err := tempFile.Chmod(0o600); err != nil {
		return fmt.Errorf("set trace file permissions: %w", err)
	}

	gzipWriter, err := gzip.NewWriterLevel(tempFile, gzip.BestSpeed)
	if err != nil {
		return fmt.Errorf("create gzip writer: %w", err)
	}
	if _, err := gzipWriter.Write(job.data); err != nil {
		_ = gzipWriter.Close()
		return fmt.Errorf("compress SSE trace: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("finish SSE trace compression: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close SSE trace file: %w", err)
	}

	fileInfo, err := os.Stat(tempName)
	if err != nil {
		return fmt.Errorf("stat SSE trace file: %w", err)
	}
	fileName := fmt.Sprintf("%s-%d.sse.gz", sanitizeRequestID(job.requestID), now.UnixNano())
	finalPath := filepath.Join(dir, fileName)
	if err := os.Rename(tempName, finalPath); err != nil {
		return fmt.Errorf("publish SSE trace file: %w", err)
	}
	cleanupTemp = false

	oldTrace, _ := model.GetSSETraceByRequestID(job.requestID)
	trace := &model.SSETrace{
		RequestId:      job.requestID,
		FilePath:       finalPath,
		OriginalSize:   job.originalSize,
		CapturedSize:   int64(len(job.data)),
		CompressedSize: fileInfo.Size(),
		EventCount:     countSSEEvents(job.data),
		Truncated:      job.truncated,
		CreatedAt:      now.Unix(),
		ExpiresAt:      now.Add(time.Duration(job.retentionHour) * time.Hour).Unix(),
	}
	if err := model.UpsertSSETrace(trace); err != nil {
		_ = os.Remove(finalPath)
		return fmt.Errorf("save SSE trace metadata: %w", err)
	}
	if oldTrace != nil && oldTrace.FilePath != "" && oldTrace.FilePath != finalPath {
		_ = os.Remove(oldTrace.FilePath)
	}
	return nil
}

func countSSEEvents(data []byte) int {
	count := bytes.Count(data, []byte("\ndata:"))
	if bytes.HasPrefix(data, []byte("data:")) {
		count++
	}
	return count
}

func sanitizeRequestID(requestID string) string {
	value := requestIDSanitizer.ReplaceAllString(requestID, "_")
	value = strings.Trim(value, "._-")
	if value == "" {
		return "request"
	}
	if len(value) > 96 {
		return value[:96]
	}
	return value
}

func Read(requestID string) (*TraceContent, error) {
	trace, err := model.GetSSETraceByRequestID(requestID)
	if err != nil {
		if model.IsSSETraceNotFound(err) {
			return nil, ErrTraceNotFound
		}
		return nil, err
	}
	if trace.ExpiresAt <= time.Now().Unix() {
		if err := removeTraceFile(trace.FilePath); err != nil {
			return nil, fmt.Errorf("remove expired SSE trace: %w", err)
		}
		if err := model.DeleteSSETraceByRequestID(trace.RequestId); err != nil {
			return nil, fmt.Errorf("delete expired SSE trace metadata: %w", err)
		}
		return nil, ErrTraceExpired
	}

	file, err := os.Open(trace.FilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_ = model.DeleteSSETraceByRequestID(trace.RequestId)
			return nil, ErrTraceNotFound
		}
		return nil, err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()

	data, err := io.ReadAll(io.LimitReader(gzipReader, readHardLimit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > readHardLimit {
		return nil, ErrTraceTooLarge
	}
	return &TraceContent{Metadata: *trace, Content: string(data)}, nil
}

func CleanupExpired() error {
	for {
		traces, err := model.GetExpiredSSETraces(cleanupBatchSize)
		if err != nil {
			return err
		}
		if len(traces) == 0 {
			return nil
		}
		ids := make([]int64, 0, len(traces))
		var removeErr error
		for _, trace := range traces {
			if err := removeTraceFile(trace.FilePath); err != nil {
				if removeErr == nil {
					removeErr = fmt.Errorf("remove expired SSE trace %q: %w", trace.RequestId, err)
				}
				continue
			}
			ids = append(ids, trace.Id)
		}
		if err := model.DeleteSSETracesByIDs(ids); err != nil {
			return err
		}
		if removeErr != nil {
			return removeErr
		}
		if len(traces) < cleanupBatchSize {
			return nil
		}
	}
}

func removeTraceFile(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		if err := CleanupExpired(); err != nil {
			common.SysError("failed to clean expired SSE traces: " + err.Error())
		}
	}
}

func GetStats() (Stats, error) {
	setting := sse_trace_setting.GetSetting()
	dbStats, err := model.GetSSETraceStats()
	if err != nil {
		return Stats{}, err
	}
	return Stats{
		Enabled:        setting.Enabled,
		StoragePath:    setting.StoragePath,
		MaxRequestMB:   setting.MaxRequestMB,
		QueueMaxMB:     setting.QueueMaxMB,
		RetentionHours: setting.RetentionHours,
		QueuedBytes:    globalManager.queuedBytes.Load(),
		Dropped:        globalManager.dropped.Load(),
		Written:        globalManager.written.Load(),
		WriteErrors:    globalManager.writeErrors.Load(),
		FileCount:      dbStats.FileCount,
		TotalSize:      dbStats.TotalSize,
	}, nil
}
