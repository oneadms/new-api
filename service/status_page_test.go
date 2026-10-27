package service

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func statusPageFixture(t *testing.T) (*model.QualitySettings, []model.QualityProbeRun) {
	t.Helper()
	qualityServiceFixture(t, nil)
	require.NoError(t, model.DB.AutoMigrate(&model.Ability{}, &model.PerfMetric{}))
	settings, err := GetQualitySettings()
	require.NoError(t, err)
	settings.Enabled, settings.StatusPageEnabled = true, true
	settings.PublicGroups = []model.QualityPublicGroup{{Name: "internal-standard", Alias: "Standard"}, {Name: "internal-member", Alias: "Member"}, {Name: "empty-internal", Alias: "No sample"}}
	settings, err = UpdateQualitySettings(*settings)
	require.NoError(t, err)
	runs := []model.QualityProbeRun{}
	for id, groups := range map[int]string{1: "internal-standard,internal-member", 2: "internal-member", 3: "private-group"} {
		qualityServiceChannel(t, id, groups)
		require.NoError(t, model.DB.Model(&model.ChannelQualityProfile{}).Where("channel_id = ?", id).Updates(map[string]any{"publish_sample": true, "public_alias": "Public channel"}).Error)
		channel, err := model.GetChannelById(id, false)
		require.NoError(t, err)
		var profile model.ChannelQualityProfile
		require.NoError(t, model.DB.First(&profile, "channel_id = ?", id).Error)
		fingerprint, err := QualityConfigFingerprint(channel, profile, settings)
		require.NoError(t, err)
		run := model.QualityProbeRun{PublicID: common.GetUUID(), BatchID: 1, ChannelID: id, ChannelName: "private-name", Model: "text-model",
			PromptVersionID: profile.PromptVersionID, Status: "completed", QualityResult: "rules_passed", RequestStarted: true,
			CompletedAt: time.Now().Unix(), ConfigFingerprint: fingerprint, RenderStatus: "unavailable", PromptSnapshot: "private-prompt", ResponseModel: "private-upstream-model"}
		require.NoError(t, model.DB.Create(&run).Error)
		for _, group := range channel.GetGroups() {
			require.NoError(t, model.DB.Create(&model.QualityProbeRunGroup{RunID: run.ID, GroupName: group}).Error)
			for _, name := range []string{"text-model", "other-model"} {
				if id == 3 {
					name = "private-model"
				}
				require.NoError(t, model.DB.Save(&model.Ability{ChannelId: id, Group: group, Model: name, Enabled: true}).Error)
			}
		}
		runs = append(runs, run)
	}
	return settings, runs
}

