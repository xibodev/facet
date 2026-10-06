package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/xibodev/facet/internal/toolbox"
	"github.com/xibodev/facet/internal/wire"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("expected non-nil default config")
	}

	if cfg.Defaults.Engine != "claude" {
		t.Errorf("expected engine claude, got %s", cfg.Defaults.Engine)
	}
	if cfg.Defaults.Voice != "en-US-ChristopherNeural" {
		t.Errorf("expected voice en-US-ChristopherNeural, got %s", cfg.Defaults.Voice)
	}
	if cfg.Defaults.Resolution != "1920x1080" {
		t.Errorf("expected resolution 1920x1080, got %s", cfg.Defaults.Resolution)
	}
	if cfg.Defaults.FPS != 30 {
		t.Errorf("expected fps 30, got %d", cfg.Defaults.FPS)
	}
	if cfg.Defaults.AspectRatio != "16:9" {
		t.Errorf("expected aspect ratio 16:9, got %s", cfg.Defaults.AspectRatio)
	}
	if cfg.Defaults.PermissionMode != "rw" {
		t.Errorf("expected permission mode rw, got %s", cfg.Defaults.PermissionMode)
	}
	if len(cfg.EnvProbes) == 0 {
		t.Errorf("expected non-empty env probes")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".facet.yaml")

	cfg := DefaultConfig()
	cfg.Project = "test-video"
	cfg.Defaults.Engine = "opencode"
	cfg.Defaults.Voice = "custom-voice"
	cfg.Paths.FFmpeg = "/custom/path/ffmpeg"

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file was not created: %v", err)
	}

	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.Project != "test-video" {
		t.Errorf("expected project test-video, got %s", loaded.Project)
	}
	if loaded.Defaults.Engine != "opencode" {
		t.Errorf("expected engine opencode, got %s", loaded.Defaults.Engine)
	}
	if loaded.Defaults.Voice != "custom-voice" {
		t.Errorf("expected voice custom-voice, got %s", loaded.Defaults.Voice)
	}
	if loaded.Paths.FFmpeg != "/custom/path/ffmpeg" {
		t.Errorf("expected ffmpeg path /custom/path/ffmpeg, got %s", loaded.Paths.FFmpeg)
	}
}

func TestAutoDetect(t *testing.T) {
	cfg := DefaultConfig()
	// Pin custom FFmpeg path
	cfg.Paths.FFmpeg = "/pinned/ffmpeg"

	detected := cfg.AutoDetect()
	if detected == nil {
		t.Fatal("expected non-nil detected map")
	}

	// Pinned path should not be overwritten
	if cfg.Paths.FFmpeg != "/pinned/ffmpeg" {
		t.Errorf("pinned FFmpeg path was overwritten: %s", cfg.Paths.FFmpeg)
	}
}

// isolate points every home-directory lookup at a fresh temporary home and
// moves into an empty working directory, so the doctor reads nothing of the
// real user's environment.
func isolate(t *testing.T) (home, cwd string) {
	t.Helper()
	root := t.TempDir()
	home, cwd = filepath.Join(root, "home"), filepath.Join(root, "work")
	for _, dir := range []string{home, cwd} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	t.Chdir(cwd)
	return home, cwd
}

