package studio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RegisterOrUpdateProject returns an error and discarded the only one it could
// produce:
//
//	_ = SaveCatalog(cat, rootDir...)
//	return found, nil
//
// SaveCatalog fails on four real paths — mkdir, marshal, write, rename. Every
// one left the project registered in memory, absent from disk, and reported to
// the caller as SUCCESS. The Studio then listed a project that would not exist
// after a restart.
//
// Same shape as a warning computed and never appended: the value is right
// inside the function and dies at the boundary a caller reads.
func TestCatalogSaveFailureIsReported(t *testing.T) {
	root := t.TempDir()

	// Make the catalog path unwritable by putting a FILE where the catalog's
	// parent directory must be. MkdirAll then fails for a real reason rather
	// than a simulated one.
	catPath := GetCatalogPath(root)
	parent := filepath.Dir(catPath)
	if err := os.MkdirAll(filepath.Dir(parent), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parent, []byte("not a directory"), 0o644); err != nil {
		t.Skipf("could not stage an unwritable catalog path: %v", err)
	}

	proj := filepath.Join(root, "demo")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := RegisterOrUpdateProject("demo", proj, "claude", nil, root)
	if err == nil {
		t.Fatalf("a project was registered with an unwritable catalog and reported success (got %+v); "+
			"it would vanish on restart", got)
	}
	if !strings.Contains(err.Error(), "catalog could not be saved") {
		t.Errorf("error does not say the catalog failed to save: %v", err)
	}
	// The registration itself still happened in memory, so returning the
	// project alongside the error is honest: the caller can see WHAT was
	// registered and that it did not persist.
	if got == nil {
		t.Error("no project returned; the caller cannot tell what failed to persist")
	}
}

// The success path must stay silent — an error type that fires when nothing is
// wrong is worse than none.
func TestCatalogSaveSucceedsNormally(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "demo")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterOrUpdateProject("demo", proj, "claude", nil, root); err != nil {
		t.Errorf("a normal registration failed: %v", err)
	}
	if _, err := os.Stat(GetCatalogPath(root)); err != nil {
		t.Errorf("catalog was not written: %v", err)
	}
}
