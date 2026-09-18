package studio

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogOperations(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmpDir)
	t.Setenv("HOME", tmpDir)

	// 1. Initial Load should be empty
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}
	if len(cat.Projects) != 0 {
		t.Errorf("expected empty projects, got %d", len(cat.Projects))
	}

	// 2. Register project
	projDir := filepath.Join(tmpDir, "my-test-proj")
	_ = os.MkdirAll(projDir, 0755)

	registered, err := RegisterOrUpdateProject("My Test Video", projDir, "claude", []string{"explainer"})
	if err != nil {
		t.Fatalf("RegisterOrUpdateProject failed: %v", err)
	}
	if registered.Name != "My Test Video" {
		t.Errorf("expected name 'My Test Video', got %s", registered.Name)
	}
	if len(registered.Packs) != 1 || registered.Packs[0] != "explainer" {
		t.Errorf("expected pack 'explainer', got %v", registered.Packs)
	}

	// 3. Reload catalog and verify persistence
	cat2, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog reload failed: %v", err)
	}
	if len(cat2.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(cat2.Projects))
	}
	if !cat2.Projects[0].Exists {
		t.Errorf("expected project to exist on disk")
	}
}

func TestCreateProjectRefusesExistingAndEscapingDirectory(t *testing.T) {
	root := t.TempDir()
	if _, err := CreateNewProject("Existing", "existing", root, "studio", nil, root); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNewProject("Existing", "existing", root, "studio", nil, root); err == nil {
		t.Fatal("existing project silently reused")
	}
	for _, slug := range []string{"../escape", "nested/folder", "nested\\folder", "."} {
		if _, err := CreateNewProject("Invalid", slug, root, "studio", nil, root); err == nil {
			t.Fatalf("unsafe slug accepted: %s", slug)
		}
	}
}

func TestDiscoverPacks(t *testing.T) {
	// Root dir of the workspace
	packs := DiscoverAvailablePacks(".")
	if len(packs) == 0 {
		t.Fatal("expected at least one pack discovered")
	}

	foundExplainer := false
	for _, p := range packs {
		if p.ID == "explainer" {
			foundExplainer = true
			if !p.Installed {
				t.Errorf("expected explainer pack to be marked installed")
			}
			break
		}
	}
	if !foundExplainer {
		t.Errorf("expected explainer pack in discovered packs list: %v", packs)
	}
}

