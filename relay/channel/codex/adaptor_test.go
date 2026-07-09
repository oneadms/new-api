package codex

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
)

func TestConvertOpenAIResponsesRequestNormalizesFunctionCallArguments(t *testing.T) {
	request := dto.OpenAIResponsesRequest{
		Model: "codex",
		Input: json.RawMessage(`[
			{"role":"user","content":"weather"},
			{"type":"function_call","call_id":"call_1","name":"lookup_weather","arguments":"{\"city\":\"Shanghai\",\"days\":3}"}
		]`),
		MaxOutputTokens: common.GetPointer(uint(1024)),
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, codexRelayInfo(), request)
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest returned error: %v", err)
	}

	responsesRequest, ok := converted.(dto.OpenAIResponsesRequest)
	if !ok {
		t.Fatalf("converted request type = %T, want dto.OpenAIResponsesRequest", converted)
	}

	var input []map[string]any
	if err := common.Unmarshal(responsesRequest.Input, &input); err != nil {
		t.Fatalf("failed to unmarshal converted input: %v", err)
	}
	if len(input) != 2 {
		t.Fatalf("converted input len = %d, want 2", len(input))
	}

	args, ok := input[1]["arguments"].(map[string]any)
	if !ok {
		t.Fatalf("arguments type = %T, want object", input[1]["arguments"])
	}
	if args["city"] != "Shanghai" {
		t.Fatalf("city = %v, want Shanghai", args["city"])
	}
	if args["days"] != float64(3) {
		t.Fatalf("days = %v, want 3", args["days"])
	}
	if responsesRequest.MaxOutputTokens != nil {
		t.Fatalf("MaxOutputTokens should be removed for codex responses requests")
	}
}

func TestConvertOpenAIResponsesRequestRejectsNonObjectFunctionCallArguments(t *testing.T) {
	request := dto.OpenAIResponsesRequest{
		Model: "codex",
		Input: json.RawMessage(`[
			{"type":"function_call","call_id":"call_1","name":"lookup_weather","arguments":"[]"}
		]`),
	}

	_, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, codexRelayInfo(), request)
	if err == nil {
		t.Fatal("expected error for non-object function_call arguments")
	}
}

func codexRelayInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}
}
