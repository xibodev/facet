package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
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
	EntryType      string `json:"entry_type"` // "directory-junction", "symlink", "copy", "instruction-section"
	Target         string `json:"target"`
	ContentSHA256  string `json:"content_sha256,omitempty"`
	AddedSeparator bool   `json:"added_separator,omitempty"`
	CreatedOn      string `json:"created_on"`
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

	desiredProjections := make(map[string]bool, len(opts.Packs)+1)
	desiredProjections[projectionKey(targetDir, getSkillsTargetPath(targetDir, engine, "facet"))] = true
	for _, packName := range opts.Packs {
		desiredProjections[projectionKey(targetDir, getSkillsTargetPath(targetDir, engine, packName))] = true
	}
	obsoleteProjections, err := planObsoleteProjections(targetDir, ownership, desiredProjections)
	if err != nil {
		return nil, err
	}

	// 4. Link core producer skill
	coreSource := findCoreSkillSource(cfg)
	coreTarget := getSkillsTargetPath(targetDir, engine, "facet")
	result.SkillsPath = coreTarget

	if coreSource != "" {
		method, err := linkOrCopySkillSafe(coreSource, coreTarget, targetDir, ownership)
		if err != nil {
			return nil, fmt.Errorf("reconcile core skill projection: %w", err)
		}
		result.LinkMethod = method
		result.Projections[coreTarget] = method
		if w != nil {
			fmt.Fprintf(w, "  Linked core skill: %s -> %s (%s)\n", coreTarget, coreSource, method)
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
			return nil, fmt.Errorf("reconcile pack %s projection: %w", packName, err)
		}
		result.Projections[packTarget] = method
		if w != nil {
			fmt.Fprintf(w, "  Linked pack '%s': %s -> %s (%s)\n", packName, packTarget, packSource, method)
		}
	}
	if err := removeObsoleteProjections(targetDir, ownership, obsoleteProjections); err != nil {
		return nil, err
	}

	// 6. Scaffold agent instruction files (CLAUDE.md, AGENTS.md, copilot-instructions.md)
	if err := scaffoldAgentInstructions(targetDir, engine, opts.Packs, ownership); err != nil {
		return nil, err
	}

	// 7. Save ownership record
	if err := saveOwnership(targetDir, ownership); err != nil {
		return nil, err
	}

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

const (
	facetInstructionStart = "<!-- facet:managed:start -->"
	facetInstructionEnd   = "<!-- facet:managed:end -->"
)

var (
	facetInstructionStartPattern = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(facetInstructionStart) + `\r?$`)
	facetInstructionEndPattern   = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(facetInstructionEnd) + `\r?$`)
	facetInstructionPattern      = regexp.MustCompile(`(?ms)^` + regexp.QuoteMeta(facetInstructionStart) + `\r?\n.*?^` + regexp.QuoteMeta(facetInstructionEnd) + `\r?$`)
)