func TestPackEndpointFallbackExposesOnlyRetainedSkillGuidance(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "workspace", "project")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOCALAPPDATA", filepath.Join(tmpDir, "appdata"))
	t.Setenv("HOME", tmpDir)

	server := NewServer(rootDir)
	req := httptest.NewRequest(http.MethodGet, "/api/packs", nil)
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Origin", "http://127.0.0.1:8787")
	rec := httptest.NewRecorder()
	server.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/packs returned %d: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		Packs []map[string]any `json:"packs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode fallback packs: %v", err)
	}
	packs := response.Packs
	if len(packs) == 0 {
		t.Fatal("expected Facet-owned fallback packs")
	}

	retainedSkills := map[string]bool{
		"explainer":           false,
		"cinematic":           false,
		"screen-demo":         false,
		"talking-head":        false,
		"social":              false,
		"character-animation": false,
		"localization":        false,
	}
	deletedIDs := map[string]struct{}{
		"animated-explainer":  {},
		"documentary-montage": {},
		"avatar-spokesperson": {},
		"clip-factory":        {},
		"podcast-repurpose":   {},
		"localization-dub":    {},
		"explainer-producer":  {},
		"animation":           {},
	}

	var findDeletedID func(any) string
	findDeletedID = func(value any) string {
		switch value := value.(type) {
		case string:
			if _, deleted := deletedIDs[value]; deleted {
				return value
			}
		case []any:
			for _, item := range value {
				if id := findDeletedID(item); id != "" {
					return id
				}
			}
		case map[string]any:
			for _, item := range value {
				if id := findDeletedID(item); id != "" {
					return id
				}
			}
		}
		return ""
	}

	for _, pack := range packs {
		if _, exposed := pack["pipelines"]; exposed {
			t.Fatalf("fallback pack exposes deleted pipelines contract: %#v", pack)
		}
		if id := findDeletedID(pack); id != "" {
			t.Errorf("fallback pack exposes deleted ID %q", id)
		}
		skills, ok := pack["skills"].([]any)
		if !ok {
			t.Fatalf("fallback pack has invalid skills: %#v", pack)
		}
		for _, value := range skills {
			id, ok := value.(string)
			if !ok {
				t.Fatalf("fallback pack has non-string skill ID: %#v", pack)
			}
			if _, retained := retainedSkills[id]; !retained {
				t.Errorf("fallback pack exposes non-canonical skill ID %q", id)
				continue
			}
			retainedSkills[id] = true
		}
	}
	for id, found := range retainedSkills {
		if !found {
			t.Errorf("fallback packs omit retained skill guidance %q", id)
		}
	}
}

func TestCatalogEndpoints(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmpDir)
	t.Setenv("HOME", tmpDir)

	server := NewServer(tmpDir)

	newReq := func(method, target string, body []byte) *http.Request {
		var r *http.Request
		if len(body) > 0 {
			r = httptest.NewRequest(method, target, bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
		} else {
			r = httptest.NewRequest(method, target, nil)
		}
		r.Host = "127.0.0.1:8787"
		r.Header.Set("Origin", "http://127.0.0.1:8787")
		return r
	}

	// 1. GET /api/catalog
	req := newReq("GET", "/api/catalog", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/catalog returned %d: %s", w.Code, w.Body.String())
	}

	// 2. GET /api/packs
	reqPacks := newReq("GET", "/api/packs", nil)
	wPacks := httptest.NewRecorder()
	server.mux.ServeHTTP(wPacks, reqPacks)
	if wPacks.Code != http.StatusOK {
		t.Fatalf("GET /api/packs returned %d: %s", wPacks.Code, wPacks.Body.String())
	}

	// 3. POST /api/catalog/new
	newPayload := map[string]any{
		"name":      "Unit Test Production",
		"slug":      "unit-test-prod",
		"directory": filepath.Join(tmpDir, "productions"),
		"engine":    "claude",
		"packs":     []string{"explainer"},
	}
	body, _ := json.Marshal(newPayload)
	reqNew := newReq("POST", "/api/catalog/new", body)
	wNew := httptest.NewRecorder()
	server.mux.ServeHTTP(wNew, reqNew)
	if wNew.Code != http.StatusOK {
		t.Fatalf("POST /api/catalog/new returned %d: %s", wNew.Code, wNew.Body.String())
	}

	// Verify project was created on disk
	createdDir := filepath.Join(tmpDir, "productions", "unit-test-prod")
	if _, err := os.Stat(createdDir); err != nil {
		t.Fatalf("expected created project directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(createdDir, "facet.lock.json")); err != nil {
		t.Errorf("expected facet.lock.json in created project: %v", err)
	}

	// 4. POST /api/catalog/open
	openPayload := map[string]any{
		"path":   createdDir,
		"engine": "claude",
	}
	openBody, _ := json.Marshal(openPayload)
	reqOpen := newReq("POST", "/api/catalog/open", openBody)
	wOpen := httptest.NewRecorder()
	server.mux.ServeHTTP(wOpen, reqOpen)
	if wOpen.Code != http.StatusOK {
		t.Fatalf("POST /api/catalog/open returned %d: %s", wOpen.Code, wOpen.Body.String())
	}
}

func TestCatalogProjectJourney(t *testing.T) {
	for _, withProjects := range []bool{false, true} {
		name := "catalog-only"
		if withProjects {
			name = "with-local-projects"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if withProjects {
				mustWriteTestFile(t, filepath.Join(root, "projects", "local", "brief.md"), "# Local")
			}
			// The catalog ID intentionally differs from the external folder name.
			project := filepath.Join(t.TempDir(), "external-folder")
			files := map[string]string{
				"brief.md":                     "# External Production",
				"artifacts/script.json":        `{"title":"Script","beats":[{"narration":"Hello"}]}`,
				"artifacts/edit.json":          `{"clips":[]}`,
				"narration/voice 100% #1.mp3":  "audio",
				"qa/frame #1.png":              "image",
				"review/final-frames/shot.png": "review image",
				"review/report.json":           `{"status":"passed"}`,
				"renders/final.mp4":            "video",
				"facet.lock.json":              `{"engine":"claude"}`,
			}
			for path, content := range files {
				mustWriteTestFile(t, filepath.Join(project, filepath.FromSlash(path)), content)
			}
			registered, err := RegisterOrUpdateProject("Catalog Movie", project, "codex", nil, root)
			if err != nil {
				t.Fatal(err)
			}
			server := NewServer(root)
			get := func(path string) *httptest.ResponseRecorder {
				t.Helper()
				rec := httptest.NewRecorder()
				server.Handler().ServeHTTP(rec, newSecurityRequest(http.MethodGet, path, ""))
				if rec.Code != http.StatusOK {
					t.Fatalf("GET %s = %d: %s", path, rec.Code, rec.Body.String())
				}
				return rec
			}
			var summaries []ProjectSummary
			if err := json.Unmarshal(get("/api/projects").Body.Bytes(), &summaries); err != nil {
				t.Fatal(err)
			}
			var summary *ProjectSummary
			for i := range summaries {
				if summaries[i].Slug == registered.ID {
					summary = &summaries[i]
				}
			}
			if summary == nil || summary.Engine != "codex" || summary.Path != project {
				t.Fatalf("missing catalog identity/engine: %#v", summary)
			}
			var details ProjectDetails
			if err := json.Unmarshal(get("/api/projects/"+registered.ID).Body.Bytes(), &details); err != nil {
				t.Fatal(err)
			}
			if details.Engine != "codex" || details.Path != project || details.Brief != files["brief.md"] || len(details.Beats) != 1 {
				t.Fatalf("catalog details = %#v", details)
			}
			if len(details.Narration) != 1 || len(details.QAFrames) != 1 || len(details.ReviewFrames) != 1 {
				t.Fatalf("missing catalog media: %#v", details)
			}
			urls := map[string]string{
				details.BriefURL:            files["brief.md"],
				details.ScriptURL:           files["artifacts/script.json"],
				details.CompositionURL:      files["artifacts/edit.json"],
				details.Narration[0].URL:    "audio",
				details.QAFrames[0].URL:     "image",
				details.ReviewFrames[0].URL: "review image",
				details.ThumbnailURL:        "review image",
				details.VideoURL:            "video",
			}
			for mediaURL, content := range urls {
				if !strings.HasPrefix(mediaURL, "/api/media/catalog/"+registered.ID+"/") {
					t.Fatalf("media URL lacks catalog ID: %q", mediaURL)
				}
				if got := get(mediaURL).Body.String(); got != content {
					t.Errorf("GET %s = %q, want %q", mediaURL, got, content)
				}
			}
			if summary.VideoURL != details.VideoURL || summary.ThumbnailURL != details.ThumbnailURL {
				t.Fatal("summary and details media URLs differ")
			}
			// Existing basename aliases must still produce URLs using the catalog ID.
			alias, err := GetProjectDetails(root, filepath.Base(project))
			if err != nil || alias.VideoURL != details.VideoURL {
				t.Fatalf("catalog alias details = %#v, %v", alias, err)
			}
			for _, dir := range []string{details.Path, filepath.Join(project, "artifacts")} {
				sess, err := server.newSession(dir, "rw", details.Engine)
				if err != nil {
					t.Fatalf("catalog chat session: %v", err)
				}
				canonical, err := canonicalExistingPath(dir)
				if err != nil || sess.Dir != canonical || sess.Engine != "codex" {
					t.Fatalf("session dir/engine = %q/%q, want %q/codex: %v", sess.Dir, sess.Engine, canonical, err)
				}
			}
			if _, err := server.resolveSessionDir(filepath.Dir(project)); err == nil {
				t.Fatal("unregistered parent directory accepted for chat")
			}
			mustWriteTestFile(t, filepath.Join(project, "private.env"), "not media")
			for _, path := range []string{
				"/api/media/catalog/" + registered.ID + "/private.env",
				"/api/media/catalog/" + registered.ID + "/%2e%2e/private.txt",
				"/api/projects/" + url.PathEscape("../external-folder"),
			} {
				rec := httptest.NewRecorder()
				server.Handler().ServeHTTP(rec, newSecurityRequest(http.MethodGet, path, ""))
				if rec.Code == http.StatusOK {
					t.Errorf("unsafe path accepted: %s", path)
				}
			}
			if !withProjects {
				if _, err := os.Stat(filepath.Join(root, "projects")); !os.IsNotExist(err) {
					t.Fatalf("catalog access must not create root/projects: %v", err)
				}
			}
		})
	}
}

func TestProjectEngineFromLock(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "projects", "local")
	mustWriteTestFile(t, filepath.Join(project, "facet.lock.json"), `{"engine":"opencode"}`)
	list, err := ListProjects(root)
	if err != nil || len(list) != 1 || list[0].Engine != "opencode" {
		t.Fatalf("local engine summary = %#v, %v", list, err)
	}
	details, err := GetProjectDetails(root, "local")
	if err != nil || details.Engine != "opencode" {
		t.Fatalf("local engine details = %#v, %v", details, err)
	}
	if _, err := RegisterOrUpdateProject("External Lock", project, "", nil, root); err != nil {
		t.Fatal(err)
	}
	details, err = GetProjectDetails(root, "external-lock")
	if err != nil || details.Engine != "opencode" {
		t.Fatalf("catalog lock fallback = %#v, %v", details, err)
	}
}

func TestCatalogProjectRejectsEscapedSymlinks(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	outside := t.TempDir()
	mustWriteTestFile(t, filepath.Join(outside, "secret.txt"), "outside")
	if !makeTestSymlink(t, outside, filepath.Join(project, "escaped")) {
		return
	}
	if _, err := RegisterOrUpdateProject("External", project, "codex", nil, root); err != nil {
		t.Fatal(err)
	}
	server := NewServer(root)
	if _, err := server.resolveSessionDir(filepath.Join(project, "escaped")); err == nil {
		t.Fatal("escaped catalog session directory accepted")
	}
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, newSecurityRequest(http.MethodGet, "/api/media/catalog/external/escaped/secret.txt", ""))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("escaped media returned %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCatalogMediaDistinguishesMatchingFolderNames(t *testing.T) {
	root := t.TempDir()
	server := NewServer(root)
	for _, name := range []string{"first", "second"} {
		project := filepath.Join(t.TempDir(), "same-folder")
		mustWriteTestFile(t, filepath.Join(project, "renders", "edit.mp4"), name)
		if _, err := RegisterOrUpdateProject(name, project, "codex", nil, root); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"first", "second"} {
		details, err := GetProjectDetails(root, name)
		if err != nil {
			t.Fatal(err)
		}
		if details.Stages.Master || details.VideoURL != "" || details.PreviewVideoURL == "" {
			t.Fatalf("preview promoted to master: %#v", details)
		}
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, newSecurityRequest(http.MethodGet, details.PreviewVideoURL, ""))
		if rec.Code != http.StatusOK || rec.Body.String() != name {
			t.Fatalf("preview for %s = %d %q", name, rec.Code, rec.Body.String())
		}
	}
}
