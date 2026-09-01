package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// GenericAdapter implements a flexible fallback EngineAdapter for arbitrary CLI tools.
type GenericAdapter struct {
	Executable string
}

// NewGenericAdapter creates a new GenericAdapter instance.
func NewGenericAdapter() *GenericAdapter {
	return &GenericAdapter{
		Executable: "generic",
	}
}

func (a *GenericAdapter) Name() string {
	return "generic"
}

func (a *GenericAdapter) DisplayName() string {
	return "Generic CLI"
}

func (a *GenericAdapter) ExecutableName() string {
	if a.Executable != "" {
		return a.Executable
	}
	return "generic"
}

// BuildArgs constructs the arguments by simply returning extraArgs.
func (a *GenericAdapter) BuildArgs(dir string, mode string, extraArgs []string) ([]string, error) {
	if extraArgs == nil {
		return []string{}, nil
	}
	return extraArgs, nil
}

// FormatUserMessage formats a prompt into a standard user message JSON payload.
func (a *GenericAdapter) FormatUserMessage(prompt string) ([]byte, error) {
	msg := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role": "user",
			"content": []map[string]any{
				{
					"type": "text",
					"text": prompt,
				},
			},
		},
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal generic user message: %w", err)
	}
	return append(b, '\n'), nil
}

// NormalizeEvent normalizes a JSON or plain-text line into a NormalizedEvent.
func (a *GenericAdapter) NormalizeEvent(line []byte) (*NormalizedEvent, error) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return nil, nil
	}

	var raw map[string]any
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		s := string(trimmed)
		if strings.HasPrefix(strings.ToLower(s), "error:") {
			return &NormalizedEvent{
				Type:    EventError,
				Content: s,
				IsError: true,
			}, nil
		}
		if spec, cleaned, ok := ExtractQuestion(s); ok {
			return &NormalizedEvent{
				Type:     EventQuestion,
				Content:  cleaned,
				Question: spec,
			}, nil
		}
		return &NormalizedEvent{
			Type:    EventTextDelta,
			Content: s,
		}, nil
	}

	evType, _ := raw["type"].(string)

	switch strings.ToLower(evType) {
	case "session", "init":
		sessionID, _ := raw["session_id"].(string)
		if sessionID == "" {
			sessionID, _ = raw["id"].(string)
		}
		return &NormalizedEvent{
			Type:      EventSession,
			SessionID: sessionID,
			Raw:       raw,
		}, nil

	case "text_delta", "text", "message":
		var content string
		if c, ok := raw["content"].(string); ok {
			content = c
		} else if d, ok := raw["delta"].(string); ok {
			content = d
		} else if t, ok := raw["text"].(string); ok {
			content = t
		}
		if spec, cleaned, ok := ExtractQuestion(content); ok {
			return &NormalizedEvent{
				Type:     EventQuestion,
				Content:  cleaned,
				Question: spec,
				Raw:      raw,
			}, nil
		}
		return &NormalizedEvent{
			Type:    EventTextDelta,
			Content: content,
			Raw:     raw,
		}, nil

	case "think_delta", "thinking", "reasoning":
		var th string
		if t, ok := raw["thinking"].(string); ok {
			th = t
		} else if d, ok := raw["delta"].(string); ok {
			th = d
		} else if r, ok := raw["reasoning"].(string); ok {
			th = r
		}
		return &NormalizedEvent{
			Type:     EventThinkDelta,
			Thinking: th,
			Raw:      raw,
		}, nil

	case "tool_use", "tool_call", "action":
		name, _ := raw["name"].(string)
		if name == "" {
			name, _ = raw["tool_name"].(string)
		}
		id, _ := raw["id"].(string)
		if id == "" {
			id, _ = raw["tool_id"].(string)
		}
		input := raw["input"]
		if input == nil {
			input = raw["arguments"]
		}
		return &NormalizedEvent{
			Type:      EventToolUse,
			ToolName:  name,
			ToolID:    id,
			ToolInput: input,
			Raw:       raw,
		}, nil

	case "tool_result", "tool_output", "action_result":
		id, _ := raw["id"].(string)
		if id == "" {
			id, _ = raw["tool_id"].(string)
		}
		if id == "" {
			id, _ = raw["tool_use_id"].(string)
		}
		var outStr string
		if out, ok := raw["output"].(string); ok {
			outStr = out
		} else if c, ok := raw["content"].(string); ok {
			outStr = c
		} else if res := raw["result"]; res != nil {
			if s, ok := res.(string); ok {
				outStr = s
			} else {
				b, _ := json.Marshal(res)
				outStr = string(b)
			}
		}
		isErr, _ := raw["is_error"].(bool)
		return &NormalizedEvent{
			Type:       EventToolResult,
			ToolID:     id,
			ToolOutput: outStr,
			IsError:    isErr,
			Raw:        raw,
		}, nil

	case "question":
		q := raw["question"]
		if q == nil {
			q = raw["data"]
		}
		return &NormalizedEvent{
			Type:     EventQuestion,
			Question: q,
			Raw:      raw,
		}, nil

	case "permission":
		msg, _ := raw["message"].(string)
		return &NormalizedEvent{
			Type:    EventPermission,
			Content: msg,
			Raw:     raw,
		}, nil

	case "done", "result", "finish":
		cost, _ := getFloat(raw["cost_usd"])
		if cost == 0 {
			cost, _ = getFloat(raw["total_cost_usd"])
		}
		dur, _ := getInt64(raw["duration_ms"])
		tokens, _ := getInt64(raw["tokens"])
		if tokens == 0 {
			if usage, ok := raw["usage"].(map[string]any); ok {
				outTok, _ := getInt64(usage["output_tokens"])
				inTok, _ := getInt64(usage["input_tokens"])
				tokens = outTok + inTok
			}
		}
		isErr, _ := raw["is_error"].(bool)
		return &NormalizedEvent{
			Type:       EventDone,
			CostUSD:    cost,
			DurationMs: dur,
			Tokens:     tokens,
			IsError:    isErr,
			Raw:        raw,
		}, nil

	case "error":
		msg, _ := raw["message"].(string)
		if msg == "" {
			msg, _ = raw["error"].(string)
		}
		return &NormalizedEvent{
			Type:    EventError,
			Content: msg,
			IsError: true,
			Raw:     raw,
		}, nil
	}

	// If type is not recognized but it has common fields
	if content, ok := raw["content"].(string); ok {
		if spec, cleaned, ok := ExtractQuestion(content); ok {
			return &NormalizedEvent{
				Type:     EventQuestion,
				Content:  cleaned,
				Question: spec,
				Raw:      raw,
			}, nil
		}
		return &NormalizedEvent{
			Type:    EventTextDelta,
			Content: content,
			Raw:     raw,
		}, nil
	}

	return nil, nil
}
