package toolbox

import (
	"testing"
	"time"
)

// The estimate's "will this fit the host's deadline" threshold was the literal
// 55: correct when the safety margin was 5s (60-5), stale the moment the margin
// dropped to 1s. Nothing failed, because a stale NUMBER stays plausible in a way
// a stale LIST does not — the estimate simply warned about renders that had four
// spare seconds.
//
// This is the same duplicated-constant defect already fixed for chargeability and
// determinism. It survived here because the duplicate was a literal in an
// expression rather than a second list.
func TestDeadlineThresholdIsDerivedNotHardcoded(t *testing.T) {
	want := DefaultHostDeadline - DeadlineSafetyMargin
	if FitsDefaultHostDeadline != want {
		t.Errorf("threshold %v, want %v (default %v less margin %v)",
			FitsDefaultHostDeadline, want, DefaultHostDeadline, DeadlineSafetyMargin)
	}

	// The value the stale literal had. Asserting it is WRONG is what proves the
	// fix moved behaviour: with margin at 1s, 55 reserves 5s and warns about a
	// render that fits.
	if FitsDefaultHostDeadline == 55*time.Second {
		t.Error("threshold is still 55s, the value from when the margin was 5s")
	}
	if got := FitsDefaultHostDeadline.Seconds(); got != 59 {
		t.Errorf("threshold %.0fs, want 59s", got)
	}
}

// A render between the old and new threshold is the behavioural difference: it
// was reported as NOT fitting the default deadline, and it does fit.
func TestRenderInTheStaleGapNowReportsFitting(t *testing.T) {
	// Find a frame count whose pessimistic estimate lands in (55s, 59s].
	var frames int
	for f := 1; f < 4000; f++ {
		max := (renderFixedSeconds + float64(f)*renderSecondsPerFrame) * renderLoadFactor
		if max > 55 && max <= 59 {
			frames = f
			break
		}
	}
	if frames == 0 {
		t.Skip("no frame count lands in the stale gap at this reference size")
	}

	got := estimateRender([]string{"video_compose_remotion_render"}, frames, 1280, 720)
	exceeds, ok := got["exceeds_default_host_deadline"].(bool)
	if !ok {
		t.Fatal("exceeds_default_host_deadline missing or not a bool")
	}
	if exceeds {
		max := got["estimated_duration_seconds_max"]
		t.Errorf("a %d-frame render estimated at %v s max is reported as exceeding the "+
			"%v budget; it fits with room to spare", frames, max, DefaultHostDeadline)
	}
}

// The margin must stay small enough to be worth reserving. A margin approaching
// the budget would make the threshold meaningless.
func TestMarginIsASmallFractionOfTheBudget(t *testing.T) {
	if DeadlineSafetyMargin >= DefaultHostDeadline/10 {
		t.Errorf("margin %v is %.0f%% of the %v budget; too much to reserve for envelope writing",
			DeadlineSafetyMargin,
			100*DeadlineSafetyMargin.Seconds()/DefaultHostDeadline.Seconds(),
			DefaultHostDeadline)
	}
}
