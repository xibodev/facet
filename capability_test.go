package facet

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedCapabilityMatchesCanonicalHostSources(t *testing.T) {
	paths := []string{"skills/facet/SKILL.md", "agents/facet-creative.md"}
	for _, pack := range RetainedPacks() {
		for _, guidance := range pack.Guidance {
			paths = append(paths, guidance.Path)
		}
	}
	for _, name := range paths {
		embedded, err := Guidance(name)
		if err != nil {
			t.Fatal(err)
		}
		source, err := os.ReadFile(filepath.FromSlash(name))
		if err != nil {
			t.Fatal(err)
		}
		if embedded != string(source) {
			t.Fatalf("native capability drifted from CLI/module source: %s", name)
		}
	}
}

func TestRetainedPackCatalogIsCompleteAndProgressivelyLoadable(t *testing.T) {
	packs := RetainedPacks()
	if len(packs) == 0 {
		t.Fatal("no retained packs")
	}
	for _, pack := range packs {
		if pack.ID == "" || pack.Title == "" || pack.Summary == "" {
			t.Errorf("incomplete pack metadata: %#v", pack)
		}
		if len(pack.Guidance) == 0 {
			t.Errorf("pack %s declares no progressively loadable guidance", pack.ID)
		}
		for _, guidance := range pack.Guidance {
			if guidance.ID == "" || guidance.Title == "" || guidance.Summary == "" {
				t.Errorf("incomplete guidance metadata for %s: %#v", pack.ID, guidance)
			}
			if _, err := Guidance(guidance.Path); err != nil {
				t.Errorf("%s guidance %s is not embedded: %v", pack.ID, guidance.Path, err)
			}

			declared := map[string]bool{}
			for _, pack := range packs {
				for _, guidance := range pack.Guidance {
					declared[guidance.Path] = true
				}
			}
			if err := fs.WalkDir(Assets, "packs", func(name string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !entry.IsDir() && strings.HasSuffix(name, ".md") && !declared[name] {
					t.Errorf("embedded pack guidance is outside the retained catalog: %s", name)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
}
