package engine

import (
	"encoding/json"
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
	BuildArgs(dir string, mode string, extraArgs []string) ([]string, error)
	FormatUserMessage(prompt string) ([]byte, error)
	NormalizeEvent(line []byte) (*NormalizedEvent, error)
}

// GetAdapter returns the EngineAdapter corresponding to the given name.
// Supported names: "claude", "opencode", "copilot", "generic".
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
		return &CodexAdapter{}
	case "generic":
		return NewGenericAdapter()
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
		&CodexAdapter{},
		NewGenericAdapter(),
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
