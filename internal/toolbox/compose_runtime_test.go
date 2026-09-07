package toolbox

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func composeRuntimeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func isolateComposeRuntime(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", "")
	t.Chdir(workspace)
	return home, workspace
}

func TestFindComposerDirHomeInstalled(t *testing.T) {
	home, _ := isolateComposeRuntime(t)
	want := filepath.Join(home, ".facet", "bundle", "remotion-composer")
	composeRuntimeFixture(t, filepath.Join(want, "package.json"), "{}")
	got, err := findComposerDir()
	if err != nil || got != want {
		t.Fatalf("findComposerDir() = %q, %v; want %q", got, err, want)
	}
}

func TestFindComposerDirConfigured(t *testing.T) {
	for _, scope := range []string{"local", "global"} {
		for _, key := range []string{"remotion_composer", "bundle"} {
			t.Run(scope+"/"+key, func(t *testing.T) {
				home, workspace := isolateComposeRuntime(t)
				root := filepath.Join(home, "custom runtime")
				want := root
				if key == "bundle" {
					want = filepath.Join(root, "remotion-composer")
				}
				composeRuntimeFixture(t, filepath.Join(want, "package.json"), "{}")
				composeRuntimeFixture(t, filepath.Join(workspace, "remotion-composer", "package.json"), "{}")
				configPath := filepath.Join(home, ".config", "facet", "config.yaml")
				if scope == "local" {
					composeRuntimeFixture(t, configPath, "paths:\n  remotion_composer: missing-global-runtime\n")
					configPath = filepath.Join(workspace, ".facet.yaml")
				}
				composeRuntimeFixture(t, configPath, "paths:\n  "+key+": '"+filepath.ToSlash(root)+"'\n")
				got, err := findComposerDir()
				if err != nil || got != want {
					t.Fatalf("findComposerDir() = %q, %v; want %q", got, err, want)
				}
			})
		}
	}
}

func TestFindComposerDirPreservesLocations(t *testing.T) {
	for _, location := range []string{"remotion-composer", "packs/explainer/runtime", "ancestor", "windows-current", "windows-runtime"} {
		t.Run(location, func(t *testing.T) {
			home, workspace := isolateComposeRuntime(t)
			want := filepath.Join(workspace, filepath.FromSlash(location))
			switch location {
			case "ancestor":
				want = filepath.Join(workspace, "remotion-composer")
				nested := filepath.Join(workspace, "a", "b", "c", "d")
				if err := os.MkdirAll(nested, 0755); err != nil {
					t.Fatal(err)
				}
				t.Chdir(nested)
			case "windows-current", "windows-runtime":
				t.Setenv("LOCALAPPDATA", home)
				want = filepath.Join(home, "Facet", "runtimes", "remotion")
				if location == "windows-current" {
					want = filepath.Join(want, "current")
				}
			}
			composeRuntimeFixture(t, filepath.Join(want, "package.json"), "{}")
			got, err := findComposerDir()
			if err != nil || got != want {
				t.Fatalf("findComposerDir() = %q, %v; want %q", got, err, want)
			}
		})
	}
}

func TestFindComposerDirRejectsInvalidConfig(t *testing.T) {
	for _, config := range []string{"paths:\n  remotion_composer: missing-runtime\n", "paths: ["} {
		t.Run(config, func(t *testing.T) {
			_, workspace := isolateComposeRuntime(t)
			composeRuntimeFixture(t, filepath.Join(workspace, "remotion-composer", "package.json"), "{}")
			composeRuntimeFixture(t, filepath.Join(workspace, ".facet.yaml"), config)
			if got, err := findComposerDir(); err == nil || got != "" {
				t.Fatalf("invalid config must not silently select another runtime: %q, %v", got, err)
			}
		})
	}
}

