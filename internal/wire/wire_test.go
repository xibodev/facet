package wire

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/xibodev/facet/internal/bundle"
)

const testVersion = "2.0.0-test"

// The test binary doubles as fake agentic CLIs: copied onto a temporary PATH
// as claude[.exe] or codex[.exe], it records its argv and emulates the
// configuration writes of `mcp add` and `mcp remove`.
//
// Per-test fakes are hard links to one copy of the test binary made here:
// Windows cannot delete a link to the image of a running process, so they
// must not link to the test binary itself.
var fakeSource string

func TestMain(m *testing.M) {
	if logPath := os.Getenv("FACET_WIRE_FAKE_LOG"); logPath != "" {
		name := strings.TrimSuffix(strings.ToLower(filepath.Base(os.Args[0])), ".exe")
		switch name {
		case "claude", "codex", "copilot", "opencode":
			os.Exit(fakeCLI(name, os.Args[1:], logPath))
		}
	}
	dir, err := os.MkdirTemp("", "facet-wire-fake-")
	if err == nil {
		if self, err := os.Executable(); err == nil {
			dest := filepath.Join(dir, "fake")
			if runtime.GOOS == "windows" {
				dest += ".exe"
			}
			if copyFile(self, dest) == nil {
				fakeSource = dest
			}
		}
	}
	code := m.Run()
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
	os.Exit(code)
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

type fakeCall struct {
	CLI  string   `json:"cli"`
	Args []string `json:"args"`
	Dir  string   `json:"dir"`
}

func fakeCLI(name string, args []string, logPath string) int {
	dir, _ := os.Getwd()
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 9
	}
	_ = json.NewEncoder(f).Encode(fakeCall{CLI: name, Args: args, Dir: dir})
	f.Close()
	if os.Getenv("FACET_WIRE_FAKE_FAIL") == name {
		fmt.Fprintln(os.Stderr, "simulated failure")
		return 3
	}
	if len(args) < 3 || args[0] != "mcp" {
		return 0
	}
	scope := "local"
	var rest []string
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--scope", "-s":
			scope = args[i+1]
			i++
		case "--transport", "-t":
			i++
		case "--":
			rest = append(rest, args[i+1:]...)
			i = len(args)
		default:
			rest = append(rest, args[i])
		}
	}
	home, _ := os.UserHomeDir()
	server := rest[0]
	switch name {
	case "claude":
		file := filepath.Join(home, ".claude.json")
		if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
			file = filepath.Join(d, ".claude.json")
		}
		if scope == "project" {
			file = filepath.Join(dir, ".mcp.json")
		}
		doc := map[string]any{}
		if data, err := os.ReadFile(file); err == nil {
			_ = json.Unmarshal(data, &doc)
		}
		servers, _ := doc["mcpServers"].(map[string]any)
		if servers == nil {
			servers = map[string]any{}
		}
		switch args[1] {
		case "add":
			if _, exists := servers[server]; exists {
				fmt.Fprintf(os.Stderr, "MCP server %s already exists\n", server)
				return 1
			}
			servers[server] = map[string]any{"type": "stdio", "command": rest[1], "args": rest[2:], "env": map[string]any{}}
		case "remove":
			if _, exists := servers[server]; !exists {
				fmt.Fprintf(os.Stderr, "No MCP server found with name: %s\n", server)
				return 1
			}
			delete(servers, server)
		}
		doc["mcpServers"] = servers
		data, _ := json.MarshalIndent(doc, "", "  ")
		_ = os.WriteFile(file, data, 0o644)
	case "codex":
		file := filepath.Join(home, ".codex", "config.toml")
		if d := os.Getenv("CODEX_HOME"); d != "" {
			file = filepath.Join(d, "config.toml")
		}
		data, _ := os.ReadFile(file)
		content := fakeRemoveTable(string(data), "mcp_servers."+server)
		switch args[1] {
		case "add":
			quoted := make([]string, len(rest[2:]))
			for i, a := range rest[2:] {
				quoted[i] = tomlString(a)
			}
			content += fmt.Sprintf("[mcp_servers.%s]\ncommand = %s\nargs = [%s]\n", server, tomlString(rest[1]), strings.Join(quoted, ", "))
		case "remove":
			if !inspectTOMLServer(string(data), server).defined {
				fmt.Fprintf(os.Stderr, "No MCP server named '%s' found\n", server)
				return 1
			}
		}
		_ = os.MkdirAll(filepath.Dir(file), 0o755)
		_ = os.WriteFile(file, []byte(content), 0o644)
	}
	return 0
}

