package engine

import (
	"encoding/json"
	"strings"
)

// CodexAdapter implements EngineAdapter for the OpenAI Codex CLI.
type CodexAdapter struct {
	Executable string
}

func NewCodexAdapter() *CodexAdapter {
	return &CodexAdapter{Executable: "codex"}
}

func (a *CodexAdapter) Name() string {
	return "codex"
}

func (a *CodexAdapter) DisplayName() string {
	return "OpenAI Codex CLI"
}

func (a *CodexAdapter) ExecutableName() string {
	if a.Executable != "" {
		return a.Executable
	}
	return "codex"
}

func (a *CodexAdapter) BuildTurnArgs(dir, mode, prompt, nativeID string, extraArgs []string) ([]string, error) {
	if err := validateAutonomousMode(mode); err != nil {
		return nil, err
	}
	args := []string{"exec"}
	if nativeID != "" {
		args = append(args, "resume")
	}
	args = append(args, "--json", "--dangerously-bypass-approvals-and-sandbox")
	args = append(args, extraArgs...)
	if nativeID != "" {
		args = append(args, nativeID)
	}
	args = append(args, prompt)
	return args, nil
}

func (a *CodexAdapter) NormalizeEvent(line []byte) (*NormalizedEvent, error) {
	line = []byte(strings.TrimSpace(string(line)))
	if len(line) == 0 {
		return nil, nil
	}

	var raw map[string]any
	if err := json.Unmarshal(line, &raw); err != nil {
		text := string(line)
		if strings.HasPrefix(strings.ToLower(text), "error:") {
			return &NormalizedEvent{Type: EventError, Content: text, IsError: true}, nil
		}
		return &NormalizedEvent{
			Type:    EventTextDelta,
			Content: text,
			Raw:     raw,
		}, nil
	}

	evType, _ := raw["type"].(string)
	item := mapValue(raw["item"])
	data := mapValue(raw["data"])
	maps := []map[string]any{raw, item, data}
	sessionID := firstString(maps, "thread_id", "session_id")
	norm := &NormalizedEvent{SessionID: sessionID, Raw: raw}

	switch evType {
	case "thread.started":
		norm.Type = EventSession
		return norm, nil

	case "turn.started":
		if sessionID == "" {
			return nil, nil
		}
		norm.Type = EventSession
		return norm, nil

	case "item.started":
		if firstString([]map[string]any{item}, "type") != "command_execution" {
			return nil, nil
		}
		norm.Type = EventToolUse
		norm.ToolName = "command_execution"
		norm.ToolID = firstString([]map[string]any{item}, "id")
		norm.ToolInput = item["command"]
		return norm, nil

	case "item.completed":
		itemType, _ := item["type"].(string)
		switch itemType {
		case "agent_message":
			norm.Type = EventTextDelta
			norm.Content = contentString(item["text"])
			if norm.Content == "" {
				norm.Content = contentString(item["content"])
			}
			if q, cleaned, found := ExtractQuestion(norm.Content); found {
				norm.Type = EventQuestion
				norm.Question = q
				norm.Content = cleaned
			}
			return norm, nil
		case "reasoning":
			norm.Type = EventThinkDelta
			norm.Thinking = contentString(item["text"])
			return norm, nil
		case "command_execution":
			norm.Type = EventToolResult
			norm.ToolName = "command_execution"
			norm.ToolID = firstString([]map[string]any{item}, "id")
			norm.ToolInput = item["command"]
			norm.ToolOutput = valueString(item["aggregated_output"])
			norm.IsError = firstBoolLike([]map[string]any{item}, "is_error", "isError")
			if status := strings.ToLower(firstString([]map[string]any{item}, "status")); status == "failed" || status == "error" || status == "cancelled" || status == "canceled" {
				norm.IsError = true
			}
			if exitCode, ok := getInt64(item["exit_code"]); ok && exitCode != 0 {
				norm.IsError = true
			}
			return norm, nil
		}
		return nil, nil

	case "turn.completed":
		norm.Type = EventDone
		turn := mapValue(raw["turn"])
		norm.Content = bestReason(raw, data, turn)
		norm.IsError = firstBoolLike(append(maps, turn), "is_error", "isError")
		if status := strings.ToLower(firstString([]map[string]any{raw, data, turn}, "status")); status == "failed" || status == "error" || status == "cancelled" || status == "canceled" {
			norm.IsError = true
		}
		if usage := mapValue(raw["usage"]); usage != nil {
			inTok, _ := getInt64(usage["input_tokens"])
			outTok, _ := getInt64(usage["output_tokens"])
			norm.Tokens = inTok + outTok
		}
		return norm, nil

	case "turn.failed":
		norm.Type = EventError
		norm.Content = bestReason(raw, data)
		norm.IsError = true
		return norm, nil

	case "system":
		sub, _ := raw["subtype"].(string)
		if sub == "init" {
			norm.Type = EventSession
			norm.SessionID, _ = raw["session_id"].(string)
			return norm, nil
		}
		if sub == "permission_denied" {
			norm.Type = EventPermission
			norm.ToolName, _ = raw["tool_name"].(string)
			norm.ToolID, _ = raw["tool_use_id"].(string)
			norm.Content, _ = raw["message"].(string)
			return norm, nil
		}
		return nil, nil

	case "stream_event":
		subEv, _ := raw["event"].(map[string]any)
		if subEv == nil {
			return nil, nil
		}
		subType, _ := subEv["type"].(string)

		switch subType {
		case "content_block_start":
			cb, _ := subEv["content_block"].(map[string]any)
			if cb == nil {
				return nil, nil
			}
			cbType, _ := cb["type"].(string)
			if cbType == "tool_use" {
				norm.Type = EventToolUse
				norm.ToolName, _ = cb["name"].(string)
				norm.ToolID, _ = cb["id"].(string)
				return norm, nil
			}
			return nil, nil

		case "content_block_delta":
			delta, _ := subEv["delta"].(map[string]any)
			if delta == nil {
				return nil, nil
			}
			deltaType, _ := delta["type"].(string)

			switch deltaType {
			case "text_delta":
				norm.Type = EventTextDelta
				norm.Content, _ = delta["text"].(string)
				return norm, nil
			case "thinking_delta":
				norm.Type = EventThinkDelta
				norm.Thinking, _ = delta["thinking"].(string)
				return norm, nil
			case "input_json_delta":
				norm.Type = EventToolUse
				norm.Content, _ = delta["partial_json"].(string)
				return norm, nil
			}
			return nil, nil
		}

	case "assistant":
		norm.Type = EventTextDelta
		if msg, ok := raw["message"].(map[string]any); ok {
			if contentList, ok := msg["content"].([]any); ok {
				var sb strings.Builder
				for _, item := range contentList {
					if cm, ok := item.(map[string]any); ok {
						if t, ok := cm["text"].(string); ok {
							sb.WriteString(t)
						}
					}
				}
				norm.Content = sb.String()
				if q, cleanText, found := ExtractQuestion(norm.Content); found {
					norm.Type = EventQuestion
					norm.Question = q
					norm.Content = cleanText
				}
			}
		}
		return norm, nil

	case "user":
		norm.Type = EventToolResult
		if msg, ok := raw["message"].(map[string]any); ok {
			if contentList, ok := msg["content"].([]any); ok {
				for _, item := range contentList {
					if cm, ok := item.(map[string]any); ok {
						if cm["type"] == "tool_result" {
							norm.ToolID, _ = cm["tool_use_id"].(string)
							if isErr, ok := cm["is_error"].(bool); ok {
								norm.IsError = isErr
							}
							norm.ToolOutput = contentString(cm["content"])
							return norm, nil
						}
					}
				}
			}
		}
		return norm, nil

	case "result":
		norm.Type = EventDone
		if cost, ok := raw["total_cost_usd"].(float64); ok {
			norm.CostUSD = cost
		}
		if dur, ok := raw["duration_ms"].(float64); ok {
			norm.DurationMs = int64(dur)
		}
		return norm, nil

	case "error":
		norm.Type = EventError
		norm.Content = bestReason(raw, data)
		norm.IsError = true
		return norm, nil
	}
	if bestReason(raw, data) != "" && (raw["error"] != nil || data["error"] != nil || firstBoolLike(maps, "is_error", "isError")) {
		norm.Type = EventError
		norm.Content = bestReason(raw, data)
		norm.IsError = true
		return norm, nil
	}
	if sessionID != "" {
		return norm, nil
	}

	return nil, nil
}
