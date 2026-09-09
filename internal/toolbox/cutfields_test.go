package toolbox

import (
	"encoding/json"
	"testing"
)

// A CUT MUST REACH THE COMPOSITION WITH THE FIELDS THE CALLER SENT.
//
// composeCut named 9 fields; the composition reads 60. Via the envelope path
// every other field -- heroSubtitle, leftLabel, chartData, steps, columns --
// was dropped at decode, so a cut whose scene type needed one rendered EMPTY
// and reported success.
//
// Reported by a target agentic CLI reading this source. It avoided the bug by
// rewriting its scene plan to text-only types, which is the correct workaround
// and not something a user should have to derive from Go structs.
func TestCutPreservesFieldsFacetDoesNotName(t *testing.T) {
	body := []byte(`{
	  "id":"c1","source":"","type":"comparison",
	  "in_seconds":0,"out_seconds":4,
	  "title":"named field",
	  "heroSubtitle":"unnamed",
	  "leftLabel":"Hammer","leftValue":"1.62",
	  "chartData":[{"label":"a","value":1}]
	}`)

	var cut composeCut
	if err := json.Unmarshal(body, &cut); err != nil {
		t.Fatal(err)
	}

	// Named fields still decode normally.
	if cut.Type != "comparison" || cut.Title != "named field" {
		t.Errorf("named fields lost: type=%q title=%q", cut.Type, cut.Title)
	}

	// Re-marshal is what actually reaches Remotion.
	out, err := json.Marshal(cut)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}

	for _, field := range []string{"heroSubtitle", "leftLabel", "leftValue", "chartData"} {
		if _, ok := got[field]; !ok {
			t.Errorf("field %q was dropped; the cut renders blank and the run reports success", field)
		}
	}
	// And the named ones must survive the merge.
	for _, field := range []string{"type", "title", "in_seconds", "out_seconds"} {
		if _, ok := got[field]; !ok {
			t.Errorf("named field %q was lost while preserving unknown fields", field)
		}
	}
}

// A cut missing the field its scene type requires must WARN, on every path.
//
// The guard ran only when RawProps was set -- absent from the envelope path,
// which is the path that used to drop the fields it checks for.
func TestBlankCutIsDetectedFromDecodedCuts(t *testing.T) {
	var cut composeCut
	if err := json.Unmarshal([]byte(`{"id":"c1","source":"","type":"bar_chart","in_seconds":0,"out_seconds":2}`), &cut); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal([]composeCut{cut})
	if err != nil {
		t.Fatal(err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	warns := blankCutWarnings(decoded)
	if len(warns) == 0 {
		t.Error("a bar_chart with no chartData produced no warning; it renders blank and reports success")
	}
}
