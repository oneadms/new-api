package model

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func qualityModelFixture(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	previous := DB
	DB = db
	t.Cleanup(func() { DB = previous; require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&QualitySettings{}, &ChannelQualityProfile{}, &QualityPromptVersion{}, &QualityProbeBatch{},
		&QualityProbeRun{}, &QualityProbeRunGroup{}, &QualityProbeDailyBudget{}, &QualityReviewAudit{}, &SystemTask{}, &SystemTaskLock{}))
}

func qualityModelBatch(t *testing.T, channelID int) (*QualityProbeBatch, *SystemTask, QualityProbeRun) {
	t.Helper()
	settings := DefaultQualitySettings()
	data, err := common.Marshal(settings)
	require.NoError(t, err)
	run := QualityProbeRun{ChannelID: channelID, PublicID: common.GetUUID(), Status: "queued", QualityResult: "not_evaluated",
		BudgetTokens: 100, Groups: []string{"standard", "member"}, GroupsJSON: `["standard","member"]`, BudgetState: "unreserved"}
	batch, err := CreateQualityBatch(&QualityProbeBatch{Trigger: "manual", Status: "queued", CreatedAt: time.Now().Unix(), SettingsJSON: string(data)}, []QualityProbeRun{run}, nil, "")
	require.NoError(t, err)
	task, err := GetSystemTaskByTaskID(batch.TaskID)
	require.NoError(t, err)
	require.NotNil(t, task)
	task, claimed, err := ClaimSystemTask(task.ID, task.Type, "owner", time.Now().Unix()+120)
	require.NoError(t, err)
	require.True(t, claimed)
	_, runs, err := GetQualityBatch(batch.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	return batch, task, runs[0]
}

func TestQualitySettingsDefaultDisabledAndRejectStaleVersion(t *testing.T) {
	qualityModelFixture(t)
	settings, err := GetQualitySettings()
	require.NoError(t, err)
	assert.False(t, settings.Enabled)
	assert.False(t, settings.StatusPageEnabled)
	assert.Empty(t, settings.PublicGroups)
	old := *settings
	settings.Enabled = true
	updated, err := SaveQualitySettings(*settings)
	require.NoError(t, err)
	assert.Equal(t, old.Version+1, updated.Version)
	_, err = SaveQualitySettings(old)
	require.ErrorIs(t, err, ErrQualityConflict)
	current, err := GetQualitySettings()
	require.NoError(t, err)
	assert.True(t, current.Enabled)
}

func TestQualityDuplicateChannelRollsBackBatchAndTaskTogether(t *testing.T) {
	qualityModelFixture(t)
	runs := []QualityProbeRun{{PublicID: "first", ChannelID: 4, Status: "queued"}, {PublicID: "second", ChannelID: 4, Status: "queued"}}
	_, err := CreateQualityBatch(&QualityProbeBatch{Status: "queued"}, runs, nil, "")
	require.Error(t, err)
	for _, entity := range []any{&QualityProbeBatch{}, &QualityProbeRun{}, &QualityProbeRunGroup{}, &SystemTask{}} {
		var count int64
		require.NoError(t, DB.Model(entity).Count(&count).Error)
		assert.Zero(t, count)
	}
}

func TestQualityConcurrentBudgetReservationIsAtomicAndIdempotent(t *testing.T) {
	qualityModelFixture(t)
	batch, _, _ := qualityModelBatch(t, 1)
	settings := DefaultQualitySettings()
	settings.DailyCallLimit, settings.DailyTokenLimit = 1, 100
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			results <- ReserveQualityBatchBudget(batch.ID, "owner", settings, "2026-10-26")
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	var budget QualityProbeDailyBudget
	require.NoError(t, DB.First(&budget).Error)
	assert.Equal(t, int64(1), budget.Calls)
	assert.Equal(t, int64(100), budget.Tokens)
	_, err := FinishQualityBatch(batch.ID, "owner", "interrupted")
	require.NoError(t, err)
}

func TestQualityBudgetRejectionDoesNotPartiallyReserve(t *testing.T) {
	qualityModelFixture(t)
	batch, _, run := qualityModelBatch(t, 1)
	settings := DefaultQualitySettings()
	settings.DailyTokenLimit = 99
	require.ErrorIs(t, ReserveQualityBatchBudget(batch.ID, "owner", settings, "2026-10-26"), ErrQualityBudget)
	var count int64
	require.NoError(t, DB.Model(&QualityProbeDailyBudget{}).Count(&count).Error)
	assert.Zero(t, count)
	stored, err := GetQualityRun(run.ID)
	require.NoError(t, err)
	assert.Equal(t, "unreserved", stored.BudgetState)
	_, err = ClaimQualityRun(run.ID, "owner")
	require.ErrorIs(t, err, ErrQualityConflict)
}

func TestQualityUnknownUsageKeepsReservationAndKnownUsageSettlesOriginalUTCDay(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		name := "known"
		if unknown {
			name = "unknown"
		}
		t.Run(name, func(t *testing.T) {
			qualityModelFixture(t)
			batch, _, queued := qualityModelBatch(t, 1)
			require.NoError(t, ReserveQualityBatchBudget(batch.ID, "owner", DefaultQualitySettings(), "2026-10-26"))
			run, err := ClaimQualityRun(queued.ID, "owner")
			require.NoError(t, err)
			run.RequestStarted, run.Status, run.CompletedAt = true, "completed", time.Now().Unix()
			if !unknown {
				input, output := int64(10), int64(20)
				run.InputTokens, run.OutputTokens = &input, &output
			}
			require.NoError(t, SaveQualityRun(run, "owner"))
			var budgets []QualityProbeDailyBudget
			require.NoError(t, DB.Find(&budgets).Error)
			require.Len(t, budgets, 1)
			assert.Equal(t, "2026-10-26", budgets[0].Date)
			assert.Equal(t, int64(1), budgets[0].Calls)
			if unknown {
				assert.Equal(t, int64(100), budgets[0].Tokens)
				assert.Equal(t, "unknown", run.BudgetState)
			} else {
				assert.Equal(t, int64(30), budgets[0].Tokens)
				assert.Equal(t, int64(30), budgets[0].SettledTokens)
				assert.Equal(t, "settled", run.BudgetState)
			}
			// Re-submitting terminal metadata cannot reconcile a second time.
			require.ErrorIs(t, SaveQualityRun(run, "owner"), ErrQualityConflict)
		})
	}
}

