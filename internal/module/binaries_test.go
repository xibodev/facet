package module

import (
	"encoding/json"
	"testing"

	"github.com/xibodev/facet/internal/toolbox"
)

// A module under the host inherits no environment, so the binary map is its
// entire execution authority. These rules are authority checks, not validation
// niceties.
// The host's binary map must be USED, not merely validated.
//
// This is the bug this test exists to prevent: `Request.Binaries` was declared
// in the type, validated on every invocation, and then dropped — so under a
// host supplying no environment every subprocess still resolved against an
// empty PATH and failed `dependency_missing`. Declared, checked, never wired.
// It is the same shape as ExternalWrite being declared and never assigned.
func TestHostBinariesAreActuallyUsed(t *testing.T) {
	granted := `C:\host\granted\ffprobe.exe`

	restore := useBinaries(map[string]string{"ffprobe": granted})
	got, err := toolbox.LookPathForTest("ffprobe")
	if err != nil {
		t.Fatalf("granted binary did not resolve: %v", err)
	}
	if got != granted {
		t.Errorf("resolved %q, want the host-granted %q; the map was not installed", got, granted)
	}
	restore()

	// The grant must not outlive the invocation it was granted for.
	after, _ := toolbox.LookPathForTest("ffprobe")
	if after == granted {
		t.Error("the grant survived the invocation; authority is per-call")
	}
}

func TestBinaryMapRules(t *testing.T) {
	abs := "C:\\tools\\ffmpeg.exe"
	if testing.Short() {
		abs = "/usr/bin/ffmpeg"
	}

	t.Run("undeclared binary is refused", func(t *testing.T) {
		err := ValidateBinaries(map[string]string{"curl": abs})
		if err == nil {
			t.Error("a binary Facet never declared was accepted; " +
				"silently ignoring it would let the host believe the grant was honoured")
		}
	})

	t.Run("empty path is refused", func(t *testing.T) {
		if err := ValidateBinaries(map[string]string{"ffmpeg": ""}); err == nil {
			t.Error("empty path accepted; unresolvable must be ABSENT, not empty")
		}
	})

	t.Run("relative path is refused", func(t *testing.T) {
		for _, rel := range []string{"ffmpeg", "./ffmpeg", "bin/ffmpeg"} {
			if err := ValidateBinaries(map[string]string{"ffmpeg": rel}); err == nil {
				t.Errorf("relative path %q accepted; a search may resolve to an "+
					"unauthorized binary", rel)
			}
		}
	})

	t.Run("empty map is valid", func(t *testing.T) {
		// A capability needing no subprocess is legitimate.
		if err := ValidateBinaries(nil); err != nil {
			t.Errorf("nil binary map rejected: %v", err)
		}
	})

	// The HOST resolves these paths, so the host's platform decides their
	// shape — not the module's. filepath.IsAbs is platform-specific and would
	// have made a Windows module refuse a valid POSIX grant from a Linux host,
	// and vice versa. Found because a checked-in request example using
	// /absolute/path/to/ffprobe was rejected on Windows.
	t.Run("both platform absolute shapes are accepted", func(t *testing.T) {
		for _, p := range []string{
			"/usr/bin/ffmpeg",
			"/opt/homebrew/bin/ffmpeg",
			"C:\\tools\\ffmpeg.exe",
			"C:/tools/ffmpeg.exe",
			"\\\\server\\share\\ffmpeg.exe",
		} {
			if err := ValidateBinaries(map[string]string{"ffmpeg": p}); err != nil {
				t.Errorf("absolute path %q rejected: %v", p, err)
			}
		}
	})
}

// RequireBinary must fail closed: an ungranted binary is an error, never a
// reason to search PATH.
func TestRequireBinaryFailsClosed(t *testing.T) {
	bins := map[string]string{"ffprobe": "/usr/bin/ffprobe"}

	if _, err := RequireBinary(bins, "ffmpeg"); err == nil {
		t.Error("an ungranted binary resolved; Facet must not fall back to PATH")
	}
	if _, err := RequireBinary(bins, "ffprobe"); err != nil {
		t.Errorf("a granted binary failed to resolve: %v", err)
	}
	if _, err := RequireBinary(map[string]string{"ffmpeg": "  "}, "ffmpeg"); err == nil {
		t.Error("a whitespace path resolved")
	}
}

// A bad binary map must be refused before any tool runs, and must surface as a
// protocol envelope rather than a panic or a silent skip.
func TestInvokeRefusesBadBinaryMap(t *testing.T) {
	body, _ := json.Marshal(Request{
		RequestID: "req_bad_bins",
		Tool:      "media_probe",
		Input:     json.RawMessage(`{"input":"x.mp4"}`),
		Binaries:  map[string]string{"curl": "/usr/bin/curl"},
	})
	env := Invoke(CapToolsRun, body)
	if env.OK {
		t.Fatal("invocation proceeded with an undeclared binary granted")
	}
	if env.Error.Code != "invalid_request" {
		t.Errorf("error code = %q, want invalid_request", env.Error.Code)
	}
	if err := ValidateEnvelopeBytes(mustMarshal(t, env)); err != nil {
		t.Errorf("refusal envelope is not protocol-valid: %v", err)
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
