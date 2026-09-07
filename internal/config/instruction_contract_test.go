package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/xibodev/facet/internal/toolbox"
)

func TestRepositoryInstructionContract(t *testing.T) {
	const canonical = "skills/facet/SKILL.md"
	for _, file := range []string{"CLAUDE.md", canonical, "packs/explainer/SKILL.md"} {
		data, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(file)))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if file == "CLAUDE.md" && !strings.Contains(text, canonical) {
			t.Errorf("%s must reference %s", file, canonical)
		}
		for _, forbidden := range []string{`(?i)\bturn\s*[123]\b`, `"(?:file_path|video_path)"\s*:`, `(?i)\b(?:must|required to|always)\s+(?:read|load|scan|discover)\b`, `(?i)\b(?:must|required to|always)\s+(?:write|draft|create)\s+(?:a\s+)?script\b`} {
			if regexp.MustCompile(forbidden).MatchString(text) {
				t.Errorf("%s retains a numbered turn or stale request key matching %q", file, forbidden)
			}
		}
		if !regexp.MustCompile(`(?i)\bnarration\b[^.\n]*(?:optional|depend|unwanted)|\bno narration\b`).MatchString(text) {
			t.Errorf("%s must make narration conditional on the request", file)
		}
		if file != "CLAUDE.md" {
			for _, required := range []string{"authoritative for normal production", "take precedence over deep legacy references", "supplied intent", "core", "pack entry", "relevant specialized need or an actual error", "never", "preflight requirement", "source archaeology", "persona/pipeline ceremony", "explicit consent", "plan"} {
				if !strings.Contains(text, required) {
					t.Errorf("%s lacks %q", file, required)
				}
			}
		}
	}
}

func TestGeneratedInstructionContract(t *testing.T) {
	for _, engine := range []struct {
		name string
		root string
	}{
		{"claude", ".claude/skills/"},
		{"opencode", ".opencode/skills/"},
		{"copilot", ".github/skills/"},
		{"codex", ".codex/skills/"},
	} {
		for _, packs := range []struct {
			name  string
			packs []string
		}{
			{"core", nil},
			{"explainer", []string{"explainer"}},
			{"seven-packs", []string{"explainer", "cinematic", "screen-demo", "talking-head", "social", "character-animation", "localization"}},
		} {
			t.Run(engine.name+"/"+packs.name, func(t *testing.T) {
				dir := t.TempDir()
				owned := loadOwnership(dir)
				scaffoldAgentInstructions(dir, engine.name, packs.packs, owned)
				for _, file := range []string{"CLAUDE.md", "AGENTS.md", ".github/copilot-instructions.md"} {
					data, err := os.ReadFile(filepath.Join(dir, file))
					if err != nil {
						t.Fatal(err)
					}
					text := string(data)
					if lines := len(strings.Split(strings.TrimSpace(text), "\n")); lines > 50 {
						t.Errorf("%s has %d lines", file, lines)
					} else {
						t.Logf("%s: %d lines", file, lines)
					}
					for _, required := range []string{"explicit consent", "silent-video", "mock:true", "gflow_image", "gflow_video", "strategy", "1920x1080/30fps", "`" + engine.root + "facet/SKILL.md`", "workspace-relative", "core skill and active pack entry SKILL.md", "authoritative for normal production", "take precedence over deep legacy references", "relevant specialized need or an actual error", "never as a preflight requirement", "supplied intent", "documented core tools", "not source archaeology or persona/pipeline ceremony", "briefly explain the plan"} {
						if !strings.Contains(text, required) {
							t.Errorf("%s lacks %q", file, required)
						}
					}
					for _, pack := range packs.packs {
						if !strings.Contains(text, "`"+engine.root+pack+"/SKILL.md`") {
							t.Errorf("%s lacks engine-local %s entry", file, pack)
						}
					}
					for _, other := range []string{".claude/skills/", ".opencode/skills/", ".github/skills/", ".codex/skills/", ".copilot/skills/"} {
						if other != engine.root && strings.Contains(text, other) {
							t.Errorf("%s contains wrong engine path %q", file, other)
						}
					}
					for _, forbidden := range []string{"Turn 1", "Turn 2", "Turn 3", "sole", `"file_path":`, `"video_path":`} {
						if strings.Contains(text, forbidden) {
							t.Errorf("%s retains %q", file, forbidden)
						}
					}
					for _, forbidden := range []string{`(?i)\b(?:must|required to|always)\s+(?:read|load|scan|discover)\b`, `(?i)\b(?:must|required to|always)\s+(?:write|draft|create)\s+(?:a\s+)?script\b`} {
						if regexp.MustCompile(forbidden).MatchString(text) {
							t.Errorf("%s requires boilerplate discovery or a script matching %q", file, forbidden)
						}
					}
				}
			})
		}
	}
}

func TestInstructionRepairPreservesUserFiles(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "AGENTS.md")
	const user = "User-owned instructions stay untouched.\n"
	if err := os.WriteFile(file, []byte(user), 0600); err != nil {
		t.Fatal(err)
	}
	scaffoldAgentInstructions(dir, "opencode", nil, loadOwnership(dir))
	data, err := os.ReadFile(file)
	if err != nil || string(data) != user {
		t.Fatalf("unmanaged instructions overwritten: %q, %v", data, err)
	}
}

func TestGeneratedRequestsEstimateThroughCLI(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("PATH", t.TempDir())
	scaffoldAgentInstructions(dir, "opencode", []string{"explainer"}, loadOwnership(dir))
	for _, file := range []string{"assets/source.mp4", "renders/final.mp4"} {
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("estimate-only fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile("AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	examples := regexp.MustCompile(`facet tools run (\w+) --input '(\{[^\n]+\})'`).FindAllStringSubmatch(string(data), -1)
	if len(examples) != 4 {
		t.Fatalf("expected four inline examples, got %d", len(examples))
	}
	for _, example := range examples {
		if env, ok := toolbox.CLI([]string{"tools", "estimate", example[1], "--input", example[2]}); !ok {
			t.Errorf("generated %s request failed: %+v", example[1], env.Error)
		}
	}
}
