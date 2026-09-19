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
		if file == canonical {
			for _, required := range []string{"stateless toolbox", "explicit human consent", "unknown", "mock: true", "creative acceptance", "matching pack"} {
				if !strings.Contains(text, required) {
					t.Errorf("%s lacks %q", file, required)
				}
			}
		}
		if file == "packs/explainer/SKILL.md" {
			for _, required := range []string{"Narration and music are optional", "Estimate before rendering", "review the final MP4"} {
				if !strings.Contains(text, required) {
					t.Errorf("%s lacks %q", file, required)
				}
			}
		}
	}
}

func TestGeneratedInstructionContract(t *testing.T) {
	for _, engine := range []struct {
		name        string
		root        string
		instruction string
	}{
		{"claude", ".claude/skills/", "CLAUDE.md"},
		{"opencode", ".opencode/skills/", "AGENTS.md"},
		{"copilot", ".github/skills/", ".github/copilot-instructions.md"},
		{"codex", ".agents/skills/", "AGENTS.md"},
		{"studio", "skills/", "AGENTS.md"},
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
				selected := filepath.Join(dir, filepath.FromSlash(engine.instruction))
				if err := os.MkdirAll(filepath.Dir(selected), 0755); err != nil {
					t.Fatal(err)
				}
				const userInstructions = "Keep adapter-specific user instructions.\n"
				if err := os.WriteFile(selected, []byte(userInstructions), 0600); err != nil {
					t.Fatal(err)
				}
				if err := scaffoldAgentInstructions(dir, engine.name, packs.packs, owned); err != nil {
					t.Fatal(err)
				}
				for _, file := range []string{"CLAUDE.md", "AGENTS.md", ".github/copilot-instructions.md"} {
					data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(file)))
					if file != engine.instruction {
						if !os.IsNotExist(err) {
							t.Errorf("%s wrote unrelated governing file %s", engine.name, file)
						}
						continue
					}
					if err != nil {
						t.Fatal(err)
					}
					text := string(data)
					if !strings.HasPrefix(text, userInstructions) {
						t.Errorf("%s did not preserve pre-existing user instructions", file)
					}
					if lines := len(strings.Split(strings.TrimSpace(text), "\n")); lines > 50 {
						t.Errorf("%s has %d lines", file, lines)
					} else {
						t.Logf("%s: %d lines", file, lines)
					}
					for _, required := range []string{"explicit consent", "silent-video", "mock:true", "gflow_image", "gflow_video", "strategy", "1920x1080/30fps", "`" + engine.root + "facet/SKILL.md`", "workspace-relative", "canonical core skill", "active pack entry", "Facet-owned files define the production contract", "briefly explain the plan"} {
						if !strings.Contains(text, required) {
							t.Errorf("%s lacks %q", file, required)
						}
					}
					for _, required := range []string{
						"exactly four scene primitives",
						"`text_card` requires `text`",
						"`hero_title` requires `text`",
						"`stat_card` requires `stat`",
						"`media` requires `source` and `media_kind` (`image` or `video`)",
						"Every cut requires `type`, `in_seconds`, and `out_seconds`",
						"`id` is optional",
						"`source` is required only for `media` cuts",
						"Omit `duration_seconds` to end exactly at the last cut, with no padding",
					} {
						if !strings.Contains(text, required) {
							t.Errorf("%s lacks composer contract %q", file, required)
						}
					}
					if strings.Contains(strings.ToLower(text), "theme") {
						t.Errorf("%s documents unsupported theme", file)
					}
					for _, pack := range packs.packs {
						if !strings.Contains(text, "`"+engine.root+pack+"/SKILL.md`") {
							t.Errorf("%s lacks engine-local %s entry", file, pack)
						}
					}
					for _, other := range []string{".claude/skills/", ".opencode/skills/", ".github/skills/", ".agents/skills/", ".codex/skills/", ".copilot/skills/"} {
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
					if len(packs.packs) == 0 && strings.Contains(strings.ToLower(text), "explainer") {
						t.Errorf("%s core-only guidance implicitly activates explainer", file)
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
	owned := loadOwnership(dir)
	if err := scaffoldAgentInstructions(dir, "opencode", nil, owned); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.HasPrefix(text, user) || strings.Count(text, "<!-- facet:managed:start -->") != 1 || strings.Count(text, "<!-- facet:managed:end -->") != 1 {
		t.Fatalf("unmanaged instructions were not preserved around one Facet section: %q", text)
	}
	if err := os.WriteFile(file, append(data, []byte("\nLater user edit.\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	if err := saveOwnership(dir, owned); err != nil {
		t.Fatal(err)
	}
	owned = loadOwnership(dir)
	if err := scaffoldAgentInstructions(dir, "opencode", []string{"explainer"}, owned); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "Later user edit.") || strings.Count(string(updated), "<!-- facet:managed:start -->") != 1 {
		t.Fatalf("rerun did not preserve user edits outside the managed section: %q", updated)
	}
}

func TestInstructionEngineChangeRemovesOnlyManagedGoverningFile(t *testing.T) {
	dir := t.TempDir()
	owned := loadOwnership(dir)
	const claudeUser = "Keep these user-owned Claude instructions.\n"
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(claudeUser), 0600); err != nil {
		t.Fatal(err)
	}
	if err := scaffoldAgentInstructions(dir, "claude", nil, owned); err != nil {
		t.Fatal(err)
	}
	const user = "Keep these user-owned Codex instructions.\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(user), 0600); err != nil {
		t.Fatal(err)
	}
	if err := scaffoldAgentInstructions(dir, "copilot", nil, owned); err != nil {
		t.Fatal(err)
	}
	claude, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil || string(claude) != claudeUser {
		t.Fatalf("adapter switch did not remove only the prior Facet section: %q, %v", claude, err)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md")); err != nil || string(data) != user {
		t.Fatalf("unmanaged AGENTS.md changed: %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".github", "copilot-instructions.md")); err != nil {
		t.Fatal(err)
	}
}

func TestInstructionManagedSectionRejectsTampering(t *testing.T) {
	dir := t.TempDir()
	owned := loadOwnership(dir)
	if err := scaffoldAgentInstructions(dir, "codex", nil, owned); err != nil {
		t.Fatal(err)
	}
	if err := saveOwnership(dir, owned); err != nil {
		t.Fatal(err)
	}
	owned = loadOwnership(dir)
	file := filepath.Join(dir, "AGENTS.md")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(strings.Replace(string(data), "# Facet Video Production Workspace", "# User changed Facet", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := scaffoldAgentInstructions(dir, "codex", nil, owned); err == nil || !strings.Contains(err.Error(), "preserving modified Facet instruction section") {
		t.Fatalf("tampered managed section error = %v", err)
	}
}

func TestGeneratedRequestsEstimateThroughCLI(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("PATH", t.TempDir())
	if err := scaffoldAgentInstructions(dir, "opencode", []string{"explainer"}, loadOwnership(dir)); err != nil {
		t.Fatal(err)
	}
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
