package toolbox

import (
	"os"
	"strings"
	"testing"
)

// A fact that is built but not projected must say WHY, at the source.
//
// facet-studio's practice, adopted: an unwired thing that says why is a
// decision; one that says nothing is a bug waiting to be discovered. Their
// repo has shipped unwired-because-forgotten twice, and from outside it looks
// identical to unwired-because-no-wire.
//
// Their refinement, also adopted: the label must name the UNBLOCKING EVENT,
// not merely the state. "Not wired yet" tells a reader nothing they can act
// on; "v1 has no per-Operation effects and is immutable" tells them what has
// to happen first.
//
// Message history is the wrong place for this. The next reader will not have
// our thread.
func TestUnprojectedFactsNameTheirUnblockingEvent(t *testing.T) {
	raw, err := os.ReadFile("toolbox.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)

	// Each built-but-unprojected fact, with a phrase that only appears if the
	// unblocking event is named rather than the state alone.
	for _, c := range []struct{ fact, event string }{
		{"MayCharge", "v1 carries effects only on the"},
		{"Deterministic", "v1 has no per-Operation effects"},
		{"resolutionOf", "v1 has no Resolution"},
	} {
		i := strings.Index(text, "func "+c.fact)
		if i < 0 {
			t.Errorf("%s not found; this test can no longer see what it guards", c.fact)
			continue
		}
		// The label must be NEAR the fact, not merely somewhere in the file.
		// A fixed byte window and a contiguous-comment walk both failed here:
		// resolutionOf's label is separated from its declaration by a const
		// block, so the doc comment is not contiguous and 2000 bytes was not
		// enough. Searching the region before the declaration is what the
		// requirement actually is — a reader scrolling up must meet it.
		start := i - 4000
		if start < 0 {
			start = 0
		}
		doc := text[start:i]
		if !strings.Contains(doc, "NOT PROJECTED") {
			t.Errorf("%s is unprojected but does not say so at the source", c.fact)
		}
		if !strings.Contains(doc, c.event) {
			t.Errorf("%s says it is unprojected but does not name the unblocking "+
				"event; a reader learns the state and not what must happen first", c.fact)
		}
	}
}
