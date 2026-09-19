package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("expected non-nil default config")
	}

	if cfg.Defaults.Engine != "claude" {
		t.Errorf("expected engine claude, got %s", cfg.Defaults.Engine)
	}
	if cfg.Defaults.Voice != "en-US-ChristopherNeural" {
		t.Errorf("expected voice en-US-ChristopherNeural, got %s", cfg.Defaults.Voice)
	}
	if cfg.Defaults.Resolution != "1920x1080" {
		t.Errorf("expected resolution 1920x1080, got %s", cfg.Defaults.Resolution)
	}
	if cfg.Defaults.FPS != 30 {
		t.Errorf("expected fps 30, got %d", cfg.Defaults.FPS)
	}
	if cfg.Defaults.AspectRatio != "16:9" {
		t.Errorf("expected aspect ratio 16:9, got %s", cfg.Defaults.AspectRatio)
	}
	if cfg.Defaults.PermissionMode != "rw" {
		t.Errorf("expected permission mode rw, got %s", cfg.Defaults.PermissionMode)
	}
	if len(cfg.EnvProbes) == 0 {
		t.Errorf("expected non-empty env probes")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".facet.yaml")

	cfg := DefaultConfig()
	cfg.Project = "test-video"
	cfg.Defaults.Engine = "opencode"
	cfg.Defaults.Voice = "custom-voice"
	cfg.Paths.FFmpeg = "/custom/path/ffmpeg"

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file was not created: %v", err)
	}

	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.Project != "test-video" {
		t.Errorf("expected project test-video, got %s", loaded.Project)
	}
	if loaded.Defaults.Engine != "opencode" {
		t.Errorf("expected engine opencode, got %s", loaded.Defaults.Engine)
	}
	if loaded.Defaults.Voice != "custom-voice" {
		t.Errorf("expected voice custom-voice, got %s", loaded.Defaults.Voice)
	}
	if loaded.Paths.FFmpeg != "/custom/path/ffmpeg" {
		t.Errorf("expected ffmpeg path /custom/path/ffmpeg, got %s", loaded.Paths.FFmpeg)
	}
}

func TestAutoDetect(t *testing.T) {
	cfg := DefaultConfig()
	// Pin custom FFmpeg path
	cfg.Paths.FFmpeg = "/pinned/ffmpeg"

	detected := cfg.AutoDetect()
	if detected == nil {
		t.Fatal("expected non-nil detected map")
	}

	// Pinned path should not be overwritten
	if cfg.Paths.FFmpeg != "/pinned/ffmpeg" {
		t.Errorf("pinned FFmpeg path was overwritten: %s", cfg.Paths.FFmpeg)
	}
}

func TestRunDoctor(t *testing.T) {
	cfg := DefaultConfig()
	var buf bytes.Buffer

	report, err := RunDoctorWithWriter(cfg, &buf)
	if err != nil {
		t.Fatalf("RunDoctorWithWriter failed: %v", err)
	}

	if report == nil {
		t.Fatal("expected non-nil report")
	}

	// Verify System Runtimes
	if len(report.Runtimes) < 5 {
		t.Errorf("expected at least 5 runtimes probed, got %d", len(report.Runtimes))
	}
	runtimeNames := map[string]bool{}
	for _, r := range report.Runtimes {
		runtimeNames[r.Name] = true
	}
	for _, expected := range []string{"FFmpeg", "FFprobe", "Node", "Remotion Composer", "Edge-TTS"} {
		if !runtimeNames[expected] {
			t.Errorf("missing runtime check for %s", expected)
		}
	}

	// Verify Agent CLIs
	if len(report.CLIs) < 3 {
		t.Errorf("expected at least 3 CLIs probed, got %d", len(report.CLIs))
	}
	cliNames := map[string]bool{}
	for _, c := range report.CLIs {
		cliNames[c.Name] = true
	}
	for _, expected := range []string{"Claude Code", "OpenCode", "GitHub Copilot"} {
		if !cliNames[expected] {
			t.Errorf("missing CLI check for %s", expected)
		}
	}

	// Verify Tools
	if len(report.Tools) < 33 {
		t.Errorf("expected at least 33 toolbox tools, got %d", len(report.Tools))
	}

	// Verify Env Vars
	if len(report.EnvVars) == 0 {
		t.Errorf("expected non-empty env vars in report")
	}

	// Verify Output Formatting
	out := buf.String()
	if !strings.Contains(out, "=== Facet System Doctor ===") {
		t.Errorf("output missing title banner")
	}
	if !strings.Contains(out, "[System Runtimes]") {
		t.Errorf("output missing [System Runtimes]")
	}
	if !strings.Contains(out, "[Agent CLIs]") {
		t.Errorf("output missing [Agent CLIs]")
	}
	if !strings.Contains(out, "[Environment Variables]") {
		t.Errorf("output missing [Environment Variables]")
	}
	if !strings.Contains(out, "[Toolbox Tools (") {
		t.Errorf("output missing [Toolbox Tools], output: %s", out)
	}

	// Verify JSON output helper
	jsonBytes, err := report.JSON()
	if err != nil {
		t.Fatalf("report.JSON() error: %v", err)
	}
	if len(jsonBytes) == 0 {
		t.Errorf("expected non-empty json output")
	}
}

