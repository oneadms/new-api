package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualityQueuedProfileChangesSkipDispatchAndReleaseReservation(t *testing.T) {
	for _, change := range []string{"disabled", "model", "prompt", "protocol", "reasoning", "profile_timeout", "global_timeout", "output_limit"} {
		t.Run(change, func(t *testing.T) {
			calls := 0
			qualityServiceFixture(t, func(_ context.Context, _ QualityExecutionRequest, emit func(string) error) (QualityExecutionResult, error) {
				calls++
				return QualityExecutionResult{Started: true, Complete: true}, emit("<html><body>unexpected generation</body></html>")
			})
			qualityServiceChannel(t, 1, "standard")
			batch, err := CreateQualityBatch(QualityBatchRequest{}, "queued-config-change", 1)
			require.NoError(t, err)
			_, frozen, err := model.GetQualityBatch(batch.ID)
			require.NoError(t, err)
			require.Len(t, frozen, 1)

			profile, err := model.GetQualityProfile(1)
			require.NoError(t, err)
			switch change {
			case "disabled":
				profile.Enabled = false
			case "model":
				require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 1).Update("models", "text-model,other-model").Error)
				profile.Model = "other-model"
			case "prompt":
				prompt, err := CreateQualityPrompt("replacement", "Replacement", "Produce a different HTML page.", "visual", 1)
				require.NoError(t, err)
				profile.PromptVersionID = prompt.ID
			case "protocol":
				profile.Protocol = "responses"
			case "reasoning":
				profile.ReasoningEffort = "high"
			case "profile_timeout":
				profile.TimeoutSeconds = 120
			case "global_timeout", "output_limit":
				settings, err := GetQualitySettings()
				require.NoError(t, err)
				if change == "global_timeout" {
					settings.TimeoutSeconds = 120
				} else {
					settings.MaxOutputTokens++
				}
				_, err = UpdateQualitySettings(*settings)
				require.NoError(t, err)
			}
			if change != "global_timeout" && change != "output_limit" {
				_, err = UpdateQualityChannel(1, *profile)
				require.NoError(t, err)
			}

			QualityDetectionHandler{}.Run(context.Background(), qualityServiceClaim(t, batch), "quality-test-owner")
			_, runs, err := model.GetQualityBatch(batch.ID)
			require.NoError(t, err)
			require.Len(t, runs, 1)
			assert.Zero(t, calls)
			assert.Equal(t, "skipped", runs[0].Status)
			assert.Equal(t, "configuration_changed", runs[0].ErrorCode)
			assert.Equal(t, "released", runs[0].BudgetState)
			assert.False(t, runs[0].RequestStarted)
			assert.Equal(t, frozen[0].Model, runs[0].Model)
			assert.Equal(t, frozen[0].PromptSnapshot, runs[0].PromptSnapshot)
			var budget model.QualityProbeDailyBudget
			require.NoError(t, model.DB.First(&budget).Error)
			assert.Zero(t, budget.Calls)
			assert.Zero(t, budget.Tokens)
		})
	}
}