func TestQualityIncompleteOrInvalidUsageCannotRefundReservation(t *testing.T) {
	cases := []struct {
		name, status  string
		input, output int64
	}{
		{"partial_usage", "failed", 10, 20},
		{"negative_usage", "completed", -1, 20},
		{"oversized_usage", "completed", 1 << 40, 20},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			qualityModelFixture(t)
			batch, _, queued := qualityModelBatch(t, 1)
			require.NoError(t, ReserveQualityBatchBudget(batch.ID, "owner", DefaultQualitySettings(), "2026-10-26"))
			run, err := ClaimQualityRun(queued.ID, "owner")
			require.NoError(t, err)
			run.RequestStarted, run.Status, run.CompletedAt = true, tt.status, time.Now().Unix()
			run.InputTokens, run.OutputTokens = &tt.input, &tt.output
			require.NoError(t, SaveQualityRun(run, "owner"))
			var budget QualityProbeDailyBudget
			require.NoError(t, DB.First(&budget).Error)
			assert.Equal(t, int64(100), budget.Tokens)
			assert.Equal(t, "unknown", run.BudgetState)
			assert.Nil(t, run.InputTokens)
			assert.Nil(t, run.OutputTokens)
		})
	}
}

func TestQualityExpiredLeaseCannotWriteChildResultOrRefundReservation(t *testing.T) {
	qualityModelFixture(t)
	batch, task, queued := qualityModelBatch(t, 1)
	require.NoError(t, ReserveQualityBatchBudget(batch.ID, "owner", DefaultQualitySettings(), "2026-10-26"))
	run, err := ClaimQualityRun(queued.ID, "owner")
	require.NoError(t, err)
	require.NoError(t, DB.Model(&SystemTaskLock{}).Where("task_id = ?", task.TaskID).Update("locked_until", time.Now().Unix()-1).Error)
	zero := int64(0)
	run.Status, run.RequestStarted, run.InputTokens, run.OutputTokens = "completed", true, &zero, &zero
	run.HasOutput, run.RawKey = true, "late-output"
	require.ErrorIs(t, SaveQualityRun(run, "owner"), ErrSystemTaskLockLost)
	require.NoError(t, ExpireStaleSystemTaskLocks(time.Now().Unix()))
	stored, err := GetQualityRun(run.ID)
	require.NoError(t, err)
	assert.Equal(t, "interrupted", stored.Status)
	assert.Empty(t, stored.RawKey)
	assert.False(t, stored.HasOutput)
	var budget QualityProbeDailyBudget
	require.NoError(t, DB.First(&budget).Error)
	assert.Equal(t, int64(100), budget.Tokens)
	_, err = ClaimQualityRun(run.ID, "owner")
	require.ErrorIs(t, err, ErrSystemTaskLockLost)
}

