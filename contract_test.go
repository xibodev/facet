package facet

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var productPathPattern = regexp.MustCompile("`((?:skills|packs|agents|schemas|styles|pipeline_defs|\\.agents)/[^`\\s,;:)]+)")
var documentedTargetInstallRoots = []string{".agents/skills"}

func TestDonorGuardChecksTrackedPathNames(t *testing.T) {
	banned := []*regexp.Regexp{
		regexp.MustCompile(`(?i)open[\s._-]*` + "montage"),
		regexp.MustCompile(`(?i)video[\s._-]*` + "kit"),
	}
	tracked := []string{
		"docs/Open" + "Montage.md",
		"packs/video" + "-kit/README.md",
		"docs/FACET.md",
	}
	violations := donorPathViolations(tracked, banned)
	if len(violations) != 2 {
		t.Fatalf("donor path scan found %d violations, want 2: %v", len(violations), violations)
	}
}

func TestRequiredRetainedPortLegalNotice(t *testing.T) {
	data, err := os.ReadFile("THIRD_PARTY_NOTICES.md")
	if err != nil {
		t.Fatal(err)
	}
	notice := strings.Join(strings.Fields(string(data)), " ")
	required := []string{
		"Open" + "Montage",
		"https://github.com/calesthio/" + "Open" + "Montage",
		"cd9f3c1f03368be87b140af494914b8ee4e3c7a4",
		"AGPL-3.0",
		"toolbox contract/registry/process execution",
		"media probe/frame sampling/scene detection",
		"supplied-footage source editing",
		"audio mixing/normalization",
		"technical output review",
		"no donor skills, pipelines, schemas, fixtures, or product guidance",
		"composer implementation in `remotion-composer/src` is independently authored by Facet",
	}
	for _, text := range required {
		if !strings.Contains(notice, text) {
			t.Errorf("THIRD_PARTY_NOTICES.md omits required legal attribution %q", text)
		}
	}
}