func TestStatusPageSharesSamplesAndWeightsOnlyClosedBusinessCounters(t *testing.T) {
	_, _ = statusPageFixture(t)
	bucket := perf_metrics_setting.GetBucketSeconds()
	end := time.Now().Unix()
	end -= end % bucket
	for _, metric := range []model.PerfMetric{
		{ModelName: "text-model", Group: "internal-member", BucketTs: end - bucket, RequestCount: 9, SuccessCount: 9, TotalLatencyMs: 900, OutputTokens: 90, GenerationMs: 9000},
		{ModelName: "other-model", Group: "internal-member", BucketTs: end - bucket, RequestCount: 1, SuccessCount: 0, TotalLatencyMs: 300, OutputTokens: 10, GenerationMs: 1000},
		{ModelName: "text-model", Group: "internal-member", BucketTs: end, RequestCount: 100, SuccessCount: 0},
		{ModelName: "private-model", Group: "private-group", BucketTs: end - bucket, RequestCount: 500, SuccessCount: 0},
	} {
		require.NoError(t, model.DB.Create(&metric).Error)
	}
	query := StatusPageQuery{Hours: 24, Page: 1, PageSize: 20, Sort: "name"}
	data, err := GetStatusPage(context.Background(), query)
	require.NoError(t, err)
	assert.Equal(t, 3, data.Summary.Groups)
	assert.Equal(t, 2, data.Summary.Channels)
	assert.Equal(t, end, data.MetricsDataUntil)
	member, err := GetStatusGroup(context.Background(), statusGroupID("internal-member"), query)
	require.NoError(t, err)
	standard, err := GetStatusGroup(context.Background(), statusGroupID("internal-standard"), query)
	require.NoError(t, err)
	assert.Equal(t, 2, member.Group.ChannelCount)
	assert.Equal(t, 2, member.Group.FreshCount)
	assert.Equal(t, "passed", member.Group.State)
	require.NotNil(t, member.Group.SuccessRate)
	assert.Equal(t, 90.0, *member.Group.SuccessRate)
	require.NotNil(t, member.Group.AvgLatencyMS)
	assert.Equal(t, 120.0, *member.Group.AvgLatencyMS)
	require.NotNil(t, member.Group.TokensPerSecond)
	assert.Equal(t, 10.0, *member.Group.TokensPerSecond)
	require.Len(t, standard.Samples, 1)
	var shared int
	for _, sample := range member.Samples {
		if sample.PublicID == standard.Samples[0].PublicID {
			shared++
		}
	}
	assert.Equal(t, 1, shared)
	assert.Nil(t, member.Group.Series[0].Value)
	require.NotNil(t, member.Group.Series[len(member.Group.Series)-1].Value)
	assert.Equal(t, 90.0, *member.Group.Series[len(member.Group.Series)-1].Value)
	assert.Nil(t, standard.Group.SuccessRate)
	serialized, err := common.Marshal(data)
	require.NoError(t, err)
	for _, secret := range []string{"private-group", "private-model", "private-name", "internal-standard", "internal-member", "private-prompt", "private-upstream-model", "channel_id", "prompt_snapshot"} {
		assert.NotContains(t, string(serialized), secret)
	}
	query.PageSize = 1
	page, err := GetStatusPage(context.Background(), query)
	require.NoError(t, err)
	assert.Len(t, page.Groups, 1)
	assert.Equal(t, data.Summary, page.Summary)
	assert.Equal(t, 3, page.Total)
}

func TestStatusPageModelFilterDoesNotRepublishAnotherModelsSample(t *testing.T) {
	_, _ = statusPageFixture(t)
	// Add a second genuine model to channel metadata and abilities. Its public
	// status must not inherit the representative model's result.
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id IN ?", []int{1, 2}).Update("models", "text-model,other-model").Error)
	query := StatusPageQuery{Hours: 24, Model: "other-model", Page: 1, PageSize: 20, Sort: "name"}
	data, err := GetStatusPage(context.Background(), query)
	require.NoError(t, err)
	require.Len(t, data.Groups, 2)
	for _, group := range data.Groups {
		assert.Nil(t, group.LatestSample)
		assert.Zero(t, group.FreshCount)
		assert.Equal(t, "unknown", group.State)
	}
	assert.Zero(t, data.Summary.Channels)
	query.Model = "private-model"
	data, err = GetStatusPage(context.Background(), query)
	require.NoError(t, err)
	assert.Empty(t, data.Groups)
	assert.NotContains(t, data.Models, "private-model")
}

func TestStatusPageChangedConfigurationOrGroupMembershipInvalidatesCurrentEvidence(t *testing.T) {
	_, _ = statusPageFixture(t)
	require.NoError(t, model.DB.Model(&model.ChannelQualityProfile{}).Where("channel_id = ?", 1).Update("reasoning_effort", "high").Error)
	query := StatusPageQuery{Hours: 24, Page: 1, PageSize: 20, Sort: "name"}
	group, err := GetStatusGroup(context.Background(), statusGroupID("internal-standard"), query)
	require.NoError(t, err)
	assert.Equal(t, "stale", group.Group.State)
	assert.Zero(t, group.Group.FreshCount)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 1).Update("group", "internal-member").Error)
	group, err = GetStatusGroup(context.Background(), statusGroupID("internal-standard"), query)
	require.NoError(t, err)
	assert.Empty(t, group.Samples)
	assert.Zero(t, group.Group.ChannelCount)
}

func statusHistoryRun(t *testing.T, source model.QualityProbeRun, batchID, completedAt int64, groups ...string) model.QualityProbeRun {
	t.Helper()
	source.ID, source.PublicID, source.BatchID, source.CompletedAt = 0, common.GetUUID(), batchID, completedAt
	require.NoError(t, model.DB.Create(&source).Error)
	for _, group := range groups {
		require.NoError(t, model.DB.Create(&model.QualityProbeRunGroup{RunID: source.ID, GroupName: group}).Error)
	}
	return source
}

