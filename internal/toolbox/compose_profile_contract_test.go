package toolbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExplainerProfileSchemaContract(t *testing.T) {
	schema := contractJSON(t, schemas["video_compose"]).(map[string]any)
	properties := schema["properties"].(map[string]any)
	for field, want := range map[string]float64{"width": 1920, "height": 1080, "fps": 30} {
		if properties[field].(map[string]any)["default"] != want {
			t.Fatalf("wrong %s default", field)
		}
	}
	if _, ok := properties["duration_seconds"].(map[string]any)["default"]; ok {
		t.Fatal("duration default depends on cuts; it must not be a fixed schema value")
	}
	if !strings.Contains(schema["description"].(string), "do not prove a render") {
		t.Fatal("schema must disclose the renderer-only validation boundary")
	}
	request := map[string]any{
		"cuts":  []any{map[string]any{"type": "text_card", "text": "Test", "in_seconds": float64(0), "out_seconds": float64(3)}},
		"width": float64(320), "height": float64(180), "fps": float64(24), "duration_seconds": float64(3),
	}
	if !contractValid(schema, request) {
		t.Fatal("schema rejects explicit render profile")
	}
	for field, values := range map[string][]any{
		"width":            {nil, "320", float64(0), float64(321), 320.5, float64(9007199254740992)},
		"height":           {nil, "180", float64(-2), float64(181)},
		"fps":              {nil, "24", float64(0), float64(-1)},
		"duration_seconds": {nil, "3", float64(0), float64(-1)},
	} {
		original := request[field]
		for _, value := range values {
			request[field] = value
			if contractValid(schema, request) {
				t.Errorf("schema accepts invalid %s=%v", field, value)
			}
		}
		request[field] = original
	}
	// Go estimates route direct props without evaluating Remotion's cross-field rules.
	t.Setenv("PATH", t.TempDir())
	for _, duration := range []float64{3, 0.1, 2} {
		request["duration_seconds"] = duration
		data, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		env, ok := CLI([]string{"tools", "estimate", "video_compose", "--input", string(data)})
		if !ok || !strings.Contains(string(mustProfileJSON(t, env.Result)), "video_compose_remotion_render") {
			t.Fatalf("estimate should route only, not claim metadata validation: %+v", env)
		}
	}
}

func TestExplainerDirectProfileReachesRenderer(t *testing.T) {
	// Capture actual --props bytes and fail intentionally before any media is rendered.
	workspace := composeDeliveryFixture(t, `
const fs = require('fs');
const arg = process.argv.find(value => value.startsWith('--props='));
fs.copyFileSync(arg.slice('--props='.length), '../captured-profile.json');
process.exit(23);
`)
	request := map[string]any{
		"composition_id": "Explainer", "output": filepath.Join(workspace, "output.mp4"),
		"width": 320, "height": 180, "fps": 24, "duration_seconds": 3,
		"cuts": []map[string]any{{"id": "intro", "type": "text_card", "source": "", "in_seconds": 0, "out_seconds": 3, "text": "Test"}},
	}

	_, _, err := doVideoCompose("run", mustProfileJSON(t, request))
	if err == nil {
		t.Fatal("capture-only renderer must fail, never report a successful render")
	}
	data, err := os.ReadFile(filepath.Join(workspace, "captured-profile.json"))
	if err != nil {
		t.Fatal(err)
	}
	var props map[string]any
	if err := json.Unmarshal(data, &props); err != nil {
		t.Fatal(err)
	}
	for field, want := range map[string]float64{"width": 320, "height": 180, "fps": 24, "duration_seconds": 3} {
		if props[field] != want {
			t.Errorf("renderer lost %s: got %v, want %v", field, props[field], want)
		}
	}
}

func TestExplainerSchemaRejectsRemovedSceneTypes(t *testing.T) {
	schema := contractJSON(t, schemas["video_compose"]).(map[string]any)
	for _, request := range []map[string]any{
		{"cuts": []any{map[string]any{"type": "bar_chart", "chartData": []any{1}, "in_seconds": float64(0), "out_seconds": float64(1)}}},
		{"cuts": []any{map[string]any{"type": "text_card", "text": "", "in_seconds": float64(0), "out_seconds": float64(1)}}},
		{"cuts": []any{map[string]any{"type": "media", "source": "clip.mp4", "in_seconds": float64(0), "out_seconds": float64(1)}}},
	} {
		if contractValid(schema, request) {
			t.Errorf("schema accepted unsupported or blank request: %v", request)
		}
	}
}

func mustProfileJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
