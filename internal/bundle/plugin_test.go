package bundle

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/xibodev/facet/internal/toolbox"
)

var pluginTargets = []Target{TargetClaude, TargetCodex, TargetCopilot}

func readJSONFile(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return out
}

func stringList(v any) []string {
	list, _ := v.([]any)
	out := []string{}
	for _, item := range list {
		s, _ := item.(string)
		out = append(out, s)
	}
	return out
}

// A plugin is the same guidance as every other layout plus the CLI's plugin
// manifest and an MCP registration that runs facet from PATH.
func TestPluginLayoutsCarryGuidanceManifestAndServer(t *testing.T) {
	for _, target := range pluginTargets {
		dir := filepath.Join(t.TempDir(), string(target))
		m := build(t, target, ScopePlugin, dir)
		if m.Scope != ScopePlugin || m.InstallBase != "plugin" || m.InstallRoot != "" {
			t.Errorf("%s: manifest identity = %+v", target, m)
		}
		if got, err := os.ReadFile(filepath.Join(dir, "skills", "facet", "SKILL.md")); err != nil || !bytes.Equal(got, canonical(t, "skills/facet/SKILL.md")) {
			t.Errorf("%s: the core skill is not at the plugin root: %v", target, err)
		}
		if _, err := Verify(dir, Expect{Target: target, Scope: ScopePlugin, Version: testVersion, Tools: testTools, Current: true}); err != nil {
			t.Errorf("%s: a fresh plugin failed verification: %v", target, err)
		}

		manifest := readJSONFile(t, filepath.Join(dir, filepath.FromSlash(PluginManifestPath(target))))
		if manifest["name"] != PluginName || manifest["version"] != testVersion || manifest["license"] != PluginLicense ||
			manifest["repository"] != PluginRepository || manifest["homepage"] != PluginWebsite {
			t.Errorf("%s: plugin manifest = %v", target, manifest)
		}
		if desc, _ := manifest["description"].(string); !strings.Contains(desc, "Facet Toolkit") || !strings.Contains(desc, PluginWebsite) {
			t.Errorf("%s: the description does not say the Toolkit is needed: %q", target, desc)
		}

		servers, _ := readJSONFile(t, filepath.Join(dir, ".mcp.json"))["mcpServers"].(map[string]any)
		server, _ := servers[MCPServerName].(map[string]any)
		if len(servers) != 1 || server["command"] != PluginCommand || !reflect.DeepEqual(stringList(server["args"]), MCPServerArgs()) {
			t.Fatalf("%s: .mcp.json servers = %v", target, servers)
		}
		switch target {
		case TargetClaude:
			// Claude Code starts plugin servers in the project and offers it
			// as the MCP root; a cwd would only move them.
			if _, ok := server["cwd"]; ok || len(server) != 2 {
				t.Errorf("claude: server entry = %v", server)
			}
		case TargetCopilot:
			// Copilot CLI would start the server in the plugin directory.
			if server["cwd"] != "." || server["type"] != "local" || !reflect.DeepEqual(stringList(server["tools"]), []string{"*"}) {
				t.Errorf("copilot: server entry = %v", server)
			}
			if msg, _ := manifest["postInstallMessage"].(string); !strings.Contains(msg, PluginWebsite) {
				t.Errorf("copilot: the post-install message does not point at the Toolkit: %q", msg)
			}
			if !reflect.DeepEqual(stringList(manifest["skills"]), []string{"skills/"}) || !reflect.DeepEqual(stringList(manifest["agents"]), []string{"agents/"}) {
				t.Errorf("copilot: component paths = %v %v", manifest["skills"], manifest["agents"])
			}
		case TargetCodex:
			if manifest["skills"] != "./skills/" || manifest["mcpServers"] != "./.mcp.json" {
				t.Errorf("codex: component paths = %v %v", manifest["skills"], manifest["mcpServers"])
			}
			if ui, _ := manifest["interface"].(map[string]any); ui["displayName"] != "Facet" || len(stringList(ui["defaultPrompt"])) > 3 {
				t.Errorf("codex: interface = %v", ui)
			}
			if !reflect.DeepEqual(stringList(server["env_vars"]), toolbox.EnvVars()) {
				t.Errorf("codex: env_vars = %v, want %v", server["env_vars"], toolbox.EnvVars())
			}
			if server["tool_timeout_sec"] != float64(CodexToolTimeoutSec) {
				t.Errorf("codex: tool_timeout_sec = %v", server["tool_timeout_sec"])
			}
			tools, _ := server["tools"].(map[string]any)
			var asked []string
			for tool, cfg := range tools {
				if mode, _ := cfg.(map[string]any)["approval_mode"].(string); mode == "prompt" {
					asked = append(asked, tool)
				}
			}
			sort.Strings(asked)
			if paid := toolbox.PaidTools(); len(paid) == 0 || !reflect.DeepEqual(asked, paid) || len(tools) != len(paid) {
				t.Errorf("codex: asks before %v, want every paid tool %v", asked, paid)
			}
		}
	}
}