func TestStatusGroupHistoryPaginatesRetainedRunsWithoutChangingCurrentAggregate(t *testing.T) {
	_, _ = statusPageFixture(t)
	var oldest model.QualityProbeRun
	require.NoError(t, model.DB.First(&oldest, "channel_id = ?", 1).Error)
	now := time.Now().Unix()
	require.NoError(t, model.DB.Model(&model.QualityProbeRun{}).Where("id = ?", oldest.ID).Updates(map[string]any{"completed_at": now - 49*3600, "status": "failed", "quality_result": "not_evaluated"}).Error)
	tieFirst := statusHistoryRun(t, oldest, 2, now-2*3600, "internal-standard", "internal-member")
	tieSecond := statusHistoryRun(t, oldest, 3, now-2*3600, "internal-standard", "internal-member")
	latest := statusHistoryRun(t, oldest, 4, now, "internal-standard", "internal-member")
	for i, status := range []string{"queued", "running"} {
		unfinished := latest
		unfinished.Status = status
		statusHistoryRun(t, unfinished, int64(5+i), 0, "internal-standard")
	}
	bucket := perf_metrics_setting.GetBucketSeconds()
	end := now - now%bucket
	for _, metric := range []model.PerfMetric{
		{ModelName: "text-model", Group: "internal-standard", BucketTs: end - bucket, RequestCount: 1, SuccessCount: 1},
		{ModelName: "text-model", Group: "internal-standard", BucketTs: end - 2*3600, RequestCount: 1, SuccessCount: 0},
	} {
		require.NoError(t, model.DB.Create(&metric).Error)
	}
	query := StatusPageQuery{Hours: 1, Page: 1, PageSize: 2, Sort: "name"}
	for page, expected := range [][]string{{latest.PublicID, tieSecond.PublicID}, {tieFirst.PublicID, oldest.PublicID}, {}} {
		query.Page = page + 1
		detail, err := GetStatusGroup(context.Background(), statusGroupID("internal-standard"), query)
		require.NoError(t, err)
		ids := make([]string, 0, len(detail.Samples))
		for _, sample := range detail.Samples {
			ids = append(ids, sample.PublicID)
			assert.Nil(t, sample.ThumbnailURL)
		}
		assert.Equal(t, expected, ids, "page %d", query.Page)
		assert.Equal(t, "passed", detail.Group.State)
		assert.Equal(t, 1, detail.Group.FreshCount)
		require.NotNil(t, detail.Group.LatestSample)
		assert.Equal(t, latest.PublicID, detail.Group.LatestSample.PublicID)
		require.NotNil(t, detail.Group.SuccessRate)
		assert.Equal(t, 100.0, *detail.Group.SuccessRate)
		assert.Equal(t, int64(4), detail.Total)
		assert.Equal(t, query.Page, detail.Page)
		assert.Equal(t, query.PageSize, detail.PageSize)
	}
	query.Hours, query.Page = 24, 1
	detail, err := GetStatusGroup(context.Background(), statusGroupID("internal-standard"), query)
	require.NoError(t, err)
	require.NotNil(t, detail.Group.SuccessRate)
	assert.Equal(t, 50.0, *detail.Group.SuccessRate)
	data, err := GetStatusPage(context.Background(), query)
	require.NoError(t, err)
	for _, group := range data.Groups {
		if group.PublicID == detail.Group.PublicID {
			assert.Equal(t, detail.Group, group)
		}
	}
}

