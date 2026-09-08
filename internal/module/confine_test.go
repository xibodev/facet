package module

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A granted root is a CONFINEMENT, not merely a base for resolution.
//
// Verified before this fix: output_path "../escaped.mp4" with a project_root
// grant wrote a real 40KB file outside the granted root and reported it back
// as the relative path "../escaped.mp4" — which the host validator ACCEPTS,
// because its rule is "not absolute". Resolving against a root without
// enforcing it turns the grant into a suggestion.
func TestRequestPathsCannotLeaveTheGrantedRoot(t *testing.T) {
	escaping := []struct {
		name string
		body string
	}{
		{"a leading parent segment", `{"output_path":"../escaped.mp4"}`},
		{"a parent segment deeper in", `{"output_path":"renders/../../deep.mp4"}`},
		{"a nested value", `{"spec":{"clips":[{"path":"../../x.mp4"}]}}`},
		{"a value inside an array", `{"segments":["ok.mp4","../../evil.mp4"]}`},
	}
	for _, c := range escaping {
		t.Run(c.name, func(t *testing.T) {
			bad, ok := requestEscapesRoot(json.RawMessage(c.body))
			if !ok {
				t.Fatalf("an escaping path was allowed: %s", c.body)
			}
			if !strings.Contains(bad, "..") {
				t.Errorf("the reported path %q is not the offending one", bad)
			}
		})
	}

	confined := []string{
		`{"output_path":"renders/final.mp4"}`,
		`{"input_path":"media/src.mp4","output_path":"out.mp4"}`,
		// A parent segment that stays inside is legitimate.
		`{"output_path":"a/b/../c.mp4"}`,
		// Absolute paths are the host validator's concern, not this check's.
		`{"output_path":"C:/tmp/x.mp4"}`,
		`{"output_path":"/tmp/x.mp4"}`,
		// Non-path values must not be mistaken for paths.
		`{"operation":"cut","start_seconds":0}`,
	}
	for _, body := range confined {
		if bad, ok := requestEscapesRoot(json.RawMessage(body)); ok {
			t.Errorf("a confined request was refused: %s (blamed %q)", body, bad)
		}
	}
}

// The check runs BEFORE the tool, because after the write the file already
// exists outside the root and no envelope can undo it.
func TestEscapingRequestIsRefusedBeforeAnyWrite(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "escaped-by-test.mp4")
	_ = os.Remove(outside)

	rootJSON, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{
	  "tool":"video_trimmer",
	  "input":{"operation":"cut","input_path":"src.mp4",
	           "output_path":"../escaped-by-test.mp4","start_seconds":0,"end_seconds":1},
	  "roots":{"project_root":{"path":` + string(rootJSON) + `,"mode":"rw"}}
	}`)

	env := Invoke(CapToolsRun, body)
	if env.OK {
		t.Fatal("a request writing outside the granted root succeeded")
	}
	if env.Error.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", env.Error.Code)
	}
	if _, err := os.Stat(outside); err == nil {
		t.Error("the file was written outside the root despite the refusal")
	}
}
