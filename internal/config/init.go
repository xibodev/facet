package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// InitOptions controls workspace creation, engine selection, and packs.
type InitOptions struct {
	ProjectDir  string   `json:"project_dir"`
	Engine      string   `json:"engine"`
	Packs       []string `json:"packs"`
	Resolution  string   `json:"resolution,omitempty"`
	FPS         int      `json:"fps,omitempty"`
	AspectRatio string   `json:"aspect_ratio,omitempty"`
	Voice       string   `json:"voice,omitempty"`
}

// InitResult records the artifacts and paths created during initialization.
type InitResult struct {
	ProjectDir  string            `json:"project_dir"`
	Engine      string            `json:"engine"`
	SkillsPath  string            `json:"skills_path"`
	LinkMethod  string            `json:"link_method"` // "junction", "symlink", "copy"
	ConfigFile  string            `json:"config_file"`
	TemplateDir string            `json:"template_dir,omitempty"`
	Packs       []string          `json:"packs,omitempty"`
	Projections map[string]string `json:"projections,omitempty"`
}

// OwnershipRecord tracks entries managed by Facet to prevent clobbering user files.
type OwnershipRecord struct {
	Schema         string                  `json:"schema"`
	InstallationID string                  `json:"installation_id"`
	FacetVersion   string                  `json:"facet_version"`
	ManagedEntries map[string]ManagedEntry `json:"managed_entries"`
}

// ManagedEntry represents an individual managed path or projection.
type ManagedEntry struct {
	EntryType string `json:"entry_type"` // "directory-junction", "symlink", "copy"
	Target    string `json:"target"`
	CreatedOn string `json:"created_on"`
}

// ProjectLock represents the portable facet.lock.json pinned to a project.
type ProjectLock struct {
	Version  string   `json:"version"`
	Engine   string   `json:"engine"`
	Packs    []string `json:"packs"`
	LockedAt string   `json:"locked_at"`
}

// RunInit initializes a project workspace with default packs.
func RunInit(projectSlug string, engine string, cfg *Config) error {
	_, err := RunInitWithWriter(projectSlug, engine, cfg, os.Stdout)
	return err
}

// RunInitWithWriter initializes the workspace and prints progress to w.
func RunInitWithWriter(projectSlug string, engine string, cfg *Config, w io.Writer) (*InitResult, error) {
	opts := InitOptions{
		ProjectDir: projectSlug,
		Engine:     engine,
	}
	return RunInitWithOptions(opts, cfg, w)
}

