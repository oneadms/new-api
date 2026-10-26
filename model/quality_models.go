package model

// QualityPublicGroup explicitly publishes a real channel group under an alias.
type QualityPublicGroup struct {
	Name  string `json:"name"`
	Alias string `json:"alias"`
}

type QualitySettings struct {
	ID                int                  `json:"-" gorm:"primaryKey"`
	Version           int64                `json:"version"`
	Enabled           bool                 `json:"enabled"`
	IntervalMinutes   int                  `json:"interval_minutes"`
	Concurrency       int                  `json:"concurrency"`
	TimeoutSeconds    int                  `json:"timeout_seconds"`
	MaxOutputTokens   int                  `json:"max_output_tokens"`
	DailyCallLimit    int64                `json:"daily_call_limit"`
	DailyTokenLimit   int64                `json:"daily_token_limit"`
	RetentionDays     int                  `json:"retention_days"`
	StatusPageEnabled bool                 `json:"status_page_enabled"`
	PublicGroups      []QualityPublicGroup `json:"public_groups" gorm:"-"`
	PublicGroupsJSON  string               `json:"-" gorm:"type:text"`
	RendererAvailable bool                 `json:"renderer_available" gorm:"-"`
}

type ChannelQualityProfile struct {
	ChannelID       int    `json:"channel_id" gorm:"primaryKey;autoIncrement:false"`
	Enabled         bool   `json:"enabled"`
	Model           string `json:"model" gorm:"type:varchar(256)"`
	Protocol        string `json:"protocol" gorm:"type:varchar(16)"`
	PromptVersionID int64  `json:"prompt_version_id"`
	ReasoningEffort string `json:"reasoning_effort" gorm:"type:varchar(32)"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
	PublicAlias     string `json:"public_alias" gorm:"type:varchar(128)"`
	PublishSample   bool   `json:"publish_sample"`
	Version         int64  `json:"version"`
	BaselineID      int64  `json:"-"`
}

type QualityPromptVersion struct {
	ID          int64  `json:"id" gorm:"primaryKey"`
	Key         string `json:"key" gorm:"column:prompt_key;type:varchar(64);uniqueIndex:idx_quality_prompt_version"`
	Version     int    `json:"version" gorm:"uniqueIndex:idx_quality_prompt_version"`
	Name        string `json:"name" gorm:"type:varchar(128)"`
	Content     string `json:"content" gorm:"type:text"`
	Rule        string `json:"rule" gorm:"type:varchar(32)"`
	Archived    bool   `json:"archived"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
	CreatedBy   int    `json:"-"`
	RuleVersion string `json:"-" gorm:"type:varchar(32)"`
	Builtin     bool   `json:"-"`
}

type QualityProbeBatch struct {
	ID                 int64   `json:"id" gorm:"primaryKey"`
	TaskID             string  `json:"task_id" gorm:"type:varchar(64);uniqueIndex"`
	Trigger            string  `json:"trigger" gorm:"type:varchar(16)"`
	Status             string  `json:"status" gorm:"type:varchar(32);index"`
	Total              int     `json:"total"`
	Completed          int     `json:"completed"`
	Failed             int     `json:"failed"`
	CancelRequested    bool    `json:"cancel_requested"`
	CreatedAt          int64   `json:"created_at" gorm:"bigint;index"`
	CompletedAt        int64   `json:"completed_at" gorm:"bigint"`
	Summary            string  `json:"summary" gorm:"type:varchar(256)"`
	CreatedBy          int     `json:"-"`
	IdempotencyKey     *string `json:"-" gorm:"type:varchar(128);uniqueIndex"`
	RequestFingerprint string  `json:"-" gorm:"type:varchar(64)"`
	SettingsJSON       string  `json:"-" gorm:"type:text"`
	ConfigVersion      int64   `json:"-"`
	BudgetDate         string  `json:"-" gorm:"type:varchar(10)"`
	BudgetReserved     bool    `json:"-"`
}

