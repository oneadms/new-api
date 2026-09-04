package model

// Codex2APIPolicyStrike stores a verified policy decision received from a
// Codex2API upstream.  The table intentionally contains digests and request
// metadata only; raw API keys and prompt bodies are never persisted here.
type Codex2APIPolicyStrike struct {
	Id             int64  `json:"id"`
	UserId         int    `json:"user_id" gorm:"index"`
	TokenId        int    `json:"token_id" gorm:"index"`
	ClientIP       string `json:"client_ip" gorm:"type:varchar(64);index"`
	RequestID      string `json:"request_id" gorm:"type:varchar(128);index"`
	DecisionID     string `json:"decision_id" gorm:"type:varchar(128);uniqueIndex"`
	EventID        string `json:"event_id" gorm:"type:varchar(128);index"`
	Action         string `json:"action" gorm:"type:varchar(32)"`
	ReasonCode     string `json:"reason_code" gorm:"type:varchar(128)"`
	Severity       string `json:"severity" gorm:"type:varchar(32)"`
	StrikeEligible bool   `json:"strike_eligible"`
	RuleVersion    string `json:"rule_version" gorm:"type:varchar(64)"`
	EvidenceSHA256 string `json:"evidence_sha256" gorm:"type:char(64)"`
	Platform       string `json:"platform" gorm:"type:varchar(128)"`
	CreatedAt      int64  `json:"created_at" gorm:"index"`
}

// Codex2APIPolicyIPBlock is a shared, expiring IP deny-list entry.  It lives
// in the primary database so multiple NewAPI instances can enforce the same
// block when they share that database.
type Codex2APIPolicyIPBlock struct {
	Id        int64  `json:"id"`
	IP        string `json:"ip" gorm:"type:varchar(64);uniqueIndex"`
	Reason    string `json:"reason" gorm:"type:varchar(255)"`
	ExpiresAt int64  `json:"expires_at" gorm:"index"`
	CreatedAt int64  `json:"created_at" gorm:"index"`
}
