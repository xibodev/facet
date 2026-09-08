package toolbox

import "testing"

// A scene plan states when it ends, and the rendered video should end there.
//
// Facet never passed duration_seconds, so the composition fell back to
// `lastEnd + 1` — a full second of tail padding — and a 2s request rendered as
// 3s. Measured before the fix: 90 frames at 30fps for a plan ending at 2s.
// After: 60 frames, duration exactly 2.000000.
//
// The padding exists for a final fade that is 8 frames long, so 22 of those 30
// frames served nothing. The composition already honoured duration_seconds
// exactly; the caller's intent was simply never sent.
func TestLastCutEndDrivesTheDuration(t *testing.T) {
	cuts := []map[string]any{
		{"in_seconds": 0.0, "out_seconds": 2.0},
		{"in_seconds": 2.0, "out_seconds": 4.5},
		// Out of order on purpose: the LAST end is the largest, not the final
		// element, and a plan may list scenes in any order.
		{"in_seconds": 1.0, "out_seconds": 3.0},
	}
	if got := lastCutEnd(cuts); got != 4.5 {
		t.Errorf("lastCutEnd = %v, want 4.5", got)
	}
}

// With nothing usable to measure, report 0 so the caller leaves
// duration_seconds unset and the composition keeps its own default. Sending a
// fabricated zero would fail the render outright.
func TestNoUsableEndReportsZero(t *testing.T) {
	for _, cuts := range [][]map[string]any{
		nil,
		{},
		{{"in_seconds": 0.0}},
		{{"out_seconds": "not a number"}},
	} {
		if got := lastCutEnd(cuts); got != 0 {
			t.Errorf("lastCutEnd(%v) = %v, want 0 so the default is kept", cuts, got)
		}
	}
}
