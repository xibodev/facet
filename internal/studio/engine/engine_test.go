package engine

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestGetAdapter(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
	}{
		{"claude", "claude"},
		{"claude-code", "claude"},
		{"", "claude"},
		{"opencode", "opencode"},
		{"copilot", "copilot"},
		{"gh-copilot", "copilot"},
		{"github-copilot", "copilot"},
		{"codex", "codex"},
		{"openai-codex", "codex"},
		{"unknown-engine", "claude"}, // defaults to claude
	}

	for _, tt := range tests {
		adapter := GetAdapter(tt.input)
		if adapter == nil {
			t.Fatalf("GetAdapter(%q) returned nil", tt.input)
		}
		if adapter.Name() != tt.wantName {
			t.Errorf("GetAdapter(%q).Name() = %q, want %q", tt.input, adapter.Name(), tt.wantName)
		}
	}
}

func TestListAdapters(t *testing.T) {
	adapters := ListAdapters()
	if len(adapters) != 4 {
		t.Fatalf("expected 4 adapters, got %d", len(adapters))
	}
	names := make(map[string]bool)
	for _, a := range adapters {
		names[a.Name()] = true
		if a.DisplayName() == "" {
			t.Errorf("adapter %s has empty DisplayName", a.Name())
		}
		if a.ExecutableName() == "" {
			t.Errorf("adapter %s has empty ExecutableName", a.Name())
		}
	}
	for _, expected := range []string{"claude", "opencode", "codex", "copilot"} {
		if !names[expected] {
			t.Errorf("missing adapter %q in ListAdapters()", expected)
		}
	}
}

func TestBuildTurnArgsExactFirstAndResume(t *testing.T) {
	prompt := "Make a precise cut -- without parsing this as flags"
	tests := []struct {
		name    string
		adapter EngineAdapter
		native  string
		want    []string
	}{
		{
			name:    "claude first",
			adapter: NewClaudeAdapter(),
			want: []string{"--print", "--output-format", "stream-json", "--include-partial-messages", "--verbose",
				"--forward-subagent-text", "--dangerously-skip-permissions", prompt},
		},
		{
			name:    "claude resume",
			adapter: NewClaudeAdapter(),
			native:  "claude-native",
			want: []string{"--print", "--output-format", "stream-json", "--include-partial-messages", "--verbose",
				"--forward-subagent-text", "--dangerously-skip-permissions", "--resume", "claude-native", prompt},
		},
		{name: "opencode first", adapter: NewOpenCodeAdapter(), want: []string{"run", "--format", "json", "--auto", prompt}},
		{name: "opencode resume", adapter: NewOpenCodeAdapter(), native: "open-native", want: []string{"run", "--format", "json", "--auto", "--session", "open-native", prompt}},
		{name: "codex first", adapter: NewCodexAdapter(), want: []string{"exec", "--json", "--dangerously-bypass-approvals-and-sandbox", prompt}},
		{name: "codex resume", adapter: NewCodexAdapter(), native: "thread-native", want: []string{"exec", "resume", "--json", "--dangerously-bypass-approvals-and-sandbox", "thread-native", prompt}},
		{name: "copilot first", adapter: NewCopilotAdapter(), want: []string{"--allow-all", "--output-format", "json", "--stream", "on", "--prompt", prompt}},
		{name: "copilot resume", adapter: NewCopilotAdapter(), native: "copilot-native", want: []string{"--allow-all", "--output-format", "json", "--stream", "on", "--resume=copilot-native", "--prompt", prompt}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.adapter.BuildTurnArgs("ignored", "", prompt, tt.native, nil)
			if err != nil {
				t.Fatalf("BuildTurnArgs failed: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("args = %#v, want %#v", got, tt.want)
			}
			if containsString(got, "--allow-dangerously-skip-permissions") {
				t.Fatalf("args contain Claude allow-only flag: %#v", got)
			}
		})
	}

	got, err := NewOpenCodeAdapter().BuildTurnArgs(".", "rw", prompt, "", []string{"--model", "test"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"run", "--format", "json", "--auto", "--model", "test", prompt}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extra args = %#v, want %#v", got, want)
	}
}

func TestBuildTurnArgsRejectsUnverifiedModes(t *testing.T) {
	for _, adapter := range ListAdapters() {
		t.Run(adapter.Name(), func(t *testing.T) {
			for _, mode := range []string{"ro", "ask", "auto", "RW", " rw "} {
				args, err := adapter.BuildTurnArgs(".", mode, "prompt", "", nil)
				if err == nil || args != nil {
					t.Fatalf("BuildTurnArgs mode %q = %#v, %v; want nil args and error", mode, args, err)
				}
				if !strings.Contains(err.Error(), "only autonomous mode (rw) is verified") {
					t.Fatalf("BuildTurnArgs mode %q error = %q", mode, err)
				}
			}
		})
	}
}

