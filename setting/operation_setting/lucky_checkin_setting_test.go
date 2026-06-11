package operation_setting

import "testing"

func TestLuckyCheckinFailureBps(t *testing.T) {
	setting := LuckyCheckinSetting{
		Enabled:             true,
		MinStakeQuota:       1000,
		MaxStakeQuota:       10000,
		MinFailureBps:       2500,
		MaxFailureBps:       7500,
		ActualMinFailureBps: 3000,
		ActualMaxFailureBps: 8000,
	}

	tests := []struct {
		name  string
		stake int
		want  int
	}{
		{name: "minimum stake", stake: 1000, want: 3000},
		{name: "middle stake", stake: 5500, want: 5500},
		{name: "maximum stake", stake: 10000, want: 8000},
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
		MinStakeQuota:       1000,
		MaxStakeQuota:       10000,
		MinFailureBps:       2500,
		MaxFailureBps:       7500,
		ActualMinFailureBps: 2500,
		ActualMaxFailureBps: 7500,
	}

	if _, err := setting.FailureBps(999); err == nil {
		t.Fatal("FailureBps() expected an error for a stake below the configured range")
	}
}

func TestLuckyCheckinDisplayFailureBps(t *testing.T) {
	setting := LuckyCheckinSetting{
		MinStakeQuota:       1000,
		MaxStakeQuota:       10000,
		MinFailureBps:       2500,
		MaxFailureBps:       7500,
		ActualMinFailureBps: 1000,
		ActualMaxFailureBps: 2000,
	}

	got, err := setting.DisplayFailureBps(5500)
	if err != nil {
		t.Fatalf("DisplayFailureBps() error = %v", err)
	}
	if got != 5000 {
		t.Fatalf("DisplayFailureBps() = %d, want %d", got, 5000)
	}
}
