package operation_setting

import "testing"

func TestLuckyCheckinFailureBps(t *testing.T) {
	setting := LuckyCheckinSetting{
		Enabled:       true,
		MinStakeQuota: 1000,
		MaxStakeQuota: 10000,
		MinFailureBps: 2500,
		MaxFailureBps: 7500,
	}

	tests := []struct {
		name  string
		stake int
		want  int
	}{
		{name: "minimum stake", stake: 1000, want: 2500},
		{name: "middle stake", stake: 5500, want: 5000},
		{name: "maximum stake", stake: 10000, want: 7500},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := setting.FailureBps(test.stake)
			if err != nil {
				t.Fatalf("FailureBps() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("FailureBps() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestLuckyCheckinFailureBpsRejectsInvalidStake(t *testing.T) {
	setting := LuckyCheckinSetting{
		MinStakeQuota: 1000,
		MaxStakeQuota: 10000,
		MinFailureBps: 2500,
		MaxFailureBps: 7500,
	}

	if _, err := setting.FailureBps(999); err == nil {
		t.Fatal("FailureBps() expected an error for a stake below the configured range")
	}
}