func TestLegacyComposerManifestGuardsCurrentSource(t *testing.T) {
	var manifest struct {
		AllowedSourcePaths []string `json:"allowedSourcePaths"`
		BannedLegacyPaths  []string `json:"bannedLegacyPaths"`
		BannedLegacyTokens []string `json:"bannedLegacyTokens"`
	}
	data, err := os.ReadFile(filepath.Join("remotion-composer", "legacy-composer-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	var actual []string
	var source strings.Builder
	err = filepath.WalkDir(filepath.Join("remotion-composer", "src"), func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel("remotion-composer", name)
		if err != nil {
			return err
		}
		actual = append(actual, filepath.ToSlash(relative))
		content, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		source.Write(content)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(actual)
	sort.Strings(manifest.AllowedSourcePaths)
	if strings.Join(actual, "\n") != strings.Join(manifest.AllowedSourcePaths, "\n") {
		t.Fatalf("composer source differs from the clean-room allowlist:\ngot %v\nwant %v", actual, manifest.AllowedSourcePaths)
	}
	for _, name := range manifest.BannedLegacyPaths {
		if _, err := os.Stat(filepath.Join("remotion-composer", filepath.FromSlash(name))); !os.IsNotExist(err) {
			t.Errorf("banned legacy composer path returned: %s", name)
		}
	}
	for _, token := range manifest.BannedLegacyTokens {
		if strings.Contains(source.String(), token) {
			t.Errorf("banned legacy composer token returned: %s", token)
		}
	}
}

func TestDonorTermExceptionIsLimitedToThirdPartyNotices(t *testing.T) {
	tests := map[string]bool{
		"THIRD_PARTY_NOTICES.md":        true,
		"docs/THIRD_PARTY_NOTICES.md":   false,
		"THIRD_PARTY_NOTICES.md.backup": false,
		"README.md":                     false,
	}
	for name, want := range tests {
		if got := isLegalNotice(name); got != want {
			t.Errorf("isLegalNotice(%q) = %v, want %v", name, got, want)
		}
	}
}

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
	tracked := trackedFiles(t)
	violations = append(violations, donorPathViolations(tracked, banned)...)
	for _, name := range tracked {
		if !isProductTextFile(name) {
			continue
		}
		if isLegalNotice(name) {
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

func TestTrackedTextContainsNoPersonalWindowsUserPaths(t *testing.T) {
	pattern := regexp.MustCompile(`(?i)[a-z]:[/\\]+users[/\\]+([^/\\\r\n"'<>]+)`)
	allowed := map[string]bool{"user": true, "test user": true}
	var violations []string
	for _, name := range trackedFiles(t) {
		if !isProductTextFile(name) {
			continue
		}
		data, err := os.ReadFile(filepath.FromSlash(name))
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range pattern.FindAllSubmatch(data, -1) {
			username := strings.ToLower(strings.TrimSpace(string(match[1])))
			if !allowed[username] {
				violations = append(violations, name+": personal Windows user path")
			}
		}
	}
	if len(violations) != 0 {
		t.Fatalf("personal host paths remain:\n%s", strings.Join(violations, "\n"))
	}
}

func isLegalNotice(name string) bool {
	return filepath.ToSlash(name) == "THIRD_PARTY_NOTICES.md"
}

func donorPathViolations(names []string, banned []*regexp.Regexp) []string {
	var violations []string
	for _, name := range names {
		for _, pattern := range banned {
			if pattern.MatchString(name) {
				violations = append(violations, "tracked path:"+name+": "+pattern.String())
			}
		}
	}
	return violations
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

func TestActiveProductDocsDescribeGuidanceNotWorkflowContracts(t *testing.T) {
	files := []string{
		"docs/PRODUCT_MODEL.md",
		"docs/RELEASE_MANIFEST.md",
		"docs/index.html",
		"internal/toolbox/productmodel_test.go",
	}
	banned := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bcanonical\b[^\n]{0,80}\bpipelines?\b`),
		regexp.MustCompile(`(?i)\bpipeline definitions?\b`),
		regexp.MustCompile(`(?i)\b(?:pipeline|style) YAML files?\b`),
		regexp.MustCompile(`(?i)\bdirector skills?\b`),
		regexp.MustCompile(`(?i)\bprovider[-_ ]selection\b`),
		regexp.MustCompile(`(?i)\bworkflow schemas?\b`),
		regexp.MustCompile(`(?i)\bauthoring schemas?\b`),
		regexp.MustCompile(`(?i)\b(?:112|34)\s+files?\b`),
	}
	var violations []string
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, pattern := range banned {
			if match := pattern.Find(data); match != nil {
				violations = append(violations, name+": "+string(match))
			}
		}
	}
	sort.Strings(violations)
	if len(violations) != 0 {
		t.Fatalf("active product docs advertise removed workflow contracts:\n%s",
			strings.Join(violations, "\n"))
	}
}

func TestNpmPackageShipsCanonicalContractAndNotices(t *testing.T) {
	command := exec.Command("npm", "pack", "--dry-run", "--json", "--ignore-scripts")
	raw, err := command.Output()
	if err != nil {
		t.Fatalf("inspect npm package payload: %v", err)
	}
	var result []struct {
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode npm package payload: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("npm pack returned %d payloads, want 1", len(result))
	}
	shipped := make(map[string]bool, len(result[0].Files))
	for _, file := range result[0].Files {
		shipped[filepath.ToSlash(file.Path)] = true
	}

	required := []string{
		"LICENSE",
		"THIRD_PARTY_NOTICES.md",
		"skills/facet/SKILL.md",
		"agents/facet-creative.md",
	}
	for _, pack := range PackNames() {
		required = append(required, "packs/"+pack+"/SKILL.md")
	}
	var missing []string
	for _, name := range required {
		if !shipped[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) != 0 {
		t.Fatalf("npm package omits canonical product files:\n%s", strings.Join(missing, "\n"))
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

func TestLocalUATAllowsAnExplicitNPMRegistry(t *testing.T) {
	checks := map[string][]string{
		".release-harness/docker/Dockerfile": {
			"ARG NPM_REGISTRY=https://registry.npmjs.org",
			`npm install --global opencode-ai@1.18.29 --registry="${NPM_REGISTRY}"`,
			`npm install --prefix /opt/uat --registry="${NPM_REGISTRY}"`,
		},
		"docker-compose.test.yml": {
			"NPM_REGISTRY: ${NPM_REGISTRY:-https://registry.npmjs.org}",
		},
	}
	for name, required := range checks {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, term := range required {
			if !strings.Contains(string(data), term) {
				t.Errorf("%s does not contain %q", name, term)
			}
		}
	}
}

func TestLocalUATIncludesTheInstallerPackage(t *testing.T) {
	data, err := os.ReadFile(".dockerignore")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"!installer/", "!installer/**"} {
		if !strings.Contains(string(data), required) {
			t.Errorf(".dockerignore does not contain %q", required)
		}
	}
}

func TestLocalUATRunsTheInstallerNonInteractively(t *testing.T) {
	data, err := os.ReadFile(".release-harness/docker/Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"bash install.sh --yes --target opencode --project /home/facet/studio --install-dir /home/facet/.facet/runtime --components none",
		"ln -s ../runtime/bin/facet /home/facet/.facet/bin/facet",
	} {
		if !strings.Contains(string(data), required) {
			t.Errorf(".release-harness/docker/Dockerfile does not contain %q", required)
		}
	}
}

func TestMarkdownProductPathsDistinguishesTargetInstallRoots(t *testing.T) {
	tests := []struct {
		name           string
		reference      string
		wantViolations int
	}{
		{name: "target root", reference: ".agents/skills"},
		{name: "target descendant", reference: ".agents/skills/facet/SKILL.md"},
		{name: "dangling repository path", reference: "skills/missing-contract/SKILL.md", wantViolations: 1},
		{name: "adjacent agents path", reference: ".agents/missing-contract/SKILL.md", wantViolations: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			name := filepath.Join(t.TempDir(), "contract.md")
			if err := os.WriteFile(name, []byte("`"+test.reference+"`"), 0o600); err != nil {
				t.Fatal(err)
			}
			var violations []string
			checkMarkdownProductPaths(name, &violations)
			if len(violations) != test.wantViolations {
				t.Fatalf("checkMarkdownProductPaths(%q) found %d violations, want %d: %v",
					test.reference, len(violations), test.wantViolations, violations)
			}
		})
	}
}

func TestCanonicalGuidanceRequiresExplicitOutputReviewExpectations(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("skills", "facet", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, required := range []string{
		"explicit expected profile",
		"duration",
		"audio presence",
		"`assumed`",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("canonical guidance does not explain output review expectation %q", required)
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
		if isDocumentedTargetInstallPath(ref) {
			continue
		}
		if _, err := os.Stat(filepath.FromSlash(ref)); err != nil {
			*violations = append(*violations, filepath.ToSlash(name)+": missing "+match[1])
		}
	}
}

func isDocumentedTargetInstallPath(name string) bool {
	clean := path.Clean(name)
	for _, root := range documentedTargetInstallRoots {
		if clean == root || strings.HasPrefix(clean, root+"/") {
			return true
		}
	}
	return false
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
	command := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	raw, err := command.Output()
	if err != nil {
		t.Fatalf("list tracked files: %v", err)
	}
	parts := strings.Split(string(raw), "\x00")
	files := make([]string, 0, len(parts))
	for _, name := range parts {
		if name != "" {
			if _, err := os.Stat(filepath.FromSlash(name)); os.IsNotExist(err) {
				continue
			}
			files = append(files, filepath.ToSlash(name))
		}
	}
	sort.Strings(files)
	return files
}
