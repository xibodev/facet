package module

import (
	"testing"
	"time"

	"github.com/xibodev/facet/internal/toolbox"
)

func TestAsyncCompatibilityAliasesKeepCanonicalJobIdentity(t *testing.T) {
	for _, tc := range []struct {
		alias     string
		canonical string
	}{
		{alias: "audio_mixer", canonical: "audio_mix"},
		{alias: "frame_sampler", canonical: "frame_sample"},
	} {
		t.Run(tc.alias, func(t *testing.T) {
			for _, outcome := range []string{"result", "error"} {
				t.Run(outcome, func(t *testing.T) {
					release := make(chan struct{})
					args := []string{"tools", "run", tc.alias, "--input", `{}`}
					runner := func(got []string) (toolbox.Envelope, bool) {
						if len(got) < 3 || got[2] != tc.alias {
							t.Errorf("dispatch tool = %v, want raw alias %q", got, tc.alias)
						}
						<-release
						if outcome == "result" {
							return toolbox.Envelope{
								OK: true, Tool: tc.canonical, Operation: "run",
								Result:   map[string]any{"status": "complete"},
								Warnings: []string{},
								Execution: toolbox.Execution{
									Provider: "local", ExternalWrite: true,
								},
							}, true
						}
						return toolbox.Envelope{
							OK: false, Tool: tc.canonical, Operation: "run",
							Error: &toolbox.ToolError{
								Code: "invalid_request", Message: "synthetic failure",
								Details: map[string]any{},
							},
							Warnings: []string{},
							Execution: toolbox.Execution{
								Provider: "local", ExternalWrite: true,
							},
						}, false
					}

					handle := startAsyncInvocation(
						"req-"+outcome, CapToolsRun, tc.canonical, args, nil, runner,
					)
					if !handle.OK {
						t.Fatalf("async handle failed: %+v", handle.Error)
					}
					result := handle.Result.(map[string]any)
					if result["tool"] != tc.canonical {
						t.Fatalf("handle tool = %q, want canonical %q", result["tool"], tc.canonical)
					}
					jobID := result["job_id"].(string)
					job, ok := lookupJob(jobID)
					if !ok {
						t.Fatal("started job was not stored")
					}
					if job.Tool != tc.canonical {
						t.Fatalf("stored job tool = %q, want canonical %q", job.Tool, tc.canonical)
					}

					close(release)
					deadline := time.Now().Add(2 * time.Second)
					for time.Now().Before(deadline) {
						job, ok = lookupJob(jobID)
						if !ok {
							t.Fatal("job vanished")
						}
						if job.State != JobRunning {
							break
						}
						time.Sleep(10 * time.Millisecond)
					}
					if job.State == JobRunning {
						t.Fatal("job did not finish")
					}
					if job.Tool != tc.canonical {
						t.Fatalf("terminal job tool = %q, want canonical %q", job.Tool, tc.canonical)
					}
					if outcome == "result" {
						if job.State != JobSucceeded {
							t.Fatalf("state = %q, want succeeded: %+v", job.State, job.Error)
						}
						projected := job.Result.(map[string]any)
						if projected["tool"] != tc.canonical {
							t.Fatalf("result tool = %q, want canonical %q", projected["tool"], tc.canonical)
						}
					} else {
						if job.State != JobFailed || job.Error == nil {
							t.Fatalf("state = %q error=%+v, want failed", job.State, job.Error)
						}
						if job.Error.Details["tool"] != tc.canonical {
							t.Fatalf("error tool = %q, want canonical %q",
								job.Error.Details["tool"], tc.canonical)
						}
					}
				})
			}
		})
	}
}
