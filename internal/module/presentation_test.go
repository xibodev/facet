package module

import "testing"

// presentation exists for what a media type cannot express.
//
// Captions are written as .srt/.vtt and detected as text/plain, which tells a
// host to render prose. They are timed cues: as a paragraph the timings become
// noise, and the one question a reviewer actually has — does this line up with
// the video — cannot be answered.
//
// timeline is already the primitive used for scene plans and edit decisions,
// so this names a value the host owns rather than inventing one.
func TestCaptionsArePresentedAsATimeline(t *testing.T) {
	for _, path := range []string{"renders/captions.srt", "out/subs.vtt"} {
		if got := presentationFor(path, "text/plain; charset=utf-8"); got != "timeline" {
			t.Errorf("%s presentation = %q, want timeline", path, got)
		}
	}
}

// A media type that already says what it is must not be second-guessed. An
// unknown value is ignored by the host, so a wrong guess is silent — which is
// why absent is the honest answer when nothing needs adding.
func TestSelfDescribingMediaGetsNoHint(t *testing.T) {
	for _, c := range []struct{ path, mediaType string }{
		{"renders/final.mp4", "video/mp4"},
		{"audio/vo.mp3", "audio/mpeg"},
		{"frames/f1.jpg", "image/jpeg"},
	} {
		if got := presentationFor(c.path, c.mediaType); got != "" {
			t.Errorf("%s got presentation %q; %s already says what it is",
				c.path, got, c.mediaType)
		}
	}
}

// Removed workflow-document contracts must not survive as special presentation
// behavior after their schemas are gone.
func TestWorkflowDocumentsGetNoSpecialPresentation(t *testing.T) {
	for _, path := range []string{"artifacts/scene_plan.json", "artifacts/edit_decisions.json"} {
		if got := presentationFor(path, "application/json"); got != "" {
			t.Errorf("%s presentation = %q, want no workflow-specific hint", path, got)
		}
	}
}
