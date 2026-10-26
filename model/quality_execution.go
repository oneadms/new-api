package model

import (
	"errors"
	"math"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func FindQualityBatchByKey(key, fingerprint string) (*QualityProbeBatch, error) {
	var batch QualityProbeBatch
	err := DB.Where("idempotency_key = ?", key).First(&batch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if batch.RequestFingerprint != fingerprint {
		return &batch, ErrQualityIdempotency
	}
	return &batch, nil
}

func FindQualityBatchByTask(taskID string) (*QualityProbeBatch, error) {
	var batch QualityProbeBatch
	err := DB.Where("task_id = ?", taskID).First(&batch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &batch, err
}

// CreateQualityBatch atomically publishes the task and its frozen targets. A
// scheduled task may initialize its batch only while it owns a valid lease.
func CreateQualityBatch(batch *QualityProbeBatch, runs []QualityProbeRun, scheduledTask *SystemTask, owner string) (*QualityProbeBatch, error) {
	err := DB.Transaction(func(tx *gorm.DB) error {
		if scheduledTask == nil {
			task, err := CreateSystemTaskTx(tx, SystemTaskTypeQualityDetection, nil, nil)
			if err != nil {
				return err
			}
			batch.TaskID = task.TaskID
		} else {
			if err := qualityLease(tx, scheduledTask.TaskID, owner); err != nil {
				return err
			}
			var existing QualityProbeBatch
			err := tx.Where("task_id = ?", scheduledTask.TaskID).First(&existing).Error
			if err == nil {
				*batch = existing
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			batch.TaskID = scheduledTask.TaskID
		}
		batch.Total = len(runs)
		if err := tx.Create(batch).Error; err != nil {
			return err
		}
		for i := range runs {
			run := &runs[i]
			run.BatchID = batch.ID
			if err := tx.Create(run).Error; err != nil {
				return err
			}
			for _, group := range run.Groups {
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&QualityProbeRunGroup{RunID: run.ID, GroupName: group}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err == nil {
		return batch, nil
	}
	if batch.IdempotencyKey != nil {
		existing, lookupErr := FindQualityBatchByKey(*batch.IdempotencyKey, batch.RequestFingerprint)
		if lookupErr != nil || existing != nil {
			return existing, lookupErr
		}
	}
	if scheduledTask == nil {
		active, lookupErr := GetActiveSystemTask(SystemTaskTypeQualityDetection)
		if lookupErr == nil && active != nil {
			existing, _ := FindQualityBatchByTask(active.TaskID)
			return existing, ErrQualityConflict
		}
	}
	return nil, err
}

// qualityLease takes the same database lease row that renew/takeover updates.
// Child writes also have state predicates, so SQLite's conditional writes and
// transaction conflict detection provide fencing where FOR UPDATE is absent.
func qualityLease(tx *gorm.DB, taskID, owner string) error {
	var lease SystemTaskLock
	err := lockForUpdate(tx).Where("type = ? AND task_id = ? AND locked_by = ? AND locked_until >= ?",
		SystemTaskTypeQualityDetection, taskID, owner, time.Now().Unix()).First(&lease).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrSystemTaskLockLost
	}
	if err != nil {
		return err
	}
	var task SystemTask
	err = lockForUpdate(tx).Select("id").Where("task_id = ? AND locked_by = ? AND status = ?", taskID, owner, SystemTaskStatusRunning).First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrSystemTaskLockLost
	}
	return err
}

func CheckQualityBatchLease(taskID, owner string) (bool, error) {
	var batch QualityProbeBatch
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := qualityLease(tx, taskID, owner); err != nil {
			return err
		}
		return tx.Where("task_id = ?", taskID).First(&batch).Error
	})
	return batch.CancelRequested, err
}

func ReserveQualityBatchBudget(batchID int64, owner string, settings QualitySettings, date string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var batch QualityProbeBatch
		if err := tx.First(&batch, batchID).Error; err != nil {
			return err
		}
		if err := qualityLease(tx, batch.TaskID, owner); err != nil {
			return err
		}
		if err := lockForUpdate(tx).First(&batch, batchID).Error; err != nil {
			return err
		}
		if batch.CancelRequested {
			return ErrQualityCancelled
		}
		if batch.BudgetReserved {
			return nil
		}
		if batch.CompletedAt != 0 {
			return ErrQualityConflict
		}
		var runs []QualityProbeRun
		if err := tx.Where("batch_id = ? AND status = ?", batch.ID, "queued").Find(&runs).Error; err != nil {
			return err
		}
		calls, tokens := int64(len(runs)), int64(0)
		for _, run := range runs {
			if run.BudgetTokens < 1 || run.BudgetTokens > settings.DailyTokenLimit-tokens {
				return ErrQualityBudget
			}
			tokens += run.BudgetTokens
		}
		if calls > settings.DailyCallLimit || settings.DailyTokenLimit < 1 || settings.DailyTokenLimit > 1000000000000 {
			return ErrQualityBudget
		}
		budget := QualityProbeDailyBudget{Date: date}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&budget).Error; err != nil {
			return err
		}
		result := tx.Model(&QualityProbeDailyBudget{}).Where("date = ? AND calls <= ? AND tokens <= ?", date, settings.DailyCallLimit-calls, settings.DailyTokenLimit-tokens).
			Updates(map[string]any{"calls": gorm.Expr("calls + ?", calls), "tokens": gorm.Expr("tokens + ?", tokens)})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 && calls > 0 {
			return ErrQualityBudget
		}
		if err := tx.Model(&QualityProbeRun{}).Where("batch_id = ? AND status = ?", batch.ID, "queued").Update("budget_state", "reserved").Error; err != nil {
			return err
		}
		return tx.Model(&batch).Updates(map[string]any{"budget_reserved": true, "budget_date": date, "status": "running"}).Error
	})
}

func ClaimQualityRun(id int64, owner string) (*QualityProbeRun, error) {
	var run QualityProbeRun
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&run, id).Error; err != nil {
			return err
		}
		var batch QualityProbeBatch
		if err := tx.First(&batch, run.BatchID).Error; err != nil {
			return err
		}
		if err := qualityLease(tx, batch.TaskID, owner); err != nil {
			return err
		}
		if err := lockForUpdate(tx).First(&batch, run.BatchID).Error; err != nil {
			return err
		}
		if batch.CancelRequested {
			return ErrQualityCancelled
		}
		if !batch.BudgetReserved || batch.CompletedAt != 0 {
			return ErrQualityConflict
		}
		now := time.Now().Unix()
		result := tx.Model(&QualityProbeRun{}).Where("id = ? AND status = ? AND budget_state = ?", id, "queued", "reserved").
			Updates(map[string]any{"status": "running", "started_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrQualityConflict
		}
		run.Status, run.StartedAt = "running", now
		return nil
	})
	return &run, err
}

