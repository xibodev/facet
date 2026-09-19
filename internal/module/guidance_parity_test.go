package module

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	facet "github.com/xibodev/facet"
)

func TestModuleGuidanceMatchesCanonicalRetainedPackCatalog(t *testing.T) {
	env := Describe("test")
	if !env.OK {
		t.Fatalf("describe failed: %+v", env.Error)
	}
	desc := env.Result.(Descriptor)
	declared := make(map[string]Skill, len(desc.Skills))
	for _, skill := range desc.Skills {
		declared[skill.Path] = skill
	}

	expected := map[string]bool{"skills/facet/SKILL.md": true}
	for _, pack := range facet.RetainedPacks() {
		for _, guidance := range pack.Guidance {
			expected[guidance.Path] = true
			skill, ok := declared[guidance.Path]
			if !ok {
				t.Errorf("module omits canonical guidance %s", guidance.Path)
				continue
			}
			if skill.ID != guidance.ID || skill.Title != guidance.Title || skill.Summary != guidance.Summary {
				t.Errorf("module metadata drift for %s: %#v != %#v", guidance.Path, skill, guidance)
			}
			content, err := facet.Guidance(guidance.Path)
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256([]byte(content))
			wantDigest := "sha256:" + hex.EncodeToString(sum[:])
			if skill.Digest != wantDigest {
				t.Errorf("module provenance drift for %s: %s != %s",
					guidance.Path, skill.Digest, wantDigest)
			}
		}
	}
	for path := range declared {
		if !expected[path] {
			t.Errorf("module declares guidance outside the canonical catalog: %s", path)
		}
	}
}
