package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type qualityTestExecutor struct {
	execute func(context.Context, QualityExecutionRequest, func(string) error) (QualityExecutionResult, error)
}

func (e qualityTestExecutor) Supports(int, string) bool { return true }
func (e qualityTestExecutor) Execute(ctx context.Context, request QualityExecutionRequest, emit func(string) error) (QualityExecutionResult, error) {
	return e.execute(ctx, request, emit)
}

func qualityServiceFixture(t *testing.T, execute func(context.Context, QualityExecutionRequest, func(string) error) (QualityExecutionResult, error)) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	savedDB := model.DB
	savedExecutor, _ := GetQualityExecutor()
	mainType, logType := common.MainDatabaseType(), common.LogDatabaseType()
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	RegisterQualityExecutor(qualityTestExecutor{execute: execute})
	t.Setenv("QUALITY_ARTIFACT_DIR", t.TempDir())
	t.Setenv("QUALITY_RENDERER_URL", "")
	t.Setenv("QUALITY_RENDERER_TOKEN", "")
	t.Cleanup(func() {
		model.DB = savedDB
		RegisterQualityExecutor(savedExecutor)
		common.SetDatabaseTypes(mainType, logType)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.QualitySettings{}, &model.ChannelQualityProfile{}, &model.QualityPromptVersion{},
		&model.QualityProbeBatch{}, &model.QualityProbeRun{}, &model.QualityProbeRunGroup{}, &model.QualityProbeDailyBudget{}, &model.QualityReviewAudit{},
		&model.SystemTask{}, &model.SystemTaskLock{}))
}

func qualityServiceChannel(t *testing.T, id int, groups string) {
	t.Helper()
	channel := model.Channel{Id: id, Name: "private-channel", Type: 1, Key: "private-api-key", Models: "text-model", Group: groups, Status: common.ChannelStatusEnabled}
	require.NoError(t, model.DB.Create(&channel).Error)
	prompts, err := ListQualityPrompts()
	require.NoError(t, err)
	_, err = UpdateQualityChannel(id, model.ChannelQualityProfile{ChannelID: id, Enabled: true, Model: "text-model", Protocol: "chat", PromptVersionID: prompts[0].ID})
	require.NoError(t, err)
}

func qualityServiceClaim(t *testing.T, batch *model.QualityProbeBatch) *model.SystemTask {
	t.Helper()
	task, err := model.GetSystemTaskByTaskID(batch.TaskID)
	require.NoError(t, err)
	require.NotNil(t, task)
	claimed, ok, err := model.ClaimSystemTask(task.ID, task.Type, "quality-test-owner", time.Now().Unix()+120)
	require.NoError(t, err)
	require.True(t, ok)
	return claimed
}

func TestQualityBatchDeduplicatesGroupsAndReusesCanonicalIdempotentRequest(t *testing.T) {
	var mu sync.Mutex
	calls := map[int]int{}
	qualityServiceFixture(t, func(_ context.Context, request QualityExecutionRequest, emit func(string) error) (QualityExecutionResult, error) {
		mu.Lock()
		calls[request.ChannelID]++
		mu.Unlock()
		err := emit("<!doctype html><html><body><svg></svg></body></html>")
		return QualityExecutionResult{Started: true, Complete: true, GenerationMS: 100}, err
	})
	qualityServiceChannel(t, 1, "standard,member,premium")
	qualityServiceChannel(t, 2, "member")
	batch, err := CreateQualityBatch(QualityBatchRequest{ChannelIDs: []int{1, 1}, GroupNames: []string{"member", "standard", "member"}}, "request-A", 1)
	require.NoError(t, err)
	require.Equal(t, 2, batch.Total)
	repeated, err := CreateQualityBatch(QualityBatchRequest{ChannelIDs: []int{1}, GroupNames: []string{"standard", "member"}}, "request-A", 1)
	require.NoError(t, err)
	assert.Equal(t, batch.ID, repeated.ID)
	_, err = CreateQualityBatch(QualityBatchRequest{ChannelIDs: []int{2}}, "request-A", 1)
	require.ErrorIs(t, err, model.ErrQualityIdempotency)
	active, err := CreateQualityBatch(QualityBatchRequest{ChannelIDs: []int{2}}, "request-B", 1)
	require.ErrorIs(t, err, model.ErrQualityConflict)
	require.NotNil(t, active)
	assert.Equal(t, batch.ID, active.ID)
	task := qualityServiceClaim(t, batch)
	QualityDetectionHandler{}.Run(context.Background(), task, "quality-test-owner")
	finished, runs, err := model.GetQualityBatch(batch.ID)
	require.NoError(t, err)
	assert.Equal(t, "completed", finished.Status)
	assert.Equal(t, 2, finished.Completed)
	assert.Equal(t, map[int]int{1: 1, 2: 1}, calls)
	var links []model.QualityProbeRunGroup
	require.NoError(t, model.DB.Find(&links).Error)
	assert.Len(t, links, 4)
	for _, run := range runs {
		assert.Equal(t, "needs_review", run.QualityResult)
		assert.Equal(t, "unavailable", run.RenderStatus)
		assert.Nil(t, run.InputTokens)
		assert.Nil(t, run.OutputTokens)
		assert.Equal(t, "unknown", run.BudgetState)
		assert.True(t, run.HasOutput)
		assert.True(t, run.HasHTML)
		data, err := ReadQualityArtifact(&run, "html")
		require.NoError(t, err)
		assert.Equal(t, "<!doctype html><html><body><svg></svg></body></html>", string(data))
		jsonData, err := common.Marshal(run)
		require.NoError(t, err)
		for _, secret := range []string{"private-api-key", "<html>", run.RawKey, run.HTMLKey, "prompt_snapshot", "groups_json", "config_fingerprint"} {
			assert.NotContains(t, string(jsonData), secret)
		}
	}
	var budget model.QualityProbeDailyBudget
	require.NoError(t, model.DB.First(&budget).Error)
	assert.Equal(t, int64(2), budget.Calls)
	assert.Positive(t, budget.Tokens)
	assert.Zero(t, budget.SettledCalls)
	channel, err := model.GetChannelById(1, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusEnabled, channel.Status)
	assert.Equal(t, "private-api-key", channel.Key)
}

