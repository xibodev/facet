package module

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// Async work must honour a granted project_root.
//
// The first attempt REFUSED the combination, because the working directory is
// process-global and restored when Invoke returns — before a job finishes.
// That was safe but it disabled the only reason async exists: an 83s 1080p
// render leaves the cockpit with nothing to show unless it gets a handle
// immediately.
//
// Resolving the caller's paths against the root while that root is still
// current removes the dependency instead of racing on it. The job then runs on
// absolute paths and needs no working directory at all.
func TestAsyncPathsAreResolvedAgainstTheRoot(t *testing.T) {
	root := filepath.ToSlash(t.TempDir())

	in := json.RawMessage(`{"operation":"remotion_render","output":"renders/out.mp4",
	                        "audio_path":"audio/vo.mp3","width":640}`)
	got := absolutizeRequestPaths(in, filepath.FromSlash(root))

	var body map[string]any
	if err := json.Unmarshal(got, &body); err != nil {
		t.Fatalf("rewritten request does not decode: %v", err)
	}

	for _, field := range []string{"output", "audio_path"} {
		v, _ := body[field].(string)
		if !isAbsolutePath(v) {
			t.Errorf("%s is %q; a job cannot resolve that without a working directory", field, v)
		}
		if !strings.HasPrefix(filepath.ToSlash(v), root) {
			t.Errorf("%s resolved to %q, outside the granted root %q", field, v, root)
		}
	}

	// Non-path fields must be untouched: rewriting a width or a caption would
	// corrupt the request.
	if body["width"] != float64(640) {
		t.Errorf("a non-path field was altered: width = %v", body["width"])
	}
	if body["operation"] != "remotion_render" {
		t.Errorf("a non-path field was altered: operation = %v", body["operation"])
	}
}

// An absolute path the caller supplied is already resolved and must not be
// re-rooted, which would produce a nonsense path under the project.
func TestAbsolutePathsAreNotRerooted(t *testing.T) {
	root := t.TempDir()
	in := json.RawMessage(`{"output":"C:/tmp/keep.mp4"}`)
	var body map[string]any
	if err := json.Unmarshal(absolutizeRequestPaths(in, root), &body); err != nil {
		t.Fatal(err)
	}
	if got, _ := body["output"].(string); got != "C:/tmp/keep.mp4" {
		t.Errorf("an absolute path was rewritten to %q", got)
	}
}

// Prose must survive: the same key-based rule that stops the confinement check
// refusing captions stops the absolutizer mangling them into paths.
func TestProseIsNotTurnedIntoAPath(t *testing.T) {
	root := t.TempDir()
	in := json.RawMessage(`{"scenes":[{"elements":[{"type":"text","text":"see ../docs"}]}]}`)
	out := absolutizeRequestPaths(in, root)
	if strings.Contains(string(out), filepath.ToSlash(root)) {
		t.Errorf("caption text was rewritten as a path: %s", out)
	}
}