func TestBuildTurnArgsAcceptsAutonomousModes(t *testing.T) {
	for _, adapter := range ListAdapters() {
		t.Run(adapter.Name(), func(t *testing.T) {
			for _, mode := range []string{"", "rw"} {
				args, err := adapter.BuildTurnArgs(".", mode, "prompt", "", nil)
				if err != nil || len(args) == 0 {
					t.Fatalf("BuildTurnArgs mode %q = %#v, %v", mode, args, err)
				}
			}
		})
	}
}

func TestBuildTurnArgsKeepsExtraArgsBeforeResumeAndPrompt(t *testing.T) {
	prompt := "prompt"
	tests := []struct {
		adapter EngineAdapter
		want    []string
	}{
		{NewClaudeAdapter(), []string{"--print", "--output-format", "stream-json", "--include-partial-messages", "--verbose", "--forward-subagent-text", "--dangerously-skip-permissions", "--model", "test", "--resume", "native", prompt}},
		{NewOpenCodeAdapter(), []string{"run", "--format", "json", "--auto", "--model", "test", "--session", "native", prompt}},
		{NewCodexAdapter(), []string{"exec", "resume", "--json", "--dangerously-bypass-approvals-and-sandbox", "--model", "test", "native", prompt}},
		{NewCopilotAdapter(), []string{"--allow-all", "--output-format", "json", "--stream", "on", "--model", "test", "--resume=native", "--prompt", prompt}},
	}
	for _, tt := range tests {
		t.Run(tt.adapter.Name(), func(t *testing.T) {
			got, err := tt.adapter.BuildTurnArgs(".", "rw", prompt, "native", []string{"--model", "test"})
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("BuildTurnArgs = %#v, %v; want %#v", got, err, tt.want)
			}
		})
	}
}

func TestExtractQuestion(t *testing.T) {
	// Case 1: Structured questions container
	text1 := `Here is the current state.
` + "```question" + `
{
  "questions": [
    {
      "header": "Ratio",
      "question": "What aspect ratio?",
      "multi_select": false,
      "options": [
        {"label": "16:9 Landscape", "description": "YouTube format"},
        {"label": "9:16 Vertical", "description": "Shorts format"}
      ]
    }
  ]
}
` + "```" + `
Please choose one.`

	spec1, cleaned1, ok1 := ExtractQuestion(text1)
	if !ok1 || spec1 == nil {
		t.Fatalf("ExtractQuestion case 1 failed to parse")
	}
	if len(spec1.Questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(spec1.Questions))
	}
	if spec1.Questions[0].Header != "Ratio" || spec1.Questions[0].Question != "What aspect ratio?" {
		t.Errorf("unexpected question item 1: %#v", spec1.Questions[0])
	}
	if len(spec1.Questions[0].Options) != 2 {
		t.Errorf("expected 2 options, got %d", len(spec1.Questions[0].Options))
	}
	if strings.Contains(cleaned1, "```question") {
		t.Errorf("cleaned text still contains fence: %q", cleaned1)
	}
	if !strings.Contains(cleaned1, "Here is the current state.") || !strings.Contains(cleaned1, "Please choose one.") {
		t.Errorf("cleaned text missing original prose: %q", cleaned1)
	}

	// Case 2: Array format
	text2 := "```question\n[\n  {\"question\": \"Select voice\", \"options\": [{\"label\": \"En-US-Guy\"}]}\n]\n```"
	spec2, _, ok2 := ExtractQuestion(text2)
	if !ok2 || len(spec2.Questions) != 1 || spec2.Questions[0].Question != "Select voice" {
		t.Fatalf("ExtractQuestion case 2 failed: %#v", spec2)
	}

	// Case 3: Single item object
	text3 := "```question\n{\"question\": \"Confirm deletion?\", \"options\": [{\"label\": \"Yes\"}, {\"label\": \"No\"}]}\n```"
	spec3, _, ok3 := ExtractQuestion(text3)
	if !ok3 || len(spec3.Questions) != 1 || spec3.Questions[0].Question != "Confirm deletion?" {
		t.Fatalf("ExtractQuestion case 3 failed: %#v", spec3)
	}

	// Case 4: No question
	text4 := "Just plain output text with no question block."
	_, _, ok4 := ExtractQuestion(text4)
	if ok4 {
		t.Errorf("ExtractQuestion should return false for plain text")
	}
}

