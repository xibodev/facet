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
type FacetToolProvider struct{}

// NewFacetToolProvider creates a new Facet tool provider.
func NewFacetToolProvider() *FacetToolProvider {
	return &FacetToolProvider{}
}

// RegisterTools contributes all enabled Facet tools to the Studio agent.
func (p *FacetToolProvider) RegisterTools(
	workspace string,
	register func(agent.Tool),
) ([]string, func(string) (string, string, []string)) {
	var summaries []string

	for _, name := range toolbox.Names() {
		t := &facetTool{name: name}
		register(t)
		summaries = append(summaries, name)
	}

	return summaries, nil
}

// facetTool wraps a single Facet toolbox operation as a Studio Tool.
type facetTool struct {
	name string
}

func (t *facetTool) Name() string        { return t.name }
func (t *facetTool) Description() string  { return toolbox.Description(t.name) }
func (t *facetTool) Parameters() map[string]any { return toolbox.Parameters(t.name) }

func (t *facetTool) Execute(ctx context.Context, args map[string]any) *toolshared.ToolResult {
	data, err := json.Marshal(args)
	if err != nil {
		return &toolshared.ToolResult{
			ForLLM:  fmt.Sprintf("Error marshaling arguments: %v", err),
			IsError: true,
		}
	}

	env := toolbox.Run(t.name, data)

	if !env.OK {
		msg := "unknown error"
		if env.Error != nil {
			msg = env.Error.Message
		}
		return &toolshared.ToolResult{
			ForLLM:  msg,
			IsError: true,
		}
	}

	resultJSON, err := json.Marshal(env.Result)
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
