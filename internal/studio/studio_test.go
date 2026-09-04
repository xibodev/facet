package studio

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xibodev/facet/internal/studio/engine"
)

type studioHelperAdapter struct {
	executable string
	scenario   string
	mu         sync.Mutex
	builds     [][]string
}

func (a *studioHelperAdapter) Name() string           { return "opencode" }
func (a *studioHelperAdapter) DisplayName() string    { return "Studio test helper" }
func (a *studioHelperAdapter) ExecutableName() string { return a.executable }

func (a *studioHelperAdapter) BuildTurnArgs(dir, mode, prompt, nativeID string, extraArgs []string) ([]string, error) {
	a.mu.Lock()
	a.builds = append(a.builds, []string{dir, mode, prompt, nativeID})
	a.mu.Unlock()
	return []string{"-test.run=^TestStudioHelperProcess$", "--", a.scenario, nativeID, prompt}, nil
}

func (a *studioHelperAdapter) NormalizeEvent(line []byte) (*engine.NormalizedEvent, error) {
	return engine.NewOpenCodeAdapter().NormalizeEvent(line)
}

func (a *studioHelperAdapter) buildHistory() [][]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([][]string, len(a.builds))
	for i := range a.builds {
		result[i] = append([]string(nil), a.builds[i]...)
	}
	return result
}

func newStudioHelperAdapter(t *testing.T, scenario string) *studioHelperAdapter {
	t.Helper()
	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	return &studioHelperAdapter{executable: executable, scenario: scenario}
}

