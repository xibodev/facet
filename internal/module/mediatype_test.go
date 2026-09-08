package module

import (
	"os"
	"path/filepath"
	"testing"
)

// A host picks how to present an artefact from its media type, so an artefact
// without one is offered as a file to download rather than the video, audio or
// image it is. Detection reads the bytes: a declared type is a claim, and a
// filename is not evidence at all.
func TestArtifactMediaTypeFromBytes(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, body []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, body, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	pad := func(prefix []byte) []byte {
		b := make([]byte, 64)
		copy(b, prefix)
		return b
	}

	cases := []struct {
		name, file string
		body       []byte
		want       string
	}{
		// net/http knows these.
		{"jpeg", "a.jpg", pad([]byte{0xff, 0xd8, 0xff, 0xe0}), "image/jpeg"},
		{"png", "b.png", pad([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}), "image/png"},
		// net/http does NOT know these, and a media toolbox emits them constantly.
		{"mp3 without an ID3 tag", "c.mp3", pad([]byte{0xff, 0xf3, 0x64, 0xc4}), "audio/mpeg"},
		{"mp4", "d.mp4", pad([]byte{0, 0, 0, 0x20, 'f', 't', 'y', 'p'}), "video/mp4"},
		{"flac", "e.flac", pad([]byte("fLaC")), "audio/flac"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := detectArtifactMediaType(write(c.file, c.body)); got != c.want {
				t.Errorf("detected %q, want %q", got, c.want)
			}
		})
	}

	t.Run("an unidentifiable file reports nothing", func(t *testing.T) {
		// Reporting application/octet-stream tells a host nothing it did not
		// already know, so say nothing rather than something meaningless.
		if got := detectArtifactMediaType(write("f.bin", pad([]byte{0x01, 0x02, 0x03}))); got != "" {
			t.Errorf("detected %q for unidentifiable bytes, want empty", got)
		}
	})

	t.Run("the extension is never trusted", func(t *testing.T) {
		// An mp4 written with a .png name is an mp4. A host that trusted the
		// name would offer the wrong viewer.
		p := write("lying.png", pad([]byte{0, 0, 0, 0x20, 'f', 't', 'y', 'p'}))
		if got := detectArtifactMediaType(p); got != "video/mp4" {
			t.Errorf("detected %q, want video/mp4 from the bytes", got)
		}
	})

	t.Run("a missing file reports nothing", func(t *testing.T) {
		if got := detectArtifactMediaType(filepath.Join(dir, "absent")); got != "" {
			t.Errorf("detected %q for a missing file", got)
		}
	})
}

// A tool builds its result with a concrete slice type, so a single []any
// assertion silently yields nothing for half of them: the field is present, the
// assertion fails, and the artefacts vanish with no error. frame_sample returns
// []map[string]any and reported zero artifacts for exactly that reason.
func TestMapSliceHandlesBothConcreteShapes(t *testing.T) {
	typed := []map[string]any{{"path": "a.jpg"}, {"path": "b.jpg"}}
	if got := mapSlice(typed); len(got) != 2 {
		t.Errorf("[]map[string]any yielded %d entries, want 2", len(got))
	}
	generic := []any{map[string]any{"path": "a.jpg"}}
	if got := mapSlice(generic); len(got) != 1 {
		t.Errorf("[]any yielded %d entries, want 1", len(got))
	}
	if got := mapSlice("not a slice"); got != nil {
		t.Error("a non-slice produced entries")
	}
	if got := mapSlice(nil); got != nil {
		t.Error("nil produced entries")
	}
}
