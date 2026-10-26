package model

import (
	"context"

	"gorm.io/gorm"
)

func QualityArtifactReferenced(key string) (bool, error) {
	var count int64
	err := DB.Model(&QualityProbeRun{}).Where("raw_key = ? OR html_key = ? OR thumbnail_key = ?", key, key, key).Count(&count).Error
	return count > 0, err
}

// PruneQualityArtifacts protects current baselines, any historical baseline
// reference, and latest outputs before unlinking expired object references.
func PruneQualityArtifacts(ctx context.Context, artifactBefore, metadataBefore int64) ([]string, error) {
	var candidates []QualityProbeRun
	latestRuns := DB.Model(&QualityProbeRun{}).Select("MAX(id)").Group("channel_id")
	latestOutputs := DB.Model(&QualityProbeRun{}).Select("MAX(id)").Where("has_output = ?", true).Group("channel_id")
	baselines := DB.Model(&ChannelQualityProfile{}).Select("baseline_id").Where("baseline_id > 0")
	referencedBaselines := DB.Model(&QualityProbeRun{}).Select("baseline_id").Where("baseline_id > 0")
	if err := DB.WithContext(ctx).Where("completed_at > 0 AND completed_at < ?", artifactBefore).
		Where("has_output = ? OR has_html = ? OR has_thumbnail = ? OR completed_at < ?", true, true, true, metadataBefore).
		Where("id NOT IN (?) AND id NOT IN (?) AND id NOT IN (?) AND id NOT IN (?)", latestRuns, latestOutputs, baselines, referencedBaselines).
		Order("id").Limit(500).Find(&candidates).Error; err != nil {
		return nil, err
	}
	keys := []string{}
	for _, candidate := range candidates {
		err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var run QualityProbeRun
			if err := lockForUpdate(tx).First(&run, candidate.ID).Error; err != nil {
				return err
			}
			var count int64
			if err := tx.Model(&ChannelQualityProfile{}).Where("baseline_id = ?", run.ID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return nil
			}
			if err := tx.Model(&QualityProbeRun{}).Where("baseline_id = ?", run.ID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return nil
			}
			var latest QualityProbeRun
			if err := tx.Where("channel_id = ?", run.ChannelID).Order("id desc").First(&latest).Error; err != nil {
				return err
			}
			if latest.ID == run.ID {
				return nil
			}
			if run.HasOutput {
				var latestOutput QualityProbeRun
				if err := tx.Where("channel_id = ? AND has_output = ?", run.ChannelID, true).
					Order("id desc").First(&latestOutput).Error; err != nil {
					return err
				}
				if latestOutput.ID == run.ID {
					return nil
				}
			}
			var batch QualityProbeBatch
			if err := tx.First(&batch, run.BatchID).Error; err != nil {
				return err
			}
			if batch.CompletedAt == 0 {
				return nil
			}
			artifactKeys := []string{run.RawKey, run.HTMLKey, run.ThumbnailKey}
			if run.CompletedAt < metadataBefore {
				if err := tx.Where("run_id = ?", run.ID).Delete(&QualityProbeRunGroup{}).Error; err != nil {
					return err
				}
				if err := tx.Where("run_id = ?", run.ID).Delete(&QualityReviewAudit{}).Error; err != nil {
					return err
				}
				if err := tx.Delete(&run).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Model(&run).Updates(map[string]any{"raw_key": "", "html_key": "", "thumbnail_key": "", "has_output": false,
					"has_html": false, "has_thumbnail": false, "render_status": "expired"}).Error; err != nil {
					return err
				}
			}
			keys = append(keys, artifactKeys...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return keys, nil
}
