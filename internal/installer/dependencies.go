package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func (s *setup) has(name string) bool {
	for _, item := range strings.Split(s.components, ",") {
		if strings.TrimSpace(item) == name {
			return true
		}
	}
	return false
}

func (s *setup) selectComponents() error {
	fmt.Fprintln(s.out, `Core: FFmpeg + FFprobe — editing, audio mixing, inspection. Download varies by OS/package manager.
Optional components (download estimates, not installed sizes):
  1. remotion    Animated compositions/captions. About 150–300 MB including browser.
  2. piper       Offline speech. Runtime wheel about 34 MB + voice about 63 MB,
                 plus Python/ONNX dependencies (additional size varies).
  3. gflow       Image/video generation via gflow providers. About 3–4 MB;
                 external app/browser setup is separate and diagnosed by gflow.
  4. hyperframes HTML video renderer. Roughly 150–400 MB with dependencies/browser.
Node.js/npm: roughly 30–60 MB if needed by Remotion/HyperFrames.
Estimates are planning ranges, not exact totals; caches/platforms change downloads.
Existing compatible system runtimes are reused. Media services/API keys are optional.`)
	for _, name := range []string{"ffmpeg", "ffprobe", "node", "npm", "piper", "gflow"} {
		if path, err := exec.LookPath(name); err == nil {
			fmt.Fprintf(s.out, "  Found %s: %s\n", name, path)
		} else {
			fmt.Fprintf(s.out, "  %s: not on PATH\n", name)
		}
	}
	if s.yes {
		return nil
	}
	choice, err := s.ask("Select optional components (numbers or names, comma-separated; none)", s.components)
	if err != nil {
		return err
	}
	names := map[string]string{"1": "remotion", "2": "piper", "3": "gflow", "4": "hyperframes"}
	items := strings.Split(choice, ",")
	for i, item := range items {
		item = strings.TrimSpace(item)
		if n, ok := names[item]; ok {
			item = n
		}
		items[i] = item
	}
	s.components = strings.Join(items, ",")
	return nil
}

func (s *setup) validateComponents() error {
	seen := map[string]bool{}
	for _, item := range strings.Split(s.components, ",") {
		item = strings.TrimSpace(item)
		if item != "none" && item != "remotion" && item != "piper" && item != "gflow" && item != "hyperframes" {
			return fmt.Errorf("unknown optional component %q", item)
		}
		if seen[item] {
			return fmt.Errorf("duplicate component %s", item)
		}
		seen[item] = true
	}
	if seen["none"] && len(seen) != 1 {
		return fmt.Errorf("none cannot be combined with other components")
	}
	if seen["piper"] && runtime.GOOS == "windows" && runtime.GOARCH == "arm64" {
		return fmt.Errorf("Piper 1.8.0 has no Windows ARM64 wheel; deselect piper")
	}
	return nil
}

func dependencyPaths(root string) []string {
	venvBin := "bin"
	if runtime.GOOS == "windows" {
		venvBin = "Scripts"
	}
	paths := []string{filepath.Join(root, "dependencies", "node", "bin"), filepath.Join(root, "dependencies", "gflow"), filepath.Join(root, "dependencies", "piper", venvBin), filepath.Join(root, "dependencies", "hyperframes", "node_modules", ".bin")}
	// Record actual resolved system runtime locations for new agent shells too.
	for _, name := range []string{"ffmpeg", "ffprobe", "node", "npm"} {
		if p, err := exec.LookPath(name); err == nil {
			paths = append(paths, filepath.Dir(p))
		}
	}
	return paths
}

func (s *setup) approve(command string, args []string) error {
	fmt.Fprintf(s.out, "Dependency install: %s %q\n", command, args)
	if !s.yes {
		answer, err := s.ask("Run this command?", "y")
		if err != nil {
			return err
		}
		if answer != "y" && answer != "yes" {
			return fmt.Errorf("dependency setup declined; rerun when ready")
		}
	}
	return nil
}

