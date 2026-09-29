package module

import (
	"encoding/json"
	"testing"
)

// async is accepted for compatibility, but a per-invocation host cannot poll
// in-memory state from its next process. Facet therefore executes normally and
// never returns a handle that advertises an unusable lifecycle.
func TestAsyncDoesNotReturnAnUnpollableHandle(t *testing.T) {
	body, _ := json.Marshal(Request{
		RequestID: "req_async",
		Tool:      "media_probe",
		Input:     json.RawMessage(`{"input":"missing.mp4"}`),
		Async:     true,
	})

	env := Invoke(CapToolsRun, body)
	if env.OK {
		if res, ok := env.Result.(map[string]any); ok {
			if _, handle := res["job_id"]; handle {
				t.Fatal("per-invocation module returned an unpollable job handle")
			}
		}
	} else if env.Error == nil || env.Error.Code != "input_not_found" {
		t.Fatalf("unexpected synchronous outcome: %+v", env)
	}
	if HasRunningJobs() {
		t.Error("async compatibility flag started process-local background work")
	}
}

// Async is opt-in. Without it the caller gets a finished result, because the
// CLI and every existing consumer expect one.
func TestSyncRemainsTheDefault(t *testing.T) {
	body, _ := json.Marshal(Request{
		RequestID: "req_sync",
		Tool:      "media_probe",
		Input:     json.RawMessage(`{"input":"../../assets/source.mp4"}`),
	})
	env := Invoke(CapToolsRun, body)
	if !env.OK {
		t.Skipf("probe unavailable: %+v", env.Error)
	}
	res := env.Result.(map[string]any)
	if _, handle := res["job_id"]; handle {
		t.Error("a synchronous request returned a job handle")
	}
	if res["output"] == nil {
		t.Error("a synchronous request returned no output")
	}
}

// A module host launches one process per invocation. In-memory job state cannot
// be polled from the next process, so the descriptor must not advertise a
// long-running handle or a polling capability that always returns unknown_job.
func TestPerInvocationDescriptorDoesNotAdvertiseCrossProcessPolling(t *testing.T) {
	env := Describe("test")
	desc, ok := env.Result.(Descriptor)
	if !ok {
		t.Fatalf("result is not a Descriptor: %T", env.Result)
	}
	for _, c := range desc.Capabilities {
		if c.LongRunning {
			t.Errorf("capability %s advertises long_running to a per-invocation host", c.ID)
		}
		if c.PollCapability != "" {
			t.Errorf("capability %s advertises cross-process polling through %s",
				c.ID, c.PollCapability)
		}
		if c.ID == CapJobsStatus {
			t.Error("descriptor advertises an in-memory polling capability to fresh processes")
		}
	}
	runSchema := desc.RequestSchemas["creative.tools.run.request/v1"].(map[string]any)
	properties := runSchema["properties"].(map[string]any)
	if _, advertised := properties["async"]; advertised {
		t.Error("run schema advertises an async handle that a fresh process cannot poll")
	}
}
