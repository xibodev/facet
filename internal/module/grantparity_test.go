package module

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// estimate and run must agree about grants.
//
// Estimate is a separate entry point and had NONE of the grant handling:
// it never entered the granted project_root, never installed binary grants,
// and never honoured the bundle root. An identical request succeeded through
// run and failed through estimate:
//
//	creative.tools.run      -> ok
//	creative.tools.estimate -> input_not_found
//
// The skill instructs an agent to estimate before anything consequential, so
// the check meant to prevent a wasted call was the one that could not resolve
// the caller's paths. Found by sweeping the whole tool surface rather than the
// handful of tools I had been re-testing — 15 of 22 tools failed to estimate,
// and it was one bug, not fifteen.
func TestEstimateAndRunAgreeOnGrants(t *testing.T) {
	root := t.TempDir()
	media := filepath.Join(root, "clip.txt")
	if err := os.WriteFile(media, []byte("not really media"), 0600); err != nil {
		t.Fatal(err)
	}
	rootJSON, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}

	// A path that only resolves INSIDE the granted root.
	body := []byte(`{
	  "tool":"media_probe",
	  "input":{"input":"clip.txt"},
	  "roots":{"project_root":{"path":` + string(rootJSON) + `,"mode":"rw"}}
	}`)

	est := Estimate(CapToolsEstimate, body)
	run := Invoke(CapToolsRun, body)

	// The file is not real media, so both should fail for the SAME reason —
	// what must never happen is one reporting the path missing while the
	// other finds it.
	estMissing := est.Error != nil && est.Error.Code == "input_not_found"
	runMissing := run.Error != nil && run.Error.Code == "input_not_found"
	if estMissing != runMissing {
		t.Errorf("estimate and run disagree about whether the input exists: "+
			"estimate=%v run=%v; the granted root is not applied on both paths",
			codeOf(est), codeOf(run))
	}
	if estMissing {
		t.Errorf("estimate could not resolve a path inside the granted root: %s",
			est.Error.Message)
	}
}

// A path escaping the granted root must be refused on BOTH paths. An estimate
// that accepts what run refuses would tell an agent a request is fine moments
// before it is rejected.
func TestEstimateRefusesEscapingPathsToo(t *testing.T) {
	root := t.TempDir()
	rootJSON, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{
	  "tool":"media_probe",
	  "input":{"input":"../../escaped.mp4"},
	  "roots":{"project_root":{"path":` + string(rootJSON) + `,"mode":"rw"}}
	}`)

	if env := Estimate(CapToolsEstimate, body); env.OK {
		t.Error("estimate accepted a path leaving the granted root")
	}
	if env := Invoke(CapToolsRun, body); env.OK {
		t.Error("run accepted a path leaving the granted root")
	}
}

func codeOf(env Envelope) string {
	if env.Error == nil {
		return "ok"
	}
	return env.Error.Code
}
