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

// Both input shapes must produce the same duration from the same timings.
//
// The scene path was fixed to state its own end; the cuts path was not, so the
// same tool rendered 2s from a scene plan and 3s from the equivalent cuts plan
// — 60 frames versus 90. A caller switching between the documented shapes got
// a different video for the same intent.
//
// Fixing one input shape and not the other is the same mistake as the earlier
// width/height fix, which honoured dimensions on the direct-props branch and
// silently ignored them for scenes.
func TestCutsAndScenesAgreeOnDuration(t *testing.T) {
	cuts := []map[string]any{
		{"id": "c1", "type": "text_card", "in_seconds": 0.0, "out_seconds": 2.0},
		{"id": "c2", "type": "text_card", "in_seconds": 2.0, "out_seconds": 4.5},
	}
	scenes := []map[string]any{
		{"id": "c1", "type": "text_card", "start_seconds": 0.0, "end_seconds": 2.0},
		{"id": "c2", "type": "text_card", "start_seconds": 2.0, "end_seconds": 4.5},
	}

	// The scene path derives its duration from mapped cuts.
	mapped := make([]map[string]any, 0, len(scenes))
	for _, s := range scenes {
		mapped = append(mapped, mapSceneToCut(s))
	}
	fromScenes := lastCutEnd(mapped)

	// The cuts path derives it from the cuts directly.
	fromCuts := lastCutEnd(cuts)

	if fromCuts != fromScenes {
		t.Errorf("cuts end at %v but the equivalent scenes end at %v; "+
			"the same timings must render the same length", fromCuts, fromScenes)
	}
	if fromCuts != 4.5 {
		t.Errorf("end = %v, want 4.5 — the last cut's out_seconds", fromCuts)
	}
}
