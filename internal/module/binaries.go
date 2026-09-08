package module

import (
	"fmt"
	"sort"
	"strings"

	"github.com/xibodev/facet/internal/toolbox"
)

// declaredBinaries is the set Facet may ever be granted. It matches
// Permissions.Subprocess and is derived from actual invocation sites.
var declaredBinaries = map[string]bool{
	"ffmpeg": true, "ffprobe": true, "gflow": true,
	"node": true, "npx": true, "piper": true,
}

// ValidateBinaries checks a host-supplied binary map before any of it is used.
//
// The rules exist because a module running under the host inherits NO
// environment: this map is the module's entire execution authority, so a bad
// entry is an authority problem rather than a convenience problem.
//
//   - A name Facet never declared is REFUSED rather than ignored. Silently
//     dropping it would let a host believe it granted something Facet honours.
//   - A relative path is REFUSED. Relative means "search", and a search can
//     resolve to a binary the host never authorized.
//   - An empty value is REFUSED. "Could not resolve" must arrive as absence,
//     not as an empty string, so the two stay distinguishable — the same
//     null-versus-zero rule the cost model rests on.
//
// Facet never falls back to PATH, a home directory, or its own lookup when a
// binary is missing. A fallback would quietly restore the ambient authority the
// host removed and make its enforcement decorative.
func ValidateBinaries(bins map[string]string) error {
	names := make([]string, 0, len(bins))
	for name := range bins {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		path := bins[name]
		if !declaredBinaries[name] {
			return fmt.Errorf(
				"host supplied binary %q which Facet never declared in permissions.subprocess", name)
		}
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf(
				"binary %q was supplied with an empty path; an unresolvable binary must be absent, not empty", name)
		}
		if !isAbsoluteBinaryPath(path) {
			return fmt.Errorf(
				"binary %q path %q is not absolute; a relative path is a search and may resolve to an unauthorized binary",
				name, path)
		}
	}
	return nil
}

// isAbsoluteBinaryPath accepts BOTH POSIX and Windows absolute shapes,
// regardless of the platform this code is running on.
//
// filepath.IsAbs is platform-specific: on Windows it rejects "/usr/bin/ffprobe"
// and on POSIX it rejects "C:\\tools\\ffmpeg.exe". Using it here would make a
// module refuse a perfectly good grant from a host on the other platform — and
// the host resolves these paths, so it is the host's platform that decides
// their shape, not the module's.
func isAbsoluteBinaryPath(p string) bool {
	if p == "" {
		return false
	}
	// POSIX absolute, and Windows UNC/rooted forms.
	if p[0] == '/' || p[0] == '\\' {
		return true
	}
	// Windows drive-letter form: C:\ or C:/
	if len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		c := p[0]
		return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
	}
	return false
}

// RequireBinary reports the absolute path for a declared binary, or a
// dependency_missing failure naming it.
//
// This fails CLOSED: an ungranted binary is an error, never an invitation to
// look it up.
func RequireBinary(bins map[string]string, name string) (string, error) {
	path, ok := bins[name]
	if !ok || strings.TrimSpace(path) == "" {
		return "", fmt.Errorf(
			"binary %q was not supplied by the host; declare it in permissions.subprocess "+
				"and have the host resolve it (Facet does not search PATH)", name)
	}
	return path, nil
}

// useBinaries installs the host-supplied paths as the toolbox's resolution
// override and returns a function restoring the previous state.
//
// Scoping it to a single invocation matters: authority is granted per call, so
// it must not outlive the call that was granted it. An empty map clears the
// override rather than leaving a previous invocation's grant in place.
func useBinaries(bins map[string]string) (restore func()) {
	toolbox.SetBinaryPaths(bins)
	return func() { toolbox.SetBinaryPaths(nil) }
}

// useBundleRoot installs a host-supplied read-only bundle location for one
// invocation and returns a function restoring discovery.
//
// Scoped per call for the same reason binary grants are: authority is granted
// per invocation and must not outlive the call it was granted for.
func useBundleRoot(path string) (restore func()) {
	toolbox.SetBundleRoot(path)
	return func() { toolbox.SetBundleRoot("") }
}
