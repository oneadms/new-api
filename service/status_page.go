package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
)

var ErrStatusPageNotFound = errors.New("published status resource not found")

type StatusPageQuery struct {
	Hours    int
	Model    string
	Group    string
	State    string
	Page     int
	PageSize int
	Sort     string
}

type StatusSample struct {
	PublicID      string  `json:"public_id"`
	ChannelName   string  `json:"channel_name"`
	Model         string  `json:"model"`
	Status        string  `json:"status"`
	QualityResult string  `json:"quality_result"`
	CompletedAt   int64   `json:"completed_at"`
	StaleAfter    int64   `json:"stale_after"`
	FirstTextMS   *int64  `json:"first_text_ms"`
	GenerationMS  *int64  `json:"generation_ms"`
	ThumbnailURL  *string `json:"thumbnail_url"`
	RenderStatus  string  `json:"render_status"`
}

type StatusPoint struct {
	Ts    int64    `json:"ts"`
	Value *float64 `json:"value"`
}

type StatusGroup struct {
	PublicID        string        `json:"public_id"`
	Name            string        `json:"name"`
	ChannelCount    int           `json:"channel_count"`
	ConfiguredCount int           `json:"configured_count"`
	FreshCount      int           `json:"fresh_count"`
	State           string        `json:"state"`
	SuccessRate     *float64      `json:"success_rate"`
	AvgLatencyMS    *float64      `json:"avg_latency_ms"`
	TokensPerSecond *float64      `json:"tokens_per_second"`
	Series          []StatusPoint `json:"series"`
	LatestSample    *StatusSample `json:"latest_sample"`
}

type StatusGroupDetail struct {
	Group   StatusGroup    `json:"group"`
	Samples []StatusSample `json:"samples"`
}

type StatusPageData struct {
	Enabled              bool     `json:"enabled"`
	Paused               bool     `json:"paused"`
	GeneratedAt          int64    `json:"generated_at"`
	MetricsDataUntil     int64    `json:"metrics_data_until"`
	LastProbeCompletedAt int64    `json:"last_probe_completed_at"`
	WindowStart          int64    `json:"window_start"`
	WindowEnd            int64    `json:"window_end"`
	BucketSeconds        int64    `json:"bucket_seconds"`
	Total                int      `json:"total"`
	Page                 int      `json:"page"`
	PageSize             int      `json:"page_size"`
	Models               []string `json:"models"`
	Summary              struct {
		Groups    int `json:"groups"`
		Attention int `json:"attention"`
		Channels  int `json:"channels"`
	} `json:"summary"`
	Groups []StatusGroup `json:"groups"`
}

func ValidateStatusPageQuery(query StatusPageQuery) error {
	if !slices.Contains([]int{1, 6, 24, 168}, query.Hours) || query.Page < 1 || query.Page > 100000 || query.PageSize < 1 || query.PageSize > 100 ||
		len(query.Model) > 256 || len(query.Group) > 128 || !slices.Contains([]string{"", "passed", "review", "partial", "failed", "unknown", "stale", "paused"}, query.State) ||
		!slices.Contains([]string{"name", "state"}, query.Sort) {
		return &QualityValidationError{"Invalid status page filters"}
	}
	return nil
}

func GetStatusPage(ctx context.Context, query StatusPageQuery) (*StatusPageData, error) {
	data, _, err := loadStatusPage(ctx, query, "")
	return data, err
}

func GetStatusGroup(ctx context.Context, id string, query StatusPageQuery) (*StatusGroupDetail, error) {
	_, details, err := loadStatusPage(ctx, query, id)
	if err != nil {
		return nil, err
	}
	detail, ok := details[id]
	if !ok {
		return nil, ErrStatusPageNotFound
	}
	return &detail, nil
}

