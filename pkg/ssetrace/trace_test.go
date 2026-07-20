package ssetrace

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCaptureTruncatesWithoutBlockingRelayData(t *testing.T) {
	manager := newManager()
	capture := &capture{
		manager:       manager,
		requestID:     "req_truncated",
		maxBytes:      8,
		queueMaxBytes: 1024,
		storagePath:   t.TempDir(),
		retentionHour: 1,
	}

	capture.append([]byte("data: first\n\n"))
	capture.finish()

	select {
	case job := <-manager.jobs:
		require.Equal(t, "data: fi", string(job.data))
		require.Equal(t, int64(len("data: first\n\n")), job.originalSize)
		require.True(t, job.truncated)
	case <-time.After(time.Second):
		t.Fatal("trace was not enqueued")
	}
}

func TestManagerDropsTraceWhenByteBudgetIsFull(t *testing.T) {
	manager := newManager()
	manager.enqueue(writeJob{data: []byte("123456")}, 5)

	require.Equal(t, int64(1), manager.dropped.Load())
	require.Equal(t, int64(0), manager.queuedBytes.Load())
	require.Len(t, manager.jobs, 0)
}

func TestWriteAndReadTrace(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:sse-trace-test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.SSETrace{}))

	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB })

	content := "data: {\"id\":1}\n\ndata: [DONE]\n\n"
	err = writeTrace(writeJob{
		requestID:     "req_read_write",
		data:          []byte(content),
		originalSize:  int64(len(content)),
		storagePath:   t.TempDir(),
		retentionHour: 1,
	})
	require.NoError(t, err)

	trace, err := Read("req_read_write")
	require.NoError(t, err)
	require.Equal(t, content, trace.Content)
	require.Equal(t, int64(len(content)), trace.Metadata.OriginalSize)
	require.Equal(t, int64(len(content)), trace.Metadata.CapturedSize)
	require.Equal(t, 2, trace.Metadata.EventCount)
	require.False(t, trace.Metadata.Truncated)
	require.True(t, strings.HasSuffix(trace.Metadata.RequestId, "read_write"))

	require.NoError(t, db.Model(&model.SSETrace{}).
		Where("request_id = ?", "req_read_write").
		Update("expires_at", time.Now().Add(-time.Hour).Unix()).Error)
	require.NoError(t, CleanupExpired())
	_, err = Read("req_read_write")
	require.ErrorIs(t, err, ErrTraceNotFound)
}
