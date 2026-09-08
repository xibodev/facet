package module

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// A seed's attachments are the documents written to brief this render.
//
// Seed.Attachments was parsed and never read: Midden's video_brief — the
// document produced specifically as the handoff to Facet — was dropped, and
// nothing downstream could tell it had existed. The ninth instance of a field
// accepted and ignored.
//
// An artifact claiming provenance from a seed should say which of that seed's
// documents it actually had.
func TestSeedAttachmentsAreResolvedAndDigested(t *testing.T) {
	root, brief := stageSeedWithAttachment(t, "attachments/brief.md",
		"# Video brief\nSuggested title: How Rain Forms\n")

	_, res := mustLoad(t, root)
	if len(res.Attachments) != 1 {
		t.Fatalf("resolved %d attachments, want 1: %+v", len(res.Attachments), res.Attachments)
	}
	got := res.Attachments[0]
	if got.Path != "attachments/brief.md" {
		t.Errorf("path = %q, want the bundle-relative name", got.Path)
	}
	if got.Missing {
		t.Error("an attachment that exists was reported missing")
	}
	sum := sha256.Sum256([]byte(brief))
	if want := "sha256:" + hex.EncodeToString(sum[:]); got.Digest != want {
		t.Errorf("digest %q does not describe the bytes on disk (%q)", got.Digest, want)
	}
	if got.Bytes != len(brief) {
		t.Errorf("bytes = %d, want %d", got.Bytes, len(brief))
	}
}

// A named file that did not survive staging is REPORTED, not fatal: the
// evidence and manifest may still be exactly what the caller wanted, and a
// silent omission would leave a consumer unable to tell an absent brief from
// one that was never named.
func TestMissingAttachmentIsReportedNotFatal(t *testing.T) {
	root, _ := stageSeedWithAttachment(t, "attachments/gone.md", "")
	if err := os.Remove(filepath.Join(root, "attachments", "gone.md")); err != nil {
		t.Fatal(err)
	}
	_, res := mustLoad(t, root)
	if len(res.Attachments) != 1 || !res.Attachments[0].Missing {
		t.Errorf("a missing attachment was not reported: %+v", res.Attachments)
	}
	if res.Attachments[0].Digest != "" {
		t.Error("a digest was reported for a file that could not be read")
	}
}

// A seed is data from another module. A path in it that escapes the bundle or
// is absolute must not be followed: that would let a seed name any file on the
// machine and have Facet read and digest it.
func TestAttachmentPathsCannotLeaveTheBundle(t *testing.T) {
	for _, name := range []string{"../../etc/passwd", "attachments/../../x", "C:/Windows/win.ini", "/etc/hosts"} {
		root, _ := stageSeedWithAttachment(t, name, "should not be read")
		_, res := mustLoad(t, root)
		if len(res.Attachments) != 1 {
			t.Fatalf("%s: resolved %d attachments", name, len(res.Attachments))
		}
		if !res.Attachments[0].Missing || res.Attachments[0].Digest != "" {
			t.Errorf("%s was followed out of the bundle: %+v", name, res.Attachments[0])
		}
	}
}

func stageSeedWithAttachment(t *testing.T, name, body string) (root, written string) {
	t.Helper()
	root = t.TempDir()
	// Only stage a real file for a name that stays inside the bundle.
	if !isAbsolutePath(name) && !escapesRoot(name) {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		written = body
	}
	m := map[string]any{
		"schema": SeedSchemaID, "goal": "Explain how rain forms",
		"title": "How Rain Forms", "attachments": []string{name},
	}
	raw, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, SeedManifestFile), raw, 0600); err != nil {
		t.Fatal(err)
	}
	return root, written
}

func mustLoad(t *testing.T, root string) (*Seed, *SeedResolution) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, SeedManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	seed, res, err := LoadSeed(&SeedRef{
		Schema: SeedSchemaID, Path: root, Digest: hex.EncodeToString(sum[:]),
	})
	if err != nil {
		t.Fatalf("a valid seed failed to load: %v", err)
	}
	return seed, res
}