type QualityProbeRun struct {
	ID              int64  `json:"id" gorm:"primaryKey"`
	PublicID        string `json:"public_id" gorm:"type:varchar(64);uniqueIndex"`
	BatchID         int64  `json:"batch_id" gorm:"uniqueIndex:idx_quality_batch_channel;index"`
	ChannelID       int    `json:"channel_id" gorm:"uniqueIndex:idx_quality_batch_channel;index:idx_quality_channel_created"`
	ChannelName     string `json:"channel_name" gorm:"type:varchar(256)"`
	Model           string `json:"model" gorm:"type:varchar(256)"`
	MappedModel     string `json:"mapped_model" gorm:"type:varchar(256)"`
	PromptVersionID int64  `json:"prompt_version_id"`
	PromptName      string `json:"prompt_name" gorm:"type:varchar(128)"`
	Status          string `json:"status" gorm:"type:varchar(32);index"`
	QualityResult   string `json:"quality_result" gorm:"type:varchar(32)"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint;index:idx_quality_channel_created"`
	StartedAt       int64  `json:"started_at" gorm:"bigint"`
	CompletedAt     int64  `json:"completed_at" gorm:"bigint;index"`
	FirstTextMS     *int64 `json:"first_text_ms" gorm:"column:first_text_ms;type:bigint"`
	GenerationMS    *int64 `json:"generation_ms" gorm:"column:generation_ms;type:bigint"`
	InputTokens     *int64 `json:"input_tokens" gorm:"type:bigint"`
	OutputTokens    *int64 `json:"output_tokens" gorm:"type:bigint"`
	ErrorCode       string `json:"error_code" gorm:"type:varchar(64)"`
	HasOutput       bool   `json:"has_output"`
	HasHTML         bool   `json:"has_html"`
	HasThumbnail    bool   `json:"has_thumbnail"`
	RenderStatus    string `json:"render_status" gorm:"type:varchar(32)"`
	ReviewStatus    string `json:"review_status" gorm:"type:varchar(16)"`
	ReviewNote      string `json:"review_note" gorm:"type:text"`
	BaselineID      int64  `json:"baseline_id"`

	ConfigFingerprint  string   `json:"-" gorm:"type:varchar(64)"`
	RequestFingerprint string   `json:"-" gorm:"type:varchar(64)"`
	ProfileVersion     int64    `json:"-"`
	Groups             []string `json:"-" gorm:"-"`
	GroupsJSON         string   `json:"-" gorm:"type:text"`
	PromptSnapshot     string   `json:"-" gorm:"type:text"`
	Protocol           string   `json:"-" gorm:"type:varchar(16)"`
	ReasoningEffort    string   `json:"-" gorm:"type:varchar(32)"`
	MaxOutputTokens    int      `json:"-"`
	TimeoutSeconds     int      `json:"-"`
	Rule               string   `json:"-" gorm:"type:varchar(32)"`
	RuleVersion        string   `json:"-" gorm:"type:varchar(32)"`
	RuleResults        string   `json:"-" gorm:"type:text"`
	ExtractorVersion   string   `json:"-" gorm:"type:varchar(32)"`
	RendererVersion    string   `json:"-" gorm:"type:varchar(32)"`
	ResponseModel      string   `json:"-" gorm:"type:varchar(256)"`
	RequestStarted     bool     `json:"-"`
	BudgetTokens       int64    `json:"-" gorm:"bigint"`
	BudgetState        string   `json:"-" gorm:"type:varchar(32)"`
	RawKey             string   `json:"-" gorm:"type:varchar(128)"`
	HTMLKey            string   `json:"-" gorm:"type:varchar(128)"`
	ThumbnailKey       string   `json:"-" gorm:"type:varchar(128)"`
	ReviewedBy         int      `json:"-"`
	ReviewedAt         int64    `json:"-" gorm:"bigint"`
}

type QualityProbeRunGroup struct {
	ID        int64  `json:"-" gorm:"primaryKey"`
	RunID     int64  `json:"-" gorm:"uniqueIndex:idx_quality_run_group;index"`
	GroupName string `json:"-" gorm:"type:varchar(128);uniqueIndex:idx_quality_run_group;index"`
}

type QualityProbeDailyBudget struct {
	Date          string `gorm:"type:varchar(10);primaryKey"`
	Calls         int64  `gorm:"bigint"`
	Tokens        int64  `gorm:"bigint"`
	SettledCalls  int64  `gorm:"bigint"`
	SettledTokens int64  `gorm:"bigint"`
}

// QualityReviewAudit preserves review/baseline history without altering outputs.
type QualityReviewAudit struct {
	ID        int64 `gorm:"primaryKey"`
	RunID     int64 `gorm:"index"`
	ActorID   int
	Action    string `gorm:"type:varchar(32)"`
	Note      string `gorm:"type:text"`
	CreatedAt int64  `gorm:"bigint"`
}
