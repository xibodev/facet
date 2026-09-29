package toolbox

import (
	"os/exec"
	"strings"
	"sync"
)

// Binary resolution.
//
// The toolbox normally finds an executable on PATH. Under a module host that is
// impossible: a detached module inherits NO environment, so PATH is empty and
// every LookPath fails even though the binary exists. The host instead resolves
// each binary the module DECLARED and supplies absolute paths per invocation.
//
// SetBinaryPaths installs those paths as an override consulted before PATH.
// It exists so the toolbox stays mechanical and unaware of the module protocol:
// the module adapter owns the protocol and simply tells the toolbox where the
// binaries are.
//
// An absolute path is an identity rather than a search, so an override cannot
// resolve to something other than what the host authorized.
var (
	binaryMu    sync.RWMutex
	binaryPaths map[string]string
)

// SetBinaryPaths replaces the resolution override. A nil or empty map restores
// ordinary PATH lookup, which is what the human-facing `facet tools` CLI uses.
//
// Names are matched case-insensitively and without any extension, so a host may
// supply "ffmpeg" for ffmpeg.exe.
func SetBinaryPaths(paths map[string]string) {
	binaryMu.Lock()
	defer binaryMu.Unlock()
	if len(paths) == 0 {
		binaryPaths = nil
		return
	}
	next := make(map[string]string, len(paths))
	for name, path := range paths {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" || strings.TrimSpace(path) == "" {
			continue
		}
		next[key] = path
	}
	binaryPaths = next
}

// lookPath resolves a program name, preferring a host-supplied absolute path.
//
// This is the single resolution point for the whole toolbox. Falling back to
// PATH is correct for the CLI, where the user's environment is the authority;
// under a host the override is populated and PATH is empty, so the fallback
// simply fails and reports the binary as unavailable rather than silently
// finding an unauthorized one.
func lookPath(program string) (string, error) {
	binaryMu.RLock()
	override, ok := binaryPaths[strings.ToLower(strings.TrimSpace(program))]
	binaryMu.RUnlock()
	if ok {
		return override, nil
	}
	return exec.LookPath(program)
}

// LookPathForTest exposes the resolution seam so a sibling package can verify
// that a host-supplied grant was actually installed rather than only validated.
func LookPathForTest(program string) (string, error) { return lookPath(program) }