// SaveQualityRun commits immutable generation metadata and trusted usage in the
// same fenced transaction. Nil/invalid usage keeps the reservation unchanged.
func SaveQualityRun(run *QualityProbeRun, owner string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var batch QualityProbeBatch
		if err := tx.First(&batch, run.BatchID).Error; err != nil {
			return err
		}
		if err := qualityLease(tx, batch.TaskID, owner); err != nil {
			return err
		}
		if err := lockForUpdate(tx).First(&batch, run.BatchID).Error; err != nil {
			return err
		}
		var stored QualityProbeRun
		if err := lockForUpdate(tx).First(&stored, run.ID).Error; err != nil {
			return err
		}
		if stored.Status != "running" {
			return ErrQualityConflict
		}
		if batch.CancelRequested && run.Status != "completed" {
			run.Status, run.ErrorCode = "cancelled", "cancelled"
		}
		run.BudgetState = stored.BudgetState
		if stored.BudgetState == "reserved" {
			known := run.Status == "completed" && run.InputTokens != nil && run.OutputTokens != nil && *run.InputTokens >= 0 && *run.OutputTokens >= 0 &&
				*run.InputTokens <= math.MaxInt32 && *run.OutputTokens <= math.MaxInt32
			if !run.RequestStarted {
				result := tx.Model(&QualityProbeDailyBudget{}).Where("date = ? AND calls >= 1 AND tokens >= ?", batch.BudgetDate, stored.BudgetTokens).
					Updates(map[string]any{"calls": gorm.Expr("calls - 1"), "tokens": gorm.Expr("tokens - ?", stored.BudgetTokens)})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != 1 {
					return ErrQualityConflict
				}
				run.BudgetState = "released"
			} else if known {
				actual := *run.InputTokens + *run.OutputTokens
				result := tx.Model(&QualityProbeDailyBudget{}).Where("date = ? AND tokens >= ?", batch.BudgetDate, stored.BudgetTokens).
					Updates(map[string]any{"tokens": gorm.Expr("tokens - ? + ?", stored.BudgetTokens, actual),
						"settled_calls": gorm.Expr("settled_calls + 1"), "settled_tokens": gorm.Expr("settled_tokens + ?", actual)})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != 1 {
					return ErrQualityConflict
				}
				run.BudgetState = "settled"
			} else {
				run.InputTokens, run.OutputTokens = nil, nil
				run.BudgetState = "unknown"
			}
		}
		// Explicit fields prevent review, snapshots, or references from being
		// accidentally overwritten by a stale in-memory worker.
		if err := tx.Model(&stored).Updates(map[string]any{
			"status": run.Status, "quality_result": run.QualityResult, "completed_at": run.CompletedAt,
			"first_text_ms": run.FirstTextMS, "generation_ms": run.GenerationMS,
			"input_tokens": run.InputTokens, "output_tokens": run.OutputTokens,
			"error_code": run.ErrorCode, "mapped_model": run.MappedModel, "response_model": run.ResponseModel,
			"request_started": run.RequestStarted, "request_fingerprint": run.RequestFingerprint, "budget_state": run.BudgetState,
			"has_output": run.HasOutput, "has_html": run.HasHTML, "raw_key": run.RawKey, "html_key": run.HTMLKey,
			"render_status": run.RenderStatus, "extractor_version": run.ExtractorVersion, "rule_results": run.RuleResults,
		}).Error; err != nil {
			return err
		}
		if run.Status == "completed" {
			return tx.Model(&batch).Update("completed", gorm.Expr("completed + 1")).Error
		}
		if run.Status == "failed" || run.Status == "interrupted" {
			return tx.Model(&batch).Update("failed", gorm.Expr("failed + 1")).Error
		}
		return nil
	})
}

