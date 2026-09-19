package toolbox

import (
	"errors"
	"strings"
	"testing"
)

func TestExplainerCutsAcceptOnlyRetainedPrimitives(t *testing.T) {
	for _, cut := range []map[string]any{
		{"type": "text_card", "text": "hello"},
		{"type": "hero_title", "text": "hello"},
		{"type": "stat_card", "stat": "42%"},
		{"type": "media", "source": "asset.png", "media_kind": "image"},
		{"type": "media", "source": "asset.mp4", "media_kind": "video"},
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
		{"unknown", map[string]any{"type": "bar_chart", "chartData": []any{1}}, "unsupported type"},
		{"missing type", map[string]any{"text": "hello"}, "unsupported type"},
		{"blank text", map[string]any{"type": "text_card", "text": " "}, "nonblank text"},
		{"missing stat", map[string]any{"type": "stat_card"}, "nonblank stat"},
		{"missing media kind", map[string]any{"type": "media", "source": "asset.png"}, "media_kind"},
		{"bad media kind", map[string]any{"type": "media", "source": "asset.png", "media_kind": "audio"}, "media_kind"},
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

func TestExplainerRequiresCuts(t *testing.T) {
	if err := validateExplainerCuts(nil); err == nil {
		t.Fatal("empty cuts were accepted")
	}
}
