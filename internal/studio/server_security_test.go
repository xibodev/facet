package studio

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type deadlineResponseRecorder struct {
	*httptest.ResponseRecorder
}

func newDeadlineResponseRecorder() *deadlineResponseRecorder {
	return &deadlineResponseRecorder{ResponseRecorder: httptest.NewRecorder()}
}

func (*deadlineResponseRecorder) SetWriteDeadline(time.Time) error {
	return nil
}

func TestNewSessionAcceptsOnlyAutonomousReadWriteMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		wantMode string
		wantErr  bool
	}{
		{name: "empty defaults to read-write", wantMode: "rw"},
		{name: "read-write", mode: "rw", wantMode: "rw"},
		{name: "read-only", mode: "ro", wantErr: true},
		{name: "ask", mode: "ask", wantErr: true},
		{name: "unknown", mode: "anything", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(t.TempDir())
			sess, err := server.newSession("", tt.mode, "claude")
			if tt.wantErr {
				if err == nil || sess != nil {
					t.Fatalf("newSession mode %q = %#v, %v; want rejection", tt.mode, sess, err)
				}
				if len(server.sessions) != 0 {
					t.Fatalf("rejected mode %q created a session", tt.mode)
				}
				return
			}
			if err != nil {
				t.Fatalf("newSession mode %q: %v", tt.mode, err)
			}
			if sess.Mode != tt.wantMode {
				t.Fatalf("newSession mode %q normalized to %q, want %q", tt.mode, sess.Mode, tt.wantMode)
			}
		})
	}
}

func TestMediaEndpointEnforcesExtensionAllowlist(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"projects/demo/.env":                 "TEST_ONLY_ENV_VALUE",
		"projects/demo/source.go":            "package denied",
		"projects/demo/extensionless":        "extensionless denied",
		"projects/demo/archive.bin":          "unknown denied",
		"projects/demo/artifacts/brief.json": `{"title":"Allowed artifact"}`,
		"projects/demo/artifacts/script.md":  "# Allowed artifact\n",
		"projects/demo/assets/frame.png":     "png-data",
		"projects/demo/renders/final.mp4":    "mp4-data",
	}
	for path, contents := range files {
		writeSecurityTestFile(t, filepath.Join(root, filepath.FromSlash(path)), contents)
	}

	server := NewServer(root)
	for _, path := range []string{
		"projects/demo/.env",
		"projects/demo/source.go",
		"projects/demo/extensionless",
		"projects/demo/archive.bin",
	} {
		t.Run("deny_"+strings.ReplaceAll(filepath.Base(path), ".", "_"), func(t *testing.T) {
			rec := serveSecurityRequest(server, http.MethodGet, "/api/media/"+path, "")
			if rec.Code != http.StatusNotFound && rec.Code != http.StatusUnsupportedMediaType {
				t.Fatalf("GET %s returned %d, want 404 or 415", path, rec.Code)
			}
			if contentType := rec.Header().Get("Content-Type"); contentType == "application/octet-stream" {
				t.Fatalf("GET %s fell back to generic octet-stream", path)
			}
			if strings.Contains(rec.Body.String(), files[path]) {
				t.Fatalf("GET %s disclosed denied file contents", path)
			}
		})
	}

	allowed := map[string]string{
		"projects/demo/artifacts/brief.json": "application/json",
		"projects/demo/artifacts/script.md":  "text/markdown; charset=utf-8",
		"projects/demo/assets/frame.png":     "image/png",
		"projects/demo/renders/final.mp4":    "video/mp4",
	}
	for path, wantContentType := range allowed {
		t.Run("allow_"+strings.ReplaceAll(filepath.Base(path), ".", "_"), func(t *testing.T) {
			rec := serveSecurityRequest(server, http.MethodGet, "/api/media/"+path, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s returned %d: %s", path, rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); got != wantContentType {
				t.Fatalf("GET %s Content-Type = %q, want %q", path, got, wantContentType)
			}
		})
	}
}

func TestConfigRejectsUnknownKeyAtomically(t *testing.T) {
	const (
		key       = "PEXELS_API_KEY"
		before    = "test-before-1111"
		attempted = "test-after-2222"
	)
	t.Setenv(key, before)
	server := NewServer(t.TempDir())

	rec := serveSecurityRequest(server, http.MethodPost, "/api/config", `{"PEXELS_API_KEY":"`+attempted+`","FACET_UNKNOWN_TEST_KEY":"denied"}`)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "unknown configuration key") {
		t.Fatalf("unknown-key POST returned %d: %s", rec.Code, rec.Body.String())
	}
	if got, _ := environmentValue(server.environmentSnapshot(), key); got != before {
		t.Fatalf("server snapshot changed after rejected POST")
	}
	if got := os.Getenv(key); got != before {
		t.Fatalf("process environment changed after rejected POST")
	}
}

