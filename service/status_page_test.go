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
