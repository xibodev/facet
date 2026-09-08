package module

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The host enforces deadline_ms by killing the process tree, and its default is
// 60s while a render's own timeout is 600s. Verified before this fix: a 30s
// 1080p render exceeded 60s, the process was killed, and the work was lost with
// NO envelope — the host saw a dead process rather than a failure it could
// report or explain.
//
// Clamping means Facet returns a real command_timeout inside the budget. The
// same render now answers at 55s instead of being killed at 60s.
func TestDeadlineClamping(t *testing.T) {
	timeoutOf := func(raw json.RawMessage) (float64, bool) {
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			return 0, false
		}
		v, ok := body["timeout_seconds"].(float64)
		return v, ok
	}

	t.Run("a long tool timeout is clamped under the host budget", func(t *testing.T) {
		out := applyDeadline(json.RawMessage(`{"timeout_seconds":600}`), 60000)
		got, ok := timeoutOf(out)
		if !ok {
			t.Fatal("timeout_seconds disappeared")
		}
		if got >= 60 {
			t.Errorf("timeout %v is not inside the 60s budget", got)
		}
		// A margin is reserved so the envelope can still be written after the
		// tool gives up; returning exactly at the deadline is still a kill.
		if got > 55 {
			t.Errorf("timeout %v leaves no margin to report the failure", got)
		}
	})

	t.Run("a tool with no timeout gets one from the budget", func(t *testing.T) {
		out := applyDeadline(json.RawMessage(`{"width":1280}`), 60000)
		got, ok := timeoutOf(out)
		if !ok {
			t.Fatal("no timeout was applied, so the tool can outlive the budget")
		}
		if got <= 0 || got >= 60 {
			t.Errorf("timeout %v is not a sensible value inside 60s", got)
		}
	})

	t.Run("a shorter caller timeout is never extended", func(t *testing.T) {
		// Asking for 30s must mean 30s even when the host allows 600.
		out := applyDeadline(json.RawMessage(`{"timeout_seconds":30}`), 600000)
		got, _ := timeoutOf(out)
		if got != 30 {
			t.Errorf("timeout became %v; a caller's shorter budget must be honoured", got)
		}
	})

	t.Run("no deadline leaves the request untouched", func(t *testing.T) {
		in := json.RawMessage(`{"timeout_seconds":600}`)
		if got := string(applyDeadline(in, 0)); got != string(in) {
			t.Errorf("request altered without a host deadline: %s", got)
		}
	})

	t.Run("a budget too small to reserve a margin is left alone", func(t *testing.T) {
		// Fabricating a zero or negative timeout would be worse than leaving
		// the tool's own value: the host will kill it either way, and at least
		// the request still says what the caller asked for.
		in := json.RawMessage(`{"timeout_seconds":600}`)
		if got := string(applyDeadline(in, 1000)); got != string(in) {
			t.Errorf("a sub-margin budget rewrote the request: %s", got)
		}
	})

	t.Run("a non-object body is passed through", func(t *testing.T) {
		in := json.RawMessage(`"not an object"`)
		if got := string(applyDeadline(in, 60000)); got != string(in) {
			t.Errorf("a non-object body was rewritten: %s", got)
		}
	})

	t.Run("an empty body is passed through", func(t *testing.T) {
		if got := applyDeadline(nil, 60000); got != nil {
			t.Errorf("an empty body became %s", got)
		}
	})
}

// project_root was declared, used to label artifacts, and never resolved
// against. The host sets the working directory to the module's install
// directory, so a caller's "source.mp4" named nothing findable and every
// relative path failed input_not_found — the seventh instance of a field
// accepted and ignored.
//
// A caller's relative path means relative to their project, not to wherever
// the host launched the module.
func TestGrantedProjectRootIsEnteredAndRestored(t *testing.T) {
	before, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()

	restore, err := useWorkingRoot(target)
	if err != nil {
		t.Fatalf("a granted root could not be entered: %v", err)
	}
	during, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Compare resolved paths: a temp dir may be reached through a symlink.
	wantDir, _ := filepath.EvalSymlinks(target)
	gotDir, _ := filepath.EvalSymlinks(during)
	if gotDir != wantDir {
		t.Errorf("working directory is %q, want the granted root %q", gotDir, wantDir)
	}

	restore()
	after, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Errorf("working directory not restored: %q, want %q", after, before)
	}
}

// A root that cannot be entered is a request error, not a panic or a silent
// continue in the wrong directory.
func TestUnenterableRootIsRefused(t *testing.T) {
	if _, err := useWorkingRoot(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Error("a nonexistent root was entered")
	}
}

// The working directory is process-global, and an async job outlives the call
// that started it: `restore` runs when Invoke returns, before the job finishes.
// A job would resolve the caller's relative paths wherever the process happened
// to be, and a concurrent request entering its own root would move a running
// render mid-flight.
//
// Refusing beats a race that silently reads the wrong files.
func TestAsyncWithProjectRootIsRefused(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := json.Marshal(cwd)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{
	  "tool":"video_compose",
	  "input":{"spec":{}},
	  "async":true,
	  "roots":{"project_root":{"path":` + string(root) + `,"mode":"rw"}}
	}`)

	env := Invoke(CapToolsRun, body)
	if env.OK {
		t.Fatal("async work was accepted with a project_root it cannot honour")
	}
	if env.Error.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", env.Error.Code)
	}
	if !strings.Contains(env.Error.Message, "project_root") {
		t.Errorf("the refusal does not say what is wrong: %q", env.Error.Message)
	}

	// The working directory must not be left moved by a refused request.
	after, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if after != cwd {
		t.Errorf("a refused request left the process in %q, want %q", after, cwd)
	}
}
