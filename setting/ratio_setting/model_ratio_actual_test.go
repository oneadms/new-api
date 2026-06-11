package ratio_setting

import "testing"

func restoreModelRatioSettings(t *testing.T, modelRatio, actualModelRatio string) {
	t.Helper()
	if err := UpdateModelRatioByJSONString(modelRatio); err != nil {
		t.Fatalf("restore model ratio: %v", err)
	}
	if err := UpdateActualModelRatioByJSONString(actualModelRatio); err != nil {
		t.Fatalf("restore actual model ratio: %v", err)
	}
}

func TestGetActualModelRatioFallsBackToDisplayedRatio(t *testing.T) {
	originalModelRatio := ModelRatio2JSONString()
	originalActualModelRatio := ActualModelRatio2JSONString()
	defer restoreModelRatioSettings(t, originalModelRatio, originalActualModelRatio)

	if err := UpdateModelRatioByJSONString(`{"actual-ratio-test":2.5}`); err != nil {
		t.Fatalf("update model ratio: %v", err)
	}
	if err := UpdateActualModelRatioByJSONString(`{}`); err != nil {
		t.Fatalf("update actual model ratio: %v", err)
	}

	ratio, ok, matchName := GetActualModelRatio("actual-ratio-test")
	if !ok {
		t.Fatal("expected actual ratio lookup to fall back to displayed ratio")
	}
	if ratio != 2.5 {
		t.Fatalf("ratio = %v, want 2.5", ratio)
	}
	if matchName != "actual-ratio-test" {
		t.Fatalf("matchName = %q, want actual-ratio-test", matchName)
	}
}

func TestGetActualModelRatioOverridesDisplayedRatio(t *testing.T) {
	originalModelRatio := ModelRatio2JSONString()
	originalActualModelRatio := ActualModelRatio2JSONString()
	defer restoreModelRatioSettings(t, originalModelRatio, originalActualModelRatio)

	if err := UpdateModelRatioByJSONString(`{"actual-ratio-test":2.5}`); err != nil {
		t.Fatalf("update model ratio: %v", err)
	}
	if err := UpdateActualModelRatioByJSONString(`{"actual-ratio-test":1.25}`); err != nil {
		t.Fatalf("update actual model ratio: %v", err)
	}

	ratio, ok, _ := GetActualModelRatio("actual-ratio-test")
	if !ok {
		t.Fatal("expected actual ratio lookup to succeed")
	}
	if ratio != 1.25 {
		t.Fatalf("ratio = %v, want 1.25", ratio)
	}
}

func TestGetActualModelRatioPreservesExplicitZero(t *testing.T) {
	originalModelRatio := ModelRatio2JSONString()
	originalActualModelRatio := ActualModelRatio2JSONString()
	defer restoreModelRatioSettings(t, originalModelRatio, originalActualModelRatio)

	if err := UpdateModelRatioByJSONString(`{"actual-ratio-test":2.5}`); err != nil {
		t.Fatalf("update model ratio: %v", err)
	}
	if err := UpdateActualModelRatioByJSONString(`{"actual-ratio-test":0}`); err != nil {
		t.Fatalf("update actual model ratio: %v", err)
	}

	ratio, ok, _ := GetActualModelRatio("actual-ratio-test")
	if !ok {
		t.Fatal("expected explicit zero actual ratio lookup to succeed")
	}
	if ratio != 0 {
		t.Fatalf("ratio = %v, want 0", ratio)
	}
}