func newStudioHelperSession(t *testing.T, adapter engine.EngineAdapter) *Session {
	t.Helper()
	sess := &Session{
		ID:       "helper-session",
		Dir:      t.TempDir(),
		Mode:     "rw",
		Engine:   adapter.Name(),
		valid:    true,
		turnGate: make(chan struct{}, 1),
		adapter:  adapter,
	}
	sess.turnGate <- struct{}{}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

func TestStudioHelperProcess(t *testing.T) {
	separator := -1
	for i, arg := range os.Args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if separator < 0 || len(os.Args) < separator+4 {
		return
	}

	scenario := os.Args[separator+1]
	nativeID := os.Args[separator+2]
	prompt := os.Args[separator+3]
	if nativeID == "" {
		nativeID = "helper-native-id"
	}
	encode := json.NewEncoder(os.Stdout)
	emit := func(value map[string]any) { _ = encode.Encode(value) }
	emit(map[string]any{"type": "step_start", "sessionID": nativeID, "part": map[string]any{"type": "step-start", "sessionID": nativeID}})

	switch scenario {
	case "success":
		emit(map[string]any{"type": "text", "sessionID": nativeID, "part": map[string]any{"type": "text", "text": "completed " + prompt}})
		emit(map[string]any{"type": "step_finish", "sessionID": nativeID, "part": map[string]any{"type": "step-finish", "reason": "stop"}})
	case "stream":
		for {
			emit(map[string]any{"type": "text", "sessionID": nativeID, "part": map[string]any{"type": "text", "text": "streaming"}})
			time.Sleep(10 * time.Millisecond)
		}
	case "useful":
		emit(map[string]any{"type": "text", "sessionID": nativeID, "part": map[string]any{"type": "text", "text": "useful output"}})
	case "error":
		emit(map[string]any{"type": "error", "sessionID": nativeID, "message": "helper failed"})
		emit(map[string]any{"type": "step_finish", "sessionID": nativeID, "part": map[string]any{"type": "step-finish"}})
	case "failed_done":
		emit(map[string]any{"type": "step_finish", "sessionID": nativeID, "isError": "true", "result": "terminal failed"})
	case "exit":
		os.Exit(3)
	case "sleep":
		if prompt != "" {
			_ = os.WriteFile(prompt, []byte(strconv.Itoa(os.Getpid())), 0o600)
		}
		time.Sleep(30 * time.Second)
	case "oversized":
		chunk := make([]byte, 64*1024)
		for i := range chunk {
			chunk[i] = 'x'
		}
		for written := 0; written <= 32*1024*1024; written += len(chunk) {
			if _, err := os.Stdout.Write(chunk); err != nil {
				os.Exit(0)
			}
		}
		time.Sleep(30 * time.Second)
	case "descendant", "descendant-cancel":
		child := exec.Command(os.Args[0], "-test.run=^TestStudioHelperProcess$", "--", "descendant-child", nativeID, prompt)
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(4)
		}
		ready := filepath.Join(prompt, "descendant.ready")
		deadline := time.Now().Add(5 * time.Second)
		for {
			if _, err := os.Stat(ready); err == nil {
				break
			}
			if time.Now().After(deadline) {
				os.Exit(6)
			}
			time.Sleep(10 * time.Millisecond)
		}
		if scenario == "descendant-cancel" {
			keepalive := filepath.Join(prompt, "keepalive")
			for {
				if _, err := os.Stat(keepalive); err != nil {
					os.Exit(0)
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
		emit(map[string]any{"type": "text", "sessionID": nativeID, "part": map[string]any{"type": "text", "text": "root exited with descendant running"}})
		os.Exit(0)
	case "descendant-child":
		pidFile := filepath.Join(prompt, "descendant.pid")
		heartbeat := filepath.Join(prompt, "descendant.heartbeat")
		ready := filepath.Join(prompt, "descendant.ready")
		keepalive := filepath.Join(prompt, "keepalive")
		if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			os.Exit(5)
		}
		if err := os.WriteFile(heartbeat, []byte("0"), 0o600); err != nil {
			os.Exit(5)
		}
		if err := os.WriteFile(ready, []byte("ready"), 0o600); err != nil {
			os.Exit(5)
		}
		for tick := 1; ; tick++ {
			if _, err := os.Stat(keepalive); err != nil {
				os.Exit(0)
			}
			if err := os.WriteFile(heartbeat, []byte(strconv.Itoa(tick)), 0o600); err != nil {
				os.Exit(5)
			}
			time.Sleep(10 * time.Millisecond)
		}
	case "empty":
		os.Exit(0)
	default:
		os.Exit(2)
	}
	os.Exit(0)
}

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

func TestListProjectsMissingProjectsRoot(t *testing.T) {
	root := t.TempDir()

	projects, err := ListProjects(root)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if projects == nil || len(projects) != 0 {
		t.Fatalf("ListProjects = %#v, want non-nil empty slice", projects)
	}
	if _, err := GetProjectDetails(root, "missing-project"); err == nil {
		t.Fatal("GetProjectDetails succeeded without a projects directory")
	}
}

func TestListProjectsEmptyProjectsRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}

	projects, err := ListProjects(root)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if projects == nil || len(projects) != 0 {
		t.Fatalf("ListProjects = %#v, want non-nil empty slice", projects)
	}

	server := NewServer(root)
	rec := serveSecurityRequest(server, http.MethodGet, "/api/projects", "")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("GET /api/projects returned %d %q, want 200 []", rec.Code, strings.TrimSpace(rec.Body.String()))
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
	if cine.VideoVersion == "" {
		t.Fatal("expected a master video version")
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
	if fine.CompositionPath != "projects/finetuning-explainer/remotion_props.json" || fine.CompositionURL != "/api/media/projects/finetuning-explainer/remotion_props.json" {
		t.Fatalf("unexpected composition location: path=%q url=%q", fine.CompositionPath, fine.CompositionURL)
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
	rec := newDeadlineResponseRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / returned %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Video Kit Studio") {
		t.Fatalf("GET / body missing Video Kit Studio title")
	}

	// 2. GET /non-existent
	req = httptest.NewRequest("GET", "/non-existent", nil)
	rec = newDeadlineResponseRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /non-existent returned %d, want 404", rec.Code)
	}

	// 3. GET /api/projects
	req = newSecurityRequest(http.MethodGet, "/api/projects", "")
	rec = newDeadlineResponseRecorder()
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

	// 4. GET /api/engines
	req = newSecurityRequest(http.MethodGet, "/api/engines", "")
	rec = newDeadlineResponseRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"display_name"`) {
		t.Fatalf("GET /api/engines returned %d: %s", rec.Code, rec.Body.String())
	}

	// 5. GET /api/projects/cinematic-documentary
	req = newSecurityRequest(http.MethodGet, "/api/projects/cinematic-documentary", "")
	rec = newDeadlineResponseRecorder()
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

	// 6. GET /api/projects/non-existent
	req = newSecurityRequest(http.MethodGet, "/api/projects/non-existent-project", "")
	rec = newDeadlineResponseRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing project, got %d", rec.Code)
	}

	// 7. GET /api/media/...
	req = newSecurityRequest(http.MethodGet, "/api/media/projects/cinematic-documentary/brief.md", "")
	rec = newDeadlineResponseRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/media/projects/cinematic-documentary/brief.md returned %d", rec.Code)
	}

	// 8. Range request on media file
	req = newSecurityRequest(http.MethodGet, "/api/media/projects/cinematic-documentary/brief.md", "")
	req.Header.Set("Range", "bytes=0-10")
	rec = newDeadlineResponseRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent && rec.Code != http.StatusOK {
		t.Fatalf("Range request returned %d", rec.Code)
	}

	// 9. Prevent directory traversal
	req = newSecurityRequest(http.MethodGet, "/api/media/%2e%2e/%2e%2e/go.mod", "")
	rec = newDeadlineResponseRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
		t.Fatalf("Directory traversal attempt returned %d, want 403 or 404", rec.Code)
	}

	// 10. GET /api/config
	req = newSecurityRequest(http.MethodGet, "/api/config", "")
	rec = newDeadlineResponseRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/config returned %d", rec.Code)
	}

	// 11. POST /api/config
	req = newSecurityRequest(http.MethodPost, "/api/config", `{"PEXELS_API_KEY":"12345678"}`)
	req.Header.Set(sessionTokenHeader, server.sessionToken)
	rec = newDeadlineResponseRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/config returned %d", rec.Code)
	}

	// 12. POST /api/close (non-existent session)
	req = newSecurityRequest(http.MethodPost, "/api/close?session=non-existent", "")
	req.Header.Set(sessionTokenHeader, server.sessionToken)
	rec = newDeadlineResponseRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("POST /api/close non-existent returned %d, want 404", rec.Code)
	}

	// 13. GET /api/chat (missing prompt)
	req = newSecurityRequest(http.MethodGet, "/api/chat", "")
	query := req.URL.Query()
	query.Set(sessionTokenQuery, server.sessionToken)
	req.URL.RawQuery = query.Encode()
	rec = newDeadlineResponseRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET /api/chat without prompt returned %d, want 400", rec.Code)
	}
}

func TestZeroByteVideoIsNotServed(t *testing.T) {
	root := t.TempDir()
	mustWriteTestFile(t, filepath.Join(root, "projects", "empty-video", "renders", "final.mp4"), "")
	server := NewServer(root)
	req := newSecurityRequest(http.MethodGet, "/api/media/projects/empty-video/renders/final.mp4", "")
	rec := newDeadlineResponseRecorder()

	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("zero-byte video returned %d, want 404", rec.Code)
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
		ID:       sessID,
		Dir:      server.rootDir,
		Mode:     "rw",
		Engine:   "mock",
		valid:    true,
		turnGate: make(chan struct{}, 1),
	}
	mockSess.turnGate <- struct{}{}

	server.sessionsMu.Lock()
	server.sessions[sessID] = mockSess
	server.sessionsMu.Unlock()

	retrieved := server.getSession(sessID)
	if retrieved != mockSess {
		t.Fatalf("expected retrieved session to match mockSess")
	}

	// Test Close
	if err := mockSess.Close(); err != nil {
		t.Fatalf("Session.Close failed: %v", err)
	}

	// Test handleCloseSession via HTTP POST
	req := newSecurityRequest(http.MethodPost, "/api/close?session="+sessID, "")
	req.Header.Set(sessionTokenHeader, server.sessionToken)
	rec := newDeadlineResponseRecorder()
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
	if sess := server.getSession(sessID); sess != nil {
		t.Fatalf("closed session was not evicted: %#v", sess)
	}
}

func TestUnknownEngineRejected(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	server := NewServer(root)
	req := newSecurityRequest(http.MethodGet, "/api/chat?prompt=test&engine=cmd.exe", "")
	query := req.URL.Query()
	query.Set(sessionTokenQuery, server.sessionToken)
	req.URL.RawQuery = query.Encode()
	rec := newDeadlineResponseRecorder()

	server.Handler().ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "unknown engine") || !strings.Contains(body, `"ok":false`) {
		t.Fatalf("unknown engine was not rejected cleanly: %s", body)
	}
	if len(server.sessions) != 0 {
		t.Fatalf("unknown engine created a session: %#v", server.sessions)
	}
}

func TestOnlyCanonicalEnginesAreRegistered(t *testing.T) {
	for _, name := range []string{"", "claude", "opencode", "codex", "copilot"} {
		if _, ok := registeredAdapter(name); !ok {
			t.Errorf("registeredAdapter(%q) rejected a canonical engine", name)
		}
	}
	for _, name := range []string{"claude-code", "gh-copilot", "openai-codex", "generic", "cmd.exe"} {
		if _, ok := registeredAdapter(name); ok {
			t.Errorf("registeredAdapter(%q) accepted a non-canonical engine", name)
		}
	}
}

func TestDeadRequestedSessionIsRejected(t *testing.T) {
	server := NewServer(t.TempDir())
	sess := &Session{ID: "dead-session", Engine: "claude", turnGate: make(chan struct{}, 1)}
	server.sessions[sess.ID] = sess

	req := newSecurityRequest(http.MethodGet, "/api/chat?prompt=test&session="+sess.ID, "")
	query := req.URL.Query()
	query.Set(sessionTokenQuery, server.sessionToken)
	req.URL.RawQuery = query.Encode()
	rec := newDeadlineResponseRecorder()
	server.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "is not resumable") || !strings.Contains(body, `"ok":false`) {
		t.Fatalf("dead requested session was not rejected: %s", body)
	}
	if server.getSession(sess.ID) != nil {
		t.Fatal("dead requested session was not evicted")
	}
}

func TestOneShotTurnReapsAndResumesByNativeID(t *testing.T) {
	adapter := newStudioHelperAdapter(t, "success")
	sess := newStudioHelperSession(t, adapter)

	var first []turnEvent
	result := sess.runTurn(context.Background(), "first prompt", func(event turnEvent) error {
		first = append(first, event)
		return nil
	})
	if !result.ok || !sess.IsAlive() || sess.isRunning() {
		t.Fatalf("first turn = %#v, alive=%v running=%v", result, sess.IsAlive(), sess.isRunning())
	}
	if sess.NativeID != "helper-native-id" || len(first) != 3 {
		t.Fatalf("first turn native/events = %q/%#v", sess.NativeID, first)
	}
	if first[1].payload["type"] != engine.EventTextDelta || first[1].payload["content"] != "completed first prompt" {
		t.Fatalf("non-Claude event was not normalized: %#v", first[1].payload)
	}

	result = sess.runTurn(context.Background(), "second prompt", func(turnEvent) error { return nil })
	if !result.ok || !sess.IsAlive() || sess.isRunning() {
		t.Fatalf("resume turn = %#v, alive=%v running=%v", result, sess.IsAlive(), sess.isRunning())
	}
	builds := adapter.buildHistory()
	if len(builds) != 2 || builds[0][2] != "first prompt" || builds[0][3] != "" || builds[1][2] != "second prompt" || builds[1][3] != "helper-native-id" {
		t.Fatalf("turn build history = %#v", builds)
	}
}

func TestTailBufferIsBounded(t *testing.T) {
	buffer := &tailBuffer{limit: 8}
	_, _ = buffer.Write([]byte("012345"))
	_, _ = buffer.Write([]byte("6789"))
	if got := buffer.String(); got != "23456789" {
		t.Fatalf("tail = %q, want %q", got, "23456789")
	}
	_, _ = buffer.Write([]byte("abcdefghijk"))
	if got := buffer.String(); got != "defghijk" {
		t.Fatalf("oversized tail = %q, want %q", got, "defghijk")
	}
}

func TestEmitterErrorCancelsAndReapsProcess(t *testing.T) {
	adapter := newStudioHelperAdapter(t, "stream")
	sess := newStudioHelperSession(t, adapter)
	emitErr := errors.New("transport closed")

	started := time.Now()
	result := sess.runTurn(context.Background(), "stream", func(turnEvent) error {
		return emitErr
	})
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("emitter failure took %s to cancel and reap the process", elapsed)
	}
	if result.ok || !result.abnormal || !result.canceled || !strings.Contains(result.reason, emitErr.Error()) {
		t.Fatalf("emitter failure result = %#v", result)
	}
	if !result.emitFailed {
		t.Fatalf("emitter failure was not identified as a transport failure: %#v", result)
	}
	if sess.IsAlive() || sess.isRunning() {
		t.Fatalf("emitter-failed session alive=%v running=%v", sess.IsAlive(), sess.isRunning())
	}

	closeStarted := time.Now()
	if err := sess.Close(); err != nil {
		t.Fatalf("Close after emitter failure: %v", err)
	}
	if elapsed := time.Since(closeStarted); elapsed > time.Second {
		t.Fatalf("Close after emitter failure took %s", elapsed)
	}
}

func TestChatSSEOneShotKeepsConversationAlive(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "projects", "demo")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	server := NewServer(root)
	adapter := newStudioHelperAdapter(t, "success")
	sess := newStudioHelperSession(t, adapter)
	sess.ID = "sse-helper"
	sess.Dir = project
	server.sessions[sess.ID] = sess

	req := newSecurityRequest(http.MethodGet, "/api/chat?prompt=hello&session="+sess.ID, "")
	query := req.URL.Query()
	query.Set(sessionTokenQuery, server.sessionToken)
	req.URL.RawQuery = query.Encode()
	rec := newDeadlineResponseRecorder()
	server.Handler().ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"type":"text_delta"`) || !strings.Contains(body, `"ok":true`) || !strings.Contains(body, `"alive":true`) {
		t.Fatalf("unexpected SSE response: status=%d body=%s", rec.Code, body)
	}
	if strings.Contains(body, "event: process_exit") || !sess.IsAlive() || sess.isRunning() || server.getSession(sess.ID) != sess {
		t.Fatalf("successful one-shot did not remain resumable/reaped: %s", body)
	}
}