func TestRunDoctor(t *testing.T) {
	home, cwd := isolate(t)
	cfg := DefaultConfig()
	var buf bytes.Buffer

	report, err := RunDoctorWithWriter(cfg, &buf)
	if err != nil {
		t.Fatalf("RunDoctorWithWriter failed: %v", err)
	}

	if report == nil {
		t.Fatal("expected non-nil report")
	}

	// Verify System Runtimes
	if len(report.Runtimes) < 5 {
		t.Errorf("expected at least 5 runtimes probed, got %d", len(report.Runtimes))
	}
	runtimeNames := map[string]bool{}
	for _, r := range report.Runtimes {
		runtimeNames[r.Name] = true
	}
	for _, expected := range []string{"FFmpeg", "FFprobe", "Node", "Remotion Composer", "Edge-TTS"} {
		if !runtimeNames[expected] {
			t.Errorf("missing runtime check for %s", expected)
		}
	}

	// Verify Agent CLIs
	if len(report.CLIs) < 4 {
		t.Errorf("expected at least 4 CLIs probed, got %d", len(report.CLIs))
	}
	cliNames := map[string]bool{}
	for _, c := range report.CLIs {
		cliNames[c.Name] = true
	}
	for _, expected := range []string{"Claude Code", "OpenCode", "GitHub Copilot", "OpenAI Codex"} {
		if !cliNames[expected] {
			t.Errorf("missing CLI check for %s", expected)
		}
	}

	// Verify Tools
	if len(report.Tools) < 33 {
		t.Errorf("expected at least 33 toolbox tools, got %d", len(report.Tools))
	}

	// Verify Env Vars
	if len(report.EnvVars) == 0 {
		t.Errorf("expected non-empty env vars in report")
	}

	// A fresh home has no runtime, no wiring, and no 1.x leftovers.
	if report.Facet.Status != StatusNotFound || len(report.Wiring) != 0 || report.WiringError != "" || len(report.Legacy) != 0 {
		t.Errorf("fresh home report: facet=%+v wiring=%v (%s) legacy=%v", report.Facet, report.Wiring, report.WiringError, report.Legacy)
	}

	// Verify Output Formatting
	out := buf.String()
	for _, section := range []string{"=== Facet System Doctor ===", "[Facet]", "[Wiring]", "[Facet 1.x Integrations]", "[System Runtimes]", "[Agent CLIs]", "[Environment Variables]", "[Toolbox Tools ("} {
		if !strings.Contains(out, section) {
			t.Errorf("output missing %s:\n%s", section, out)
		}
	}
	if !strings.Contains(out, "No wirings recorded") {
		t.Errorf("output does not explain the empty wiring:\n%s", out)
	}

	// Verify JSON output helper
	jsonBytes, err := report.JSON()
	if err != nil {
		t.Fatalf("report.JSON() error: %v", err)
	}
	if len(jsonBytes) == 0 {
		t.Errorf("expected non-empty json output")
	}

	// The doctor only reads.
	for _, dir := range []string{home, cwd} {
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("doctor wrote into %s: %v", dir, entries)
		}
	}
}

func TestDoctorProbesOnlyToolCredentials(t *testing.T) {
	for _, name := range DefaultEnvProbes() {
		if name == "ANTHROPIC_API_KEY" {
			t.Error("the doctor probes a model credential; models are the harness's concern")
		}
	}
}

