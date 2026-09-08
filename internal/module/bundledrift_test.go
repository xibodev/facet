package module

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// The installed bundle is a COPY, and it is what a real installation reads.
//
// The descriptor's skill digests describe the bundle, not this checkout, so
// both can be perfectly honest while the content is months apart. That is what
// happened: the installed SKILL.md still taught the `out_seconds + 1 second`
// padding rule corrected here, and knew nothing about the estimate or QA
// fields added since. Every digest check passed while an agent read stale
// instructions.
//
// This compares CONTENT rather than digests, because a digest check cannot see
// the problem — it confirms the bundle is intact, not that it is current.
func TestInstalledBundleGuidanceIsCurrent(t *testing.T) {
	bundle := os.Getenv("FACET_BUNDLE")
	if bundle == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("no home directory to locate an installed bundle")
		}
		bundle = filepath.Join(home, ".facet", "bundle")
	}
	if _, err := os.Stat(bundle); err != nil {
		t.Skip("no installed bundle to compare against")
	}

	repo := filepath.Join("..", "..")
	for _, rel := range []string{
		filepath.Join("agents", "facet-creative.md"),
		filepath.Join("skills", "facet", "SKILL.md"),
		filepath.Join("packs", "explainer", "SCENE-TYPES.md"),
		filepath.Join("packs", "explainer", "NARRATED-WALKTHROUGH.md"),
		// The Remotion composer drifted worse than the guidance: the installed
		// copy hardcoded `(lastEnd + 1) * 30` and ignored duration_seconds, so
		// a 2-second plan rendered 3.000s through the installed path while the
		// same request through a fresh build rendered 2.000s. Identical
		// binaries, identical request, different videos.
		filepath.Join("remotion-composer", "src", "Root.tsx"),
		filepath.Join("remotion-composer", "src", "Explainer.tsx"),
		filepath.Join("remotion-composer", "src", "explainerMetadata.ts"),
		filepath.Join("remotion-composer", "package.json"),
	} {
		want, err := os.ReadFile(filepath.Join(repo, rel))
		if err != nil {
			t.Errorf("declared guidance missing from the repository: %s", rel)
			continue
		}
		got, err := os.ReadFile(filepath.Join(bundle, rel))
		if err != nil {
			t.Errorf("declared guidance missing from the installed bundle: %s", rel)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("installed bundle is stale: %s\n"+
				"    an installed agent reads the bundle copy, not this repository\n"+
				"    refresh with: ./scripts/build-module.sh %q", rel, bundle)
		}
	}
}
