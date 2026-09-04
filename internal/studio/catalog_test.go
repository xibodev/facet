package studio

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCatalogOperations(t *testing.T) {
	tmpDir := t.TempDir()
	origLocalAppData := os.Getenv("LOCALAPPDATA")
	defer os.Setenv("LOCALAPPDATA", origLocalAppData)
	_ = os.Setenv("LOCALAPPDATA", tmpDir)

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

func TestCatalogEndpoints(t *testing.T) {
	tmpDir := t.TempDir()
	origLocalAppData := os.Getenv("LOCALAPPDATA")
	defer os.Setenv("LOCALAPPDATA", origLocalAppData)
	_ = os.Setenv("LOCALAPPDATA", tmpDir)

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
