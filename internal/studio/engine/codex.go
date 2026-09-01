package engine

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CodexAdapter implements EngineAdapter for the OpenAI Codex CLI.
type CodexAdapter struct{}

func (a *CodexAdapter) Name() string {
	return "codex"
}

func (a *CodexAdapter) DisplayName() string {
	return "OpenAI Codex CLI"
}

func (a *CodexAdapter) ExecutableName() string {
	return "codex"
}

func (a *CodexAdapter) BuildArgs(dir string, mode string, extraArgs []string) ([]string, error) {
	args := []string{
		"-p",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--verbose",
	}

	switch strings.ToLower(mode) {
	case "rw":
		args = append(args, "--full-auto")
	case "ro":
		args = append(args, "--read-only")
	case "ask":
		args = append(args, "--ask-permission")
	default:
		args = append(args, "--full-auto")
	}

	if len(extraArgs) > 0 {
		args = append(args, extraArgs...)
	}

	return args, nil
}

func (a *CodexAdapter) FormatUserMessage(prompt string) ([]byte, error) {
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
	return json.Marshal(msg)
}

func (a *CodexAdapter) NormalizeEvent(line []byte) (*NormalizedEvent, error) {
	line = []byte(strings.TrimSpace(string(line)))
	if len(line) == 0 {
		return nil, nil
	}

	var raw map[string]any
	if err := json.Unmarshal(line, &raw); err != nil {
		return &NormalizedEvent{
			Type:    "text_delta",
			Content: string(line),
			Raw:     raw,
		}, nil
	}

	evType, _ := raw["type"].(string)
	norm := &NormalizedEvent{
		Raw: raw,
	}

	switch evType {
	case "system":
		sub, _ := raw["subtype"].(string)
		if sub == "init" {
			norm.Type = "session"
			norm.SessionID, _ = raw["session_id"].(string)
			return norm, nil
		}
		if sub == "permission_denied" {
			norm.Type = "permission"
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
				norm.Type = "tool_use"
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
				norm.Type = "text_delta"
				norm.Content, _ = delta["text"].(string)
				return norm, nil
			case "thinking_delta":
				norm.Type = "think_delta"
				norm.Thinking, _ = delta["thinking"].(string)
				return norm, nil
			case "input_json_delta":
				norm.Type = "tool_use"
				norm.Content, _ = delta["partial_json"].(string)
				return norm, nil
			}
			return nil, nil
		}

	case "assistant":
		norm.Type = "text_delta"
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
					norm.Type = "question"
					norm.Question = q
					norm.Content = cleanText
				}
			}
		}
		return norm, nil

	case "user":
		norm.Type = "tool_result"
		if msg, ok := raw["message"].(map[string]any); ok {
			if contentList, ok := msg["content"].([]any); ok {
				for _, item := range contentList {
					if cm, ok := item.(map[string]any); ok {
						if cm["type"] == "tool_result" {
							norm.ToolID, _ = cm["tool_use_id"].(string)
							if isErr, ok := cm["is_error"].(bool); ok {
								norm.IsError = isErr
							}
							norm.ToolOutput = fmt.Sprintf("%v", cm["content"])
							return norm, nil
						}
					}
				}
			}
		}
		return norm, nil

	case "result":
		norm.Type = "done"
		if cost, ok := raw["total_cost_usd"].(float64); ok {
			norm.CostUSD = cost
		}
		if dur, ok := raw["duration_ms"].(float64); ok {
			norm.DurationMs = int64(dur)
		}
		return norm, nil

	case "error":
		norm.Type = "error"
		norm.Content = fmt.Sprintf("%v", raw["error"])
		norm.IsError = true
		return norm, nil
	}

	return nil, nil
}
