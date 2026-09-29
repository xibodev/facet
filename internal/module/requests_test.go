package module

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShippedModuleExamplesDoNotPromiseProcessLocalLifecycle(t *testing.T) {
	root := "../../fixtures/module"
	banned := [][]byte{
		[]byte(`"async"`),
		[]byte(`"job_id"`),
		[]byte(`"long_running"`),
		[]byte(`"poll_capability"`),
		[]byte("creative.jobs.status"),
		[]byte("job handle"),
		[]byte("polling"),
		[]byte("unknown_job"),
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == "legacy" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".json" && filepath.Ext(path) != ".md" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := bytes.ToLower(raw)
		for _, promise := range banned {
			if bytes.Contains(lower, bytes.ToLower(promise)) {
				t.Errorf("shipped host example %s advertises process-local lifecycle %q",
					filepath.ToSlash(path), promise)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(root, "requests", "list-synchronous.json"))
	if err != nil {
		t.Fatal(err)
	}
	before := runningJobCountForTest()
	env := Invoke(CapToolsList, raw)
	if !env.OK {
		t.Fatalf("synchronous fixture failed: %+v", env.Error)
	}
	result := env.Result.(map[string]any)
	for _, key := range []string{"job_id", "state", "poll_capability"} {
		if _, exists := result[key]; exists {
			t.Errorf("synchronous fixture returned lifecycle field %q", key)
		}
	}
	if after := runningJobCountForTest(); after != before {
		t.Errorf("synchronous fixture changed running jobs from %d to %d", before, after)
	}
}

func runningJobCountForTest() int {
	jobsMu.RLock()
	defer jobsMu.RUnlock()
	running := 0
	for _, job := range jobs {
		if job.State == JobRunning {
			running++
		}
	}
	return running
}

func TestLegacyUnknownJobFixtureIsExplicitlyCompatibilityOnly(t *testing.T) {
	root := "../../fixtures/module/legacy"
	note, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	normalizedNote := strings.Join(strings.Fields(strings.ToLower(string(note))), " ")
	if !strings.Contains(normalizedNote, "not host guidance") {
		t.Fatal("legacy lifecycle fixture is not clearly excluded from host guidance")
	}

	raw, err := os.ReadFile(filepath.Join(root, "unknown-job-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	env := JobStatus(raw)
	if env.OK || env.Error == nil || env.Error.Code != "unknown_job" {
		t.Fatalf("legacy input did not preserve unknown_job: %+v", env)
	}

	want, err := os.ReadFile(filepath.Join(root, "error-unknown-job.json"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, bytes.TrimSpace(want)) {
		t.Errorf("legacy unknown_job fixture drifted\n got: %s\nwant: %s", got, want)
	}
}

// The checked-in request examples must be exactly what the module accepts.
//
// The host built requests by reading the schema and guessing, and guessed
// wrong. An example the module actually accepts removes the guess — but only
// if the example is verified rather than hand-written and hoped for.
func TestRequestFixturesAreAccepted(t *testing.T) {
	dir := "../../fixtures/module/requests"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("request fixtures unavailable: %v", err)
	}

	// Capability each example is written for, and whether it should run here.
	// Paid and binary-dependent examples are shape-checked only: running them
	// would contact a provider or need a real binary path.
	shapeOnly := map[string]bool{
		"estimate-paid.json":           true,
		"run-paid-with-consent.json":   true,
		"run-local-deterministic.json": true,
		"run-from-seed.json":           true,
	}
	capability := map[string]string{
		"describe-tool.json":           CapToolsDescribe,
		"list-synchronous.json":        CapToolsList,
		"run-local-deterministic.json": CapToolsRun,
		"estimate-paid.json":           CapToolsEstimate,
		"run-paid-with-consent.json":   CapToolsRun,
		"run-from-seed.json":           CapToolsRun,
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}

			var req Request
			if err := json.Unmarshal(raw, &req); err != nil {
				t.Fatalf("example does not parse as a Request: %v", err)
			}
			if capability[name] != CapToolsList && strings.TrimSpace(req.Tool) == "" {
				t.Error("example carries no `tool` at the request root")
			}
			// The mistake this fixture set exists to prevent.
			if len(req.Input) > 0 {
				var probe map[string]any
				if err := json.Unmarshal(req.Input, &probe); err == nil {
					if _, nested := probe["tool"]; nested {
						t.Error("example nests `tool` inside `input`; it is a sibling")
					}
				}
			}
			// Host-supplied binary paths must satisfy the authority rules.
			if err := ValidateBinaries(req.Binaries); err != nil {
				t.Errorf("example binary map is invalid: %v", err)
			}

			if shapeOnly[name] {
				return
			}
			env := Invoke(capability[name], raw)
			if !env.OK {
				t.Errorf("example was REJECTED by the module: %+v", env.Error)
			}
		})
	}
}