func TestConfigUpdateIsAtomicAndSessionScoped(t *testing.T) {
	const (
		key    = "PEXELS_API_KEY"
		before = "test-before-1111"
		after  = "test-after-2222"
	)
	t.Setenv(key, before)
	server := NewServer(t.TempDir())

	oldSession, err := server.newSession("", "", "claude")
	if err != nil {
		t.Fatal(err)
	}
	rec := serveSecurityRequest(server, http.MethodPost, "/api/config", `{"PEXELS_API_KEY":"`+after+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("config POST returned %d: %s", rec.Code, rec.Body.String())
	}
	newSession, err := server.newSession("", "rw", "claude")
	if err != nil {
		t.Fatal(err)
	}

	if got, _ := environmentValue(oldSession.environment, key); got != before {
		t.Fatalf("existing session environment changed after config POST")
	}
	if got, _ := environmentValue(newSession.environment, key); got != after {
		t.Fatalf("new session did not capture updated configuration")
	}

	if err := os.Setenv(key, "external-change-3333"); err != nil {
		t.Fatal(err)
	}
	rec = serveSecurityRequest(server, http.MethodGet, "/api/config", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("config GET returned %d: %s", rec.Code, rec.Body.String())
	}
	var config map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &config); err != nil {
		t.Fatal(err)
	}
	if got := config[key]; got != maskSecret(after) {
		t.Fatalf("config GET did not read the server snapshot: got %q", got)
	}
	if strings.Contains(rec.Body.String(), after) || strings.Contains(rec.Body.String(), "external-change-3333") {
		t.Fatal("config GET disclosed an unmasked value")
	}
}

