package codex2api

import "github.com/QuantumNous/new-api/relay/channel/newapi"

// Codex2API exposes native OpenAI, Claude and Gemini endpoints with an API key.
// Identity signing is handled by the shared transport after header overrides.
type Adaptor struct {
	newapi.Adaptor
}

func (a *Adaptor) GetModelList() []string {
	// Available models depend on the accounts configured on this gateway.
	return nil
}

func (a *Adaptor) GetChannelName() string {
	return "codex2api"
}
