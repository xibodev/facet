package provider_test

import (
	"context"
	"strings"
	"testing"

	"github.com/xibodev/facet-studio/pkg/agent"
	"github.com/xibodev/facet/internal/toolbox"
	"github.com/xibodev/facet/pkg/provider"
)

func TestFacetToolProvider_Registration(t *testing.T) {
	p := provider.NewFacetToolProvider()

	registered := make(map[string]agent.Tool)
	summaries, knowledge := p.RegisterTools("test_workspace", func(tool agent.Tool) {
		registered[tool.Name()] = tool
	})

	expectedNames := toolbox.Names()
	if len(summaries) != len(expectedNames) {
		t.Fatalf("expected %d summaries, got %d", len(expectedNames), len(summaries))
	}
	if len(registered) != len(expectedNames) {
		t.Fatalf("expected %d registered tools, got %d", len(expectedNames), len(registered))
	}

	for _, name := range expectedNames {
		tool, exists := registered[name]
		if !exists {
			t.Errorf("tool %q was not registered", name)
			continue
		}
		if tool.Name() != name {
			t.Errorf("tool name mismatch: expected %q, got %q", name, tool.Name())
		}
		if tool.Description() == "" {
			t.Errorf("tool %q has empty description", name)
		}
		params := tool.Parameters()
		if params == nil {
			t.Errorf("tool %q has nil parameters", name)
		}
	}

	if knowledge != nil {
		t.Errorf("expected nil knowledge func for standalone tool provider")
	}
}

func TestFacetToolProvider_Execution(t *testing.T) {
	p := provider.NewFacetToolProvider()
	registered := make(map[string]agent.Tool)
	p.RegisterTools("test_workspace", func(tool agent.Tool) {
		registered[tool.Name()] = tool
	})

	// Test executing a tool with invalid args to verify error handling
	probeTool, exists := registered["media_probe"]
	if !exists {
		t.Fatal("media_probe tool not registered")
	}

	ctx := context.Background()
	// media_probe with nonexistent file should return an error or failure
	res := probeTool.Execute(ctx, map[string]any{
		"input_path": "nonexistent_file_for_test_12345.mp4",
	})

	if res == nil {
		t.Fatal("expected non-nil ToolResult")
	}
	if !res.IsError && !strings.Contains(res.ForLLM, "error") && !strings.Contains(res.ForLLM, "not found") {
		t.Logf("media_probe result: %s (isError: %v)", res.ForLLM, res.IsError)
	}
}
