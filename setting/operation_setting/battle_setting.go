package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type BattleSetting struct {
	Enabled              bool  `json:"enabled"`
	HideRoomInput        bool  `json:"hide_room_input"`
	MinDropQuota         int   `json:"min_drop_quota"`
	MaxDropQuota         int   `json:"max_drop_quota"`
	MaxRoundLossQuota    int   `json:"max_round_loss_quota"`
	MaxRoundGainQuota    int   `json:"max_round_gain_quota"`
	MaxDailyLossQuota    int   `json:"max_daily_loss_quota"`
	MaxDailyGainQuota    int   `json:"max_daily_gain_quota"`
	AllowNegativeBalance bool  `json:"allow_negative_balance"`
	CapQuota             int   `json:"cap_quota"`
	MaxPlayersPerRoom    int   `json:"max_players_per_room"`
	TickRate             int   `json:"tick_rate"`
	MapWidth             int   `json:"map_width"`
	MapHeight            int   `json:"map_height"`
	PlayerSpeed          int   `json:"player_speed"`
	BulletSpeed          int   `json:"bullet_speed"`
	BulletDamage         int   `json:"bullet_damage"`
	FireCooldownMs       int   `json:"fire_cooldown_ms"`
	RespawnSeconds       int   `json:"respawn_seconds"`
	DropPickupRadius     int   `json:"drop_pickup_radius"`
	DropExpireSeconds    int   `json:"drop_expire_seconds"`
	IdleRoomTTLSeconds   int   `json:"idle_room_ttl_seconds"`
	MatchModeEnabled     bool  `json:"match_mode_enabled"`
	MatchEntryQuota      int   `json:"match_entry_quota"`
	MatchMinPlayers      int   `json:"match_min_players"`
	MatchDurationSecs    int   `json:"match_duration_seconds"`
	MatchStartAt         int64 `json:"match_start_at"`
}

var battleSetting = BattleSetting{
	Enabled:              false,
	HideRoomInput:        true,
	MinDropQuota:         100,
	MaxDropQuota:         1000,
	MaxRoundLossQuota:    5000,
	MaxRoundGainQuota:    5000,
	MaxDailyLossQuota:    20000,
	MaxDailyGainQuota:    20000,
	AllowNegativeBalance: false,
	CapQuota:             100,
	MaxPlayersPerRoom:    8,
	TickRate:             30,
	MapWidth:             1600,
	MapHeight:            900,
	PlayerSpeed:          260,
	BulletSpeed:          780,
	BulletDamage:         34,
	FireCooldownMs:       220,
	RespawnSeconds:       3,
	DropPickupRadius:     38,
	DropExpireSeconds:    18,
	IdleRoomTTLSeconds:   30,
	MatchModeEnabled:     false,
	MatchEntryQuota:      5000,
	MatchMinPlayers:      4,
	MatchDurationSecs:    300,
	MatchStartAt:         0,
}

func init() {
	config.GlobalConfig.Register("battle_setting", &battleSetting)
}

func GetBattleSetting() *BattleSetting {
	return &battleSetting
}

func IsBattleEnabled() bool {
	return battleSetting.Enabled
}