func TestEndPayloadAlwaysIncludesAlive(t *testing.T) {
	rec := newDeadlineResponseRecorder()
	if err := sendEnd(rec, false, false, "failed"); err != nil {
		t.Fatal(err)
	}
	body := rec.Body.String()
	if strings.Count(body, "event: end\n") != 1 || !strings.Contains(body, `"alive":false`) || !strings.Contains(body, `"ok":false`) {
		t.Fatalf("end payload = %s", body)
	}
}

type blockingDeadlineResponseWriter struct {
	header          http.Header
	blockedWrite    chan struct{}
	writeDeadline   chan struct{}
	blockedWriteOne sync.Once
	mu              sync.Mutex
	status          int
	writeCalls      int
	deadline        time.Time
}

func newBlockingDeadlineResponseWriter() *blockingDeadlineResponseWriter {
	return &blockingDeadlineResponseWriter{
		header:        make(http.Header),
		blockedWrite:  make(chan struct{}),
		writeDeadline: make(chan struct{}, 1),
	}
}

func (w *blockingDeadlineResponseWriter) Header() http.Header {
	return w.header
}

func (w *blockingDeadlineResponseWriter) WriteHeader(status int) {
	w.mu.Lock()
	w.status = status
	w.mu.Unlock()
}

func (w *blockingDeadlineResponseWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	w.writeCalls++
	call := w.writeCalls
	w.mu.Unlock()
	if call == 1 {
		return len(p), nil
	}
	w.blockedWriteOne.Do(func() { close(w.blockedWrite) })
	<-w.writeDeadline
	w.mu.Lock()
	deadline := w.deadline
	w.mu.Unlock()
	delay := time.Until(deadline)
	if delay > 0 {
		timer := time.NewTimer(delay)
		<-timer.C
	}
	return 0, os.ErrDeadlineExceeded
}

func (*blockingDeadlineResponseWriter) Flush() {}

func (w *blockingDeadlineResponseWriter) SetWriteDeadline(deadline time.Time) error {
	w.mu.Lock()
	w.deadline = deadline
	w.mu.Unlock()
	if deadline.IsZero() {
		return nil
	}
	select {
	case w.writeDeadline <- struct{}{}:
	default:
	}
	return nil
}

func (w *blockingDeadlineResponseWriter) calls() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writeCalls
}

func TestChatSSEWriteDeadlineCancelsAndReapsProcess(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "projects", "demo")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	server := NewServer(root)
	sess := newStudioHelperSession(t, newStudioHelperAdapter(t, "stream"))
	sess.ID = "blocked-sse"
	sess.Dir = project
	server.sessions[sess.ID] = sess

	req := newSecurityRequest(http.MethodGet, "/api/chat?prompt=stream&session="+sess.ID, "")
	query := req.URL.Query()
	query.Set(sessionTokenQuery, server.sessionToken)
	req.URL.RawQuery = query.Encode()
	w := newBlockingDeadlineResponseWriter()
	done := make(chan struct{})
	started := time.Now()
	go func() {
		server.Handler().ServeHTTP(w, req)
		close(done)
	}()

	select {
	case <-w.blockedWrite:
	case <-time.After(time.Second):
		t.Fatal("streaming SSE write did not start")
	}
	closeStarted := time.Now()
	if err := sess.Close(); err != nil {
		t.Fatalf("Close during blocked SSE: %v", err)
	}
	if elapsed := time.Since(closeStarted); elapsed > sseWriteTimeout+3*time.Second {
		t.Fatalf("Close during blocked SSE took %s", elapsed)
	}
	select {
	case <-done:
	case <-time.After(sseWriteTimeout + 5*time.Second):
		t.Fatal("deadline-aware blocked SSE did not return")
	}
	if elapsed := time.Since(started); elapsed > sseWriteTimeout+3*time.Second {
		t.Fatalf("blocked SSE handler took %s", elapsed)
	}
	if w.calls() != 2 {
		t.Fatalf("blocked transport received %d write attempts, want initial session plus one stream event", w.calls())
	}
	if sess.IsAlive() || sess.isRunning() || server.getSession(sess.ID) != nil {
		t.Fatalf("blocked SSE session alive=%v running=%v registered=%v", sess.IsAlive(), sess.isRunning(), server.getSession(sess.ID) != nil)
	}
}

func TestEngineErrorCannotBecomeSuccess(t *testing.T) {
	adapter := newStudioHelperAdapter(t, "error")
	sess := newStudioHelperSession(t, adapter)
	var types []string
	result := sess.runTurn(context.Background(), "fail", func(event turnEvent) error {
		types = append(types, event.normalized.Type)
		return nil
	})
	if result.ok || result.abnormal || result.reason != "helper failed" || sess.IsAlive() || sess.isRunning() {
		t.Fatalf("failed turn = %#v alive=%v running=%v", result, sess.IsAlive(), sess.isRunning())
	}
	if !reflect.DeepEqual(types, []string{engine.EventSession, engine.EventError}) {
		t.Fatalf("events after top-level error = %#v", types)
	}
}

func TestFailedTurnSSEHasOneEndAndNoSuccess(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "projects", "demo")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	server := NewServer(root)
	sess := newStudioHelperSession(t, newStudioHelperAdapter(t, "error"))
	sess.ID = "failed-sse"
	sess.Dir = project
	server.sessions[sess.ID] = sess

	req := newSecurityRequest(http.MethodGet, "/api/chat?prompt=fail&session="+sess.ID, "")
	query := req.URL.Query()
	query.Set(sessionTokenQuery, server.sessionToken)
	req.URL.RawQuery = query.Encode()
	rec := newDeadlineResponseRecorder()
	server.Handler().ServeHTTP(rec, req)
	body := rec.Body.String()
	if strings.Count(body, "event: end\n") != 1 || !strings.Contains(body, `"ok":false`) || strings.Contains(body, `"ok":true`) {
		t.Fatalf("failed SSE terminal sequence: %s", body)
	}
	if strings.Contains(body, "event: process_exit\n") || !strings.Contains(body, `"alive":false`) || server.getSession(sess.ID) != nil {
		t.Fatalf("failed SSE lifecycle sequence: %s", body)
	}
}

func TestFailedTerminalCannotBecomeSuccess(t *testing.T) {
	adapter := newStudioHelperAdapter(t, "failed_done")
	sess := newStudioHelperSession(t, adapter)
	result := sess.runTurn(context.Background(), "fail", func(turnEvent) error { return nil })
	if result.ok || result.reason != "terminal failed" || sess.IsAlive() || sess.isRunning() {
		t.Fatalf("failed terminal = %#v alive=%v running=%v", result, sess.IsAlive(), sess.isRunning())
	}
}

func TestAbnormalProcessExitEmitsProcessExit(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "projects", "demo")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	server := NewServer(root)
	sess := newStudioHelperSession(t, newStudioHelperAdapter(t, "exit"))
	sess.ID = "exit-sse"
	sess.Dir = project
	server.sessions[sess.ID] = sess

	req := newSecurityRequest(http.MethodGet, "/api/chat?prompt=exit&session="+sess.ID, "")
	query := req.URL.Query()
	query.Set(sessionTokenQuery, server.sessionToken)
	req.URL.RawQuery = query.Encode()
	rec := newDeadlineResponseRecorder()
	server.Handler().ServeHTTP(rec, req)
	body := rec.Body.String()
	if strings.Count(body, "event: process_exit\n") != 1 || strings.Count(body, "event: end\n") != 1 || !strings.Contains(body, `"ok":false`) {
		t.Fatalf("abnormal exit lifecycle: %s", body)
	}
}