func loadStatusPage(ctx context.Context, query StatusPageQuery, detailID string) (*StatusPageData, map[string]StatusGroupDetail, error) {
	if err := ValidateStatusPageQuery(query); err != nil {
		return nil, nil, err
	}
	settings, err := model.GetQualitySettings()
	if err != nil {
		return nil, nil, err
	}
	now := time.Now().Unix()
	sourceBucket := perf_metrics_setting.GetBucketSeconds()
	end := now - now%sourceBucket
	// Large windows use at most 168 chart points. All aggregation is still
	// based on summed counters, not averages of per-model percentages.
	bucket := sourceBucket
	if int64(query.Hours)*3600/bucket > 168 {
		bucket = 3600
	}
	end -= end % bucket
	start := end - int64(query.Hours)*3600
	data := &StatusPageData{Enabled: settings.StatusPageEnabled, Paused: !settings.Enabled, GeneratedAt: now,
		MetricsDataUntil: end, WindowStart: start, WindowEnd: end, BucketSeconds: bucket,
		Page: query.Page, PageSize: query.PageSize, Models: []string{}, Groups: []StatusGroup{}}
	if !settings.StatusPageEnabled {
		return data, map[string]StatusGroupDetail{}, nil
	}
	groups := make([]string, 0, len(settings.PublicGroups))
	for _, group := range settings.PublicGroups {
		if detailID == "" || statusGroupID(group.Name) == detailID {
			groups = append(groups, group.Name)
		}
	}
	if len(groups) == 0 {
		return data, map[string]StatusGroupDetail{}, nil
	}
	snapshot, err := model.LoadStatusPageSnapshot(ctx, groups, query.Model, start, end)
	if err != nil {
		return nil, nil, err
	}
	return aggregateStatusPage(data, settings, snapshot, query, groups)
}

func statusGroupID(name string) string {
	digest := sha256.Sum256([]byte("new-api:status-group:" + name))
	return hex.EncodeToString(digest[:16])
}