func scaffoldAgentInstructions(targetDir, engine string, packs []string, ownership *OwnershipRecord) error {
	var packLines strings.Builder
	if len(packs) == 0 {
		packLines.WriteString("- Core Source-Edit (No additional packs active)\n")
	} else {
		for _, p := range packs {
			packLines.WriteString(fmt.Sprintf("- `%s` (`%s/SKILL.md`)\n", p, filepath.ToSlash(getSkillsTargetPath(".", engine, p))))
		}
	}

	instructions := fmt.Sprintf(`%s
# Facet Video Production Workspace

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
%s
`, facetInstructionStart, filepath.ToSlash(getSkillsTargetPath(".", engine, "facet")), packLines.String(), facetInstructionEnd)
	instructions = strings.TrimRight(instructions, "\r\n")

	selected := instructionPathForEngine(engine)
	type pendingInstruction struct {
		rel     string
		path    string
		content string
		entry   ManagedEntry
	}
	var removals []pendingInstruction
	for _, rel := range []string{"CLAUDE.md", "AGENTS.md", ".github/copilot-instructions.md"} {
		if rel == selected || ownership == nil {
			continue
		}
		entry, ok := ownership.ManagedEntries[rel]
		if !ok || entry.EntryType != "instruction-section" {
			continue
		}
		path := filepath.Join(targetDir, filepath.FromSlash(rel))
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("managed Facet instruction section in %s is missing: %w", rel, err)
		}
		start, end, section, found, err := findFacetInstructionSection(string(content))
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		if !found || instructionSectionHash(section) != entry.ContentSHA256 {
			return fmt.Errorf("preserving modified Facet instruction section in %s", rel)
		}
		remaining := removeFacetInstructionSection(string(content), start, end, entry.AddedSeparator)
		removals = append(removals, pendingInstruction{rel: rel, path: path, content: remaining})
	}
	if selected == "" {
		return nil
	}
	selectedPath := filepath.Join(targetDir, filepath.FromSlash(selected))
	selectedContent, err := os.ReadFile(selectedPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read governing instructions %s: %w", selected, err)
	}
	existing := string(selectedContent)
	start, end, currentSection, found, err := findFacetInstructionSection(existing)
	if err != nil {
		return fmt.Errorf("%s: %w", selected, err)
	}
	entry, owned := ownership.ManagedEntries[selected]
	addedSeparator := false
	merged := instructions + "\n"
	if found {
		if !owned || entry.EntryType != "instruction-section" {
			return fmt.Errorf("preserving unmanaged Facet instruction section in %s", selected)
		}
		if instructionSectionHash(currentSection) != entry.ContentSHA256 {
			return fmt.Errorf("preserving modified Facet instruction section in %s", selected)
		}
		addedSeparator = entry.AddedSeparator
		merged = existing[:start] + instructions + existing[end:]
	} else if owned && entry.EntryType == "instruction-section" {
		return fmt.Errorf("managed Facet instruction section in %s is missing", selected)
	} else if existing != "" {
		separator := ""
		if !strings.HasSuffix(existing, "\n") {
			separator = "\n"
			addedSeparator = true
		}
		merged = existing + separator + instructions + "\n"
	}

	for _, removal := range removals {
		if err := os.WriteFile(removal.path, []byte(removal.content), 0644); err != nil {
			return fmt.Errorf("remove prior Facet instruction section from %s: %w", removal.rel, err)
		}
		delete(ownership.ManagedEntries, removal.rel)
	}
	if err := os.MkdirAll(filepath.Dir(selectedPath), 0755); err != nil {
		return fmt.Errorf("create governing instruction directory: %w", err)
	}
	if err := os.WriteFile(selectedPath, []byte(merged), 0644); err != nil {
		return fmt.Errorf("write governing instructions %s: %w", selected, err)
	}
	ownership.ManagedEntries[selected] = ManagedEntry{
		EntryType:      "instruction-section",
		Target:         selectedPath,
		ContentSHA256:  instructionSectionHash(instructions),
		AddedSeparator: addedSeparator,
		CreatedOn:      time.Now().UTC().Format(time.RFC3339),
	}
	return nil
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

func findFacetInstructionSection(content string) (start, end int, section string, found bool, err error) {
	starts := facetInstructionStartPattern.FindAllStringIndex(content, -1)
	ends := facetInstructionEndPattern.FindAllStringIndex(content, -1)
	if len(starts) == 0 && len(ends) == 0 {
		return 0, 0, "", false, nil
	}
	if len(starts) != 1 || len(ends) != 1 {
		return 0, 0, "", false, fmt.Errorf("malformed Facet instruction section")
	}
	loc := facetInstructionPattern.FindStringIndex(content)
	if loc == nil {
		return 0, 0, "", false, fmt.Errorf("malformed Facet instruction section")
	}
	return loc[0], loc[1], content[loc[0]:loc[1]], true, nil
}

func instructionSectionHash(content string) string {
	content = strings.TrimRight(strings.ReplaceAll(content, "\r\n", "\n"), "\r\n")
	return fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
}

func removeInstructionSeparator(prefix string) string {
	if strings.HasSuffix(prefix, "\r\n") {
		return strings.TrimSuffix(prefix, "\r\n")
	}
	return strings.TrimSuffix(prefix, "\n")
}