func SaveQualityThumbnail(runID int64, owner, key, status, version string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var run QualityProbeRun
		if err := tx.First(&run, runID).Error; err != nil {
			return err
		}
		var batch QualityProbeBatch
		if err := tx.First(&batch, run.BatchID).Error; err != nil {
			return err
		}
		if err := qualityLease(tx, batch.TaskID, owner); err != nil {
			return err
		}
		if run.RenderStatus != "pending" {
			return ErrQualityConflict
		}
		return tx.Model(&run).Where("render_status = ?", "pending").Updates(map[string]any{
			"thumbnail_key": key, "has_thumbnail": key != "", "render_status": status, "renderer_version": version,
		}).Error
	})
}

func FinishQualityBatch(id int64, owner, reason string) (*QualityProbeBatch, error) {
	var batch QualityProbeBatch
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&batch, id).Error; err != nil {
			return err
		}
		if err := qualityLease(tx, batch.TaskID, owner); err != nil {
			return err
		}
		if err := lockForUpdate(tx).First(&batch, id).Error; err != nil {
			return err
		}
		if batch.CompletedAt != 0 {
			return nil
		}
		now := time.Now().Unix()
		if batch.CancelRequested {
			reason = "cancelled"
		}
		if reason == "" {
			var unfinished int64
			if err := tx.Model(&QualityProbeRun{}).Where("batch_id = ? AND status IN ?", id, []string{"queued", "running"}).Count(&unfinished).Error; err != nil {
				return err
			}
			if unfinished > 0 {
				reason = "interrupted"
			}
		}
		if reason != "" {
			var queued []QualityProbeRun
			if err := tx.Where("batch_id = ? AND status = ?", id, "queued").Find(&queued).Error; err != nil {
				return err
			}
			var calls, tokens int64
			for _, run := range queued {
				if run.BudgetState == "reserved" {
					calls++
					tokens += run.BudgetTokens
				}
			}
			if calls > 0 {
				result := tx.Model(&QualityProbeDailyBudget{}).Where("date = ? AND calls >= ? AND tokens >= ?", batch.BudgetDate, calls, tokens).
					Updates(map[string]any{"calls": gorm.Expr("calls - ?", calls), "tokens": gorm.Expr("tokens - ?", tokens)})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != 1 {
					return ErrQualityConflict
				}
			}
			status := "skipped"
			if reason == "cancelled" {
				status = "cancelled"
			}
			if err := tx.Model(&QualityProbeRun{}).Where("batch_id = ? AND status = ?", id, "queued").Updates(map[string]any{
				"status": status, "error_code": reason, "completed_at": now, "budget_state": "released"}).Error; err != nil {
				return err
			}
			if err := tx.Model(&QualityProbeRun{}).Where("batch_id = ? AND status = ?", id, "running").Updates(map[string]any{
				"status": "interrupted", "error_code": "interrupted", "completed_at": now, "budget_state": "unknown"}).Error; err != nil {
				return err
			}
		}
		var runs []QualityProbeRun
		if err := tx.Where("batch_id = ?", id).Find(&runs).Error; err != nil {
			return err
		}
		batch.Completed, batch.Failed = 0, 0
		for _, run := range runs {
			if run.Status == "queued" || run.Status == "running" {
				return ErrQualityConflict
			}
			if run.Status == "completed" {
				batch.Completed++
			}
			if run.Status == "failed" || run.Status == "interrupted" {
				batch.Failed++
			}
		}
		batch.Status = "completed"
		if batch.Failed > 0 {
			batch.Status = "partial"
			if batch.Completed == 0 {
				batch.Status = "failed"
			}
		}
		if reason != "" {
			batch.Status = reason
		}
		batch.CompletedAt, batch.Summary = now, batch.Status
		return tx.Model(&batch).Updates(map[string]any{"status": batch.Status, "completed": batch.Completed, "failed": batch.Failed,
			"completed_at": batch.CompletedAt, "summary": batch.Summary}).Error
	})
	return &batch, err
}

