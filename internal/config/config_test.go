package config

import (
	"bytes"
	"os"
	"path/filepath"
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

	// 1. Test Claude engine init
	t.Run("ClaudeEngine", func(t *testing.T) {
		projectDir := filepath.Join(t.TempDir(), "claude-project")
		var buf bytes.Buffer
		res, err := RunInitWithWriter(projectDir, "claude", cfg, &buf)
		if err != nil {
			t.Fatalf("RunInitWithWriter failed: %v", err)
		}

		if res.Engine != "claude" {
			t.Errorf("expected engine claude, got %s", res.Engine)
		}
		if _, err := os.Stat(filepath.Join(projectDir, ".facet.yaml")); err != nil {
			t.Errorf("expected .facet.yaml in project dir: %v", err)
		}
		if _, err := os.Stat(filepath.Join(projectDir, "artifacts", "brief.md")); err != nil {
			t.Errorf("expected artifacts/brief.md in project dir: %v", err)
		}
		if _, err := os.Stat(filepath.Join(projectDir, "assets")); err != nil {
			t.Errorf("expected assets dir: %v", err)
		}
		if _, err := os.Stat(filepath.Join(projectDir, ".claude", "skills", "facet")); err != nil {
			t.Errorf("expected .claude/skills/facet dir or link: %v", err)
		}
	})

	// 2. Test OpenCode engine init
	t.Run("OpenCodeEngine", func(t *testing.T) {
		projectDir := filepath.Join(t.TempDir(), "opencode-project")
		var buf bytes.Buffer
		res, err := RunInitWithWriter(projectDir, "opencode", cfg, &buf)
		if err != nil {
			t.Fatalf("RunInitWithWriter failed: %v", err)
		}

		if res.Engine != "opencode" {
			t.Errorf("expected engine opencode, got %s", res.Engine)
		}
		if _, err := os.Stat(filepath.Join(projectDir, ".opencode", "skills", "facet")); err != nil {
			t.Errorf("expected .opencode/skills/facet dir or link: %v", err)
		}
	})

	// 3. Test Copilot engine init
	t.Run("CopilotEngine", func(t *testing.T) {
		projectDir := filepath.Join(t.TempDir(), "copilot-project")
		var buf bytes.Buffer
		res, err := RunInitWithWriter(projectDir, "copilot", cfg, &buf)
		if err != nil {
			t.Fatalf("RunInitWithWriter failed: %v", err)
		}

		if res.Engine != "copilot" {
			t.Errorf("expected engine copilot, got %s", res.Engine)
		}
		if _, err := os.Stat(filepath.Join(projectDir, ".github", "skills", "facet")); err != nil {
			t.Errorf("expected .github/skills/facet dir or link: %v", err)
		}
		if _, err := os.Stat(filepath.Join(projectDir, ".github", "copilot-instructions.md")); err != nil {
			t.Errorf("expected .github/copilot-instructions.md: %v", err)
		}
	})

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

		// Verify CLAUDE.md and AGENTS.md were scaffolded with anti-drift directives
		claudeFile := filepath.Join(projectDir, "CLAUDE.md")
		claudeContent, err := os.ReadFile(claudeFile)
		if err != nil {
			t.Errorf("expected CLAUDE.md: %v", err)
		} else {
			contentStr := string(claudeContent)
			if !strings.Contains(contentStr, "Never Suggest External Manual Tools") {
				t.Errorf("expected CLAUDE.md to contain anti-drift rules")
			}
			if !strings.Contains(contentStr, "Immediate Action on Turn 1") {
				t.Errorf("expected CLAUDE.md to mandate Turn 1 immediate action")
			}
			if !strings.Contains(contentStr, "facet tools run edgetts") {
				t.Errorf("expected CLAUDE.md to include edgetts signature")
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

		agentsFile := filepath.Join(projectDir, "AGENTS.md")
		if _, err := os.Stat(agentsFile); err != nil {
			t.Errorf("expected AGENTS.md: %v", err)
		}
	})
}

