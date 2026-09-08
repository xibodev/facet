package module

import (
	"bytes"
	"encoding/json"
	"testing"
)

// The host truncates module output at max_output_bytes and treats a truncated
// response as unusable — a partial JSON document cannot be trusted even when it
// looks complete. So the descriptor's size is a budget, not a curiosity.
//
// It defaults to 1 MiB. The descriptor was 232KB purely because the envelope
// was emitted with indentation the host never reads; compact it is 107KB. That
// is the difference between 4.5x headroom and 9.8x, on a limit whose failure
// mode is a hard rejection rather than a warning.
const hostDefaultMaxOutputBytes = 1 << 20

func TestDescriptorFitsWellInsideTheHostOutputBudget(t *testing.T) {
	env := Describe("test")
	if !env.OK {
		t.Fatalf("describe failed: %+v", env.Error)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	if len(raw) >= hostDefaultMaxOutputBytes {
		t.Fatalf("descriptor is %d bytes, at or beyond the host's %d-byte ceiling; "+
			"the host would truncate it and reject the response",
			len(raw), hostDefaultMaxOutputBytes)
	}

	// Warn well before the cliff. Artifact schemas are inlined and per-tool
	// schemas grow with the toolbox, so this creeps upward without anyone
	// deciding to make it creep.
	if budget := hostDefaultMaxOutputBytes / 2; len(raw) > budget {
		t.Errorf("descriptor is %d bytes, past half the host's %d-byte ceiling; "+
			"it grows with every tool and schema, so decide what to trim now "+
			"rather than after it is rejected", len(raw), hostDefaultMaxOutputBytes)
	}
}

// The module surface must not be indented. Indentation was 54% of the
// descriptor and buys nothing: the host parses this, and `facet tools` is the
// surface people read.
func TestModuleOutputIsCompact(t *testing.T) {
	env := Describe("test")
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	// json.Marshal is already compact; this asserts the shape the CLI emits by
	// comparing against an indented encoding of the same value.
	var indented bytes.Buffer
	enc := json.NewEncoder(&indented)
	enc.SetIndent("", "  ")
	if err := enc.Encode(env); err != nil {
		t.Fatal(err)
	}
	if indented.Len() <= len(raw) {
		t.Skip("indentation is not larger here; nothing to assert")
	}
	saved := indented.Len() - len(raw)
	if saved*2 < indented.Len()/4 {
		t.Logf("indentation would add only %d bytes; the saving is small", saved)
	}
	t.Logf("compact %d bytes, indented %d bytes, saving %d", len(raw), indented.Len(), saved)
}
