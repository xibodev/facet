package toolbox

import (
	"strings"
	"testing"
)

// A cut whose type is recognised renders NOTHING unless its content field is
// present, and the resulting file gives the caller no reason to doubt it:
// ok:true, an artifact on disk, a plausible byte count. The only thing that
// disagrees is output_review's content gate, which is a separate opt-in step.
//
// Verified: a scene with "txt" instead of "text" rendered blank and reported
// success. The render path itself said nothing about its own most expensive
// silent failure.
func TestMisspelledContentFieldIsWarnedAbout(t *testing.T) {
	got := blankCutWarnings([]map[string]any{
		{"type": "text_card", "txt": "misspelled", "in_seconds": 0.0, "out_seconds": 2.0},
	})
	if len(got) != 1 {
		t.Fatalf("a cut that will render blank produced %d warnings: %v", len(got), got)
	}
	// The message must name the field, or it does not help find the typo.
	if !strings.Contains(got[0], "text") || !strings.Contains(got[0], "text_card") {
		t.Errorf("the warning does not name the missing field or the type: %q", got[0])
	}
}

// Every requirement here mirrors a guard in the composition. A complete cut
// must never be warned about, or the warning becomes noise callers learn to
// ignore — which would be worse than silence.
func TestCompleteCutsAreNotWarnedAbout(t *testing.T) {
	for _, cut := range []map[string]any{
		{"type": "text_card", "text": "hello"},
		{"type": "stat_card", "stat": "42%"},
		{"type": "bar_chart", "chartData": []any{1, 2}},
		{"type": "line_chart", "chartSeries": []any{1}},
		{"type": "progress_bar", "progress": 0.5},
		{"type": "comparison", "leftLabel": "a", "rightLabel": "b",
			"leftValue": "1", "rightValue": "2"},
	} {
		if got := blankCutWarnings([]map[string]any{cut}); len(got) != 0 {
			t.Errorf("a complete %v cut was warned about: %v", cut["type"], got)
		}
	}
}

// A type this map does not know must pass silently. The composition may gain
// one, and refusing or warning about a valid cut is worse than staying quiet.
func TestUnknownCutTypesArePassedOver(t *testing.T) {
	if got := blankCutWarnings([]map[string]any{{"type": "some_future_type"}}); len(got) != 0 {
		t.Errorf("an unknown cut type was warned about: %v", got)
	}
}

// A progress of 0 is a legitimate value, not an absent field. Treating a zero
// as missing would warn about a correct cut.
func TestZeroProgressIsNotTreatedAsMissing(t *testing.T) {
	if got := blankCutWarnings([]map[string]any{{"type": "progress_bar", "progress": 0.0}}); len(got) != 0 {
		t.Errorf("progress:0 was reported as missing: %v", got)
	}
}
