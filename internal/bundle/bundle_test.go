package bundle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testSource(t *testing.T) Source {
	t.Helper()
	return Source{
		SkillsDir:    filepath.Join("..", "..", "skills", "facet"),
		PacksDir:     filepath.Join("..", "..", "packs"),
		Tools:        []string{"video_compose", "media_probe", "edge_tts"},
		FacetVersion: "1.0.2-test",
	}
}

// A bundle must carry identity and provenance, or an installer cannot decide
// whether to upgrade, and a human cannot tell what was installed.
func TestBundleCarriesIdentityAndProvenance(t *testing.T) {
	dir := t.TempDir()
	m, err := Build(testSource(t), TargetClaude, dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Schema != ManifestSchema {
		t.Errorf("schema %q, want %q", m.Schema, ManifestSchema)
	}
	if m.FacetVersion == "" {
		t.Error("no facet version; the bundle cannot be compared against a running Facet")
	}
	if m.AdapterVersion == "" {
		t.Error("no adapter version; a layout fix would be indistinguishable from a product change")
	}
	if m.Target != TargetClaude {
		t.Errorf("target %q, want claude", m.Target)
	}
	if !strings.HasPrefix(m.BundleDigest, "sha256:") {
		t.Errorf("bundle digest %q is not a sha256", m.BundleDigest)
	}
	if len(m.Tools) == 0 {
		t.Error("no tools recorded; a stale install would be invisible")
	}
	if len(m.Entries) == 0 {
		t.Fatal("no entries")
	}
}

// The adapter must produce TARGET-NATIVE shapes. Same product semantics,
// different packaging -- that is the whole rule.
func TestEachTargetGetsItsNativeShape(t *testing.T) {
	want := map[Target]struct{ instruction, root string }{
		TargetClaude:   {"CLAUDE.md", ".claude"},
		TargetCodex:    {"AGENTS.md", ".codex"},
		TargetCopilot:  {"copilot-instructions.md", ".github"},
		TargetOpenCode: {"AGENTS.md", ".opencode"},
	}
	for tgt, exp := range want {
		dir := t.TempDir()
		m, err := Build(testSource(t), tgt, dir)
		if err != nil {
			t.Fatalf("%s: %v", tgt, err)
		}
		if _, err := os.Stat(filepath.Join(dir, exp.instruction)); err != nil {
			t.Errorf("%s: expected native instruction file %s: %v", tgt, exp.instruction, err)
		}
		if m.Compatibility.InstallRoot != exp.root {
			t.Errorf("%s: install root %q, want %q", tgt, m.Compatibility.InstallRoot, exp.root)
		}
	}
}

// Every target must receive the SAME product semantics. Divergent guidance is
// the four-hand-maintained-copies failure the adapter rule exists to prevent.
func TestTargetsShareProductSemantics(t *testing.T) {
	bodies := map[Target]string{}
	for _, tgt := range Targets() {
		dir := t.TempDir()
		if _, err := Build(testSource(t), tgt, dir); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(dir, instructionFileFor(tgt)))
		if err != nil {
			t.Fatal(err)
		}
		bodies[tgt] = string(b)
	}
	// These are product guarantees, not phrasing. Every target must state them.
	required := []string{
		"consent",           // paid work needs approval
		"not the exit code", // verify output, not process exit
		"estimate",          // estimate before running
		"Never substitute mock",
		// A stale binary earlier on PATH rejects the shapes this bundle
		// documents, and the failure reads as bad guidance rather than a
		// wrong binary. Every target must tell the agent to check.
		"version",
	}
	for tgt, body := range bodies {
		for _, r := range required {
			if !strings.Contains(body, r) {
				t.Errorf("%s guidance omits %q; product semantics must not differ by target", tgt, r)
			}
		}
	}
}

