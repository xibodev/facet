package installer

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestZIPRejectsUnsafeAndDuplicateEntries(t *testing.T) {
	for _, entries := range [][]string{{"../escape"}, {"/absolute"}, {"C:/escape"}, {"a", "A"}, {"a.", "b"}, {"a/../../b"}} {
		t.Run(strings.Join(entries, "_"), func(t *testing.T) {
			root := t.TempDir()
			archive := filepath.Join(root, "bad.zip")
			f, err := os.Create(archive)
			if err != nil {
				t.Fatal(err)
			}
			w := zip.NewWriter(f)
			for _, name := range entries {
				entry, err := w.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				entry.Write([]byte("unsafe"))
			}
			w.Close()
			f.Close()
			if err := extractZip(archive, filepath.Join(root, "stage")); err == nil {
				t.Fatal("accepted unsafe archive")
			}
		})
	}
}

func TestArchivesRejectLinks(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "link.zip")
	f, _ := os.Create(zipPath)
	w := zip.NewWriter(f)
	h := &zip.FileHeader{Name: "link"}
	h.SetMode(os.ModeSymlink | 0777)
	e, _ := w.CreateHeader(h)
	e.Write([]byte("../escape"))
	w.Close()
	f.Close()
	if err := extractZip(zipPath, filepath.Join(root, "zip")); err == nil {
		t.Fatal("accepted ZIP symlink")
	}
	tarPath := filepath.Join(root, "link.tar.gz")
	f, _ = os.Create(tarPath)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "link", Linkname: "../escape", Typeflag: tar.TypeSymlink})
	tw.Close()
	gz.Close()
	f.Close()
	if err := extractTar(tarPath, filepath.Join(root, "tar")); err == nil {
		t.Fatal("accepted tar symlink")
	}
}

func TestChecksumMustMatchOneExactAsset(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "payload.zip")
	sums := filepath.Join(root, "sums")
	fixtureFile(t, archive, "payload")
	hash, _ := digest(archive)
	for _, content := range []string{"", hash + "  another.zip\n", strings.Repeat("0", 64) + "  asset.zip\n", hash + "  asset.zip\n" + hash + "  asset.zip\n"} {
		fixtureFile(t, sums, content)
		if err := verifyChecksum(archive, sums, "asset.zip"); err == nil {
			t.Fatal("accepted missing, wrong or duplicate checksum")
		}
	}
	fixtureFile(t, sums, hash+"  asset.zip\n")
	if err := verifyChecksum(archive, sums, "asset.zip"); err != nil {
		t.Fatal(err)
	}
}

func TestFourHostsPreserveUserInstructions(t *testing.T) {
	for host, config := range hosts {
		t.Run(host, func(t *testing.T) {
			root := t.TempDir()
			install := filepath.Join(root, "installed release")
			project := filepath.Join(root, "my project")
			fixtureFile(t, filepath.Join(install, "bundle", "skills", "facet", "SKILL.md"), "---\nname: facet\ndescription: Video production\n---\n")
			fixtureFile(t, filepath.Join(install, "bundle", "packs", "explainer", "SKILL.md"), "pack")
			fixtureFile(t, filepath.Join(project, "AGENTS.md"), "user instructions")
			fixtureFile(t, filepath.Join(project, "CLAUDE.md"), "user Claude instructions")
			fixtureFile(t, filepath.Join(project, ".facet.yaml"), "user config")
			s := setup{root: install, project: project, target: host, version: "1.0.2", components: "none", out: &bytes.Buffer{}}
			if err := s.integrate(); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(filepath.Join(project, config, "skills", "facet", "SKILL.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(b), "run-facet") {
				t.Fatal("missing runtime binding")
			}
			if _, err := os.Stat(filepath.Join(project, config, "skills", "facet", "packs")); !os.IsNotExist(err) {
				t.Fatal("packs must not leak into recursive host discovery")
			}
			if _, err := os.Stat(filepath.Join(project, ".facet-install", "packs", "explainer", "SKILL.md")); err != nil {
				t.Fatal(err)
			}
			for file, want := range map[string]string{"AGENTS.md": "user instructions", "CLAUDE.md": "user Claude instructions", ".facet.yaml": "user config"} {
				b, _ := os.ReadFile(filepath.Join(project, file))
				if string(b) != want {
					t.Fatalf("overwrote %s", file)
				}
			}
			if err := s.integrate(); err == nil {
				t.Fatal("overwrote existing integration")
			}
			if host == "codex" {
				if _, err := os.Stat(filepath.Join(project, ".codex")); !os.IsNotExist(err) {
					t.Fatal("used obsolete Codex directory")
				}
			}
		})
	}
}

func TestReceiptDetectsChangedRelease(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bin", executable("facet"))
	fixtureFile(t, path, "binary")
	if err := saveReceipt(root, "1.0.2"); err != nil {
		t.Fatal(err)
	}
	if err := verifyReceipt(root, "1.0.2"); err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, path, "changed")
	if err := verifyReceipt(root, "1.0.2"); err == nil {
		t.Fatal("accepted modified release")
	}
}