func TestConfigProcessSyncFailureRollsBackSnapshotAndEnvironment(t *testing.T) {
	const (
		firstKey     = "OPENAI_API_KEY"
		secondKey    = "ELEVENLABS_API_KEY"
		firstBefore  = "open-before-1111"
		secondBefore = "voice-before-2222"
	)
	t.Setenv(firstKey, firstBefore)
	t.Setenv(secondKey, secondBefore)
	server := NewServer(t.TempDir())

	rec := serveSecurityRequest(server, http.MethodPost, "/api/config", `{"OPENAI_API_KEY":"open-after-3333","ELEVENLABS_API_KEY":"invalid\u0000value"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("invalid environment POST returned %d, want 500: %s", rec.Code, rec.Body.String())
	}
	for key, want := range map[string]string{firstKey: firstBefore, secondKey: secondBefore} {
		if got, _ := environmentValue(server.environmentSnapshot(), key); got != want {
			t.Fatalf("server snapshot for %s changed after failed POST", key)
		}
		if got := os.Getenv(key); got != want {
			t.Fatalf("process environment for %s changed after failed POST", key)
		}
	}
}

func TestCloseSessionReportsCloseError(t *testing.T) {
	server := NewServer(t.TempDir())
	sess := &Session{
		ID:    "stuck-session",
		valid: true,
		done:  make(chan struct{}),
	}
	server.sessions[sess.ID] = sess

	rec := serveSecurityRequest(server, http.MethodPost, "/api/close?session="+sess.ID, "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("close failure returned %d, want 500: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "close session") || !strings.Contains(rec.Body.String(), "timed out") {
		t.Fatalf("close failure was not reported: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("close failure was reported as success: %s", rec.Body.String())
	}
	if got := server.getSession(sess.ID); got != sess {
		t.Fatalf("close failure evicted the retry handle: %#v", got)
	}
}

func TestSessionTokenEndpointSameOriginOnly(t *testing.T) {
	server := NewServer(t.TempDir())
	other := NewServer(t.TempDir())
	if server.sessionToken == "" || server.sessionToken == other.sessionToken {
		t.Fatal("NewServer did not generate a unique session token")
	}

	req := newSecurityRequest(http.MethodGet, "/api/session-token", "")
	rec := serveSecurityHTTPRequest(server, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("same-origin token request returned %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["token"] != server.sessionToken {
		t.Fatal("same-origin token response did not contain this Server's token")
	}
	if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("unsafe token response headers: %#v", rec.Header())
	}

	tests := []struct {
		name      string
		host      string
		origin    string
		fetchSite string
	}{
		{name: "cross-site metadata", origin: "http://127.0.0.1:8787", fetchSite: "cross-site"},
		{name: "cross-origin", origin: "http://attacker.example"},
		{name: "scheme mismatch", origin: "https://127.0.0.1:8787"},
		{name: "dns rebinding host", host: "attacker.example:8787", origin: "http://attacker.example:8787"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newSecurityRequest(http.MethodGet, "/api/session-token", "")
			if tt.host != "" {
				req.Host = tt.host
			}
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.fetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", tt.fetchSite)
			}
			rec := serveSecurityHTTPRequest(server, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("token request returned %d, want 403: %s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), server.sessionToken) {
				t.Fatal("forbidden token response disclosed the token")
			}
		})
	}
}

func TestReadOnlyEndpointsDoNotRequireMutationToken(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	server := NewServer(root)
	for _, target := range []string{"/api/projects", "/api/engines", "/api/config"} {
		req := newSecurityRequest(http.MethodGet, target, "")
		rec := serveSecurityHTTPRequest(server, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s returned %d without a mutation token: %s", target, rec.Code, rec.Body.String())
		}
	}
}

func TestEmptyServerTokenFailsClosed(t *testing.T) {
	server := NewServer(t.TempDir())
	server.sessionToken = ""

	rec := serveSecurityHTTPRequest(server, newSecurityRequest(http.MethodGet, "/api/session-token", ""))
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("empty-token endpoint returned %d with Cache-Control %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	if strings.Contains(rec.Body.String(), "token") {
		t.Fatalf("empty-token endpoint returned token-like data: %s", rec.Body.String())
	}

	req := newSecurityRequest(http.MethodPost, "/api/config", `{}`)
	rec = serveSecurityHTTPRequest(server, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("empty-token mutation returned %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChatRequiresQueryTokenBeforeStartingCLI(t *testing.T) {
	server := NewServer(t.TempDir())
	target := "/api/chat?prompt=test&engine=not-a-real-engine"

	for _, tt := range []struct {
		name  string
		token string
	}{
		{name: "missing"},
		{name: "wrong", token: "wrong-token"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := newSecurityRequest(http.MethodGet, target, "")
			if tt.token != "" {
				query := req.URL.Query()
				query.Set(sessionTokenQuery, tt.token)
				req.URL.RawQuery = query.Encode()
			}
			rec := serveSecurityHTTPRequest(server, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("chat with %s token returned %d: %s", tt.name, rec.Code, rec.Body.String())
			}
			if len(server.sessions) != 0 {
				t.Fatalf("chat with %s token created a session", tt.name)
			}
		})
	}

	req := newSecurityRequest(http.MethodGet, target, "")
	query := req.URL.Query()
	query.Set(sessionTokenQuery, server.sessionToken)
	req.URL.RawQuery = query.Encode()
	rec := serveSecurityHTTPRequest(server, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "unknown engine") {
		t.Fatalf("guarded chat did not reach engine validation: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(server.sessions) != 0 {
		t.Fatal("invalid engine created a session after the guard")
	}
}

func TestChatGuardPrecedesRequestValidation(t *testing.T) {
	server := NewServer(t.TempDir())
	rec := serveSecurityHTTPRequest(server, newSecurityRequest(http.MethodGet, "/api/chat", ""))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("tokenless invalid chat returned %d, want guard 403: %s", rec.Code, rec.Body.String())
	}
}

func TestConfigMutationRequiresHeaderToken(t *testing.T) {
	const (
		key    = "PEXELS_API_KEY"
		before = "guard-before-1111"
		after  = "guard-after-2222"
	)
	t.Setenv(key, before)
	server := NewServer(t.TempDir())

	for _, tt := range []struct {
		name  string
		token string
	}{
		{name: "missing"},
		{name: "wrong", token: "wrong-token"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := newSecurityRequest(http.MethodPost, "/api/config", `{"PEXELS_API_KEY":"`+after+`"}`)
			if tt.token != "" {
				req.Header.Set(sessionTokenHeader, tt.token)
			}
			rec := serveSecurityHTTPRequest(server, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("config POST with %s token returned %d: %s", tt.name, rec.Code, rec.Body.String())
			}
			if got, _ := environmentValue(server.environmentSnapshot(), key); got != before || os.Getenv(key) != before {
				t.Fatalf("config POST with %s token changed configuration", tt.name)
			}
		})
	}

	req := newSecurityRequest(http.MethodPost, "/api/config", `{"PEXELS_API_KEY":"`+after+`"}`)
	req.Header.Set(sessionTokenHeader, server.sessionToken)
	rec := serveSecurityHTTPRequest(server, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("config POST with correct token returned %d: %s", rec.Code, rec.Body.String())
	}
	if got, _ := environmentValue(server.environmentSnapshot(), key); got != after || os.Getenv(key) != after {
		t.Fatal("guarded config POST did not apply configuration")
	}
}

func TestConfigQueryTokenIsRejected(t *testing.T) {
	server := NewServer(t.TempDir())
	req := newSecurityRequest(http.MethodPost, "/api/config?token="+server.sessionToken, `{}`)
	rec := serveSecurityHTTPRequest(server, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("config POST accepted query token: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMutationGuardRejectsCrossOrigin(t *testing.T) {
	server := NewServer(t.TempDir())
	tests := []struct {
		name      string
		method    string
		target    string
		origin    string
		fetchSite string
	}{
		{name: "config origin", method: http.MethodPost, target: "/api/config", origin: "http://attacker.example"},
		{name: "config fetch metadata", method: http.MethodPost, target: "/api/config", origin: "http://127.0.0.1:8787", fetchSite: "cross-site"},
		{name: "chat origin", method: http.MethodGet, target: "/api/chat?prompt=test&engine=not-a-real-engine", origin: "http://attacker.example"},
		{name: "chat fetch metadata", method: http.MethodGet, target: "/api/chat?prompt=test&engine=not-a-real-engine", origin: "http://127.0.0.1:8787", fetchSite: "cross-site"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := ""
			if tt.method == http.MethodPost {
				body = `{}`
			}
			req := newSecurityRequest(tt.method, tt.target, body)
			if tt.method == http.MethodPost {
				req.Header.Set(sessionTokenHeader, server.sessionToken)
			} else {
				query := req.URL.Query()
				query.Set(sessionTokenQuery, server.sessionToken)
				req.URL.RawQuery = query.Encode()
			}
			req.Header.Set("Origin", tt.origin)
			if tt.fetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", tt.fetchSite)
			}
			rec := serveSecurityHTTPRequest(server, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("cross-origin mutation returned %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestCloseEvictsOnlyAfterConfirmedSuccess(t *testing.T) {
	server := NewServer(t.TempDir())
	sess := &Session{ID: "closed-session", valid: true}
	server.sessions[sess.ID] = sess

	for _, tt := range []struct {
		name  string
		token string
	}{
		{name: "missing"},
		{name: "wrong", token: "wrong-token"},
		{name: "query only", token: server.sessionToken},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := newSecurityRequest(http.MethodPost, "/api/close?session="+sess.ID, "")
			if tt.name == "query only" {
				query := req.URL.Query()
				query.Set(sessionTokenQuery, tt.token)
				req.URL.RawQuery = query.Encode()
			} else if tt.token != "" {
				req.Header.Set(sessionTokenHeader, tt.token)
			}
			rec := serveSecurityHTTPRequest(server, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("close with %s token returned %d: %s", tt.name, rec.Code, rec.Body.String())
			}
			if server.getSession(sess.ID) != sess || !sess.IsAlive() {
				t.Fatalf("close with %s token changed the session", tt.name)
			}
		})
	}

	rec := serveSecurityRequest(server, http.MethodPost, "/api/close?session="+sess.ID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("successful close returned %d: %s", rec.Code, rec.Body.String())
	}
	if server.getSession(sess.ID) != nil {
		t.Fatal("successfully closed session was not evicted")
	}
}

func TestGETCloseRouteRemoved(t *testing.T) {
	server := NewServer(t.TempDir())
	sess := &Session{ID: "get-close-session", valid: true}
	server.sessions[sess.ID] = sess

	rec := serveSecurityRequest(server, http.MethodGet, "/api/close?session="+sess.ID, "")
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET close returned %d, want 404 or 405: %s", rec.Code, rec.Body.String())
	}
	if server.getSession(sess.ID) != sess || !sess.IsAlive() {
		t.Fatal("GET close mutated the session")
	}
}

func serveSecurityRequest(server *Server, method, target, body string) *deadlineResponseRecorder {
	req := newSecurityRequest(method, target, body)
	if method == http.MethodPost {
		req.Header.Set(sessionTokenHeader, server.sessionToken)
	}
	if method == http.MethodGet && req.URL.Path == "/api/chat" {
		query := req.URL.Query()
		query.Set(sessionTokenQuery, server.sessionToken)
		req.URL.RawQuery = query.Encode()
	}
	return serveSecurityHTTPRequest(server, req)
}

func newSecurityRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Origin", "http://127.0.0.1:8787")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func serveSecurityHTTPRequest(server *Server, req *http.Request) *deadlineResponseRecorder {
	rec := newDeadlineResponseRecorder()
	server.Handler().ServeHTTP(rec, req)
	return rec
}

func writeSecurityTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create test directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}