func fakeRemoveTable(content, header string) string {
	var out []string
	skipping := false
	for _, line := range strings.SplitAfter(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			skipping = trimmed == "["+header+"]"
		}
		if !skipping && line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "")
}

type sandbox struct {
	root, home, project, bin, log string
	exe                           string
}

func newSandbox(t *testing.T, clis ...string) *sandbox {
	t.Helper()
	root := t.TempDir()
	s := &sandbox{
		root:    root,
		home:    filepath.Join(root, "home"),
		project: filepath.Join(root, "work", "demo"),
		bin:     filepath.Join(root, "bin"),
		log:     filepath.Join(root, "calls.jsonl"),
	}
	for _, dir := range []string{s.home, s.project, s.bin} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", s.home)
	t.Setenv("USERPROFILE", s.home)
	t.Setenv("APPDATA", filepath.Join(s.home, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(s.home, "AppData", "Local"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(s.home, ".config"))
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("PATH", s.bin)
	t.Setenv("FACET_WIRE_FAKE_LOG", s.log)
	t.Setenv("FACET_WIRE_FAKE_FAIL", "")
	for _, cli := range clis {
		s.installFake(t, cli)
	}
	// A stable executable makes registrations deterministic.
	name := "facet"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	s.exe = filepath.Join(s.home, ".facet", "current", "bin", name)
	if err := os.MkdirAll(filepath.Dir(s.exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.exe, []byte("stable facet"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(s.project)
	return s
}

func (s *sandbox) installFake(t *testing.T, cli string) {
	t.Helper()
	if fakeSource == "" {
		t.Fatal("no copy of the test binary is available for fake CLIs")
	}
	dest := filepath.Join(s.bin, cli)
	if runtime.GOOS == "windows" {
		dest += ".exe"
	}
	if err := os.Link(fakeSource, dest); err == nil {
		return
	}
	if err := copyFile(fakeSource, dest); err != nil {
		t.Fatal(err)
	}
}

func (s *sandbox) calls(t *testing.T) []fakeCall {
	t.Helper()
	f, err := os.Open(s.log)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var calls []fakeCall
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var c fakeCall
		if err := json.Unmarshal(scanner.Bytes(), &c); err != nil {
			t.Fatal(err)
		}
		calls = append(calls, c)
	}
	return calls
}

func run(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := CLI(args, &stdout, &stderr, testVersion)
	return stdout.String(), stderr.String(), code
}

func runVersion(t *testing.T, version string, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := CLI(args, &stdout, &stderr, version)
	return stdout.String(), stderr.String(), code
}

func mustRun(t *testing.T, args ...string) string {
	t.Helper()
	out, errOut, code := run(t, args...)
	if code != 0 {
		t.Fatalf("facet wire %s: exit %d\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), code, out, errOut)
	}
	return out
}

func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, name)
		if entry.IsDir() {
			out[filepath.ToSlash(rel)+"/"] = ""
			return nil
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(plainJSON(data), &doc); err != nil {
		t.Fatalf("%s: %v\n%s", path, err, data)
	}
	return doc
}

func dig(doc map[string]any, keys ...string) any {
	var cur any = doc
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[k]
	}
	return cur
}

func expectFiles(t *testing.T, root string, target bundle.Target) {
	t.Helper()
	files, err := bundle.Files(target)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
		if err != nil {
			t.Errorf("%s: %s not installed: %v", target, f.Path, err)
			continue
		}
		if !bytes.Equal(got, f.Content) {
			t.Errorf("%s: %s differs from the projection", target, f.Path)
		}
	}
}

func loadRegistry(t *testing.T, home string) *Registry {
	t.Helper()
	reg, err := LoadRegistry(RegistryPath(home))
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

func TestWireUserScopeUsesEachCLIsNativeLocationsAndRegistration(t *testing.T) {
	s := newSandbox(t, "claude", "codex")
	out := mustRun(t, "all")

	roots := map[bundle.Target]string{
		bundle.TargetClaude:   filepath.Join(s.home, ".claude"),
		bundle.TargetCodex:    filepath.Join(s.home, ".codex"),
		bundle.TargetCopilot:  filepath.Join(s.home, ".copilot"),
		bundle.TargetOpenCode: filepath.Join(s.home, ".config", "opencode"),
	}
	for target, root := range roots {
		expectFiles(t, root, target)
	}
	if _, err := os.Stat(filepath.Join(s.home, ".claude", "agents", "facet-creative.md")); err != nil {
		t.Error("Claude Code persona not installed as a native agent")
	}
	if _, err := os.Stat(filepath.Join(s.home, ".copilot", "agents", "facet-creative.agent.md")); err != nil {
		t.Error("Copilot persona not installed as a native agent")
	}
	if _, err := os.Stat(filepath.Join(s.home, ".codex", "agents")); !os.IsNotExist(err) {
		t.Error("a persona was installed for Codex, whose agent format is not validated")
	}

	calls := s.calls(t)
	want := map[string][]string{
		"claude": {"mcp", "add", "--scope", "user", "--transport", "stdio", "facet", "--", s.exe, "mcp"},
		"codex":  {"mcp", "add", "facet", "--", s.exe, "mcp"},
	}
	if len(calls) != 2 {
		t.Fatalf("CLI calls = %+v, want one registration each for claude and codex", calls)
	}
	for _, c := range calls {
		if strings.Join(c.Args, "\x00") != strings.Join(want[c.CLI], "\x00") {
			t.Errorf("%s argv = %q, want %q", c.CLI, c.Args, want[c.CLI])
		}
	}
	if got := dig(readJSON(t, filepath.Join(s.home, ".claude.json")), "mcpServers", "facet", "command"); got != s.exe {
		t.Errorf("claude registration command = %v", got)
	}
	copilot := readJSON(t, filepath.Join(s.home, ".copilot", "mcp-config.json"))
	if dig(copilot, "mcpServers", "facet", "command") != s.exe || dig(copilot, "mcpServers", "facet", "type") != "local" {
		t.Errorf("copilot registration = %v", copilot)
	}
	opencode := readJSON(t, filepath.Join(s.home, ".config", "opencode", "opencode.json"))
	if cmd, _ := dig(opencode, "mcp", "facet", "command").([]any); len(cmd) != 2 || cmd[0] != s.exe || cmd[1] != "mcp" {
		t.Errorf("opencode registration = %v", opencode)
	}

	reg := loadRegistry(t, s.home)
	if len(reg.Wirings) != 4 {
		t.Fatalf("registry wirings = %d, want 4", len(reg.Wirings))
	}
	methods := map[string]string{}
	for _, w := range reg.Wirings {
		if w.FacetVersion != testVersion || w.Executable != s.exe || w.Pending || w.MCP == nil || len(w.Files) == 0 {
			t.Errorf("incomplete record %+v", w)
			continue
		}
		methods[w.CLI] = w.MCP.Method
		for _, f := range w.Files {
			if got, err := fileDigest(f.Path); err != nil || got != f.Digest {
				t.Errorf("recorded digest of %s does not match the file", f.Path)
			}
		}
	}
	if methods["claude"] != MethodCommand || methods["codex"] != MethodCommand || methods["copilot"] != MethodJSON || methods["opencode"] != MethodJSON {
		t.Errorf("registration methods = %v", methods)
	}

	// Facet never writes instruction files.
	for name := range tree(t, s.home) {
		switch filepath.Base(name) {
		case "AGENTS.md", "CLAUDE.md", "copilot-instructions.md", "AGENT.md":
			t.Errorf("wrote instruction file %s", name)
		}
	}
	if !strings.Contains(out, "wired") {
		t.Errorf("output does not confirm the wiring:\n%s", out)
	}
}

func TestWireIsIdempotent(t *testing.T) {
	s := newSandbox(t, "claude", "codex")
	mustRun(t, "all")
	before := tree(t, s.home)
	calls := len(s.calls(t))
	out := mustRun(t, "claude,codex,copilot,opencode")
	if after := tree(t, s.home); len(after) != len(before) {
		t.Fatalf("second run changed the tree: %d vs %d entries", len(after), len(before))
	} else {
		for name, content := range before {
			if after[name] != content {
				t.Errorf("second run changed %s", name)
			}
		}
	}
	if got := len(s.calls(t)); got != calls {
		t.Errorf("second run invoked a CLI %d more time(s)", got-calls)
	}
	if strings.Contains(out, "create") || strings.Contains(out, " run ") {
		t.Errorf("second run planned changes:\n%s", out)
	}
}

func TestWireProjectScope(t *testing.T) {
	s := newSandbox(t, "claude", "codex")
	homeBefore := tree(t, s.home)
	mustRun(t, "all", "--scope", "project")

	roots := map[bundle.Target]string{
		bundle.TargetClaude:   filepath.Join(s.project, ".claude"),
		bundle.TargetCodex:    filepath.Join(s.project, ".agents"),
		bundle.TargetCopilot:  filepath.Join(s.project, ".github"),
		bundle.TargetOpenCode: filepath.Join(s.project, ".opencode"),
	}
	for target, root := range roots {
		expectFiles(t, root, target)
	}
	calls := s.calls(t)
	if len(calls) != 1 || calls[0].CLI != "claude" || !samePath(calls[0].Dir, s.project) ||
		strings.Join(calls[0].Args, " ") != strings.Join([]string{"mcp", "add", "--scope", "project", "--transport", "stdio", "facet", "--", s.exe, "mcp"}, " ") {
		t.Fatalf("project-scope CLI calls = %+v; only Claude has a project-scope command", calls)
	}
	if dig(readJSON(t, filepath.Join(s.project, ".mcp.json")), "mcpServers", "facet", "command") != s.exe {
		t.Error("Claude project registration missing from .mcp.json")
	}
	toml, err := os.ReadFile(filepath.Join(s.project, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if st := inspectTOMLServer(string(toml), "facet"); !st.parsed || st.command != s.exe || strings.Join(st.args, " ") != "mcp" {
		t.Errorf("codex project config = %q", toml)
	}
	if dig(readJSON(t, filepath.Join(s.project, ".github", "mcp.json")), "mcpServers", "facet", "command") != s.exe {
		t.Error("Copilot project registration missing")
	}
	if dig(readJSON(t, filepath.Join(s.project, "opencode.json")), "mcp", "facet", "enabled") != true {
		t.Error("OpenCode project registration missing")
	}
	// Only the registry changes in the home directory.
	homeAfter := tree(t, s.home)
	for name := range homeAfter {
		if _, ok := homeBefore[name]; !ok && name != ".facet/wiring.json" {
			t.Errorf("project-scope wiring wrote %s in the home directory", name)
		}
	}
	reg := loadRegistry(t, s.home)
	for _, w := range reg.Wirings {
		if w.Scope != "project" || !samePath(w.Project, s.project) {
			t.Errorf("record %s/%s/%s", w.CLI, w.Scope, w.Project)
		}
	}

	// --project implies project scope and names the same wiring.
	out := mustRun(t, "opencode", "--project", s.project)
	if !strings.Contains(out, "unchanged") {
		t.Errorf("rewiring the same project planned changes:\n%s", out)
	}
}

func TestDryRunHasNoSideEffects(t *testing.T) {
	s := newSandbox(t, "claude", "codex")
	before := tree(t, s.root)
	out := mustRun(t, "all", "--dry-run")
	if after := tree(t, s.root); len(after) != len(before) {
		t.Fatalf("dry run changed the file system")
	}
	if calls := s.calls(t); len(calls) != 0 {
		t.Fatalf("dry run invoked CLIs: %+v", calls)
	}
	for _, want := range []string{
		"Dry run",
		"create",
		commandLine([]string{"claude", "mcp", "add", "--scope", "user", "--transport", "stdio", "facet", "--", s.exe, "mcp"}),
		commandLine([]string{"codex", "mcp", "add", "facet", "--", s.exe, "mcp"}),
		filepath.Join(s.home, ".copilot", "mcp-config.json"),
		filepath.Join(s.home, ".config", "opencode", "opencode.json"),
		RegistryPath(s.home),
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dry run output omits %q:\n%s", want, out)
		}
	}

	mustRun(t, "claude")
	before = tree(t, s.root)
	calls := len(s.calls(t))
	out = mustRun(t, "--remove", "claude", "--dry-run")
	if after := tree(t, s.root); len(after) != len(before) {
		t.Fatal("dry-run removal changed the file system")
	}
	if len(s.calls(t)) != calls || !strings.Contains(out, "mcp remove --scope user facet") {
		t.Fatalf("dry-run removal output:\n%s", out)
	}
}

func TestRemoveReversesOnlyRecordedUnmodifiedItems(t *testing.T) {
	s := newSandbox(t, "claude", "codex")
	// Pre-existing user content that must survive.
	ownSkill := filepath.Join(s.home, ".claude", "skills", "mine", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(ownSkill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ownSkill, []byte("---\nname: mine\ndescription: mine\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	copilotConfig := filepath.Join(s.home, ".copilot", "mcp-config.json")
	if err := os.MkdirAll(filepath.Dir(copilotConfig), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("{\n  \"mcpServers\": {\n    \"other\": {\n      \"type\": \"http\",\n      \"url\": \"http://127.0.0.1:9/mcp\"\n    }\n  }\n}\n")
	if err := os.WriteFile(copilotConfig, original, 0o600); err != nil {
		t.Fatal(err)
	}

	mustRun(t, "all")
	modified := filepath.Join(s.home, ".config", "opencode", "skills", "facet-social", "SKILL.md")
	if err := os.WriteFile(modified, []byte("my edits"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := mustRun(t, "--remove", "all")
	if !strings.Contains(out, "left in place") {
		t.Errorf("the kept modified file is not reported:\n%s", out)
	}
	if data, err := os.ReadFile(modified); err != nil || string(data) != "my edits" {
		t.Error("a modified file was deleted")
	}
	if _, err := os.Stat(ownSkill); err != nil {
		t.Error("a skill facet wire did not install was removed")
	}
	if data, err := os.ReadFile(copilotConfig); err != nil || !bytes.Equal(data, original) {
		t.Errorf("copilot configuration was not restored exactly:\n%s", data)
	}
	for _, gone := range []string{
		filepath.Join(s.home, ".claude", "skills", "facet"),
		filepath.Join(s.home, ".claude", "agents"),
		filepath.Join(s.home, ".codex", "skills"),
		filepath.Join(s.home, ".copilot", "skills"),
		filepath.Join(s.home, ".copilot", "agents"),
		filepath.Join(s.home, ".config", "opencode", "opencode.json"),
		filepath.Join(s.home, ".config", "opencode", "skills", "facet"),
	} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Errorf("%s survived removal", gone)
		}
	}
	// The CLI's own configuration file stays; only the registration left it.
	if st := observeTOMLServer(filepath.Join(s.home, ".codex", "config.toml")); st.present {
		t.Error("codex registration survived removal")
	}
	if dig(readJSON(t, filepath.Join(s.home, ".claude.json")), "mcpServers", "facet") != nil {
		t.Error("claude registration survived removal")
	}
	var removes []string
	for _, c := range s.calls(t) {
		if len(c.Args) > 1 && c.Args[1] == "remove" {
			removes = append(removes, c.CLI+" "+strings.Join(c.Args, " "))
		}
	}
	sort.Strings(removes)
	if strings.Join(removes, "|") != "claude mcp remove --scope user facet|codex mcp remove facet" {
		t.Errorf("unregister commands = %v", removes)
	}
	if reg := loadRegistry(t, s.home); len(reg.Wirings) != 0 {
		t.Errorf("registry still records %d wirings", len(reg.Wirings))
	}
	// Removing again is a no-op.
	if out := mustRun(t, "--remove", "claude"); !strings.Contains(out, "nothing to remove") {
		t.Errorf("second removal output:\n%s", out)
	}
}

func TestWireRefusesWhatItDoesNotOwn(t *testing.T) {
	t.Run("unowned skill file", func(t *testing.T) {
		s := newSandbox(t, "claude")
		skill := filepath.Join(s.home, ".claude", "skills", "facet", "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(skill), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(skill, []byte("my facet notes"), 0o644); err != nil {
			t.Fatal(err)
		}
		before := tree(t, s.root)
		_, errOut, code := run(t, "claude")
		if code != 1 || !strings.Contains(errOut, "was not installed by facet wire") || !strings.Contains(errOut, "Nothing was changed") {
			t.Fatalf("exit %d: %s", code, errOut)
		}
		if after := tree(t, s.root); len(after) != len(before) || after[".claude/skills/facet/SKILL.md"] != before[".claude/skills/facet/SKILL.md"] {
			t.Fatal("a refused wiring changed files")
		}
		if len(s.calls(t)) != 0 {
			t.Fatal("a refused wiring ran the CLI")
		}
	})
	t.Run("modified owned file", func(t *testing.T) {
		s := newSandbox(t)
		mustRun(t, "copilot")
		skill := filepath.Join(s.home, ".copilot", "skills", "facet", "SKILL.md")
		if err := os.WriteFile(skill, []byte("edited"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, errOut, code := run(t, "copilot")
		if code != 1 || !strings.Contains(errOut, "changed after facet wire installed it") {
			t.Fatalf("exit %d: %s", code, errOut)
		}
		if data, _ := os.ReadFile(skill); string(data) != "edited" {
			t.Fatal("a modified file was overwritten")
		}
	})
	t.Run("foreign MCP entry", func(t *testing.T) {
		s := newSandbox(t)
		config := filepath.Join(s.home, ".config", "opencode", "opencode.json")
		if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
			t.Fatal(err)
		}
		original := []byte(`{"mcp": {"facet": {"type": "local", "command": ["other-facet", "mcp"]}}}`)
		if err := os.WriteFile(config, original, 0o644); err != nil {
			t.Fatal(err)
		}
		_, errOut, code := run(t, "opencode")
		if code != 1 || !strings.Contains(errOut, "already defines mcp.facet") {
			t.Fatalf("exit %d: %s", code, errOut)
		}
		if data, _ := os.ReadFile(config); !bytes.Equal(data, original) {
			t.Fatal("a foreign registration was changed")
		}
		if _, err := os.Stat(filepath.Join(s.home, ".config", "opencode", "skills")); !os.IsNotExist(err) {
			t.Fatal("skills were installed although the wiring was refused")
		}
	})
	t.Run("foreign claude registration", func(t *testing.T) {
		s := newSandbox(t, "claude")
		config := filepath.Join(s.home, ".claude.json")
		if err := os.WriteFile(config, []byte(`{"mcpServers": {"facet": {"type": "stdio", "command": "elsewhere", "args": []}}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		_, errOut, code := run(t, "claude")
		if code != 1 || !strings.Contains(errOut, "claude mcp remove --scope user facet") {
			t.Fatalf("exit %d: %s", code, errOut)
		}
		if len(s.calls(t)) != 0 {
			t.Fatal("the CLI ran for a refused wiring")
		}
	})
	t.Run("missing CLI", func(t *testing.T) {
		s := newSandbox(t)
		_, errOut, code := run(t, "codex")
		if code != 1 || !strings.Contains(errOut, "codex was not found on PATH") {
			t.Fatalf("exit %d: %s", code, errOut)
		}
		if _, err := os.Stat(filepath.Join(s.home, ".codex")); !os.IsNotExist(err) {
			t.Fatal("files were written although the CLI is missing")
		}
	})
	t.Run("project is the home directory", func(t *testing.T) {
		s := newSandbox(t)
		_, errOut, code := run(t, "opencode", "--project", s.home)
		if code != 1 || !strings.Contains(errOut, "home directory") {
			t.Fatalf("exit %d: %s", code, errOut)
		}
	})
	t.Run("v1 skill copy", func(t *testing.T) {
		s := newSandbox(t)
		skill := filepath.Join(s.project, ".opencode", "skills", "facet", "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(skill), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(skill, []byte("# Facet\n\n## This installation\n- Invoke Facet through run-facet.sh\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, errOut, code := run(t, "opencode", "--scope", "project")
		if code != 1 || !strings.Contains(errOut, "Facet 1.x installation") {
			t.Fatalf("exit %d: %s", code, errOut)
		}
	})
}

func TestCLIFailureKeepsSkillsAndReportsDrift(t *testing.T) {
	s := newSandbox(t, "claude")
	t.Setenv("FACET_WIRE_FAKE_FAIL", "claude")
	_, errOut, code := run(t, "claude")
	if code != 1 || !strings.Contains(errOut, "simulated failure") {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	reg := loadRegistry(t, s.home)
	if len(reg.Wirings) != 1 || reg.Wirings[0].MCP != nil || len(reg.Wirings[0].Files) == 0 {
		t.Fatalf("record after a failed registration = %+v", reg.Wirings)
	}
	out := mustRun(t, "--status")
	if !strings.Contains(out, "drift") || !strings.Contains(out, "no MCP server registration") {
		t.Errorf("status does not report the missing registration:\n%s", out)
	}
	t.Setenv("FACET_WIRE_FAKE_FAIL", "")
	mustRun(t, "claude")
	if out := mustRun(t, "--status"); strings.Contains(out, "drift") {
		t.Errorf("rewiring did not repair the registration:\n%s", out)
	}
}

func TestStatusReportsDrift(t *testing.T) {
	s := newSandbox(t, "claude", "codex")
	if out := mustRun(t, "--status"); !strings.Contains(out, "No wirings recorded") {
		t.Fatalf("empty status:\n%s", out)
	}
	mustRun(t, "all")
	out := mustRun(t, "--status")
	if strings.Contains(out, "drift") || strings.Count(out, ": ok") != 4 {
		t.Fatalf("fresh wirings are not ok:\n%s", out)
	}

	modified := filepath.Join(s.home, ".claude", "skills", "facet", "SKILL.md")
	if err := os.WriteFile(modified, []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(s.home, ".codex", "skills", "facet-social", "SKILL.md")
	if err := os.Remove(missing); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(s.home, ".config", "opencode", "opencode.json")
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, bytes.Replace(data, []byte(`"enabled": true`), []byte(`"enabled": false`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(s.home, ".copilot", "mcp-config.json")); err != nil {
		t.Fatal(err)
	}

	out, _, code := runVersion(t, "2.1.0", "--status")
	if code != 0 {
		t.Fatalf("status exit %d", code)
	}
	for _, want := range []string{
		"modified: " + modified,
		"missing:  " + missing,
		"mcp:      modified",
		"mcp:      missing",
		"wired by facet v" + testVersion + "; this is facet v2.1.0",
		"fix:      facet wire claude",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("status omits %q:\n%s", want, out)
		}
	}

	report, err := Inspect(testVersion)
	if err != nil {
		t.Fatal(err)
	}
	drifted := 0
	for _, w := range report.Wirings {
		if w.Drifted() {
			drifted++
		}
	}
	if len(report.Wirings) != 4 || drifted != 4 {
		t.Errorf("Inspect: %d wirings, %d drifted", len(report.Wirings), drifted)
	}
}

func TestRewireFollowsTheStableExecutable(t *testing.T) {
	s := newSandbox(t, "claude")
	// Without ~/.facet/current, the running executable is registered.
	if err := os.RemoveAll(filepath.Join(s.home, ".facet", "current")); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "claude")
	running, _ := os.Executable()
	if got := dig(readJSON(t, filepath.Join(s.home, ".claude.json")), "mcpServers", "facet", "command"); !samePath(fmt.Sprint(got), running) {
		t.Fatalf("registered %v, want the running executable %s", got, running)
	}
	if err := os.MkdirAll(filepath.Dir(s.exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.exe, []byte("stable facet"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out := mustRun(t, "--status"); !strings.Contains(out, "stable executable is now another one") {
		t.Errorf("status does not report the executable change:\n%s", out)
	}
	mustRun(t, "claude")
	calls := s.calls(t)
	last := calls[len(calls)-2:]
	if last[0].Args[1] != "remove" || last[1].Args[1] != "add" || last[1].Args[len(last[1].Args)-2] != s.exe {
		t.Fatalf("re-registration calls = %+v", last)
	}
	if got := dig(readJSON(t, filepath.Join(s.home, ".claude.json")), "mcpServers", "facet", "command"); got != s.exe {
		t.Fatalf("registered %v after rewiring, want %s", got, s.exe)
	}
}

func TestJSONCConfigIsMergedAndRestoredByteForByte(t *testing.T) {
	s := newSandbox(t)
	config := filepath.Join(s.home, ".config", "opencode", "opencode.jsonc")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("// my settings\n{\n\t\"theme\": \"dark\", // keep\n\t/* servers */\n\t\"mcp\": {\n\t\t\"other\": {\"type\": \"remote\", \"url\": \"http://127.0.0.1:9\"},\n\t},\n}\n")
	if err := os.WriteFile(config, original, 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "opencode")
	merged, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	for _, keep := range []string{"// my settings", "// keep", "/* servers */", `"other": {"type": "remote", "url": "http://127.0.0.1:9"},`} {
		if !bytes.Contains(merged, []byte(keep)) {
			t.Errorf("merge lost %q:\n%s", keep, merged)
		}
	}
	doc := readJSON(t, config)
	if dig(doc, "mcp", "facet", "type") != "local" || dig(doc, "mcp", "other", "type") != "remote" || dig(doc, "theme") != "dark" {
		t.Errorf("merged document = %v", doc)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(config), "opencode.json")); !os.IsNotExist(err) {
		t.Error("a second configuration file was created beside the existing one")
	}
	mustRun(t, "--remove", "opencode")
	restored, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, original) {
		t.Errorf("removal did not restore the file:\n%s\nwant:\n%s", restored, original)
	}
}

func TestCodexProjectConfigIsAppendedAndRestored(t *testing.T) {
	s := newSandbox(t)
	config := filepath.Join(s.project, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("model = \"o3\" # mine\n\n[mcp_servers.other]\ncommand = \"other\"\nargs = []")
	if err := os.WriteFile(config, original, 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "codex", "--scope", "project")
	merged, _ := os.ReadFile(config)
	if !bytes.HasPrefix(merged, original) {
		t.Fatalf("existing TOML was rewritten:\n%s", merged)
	}
	if st := inspectTOMLServer(string(merged), "facet"); !st.parsed || st.command != s.exe {
		t.Fatalf("appended table not readable: %+v\n%s", st, merged)
	}
	if st := inspectTOMLServer(string(merged), "other"); !st.parsed || st.command != "other" {
		t.Fatalf("existing table damaged: %+v", st)
	}
	mustRun(t, "--remove", "codex", "--scope", "project")
	restored, _ := os.ReadFile(config)
	if !bytes.Equal(restored, original) {
		t.Fatalf("removal did not restore the file:\n%q\nwant:\n%q", restored, original)
	}
	if _, err := os.Stat(filepath.Join(s.project, ".agents")); !os.IsNotExist(err) {
		t.Error("the project skill directory created by facet wire survived removal")
	}
}

func TestInterruptedRunIsRecoverable(t *testing.T) {
	s := newSandbox(t)
	mustRun(t, "copilot")
	reg := loadRegistry(t, s.home)
	w := reg.Wirings[0]
	// Simulate an interrupted update: the record names a newer digest and
	// keeps the on-disk digest only as an accepted alternative.
	for i := range w.Files {
		w.Files[i].Alt = []string{w.Files[i].Digest}
		w.Files[i].Digest = "sha256:" + strings.Repeat("0", 64)
	}
	w.Pending = true
	if err := reg.Save(RegistryPath(s.home)); err != nil {
		t.Fatal(err)
	}
	if out := mustRun(t, "--status"); !strings.Contains(out, "did not finish") {
		t.Errorf("status does not report the interrupted run:\n%s", out)
	}
	mustRun(t, "copilot")
	reg = loadRegistry(t, s.home)
	if reg.Wirings[0].Pending {
		t.Fatal("rewiring did not complete the interrupted run")
	}
	if out := mustRun(t, "--status"); strings.Contains(out, "drift") {
		t.Errorf("drift after recovery:\n%s", out)
	}
}

func TestApplyRefusesFilesThatAppearAfterPlanning(t *testing.T) {
	s := newSandbox(t)
	e, err := newEnv(testVersion, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	reg := &Registry{Schema: RegistrySchema}
	p := e.planInstall(reg, bundle.TargetOpenCode, bundle.ScopeUser, "")
	if len(p.problems) != 0 || len(p.files) == 0 || p.files[0].op != "create" {
		t.Fatalf("plan = %+v", p)
	}
	late := p.files[0].path
	if err := os.MkdirAll(filepath.Dir(late), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(late, []byte("written meanwhile"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.applyInstall(reg, p); err == nil || !strings.Contains(err.Error(), "appeared") {
		t.Fatalf("apply = %v", err)
	}
	if data, _ := os.ReadFile(late); string(data) != "written meanwhile" {
		t.Fatal("a file that appeared after planning was overwritten")
	}
	if _, err := os.Stat(filepath.Join(s.home, ".config", "opencode", "opencode.json")); !os.IsNotExist(err) {
		t.Fatal("the MCP server was registered although applying failed")
	}
}

func TestRemoveWorksAfterTheProjectIsGone(t *testing.T) {
	s := newSandbox(t)
	mustRun(t, "opencode", "--scope", "project")
	// Not t.Chdir: it holds the previous directory open until cleanup, and
	// Windows cannot delete an open directory. The sandbox's t.Chdir still
	// restores the original working directory.
	if err := os.Chdir(s.root); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(s.project); err != nil {
		t.Fatal(err)
	}
	if _, _, code := run(t, "opencode", "--project", s.project); code != 1 {
		t.Fatal("wiring a missing project directory succeeded")
	}
	mustRun(t, "--remove", "opencode", "--project", s.project)
	if reg := loadRegistry(t, s.home); len(reg.Wirings) != 0 {
		t.Fatalf("registry still records %+v", reg.Wirings)
	}
}

func TestArgumentHandling(t *testing.T) {
	s := newSandbox(t)
	stdout, _, code := run(t, "--help")
	if code != 0 || !strings.Contains(stdout, "facet wire --status") {
		t.Fatalf("help exit %d: %s", code, stdout)
	}
	for name, args := range map[string][]string{
		"no CLI":              {},
		"unknown CLI":         {"app"},
		"unknown option":      {"claude", "--bogus"},
		"bad scope":           {"claude", "--scope", "global"},
		"project with user":   {"claude", "--scope", "user", "--project", s.project},
		"status with others":  {"--status", "claude"},
		"status with remove":  {"--status", "--remove"},
		"scope without value": {"claude", "--scope"},
		"remove without CLI":  {"--remove"},
	} {
		if _, errOut, code := run(t, args...); code != 2 || !strings.Contains(errOut, "Usage:") {
			t.Errorf("%s: exit %d: %s", name, code, errOut)
		}
	}
	if entries, _ := os.ReadDir(s.home); len(entries) != 1 { // only .facet/current from the sandbox
		t.Errorf("argument errors wrote to the home directory: %v", entries)
	}
	req, _, err := parseArgs([]string{"--dry-run", "opencode,claude", "--project=" + s.project})
	if err != nil || !req.dryRun || req.scope != bundle.ScopeProject || len(req.targets) != 2 {
		t.Fatalf("parseArgs = %+v, %v", req, err)
	}
}
