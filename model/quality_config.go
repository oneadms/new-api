package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrQualityConflict    = errors.New("quality configuration or active batch conflict")
	ErrQualityIdempotency = errors.New("idempotency key was already used for another request")
	ErrQualityBudget      = errors.New("daily quality detection budget exhausted")
	ErrQualityCancelled   = errors.New("quality batch cancellation requested")
)

func DefaultQualitySettings() QualitySettings {
	return QualitySettings{ID: 1, Version: 1, IntervalMinutes: 60, Concurrency: 3,
		TimeoutSeconds: 600, MaxOutputTokens: 8192, DailyCallLimit: 500,
		DailyTokenLimit: 5000000, RetentionDays: 14, PublicGroups: []QualityPublicGroup{}, PublicGroupsJSON: "[]"}
}

func GetQualitySettings() (*QualitySettings, error) {
	defaults := DefaultQualitySettings()
	if err := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&defaults).Error; err != nil {
		return nil, err
	}
	var settings QualitySettings
	if err := DB.First(&settings, 1).Error; err != nil {
		return nil, err
	}
	settings.PublicGroups = []QualityPublicGroup{}
	if err := common.UnmarshalJsonStr(settings.PublicGroupsJSON, &settings.PublicGroups); err != nil {
		return nil, err
	}
	return &settings, nil
}

func SaveQualitySettings(settings QualitySettings) (*QualitySettings, error) {
	data, err := common.Marshal(settings.PublicGroups)
	if err != nil {
		return nil, err
	}
	settings.ID = 1
	settings.PublicGroupsJSON = string(data)
	version := settings.Version
	settings.Version++
	result := DB.Model(&QualitySettings{}).Where("id = ? AND version = ?", 1, version).
		Select("*").Omit("id").Updates(&settings)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrQualityConflict
	}
	return &settings, nil
}

func ListQualityProfiles() ([]ChannelQualityProfile, error) {
	profiles := []ChannelQualityProfile{}
	err := DB.Order("channel_id").Find(&profiles).Error
	return profiles, err
}

func GetQualityProfile(channelID int) (*ChannelQualityProfile, error) {
	var profile ChannelQualityProfile
	if err := DB.Where("channel_id = ?", channelID).First(&profile).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func SaveQualityProfile(profile ChannelQualityProfile) (*ChannelQualityProfile, error) {
	if profile.Version == 0 {
		profile.Version = 1
		if err := DB.Create(&profile).Error; err != nil {
			if _, lookupErr := GetQualityProfile(profile.ChannelID); lookupErr == nil {
				return nil, ErrQualityConflict
			}
			return nil, err
		}
		return &profile, nil
	}
	version := profile.Version
	profile.Version++
	result := DB.Model(&ChannelQualityProfile{}).Where("channel_id = ? AND version = ?", profile.ChannelID, version).
		Select("*").Omit("channel_id", "baseline_id").Updates(&profile)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrQualityConflict
	}
	return &profile, nil
}

