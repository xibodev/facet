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
	if len(parts) != 1 || !strings.Contains(parts[0].Content, "Selected pack: explainer") || !strings.Contains(parts[0].Content, "Facet production contract") {
		t.Fatalf("missing canonical guidance: %#v", parts)
	}
}

func TestBundleGuidanceToolReadsInstalledContent(t *testing.T) {
	bundleDir := t.TempDir()
	skillDir := filepath.Join(bundleDir, "skills", "facet")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const marker = "installed-guidance-only"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(marker), 0o644); err != nil {
		t.Fatal(err)
	}
	result := (capabilityTool{
		operation:     "guidance",
		bundleDir:     bundleDir,
		bundleEntries: entrySet([]string{"skills/facet/SKILL.md"}),
	}).Execute(context.Background(), map[string]any{"path": "skills/facet/SKILL.md"})
	if result.IsError || result.ForLLM != marker {
		t.Fatalf("bundle guidance result = %#v", result)
	}
}

func TestBundleGuidanceToolRejectsEscapingPath(t *testing.T) {
	result := (capabilityTool{
		operation: "guidance",
		bundleDir: t.TempDir(),
	}).Execute(context.Background(), map[string]any{"path": "../outside"})
	if !result.IsError || !strings.Contains(result.ForLLM, "invalid Facet guidance path") {
		t.Fatalf("escaping path result = %#v", result)
	}
}

func TestBundleGuidanceToolReadsCanonicalSupportAssets(t *testing.T) {
	bundleDir := t.TempDir()
	path := filepath.Join(bundleDir, "agents", "facet-creative.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("agent-marker"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := (capabilityTool{
		operation:     "guidance",
		bundleDir:     bundleDir,
		bundleEntries: entrySet([]string{"agents/facet-creative.md"}),
	}).Execute(context.Background(), map[string]any{"path": "agents/facet-creative.md"})
	if result.IsError || result.ForLLM != "agent-marker" {
		t.Fatalf("support guidance result = %#v", result)
	}
}

func TestBundleGuidanceToolRejectsUndeclaredFile(t *testing.T) {
	bundleDir := t.TempDir()
	path := filepath.Join(bundleDir, "skills", "stale", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := (capabilityTool{
		operation:     "guidance",
		bundleDir:     bundleDir,
		bundleEntries: entrySet([]string{"skills/facet/SKILL.md"}),
	}).Execute(context.Background(), map[string]any{"path": "skills/stale/SKILL.md"})
	if !result.IsError || !strings.Contains(result.ForLLM, "not declared") {
		t.Fatalf("undeclared guidance result = %#v", result)
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
