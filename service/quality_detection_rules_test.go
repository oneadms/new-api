package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualityInteractiveElementsStillRequireRealBrowserVerification(t *testing.T) {
	qualityServiceFixture(t, func(_ context.Context, _ QualityExecutionRequest, emit func(string) error) (QualityExecutionResult, error) {
		return QualityExecutionResult{Started: true, Complete: true}, emit(`<html><body><button id="toggle-button">Toggle message</button><p id="message">Ready</p></body></html>`)
	})
	qualityServiceChannel(t, 1, "standard")
	prompts, err := ListQualityPrompts()
	require.NoError(t, err)
	profile, err := model.GetQualityProfile(1)
	require.NoError(t, err)
	for _, prompt := range prompts {
		if prompt.Rule == "interactive" {
			profile.PromptVersionID = prompt.ID
		}
	}
	_, err = UpdateQualityChannel(1, *profile)
	require.NoError(t, err)
	batch, err := CreateQualityBatch(QualityBatchRequest{}, "interactive-unverified", 1)
	require.NoError(t, err)
	QualityDetectionHandler{}.Run(context.Background(), qualityServiceClaim(t, batch), "quality-test-owner")
	_, runs, err := model.GetQualityBatch(batch.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, "completed", runs[0].Status)
	assert.Equal(t, "needs_review", runs[0].QualityResult)
	var evidence map[string]any
	require.NoError(t, common.UnmarshalJsonStr(runs[0].RuleResults, &evidence))
	assert.Equal(t, true, evidence["required_elements_present"])
	assert.Equal(t, false, evidence["browser_interaction_verified"])
}

func TestQualityExternalResourcesAreRecordedWithoutChangingGenerationSuccess(t *testing.T) {
	run := &model.QualityProbeRun{Status: "completed", Rule: "visual", RuleVersion: "visual-v1"}
	evidenceJSON := qualityRuleEvidence(run, []byte(`<html><head><link href="https://example.invalid/style.css" rel="stylesheet"></head></html>`))
	var evidence map[string]any
	require.NoError(t, common.UnmarshalJsonStr(evidenceJSON, &evidence))
	assert.Equal(t, true, evidence["generation_complete"])
	assert.Equal(t, true, evidence["external_resource_references"])
	assert.NotContains(t, evidenceJSON, "example.invalid")
}

func TestQualityBaselineRequiresAcceptedReviewAndPreservesImmutableOutput(t *testing.T) {
	qualityServiceFixture(t, func(_ context.Context, _ QualityExecutionRequest, emit func(string) error) (QualityExecutionResult, error) {
		return QualityExecutionResult{Started: true, Complete: true}, emit("<html><body>baseline</body></html>")
	})
	qualityServiceChannel(t, 1, "standard")
	batch, err := CreateQualityBatch(QualityBatchRequest{}, "baseline-review", 1)
	require.NoError(t, err)
	QualityDetectionHandler{}.Run(context.Background(), qualityServiceClaim(t, batch), "quality-test-owner")
	_, runs, err := model.GetQualityBatch(batch.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	run := runs[0]
	require.ErrorIs(t, SetQualityBaseline(run.ID, 7), model.ErrQualityConflict)
	require.NoError(t, ReviewQualityRun(run.ID, 7, "accepted", "Confirmed illustration"))
	require.NoError(t, SetQualityBaseline(run.ID, 7))
	profile, err := model.GetQualityProfile(1)
	require.NoError(t, err)
	assert.Equal(t, run.ID, profile.BaselineID)
	stored, err := GetQualityRun(run.ID)
	require.NoError(t, err)
	assert.Equal(t, run.RawKey, stored.RawKey)
	assert.Equal(t, run.HTMLKey, stored.HTMLKey)
	assert.Equal(t, "needs_review", stored.QualityResult)
	assert.Equal(t, "accepted", stored.ReviewStatus)
	var audits []model.QualityReviewAudit
	require.NoError(t, model.DB.Where("run_id = ?", run.ID).Order("id").Find(&audits).Error)
	require.Len(t, audits, 2)
	assert.Equal(t, 7, audits[0].ActorID)
	assert.Equal(t, "accepted", audits[0].Action)
	assert.Equal(t, "baseline", audits[1].Action)
	settings, err := GetQualitySettings()
	require.NoError(t, err)
	settings.MaxOutputTokens++
	_, err = UpdateQualitySettings(*settings)
	require.NoError(t, err)
	require.ErrorIs(t, SetQualityBaseline(run.ID, 7), model.ErrQualityConflict)
}

func TestQualityUnconfiguredSupportedChannelCanBeConfiguredWithoutInventingModel(t *testing.T) {
	qualityServiceFixture(t, nil)
	require.NoError(t, model.DB.Create(&model.Channel{Id: 1, Name: "new", Type: 1, Key: "secret", Models: "text-model", Group: "standard", Status: common.ChannelStatusEnabled}).Error)
	channels, err := ListQualityChannels()
	require.NoError(t, err)
	require.Len(t, channels, 1)
	assert.True(t, channels[0].Supported)
	assert.Empty(t, channels[0].Profile.Model)
	assert.False(t, channels[0].Profile.Enabled)
}