func removeFacetInstructionSection(content string, start, end int, addedSeparator bool) string {
	prefix := content[:start]
	if addedSeparator {
		prefix = removeInstructionSeparator(prefix)
	}
	suffix := content[end:]
	if strings.HasPrefix(suffix, "\r\n") {
		suffix = strings.TrimPrefix(suffix, "\r\n")
	} else {
		suffix = strings.TrimPrefix(suffix, "\n")
	}
	return prefix + suffix
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
		return filepath.Join(targetDir, ".agents", "skills", skillName)
	default:
		return filepath.Join(targetDir, ".claude", "skills", skillName)
	}
}

func linkOrCopySkillSafe(sourceDir, targetDir, projectDir string, ownership *OwnershipRecord) (string, error) {
	parentDir := filepath.Dir(targetDir)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create parent dir %s: %w", parentDir, err)
	}

	relTarget := projectionKey(projectDir, targetDir)

	// Check if target already exists
	if _, err := os.Lstat(targetDir); err == nil {
		if ownership == nil {
			return "", fmt.Errorf("preserving existing unmanaged projection %s", targetDir)
		}
		entry, isManaged := ownership.ManagedEntries[relTarget]
		if !isManaged || !isProjectionEntry(entry) {
			return "", fmt.Errorf("preserving existing unmanaged projection %s", targetDir)
		}
		if err := removeManagedProjection(targetDir, entry); err != nil {
			return "", err
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect projection %s: %w", targetDir, err)
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
		treeHash, err := hashProjectionTree(targetDir)
		if err != nil {
			return "", fmt.Errorf("hash copied projection %s: %w", targetDir, err)
		}
		ownership.ManagedEntries[relTarget] = ManagedEntry{
			EntryType:     "copy",
			Target:        sourceDir,
			ContentSHA256: treeHash,
			CreatedOn:     time.Now().UTC().Format(time.RFC3339),
		}
	}

	return "copy", nil
}

func projectionKey(projectDir, targetDir string) string {
	relTarget, err := filepath.Rel(projectDir, targetDir)
	if err != nil {
		return filepath.Clean(targetDir)
	}
	return filepath.Clean(relTarget)
}

func isProjectionEntry(entry ManagedEntry) bool {
	switch entry.EntryType {
	case "directory-junction", "symlink", "copy":
		return true
	default:
		return false
	}
}

func planObsoleteProjections(projectDir string, ownership *OwnershipRecord, desired map[string]bool) ([]string, error) {
	var obsolete []string
	for key, entry := range ownership.ManagedEntries {
		cleanKey := filepath.Clean(key)
		if !isProjectionKey(cleanKey) || desired[cleanKey] {
			continue
		}
		if !isProjectionEntry(entry) {
			return nil, fmt.Errorf("preserving obsolete projection %s: unexpected ownership type %q", key, entry.EntryType)
		}
		path := filepath.Join(projectDir, key)
		if err := verifyManagedProjection(path, entry); err != nil {
			return nil, fmt.Errorf("preserving obsolete projection %s: %w", key, err)
		}
		obsolete = append(obsolete, key)
	}
	sort.Slice(obsolete, func(i, j int) bool { return len(obsolete[i]) > len(obsolete[j]) })
	return obsolete, nil
}

func isProjectionKey(key string) bool {
	slash := filepath.ToSlash(filepath.Clean(key))
	for _, prefix := range []string{"skills/", ".claude/skills/", ".opencode/skills/", ".github/skills/", ".agents/skills/"} {
		if strings.HasPrefix(slash, prefix) && strings.TrimPrefix(slash, prefix) != "" {
			return true
		}
	}
	return false
}

func removeObsoleteProjections(projectDir string, ownership *OwnershipRecord, obsolete []string) error {
	for _, key := range obsolete {
		entry := ownership.ManagedEntries[key]
		if err := removeManagedProjection(filepath.Join(projectDir, key), entry); err != nil {
			return fmt.Errorf("remove obsolete projection %s: %w", key, err)
		}
		delete(ownership.ManagedEntries, key)
	}
	return nil
}