// Verify must FAIL on a tampered bundle. A verifier that cannot fail certifies
// whatever it is given.
func TestVerifyDetectsTampering(t *testing.T) {
	dir := t.TempDir()
	m, err := Build(testSource(t), TargetClaude, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(dir); err != nil {
		t.Fatalf("a freshly built bundle failed verification: %v", err)
	}

	// Content changed, same length: only the digest can catch it.
	victim := filepath.Join(dir, filepath.FromSlash(m.Entries[0].Path))
	orig, err := os.ReadFile(victim)
	if err != nil {
		t.Fatal(err)
	}
	if len(orig) == 0 {
		t.Skip("first entry is empty; nothing to corrupt in place")
	}
	corrupt := append([]byte(nil), orig...)
	if corrupt[0] == 'X' {
		corrupt[0] = 'Y'
	} else {
		corrupt[0] = 'X'
	}
	if err := os.WriteFile(victim, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(dir); err == nil {
		t.Error("a bundle with altered content passed verification")
	}
	os.WriteFile(victim, orig, 0o644)

	// A missing declared file must also fail.
	if err := os.Remove(victim); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(dir); err == nil {
		t.Error("a bundle missing a declared entry passed verification")
	}
}

// Building twice must produce an identical bundle. A non-reproducible build
// makes the digest meaningless as an upgrade signal.
func TestBuildIsReproducible(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	m1, err := Build(testSource(t), TargetCodex, a)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := Build(testSource(t), TargetCodex, b)
	if err != nil {
		t.Fatal(err)
	}
	if m1.BundleDigest != m2.BundleDigest {
		t.Errorf("two builds of the same source differ: %s vs %s", m1.BundleDigest, m2.BundleDigest)
	}
	if len(m1.Entries) != len(m2.Entries) {
		t.Errorf("entry counts differ: %d vs %d", len(m1.Entries), len(m2.Entries))
	}
}

// A bundle that names no tools would install guidance for a product the agent
// cannot invoke. Refuse rather than ship something decorative.
func TestEmptyVocabularyIsRefused(t *testing.T) {
	src := testSource(t)
	src.Tools = nil
	if _, err := Build(src, TargetClaude, t.TempDir()); err == nil {
		t.Error("a bundle with no tools was accepted")
	}
	src = testSource(t)
	src.FacetVersion = ""
	if _, err := Build(src, TargetClaude, t.TempDir()); err == nil {
		t.Error("a bundle with no version was accepted")
	}
}

// The manifest must describe the directory it sits in. A manifest listing
// entries that are not there, or a directory holding files the manifest does
// not name, is a bundle nobody can trust.
func TestManifestDescribesTheDirectory(t *testing.T) {
	dir := t.TempDir()
	m, err := Build(testSource(t), TargetOpenCode, dir)
	if err != nil {
		t.Fatal(err)
	}
	declared := map[string]bool{}
	for _, e := range m.Entries {
		declared[e.Path] = true
	}
	var undeclared []string
	err = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if rel == "facet-bundle.json" {
			return nil
		}
		if !declared[rel] {
			undeclared = append(undeclared, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(undeclared) > 0 {
		t.Errorf("%d files on disk are not in the manifest: %v", len(undeclared), undeclared[:min(3, len(undeclared))])
	}

	// And the manifest must round-trip as JSON an installer can read.
	raw, err := os.ReadFile(filepath.Join(dir, "facet-bundle.json"))
	if err != nil {
		t.Fatal(err)
	}
	var back Manifest
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("manifest does not decode: %v", err)
	}
	if back.BundleDigest != m.BundleDigest {
		t.Error("the written manifest disagrees with the built one")
	}
}

func TestBuiltBundlesContainOnlyCanonicalFacetContent(t *testing.T) {
	banned := []string{"Open" + "Montage", "Video" + " Kit"}
	for _, target := range Targets() {
		t.Run(string(target), func(t *testing.T) {
			dir := t.TempDir()
			manifest, err := Build(testSource(t), target, dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range manifest.Entries {
				name := filepath.ToSlash(entry.Path)
				for _, legacy := range []string{"pipeline_defs/", "styles/", "skills/core/", "skills/creative/", "skills/meta/", "skills/pipelines/", ".agents/skills/"} {
					if strings.Contains(name, legacy) {
						t.Errorf("bundle contains removed surface %s", name)
					}
				}
				switch strings.ToLower(filepath.Ext(name)) {
				case ".json", ".md", ".yaml", ".yml":
				default:
					continue
				}
				data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(entry.Path)))
				if err != nil {
					t.Fatal(err)
				}
				for _, term := range banned {
					if strings.Contains(strings.ToLower(string(data)), strings.ToLower(term)) {
						t.Errorf("%s contains banned donor term %q", name, term)
					}
				}
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
