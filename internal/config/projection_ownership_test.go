package config

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInitReconcilesAdapterSwitchAndPackDeselection(t *testing.T) {
	cfg := projectionTestConfig(t)
	adapters := []struct {
		engine string
		root   string
	}{
		{"claude", ".claude/skills"},
		{"copilot", ".github/skills"},
		{"codex", ".agents/skills"},
		{"opencode", ".opencode/skills"},
		{"studio", "skills"},
	}
	for index, adapter := range adapters {
		next := adapters[(index+1)%len(adapters)]
		t.Run(adapter.engine+"-to-"+next.engine, func(t *testing.T) {
			project := filepath.Join(t.TempDir(), "project")
			runProjectionInit(t, project, adapter.engine, []string{"explainer", "cinematic"}, cfg)
			runProjectionInit(t, project, next.engine, []string{"cinematic"}, cfg)

			for _, name := range []string{"facet", "explainer", "cinematic"} {
				oldPath := filepath.Join(project, filepath.FromSlash(adapter.root), name)
				if adapter.root == next.root && name != "explainer" {
					continue
				}
				if _, err := os.Lstat(oldPath); !os.IsNotExist(err) {
					t.Errorf("obsolete projection survived: %s (%v)", oldPath, err)
				}
			}
			for _, name := range []string{"facet", "cinematic"} {
				if _, err := os.Stat(filepath.Join(project, filepath.FromSlash(next.root), name, "SKILL.md")); err != nil {
					t.Errorf("selected projection missing: %s/%s: %v", next.root, name, err)
				}
			}
			if _, err := os.Lstat(filepath.Join(project, filepath.FromSlash(next.root), "explainer")); !os.IsNotExist(err) {
				t.Errorf("deselected pack survived: %v", err)
			}
			owned := loadOwnership(project)
			for key, entry := range owned.ManagedEntries {
				if isProjectionEntry(entry) && strings.HasPrefix(filepath.ToSlash(key), adapter.root+"/") && adapter.root != next.root {
					t.Errorf("obsolete ownership survived: %s", key)
				}
			}
		})
	}
}

func TestInitPreservesTamperedObsoleteProjection(t *testing.T) {
	cfg := projectionTestConfig(t)
	project := filepath.Join(t.TempDir(), "project")
	runProjectionInit(t, project, "claude", []string{"explainer"}, cfg)
	projection := filepath.Join(project, ".claude", "skills", "explainer")
	if err := os.RemoveAll(projection); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(projection, 0755); err != nil {
		t.Fatal(err)
	}
	userFile := filepath.Join(projection, "user.txt")
	if err := os.WriteFile(userFile, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := RunInitWithOptions(InitOptions{ProjectDir: project, Engine: "codex"}, cfg, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "preserving") {
		t.Fatalf("tampered obsolete projection error = %v", err)
	}
	if data, readErr := os.ReadFile(userFile); readErr != nil || string(data) != "keep" {
		t.Fatalf("tampered projection was removed: %q, %v", data, readErr)
	}
}

func TestInitRejectsReplacedManagedSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows init uses directory junctions")
	}
	cfg := projectionTestConfig(t)
	project := filepath.Join(t.TempDir(), "project")
	runProjectionInit(t, project, "codex", nil, cfg)
	projection := filepath.Join(project, ".agents", "skills", "facet")
	if err := os.Remove(projection); err != nil {
		t.Fatal(err)
	}
	replacement := t.TempDir()
	if err := os.WriteFile(filepath.Join(replacement, "user.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(replacement, projection); err != nil {
		t.Fatal(err)
	}

	_, err := RunInitWithOptions(InitOptions{ProjectDir: project, Engine: "codex"}, cfg, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "target") {
		t.Fatalf("replaced symlink error = %v", err)
	}
	if data, readErr := os.ReadFile(filepath.Join(replacement, "user.txt")); readErr != nil || string(data) != "keep" {
		t.Fatalf("replacement target was modified: %q, %v", data, readErr)
	}
}

func TestInitRejectsModifiedCopiedProjection(t *testing.T) {
	cfg := projectionTestConfig(t)
	project := filepath.Join(t.TempDir(), "project")
	runProjectionInit(t, project, "studio", nil, cfg)
	projection := filepath.Join(project, "skills", "facet")
	owned := loadOwnership(project)
	key := filepath.Join("skills", "facet")
	if err := removeManagedProjection(projection, owned.ManagedEntries[key]); err != nil {
		t.Fatal(err)
	}
	if err := copyDirectory(filepath.Join(cfg.Paths.Bundle, "skills", "facet"), projection); err != nil {
		t.Fatal(err)
	}
	entry := owned.ManagedEntries[key]
	entry.EntryType = "copy"
	entry.ContentSHA256, _ = hashProjectionTree(projection)
	owned.ManagedEntries[key] = entry
	if err := saveOwnership(project, owned); err != nil {
		t.Fatal(err)
	}
	userFile := filepath.Join(projection, "user.txt")
	if err := os.WriteFile(userFile, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := RunInitWithOptions(InitOptions{ProjectDir: project, Engine: "studio"}, cfg, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "modified copied projection") {
		t.Fatalf("modified copy error = %v", err)
	}
	if data, readErr := os.ReadFile(userFile); readErr != nil || string(data) != "keep" {
		t.Fatalf("modified copied projection was removed: %q, %v", data, readErr)
	}
}

func TestInitRejectsObsoleteProjectionWithWrongOwnershipType(t *testing.T) {
	cfg := projectionTestConfig(t)
	project := filepath.Join(t.TempDir(), "project")
	runProjectionInit(t, project, "claude", []string{"explainer"}, cfg)
	owned := loadOwnership(project)
	key := filepath.Join(".claude", "skills", "explainer")
	entry := owned.ManagedEntries[key]
	entry.EntryType = "instruction-section"
	owned.ManagedEntries[key] = entry
	if err := saveOwnership(project, owned); err != nil {
		t.Fatal(err)
	}

	_, err := RunInitWithOptions(InitOptions{ProjectDir: project, Engine: "codex"}, cfg, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unexpected ownership type") {
		t.Fatalf("wrong ownership type error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(project, ".claude", "skills", "explainer", "SKILL.md")); statErr != nil {
		t.Fatalf("projection with invalid ownership was removed: %v", statErr)
	}
}

func projectionTestConfig(t *testing.T) *Config {
	t.Helper()
	bundle := filepath.Join(t.TempDir(), "bundle")
	writeConfigFixture(t, filepath.Join(bundle, "skills", "facet", "SKILL.md"), "# Core")
	for _, pack := range []string{"explainer", "cinematic"} {
		writeConfigFixture(t, filepath.Join(bundle, "packs", pack, "SKILL.md"), "# "+pack)
	}
	cfg := DefaultConfig()
	cfg.Paths.Bundle = bundle
	return cfg
}

func runProjectionInit(t *testing.T, project, engine string, packs []string, cfg *Config) {
	t.Helper()
	if _, err := RunInitWithOptions(InitOptions{ProjectDir: project, Engine: engine, Packs: packs}, cfg, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}
