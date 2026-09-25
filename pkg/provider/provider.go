// Package provider implements agent.ToolProvider for Facet's native toolbox.
//
// This is the NATIVE binding path: it registers Facet's video-production
// tools directly with the Studio kernel, paying no subprocess or protocol
// cost to host itself.
//
// The three projection shapes:
//
//	F-SKILL  -- external-agent CLI uses toolbox tools via skill guidance
//	F-APP    -- Studio kernel with this provider (native, in-process)
//	F-MOD    -- Studio kernel with module-v2 projection (detached host)
package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xibodev/facet-studio/pkg/agent"
	toolshared "github.com/xibodev/facet-studio/pkg/tools/shared"

	"github.com/xibodev/facet/internal/toolbox"
)

// FacetToolProvider implements agent.ToolProvider by wrapping Facet's native
// toolbox operations as Studio-compatible tools.
type FacetToolProvider struct {
	bundleDir     string
	bundleEntries map[string]struct{}
}

// NewFacetToolProvider creates a new Facet tool provider.
func NewFacetToolProvider() *FacetToolProvider {
	return &FacetToolProvider{}
}

// NewBundleToolProvider creates a provider whose guidance tools read only from
// a verified installed bundle.
func NewBundleToolProvider(bundleDir string, entries ...string) *FacetToolProvider {
	return &FacetToolProvider{bundleDir: bundleDir, bundleEntries: entrySet(entries)}
}

// RegisterTools contributes all enabled Facet tools to the Studio agent.
func (p *FacetToolProvider) RegisterTools(
	workspace string,
	register func(agent.Tool),
) ([]string, func(string) (string, string, []string)) {
	var summaries []string

	for _, name := range toolbox.Names() {
		t := &facetTool{name: name, workspace: workspace}
		register(t)
		summaries = append(summaries, name)
	}
	for _, operation := range []string{"describe", "estimate", "guidance"} {
		register(capabilityTool{
			operation:     operation,
			workspace:     workspace,
			bundleDir:     p.bundleDir,
			bundleEntries: p.bundleEntries,
		})
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