func aggregateStatusPage(data *StatusPageData, settings *model.QualitySettings, snapshot model.StatusPageSnapshot, query StatusPageQuery, groups []string) (*StatusPageData, map[string]StatusGroupDetail, error) {
	profiles := map[int]model.ChannelQualityProfile{}
	for _, profile := range snapshot.Profiles {
		if profile.PublishSample && profile.PublicAlias != "" {
			profiles[profile.ChannelID] = profile
		}
	}
	channels := map[int]*model.Channel{}
	for i := range snapshot.Channels {
		channels[snapshot.Channels[i].Id] = &snapshot.Channels[i]
	}
	runs := map[int]model.QualityProbeRun{}
	for _, run := range snapshot.Runs {
		runs[run.ChannelID] = run
	}
	history := map[int64]map[string]bool{}
	for _, relation := range snapshot.RunGroups {
		if history[relation.RunID] == nil {
			history[relation.RunID] = map[string]bool{}
		}
		history[relation.RunID][relation.GroupName] = true
	}
	groupChannels := map[string]map[int]bool{}
	models := map[string]bool{}
	for _, ability := range snapshot.Abilities {
		channel := channels[ability.ChannelId]
		_, published := profiles[ability.ChannelId]
		if channel == nil || !published || !slices.Contains(channel.GetGroups(), ability.Group) || !slices.Contains(channel.GetModels(), ability.Model) {
			continue
		}
		models[ability.Model] = true
		if query.Model != "" && query.Model != ability.Model {
			continue
		}
		if groupChannels[ability.Group] == nil {
			groupChannels[ability.Group] = map[int]bool{}
		}
		groupChannels[ability.Group][ability.ChannelId] = true
	}
	for name := range models {
		data.Models = append(data.Models, name)
	}
	sort.Strings(data.Models)
	details := map[string]StatusGroupDetail{}
	detected := map[int]bool{}
	for _, published := range settings.PublicGroups {
		if !slices.Contains(groups, published.Name) || (query.Group != "" && !strings.Contains(strings.ToLower(published.Alias), strings.ToLower(query.Group))) {
			continue
		}
		detail := StatusGroupDetail{Group: StatusGroup{PublicID: statusGroupID(published.Name), Name: published.Alias, State: "unknown", Series: []StatusPoint{}}, Samples: []StatusSample{}}
		group := &detail.Group
		members := groupChannels[published.Name]
		if query.Model != "" && len(members) == 0 {
			continue
		}
		group.ChannelCount = len(members)
		failed, completed, review, passed, old := 0, 0, 0, 0, 0
		freshChannels := []int{}
		for channelID := range members {
			profile, channel := profiles[channelID], channels[channelID]
			configured := profile.Enabled && profile.Model != "" && profile.PromptVersionID > 0 && (query.Model == "" || profile.Model == query.Model)
			if configured {
				group.ConfiguredCount++
			}
			run, found := runs[channelID]
			if !found || !history[run.ID][published.Name] || (query.Model != "" && run.Model != query.Model) {
				continue
			}
			fingerprint, err := QualityConfigFingerprint(channel, profile, settings)
			if err != nil {
				return nil, nil, err
			}
			staleAfter := run.CompletedAt + int64(settings.IntervalMinutes)*120 + 1800 + 60
			current := configured && fingerprint == run.ConfigFingerprint && channel.Status == common.ChannelStatusEnabled
			if !current {
				staleAfter = run.CompletedAt
			}
			sample := StatusSample{PublicID: run.PublicID, ChannelName: profile.PublicAlias, Model: run.Model,
				Status: run.Status, QualityResult: run.QualityResult, CompletedAt: run.CompletedAt, StaleAfter: staleAfter,
				FirstTextMS: run.FirstTextMS, GenerationMS: run.GenerationMS, RenderStatus: run.RenderStatus}
			if run.HasThumbnail {
				url := "/api/status-page/artifacts/" + run.PublicID + "/thumbnail"
				sample.ThumbnailURL = &url
			}
			detail.Samples = append(detail.Samples, sample)
			if data.GeneratedAt >= staleAfter || !current {
				old++
				continue
			}
			// Cancelled/unstarted work is not evidence of upstream availability.
			if run.Status == "cancelled" || run.Status == "skipped" || !run.RequestStarted {
				continue
			}
			group.FreshCount++
			freshChannels = append(freshChannels, channelID)
			if run.Status == "completed" {
				completed++
				if run.QualityResult == "rules_passed" && run.ReviewStatus != "rejected" {
					passed++
				} else {
					review++
				}
			} else {
				failed++
			}
		}
		switch {
		case !settings.Enabled:
			group.State = "paused"
		case failed > 0 && completed > 0:
			group.State = "partial"
		case failed > 0 && failed == group.ChannelCount:
			group.State = "failed"
		case review > 0 || failed > 0:
			group.State = "review"
		case passed > 0 && passed == group.ChannelCount:
			group.State = "passed"
		case old > 0:
			group.State = "stale"
		}
		// Stable IDs and timestamps keep automatic refresh from randomly
		// selecting a different sample. Attention samples take precedence.
		sort.Slice(detail.Samples, func(i, j int) bool {
			a, b := detail.Samples[i], detail.Samples[j]
			attentionA, attentionB := a.QualityResult != "rules_passed", b.QualityResult != "rules_passed"
			if attentionA != attentionB {
				return attentionA
			}
			if a.CompletedAt != b.CompletedAt {
				return a.CompletedAt > b.CompletedAt
			}
			return a.PublicID < b.PublicID
		})
		if len(detail.Samples) > 0 {
			group.LatestSample = &detail.Samples[0]
		}
		applyStatusMetrics(group, snapshot.Metrics, published.Name, data.WindowStart, data.WindowEnd, data.BucketSeconds)
		if query.State != "" && group.State != query.State {
			continue
		}
		for _, channelID := range freshChannels {
			detected[channelID] = true
		}
		for _, sample := range detail.Samples {
			if sample.CompletedAt > data.LastProbeCompletedAt {
				data.LastProbeCompletedAt = sample.CompletedAt
			}
		}
		details[group.PublicID] = detail
		data.Groups = append(data.Groups, *group)
		if group.State == "partial" || group.State == "failed" || group.State == "review" {
			data.Summary.Attention++
		}
	}
	sort.Slice(data.Groups, func(i, j int) bool {
		if query.Sort == "state" && data.Groups[i].State != data.Groups[j].State {
			return data.Groups[i].State < data.Groups[j].State
		}
		if data.Groups[i].Name != data.Groups[j].Name {
			return data.Groups[i].Name < data.Groups[j].Name
		}
		return data.Groups[i].PublicID < data.Groups[j].PublicID
	})
	data.Total, data.Summary.Groups, data.Summary.Channels = len(data.Groups), len(data.Groups), len(detected)
	start := min((query.Page-1)*query.PageSize, len(data.Groups))
	data.Groups = data.Groups[start:min(start+query.PageSize, len(data.Groups))]
	return data, details, nil
}

