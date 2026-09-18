package module

import (
	"encoding/json"
	"testing"
	"time"
)

// The whole point of a handle is that it returns before the work finishes.
// A "handle" that arrives after 83 seconds solves nothing.
func TestAsyncReturnsAHandleImmediately(t *testing.T) {
	body, _ := json.Marshal(Request{
		RequestID: "req_async",
		Tool:      "media_probe",
		Input:     json.RawMessage(`{"input":"../../assets/source.mp4"}`),
		Async:     true,
	})

	start := time.Now()
	env := Invoke(CapToolsRun, body)
	elapsed := time.Since(start)

	if !env.OK {
		t.Fatalf("async invocation failed: %+v", env.Error)
	}
	if elapsed > 2*time.Second {
		t.Errorf("handle took %v to return; it must not wait for the work", elapsed)
	}

	res := env.Result.(map[string]any)
	jobID, _ := res["job_id"].(string)
	if jobID == "" {
		t.Fatal("no job_id in the handle")
	}
	if res["state"] != string(JobRunning) {
		t.Errorf("state = %v, want running", res["state"])
	}
	if res["poll_capability"] != CapJobsStatus {
		t.Error("the handle must name the capability that polls it")
	}

	// Provisional execution: the work has not run, so claiming a cost or
	// naming artifacts would be a claim about something that did not happen.
	if env.Execution.ActualCost != nil {
		t.Errorf("actual_cost = %v on a handle; the work has not run", *env.Execution.ActualCost)
	}
	if len(env.Execution.Artifacts) != 0 {
		t.Error("a handle must not name artifacts that do not exist yet")
	}

	if err := ValidateEnvelopeBytes(mustMarshal(t, env)); err != nil {
		t.Errorf("handle envelope is not protocol-valid: %v", err)
	}

	// And the job must actually reach a terminal state rather than hang.
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := lookupJob(jobID)
		if !ok {
			t.Fatal("the job vanished")
		}
		if job.State != JobRunning {
			if job.State != JobSucceeded {
				t.Skipf("probe unavailable in this environment: %+v", job.Error)
			}
			if job.Execution == nil {
				t.Error("a terminal job carries no execution; the poll would report nothing")
			}
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Error("the job never left the running state")
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

// Async on a capability that is not declared long-running must run normally
// rather than hand back a handle the descriptor never promised.
func TestAsyncIgnoredForShortCapabilities(t *testing.T) {
	body, _ := json.Marshal(Request{RequestID: "req_async_list", Async: true})
	env := Invoke(CapToolsList, body)
	if !env.OK {
		t.Fatalf("list failed: %+v", env.Error)
	}
	if _, handle := env.Result.(map[string]any)["job_id"]; handle {
		t.Error("a capability that does not declare long_running returned a handle")
	}
}

// The descriptor and the code must agree about which capabilities can produce
// a handle. A capability declaring long_running that refuses to produce one is
// a contract violation the host cannot see until it asks.
func TestLongRunningDeclarationMatchesBehaviour(t *testing.T) {
	env := Describe("test")
	desc, ok := env.Result.(Descriptor)
	if !ok {
		t.Fatalf("result is not a Descriptor: %T", env.Result)
	}
	for _, c := range desc.Capabilities {
		if c.LongRunning != isLongRunning(c.ID) {
			t.Errorf("capability %s declares long_running=%v but the code says %v",
				c.ID, c.LongRunning, isLongRunning(c.ID))
		}
		if c.LongRunning && c.PollCapability == "" {
			t.Errorf("capability %s is long_running but names no poll capability", c.ID)
		}
	}
}
