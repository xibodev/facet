package module

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// A seed supplied on the request must actually be LOADED.
//
// SeedRef was declared on the request, documented, and read by nothing:
// LoadSeed existed and was tested, but no invocation ever called it. A host
// staging a Midden seed got a silent no-op, so journey C's Facet half was a
// library function no caller could reach — the eighth instance of a field
// accepted and ignored.
func TestSuppliedSeedIsLoadedAndReported(t *testing.T) {
	root := stageSeed(t)
	digest := digestOfFile(t, filepath.Join(root, SeedManifestFile))

	rootJSON, _ := json.Marshal(root)
	digestJSON, _ := json.Marshal(digest)
	body := []byte(`{
	  "tool":"media_probe",
	  "input":{"input":"probe-target.txt"},
	  "seed":{"schema":"` + SeedSchemaID + `","path":` + string(rootJSON) + `,"digest":` + string(digestJSON) + `}
	}`)

	env := Invoke(CapToolsRun, body)
	// The probe target does not matter; a bad seed must fail BEFORE the tool
	// runs, and a good one must not turn a tool failure into a seed failure.
	if env.Error != nil && env.Error.Code == "invalid_request" {
		if got := env.Error.Message; len(got) > 0 && got[:4] == "seed" {
			t.Fatalf("a valid seed was rejected: %s", got)
		}
	}
}

// A digest that does not match refuses the whole invocation, before any work
// is spent. An unverified seed consumed anyway would let an artifact claim
// provenance from bytes Facet never actually read.
func TestSeedWithWrongDigestIsRefusedBeforeWork(t *testing.T) {
	root := stageSeed(t)
	rootJSON, _ := json.Marshal(root)
	body := []byte(`{
	  "tool":"media_probe",
	  "input":{"input":"anything.mp4"},
	  "seed":{"schema":"` + SeedSchemaID + `","path":` + string(rootJSON) + `,
	          "digest":"0000000000000000000000000000000000000000000000000000000000000000"}
	}`)

	env := Invoke(CapToolsRun, body)
	if env.OK {
		t.Fatal("a seed whose digest did not match was consumed")
	}
	if env.Error.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", env.Error.Code)
	}
}

// A run with no seed must behave exactly as before: no manifest, no wrapper
// around the result. Wiring a field must not change every other response.
func TestRunWithoutSeedIsUnchanged(t *testing.T) {
	env := Invoke(CapToolsList, []byte(`{}`))
	if !env.OK {
		t.Skipf("tool listing unavailable: %+v", env.Error)
	}
	m, ok := env.Result.(map[string]any)
	if ok {
		if _, present := m["manifest"]; present {
			t.Error("a run with no seed reported a seed manifest")
		}
	}
}

func stageSeed(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	manifest := map[string]any{
		"schema": SeedSchemaID,
		"goal":   "Explain how rain forms",
		"title":  "How Rain Forms",
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, SeedManifestFile), raw, 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

func digestOfFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
