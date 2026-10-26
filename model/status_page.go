package model

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StatusPageSnapshot contains only published targets and database-persisted
// counters. It never merges a process-local active performance bucket.
type StatusPageSnapshot struct {
	Channels  []Channel
	Profiles  []ChannelQualityProfile
	Abilities []Ability
	Runs      []QualityProbeRun
	RunGroups []QualityProbeRunGroup
	Metrics   []PerfMetric
}

func LoadStatusPageSnapshot(ctx context.Context, groups []string, selectedModel string, start, end int64) (StatusPageSnapshot, error) {
	var result StatusPageSnapshot
	if len(groups) == 0 {
		return result, nil
	}
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("publish_sample = ?", true).Find(&result.Profiles).Error; err != nil {
			return err
		}
		ids := make([]int, 0, len(result.Profiles))
		for _, profile := range result.Profiles {
			ids = append(ids, profile.ChannelID)
		}
		if len(ids) > 0 {
			// Overrides are needed only for private configuration fingerprinting.
			// Channel credentials are never loaded by the public query path.
			if err := tx.Omit("key").Where("id IN ?", ids).Find(&result.Channels).Error; err != nil {
				return err
			}
			if err := tx.Where("channel_id IN ?", ids).Where(map[string]any{"group": groups}).Find(&result.Abilities).Error; err != nil {
				return err
			}
			latest := tx.Model(&QualityProbeRun{}).Select("MAX(id)").Where("channel_id IN ? AND completed_at > 0", ids).Group("channel_id")
			if err := tx.Where("id IN (?)", latest).Find(&result.Runs).Error; err != nil {
				return err
			}
			runIDs := make([]int64, 0, len(result.Runs))
			for _, run := range result.Runs {
				runIDs = append(runIDs, run.ID)
			}
			if len(runIDs) > 0 {
				if err := tx.Where("run_id IN ? AND group_name IN ?", runIDs, groups).Find(&result.RunGroups).Error; err != nil {
					return err
				}
			}
		}
		query := tx.Model(&PerfMetric{}).
			Select("?, bucket_ts, SUM(request_count) AS request_count, SUM(success_count) AS success_count, SUM(total_latency_ms) AS total_latency_ms, SUM(output_tokens) AS output_tokens, SUM(generation_ms) AS generation_ms", clause.Column{Name: "group"}).
			Where(map[string]any{"group": groups}).Where("bucket_ts >= ? AND bucket_ts < ?", start, end)
		if selectedModel != "" {
			query = query.Where("model_name = ?", selectedModel)
		}
		return query.Group("group").Group("bucket_ts").Order("bucket_ts").Find(&result.Metrics).Error
	})
	return result, err
}

func GetQualityRunByPublicID(id string) (*QualityProbeRun, error) {
	var run QualityProbeRun
	if err := DB.Where("public_id = ?", id).First(&run).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

// QualityRunPublishedInGroups checks historical membership independently of
// current channel membership, so moving a channel cannot move its old history.
func QualityRunPublishedInGroups(runID int64, groups []string) (bool, error) {
	if len(groups) == 0 {
		return false, nil
	}
	var count int64
	err := DB.Model(&QualityProbeRunGroup{}).Where("run_id = ? AND group_name IN ?", runID, groups).Count(&count).Error
	return count > 0, err
}
