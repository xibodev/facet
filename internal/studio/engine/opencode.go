package engine

import (
	"bytes"
	"encoding/json"
	"strings"
)

// OpenCodeAdapter implements EngineAdapter for OpenCode CLI.
type OpenCodeAdapter struct {
	Executable string
}

// NewOpenCodeAdapter creates a new OpenCodeAdapter instance.
func NewOpenCodeAdapter() *OpenCodeAdapter {
	return &OpenCodeAdapter{
		Executable: "opencode",
	}
}

func (a *OpenCodeAdapter) Name() string {
	return "opencode"
}

func (a *OpenCodeAdapter) DisplayName() string {
	return "OpenCode CLI"
}

func (a *OpenCodeAdapter) ExecutableName() string {
	if a.Executable != "" {
		return a.Executable
	}
	return "opencode"
}

// BuildTurnArgs constructs one autonomous OpenCode invocation.
func (a *OpenCodeAdapter) BuildTurnArgs(dir, mode, prompt, nativeID string, extraArgs []string) ([]string, error) {
	if err := validateAutonomousMode(mode); err != nil {
		return nil, err
	}
	args := []string{"run", "--format", "json", "--auto"}
	args = append(args, extraArgs...)
	if nativeID != "" {
		args = append(args, "--session", nativeID)
	}
	args = append(args, prompt)
	return args, nil
}