func TestStatusGroupHistorySharesHistoricalScreenshotsAndExcludesPrivateFields(t *testing.T) {
	_, _ = statusPageFixture(t)
	var source model.QualityProbeRun
	require.NoError(t, model.DB.First(&source, "channel_id = ?", 1).Error)
	source.RawKey, source.HTMLKey, source.ThumbnailKey = "private-raw-key", "private-html-key", "private-thumbnail-key"
	source.MappedModel, source.PromptName, source.ReviewNote = "private-mapped-model", "private-prompt-name", "private-review-note"
	source.HasThumbnail = true
	old := statusHistoryRun(t, source, 2, source.CompletedAt-48*3600, "internal-standard", "internal-member")
	latest := statusHistoryRun(t, source, 3, source.CompletedAt, "internal-standard", "internal-member")
	query := StatusPageQuery{Hours: 1, Page: 1, PageSize: 20, Sort: "name"}
	var shared []StatusSample
	for _, name := range []string{"internal-standard", "internal-member"} {
		detail, err := GetStatusGroup(context.Background(), statusGroupID(name), query)
		require.NoError(t, err)
		for _, sample := range detail.Samples {
			if sample.PublicID == old.PublicID {
				shared = append(shared, sample)
			}
		}
		if name == "internal-standard" {
			require.NotNil(t, detail.Group.LatestSample)
			assert.Equal(t, latest.PublicID, detail.Group.LatestSample.PublicID)
		}
		serialized, err := common.Marshal(detail)
		require.NoError(t, err)
		for _, secret := range []string{"private-", "internal-standard", "internal-member", "channel_id", "prompt_snapshot", "raw_key", "html_key", "thumbnail_key", "config_fingerprint", "review_note"} {
			assert.NotContains(t, string(serialized), secret)
		}
	}
	require.Len(t, shared, 2)
	assert.Equal(t, shared[0], shared[1])
	require.NotNil(t, shared[0].ThumbnailURL)
	assert.Equal(t, "/api/status-page/artifacts/"+old.PublicID+"/thumbnail", *shared[0].ThumbnailURL)
}

func TestStatusGroupHistoryModelFilterKeepsOldRepresentativeModelOnlyWhileExposed(t *testing.T) {
	_, _ = statusPageFixture(t)
	var old model.QualityProbeRun
	require.NoError(t, model.DB.First(&old, "channel_id = ?", 1).Error)
	var imageBytes bytes.Buffer
	require.NoError(t, png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	key, err := writeQualityArtifact("png", imageBytes.Bytes())
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&old).Updates(map[string]any{"has_thumbnail": true, "thumbnail_key": key}).Error)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 1).Update("models", "text-model,other-model").Error)
	require.NoError(t, model.DB.Model(&model.ChannelQualityProfile{}).Where("channel_id = ?", 1).Update("model", "other-model").Error)
	source := old
	source.Model = "other-model"
	latest := statusHistoryRun(t, source, 2, old.CompletedAt, "internal-standard", "internal-member")
	query := StatusPageQuery{Hours: 24, Page: 1, PageSize: 20, Sort: "name", Model: "text-model"}
	detail, err := GetStatusGroup(context.Background(), statusGroupID("internal-standard"), query)
	require.NoError(t, err)
	require.Len(t, detail.Samples, 1)
	assert.Equal(t, old.PublicID, detail.Samples[0].PublicID)
	assert.Equal(t, int64(1), detail.Total)
	assert.Equal(t, old.CompletedAt, detail.Samples[0].StaleAfter)
	require.NotNil(t, detail.Samples[0].ThumbnailURL)
	actual, err := ReadPublicStatusThumbnail(old.PublicID)
	require.NoError(t, err)
	assert.Equal(t, imageBytes.Bytes(), actual)
	assert.Nil(t, detail.Group.LatestSample)
	assert.Zero(t, detail.Group.FreshCount)
	assert.Zero(t, detail.Group.ConfiguredCount)
	query.Model = "other-model"
	detail, err = GetStatusGroup(context.Background(), statusGroupID("internal-standard"), query)
	require.NoError(t, err)
	require.Len(t, detail.Samples, 1)
	assert.Equal(t, latest.PublicID, detail.Samples[0].PublicID)
	query.Model = ""
	detail, err = GetStatusGroup(context.Background(), statusGroupID("internal-standard"), query)
	require.NoError(t, err)
	assert.Len(t, detail.Samples, 2)
	// Stale ability rows cannot keep publishing a model removed from the channel.
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 1).Update("models", "other-model").Error)
	detail, err = GetStatusGroup(context.Background(), statusGroupID("internal-standard"), query)
	require.NoError(t, err)
	require.Len(t, detail.Samples, 1)
	assert.Equal(t, latest.PublicID, detail.Samples[0].PublicID)
	// Channel metadata alone is insufficient when the group's model ability is removed.
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 1).Update("models", "text-model,other-model").Error)
	require.NoError(t, model.DB.Where(map[string]any{"channel_id": 1, "group": "internal-standard", "model": "text-model"}).Delete(&model.Ability{}).Error)
	detail, err = GetStatusGroup(context.Background(), statusGroupID("internal-standard"), query)
	require.NoError(t, err)
	require.Len(t, detail.Samples, 1)
	assert.Equal(t, latest.PublicID, detail.Samples[0].PublicID)
}

