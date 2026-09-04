package engine

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Event type constants for NormalizedEvent.
const (
	EventSession    = "session"
	EventTextDelta  = "text_delta"
	EventThinkDelta = "think_delta"
	EventToolUse    = "tool_use"
	EventToolResult = "tool_result"
	EventQuestion   = "question"
	EventPermission = "permission"
	EventDone       = "done"
	EventError      = "error"
)

// QuestionOption represents a single option in a question item.
type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// QuestionItem represents a single question prompt with optional options and multi-selection.
type QuestionItem struct {
	Header      string           `json:"header,omitempty"`
	Question    string           `json:"question"`
	MultiSelect bool             `json:"multi_select,omitempty"`
	Options     []QuestionOption `json:"options,omitempty"`
}

// QuestionSpec represents structured questions parsed from an agent's output.
type QuestionSpec struct {
	Questions []QuestionItem `json:"questions,omitempty"`
	Raw       string         `json:"raw,omitempty"`
}

// NormalizedEvent is a unified, engine-agnostic event model for CLI stream events.
type NormalizedEvent struct {
	Type       string         `json:"type"`
	SessionID  string         `json:"session_id,omitempty"`
	Content    string         `json:"content,omitempty"`
	Thinking   string         `json:"thinking,omitempty"`
	ToolName   string         `json:"tool_name,omitempty"`
	ToolID     string         `json:"tool_id,omitempty"`
	ToolInput  any            `json:"tool_input,omitempty"`
	ToolOutput string         `json:"tool_output,omitempty"`
	IsError    bool           `json:"is_error,omitempty"`
	CostUSD    float64        `json:"cost_usd,omitempty"`
	DurationMs int64          `json:"duration_ms,omitempty"`
	Tokens     int64          `json:"tokens,omitempty"`
	Question   any            `json:"question,omitempty"`
	Raw        map[string]any `json:"raw,omitempty"`
}

// EngineAdapter defines the interface for interacting with different agent CLI engines.
type EngineAdapter interface {
	Name() string
	DisplayName() string
	ExecutableName() string
	BuildTurnArgs(dir, mode, prompt, nativeID string, extraArgs []string) ([]string, error)
	NormalizeEvent(line []byte) (*NormalizedEvent, error)
}

func validateAutonomousMode(mode string) error {
	switch mode {
	case "", "rw":
		return nil
	default:
		return fmt.Errorf("unsupported engine mode %q: only autonomous mode (rw) is verified", mode)
	}
}

// GetAdapter returns the EngineAdapter corresponding to the given name.
// Supported names: "claude", "opencode", "codex", and "copilot".
// Defaults to "claude" if name is empty or unrecognized.
func GetAdapter(name string) EngineAdapter {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "claude", "claude-code":
		return NewClaudeAdapter()
	case "opencode":
		return NewOpenCodeAdapter()
	case "copilot", "gh-copilot", "github-copilot":
		return NewCopilotAdapter()
	case "codex", "openai-codex":
		return NewCodexAdapter()
	case "":
		return NewClaudeAdapter()
	default:
		return NewClaudeAdapter()
	}
}

// ListAdapters returns a slice of all registered standard adapters.
func ListAdapters() []EngineAdapter {
	return []EngineAdapter{
		NewClaudeAdapter(),
		NewOpenCodeAdapter(),
		NewCopilotAdapter(),
		NewCodexAdapter(),
	}
}

var questionFenceRegex = regexp.MustCompile("(?s)```question\\s*\\n?([\\s\\S]*?)\\n?```")

// ExtractQuestion checks if text contains a fenced ```question JSON block and parses it.
// Returns the parsed QuestionSpec, the text with the block removed, and a boolean indicating match.
func ExtractQuestion(text string) (*QuestionSpec, string, bool) {
	loc := questionFenceRegex.FindStringSubmatchIndex(text)
	if len(loc) < 4 {
		return nil, text, false
	}

	rawJSON := strings.TrimSpace(text[loc[2]:loc[3]])
	if rawJSON == "" {
		return nil, text, false
	}

	spec := &QuestionSpec{Raw: rawJSON}

	// 1. Try format: { "questions": [ ... ] }
	var container struct {
		Questions []QuestionItem `json:"questions"`
	}
	if err := json.Unmarshal([]byte(rawJSON), &container); err == nil && len(container.Questions) > 0 {
		spec.Questions = container.Questions
	} else {
		// 2. Try format: [ { "question": ... }, ... ]
		var items []QuestionItem
		if err := json.Unmarshal([]byte(rawJSON), &items); err == nil && len(items) > 0 {
			spec.Questions = items
		} else {
			// 3. Try format: single { "question": ... }
			var single QuestionItem
			if err := json.Unmarshal([]byte(rawJSON), &single); err == nil && single.Question != "" {
				spec.Questions = []QuestionItem{single}
			} else {
				// 4. Try generic object
				var generic any
				if err := json.Unmarshal([]byte(rawJSON), &generic); err == nil {
					// valid JSON, spec.Raw holds the raw data
				} else {
					return nil, text, false
				}
			}
		}
	}

	cleaned := strings.TrimSpace(text[:loc[0]] + " " + text[loc[1]:])
	return spec, cleaned, true
}

func getFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func getInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	}
	return 0, false
}

func getBoolLike(v any) (bool, bool) {
	switch value := v.(type) {
	case bool:
		return value, true
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "1", "yes", "on":
			return true, true
		case "false", "0", "no", "off":
			return false, true
		}
	case int:
		if value == 0 || value == 1 {
			return value == 1, true
		}
	case int64:
		if value == 0 || value == 1 {
			return value == 1, true
		}
	case float64:
		if value == 0 || value == 1 {
			return value == 1, true
		}
	case json.Number:
		if value.String() == "0" || value.String() == "1" {
			return value.String() == "1", true
		}
	}
	return false, false
}

func mapValue(value any) map[string]any {
	m, _ := value.(map[string]any)
	return m
}

func firstString(maps []map[string]any, keys ...string) string {
	for _, m := range maps {
		for _, key := range keys {
			if value, ok := m[key].(string); ok && value != "" {
				return value
			}
		}
	}
	return ""
}

func firstBoolLike(maps []map[string]any, keys ...string) bool {
	value, _ := firstBoolLikeValue(maps, keys...)
	return value
}

func firstBoolLikeValue(maps []map[string]any, keys ...string) (bool, bool) {
	for _, m := range maps {
		for _, key := range keys {
			if value, ok := getBoolLike(m[key]); ok {
				return value, true
			}
		}
	}
	return false, false
}

func contentString(value any) string {
	switch content := value.(type) {
	case string:
		return content
	case []any:
		var text strings.Builder
		for _, item := range content {
			text.WriteString(contentString(item))
		}
		return text.String()
	case map[string]any:
		for _, key := range []string{"text", "deltaContent", "content"} {
			if text := contentString(content[key]); text != "" {
				return text
			}
		}
	default:
		if value != nil {
			if encoded, err := json.Marshal(value); err == nil && string(encoded) != "null" {
				return string(encoded)
			}
		}
	}
	return ""
}

func valueString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	if value == nil {
		return ""
	}
	encoded, err := json.Marshal(value)
	if err != nil || string(encoded) == "null" {
		return ""
	}
	return string(encoded)
}

func firstFloatValue(maps []map[string]any, keys ...string) (float64, bool) {
	for _, m := range maps {
		for _, key := range keys {
			if value, ok := getFloat(m[key]); ok {
				return value, true
			}
		}
	}
	return 0, false
}

func firstInt64Value(maps []map[string]any, keys ...string) (int64, bool) {
	for _, m := range maps {
		for _, key := range keys {
			if value, ok := getInt64(m[key]); ok {
				return value, true
			}
		}
	}
	return 0, false
}

func tokenCount(values ...any) (int64, bool) {
	for _, value := range values {
		if count, ok := getInt64(value); ok {
			return count, true
		}
		tokens := mapValue(value)
		if tokens == nil {
			continue
		}
		if total, ok := firstInt64Value([]map[string]any{tokens}, "total", "total_tokens", "totalTokens"); ok {
			return total, true
		}

		var total int64
		found := false
		groups := [][]string{
			{"input", "input_tokens", "inputTokens", "prompt_tokens", "promptTokens"},
			{"output", "output_tokens", "outputTokens", "completion_tokens", "completionTokens"},
			{"reasoning", "reasoning_tokens", "reasoningTokens"},
			{"cache_read", "cacheRead", "cache_read_tokens", "cacheReadTokens"},
			{"cache_write", "cacheWrite", "cache_write_tokens", "cacheWriteTokens"},
		}
		for _, keys := range groups {
			if count, ok := firstInt64Value([]map[string]any{tokens}, keys...); ok {
				total += count
				found = true
			}
		}
		if cacheTotal, ok := cacheTokenCount(tokens["cache"]); ok {
			total += cacheTotal
			found = true
		}
		if found {
			return total, true
		}
	}
	return 0, false
}

func cacheTokenCount(value any) (int64, bool) {
	cache := mapValue(value)
	if cache == nil {
		return 0, false
	}
	var total int64
	found := false
	for _, keys := range [][]string{
		{"read", "cache_read", "cacheRead", "cache_read_tokens", "cacheReadTokens"},
		{"write", "cache_write", "cacheWrite", "cache_write_tokens", "cacheWriteTokens"},
	} {
		if count, ok := firstInt64Value([]map[string]any{cache}, keys...); ok {
			total += count
			found = true
		}
	}
	return total, found
}

func bestReason(maps ...map[string]any) string {
	for _, m := range maps {
		for _, key := range []string{"content", "message", "result", "error", "reason"} {
			value := m[key]
			if object, ok := value.(map[string]any); ok {
				if text := bestReason(object); text != "" {
					return text
				}
			}
			if text := strings.TrimSpace(contentString(value)); text != "" {
				return text
			}
			if object, ok := value.(map[string]any); ok && len(object) > 0 {
				if encoded, err := json.Marshal(object); err == nil {
					return string(encoded)
				}
			}
		}
	}
	return ""
}