// NormalizeEvent parses an OpenCode event line into a NormalizedEvent.
func (a *OpenCodeAdapter) NormalizeEvent(line []byte) (*NormalizedEvent, error) {
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
	part := mapValue(raw["part"])
	data := mapValue(raw["data"])
	maps := []map[string]any{raw, part, data}
	sessionID := firstString(maps, "sessionID", "session_id")
	state := mapValue(part["state"])
	if state != nil {
		status := strings.ToLower(strings.TrimSpace(firstString([]map[string]any{state}, "status")))
		toolName := firstString([]map[string]any{part}, "tool")
		toolID := firstString([]map[string]any{part}, "callID", "callId", "call_id")
		switch status {
		case "pending", "running":
			return &NormalizedEvent{
				Type:      EventToolUse,
				SessionID: sessionID,
				ToolName:  toolName,
				ToolID:    toolID,
				ToolInput: state["input"],
				Raw:       raw,
			}, nil
		case "completed", "error", "failed":
			output := state["output"]
			isError := status == "error" || status == "failed"
			if isError && state["error"] != nil {
				output = state["error"]
			}
			return &NormalizedEvent{
				Type:       EventToolResult,
				SessionID:  sessionID,
				ToolName:   toolName,
				ToolID:     toolID,
				ToolInput:  state["input"],
				ToolOutput: valueString(output),
				IsError:    isError,
				Raw:        raw,
			}, nil
		}
	}

	switch evType {
	case "step_start", "session", "init":
		if sessionID == "" {
			sessionID = firstString(maps, "id")
		}
		return &NormalizedEvent{
			Type:      EventSession,
			SessionID: sessionID,
			Raw:       raw,
		}, nil

	case "text_delta", "text", "message":
		var content string
		for _, value := range []any{part["text"], data["text"], raw["content"], raw["delta"], raw["text"]} {
			if content = contentString(value); content != "" {
				break
			}
		}
		if spec, cleaned, ok := ExtractQuestion(content); ok {
			return &NormalizedEvent{
				Type:      EventQuestion,
				SessionID: sessionID,
				Content:   cleaned,
				Question:  spec,
				Raw:       raw,
			}, nil
		}
		return &NormalizedEvent{
			Type:      EventTextDelta,
			SessionID: sessionID,
			Content:   content,
			Raw:       raw,
		}, nil

	case "think_delta", "thinking", "reasoning":
		var thinking string
		for _, value := range []any{part["text"], data["text"], raw["thinking"], raw["delta"], raw["content"]} {
			if thinking = contentString(value); thinking != "" {
				break
			}
		}
		return &NormalizedEvent{
			Type:      EventThinkDelta,
			SessionID: sessionID,
			Thinking:  thinking,
			Raw:       raw,
		}, nil

	case "tool_use", "tool_call", "action":
		name := firstString(maps, "tool", "name", "tool_name")
		id := firstString(maps, "callID", "callId", "call_id", "id", "tool_id")
		input := part["input"]
		if input == nil {
			input = raw["input"]
		}
		if input == nil {
			input = raw["arguments"]
		}
		return &NormalizedEvent{
			Type:      EventToolUse,
			SessionID: sessionID,
			ToolName:  name,
			ToolID:    id,
			ToolInput: input,
			Raw:       raw,
		}, nil

	case "tool_result", "tool_output", "action_result":
		id := firstString(maps, "callID", "callId", "call_id", "id", "tool_id", "tool_use_id")
		var output any
		for _, value := range []any{part["output"], data["output"], raw["output"], raw["content"], raw["result"]} {
			if value != nil {
				output = value
				break
			}
		}
		isErr := firstBoolLike(maps, "is_error", "isError")
		if reason := strings.ToLower(firstString(maps, "reason", "status")); reason == "error" || reason == "failed" || reason == "cancelled" || reason == "canceled" {
			isErr = true
		}
		return &NormalizedEvent{
			Type:       EventToolResult,
			SessionID:  sessionID,
			ToolName:   firstString(maps, "tool", "name", "tool_name"),
			ToolID:     id,
			ToolOutput: valueString(output),
			IsError:    isErr,
			Raw:        raw,
		}, nil

	case "question":
		q := raw["question"]
		if q == nil {
			q = raw["data"]
		}
		return &NormalizedEvent{
			Type:      EventQuestion,
			SessionID: sessionID,
			Question:  q,
			Raw:       raw,
		}, nil

	case "permission", "permission_request":
		msg, _ := raw["message"].(string)
		return &NormalizedEvent{
			Type:      EventPermission,
			SessionID: sessionID,
			Content:   msg,
			Raw:       raw,
		}, nil

	case "step_finish":
		reason := strings.ToLower(strings.TrimSpace(firstString([]map[string]any{part, data, raw}, "reason", "finishReason", "finish_reason")))
		isErr := firstBoolLike(maps, "is_error", "isError")
		if isOpenCodeFailureReason(reason) {
			isErr = true
		}
		if isErr {
			content := bestReason(raw, part, data)
			if content == "" {
				content = reason
			}
			if content == "" {
				content = "OpenCode step failed"
			}
			return &NormalizedEvent{
				Type:      EventError,
				SessionID: sessionID,
				Content:   content,
				IsError:   true,
				Raw:       raw,
			}, nil
		}
		if !isOpenCodeFinalReason(reason) {
			return &NormalizedEvent{SessionID: sessionID, Raw: raw}, nil
		}
		return openCodeDoneEvent(raw, part, data, sessionID, false), nil

	case "done", "result", "finish":
		isErr := firstBoolLike(maps, "is_error", "isError")
		return openCodeDoneEvent(raw, part, data, sessionID, isErr), nil

	case "error":
		return &NormalizedEvent{
			Type:      EventError,
			SessionID: sessionID,
			Content:   bestReason(raw, part, data),
			IsError:   true,
			Raw:       raw,
		}, nil
	}
	if bestReason(raw, part, data) != "" && (raw["error"] != nil || data["error"] != nil || firstBoolLike(maps, "is_error", "isError")) {
		return &NormalizedEvent{Type: EventError, SessionID: sessionID, Content: bestReason(raw, part, data), IsError: true, Raw: raw}, nil
	}
	if sessionID != "" {
		return &NormalizedEvent{SessionID: sessionID, Raw: raw}, nil
	}

	return nil, nil
}

func isOpenCodeFinalReason(reason string) bool {
	switch reason {
	case "stop", "complete", "completed", "completion", "done", "end-turn", "end_turn":
		return true
	default:
		return false
	}
}

func isOpenCodeFailureReason(reason string) bool {
	switch reason {
	case "length", "content-filter", "content_filter", "cancellation", "cancelled", "canceled", "failure", "failed", "error":
		return true
	default:
		return false
	}
}

func openCodeDoneEvent(raw, part, data map[string]any, sessionID string, isError bool) *NormalizedEvent {
	maps := []map[string]any{part, data, raw}
	cost, _ := firstFloatValue(maps, "cost", "cost_usd", "costUSD", "total_cost_usd", "totalCostUSD")
	duration, _ := firstInt64Value(maps, "duration_ms", "durationMs")
	tokens, _ := tokenCount(part["tokens"], data["tokens"], raw["tokens"], part["usage"], data["usage"], raw["usage"])
	return &NormalizedEvent{
		Type:       EventDone,
		SessionID:  sessionID,
		Content:    bestReason(raw, part, data),
		CostUSD:    cost,
		DurationMs: duration,
		Tokens:     tokens,
		IsError:    isError,
		Raw:        raw,
	}
}
