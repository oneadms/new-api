package service

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

var qualityPromptKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// QualityValidationError is safe to expose; raw upstream and database errors are not.
type QualityValidationError struct{ Message string }

func (e *QualityValidationError) Error() string { return e.Message }

func GetQualitySettings() (*model.QualitySettings, error) {
	settings, err := model.GetQualitySettings()
	if err != nil {
		return nil, err
	}
	settings.RendererAvailable = QualityRendererAvailable()
	return settings, nil
}

func UpdateQualitySettings(settings model.QualitySettings) (*model.QualitySettings, error) {
	if settings.Version < 1 || settings.IntervalMinutes < 5 || settings.IntervalMinutes > 10080 ||
		settings.Concurrency < 1 || settings.Concurrency > 10 || settings.TimeoutSeconds < 60 || settings.TimeoutSeconds > 1800 ||
		settings.MaxOutputTokens < 1 || settings.MaxOutputTokens > common.MaxTokensLimit ||
		settings.DailyCallLimit < 1 || settings.DailyCallLimit > 1000000 || settings.DailyTokenLimit < 1 || settings.DailyTokenLimit > 1000000000000 ||
		settings.RetentionDays < 1 || settings.RetentionDays > 90 || len(settings.PublicGroups) > 500 {
		return nil, &QualityValidationError{"Invalid quality detection settings"}
	}
	seen := map[string]bool{}
	for i := range settings.PublicGroups {
		group := &settings.PublicGroups[i]
		group.Name = strings.TrimSpace(group.Name)
		group.Alias = strings.TrimSpace(group.Alias)
		if group.Name == "" || group.Name == "auto" || len(group.Name) > 128 || strings.Contains(group.Name, ",") ||
			group.Alias == "" || len(group.Alias) > 128 || seen[group.Name] || !utf8.ValidString(group.Name+group.Alias) {
			return nil, &QualityValidationError{"Invalid public group configuration"}
		}
		seen[group.Name] = true
	}
	if settings.PublicGroups == nil {
		settings.PublicGroups = []model.QualityPublicGroup{}
	}
	encodedGroups, err := common.Marshal(settings.PublicGroups)
	if err != nil {
		return nil, err
	}
	// Stay within MySQL TEXT as well as SQLite/PostgreSQL text storage.
	if len(encodedGroups) > 60000 {
		return nil, &QualityValidationError{"Public group configuration is too large"}
	}
	saved, err := model.SaveQualitySettings(settings)
	if err == nil {
		saved.RendererAvailable = QualityRendererAvailable()
	}
	return saved, err
}

type QualityChannel struct {
	ID                int                         `json:"id"`
	Name              string                      `json:"name"`
	Groups            []string                    `json:"groups"`
	Models            []string                    `json:"models"`
	Status            int                         `json:"status"`
	Supported         bool                        `json:"supported"`
	UnsupportedReason string                      `json:"unsupported_reason"`
	Profile           model.ChannelQualityProfile `json:"profile"`
}

func ListQualityChannels() ([]QualityChannel, error) {
	channels, err := model.GetAllChannels(0, -1, false, true)
	if err != nil {
		return nil, err
	}
	profiles, err := model.ListQualityProfiles()
	if err != nil {
		return nil, err
	}
	byChannel := map[int]model.ChannelQualityProfile{}
	for _, profile := range profiles {
		byChannel[profile.ChannelID] = profile
	}
	executor, _ := GetQualityExecutor()
	result := make([]QualityChannel, 0, len(channels))
	for _, channel := range channels {
		profile, ok := byChannel[channel.Id]
		if !ok {
			profile = model.ChannelQualityProfile{ChannelID: channel.Id, Protocol: "chat"}
			if executor != nil && !executor.Supports(channel.Type, profile.Protocol) {
				for _, protocol := range []string{"responses", "claude"} {
					if executor.Supports(channel.Type, protocol) {
						profile.Protocol = protocol
						break
					}
				}
			}
		}
		reason := qualityChannelUnsupportedReason(channel, profile, executor)
		// A supported channel without a selected model must remain configurable.
		// Choosing that model is explicit; this does not invent a default target.
		if reason == "model_not_configured" {
			reason = ""
		}
		result = append(result, QualityChannel{ID: channel.Id, Name: channel.Name, Groups: channel.GetGroups(), Models: channel.GetModels(),
			Status: channel.Status, Supported: reason == "", UnsupportedReason: reason, Profile: profile})
	}
	return result, nil
}

func qualityChannelUnsupportedReason(channel *model.Channel, profile model.ChannelQualityProfile, executor QualityExecutor) string {
	if executor == nil {
		return "executor_unavailable"
	}
	if !executor.Supports(channel.Type, profile.Protocol) {
		return "unsupported_protocol"
	}
	if channel.Status != common.ChannelStatusEnabled {
		return "channel_disabled"
	}
	if profile.Model == "" {
		return "model_not_configured"
	}
	if !slices.Contains(channel.GetModels(), profile.Model) {
		return "model_unavailable"
	}
	// Non-text models must never be silently treated as HTML-capable targets.
	name := strings.ToLower(profile.Model)
	for _, marker := range []string{"embedding", "whisper", "tts", "dall-e", "imagen", "sora", "veo-", "stable-diffusion", "rerank"} {
		if strings.Contains(name, marker) {
			return "unsupported_model"
		}
	}
	return ""
}