func TestInputAndComponentValidation(t *testing.T) {
	for _, args := range [][]string{{"--yes"}, {"--yes", "--target", "unknown", "--project", "x"}, {"--version", "../evil"}, {"--archive", "x"}, {"--yes", "--target", "codex", "--project", "x", "--components", "all"}} {
		if err := Run(args, strings.NewReader(""), &bytes.Buffer{}, "1.0.2"); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	if err := Run([]string{"--help"}, strings.NewReader(""), &bytes.Buffer{}, "1.0.2"); err != nil {
		t.Fatal(err)
	}
	if err := Run(nil, strings.NewReader(""), &bytes.Buffer{}, "1.0.2"); err == nil {
		t.Fatal("EOF silently accepted")
	}
}

func TestInteractiveChecklistAndCancellation(t *testing.T) {
	root := t.TempDir()
	out := &bytes.Buffer{}
	input := strings.NewReader("codex\n" + filepath.Join(root, "project") + "\n1,3\nn\n")
	err := Run([]string{"--install-dir", filepath.Join(root, "release")}, input, out, "1.0.2")
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancellation: %v", err)
	}
	if !strings.Contains(out.String(), "Optional components: remotion,gflow") {
		t.Fatal(out.String())
	}
	if _, err := os.Stat(filepath.Join(root, "release")); !os.IsNotExist(err) {
		t.Fatal("cancelled setup wrote release")
	}
}

func TestDownloadRejectsHTTPFailureAndExistingDestination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("verified separately"))
	}))
	defer server.Close()
	root := t.TempDir()
	path := filepath.Join(root, "asset")
	if err := download(server.URL+"/missing", path); err == nil {
		t.Fatal("accepted 404")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("HTTP failure created file")
	}
	if err := download(server.URL+"/ok", path); err != nil {
		t.Fatal(err)
	}
	if err := download(server.URL+"/ok", path); err == nil {
		t.Fatal("overwrote existing download")
	}
}

func TestWindowsCommandShimPreservesArguments(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows script invocation")
	}
	root := t.TempDir()
	script := filepath.Join(root, "echo args.cmd")
	fixtureFile(t, script, "@echo off\r\necho %~1\r\n")
	out := &bytes.Buffer{}
	s := setup{out: out}
	if err := s.run(root, script, "path with spaces"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "path with spaces") {
		t.Fatal(out.String())
	}
}

func TestRuntimeCopyKeepsBundledNPM(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "target")
	path := filepath.Join("lib", "node_modules", "npm", "bin", "npm-cli.js")
	fixtureFile(t, filepath.Join(source, path), "npm runtime")
	if err := copyTree(source, target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, path)); err != nil {
		t.Fatal("runtime copy dropped npm:", err)
	}
}
