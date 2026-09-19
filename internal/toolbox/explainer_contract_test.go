package toolbox

import (
	"errors"
	"strings"
	"testing"
)

func TestExplainerCutsAcceptOnlyRetainedPrimitives(t *testing.T) {
	for _, cut := range []map[string]any{
		{"type": "text_card", "text": "hello", "in_seconds": 0.0, "out_seconds": 1.0},
		{"type": "hero_title", "text": "hello", "in_seconds": 0.0, "out_seconds": 1.0},
		{"type": "stat_card", "stat": "42%", "in_seconds": 0.0, "out_seconds": 1.0},
		{"type": "media", "source": "asset.png", "media_kind": "image", "in_seconds": 0.0, "out_seconds": 1.0},
		{"type": "media", "source": "asset.mp4", "media_kind": "video", "in_seconds": 0.0, "out_seconds": 1.0},
	} {
		if err := validateExplainerCuts([]map[string]any{cut}); err != nil {
			t.Errorf("retained cut rejected: %v: %v", cut, err)
		}
	}
}

func TestExplainerCutsRejectBlankAndUnknownScenes(t *testing.T) {
	for _, tc := range []struct {
		name string
		cut  map[string]any
		want string
	}{
		{"unknown", map[string]any{"type": "bar_chart", "chartData": []any{1}, "in_seconds": 0.0, "out_seconds": 1.0}, "unsupported type"},
		{"missing type", map[string]any{"text": "hello", "in_seconds": 0.0, "out_seconds": 1.0}, "unsupported type"},
		{"blank text", map[string]any{"type": "text_card", "text": " ", "in_seconds": 0.0, "out_seconds": 1.0}, "nonblank text"},
		{"missing stat", map[string]any{"type": "stat_card", "in_seconds": 0.0, "out_seconds": 1.0}, "nonblank stat"},
		{"missing media kind", map[string]any{"type": "media", "source": "asset.png", "in_seconds": 0.0, "out_seconds": 1.0}, "media_kind"},
		{"bad media kind", map[string]any{"type": "media", "source": "asset.png", "media_kind": "audio", "in_seconds": 0.0, "out_seconds": 1.0}, "media_kind"},
		{"missing start", map[string]any{"type": "text_card", "text": "hello", "out_seconds": 1.0}, "in_seconds"},
		{"missing end", map[string]any{"type": "text_card", "text": "hello", "in_seconds": 0.0}, "out_seconds"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateExplainerCuts([]map[string]any{tc.cut})
			var failed *toolFailure
			if !errors.As(err, &failed) || failed.err.Code != "invalid_request" || !strings.Contains(failed.err.Message, tc.want) {
				t.Fatalf("got %v, want invalid_request containing %q", err, tc.want)
			}
		})
	}
}

func TestVideoComposeEstimatePreflightsReducedComposerScenes(t *testing.T) {
	valid := []string{
		`{"scenes":[{"type":"text_card","text":"hello","start_seconds":0,"end_seconds":1}]}`,
		`{"scenes":[{"type":"hero_title","text":"hello","start_seconds":0,"end_seconds":1}]}`,
		`{"scenes":[{"type":"stat_card","stat":"42%","start_seconds":0,"end_seconds":1}]}`,
		`{"scenes":[{"type":"media","source":"asset.png","media_kind":"image","start_seconds":0,"end_seconds":1}]}`,
	}
	for _, input := range valid {
		if _, _, err := doVideoCompose("estimate", []byte(input)); err != nil {
			t.Errorf("valid reduced-composer scene rejected: %s: %v", input, err)
		}
	}

	invalid := []struct {
		input string
		want  string
	}{
		{`{"scenes":[{"type":"bar_chart","start_seconds":0,"end_seconds":1}]}`, "unsupported type"},
		{`{"scenes":[{"type":"text_card","start_seconds":0,"end_seconds":1}]}`, "text"},
		{`{"scenes":[{"type":"media","source":"asset.png","start_seconds":0,"end_seconds":1}]}`, "media_kind"},
		{`{"cuts":[{"type":"hero_title","text":"hello","out_seconds":1}]}`, "in_seconds"},
	}
	for _, tc := range invalid {
		_, _, err := doVideoCompose("estimate", []byte(tc.input))
		var failed *toolFailure
		if !errors.As(err, &failed) || failed.err.Code != "invalid_request" || !strings.Contains(failed.err.Message, tc.want) {
			t.Errorf("estimate(%s) = %v, want invalid_request containing %q", tc.input, err, tc.want)
		}
	}
}

func TestExplainerRequiresCuts(t *testing.T) {
	if err := validateExplainerCuts(nil); err == nil {
		t.Fatal("empty cuts were accepted")
	}
}