func (s *setup) systemPackage(pkg string) error {
	var command string
	var args []string
	switch runtime.GOOS {
	case "windows":
		id := map[string]string{"ffmpeg": "Gyan.FFmpeg", "node": "OpenJS.NodeJS.LTS", "python": "Python.Python.3.12"}[pkg]
		command, args = "winget", []string{"install", "--id", id, "--exact", "--source", "winget", "--accept-package-agreements", "--accept-source-agreements"}
	case "darwin":
		id := map[string]string{"ffmpeg": "ffmpeg", "node": "node@24", "python": "python@3.12"}[pkg]
		command, args = "brew", []string{"install", id}
	case "linux":
		if pkg == "node" {
			return s.installLinuxNode()
		}
		command, args = "sudo", []string{"apt-get", "install", "-y", "ffmpeg"}
		if pkg == "python" {
			args = []string{"apt-get", "install", "-y", "python3", "python3-venv"}
		}
		if _, err := exec.LookPath("apt-get"); err != nil {
			return fmt.Errorf("install %s with your distribution's package manager, then rerun", pkg)
		}
	}
	if _, err := exec.LookPath(command); err != nil {
		return fmt.Errorf("%s is unavailable; install %s manually and rerun", command, pkg)
	}
	if err := s.approve(command, args); err != nil {
		return err
	}
	if err := s.run("", command, args...); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		b, err := exec.Command("powershell.exe", "-NoProfile", "-Command", "[Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [Environment]::GetEnvironmentVariable('Path','User')").Output()
		if err != nil {
			return err
		}
		os.Setenv("PATH", strings.TrimSpace(string(b))+";"+os.Getenv("PATH"))
	}
	if runtime.GOOS == "darwin" && (pkg == "node" || pkg == "python") {
		formula := "node@24"
		if pkg == "python" {
			formula = "python@3.12"
		}
		b, err := exec.Command("brew", "--prefix", formula).Output()
		if err != nil {
			return err
		}
		os.Setenv("PATH", filepath.Join(strings.TrimSpace(string(b)), "bin")+":"+os.Getenv("PATH"))
	}
	return nil
}

func (s *setup) installDependencies() error {
	os.Setenv("PATH", strings.Join(dependencyPaths(s.root), string(os.PathListSeparator))+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(name); err != nil {
			if err := s.systemPackage("ffmpeg"); err != nil {
				return err
			}
			break
		}
	}
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		b, err := exec.Command(name, "-version").Output()
		if err != nil {
			return fmt.Errorf("%s version check failed: %w", name, err)
		}
		fmt.Fprintln(s.out, strings.SplitN(string(b), "\n", 2)[0])
	}
	if s.has("remotion") || s.has("hyperframes") {
		if !nodeCompatible() {
			if err := s.systemPackage("node"); err != nil {
				return err
			}
		}
		if !nodeCompatible() {
			return fmt.Errorf("Node.js 22+ required; reopen the terminal or update Node and rerun")
		}
	}
	deps := filepath.Join(s.root, "dependencies")
	if err := os.MkdirAll(deps, 0755); err != nil {
		return err
	}
	if s.has("remotion") {
		dir := filepath.Join(s.root, "bundle", "remotion-composer")
		if err := s.npm(dir, "ci", "--no-audit", "--no-fund"); err != nil {
			return err
		}
		if err := s.run(dir, "node", filepath.Join(dir, "node_modules", "@remotion", "cli", "remotion-cli.js"), "browser", "ensure"); err != nil {
			return err
		}
	}
	if s.has("hyperframes") {
		dir := filepath.Join(deps, "hyperframes")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(dir, "package.json")); os.IsNotExist(err) {
			if err := writeJSON(filepath.Join(dir, "package.json"), map[string]any{"private": true, "dependencies": map[string]string{"hyperframes": "0.8.46"}}); err != nil {
				return err
			}
		}
		if _, err := os.Stat(filepath.Join(dir, "package-lock.json")); err == nil {
			if err := s.npm(dir, "ci", "--no-audit", "--no-fund"); err != nil {
				return err
			}
		} else if err := s.npm(dir, "install", "--no-audit", "--no-fund"); err != nil {
			return err
		}
		if err := s.run(dir, "node", filepath.Join(dir, "node_modules", "hyperframes", "bin", "hyperframes.mjs"), "browser", "ensure"); err != nil {
			return err
		}
	}
	if s.has("gflow") {
		if err := s.installGflow(); err != nil {
			return err
		}
	}
	if s.has("piper") {
		if err := s.installPiper(); err != nil {
			return err
		}
	}
	os.Setenv("PATH", strings.Join(dependencyPaths(s.root), string(os.PathListSeparator))+string(os.PathListSeparator)+os.Getenv("PATH"))
	return nil
}

