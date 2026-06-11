package operation_setting

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/setting/config"
)

const FailureProbabilityBps = 10000

type LuckyCheckinSetting struct {
	Enabled             bool `json:"enabled"`
	MinStakeQuota       int  `json:"min_stake_quota"`
	MaxStakeQuota       int  `json:"max_stake_quota"`
	MinFailureBps       int  `json:"min_failure_bps"`
	MaxFailureBps       int  `json:"max_failure_bps"`
	ActualMinFailureBps int  `json:"actual_min_failure_bps"`
	ActualMaxFailureBps int  `json:"actual_max_failure_bps"`
}

var luckyCheckinSetting = LuckyCheckinSetting{
	Enabled:             false,
	MinStakeQuota:       1000,
	MaxStakeQuota:       10000,
	MinFailureBps:       2500,
	MaxFailureBps:       7500,
	ActualMinFailureBps: 2500,
	ActualMaxFailureBps: 7500,
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
	if err := validateFailureBpsRange(s.MinFailureBps, s.MaxFailureBps, "展示"); err != nil {
		return err
	}
	if err := validateFailureBpsRange(s.ActualMinFailureBps, s.ActualMaxFailureBps, "实际"); err != nil {
		return err
	}
	return nil
}

func (s *LuckyCheckinSetting) FailureBps(stakeQuota int) (int, error) {
	return s.failureBps(stakeQuota, s.ActualMinFailureBps, s.ActualMaxFailureBps)
}

func (s *LuckyCheckinSetting) DisplayFailureBps(stakeQuota int) (int, error) {
	return s.failureBps(stakeQuota, s.MinFailureBps, s.MaxFailureBps)
}

func (s *LuckyCheckinSetting) failureBps(stakeQuota int, minFailureBps int, maxFailureBps int) (int, error) {
	if err := s.Validate(); err != nil {
		return 0, err
	}
	if stakeQuota < s.MinStakeQuota || stakeQuota > s.MaxStakeQuota {
		return 0, fmt.Errorf("押注额度必须在 %d 到 %d 之间", s.MinStakeQuota, s.MaxStakeQuota)
	}
	if s.MaxStakeQuota == s.MinStakeQuota {
		return minFailureBps, nil
	}
	return minFailureBps +
		(stakeQuota-s.MinStakeQuota)*(maxFailureBps-minFailureBps)/(s.MaxStakeQuota-s.MinStakeQuota), nil
}

func validateFailureBpsRange(minBps int, maxBps int, label string) error {
	if minBps < 0 || minBps > FailureProbabilityBps {
		return fmt.Errorf("运气签到%s最小失败概率必须在 0 到 %d 基点之间", label, FailureProbabilityBps)
	}
	if maxBps < minBps || maxBps > FailureProbabilityBps {
		return fmt.Errorf("运气签到%s最大失败概率必须在最小失败概率到 %d 基点之间", label, FailureProbabilityBps)
	}
	return nil
}
