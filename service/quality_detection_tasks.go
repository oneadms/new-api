package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// qualityProtocolFramingTokens reserves a conservative allowance for message
// roles, boundaries, and protocol framing beyond the frozen prompt bytes.
const qualityProtocolFramingTokens = 512

type QualityBatchRequest struct {
	ChannelIDs []int    `json:"channel_ids"`
	GroupNames []string `json:"group_names"`
}

func CreateQualityBatch(request QualityBatchRequest, idempotencyKey string, actor int) (*model.QualityProbeBatch, error) {
	if len(idempotencyKey) < 1 || len(idempotencyKey) > 128 || len(request.ChannelIDs) > 5000 || len(request.GroupNames) > 500 {
		return nil, &QualityValidationError{"A valid Idempotency-Key and bounded target selection are required"}
	}
	for _, char := range idempotencyKey {
		if char < 33 || char > 126 {
			return nil, &QualityValidationError{"Invalid Idempotency-Key"}
		}
	}
	for _, id := range request.ChannelIDs {
		if id <= 0 {
			return nil, &QualityValidationError{"Invalid channel selection"}
		}
	}
	for _, group := range request.GroupNames {
		if group == "" || len(group) > 128 || group == "auto" {
			return nil, &QualityValidationError{"Invalid group selection"}
		}
	}
	request.ChannelIDs = append([]int{}, request.ChannelIDs...)
	request.GroupNames = append([]string{}, request.GroupNames...)
	slices.Sort(request.ChannelIDs)
	request.ChannelIDs = slices.Compact(request.ChannelIDs)
	slices.Sort(request.GroupNames)
	request.GroupNames = slices.Compact(request.GroupNames)
	data, err := common.Marshal(request)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(data)
	fingerprint := hex.EncodeToString(digest[:])
	// Hash keys so case-insensitive MySQL collations do not conflate distinct
	// client keys, and request headers are never copied to task records.
	keyDigest := sha256.Sum256([]byte(idempotencyKey))
	key := hex.EncodeToString(keyDigest[:])
	if batch, err := model.FindQualityBatchByKey(key, fingerprint); err != nil || batch != nil {
		return batch, err
	}
	settings, err := GetQualitySettings()
	if err != nil {
		return nil, err
	}
	runs, err := freezeQualityTargets(request, settings)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, &QualityValidationError{"No enabled quality detection targets match this selection"}
	}
	settingsJSON, err := common.Marshal(settings)
	if err != nil {
		return nil, err
	}
	batch := &model.QualityProbeBatch{Trigger: "manual", Status: "queued", CreatedAt: time.Now().Unix(), CreatedBy: actor,
		IdempotencyKey: &key, RequestFingerprint: fingerprint, ConfigVersion: settings.Version, SettingsJSON: string(settingsJSON)}
	batch, err = model.CreateQualityBatch(batch, runs, nil, "")
	if err == nil {
		notifySystemTaskRunner()
	}
	return batch, err
}

