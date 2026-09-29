package module

import (
	"encoding/json"
	"testing"
)

// A poll must report what was observed and nothing more.
func TestJobLifecycle(t *testing.T) {
	j := startJob(CapToolsRun, "video_compose")

	env := JobStatus([]byte(`{"job_id":"` + j.JobID + `"}`))
	if !env.OK {
		t.Fatalf("polling a live job failed: %+v", env.Error)
	}
	res := env.Result.(map[string]any)
	job := res["job"].(Job)
	if job.State != JobRunning {
		t.Errorf("state = %q, want running", job.State)
	}
	// Progress is unknown until the renderer reports it. A fabricated number
	// is worse than none: a stalled render showing a confident 40% reads as
	// healthy.
	if job.Percent != nil {
		t.Errorf("percent = %v, want null while progress is unknown", *job.Percent)
	}
	// A running poll must not claim a cost the work has not incurred.
	if env.Execution.ActualCost == nil || *env.Execution.ActualCost != 0 {
		t.Error("a local poll should report its own cost as 0")
	}

	// The terminal poll carries the authoritative execution of the work.
	cost := 1.25
	finishJob(j.JobID, Envelope{
		OK:     true,
		Result: map[string]any{"output": "renders/final.mp4"},
		Execution: Execution{
			Local: false, Network: true, ExternalWrites: true,
			Provider: "google_flow", ActualCost: &cost, Artifacts: []Artifact{},
		},
	})

	env = JobStatus([]byte(`{"job_id":"` + j.JobID + `"}`))
	job = env.Result.(map[string]any)["job"].(Job)
	if job.State != JobSucceeded {
		t.Errorf("state = %q, want succeeded", job.State)
	}
	if env.Execution.ActualCost == nil || *env.Execution.ActualCost != cost {
		t.Error("the terminal poll must carry the work's authoritative cost")
	}
	if job.FinishedAt == nil {
		t.Error("a terminal job must record when it finished")
	}
}

// A failed job reports the failure rather than a silent success.
func TestJobFailureIsReported(t *testing.T) {
	j := startJob(CapToolsRun, "gflow_image")
	finishJob(j.JobID, Envelope{
		OK:        false,
		Error:     &Error{Code: "command_failed", Message: "provider rejected the request"},
		Execution: Execution{Provider: "google_flow", Artifacts: []Artifact{}},
	})
	job := JobStatus([]byte(`{"job_id":"` + j.JobID + `"}`)).
		Result.(map[string]any)["job"].(Job)
	if job.State != JobFailed {
		t.Errorf("state = %q, want failed", job.State)
	}
	if job.Error == nil || job.Error.Code != "command_failed" {
		t.Error("the failure reason was lost")
	}
}

// Job state does not survive the module process, so an unknown id must say so
// rather than report a state nobody observed.
func TestUnknownJobIsRefused(t *testing.T) {
	env := JobStatus([]byte(`{"job_id":"job_never_existed"}`))
	if env.OK {
		t.Fatal("an unknown job reported a state")
	}
	if env.Error.Code != "unknown_job" {
		t.Errorf("code = %q, want unknown_job", env.Error.Code)
	}
	if err := ValidateEnvelopeBytes(mustMarshal(t, env)); err != nil {
		t.Errorf("refusal envelope is not protocol-valid: %v", err)
	}
}

// A poll with no job_id is a malformed request, not an unknown job.
func TestJobStatusRequiresID(t *testing.T) {
	env := JobStatus([]byte(`{}`))
	if env.OK || env.Error.Code != "invalid_request" {
		t.Errorf("missing job_id: ok=%v code=%v", env.OK, env.Error)
	}
}

// The poll result must survive the wire as the host will read it.
func TestJobStatusEnvelopeIsProtocolValid(t *testing.T) {
	j := startJob(CapToolsRun, "video_compose")
	raw, err := json.Marshal(JobStatus([]byte(`{"job_id":"` + j.JobID + `"}`)))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateEnvelopeBytes(raw); err != nil {
		t.Errorf("poll envelope is not protocol-valid: %v", err)
	}
	var probe struct {
		Result struct {
			Job struct {
				Percent *float64 `json:"percent"`
			} `json:"job"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
	if probe.Result.Job.Percent != nil {
		t.Error("unknown progress must serialize as null, never as a number")
	}
}