func UpdateQualityChannel(id int, profile model.ChannelQualityProfile) (*model.ChannelQualityProfile, error) {
	profile.Model = strings.TrimSpace(profile.Model)
	profile.PublicAlias = strings.TrimSpace(profile.PublicAlias)
	if id <= 0 || (profile.ChannelID != 0 && profile.ChannelID != id) || profile.Version < 0 || len(profile.Model) > 256 ||
		len(profile.PublicAlias) > 128 || (profile.TimeoutSeconds != 0 && (profile.TimeoutSeconds < 60 || profile.TimeoutSeconds > 1800)) ||
		!slices.Contains([]string{"chat", "responses", "claude"}, profile.Protocol) ||
		!slices.Contains([]string{"", "none", "minimal", "low", "medium", "high", "xhigh"}, profile.ReasoningEffort) ||
		(profile.PublishSample && profile.PublicAlias == "") || !utf8.ValidString(profile.Model+profile.PublicAlias) {
		return nil, &QualityValidationError{"Invalid channel quality profile"}
	}
	channel, err := model.GetChannelById(id, false)
	if err != nil {
		return nil, err
	}
	if profile.Enabled {
		executor, _ := GetQualityExecutor()
		if reason := qualityChannelUnsupportedReason(channel, profile, executor); reason != "" {
			return nil, &QualityValidationError{"Channel does not support the configured detection target: " + reason}
		}
	}
	if profile.Enabled || profile.PromptVersionID != 0 {
		prompts, err := model.ListQualityPrompts()
		if err != nil {
			return nil, err
		}
		valid := false
		for _, prompt := range prompts {
			if prompt.ID == profile.PromptVersionID && !prompt.Archived {
				valid = true
				break
			}
		}
		if !valid {
			return nil, &QualityValidationError{"An active prompt version is required"}
		}
	}
	profile.ChannelID = id
	profile.BaselineID = 0
	return model.SaveQualityProfile(profile)
}

func ListQualityPrompts() ([]model.QualityPromptVersion, error) { return model.ListQualityPrompts() }

func CreateQualityPrompt(key, name, content, rule string, actor int) (*model.QualityPromptVersion, error) {
	name = strings.TrimSpace(name)
	if key == "" {
		key = "custom-" + common.GetUUID()
	}
	if !qualityPromptKeyPattern.MatchString(key) || strings.HasPrefix(key, "builtin-") || name == "" || len(name) > 128 ||
		strings.TrimSpace(content) == "" || len(content) > 16000 || !utf8.ValidString(name+content) || !slices.Contains([]string{"visual", "interactive"}, rule) {
		return nil, &QualityValidationError{"Invalid prompt; use a name, at most 16000 UTF-8 bytes, and a supported rule"}
	}
	// Custom prompts never acquire the trusted builtin interaction rule merely
	// by selecting a label. Their content still requires human review.
	return model.CreateQualityPrompt(model.QualityPromptVersion{Key: key, Name: name, Content: content, Rule: rule,
		RuleVersion: "custom-v1", CreatedBy: actor})
}

func GetQualityRun(id int64) (*model.QualityProbeRun, error) { return model.GetQualityRun(id) }

func SetQualityBaseline(id int64, actor int) error {
	run, err := GetQualityRun(id)
	if err != nil {
		return err
	}
	channel, err := model.GetChannelById(run.ChannelID, false)
	if err != nil {
		return err
	}
	profile, err := model.GetQualityProfile(run.ChannelID)
	if err != nil {
		return err
	}
	settings, err := GetQualitySettings()
	if err != nil {
		return err
	}
	fingerprint, err := QualityConfigFingerprint(channel, *profile, settings)
	if err != nil {
		return err
	}
	if fingerprint != run.ConfigFingerprint {
		return model.ErrQualityConflict
	}
	if _, err := ReadQualityArtifact(run, "html"); err != nil {
		return err
	}
	return model.SetQualityBaseline(id, actor)
}

func ReviewQualityRun(id int64, actor int, result, note string) error {
	if !slices.Contains([]string{"accepted", "rejected"}, result) || len(note) > 4000 || !utf8.ValidString(note) {
		return &QualityValidationError{"Invalid review result or note"}
	}
	return model.ReviewQualityRun(id, actor, result, note)
}

// QualityConfigFingerprint identifies immutable execution inputs without storing
// credentials or raw channel override values in run metadata.
func QualityConfigFingerprint(channel *model.Channel, profile model.ChannelQualityProfile, settings *model.QualitySettings) (string, error) {
	value := struct {
		ChannelType                                                     int
		Model                                                           string
		Protocol                                                        string
		PromptID                                                        int64
		Effort                                                          string
		Timeout                                                         int
		MaxTokens                                                       int
		Mapping, Params, Headers, Setting, Other, BaseURL, Organization *string
		OtherSettings                                                   string
	}{ChannelType: channel.Type, Model: profile.Model, Protocol: profile.Protocol, PromptID: profile.PromptVersionID,
		Effort: profile.ReasoningEffort, Timeout: profile.TimeoutSeconds, MaxTokens: settings.MaxOutputTokens,
		Mapping: channel.ModelMapping, Params: channel.ParamOverride, Headers: channel.HeaderOverride,
		Setting: channel.Setting, Other: &channel.Other, BaseURL: channel.BaseURL, Organization: channel.OpenAIOrganization,
		OtherSettings: channel.OtherSettings}
	if value.Timeout == 0 {
		value.Timeout = settings.TimeoutSeconds
	}
	data, err := common.Marshal(value)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}