// interruptQualityTaskTx is called together with parent lease expiration. No
// uncertain reservation is refunded and no record is made runnable again.
func interruptQualityTaskTx(tx *gorm.DB, taskID string) error {
	var batch QualityProbeBatch
	err := tx.Where("task_id = ?", taskID).First(&batch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil || batch.CompletedAt != 0 {
		return err
	}
	now := common.GetTimestamp()
	if err := tx.Model(&QualityProbeRun{}).Where("batch_id = ? AND status IN ?", batch.ID, []string{"running", "queued"}).Updates(map[string]any{
		"status": "interrupted", "error_code": "lease_expired", "completed_at": now, "budget_state": "unknown"}).Error; err != nil {
		return err
	}
	if err := tx.Model(&QualityProbeRun{}).Where("batch_id = ? AND render_status = ?", batch.ID, "pending").Update("render_status", "interrupted").Error; err != nil {
		return err
	}
	var completed, failed int64
	if err := tx.Model(&QualityProbeRun{}).Where("batch_id = ? AND status = ?", batch.ID, "completed").Count(&completed).Error; err != nil {
		return err
	}
	if err := tx.Model(&QualityProbeRun{}).Where("batch_id = ? AND status IN ?", batch.ID, []string{"failed", "interrupted"}).Count(&failed).Error; err != nil {
		return err
	}
	return tx.Model(&batch).Updates(map[string]any{"status": "interrupted", "completed_at": now, "summary": "lease_expired", "completed": completed, "failed": failed}).Error
}
