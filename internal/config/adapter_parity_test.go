package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xibodev/facet/internal/bundle"
)

func TestInstallerAdapterParity(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "installer", "manifest.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	hosts := map[string]string{}
	instructions := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) < 4 {
			continue
		}
		switch fields[0] {
		case "host":
			hosts[fields[1]] = fields[3]
		case "instruction":
			instructions[strings.TrimSuffix(fields[1], "-instructions")] = fields[3]
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	for _, engine := range []string{"claude", "copilot", "codex", "opencode", "studio"} {
		wantRoot := filepath.ToSlash(getSkillsTargetPath("", engine, ""))
		wantRoot = strings.TrimSuffix(wantRoot, "/facet")
		if hosts[engine] != wantRoot {
			t.Errorf("%s manifest skill root = %q, init = %q", engine, hosts[engine], wantRoot)
		}
		if instructions[engine] != instructionPathForEngine(engine) {
			t.Errorf("%s manifest instruction = %q, init = %q", engine, instructions[engine], instructionPathForEngine(engine))
		}
	}
	if hosts["codex"] != ".agents/skills" {
		t.Fatalf("Codex project skills must use .agents/skills, got %q", hosts["codex"])
	}

	for engine, target := range map[string]bundle.Target{
		"claude": bundle.TargetClaude, "copilot": bundle.TargetCopilot,
		"codex": bundle.TargetCodex, "opencode": bundle.TargetOpenCode,
	} {
		out := t.TempDir()
		manifest, err := bundle.Build(bundle.Source{
			SkillsDir:    filepath.Join("..", "..", "skills", "facet"),
			PacksDir:     filepath.Join("..", "..", "packs"),
			Tools:        []string{"media_probe"},
			FacetVersion: "parity-test",
		}, target, out)
		if err != nil {
			t.Fatal(err)
		}
		wantInstallRoot := strings.TrimSuffix(hosts[engine], "/skills")
		if manifest.Compatibility.InstallRoot != wantInstallRoot {
			t.Errorf("%s bundle root = %q, installer root = %q", engine, manifest.Compatibility.InstallRoot, wantInstallRoot)
		}
	}
}
