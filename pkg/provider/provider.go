// Package provider is the in-process adapter that registers Facet's tool
// registry with a harness built on Compa's public agent.ToolProvider
// extension point.
//
// It is a stand-in. Agent harnesses reach Facet through MCP (`facet mcp`);
// this adapter exists only until Compa's MCP client carries long renders
// (progress, cancellation, structured results, no replay of side-effecting
// calls). It registers tools only: guidance comes from the harness's own
// skill loading, never from this package.
package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xibodev/compa/pkg/agent"
	toolshared "github.com/xibodev/compa/pkg/tools/shared"

	"github.com/xibodev/facet/internal/toolbox"
)

// FacetToolProvider implements agent.ToolProvider by wrapping Facet's
// toolbox operations as Compa-compatible tools.
type FacetToolProvider struct{}

// NewFacetToolProvider creates a new Facet tool provider.
func NewFacetToolProvider() *FacetToolProvider {
	return &FacetToolProvider{}
}

// RegisterTools contributes all Facet tools to the Compa agent.
func (p *FacetToolProvider) RegisterTools(
	workspace string,
	register func(agent.Tool),
) ([]string, func(string) (string, string, []string)) {
	var summaries []string
	for _, name := range toolbox.Names() {
		register(&facetTool{name: name, workspace: workspace})
		summaries = append(summaries, name)
	}
	return summaries, nil
}

// facetTool wraps a single Facet toolbox operation as a Studio Tool.
type facetTool struct {
	name      string
	workspace string
}

func (t *facetTool) Name() string               { return t.name }
func (t *facetTool) Description() string        { return toolbox.Description(t.name) }
func (t *facetTool) Parameters() map[string]any { return kernelSchema(toolbox.Parameters(t.name)) }

func (t *facetTool) Execute(ctx context.Context, args map[string]any) *toolshared.ToolResult {
	if err := ctx.Err(); err != nil {
		return &toolshared.ToolResult{ForLLM: err.Error(), IsError: true}
	}
	args = toolbox.ProjectArguments(args, t.workspace)
	data, err := json.Marshal(args)
	if err != nil {
		return &toolshared.ToolResult{
			ForLLM:  fmt.Sprintf("Error marshaling arguments: %v", err),
			IsError: true,
		}
	}

	env := toolbox.RunContext(ctx, t.name, data)

	if !env.OK {
		msg := "unknown error"
		if env.Error != nil {
			msg = env.Error.Message
		}
		data, _ := json.Marshal(env)
		return &toolshared.ToolResult{
			ForLLM:  msg + "\n" + string(data),
			IsError: true,
		}
	}

	// Preserve warnings, cost uncertainty and execution provenance alongside the
	// result; a native host must not lose information exposed to CLI/module hosts.
	resultJSON, err := json.Marshal(env)
	if err != nil {
		return &toolshared.ToolResult{
			ForLLM:  fmt.Sprintf("Error serializing result: %v", err),
			IsError: true,
		}
	}

	return &toolshared.ToolResult{
		ForLLM: string(resultJSON),
	}
}
