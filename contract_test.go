package facet

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var productPathPattern = regexp.MustCompile("`((?:skills|packs|agents|schemas|styles|pipeline_defs|\\.agents)/[^`\\s,;:)]+)")

func TestProductContractContainsNoBannedTerms(t *testing.T) {
	banned := []*regexp.Regexp{
		regexp.MustCompile(`(?i)open[\s._-]*` + "montage"),
		regexp.MustCompile(`(?i)video[\s._-]*` + "kit"),
	}
	var violations []string
	err := fs.WalkDir(Assets, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !isProductTextFile(name) {
			return err
		}
		data, err := Assets.ReadFile(name)
		if err != nil {
			return err
		}
		for _, pattern := range banned {
			if pattern.Match(data) {
				violations = append(violations, "embedded:"+name+": "+pattern.String())
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range trackedFiles(t) {
		if !isProductTextFile(name) {
			continue
		}
		data, err := os.ReadFile(filepath.FromSlash(name))
		if err != nil {
			t.Fatal(err)
		}
		for _, pattern := range banned {
			if pattern.Match(data) {
				violations = append(violations, name+": "+pattern.String())
			}
		}
	}
	sort.Strings(violations)
	if len(violations) != 0 {
		t.Fatalf("banned donor product terms remain:\n%s", strings.Join(violations, "\n"))
	}
}

func TestRetainedPacksReferenceOnlyExistingFacetAssets(t *testing.T) {
	entries, err := os.ReadDir("packs")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no Facet packs retained")
	}

	var violations []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		packDir := filepath.Join("packs", entry.Name())
		manifestPath := filepath.Join(packDir, "facet-pack.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			violations = append(violations, filepath.ToSlash(manifestPath)+": missing manifest")
			continue
		}
		var manifest struct {
			Exports map[string]json.RawMessage `json:"exports"`
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			violations = append(violations, filepath.ToSlash(manifestPath)+": invalid JSON")
			continue
		}
		for kind, raw := range manifest.Exports {
			switch kind {
			case "skills":
				var skills []struct {
					Path string `json:"path"`
				}
				if err := json.Unmarshal(raw, &skills); err != nil {
					violations = append(violations, filepath.ToSlash(manifestPath)+": invalid skills export")
					continue
				}
				for _, skill := range skills {
					checkPackPath(packDir, manifestPath, skill.Path, &violations)
				}
			case "schemas", "styles":
				var paths []string
				if err := json.Unmarshal(raw, &paths); err != nil {
					violations = append(violations, filepath.ToSlash(manifestPath)+": invalid "+kind+" export")
					continue
				}
				for _, name := range paths {
					checkPackPath(packDir, manifestPath, name, &violations)
				}
			default:
				violations = append(violations, filepath.ToSlash(manifestPath)+": legacy export "+kind)
			}
		}

		for _, name := range []string{filepath.Join(packDir, "SKILL.md")} {
			checkMarkdownProductPaths(name, &violations)
		}
	}
	for _, name := range []string{
		"README.md",
		"OBJECTIVE.md",
		filepath.Join("skills", "facet", "SKILL.md"),
		filepath.Join("agents", "facet-creative.md"),
	} {
		checkMarkdownProductPaths(name, &violations)
	}

	sort.Strings(violations)
	if len(violations) != 0 {
		t.Fatalf("retained contract has dangling or legacy references:\n%s", strings.Join(violations, "\n"))
	}
}

func TestRemovedDonorSurfacesStayRemoved(t *testing.T) {
	legacyCommand := "cmd/" + "video" + "kit"
	removed := []string{
		".agents/skills",
		legacyCommand,
		"pipeline_defs",
		"skills/core",
		"skills/creative",
		"skills/meta",
		"skills/methods",
		"skills/pipelines",
		"skills/shared",
		"schemas/artifacts",
		"schemas/checkpoints",
		"schemas/pipelines",
		"schemas/styles",
		"projects",
		"fixtures/module/describe.json",
		"provenance/video-agent-bundle",
		".claude/skills/facet",
		"SKILL.md",
		"DESIGN.md",
		"DONORS.md",
		"PHASE1_SLICE.md",
		"PIPELINE_MAP.md",
		"TOOL_PORT_MATRIX.md",
		"UPSTREAM_SURFACE_CENSUS.md",
	}
	var found []string
	tracked := trackedFiles(t)
	for _, name := range removed {
		for _, trackedName := range tracked {
			if trackedName == name || strings.HasPrefix(trackedName, name+"/") {
				found = append(found, name+" (tracked as "+trackedName+")")
				break
			}
		}
		if _, err := os.Stat(filepath.FromSlash(name)); err == nil {
			found = append(found, name)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	if len(found) != 0 {
		t.Fatalf("removed donor surfaces returned: %s", strings.Join(found, ", "))
	}
}

func TestShippedSchemasAreStatelessToolContracts(t *testing.T) {
	var violations []string
	for _, name := range trackedFiles(t) {
		if !strings.HasPrefix(name, "schemas/") {
			continue
		}
		if strings.HasPrefix(name, "schemas/tools/") && strings.HasSuffix(name, ".schema.json") {
			continue
		}
		violations = append(violations, name)
	}
	if len(violations) != 0 {
		t.Fatalf("shipped schema tree contains non-tool or workflow contracts:\n%s",
			strings.Join(violations, "\n"))
	}
}

func TestRetainedPackMetadataDescribesGuidanceNotPipelines(t *testing.T) {
	entries, err := os.ReadDir("packs")
	if err != nil {
		t.Fatal(err)
	}
	banned := regexp.MustCompile(`(?i)\b(pipelines?|director skills?|styles?)\b`)
	var violations []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := filepath.Join("packs", entry.Name(), "package.json")
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		var manifest struct {
			Description string `json:"description"`
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if match := banned.FindString(manifest.Description); match != "" {
			violations = append(violations, filepath.ToSlash(name)+": "+match)
		}
	}
	sort.Strings(violations)
	if len(violations) != 0 {
		t.Fatalf("retained pack metadata advertises removed workflow contracts:\n%s",
			strings.Join(violations, "\n"))
	}
}

func TestReleasePackagingExcludesRemovedProductSurfaces(t *testing.T) {
	checks := map[string][]string{
		"scripts/package-release.py": {
			`"PROVENANCE.md"`,
			`"pipeline_defs"`,
			`"styles"`,
		},
		".dockerignore": {
			"!pipeline_defs/",
			"!pipeline_defs/**",
			"!styles/",
			"!styles/**",
		},
	}
	var violations []string
	for name, removed := range checks {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, term := range removed {
			if strings.Contains(string(data), term) {
				violations = append(violations, name+": "+term)
			}
		}
	}
	sort.Strings(violations)
	if len(violations) != 0 {
		t.Fatalf("release packaging still includes removed product surfaces:\n%s", strings.Join(violations, "\n"))
	}
}

func TestStudioProjectionUsesCanonicalContract(t *testing.T) {
	files := []string{"web/DESIGN.md", "web/web.go"}
	banned := []string{
		"working and supported",
		"FROZEN.md",
		"six evidence stages",
		"permission prompts are disabled",
	}
	var violations []string
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(string(data))
		for _, term := range banned {
			if strings.Contains(lower, strings.ToLower(term)) {
				violations = append(violations, name+": "+term)
			}
		}
	}
	sort.Strings(violations)
	if len(violations) != 0 {
		t.Fatalf("Studio projection contradicts the canonical contract:\n%s", strings.Join(violations, "\n"))
	}
}

func TestProductTextGuardCoversShippingDefinitions(t *testing.T) {
	for _, name := range []string{".dockerignore", "Dockerfile", "scripts/package-release.py"} {
		if !isProductTextFile(name) {
			t.Errorf("product text guard skips %s", name)
		}
	}
}

func checkPackPath(packDir, manifestPath, name string, violations *[]string) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if name == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		*violations = append(*violations, filepath.ToSlash(manifestPath)+": non-Facet path "+name)
		return
	}
	if _, err := os.Stat(filepath.Join(packDir, clean)); err != nil {
		*violations = append(*violations, filepath.ToSlash(manifestPath)+": missing "+name)
	}
}

