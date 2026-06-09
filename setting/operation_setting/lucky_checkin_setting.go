package operation_setting

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/setting/config"
)

const FailureProbabilityBps = 10000

type LuckyCheckinSetting struct {
	Enabled       bool `json:"enabled"`
	MinStakeQuota int  `json:"min_stake_quota"`
	MaxStakeQuota int  `json:"max_stake_quota"`
	MinFailureBps int  `json:"min_failure_bps"`
	MaxFailureBps int  `json:"max_failure_bps"`
}

var luckyCheckinSetting = LuckyCheckinSetting{
	Enabled:       false,
	MinStakeQuota: 1000,
	MaxStakeQuota: 10000,
	MinFailureBps: 2500,
	MaxFailureBps: 7500,
}

func init() {
	config.GlobalConfig.Register("lucky_checkin_setting", &luckyCheckinSetting)
}

func GetLuckyCheckinSetting() *LuckyCheckinSetting {
	return &luckyCheckinSetting
}

func (s *LuckyCheckinSetting) Validate() error {
	if s.MinStakeQuota <= 0 {
		return errors.New("运气签到最小押注额度必须大于 0")
	}
	if s.MaxStakeQuota < s.MinStakeQuota {
		return errors.New("运气签到最大押注额度不能小于最小押注额度")
	}
	if s.MinFailureBps < 0 || s.MinFailureBps > FailureProbabilityBps {
		return fmt.Errorf("运气签到最小失败概率必须在 0 到 %d 基点之间", FailureProbabilityBps)
	}
	if s.MaxFailureBps < s.MinFailureBps || s.MaxFailureBps > FailureProbabilityBps {
		return fmt.Errorf("运气签到最大失败概率必须在最小失败概率到 %d 基点之间", FailureProbabilityBps)
	}
	return nil
}

func (s *LuckyCheckinSetting) FailureBps(stakeQuota int) (int, error) {
	if err := s.Validate(); err != nil {
		return 0, err
	}
	if stakeQuota < s.MinStakeQuota || stakeQuota > s.MaxStakeQuota {
		return 0, fmt.Errorf("押注额度必须在 %d 到 %d 之间", s.MinStakeQuota, s.MaxStakeQuota)
	}
	if s.MaxStakeQuota == s.MinStakeQuota {
		return s.MinFailureBps, nil
	}
	return s.MinFailureBps +
		(stakeQuota-s.MinStakeQuota)*(s.MaxFailureBps-s.MinFailureBps)/(s.MaxStakeQuota-s.MinStakeQuota), nil
}
