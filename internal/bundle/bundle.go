// Package bundle projects Facet's canonical guidance into the native layouts
// of agentic CLIs.
//
// The target-adapter rule:
//
//	canonical Facet asset  ->  target adapter  ->  target-native file
//
// Different packaging is allowed. Different product semantics are not: every
// target receives the same core skill and the same seven pack skills, byte for
// byte, and the creative persona wherever the target's agent format has been
// validated. The canonical assets are the ones compiled into the binary
// (facet.Assets), so a projection never depends on the working directory.
//
// This package is the only place that knows a target's folder conventions.
// Registering the MCP server in a person's own configuration is wiring, not
// projection, and lives in internal/wire. A plugin is the exception: its
// format carries the MCP registration, so the plugin layout (plugin.go)
// writes one, as a static file that runs the facet command from PATH.
package bundle

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	facet "github.com/xibodev/facet"
)

const (
	// CapabilityID identifies the product a bundle or wiring belongs to.
	CapabilityID = "xibodev.facet"
	// MCPServerName is the name every target registers Facet's MCP server under.
	MCPServerName = "facet"
	// CodexToolTimeoutSec is the MCP call limit Facet sets in Codex, through
	// facet wire and the Codex plugin. Codex stops an MCP call after the
	// server's tool_timeout_sec, 60 seconds unless configured, which a render
	// often needs more than. Longer work belongs on the shell route (facet
	// tools run), where no MCP limit applies.
	CodexToolTimeoutSec = 600
	// CoreSkillName is the core production-contract skill.
	CoreSkillName = "facet"
	// PersonaName is the creative persona agent.
	PersonaName = "facet-creative"
	// AdapterVersion versions the projection logic separately from the Facet
	// version: a layout fix changes it without changing product truth.
	AdapterVersion = "2"

	coreSkillSource = "skills/facet"
	personaSource   = "agents/facet-creative.md"
)

// MCPServerArgs returns the arguments that start Facet's MCP server when
// appended to the facet executable.
func MCPServerArgs() []string { return []string{"mcp"} }

// Target is an agentic CLI Facet can be projected into.
type Target string

const (
	TargetClaude   Target = "claude"
	TargetCodex    Target = "codex"
	TargetCopilot  Target = "copilot"
	TargetOpenCode Target = "opencode"
	// TargetCompa is Compa's agent engine (compa-kernel). It is wired at
	// user scope only: its MCP servers and approval rules live in its own
	// configuration, not in a project.
	TargetCompa Target = "compa"
)

// Targets returns every supported target in a stable order.
func Targets() []Target {
	return []Target{TargetClaude, TargetCodex, TargetCopilot, TargetOpenCode, TargetCompa}
}

func (t Target) valid() bool {
	for _, known := range Targets() {
		if t == known {
			return true
		}
	}
	return false
}

// SupportsScope reports whether target t can be installed at scope s.
// Compa is wired at user scope only. Plugins exist for the CLIs whose plugin
// format carries skills and an MCP server: Claude Code, Codex and Copilot CLI.
func SupportsScope(t Target, s Scope) bool {
	switch s {
	case ScopePlugin:
		return t == TargetClaude || t == TargetCodex || t == TargetCopilot
	case ScopeProject:
		return t != TargetCompa
	}
	return true
}

// ParseTarget validates one target name.
func ParseTarget(name string) (Target, error) {
	t := Target(strings.ToLower(strings.TrimSpace(name)))
	if !t.valid() {
		return "", fmt.Errorf("unknown CLI %q; choose claude, codex, copilot, opencode, compa, or all", name)
	}
	return t, nil
}

// ParseTargets parses a comma-separated target list. "all" selects every
// target. Duplicates are removed and the result keeps the order of Targets().
func ParseTargets(list ...string) ([]Target, error) {
	want := map[Target]bool{}
	for _, item := range list {
		for _, name := range strings.Split(item, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if strings.EqualFold(name, "all") {
				for _, t := range Targets() {
					want[t] = true
				}
				continue
			}
			t, err := ParseTarget(name)
			if err != nil {
				return nil, err
			}
			want[t] = true
		}
	}
	if len(want) == 0 {
		return nil, fmt.Errorf("no CLI named; choose claude, codex, copilot, opencode, compa, or all")
	}
	var out []Target
	for _, t := range Targets() {
		if want[t] {
			out = append(out, t)
		}
	}
	return out, nil
}