func TestStatusGroupHistoryRequiresBothCurrentAndHistoricalGroupMembership(t *testing.T) {
	_, _ = statusPageFixture(t)
	var source model.QualityProbeRun
	require.NoError(t, model.DB.First(&source, "channel_id = ?", 1).Error)
	memberOnly := statusHistoryRun(t, source, 2, source.CompletedAt-3600, "internal-member")
	statusHistoryRun(t, source, 3, source.CompletedAt, "internal-standard", "internal-member")
	query := StatusPageQuery{Hours: 24, Page: 1, PageSize: 20, Sort: "name"}
	standard, err := GetStatusGroup(context.Background(), statusGroupID("internal-standard"), query)
	require.NoError(t, err)
	for _, sample := range standard.Samples {
		assert.NotEqual(t, memberOnly.PublicID, sample.PublicID)
	}
	member, err := GetStatusGroup(context.Background(), statusGroupID("internal-member"), query)
	require.NoError(t, err)
	var found bool
	for _, sample := range member.Samples {
		found = found || sample.PublicID == memberOnly.PublicID
	}
	assert.True(t, found)
	// Moving a channel must neither preserve access in the old group nor import
	// old runs into the new group, even if ability rows lag behind metadata.
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 1).Update("group", "empty-internal").Error)
	require.NoError(t, model.DB.Create(&model.Ability{ChannelId: 1, Group: "empty-internal", Model: "text-model", Enabled: true}).Error)
	for _, group := range []string{"internal-standard", "empty-internal"} {
		detail, err := GetStatusGroup(context.Background(), statusGroupID(group), query)
		require.NoError(t, err)
		assert.Empty(t, detail.Samples)
	}
}

