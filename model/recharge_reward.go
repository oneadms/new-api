package model

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	rechargeRewardConfigID       = 1
	maxRewardConfigItems         = 100
	maxGroupPassDurationMinutes  = 24 * 60
	maxGroupPassValidityDays     = 3650
	maxRewardQuantity            = 100
	maxLotteryDrawsPerRecharge   = 100
	lotteryProbabilityBasisPoint = 10000
)

const (
	GroupPassStatusUnused  = "unused"
	GroupPassStatusActive  = "active"
	GroupPassStatusExpired = "expired"

	LotteryPrizeTypeQuota     = "quota"
	LotteryPrizeTypeGroupPass = "group_pass"
	LotteryPrizeTypeNone      = "none"
)

var (
	ErrRechargeRewardConfigConflict = errors.New("奖励配置已被其他管理员更新，请刷新后重试")
	ErrGroupPassDisabled            = errors.New("速通卡功能未启用")
	ErrGroupPassNotFound            = errors.New("速通卡不存在")
	ErrGroupPassUnavailable         = errors.New("速通卡已使用或已过期")
	ErrGroupPassAlreadyActive       = errors.New("同一分组已有生效中的速通卡")
	ErrRechargeLotteryDisabled      = errors.New("充值抽奖未启用")
	ErrNoLotteryChance              = errors.New("暂无可用抽奖次数")
	ErrLotteryPrizeUnavailable      = errors.New("抽奖奖品配置不可用")
)

var rewardConfigIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// RechargeRewardConfig is the single persisted campaign configuration row.
// JSON arrays are stored as TEXT so the schema works on SQLite, MySQL, and PostgreSQL.
type RechargeRewardConfig struct {
	Id                      int    `json:"id" gorm:"primaryKey"`
	Version                 int64  `json:"version" gorm:"not null"`
	GroupPassEnabled        bool   `json:"group_pass_enabled" gorm:"not null"`
	LotteryEnabled          bool   `json:"lottery_enabled" gorm:"not null"`
	LotteryMinRechargeQuota int64  `json:"lottery_min_recharge_quota" gorm:"not null"`
	LotteryDrawsPerRecharge int    `json:"lottery_draws_per_recharge" gorm:"not null"`
	GroupPassTemplatesJSON  string `json:"-" gorm:"column:group_pass_templates;type:text;not null"`
	RechargeRewardRulesJSON string `json:"-" gorm:"column:recharge_reward_rules;type:text;not null"`
	LotteryPrizesJSON       string `json:"-" gorm:"column:lottery_prizes;type:text;not null"`
	UpdatedAt               int64  `json:"updated_at" gorm:"not null"`
}

type GroupPassTemplate struct {
	Id              string `json:"id"`
	Name            string `json:"name"`
	GroupName       string `json:"group_name"`
	DurationMinutes int    `json:"duration_minutes"`
	ValidDays       int    `json:"valid_days"`
	Enabled         bool   `json:"enabled"`
}

type RechargeRewardRule struct {
	Id               string `json:"id"`
	Name             string `json:"name"`
	MinRechargeQuota int64  `json:"min_recharge_quota"`
	TemplateId       string `json:"template_id"`
	Quantity         int    `json:"quantity"`
	Enabled          bool   `json:"enabled"`
}

