package toolbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePiperModelUsesInstalledVoiceForDefaultName(t *testing.T) {
	model := filepath.Join(t.TempDir(), "en_US-lessac-medium.onnx")
	if err := os.WriteFile(model, []byte("model"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FACET_PIPER_MODEL", model)

	for _, requested := range []string{"", "en_US-lessac-medium", "en_US-lessac-medium.onnx"} {
		if got := resolvePiperModel(requested); got != model {
			t.Errorf("resolvePiperModel(%q) = %q, want %q", requested, got, model)
		}
	}

	explicit := filepath.Join(t.TempDir(), "other.onnx")
	if got := resolvePiperModel(explicit); got != explicit {
		t.Errorf("explicit model = %q, want %q", got, explicit)
	}
}
