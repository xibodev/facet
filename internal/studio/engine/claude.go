package engine

import (
	"bytes"
	"encoding/json"
	"strings"
)

// ClaudeAdapter implements EngineAdapter for Claude Code CLI.
type ClaudeAdapter struct {
	Executable string
}

// NewClaudeAdapter creates a new ClaudeAdapter instance.
func NewClaudeAdapter() *ClaudeAdapter {
	return &ClaudeAdapter{
		Executable: "claude",
	}
}

func (a *ClaudeAdapter) Name() string {
	return "claude"
}

func (a *ClaudeAdapter) DisplayName() string {
	return "Claude Code CLI"
}

func (a *ClaudeAdapter) ExecutableName() string {
	if a.Executable != "" {
		return a.Executable
	}
	return "claude"
}

// BuildTurnArgs constructs one autonomous Claude invocation.
func (a *ClaudeAdapter) BuildTurnArgs(dir, mode, prompt, nativeID string, extraArgs []string) ([]string, error) {
	if err := validateAutonomousMode(mode); err != nil {
		return nil, err
	}
	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--include-partial-messages",
		"--verbose",
		"--forward-subagent-text",
		"--dangerously-skip-permissions",
	}
	args = append(args, extraArgs...)
	if nativeID != "" {
		args = append(args, "--resume", nativeID)
	}
	args = append(args, prompt)
	return args, nil
}

