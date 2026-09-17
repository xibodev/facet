package installer

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type receipt struct {
	Version string            `json:"version"`
	Files   map[string]string `json:"files"`
}

func saveReceipt(root, version string) error {
	r := receipt{Version: version, Files: map[string]string{}}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		hash, err := digest(path)
		if err != nil {
			return err
		}
		r.Files[filepath.ToSlash(rel)] = hash
		return nil
	})
	if err != nil {
		return err
	}
	return writeJSON(filepath.Join(root, "facet-install.json"), r)
}

func verifyReceipt(root, version string) error {
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("installation must be a real directory")
	}
	b, err := os.ReadFile(filepath.Join(root, "facet-install.json"))
	if err != nil {
		return fmt.Errorf("existing directory is not managed by this installer; choose a fresh --install-dir")
	}
	var r receipt
	if err = json.Unmarshal(b, &r); err != nil {
		return err
	}
	if r.Version != version || len(r.Files) == 0 {
		return fmt.Errorf("installation receipt version/content mismatch")
	}
	if _, ok := r.Files["bin/"+executable("facet")]; !ok {
		return fmt.Errorf("receipt does not contain Facet")
	}
	for path, hash := range r.Files {
		if !fs.ValidPath(path) || strings.ContainsAny(path, ":\\") {
			return fmt.Errorf("invalid receipt path")
		}
		file := filepath.Join(root, filepath.FromSlash(path))
		for ancestor := file; ancestor != root; ancestor = filepath.Dir(ancestor) {
			info, err := os.Lstat(ancestor)
			if err != nil || info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("installed entry missing or replaced by link: %s", path)
			}
		}
		actual, err := digest(file)
		if err != nil || actual != hash {
			return fmt.Errorf("installed file changed or missing: %s; choose a fresh --install-dir", path)
		}
	}
	return nil
}

func preflightProject(project, host string) error {
	for _, path := range []string{filepath.Join(project, ".facet-install"), filepath.Join(project, hosts[host], "skills", "facet")} {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("preserving existing project entry %s; select a fresh project", path)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	// Never traverse a host-config link while installing project files.
	for path := filepath.Join(project, hosts[host], "skills"); ; path = filepath.Dir(path) {
		if info, err := os.Lstat(path); err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return fmt.Errorf("project ancestor is not a real directory: %s", path)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if path == project || filepath.Dir(path) == path {
			break
		}
	}
	return nil
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func psQuote(s string) string    { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func (s *setup) integrate() error {
	if err := preflightProject(s.project, s.target); err != nil {
		return err
	}
	if err := os.MkdirAll(s.project, 0755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(s.project, ".facet-install-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	skill := filepath.Join(stage, "skill")
	if err := copyTree(filepath.Join(s.root, "bundle", "skills", "facet"), skill); err != nil {
		return err
	}
	// Pack resources live outside skills/: some hosts recursively discover every
	// SKILL.md and would otherwise register generic names such as 'social'.
	launcher := filepath.Join(s.project, ".facet-install", "run-facet.sh")
	binary := filepath.Join(s.root, "bin", executable("facet"))
	paths := dependencyPaths(s.root)
	invocation := shellQuote(launcher)
	content := "#!/bin/sh\nexport PATH=" + shellQuote(strings.Join(paths, string(os.PathListSeparator))) + ":\"$PATH\"\nexec " + shellQuote(binary) + " \"$@\"\n"
	if runtime.GOOS == "windows" {
		launcher = filepath.Join(s.project, ".facet-install", "run-facet.ps1")
		invocation = "& " + psQuote(launcher)
		content = "$ErrorActionPreference = 'Stop'\n$env:PATH = " + psQuote(strings.Join(paths, ";")+";") + " + $env:PATH\n& " + psQuote(binary) + " @args\nexit $LASTEXITCODE\n"
	}
	state := filepath.Join(stage, "state")
	if err = os.Mkdir(state, 0755); err != nil {
		return err
	}
	if err = copyTree(filepath.Join(s.root, "bundle", "packs"), filepath.Join(state, "packs")); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(state, filepath.Base(launcher)), []byte(content), 0755); err != nil {
		return err
	}
	if err = writeJSON(filepath.Join(state, "installation.json"), map[string]any{"version": s.version, "installation": s.root, "host": s.target, "components": s.components}); err != nil {
		return err
	}
	corePath := filepath.Join(skill, "SKILL.md")
	b, err := os.ReadFile(corePath)
	if err != nil {
		return err
	}
	b = append(b, []byte(fmt.Sprintf("\n## This installation\n- Run Facet through `%s` followed by the normal arguments. This launcher selects the installed binary and dependencies; use it instead of bare `facet` in examples.\n- Bundle: `%s`. Resolve all `packs/...` references under `%s`. Load a relevant pack's SKILL.md on demand. Existing project runtime configuration takes precedence over bundle discovery.\n- Installed optional components: %s. A component being installed does not prove external service access. Missing API keys or provider sessions are reported by the tools; help the user configure only the service needed for their request.\n- Local editing and rendering do not require media-provider API keys. Edge TTS is keyless but requires network access.\n", invocation, filepath.Join(s.root, "bundle"), filepath.Join(s.project, ".facet-install"), s.components))...)
	if s.has("hyperframes") {
		entry := filepath.Join(s.root, "dependencies", "hyperframes", "node_modules", "hyperframes", "bin", "hyperframes.mjs")
		b = append(b, []byte("- HyperFrames: invoke `node \""+entry+"\" <command>` directly for the pinned renderer (including on Windows); avoid unpinned `npx hyperframes`. Use `--help` for its current contract.\n")...)
	}
	if s.has("gflow") {
		b = append(b, []byte("- Optional gflow executable: `"+filepath.Join(s.root, "dependencies", "gflow", executable("gflow"))+"`. Its `--help` and errors describe provider setup. Do not assume a browser extension is required.\n")...)
	}
	if s.has("piper") {
		b = append(b, []byte("- For Piper, pass the absolute `model` path `"+filepath.Join(s.root, "dependencies", "voices", "en_US-lessac-medium.onnx")+"`.\n")...)
	}
	if err = os.WriteFile(corePath, b, 0644); err != nil {
		return err
	}
	target := filepath.Join(s.project, hosts[s.target], "skills", "facet")
	if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	if err = os.Rename(state, filepath.Join(s.project, ".facet-install")); err != nil {
		return err
	}
	if err = os.Rename(skill, target); err != nil {
		os.RemoveAll(filepath.Join(s.project, ".facet-install"))
		return err
	}
	fmt.Fprintln(s.out, "Registered Facet skill:", target)
	return nil
}
