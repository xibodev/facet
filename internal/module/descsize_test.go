package module

import (
	"encoding/json"
	"testing"
)

// The descriptor is paid by every agent at session start, before it knows
// whether it will call anything. It was 107KB, of which 67KB was artifact
// schema bodies and 27KB per-tool schemas — and the host confirmed it reads
// neither: the cockpit renders artifacts from a host-resolved primitive, and
// per-tool schemas are served by creative.tools.describe for ~1.7KB on demand.
//
// This bounds the descriptor so re-inlining bulk is a failing test rather than
// a cost nobody notices. The bound is deliberately far below the host's 1 MiB
// cap: fitting the cap is not the goal, not wasting an agent's context is.
func TestDescriptorStaysSmall(t *testing.T) {
	raw, err := json.Marshal(Describe("test"))
	if err != nil {
		t.Fatal(err)
	}
	const bound = 32 * 1024
	t.Logf("descriptor is %d bytes", len(raw))
	if len(raw) > bound {
		t.Errorf("descriptor is %d bytes, past the %d-byte bound; "+
			"large content belongs behind a capability, not in every session's context",
			len(raw), bound)
	}
}

// Dropping the schema BODIES must not break the references into them, and must
// not make a schema unverifiable: an ID alone proves a reference resolves but
// says nothing about the document a host would then read.
func TestArtifactSchemaReferencesStayResolvableAndVerifiable(t *testing.T) {
	env := Describe("test")
	if !env.OK {
		t.Fatalf("describe failed: %+v", env.Error)
	}
	desc, ok := env.Result.(Descriptor)
	if !ok {
		t.Fatalf("result is %T, not a Descriptor", env.Result)
	}

	for id, ref := range desc.ArtifactSchemas {
		m, ok := ref.(map[string]any)
		if !ok {
			t.Errorf("artifact schema %q is %T, not a reference object", id, ref)
			continue
		}
		digest, _ := m["digest"].(string)
		if !ValidDigest(digest) {
			t.Errorf("artifact schema %q digest %q is not sha256 hex", id, digest)
		}
		if n, _ := m["bytes"].(int); n <= 0 {
			t.Errorf("artifact schema %q reports %v bytes", id, m["bytes"])
		}
	}

	// Every ID a capability names must still be a present key, or the host
	// cannot resolve what a capability produces.
	for _, c := range desc.Capabilities {
		for _, id := range c.ArtifactSchemas {
			if _, present := desc.ArtifactSchemas[id]; !present {
				t.Errorf("capability %s references artifact schema %q which is not declared",
					c.ID, id)
			}
		}
	}
}
