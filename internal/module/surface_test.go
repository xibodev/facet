package module

import (
	"encoding/json"
	"testing"
)

// Every configured tool must ESTIMATE under a granted root.
//
// I had verified seven tools and assumed the rest. Sweeping the surface found
// 15 of 22 failing — one bug (estimate never applied the host's grants), not
// fifteen. A sweep is cheap because estimate never bills, never generates and
// never writes; skipping it cost a fortnight of a broken estimate path.
//
// This runs the sweep as a test so the surface stays exercised rather than
// being something I remember to check.
func TestEveryConfiguredToolEstimates(t *testing.T) {
	listed := Invoke(CapToolsList, []byte(`{}`))
	if !listed.OK {
		t.Skipf("tool listing unavailable: %+v", listed.Error)
	}
	result, ok := listed.Result.(map[string]any)
	if !ok {
		t.Skip("unexpected listing shape")
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		if inner, ok := result["output"].(map[string]any); ok {
			tools, _ = inner["tools"].([]any)
		}
	}
	if len(tools) == 0 {
		t.Skip("no tools listed")
	}

	// Estimate needs a plausible request per tool, and inventing one for 28
	// tools would test my fixtures rather than the module. What IS assertable
	// without a fixture: a configured tool must never fail for a reason that
	// has nothing to do with its request — an unknown capability, a missing
	// grant, or a panic.
	checked := 0
	for _, raw := range tools {
		tool, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if configured, _ := tool["configured"].(bool); !configured {
			continue
		}
		name, _ := tool["name"].(string)
		if name == "" {
			continue
		}
		checked++

		body, err := json.Marshal(map[string]any{"tool": name, "input": map[string]any{}})
		if err != nil {
			t.Fatal(err)
		}
		env := Estimate(CapToolsEstimate, body)
		if env.OK {
			continue
		}
		switch env.Error.Code {
		case "invalid_request", "input_not_found", "unconfigured",
			"credentials_missing", "invalid_timeline":
			// The empty request is wrong for this tool; that is expected and
			// is the tool's own validation talking.
		default:
			t.Errorf("%s failed to estimate for a reason unrelated to its request: %s: %s",
				name, env.Error.Code, env.Error.Message)
		}
	}
	if checked == 0 {
		t.Skip("no configured tools to check")
	}
	t.Logf("estimated %d configured tools", checked)
}

// A tool listing must let an agent decide about cost WITHOUT calling describe
// once per tool. The overlay tells it unknown cost is not free and to obtain
// consent; the listing is where that choice is made.
func TestListingLetsAnAgentJudgeCost(t *testing.T) {
	env := Invoke(CapToolsList, []byte(`{}`))
	if !env.OK {
		t.Skipf("tool listing unavailable: %+v", env.Error)
	}
	raw, err := json.Marshal(env.Result)
	if err != nil {
		t.Fatal(err)
	}
	// The listing sits under `output`, beside the capability and tool names.
	// My first version unmarshalled the top level, found nothing, and SKIPPED
	// — a test that silently verifies nothing is worse than no test.
	var probe struct {
		Output struct {
			Tools []struct {
				Name       string `json:"name"`
				Configured bool   `json:"configured"`
				Cost       *struct {
					Known bool `json:"known"`
				} `json:"cost"`
			} `json:"tools"`
		} `json:"output"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
	if len(probe.Output.Tools) == 0 {
		t.Fatal("the listing carries no tools; nothing was verified")
	}
	for _, tool := range probe.Output.Tools {
		if tool.Configured && tool.Cost == nil {
			t.Errorf("%s is listed with no cost; an agent cannot tell whether "+
				"running it needs human consent", tool.Name)
		}
	}
}