func applyStatusMetrics(group *StatusGroup, metrics []model.PerfMetric, name string, start, end, bucket int64) {
	var requests, successes, latency, tokens, generation float64
	counts := map[int64][2]float64{}
	for _, metric := range metrics {
		if metric.Group != name || metric.BucketTs < start || metric.BucketTs >= end || metric.RequestCount <= 0 {
			continue
		}
		requests += float64(metric.RequestCount)
		successes += float64(metric.SuccessCount)
		latency += float64(metric.TotalLatencyMs)
		tokens += float64(metric.OutputTokens)
		generation += float64(metric.GenerationMs)
		key := metric.BucketTs - metric.BucketTs%bucket
		count := counts[key]
		count[0] += float64(metric.RequestCount)
		count[1] += float64(metric.SuccessCount)
		counts[key] = count
	}
	if requests > 0 {
		rate, average := 100*successes/requests, latency/requests
		group.SuccessRate, group.AvgLatencyMS = &rate, &average
	}
	if generation > 0 {
		speed := tokens * 1000 / generation
		group.TokensPerSecond = &speed
	}
	for ts := start; ts < end; ts += bucket {
		point := StatusPoint{Ts: ts}
		if count := counts[ts]; count[0] > 0 {
			value := 100 * count[1] / count[0]
			point.Value = &value
		}
		group.Series = append(group.Series, point)
	}
}

func ReadPublicStatusThumbnail(id string) ([]byte, error) {
	if len(id) != 32 {
		return nil, ErrStatusPageNotFound
	}
	if _, err := hex.DecodeString(id); err != nil {
		return nil, ErrStatusPageNotFound
	}
	settings, err := model.GetQualitySettings()
	if err != nil {
		return nil, err
	}
	if !settings.StatusPageEnabled {
		return nil, ErrStatusPageNotFound
	}
	run, err := model.GetQualityRunByPublicID(id)
	if err != nil {
		return nil, ErrStatusPageNotFound
	}
	profiles, err := model.ListQualityProfiles()
	if err != nil {
		return nil, err
	}
	published := false
	for _, profile := range profiles {
		if profile.ChannelID == run.ChannelID && profile.PublishSample && profile.PublicAlias != "" {
			published = true
			break
		}
	}
	if !published {
		return nil, ErrStatusPageNotFound
	}
	channel, err := model.GetChannelById(run.ChannelID, false)
	if err != nil {
		return nil, ErrStatusPageNotFound
	}
	groups := []string{}
	for _, group := range settings.PublicGroups {
		if slices.Contains(channel.GetGroups(), group.Name) {
			groups = append(groups, group.Name)
		}
	}
	allowed, err := model.QualityRunPublishedInGroups(run.ID, groups)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrStatusPageNotFound
	}
	return ReadQualityArtifact(run, "thumbnail")
}
