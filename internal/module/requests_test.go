package module

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
			if strings.TrimSpace(req.Tool) == "" {
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
