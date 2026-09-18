package module

import (
	"os"
	"path/filepath"
	"testing"
)

// A packaged module must carry the content its descriptor declares.
//
// A module package must not borrow guidance or renderer assets from another
// installation. It carries every declared content file beside the binary.
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

	// Every content path the descriptor declares.
	for _, rel := range []string{
		filepath.Join("agents", "facet-creative.md"),
		filepath.Join("skills", "facet", "SKILL.md"),
		filepath.Join("packs", "explainer", "SCENE-TYPES.md"),
		filepath.Join("packs", "explainer", "NARRATED-WALKTHROUGH.md"),
		filepath.Join("remotion-composer", "src", "Root.tsx"),
		filepath.Join("remotion-composer", "src", "contract.ts"),
		filepath.Join("remotion-composer", "legacy-composer-manifest.json"),
	} {
		if _, err := os.Stat(filepath.Join(pkg, rel)); err != nil {
			t.Errorf("the packaged module is missing declared content: %s\n"+
				"    an install without it borrows another bundle, or has nothing", rel)
		}
	}
}