func TestPluginScopeIsOnlyForCLIsWithAPluginFormat(t *testing.T) {
	for _, target := range Targets() {
		want := target == TargetClaude || target == TargetCodex || target == TargetCopilot
		if SupportsScope(target, ScopePlugin) != want {
			t.Errorf("SupportsScope(%s, plugin) = %v", target, !want)
		}
		if _, _, err := Plan(Options{Target: target, Scope: ScopePlugin, Version: testVersion}); (err == nil) != want {
			t.Errorf("Plan(%s, plugin) error = %v", target, err)
		}
	}
	// facet wire installs at user or project scope; only the bundle has a
	// plugin layout.
	if _, err := ParseScope("plugin"); err == nil {
		t.Error("ParseScope accepted plugin, which facet wire would then try to install")
	}
	if s, err := ParseBundleScope("Plugin"); err != nil || s != ScopePlugin {
		t.Errorf("ParseBundleScope(Plugin) = %q, %v", s, err)
	}
	if _, err := ParseBundleScope("global"); err == nil {
		t.Error("ParseBundleScope accepted an unknown scope")
	}

	out := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := CLI([]string{"--target", "all", "--scope", "plugin", "--out", out}, &stdout, &stderr, testVersion); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	var built []string
	for _, e := range entries {
		built = append(built, e.Name())
	}
	if strings.Join(built, ",") != "claude,codex,copilot" {
		t.Errorf("all at plugin scope built %v", built)
	}
	stderr.Reset()
	if code := CLI([]string{"--target", "opencode", "--scope", "plugin", "--out", out}, io.Discard, &stderr, testVersion); code != 2 || !strings.Contains(stderr.String(), "no plugin layout") {
		t.Errorf("opencode at plugin scope: exit %d, %s", code, stderr.String())
	}
}

// The plugins in the repository are what the marketplaces serve. They must
// be exactly what this facet projects at the release version in
// package.json, so a plugin never carries guidance the Toolkit of the same
// release does not.
//
// To regenerate them after changing guidance, the plugin layout or the
// version: FACET_UPDATE_PLUGINS=1 go test ./internal/bundle -run TestCommittedPluginsMatchThisFacet
func TestCommittedPluginsMatchThisFacet(t *testing.T) {
	repo := filepath.Join("..", "..")
	var pkg struct {
		Version string `json:"version"`
		License string `json:"license"`
	}
	data, err := os.ReadFile(filepath.Join(repo, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &pkg); err != nil || pkg.Version == "" {
		t.Fatalf("package.json: version %q, %v", pkg.Version, err)
	}
	if pkg.License != PluginLicense {
		t.Errorf("plugins declare license %s, package.json %s", PluginLicense, pkg.License)
	}
	tools := toolbox.Names()
	update := os.Getenv("FACET_UPDATE_PLUGINS") == "1"
	for _, target := range pluginTargets {
		committed := filepath.Join(repo, "plugins", string(target))
		if update {
			if _, err := Build(Options{Target: target, Scope: ScopePlugin, Version: pkg.Version, Tools: tools}, committed); err != nil {
				t.Fatalf("regenerating %s: %v", committed, err)
			}
		}
		want := filepath.Join(t.TempDir(), string(target))
		if _, err := Build(Options{Target: target, Scope: ScopePlugin, Version: pkg.Version, Tools: tools}, want); err != nil {
			t.Fatal(err)
		}
		if _, err := Verify(committed, Expect{Target: target, Scope: ScopePlugin, Version: pkg.Version, Tools: tools, Current: true}); err != nil {
			t.Errorf("plugins/%s: %v; regenerate with FACET_UPDATE_PLUGINS=1 go test ./internal/bundle -run TestCommittedPluginsMatchThisFacet", target, err)
			continue
		}
		got, wantFiles := snapshot(t, committed), snapshot(t, want)
		if !reflect.DeepEqual(got, wantFiles) {
			for name := range wantFiles {
				if got[name] != wantFiles[name] {
					t.Errorf("plugins/%s/%s differs from this facet's projection (a CRLF checkout? see .gitattributes)", target, name)
				}
			}
			for name := range got {
				if _, ok := wantFiles[name]; !ok {
					t.Errorf("plugins/%s/%s is not part of the plugin", target, name)
				}
			}
		}
	}
	// No other directory may pose as a plugin.
	entries, err := os.ReadDir(filepath.Join(repo, "plugins"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !SupportsScope(Target(e.Name()), ScopePlugin) {
			t.Errorf("plugins/%s is not a Facet plugin", e.Name())
		}
	}
}
