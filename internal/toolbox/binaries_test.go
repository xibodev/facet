package toolbox

import (
	"os/exec"
	"testing"
)

// The override must be preferred over PATH, and clearing it must restore
// ordinary lookup. The human-facing `facet tools` CLI depends on the fallback.
func TestBinaryOverridePreferredThenRestored(t *testing.T) {
	t.Cleanup(func() { SetBinaryPaths(nil) })

	fake := `C:\host\supplied\ffprobe.exe`
	SetBinaryPaths(map[string]string{"ffprobe": fake})
	got, err := lookPath("ffprobe")
	if err != nil {
		t.Fatalf("override not used: %v", err)
	}
	if got != fake {
		t.Errorf("lookPath = %q, want the host-supplied %q", got, fake)
	}

	// Case-insensitive, since a host may supply "FFprobe" for ffprobe.exe.
	if got, _ := lookPath("FFprobe"); got != fake {
		t.Errorf("case-insensitive lookup failed: %q", got)
	}

	// An unlisted binary is NOT invented; it falls through to PATH.
	if _, err := lookPath("definitely-not-a-real-binary-xyz"); err == nil {
		t.Error("an unlisted binary resolved; the override must not invent paths")
	}

	SetBinaryPaths(nil)
	real, realErr := exec.LookPath("ffprobe")
	got2, err2 := lookPath("ffprobe")
	if (realErr == nil) != (err2 == nil) || (realErr == nil && got2 != real) {
		t.Errorf("PATH lookup not restored: got %q/%v, want %q/%v", got2, err2, real, realErr)
	}
}

// An empty or nil map clears the override rather than leaving a previous
// invocation's grant in place. Authority is per-invocation.
func TestEmptyBinaryMapClearsGrant(t *testing.T) {
	t.Cleanup(func() { SetBinaryPaths(nil) })

	granted := `C:\a\ffprobe.exe`
	SetBinaryPaths(map[string]string{"ffprobe": granted})
	SetBinaryPaths(map[string]string{})
	if got, _ := lookPath("ffprobe"); got == granted {
		t.Error("a cleared grant survived; authority must not outlive its invocation")
	}

	// Blank names and blank paths are dropped rather than stored.
	SetBinaryPaths(map[string]string{"": `C:\x.exe`, "ffprobe": "   "})
	if got, _ := lookPath("ffprobe"); got == "   " {
		t.Error("a blank path was installed as an override")
	}
}