func TestRunInit(t *testing.T) {
	// Create a dummy bundle directory with skills
	bundleDir := t.TempDir()
	skillsDir := filepath.Join(bundleDir, "skills", "creative")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("failed to create dummy skills dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "test-skill.md"), []byte("# Test Skill"), 0644); err != nil {
		t.Fatalf("failed to write dummy skill: %v", err)
	}

	cfg := DefaultConfig()
	cfg.Paths.Bundle = bundleDir
	for _, name := range []string{"explainer", "cinematic"} {
		writeConfigFixture(t, filepath.Join(bundleDir, "packs", name, "SKILL.md"), "# Pack")
	}

	adapters := []struct {
		engine      string
		skillsRoot  string
		instruction string
	}{
		{"claude", ".claude/skills", "CLAUDE.md"},
		{"copilot", ".github/skills", ".github/copilot-instructions.md"},
		{"codex", ".agents/skills", "AGENTS.md"},
		{"opencode", ".opencode/skills", "AGENTS.md"},
		{"studio", "skills", "AGENTS.md"},
	}
	allInstructions := []string{"CLAUDE.md", "AGENTS.md", ".github/copilot-instructions.md"}
	for _, adapter := range adapters {
		t.Run("CoreOnly/"+adapter.engine, func(t *testing.T) {
			projectDir := filepath.Join(t.TempDir(), adapter.engine+"-project")
			var buf bytes.Buffer
			res, err := RunInitWithWriter(projectDir, adapter.engine, cfg, &buf)
			if err != nil {
				t.Fatalf("RunInitWithWriter failed: %v", err)
			}
			if res.Engine != adapter.engine {
				t.Errorf("expected engine %s, got %s", adapter.engine, res.Engine)
			}
			if len(res.Packs) != 0 {
				t.Fatalf("default init activated packs: %v", res.Packs)
			}
			for _, path := range []string{".facet.yaml", "facet.lock.json", filepath.ToSlash(filepath.Join(adapter.skillsRoot, "facet", "SKILL.md")), adapter.instruction} {
				if _, err := os.Stat(filepath.Join(projectDir, filepath.FromSlash(path))); err != nil {
					t.Errorf("expected %s: %v", path, err)
				}
			}
			for _, path := range []string{"assets", "artifacts", "renders", "narration"} {
				if _, err := os.Stat(filepath.Join(projectDir, path)); !os.IsNotExist(err) {
					t.Errorf("core-only init created unsolicited %s", path)
				}
			}
			for _, instruction := range allInstructions {
				_, err := os.Stat(filepath.Join(projectDir, filepath.FromSlash(instruction)))
				if instruction == adapter.instruction {
					if err != nil {
						t.Errorf("selected instruction %s missing: %v", instruction, err)
					}
				} else if !os.IsNotExist(err) {
					t.Errorf("%s init wrote unrelated governing file %s", adapter.engine, instruction)
				}
			}
		})
	}

	// 4. Test InitWithOptions with packs and ownership
	t.Run("PacksAndOwnership", func(t *testing.T) {
		projectDir := filepath.Join(t.TempDir(), "packs-project")
		var buf bytes.Buffer
		opts := InitOptions{
			ProjectDir: projectDir,
			Engine:     "claude",
			Packs:      []string{"explainer", "cinematic"},
		}
		res, err := RunInitWithOptions(opts, cfg, &buf)
		if err != nil {
			t.Fatalf("RunInitWithOptions failed: %v", err)
		}

		if len(res.Packs) != 2 {
			t.Errorf("expected 2 packs, got %d", len(res.Packs))
		}

		// Verify facet.lock.json
		lockFile := filepath.Join(projectDir, "facet.lock.json")
		if _, err := os.Stat(lockFile); err != nil {
			t.Errorf("expected facet.lock.json: %v", err)
		}

		// Verify .facet/ownership.json
		ownFile := filepath.Join(projectDir, ".facet", "ownership.json")
		if _, err := os.Stat(ownFile); err != nil {
			t.Errorf("expected .facet/ownership.json: %v", err)
		}

		// Verify pack skill was linked
		explainerSkill := filepath.Join(projectDir, ".claude", "skills", "explainer")
		if _, err := os.Stat(explainerSkill); err != nil {
			t.Errorf("expected .claude/skills/explainer: %v", err)
		}

		// Verify only the selected engine's governing file was scaffolded.
		claudeFile := filepath.Join(projectDir, "CLAUDE.md")
		claudeContent, err := os.ReadFile(claudeFile)
		if err != nil {
			t.Errorf("expected CLAUDE.md: %v", err)
		} else {
			contentStr := string(claudeContent)
			if !strings.Contains(contentStr, "Produce and review the requested video here") {
				t.Errorf("expected CLAUDE.md to contain anti-drift rules")
			}
			if !strings.Contains(contentStr, "Never substitute mock media in production") {
				t.Errorf("expected CLAUDE.md to prohibit production mock substitution")
			}
			if !strings.Contains(contentStr, "facet tools run edge_tts") {
				t.Errorf("expected CLAUDE.md to include edge_tts signature")
			}
			if !strings.Contains(contentStr, "facet tools run output_review") {
				t.Errorf("expected CLAUDE.md to include output_review signature")
			}
			// Acceptance criteria: CLAUDE.md must be lean (<50 lines)
			lines := strings.Split(contentStr, "\n")
			if len(lines) >= 50 {
				t.Errorf("expected CLAUDE.md to be <50 lines, got %d lines", len(lines))
			}
		}

		for _, unrelated := range []string{"AGENTS.md", ".github/copilot-instructions.md"} {
			if _, err := os.Stat(filepath.Join(projectDir, filepath.FromSlash(unrelated))); !os.IsNotExist(err) {
				t.Errorf("unexpected unrelated instruction file %s", unrelated)
			}
		}
	})
}

func writeConfigFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestInstalledBundleFromUnrelatedDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", "")
	t.Chdir(t.TempDir())
	bundle := filepath.Join(home, ".facet", "bundle")
	writeConfigFixture(t, filepath.Join(bundle, "skills", "facet", "SKILL.md"), "# Installed core")
	writeConfigFixture(t, filepath.Join(bundle, "packs", "explainer", "SKILL.md"), "# Installed pack")
	writeConfigFixture(t, filepath.Join(bundle, "remotion-composer", "package.json"), "{}")
	cfg := DefaultConfig()
	cfg.AutoDetect()
	if cfg.Paths.Bundle != bundle || cfg.Paths.RemotionComposer != filepath.Join(bundle, "remotion-composer") {
		t.Fatalf("unexpected installed paths: %+v", cfg.Paths)
	}
	if got := findPackSource("@xibodev/facet-pack-explainer", nil); got != filepath.Join(bundle, "packs", "explainer") {
		t.Fatalf("home pack resolution = %q", got)
	}
	project := filepath.Join(t.TempDir(), "production")
	_, err := RunInitWithOptions(InitOptions{ProjectDir: project, Engine: "opencode", Packs: []string{"explainer"}}, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, skill := range []string{"facet", "explainer"} {
		if _, err := os.ReadFile(filepath.Join(project, ".opencode", "skills", skill, "SKILL.md")); err != nil {
			t.Fatalf("installed skill %s not projected: %v", skill, err)
		}
	}
}