// NormalizeEvent normalizes a raw Claude stream-json line into a NormalizedEvent.
func (a *ClaudeAdapter) NormalizeEvent(line []byte) (normalized *NormalizedEvent, err error) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return nil, nil
	}

	var raw map[string]any
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		// Non-JSON fallback: check if error or plain text
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
	sessionID, _ := raw["session_id"].(string)
	defer func() {
		if normalized != nil && normalized.SessionID == "" {
			normalized.SessionID = sessionID
		}
	}()

	evType, _ := raw["type"].(string)

	switch evType {
	case "system":
		subtype, _ := raw["subtype"].(string)
		if subtype == "init" {
			return &NormalizedEvent{
				Type:      EventSession,
				SessionID: sessionID,
				Raw:       raw,
			}, nil
		}
		if subtype == "permission_denied" {
			msg, _ := raw["message"].(string)
			if msg == "" {
				msg = "Permission denied"
			}
			return &NormalizedEvent{
				Type:    EventPermission,
				Content: msg,
				IsError: true,
				Raw:     raw,
			}, nil
		}
		// Fallback for general system events
		msg, _ := raw["message"].(string)
		return &NormalizedEvent{
			Type:      EventSession,
			SessionID: sessionID,
			Content:   msg,
			Raw:       raw,
		}, nil

	case "stream_event":
		event, ok := raw["event"].(map[string]any)
		if !ok {
			return nil, nil
		}
		subType, _ := event["type"].(string)

		switch subType {
		case "content_block_start":
			cb, ok := event["content_block"].(map[string]any)
			if !ok {
				return nil, nil
			}
			cbType, _ := cb["type"].(string)
			switch cbType {
			case "text":
				txt, _ := cb["text"].(string)
				if spec, cleaned, ok := ExtractQuestion(txt); ok {
					return &NormalizedEvent{
						Type:     EventQuestion,
						Content:  cleaned,
						Question: spec,
						Raw:      raw,
					}, nil
				}
				return &NormalizedEvent{
					Type:    EventTextDelta,
					Content: txt,
					Raw:     raw,
				}, nil
			case "thinking":
				th, _ := cb["thinking"].(string)
				return &NormalizedEvent{
					Type:     EventThinkDelta,
					Thinking: th,
					Raw:      raw,
				}, nil
			case "tool_use":
				name, _ := cb["name"].(string)
				id, _ := cb["id"].(string)
				input := cb["input"]
				return &NormalizedEvent{
					Type:      EventToolUse,
					ToolName:  name,
					ToolID:    id,
					ToolInput: input,
					Raw:       raw,
				}, nil
			}

		case "content_block_delta":
			delta, ok := event["delta"].(map[string]any)
			if !ok {
				return nil, nil
			}
			deltaType, _ := delta["type"].(string)

			if deltaType == "text_delta" || delta["text"] != nil {
				txt, _ := delta["text"].(string)
				if spec, cleaned, ok := ExtractQuestion(txt); ok {
					return &NormalizedEvent{
						Type:     EventQuestion,
						Content:  cleaned,
						Question: spec,
						Raw:      raw,
					}, nil
				}
				return &NormalizedEvent{
					Type:    EventTextDelta,
					Content: txt,
					Raw:     raw,
				}, nil
			}
			if deltaType == "thinking_delta" || delta["thinking"] != nil {
				th, _ := delta["thinking"].(string)
				return &NormalizedEvent{
					Type:     EventThinkDelta,
					Thinking: th,
					Raw:      raw,
				}, nil
			}
			if deltaType == "input_json_delta" || delta["partial_json"] != nil {
				pj, _ := delta["partial_json"].(string)
				return &NormalizedEvent{
					Type:    EventToolUse,
					Content: pj,
					Raw:     raw,
				}, nil
			}

		case "content_block_stop":
			return &NormalizedEvent{Raw: raw}, nil
		}

	case "assistant":
		msg, ok := raw["message"].(map[string]any)
		if !ok {
			return nil, nil
		}
		contentList, ok := msg["content"].([]any)
		if !ok || len(contentList) == 0 {
			return nil, nil
		}

		for _, itemVal := range contentList {
			item, ok := itemVal.(map[string]any)
			if !ok {
				continue
			}
			itemType, _ := item["type"].(string)
			switch itemType {
			case "text":
				txt, _ := item["text"].(string)
				if spec, cleaned, ok := ExtractQuestion(txt); ok {
					return &NormalizedEvent{
						Type:     EventQuestion,
						Content:  cleaned,
						Question: spec,
						Raw:      raw,
					}, nil
				}
				return &NormalizedEvent{
					Type:    EventTextDelta,
					Content: txt,
					Raw:     raw,
				}, nil
			case "tool_use":
				name, _ := item["name"].(string)
				id, _ := item["id"].(string)
				input := item["input"]
				return &NormalizedEvent{
					Type:      EventToolUse,
					ToolName:  name,
					ToolID:    id,
					ToolInput: input,
					Raw:       raw,
				}, nil
			case "thinking":
				th, _ := item["thinking"].(string)
				return &NormalizedEvent{
					Type:     EventThinkDelta,
					Thinking: th,
					Raw:      raw,
				}, nil
			}
		}

	case "user":
		msg, ok := raw["message"].(map[string]any)
		if !ok {
			return nil, nil
		}
		contentList, ok := msg["content"].([]any)
		if !ok {
			return nil, nil
		}

		for _, itemVal := range contentList {
			item, ok := itemVal.(map[string]any)
			if !ok {
				continue
			}
			itemType, _ := item["type"].(string)
			if itemType == "tool_result" {
				toolID, _ := item["tool_use_id"].(string)
				isErr, _ := item["is_error"].(bool)
				var outStr string
				switch v := item["content"].(type) {
				case string:
					outStr = v
				case []any:
					var sb strings.Builder
					for _, elem := range v {
						if em, ok := elem.(map[string]any); ok {
							if t, ok := em["text"].(string); ok {
								sb.WriteString(t)
							}
						}
					}
					outStr = sb.String()
				default:
					if v != nil {
						b, _ := json.Marshal(v)
						outStr = string(b)
					}
				}
				return &NormalizedEvent{
					Type:       EventToolResult,
					ToolID:     toolID,
					ToolOutput: outStr,
					IsError:    isErr,
					Raw:        raw,
				}, nil
			}
		}

	case "permission":
		msg, _ := raw["message"].(string)
		return &NormalizedEvent{
			Type:    EventPermission,
			Content: msg,
			Raw:     raw,
		}, nil

	case "result":
		cost, _ := getFloat(raw["total_cost_usd"])
		dur, _ := getInt64(raw["duration_ms"])
		isErr, _ := getBoolLike(raw["is_error"])
		var tokens int64
		if usage, ok := raw["usage"].(map[string]any); ok {
			outTok, _ := getInt64(usage["output_tokens"])
			inTok, _ := getInt64(usage["input_tokens"])
			tokens = outTok + inTok
			if tokens == 0 && outTok > 0 {
				tokens = outTok
			}
		}
		return &NormalizedEvent{
			Type:       EventDone,
			SessionID:  firstString([]map[string]any{raw}, "session_id"),
			Content:    bestReason(raw),
			CostUSD:    cost,
			DurationMs: dur,
			Tokens:     tokens,
			IsError:    isErr,
			Raw:        raw,
		}, nil

	case "error":
		return &NormalizedEvent{
			Type:    EventError,
			Content: bestReason(raw),
			IsError: true,
			Raw:     raw,
		}, nil
	}
	if bestReason(raw) != "" && (raw["error"] != nil || raw["is_error"] != nil) {
		return &NormalizedEvent{
			Type:    EventError,
			Content: bestReason(raw),
			IsError: true,
			Raw:     raw,
		}, nil
	}

	return &NormalizedEvent{Raw: raw}, nil
}
