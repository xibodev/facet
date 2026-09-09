package module

import (
	"encoding/json"
	"testing"
)

// THE WIRE NAMES ARE THE CONTRACT, and Facet shipped five that diverged from
// the frozen shape while every local test passed.
//
// Measured by facet-studio against the real bundle: their host decoded 35
// operations, 0 requirements, 0 external writes and 0 projections, then
// reported ZERO findings — because the check had nothing to compare. A check
// with no input and a check that passed produce the same observable, which is
// the vacuous-green this repo named for the zero-Operations state and then
// reached WITH 35 operations present.
//
// What diverged, and why a Go test could not see it: every one of these is a
// json TAG. The Go field names were right, the values were right, and the
// serialized names were wrong. Only a comparison against the frozen spelling
// catches that.
//
//	frozen              shipped            consequence
//	------------------  -----------------  ---------------------------------
//	requirements        requires           host read zero requirements
//	kind (on a req)     type               host read no requirement kind
//	mandatory           required           not in the frozen vocabulary
//	external_writes     external_write     FAILED OPEN: an ungated real write
//	projects            (absent)           no-weakening compared nothing
//
// external_writes is the one that spends something unrecoverable: an external
// write cannot be un-done by an error, so a host that does not gate it has
// already let the effect happen.
func TestWireNamesMatchTheFrozenShape(t *testing.T) {
	env := Describe("test")
	if !env.OK {
		t.Fatalf("describe failed: %+v", env.Error)
	}
	raw, err := json.Marshal(env.Result)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}

	// --- Operations ---
	var ops []map[string]json.RawMessage
	if err := json.Unmarshal(doc["operations"], &ops); err != nil {
		t.Fatal(err)
	}
	if len(ops) == 0 {
		t.Fatal("no operations to check")
	}

	var sawRequirements bool
	for _, op := range ops {
		if _, wrong := op["requires"]; wrong {
			t.Error(`operation uses "requires"; the frozen shape says "requirements"`)
		}
		rawReqs, ok := op["requirements"]
		if !ok {
			continue
		}
		var reqs []map[string]json.RawMessage
		if err := json.Unmarshal(rawReqs, &reqs); err != nil {
			t.Fatal(err)
		}
		for _, r := range reqs {
			sawRequirements = true
			if _, wrong := r["type"]; wrong {
				t.Error(`requirement uses "type"; the frozen shape says "kind"`)
			}
			if _, ok := r["kind"]; !ok {
				t.Error(`requirement has no "kind"`)
			}
			var strength string
			if err := json.Unmarshal(r["strength"], &strength); err != nil {
				t.Fatal(err)
			}
			// The frozen vocabulary is mandatory|preferred. "required" reads
			// correct in English and is not in the contract.
			if strength != "mandatory" && strength != "preferred" {
				t.Errorf("requirement strength %q is not in the frozen vocabulary (mandatory|preferred)", strength)
			}
		}

		var eff map[string]json.RawMessage
		if err := json.Unmarshal(op["effects"], &eff); err != nil {
			t.Fatal(err)
		}
		if _, wrong := eff["external_write"]; wrong {
			t.Error(`effects use "external_write"; the frozen shape says "external_writes". ` +
				`A host with no such field decodes it as FALSE and does not gate a real write`)
		}
		if _, ok := eff["external_writes"]; !ok {
			t.Error(`effects have no "external_writes"`)
		}
	}
	if !sawRequirements {
		t.Error("no operation declared any requirement; the field would be untested")
	}

	// --- Capabilities ---
	var caps []map[string]json.RawMessage
	if err := json.Unmarshal(doc["capabilities"], &caps); err != nil {
		t.Fatal(err)
	}
	var totalProjections int
	for _, c := range caps {
		rawProj, ok := c["projects"]
		if !ok {
			t.Error(`capability has no "projects"; without it the host's no-weakening ` +
				`check iterates nothing and a weakened capability passes identically`)
			continue
		}
		var proj []string
		if err := json.Unmarshal(rawProj, &proj); err != nil {
			t.Fatal(err)
		}
		totalProjections += len(proj)
	}
	// Empty is legal per-capability — a registry read projects nothing — but
	// if NOTHING projects anywhere, no comparison is ever performed and the
	// host's silence is not conformance.
	if totalProjections == 0 {
		t.Error("no capability projects any Operation; the no-weakening check would compare zero pairs")
	}
}

// A capability must not declare effects weaker than the Operations it projects.
//
// This is the host's rule, asserted locally so a weakening fails here first.
// It became reachable only once `projects` was published: before that the host
// compared zero pairs and Facet's honest-but-unlinked declaration was
// indistinguishable from a dishonest one.
func TestProjectingCapabilityCoversItsOperations(t *testing.T) {
	env := Describe("test")
	if !env.OK {
		t.Fatalf("describe failed: %+v", env.Error)
	}
	raw, err := json.Marshal(env.Result)
	if err != nil {
		t.Fatal(err)
	}
	var d struct {
		Operations []struct {
			ID      string `json:"id"`
			Effects struct {
				Network        bool `json:"network"`
				ExternalWrites bool `json:"external_writes"`
				MayCharge      bool `json:"may_charge"`
			} `json:"effects"`
		} `json:"operations"`
		Capabilities []struct {
			ID       string   `json:"id"`
			Projects []string `json:"projects"`
			Effects  struct {
				Network        bool `json:"network"`
				ExternalWrites bool `json:"external_writes"`
				MayCharge      bool `json:"may_charge"`
			} `json:"effects"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}

	byID := map[string]int{}
	for i, op := range d.Operations {
		byID[op.ID] = i
	}

	var compared int
	for _, c := range d.Capabilities {
		for _, name := range c.Projects {
			i, ok := byID[name]
			if !ok {
				t.Errorf("capability %s projects %q, which is not a declared Operation", c.ID, name)
				continue
			}
			op := d.Operations[i]
			compared++
			if op.Effects.Network && !c.Effects.Network {
				t.Errorf("capability %s projects %s which networks, but declares network=false", c.ID, name)
			}
			if op.Effects.ExternalWrites && !c.Effects.ExternalWrites {
				t.Errorf("capability %s projects %s which writes externally, but declares external_writes=false", c.ID, name)
			}
			if op.Effects.MayCharge && !c.Effects.MayCharge {
				t.Errorf("capability %s projects %s which may charge, but declares may_charge=false", c.ID, name)
			}
		}
	}
	// Zero comparisons is the failure this whole test exists for.
	if compared == 0 {
		t.Error("no capability/Operation pair was compared; the check passed because it had no input")
	}
}
