package controller

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
)

// Pointer usage fields preserve unknown values. In particular an absent usage
// event must never release an admitted task's token reservation as zero usage.
type qualityStreamUsage struct {
	Input         *int64 `json:"input_tokens"`
	Output        *int64 `json:"output_tokens"`
	Prompt        *int64 `json:"prompt_tokens"`
	Completion    *int64 `json:"completion_tokens"`
	CacheRead     *int64 `json:"cache_read_input_tokens"`
	CacheCreation *int64 `json:"cache_creation_input_tokens"`
}

type qualityStreamCollector struct {
	protocol string
	emit     func(string) error
	started  time.Time
	result   *service.QualityExecutionResult
	bytes    int
	stopped  bool
	input    *int64
	output   *int64
}

func (collector *qualityStreamCollector) consume(reader io.Reader) error {
	// Bound both individual SSE events and total wire data, including reasoning,
	// ignored fields, and comments. The enclosing request also has a deadline.
	limited := &io.LimitedReader{R: reader, N: 16<<20 + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 4096), service.QualityOutputLimit+65536)
	var data bytes.Buffer
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if data.Len() == 0 {
				continue
			}
			done, err := collector.event(bytes.TrimSpace(data.Bytes()))
			data.Reset()
			if err != nil {
				return err
			}
			if done {
				if collector.bytes == 0 {
					collector.result.ErrorCode = "empty_output"
					return errors.New("upstream completed without visible text")
				}
				collector.result.Complete = true
				collector.result.ErrorCode = ""
				collector.result.InputTokens, collector.result.OutputTokens = collector.input, collector.output
				return nil
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			part := strings.TrimPrefix(line, "data:")
			part = strings.TrimPrefix(part, " ")
			if data.Len()+len(part)+1 > service.QualityOutputLimit+65536 {
				collector.result.ErrorCode = "stream_limit"
				return errors.New("upstream event exceeds the size limit")
			}
			data.WriteString(part)
			data.WriteByte('\n')
		}
	}
	collector.result.ErrorCode = "stream_interrupted"
	if limited.N == 0 || scanner.Err() != nil {
		collector.result.ErrorCode = "stream_limit"
	}
	// An unterminated SSE event is not dispatched at EOF (SSE specification).
	// Partial usage is intentionally not treated as settled usage.
	return errors.New("upstream stream ended without a complete generation")
}

