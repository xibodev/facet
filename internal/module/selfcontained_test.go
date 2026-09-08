package module

import (
	"os"
	"path/filepath"
	"testing"
)

// A packaged module must carry the content its descriptor declares.
//
// Midden reported the host logging "artifact schema directory unavailable:
// .local/modules/xibodev.facet/schemas/artifacts". That install predates the
// packaging fix — build-module.sh copied the binary and nothing else, so an
// installed module found its content only by borrowing whatever bundle
// happened to exist in HOME. On a machine without one it had no schemas, no
// skills and no overlay.
//
// Verified after the fix, from an unrelated working directory with HOME
// pointed at an empty tree: 21 artifact schemas, 3 skills, 1 overlay.
//
// This asserts the PACKAGE on disk rather than the running descriptor,
// because the descriptor can be satisfied by a borrowed bundle and still
// leave a fresh install empty.
func TestPackagedModuleCarriesItsDeclaredContent(t *testing.T) {
	pkg := os.Getenv("FACET_PACKAGE_DIR")
	if pkg == "" {
		pkg = filepath.Join("..", "..", "dist")
	}
	if _, err := os.Stat(pkg); err != nil {
		t.Skip("no packaged module to inspect; run scripts/build-module.sh")
	}

	// Every path the descriptor declares, plus the schemas it indexes.
	for _, rel := range []string{
		filepath.Join("agents", "facet-creative.md"),
		filepath.Join("skills", "facet", "SKILL.md"),
		filepath.Join("packs", "explainer", "SCENE-TYPES.md"),
		filepath.Join("packs", "explainer", "NARRATED-WALKTHROUGH.md"),
		filepath.Join("schemas", "artifacts"),
		filepath.Join("remotion-composer", "src", "Root.tsx"),
	} {
		if _, err := os.Stat(filepath.Join(pkg, rel)); err != nil {
			t.Errorf("the packaged module is missing declared content: %s\n"+
				"    an install without it borrows another bundle, or has nothing", rel)
		}
	}

	// The schema directory must hold documents, not merely exist.
	entries, err := os.ReadDir(filepath.Join(pkg, "schemas", "artifacts"))
	if err != nil {
		t.Fatalf("packaged schema directory unreadable: %v", err)
	}
	count := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			count++
		}
	}
	if count == 0 {
		t.Error("the packaged schema directory holds no schema documents")
	}
}
