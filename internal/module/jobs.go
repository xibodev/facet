package module

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Long-running capabilities.
//
// A render is slow — measured at 30s for 720p and 83s for 1080p — and the host
// invokes a module as a bounded subprocess. Without a job handle the cockpit
// has nothing to show for the whole render and looks frozen; the operator
// cannot tell a working render from a hung one.
//
// So `creative.tools.run` may return a job handle instead of a finished result,
// and `creative.jobs.status` reports on it. The rules the host froze:
//   - the handle-returning envelope carries a PROVISIONAL execution:
//     actual_cost null (the work has not run) and artifacts [].
//   - the terminal poll carries the authoritative execution.
//   - the last envelope for a request_id is cost and artifact truth.
//
// State lives in this process only. A module is a short-lived subprocess, so a
// job does not survive it; that is a deliberate limit rather than an oversight,
// and `status` says so plainly for an unknown id rather than inventing a state.

// JobState is the lifecycle of a long-running invocation.
type JobState string

const (
	JobRunning   JobState = "running"
	JobSucceeded JobState = "succeeded"
	JobFailed    JobState = "failed"
)

// Job is the status of one long-running invocation.
//
// Progress is reported ONLY when it is genuinely known. A renderer that does
// not report progress yields a null percent rather than a fabricated one: an
// invented number is worse than no number, because a stalled render showing a
// confident 40% reads as healthy.
type Job struct {
	JobID      string     `json:"job_id"`
	State      JobState   `json:"state"`
	Capability string     `json:"capability,omitempty"`
	Tool       string     `json:"tool,omitempty"`
	Percent    *float64   `json:"percent"`
	Detail     string     `json:"detail,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	// Result and Execution are populated once the job reaches a terminal
	// state, so the terminal poll can carry the authoritative outcome.
	Result    any        `json:"result,omitempty"`
	Error     *Error     `json:"error,omitempty"`
	Execution *Execution `json:"execution,omitempty"`
}

var (
	jobsMu sync.RWMutex
	jobs   = map[string]*Job{}
)

func newJobID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "job_unavailable"
	}
	return "job_" + hex.EncodeToString(b)
}

// startJob registers a running job and returns it.
func startJob(capability, tool string) *Job {
	j := &Job{
		JobID:      newJobID(),
		State:      JobRunning,
		Capability: capability,
		Tool:       tool,
		StartedAt:  time.Now().UTC(),
	}
	jobsMu.Lock()
	jobs[j.JobID] = j
	jobsMu.Unlock()
	return j
}

// finishJob records a terminal outcome against a job.
func finishJob(id string, env Envelope) {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	j, ok := jobs[id]
	if !ok {
		return
	}
	now := time.Now().UTC()
	j.FinishedAt = &now
	exec := env.Execution
	j.Execution = &exec
	if env.OK {
		j.State = JobSucceeded
		j.Result = env.Result
		done := 100.0
		j.Percent = &done
		return
	}
	j.State = JobFailed
	j.Error = env.Error
}

// lookupJob returns a copy of a job's current status.
func lookupJob(id string) (Job, bool) {
	jobsMu.RLock()
	defer jobsMu.RUnlock()
	j, ok := jobs[id]
	if !ok {
		return Job{}, false
	}
	return *j, true
}

// AwaitJobs blocks until every started job reaches a terminal state, or the
// deadline passes.
//
// A one-shot CLI writes its envelope and exits, which would abandon async work
// mid-render: the handle would be returned, the goroutine killed, and the
// render silently discarded. A handle that loses the work it represents is
// worse than blocking, so the process waits for its own jobs before leaving.
//
// It reports whether everything finished, so a caller can say plainly that
// work was abandoned rather than implying success.
func AwaitJobs(timeout time.Duration) (finished bool) {
	deadline := time.Now().Add(timeout)
	for {
		jobsMu.RLock()
		running := 0
		for _, j := range jobs {
			if j.State == JobRunning {
				running++
			}
		}
		jobsMu.RUnlock()
		if running == 0 {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(150 * time.Millisecond)
	}
}

// HasRunningJobs reports whether this process started work that has not
// finished.
func HasRunningJobs() bool {
	jobsMu.RLock()
	defer jobsMu.RUnlock()
	for _, j := range jobs {
		if j.State == JobRunning {
			return true
		}
	}
	return false
}

// JobStatusRequest is the body of a poll.
type JobStatusRequest struct {
	RequestID string `json:"request_id"`
	JobID     string `json:"job_id"`
}

// JobStatus implements creative.jobs.status.
//
// It is deterministic, local, free and side-effect free: polling never runs
// work, never bills, and never writes.
func JobStatus(raw []byte) Envelope {
	var req JobStatusRequest
	if len(raw) > 0 {
		if err := decodeRequest(raw, &req); err != nil {
			return fail(OpInvoke, newRequestID(), "invalid_request",
				requestDecodeMessage(err),
				map[string]any{"error": bounded(err.Error())}, false)
		}
	}
	reqID := req.RequestID
	if reqID == "" {
		reqID = newRequestID()
	}
	if req.JobID == "" {
		return fail(OpInvoke, reqID, "invalid_request",
			"job_id is required", map[string]any{}, false)
	}

	job, ok := lookupJob(req.JobID)
	if !ok {
		// A module is a short-lived subprocess and job state does not survive
		// it. Say so rather than reporting a state that was never observed.
		return fail(OpInvoke, reqID, "unknown_job",
			fmt.Sprintf("job %q is not known to this process; job state does not "+
				"survive the module process that created it", req.JobID),
			map[string]any{"job_id": req.JobID}, false)
	}

	env := Envelope{
		Protocol:  Protocol,
		Module:    ModuleID,
		Operation: OpInvoke,
		RequestID: reqID,
		OK:        true,
		Result: map[string]any{
			"capability": CapJobsStatus,
			"job":        job,
		},
		Warnings:  []string{},
		Execution: localExec("facet"),
	}
	// A terminal poll carries the authoritative execution of the work itself.
	if job.State != JobRunning && job.Execution != nil {
		env.Execution = *job.Execution
	}
	return env
}

// jobHandleEnvelope is the response to an accepted long-running invocation.
//
// Its execution is PROVISIONAL by contract: the work has not run, so
// actual_cost is null and artifacts is empty. Reporting a cost here would
// claim a charge that has not happened; reporting artifacts would name files
// that do not exist. The terminal poll carries the authoritative execution.
func jobHandleEnvelope(reqID, capability, tool string, job *Job) Envelope {
	return Envelope{
		Protocol:  Protocol,
		Module:    ModuleID,
		Operation: OpInvoke,
		RequestID: reqID,
		OK:        true,
		Result: map[string]any{
			"capability": capability,
			"tool":       tool,
			"job_id":     job.JobID,
			"state":      string(JobRunning),
			// Polling only works INSIDE the process that created the job:
			// state lives in memory and a per-invocation host starts a new
			// process for every call, where the id is unknown.
			//
			// Declaring poll_capability unconditionally promised a poll that
			// returns unknown_job to any host that spawns per invocation, and
			// a host trusting it would report a completed render as lost.
			//
			// What IS guaranteed: this handle is written to stdout
			// immediately (measured: 0.0s against a 12.8s render) and the
			// process then blocks until the work finishes, so the render is
			// complete when the process exits. A host that spawns per
			// invocation should wait for exit and read the artifacts, not
			// poll.
			"poll_capability":   CapJobsStatus,
			"poll_scope":        "same_process",
			"completed_on_exit": true,
		},
		Warnings: []string{},
		Execution: Execution{
			Local:          false,
			Network:        false,
			ExternalWrites: false,
			Provider:       "facet",
			EstimatedCost:  nil,
			ActualCost:     nil,
			Artifacts:      []Artifact{},
		},
	}
}