func TestQualityPromptVersionsPreservePreviousContent(t *testing.T) {
	qualityModelFixture(t)
	first, err := CreateQualityPrompt(QualityPromptVersion{Key: "custom-example", Name: "first", Content: "original", Rule: "visual"})
	require.NoError(t, err)
	second, err := CreateQualityPrompt(QualityPromptVersion{Key: "custom-example", Name: "second", Content: "replacement", Rule: "visual"})
	require.NoError(t, err)
	assert.Equal(t, 2, second.Version)
	var stored QualityPromptVersion
	require.NoError(t, DB.First(&stored, first.ID).Error)
	assert.Equal(t, "original", stored.Content)
}

func TestQualityRetentionProtectsLatestAndBaselineReferences(t *testing.T) {
	qualityModelFixture(t)
	old := time.Now().Add(-30 * 24 * time.Hour).Unix()
	batch := QualityProbeBatch{TaskID: "finished", Status: "completed", CompletedAt: old}
	require.NoError(t, DB.Create(&batch).Error)
	runs := []QualityProbeRun{
		{BatchID: batch.ID, ChannelID: 1, PublicID: "baseline", ConfigFingerprint: "same", Status: "completed", CompletedAt: old, HasOutput: true, HasHTML: true, RawKey: "baseline.raw", HTMLKey: "baseline.html"},
		{BatchID: batch.ID + 1, ChannelID: 1, PublicID: "expired", ConfigFingerprint: "same", Status: "completed", CompletedAt: old, HasOutput: true, RawKey: "expired.raw"},
		{BatchID: batch.ID + 2, ChannelID: 1, PublicID: "latest", ConfigFingerprint: "same", Status: "completed", CompletedAt: old, HasOutput: true, RawKey: "latest.raw"},
	}
	for i := 1; i <= 2; i++ {
		require.NoError(t, DB.Create(&QualityProbeBatch{ID: batch.ID + int64(i), TaskID: common.GetUUID(), Status: "completed", CompletedAt: old}).Error)
	}
	require.NoError(t, DB.Create(&runs).Error)
	require.NoError(t, DB.Create(&ChannelQualityProfile{ChannelID: 1, BaselineID: runs[0].ID}).Error)
	keys, err := PruneQualityArtifacts(context.Background(), time.Now().Add(-14*24*time.Hour).Unix(), time.Now().Add(-90*24*time.Hour).Unix())
	require.NoError(t, err)
	assert.Contains(t, keys, "expired.raw")
	assert.NotContains(t, keys, "baseline.raw")
	assert.NotContains(t, keys, "latest.raw")
	baseline, err := GetQualityRun(runs[0].ID)
	require.NoError(t, err)
	assert.True(t, baseline.HasHTML)
	expired, err := GetQualityRun(runs[1].ID)
	require.NoError(t, err)
	assert.False(t, expired.HasOutput)
}
