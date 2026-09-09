// Package bundle builds Release C artifacts: installable, target-shaped Facet
// packages for external agentic CLIs.
//
// RELEASE C is the projection where the TARGET CLI is the reasoning driver.
// Facet ships the creative intelligence -- agents, skills, packs, instructions
// and tool wiring -- shaped to whatever that harness natively expects.
//
// The target-adapter rule, same as Operations:
//
//	canonical Facet asset  ->  target adapter  ->  target-shaped release asset
//
// Different packaging is allowed. Different product semantics are not. Four
// hand-maintained copies of one skill is the failure this prevents, and it is
// the duplicated-truth defect one layer up from a second effects table.
//
// A bundle is a DIRECTORY plus a manifest, not an opaque archive: an installer
// must be able to see what it is about to write before it writes it, and a
// human must be able to read what was installed afterwards.
package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ManifestSchema identifies the bundle manifest format.
//
// Versioned separately from Facet itself: an installer reads the manifest
// BEFORE it knows anything else about the bundle, so the manifest's own shape
// has to be the one thing it can rely on.
const ManifestSchema = "xibodev.facet.bundle/v1"

// AdapterVersion is the version of the PROJECTION LOGIC, distinct from the
// Facet version.
//
// These answer different questions. FacetVersion says which product truth was
// projected; AdapterVersion says how it was shaped for this target. A fix to a
// target's layout changes the second without changing the first, and an
// installer needs to tell those apart to decide whether a reinstall is
// warranted.
const AdapterVersion = "1"

// Target is one external agentic CLI Facet can be installed into.
type Target string

const (
	TargetClaude   Target = "claude"
	TargetOpenCode Target = "opencode"
	TargetCopilot  Target = "copilot"
	TargetCodex    Target = "codex"
)

// Targets returns every supported target, sorted so a build is reproducible.
func Targets() []Target {
	return []Target{TargetClaude, TargetCodex, TargetCopilot, TargetOpenCode}
}

// Entry is one file the bundle will install, with the digest of its content.
//
// The digest is over the PROJECTED bytes rather than the canonical source: what
// an installer verifies must be what it writes. A digest of the source would
// confirm the bundle was built from something intact without confirming that
// what lands on disk is what was measured.
type Entry struct {
	// Path is relative to the target's install root, forward-slashed so a
	// manifest built on Windows installs identically elsewhere.
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	Digest string `json:"digest"`
	// Kind records which canonical asset class this came from, so an installer
	// can explain what it is writing rather than listing opaque paths.
	Kind string `json:"kind"`
}

// Manifest is the bundle's identity, provenance and install plan.
type Manifest struct {
	Schema string `json:"schema"`

	// --- identity ---
	FacetVersion   string `json:"facet_version"`
	Target         Target `json:"target"`
	AdapterVersion string `json:"adapter_version"`

	// --- compatibility ---
	// Compatibility states what the TARGET must provide, so an installer can
	// refuse a bundle rather than write files that will not be discovered.
	Compatibility Compatibility `json:"compatibility"`

	// --- content ---
	Entries []Entry `json:"entries"`

	// BundleDigest covers every entry digest in sorted order. It answers "is
	// this the bundle that was built", which is a different question from any
	// single file being intact.
	BundleDigest string `json:"bundle_digest"`

	// Tools names the public vocabulary this bundle exposes. Recorded so an
	// installed bundle can be compared against the running Facet: a bundle
	// naming tools the binary does not have is a stale install, and that is
	// otherwise invisible.
	Tools []string `json:"tools"`
}

// Compatibility is what the target harness must supply for this bundle to work.
type Compatibility struct {
	// InstallRoot is where the target discovers assets, relative to a project
	// or a user home depending on Scope.
	InstallRoot string `json:"install_root"`
	// Scope is "project" or "user".
	Scope string `json:"scope"`
	// ToolTransport is how this target actually invokes Facet.
	//
	// DELIBERATELY NOT ALWAYS MCP. Facet exposes no MCP server today -- tools
	// are CLI-invoked -- so declaring MCP would describe a transport that does
	// not exist. Every listed target can shell out, so "cli" is the honest
	// answer now, and a target better served by MCP later gains a projection
	// rather than forcing a rewrite.
	ToolTransport string `json:"tool_transport"`
	// FacetBinary is the executable an agent must invoke.
	FacetBinary string `json:"facet_binary"`
}

// layoutFor returns the target-native install shape.
//
// This is the adapter. It is the ONLY place that knows a target's folder
// conventions, so adding a target is one case rather than a search through the
// codebase.
func layoutFor(t Target) Compatibility {
	base := Compatibility{
		Scope:         "project",
		ToolTransport: "cli",
		FacetBinary:   "facet",
	}
	switch t {
	case TargetClaude:
		base.InstallRoot = ".claude"
	case TargetOpenCode:
		base.InstallRoot = ".opencode"
	case TargetCopilot:
		base.InstallRoot = ".github"
	case TargetCodex:
		base.InstallRoot = ".codex"
	}
	return base
}