func freezeQualityTargets(request QualityBatchRequest, settings *model.QualitySettings) ([]model.QualityProbeRun, error) {
	profiles, err := model.ListQualityProfiles()
	if err != nil {
		return nil, err
	}
	channels, err := model.GetAllChannels(0, -1, false, true)
	if err != nil {
		return nil, err
	}
	byID := map[int]*model.Channel{}
	for _, channel := range channels {
		byID[channel.Id] = channel
	}
	prompts, err := model.ListQualityPrompts()
	if err != nil {
		return nil, err
	}
	byPrompt := map[int64]model.QualityPromptVersion{}
	for _, prompt := range prompts {
		byPrompt[prompt.ID] = prompt
	}
	executor, _ := GetQualityExecutor()
	runs := []model.QualityProbeRun{}
	for _, profile := range profiles {
		channel := byID[profile.ChannelID]
		if !profile.Enabled || channel == nil {
			continue
		}
		groups := []string{}
		for _, group := range channel.GetGroups() {
			if group != "" && group != "auto" {
				groups = append(groups, group)
			}
		}
		slices.Sort(groups)
		groups = slices.Compact(groups)
		selected := len(request.ChannelIDs) == 0 && len(request.GroupNames) == 0
		if slices.Contains(request.ChannelIDs, channel.Id) {
			selected = true
		}
		for _, group := range groups {
			if slices.Contains(request.GroupNames, group) {
				selected = true
			}
		}
		if !selected {
			continue
		}
		prompt := byPrompt[profile.PromptVersionID]
		fingerprint, err := QualityConfigFingerprint(channel, profile, settings)
		if err != nil {
			return nil, err
		}
		groupsJSON, err := common.Marshal(groups)
		if err != nil {
			return nil, err
		}
		run := model.QualityProbeRun{PublicID: common.GetUUID(), ChannelID: channel.Id, ChannelName: channel.Name,
			Model: profile.Model, PromptVersionID: prompt.ID, PromptName: prompt.Name, Status: "queued", QualityResult: "not_evaluated",
			CreatedAt: time.Now().Unix(), ConfigFingerprint: fingerprint, ProfileVersion: profile.Version, Groups: groups, GroupsJSON: string(groupsJSON),
			PromptSnapshot: QualityHTMLInstruction + prompt.Content, Protocol: profile.Protocol, ReasoningEffort: profile.ReasoningEffort,
			MaxOutputTokens: settings.MaxOutputTokens, TimeoutSeconds: profile.TimeoutSeconds, Rule: prompt.Rule, RuleVersion: prompt.RuleVersion,
			RenderStatus: "not_started", BudgetState: "unreserved", BaselineID: profile.BaselineID}
		if run.TimeoutSeconds == 0 {
			run.TimeoutSeconds = settings.TimeoutSeconds
		}
		// One UTF-8 byte per prompt token plus protocol framing is a conservative
		// reservation; trustworthy upstream usage reconciles it after execution.
		run.BudgetTokens = int64(len(run.PromptSnapshot)) + int64(run.MaxOutputTokens) + qualityProtocolFramingTokens
		reason := qualityChannelUnsupportedReason(channel, profile, executor)
		if prompt.ID == 0 || prompt.Archived {
			reason = "prompt_unavailable"
		}
		if reason != "" {
			run.Status, run.ErrorCode, run.CompletedAt = "skipped", reason, time.Now().Unix()
		}
		runs = append(runs, run)
	}
	return runs, nil
}

// QualityDetectionHandler is registered by the application after injecting the
// executor. Scheduled initialization occurs inside Run under the claimed lease.
type QualityDetectionHandler struct{}

var _ ScheduledSystemTaskHandler = QualityDetectionHandler{}

func (QualityDetectionHandler) Type() string    { return model.SystemTaskTypeQualityDetection }
func (QualityDetectionHandler) NewPayload() any { return nil }
func (QualityDetectionHandler) Enabled() bool {
	settings, err := GetQualitySettings()
	return err == nil && settings.Enabled
}
func (QualityDetectionHandler) Interval() time.Duration {
	settings, err := GetQualitySettings()
	if err != nil {
		return time.Hour
	}
	return time.Duration(settings.IntervalMinutes) * time.Minute
}