func checkMarkdownProductPaths(name string, violations *[]string) {
	data, err := os.ReadFile(name)
	if err != nil {
		*violations = append(*violations, filepath.ToSlash(name)+": missing contract file")
		return
	}
	for _, match := range productPathPattern.FindAllStringSubmatch(string(data), -1) {
		ref := strings.TrimSuffix(match[1], "/")
		if strings.ContainsAny(ref, "*<>") {
			*violations = append(*violations, filepath.ToSlash(name)+": non-deterministic path "+match[1])
			continue
		}
		if _, err := os.Stat(filepath.FromSlash(ref)); err != nil {
			*violations = append(*violations, filepath.ToSlash(name)+": missing "+match[1])
		}
	}
}

func isProductTextFile(name string) bool {
	switch strings.ToLower(filepath.Base(name)) {
	case ".dockerignore", ".gitignore":
		return true
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return true
	}
	switch ext {
	case ".cjs", ".css", ".go", ".html", ".js", ".json", ".md", ".mjs", ".ps1", ".py", ".sh", ".ts", ".tsv", ".tsx", ".txt", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func trackedFiles(t *testing.T) []string {
	t.Helper()
	command := exec.Command("git", "ls-files", "-z")
	raw, err := command.Output()
	if err != nil {
		t.Fatalf("list tracked files: %v", err)
	}
	parts := strings.Split(string(raw), "\x00")
	files := make([]string, 0, len(parts))
	for _, name := range parts {
		if name != "" {
			files = append(files, filepath.ToSlash(name))
		}
	}
	sort.Strings(files)
	return files
}