// Source is the canonical asset tree a bundle is projected from.
type Source struct {
	// SkillsDir and PacksDir are canonical product truth. The builder READS
	// them; it never rewrites their semantics.
	SkillsDir string
	PacksDir  string
	// Tools is the public vocabulary, supplied by the caller so the builder
	// never holds a second copy of it.
	Tools []string
	// FacetVersion is the product version being projected.
	FacetVersion string
}

// Build projects the canonical source into a target-shaped bundle on disk and
// returns its manifest.
//
// outDir is created if absent and must be empty or nonexistent: a builder that
// merges into a populated directory can produce a bundle whose manifest does
// not describe its contents.
func Build(src Source, t Target, outDir string) (*Manifest, error) {
	if len(src.Tools) == 0 {
		// A bundle with no tools would install guidance for a product the
		// agent cannot invoke. Refuse rather than ship a decorative bundle.
		return nil, fmt.Errorf("no tools supplied; a bundle must name the vocabulary it exposes")
	}
	if src.FacetVersion == "" {
		return nil, fmt.Errorf("no facet version supplied; a bundle without identity cannot be upgraded or compared")
	}

	compat := layoutFor(t)
	if compat.InstallRoot == "" {
		return nil, fmt.Errorf("unsupported target %q", t)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating bundle dir: %w", err)
	}

	var entries []Entry

	// Skills project into <root>/skills/facet/, matching what the existing
	// per-engine projection already does for an initialized project.
	skillDest := filepath.Join("skills", "facet")
	got, err := copyTree(src.SkillsDir, filepath.Join(outDir, skillDest), skillDest, "skill")
	if err != nil {
		return nil, fmt.Errorf("projecting skills: %w", err)
	}
	entries = append(entries, got...)

	// Packs project one directory per pack.
	packNames, err := subdirs(src.PacksDir)
	if err != nil {
		return nil, fmt.Errorf("reading packs: %w", err)
	}
	for _, p := range packNames {
		dest := filepath.Join("skills", p)
		got, err := copyTree(filepath.Join(src.PacksDir, p), filepath.Join(outDir, dest), dest, "pack")
		if err != nil {
			return nil, fmt.Errorf("projecting pack %s: %w", p, err)
		}
		entries = append(entries, got...)
	}

	// Tool wiring: the instruction file that tells the agent Facet exists and
	// how to invoke it. This is the piece `facet init` never wrote, and without
	// it a bundle is guidance the agent has no way to act on.
	wiring := renderToolWiring(t, compat, src)
	wiringPath := instructionFileFor(t)
	if err := writeFile(filepath.Join(outDir, wiringPath), []byte(wiring)); err != nil {
		return nil, fmt.Errorf("writing tool wiring: %w", err)
	}
	entries = append(entries, entryFor(wiringPath, []byte(wiring), "wiring"))

	if len(entries) == 0 {
		// An empty bundle installs nothing and would report success.
		return nil, fmt.Errorf("bundle is empty; nothing was projected")
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	tools := append([]string(nil), src.Tools...)
	sort.Strings(tools)

	m := &Manifest{
		Schema:         ManifestSchema,
		FacetVersion:   src.FacetVersion,
		Target:         t,
		AdapterVersion: AdapterVersion,
		Compatibility:  compat,
		Entries:        entries,
		Tools:          tools,
	}
	m.BundleDigest = bundleDigest(entries)

	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := writeFile(filepath.Join(outDir, "facet-bundle.json"), raw); err != nil {
		return nil, fmt.Errorf("writing manifest: %w", err)
	}
	return m, nil
}

// instructionFileFor returns the target-native agent instruction path.
//
// Each target reads a different file. Projecting the same guidance into the
// right name is exactly the adapter's job.
func instructionFileFor(t Target) string {
	switch t {
	case TargetClaude:
		return "CLAUDE.md"
	case TargetCodex:
		return "AGENTS.md"
	case TargetCopilot:
		return filepath.ToSlash(filepath.Join("copilot-instructions.md"))
	case TargetOpenCode:
		return "AGENTS.md"
	}
	return "AGENTS.md"
}

func bundleDigest(entries []Entry) string {
	h := sha256.New()
	for _, e := range entries {
		fmt.Fprintf(h, "%s\n%s\n", e.Path, e.Digest)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func entryFor(rel string, content []byte, kind string) Entry {
	sum := sha256.Sum256(content)
	return Entry{
		Path:   filepath.ToSlash(rel),
		Bytes:  int64(len(content)),
		Digest: "sha256:" + hex.EncodeToString(sum[:]),
		Kind:   kind,
	}
}

func writeFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func subdirs(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// copyTree projects a canonical directory into the bundle, returning an entry
// per file. relBase is the bundle-relative destination prefix.
func copyTree(srcDir, dstDir, relBase, kind string) ([]Entry, error) {
	var entries []Entry
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := writeFile(filepath.Join(dstDir, rel), content); err != nil {
			return err
		}
		entries = append(entries, entryFor(filepath.Join(relBase, rel), content, kind))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// Verify checks an installed or built bundle against its own manifest.
//
// Answers "is this bundle intact and complete", which is different from "did
// the build succeed". A build that succeeded and a bundle that is whole are
// separate claims, and this repo has already shipped a bundle that reported
// success while missing the files that made it renderable.
func Verify(dir string) (*Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "facet-bundle.json"))
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("manifest does not decode: %w", err)
	}
	if m.Schema != ManifestSchema {
		return nil, fmt.Errorf("manifest schema %q, want %q", m.Schema, ManifestSchema)
	}
	if len(m.Entries) == 0 {
		return nil, fmt.Errorf("manifest declares no entries")
	}

	for _, e := range m.Entries {
		p := filepath.Join(dir, filepath.FromSlash(e.Path))
		f, err := os.Open(p)
		if err != nil {
			return nil, fmt.Errorf("declared entry %s is missing: %w", e.Path, err)
		}
		h := sha256.New()
		n, err := io.Copy(h, f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", e.Path, err)
		}
		if n != e.Bytes {
			return nil, fmt.Errorf("%s is %d bytes, manifest says %d", e.Path, n, e.Bytes)
		}
		got := "sha256:" + hex.EncodeToString(h.Sum(nil))
		if got != e.Digest {
			return nil, fmt.Errorf("%s digest %s, manifest says %s", e.Path, got, e.Digest)
		}
	}

	if got := bundleDigest(m.Entries); got != m.BundleDigest {
		return nil, fmt.Errorf("bundle digest %s, manifest says %s", got, m.BundleDigest)
	}
	return &m, nil
}

// renderToolWiring produces the target-native instruction that makes Facet
// discoverable and invocable.
//
// THE CANONICAL GUIDANCE IS THE SAME FOR EVERY TARGET. Only the surrounding
// shape differs, which is the adapter rule holding: an agent reading any of
// these learns the same product semantics.
func renderToolWiring(t Target, c Compatibility, src Source) string {
	var b strings.Builder

	b.WriteString("# Facet — creative production tools\n\n")
	b.WriteString(fmt.Sprintf("Facet v%s is installed. It turns creative intent into verified media artifacts.\n\n",
		src.FacetVersion))

	b.WriteString("## How to invoke\n\n")
	b.WriteString("Facet is a command-line tool. Discover, estimate, then run:\n\n")
	b.WriteString("```sh\n")
	b.WriteString(fmt.Sprintf("%s tools list\n", c.FacetBinary))
	b.WriteString(fmt.Sprintf("%s tools describe <tool>\n", c.FacetBinary))
	b.WriteString(fmt.Sprintf("%s tools estimate <tool> --input request.json\n", c.FacetBinary))
	b.WriteString(fmt.Sprintf("%s tools run <tool> --input request.json\n", c.FacetBinary))
	b.WriteString("```\n\n")

	b.WriteString("**Always `describe` before constructing a request**, and `estimate` before running\n")
	b.WriteString("anything that may cost money. `estimate` never bills, never writes, never generates.\n\n")

	b.WriteString("## Rules that are product guarantees, not style\n\n")
	b.WriteString("- **Paid work needs explicit human consent.** Some tools may charge. Ask the\n")
	b.WriteString("  person before running one. An unknown cost is never zero.\n")
	b.WriteString("- **Verify the output, not the exit code.** A successful process exit is not a\n")
	b.WriteString("  successful render. Check duration, resolution, frame count and visible content\n")
	b.WriteString("  with `media_probe`, `visual_qa` or `output_review` before reporting success.\n")
	b.WriteString("- **Never substitute mock output for a real asset.**\n\n")

	b.WriteString(fmt.Sprintf("## Available tools (%d)\n\n", len(src.Tools)))
	for _, name := range src.Tools {
		b.WriteString(fmt.Sprintf("- `%s`\n", name))
	}
	b.WriteString("\n")

	b.WriteString("## Guidance\n\n")
	b.WriteString(fmt.Sprintf("Production guidance is installed under `%s/skills/`. Read `skills/facet/SKILL.md`\n", c.InstallRoot))
	b.WriteString("first, then the pack matching the requested style.\n")

	return b.String()
}