func (QualityDetectionHandler) Run(parent context.Context, task *model.SystemTask, owner string) {
	batch, err := model.FindQualityBatchByTask(task.TaskID)
	if err != nil {
		qualityTaskFailure(task, owner, "initialization_failed")
		return
	}
	if batch == nil {
		settings, err := GetQualitySettings()
		if err != nil {
			qualityTaskFailure(task, owner, "settings_unavailable")
			return
		}
		var runs []model.QualityProbeRun
		if settings.Enabled {
			runs, err = freezeQualityTargets(QualityBatchRequest{}, settings)
		}
		if err != nil {
			qualityTaskFailure(task, owner, "snapshot_failed")
			return
		}
		settingsJSON, err := common.Marshal(settings)
		if err != nil {
			qualityTaskFailure(task, owner, "settings_invalid")
			return
		}
		batch, err = model.CreateQualityBatch(&model.QualityProbeBatch{Trigger: "scheduled", Status: "queued", CreatedAt: time.Now().Unix(),
			SettingsJSON: string(settingsJSON), ConfigVersion: settings.Version}, runs, task, owner)
		if err != nil {
			qualityTaskFailure(task, owner, "initialization_failed")
			return
		}
	}
	var settings model.QualitySettings
	if err := common.UnmarshalJsonStr(batch.SettingsJSON, &settings); err != nil || settings.Concurrency < 1 || settings.Concurrency > 10 {
		finishQualityTask(batch.ID, task, owner, "interrupted")
		return
	}
	if batch.CompletedAt != 0 {
		_ = model.FinishSystemTask(task.TaskID, owner, model.SystemTaskStatusSucceeded, batch, "")
		return
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	stopWatcher := make(chan struct{})
	watcherDone := make(chan struct{})
	defer func() {
		close(stopWatcher)
		<-watcherDone
		if parent.Err() == nil {
			cleanupCtx, cleanupCancel := context.WithTimeout(parent, 20*time.Second)
			defer cleanupCancel()
			_ = CleanupQualityArtifacts(cleanupCtx, settings.RetentionDays)
		}
	}()
	go func() {
		defer close(watcherDone)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stopWatcher:
				return
			case <-ticker.C:
				cancelled, err := model.CheckQualityBatchLease(task.TaskID, owner)
				if err != nil || cancelled {
					cancel()
					return
				}
			}
		}
	}()
	err = model.ReserveQualityBatchBudget(batch.ID, owner, settings, time.Now().UTC().Format("2006-01-02"))
	if err != nil {
		reason := "budget_blocked"
		if errors.Is(err, model.ErrQualityCancelled) {
			reason = "cancelled"
		} else if !errors.Is(err, model.ErrQualityBudget) {
			reason = "interrupted"
		}
		finishQualityTask(batch.ID, task, owner, reason)
		return
	}
	_, runs, err := model.GetQualityBatch(batch.ID)
	if err != nil {
		finishQualityTask(batch.ID, task, owner, "interrupted")
		return
	}
	jobs := make(chan model.QualityProbeRun)
	var workers sync.WaitGroup
	for i := 0; i < settings.Concurrency; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			defer func() {
				if recover() != nil {
					cancel()
				}
			}()
			for run := range jobs {
				if ctx.Err() != nil {
					continue
				}
				executeQualityRun(ctx, task, owner, run.ID)
			}
		}()
	}
	for _, run := range runs {
		if run.Status != "queued" {
			continue
		}
		select {
		case jobs <- run:
		case <-ctx.Done():
		}
	}
	close(jobs)
	workers.Wait()
	reason := ""
	if ctx.Err() != nil {
		reason = "interrupted"
	}
	finishQualityTask(batch.ID, task, owner, reason)
}

