package engine

import (
	"encoding/json"
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
		{"generic", "generic"},
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
	if len(adapters) != 5 {
		t.Fatalf("expected 5 adapters, got %d", len(adapters))
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
	for _, expected := range []string{"claude", "opencode", "copilot", "generic"} {
		if !names[expected] {
			t.Errorf("missing adapter %q in ListAdapters()", expected)
		}
	}
}

func TestBuildArgs(t *testing.T) {
	// 1. Claude adapter
	claude := NewClaudeAdapter()
	argsRW, err := claude.BuildArgs(".", "rw", []string{"--custom-flag"})
	if err != nil {
		t.Fatalf("Claude BuildArgs rw failed: %v", err)
	}
	if !containsString(argsRW, "-p") || !containsString(argsRW, "--input-format") || !containsString(argsRW, "--allow-dangerously-skip-permissions") || !containsString(argsRW, "--custom-flag") {
		t.Errorf("unexpected Claude argsRW: %v", argsRW)
	}

	argsRO, err := claude.BuildArgs(".", "ro", nil)
	if err != nil {
		t.Fatalf("Claude BuildArgs ro failed: %v", err)
	}
	if !containsString(argsRO, "--disallowedTools") || !containsString(argsRO, "Write") {
		t.Errorf("unexpected Claude argsRO: %v", argsRO)
	}

	argsAsk, err := claude.BuildArgs(".", "ask", nil)
	if err != nil {
		t.Fatalf("Claude BuildArgs ask failed: %v", err)
	}
	if !containsString(argsAsk, "--permission-mode") || !containsString(argsAsk, "manual") {
		t.Errorf("unexpected Claude argsAsk: %v", argsAsk)
	}

	// 2. OpenCode adapter
	opencode := NewOpenCodeAdapter()
	opArgs, err := opencode.BuildArgs(".", "ro", []string{"--verbose"})
	if err != nil {
		t.Fatalf("OpenCode BuildArgs failed: %v", err)
	}
	if !containsString(opArgs, "-p") || !containsString(opArgs, "--read-only") || !containsString(opArgs, "--verbose") {
		t.Errorf("unexpected OpenCode args: %v", opArgs)
	}

	// 3. Copilot adapter
	copilot := NewCopilotAdapter()
	coArgs, err := copilot.BuildArgs(".", "rw", []string{"--log-level", "debug"})
	if err != nil {
		t.Fatalf("Copilot BuildArgs failed: %v", err)
	}
	if !containsString(coArgs, "-p") || !containsString(coArgs, "--allow-all") || !containsString(coArgs, "--log-level") {
		t.Errorf("unexpected Copilot args: %v", coArgs)
	}

	// 4. Generic adapter
	generic := NewGenericAdapter()
	genArgs, err := generic.BuildArgs(".", "rw", []string{"--run", "auto"})
	if err != nil {
		t.Fatalf("Generic BuildArgs failed: %v", err)
	}
	if len(genArgs) != 2 || genArgs[0] != "--run" || genArgs[1] != "auto" {
		t.Errorf("unexpected Generic args: %v", genArgs)
	}
}

func TestFormatUserMessage(t *testing.T) {
	adapters := ListAdapters()
	for _, adapter := range adapters {
		msgBytes, err := adapter.FormatUserMessage("Hello agent")
		if err != nil {
			t.Fatalf("%s FormatUserMessage failed: %v", adapter.Name(), err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(msgBytes, &parsed); err != nil {
			t.Fatalf("%s produced invalid JSON: %v, raw: %s", adapter.Name(), err, string(msgBytes))
		}
		if parsed["type"] != "user" {
			t.Errorf("%s message type = %v, want 'user'", adapter.Name(), parsed["type"])
		}
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
	if evRes.Type != EventDone || evRes.CostUSD != 0.0152 || evRes.DurationMs != 3200 || evRes.Tokens != 1850 || evRes.IsError {
		t.Errorf("unexpected result event: %#v", evRes)
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

func TestOpenCodeAdapterNormalizeEvent(t *testing.T) {
	opencode := NewOpenCodeAdapter()

	// 1. Session
	line := []byte(`{"type": "session", "session_id": "open-sess-1"}`)
	ev, err := opencode.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventSession || ev.SessionID != "open-sess-1" {
		t.Fatalf("OpenCode session normalize failed: %#v, err: %v", ev, err)
	}

	// 2. Text delta
	line = []byte(`{"type": "text_delta", "delta": "Generating storyboard"}`)
	ev, err = opencode.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventTextDelta || ev.Content != "Generating storyboard" {
		t.Fatalf("OpenCode text_delta normalize failed: %#v, err: %v", ev, err)
	}

	// 3. Thinking
	line = []byte(`{"type": "thinking", "content": "Selecting best voiceover model"}`)
	ev, err = opencode.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventThinkDelta || ev.Thinking != "Selecting best voiceover model" {
		t.Fatalf("OpenCode thinking normalize failed: %#v, err: %v", ev, err)
	}

	// 4. Tool call
	line = []byte(`{"type": "tool_call", "id": "tc_1", "name": "compose", "arguments": {"output": "out.mp4"}}`)
	ev, err = opencode.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventToolUse || ev.ToolName != "compose" || ev.ToolID != "tc_1" {
		t.Fatalf("OpenCode tool_call normalize failed: %#v, err: %v", ev, err)
	}

	// 5. Tool result
	line = []byte(`{"type": "tool_result", "id": "tc_1", "output": "rendered 10s video", "is_error": false}`)
	ev, err = opencode.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventToolResult || ev.ToolID != "tc_1" || ev.ToolOutput != "rendered 10s video" {
		t.Fatalf("OpenCode tool_result normalize failed: %#v, err: %v", ev, err)
	}

	// 6. Question
	line = []byte(`{"type": "question", "question": {"prompt": "Choose style", "options": ["cinematic", "fast-cut"]}}`)
	ev, err = opencode.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventQuestion || ev.Question == nil {
		t.Fatalf("OpenCode question normalize failed: %#v, err: %v", ev, err)
	}

	// 7. Permission
	line = []byte(`{"type": "permission", "message": "Permission needed to run shell command"}`)
	ev, err = opencode.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventPermission || ev.Content != "Permission needed to run shell command" {
		t.Fatalf("OpenCode permission normalize failed: %#v, err: %v", ev, err)
	}

	// 8. Error
	line = []byte(`{"type": "error", "message": "API key expired"}`)
	ev, err = opencode.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventError || !ev.IsError || ev.Content != "API key expired" {
		t.Fatalf("OpenCode error normalize failed: %#v, err: %v", ev, err)
	}

	// 9. Finish
	line = []byte(`{"type": "finish", "cost_usd": 0.004, "tokens": 420, "duration_ms": 1100}`)
	ev, err = opencode.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventDone || ev.CostUSD != 0.004 || ev.Tokens != 420 || ev.DurationMs != 1100 {
		t.Fatalf("OpenCode finish normalize failed: %#v, err: %v", ev, err)
	}
}

func TestCopilotAdapterNormalizeEvent(t *testing.T) {
	copilot := NewCopilotAdapter()

	// 1. Conversation create
	line := []byte(`{"type": "conversation.create", "conversation_id": "copilot-conv-1"}`)
	ev, err := copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventSession || ev.SessionID != "copilot-conv-1" {
		t.Fatalf("Copilot conversation.create normalize failed: %#v, err: %v", ev, err)
	}

	// 2. Message delta
	line = []byte(`{"type": "assistant.message.delta", "content": "Applying color grade"}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventTextDelta || ev.Content != "Applying color grade" {
		t.Fatalf("Copilot message.delta normalize failed: %#v, err: %v", ev, err)
	}

	// 3. Reasoning delta
	line = []byte(`{"type": "reasoning.delta", "reasoning": "Computing LUT table"}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventThinkDelta || ev.Thinking != "Computing LUT table" {
		t.Fatalf("Copilot reasoning.delta normalize failed: %#v, err: %v", ev, err)
	}

	// 4. Function call
	line = []byte(`{"type": "function_call", "call_id": "fn_1", "name": "color_grade", "parameters": {"lut": "warm"}}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventToolUse || ev.ToolName != "color_grade" || ev.ToolID != "fn_1" {
		t.Fatalf("Copilot function_call normalize failed: %#v, err: %v", ev, err)
	}

	// 5. Function result
	line = []byte(`{"type": "function_call_result", "call_id": "fn_1", "output": "color graded successfully", "is_error": false}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventToolResult || ev.ToolID != "fn_1" || ev.ToolOutput != "color graded successfully" {
		t.Fatalf("Copilot function_call_result normalize failed: %#v, err: %v", ev, err)
	}

	// 6. Approval request (permission)
	line = []byte(`{"type": "approval_request", "message": "Approve running remotion render?"}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventPermission || ev.Content != "Approve running remotion render?" {
		t.Fatalf("Copilot approval_request normalize failed: %#v, err: %v", ev, err)
	}

	// 7. Error
	line = []byte(`{"type": "error", "message": "Network timeout"}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventError || !ev.IsError || ev.Content != "Network timeout" {
		t.Fatalf("Copilot error normalize failed: %#v, err: %v", ev, err)
	}

	// 8. Turn finish
	line = []byte(`{"type": "turn.finish", "total_cost_usd": 0.002, "duration_ms": 850}`)
	ev, err = copilot.NormalizeEvent(line)
	if err != nil || ev == nil || ev.Type != EventDone || ev.CostUSD != 0.002 || ev.DurationMs != 850 {
		t.Fatalf("Copilot turn.finish normalize failed: %#v, err: %v", ev, err)
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