func TestProcessCancellationKillsTreeAndInvalidatesSession(t *testing.T) {
	adapter := newStudioHelperAdapter(t, "descendant-cancel")
	sess := newStudioHelperSession(t, adapter)
	dir := t.TempDir()
	keepalive := filepath.Join(dir, "keepalive")
	if err := os.WriteFile(keepalive, []byte("alive"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(keepalive) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan turnResult, 1)
	go func() {
		done <- sess.runTurn(ctx, dir, func(turnEvent) error { return nil })
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(dir, "descendant.ready")); err == nil && sess.isRunning() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper process did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case result := <-done:
		if result.ok || !result.canceled {
			t.Fatalf("canceled turn = %#v", result)
		}
	case <-time.After(5 * time.Second):
		_ = os.Remove(keepalive)
		t.Fatal("canceled process tree was not reaped")
	}
	heartbeat := filepath.Join(dir, "descendant.heartbeat")
	before, err := os.ReadFile(heartbeat)
	if err != nil {
		t.Fatalf("read canceled descendant heartbeat: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	after, err := os.ReadFile(heartbeat)
	if err != nil {
		t.Fatalf("re-read canceled descendant heartbeat: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("descendant remained alive after cancellation: heartbeat advanced from %q to %q", before, after)
	}
	if sess.IsAlive() || sess.isRunning() {
		t.Fatalf("canceled session alive=%v running=%v", sess.IsAlive(), sess.isRunning())
	}
}

func TestImmediateProcessCancellationDoesNotEscapeStart(t *testing.T) {
	adapter := newStudioHelperAdapter(t, "sleep")
	sess := newStudioHelperSession(t, adapter)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan turnResult, 1)
	go func() {
		done <- sess.runTurn(ctx, "immediate", func(turnEvent) error { return nil })
	}()
	cancel()

	select {
	case result := <-done:
		if result.ok || !result.canceled {
			t.Fatalf("immediately canceled turn = %#v", result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("immediate cancellation did not finish")
	}
	if sess.IsAlive() || sess.isRunning() {
		t.Fatalf("immediately canceled session alive=%v running=%v", sess.IsAlive(), sess.isRunning())
	}
}

func TestWindowsProcessTreeTerminationAfterNaturalRootExitIsIdempotent(t *testing.T) {
	if os.PathSeparator != '\\' {
		t.Skip("Windows Job Object behavior")
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestStudioHelperProcess$", "--", "empty", "", "natural-exit")
	tree, err := prepareProcessTree(cmd)
	if err != nil {
		t.Fatalf("prepare process tree: %v", err)
	}
	defer func() {
		if err := tree.close(); err != nil {
			t.Errorf("close process tree: %v", err)
		}
	}()
	if err := cmd.Start(); err != nil {
		t.Fatalf("start process: %v", err)
	}
	if err := tree.afterStart(cmd); err != nil {
		waitErr := cmd.Wait()
		t.Fatalf("finish process tree setup: %v (wait: %v)", err, waitErr)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait for natural root exit: %v", err)
	}
	if err := tree.terminate(); err != nil {
		t.Fatalf("terminate empty process tree: %v", err)
	}
	if err := tree.terminate(); err != nil {
		t.Fatalf("terminate empty process tree again: %v", err)
	}
}

func TestCloseCancelsCurrentProcessAndInvalidatesSession(t *testing.T) {
	adapter := newStudioHelperAdapter(t, "sleep")
	sess := newStudioHelperSession(t, adapter)
	pidFile := filepath.Join(t.TempDir(), "pid")
	done := make(chan turnResult, 1)
	go func() {
		done <- sess.runTurn(context.Background(), pidFile, func(turnEvent) error { return nil })
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(pidFile); err == nil && sess.isRunning() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper process did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := sess.Close(); err != nil {
		t.Fatal(err)
	}
	if sess.isRunning() {
		t.Fatal("Close returned before the current process was reaped")
	}
	select {
	case result := <-done:
		if result.ok || !result.canceled {
			t.Fatalf("closed turn = %#v", result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("closed process was not reaped")
	}
	if sess.IsAlive() || sess.isRunning() {
		t.Fatalf("closed session alive=%v running=%v", sess.IsAlive(), sess.isRunning())
	}
}

func TestOversizedOutputTerminatesLiveProcessBeforeWait(t *testing.T) {
	adapter := newStudioHelperAdapter(t, "oversized")
	sess := newStudioHelperSession(t, adapter)
	sess.maxTokenSize = 64 * 1024
	done := make(chan turnResult, 1)
	go func() {
		done <- sess.runTurn(context.Background(), "oversized", func(turnEvent) error { return nil })
	}()

	select {
	case result := <-done:
		if result.ok || !result.abnormal || result.canceled || !strings.Contains(result.reason, "output stream") || !strings.Contains(result.reason, "token too long") {
			t.Fatalf("oversized output turn = %#v", result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("scanner error did not terminate and reap the live helper")
	}
	if sess.IsAlive() || sess.isRunning() {
		t.Fatalf("oversized-output session alive=%v running=%v", sess.IsAlive(), sess.isRunning())
	}
}

func TestRootExitKillsDescendantHoldingStdout(t *testing.T) {
	adapter := newStudioHelperAdapter(t, "descendant")
	sess := newStudioHelperSession(t, adapter)
	dir := t.TempDir()
	keepalive := filepath.Join(dir, "keepalive")
	if err := os.WriteFile(keepalive, []byte("alive"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(keepalive) })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan turnResult, 1)
	var sawText bool
	go func() {
		done <- sess.runTurn(ctx, dir, func(event turnEvent) error {
			sawText = sawText || event.normalized.Type == engine.EventTextDelta
			return nil
		})
	}()

	var result turnResult
	select {
	case result = <-done:
	case <-time.After(5 * time.Second):
		cancel()
		_ = os.Remove(keepalive)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
		t.Fatal("root exit did not terminate the descendant holding stdout")
	}
	if result.ok || !result.abnormal || result.canceled || !strings.Contains(result.reason, "without a successful terminal event") {
		t.Fatalf("descendant root exit = %#v", result)
	}
	if !sawText {
		t.Fatal("output before missing-terminal failure was not emitted")
	}
	if _, err := os.Stat(filepath.Join(dir, "descendant.pid")); err != nil {
		t.Fatalf("descendant did not report its PID before root exit: %v", err)
	}
	heartbeat := filepath.Join(dir, "descendant.heartbeat")
	before, err := os.ReadFile(heartbeat)
	if err != nil {
		t.Fatalf("read descendant heartbeat: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	after, err := os.ReadFile(heartbeat)
	if err != nil {
		t.Fatalf("re-read descendant heartbeat: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("descendant remained alive after runTurn returned: heartbeat advanced from %q to %q", before, after)
	}
	if sess.IsAlive() || sess.isRunning() {
		t.Fatalf("descendant root-exit session alive=%v running=%v", sess.IsAlive(), sess.isRunning())
	}
}

func TestCanceledQueuedTurnIsNeverBuiltOrStarted(t *testing.T) {
	adapter := newStudioHelperAdapter(t, "success")
	sess := newStudioHelperSession(t, adapter)
	<-sess.turnGate

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan turnResult, 1)
	go func() {
		done <- sess.runTurn(ctx, "queued", func(turnEvent) error { return nil })
	}()
	cancel()
	sess.turnGate <- struct{}{}
	result := <-done
	if !result.canceled || len(adapter.buildHistory()) != 0 || sess.isRunning() || sess.IsAlive() {
		t.Fatalf("queued cancellation = %#v builds=%#v running=%v alive=%v", result, adapter.buildHistory(), sess.isRunning(), sess.IsAlive())
	}
}

func TestCanceledQueuedSSEEvictsSessionAndEndsOnce(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "projects", "demo")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	server := NewServer(root)
	sess := newStudioHelperSession(t, newStudioHelperAdapter(t, "success"))
	sess.ID = "queued-sse"
	sess.Dir = project
	<-sess.turnGate
	server.sessions[sess.ID] = sess

	ctx, cancel := context.WithCancel(context.Background())
	req := newSecurityRequest(http.MethodGet, "/api/chat?prompt=queued&session="+sess.ID, "")
	query := req.URL.Query()
	query.Set(sessionTokenQuery, server.sessionToken)
	req.URL.RawQuery = query.Encode()
	req = req.WithContext(ctx)
	rec := newDeadlineResponseRecorder()
	done := make(chan struct{})
	go func() {
		server.Handler().ServeHTTP(rec, req)
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	<-done
	body := rec.Body.String()
	if strings.Count(body, "event: end\n") != 1 || !strings.Contains(body, `"ok":false`) || !strings.Contains(body, `"alive":false`) || server.getSession(sess.ID) != nil {
		t.Fatalf("canceled queued SSE lifecycle: %s", body)
	}
	sess.turnGate <- struct{}{}
}

func TestSuccessfulExitWithoutOutputFails(t *testing.T) {
	adapter := newStudioHelperAdapter(t, "empty")
	sess := newStudioHelperSession(t, adapter)
	result := sess.runTurn(context.Background(), "empty", func(turnEvent) error { return nil })
	if result.ok || !result.abnormal || !strings.Contains(result.reason, "without a successful terminal event") || sess.IsAlive() || sess.isRunning() {
		t.Fatalf("empty turn = %#v alive=%v running=%v", result, sess.IsAlive(), sess.isRunning())
	}
}

func TestSuccessfulExitWithUsefulOutputWithoutTerminalFails(t *testing.T) {
	adapter := newStudioHelperAdapter(t, "useful")
	sess := newStudioHelperSession(t, adapter)
	var text string
	result := sess.runTurn(context.Background(), "useful", func(event turnEvent) error {
		if event.normalized.Type == engine.EventTextDelta {
			text += event.normalized.Content
		}
		return nil
	})
	if result.ok || !result.abnormal || result.canceled || !strings.Contains(result.reason, "without a successful terminal event") || sess.IsAlive() || sess.isRunning() {
		t.Fatalf("useful unterminated turn = %#v alive=%v running=%v", result, sess.IsAlive(), sess.isRunning())
	}
	if text != "useful output" {
		t.Fatalf("useful output was not emitted before failure: %q", text)
	}
}

func TestEngineAvailabilityShape(t *testing.T) {
	server := NewServer(t.TempDir())
	req := newSecurityRequest(http.MethodGet, "/api/engines", "")
	rec := newDeadlineResponseRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/engines returned %d", rec.Code)
	}
	var payload struct {
		Engines []map[string]any `json:"engines"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Engines) != 4 {
		t.Fatalf("engines = %#v", payload.Engines)
	}
	for _, item := range payload.Engines {
		if item["name"] == "" || item["display_name"] == "" {
			t.Fatalf("engine missing identity: %#v", item)
		}
		if _, ok := item["available"].(bool); !ok {
			t.Fatalf("engine missing boolean availability: %#v", item)
		}
		if item["available"] == false && item["reason"] == "" {
			t.Fatalf("unavailable engine missing reason: %#v", item)
		}
	}
}

func TestSessionDirectoryConfinement(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "projects", "child")
	outside := t.TempDir()
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	server := NewServer(root)
	for _, allowed := range []string{"", root, project, filepath.Join("projects", "child")} {
		if _, err := server.resolveSessionDir(allowed); err != nil {
			t.Errorf("allowed dir %q rejected: %v", allowed, err)
		}
	}
	if _, err := server.resolveSessionDir(outside); err == nil {
		t.Fatalf("outside dir %q accepted", outside)
	}
}

func TestProjectsSymlinkOutsideRootIsRejected(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("creating directory symlinks may require elevated Windows privileges")
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "projects")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	server := NewServer(root)
	if _, err := server.resolveSessionDir(outside); err == nil {
		t.Fatal("projects symlink escaping root was accepted")
	}
}

func TestMediaOutsideProjectsRejected(t *testing.T) {
	root := t.TempDir()
	mustWriteTestFile(t, filepath.Join(root, "projects", "demo", "asset.txt"), "inside")
	mustWriteTestFile(t, filepath.Join(root, "outside.txt"), "outside")
	server := NewServer(root)

	for _, path := range []string{"/api/media/outside.txt", "/api/media/web/index.html"} {
		req := newSecurityRequest(http.MethodGet, path, "")
		rec := newDeadlineResponseRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Fatalf("outside media %q was served", path)
		}
	}
}

func TestMediaProjectsDirectoryItselfIsNotServed(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	server := NewServer(root)
	req := newSecurityRequest(http.MethodGet, "/api/media/projects", "")
	rec := newDeadlineResponseRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatal("projects directory was served")
	}
}

func TestConfigUnknownKeyRejected(t *testing.T) {
	server := NewServer(t.TempDir())
	const key = "FACET_UNKNOWN_TEST_KEY"
	_ = os.Unsetenv(key)
	req := newSecurityRequest(http.MethodPost, "/api/config", `{"FACET_UNKNOWN_TEST_KEY":"secret"}`)
	req.Header.Set(sessionTokenHeader, server.sessionToken)
	rec := newDeadlineResponseRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || os.Getenv(key) != "" || !strings.Contains(rec.Body.String(), "unknown configuration key") {
		t.Fatalf("unknown config response=%d body=%s env=%q", rec.Code, rec.Body.String(), os.Getenv(key))
	}
}

func TestSessionPayloadUsesNativeIDWithoutClaudeAlias(t *testing.T) {
	sess := &Session{ID: "studio", NativeID: "native", Engine: "codex", Dir: "dir", Mode: "rw", valid: true}
	payload := sessionPayload(sess)
	if payload["id"] != "studio" || payload["native_id"] != "native" || payload["engine"] != "codex" || payload["alive"] != true {
		t.Fatalf("session payload = %#v", payload)
	}
	if _, exists := payload["claude_id"]; exists {
		t.Fatalf("legacy claude_id alias remains: %#v", payload)
	}
}

func TestProjectArtifactLayoutFallback(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "projects", "fallback-layout")
	mustWriteTestFile(t, filepath.Join(project, "artifacts", "brief.md"), "# Artifact Brief\n")
	mustWriteTestFile(t, filepath.Join(project, "artifacts", "script.md"), "# Artifact Script\n\n| # | Beat |\n|---|---|\n| 1 | Intro |\n")

	details, err := GetProjectDetails(root, "fallback-layout")
	if err != nil {
		t.Fatalf("GetProjectDetails failed: %v", err)
	}
	if details.Brief == "" || details.Script == "" || !details.Stages.Brief || !details.Stages.Script {
		t.Fatalf("artifact fallback was not loaded: %#v", details)
	}
	if details.Name != "Artifact Brief" || len(details.Beats) != 1 {
		t.Fatalf("artifact fallback was not parsed: name=%q beats=%#v", details.Name, details.Beats)
	}
	if details.BriefPath != "projects/fallback-layout/artifacts/brief.md" || details.BriefURL != "/api/media/projects/fallback-layout/artifacts/brief.md" {
		t.Fatalf("brief fallback location was synthesized: path=%q url=%q", details.BriefPath, details.BriefURL)
	}
	if details.ScriptPath != "projects/fallback-layout/artifacts/script.md" || details.ScriptURL != "/api/media/projects/fallback-layout/artifacts/script.md" {
		t.Fatalf("script fallback location was synthesized: path=%q url=%q", details.ScriptPath, details.ScriptURL)
	}
}

func TestGetProjectDetailsRejectsTraversalAndNonChildren(t *testing.T) {
	root := t.TempDir()
	mustWriteTestFile(t, filepath.Join(root, "projects", "valid-project", "brief.md"), "# Valid\n")
	mustWriteTestFile(t, filepath.Join(root, "projects", "plain-file"), "not a directory")
	mustWriteTestFile(t, filepath.Join(root, "projects", "parent", "nested", "brief.md"), "# Nested\n")
	mustWriteTestFile(t, filepath.Join(root, "outside", "brief.md"), "# Outside\n")

	if _, err := GetProjectDetails(root, "valid-project"); err != nil {
		t.Fatalf("valid direct child was rejected: %v", err)
	}
	for _, slug := range []string{
		"",
		".",
		"..",
		"../outside",
		`..\outside`,
		"parent/nested",
		`parent\nested`,
		"/outside",
		`C:\outside`,
		"%2e%2e",
		"%2e%2e%2foutside",
		"..%2Foutside",
		"%252e%252e%252foutside",
		"missing-project",
		"plain-file",
	} {
		t.Run(strings.ReplaceAll(slug, "/", "_"), func(t *testing.T) {
			if _, err := GetProjectDetails(root, slug); err == nil {
				t.Fatalf("GetProjectDetails accepted invalid or non-child slug %q", slug)
			}
		})
	}
}

func TestProjectJSONArtifactsAndCompositionLocations(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "projects", "json-layout")
	mustWriteTestFile(t, filepath.Join(project, "artifacts", "brief.json"), `{"title":"JSON Brief","audience":"Editors"}`)
	mustWriteTestFile(t, filepath.Join(project, "artifacts", "script.json"), `{"name":"JSON Script","scenes":[{"id":"opening","start":0,"end":2.5,"title":"Open","visual":"A real frame","narration":"A real line."}]}`)
	mustWriteTestFile(t, filepath.Join(project, "artifacts", "explainer_props.json"), `{"cuts":[{"type":"hero_title","in_seconds":0,"out_seconds":2.5,"text":"Open"}]}`)

	details, err := GetProjectDetails(root, "json-layout")
	if err != nil {
		t.Fatalf("GetProjectDetails failed: %v", err)
	}
	if details.Name != "JSON Brief" || !strings.Contains(details.Brief, "\n  \"title\": \"JSON Brief\"") || !strings.HasSuffix(details.Brief, "\n") {
		t.Fatalf("brief JSON was not exposed readably: name=%q brief=%q", details.Name, details.Brief)
	}
	if !strings.Contains(details.Script, "\n  \"name\": \"JSON Script\"") || len(details.Beats) != 1 {
		t.Fatalf("script JSON was not exposed or parsed: script=%q beats=%#v", details.Script, details.Beats)
	}
	beat := details.Beats[0]
	if beat.Index != "opening" || beat.TimeRange != "0s - 2.5s" || beat.Title != "Open" || beat.Visual != "A real frame" || beat.Narration != "A real line." {
		t.Fatalf("unexpected JSON script beat: %#v", beat)
	}
	if details.BriefPath != "projects/json-layout/artifacts/brief.json" || details.BriefURL != "/api/media/projects/json-layout/artifacts/brief.json" {
		t.Fatalf("unexpected brief location: path=%q url=%q", details.BriefPath, details.BriefURL)
	}
	if details.ScriptPath != "projects/json-layout/artifacts/script.json" || details.ScriptURL != "/api/media/projects/json-layout/artifacts/script.json" {
		t.Fatalf("unexpected script location: path=%q url=%q", details.ScriptPath, details.ScriptURL)
	}
	if details.CompositionPath != "projects/json-layout/artifacts/explainer_props.json" || details.CompositionURL != "/api/media/projects/json-layout/artifacts/explainer_props.json" || details.RemotionProps == nil {
		t.Fatalf("current props artifact was not resolved: path=%q url=%q props=%#v", details.CompositionPath, details.CompositionURL, details.RemotionProps)
	}

	editProject := filepath.Join(root, "projects", "source-edit")
	mustWriteTestFile(t, filepath.Join(editProject, "artifacts", "edit.json"), `{"segments":[{"start":1,"end":2}],"output":"projects/source-edit/renders/edit.mp4"}`)
	edit, err := GetProjectDetails(root, "source-edit")
	if err != nil {
		t.Fatalf("GetProjectDetails source-edit failed: %v", err)
	}
	if !edit.Stages.Composition || edit.RemotionProps != nil || edit.CompositionPath != "projects/source-edit/artifacts/edit.json" || edit.CompositionURL != "/api/media/projects/source-edit/artifacts/edit.json" {
		t.Fatalf("edit evidence was not exposed truthfully: %#v", edit)
	}
}

func TestNestedReviewStatusIsNormalized(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "projects", "failed-review")
	mustWriteTestFile(t, filepath.Join(project, "review", "report.json"), `{"ok":true,"result":{"review_status":"fail","gates":[{"name":"audio","status":"fail"}]}}`)

	details, err := GetProjectDetails(root, "failed-review")
	if err != nil {
		t.Fatalf("GetProjectDetails failed: %v", err)
	}
	report, ok := details.ReviewReport.(map[string]any)
	if !ok || report["status"] != "fail" {
		t.Fatalf("nested review failure was not normalized: %#v", details.ReviewReport)
	}
	if _, nested := report["result"]; nested {
		t.Fatalf("review result envelope was not removed: %#v", report)
	}

	mustWriteTestFile(t, filepath.Join(root, "projects", "unknown-review", "review", "report.json"), `{"result":{"gates":[]}}`)
	unknown, err := GetProjectDetails(root, "unknown-review")
	if err != nil {
		t.Fatalf("GetProjectDetails unknown review failed: %v", err)
	}
	unknownReport, ok := unknown.ReviewReport.(map[string]any)
	if !ok || unknownReport["status"] != "unknown" {
		t.Fatalf("missing review status was treated as pass: %#v", unknown.ReviewReport)
	}

	mustWriteTestFile(t, filepath.Join(root, "projects", "frames-only", "qa", "frame.png"), "image")
	framesOnly, err := GetProjectDetails(root, "frames-only")
	if err != nil {
		t.Fatalf("GetProjectDetails frames-only review failed: %v", err)
	}
	if framesOnly.ReviewReport != nil || !framesOnly.Stages.Review {
		t.Fatalf("frames-only evidence synthesized a report or hid review evidence: report=%#v stages=%#v", framesOnly.ReviewReport, framesOnly.Stages)
	}
}

func TestOnlyNonEmptyStableRenderIsMaster(t *testing.T) {
	tests := []struct {
		name        string
		file        string
		contents    string
		wantMaster  bool
		wantPreview bool
	}{
		{name: "final", file: "final.mp4", contents: "video", wantMaster: true},
		{name: "video", file: "video.mp4", contents: "video", wantMaster: true},
		{name: "zero byte final", file: "final.mp4"},
		{name: "edit preview", file: "edit.mp4", contents: "preview", wantPreview: true},
		{name: "vo montage preview", file: "montage_vo.mp4", contents: "preview", wantPreview: true},
		{name: "montage preview", file: "montage.mp4", contents: "preview", wantPreview: true},
		{name: "arbitrary preview", file: "draft.mp4", contents: "preview", wantPreview: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			mustWriteTestFile(t, filepath.Join(root, "projects", "video", "renders", tt.file), tt.contents)
			details, err := GetProjectDetails(root, "video")
			if err != nil {
				t.Fatalf("GetProjectDetails failed: %v", err)
			}
			wantPath := "projects/video/renders/" + tt.file
			wantURL := "/api/media/" + wantPath
			if details.Stages.Master != tt.wantMaster || (details.VideoURL != "") != tt.wantMaster {
				t.Fatalf("master state = %v video=%q, want master=%v", details.Stages.Master, details.VideoURL, tt.wantMaster)
			}
			if tt.wantMaster && (details.VideoPath != wantPath || details.VideoURL != wantURL || details.VideoVersion == "") {
				t.Fatalf("master location/version = %q %q %q, want %q %q and version", details.VideoPath, details.VideoURL, details.VideoVersion, wantPath, wantURL)
			}
			if (details.PreviewVideoURL != "") != tt.wantPreview {
				t.Fatalf("preview=%q, want preview=%v", details.PreviewVideoURL, tt.wantPreview)
			}
			if tt.wantPreview && (details.PreviewVideoPath != wantPath || details.PreviewVideoURL != wantURL || details.VideoVersion == "") {
				t.Fatalf("preview location/version = %q %q %q, want %q %q and version", details.PreviewVideoPath, details.PreviewVideoURL, details.VideoVersion, wantPath, wantURL)
			}
			projects, err := ListProjects(root)
			if err != nil || len(projects) != 1 {
				t.Fatalf("ListProjects = %#v, %v", projects, err)
			}
			if projects[0].Stages.Master != tt.wantMaster || (projects[0].VideoURL != "") != tt.wantMaster || (projects[0].PreviewVideoURL != "") != tt.wantPreview {
				t.Fatalf("summary video state = %#v, want master=%v preview=%v", projects[0], tt.wantMaster, tt.wantPreview)
			}
			if tt.contents != "" && !details.Stages.Composition {
				t.Fatal("non-empty preview did not mark composition stage")
			}
			if !reflect.DeepEqual(projects[0].Stages, details.Stages) {
				t.Fatalf("summary/detail stages differ: summary=%#v detail=%#v", projects[0].Stages, details.Stages)
			}
		})
	}
}

func TestPreviewPriorityAndStableMasterSuppressesPreview(t *testing.T) {
	root := t.TempDir()
	renders := filepath.Join(root, "projects", "priority", "renders")
	for _, name := range []string{"draft.mp4", "montage.mp4", "montage_vo.mp4", "edit.mp4"} {
		mustWriteTestFile(t, filepath.Join(renders, name), name)
	}
	details, err := GetProjectDetails(root, "priority")
	if err != nil {
		t.Fatalf("GetProjectDetails failed: %v", err)
	}
	if details.PreviewVideoPath != "projects/priority/renders/edit.mp4" || details.Stages.Master || details.VideoPath != "" {
		t.Fatalf("preview priority claimed a master or selected the wrong preview: %#v", details)
	}

	mustWriteTestFile(t, filepath.Join(renders, "video.mp4"), "stable")
	details, err = GetProjectDetails(root, "priority")
	if err != nil {
		t.Fatalf("GetProjectDetails with master failed: %v", err)
	}
	if !details.Stages.Master || details.VideoPath != "projects/priority/renders/video.mp4" || details.PreviewVideoPath != "" || details.PreviewVideoURL != "" {
		t.Fatalf("stable master did not suppress preview fallback: %#v", details)
	}
}

func TestZeroByteMediaIsIgnored(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "projects", "empty-media")
	for _, path := range []string{
		filepath.Join(project, "narration", "voice.ogg"),
		filepath.Join(project, "qa", "frame.png"),
		filepath.Join(project, "review", "final-frames", "review.jpg"),
		filepath.Join(project, "assets", "raw", "thumbnail.webp"),
		filepath.Join(project, "renders", "draft.mp4"),
	} {
		mustWriteTestFile(t, path, "")
	}

	details, err := GetProjectDetails(root, "empty-media")
	if err != nil {
		t.Fatalf("GetProjectDetails failed: %v", err)
	}
	if details.Stages.Voiceover || details.Stages.Review || details.Stages.Composition || len(details.Narration) != 0 || len(details.QAFrames) != 0 || len(details.ReviewFrames) != 0 || details.ThumbnailPath != "" || details.PreviewVideoPath != "" {
		t.Fatalf("zero-byte media was exposed as evidence: %#v", details)
	}

	mustWriteTestFile(t, filepath.Join(project, "assets", "audio", "voice.ogg"), "audio")
	mustWriteTestFile(t, filepath.Join(project, "qa", "real.png"), "image")
	details, err = GetProjectDetails(root, "empty-media")
	if err != nil {
		t.Fatalf("GetProjectDetails with media failed: %v", err)
	}
	if !details.Stages.Voiceover || len(details.Narration) != 1 || details.Narration[0].Name != "voice.ogg" {
		t.Fatalf("non-empty OGG evidence was not exposed: %#v", details.Narration)
	}
	if !details.Stages.Review || len(details.QAFrames) != 1 || details.ThumbnailPath != "projects/empty-media/qa/real.png" {
		t.Fatalf("non-empty image evidence was not exposed: frames=%#v thumbnail=%q", details.QAFrames, details.ThumbnailPath)
	}
}

func TestProjectSummaryAndDetailsShareEvidenceSemantics(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "projects", "consistent")
	mustWriteTestFile(t, filepath.Join(project, "artifacts", "brief.json"), `{"name":"Consistent Project"}`)
	mustWriteTestFile(t, filepath.Join(project, "artifacts", "script.json"), `{"beats":[{"title":"Beat one","narration":"Truth"}]}`)
	mustWriteTestFile(t, filepath.Join(project, "narration", "voice.ogg"), "audio")
	mustWriteTestFile(t, filepath.Join(project, "artifacts", "explainer_props.json"), `{"cuts":[]}`)
	mustWriteTestFile(t, filepath.Join(project, "review", "report.json"), `{"status":"pass","gates":[]}`)
	mustWriteTestFile(t, filepath.Join(project, "renders", "edit.mp4"), "preview")

	details, err := GetProjectDetails(root, "consistent")
	if err != nil {
		t.Fatalf("GetProjectDetails failed: %v", err)
	}
	projects, err := ListProjects(root)
	if err != nil || len(projects) != 1 {
		t.Fatalf("ListProjects = %#v, %v", projects, err)
	}
	summary := projects[0]
	if !reflect.DeepEqual(summary.Stages, details.Stages) {
		t.Fatalf("summary/detail stages differ: summary=%#v detail=%#v", summary.Stages, details.Stages)
	}
	if !details.Stages.Brief || !details.Stages.Script || !details.Stages.Voiceover || !details.Stages.Composition || !details.Stages.Review || details.Stages.Master {
		t.Fatalf("unexpected shared evidence stages: %#v", details.Stages)
	}
	if summary.Name != details.Name || summary.BriefPath != details.BriefPath || summary.BriefURL != details.BriefURL || summary.ScriptPath != details.ScriptPath || summary.ScriptURL != details.ScriptURL || summary.CompositionPath != details.CompositionPath || summary.CompositionURL != details.CompositionURL || summary.PreviewVideoPath != details.PreviewVideoPath || summary.PreviewVideoURL != details.PreviewVideoURL || summary.VideoVersion != details.VideoVersion {
		t.Fatalf("summary/detail evidence locations differ: summary=%#v detail=%#v", summary, details)
	}
	if details.ReviewReport == nil || len(details.Narration) != 1 || len(details.Beats) != 1 {
		t.Fatalf("valid detail evidence was not returned: %#v", details)
	}

	invalidProject := filepath.Join(root, "projects", "invalid-review")
	mustWriteTestFile(t, filepath.Join(invalidProject, "review", "report.json"), `{not JSON}`)
	invalidDetails, err := GetProjectDetails(root, "invalid-review")
	if err != nil {
		t.Fatalf("GetProjectDetails invalid-review failed: %v", err)
	}
	if invalidDetails.Stages.Review || invalidDetails.ReviewReport != nil {
		t.Fatalf("invalid review JSON was exposed as valid evidence: %#v", invalidDetails)
	}
}

func TestVideoVersionChangesWhenSamePathIsReplaced(t *testing.T) {
	root := t.TempDir()
	videoPath := filepath.Join(root, "projects", "versioned", "renders", "final.mp4")
	mustWriteTestFile(t, videoPath, "first")
	firstTime := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(videoPath, firstTime, firstTime); err != nil {
		t.Fatalf("Chtimes first video: %v", err)
	}
	first, err := GetProjectDetails(root, "versioned")
	if err != nil {
		t.Fatalf("GetProjectDetails first video failed: %v", err)
	}

	mustWriteTestFile(t, videoPath, "later")
	secondTime := firstTime.Add(time.Second)
	if err := os.Chtimes(videoPath, secondTime, secondTime); err != nil {
		t.Fatalf("Chtimes replacement video: %v", err)
	}
	second, err := GetProjectDetails(root, "versioned")
	if err != nil {
		t.Fatalf("GetProjectDetails replacement video failed: %v", err)
	}
	if first.VideoPath != second.VideoPath || first.VideoURL != second.VideoURL || first.VideoVersion == "" || second.VideoVersion == "" || first.VideoVersion == second.VideoVersion {
		t.Fatalf("same-path replacement was not versioned: first=%q second=%q path=%q url=%q", first.VideoVersion, second.VideoVersion, second.VideoPath, second.VideoURL)
	}
	projects, err := ListProjects(root)
	if err != nil || len(projects) != 1 || projects[0].VideoVersion != second.VideoVersion {
		t.Fatalf("summary version does not match detail: projects=%#v err=%v detail=%q", projects, err, second.VideoVersion)
	}
}

func TestProjectEvidenceAcceptsSafeRegularFiles(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "projects", "safe-evidence")
	briefPath := filepath.Join(project, "brief.md")
	mustWriteTestFile(t, briefPath, "# Safe Evidence\n")
	mustWriteTestFile(t, filepath.Join(project, "artifacts", "script.json"), `{"beats":[{"title":"Safe beat","narration":"Safe narration"}]}`)
	mustWriteTestFile(t, filepath.Join(project, "artifacts", "safe_props.json"), `{"cuts":[]}`)
	mustWriteTestFile(t, filepath.Join(project, "narration", "voice.wav"), "audio")
	mustWriteTestFile(t, filepath.Join(project, "qa", "qa.png"), "qa frame")
	mustWriteTestFile(t, filepath.Join(project, "review", "final-frames", "review.jpg"), "review frame")
	mustWriteTestFile(t, filepath.Join(project, "review", "report.json"), `{"status":"pass"}`)
	mustWriteTestFile(t, filepath.Join(project, "renders", "final.mp4"), "video")
	mustWriteTestFile(t, filepath.Join(project, "assets", "raw", "thumbnail.webp"), "thumbnail")

	details, err := GetProjectDetails(root, "safe-evidence")
	if err != nil {
		t.Fatalf("GetProjectDetails failed: %v", err)
	}
	if details.Name != "Safe Evidence" || details.Brief == "" || details.Script == "" || details.RemotionProps == nil || details.ReviewReport == nil {
		t.Fatalf("safe artifact evidence was not loaded: %#v", details)
	}
	if len(details.Beats) != 1 || len(details.Narration) != 1 || len(details.QAFrames) != 1 || len(details.ReviewFrames) != 1 {
		t.Fatalf("safe structured and media evidence was not loaded: %#v", details)
	}
	if !details.Stages.Brief || !details.Stages.Script || !details.Stages.Voiceover || !details.Stages.Composition || !details.Stages.Review || !details.Stages.Master {
		t.Fatalf("safe evidence stages = %#v", details.Stages)
	}
	if details.VideoPath != "projects/safe-evidence/renders/final.mp4" || details.VideoURL != "/api/media/projects/safe-evidence/renders/final.mp4" || details.VideoVersion == "" {
		t.Fatalf("safe video evidence = path %q, URL %q, version %q", details.VideoPath, details.VideoURL, details.VideoVersion)
	}
	if details.ThumbnailPath != "projects/safe-evidence/review/final-frames/review.jpg" || details.ThumbnailURL != "/api/media/projects/safe-evidence/review/final-frames/review.jpg" {
		t.Fatalf("safe thumbnail evidence = path %q, URL %q", details.ThumbnailPath, details.ThumbnailURL)
	}
	if rel, mediaURL := evidenceLocation(root, briefPath); rel != "projects/safe-evidence/brief.md" || mediaURL != "/api/media/projects/safe-evidence/brief.md" {
		t.Fatalf("safe evidence location = %q, %q", rel, mediaURL)
	}
}

func TestProjectEvidenceIgnoresEscapedFileSymlinks(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "projects", "file-escape")
	otherProject := filepath.Join(root, "projects", "other-project")
	outside := t.TempDir()
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}

	targets := map[string]string{
		filepath.Join(project, "brief.md"):                        filepath.Join(outside, "brief.md"),
		filepath.Join(project, "script.json"):                     filepath.Join(otherProject, "script.json"),
		filepath.Join(project, "remotion_props.json"):             filepath.Join(outside, "props.json"),
		filepath.Join(project, "artifacts", "edit.json"):          filepath.Join(otherProject, "edit.json"),
		filepath.Join(project, "review", "report.json"):           filepath.Join(outside, "report.json"),
		filepath.Join(project, "narration", "voice.wav"):          filepath.Join(outside, "voice.wav"),
		filepath.Join(project, "qa", "frame.png"):                 filepath.Join(otherProject, "frame.png"),
		filepath.Join(project, "renders", "final.mp4"):            filepath.Join(outside, "final.mp4"),
		filepath.Join(project, "assets", "raw", "thumbnail.webp"): filepath.Join(otherProject, "thumbnail.webp"),
	}
	contents := map[string]string{
		filepath.Join(outside, "brief.md"):            "# Leaked Brief\n",
		filepath.Join(otherProject, "script.json"):    `{"title":"Leaked Script","beats":[{"title":"Leak"}]}`,
		filepath.Join(outside, "props.json"):          `{"cuts":[{"text":"Leak"}]}`,
		filepath.Join(otherProject, "edit.json"):      `{"segments":[{"name":"Leak"}]}`,
		filepath.Join(outside, "report.json"):         `{"status":"pass","secret":"leak"}`,
		filepath.Join(outside, "voice.wav"):           "leaked audio",
		filepath.Join(otherProject, "frame.png"):      "leaked frame",
		filepath.Join(outside, "final.mp4"):           "leaked video",
		filepath.Join(otherProject, "thumbnail.webp"): "leaked thumbnail",
	}
	for path, content := range contents {
		mustWriteTestFile(t, path, content)
	}
	for link, target := range targets {
		if !makeTestSymlink(t, target, link) {
			return
		}
	}

	details, err := GetProjectDetails(root, "file-escape")
	if err != nil {
		t.Fatalf("GetProjectDetails failed: %v", err)
	}
	if details.Brief != "" || details.Script != "" || details.RemotionProps != nil || details.ReviewReport != nil {
		t.Fatalf("escaped inline artifact was exposed: %#v", details)
	}
	if len(details.Beats) != 0 || len(details.Narration) != 0 || len(details.QAFrames) != 0 || len(details.ReviewFrames) != 0 {
		t.Fatalf("escaped structured or media artifact was exposed: %#v", details)
	}
	if details.BriefPath != "" || details.BriefURL != "" || details.ScriptPath != "" || details.ScriptURL != "" || details.CompositionPath != "" || details.CompositionURL != "" || details.VideoPath != "" || details.VideoURL != "" || details.PreviewVideoPath != "" || details.PreviewVideoURL != "" || details.ThumbnailPath != "" || details.ThumbnailURL != "" {
		t.Fatalf("escaped artifact location or URL was exposed: %#v", details)
	}
	if details.Stages != (StageStatuses{}) {
		t.Fatalf("escaped files affected stages: %#v", details.Stages)
	}
	for _, path := range []string{"", filepath.Join(outside, "brief.md"), filepath.Join(project, "brief.md"), filepath.Join(project, "script.json")} {
		if rel, mediaURL := evidenceLocation(root, path); rel != "" || mediaURL != "" {
			t.Fatalf("unsafe evidenceLocation(%q) = %q, %q", path, rel, mediaURL)
		}
	}

	projects, err := ListProjects(root)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	for _, summary := range projects {
		if summary.Slug == "file-escape" && (summary.Stages != (StageStatuses{}) || summary.BriefURL != "" || summary.ScriptURL != "" || summary.CompositionURL != "" || summary.VideoURL != "" || summary.PreviewVideoURL != "" || summary.ThumbnailURL != "") {
			t.Fatalf("escaped file evidence reached project summary: %#v", summary)
		}
	}
}

func TestProjectEvidenceIgnoresEscapedDirectorySymlinks(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "projects", "directory-escape")
	otherProject := filepath.Join(root, "projects", "other-project")
	outside := t.TempDir()
	mustWriteTestFile(t, filepath.Join(project, "brief.md"), "# Directory Escape\n")
	mustWriteTestFile(t, filepath.Join(outside, "artifacts", "script.json"), `{"title":"Leaked Script"}`)
	mustWriteTestFile(t, filepath.Join(outside, "artifacts", "leaked_props.json"), `{"cuts":[{"text":"Leak"}]}`)
	mustWriteTestFile(t, filepath.Join(outside, "narration", "voice.wav"), "leaked audio")
	mustWriteTestFile(t, filepath.Join(otherProject, "qa", "frame.png"), "leaked frame")
	mustWriteTestFile(t, filepath.Join(otherProject, "review", "report.json"), `{"status":"pass"}`)
	mustWriteTestFile(t, filepath.Join(otherProject, "review", "final-frames", "review.jpg"), "leaked review frame")
	mustWriteTestFile(t, filepath.Join(outside, "renders", "final.mp4"), "leaked video")

	for link, target := range map[string]string{
		filepath.Join(project, "artifacts"): filepath.Join(outside, "artifacts"),
		filepath.Join(project, "narration"): filepath.Join(outside, "narration"),
		filepath.Join(project, "qa"):        filepath.Join(otherProject, "qa"),
		filepath.Join(project, "review"):    filepath.Join(otherProject, "review"),
		filepath.Join(project, "renders"):   filepath.Join(outside, "renders"),
	} {
		if !makeTestSymlink(t, target, link) {
			return
		}
	}

	details, err := GetProjectDetails(root, "directory-escape")
	if err != nil {
		t.Fatalf("GetProjectDetails failed: %v", err)
	}
	if details.Brief == "" || !details.Stages.Brief {
		t.Fatalf("safe regular artifact stopped working: %#v", details)
	}
	if details.Script != "" || details.RemotionProps != nil || details.ReviewReport != nil || details.Stages.Voiceover || details.Stages.Review || details.Stages.Composition || details.Stages.Master || len(details.Narration) != 0 || len(details.QAFrames) != 0 || len(details.ReviewFrames) != 0 || details.VideoURL != "" || details.PreviewVideoURL != "" || details.ThumbnailURL != "" {
		t.Fatalf("escaped directory evidence was exposed: %#v", details)
	}
	for _, path := range []string{
		filepath.Join(project, "artifacts", "script.json"),
		filepath.Join(project, "artifacts", "leaked_props.json"),
		filepath.Join(project, "narration", "voice.wav"),
		filepath.Join(project, "qa", "frame.png"),
		filepath.Join(project, "review", "report.json"),
		filepath.Join(project, "renders", "final.mp4"),
	} {
		if rel, mediaURL := evidenceLocation(root, path); rel != "" || mediaURL != "" {
			t.Fatalf("escaped directory location %q = %q, %q", path, rel, mediaURL)
		}
	}
}

func TestProjectRootSymlinkCannotAliasAnotherProject(t *testing.T) {
	root := t.TempDir()
	realProject := filepath.Join(root, "projects", "real-project")
	aliasProject := filepath.Join(root, "projects", "alias-project")
	outsideProject := filepath.Join(t.TempDir(), "outside-project")
	outsideAlias := filepath.Join(root, "projects", "outside-alias")
	mustWriteTestFile(t, filepath.Join(realProject, "brief.md"), "# Real Project\n")
	mustWriteTestFile(t, filepath.Join(outsideProject, "brief.md"), "# Outside Project\n")
	if !makeTestSymlink(t, realProject, aliasProject) || !makeTestSymlink(t, outsideProject, outsideAlias) {
		return
	}

	for _, slug := range []string{"alias-project", "outside-alias"} {
		if _, err := GetProjectDetails(root, slug); err == nil {
			t.Fatalf("linked project root %q was accepted", slug)
		}
	}
	projects, err := ListProjects(root)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	for _, project := range projects {
		if project.Slug == "alias-project" || project.Slug == "outside-alias" {
			t.Fatalf("linked project root was listed: %#v", project)
		}
	}
	for _, path := range []string{filepath.Join(aliasProject, "brief.md"), filepath.Join(outsideAlias, "brief.md")} {
		if rel, mediaURL := evidenceLocation(root, path); rel != "" || mediaURL != "" {
			t.Fatalf("linked project root %q generated evidence location %q, %q", path, rel, mediaURL)
		}
	}
}

func makeTestSymlink(t *testing.T, target, link string) bool {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(link), err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symlink %q -> %q: %v", link, target, err)
		return false
	}
	return true
}

func mustWriteTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
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
