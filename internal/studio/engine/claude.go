package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// ClaudeDeniedTools contains tools disallowed in read-only mode.
var ClaudeDeniedTools = []string{
	"Write", "Edit", "NotebookEdit", "Bash", "PowerShell",
	"Task", "Agent", "Workflow", "SendMessage", "CronCreate", "CronDelete",
	"EnterWorktree", "ExitWorktree", "DesignSync", "Artifact",
}

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

// BuildArgs constructs the command-line arguments for running Claude in stream-json mode.
func (a *ClaudeAdapter) BuildArgs(dir string, mode string, extraArgs []string) ([]string, error) {
	args := []string{
		"-p",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
		"--forward-subagent-text",
	}

	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "rw":
		args = append(args, "--allow-dangerously-skip-permissions")
	case "ro":
		args = append(args, "--disallowedTools")
		args = append(args, ClaudeDeniedTools...)
		args = append(args, "--allow-dangerously-skip-permissions")
	case "ask":
		args = append(args, "--permission-mode", "manual")
	default:
		if mode == "" || mode == "rw" {
			args = append(args, "--allow-dangerously-skip-permissions")
		} else {
			args = append(args, "--permission-mode", "manual")
		}
	}

	if len(extraArgs) > 0 {
		args = append(args, extraArgs...)
	}

	return args, nil
}

// FormatUserMessage formats a prompt into a Claude stream-json user message payload.
func (a *ClaudeAdapter) FormatUserMessage(prompt string) ([]byte, error) {
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
		return nil, fmt.Errorf("marshal claude user message: %w", err)
	}
	return append(b, '\n'), nil
}

// NormalizeEvent normalizes a raw Claude stream-json line into a NormalizedEvent.
func (a *ClaudeAdapter) NormalizeEvent(line []byte) (*NormalizedEvent, error) {
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

	evType, _ := raw["type"].(string)

	switch evType {
	case "system":
		subtype, _ := raw["subtype"].(string)
		if subtype == "init" {
			sessionID, _ := raw["session_id"].(string)
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
		sessionID, _ := raw["session_id"].(string)
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
			return nil, nil
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
		isErr, _ := raw["is_error"].(bool)
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