// Scope selects where a target discovers assets: the user's own
// configuration, one project, or a plugin that a CLI installs from a
// marketplace.
type Scope string

const (
	ScopeUser    Scope = "user"
	ScopeProject Scope = "project"
	// ScopePlugin lays a target out as a plugin directory: the guidance
	// plus the plugin manifest and MCP registration (see plugin.go). It is a
	// bundle layout only; facet wire installs at user or project scope.
	ScopePlugin Scope = "plugin"
)

// ParseScope validates an install scope: user or project.
func ParseScope(name string) (Scope, error) {
	switch s := Scope(strings.ToLower(strings.TrimSpace(name))); s {
	case ScopeUser, ScopeProject:
		return s, nil
	}
	return "", fmt.Errorf("unknown scope %q; choose user or project", name)
}

// ParseBundleScope validates a bundle layout: user, project, or plugin.
func ParseBundleScope(name string) (Scope, error) {
	if s := Scope(strings.ToLower(strings.TrimSpace(name))); s == ScopePlugin {
		return s, nil
	}
	if s, err := ParseScope(name); err == nil {
		return s, nil
	}
	return "", fmt.Errorf("unknown scope %q; choose user, project, or plugin", name)
}

func (s Scope) valid() bool { return s == ScopeUser || s == ScopeProject || s == ScopePlugin }

// Asset kinds recorded in manifests and wiring records.
const (
	KindSkill = "skill"
	KindPack  = "pack"
	KindAgent = "agent"
	// KindPlugin is a plugin manifest and KindMCP a plugin's MCP server
	// registration; only the plugin layout has them.
	KindPlugin = "plugin"
	KindMCP    = "mcp"
)

// File is one target-native file. Path is slash-separated and relative to
// the target's install root (see Root).
type File struct {
	Path    string
	Kind    string
	Name    string // skill or agent name
	Content []byte
}

// Digest returns the file's content digest.
func (f File) Digest() string { return Digest(f.Content) }

