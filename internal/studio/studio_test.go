package studio

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestListProjectsRealWorkspace(t *testing.T) {
	// Root dir is repository root (which contains projects/)
	rootDir := filepath.Join("..", "..")
	projects, err := ListProjects(rootDir)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}

	if len(projects) < 3 {
		t.Fatalf("expected at least 3 projects, got %d", len(projects))
	}

	slugs := make(map[string]ProjectSummary)
	for _, p := range projects {
		slugs[p.Slug] = p
	}

	for _, expectedSlug := range []string{"cinematic-documentary", "finetuning-explainer", "phase2-source-edit"} {
		p, ok := slugs[expectedSlug]
		if !ok {
			t.Fatalf("missing expected project slug: %s", expectedSlug)
		}
		if p.Name == "" {
			t.Fatalf("empty project name for %s", expectedSlug)
		}
	}

	// Check cinematic-documentary stages
	cine := slugs["cinematic-documentary"]
	if !cine.Stages.Brief || !cine.Stages.Script || !cine.Stages.Voiceover || !cine.Stages.Review || !cine.Stages.Master {
		t.Fatalf("unexpected stages for cinematic-documentary: %#v", cine.Stages)
	}
	if cine.VideoURL == "" {
		t.Fatalf("expected video URL for cinematic-documentary")
	}
	if cine.ThumbnailURL == "" {
		t.Fatalf("expected thumbnail URL for cinematic-documentary")
	}
}

func TestGetProjectDetails(t *testing.T) {
	rootDir := filepath.Join("..", "..")

	// 1. Test cinematic-documentary
	cine, err := GetProjectDetails(rootDir, "cinematic-documentary")
	if err != nil {
		t.Fatalf("GetProjectDetails(cinematic-documentary) failed: %v", err)
	}
	if cine.Brief == "" {
		t.Fatal("expected brief to be populated")
	}
	if cine.Script == "" {
		t.Fatal("expected script to be populated")
	}
	if len(cine.Beats) == 0 {
		t.Fatal("expected beats to be parsed from script")
	}
	if len(cine.ReviewFrames) == 0 {
		t.Fatal("expected review frames")
	}
	if cine.ReviewReport == nil {
		t.Fatal("expected review report to be loaded")
	}
	if !cine.Stages.Master || cine.VideoPath == "" {
		t.Fatalf("expected master video stage: %#v, videoPath=%s", cine.Stages, cine.VideoPath)
	}

	// 2. Test finetuning-explainer
	fine, err := GetProjectDetails(rootDir, "finetuning-explainer")
	if err != nil {
		t.Fatalf("GetProjectDetails(finetuning-explainer) failed: %v", err)
	}
	if len(fine.Beats) == 0 {
		t.Fatal("expected beats parsed for finetuning-explainer")
	}
	if len(fine.Narration) == 0 {
		t.Fatal("expected narration audio files")
	}
	if len(fine.QAFrames) == 0 {
		t.Fatal("expected QA frame captures")
	}
	if fine.RemotionProps == nil {
		t.Fatal("expected remotion props loaded")
	}
}

func TestMarkdownTableParser(t *testing.T) {
	sampleScript := `# Sample Script

| # | t | Beat | Narration |
|---|---|---|---|
| 1 | 0.0-5.0 | Intro | Hello world. |
| 2 | 5.0-10.0 | Outro | Goodbye world. |
`
	beats := parseMarkdownTableBeats(sampleScript)
	if len(beats) != 2 {
		t.Fatalf("expected 2 beats, got %d", len(beats))
	}
	if beats[0].Index != "1" || beats[0].TimeRange != "0.0-5.0" || beats[0].Title != "Intro" || beats[0].Narration != "Hello world." {
		t.Fatalf("unexpected beat 0: %#v", beats[0])
	}
	if beats[1].Index != "2" || beats[1].TimeRange != "5.0-10.0" || beats[1].Title != "Outro" || beats[1].Narration != "Goodbye world." {
		t.Fatalf("unexpected beat 1: %#v", beats[1])
	}
}

