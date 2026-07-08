package common

import (
	"testing"

	projectcommon "github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestTaskSubmitReqRejectsHugeDuration(t *testing.T) {
	var req TaskSubmitReq

	err := projectcommon.Unmarshal([]byte(`{"prompt":"hi","duration":2147483648}`), &req)

	require.Error(t, err)
	require.Contains(t, err.Error(), "duration is invalid")
}

func TestValidateTaskBillingParamsRejectsHugeSeconds(t *testing.T) {
	req := TaskSubmitReq{
		Prompt:  "hi",
		Seconds: "2147483648",
	}

	taskErr := validateTaskBillingParams(req)

	require.NotNil(t, taskErr)
	require.Contains(t, taskErr.Message, "seconds is invalid")
}

func TestValidateTaskBillingParamsRejectsHugeMetadataDurationSeconds(t *testing.T) {
	req := TaskSubmitReq{
		Prompt: "hi",
		Metadata: map[string]interface{}{
			"durationSeconds": float64(2147483648),
		},
	}

	taskErr := validateTaskBillingParams(req)

	require.NotNil(t, taskErr)
	require.Contains(t, taskErr.Message, "durationSeconds is invalid")
}
