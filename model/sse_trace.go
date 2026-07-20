package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SSETrace struct {
	Id             int64  `json:"id"`
	RequestId      string `json:"request_id" gorm:"type:varchar(128);uniqueIndex"`
	FilePath       string `json:"-" gorm:"type:text;not null"`
	OriginalSize   int64  `json:"original_size" gorm:"not null;default:0"`
	CapturedSize   int64  `json:"captured_size" gorm:"not null;default:0"`
	CompressedSize int64  `json:"compressed_size" gorm:"not null;default:0"`
	EventCount     int    `json:"event_count" gorm:"not null;default:0"`
	Truncated      bool   `json:"truncated" gorm:"not null;default:false"`
	CreatedAt      int64  `json:"created_at" gorm:"index"`
	ExpiresAt      int64  `json:"expires_at" gorm:"index"`
}

type SSETraceStats struct {
	FileCount int64 `json:"file_count"`
	TotalSize int64 `json:"total_size"`
}

func UpsertSSETrace(trace *SSETrace) error {
	if trace == nil {
		return errors.New("sse trace is nil")
	}
	return LOG_DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "request_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"file_path",
			"original_size",
			"captured_size",
			"compressed_size",
			"event_count",
			"truncated",
			"created_at",
			"expires_at",
		}),
	}).Create(trace).Error
}

func GetSSETraceByRequestID(requestID string) (*SSETrace, error) {
	var trace SSETrace
	err := LOG_DB.Where("request_id = ?", requestID).First(&trace).Error
	if err != nil {
		return nil, err
	}
	return &trace, nil
}

func DeleteSSETraceByRequestID(requestID string) error {
	return LOG_DB.Where("request_id = ?", requestID).Delete(&SSETrace{}).Error
}

func GetExpiredSSETraces(limit int) ([]SSETrace, error) {
	if limit <= 0 {
		limit = 500
	}
	var traces []SSETrace
	err := LOG_DB.Where("expires_at <= ?", time.Now().Unix()).Limit(limit).Find(&traces).Error
	return traces, err
}

func DeleteSSETracesByIDs(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return LOG_DB.Where("id IN ?", ids).Delete(&SSETrace{}).Error
}

func GetSSETraceStats() (SSETraceStats, error) {
	var stats SSETraceStats
	if err := LOG_DB.Model(&SSETrace{}).Count(&stats.FileCount).Error; err != nil {
		return stats, err
	}
	var row struct {
		TotalSize *int64
	}
	if err := LOG_DB.Model(&SSETrace{}).Select("SUM(compressed_size) AS total_size").Scan(&row).Error; err != nil {
		return stats, err
	}
	if row.TotalSize != nil {
		stats.TotalSize = *row.TotalSize
	}
	return stats, nil
}

func IsSSETraceNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
