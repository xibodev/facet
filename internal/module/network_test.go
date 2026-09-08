package module

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The declared network allowlist must cover every host the toolbox actually
// contacts.
//
// The host grants exactly what the descriptor asks for, so a missing entry is
// a runtime failure with no useful cause, and a wrong entry is a permission
// the operator never knowingly gave. Three were missing when this test was
// written: queue.fal.run (fal's async queue — a different host from fal.run,
// polled for Kling status), raw.githubusercontent.com (the Hyperframes
// registry) and cdn.jsdelivr.net (the GSAP runtime in generated HTML).
//
// The list is derived from source rather than maintained by hand, because a
// hand-maintained list is exactly what drifted.
func TestDeclaredNetworkCoversEveryContactedHost(t *testing.T) {
	env := Describe("test")
	if !env.OK {
		t.Fatalf("describe failed: %+v", env.Error)
	}
	desc, ok := env.Result.(Descriptor)
	if !ok {
		t.Fatalf("result is %T, not a Descriptor", env.Result)
	}
	declared := map[string]bool{}
	for _, h := range desc.Permissions.Network {
		declared[h] = true
	}

	dir := filepath.Join("..", "toolbox")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("toolbox sources unavailable: %v", err)
	}

	url := regexp.MustCompile(`https://([a-z0-9.-]+)`)
	var missing []string
	seen := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		for _, m := range url.FindAllStringSubmatch(string(raw), -1) {
			host := m[1]
			switch {
			// Not runtime destinations: a Go import path, the project's own
			// repository in a User-Agent, and a deliberately unroutable host
			// used by tests.
			case host == "github.com", host == "example.invalid":
				continue
			case declared[host], seen[host]:
				continue
			}
			seen[host] = true
			missing = append(missing, host+" ("+name+")")
		}
	}
	sort.Strings(missing)
	for _, m := range missing {
		t.Errorf("host is contacted but not declared in permissions.network: %s", m)
	}
}
