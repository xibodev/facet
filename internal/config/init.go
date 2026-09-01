package config

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// InitResult records the artifacts and paths created during initialization.
type InitResult struct {
	ProjectDir  string `json:"project_dir"`
	Engine      string `json:"engine"`
	SkillsPath  string `json:"skills_path"`
	LinkMethod  string `json:"link_method"` // "junction", "symlink", "copy"
	ConfigFile  string `json:"config_file"`
	TemplateDir string `json:"template_dir,omitempty"`
}

// RunInit initializes a project workspace, links the appropriate agent skills,
// and sets up the project configuration and templates.
func RunInit(projectSlug string, engine string, cfg *Config) error {
	_, err := RunInitWithWriter(projectSlug, engine, cfg, os.Stdout)
	return err
}

// RunInitWithWriter initializes the workspace and prints progress to w.
func RunInitWithWriter(projectSlug string, engine string, cfg *Config, w io.Writer) (*InitResult, error) {
	if cfg == nil {
		var err error
		cfg, err = Load()
		if err != nil {
			cfg = DefaultConfig()
		}
	} else {
		cfg.AutoDetect()
	}

	// Normalize engine
	engine = strings.ToLower(strings.TrimSpace(engine))
	if engine == "" {
		if cfg.Defaults.Engine != "" {
			engine = strings.ToLower(cfg.Defaults.Engine)
		} else {
			engine = "claude"
		}
	}

	// Determine project directory
	projectSlug = strings.TrimSpace(projectSlug)
	targetDir := "."
	if projectSlug != "" {
		targetDir = filepath.Clean(projectSlug)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create project directory %s: %w", targetDir, err)
	}

	result := &InitResult{
		ProjectDir: targetDir,
		Engine:     engine,
	}

	if w != nil {
		fmt.Fprintf(w, "Initializing Facet workspace in: %s (engine: %s)\n", targetDir, engine)
	}

	// 1. Create standard project subdirectories if a project slug was provided
	if projectSlug != "" {
		_ = os.MkdirAll(filepath.Join(targetDir, "assets"), 0755)
		_ = os.MkdirAll(filepath.Join(targetDir, "artifacts"), 0755)
		_ = os.MkdirAll(filepath.Join(targetDir, "renders"), 0755)
	}

	// 2. Find bundle skills source
	skillsSource := findSkillsSource(cfg)

	// 3. Determine engine-specific skills directory
	skillsTarget := getSkillsTargetPath(targetDir, engine)
	result.SkillsPath = skillsTarget

	// 4. Link or copy skills into the engine directory
	if skillsSource != "" {
		method, err := linkOrCopySkills(skillsSource, skillsTarget)
		if err != nil {
			if w != nil {
				fmt.Fprintf(w, "  Warning: could not link skills: %v\n", err)
			}
		} else {
			result.LinkMethod = method
			if w != nil {
				fmt.Fprintf(w, "  Linked skills: %s -> %s (%s)\n", skillsTarget, skillsSource, method)
			}
		}
	} else {
		// If no source skills directory found, create empty placeholder skills directory
		_ = os.MkdirAll(skillsTarget, 0755)
		result.LinkMethod = "created"
	}

	// 5. If GitHub / Copilot engine, configure .github/copilot-instructions.md
	if engine == "copilot" || engine == "github" {
		copilotDir := filepath.Join(targetDir, ".github")
		_ = os.MkdirAll(copilotDir, 0755)
		instrPath := filepath.Join(copilotDir, "copilot-instructions.md")
		if _, err := os.Stat(instrPath); os.IsNotExist(err) {
			instrContent := `# Facet Video Kit Copilot Instructions

This repository uses the Facet Video Kit engine for autonomous video production.
Toolbox and pipeline skills are located under .github/skills/facet/ or the root skills/ directory.

## Core Directives:
- Consult pipeline definitions in pipeline_defs/
- Follow the 7-stage production lifecycle
- Use videokit tools for media operations and deterministic validation
`
			_ = os.WriteFile(instrPath, []byte(instrContent), 0644)
		}
	}

	// 6. Create initial project template or .facet.yaml
	configFilePath := filepath.Join(targetDir, ".facet.yaml")
	projectCfg := *cfg
	projectCfg.Project = projectSlug
	projectCfg.Defaults.Engine = engine

	if err := projectCfg.Save(configFilePath); err != nil {
		return nil, fmt.Errorf("failed to save project config at %s: %w", configFilePath, err)
	}
	result.ConfigFile = configFilePath

	if w != nil {
		fmt.Fprintf(w, "  Created project configuration: %s\n", configFilePath)
	}

	// 7. Scaffold initial brief if project slug was provided and brief doesn't exist
	if projectSlug != "" {
		briefPath := filepath.Join(targetDir, "artifacts", "brief.md")
		if _, err := os.Stat(briefPath); os.IsNotExist(err) {
			briefContent := fmt.Sprintf(`# Production Brief: %s

## Metadata
- Project: %s
- Engine: %s
- Target Resolution: %s
- Target FPS: %d
- Aspect Ratio: %s
- Voice: %s

## Concept & Objectives
<!-- Describe the purpose, audience, and core message of this video -->

## Pipeline & Style
<!-- Recommended pipeline: source-edit, animated-explainer, cinematic, talking-head, screen-demo -->
`, projectSlug, projectSlug, engine, projectCfg.Defaults.Resolution, projectCfg.Defaults.FPS, projectCfg.Defaults.AspectRatio, projectCfg.Defaults.Voice)
			_ = os.WriteFile(briefPath, []byte(briefContent), 0644)
		}
	}

	if w != nil {
		fmt.Fprintln(w, "Workspace initialization complete.")
	}

	return result, nil
}