func verifyManagedProjection(targetDir string, entry ManagedEntry) error {
	info, err := os.Lstat(targetDir)
	if err != nil {
		return fmt.Errorf("managed projection is missing: %w", err)
	}
	switch entry.EntryType {
	case "symlink":
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("managed symlink was replaced by %s", info.Mode().Type())
		}
		actual, err := os.Readlink(targetDir)
		if err != nil {
			return fmt.Errorf("read managed symlink target: %w", err)
		}
		if !filepath.IsAbs(actual) {
			actual = filepath.Join(filepath.Dir(targetDir), actual)
		}
		match, err := sameProjectionTarget(actual, entry.Target)
		if err != nil {
			return err
		}
		if !match {
			return fmt.Errorf("managed symlink target changed from %s to %s", entry.Target, actual)
		}
	case "directory-junction":
		if runtime.GOOS != "windows" {
			return fmt.Errorf("directory junction ownership is unsupported on %s", runtime.GOOS)
		}
		if info.Mode()&os.ModeIrregular == 0 {
			return fmt.Errorf("managed directory junction was replaced")
		}
		actual, err := os.Readlink(targetDir)
		if err != nil {
			return fmt.Errorf("read managed directory junction target: %w", err)
		}
		match, err := sameProjectionTarget(actual, entry.Target)
		if err != nil {
			return err
		}
		if !match {
			return fmt.Errorf("managed directory junction target changed from %s to %s", entry.Target, actual)
		}
	case "copy":
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("managed copied projection was replaced")
		}
		if entry.ContentSHA256 == "" {
			return fmt.Errorf("managed copied projection has no content hash")
		}
		actual, err := hashProjectionTree(targetDir)
		if err != nil {
			return err
		}
		if actual != entry.ContentSHA256 {
			return fmt.Errorf("modified copied projection: content hash changed")
		}
	default:
		return fmt.Errorf("unexpected managed projection type %q", entry.EntryType)
	}
	return nil
}

func sameProjectionTarget(actual, expected string) (bool, error) {
	actualAbs, err := filepath.Abs(actual)
	if err != nil {
		return false, fmt.Errorf("resolve projection target %s: %w", actual, err)
	}
	expectedAbs, err := filepath.Abs(expected)
	if err != nil {
		return false, fmt.Errorf("resolve owned projection target %s: %w", expected, err)
	}
	actualAbs = filepath.Clean(actualAbs)
	expectedAbs = filepath.Clean(expectedAbs)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(actualAbs, expectedAbs), nil
	}
	return actualAbs == expectedAbs, nil
}

func removeManagedProjection(targetDir string, entry ManagedEntry) error {
	if err := verifyManagedProjection(targetDir, entry); err != nil {
		return fmt.Errorf("preserving managed projection %s: %w", targetDir, err)
	}
	switch entry.EntryType {
	case "copy":
		if err := os.RemoveAll(targetDir); err != nil {
			return fmt.Errorf("remove verified copied projection %s: %w", targetDir, err)
		}
	case "symlink":
		if err := os.Remove(targetDir); err != nil {
			return fmt.Errorf("unlink verified projection %s: %w", targetDir, err)
		}
	case "directory-junction":
		if err := os.Remove(targetDir); err == nil {
			return nil
		}
		cmd := exec.Command("cmd.exe", "/c", "rmdir", filepath.FromSlash(targetDir))
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("unlink verified directory junction %s: %w: %s", targetDir, err, strings.TrimSpace(string(output)))
		}
	}
	return nil
}

func hashProjectionTree(root string) (string, error) {
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("copied projection contains symlink %s", filepath.ToSlash(rel))
		}
		kind := byte('f')
		if entry.IsDir() {
			kind = 'd'
		}
		_, _ = fmt.Fprintf(hash, "%c\x00%s\x00", kind, filepath.ToSlash(rel))
		if entry.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
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

func saveOwnership(projectDir string, record *OwnershipRecord) error {
	if record == nil {
		return nil
	}
	ownPath := filepath.Join(projectDir, ".facet", "ownership.json")
	if err := os.MkdirAll(filepath.Dir(ownPath), 0755); err != nil {
		return fmt.Errorf("create Facet ownership directory: %w", err)
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Facet ownership record: %w", err)
	}
	if err := os.WriteFile(ownPath, data, 0644); err != nil {
		return fmt.Errorf("write Facet ownership record: %w", err)
	}
	return nil
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
