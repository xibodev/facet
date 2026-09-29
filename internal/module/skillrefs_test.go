package module

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A file the skills REFERENCE must also be DECLARED.
//
// The host installs exactly what the descriptor declares and verifies each
// digest, so an undeclared file does not travel with the module. An agent then
// reads "the path is in NARRATED-WALKTHROUGH.md", finds nothing, and sequences
// by guess — documented knowledge that cannot be reached is worse than absent
// knowledge, because the reference implies availability.
//
// This is the check that would have caught two pack files being referenced by
// the core skill while neither was declared.
var skillFileRef = regexp.MustCompile(`.([A-Za-z0-9_./-]+[.]md).`)

func TestEveryReferencedSkillFileIsDeclared(t *testing.T) {
	env := Describe("test")
	desc, ok := env.Result.(Descriptor)
	if !ok {
		t.Fatalf("result is not a Descriptor: %T", env.Result)
	}

	declared := map[string]bool{}
	for _, s := range desc.Skills {
		declared[filepath.ToSlash(s.Path)] = true
		declared[filepath.Base(s.Path)] = true
	}
	for _, o := range desc.AgentOverlays {
		declared[filepath.ToSlash(o.Path)] = true
		declared[filepath.Base(o.Path)] = true
	}

	root := moduleRoot()
	for _, s := range desc.Skills {
		body, err := os.ReadFile(filepath.Join(root, s.Path))
		if err != nil {
			t.Errorf("declared skill %s cannot be read: %v", s.ID, err)
			continue
		}
		for _, m := range skillFileRef.FindAllStringSubmatch(string(body), -1) {
			ref := m[1]
			// A reference to a file that exists in the module must be declared,
			// or the host will not ship it.
			if _, err := os.Stat(filepath.Join(root, ref)); err != nil {
				// Not a module file — a bare filename mentioned in prose.
				if _, err := os.Stat(filepath.Join(root, filepath.Dir(s.Path), ref)); err != nil {
					continue
				}
			}
			if declared[ref] || declared[filepath.Base(ref)] {
				continue
			}
			t.Errorf("skill %s references %q which is not declared; the host installs "+
				"only declared files, so an agent would find nothing there", s.ID, ref)
		}
	}
}

// Every declared skill must be present, digested, and priced in tokens, since
// the host budgets context before loading and refuses unverifiable content.
func TestDeclaredSkillsAreComplete(t *testing.T) {
	env := Describe("test")
	desc, _ := env.Result.(Descriptor)
	if len(desc.Skills) == 0 {
		t.Fatal("no skills declared")
	}
	seen := map[string]bool{}
	for _, s := range desc.Skills {
		if seen[s.ID] {
			t.Errorf("duplicate skill id %q", s.ID)
		}
		seen[s.ID] = true
		if strings.TrimSpace(s.Title) == "" || strings.TrimSpace(s.Summary) == "" {
			t.Errorf("skill %s has no title or summary; the host lists these", s.ID)
		}
		if !ValidDigest(s.Digest) {
			t.Errorf("skill %s digest %q is not sha256:<64 lowercase hex>", s.ID, s.Digest)
		}
		if s.Tokens <= 0 {
			t.Errorf("skill %s declares %d tokens; the host budgets context with this",
				s.ID, s.Tokens)
		}
		if filepath.IsAbs(s.Path) {
			t.Errorf("skill %s declares an absolute path", s.ID)
		}
	}
}