func TestClaudeAdapterNormalizeEvent(t *testing.T) {
	claude := NewClaudeAdapter()

	// 1. System init
	lineInit := []byte(`{"type": "system", "subtype": "init", "session_id": "sess-xyz-123"}`)
	evInit, err := claude.NormalizeEvent(lineInit)
	if err != nil || evInit == nil {
		t.Fatalf("system/init failed: %v", err)
	}
	if evInit.Type != EventSession || evInit.SessionID != "sess-xyz-123" {
		t.Errorf("unexpected system/init event: %#v", evInit)
	}

	// 2. System permission denied
	linePerm := []byte(`{"type": "system", "subtype": "permission_denied", "message": "Command Bash was denied"}`)
	evPerm, err := claude.NormalizeEvent(linePerm)
	if err != nil || evPerm == nil {
		t.Fatalf("permission_denied failed: %v", err)
	}
	if evPerm.Type != EventPermission || !evPerm.IsError || !strings.Contains(evPerm.Content, "denied") {
		t.Errorf("unexpected permission event: %#v", evPerm)
	}

	// 3. Stream event: content_block_start text
	lineTextStart := []byte(`{"type": "stream_event", "event": {"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": "Drafting script"}}}`)
	evTextStart, err := claude.NormalizeEvent(lineTextStart)
	if err != nil || evTextStart == nil {
		t.Fatalf("content_block_start text failed: %v", err)
	}
	if evTextStart.Type != EventTextDelta || evTextStart.Content != "Drafting script" {
		t.Errorf("unexpected text start event: %#v", evTextStart)
	}

	// 4. Stream event: content_block_start thinking
	lineThinkStart := []byte(`{"type": "stream_event", "event": {"type": "content_block_start", "index": 1, "content_block": {"type": "thinking", "thinking": "Analyzing requirements..."}}}`)
	evThinkStart, err := claude.NormalizeEvent(lineThinkStart)
	if err != nil || evThinkStart == nil {
		t.Fatalf("content_block_start thinking failed: %v", err)
	}
	if evThinkStart.Type != EventThinkDelta || evThinkStart.Thinking != "Analyzing requirements..." {
		t.Errorf("unexpected thinking start event: %#v", evThinkStart)
	}

	// 5. Stream event: content_block_start tool_use
	lineToolStart := []byte(`{"type": "stream_event", "event": {"type": "content_block_start", "index": 2, "content_block": {"type": "tool_use", "id": "toolu_456", "name": "Bash", "input": {"command": "ffmpeg -version"}}}}`)
	evToolStart, err := claude.NormalizeEvent(lineToolStart)
	if err != nil || evToolStart == nil {
		t.Fatalf("content_block_start tool_use failed: %v", err)
	}
	if evToolStart.Type != EventToolUse || evToolStart.ToolName != "Bash" || evToolStart.ToolID != "toolu_456" {
		t.Errorf("unexpected tool_use start event: %#v", evToolStart)
	}

	// 6. Stream event: content_block_delta text with question block
	questionJSON := "```question\n{\"question\": \"Aspect ratio?\", \"options\": [{\"label\": \"16:9\"}]}\n```"
	qPayload := map[string]any{
		"type": "stream_event",
		"event": map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{
				"type": "text_delta",
				"text": questionJSON,
			},
		},
	}
	lineQuestionDelta, err := json.Marshal(qPayload)
	if err != nil {
		t.Fatalf("marshal question payload failed: %v", err)
	}
	evQDelta, err := claude.NormalizeEvent(lineQuestionDelta)
	if err != nil || evQDelta == nil {
		t.Fatalf("content_block_delta question failed: %v", err)
	}
	if evQDelta.Type != EventQuestion || evQDelta.Question == nil {
		t.Errorf("unexpected question delta event: %#v", evQDelta)
	}

	// 7. Assistant message with tool use
	lineAssistantTool := []byte(`{
		"type": "assistant",
		"message": {
			"role": "assistant",
			"content": [
				{
					"type": "tool_use",
					"id": "toolu_789",
					"name": "Write",
					"input": {"file_path": "projects/demo/brief.md", "content": "hello"}
				}
			]
		}
	}`)
	evAssTool, err := claude.NormalizeEvent(lineAssistantTool)
	if err != nil || evAssTool == nil {
		t.Fatalf("assistant tool_use failed: %v", err)
	}
	if evAssTool.Type != EventToolUse || evAssTool.ToolName != "Write" || evAssTool.ToolID != "toolu_789" {
		t.Errorf("unexpected assistant tool event: %#v", evAssTool)
	}

	// 8. User message with tool_result
	lineUserToolRes := []byte(`{
		"type": "user",
		"message": {
			"role": "user",
			"content": [
				{
					"type": "tool_result",
					"tool_use_id": "toolu_789",
					"content": "File written successfully",
					"is_error": false
				}
			]
		}
	}`)
	evToolRes, err := claude.NormalizeEvent(lineUserToolRes)
	if err != nil || evToolRes == nil {
		t.Fatalf("user tool_result failed: %v", err)
	}
	if evToolRes.Type != EventToolResult || evToolRes.ToolID != "toolu_789" || evToolRes.ToolOutput != "File written successfully" || evToolRes.IsError {
		t.Errorf("unexpected tool_result event: %#v", evToolRes)
	}

	// 9. Result event
	lineResult := []byte(`{
		"type": "result",
		"session_id": "sess-xyz-123",
		"total_cost_usd": 0.0152,
		"duration_ms": 3200,
		"usage": {
			"input_tokens": 1500,
			"output_tokens": 350
		},
		"is_error": false
	}`)
	evRes, err := claude.NormalizeEvent(lineResult)
	if err != nil || evRes == nil {
		t.Fatalf("result failed: %v", err)
	}
	if evRes.Type != EventDone || evRes.SessionID != "sess-xyz-123" || evRes.CostUSD != 0.0152 || evRes.DurationMs != 3200 || evRes.Tokens != 1850 || evRes.IsError {
		t.Errorf("unexpected result event: %#v", evRes)
	}

	lineFailedResult := []byte(`{"type":"result","session_id":"sess-xyz-123","is_error":"true","result":"model failed"}`)
	evFailed, err := claude.NormalizeEvent(lineFailedResult)
	if err != nil || evFailed == nil || evFailed.Type != EventDone || !evFailed.IsError || evFailed.Content != "model failed" {
		t.Fatalf("strict bool-like result failed: %#v, err: %v", evFailed, err)
	}

	// 11. Assistant message with question block
	lineAssQuestion := []byte(`{
		"type": "assistant",
		"message": {
			"role": "assistant",
			"content": [
				{
					"type": "text",
					"text": "Before I generate the voiceover:\n` + "```question\n{\"question\": \"Select tone\", \"options\": [{\"label\": \"Dramatic\"}]}\n```" + `"
				}
			]
		}
	}`)
	evAssQ, err := claude.NormalizeEvent(lineAssQuestion)
	if err != nil || evAssQ == nil {
		t.Fatalf("assistant question failed: %v", err)
	}
	if evAssQ.Type != EventQuestion || evAssQ.Question == nil {
		t.Errorf("expected EventQuestion for assistant with question block, got %#v", evAssQ)
	}

	// 12. User message with array tool_result content
	lineArrayRes := []byte(`{
		"type": "user",
		"message": {
			"role": "user",
			"content": [
				{
					"type": "tool_result",
					"tool_use_id": "toolu_arr_1",
					"content": [{"type": "text", "text": "Part 1 "}, {"type": "text", "text": "Part 2"}],
					"is_error": false
				}
			]
		}
	}`)
	evArrRes, err := claude.NormalizeEvent(lineArrayRes)
	if err != nil || evArrRes == nil {
		t.Fatalf("user array tool_result failed: %v", err)
	}
	if evArrRes.Type != EventToolResult || evArrRes.ToolOutput != "Part 1 Part 2" {
		t.Errorf("unexpected array tool_result event: %#v", evArrRes)
	}
}

