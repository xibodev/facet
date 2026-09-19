package toolbox

import (
	"encoding/json"
	"errors"
	"math"
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
		{"missing type", map[string]any{"text": "hello", "in_seconds": 0.0, "out_seconds": 1.0}, "type must be a nonblank string"},
		{"blank text", map[string]any{"type": "text_card", "text": " ", "in_seconds": 0.0, "out_seconds": 1.0}, "text must be a nonblank string"},
		{"missing stat", map[string]any{"type": "stat_card", "in_seconds": 0.0, "out_seconds": 1.0}, "stat must be a nonblank string"},
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

func TestVideoComposeEstimateMatchesReducedComposerRejections(t *testing.T) {
	baseCut := map[string]any{
		"type": "text_card", "text": "hello",
		"in_seconds": 0.0, "out_seconds": 1.0,
	}
	props := func(overrides map[string]any, cuts ...map[string]any) []byte {
		t.Helper()
		request := map[string]any{"cuts": []map[string]any{baseCut}}
		for key, value := range overrides {
			request[key] = value
		}
		if len(cuts) > 0 {
			request["cuts"] = cuts
		}
		data, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}

	cases := []struct {
		name  string
		input []byte
		want  string
	}{
		{"cuts type", []byte(`{"cuts":"not-an-array"}`), "cuts must be a nonempty array"},
		{"empty cuts", []byte(`{"cuts":[]}`), "cuts must be a nonempty array"},
		{"scenes type", []byte(`{"scenes":"not-an-array"}`), "scenes must be a nonempty array"},
		{"empty scenes", []byte(`{"scenes":[]}`), "scenes must be a nonempty array"},
		{"width type", props(map[string]any{"width": "1280"}), "width"},
		{"odd width", props(map[string]any{"width": 1279}), "positive even safe integer"},
		{"fractional height", props(map[string]any{"height": 719.5}), "positive even safe integer"},
		{"unsafe width", props(map[string]any{"width": 9007199254740992.0}), "positive even safe integer"},
		{"fps type", props(map[string]any{"fps": "30"}), "fps"},
		{"zero fps", props(map[string]any{"fps": 0}), "fps must be positive"},
		{"duration type", props(map[string]any{"duration_seconds": "1"}), "duration_seconds"},
		{"zero duration", props(map[string]any{"duration_seconds": 0}), "duration_seconds must be positive"},
		{"fractional frame count", props(map[string]any{"fps": 30, "duration_seconds": 1.01}), "positive safe integer frame count"},
		{"cut start type", props(nil, map[string]any{"type": "text_card", "text": "hello", "in_seconds": "0", "out_seconds": 1}), "in_seconds"},
		{"cut end type", props(nil, map[string]any{"type": "text_card", "text": "hello", "in_seconds": 0, "out_seconds": "1"}), "out_seconds"},
		{"overlap", props(nil,
			map[string]any{"type": "text_card", "text": "one", "in_seconds": 0, "out_seconds": 1},
			map[string]any{"type": "text_card", "text": "two", "in_seconds": 0.5, "out_seconds": 2},
		), "overlaps or is out of order"},
		{"scene overlap", []byte(`{"scenes":[{"type":"text_card","text":"one","start_seconds":0,"end_seconds":1},{"type":"text_card","text":"two","start_seconds":0.5,"end_seconds":2}]}`), "overlaps or is out of order"},
		{"outside duration", props(map[string]any{"duration_seconds": 1}, map[string]any{"type": "text_card", "text": "hello", "in_seconds": 0, "out_seconds": 2}), "exceeds duration_seconds"},
		{"subframe span", props(map[string]any{"fps": 30, "duration_seconds": 2}, map[string]any{"type": "text_card", "text": "hello", "in_seconds": 1.0, "out_seconds": 1.000000000001}), "at least one frame"},
		{"type json type", props(nil, map[string]any{"type": 7, "text": "hello", "in_seconds": 0, "out_seconds": 1}), "type must be a nonblank string"},
		{"text json type", props(nil, map[string]any{"type": "text_card", "text": 7, "in_seconds": 0, "out_seconds": 1}), "text must be a nonblank string"},
		{"font size type", props(nil, map[string]any{"type": "text_card", "text": "hello", "fontSize": "large", "in_seconds": 0, "out_seconds": 1}), "fontSize"},
		{"subtitle type", props(nil, map[string]any{"type": "hero_title", "text": "hello", "subtitle": 7, "in_seconds": 0, "out_seconds": 1}), "subtitle"},
		{"stat type", props(nil, map[string]any{"type": "stat_card", "stat": 42, "in_seconds": 0, "out_seconds": 1}), "stat must be a nonblank string"},
		{"media source type", props(nil, map[string]any{"type": "media", "source": 7, "media_kind": "image", "in_seconds": 0, "out_seconds": 1}), "source must be a nonblank string"},
		{"media fit", props(nil, map[string]any{"type": "media", "source": "asset.png", "media_kind": "image", "fit": "stretch", "in_seconds": 0, "out_seconds": 1}), "fit"},
		{"media muted type", props(nil, map[string]any{"type": "media", "source": "asset.png", "media_kind": "image", "muted": "yes", "in_seconds": 0, "out_seconds": 1}), "muted"},
		{"background type", props(map[string]any{"backgroundColor": 7}), "backgroundColor"},
		{"audio type", props(map[string]any{"audio": "voice.wav"}), "audio must be an object"},
		{"audio empty", props(map[string]any{"audio": map[string]any{}}), "audio must contain narration or music"},
		{"track source type", props(map[string]any{"audio": map[string]any{"narration": map[string]any{"src": 7}}}), "audio.narration.src"},
		{"track volume", props(map[string]any{"audio": map[string]any{"music": map[string]any{"src": "music.wav", "volume": 2}}}), "volume must be between 0 and 1"},
		{"track loop type", props(map[string]any{"audio": map[string]any{"music": map[string]any{"src": "music.wav", "loop": "yes"}}}), "loop must be a boolean"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := doVideoCompose("estimate", tc.input)
			var failed *toolFailure
			if !errors.As(err, &failed) || failed.err.Code != "invalid_request" || !strings.Contains(failed.err.Message, tc.want) {
				t.Fatalf("got %v, want invalid_request containing %q", err, tc.want)
			}
		})
	}

	for _, tc := range []struct {
		name string
		raw  map[string]any
		want string
	}{
		{"nonfinite fps", map[string]any{"fps": math.NaN(), "cuts": []map[string]any{baseCut}}, "fps"},
		{"nonfinite duration", map[string]any{"duration_seconds": math.Inf(1), "cuts": []map[string]any{baseCut}}, "duration_seconds"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cuts := tc.raw["cuts"].([]map[string]any)
			err := validateExplainerComposition(tc.raw, cuts)
			var failed *toolFailure
			if !errors.As(err, &failed) || !strings.Contains(failed.err.Message, tc.want) {
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
