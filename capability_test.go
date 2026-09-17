package facet

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedCapabilityMatchesCanonicalHostSources(t *testing.T) {
	paths := []string{"skills/facet/SKILL.md", "agents/facet-creative.md"}
	for _, pack := range PackNames() {
		paths = append(paths, "packs/"+pack+"/SKILL.md")
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