func TestClaudeAdapterPreservesRawBlockStop(t *testing.T) {
	event, err := NewClaudeAdapter().NormalizeEvent([]byte(`{"type":"stream_event","session_id":"claude-stream","event":{"type":"content_block_stop","index":2}}`))
	if err != nil || event == nil || event.Raw == nil || event.Type != "" || event.SessionID != "claude-stream" {
		t.Fatalf("Claude block stop = %#v, %v", event, err)
	}
}

func TestClaudeAdapterPreservesRawLifecycleEvents(t *testing.T) {
	event, err := NewClaudeAdapter().NormalizeEvent([]byte(`{"type":"rate_limit_event","rate_limit_info":{"status":"allowed"}}`))
	if err != nil || event == nil || event.Raw == nil || event.Type != "" {
		t.Fatalf("Claude lifecycle event = %#v, %v", event, err)
	}
}

func TestOpenCodeAdapterNormalizeEvent(t *testing.T) {
	opencode := NewOpenCodeAdapter()

	// Synthetic fixtures shaped like OpenCode's JSON event stream.
	line := []byte(`{"type":"step_start","sessionID":"open-sess-1","part":{"type":"step-start","sessionID":"open-sess-1"}}`)
	ev, err := opencode.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventSession || ev.SessionID != "open-sess-1" {
		t.Fatalf("OpenCode step_start normalize failed: %#v, err: %v", ev, err)
	}

	line = []byte(`{"type":"text","sessionID":"open-sess-1","part":{"type":"text","text":"Generating storyboard"}}`)
	ev, err = opencode.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventTextDelta || ev.Content != "Generating storyboard" || ev.SessionID != "open-sess-1" {
		t.Fatalf("OpenCode text normalize failed: %#v, err: %v", ev, err)
	}

	line = []byte(`{"type":"tool","part":{"type":"tool","sessionID":"open-sess-1","tool":"bash","callID":"call-1","state":{"status":"pending","input":{"command":"ffmpeg -version"}}}}`)
	ev, err = opencode.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventToolUse || ev.SessionID != "open-sess-1" || ev.ToolName != "bash" || ev.ToolID != "call-1" {
		t.Fatalf("OpenCode pending tool normalize failed: %#v, err: %v", ev, err)
	}
	input, ok := ev.ToolInput.(map[string]any)
	if !ok || input["command"] != "ffmpeg -version" {
		t.Fatalf("OpenCode pending tool input = %#v", ev.ToolInput)
	}

	line = []byte(`{"type":"tool","part":{"type":"tool","sessionID":"open-sess-1","tool":"bash","callID":"call-1","state":{"status":"running","input":{"command":"ffmpeg -version"}}}}`)
	ev, err = opencode.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventToolUse || ev.ToolName != "bash" || ev.ToolID != "call-1" {
		t.Fatalf("OpenCode running tool normalize failed: %#v, err: %v", ev, err)
	}

	line = []byte(`{"type":"tool","part":{"type":"tool","sessionID":"open-sess-1","tool":"bash","callID":"call-1","state":{"status":"completed","input":{"command":"ffmpeg -version"},"output":"ffmpeg version 7"}}}`)
	ev, err = opencode.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventToolResult || ev.ToolName != "bash" || ev.ToolID != "call-1" || ev.ToolOutput != "ffmpeg version 7" || ev.IsError {
		t.Fatalf("OpenCode completed tool normalize failed: %#v, err: %v", ev, err)
	}
	line = []byte(`{"type":"tool","part":{"type":"tool","sessionID":"open-sess-1","tool":"media_probe","callID":"call-object","state":{"status":"completed","input":{"path":"final.mp4"},"output":{"duration":10,"codec":"h264"},"error":null}}}`)
	ev, err = opencode.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventToolResult || ev.ToolOutput != `{"codec":"h264","duration":10}` || ev.IsError {
		t.Fatalf("OpenCode object tool output normalize failed: %#v, err: %v", ev, err)
	}

	line = []byte(`{"type":"tool","data":{"sessionID":"open-sess-data"},"part":{"type":"tool","tool":"write","callID":"call-2","state":{"status":"error","input":{"filePath":"out.mp4"},"error":"disk full"}}}`)
	ev, err = opencode.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventToolResult || ev.SessionID != "open-sess-data" || ev.ToolID != "call-2" || ev.ToolOutput != "disk full" || !ev.IsError {
		t.Fatalf("OpenCode failed tool normalize failed: %#v, err: %v", ev, err)
	}

	for i, intermediate := range []string{
		`{"type":"step_finish","sessionID":"open-sess-1","part":{"type":"step-finish","reason":"tool-calls","cost":0.001,"tokens":{"input":10,"output":2}}}`,
		`{"type":"step_finish","sessionID":"open-sess-1","part":{"type":"step-finish","reason":"tool-calls","cost":0.002,"tokens":{"input":20,"output":3}}}`,
		`{"type":"step_finish","sessionID":"open-sess-1","part":{"type":"step-finish","reason":"tool_calls","cost":0.002,"tokens":{"input":20,"output":3}}}`,
	} {
		ev, err = opencode.NormalizeEvent([]byte(intermediate))
		if err != nil || ev == nil || ev.Type == EventDone || ev.SessionID != "open-sess-1" || ev.Raw == nil {
			t.Fatalf("OpenCode intermediate step_finish %d = %#v, err: %v", i, ev, err)
		}
	}

	line = []byte(`{"type":"step_finish","part":{"type":"step-finish","sessionID":"open-sess-final","reason":"stop","cost":0.004,"tokens":{"input":400,"output":20,"cache":{"read":5,"write":2}}}}`)
	ev, err = opencode.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventDone || ev.SessionID != "open-sess-final" || ev.CostUSD != 0.004 || ev.Tokens != 427 || ev.IsError {
		t.Fatalf("OpenCode final step_finish normalize failed: %#v, err: %v", ev, err)
	}
	line = []byte(`{"type":"result","data":{"sessionID":"open-sess-result","result":"complete","cost":0.006,"tokens":{"inputTokens":30,"outputTokens":7,"cache":{"read":4,"write":1}},"durationMs":1100}}`)
	ev, err = opencode.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventDone || ev.SessionID != "open-sess-result" || ev.Content != "complete" || ev.CostUSD != 0.006 || ev.Tokens != 42 || ev.DurationMs != 1100 {
		t.Fatalf("OpenCode result normalize failed: %#v, err: %v", ev, err)
	}

	line = []byte(`{"type":"step_finish","sessionID":"open-sess-1","isError":"1","result":"OpenCode failed"}`)
	ev, err = opencode.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventError || !ev.IsError || ev.Content != "OpenCode failed" {
		t.Fatalf("OpenCode failed step_finish normalize failed: %#v, err: %v", ev, err)
	}
}