func TestDoctorReportsTheActiveRuntime(t *testing.T) {
	home, _ := isolate(t)
	if got := probeFacetRuntime(home, "2.0.0"); got.Status != StatusNotFound || !strings.Contains(got.Details, "current") {
		t.Fatalf("no runtime: %+v", got)
	}

	osName, arch := runtime.GOOS, runtime.GOARCH
	release := filepath.Join(home, ".facet", "runtimes", "2.0.0-"+osName+"-"+arch)
	current := filepath.Join(home, ".facet", "current")
	if err := os.MkdirAll(filepath.Join(release, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(release, current); err != nil {
		// Unprivileged Windows accounts cannot create symlinks; a plain
		// directory exercises the same reporting.
		current = filepath.Join(home, ".facet", "current")
		if err := os.MkdirAll(filepath.Join(current, "bin"), 0755); err != nil {
			t.Fatal(err)
		}
		release = current
	}
	if got := probeFacetRuntime(home, "2.0.0"); got.Status != StatusWarning || !strings.Contains(got.Details, "no bin/facet") {
		t.Fatalf("runtime without executable: %+v", got)
	}

	name := "facet"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	exe := filepath.Join(release, "bin", name)
	if err := os.WriteFile(exe, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}
	var probed string
	saved := facetVersionOf
	t.Cleanup(func() { facetVersionOf = saved })
	facetVersionOf = func(path string) string { probed = path; return "2.0.0" }

	got := probeFacetRuntime(home, "2.0.0")
	if got.Status != StatusOK || got.Version != "2.0.0" || got.Current != current {
		t.Fatalf("matching runtime: %+v", got)
	}
	if !strings.HasSuffix(filepath.ToSlash(probed), "/.facet/current/bin/"+name) {
		t.Errorf("probed %s, want the stable executable", probed)
	}
	if resolved, err := filepath.EvalSymlinks(release); err == nil && got.Target != resolved {
		t.Errorf("target = %s, want %s", got.Target, resolved)
	}

	got = probeFacetRuntime(home, "2.1.0")
	if got.Status != StatusWarning || !strings.Contains(got.Details, "v2.0.0") || !strings.Contains(got.Details, "v2.1.0") {
		t.Fatalf("mismatched runtime: %+v", got)
	}

	var buf bytes.Buffer
	formatDoctorReport(&DoctorReport{Facet: got}, &buf)
	if !strings.Contains(buf.String(), current) || !strings.Contains(buf.String(), "facet v2.0.0") {
		t.Errorf("formatted runtime:\n%s", buf.String())
	}

	if v := versionFromRuntimeDir(filepath.Join("x", "2.0.0-rc.1-linux-arm64")); v != "2.0.0-rc.1" {
		t.Errorf("versionFromRuntimeDir = %q", v)
	}
	if v := versionFromRuntimeDir("current"); v != "" {
		t.Errorf("versionFromRuntimeDir(current) = %q", v)
	}
}

func TestDoctorNamesV1IntegrationsAndTheirCleanup(t *testing.T) {
	home, cwd := isolate(t)
	marked := "# Facet\n\n## This installation\n- Invoke Facet through `run-facet.ps1` followed by the normal arguments.\n"
	writeConfigFixture(t, filepath.Join(home, ".facet-install", "installation.json"), `{"schema":1,"version":"1.1.0","host":"opencode","components":["remotion"],"packs":[]}`)
	writeConfigFixture(t, filepath.Join(home, ".opencode", "skills", "facet", "SKILL.md"), marked)
	writeConfigFixture(t, filepath.Join(cwd, ".facet-install", "installation.tsv"), "version\t1.1.0\ninstallation\t/opt/facet\nhost\tclaude\ncomponents\tremotion\npacks\t\n")
	// A skill without both markers is not a 1.x copy.
	writeConfigFixture(t, filepath.Join(cwd, ".agents", "skills", "facet", "SKILL.md"), "# Facet\n\n## This installation\n")
	writeConfigFixture(t, filepath.Join(cwd, ".github", "skills", "facet", "SKILL.md"), "uses run-facet but no section\n")

	checks := legacyChecks("windows", cwd, home)
	if len(checks) != 2 {
		t.Fatalf("legacy checks = %+v", checks)
	}
	byProject := map[string]LegacyCheck{}
	for _, c := range checks {
		byProject[c.Project] = c
	}
	homeCheck, cwdCheck := byProject[home], byProject[cwd]
	if homeCheck.Host != "opencode" || len(homeCheck.Files) != 2 ||
		homeCheck.Files[0] != filepath.Join(home, ".facet-install", "installation.json") ||
		homeCheck.Files[1] != filepath.Join(home, ".opencode", "skills", "facet", "SKILL.md") {
		t.Errorf("home check = %+v", homeCheck)
	}
	if want := `.\install.ps1 -Action uninstall -Target opencode -ProjectDir '` + home + `'`; homeCheck.Cleanup != want {
		t.Errorf("windows cleanup = %q, want %q", homeCheck.Cleanup, want)
	}
	if cwdCheck.Host != "claude" || len(cwdCheck.Files) != 1 || cwdCheck.Files[0] != filepath.Join(cwd, ".facet-install", "installation.tsv") {
		t.Errorf("cwd check = %+v", cwdCheck)
	}
	unix := legacyChecks("linux", cwd)
	if want := "bash install.sh --action uninstall --target claude --project '" + cwd + "'"; len(unix) != 1 || unix[0].Cleanup != want {
		t.Errorf("unix cleanup = %+v, want %q", unix, want)
	}

	var buf bytes.Buffer
	report, err := RunDoctorWithWriter(DefaultConfig(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Legacy) != 2 {
		t.Fatalf("doctor found %d 1.x installations", len(report.Legacy))
	}
	out := buf.String()
	for _, want := range append(append([]string{}, homeCheck.Files...), "Action uninstall", "action uninstall", "shadows the user-wide Facet skill") {
		if runtime.GOOS != "windows" && want == "Action uninstall" || runtime.GOOS == "windows" && want == "action uninstall" {
			continue
		}
		if !strings.Contains(out, want) {
			t.Errorf("doctor output omits %q:\n%s", want, out)
		}
	}
}

func TestDoctorSummarizesWiring(t *testing.T) {
	home, _ := isolate(t)
	var stdout, stderr bytes.Buffer
	if code := wire.CLI([]string{"copilot"}, &stdout, &stderr, toolbox.ProductVersion()); code != 0 {
		t.Fatalf("wire copilot: %d\n%s%s", code, stdout.String(), stderr.String())
	}
	checks, errText := wiringChecks(toolbox.ProductVersion())
	if errText != "" || len(checks) != 1 || checks[0].Status != StatusOK || checks[0].Label != "copilot (user)" {
		t.Fatalf("wiring checks = %+v (%s)", checks, errText)
	}
	skill := filepath.Join(home, ".copilot", "skills", "facet", "SKILL.md")
	if err := os.WriteFile(skill, []byte("edited"), 0644); err != nil {
		t.Fatal(err)
	}
	checks, _ = wiringChecks("9.9.9")
	if len(checks) != 1 || checks[0].Status != StatusWarning || checks[0].Fix != "facet wire copilot" ||
		!strings.Contains(checks[0].Details, "changed since facet wire installed them") || !strings.Contains(checks[0].Details, "v9.9.9") {
		t.Fatalf("drifted wiring checks = %+v", checks)
	}
	var buf bytes.Buffer
	formatDoctorReport(&DoctorReport{Facet: FacetRuntimeCheck{Status: StatusNotFound}, Wiring: checks}, &buf)
	if !strings.Contains(buf.String(), "fix: facet wire copilot") {
		t.Errorf("formatted wiring:\n%s", buf.String())
	}

	if err := os.WriteFile(wire.RegistryPath(home), []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, errText := wiringChecks("9.9.9"); errText == "" {
		t.Error("an unreadable registry was not reported")
	}
}

func TestEdgeTTSDoctorDistinguishesBuiltInClientFromServiceReachability(t *testing.T) {
	got := probeEdgeTTS()
	if !got.Available || got.Status != StatusOK {
		t.Fatalf("built-in Edge TTS client should be available: %#v", got)
	}
	for _, want := range []string{"built-in", "network", "not tested"} {
		if !strings.Contains(strings.ToLower(got.Details), want) {
			t.Errorf("Edge TTS details %q do not mention %q", got.Details, want)
		}
	}
}

func writeConfigFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestBundleResolutionOrder(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "checkout", "projects", "demo")
	home := filepath.Join(root, "home")
	executable := filepath.Join(root, "install", "bin", "facet")
	want := []string{cwd, filepath.Dir(cwd), filepath.Join(root, "checkout"), filepath.Join(root, "install", "bundle"), filepath.Join(root, "install"), filepath.Join(home, ".facet", "bundle")}
	if got := bundleCandidatesFor(cwd, home, executable); !reflect.DeepEqual(got, want) {
		t.Fatalf("candidate order = %v, want %v", got, want)
	}
}