func EnsureQualityBuiltinPrompts() error {
	prompts := []QualityPromptVersion{
		{Key: "builtin-bicycle", Version: 1, Name: "Animal riding a bicycle", Rule: "visual", RuleVersion: "visual-v1", Builtin: true,
			Content: "Create a complete self-contained HTML page with an inline SVG illustration of a cat riding a bicycle. Show two wheels, the bicycle frame, pedals, and a clearly recognizable cat in a riding pose. Use inline CSS to animate the wheels and pedals. Do not use external resources."},
		{Key: "builtin-interaction", Version: 1, Name: "Button interaction", Rule: "interactive", RuleVersion: "button-v1", Builtin: true,
			Content: "Create a complete self-contained HTML page. Include a button with id=\"toggle-button\" and accessible label \"Toggle message\", and a text element with id=\"message\". Initially the message must be \"Ready\". Clicking the button must change it to \"Done\"; clicking again must change it back to \"Ready\". Support keyboard activation and use inline JavaScript and CSS only."},
	}
	for i := range prompts {
		prompts[i].CreatedAt = time.Now().Unix()
		if err := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&prompts[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

func ListQualityPrompts() ([]QualityPromptVersion, error) {
	if err := EnsureQualityBuiltinPrompts(); err != nil {
		return nil, err
	}
	prompts := []QualityPromptVersion{}
	err := DB.Order("prompt_key, version desc").Find(&prompts).Error
	return prompts, err
}

func CreateQualityPrompt(prompt QualityPromptVersion) (*QualityPromptVersion, error) {
	err := DB.Transaction(func(tx *gorm.DB) error {
		var latest QualityPromptVersion
		err := lockForUpdate(tx).Where("prompt_key = ?", prompt.Key).Order("version desc").First(&latest).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if latest.Builtin {
			return ErrQualityConflict
		}
		prompt.ID = 0
		prompt.Version = latest.Version + 1
		prompt.CreatedAt = time.Now().Unix()
		return tx.Create(&prompt).Error
	})
	return &prompt, err
}

func ArchiveQualityPrompt(id int64, archived bool) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var prompt QualityPromptVersion
		if err := lockForUpdate(tx).First(&prompt, id).Error; err != nil {
			return err
		}
		return tx.Model(&prompt).Update("archived", archived).Error
	})
}

func GetQualityRun(id int64) (*QualityProbeRun, error) {
	var run QualityProbeRun
	if err := DB.First(&run, id).Error; err != nil {
		return nil, err
	}
	if run.GroupsJSON != "" {
		if err := common.UnmarshalJsonStr(run.GroupsJSON, &run.Groups); err != nil {
			return nil, err
		}
	}
	return &run, nil
}

func GetQualityBatch(id int64) (*QualityProbeBatch, []QualityProbeRun, error) {
	var batch QualityProbeBatch
	if err := DB.First(&batch, id).Error; err != nil {
		return nil, nil, err
	}
	runs := []QualityProbeRun{}
	err := DB.Where("batch_id = ?", id).Order("id").Find(&runs).Error
	return &batch, runs, err
}

func ListQualityBatches(page, pageSize int) ([]QualityProbeBatch, int64, error) {
	batches := []QualityProbeBatch{}
	var total int64
	if err := DB.Model(&QualityProbeBatch{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := DB.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&batches).Error
	return batches, total, err
}

func CancelQualityBatch(id int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var batch QualityProbeBatch
		if err := lockForUpdate(tx).First(&batch, id).Error; err != nil {
			return err
		}
		if batch.CompletedAt != 0 {
			return nil
		}
		return tx.Model(&batch).Update("cancel_requested", true).Error
	})
}

func ReviewQualityRun(id int64, actor int, result, note string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var run QualityProbeRun
		if err := lockForUpdate(tx).First(&run, id).Error; err != nil {
			return err
		}
		if run.Status != "completed" || !run.HasOutput {
			return ErrQualityConflict
		}
		now := time.Now().Unix()
		if err := tx.Model(&run).Updates(map[string]any{"review_status": result, "review_note": note, "reviewed_at": now, "reviewed_by": actor}).Error; err != nil {
			return err
		}
		return tx.Create(&QualityReviewAudit{RunID: id, ActorID: actor, Action: result, Note: note, CreatedAt: now}).Error
	})
}

func SetQualityBaseline(id int64, actor int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var run QualityProbeRun
		if err := lockForUpdate(tx).First(&run, id).Error; err != nil {
			return err
		}
		if run.Status != "completed" || !run.HasHTML || run.ReviewStatus != "accepted" {
			return ErrQualityConflict
		}
		var profile ChannelQualityProfile
		if err := lockForUpdate(tx).First(&profile, "channel_id = ?", run.ChannelID).Error; err != nil {
			return err
		}
		if profile.Model != run.Model || profile.PromptVersionID != run.PromptVersionID || profile.Version != run.ProfileVersion {
			return ErrQualityConflict
		}
		if err := tx.Model(&profile).Update("baseline_id", id).Error; err != nil {
			return err
		}
		return tx.Create(&QualityReviewAudit{RunID: id, ActorID: actor, Action: "baseline", CreatedAt: time.Now().Unix()}).Error
	})
}