type RechargeLotteryPrize struct {
	Id             string `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	ProbabilityBps int    `json:"probability_bps"`
	MinQuota       int64  `json:"min_quota"`
	MaxQuota       int64  `json:"max_quota"`
	TemplateId     string `json:"template_id"`
	Quantity       int    `json:"quantity"`
	Enabled        bool   `json:"enabled"`
}

type RechargeRewardSettings struct {
	Version                 int64                  `json:"version"`
	GroupPassEnabled        bool                   `json:"group_pass_enabled"`
	LotteryEnabled          bool                   `json:"lottery_enabled"`
	LotteryMinRechargeQuota int64                  `json:"lottery_min_recharge_quota"`
	LotteryDrawsPerRecharge int                    `json:"lottery_draws_per_recharge"`
	GroupPassTemplates      []GroupPassTemplate    `json:"group_pass_templates"`
	RechargeRewardRules     []RechargeRewardRule   `json:"recharge_reward_rules"`
	LotteryPrizes           []RechargeLotteryPrize `json:"lottery_prizes"`
	UpdatedAt               int64                  `json:"updated_at"`
}

// UserGroupPass stores a grant-time snapshot. Later campaign edits cannot
// silently change a card the user has already earned.
type UserGroupPass struct {
	Id              int    `json:"id" gorm:"primaryKey"`
	UserId          int    `json:"user_id" gorm:"not null;index:idx_user_group_pass_status,priority:1"`
	TemplateId      string `json:"template_id" gorm:"type:varchar(64);not null"`
	Name            string `json:"name" gorm:"type:varchar(100);not null"`
	GroupName       string `json:"group_name" gorm:"type:varchar(64);not null;index:idx_user_group_pass_status,priority:3"`
	DurationMinutes int    `json:"duration_minutes" gorm:"not null"`
	Status          string `json:"status" gorm:"type:varchar(16);not null;index:idx_user_group_pass_status,priority:2"`
	ExpiresAt       int64  `json:"expires_at" gorm:"not null;index"`
	ActivatedAt     int64  `json:"activated_at" gorm:"not null"`
	ActiveUntil     int64  `json:"active_until" gorm:"not null;index"`
	SourceType      string `json:"source_type" gorm:"type:varchar(32);not null"`
	SourceId        string `json:"source_id" gorm:"type:varchar(191);not null;uniqueIndex"`
	CreatedAt       int64  `json:"created_at" gorm:"not null"`
}

// RechargeRewardEvent is both the idempotency record for a completed top-up
// and the draw-chance ledger for that payment.
type RechargeRewardEvent struct {
	Id            int   `json:"id" gorm:"primaryKey"`
	TopUpId       int   `json:"topup_id" gorm:"not null;uniqueIndex"`
	UserId        int   `json:"user_id" gorm:"not null;index"`
	RechargeQuota int64 `json:"recharge_quota" gorm:"not null"`
	GrantedCards  int   `json:"granted_cards" gorm:"not null"`
	LotteryDraws  int   `json:"lottery_draws" gorm:"not null"`
	UsedDraws     int   `json:"used_draws" gorm:"not null"`
	ConfigVersion int64 `json:"config_version" gorm:"not null"`
	CreatedAt     int64 `json:"created_at" gorm:"not null"`
}

type RechargeLotteryDraw struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	UserId         int    `json:"user_id" gorm:"not null;index"`
	RewardEventId  int    `json:"reward_event_id" gorm:"not null;uniqueIndex:idx_reward_event_draw,priority:1"`
	DrawIndex      int    `json:"draw_index" gorm:"not null;uniqueIndex:idx_reward_event_draw,priority:2"`
	PrizeId        string `json:"prize_id" gorm:"type:varchar(64);not null"`
	PrizeName      string `json:"prize_name" gorm:"type:varchar(100);not null"`
	PrizeType      string `json:"prize_type" gorm:"type:varchar(24);not null"`
	ProbabilityBps int    `json:"probability_bps" gorm:"not null"`
	Roll           int    `json:"-" gorm:"not null"`
	QuotaAwarded   int64  `json:"quota_awarded" gorm:"not null"`
	GroupPassId    int    `json:"group_pass_id" gorm:"not null"`
	GroupPassCount int    `json:"group_pass_count" gorm:"not null"`
	ConfigVersion  int64  `json:"config_version" gorm:"not null"`
	CreatedAt      int64  `json:"created_at" gorm:"not null"`
}

type RechargeRewardSelf struct {
	GroupPassEnabled bool                   `json:"group_pass_enabled"`
	LotteryEnabled   bool                   `json:"lottery_enabled"`
	AvailableDraws   int                    `json:"available_draws"`
	GroupPasses      []UserGroupPass        `json:"group_passes"`
	RecentDraws      []RechargeLotteryDraw  `json:"recent_draws"`
	LotteryPrizes    []RechargeLotteryPrize `json:"lottery_prizes"`
}

type GroupPassGrantRequest struct {
	UserId     int    `json:"user_id"`
	TemplateId string `json:"template_id"`
	Quantity   int    `json:"quantity"`
	ExpiresAt  int64  `json:"expires_at"`
}

type RechargeLotteryDrawResult struct {
	Draw      RechargeLotteryDraw `json:"draw"`
	GroupPass *UserGroupPass      `json:"group_pass,omitempty"`
}

func defaultRechargeRewardSettings() RechargeRewardSettings {
	return RechargeRewardSettings{
		Version:             0,
		GroupPassTemplates:  []GroupPassTemplate{},
		RechargeRewardRules: []RechargeRewardRule{},
		LotteryPrizes:       []RechargeLotteryPrize{},
	}
}

func decodeRechargeRewardConfig(config *RechargeRewardConfig) (RechargeRewardSettings, error) {
	settings := defaultRechargeRewardSettings()
	if config == nil {
		return settings, nil
	}
	settings.Version = config.Version
	settings.GroupPassEnabled = config.GroupPassEnabled
	settings.LotteryEnabled = config.LotteryEnabled
	settings.LotteryMinRechargeQuota = config.LotteryMinRechargeQuota
	settings.LotteryDrawsPerRecharge = config.LotteryDrawsPerRecharge
	settings.UpdatedAt = config.UpdatedAt
	if strings.TrimSpace(config.GroupPassTemplatesJSON) != "" {
		if err := common.UnmarshalJsonStr(config.GroupPassTemplatesJSON, &settings.GroupPassTemplates); err != nil {
			return RechargeRewardSettings{}, fmt.Errorf("invalid group pass templates: %w", err)
		}
	}
	if strings.TrimSpace(config.RechargeRewardRulesJSON) != "" {
		if err := common.UnmarshalJsonStr(config.RechargeRewardRulesJSON, &settings.RechargeRewardRules); err != nil {
			return RechargeRewardSettings{}, fmt.Errorf("invalid recharge reward rules: %w", err)
		}
	}
	if strings.TrimSpace(config.LotteryPrizesJSON) != "" {
		if err := common.UnmarshalJsonStr(config.LotteryPrizesJSON, &settings.LotteryPrizes); err != nil {
			return RechargeRewardSettings{}, fmt.Errorf("invalid lottery prizes: %w", err)
		}
	}
	return settings, nil
}

func getRechargeRewardSettingsTx(tx *gorm.DB) (RechargeRewardSettings, error) {
	config := &RechargeRewardConfig{}
	err := tx.Where("id = ?", rechargeRewardConfigID).First(config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return defaultRechargeRewardSettings(), nil
	}
	if err != nil {
		return RechargeRewardSettings{}, err
	}
	return decodeRechargeRewardConfig(config)
}

func GetRechargeRewardSettings() (RechargeRewardSettings, error) {
	return getRechargeRewardSettingsTx(DB)
}

func normalizeAndValidateRechargeRewardSettings(settings RechargeRewardSettings) (RechargeRewardSettings, error) {
	if len(settings.GroupPassTemplates) > maxRewardConfigItems || len(settings.RechargeRewardRules) > maxRewardConfigItems || len(settings.LotteryPrizes) > maxRewardConfigItems {
		return RechargeRewardSettings{}, errors.New("每类奖励配置最多 100 项")
	}
	if settings.LotteryMinRechargeQuota < 0 || settings.LotteryMinRechargeQuota > int64(common.MaxQuota) {
		return RechargeRewardSettings{}, errors.New("抽奖最低充值额度超出有效范围")
	}
	if settings.LotteryDrawsPerRecharge < 0 || settings.LotteryDrawsPerRecharge > maxLotteryDrawsPerRecharge {
		return RechargeRewardSettings{}, errors.New("每笔充值抽奖次数必须在 0 到 100 之间")
	}

	templates := make(map[string]GroupPassTemplate, len(settings.GroupPassTemplates))
	for i := range settings.GroupPassTemplates {
		template := &settings.GroupPassTemplates[i]
		template.Id = strings.TrimSpace(template.Id)
		template.Name = strings.TrimSpace(template.Name)
		template.GroupName = strings.TrimSpace(template.GroupName)
		if !rewardConfigIDPattern.MatchString(template.Id) {
			return RechargeRewardSettings{}, errors.New("速通卡模板 ID 格式无效")
		}
		if _, exists := templates[template.Id]; exists {
			return RechargeRewardSettings{}, errors.New("速通卡模板 ID 不能重复")
		}
		if template.Name == "" || len(template.Name) > 100 {
			return RechargeRewardSettings{}, errors.New("速通卡模板名称长度必须为 1 到 100 个字符")
		}
		if template.GroupName == "" || template.GroupName == "auto" || len(template.GroupName) > 64 || !ratio_setting.ContainsGroupRatio(template.GroupName) {
			return RechargeRewardSettings{}, fmt.Errorf("速通卡模板分组 %q 不存在或不可用", template.GroupName)
		}
		if template.DurationMinutes < 1 || template.DurationMinutes > maxGroupPassDurationMinutes {
			return RechargeRewardSettings{}, errors.New("速通卡生效时长必须在 1 分钟到 24 小时之间")
		}
		if template.ValidDays < 1 || template.ValidDays > maxGroupPassValidityDays {
			return RechargeRewardSettings{}, errors.New("速通卡有效期必须在 1 到 3650 天之间")
		}
		templates[template.Id] = *template
	}

	ruleIDs := make(map[string]struct{}, len(settings.RechargeRewardRules))
	for i := range settings.RechargeRewardRules {
		rule := &settings.RechargeRewardRules[i]
		rule.Id = strings.TrimSpace(rule.Id)
		rule.Name = strings.TrimSpace(rule.Name)
		rule.TemplateId = strings.TrimSpace(rule.TemplateId)
		if !rewardConfigIDPattern.MatchString(rule.Id) {
			return RechargeRewardSettings{}, errors.New("充值奖励规则 ID 格式无效")
		}
		if _, exists := ruleIDs[rule.Id]; exists {
			return RechargeRewardSettings{}, errors.New("充值奖励规则 ID 不能重复")
		}
		ruleIDs[rule.Id] = struct{}{}
		if rule.Name == "" || len(rule.Name) > 100 {
			return RechargeRewardSettings{}, errors.New("充值奖励规则名称长度必须为 1 到 100 个字符")
		}
		if rule.MinRechargeQuota < 1 || rule.MinRechargeQuota > int64(common.MaxQuota) {
			return RechargeRewardSettings{}, errors.New("充值奖励门槛超出有效范围")
		}
		if rule.Quantity < 1 || rule.Quantity > maxRewardQuantity {
			return RechargeRewardSettings{}, errors.New("单笔充值发卡数量必须在 1 到 100 之间")
		}
		template, exists := templates[rule.TemplateId]
		if !exists {
			return RechargeRewardSettings{}, fmt.Errorf("充值奖励规则引用了不存在的模板 %q", rule.TemplateId)
		}
		if rule.Enabled && !template.Enabled {
			return RechargeRewardSettings{}, fmt.Errorf("启用的充值奖励规则引用了未启用的模板 %q", rule.TemplateId)
		}
	}

	prizeIDs := make(map[string]struct{}, len(settings.LotteryPrizes))
	totalProbability := 0
	enabledPrizeCount := 0
	for i := range settings.LotteryPrizes {
		prize := &settings.LotteryPrizes[i]
		prize.Id = strings.TrimSpace(prize.Id)
		prize.Name = strings.TrimSpace(prize.Name)
		prize.Type = strings.TrimSpace(prize.Type)
		prize.TemplateId = strings.TrimSpace(prize.TemplateId)
		if !rewardConfigIDPattern.MatchString(prize.Id) {
			return RechargeRewardSettings{}, errors.New("抽奖奖品 ID 格式无效")
		}
		if _, exists := prizeIDs[prize.Id]; exists {
			return RechargeRewardSettings{}, errors.New("抽奖奖品 ID 不能重复")
		}
		prizeIDs[prize.Id] = struct{}{}
		if prize.Name == "" || len(prize.Name) > 100 {
			return RechargeRewardSettings{}, errors.New("抽奖奖品名称长度必须为 1 到 100 个字符")
		}
		if prize.ProbabilityBps < 0 || prize.ProbabilityBps > lotteryProbabilityBasisPoint {
			return RechargeRewardSettings{}, errors.New("抽奖概率必须在 0% 到 100% 之间")
		}
		switch prize.Type {
		case LotteryPrizeTypeQuota:
			if prize.MinQuota < 1 || prize.MaxQuota < prize.MinQuota || prize.MaxQuota > int64(common.MaxQuota) {
				return RechargeRewardSettings{}, errors.New("额度奖品范围无效")
			}
			prize.TemplateId = ""
			prize.Quantity = 0
		case LotteryPrizeTypeGroupPass:
			template, exists := templates[prize.TemplateId]
			if !exists {
				return RechargeRewardSettings{}, fmt.Errorf("抽奖奖品引用了不存在的模板 %q", prize.TemplateId)
			}
			if prize.Enabled && !template.Enabled {
				return RechargeRewardSettings{}, fmt.Errorf("启用的抽奖奖品引用了未启用的模板 %q", prize.TemplateId)
			}
			if prize.Quantity < 1 || prize.Quantity > maxRewardQuantity {
				return RechargeRewardSettings{}, errors.New("抽奖发卡数量必须在 1 到 100 之间")
			}
			prize.MinQuota = 0
			prize.MaxQuota = 0
		default:
			return RechargeRewardSettings{}, errors.New("抽奖奖品类型无效")
		}
		if prize.Enabled {
			enabledPrizeCount++
			if prize.ProbabilityBps <= 0 {
				return RechargeRewardSettings{}, errors.New("启用的抽奖奖品概率必须大于 0")
			}
			totalProbability += prize.ProbabilityBps
			if totalProbability > lotteryProbabilityBasisPoint {
				return RechargeRewardSettings{}, errors.New("启用奖品的总概率不能超过 100%")
			}
		}
	}
	if settings.LotteryEnabled {
		if settings.LotteryMinRechargeQuota < 1 || settings.LotteryDrawsPerRecharge < 1 {
			return RechargeRewardSettings{}, errors.New("启用抽奖时必须设置有效的充值门槛和抽奖次数")
		}
		if enabledPrizeCount == 0 {
			return RechargeRewardSettings{}, errors.New("启用抽奖时至少需要一个启用的奖品")
		}
	}
	return settings, nil
}

func SaveRechargeRewardSettings(settings RechargeRewardSettings) (RechargeRewardSettings, error) {
	normalized, err := normalizeAndValidateRechargeRewardSettings(settings)
	if err != nil {
		return RechargeRewardSettings{}, err
	}
	templatesJSON, err := common.Marshal(normalized.GroupPassTemplates)
	if err != nil {
		return RechargeRewardSettings{}, err
	}
	rulesJSON, err := common.Marshal(normalized.RechargeRewardRules)
	if err != nil {
		return RechargeRewardSettings{}, err
	}
	prizesJSON, err := common.Marshal(normalized.LotteryPrizes)
	if err != nil {
		return RechargeRewardSettings{}, err
	}

	var saved RechargeRewardSettings
	err = DB.Transaction(func(tx *gorm.DB) error {
		config := &RechargeRewardConfig{}
		err := lockForUpdate(tx).Where("id = ?", rechargeRewardConfigID).First(config).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if normalized.Version != 0 {
				return ErrRechargeRewardConfigConflict
			}
			config = &RechargeRewardConfig{Id: rechargeRewardConfigID, Version: 1}
		} else if err != nil {
			return err
		} else {
			if normalized.Version != config.Version {
				return ErrRechargeRewardConfigConflict
			}
			config.Version++
		}

		config.GroupPassEnabled = normalized.GroupPassEnabled
		config.LotteryEnabled = normalized.LotteryEnabled
		config.LotteryMinRechargeQuota = normalized.LotteryMinRechargeQuota
		config.LotteryDrawsPerRecharge = normalized.LotteryDrawsPerRecharge
		config.GroupPassTemplatesJSON = string(templatesJSON)
		config.RechargeRewardRulesJSON = string(rulesJSON)
		config.LotteryPrizesJSON = string(prizesJSON)
		config.UpdatedAt = common.GetTimestamp()
		if err := tx.Save(config).Error; err != nil {
			return err
		}
		decoded, err := decodeRechargeRewardConfig(config)
		if err != nil {
			return err
		}
		saved = decoded
		return nil
	})
	return saved, err
}

func findGroupPassTemplate(settings RechargeRewardSettings, templateId string, requireEnabled bool) (GroupPassTemplate, error) {
	for _, template := range settings.GroupPassTemplates {
		if template.Id == templateId {
			if requireEnabled && !template.Enabled {
				return GroupPassTemplate{}, errors.New("速通卡模板未启用")
			}
			return template, nil
		}
	}
	return GroupPassTemplate{}, errors.New("速通卡模板不存在")
}

func grantGroupPassTx(tx *gorm.DB, userId int, template GroupPassTemplate, expiresAt int64, sourceType, sourceId string) (*UserGroupPass, error) {
	now := common.GetTimestamp()
	if expiresAt <= now {
		return nil, errors.New("速通卡过期时间必须晚于当前时间")
	}
	pass := &UserGroupPass{
		UserId:          userId,
		TemplateId:      template.Id,
		Name:            template.Name,
		GroupName:       template.GroupName,
		DurationMinutes: template.DurationMinutes,
		Status:          GroupPassStatusUnused,
		ExpiresAt:       expiresAt,
		SourceType:      sourceType,
		SourceId:        sourceId,
		CreatedAt:       now,
	}
	if err := tx.Create(pass).Error; err != nil {
		return nil, err
	}
	return pass, nil
}

func GrantUserGroupPasses(request GroupPassGrantRequest) ([]UserGroupPass, error) {
	if request.UserId <= 0 {
		return nil, errors.New("用户 ID 无效")
	}
	if request.Quantity < 1 || request.Quantity > maxRewardQuantity {
		return nil, errors.New("发放数量必须在 1 到 100 之间")
	}
	settings, err := GetRechargeRewardSettings()
	if err != nil {
		return nil, err
	}
	template, err := findGroupPassTemplate(settings, strings.TrimSpace(request.TemplateId), true)
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	expiresAt := request.ExpiresAt
	if expiresAt == 0 {
		expiresAt = now + int64(template.ValidDays)*24*60*60
	}
	if expiresAt <= now || expiresAt > now+int64(maxGroupPassValidityDays)*24*60*60 {
		return nil, errors.New("过期时间必须在当前时间之后且不超过 3650 天")
	}

	passes := make([]UserGroupPass, 0, request.Quantity)
	err = DB.Transaction(func(tx *gorm.DB) error {
		var userCount int64
		if err := tx.Model(&User{}).Where("id = ?", request.UserId).Count(&userCount).Error; err != nil {
			return err
		}
		if userCount == 0 {
			return errors.New("用户不存在")
		}
		grantBatchId := uuid.NewString()
		for i := 0; i < request.Quantity; i++ {
			pass, err := grantGroupPassTx(tx, request.UserId, template, expiresAt, "admin", fmt.Sprintf("manual:%s:%d", grantBatchId, i))
			if err != nil {
				return err
			}
			passes = append(passes, *pass)
		}
		return nil
	})
	return passes, err
}

func applyRechargeRewardsTx(tx *gorm.DB, topUp *TopUp, rechargeQuota int64) error {
	if topUp == nil || topUp.Id <= 0 || topUp.UserId <= 0 || rechargeQuota <= 0 {
		return errors.New("充值奖励事件参数无效")
	}
	var existing int64
	if err := tx.Model(&RechargeRewardEvent{}).Where("top_up_id = ?", topUp.Id).Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	settings, err := getRechargeRewardSettingsTx(tx)
	if err != nil {
		return err
	}

	templates := make(map[string]GroupPassTemplate, len(settings.GroupPassTemplates))
	for _, template := range settings.GroupPassTemplates {
		templates[template.Id] = template
	}
	now := common.GetTimestamp()
	grantedCards := 0
	if settings.GroupPassEnabled {
		for _, rule := range settings.RechargeRewardRules {
			if !rule.Enabled || rechargeQuota < rule.MinRechargeQuota {
				continue
			}
			template, exists := templates[rule.TemplateId]
			if !exists || !template.Enabled {
				continue
			}
			expiresAt := now + int64(template.ValidDays)*24*60*60
			for i := 0; i < rule.Quantity; i++ {
				sourceId := fmt.Sprintf("topup:%d:rule:%s:%d", topUp.Id, rule.Id, i)
				if _, err := grantGroupPassTx(tx, topUp.UserId, template, expiresAt, "recharge_rule", sourceId); err != nil {
					return err
				}
				grantedCards++
			}
		}
	}

	lotteryDraws := 0
	if settings.LotteryEnabled && rechargeQuota >= settings.LotteryMinRechargeQuota {
		lotteryDraws = settings.LotteryDrawsPerRecharge
	}
	event := &RechargeRewardEvent{
		TopUpId:       topUp.Id,
		UserId:        topUp.UserId,
		RechargeQuota: rechargeQuota,
		GrantedCards:  grantedCards,
		LotteryDraws:  lotteryDraws,
		ConfigVersion: settings.Version,
		CreatedAt:     now,
	}
	return tx.Create(event).Error
}

func ActivateUserGroupPass(userId, passId int) (*UserGroupPass, error) {
	if userId <= 0 || passId <= 0 {
		return nil, ErrGroupPassNotFound
	}
	var activated UserGroupPass
	err := DB.Transaction(func(tx *gorm.DB) error {
		settings, err := getRechargeRewardSettingsTx(tx)
		if err != nil {
			return err
		}
		if !settings.GroupPassEnabled {
			return ErrGroupPassDisabled
		}
		var user User
		if err := lockForUpdate(tx).Select("id").Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		pass := &UserGroupPass{}
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", passId, userId).First(pass).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrGroupPassNotFound
			}
			return err
		}
		now := common.GetTimestamp()
		if pass.Status != GroupPassStatusUnused || pass.ExpiresAt <= now {
			return ErrGroupPassUnavailable
		}
		var activeCount int64
		if err := tx.Model(&UserGroupPass{}).
			Where("user_id = ? AND group_name = ? AND status = ? AND active_until > ?", userId, pass.GroupName, GroupPassStatusActive, now).
			Count(&activeCount).Error; err != nil {
			return err
		}
		if activeCount > 0 {
			return ErrGroupPassAlreadyActive
		}
		activeUntil := now + int64(pass.DurationMinutes)*60
		result := tx.Model(&UserGroupPass{}).
			Where("id = ? AND user_id = ? AND status = ? AND expires_at > ?", pass.Id, userId, GroupPassStatusUnused, now).
			Updates(map[string]interface{}{
				"status":       GroupPassStatusActive,
				"activated_at": now,
				"active_until": activeUntil,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrGroupPassUnavailable
		}
		pass.Status = GroupPassStatusActive
		pass.ActivatedAt = now
		pass.ActiveUntil = activeUntil
		activated = *pass
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &activated, nil
}

func GetActiveGroupPassAccess(userId int) (map[string]int64, error) {
	access := make(map[string]int64)
	if userId <= 0 {
		return access, nil
	}
	settings, err := GetRechargeRewardSettings()
	if err != nil {
		return nil, err
	}
	if !settings.GroupPassEnabled {
		return access, nil
	}
	var rows []struct {
		GroupName   string `gorm:"column:group_name"`
		ActiveUntil int64  `gorm:"column:active_until"`
	}
	if err := DB.Model(&UserGroupPass{}).
		Select("group_name, MAX(active_until) AS active_until").
		Where("user_id = ? AND status = ? AND active_until > ?", userId, GroupPassStatusActive, common.GetTimestamp()).
		Group("group_name").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if ratio_setting.ContainsGroupRatio(row.GroupName) {
			access[row.GroupName] = row.ActiveUntil
		}
	}
	return access, nil
}

func selectLotteryPrize(prizes []RechargeLotteryPrize, roll int) *RechargeLotteryPrize {
	if roll < 0 || roll >= lotteryProbabilityBasisPoint {
		return nil
	}
	cumulative := 0
	for i := range prizes {
		prize := &prizes[i]
		if !prize.Enabled || prize.ProbabilityBps <= 0 {
			continue
		}
		cumulative += prize.ProbabilityBps
		if roll < cumulative {
			return prize
		}
	}
	return nil
}

func secureRandomInt(maxExclusive int64) (int64, error) {
	if maxExclusive <= 0 {
		return 0, errors.New("随机数范围无效")
	}
	n, err := crand.Int(crand.Reader, big.NewInt(maxExclusive))
	if err != nil {
		return 0, err
	}
	return n.Int64(), nil
}

func DrawRechargeLottery(userId int) (*RechargeLotteryDrawResult, error) {
	if userId <= 0 {
		return nil, ErrNoLotteryChance
	}
	var result RechargeLotteryDrawResult
	var quotaAwarded int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		settings, err := getRechargeRewardSettingsTx(tx)
		if err != nil {
			return err
		}
		if !settings.LotteryEnabled {
			return ErrRechargeLotteryDisabled
		}
		if _, err := normalizeAndValidateRechargeRewardSettings(settings); err != nil {
			return ErrLotteryPrizeUnavailable
		}

		event := &RechargeRewardEvent{}
		if err := lockForUpdate(tx).
			Where("user_id = ? AND used_draws < lottery_draws", userId).
			Order("id asc").
			First(event).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNoLotteryChance
			}
			return err
		}
		rollValue, err := secureRandomInt(lotteryProbabilityBasisPoint)
		if err != nil {
			return errors.New("生成安全抽奖随机数失败")
		}
		roll := int(rollValue)
		prize := selectLotteryPrize(settings.LotteryPrizes, roll)
		draw := RechargeLotteryDraw{
			UserId:        userId,
			RewardEventId: event.Id,
			DrawIndex:     event.UsedDraws + 1,
			PrizeType:     LotteryPrizeTypeNone,
			PrizeName:     "No prize",
			Roll:          roll,
			ConfigVersion: settings.Version,
			CreatedAt:     common.GetTimestamp(),
		}
		if prize != nil {
			draw.PrizeId = prize.Id
			draw.PrizeName = prize.Name
			draw.PrizeType = prize.Type
			draw.ProbabilityBps = prize.ProbabilityBps
			switch prize.Type {
			case LotteryPrizeTypeQuota:
				rangeSize := prize.MaxQuota - prize.MinQuota + 1
				offset, err := secureRandomInt(rangeSize)
				if err != nil {
					return errors.New("生成安全额度随机数失败")
				}
				quotaAwarded = prize.MinQuota + offset
				if quotaAwarded <= 0 || quotaAwarded > int64(common.MaxQuota) {
					return ErrLotteryPrizeUnavailable
				}
				if err := creditUserQuotaTx(tx, userId, quotaAwarded); err != nil {
					return err
				}
				draw.QuotaAwarded = quotaAwarded
			case LotteryPrizeTypeGroupPass:
				template, err := findGroupPassTemplate(settings, prize.TemplateId, true)
				if err != nil {
					return ErrLotteryPrizeUnavailable
				}
				expiresAt := common.GetTimestamp() + int64(template.ValidDays)*24*60*60
				for i := 0; i < prize.Quantity; i++ {
					sourceId := fmt.Sprintf("lottery:%d:%d:%d", event.Id, draw.DrawIndex, i)
					pass, err := grantGroupPassTx(tx, userId, template, expiresAt, "lottery", sourceId)
					if err != nil {
						return err
					}
					if i == 0 {
						draw.GroupPassId = pass.Id
						result.GroupPass = pass
					}
				}
				draw.GroupPassCount = prize.Quantity
			default:
				return ErrLotteryPrizeUnavailable
			}
		}

		if err := tx.Create(&draw).Error; err != nil {
			return err
		}
		updateEvent := tx.Model(&RechargeRewardEvent{}).
			Where("id = ? AND used_draws = ? AND used_draws < lottery_draws", event.Id, event.UsedDraws).
			Update("used_draws", event.UsedDraws+1)
		if updateEvent.Error != nil {
			return updateEvent.Error
		}
		if updateEvent.RowsAffected != 1 {
			return ErrNoLotteryChance
		}
		result.Draw = draw
		return nil
	})
	if err != nil {
		return nil, err
	}
	if quotaAwarded > 0 {
		cacheUserQuotaCredit(userId, quotaAwarded)
	}
	return &result, nil
}

func GetUserRechargeRewards(userId int) (*RechargeRewardSelf, error) {
	if userId <= 0 {
		return nil, errors.New("用户 ID 无效")
	}
	settings, err := GetRechargeRewardSettings()
	if err != nil {
		return nil, err
	}
	response := &RechargeRewardSelf{
		GroupPassEnabled: settings.GroupPassEnabled,
		LotteryEnabled:   settings.LotteryEnabled,
		GroupPasses:      []UserGroupPass{},
		RecentDraws:      []RechargeLotteryDraw{},
		LotteryPrizes:    []RechargeLotteryPrize{},
	}
	for _, prize := range settings.LotteryPrizes {
		if prize.Enabled {
			response.LotteryPrizes = append(response.LotteryPrizes, prize)
		}
	}
	if err := DB.Where("user_id = ?", userId).Order("id desc").Limit(200).Find(&response.GroupPasses).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	for i := range response.GroupPasses {
		response.GroupPasses[i].Status = expireGroupPassStatus(response.GroupPasses[i], now)
	}
	if err := DB.Where("user_id = ?", userId).Order("id desc").Limit(20).Find(&response.RecentDraws).Error; err != nil {
		return nil, err
	}
	var drawEvents []RechargeRewardEvent
	if err := DB.Select("lottery_draws", "used_draws").Where("user_id = ? AND used_draws < lottery_draws", userId).Find(&drawEvents).Error; err != nil {
		return nil, err
	}
	for _, event := range drawEvents {
		remaining := event.LotteryDraws - event.UsedDraws
		if remaining > 0 && response.AvailableDraws <= common.MaxQuota-remaining {
			response.AvailableDraws += remaining
		}
	}
	return response, nil
}

// expireGroupPassStatus is intentionally presentation-only: the database row
// keeps its original state for auditability, while callers can classify it by time.
func expireGroupPassStatus(pass UserGroupPass, now time.Time) string {
	timestamp := now.Unix()
	if pass.Status == GroupPassStatusUnused && pass.ExpiresAt <= timestamp {
		return GroupPassStatusExpired
	}
	if pass.Status == GroupPassStatusActive && pass.ActiveUntil <= timestamp {
		return GroupPassStatusExpired
	}
	return pass.Status
}