func TestServerEndpoints(t *testing.T) {
	rootDir := filepath.Join("..", "..")
	server := NewServer(rootDir)
	handler := server.Handler()

	// 1. GET /
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / returned %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Video Kit Studio") {
		t.Fatalf("GET / body missing Video Kit Studio title")
	}

	// 2. GET /non-existent
	req = httptest.NewRequest("GET", "/non-existent", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /non-existent returned %d, want 404", rec.Code)
	}

	// 3. GET /api/projects
	req = httptest.NewRequest("GET", "/api/projects", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/projects returned %d", rec.Code)
	}
	var projList []ProjectSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &projList); err != nil {
		t.Fatalf("failed to decode projects JSON: %v", err)
	}
	if len(projList) < 3 {
		t.Fatalf("expected >= 3 projects, got %d", len(projList))
	}

	// 4. GET /api/projects/cinematic-documentary
	req = httptest.NewRequest("GET", "/api/projects/cinematic-documentary", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/projects/cinematic-documentary returned %d", rec.Code)
	}
	var details ProjectDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &details); err != nil {
		t.Fatalf("failed to decode project details JSON: %v", err)
	}
	if details.Slug != "cinematic-documentary" {
		t.Fatalf("unexpected slug: %s", details.Slug)
	}

	// 5. GET /api/projects/non-existent
	req = httptest.NewRequest("GET", "/api/projects/non-existent-project", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing project, got %d", rec.Code)
	}

	// 6. GET /api/media/...
	req = httptest.NewRequest("GET", "/api/media/projects/cinematic-documentary/brief.md", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/media/projects/cinematic-documentary/brief.md returned %d", rec.Code)
	}

	// 7. Range request on media file
	req = httptest.NewRequest("GET", "/api/media/projects/cinematic-documentary/brief.md", nil)
	req.Header.Set("Range", "bytes=0-10")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent && rec.Code != http.StatusOK {
		t.Fatalf("Range request returned %d", rec.Code)
	}

	// 8. Prevent directory traversal
	req = httptest.NewRequest("GET", "/api/media/%2e%2e/%2e%2e/go.mod", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
		t.Fatalf("Directory traversal attempt returned %d, want 403 or 404", rec.Code)
	}

	// 9. GET /api/config
	req = httptest.NewRequest("GET", "/api/config", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/config returned %d", rec.Code)
	}

	// 10. POST /api/config
	req = httptest.NewRequest("POST", "/api/config", strings.NewReader(`{"PEXELS_API_KEY":"12345678"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/config returned %d", rec.Code)
	}

	// 11. POST /api/close (non-existent session)
	req = httptest.NewRequest("POST", "/api/close?session=non-existent", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("POST /api/close non-existent returned %d, want 404", rec.Code)
	}

	// 12. GET /api/chat (missing prompt)
	req = httptest.NewRequest("GET", "/api/chat", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET /api/chat without prompt returned %d, want 400", rec.Code)
	}
}

func TestSessionLifecycleAndClose(t *testing.T) {
	rootDir := filepath.Join("..", "..")
	server := NewServer(rootDir)

	if sess := server.getSession("non-existent"); sess != nil {
		t.Fatalf("expected nil for non-existent session, got %#v", sess)
	}

	// Create a mock session attached to server
	sessID := "test-session-123"
	mockSess := &Session{
		ID:     sessID,
		Dir:    server.rootDir,
		Mode:   "rw",
		Engine: "mock",
		events: make(chan map[string]any, 16),
		alive:  false,
	}

	server.sessionsMu.Lock()
	server.sessions[sessID] = mockSess
	server.sessionsMu.Unlock()

	retrieved := server.getSession(sessID)
	if retrieved != mockSess {
		t.Fatalf("expected retrieved session to match mockSess")
	}

	// send on non-alive session should error
	if err := mockSess.send("hello"); err == nil {
		t.Fatal("expected error sending to dead session")
	}

	// Test Close
	if err := mockSess.Close(); err != nil {
		t.Fatalf("Session.Close failed: %v", err)
	}

	// Test handleCloseSession via HTTP POST
	req := httptest.NewRequest("POST", "/api/close?session="+sessID, nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/close returned %d, want 200", rec.Code)
	}

	var res map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode close response: %v", err)
	}
	if res["ok"] != true {
		t.Fatalf("expected ok: true, got %#v", res)
	}
}

func TestChatSSEWithMockSession(t *testing.T) {
	rootDir := filepath.Join("..", "..")
	server := NewServer(rootDir)

	sessID := "test-sse-sess"
	eventCh := make(chan map[string]any, 16)
	pr, pw := io.Pipe()
	go func() {
		_, _ = io.Copy(io.Discard, pr)
	}()

	mockSess := &Session{
		ID:     sessID,
		Dir:    server.rootDir,
		Mode:   "rw",
		Engine: "mock",
		stdin:  pw,
		events: eventCh,
		alive:  true,
	}
	defer mockSess.Close()

	server.sessionsMu.Lock()
	server.sessions[sessID] = mockSess
	server.sessionsMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := httptest.NewRequest("GET", "/api/chat?prompt=test&session="+sessID, nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		server.Handler().ServeHTTP(rec, req)
		close(done)
	}()

	// Push events into eventCh
	eventCh <- map[string]any{
		"type":       "system",
		"subtype":    "init",
		"session_id": "claude-mock-id",
	}
	eventCh <- map[string]any{
		"type": "result",
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		cancel()
		<-done
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/chat returned %d, want 200", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "event: session") {
		t.Fatalf("expected SSE output to contain event: session, got: %s", body)
	}
	if !strings.Contains(body, "claude-mock-id") {
		t.Fatalf("expected SSE output to contain claude-mock-id, got: %s", body)
	}
	if !strings.Contains(body, "event: end") {
		t.Fatalf("expected SSE output to contain event: end, got: %s", body)
	}
}

func TestMaskSecret(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"short", ""},
		{"1234567", ""},
		{"12345678", "1234...5678"},
		{"sk-proj-abcdef123456", "sk-p...3456"},
	}

	for _, tc := range cases {
		got := maskSecret(tc.input)
		if got != tc.want {
			t.Errorf("maskSecret(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSlugToTitle(t *testing.T) {
	if got := slugToTitle("cinematic-documentary"); got != "Cinematic Documentary" {
		t.Errorf("slugToTitle failed: got %q", got)
	}
	if got := slugToTitle("phase2-source-edit"); got != "Phase2 Source Edit" {
		t.Errorf("slugToTitle failed: got %q", got)
	}
}

func TestExtractBeatsFromRemotionProps(t *testing.T) {
	rawJSON := `{
		"cuts": [
			{
				"type": "intro",
				"in_seconds": 0.0,
				"out_seconds": 3.5,
				"title": "Welcome",
				"text": "Hello all"
			},
			{
				"type": "scene",
				"in_seconds": 3.5,
				"out_seconds": 8.0,
				"text": "Main content"
			}
		]
	}`
	var props any
	if err := json.Unmarshal([]byte(rawJSON), &props); err != nil {
		t.Fatalf("unmarshal props failed: %v", err)
	}

	beats := extractBeatsFromRemotionProps(props)
	if len(beats) != 2 {
		t.Fatalf("expected 2 beats, got %d", len(beats))
	}
	if beats[0].Title != "Welcome" || beats[0].Type != "intro" {
		t.Errorf("unexpected beat 0: %#v", beats[0])
	}
	if beats[1].Title != "Main content" || beats[1].Type != "scene" {
		t.Errorf("unexpected beat 1: %#v", beats[1])
	}
}