func TestQualityPublicationChangesKeepFrozenExecutionValid(t *testing.T) {
	var requests []QualityExecutionRequest
	qualityServiceFixture(t, func(_ context.Context, request QualityExecutionRequest, emit func(string) error) (QualityExecutionResult, error) {
		requests = append(requests, request)
		return QualityExecutionResult{Started: true, Complete: true}, emit("<html><body>frozen request</body></html>")
	})
	qualityServiceChannel(t, 1, "standard")
	batch, err := CreateQualityBatch(QualityBatchRequest{}, "publication-only", 1)
	require.NoError(t, err)
	_, frozen, err := model.GetQualityBatch(batch.ID)
	require.NoError(t, err)
	require.Len(t, frozen, 1)
	profile, err := model.GetQualityProfile(1)
	require.NoError(t, err)
	profile.PublicAlias, profile.PublishSample = "Public sample", true
	_, err = UpdateQualityChannel(1, *profile)
	require.NoError(t, err)
	settings, err := GetQualitySettings()
	require.NoError(t, err)
	settings.StatusPageEnabled = true
	settings.PublicGroups = []model.QualityPublicGroup{{Name: "standard", Alias: "Published group"}}
	_, err = UpdateQualitySettings(*settings)
	require.NoError(t, err)

	QualityDetectionHandler{}.Run(context.Background(), qualityServiceClaim(t, batch), "quality-test-owner")
	require.Len(t, requests, 1)
	assert.Equal(t, frozen[0].Model, requests[0].Model)
	assert.Equal(t, frozen[0].PromptSnapshot, requests[0].Prompt)
	assert.Equal(t, frozen[0].MaxOutputTokens, requests[0].MaxOutputTokens)
	_, runs, err := model.GetQualityBatch(batch.ID)
	require.NoError(t, err)
	assert.Equal(t, "completed", runs[0].Status)
}

func TestQualityBudgetAdmissionReservesProtocolFramingBeforeGeneration(t *testing.T) {
	for _, framingBudget := range []int64{0, 512} {
		name := "prompt_and_output_only"
		if framingBudget > 0 {
			name = "includes_protocol_framing"
		}
		t.Run(name, func(t *testing.T) {
			calls := 0
			qualityServiceFixture(t, func(_ context.Context, _ QualityExecutionRequest, emit func(string) error) (QualityExecutionResult, error) {
				calls++
				return QualityExecutionResult{Started: true, Complete: true}, emit("<html><body>budgeted</body></html>")
			})
			qualityServiceChannel(t, 1, "standard")
			prompts, err := ListQualityPrompts()
			require.NoError(t, err)
			settings, err := GetQualitySettings()
			require.NoError(t, err)
			settings.DailyTokenLimit = int64(len(QualityHTMLInstruction)+len(prompts[0].Content)+settings.MaxOutputTokens) + framingBudget
			_, err = UpdateQualitySettings(*settings)
			require.NoError(t, err)
			batch, err := CreateQualityBatch(QualityBatchRequest{}, "framing-budget", 1)
			require.NoError(t, err)
			QualityDetectionHandler{}.Run(context.Background(), qualityServiceClaim(t, batch), "quality-test-owner")
			finished, runs, err := model.GetQualityBatch(batch.ID)
			require.NoError(t, err)
			require.Len(t, runs, 1)
			if framingBudget == 0 {
				assert.Zero(t, calls)
				assert.Equal(t, "budget_blocked", finished.Status)
				assert.Equal(t, "skipped", runs[0].Status)
				var count int64
				require.NoError(t, model.DB.Model(&model.QualityProbeDailyBudget{}).Count(&count).Error)
				assert.Zero(t, count)
				return
			}
			assert.Equal(t, 1, calls)
			assert.Equal(t, "completed", finished.Status)
			var budget model.QualityProbeDailyBudget
			require.NoError(t, model.DB.First(&budget).Error)
			assert.Equal(t, int64(1), budget.Calls)
			assert.Equal(t, settings.DailyTokenLimit, budget.Tokens)
		})
	}
}

func TestQualitySettingsUseSharedGenerationTokenBoundary(t *testing.T) {
	qualityServiceFixture(t, nil)
	settings, err := GetQualitySettings()
	require.NoError(t, err)
	settings.MaxOutputTokens = common.MaxTokensLimit + 1
	_, err = UpdateQualitySettings(*settings)
	var invalid *QualityValidationError
	require.ErrorAs(t, err, &invalid)
	settings.MaxOutputTokens = common.MaxTokensLimit
	updated, err := UpdateQualitySettings(*settings)
	require.NoError(t, err)
	assert.Equal(t, common.MaxTokensLimit, updated.MaxOutputTokens)
}
