package service

import (
	"context"
	"errors"
	"sync"
)

const QualityOutputLimit = 1 << 20

// QualityHTMLInstruction is part of the frozen prompt and token reservation.
const QualityHTMLInstruction = "Return only a complete, self-contained HTML document. Inline all SVG, CSS, and JavaScript. Do not load external resources, make network requests, or require packages. Do not include explanatory prose.\n\n"

// QualityExecutionRequest describes a single, fixed-channel generation. The
// caller owns admission, persistence, cancellation, and its independent budget.
type QualityExecutionRequest struct {
	ChannelID       int
	Model           string
	Protocol        string
	Prompt          string
	MaxOutputTokens int
	ReasoningEffort string
}

// QualityExecutionResult deliberately separates sending a request from proving
// that its stream completed. Unknown usage is nil, never a synthetic zero.
type QualityExecutionResult struct {
	Started            bool
	Complete           bool
	MappedModel        string
	RequestFingerprint string
	ResponseModel      string
	InputTokens        *int64
	OutputTokens       *int64
	FirstTextMS        *int64
	GenerationMS       int64
	ErrorCode          string
}

// QualityExecutor must perform at most one upstream generation, without normal
// gateway retries, fallback channels, billing, consume logs, or routing updates.
// emit must be called sequentially and its error must stop response collection.
type QualityExecutor interface {
	Execute(context.Context, QualityExecutionRequest, func(string) error) (QualityExecutionResult, error)
	Supports(channelType int, protocol string) bool
}

var qualityExecutorRegistry struct {
	sync.RWMutex
	executor QualityExecutor
}

func RegisterQualityExecutor(executor QualityExecutor) {
	qualityExecutorRegistry.Lock()
	defer qualityExecutorRegistry.Unlock()
	qualityExecutorRegistry.executor = executor
}

func GetQualityExecutor() (QualityExecutor, error) {
	qualityExecutorRegistry.RLock()
	defer qualityExecutorRegistry.RUnlock()
	if qualityExecutorRegistry.executor == nil {
		return nil, errors.New("quality detection executor is unavailable")
	}
	return qualityExecutorRegistry.executor, nil
}