func TestRemotionRenderFailsClosed(t *testing.T) {
	for _, missing := range []string{"cli", "node", "failed-cli"} {
		t.Run(missing, func(t *testing.T) {
			if missing == "failed-cli" {
				if _, err := exec.LookPath("node"); err != nil {
					t.Skip("node is needed to test a failing CLI without rendering")
				}
			}
			home, workspace := isolateComposeRuntime(t)
			composer := filepath.Join(home, ".facet", "bundle", "remotion-composer")
			composeRuntimeFixture(t, filepath.Join(composer, "package.json"), "{}")
			composeRuntimeFixture(t, filepath.Join(composer, "src", "index.tsx"), "")
			if missing != "cli" {
				composeRuntimeFixture(t, filepath.Join(composer, "node_modules", "@remotion", "cli", "remotion-cli.js"), "process.stderr.write('intentional CLI failure'); process.exit(23);\n")
			}
			if missing == "node" {
				t.Setenv("PATH", t.TempDir())
			}
			outPath := filepath.Join(workspace, "output.mp4")
			result, _, err := doRemotionRender(composeRequest{}, outPath, time.Second*10)
			var failure *toolFailure
			if result != nil || !errors.As(err, &failure) {
				t.Fatalf("expected runtime failure, got result=%v err=%v", result, err)
			}
			wantCode, wantMessage := "dependency_missing", "Remotion render CLI"
			if missing == "node" {
				wantMessage = "node"
			} else if missing == "failed-cli" {
				wantCode, wantMessage = "command_failed", "node failed"
				if !strings.Contains(failure.err.Details["stderr"].(string), "intentional CLI failure") {
					t.Fatalf("CLI diagnostics lost: %v", failure.err.Details)
				}
			}
			if failure.err.Code != wantCode || !strings.Contains(failure.err.Message, wantMessage) {
				t.Fatalf("runtime error replaced by fallback: %+v", failure.err)
			}
			for _, path := range []string{outPath, filepath.Join(workspace, ".remotion_props.json")} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("unexpected render artifact %s: %v", path, err)
				}
			}
		})
	}
}

func TestRemotionTimeoutRetainsSafeProgress(t *testing.T) {
	workspace := composeDeliveryFixture(t, `
const fs = require('fs');
fs.writeFileSync(process.argv[5], 'partial render');
fs.writeSync(1, 'Bundling 100%\nConcurrency          8x\nRendered 12/60, time remaining: 1m 31s\n');
fs.writeSync(1, 'https://user:secret@example.test/media?token=private\n{"password":"private"}\n');
fs.writeSync(2, 'Bearer private\nsource code and user props\n');
setInterval(() => {}, 1000);
`)
	output := filepath.Join(workspace, "output.mp4")
	composeRuntimeFixture(t, output, "previous delivery")
	result, _, err := doRemotionRender(composeRequest{}, output, 2*time.Second)
	var failed *toolFailure
	if result != nil || !errors.As(err, &failed) || failed.err.Code != "command_timeout" {
		t.Fatalf("expected timeout without fallback: result=%v err=%v", result, err)
	}
	stdout, _ := failed.err.Details["output"].(string)
	for _, want := range []string{"Bundling 100%", "Concurrency          8x", "Rendered 12/60, time remaining: 1m 31s", "REDACTED"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("missing timeout progress %q: %v", want, failed.err.Details)
		}
	}
	for _, diagnostic := range []string{stdout, failed.err.Details["stderr"].(string)} {
		if len(diagnostic) > maxDiagnostic {
			t.Fatal("unbounded timeout diagnostic")
		}
		for _, secret := range []string{"private", "secret", "example.test", "password", "source code", "user props"} {
			if strings.Contains(diagnostic, secret) {
				t.Fatalf("sensitive renderer diagnostic exposed: %q", diagnostic)
			}
		}
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != "previous delivery" {
		t.Fatalf("previous delivery lost: %q, %v", data, err)
	}
	for _, pattern := range []string{".videokit-*", ".remotion_props.json"} {
		leftovers, err := filepath.Glob(filepath.Join(workspace, pattern))
		if err != nil || len(leftovers) != 0 {
			t.Fatalf("temporary artifacts left behind: %v, %v", leftovers, err)
		}
	}
}

func TestRemotionTimeoutDiagnosticBound(t *testing.T) {
	var log strings.Builder
	for i := 0; i <= 1000; i++ {
		fmt.Fprintf(&log, "Rendered %d/1000\r", i)
	}
	log.WriteString("Rendered 1/2, time remaining: credential\nConcurrency 8x token=private\n")
	got := remotionTimeoutDiagnostic(log.String())
	if len(got) > maxDiagnostic || !strings.Contains(got, "Rendered 1000/1000") || !strings.Contains(got, "REDACTED") || strings.Contains(got, "private") || strings.Contains(got, "credential") {
		t.Fatalf("unsafe or incomplete bounded progress: %q", got)
	}
}
