package module

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Seed.OutputTypes was written, tested, and called by NOTHING — the ninth
// instance of the declared-but-never-wired class, and the second in this seed
// path alone after SeedRef itself.
//
// A producer naming "explainer-video" instead of "explainer" got silence: the
// run proceeded exactly as if the seed had suggested nothing. The check
// existed, was correct, and was unreachable.
//
// Warning rather than refusal is deliberate. Midden's vocabulary is its own and
// may legitimately grow past Facet's, so an unrecognised type is reported to
// the caller and ignored, never fatal.
func TestSeedOutputTypeWarningReachesTheCaller(t *testing.T) {
	dir := t.TempDir()
	seedPath := filepath.Join(dir, "seed.json")

	seed := map[string]any{
		"schema": SeedSchemaID,
		"goal":   "explain the thing",
		// One Facet knows, one it does not.
		"suggested_output_types": []string{"explainer", "explainer-video"},
	}
	raw, err := json.Marshal(seed)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(seedPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	req := map[string]any{
		"tool":  "media_probe",
		"input": map[string]any{"input": "does-not-matter.mp4"},
		"seed":  map[string]any{"path": seedPath, "schema": SeedSchemaID},
	}
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	env := Invoke(CapToolsRun, body)

	// The probe itself may fail (no such file); the seed warning must be
	// present either way, because it describes the SEED and not the tool.
	var found string
	for _, w := range env.Warnings {
		if strings.Contains(w, "explainer-video") {
			found = w
		}
	}
	if found == "" {
		t.Errorf("a seed suggesting an unrecognised output type produced no warning; "+
			"warnings were %v", env.Warnings)
	}
	// The recognised one must NOT be warned about, or the check is just noise.
	for _, w := range env.Warnings {
		if strings.Contains(w, "explainer ") || strings.HasSuffix(w, "explainer") {
			t.Errorf("a RECOGNISED output type was warned about: %q", w)
		}
	}
}

// The vocabulary check must accept what Facet really installs. A vocabulary
// that drifted from the packs would reject every valid suggestion.
func TestKnownOutputTypesAreNonEmptyAndLowercase(t *testing.T) {
	if len(knownOutputTypes) == 0 {
		t.Fatal("Facet recognises no output types at all")
	}
	for k := range knownOutputTypes {
		if k != strings.ToLower(strings.TrimSpace(k)) {
			t.Errorf("output type %q is not normalised; lookups lowercase the input", k)
		}
	}
}