// Digest returns "sha256:<hex>" for content.
func Digest(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// PackSkillName is the skill name a pack is installed under. Packs are
// namespaced so that installing them user-wide never collides with a user's
// own generically named skills; a route's `pack` value maps to this name.
func PackSkillName(pack string) string { return "facet-" + pack }

// DefaultRoot returns a target's install root relative to the scope's base
// directory (the home directory for user scope, the project directory for
// project scope), ignoring environment overrides. A plugin's files sit at the
// plugin's own root, so plugin scope returns "".
func DefaultRoot(t Target, s Scope) string {
	if s == ScopePlugin {
		return ""
	}
	if s == ScopeProject {
		switch t {
		case TargetClaude:
			return ".claude"
		case TargetCodex:
			// Codex discovers repository skills in .agents/skills.
			return ".agents"
		case TargetCopilot:
			return ".github"
		case TargetOpenCode:
			return ".opencode"
		}
		return ""
	}
	switch t {
	case TargetClaude:
		return ".claude"
	case TargetCodex:
		// Codex's own skill installer and skill creator write user skills to
		// $CODEX_HOME/skills (default ~/.codex/skills), where Codex also keeps
		// its system skills.
		return ".codex"
	case TargetCopilot:
		// `copilot skill --help`: personal skills live in ~/.copilot/skills.
		return ".copilot"
	case TargetOpenCode:
		return ".config/opencode"
	case TargetCompa:
		// Compa reads skills from its workspace's skills/ folder; the agent
		// may only read files inside the workspace, so the global
		// $COMPA_HOME/skills folder would list skills it cannot open. The
		// default workspace is ~/.compa/workspace; facet wire reads the
		// configured one from Compa's config.json.
		return ".compa/workspace"
	}
	return ""
}

// Root returns the absolute install root of target t at scope s. base is the
// home directory for user scope and the project directory for project scope.
// User scope honours each CLI's documented configuration-directory override.
func Root(t Target, s Scope, base string, getenv func(string) string) string {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	if s == ScopeUser {
		switch t {
		case TargetClaude:
			if dir := strings.TrimSpace(getenv("CLAUDE_CONFIG_DIR")); dir != "" {
				return filepath.Clean(dir)
			}
		case TargetCodex:
			if dir := strings.TrimSpace(getenv("CODEX_HOME")); dir != "" {
				return filepath.Clean(dir)
			}
		case TargetOpenCode:
			if dir := strings.TrimSpace(getenv("XDG_CONFIG_HOME")); dir != "" {
				return filepath.Join(dir, "opencode")
			}
		case TargetCompa:
			if dir := strings.TrimSpace(getenv("COMPA_AGENTS_DEFAULTS_WORKSPACE")); dir != "" {
				return filepath.Clean(dir)
			}
			if dir := strings.TrimSpace(getenv("COMPA_HOME")); dir != "" {
				return filepath.Join(dir, "workspace")
			}
		}
	}
	return filepath.Join(base, filepath.FromSlash(DefaultRoot(t, s)))
}

// personaFile returns the persona's file name under <root>/agents and how its
// canonical bytes are adapted, or false when the target's agent format has
// not been validated and the persona is therefore not installed there.
func personaFile(t Target) (string, func([]byte) ([]byte, error), bool) {
	switch t {
	case TargetClaude:
		// Claude Code subagents: agents/<name>.md with name and description
		// frontmatter.
		return PersonaName + ".md", keepBytes, true
	case TargetCopilot:
		// Copilot CLI custom agents: agents/<name>.agent.md with description
		// (and optional name) frontmatter.
		return PersonaName + ".agent.md", keepBytes, true
	case TargetOpenCode:
		// OpenCode agents: agents/<name>.md. The file name is the agent name,
		// and frontmatter keys outside OpenCode's agent schema are passed to
		// the model provider as options, so `name` is dropped.
		return PersonaName + ".md", dropFrontmatterKey("name"), true
	}
	// Codex: no agent format validated for this release. Compa: an agent is
	// a whole workspace (AGENT.md), not a file beside the user's own agent.
	return "", nil, false
}

// HasPersona reports whether target t receives the creative persona agent.
func HasPersona(t Target) bool {
	_, _, ok := personaFile(t)
	return ok
}

// Files projects the canonical assets for target t: the core skill, one skill
// per retained pack, and the persona where supported. Paths are relative to
// the target's install root; the result is sorted by path.
func Files(t Target) ([]File, error) {
	if !t.valid() {
		return nil, fmt.Errorf("unsupported target %q", t)
	}
	var files []File
	err := fs.WalkDir(facet.Assets, coreSkillSource, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := facet.Assets.ReadFile(name)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(name, coreSkillSource+"/")
		files = append(files, File{Path: "skills/" + CoreSkillName + "/" + rel, Kind: KindSkill, Name: CoreSkillName, Content: normalizeText(name, data)})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading the core skill: %w", err)
	}
	for _, pack := range facet.RetainedPacks() {
		if pack.ID == "" || path.Base(pack.ID) != pack.ID {
			return nil, fmt.Errorf("invalid pack id %q", pack.ID)
		}
		name := PackSkillName(pack.ID)
		prefix := "packs/" + pack.ID + "/"
		for _, guidance := range pack.Guidance {
			if !strings.HasPrefix(guidance.Path, prefix) {
				return nil, fmt.Errorf("pack %s guidance %s is outside the pack", pack.ID, guidance.Path)
			}
			data, err := facet.Assets.ReadFile(guidance.Path)
			if err != nil {
				return nil, fmt.Errorf("reading pack %s: %w", pack.ID, err)
			}
			files = append(files, File{Path: "skills/" + name + "/" + strings.TrimPrefix(guidance.Path, prefix), Kind: KindPack, Name: name, Content: normalizeText(guidance.Path, data)})
		}
	}
	if file, render, ok := personaFile(t); ok {
		data, err := facet.Assets.ReadFile(personaSource)
		if err != nil {
			return nil, fmt.Errorf("reading the persona: %w", err)
		}
		content, err := render(normalizeText(personaSource, data))
		if err != nil {
			return nil, fmt.Errorf("adapting the persona for %s: %w", t, err)
		}
		files = append(files, File{Path: "agents/" + file, Kind: KindAgent, Name: PersonaName, Content: content})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	if err := validateFiles(t, files); err != nil {
		return nil, err
	}
	return files, nil
}

var skillNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// validateFiles enforces what every target's skill loader requires, so a
// canonical edit that would make a target silently drop a skill fails here.
func validateFiles(t Target, files []File) error {
	skills := map[string]bool{}
	for _, f := range files {
		if f.Kind == KindSkill || f.Kind == KindPack {
			skills[f.Name] = false
		}
	}
	for _, f := range files {
		switch {
		case (f.Kind == KindSkill || f.Kind == KindPack) && f.Path == "skills/"+f.Name+"/SKILL.md":
			skills[f.Name] = true
			fields, ok := Frontmatter(f.Content)
			if !ok {
				return fmt.Errorf("skill %s has no frontmatter", f.Name)
			}
			if fields["name"] != f.Name {
				return fmt.Errorf("skill %s declares name %q; the name must match its directory", f.Name, fields["name"])
			}
			if len(f.Name) > 64 || !skillNamePattern.MatchString(f.Name) {
				return fmt.Errorf("skill name %q is not a lowercase hyphenated name of at most 64 characters", f.Name)
			}
			if d := fields["description"]; d == "" || len(d) > 500 {
				return fmt.Errorf("skill %s needs a one-line description of 1 to 500 characters", f.Name)
			}
		case f.Kind == KindAgent:
			fields, ok := Frontmatter(f.Content)
			if !ok || fields["description"] == "" {
				return fmt.Errorf("%s persona needs a description", t)
			}
			if name, present := fields["name"]; present && name != PersonaName {
				return fmt.Errorf("%s persona declares name %q, want %q", t, name, PersonaName)
			}
		}
	}
	for name, found := range skills {
		if !found {
			return fmt.Errorf("skill %s has no SKILL.md", name)
		}
	}
	return nil
}

// Frontmatter returns the `key: value` fields of a Markdown document's YAML
// frontmatter. It supports the flat scalar form Facet's assets use.
func Frontmatter(doc []byte) (map[string]string, bool) {
	lines, _, ok := splitFrontmatter(bytes.ReplaceAll(doc, []byte("\r\n"), []byte("\n")))
	if !ok {
		return nil, false
	}
	fields := map[string]string{}
	for _, line := range lines {
		key, value, found := strings.Cut(line, ":")
		if !found || strings.TrimSpace(key) == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "#") {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' && value[len(value)-1] == '"' || value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
		fields[strings.TrimSpace(key)] = value
	}
	return fields, true
}

// splitFrontmatter returns the frontmatter lines (without delimiters) and the
// byte offset where the body starts. doc must use LF line endings.
func splitFrontmatter(doc []byte) ([]string, int, bool) {
	text := string(doc)
	if !strings.HasPrefix(text, "---\n") {
		return nil, 0, false
	}
	rest := text[4:]
	var block string
	var bodyStart int
	switch end := strings.Index(rest, "\n---\n"); {
	case strings.HasPrefix(rest, "---\n"):
		bodyStart = 8
	case rest == "---":
		bodyStart = len(text)
	case end >= 0:
		block, bodyStart = rest[:end], 4+end+5
	case strings.HasSuffix(rest, "\n---"):
		block, bodyStart = rest[:len(rest)-4], len(text)
	default:
		return nil, 0, false
	}
	if block == "" {
		return nil, bodyStart, true
	}
	return strings.Split(block, "\n"), bodyStart, true
}

func keepBytes(b []byte) ([]byte, error) { return b, nil }

// dropFrontmatterKey removes one top-level frontmatter key, keeping every
// other byte of the document.
func dropFrontmatterKey(key string) func([]byte) ([]byte, error) {
	return func(doc []byte) ([]byte, error) {
		lines, bodyStart, ok := splitFrontmatter(doc)
		if !ok {
			return nil, fmt.Errorf("no frontmatter")
		}
		var kept []string
		for _, line := range lines {
			k, _, found := strings.Cut(line, ":")
			if found && !strings.HasPrefix(line, " ") && strings.TrimSpace(k) == key {
				continue
			}
			kept = append(kept, line)
		}
		var out bytes.Buffer
		out.WriteString("---\n")
		for _, line := range kept {
			out.WriteString(line)
			out.WriteByte('\n')
		}
		out.WriteString("---\n")
		out.Write(doc[bodyStart:])
		return out.Bytes(), nil
	}
}

// normalizeText converts CRLF to LF in text assets. A Windows checkout may
// carry CRLF line endings, and a harness that splits frontmatter on "\n"
// would read `name: facet\r` and reject the skill.
func normalizeText(name string, data []byte) []byte {
	switch strings.ToLower(path.Ext(name)) {
	case ".md", ".json", ".txt", ".yaml", ".yml":
		return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	}
	return data
}