func nodeCompatible() bool {
	b, err := exec.Command("node", "-p", "process.versions.node.split('.')[0]").Output()
	if err != nil {
		return false
	}
	var major int
	_, err = fmt.Sscanf(strings.TrimSpace(string(b)), "%d", &major)
	return err == nil && major >= 22
}

func (s *setup) npm(dir string, args ...string) error {
	command := "npm"
	if runtime.GOOS == "windows" {
		command = "npm.cmd"
	}
	if err := s.approve(command, args); err != nil {
		return err
	}
	return s.run(dir, command, args...)
}

// A provider's published checksum is checked before executing its binary.
func (s *setup) installGflow() error {
	dir := filepath.Join(s.root, "dependencies", "gflow")
	if _, err := os.Stat(filepath.Join(dir, executable("gflow"))); err == nil {
		return nil
	}
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	name := "gflow_1.2.1_" + runtime.GOOS + "_" + runtime.GOARCH + ext
	base := "https://github.com/xibodev/gflow-cli/releases/download/v1.2.1/"
	temp, err := os.MkdirTemp("", "facet-gflow-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	archive, sums := filepath.Join(temp, name), filepath.Join(temp, "checksums.txt")
	if err = download(base+name, archive); err != nil {
		return err
	}
	if err = download(base+"checksums.txt", sums); err != nil {
		return err
	}
	if err = verifyChecksum(archive, sums, name); err != nil {
		return err
	}
	stage := filepath.Join(temp, "unpacked")
	if err = os.Mkdir(stage, 0755); err != nil {
		return err
	}
	if ext == ".zip" {
		err = extractZip(archive, stage)
	} else {
		err = extractTar(archive, stage)
	}
	if err != nil {
		return err
	}
	if err = s.run("", filepath.Join(stage, executable("gflow")), "--help"); err != nil {
		return err
	}
	return copyTree(stage, dir)
}

func (s *setup) installPiper() error {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	if _, err := exec.LookPath(python); err != nil {
		if err := s.systemPackage("python"); err != nil {
			return err
		}
	}
	dir := filepath.Join(s.root, "dependencies", "piper")
	venvBin := "bin"
	if runtime.GOOS == "windows" {
		venvBin = "Scripts"
	}
	venvPython := filepath.Join(dir, venvBin, executable("python"))
	if _, err := os.Stat(venvPython); os.IsNotExist(err) {
		if err = s.run("", python, "-m", "venv", dir); err != nil {
			return err
		}
	}
	args := []string{"-m", "pip", "install", "--only-binary=:all:", "piper-tts==1.8.0"}
	if err := s.approve(venvPython, args); err != nil {
		return err
	}
	if err := s.run("", venvPython, args...); err != nil {
		return err
	}
	voices := filepath.Join(s.root, "dependencies", "voices")
	if err := os.MkdirAll(voices, 0755); err != nil {
		return err
	}
	// Piper's own downloader owns voice distribution and the accompanying config.
	if err := s.run(voices, venvPython, "-m", "piper.download_voices", "en_US-lessac-medium"); err != nil {
		return err
	}
	return nil
}

func (s *setup) verify() error {
	if err := os.MkdirAll(s.project, 0755); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(s.project, ".facet-verify-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	output := filepath.Join(temp, "test.mp4")
	if err = s.run(temp, "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=320x180:r=24:d=1", "-c:v", "libx264", "-pix_fmt", "yuv420p", output); err != nil {
		return err
	}
	if s.has("remotion") {
		// Isolate the check from a user's global or ancestor composer settings.
		if err = writeJSON(filepath.Join(temp, ".facet.yaml"), map[string]any{"paths": map[string]string{"remotion_composer": filepath.Join(s.root, "bundle", "remotion-composer")}}); err != nil {
			return err
		}
		request := filepath.Join(temp, "render.json")
		props := map[string]any{"width": 320, "height": 180, "fps": 24, "duration_seconds": 1, "output_path": filepath.Join(temp, "render.mp4"), "cuts": []any{map[string]any{"type": "text_card", "text": "Facet setup", "in_seconds": 0, "out_seconds": 1}}}
		if err = writeJSON(request, props); err != nil {
			return err
		}
		if err = s.run(temp, filepath.Join(s.root, "bin", executable("facet")), "tools", "run", "video_compose", "--input", request); err != nil {
			return err
		}
		output = filepath.Join(temp, "render.mp4")
	}
	b, err := exec.Command("ffprobe", "-v", "error", "-show_streams", "-of", "json", output).Output()
	if err != nil {
		return err
	}
	var probe struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}
	if err = json.Unmarshal(b, &probe); err != nil {
		return err
	}
	ok := false
	for _, stream := range probe.Streams {
		if stream.CodecType == "video" && stream.Width == 320 && stream.Height == 180 {
			ok = true
		}
	}
	if !ok {
		return fmt.Errorf("media check did not produce the expected 320x180 video")
	}
	if err = s.run(temp, "ffmpeg", "-v", "error", "-i", output, "-f", "null", "-"); err != nil {
		return err
	}
	if s.has("piper") {
		venvBin := "bin"
		if runtime.GOOS == "windows" {
			venvBin = "Scripts"
		}
		python := filepath.Join(s.root, "dependencies", "piper", venvBin, executable("python"))
		model := filepath.Join(s.root, "dependencies", "voices", "en_US-lessac-medium.onnx")
		c := exec.Command(python, "-m", "piper", "--model", model, "--output_file", filepath.Join(temp, "voice.wav"))
		c.Stdin = strings.NewReader("Facet setup verification.")
		c.Stdout, c.Stderr = s.out, s.out
		if err = c.Run(); err != nil {
			return fmt.Errorf("Piper voice test failed: %w", err)
		}
		if err = s.run(temp, "ffmpeg", "-v", "error", "-i", filepath.Join(temp, "voice.wav"), "-f", "null", "-"); err != nil {
			return err
		}
	}
	if s.has("hyperframes") {
		entry := filepath.Join(s.root, "dependencies", "hyperframes", "node_modules", "hyperframes", "bin", "hyperframes.mjs")
		html := `<!doctype html><html><head><meta charset="utf-8"></head><body style="margin:0;background:#102030;color:white"><div id="main" data-composition-id="main" data-no-timeline data-start="0" data-duration="1" data-width="320" data-height="180"><div id="title" class="clip" data-start="0" data-duration="1" style="font:24px sans-serif;padding:30px">Facet setup</div></div><script>window.__timelines = {};</script></body></html>`
		if err = os.WriteFile(filepath.Join(temp, "index.html"), []byte(html), 0644); err != nil {
			return err
		}
		hfOutput := filepath.Join(temp, "hyperframes.mp4")
		if err = s.run(temp, "node", entry, "render", "--output", hfOutput, "--fps", "24"); err != nil {
			return err
		}
		if err = s.run(temp, "ffmpeg", "-v", "error", "-i", hfOutput, "-f", "null", "-"); err != nil {
			return err
		}
	}
	fmt.Fprintln(s.out, "Local media check passed (encode, probe, decode). Agent skill invocation and external providers are not exercised by this check.")
	return nil
}
