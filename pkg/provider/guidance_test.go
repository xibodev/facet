package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xibodev/facet-studio/pkg/agent"
	"github.com/xibodev/facet/internal/toolbox"
)

func TestGuidanceLoadsSelectedPackOutsideCheckout(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "facet.lock.json"), []byte(`{"packs":["explainer"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	parts, err := (guidanceContributor{workspace: root}).ContributePrompt(context.Background(), agent.PromptBuildRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 1 || !strings.Contains(parts[0].Content, "Selected pack: explainer") || !strings.Contains(parts[0].Content, "Facet Video Producer") {
		t.Fatalf("missing canonical guidance: %#v", parts)
	}
}

func TestProjectArgumentsPreserveRemoteMediaAndProse(t *testing.T) {
	root := t.TempDir()
	args := map[string]any{"input": "assets/a.mp4", "prompt": "hello world", "cuts": []any{map[string]any{"source": "https://example.com/a.mp4"}}, "audio": map[string]any{"src": "narration/a.wav"}}
	got := toolbox.ProjectArguments(args, root)
	if got["input"] != filepath.Join(root, "assets", "a.mp4") || got["prompt"] != "hello world" {
		t.Fatal(got)
	}
	if got["audio"].(map[string]any)["src"] != filepath.Join(root, "narration", "a.wav") {
		t.Fatal(got)
	}
	if got["cuts"].([]any)[0].(map[string]any)["source"] != "https://example.com/a.mp4" {
		t.Fatal(got)
	}
	if args["input"] != "assets/a.mp4" {
		t.Fatal("mutated caller arguments")
	}
}

func TestKernelSchemaPreservesJSONSchemaDefaults(t *testing.T) {
	canonical := map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"cuts": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"in_seconds": map[string]any{"type": "number"}}}}}}
	projected := kernelSchema(canonical)
	items := projected["properties"].(map[string]any)["cuts"].(map[string]any)["items"].(map[string]any)
	if items["additionalProperties"] != true || projected["additionalProperties"] != false {
		t.Fatal("schema semantics changed")
	}
	original := canonical["properties"].(map[string]any)["cuts"].(map[string]any)["items"].(map[string]any)
	if _, mutated := original["additionalProperties"]; mutated {
		t.Fatal("canonical schema mutated")
	}
}
