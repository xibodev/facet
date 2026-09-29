package engine

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
)

// CopilotAdapter implements EngineAdapter for GitHub Copilot CLI.
type CopilotAdapter struct {
	Executable       string
	mu               sync.Mutex
	streamedMessages map[string]struct{}
}

// NewCopilotAdapter creates a new CopilotAdapter instance.
func NewCopilotAdapter() *CopilotAdapter {
	return &CopilotAdapter{
		Executable:       "copilot",
		streamedMessages: make(map[string]struct{}),
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

// BuildTurnArgs constructs one autonomous Copilot invocation.
func (a *CopilotAdapter) BuildTurnArgs(dir, mode, prompt, nativeID string, extraArgs []string) ([]string, error) {
	if err := validateAutonomousMode(mode); err != nil {
		return nil, err
	}
	args := []string{"--allow-all", "--output-format", "json", "--stream", "on"}
	args = append(args, extraArgs...)
	if nativeID != "" {
		args = append(args, "--resume="+nativeID)
	}
	args = append(args, "--prompt", prompt)
	return args, nil
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
	data := mapValue(raw["data"])
	message := mapValue(data["message"])
	maps := []map[string]any{raw, data, message}
	sessionID := firstString(maps, "sessionId", "session_id", "conversation_id")
	messageID := firstString([]map[string]any{data, message}, "messageId", "messageID", "message_id", "id")

	switch evType {
	case "session.start", "session", "conversation.create", "init":
		a.clearStreamedMessages()
		if sessionID == "" {
			sessionID = firstString(maps, "id")
		}
		return &NormalizedEvent{
			Type:      EventSession,
			SessionID: sessionID,
			Raw:       raw,
		}, nil

	case "assistant.message_delta":
		a.markStreamedMessage(messageID)
		var content string
		for _, value := range []any{data["deltaContent"], data["delta"], message["deltaContent"], message["delta"], data["content"], data["message"], message["content"], raw["content"], raw["delta"], raw["text"]} {
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

	case "assistant.message":
		if a.wasStreamedMessage(messageID) {
			return nil, nil
		}
		var content string
		for _, value := range []any{data["content"], data["message"], message["content"], data["deltaContent"], raw["content"], raw["text"]} {
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

	case "assistant.reasoning", "assistant.reasoning_delta":
		var thinking string
		for _, value := range []any{data["deltaContent"], data["delta"], data["content"], data["reasoning"], message["content"], raw["reasoning"], raw["thinking"], raw["delta"]} {
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

	case "tool.execution_start":
		return &NormalizedEvent{
			Type:      EventToolUse,
			SessionID: sessionID,
			ToolName:  firstString([]map[string]any{data}, "toolName"),
			ToolID:    firstString([]map[string]any{data}, "toolCallId", "toolCallID"),
			ToolInput: data["arguments"],
			Raw:       raw,
		}, nil

	case "tool.execution_complete":
		isError := false
		if success, ok := firstBoolLikeValue([]map[string]any{data}, "success"); ok {
			isError = !success
		} else if status := strings.ToLower(firstString([]map[string]any{data}, "status")); status == "failed" || status == "error" || status == "cancelled" || status == "canceled" {
			isError = true
		}
		result := data["result"]
		if result == nil && data["error"] != nil {
			result = data["error"]
		}
		return &NormalizedEvent{
			Type:       EventToolResult,
			SessionID:  sessionID,
			ToolName:   firstString([]map[string]any{data}, "toolName"),
			ToolID:     firstString([]map[string]any{data}, "toolCallId", "toolCallID"),
			ToolOutput: valueString(result),
			IsError:    isError,
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

	case "permission", "approval_request":
		msg, _ := raw["message"].(string)
		return &NormalizedEvent{
			Type:      EventPermission,
			SessionID: sessionID,
			Content:   msg,
			Raw:       raw,
		}, nil

	case "assistant.turn_end":
		return &NormalizedEvent{SessionID: sessionID, Raw: raw}, nil

	case "session.shutdown":
		a.clearStreamedMessages()
		return copilotDoneEvent(raw, data, message, sessionID), nil

	case "result":
		a.clearStreamedMessages()
		return copilotDoneEvent(raw, data, message, sessionID), nil

	case "session.error", "error":
		return &NormalizedEvent{
			Type:      EventError,
			SessionID: sessionID,
			Content:   bestReason(raw, data, message),
			IsError:   true,
			Raw:       raw,
		}, nil
	}
	if bestReason(raw, data, message) != "" && (raw["error"] != nil || data["error"] != nil || firstBoolLike(maps, "is_error", "isError")) {
		return &NormalizedEvent{Type: EventError, SessionID: sessionID, Content: bestReason(raw, data, message), IsError: true, Raw: raw}, nil
	}
	if sessionID != "" {
		return &NormalizedEvent{SessionID: sessionID, Raw: raw}, nil
	}

	return nil, nil
}

func (a *CopilotAdapter) markStreamedMessage(messageID string) {
	if messageID == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.streamedMessages == nil {
		a.streamedMessages = make(map[string]struct{})
	}
	a.streamedMessages[messageID] = struct{}{}
}

func (a *CopilotAdapter) wasStreamedMessage(messageID string) bool {
	if messageID == "" {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	_, found := a.streamedMessages[messageID]
	return found
}

func (a *CopilotAdapter) clearStreamedMessages() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.streamedMessages = make(map[string]struct{})
}

func copilotDoneEvent(raw, data, message map[string]any, sessionID string) *NormalizedEvent {
	maps := []map[string]any{data, raw}
	cost, _ := firstFloatValue(maps, "cost", "cost_usd", "costUSD", "total_cost_usd", "totalCostUSD")
	duration, _ := firstInt64Value(maps, "duration_ms", "durationMs")
	tokens, _ := tokenCount(data["tokens"], raw["tokens"], data["usage"], raw["usage"])
	isError := firstBoolLike(maps, "is_error", "isError")
	for _, terminal := range maps {
		if exitCode, ok := firstInt64Value([]map[string]any{terminal}, "exitCode", "exit_code"); ok && exitCode != 0 {
			isError = true
		}
		for _, key := range []string{"shutdown", "status"} {
			if status, ok := terminal[key].(string); ok && isCopilotFailureStatus(status) {
				isError = true
			}
		}
	}
	return &NormalizedEvent{
		Type:       EventDone,
		SessionID:  sessionID,
		Content:    bestReason(raw, data, message),
		CostUSD:    cost,
		DurationMs: duration,
		Tokens:     tokens,
		IsError:    isError,
		Raw:        raw,
	}
}

func isCopilotFailureStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "failure", "error", "canceled", "cancelled":
		return true
	default:
		return false
	}
}