// RunInitWithOptions initializes a workspace with custom options.
func RunInitWithOptions(opts InitOptions, cfg *Config, w io.Writer) (*InitResult, error) {
	// Reopening a project must not replace its pinned paths with Studio defaults.
	projectConfig := filepath.Join(opts.ProjectDir, ".facet.yaml")
	if _, err := os.Stat(projectConfig); err == nil {
		var err error
		cfg, err = Load(projectConfig)
		if err != nil {
			return nil, err
		}
	}
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
	engine := strings.ToLower(strings.TrimSpace(opts.Engine))
	if engine == "" {
		if cfg.Defaults.Engine != "" {
			engine = strings.ToLower(cfg.Defaults.Engine)
		} else {
			engine = "claude"
		}
	}
	switch engine {
	case "claude", "copilot", "codex", "opencode", "studio":
	default:
		return nil, fmt.Errorf("unsupported engine %q", engine)
	}

	packSources := make(map[string]string, len(opts.Packs))
	normalizedPacks := make([]string, 0, len(opts.Packs))
	for _, packName := range opts.Packs {
		packName = strings.ToLower(strings.TrimSpace(packName))
		if packName == "" || packName == "." || filepath.Base(packName) != packName || strings.ContainsAny(packName, `/\`) {
			return nil, fmt.Errorf("invalid pack name %q", packName)
		}
		if _, exists := packSources[packName]; exists {
			continue
		}
		packSource := findPackSource(packName, cfg)
		if packSource == "" {
			return nil, fmt.Errorf("selected pack %q was not found in the Facet bundle", packName)
		}
		packSources[packName] = packSource
		normalizedPacks = append(normalizedPacks, packName)
	}
	opts.Packs = normalizedPacks

	// Determine project directory
	projectSlug := strings.TrimSpace(opts.ProjectDir)
	targetDir := "."
	if projectSlug != "" {
		targetDir = filepath.Clean(projectSlug)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create project directory %s: %w", targetDir, err)
	}

	result := &InitResult{
		ProjectDir:  targetDir,
		Engine:      engine,
		Packs:       append([]string(nil), opts.Packs...),
		Projections: make(map[string]string),
	}

	if w != nil {
		fmt.Fprintf(w, "Initializing Facet workspace in: %s (engine: %s)\n", targetDir, engine)
	}

	// Local Facet state is required. Production directories are created only
	// when a selected method or tool actually needs them.
	_ = os.MkdirAll(filepath.Join(targetDir, ".facet"), 0755)

	// 2. Git exclude handling (add .facet/ to .git/info/exclude if git repository)
	ensureGitExclude(targetDir)

	// 3. Load or initialize ownership record
	ownership := loadOwnership(targetDir)

	// 4. Link core producer skill
	coreSource := findCoreSkillSource(cfg)
	coreTarget := getSkillsTargetPath(targetDir, engine, "facet")
	result.SkillsPath = coreTarget

	if coreSource != "" {
		method, err := linkOrCopySkillSafe(coreSource, coreTarget, targetDir, ownership)
		if err != nil {
			if w != nil {
				fmt.Fprintf(w, "  Warning: could not link core skill: %v\n", err)
			}
		} else {
			result.LinkMethod = method
			result.Projections[coreTarget] = method
			if w != nil {
				fmt.Fprintf(w, "  Linked core skill: %s -> %s (%s)\n", coreTarget, coreSource, method)
			}
		}
	} else {
		_ = os.MkdirAll(coreTarget, 0755)
		result.LinkMethod = "created"
	}

	// 5. Link only explicitly selected production-method packs.
	for _, packName := range opts.Packs {
		packSource := packSources[packName]
		packTarget := getSkillsTargetPath(targetDir, engine, packName)
		method, err := linkOrCopySkillSafe(packSource, packTarget, targetDir, ownership)
		if err != nil {
			if w != nil {
				fmt.Fprintf(w, "  Warning: could not link pack %s: %v\n", packName, err)
			}
		} else {
			result.Projections[packTarget] = method
			if w != nil {
				fmt.Fprintf(w, "  Linked pack '%s': %s -> %s (%s)\n", packName, packTarget, packSource, method)
			}
		}
	}

	// 6. Scaffold agent instruction files (CLAUDE.md, AGENTS.md, copilot-instructions.md)
	scaffoldAgentInstructions(targetDir, engine, opts.Packs, ownership)

	// 7. Save ownership record
	saveOwnership(targetDir, ownership)

	// 8. Write portable project lock: facet.lock.json
	lockPath := filepath.Join(targetDir, "facet.lock.json")
	lock := ProjectLock{
		Version:  "1.0",
		Engine:   engine,
		Packs:    opts.Packs,
		LockedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if lockBytes, err := json.MarshalIndent(lock, "", "  "); err == nil {
		_ = os.WriteFile(lockPath, lockBytes, 0644)
	}

	// 9. Create project configuration: .facet.yaml
	configFilePath := filepath.Join(targetDir, ".facet.yaml")
	projectCfg := *cfg
	projectCfg.Project = projectSlug
	projectCfg.Defaults.Engine = engine
	if opts.Resolution != "" {
		projectCfg.Defaults.Resolution = opts.Resolution
	}
	if opts.FPS > 0 {
		projectCfg.Defaults.FPS = opts.FPS
	}
	if opts.AspectRatio != "" {
		projectCfg.Defaults.AspectRatio = opts.AspectRatio
	}
	if opts.Voice != "" {
		projectCfg.Defaults.Voice = opts.Voice
	}

	if err := projectCfg.Save(configFilePath); err != nil {
		return nil, fmt.Errorf("failed to save project config at %s: %w", configFilePath, err)
	}
	result.ConfigFile = configFilePath

	if w != nil {
		fmt.Fprintf(w, "  Created project configuration: %s\n", configFilePath)
	}

	if w != nil {
		fmt.Fprintln(w, "Workspace initialization complete.")
	}

	return result, nil
}

func scaffoldAgentInstructions(targetDir, engine string, packs []string, ownership *OwnershipRecord) {
	var packLines strings.Builder
	if len(packs) == 0 {
		packLines.WriteString("- Core Source-Edit (No additional packs active)\n")
	} else {
		for _, p := range packs {
			packLines.WriteString(fmt.Sprintf("- `%s` (`%s/SKILL.md`)\n", p, filepath.ToSlash(getSkillsTargetPath(".", engine, p))))
		}
	}

	instructions := fmt.Sprintf(`# Facet Video Production Workspace

You are the **Facet Video Producer**. You autonomously create, assemble, and render finished videos directly inside this workspace.

## Guidance Precedence
Read the canonical core skill at `+"`%s/SKILL.md`"+` (workspace-relative), then only the active pack entry needed for the request. These installed Facet-owned files define the production contract; do not search for copied skill libraries or deleted pipeline guidance.

## Working Agreement
- The user's selected agent orchestrates; Facet is a stateless toolbox, not an autonomous workflow controller.
- Understand the request and supplied assets; briefly explain the plan, renderer, providers, and meaningful tradeoffs. Ask only consequential questions.
- Ask for explicit consent before paid generation or external publication. Unknown cost is not free; estimates do not verify credentials or perform generation.
- Preserve silent-video intent: narration, music, and captions are optional. Do not impose turn numbers or mandatory artifact stages.
- Produce and review the requested video here. Never substitute mock media in production; `+"`mock:true`"+` is only for explicitly requested tests.

## Active Capability Packs
%s
## Tool Commands
Use `+"`facet tools describe <tool>`"+` for schemas and `+"`facet tools estimate <tool> --input request.json`"+` before consequential work. JSON files avoid shell quoting differences.
- Optional narration: `+"`"+`facet tools run edge_tts --input '{"text":"Hello","output_path":"narration/voice.mp3"}'`+"`"+`
- Inspect: `+"`"+`facet tools run media_probe --input '{"input":"assets/source.mp4"}'`+"`"+` (also accepts input_path, not file_path).
- Sample: `+"`"+`facet tools run frame_sample --input '{"input":"renders/final.mp4","output_dir":"artifacts/frames","strategy":{"type":"uniform","count":4}}'`+"`"+`
- Render: `+"`facet tools run video_compose --input artifacts/compose.json`"+`
- Review: `+"`"+`facet tools run output_review --input '{"rendered_file":"renders/final.mp4"}'`+"`"+`; configure expected profile/audio for the brief and visually inspect samples.

## Renderer And Provider Contract
- Direct Remotion props use a nonempty, ordered cuts array with exactly four scene primitives: `+"`text_card`"+` requires `+"`text`"+`; `+"`hero_title`"+` requires `+"`text`"+`; `+"`stat_card`"+` requires `+"`stat`"+`; `+"`media`"+` requires `+"`source`"+` and `+"`media_kind`"+` (`+"`image`"+` or `+"`video`"+`).
- Every cut requires `+"`type`"+`, `+"`in_seconds`"+`, and `+"`out_seconds`"+`; `+"`id`"+` is optional, and `+"`source`"+` is required only for `+"`media`"+` cuts. Set width, height, fps, and duration_seconds for an explicit export profile; cuts must fit the duration and whole frames.
- Defaults remain 1920x1080/30fps. Omit `+"`duration_seconds`"+` to end exactly at the last cut, with no padding. Omit audio for silence; narration uses `+"`audio.narration.src`"+`.
- Set output to renders/final.mp4; direct cuts select Remotion regardless of operation. Active production-method packs may provide complete request examples. An estimate is not proof the renderer or media works.
- Use `+"`gflow_image`"+` or `+"`gflow_video`"+`, never a generic gflow tool. Both need the gflow binary on PATH and authenticated provider access; configured only checks the binary.
- Real gflow estimates have null estimated_cost (unknown). Explain provider/model and obtain paid consent; missing dependencies or credentials are errors, not permission to use mocks.
- Read returned output/outputs paths, warnings, and review evidence; deliver the verified file with concise provenance and limitations.
`, filepath.ToSlash(getSkillsTargetPath(".", engine, "facet")), packLines.String())

	selected := instructionPathForEngine(engine)
	for _, rel := range []string{"CLAUDE.md", "AGENTS.md", ".github/copilot-instructions.md"} {
		if rel == selected || ownership == nil {
			continue
		}
		if entry, ok := ownership.ManagedEntries[rel]; ok && entry.EntryType == "instruction-file" {
			_ = os.Remove(filepath.Join(targetDir, filepath.FromSlash(rel)))
			delete(ownership.ManagedEntries, rel)
		}
	}
	if selected == "" {
		return
	}
	path := filepath.Join(targetDir, filepath.FromSlash(selected))
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	writeInstructionFileSafe(path, selected, instructions, ownership)
}

func instructionPathForEngine(engine string) string {
	switch engine {
	case "claude":
		return "CLAUDE.md"
	case "copilot", "github":
		return ".github/copilot-instructions.md"
	case "codex", "opencode", "studio":
		return "AGENTS.md"
	default:
		return ""
	}
}

func writeInstructionFileSafe(filePath, relKey, content string, ownership *OwnershipRecord) {
	// If file exists, check if it's managed by Facet
	if _, err := os.Stat(filePath); err == nil {
		if ownership != nil {
			if _, isManaged := ownership.ManagedEntries[relKey]; !isManaged {
				// User owned or repo-root owned, do not overwrite!
				return
			}
		} else {
			return
		}
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err == nil {
		if ownership != nil {
			ownership.ManagedEntries[relKey] = ManagedEntry{
				EntryType: "instruction-file",
				Target:    filePath,
				CreatedOn: time.Now().UTC().Format(time.RFC3339),
			}
		}
	}
}

// findCoreSkillSource locates the canonical facet producer skill.
func findCoreSkillSource(cfg *Config) string {
	candidates := []string{}
	if cfg != nil && cfg.Paths.Bundle != "" {
		candidates = append(candidates,
			filepath.Join(cfg.Paths.Bundle, "skills", "facet"),
			filepath.Join(cfg.Paths.Bundle, ".claude", "skills", "facet"),
		)
	}

	for _, root := range bundleCandidates() {
		candidates = append(candidates, filepath.Join(root, "skills", "facet"), filepath.Join(root, ".claude", "skills", "facet"))
	}

	// Check local app data
	if appData := os.Getenv("LOCALAPPDATA"); appData != "" {
		candidates = append(candidates,
			filepath.Join(appData, "Facet", "core", "current", "skills", "facet"),
		)
	}

	for _, cand := range candidates {
		skillFile := filepath.Join(cand, "SKILL.md")
		if fi, err := os.Stat(skillFile); err == nil && !fi.IsDir() {
			if abs, err := filepath.Abs(cand); err == nil {
				return abs
			}
		}
	}

	// Fallback to older skills directory if present
	return findSkillsSource(cfg)
}

func findSkillsSource(cfg *Config) string {
	candidates := []string{}
	if cfg != nil && cfg.Paths.Bundle != "" {
		candidates = append(candidates, filepath.Join(cfg.Paths.Bundle, "skills"))
	}
	for _, root := range bundleCandidates() {
		candidates = append(candidates, filepath.Join(root, "skills"))
	}

	for _, cand := range candidates {
		if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
			if abs, err := filepath.Abs(cand); err == nil {
				return abs
			}
		}
	}
	return ""
}

// findPackSource locates an installed or bundled pack directory.
func findPackSource(packName string, cfg *Config) string {
	// Normalize pack name (e.g. "@xibodev/facet-pack-explainer" -> "explainer")
	shortName := packName
	if idx := strings.LastIndex(packName, "/"); idx >= 0 {
		shortName = packName[idx+1:]
	}
	shortName = strings.TrimPrefix(shortName, "facet-pack-")

	candidates := []string{}
	if cfg != nil && cfg.Paths.Bundle != "" {
		candidates = append(candidates, filepath.Join(cfg.Paths.Bundle, "packs", shortName))
	}
	for _, root := range bundleCandidates() {
		candidates = append(candidates, filepath.Join(root, "packs", shortName))
	}

	if appData := os.Getenv("LOCALAPPDATA"); appData != "" {
		candidates = append(candidates,
			filepath.Join(appData, "Facet", "packs", shortName),
			filepath.Join(appData, "Facet", "packs", packName),
		)
	}

	for _, cand := range candidates {
		if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
			if abs, err := filepath.Abs(cand); err == nil {
				return abs
			}
		}
	}
	return ""
}

func getSkillsTargetPath(targetDir, engine, skillName string) string {
	if skillName == "" {
		skillName = "facet"
	}
	switch engine {
	case "studio":
		return filepath.Join(targetDir, "skills", skillName)
	case "claude":
		return filepath.Join(targetDir, ".claude", "skills", skillName)
	case "opencode":
		return filepath.Join(targetDir, ".opencode", "skills", skillName)
	case "copilot", "github":
		return filepath.Join(targetDir, ".github", "skills", skillName)
	case "codex":
		return filepath.Join(targetDir, ".codex", "skills", skillName)
	default:
		return filepath.Join(targetDir, ".claude", "skills", skillName)
	}
}

func linkOrCopySkillSafe(sourceDir, targetDir, projectDir string, ownership *OwnershipRecord) (string, error) {
	parentDir := filepath.Dir(targetDir)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create parent dir %s: %w", parentDir, err)
	}

	relTarget, err := filepath.Rel(projectDir, targetDir)
	if err != nil {
		relTarget = targetDir
	}

	// Check if target already exists
	if _, err := os.Lstat(targetDir); err == nil {
		// Verify ownership before removing
		if ownership != nil {
			if _, isManaged := ownership.ManagedEntries[relTarget]; !isManaged {
				// If not tracked by Facet, do NOT remove it. It belongs to the user.
				return "", fmt.Errorf("path %s already exists and is not managed by Facet (skipping to avoid overwrite)", targetDir)
			}
		}
		// Safe unlink
		_ = safeRemoveLinkOrDir(targetDir)
	}

	// Try Windows Directory Junction on Windows
	if runtime.GOOS == "windows" {
		winSrc := filepath.FromSlash(sourceDir)
		winDst := filepath.FromSlash(targetDir)
		cmd := exec.Command("cmd.exe", "/c", "mklink", "/J", winDst, winSrc)
		if err := cmd.Run(); err == nil {
			if ownership != nil {
				ownership.ManagedEntries[relTarget] = ManagedEntry{
					EntryType: "directory-junction",
					Target:    sourceDir,
					CreatedOn: time.Now().UTC().Format(time.RFC3339),
				}
			}
			return "junction", nil
		}
	}

	// Try symlink
	if err := os.Symlink(sourceDir, targetDir); err == nil {
		if ownership != nil {
			ownership.ManagedEntries[relTarget] = ManagedEntry{
				EntryType: "symlink",
				Target:    sourceDir,
				CreatedOn: time.Now().UTC().Format(time.RFC3339),
			}
		}
		return "symlink", nil
	}

	// Fallback to recursive copy
	if err := copyDirectory(sourceDir, targetDir); err != nil {
		return "", fmt.Errorf("directory copy fallback failed: %w", err)
	}

	if ownership != nil {
		ownership.ManagedEntries[relTarget] = ManagedEntry{
			EntryType: "copy",
			Target:    sourceDir,
			CreatedOn: time.Now().UTC().Format(time.RFC3339),
		}
	}

	return "copy", nil
}

func safeRemoveLinkOrDir(targetDir string) error {
	fi, err := os.Lstat(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Try simple Remove first (works for symlinks and modern Windows junctions)
	if err := os.Remove(targetDir); err == nil {
		return nil
	}

	// On Windows, if it's a junction, rmdir safely unlinks without touching target contents
	if runtime.GOOS == "windows" {
		winDst := filepath.FromSlash(targetDir)
		cmd := exec.Command("cmd.exe", "/c", "rmdir", winDst)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// If it's a regular directory copied previously
	if fi.IsDir() {
		return os.RemoveAll(targetDir)
	}

	return os.Remove(targetDir)
}

func loadOwnership(projectDir string) *OwnershipRecord {
	ownPath := filepath.Join(projectDir, ".facet", "ownership.json")
	record := &OwnershipRecord{
		Schema:         "facet.ownership/v1",
		InstallationID: "local",
		FacetVersion:   "1.0.1",
		ManagedEntries: make(map[string]ManagedEntry),
	}

	if data, err := os.ReadFile(ownPath); err == nil {
		_ = json.Unmarshal(data, record)
		if record.ManagedEntries == nil {
			record.ManagedEntries = make(map[string]ManagedEntry)
		}
	}
	return record
}

func saveOwnership(projectDir string, record *OwnershipRecord) {
	if record == nil {
		return
	}
	ownPath := filepath.Join(projectDir, ".facet", "ownership.json")
	if data, err := json.MarshalIndent(record, "", "  "); err == nil {
		_ = os.WriteFile(ownPath, data, 0644)
	}
}

func ensureGitExclude(targetDir string) {
	// Look for .git in targetDir or parents
	curr := targetDir
	for {
		gitDir := filepath.Join(curr, ".git")
		if fi, err := os.Stat(gitDir); err == nil && fi.IsDir() {
			infoDir := filepath.Join(gitDir, "info")
			_ = os.MkdirAll(infoDir, 0755)
			excludePath := filepath.Join(infoDir, "exclude")
			content, _ := os.ReadFile(excludePath)
			if !strings.Contains(string(content), ".facet/") {
				f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err == nil {
					defer f.Close()
					_, _ = f.WriteString("\n# Facet local state\n.facet/\n")
				}
			}
			return
		}
		parent := filepath.Dir(curr)
		if parent == curr || parent == "." {
			break
		}
		curr = parent
	}
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