func TestOpenCodeStepFinishTerminalReasons(t *testing.T) {
	opencode := NewOpenCodeAdapter()

	for _, reason := range []string{"stop", "complete", "completed", "completion", "done", "end-turn", "end_turn"} {
		t.Run("success/"+reason, func(t *testing.T) {
			line, err := json.Marshal(map[string]any{
				"type":      "step_finish",
				"sessionID": "open-success-" + reason,
				"part": map[string]any{
					"type":   "step-finish",
					"reason": reason,
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			event, err := opencode.NormalizeEvent(line)
			if err != nil || event == nil || event.Type != EventDone || event.IsError || event.Content != reason {
				t.Fatalf("OpenCode successful step_finish %q = %#v, err: %v", reason, event, err)
			}
		})
	}

	for _, reason := range []string{"length", "content-filter", "content_filter", "cancellation", "failure", "error"} {
		t.Run("failure/"+reason, func(t *testing.T) {
			line, err := json.Marshal(map[string]any{
				"type":      "step_finish",
				"sessionID": "open-failure-" + reason,
				"part": map[string]any{
					"type":   "step-finish",
					"reason": reason,
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			event, err := opencode.NormalizeEvent(line)
			if err != nil || event == nil || event.Type != EventError || !event.IsError || event.Content != reason {
				t.Fatalf("OpenCode unsuccessful step_finish %q = %#v, err: %v", reason, event, err)
			}
		})
	}
}

func TestCodexAdapterNormalizeJSONLEvents(t *testing.T) {
	codex := NewCodexAdapter()
	fixtures := []struct {
		line        string
		wantType    string
		wantSession string
		wantContent string
	}{
		{`{"type":"thread.started","thread_id":"thread-123"}`, EventSession, "thread-123", ""},
		{`{"type":"turn.started","thread_id":"thread-123"}`, EventSession, "thread-123", ""},
		{`{"type":"item.completed","thread_id":"thread-123","item":{"id":"item-1","type":"agent_message","text":"Rendered the cut"}}`, EventTextDelta, "thread-123", "Rendered the cut"},
		{`{"type":"turn.completed","thread_id":"thread-123","usage":{"input_tokens":12,"output_tokens":5}}`, EventDone, "thread-123", ""},
	}
	failed, err := codex.NormalizeEvent([]byte(`{"type":"turn.failed","thread_id":"thread-123","error":{"message":"Codex failed"}}`))
	if err != nil || failed == nil || failed.Type != EventError || !failed.IsError || failed.Content != "Codex failed" {
		t.Fatalf("Codex turn.failed = %#v, %v", failed, err)
	}
	for _, fixture := range fixtures {
		event, err := codex.NormalizeEvent([]byte(fixture.line))
		if err != nil || event == nil {
			t.Fatalf("NormalizeEvent(%s) = %#v, %v", fixture.line, event, err)
		}
		if event.Type != fixture.wantType || event.SessionID != fixture.wantSession || event.Content != fixture.wantContent {
			t.Fatalf("NormalizeEvent(%s) = %#v", fixture.line, event)
		}
	}

	started, err := codex.NormalizeEvent([]byte(`{"type":"item.started","thread_id":"thread-123","item":{"id":"cmd-1","type":"command_execution","command":"ffmpeg -version","status":"in_progress"}}`))
	if err != nil || started == nil || started.Type != EventToolUse || started.ToolName != "command_execution" || started.ToolID != "cmd-1" || started.ToolInput != "ffmpeg -version" {
		t.Fatalf("Codex command start = %#v, %v", started, err)
	}
	completed, err := codex.NormalizeEvent([]byte(`{"type":"item.completed","thread_id":"thread-123","item":{"id":"cmd-1","type":"command_execution","command":"ffmpeg -version","aggregated_output":"ffmpeg version 7","exit_code":0,"status":"completed"}}`))
	if err != nil || completed == nil || completed.Type != EventToolResult || completed.ToolName != "command_execution" || completed.ToolID != "cmd-1" || completed.ToolOutput != "ffmpeg version 7" || completed.IsError {
		t.Fatalf("Codex command completion = %#v, %v", completed, err)
	}
	failedCommand, err := codex.NormalizeEvent([]byte(`{"type":"item.completed","thread_id":"thread-123","item":{"id":"cmd-2","type":"command_execution","command":"false","aggregated_output":"command failed","exit_code":1,"status":"failed"}}`))
	if err != nil || failedCommand == nil || failedCommand.Type != EventToolResult || !failedCommand.IsError || failedCommand.ToolOutput != "command failed" {
		t.Fatalf("Codex failed command = %#v, %v", failedCommand, err)
	}
}

func TestNormalizerExtractsNestedNativeSessionIDs(t *testing.T) {
	tests := []struct {
		name    string
		adapter EngineAdapter
		line    string
		want    string
	}{
		{"opencode nested part", NewOpenCodeAdapter(), `{"type":"metadata","part":{"sessionID":"open-nested"}}`, "open-nested"},
		{"codex nested data", NewCodexAdapter(), `{"type":"metadata","data":{"thread_id":"codex-nested"}}`, "codex-nested"},
		{"copilot nested data", NewCopilotAdapter(), `{"type":"metadata","data":{"sessionId":"copilot-nested"}}`, "copilot-nested"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := tt.adapter.NormalizeEvent([]byte(tt.line))
			if err != nil || event == nil || event.Type != "" || event.SessionID != tt.want {
				t.Fatalf("NormalizeEvent = %#v, %v", event, err)
			}
		})
	}
}

func TestNormalizersTreatTopLevelErrorFieldsAsFailure(t *testing.T) {
	tests := []struct {
		name    string
		adapter EngineAdapter
		line    string
	}{
		{"claude", NewClaudeAdapter(), `{"type":"unexpected","session_id":"c","error":{"message":"failed"}}`},
		{"opencode", NewOpenCodeAdapter(), `{"type":"unexpected","sessionID":"o","error":{"message":"failed"}}`},
		{"codex", NewCodexAdapter(), `{"type":"unexpected","thread_id":"x","error":{"message":"failed"}}`},
		{"copilot", NewCopilotAdapter(), `{"type":"unexpected","data":{"sessionId":"p","error":{"message":"failed"}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := tt.adapter.NormalizeEvent([]byte(tt.line))
			if err != nil || event == nil || event.Type != EventError || !event.IsError || event.Content != "failed" {
				t.Fatalf("NormalizeEvent = %#v, %v", event, err)
			}
		})
	}
}

func TestCopilotAdapterNormalizeEvent(t *testing.T) {
	copilot := NewCopilotAdapter()

	// Synthetic fixtures shaped like captured Copilot JSONL records.
	line := []byte(`{"type":"session.start","data":{"sessionId":"copilot-conv-1"}}`)
	ev, err := copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventSession || ev.SessionID != "copilot-conv-1" {
		t.Fatalf("Copilot session.start normalize failed: %#v, err: %v", ev, err)
	}

	line = []byte(`{"type":"assistant.reasoning_delta","data":{"sessionId":"copilot-conv-1","deltaContent":"Checking the edit"}}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventThinkDelta || ev.Thinking != "Checking the edit" {
		t.Fatalf("Copilot reasoning delta normalize failed: %#v, err: %v", ev, err)
	}

	line = []byte(`{"type":"assistant.reasoning","data":{"sessionId":"copilot-conv-1","content":"The edit is valid"}}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventThinkDelta || ev.Thinking != "The edit is valid" {
		t.Fatalf("Copilot reasoning normalize failed: %#v, err: %v", ev, err)
	}

	line = []byte(`{"type":"tool.execution_start","data":{"sessionId":"copilot-conv-1","toolCallId":"tool-1","toolName":"shell","arguments":{"command":"ffmpeg -version"}}}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventToolUse || ev.ToolID != "tool-1" || ev.ToolName != "shell" {
		t.Fatalf("Copilot tool start normalize failed: %#v, err: %v", ev, err)
	}
	input, ok := ev.ToolInput.(map[string]any)
	if !ok || input["command"] != "ffmpeg -version" {
		t.Fatalf("Copilot tool input = %#v", ev.ToolInput)
	}

	line = []byte(`{"type":"tool.execution_complete","data":{"sessionId":"copilot-conv-1","toolCallId":"tool-1","toolName":"shell","result":{"stdout":"ffmpeg version 7"},"success":true}}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventToolResult || ev.ToolID != "tool-1" || ev.ToolName != "shell" || ev.ToolOutput != `{"stdout":"ffmpeg version 7"}` || ev.IsError {
		t.Fatalf("Copilot tool completion normalize failed: %#v, err: %v", ev, err)
	}

	line = []byte(`{"type":"tool.execution_complete","data":{"sessionId":"copilot-conv-1","toolCallId":"tool-2","result":"permission denied","success":false}}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventToolResult || ev.ToolID != "tool-2" || ev.ToolOutput != "permission denied" || !ev.IsError {
		t.Fatalf("Copilot failed tool normalize failed: %#v, err: %v", ev, err)
	}

	line = []byte(`{"type":"assistant.message_delta","data":{"sessionId":"copilot-conv-1","messageId":"msg-streamed","message":{"deltaContent":"Applying "}}}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventTextDelta || ev.Content != "Applying " {
		t.Fatalf("Copilot first message delta normalize failed: %#v, err: %v", ev, err)
	}
	line = []byte(`{"type":"assistant.message_delta","data":{"sessionId":"copilot-conv-1","messageId":"msg-streamed","deltaContent":"color grade"}}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventTextDelta || ev.Content != "color grade" {
		t.Fatalf("Copilot second message delta normalize failed: %#v, err: %v", ev, err)
	}
	line = []byte(`{"type":"assistant.message","data":{"sessionId":"copilot-conv-1","messageId":"msg-streamed","content":"Applying color grade"}}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev != nil {
		t.Fatalf("Copilot streamed final message should be deduplicated: %#v, err: %v", ev, err)
	}
	line = []byte(`{"type":"assistant.message","data":{"sessionId":"copilot-conv-1","messageId":"msg-final-only","content":"A non-streamed final answer"}}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventTextDelta || ev.Content != "A non-streamed final answer" {
		t.Fatalf("Copilot non-streamed final message normalize failed: %#v, err: %v", ev, err)
	}

	for i := 0; i < 2; i++ {
		line = []byte(`{"type":"assistant.turn_end","data":{"sessionId":"copilot-conv-1"}}`)
		ev, err = copilot.NormalizeEvent(line)
		if err != nil || ev == nil || ev.Type == EventDone || ev.SessionID != "copilot-conv-1" || ev.Raw == nil {
			t.Fatalf("Copilot assistant.turn_end %d = %#v, err: %v", i, ev, err)
		}
	}

	line = []byte(`{"type":"session.shutdown","status":"completed","data":{"sessionId":"copilot-conv-1","reason":"complete","shutdown":"completed","exitCode":0,"usage":{"input_tokens":10,"output_tokens":4}}}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventDone || ev.SessionID != "copilot-conv-1" || ev.Content != "complete" || ev.Tokens != 14 || ev.IsError {
		t.Fatalf("Copilot session.shutdown normalize failed: %#v, err: %v", ev, err)
	}

	line = []byte(`{"type":"assistant.message","data":{"sessionId":"copilot-conv-1","messageId":"msg-streamed","content":"Allowed after state reset"}}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventTextDelta || ev.Content != "Allowed after state reset" {
		t.Fatalf("Copilot shutdown did not clear dedup state: %#v, err: %v", ev, err)
	}

	line = []byte(`{"type":"result","exitCode":0,"data":{"sessionId":"copilot-conv-1","result":"finished","status":"completed","cost":0.002,"durationMs":850,"tokens":{"input":5,"output":2}}}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventDone || ev.Content != "finished" || ev.CostUSD != 0.002 || ev.DurationMs != 850 || ev.Tokens != 7 || ev.IsError {
		t.Fatalf("Copilot final result normalize failed: %#v, err: %v", ev, err)
	}
	line = []byte(`{"type":"assistant.message","data":{"sessionId":"copilot-conv-1","messageId":"msg-streamed","content":"Allowed after result reset"}}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventTextDelta || ev.Content != "Allowed after result reset" {
		t.Fatalf("Copilot result did not clear dedup state: %#v, err: %v", ev, err)
	}
	line = []byte(`{"type":"result","data":{"sessionId":"copilot-conv-1","isError":true,"error":{"message":"Final result failed"}}}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventDone || !ev.IsError || ev.Content != "Final result failed" {
		t.Fatalf("Copilot failed result normalize failed: %#v, err: %v", ev, err)
	}

	line = []byte(`{"type":"session.error","data":{"sessionId":"copilot-conv-1","error":{"message":"Copilot failed"}}}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventError || !ev.IsError || ev.Content != "Copilot failed" {
		t.Fatalf("Copilot session.error normalize failed: %#v, err: %v", ev, err)
	}

	line = []byte(`{"type":"assistant.idle","data":{"sessionId":"copilot-conv-1"}}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != "" || ev.SessionID != "copilot-conv-1" {
		t.Fatalf("Copilot invented assistant.idle should not terminate: %#v, err: %v", ev, err)
	}
}

func TestCopilotTerminalFailureSignals(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{
			name: "shutdown data exit code",
			line: `{"type":"session.shutdown","data":{"sessionId":"copilot-exit-data","exitCode":17,"reason":"agent process exited"}}`,
		},
		{
			name: "result top-level exit code",
			line: `{"type":"result","exitCode":2,"data":{"sessionId":"copilot-exit-top","result":"agent process exited"}}`,
		},
		{
			name: "shutdown failed",
			line: `{"type":"session.shutdown","data":{"sessionId":"copilot-shutdown-failed","shutdown":"failed","reason":"shutdown failed"}}`,
		},
		{
			name: "result error status",
			line: `{"type":"result","status":"error","data":{"sessionId":"copilot-status-error","result":"request failed"}}`,
		},
		{
			name: "shutdown canceled status",
			line: `{"type":"session.shutdown","data":{"sessionId":"copilot-status-canceled","status":"canceled","reason":"request canceled"}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event, err := NewCopilotAdapter().NormalizeEvent([]byte(test.line))
			if err != nil || event == nil || event.Type != EventDone || !event.IsError || event.Content == "" {
				t.Fatalf("Copilot terminal failure = %#v, err: %v", event, err)
			}
		})
	}
}

func TestGenericAdapterNormalizeEvent(t *testing.T) {
	generic := NewGenericAdapter()

	// 1. Empty line
	ev, err := generic.NormalizeEvent([]byte("   \n"))
	if err != nil || ev != nil {
		t.Fatalf("expected nil for empty line, got %#v", ev)
	}

	// 2. Plain text line
	ev, err = generic.NormalizeEvent([]byte("Plain text log line"))
	if err != nil || ev == nil || ev.Type != EventTextDelta || ev.Content != "Plain text log line" {
		t.Fatalf("Generic plain text failed: %#v, err: %v", ev, err)
	}

	// 3. Plain text error line
	ev, err = generic.NormalizeEvent([]byte("error: failed to execute tool"))
	if err != nil || ev == nil || ev.Type != EventError || !ev.IsError || ev.Content != "error: failed to execute tool" {
		t.Fatalf("Generic error line failed: %#v, err: %v", ev, err)
	}

	// 4. JSON with unknown type but text
	ev, err = generic.NormalizeEvent([]byte(`{"content": "Hello from custom engine"}`))
	if err != nil || ev == nil || ev.Type != EventTextDelta || ev.Content != "Hello from custom engine" {
		t.Fatalf("Generic custom JSON failed: %#v, err: %v", ev, err)
	}
}

func containsString(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
