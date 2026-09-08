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

// The declared credential list must cover every API key the toolbox reads.
//
// A host grants exactly what the descriptor declares, so an undeclared key is
// never supplied: the tool reports itself configured, then fails to
// authenticate. Three were missing when this was written — FLUX_API_KEY,
// KLING_API_KEY and GOOGLE_API_KEY — all belonging to providers already
// declared as paid, which is what made the gap easy to miss.
//
// Only *_API_KEY and *_KEY names are treated as credentials; FACET_* and
// path-like variables are configuration, not secrets.
func TestDeclaredCredentialsCoverEveryKeyRead(t *testing.T) {
	env := Describe("test")
	if !env.OK {
		t.Fatalf("describe failed: %+v", env.Error)
	}
	desc, ok := env.Result.(Descriptor)
	if !ok {
		t.Fatalf("result is %T, not a Descriptor", env.Result)
	}
	declared := map[string]bool{}
	for _, c := range desc.Permissions.Credentials {
		declared[c] = true
	}

	dir := filepath.Join("..", "toolbox")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("toolbox sources unavailable: %v", err)
	}

	getenv := regexp.MustCompile(`os\.Getenv\("([A-Z_0-9]+)"\)`)
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
		for _, m := range getenv.FindAllStringSubmatch(string(raw), -1) {
			key := m[1]
			if !strings.HasSuffix(key, "_KEY") || declared[key] || seen[key] {
				continue
			}
			seen[key] = true
			missing = append(missing, key+" ("+name+")")
		}
	}
	sort.Strings(missing)
	for _, m := range missing {
		t.Errorf("credential is read but not declared in permissions.credentials: %s", m)
	}
}
