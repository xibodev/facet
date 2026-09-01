package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// CopilotAdapter implements EngineAdapter for GitHub Copilot CLI.
type CopilotAdapter struct {
	Executable string
}

// NewCopilotAdapter creates a new CopilotAdapter instance.
func NewCopilotAdapter() *CopilotAdapter {
	return &CopilotAdapter{
		Executable: "copilot",
	}
}

func (a *CopilotAdapter) Name() string {
	return "copilot"
}

func (a *CopilotAdapter) DisplayName() string {
	return "GitHub Copilot CLI"
}

func (a *CopilotAdapter) ExecutableName() string {
	if a.Executable != "" {
		return a.Executable
	}
	return "copilot"
}

// BuildArgs constructs the command-line arguments for running Copilot.
func (a *CopilotAdapter) BuildArgs(dir string, mode string, extraArgs []string) ([]string, error) {
	args := []string{"-p"}

	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "rw":
		args = append(args, "--allow-all")
	case "ro":
		args = append(args, "--read-only")
	case "ask":
		args = append(args, "--confirm")
	}

	if len(extraArgs) > 0 {
		args = append(args, extraArgs...)
	}

	return args, nil
}

// FormatUserMessage formats a prompt into a user message payload.
func (a *CopilotAdapter) FormatUserMessage(prompt string) ([]byte, error) {
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
		return nil, fmt.Errorf("marshal copilot user message: %w", err)
	}
	return append(b, '\n'), nil
}

// NormalizeEvent parses a GitHub Copilot event line into a NormalizedEvent.
func (a *CopilotAdapter) NormalizeEvent(line []byte) (*NormalizedEvent, error) {
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

	switch evType {
	case "session", "conversation.create", "init":
		sessionID, _ := raw["session_id"].(string)
		if sessionID == "" {
			sessionID, _ = raw["conversation_id"].(string)
		}
		if sessionID == "" {
			sessionID, _ = raw["id"].(string)
		}
		return &NormalizedEvent{
			Type:      EventSession,
			SessionID: sessionID,
			Raw:       raw,
		}, nil

	case "text_delta", "message.delta", "assistant.message.delta", "text", "message":
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

	case "think_delta", "reasoning.delta", "thinking":
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

	case "tool_use", "tool_call", "function_call":
		name, _ := raw["name"].(string)
		if name == "" {
			name, _ = raw["tool_name"].(string)
		}
		id, _ := raw["id"].(string)
		if id == "" {
			id, _ = raw["tool_id"].(string)
		}
		if id == "" {
			id, _ = raw["call_id"].(string)
		}
		input := raw["input"]
		if input == nil {
			input = raw["arguments"]
		}
		if input == nil {
			input = raw["parameters"]
		}
		return &NormalizedEvent{
			Type:      EventToolUse,
			ToolName:  name,
			ToolID:    id,
			ToolInput: input,
			Raw:       raw,
		}, nil

	case "tool_result", "tool_call_result", "function_call_result":
		id, _ := raw["id"].(string)
		if id == "" {
			id, _ = raw["tool_id"].(string)
		}
		if id == "" {
			id, _ = raw["tool_use_id"].(string)
		}
		if id == "" {
			id, _ = raw["call_id"].(string)
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

	case "permission", "approval_request":
		msg, _ := raw["message"].(string)
		return &NormalizedEvent{
			Type:    EventPermission,
			Content: msg,
			Raw:     raw,
		}, nil

	case "done", "turn.finish", "conversation.end", "result":
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

	return nil, nil
}