func TestQualityCancellationBeforeExecutionMakesNoCallsOrReservations(t *testing.T) {
	calls := 0
	qualityServiceFixture(t, func(context.Context, QualityExecutionRequest, func(string) error) (QualityExecutionResult, error) {
		calls++
		return QualityExecutionResult{}, nil
	})
	qualityServiceChannel(t, 1, "standard")
	batch, err := CreateQualityBatch(QualityBatchRequest{}, "cancel-before", 1)
	require.NoError(t, err)
	require.NoError(t, model.CancelQualityBatch(batch.ID))
	require.NoError(t, model.CancelQualityBatch(batch.ID))
	QualityDetectionHandler{}.Run(context.Background(), qualityServiceClaim(t, batch), "quality-test-owner")
	finished, runs, err := model.GetQualityBatch(batch.ID)
	require.NoError(t, err)
	assert.Zero(t, calls)
	assert.Equal(t, "cancelled", finished.Status)
	require.Len(t, runs, 1)
	assert.Equal(t, "cancelled", runs[0].Status)
	var count int64
	require.NoError(t, model.DB.Model(&model.QualityProbeDailyBudget{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestQualityLeaseLossFencesOutputAndRestartNeverReplaysGeneration(t *testing.T) {
	calls := 0
	qualityServiceFixture(t, func(_ context.Context, _ QualityExecutionRequest, emit func(string) error) (QualityExecutionResult, error) {
		calls++
		err := emit("<html><body>one generation</body></html>")
		if err != nil {
			return QualityExecutionResult{Started: true}, err
		}
		if err := model.DB.Where("type = ?", model.SystemTaskTypeQualityDetection).Delete(&model.SystemTaskLock{}).Error; err != nil {
			return QualityExecutionResult{Started: true}, err
		}
		return QualityExecutionResult{Started: true, Complete: true}, nil
	})
	qualityServiceChannel(t, 1, "standard")
	batch, err := CreateQualityBatch(QualityBatchRequest{}, "lost-lease", 1)
	require.NoError(t, err)
	task := qualityServiceClaim(t, batch)
	QualityDetectionHandler{}.Run(context.Background(), task, "quality-test-owner")
	_, runs, err := model.GetQualityBatch(batch.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, "running", runs[0].Status)
	assert.False(t, runs[0].HasOutput)
	assert.Empty(t, runs[0].RawKey)
	require.NoError(t, model.ExpireStaleSystemTaskLocks(time.Now().Unix()))
	finished, runs, err := model.GetQualityBatch(batch.ID)
	require.NoError(t, err)
	assert.Equal(t, "interrupted", finished.Status)
	assert.Equal(t, "interrupted", runs[0].Status)
	QualityDetectionHandler{}.Run(context.Background(), task, "quality-test-owner")
	assert.Equal(t, 1, calls)
	var budget model.QualityProbeDailyBudget
	require.NoError(t, model.DB.First(&budget).Error)
	assert.Equal(t, int64(1), budget.Calls)
	assert.Positive(t, budget.Tokens)
}

func TestQualityOutputLimitCannotBeOverriddenByExecutorCompletion(t *testing.T) {
	calls := 0
	qualityServiceFixture(t, func(_ context.Context, _ QualityExecutionRequest, emit func(string) error) (QualityExecutionResult, error) {
		calls++
		_ = emit(strings.Repeat("x", QualityOutputLimit+1))
		return QualityExecutionResult{Started: true, Complete: true}, nil
	})
	qualityServiceChannel(t, 1, "standard")
	batch, err := CreateQualityBatch(QualityBatchRequest{}, "oversized-output", 1)
	require.NoError(t, err)
	QualityDetectionHandler{}.Run(context.Background(), qualityServiceClaim(t, batch), "quality-test-owner")
	_, runs, err := model.GetQualityBatch(batch.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, 1, calls)
	assert.Equal(t, "failed", runs[0].Status)
	assert.Equal(t, "output_limit", runs[0].ErrorCode)
	assert.Equal(t, "unknown", runs[0].BudgetState)
	assert.False(t, runs[0].HasOutput)
}

func TestQualityPersistedCancellationCancelsTheRunningRequest(t *testing.T) {
	var batchID int64
	calls := 0
	qualityServiceFixture(t, func(ctx context.Context, _ QualityExecutionRequest, _ func(string) error) (QualityExecutionResult, error) {
		calls++
		if err := model.CancelQualityBatch(batchID); err != nil {
			return QualityExecutionResult{Started: true}, err
		}
		<-ctx.Done()
		return QualityExecutionResult{Started: true}, ctx.Err()
	})
	qualityServiceChannel(t, 1, "standard")
	qualityServiceChannel(t, 2, "member")
	settings, err := GetQualitySettings()
	require.NoError(t, err)
	settings.Concurrency = 1
	_, err = UpdateQualitySettings(*settings)
	require.NoError(t, err)
	batch, err := CreateQualityBatch(QualityBatchRequest{}, "cancel-running", 1)
	require.NoError(t, err)
	batchID = batch.ID
	QualityDetectionHandler{}.Run(context.Background(), qualityServiceClaim(t, batch), "quality-test-owner")
	finished, runs, err := model.GetQualityBatch(batch.ID)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", finished.Status)
	require.Len(t, runs, 2)
	assert.Equal(t, 1, calls)
	assert.Equal(t, "cancelled", runs[0].Status)
	assert.Equal(t, "unknown", runs[0].BudgetState)
	assert.Equal(t, "cancelled", runs[1].Status)
	assert.Equal(t, "released", runs[1].BudgetState)
	var budget model.QualityProbeDailyBudget
	require.NoError(t, model.DB.First(&budget).Error)
	assert.Equal(t, int64(1), budget.Calls)
	assert.Equal(t, runs[0].BudgetTokens, budget.Tokens)
}

func TestQualityScheduledInitializationRequiresLeaseAndReusesFrozenBatch(t *testing.T) {
	calls := 0
	qualityServiceFixture(t, func(_ context.Context, _ QualityExecutionRequest, emit func(string) error) (QualityExecutionResult, error) {
		calls++
		return QualityExecutionResult{Started: true, Complete: true}, emit("<html><body>ok</body></html>")
	})
	qualityServiceChannel(t, 1, "standard")
	settings, err := GetQualitySettings()
	require.NoError(t, err)
	settings.Enabled = true
	_, err = UpdateQualitySettings(*settings)
	require.NoError(t, err)
	task, err := model.CreateSystemTask(model.SystemTaskTypeQualityDetection, nil, nil)
	require.NoError(t, err)
	QualityDetectionHandler{}.Run(context.Background(), task, "not-owner")
	batch, err := model.FindQualityBatchByTask(task.TaskID)
	require.NoError(t, err)
	assert.Nil(t, batch)
	claimed, ok, err := model.ClaimSystemTask(task.ID, task.Type, "quality-test-owner", time.Now().Unix()+120)
	require.NoError(t, err)
	require.True(t, ok)
	QualityDetectionHandler{}.Run(context.Background(), claimed, "quality-test-owner")
	batch, err = model.FindQualityBatchByTask(task.TaskID)
	require.NoError(t, err)
	require.NotNil(t, batch)
	assert.Equal(t, "scheduled", batch.Trigger)
	assert.Equal(t, "completed", batch.Status)
	QualityDetectionHandler{}.Run(context.Background(), claimed, "quality-test-owner")
	assert.Equal(t, 1, calls)
}

func TestQualityRawErrorsRemainPrivateAndNeverChangeChannelState(t *testing.T) {
	qualityServiceFixture(t, func(context.Context, QualityExecutionRequest, func(string) error) (QualityExecutionResult, error) {
		return QualityExecutionResult{Started: true, ErrorCode: "secret upstream response"}, errors.New("https://private:api-key@internal.example secret error")
	})
	qualityServiceChannel(t, 1, "standard")
	batch, err := CreateQualityBatch(QualityBatchRequest{}, "failed-generation", 1)
	require.NoError(t, err)
	QualityDetectionHandler{}.Run(context.Background(), qualityServiceClaim(t, batch), "quality-test-owner")
	_, runs, err := model.GetQualityBatch(batch.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, "failed", runs[0].Status)
	assert.Equal(t, "stream_incomplete", runs[0].ErrorCode)
	data, err := common.Marshal(runs[0])
	require.NoError(t, err)
	assert.NotContains(t, string(data), "secret")
	assert.NotContains(t, string(data), "internal.example")
	channel, err := model.GetChannelById(1, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusEnabled, channel.Status)
}
