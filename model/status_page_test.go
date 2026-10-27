package model

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusGroupHistoryCountsOnlyFinalRunsMatchingPublishedPairsAndOriginalGroup(t *testing.T) {
	qualityModelFixture(t)
	for i, fixture := range []struct {
		id, model, status, group string
		channel                  int
		completedAt              int64
	}{
		{"completed", "text-model", "completed", "published", 1, 100},
		{"failed", "text-model", "failed", "published", 1, 200},
		{"cancelled", "text-model", "cancelled", "published", 1, 200},
		{"interrupted", "text-model", "interrupted", "published", 1, 300},
		{"skipped", "text-model", "skipped", "published", 1, 400},
		{"queued", "text-model", "queued", "published", 1, 0},
		{"running", "text-model", "running", "published", 1, 0},
		{"unfinished-with-timestamp", "text-model", "running", "published", 1, 500},
		{"missing-timestamp", "text-model", "completed", "published", 1, 0},
		{"other-completed", "other-model", "completed", "published", 2, 600},
		{"unpublished-model", "other-model", "completed", "published", 1, 700},
		{"unpublished-channel-model-pair", "text-model", "completed", "published", 2, 800},
		{"unpublished-channel", "text-model", "completed", "published", 3, 900},
		{"private-history", "text-model", "completed", "private", 1, 1000},
		{"backfilled", "text-model", "completed", "published", 1, 150},
	} {
		run := QualityProbeRun{PublicID: fixture.id, BatchID: int64(i + 1), ChannelID: fixture.channel, Model: fixture.model, Status: fixture.status, CompletedAt: fixture.completedAt}
		require.NoError(t, DB.Create(&run).Error)
		require.NoError(t, DB.Create(&QualityProbeRunGroup{RunID: run.ID, GroupName: fixture.group}).Error)
	}
	published := map[int][]string{1: {"text-model"}, 2: {"other-model"}}
	for page, ids := range [][]string{{"other-completed", "skipped"}, {"interrupted", "cancelled"}, {"failed", "backfilled"}, {"completed"}, {}} {
		runs, total, err := LoadStatusGroupHistory(context.Background(), "published", published, page+1, 2)
		require.NoError(t, err)
		assert.Equal(t, int64(7), total)
		actual := make([]string, 0, len(runs))
		for _, run := range runs {
			actual = append(actual, run.PublicID)
		}
		assert.Equal(t, ids, actual)
	}
	runs, total, err := LoadStatusGroupHistory(context.Background(), "published", nil, 1, 20)
	require.NoError(t, err)
	assert.Empty(t, runs)
	assert.Zero(t, total)
}

func TestStatusPageSnapshotRetainsLatestOnlyWhenHistoricalRunsExist(t *testing.T) {
	qualityModelFixture(t)
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Ability{}, &PerfMetric{}))
	require.NoError(t, DB.Create(&Channel{Id: 1, Key: "private-credential", Group: "published", Models: "text-model"}).Error)
	require.NoError(t, DB.Create(&ChannelQualityProfile{ChannelID: 1, PublishSample: true, PublicAlias: "Public channel"}).Error)
	require.NoError(t, DB.Create(&Ability{ChannelId: 1, Group: "published", Model: "text-model"}).Error)
	var latest QualityProbeRun
	for i, completedAt := range []int64{1, 100, 200} {
		run := QualityProbeRun{PublicID: common.GetUUID(), BatchID: int64(i + 1), ChannelID: 1, Model: "text-model", Status: "completed", CompletedAt: completedAt}
		require.NoError(t, DB.Create(&run).Error)
		require.NoError(t, DB.Create(&QualityProbeRunGroup{RunID: run.ID, GroupName: "published"}).Error)
		latest = run
	}
	snapshot, err := LoadStatusPageSnapshot(context.Background(), []string{"published"}, "", 100, 300)
	require.NoError(t, err)
	require.Len(t, snapshot.Runs, 1)
	assert.Equal(t, latest.ID, snapshot.Runs[0].ID)
	require.Len(t, snapshot.RunGroups, 1)
	assert.Equal(t, latest.ID, snapshot.RunGroups[0].RunID)
	require.Len(t, snapshot.Channels, 1)
	assert.Empty(t, snapshot.Channels[0].Key)
	runs, total, err := LoadStatusGroupHistory(context.Background(), "published", map[int][]string{1: {"text-model"}}, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, runs, 3)
}