func (collector *qualityStreamCollector) event(data []byte) (bool, error) {
	collector.result.ErrorCode = "invalid_stream"
	if collector.protocol == "chat" && bytes.Equal(data, []byte("[DONE]")) {
		if !collector.stopped {
			return false, errors.New("chat stream has no successful finish reason")
		}
		return true, nil
	}
	var header struct {
		Type  string          `json:"type"`
		Error json.RawMessage `json:"error"`
	}
	if err := common.Unmarshal(data, &header); err != nil {
		return false, errors.New("invalid upstream event")
	}
	if header.Type == "error" || (len(header.Error) > 0 && string(header.Error) != "null") {
		collector.result.ErrorCode = "upstream_error"
		return false, errors.New("upstream reported a stream error")
	}
	var text string
	done := false
	switch collector.protocol {
	case "chat":
		var event struct {
			Model   string `json:"model"`
			Choices []struct {
				Index int `json:"index"`
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
			Usage *qualityStreamUsage `json:"usage"`
		}
		if err := common.Unmarshal(data, &event); err != nil {
			return false, errors.New("invalid chat event")
		}
		if event.Model != "" {
			collector.result.ResponseModel = event.Model
		}
		for _, choice := range event.Choices {
			if choice.Index != 0 {
				return false, errors.New("unexpected extra generation choice")
			}
			if collector.stopped && choice.Delta.Content != "" {
				return false, errors.New("text after finished choice")
			}
			text += choice.Delta.Content
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				if *choice.FinishReason != "stop" {
					collector.result.ErrorCode = "incomplete_output"
					return false, errors.New("chat generation did not finish normally")
				}
				collector.stopped = true
			}
		}
		if event.Usage != nil {
			collector.setUsage(event.Usage.Prompt, event.Usage.Completion)
		}
	case "responses":
		var event struct {
			Delta    string `json:"delta"`
			Response *struct {
				Status string              `json:"status"`
				Model  string              `json:"model"`
				Usage  *qualityStreamUsage `json:"usage"`
			} `json:"response"`
		}
		if err := common.Unmarshal(data, &event); err != nil {
			return false, errors.New("invalid responses event")
		}
		if event.Response != nil && event.Response.Model != "" {
			collector.result.ResponseModel = event.Response.Model
		}
		switch header.Type {
		case "response.output_text.delta":
			text = event.Delta
		case "response.failed", "response.incomplete", "response.cancelled":
			collector.result.ErrorCode = "incomplete_output"
			return false, errors.New("response generation did not complete")
		case "response.completed":
			if event.Response == nil || event.Response.Status != "completed" {
				return false, errors.New("invalid response completion")
			}
			if event.Response.Usage != nil {
				collector.setUsage(event.Response.Usage.Input, event.Response.Usage.Output)
			}
			done = true
		}
	case "claude":
		var event struct {
			Message *struct {
				Model string              `json:"model"`
				Usage *qualityStreamUsage `json:"usage"`
			} `json:"message"`
			Delta struct {
				Type       string `json:"type"`
				Text       string `json:"text"`
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			Usage *qualityStreamUsage `json:"usage"`
		}
		if err := common.Unmarshal(data, &event); err != nil {
			return false, errors.New("invalid Claude event")
		}
		switch header.Type {
		case "message_start":
			if event.Message != nil {
				collector.result.ResponseModel = event.Message.Model
				usage := event.Message.Usage
				if usage != nil && usage.Input != nil && *usage.Input >= 0 && *usage.Input <= math.MaxInt32 {
					input := *usage.Input
					for _, count := range []*int64{usage.CacheRead, usage.CacheCreation} {
						if count != nil {
							if *count < 0 || *count > math.MaxInt32 || input > math.MaxInt32-*count {
								input = -1
								break
							}
							input += *count
						}
					}
					collector.setUsage(&input, nil)
				}
			}
		case "content_block_delta":
			if event.Delta.Type == "text_delta" {
				text = event.Delta.Text
			}
		case "message_delta":
			if event.Delta.StopReason != "" {
				if event.Delta.StopReason != "end_turn" && event.Delta.StopReason != "stop_sequence" {
					collector.result.ErrorCode = "incomplete_output"
					return false, errors.New("Claude generation did not finish normally")
				}
				collector.stopped = true
			}
			if event.Usage != nil {
				collector.setUsage(collector.input, event.Usage.Output)
			}
		case "message_stop":
			if !collector.stopped {
				return false, errors.New("Claude stream has no successful stop reason")
			}
			done = true
		}
	default:
		return false, errors.New("unsupported stream protocol")
	}
	if text != "" {
		if len(text) > service.QualityOutputLimit-collector.bytes {
			collector.result.ErrorCode = "output_limit"
			return false, errors.New("generated output exceeds the size limit")
		}
		if collector.result.FirstTextMS == nil {
			elapsed := time.Since(collector.started).Milliseconds()
			collector.result.FirstTextMS = &elapsed
		}
		if err := collector.emit(text); err != nil {
			collector.result.ErrorCode = "output_write_failed"
			return false, errors.New("generated output could not be saved")
		}
		collector.bytes += len(text)
	}
	return done, nil
}

func (collector *qualityStreamCollector) setUsage(input, output *int64) {
	collector.input, collector.output = nil, nil
	if input != nil && *input >= 0 && *input <= math.MaxInt32 {
		collector.input = input
	}
	if output != nil && *output >= 0 && *output <= math.MaxInt32 {
		collector.output = output
	}
}