func executeQualityRun(parent context.Context, task *model.SystemTask, owner string, id int64) {
	run, err := model.ClaimQualityRun(id, owner)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(run.TimeoutSeconds)*time.Second)
	defer cancel()
	run.Status, run.QualityResult = "failed", "not_evaluated"
	executor, executorErr := GetQualityExecutor()
	channel, channelErr := model.GetChannelById(run.ChannelID, false)
	profile, profileErr := model.GetQualityProfile(run.ChannelID)
	settings, settingsErr := GetQualitySettings()
	var output bytes.Buffer
	var result QualityExecutionResult
	var executionErr error
	validConfig := false
	if channelErr == nil && executorErr == nil && profileErr == nil && settingsErr == nil && profile.Enabled {
		// Current execution inputs must still match the queued snapshot. Alias,
		// publication, and scheduling changes do not retarget a frozen request.
		fingerprint, hashErr := QualityConfigFingerprint(channel, *profile, settings)
		validConfig = hashErr == nil && fingerprint == run.ConfigFingerprint && qualityChannelUnsupportedReason(channel, *profile, executor) == ""
	}
	cancelled, leaseErr := model.CheckQualityBatchLease(task.TaskID, owner)
	if leaseErr != nil {
		return
	}
	if cancelled || ctx.Err() != nil {
		run.Status, run.ErrorCode = "cancelled", "cancelled"
	} else if !validConfig {
		run.Status, run.ErrorCode = "skipped", "configuration_changed"
	} else {
		// This is the only generation call. Neither transport failures nor
		// screenshot failures enter a retry path.
		outputLimited := false
		result, executionErr = executor.Execute(ctx, QualityExecutionRequest{ChannelID: run.ChannelID, Model: run.Model,
			Protocol: run.Protocol, Prompt: run.PromptSnapshot, MaxOutputTokens: run.MaxOutputTokens, ReasoningEffort: run.ReasoningEffort}, func(text string) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if outputLimited || len(text) > QualityOutputLimit-output.Len() {
				outputLimited = true
				return errQualityOutputLimit
			}
			_, err := output.WriteString(text)
			return err
		})
		if outputLimited {
			executionErr = errQualityOutputLimit
		}
		run.RequestStarted = result.Started || result.Complete || output.Len() > 0 || outputLimited
		if len(result.RequestFingerprint) == 64 {
			run.RequestFingerprint = result.RequestFingerprint
		}
		run.InputTokens, run.OutputTokens = result.InputTokens, result.OutputTokens
		run.FirstTextMS, run.GenerationMS = result.FirstTextMS, &result.GenerationMS
		run.MappedModel, run.ResponseModel = truncateQualityModel(result.MappedModel), truncateQualityModel(result.ResponseModel)
		run.ErrorCode = safeQualityErrorCode(result.ErrorCode)
		if errors.Is(executionErr, errQualityOutputLimit) {
			run.ErrorCode = "output_limit"
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			run.ErrorCode = "timeout"
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			run.Status, run.ErrorCode = "interrupted", "interrupted"
		}
		if !run.RequestStarted && run.Status == "failed" {
			run.Status = "skipped"
		}
		if result.Complete && executionErr == nil && ctx.Err() == nil {
			run.Status, run.QualityResult, run.ErrorCode = "completed", "needs_review", ""
		} else if run.ErrorCode == "" {
			run.ErrorCode = "stream_incomplete"
		}
	}
	raw := output.Bytes()
	html := ExtractQualityHTML(raw)
	run.ExtractorVersion = "html-extract-v1"
	run.RuleResults = qualityRuleEvidence(run, html)
	if len(raw) > 0 {
		run.RawKey, err = writeQualityArtifact("raw", raw)
		run.HasOutput = err == nil
		if err != nil {
			run.ErrorCode = "artifact_unavailable"
		}
	}
	if len(html) > 0 && run.HasOutput {
		run.HTMLKey, err = writeQualityArtifact("html", html)
		run.HasHTML = err == nil
		if err != nil {
			run.ErrorCode = "artifact_unavailable"
		}
	}
	run.RenderStatus = "not_applicable"
	if run.HasHTML {
		run.RenderStatus = "unavailable"
		if QualityRendererAvailable() && ctx.Err() == nil {
			run.RenderStatus = "pending"
		}
	}
	run.CompletedAt = time.Now().Unix()
	if err := model.SaveQualityRun(run, owner); err != nil {
		return
	}
	if run.RenderStatus == "pending" {
		renderQualityRun(ctx, run, owner, html)
	}
}

var errQualityOutputLimit = errors.New("quality output limit exceeded")

func safeQualityErrorCode(code string) string {
	switch code {
	case "channel_disabled", "model_unavailable", "unsupported_protocol", "unsupported_model", "no_available_key", "configuration_conflict",
		"upstream_error", "upstream_rejected", "upstream_unavailable", "timeout", "cancelled", "stream_incomplete", "output_limit", "invalid_response", "credential_unavailable",
		"empty_output", "stream_limit", "stream_interrupted", "invalid_stream", "incomplete_output", "output_write_failed":
		return code
	default:
		return ""
	}
}

func truncateQualityModel(name string) string {
	if len(name) > 256 {
		return ""
	}
	return strings.ToValidUTF8(name, "")
}

func finishQualityTask(id int64, task *model.SystemTask, owner, reason string) {
	batch, err := model.FinishQualityBatch(id, owner, reason)
	if err != nil {
		return
	}
	status := model.SystemTaskStatusSucceeded
	if batch.Status == "failed" || batch.Status == "interrupted" {
		status = model.SystemTaskStatusFailed
	}
	result := map[string]any{"batch_id": batch.ID, "status": batch.Status, "total": batch.Total,
		"completed": batch.Completed, "failed": batch.Failed, "summary": fmt.Sprintf("%s: %d/%d completed", batch.Status, batch.Completed, batch.Total)}
	_ = model.FinishSystemTask(task.TaskID, owner, status, result, "")
}

func qualityTaskFailure(task *model.SystemTask, owner, code string) {
	// Only fixed internal codes reach the generic task panel or its logs.
	_ = model.FinishSystemTask(task.TaskID, owner, model.SystemTaskStatusFailed, nil, code)
}