func TestBundleResolutionOrder(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "checkout", "projects", "demo")
	home := filepath.Join(root, "home")
	executable := filepath.Join(root, "install", "bin", "facet")
	want := []string{cwd, filepath.Dir(cwd), filepath.Join(root, "checkout"), filepath.Join(root, "install", "bundle"), filepath.Join(root, "install"), filepath.Join(home, ".facet", "bundle")}
	if got := bundleCandidatesFor(cwd, home, executable); !reflect.DeepEqual(got, want) {
		t.Fatalf("candidate order = %v, want %v", got, want)
	}
}

func TestPinnedPathsAndProjectDefaultsSurviveInit(t *testing.T) {
	t.Chdir(t.TempDir())
	bundle := t.TempDir()
	writeConfigFixture(t, filepath.Join(bundle, "skills", "facet", "SKILL.md"), "# Custom core")
	writeConfigFixture(t, filepath.Join(bundle, "packs", "explainer", "SKILL.md"), "# Custom pack")
	// A local pack must not shadow the configured bundle.
	writeConfigFixture(t, filepath.Join("packs", "explainer", "SKILL.md"), "# Other pack")
	cfg := DefaultConfig()
	cfg.Paths = PathsConfig{Bundle: bundle, RemotionComposer: "/custom/composer", FFmpeg: "/custom/ffmpeg", Node: "/custom/node", OpenCode: "/custom/opencode"}
	cfg.Defaults.Voice = "custom-voice"
	cfg.Defaults.Resolution = "720x1280"
	cfg.Defaults.FPS = 24
	wantPaths := cfg.Paths
	cfg.AutoDetect()
	if cfg.Paths.Bundle != wantPaths.Bundle || cfg.Paths.RemotionComposer != wantPaths.RemotionComposer || cfg.Paths.FFmpeg != wantPaths.FFmpeg || cfg.Paths.Node != wantPaths.Node || cfg.Paths.OpenCode != wantPaths.OpenCode {
		t.Fatalf("pinned paths overwritten: %+v", cfg.Paths)
	}

	if got := findPackSource("explainer", cfg); got != filepath.Join(bundle, "packs", "explainer") {
		t.Fatalf("configured pack not preferred: %s", got)
	}
	project := t.TempDir()
	if err := cfg.Save(filepath.Join(project, ".facet.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := RunInitWithOptions(InitOptions{ProjectDir: project, Engine: "opencode"}, DefaultConfig(), nil); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(filepath.Join(project, ".facet.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Paths != cfg.Paths || loaded.Defaults.Voice != "custom-voice" || loaded.Defaults.Resolution != "720x1280" || loaded.Defaults.FPS != 24 || loaded.Defaults.Engine != "opencode" {
		t.Fatalf("custom project configuration lost: %+v", loaded)
	}
}

func TestSelectedPackMustExistBeforeProjectWrites(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	cfg := DefaultConfig()
	cfg.Paths.Bundle = t.TempDir()
	if _, err := RunInitWithOptions(InitOptions{
		ProjectDir: project,
		Engine:     "claude",
		Packs:      []string{"missing"},
	}, cfg, nil); err == nil {
		t.Fatal("missing selected pack was accepted")
	}
	if _, err := os.Stat(project); !os.IsNotExist(err) {
		t.Fatalf("failed pack selection wrote project files: %v", err)
	}
}