func TestStatusGroupHistoryAndThumbnailHonorCurrentPublicationRevocation(t *testing.T) {
	cases := []struct {
		name        string
		revoke      func(*testing.T, *model.QualitySettings, model.QualityProbeRun)
		hiddenGroup bool
	}{
		{"unpublished_channel", func(t *testing.T, _ *model.QualitySettings, _ model.QualityProbeRun) {
			require.NoError(t, model.DB.Model(&model.ChannelQualityProfile{}).Where("channel_id = ?", 1).Update("publish_sample", false).Error)
		}, false},
		{"empty_alias", func(t *testing.T, _ *model.QualitySettings, _ model.QualityProbeRun) {
			require.NoError(t, model.DB.Model(&model.ChannelQualityProfile{}).Where("channel_id = ?", 1).Update("public_alias", "").Error)
		}, false},
		{"disabled_status_page", func(t *testing.T, settings *model.QualitySettings, _ model.QualityProbeRun) {
			settings.StatusPageEnabled = false
			_, err := UpdateQualitySettings(*settings)
			require.NoError(t, err)
		}, true},
		{"unpublished_groups", func(t *testing.T, settings *model.QualitySettings, _ model.QualityProbeRun) {
			settings.PublicGroups = nil
			_, err := UpdateQualitySettings(*settings)
			require.NoError(t, err)
		}, true},
		{"deleted_channel", func(t *testing.T, _ *model.QualitySettings, _ model.QualityProbeRun) {
			require.NoError(t, model.DB.Delete(&model.Channel{}, 1).Error)
		}, false},
		{"removed_model", func(t *testing.T, _ *model.QualitySettings, _ model.QualityProbeRun) {
			require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 1).Update("models", "other-model").Error)
		}, false},
		{"removed_abilities", func(t *testing.T, _ *model.QualitySettings, _ model.QualityProbeRun) {
			require.NoError(t, model.DB.Where("channel_id = ? AND model = ?", 1, "text-model").Delete(&model.Ability{}).Error)
		}, false},
		{"moved_channel", func(t *testing.T, _ *model.QualitySettings, _ model.QualityProbeRun) {
			require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 1).Update("group", "empty-internal").Error)
			require.NoError(t, model.DB.Create(&model.Ability{ChannelId: 1, Group: "empty-internal", Model: "text-model", Enabled: true}).Error)
		}, false},
		{"ability_and_history_in_different_groups", func(t *testing.T, _ *model.QualitySettings, run model.QualityProbeRun) {
			require.NoError(t, model.DB.Where("run_id = ? AND group_name = ?", run.ID, "internal-standard").Delete(&model.QualityProbeRunGroup{}).Error)
			require.NoError(t, model.DB.Where(map[string]any{"channel_id": 1, "model": "text-model", "group": "internal-member"}).Delete(&model.Ability{}).Error)
		}, false},
		{"removed_historical_membership", func(t *testing.T, _ *model.QualitySettings, run model.QualityProbeRun) {
			require.NoError(t, model.DB.Where("run_id = ?", run.ID).Delete(&model.QualityProbeRunGroup{}).Error)
		}, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			settings, _ := statusPageFixture(t)
			var source model.QualityProbeRun
			require.NoError(t, model.DB.First(&source, "channel_id = ?", 1).Error)
			var imageBytes bytes.Buffer
			require.NoError(t, png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))))
			key, err := writeQualityArtifact("png", imageBytes.Bytes())
			require.NoError(t, err)
			source.HasThumbnail, source.ThumbnailKey = true, key
			old := statusHistoryRun(t, source, 2, source.CompletedAt-48*3600, "internal-standard", "internal-member")
			statusHistoryRun(t, source, 3, source.CompletedAt, "internal-standard", "internal-member")
			actual, err := ReadPublicStatusThumbnail(old.PublicID)
			require.NoError(t, err)
			assert.Equal(t, imageBytes.Bytes(), actual)
			tt.revoke(t, settings, old)
			detail, err := GetStatusGroup(context.Background(), statusGroupID("internal-standard"), StatusPageQuery{Hours: 1, Page: 1, PageSize: 20, Sort: "name"})
			if tt.hiddenGroup {
				require.ErrorIs(t, err, ErrStatusPageNotFound)
			} else {
				require.NoError(t, err)
				if tt.name == "removed_model" || tt.name == "removed_abilities" {
					assert.Nil(t, detail.Group.LatestSample)
				}
				for _, sample := range detail.Samples {
					assert.NotEqual(t, old.PublicID, sample.PublicID)
				}
			}
			_, err = ReadPublicStatusThumbnail(old.PublicID)
			require.ErrorIs(t, err, ErrStatusPageNotFound)
		})
	}
}

func TestStatusPageThumbnailRevocationAndPrivateGroupsDenyPreviouslyValidIDs(t *testing.T) {
	settings, runs := statusPageFixture(t)
	var run model.QualityProbeRun
	for _, value := range runs {
		if value.ChannelID == 1 {
			run = value
		}
	}
	var imageBytes bytes.Buffer
	require.NoError(t, png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	key, err := writeQualityArtifact("png", imageBytes.Bytes())
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&run).Updates(map[string]any{"thumbnail_key": key, "has_thumbnail": true}).Error)
	actual, err := ReadPublicStatusThumbnail(run.PublicID)
	require.NoError(t, err)
	assert.Equal(t, imageBytes.Bytes(), actual)
	require.NoError(t, model.DB.Model(&model.ChannelQualityProfile{}).Where("channel_id = ?", 1).Update("publish_sample", false).Error)
	_, err = ReadPublicStatusThumbnail(run.PublicID)
	require.ErrorIs(t, err, ErrStatusPageNotFound)
	require.NoError(t, model.DB.Model(&model.ChannelQualityProfile{}).Where("channel_id = ?", 1).Update("publish_sample", true).Error)
	settings.PublicGroups = []model.QualityPublicGroup{}
	_, err = UpdateQualitySettings(*settings)
	require.NoError(t, err)
	_, err = ReadPublicStatusThumbnail(run.PublicID)
	require.ErrorIs(t, err, ErrStatusPageNotFound)
	_, err = GetStatusGroup(context.Background(), statusGroupID("private-group"), StatusPageQuery{Hours: 24, Page: 1, PageSize: 20, Sort: "name"})
	require.ErrorIs(t, err, ErrStatusPageNotFound)
}
