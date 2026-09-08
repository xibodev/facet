package module

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

var hexDigest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// The host REJECTS an artifact missing an id, a root, a relative path or a
// valid digest — pkg/modproto/validate.go:133. Facet emitted none of them, so
// every artifact-producing run would have been refused while the rest of the
// envelope was correct.
//
// This asserts the host's rules directly rather than Facet's opinion of them.
func TestArtifactsSatisfyHostValidation(t *testing.T) {
	env := Invoke(CapToolsRun, []byte(`{
	  "tool":"frame_sample",
	  "input":{"input":"../../projects/cinematic-documentary/assets/video/shot1_raw.mp4",
	           "output_dir":"../../.quality-run/arttest","strategy":{"type":"uniform","count":2},
	           "overwrite":true}
	}`))
	if !env.OK {
		t.Skipf("frame extraction unavailable here: %+v", env.Error)
	}
	if len(env.Execution.Artifacts) == 0 {
		t.Fatal("a tool that wrote files reported no artifacts")
	}

	const backslash = `\`
	for i, a := range env.Execution.Artifacts {
		if a.ID == "" {
			t.Errorf("artifact[%d] has no id; the host rejects it", i)
		}
		if a.Root == "" {
			t.Errorf("artifact[%d] names no root; the host cannot confine it", i)
		}
		if a.Path == "" {
			t.Errorf("artifact[%d] has no path", i)
		}
		if isAbsolutePath(a.Path) {
			t.Errorf("artifact[%d] path %q is absolute", i, a.Path)
		}
		// A backslash path cannot be checked against a root declared with
		// forward slashes: confinement becomes uncheckable, not merely ugly.
		if strings.Contains(a.Path, backslash) {
			t.Errorf("artifact[%d] path %q uses backslashes", i, a.Path)
		}
		if !hexDigest.MatchString(a.Digest) {
			t.Errorf("artifact[%d] digest %q is not sha256 hex", i, a.Digest)
		}
		if a.Bytes <= 0 {
			t.Errorf("artifact[%d] reports %d bytes for a file that exists", i, a.Bytes)
		}
	}
}

// A digest must describe the bytes actually on disk. A digest the module did
// not compute from the file is a claim the host would record as provenance.
func TestArtifactDigestMatchesTheFile(t *testing.T) {
	env := Invoke(CapToolsRun, []byte(`{
	  "tool":"frame_sample",
	  "input":{"input":"../../projects/cinematic-documentary/assets/video/shot1_raw.mp4",
	           "output_dir":"../../.quality-run/arttest2","strategy":{"type":"uniform","count":1},
	           "overwrite":true}
	}`))
	if !env.OK {
		t.Skipf("frame extraction unavailable: %+v", env.Error)
	}
	for _, a := range env.Execution.Artifacts {
		if want := fileDigestOf(a.Path); want != "" && a.Digest != want {
			t.Errorf("artifact %s reports %q but the file hashes to %q", a.Path, a.Digest, want)
		}
	}
}

// A run that writes nothing must report no artifacts rather than inventing one.
func TestReadOnlyToolReportsNoArtifacts(t *testing.T) {
	env := Invoke(CapToolsRun, []byte(`{
	  "tool":"media_probe",
	  "input":{"input":"../../projects/cinematic-documentary/assets/video/shot1_raw.mp4"}
	}`))
	if !env.OK {
		t.Skipf("probe unavailable: %+v", env.Error)
	}
	if len(env.Execution.Artifacts) != 0 {
		raw, _ := json.Marshal(env.Execution.Artifacts)
		t.Errorf("a read-only tool reported artifacts: %s", raw)
	}
}
