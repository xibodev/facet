package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	facet "github.com/xibodev/facet"
	"github.com/xibodev/facet-studio/pkg/agent"
	shared "github.com/xibodev/facet-studio/pkg/tools/shared"
	"github.com/xibodev/facet/internal/toolbox"
)

// MountGuidance composes Facet's canonical core and selected packs through the
// kernel's prompt extension contract. No second prompt engine or copied skills.
func MountGuidance(loop *agent.AgentLoop) error {
	for _, id := range loop.GetRegistry().ListAgentIDs() {
		instance, ok := loop.GetRegistry().GetAgent(id)
		if !ok {
			continue
		}
		if err := instance.ContextBuilder.RegisterPromptContributor(guidanceContributor{workspace: instance.Workspace}); err != nil {
			return err
		}
	}
	return nil
}

type guidanceContributor struct{ workspace string }

func (guidanceContributor) PromptSource() agent.PromptSourceDescriptor {
	return agent.PromptSourceDescriptor{ID: "facet.capability", Owner: "facet", Description: "Canonical Facet producer guidance", Allowed: []agent.PromptPlacement{{Layer: agent.PromptLayerCapability, Slot: agent.PromptSlotActiveSkill}}}
}

func (g guidanceContributor) ContributePrompt(ctx context.Context, _ agent.PromptBuildRequest) ([]agent.PromptPart, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	core, err := facet.Guidance("skills/facet/SKILL.md")
	if err != nil {
		return nil, err
	}
	var lock struct {
		Packs []string `json:"packs"`
	}
	if data, err := os.ReadFile(filepath.Join(g.workspace, "facet.lock.json")); err == nil {
		if err := json.Unmarshal(data, &lock); err != nil {
			return nil, fmt.Errorf("read selected Facet packs: %w", err)
		}
	}
	var text strings.Builder
	text.WriteString("You are Facet's video-production assistant. The application is Facet; Facet Studio supplies its embedded runtime.\n")
	text.WriteString("Transport binding: Facet tool names below are native callable tools. Execute them directly, not through an external agent CLI or a facet subprocess. Use facet_describe and facet_estimate for contracts and estimates. Read supporting guidance with facet_guidance using its canonical path; these bundled files need not exist in the project. Project-relative input and output paths refer to the current project.\n")
	text.WriteString(core)
	fmt.Fprintf(&text, "\nAvailable capability packs: %s. Read an unselected pack through facet_guidance when the task calls for it; do not load every pack in advance.\n", strings.Join(facet.PackNames(), ", "))
	for _, pack := range lock.Packs {
		body, err := facet.Guidance("packs/" + pack + "/SKILL.md")
		if err != nil {
			return nil, fmt.Errorf("load selected pack %s: %w", pack, err)
		}
		fmt.Fprintf(&text, "\n\n## Selected pack: %s\n%s", pack, body)
	}
	text.WriteString("\n## Standalone transport binding (applies to all examples above)\nThis is the embedded Facet application. Any `facet tools run NAME --input FILE` example means: read FILE, then call the native NAME tool with the file's JSON object as its arguments. For video_compose, passing only input_path pointing at saved props JSON loads those props directly, preserving their exact profile. Do not run a facet executable from PATH: it may be a different version. Use native facet_describe and facet_estimate. File-writing tools accept serialized text content, not a JSON object in their content field. Never substitute a renderer or silently change the requested profile after a tool failure.\n")
	return []agent.PromptPart{{ID: "facet.producer", Layer: agent.PromptLayerCapability, Slot: agent.PromptSlotActiveSkill, Source: agent.PromptSource{ID: "facet.capability"}, Content: text.String()}}, nil
}

type capabilityTool struct {
	operation string
	workspace string
}

func (t capabilityTool) Name() string { return "facet_" + t.operation }
func (t capabilityTool) Description() string {
	switch t.operation {
	case "guidance":
		return "Read bundled Facet guidance and schemas by canonical path (skills/, packs/, agents/, schemas/)."
	case "estimate":
		return "Estimate a Facet operation without executing it."
	default:
		return "Describe a native Facet tool's input contract."
	}
}
func (t capabilityTool) Parameters() map[string]any {
	if t.operation == "guidance" {
		return map[string]any{"type": "object", "required": []string{"path"}, "properties": map[string]any{"path": map[string]any{"type": "string"}}}
	}
	return map[string]any{"type": "object", "required": []string{"tool"}, "properties": map[string]any{"tool": map[string]any{"type": "string"}, "input": map[string]any{"type": "object"}}}
}
func (t capabilityTool) Execute(ctx context.Context, args map[string]any) *shared.ToolResult {
	if err := ctx.Err(); err != nil {
		return &shared.ToolResult{ForLLM: err.Error(), IsError: true}
	}
	if t.operation == "guidance" {
		name, _ := args["path"].(string)
		body, err := facet.Guidance(name)
		if err != nil {
			return &shared.ToolResult{ForLLM: err.Error(), IsError: true}
		}
		return &shared.ToolResult{ForLLM: body}
	}
	name, _ := args["tool"].(string)
	argv := []string{"tools", t.operation, name}
	if t.operation == "estimate" {
		input, _ := args["input"].(map[string]any)
		if input == nil {
			input = map[string]any{}
		}
		data, _ := json.Marshal(toolbox.ProjectArguments(input, t.workspace))
		argv = append(argv, "--input", string(data))
	}
	env, ok := toolbox.CLI(argv)
	data, _ := json.Marshal(env)
	return &shared.ToolResult{ForLLM: string(data), IsError: !ok}
}