func findSkillsSource(cfg *Config) string {
	candidates := []string{}
	if cfg != nil && cfg.Paths.Bundle != "" {
		candidates = append(candidates, filepath.Join(cfg.Paths.Bundle, "skills"))
	}
	candidates = append(candidates,
		"skills",
		filepath.Join("..", "skills"),
		filepath.Join("..", "..", "skills"),
	)

	for _, cand := range candidates {
		if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
			if abs, err := filepath.Abs(cand); err == nil {
				return abs
			}
		}
	}
	return ""
}

func getSkillsTargetPath(targetDir, engine string) string {
	switch engine {
	case "claude":
		return filepath.Join(targetDir, ".claude", "skills", "facet")
	case "opencode":
		return filepath.Join(targetDir, ".opencode", "skills", "facet")
	case "copilot", "github":
		return filepath.Join(targetDir, ".github", "skills", "facet")
	default:
		return filepath.Join(targetDir, ".claude", "skills", "facet")
	}
}

func linkOrCopySkills(sourceDir, targetDir string) (string, error) {
	parentDir := filepath.Dir(targetDir)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create parent dir %s: %w", parentDir, err)
	}

	// Remove target if it already exists
	if _, err := os.Lstat(targetDir); err == nil {
		_ = os.RemoveAll(targetDir)
	}

	// Try Windows Directory Junction on Windows
	if runtime.GOOS == "windows" {
		winSrc := filepath.FromSlash(sourceDir)
		winDst := filepath.FromSlash(targetDir)
		cmd := exec.Command("cmd.exe", "/c", "mklink", "/J", winDst, winSrc)
		if err := cmd.Run(); err == nil {
			return "junction", nil
		}
	}

	// Try symlink
	if err := os.Symlink(sourceDir, targetDir); err == nil {
		return "symlink", nil
	}

	// Fallback to recursive copy
	if err := copyDirectory(sourceDir, targetDir); err != nil {
		return "", fmt.Errorf("directory copy fallback failed: %w", err)
	}

	return "copy", nil
}

func copyDirectory(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDirectory(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err == nil {
		_ = os.Chmod(dst, srcInfo.Mode())
	}
	return nil
}
