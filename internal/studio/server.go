package studio

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xibodev/facet/internal/studio/engine"
	"github.com/xibodev/facet/web"
)

var configurableEnvironmentKeys = [...]string{
	"OPENAI_API_KEY",
	"ELEVENLABS_API_KEY",
	"FAL_KEY",
	"PEXELS_API_KEY",
	"PIXABAY_API_KEY",
	"ANTHROPIC_API_KEY",
}

var processEnvironmentMu sync.Mutex

const (
	sessionTokenHeader = "X-Facet-Session-Token"
	sessionTokenQuery  = "token"
	sseWriteTimeout    = 2 * time.Second
)

type tokenLocation uint8

const (
	noToken tokenLocation = iota
	tokenHeader
	tokenQuery
)

// Server coordinates the Studio web UI, REST API, and CLI session manager.
type Server struct {
	rootDir       string
	mux           *http.ServeMux
	sessions      map[string]*Session
	sessionsMu    sync.Mutex
	environment   []string
	environmentMu sync.RWMutex
	sessionToken  string
}

// NewServer constructs a new Studio Server.
func NewServer(rootDir string) *Server {
	tokenBytes := make([]byte, 32)
	sessionToken := ""
	if _, err := rand.Read(tokenBytes); err == nil {
		sessionToken = base64.RawURLEncoding.EncodeToString(tokenBytes)
	}

	if rootDir == "" {
		rootDir = "."
	}
	absRoot, err := filepath.Abs(rootDir)
	if err == nil {
		rootDir = absRoot
	}

	processEnvironmentMu.Lock()
	environment := append([]string(nil), os.Environ()...)
	processEnvironmentMu.Unlock()

	s := &Server{
		rootDir:      rootDir,
		mux:          http.NewServeMux(),
		sessions:     make(map[string]*Session),
		environment:  environment,
		sessionToken: sessionToken,
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	// Web UI
	s.mux.HandleFunc("GET /", s.handleIndex)

	// Projects API
	s.mux.HandleFunc("GET /api/projects", s.guardRequest(noToken, s.handleListProjects))
	s.mux.HandleFunc("GET /api/projects/{slug}", s.guardRequest(noToken, s.handleGetProject))
	s.mux.HandleFunc("GET /api/engines", s.guardRequest(noToken, s.handleListEngines))
	s.mux.HandleFunc("GET /api/session-token", s.guardRequest(noToken, s.handleSessionToken))

	// Media File Serving
	s.mux.HandleFunc("GET /api/media/{path...}", s.guardRequest(noToken, s.handleServeMedia))

	// CLI Chat / SSE Stream
	s.mux.HandleFunc("GET /api/chat", s.guardRequest(tokenQuery, s.handleChatSSE))

	// CLI Session Close
	s.mux.HandleFunc("POST /api/close", s.guardRequest(tokenHeader, s.handleCloseSession))

	// Config / Environment
	s.mux.HandleFunc("GET /api/config", s.guardRequest(noToken, s.handleGetConfig))
	s.mux.HandleFunc("POST /api/config", s.guardRequest(tokenHeader, s.handlePostConfig))

	// Catalog & Packs API
	s.mux.HandleFunc("GET /api/catalog", s.guardRequest(noToken, s.handleGetCatalog))
	s.mux.HandleFunc("POST /api/catalog/new", s.guardRequest(noToken, s.handleNewProject))
	s.mux.HandleFunc("POST /api/catalog/open", s.guardRequest(noToken, s.handleOpenProject))
	s.mux.HandleFunc("GET /api/packs", s.guardRequest(noToken, s.handleListPacks))
}

func (s *Server) guardRequest(location tokenLocation, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requestIsSameOrigin(r) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		var supplied string
		switch location {
		case tokenHeader:
			supplied = r.Header.Get(sessionTokenHeader)
		case tokenQuery:
			supplied = r.URL.Query().Get(sessionTokenQuery)
		}
		if location != noToken && (s.sessionToken == "" || !constantTimeTokenEqual(supplied, s.sessionToken)) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func requestIsSameOrigin(r *http.Request) bool {
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")), "cross-site") {
		return false
	}

	requestHost, requestPort, ok := splitHTTPHost(r.Host)
	if !ok || !isLoopbackHostname(requestHost) {
		return false
	}

	originValue := strings.TrimSpace(r.Header.Get("Origin"))
	if originValue == "" {
		return true
	}
	origin, err := url.Parse(originValue)
	if err != nil || origin.Scheme == "" || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || origin.Path != "" {
		return false
	}
	originHost, originPort, ok := splitHTTPHost(origin.Host)
	if !ok {
		return false
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return strings.EqualFold(origin.Scheme, scheme) && sameHostname(originHost, requestHost) && effectivePort(origin.Scheme, originPort) == effectivePort(scheme, requestPort)
}

func splitHTTPHost(value string) (string, string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "/\\@") {
		return "", "", false
	}
	if host, port, err := net.SplitHostPort(value); err == nil {
		if host == "" || port == "" {
			return "", "", false
		}
		return strings.Trim(host, "[]"), port, true
	}
	if strings.Contains(value, ":") {
		return "", "", false
	}
	return value, "", true
}

func isLoopbackHostname(host string) bool {
	if strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func sameHostname(a, b string) bool {
	if aIP, bIP := net.ParseIP(a), net.ParseIP(b); aIP != nil && bIP != nil {
		return aIP.Equal(bIP)
	}
	return strings.EqualFold(strings.TrimSuffix(a, "."), strings.TrimSuffix(b, "."))
}

func effectivePort(scheme, port string) string {
	if port != "" {
		return port
	}
	if strings.EqualFold(scheme, "https") {
		return "443"
	}
	return "80"
}

func constantTimeTokenEqual(supplied, expected string) bool {
	suppliedHash := sha256.Sum256([]byte(supplied))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(suppliedHash[:], expectedHash[:]) == 1
}

// Run starts the Studio server on addr for the given root directory and auto-opens browser.
func Run(addr, dir string) error {
	return RunWithOption(addr, dir, true)
}

// RunWithOption starts the Studio server with optional browser opening.
func RunWithOption(addr, dir string, autoOpen bool) error {
	return NewServer(dir).RunWithOption(addr, autoOpen)
}

// Handler returns the underlying http.Handler for testing or mounting.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// Run starts the HTTP server listening on addr with auto-open.
func (s *Server) Run(addr string) error {
	return s.RunWithOption(addr, true)
}

// RunWithOption starts the HTTP server listening on addr or next open port.
func (s *Server) RunWithOption(addr string, autoOpen bool) error {
	host := "127.0.0.1"
	startPort := 8787
	if addr != "" {
		requestedHost, requestedPort, err := net.SplitHostPort(addr)
		if err == nil {
			if port, portErr := strconv.Atoi(requestedPort); portErr == nil {
				startPort = port
			}
			requestedHost = strings.Trim(requestedHost, "[]")
			if strings.EqualFold(requestedHost, "localhost") {
				host = "localhost"
			} else if ip := net.ParseIP(requestedHost); ip != nil && ip.IsLoopback() {
				host = requestedHost
			}
		} else if port, portErr := strconv.Atoi(strings.TrimPrefix(addr, ":")); portErr == nil {
			startPort = port
		}
	}

	var listener net.Listener
	var err error
	var boundPort int

	for port := startPort; port < startPort+20; port++ {
		tryAddr := net.JoinHostPort(host, strconv.Itoa(port))
		listener, err = net.Listen("tcp", tryAddr)
		if err == nil {
			boundPort = port
			break
		}
	}

	if listener == nil {
		return fmt.Errorf("failed to listen on any port between %d and %d: %w", startPort, startPort+20, err)
	}
	defer listener.Close()

	studioURL := "http://" + net.JoinHostPort(host, strconv.Itoa(boundPort))

	fmt.Printf("\n🎬 Facet Studio is running!\n")
	fmt.Printf("👉 URL: %s\n", studioURL)
	fmt.Printf("📁 Workspace: %s\n\n", s.rootDir)

	if autoOpen {
		go func() {
			time.Sleep(300 * time.Millisecond)
			OpenBrowser(studioURL)
		}()
	}

	srv := &http.Server{
		Handler: s.mux,
	}
	return srv.Serve(listener)
}

// OpenBrowser launches the user's default browser to targetURL.
func OpenBrowser(targetURL string) {
	var cmd *exec.Cmd
	if os.PathSeparator == '\\' {
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL)
	} else if os.Getenv("TERM_PROGRAM") == "Apple_Terminal" || strings.Contains(os.Getenv("PATH"), "/Applications") {
		cmd = exec.Command("open", targetURL)
	} else {
		cmd = exec.Command("xdg-open", targetURL)
	}
	_ = cmd.Start()
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	// Prefer index.html from disk if present, else fallback to embedded
	diskPath := filepath.Join(s.rootDir, "web", "index.html")
	if data, err := os.ReadFile(diskPath); err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(web.IndexHTML)
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := ListProjectsWithCatalog(s.rootDir)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, projects)
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "missing project slug"})
		return
	}

	details, err := GetProjectDetails(s.rootDir, slug)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": fmt.Sprintf("project %q not found: %v", slug, err)})
		return
	}
	respondJSON(w, http.StatusOK, details)
}

func (s *Server) handleListEngines(w http.ResponseWriter, r *http.Request) {
	engines := make([]map[string]any, 0, len(engine.ListAdapters()))
	for _, adapter := range engine.ListAdapters() {
		_, err := exec.LookPath(adapter.ExecutableName())
		item := map[string]any{
			"name":         adapter.Name(),
			"display_name": adapter.DisplayName(),
			"available":    err == nil,
		}
		if err != nil {
			item["reason"] = fmt.Sprintf("%s executable not found in PATH", adapter.ExecutableName())
		}
		engines = append(engines, item)
	}
	respondJSON(w, http.StatusOK, map[string]any{"engines": engines})
}

func (s *Server) handleGetCatalog(w http.ResponseWriter, r *http.Request) {
	cat, err := LoadCatalog(s.rootDir)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, cat)
}

func (s *Server) handleNewProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string   `json:"name"`
		Slug      string   `json:"slug"`
		Directory string   `json:"directory"`
		Engine    string   `json:"engine"`
		Packs     []string `json:"packs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body: " + err.Error()})
		return
	}
	proj, err := CreateNewProject(req.Name, req.Slug, req.Directory, req.Engine, req.Packs, s.rootDir)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, proj)
}

func (s *Server) handleOpenProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path   string `json:"path"`
		Engine string `json:"engine"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body: " + err.Error()})
		return
	}
	proj, err := OpenExistingProject(req.Path, req.Engine, s.rootDir)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, proj)
}

func (s *Server) handleListPacks(w http.ResponseWriter, r *http.Request) {
	packs := DiscoverAvailablePacks(s.rootDir)
	respondJSON(w, http.StatusOK, map[string]any{"packs": packs})
}

func (s *Server) handleSessionToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	if s.sessionToken == "" {
		http.Error(w, "Local security unavailable", http.StatusServiceUnavailable)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"token": s.sessionToken})
}

func (s *Server) handleServeMedia(w http.ResponseWriter, r *http.Request) {
	reqPath := r.PathValue("path")
	if reqPath == "" {
		http.NotFound(w, r)
		return
	}

	unescaped, err := url.PathUnescape(reqPath)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	cleaned := filepath.Clean(filepath.FromSlash(unescaped))
	if filepath.IsAbs(cleaned) || filepath.VolumeName(cleaned) != "" || cleaned == "." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	var fullPath string
	var allowed bool

	projectsRoot, err := s.canonicalProjectsRoot()
	if err == nil {
		candidate, cErr := canonicalExistingPath(filepath.Join(s.rootDir, cleaned))
		if cErr == nil && pathStrictlyWithin(projectsRoot, candidate) {
			fullPath = candidate
			allowed = true
		}
	}

	if !allowed {
		if cat, err := LoadCatalog(s.rootDir); err == nil {
			for _, cp := range cat.Projects {
				dirName := filepath.Base(cp.Path)
				if cleaned == dirName || strings.HasPrefix(cleaned, dirName+string(filepath.Separator)) {
					sub := strings.TrimPrefix(cleaned, dirName)
					sub = strings.TrimPrefix(sub, string(filepath.Separator))
					candidate, cErr := canonicalExistingPath(filepath.Join(cp.Path, sub))
					if cErr == nil && (samePath(cp.Path, candidate) || pathStrictlyWithin(cp.Path, candidate)) {
						fullPath = candidate
						allowed = true
						break
					}
				}
			}
		}
	}

	if !allowed || fullPath == "" {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	if strings.EqualFold(filepath.Ext(fullPath), ".mp4") && info.Size() == 0 {
		http.NotFound(w, r)
		return
	}

	allowedExts := map[string]string{
		".mp4":  "video/mp4",
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".aac":  "audio/aac",
		".m4a":  "audio/mp4",
		".ogg":  "audio/ogg",
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".webp": "image/webp",
		".json": "application/json",
		".md":   "text/markdown; charset=utf-8",
		".srt":  "text/plain; charset=utf-8",
		".vtt":  "text/vtt; charset=utf-8",
		".txt":  "text/plain; charset=utf-8",
	}

	ext := strings.ToLower(filepath.Ext(fullPath))
	contentType, allowed := allowedExts[ext]
	if !allowed {
		http.Error(w, "Unsupported media type", http.StatusUnsupportedMediaType)
		return
	}

	w.Header().Set("Content-Type", contentType)
	http.ServeFile(w, r, fullPath)
}

func sse(w http.ResponseWriter, event string, payload any) (returnErr error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode %s SSE payload: %w", event, err)
	}

	controller := http.NewResponseController(w)
	if err := controller.SetWriteDeadline(time.Now().Add(sseWriteTimeout)); err != nil {
		return fmt.Errorf("set %s SSE write deadline: %w", event, err)
	}
	defer func() {
		if err := controller.SetWriteDeadline(time.Time{}); err != nil && returnErr == nil {
			returnErr = fmt.Errorf("clear %s SSE write deadline: %w", event, err)
		}
	}()
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b); err != nil {
		return fmt.Errorf("write %s SSE event: %w", event, err)
	}
	if err := controller.Flush(); err != nil {
		return fmt.Errorf("flush %s SSE event: %w", event, err)
	}
	return nil
}

func (s *Server) getSession(id string) *Session {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	return s.sessions[id]
}

func (s *Server) evictSession(id string, expected *Session) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	if s.sessions[id] == expected {
		delete(s.sessions, id)
	}
}

func registeredAdapter(name string) (engine.EngineAdapter, bool) {
	canonical := strings.ToLower(strings.TrimSpace(name))
	if canonical == "" {
		canonical = "claude"
	}
	for _, adapter := range engine.ListAdapters() {
		if adapter.Name() == canonical {
			return adapter, true
		}
	}
	return nil, false
}

func (s *Server) resolveSessionDir(dir string) (string, error) {
	root, err := canonicalExistingPath(s.rootDir)
	if err != nil {
		return "", fmt.Errorf("resolve Studio root: %w", err)
	}
	if strings.TrimSpace(dir) == "" {
		return root, nil
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	resolved, err := canonicalExistingPath(dir)
	if err != nil {
		return "", fmt.Errorf("resolve requested working directory: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("requested working directory is not a directory")
	}
	if samePath(resolved, root) {
		return resolved, nil
	}
	projects, err := s.canonicalProjectsRoot()
	if err != nil {
		return "", fmt.Errorf("resolve Studio projects directory: %w", err)
	}
	if pathStrictlyWithin(projects, resolved) {
		return resolved, nil
	}
	// Also permit catalog projects
	if cat, err := LoadCatalog(s.rootDir); err == nil {
		for _, cp := range cat.Projects {
			if canonP, err := canonicalExistingPath(cp.Path); err == nil {
				if samePath(canonP, resolved) || pathStrictlyWithin(canonP, resolved) {
					return resolved, nil
				}
			}
		}
	}
	return "", fmt.Errorf("requested working directory must be the Studio root, within %s, or a catalog project", projects)
}

func (s *Server) canonicalProjectsRoot() (string, error) {
	root, err := canonicalExistingPath(s.rootDir)
	if err != nil {
		return "", err
	}
	projects, err := canonicalExistingPath(filepath.Join(root, "projects"))
	if err != nil {
		return "", err
	}
	if !pathStrictlyWithin(root, projects) {
		return "", fmt.Errorf("projects directory resolves outside the Studio root")
	}
	return projects, nil
}

func canonicalExistingPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func samePath(a, b string) bool {
	rel, err := filepath.Rel(a, b)
	return err == nil && rel == "."
}

func pathWithin(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func pathStrictlyWithin(base, target string) bool {
	return !samePath(base, target) && pathWithin(base, target)
}

func (s *Server) newSession(dir, mode, engineName string) (*Session, error) {
	id := fmt.Sprintf("s%d", time.Now().UnixNano())
	if mode == "" {
		mode = "rw"
	} else if mode != "rw" {
		return nil, fmt.Errorf("unsupported session mode %q; Studio engine adapters require autonomous rw mode", mode)
	}
	resolvedDir, err := s.resolveSessionDir(dir)
	if err != nil {
		return nil, err
	}

	adapter, ok := registeredAdapter(engineName)
	if !ok {
		return nil, fmt.Errorf("unknown engine %q; expected a registered engine", engineName)
	}
	engineName = adapter.Name()
	environment := s.environmentSnapshot()
	sess := &Session{
		ID:          id,
		Dir:         resolvedDir,
		Mode:        mode,
		Engine:      engineName,
		adapter:     adapter,
		valid:       true,
		turnGate:    make(chan struct{}, 1),
		environment: environment,
	}
	sess.turnGate <- struct{}{}

	s.sessionsMu.Lock()
	s.sessions[id] = sess
	s.sessionsMu.Unlock()
	return sess, nil
}

func sessionPayload(sess *Session) map[string]any {
	nativeID, dir, mode, engineName, alive := sess.status()
	return map[string]any{
		"id":        sess.ID,
		"native_id": nativeID,
		"dir":       dir,
		"mode":      mode,
		"engine":    engineName,
		"alive":     alive,
	}
}

func sendEnd(w http.ResponseWriter, ok, alive bool, reason string) error {
	payload := map[string]any{"ok": ok, "alive": alive}
	if reason != "" {
		payload["reason"] = reason
	}
	return sse(w, "end", payload)
}

func sendProcessExit(w http.ResponseWriter, sess *Session, reason string) error {
	return sse(w, "process_exit", map[string]any{
		"message": reason,
		"reason":  reason,
		"engine":  sess.Engine,
		"alive":   false,
	})
}

func (s *Server) handleChatSSE(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	prompt := q.Get("prompt")
	if strings.TrimSpace(prompt) == "" {
		http.Error(w, "prompt required", 400)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	if _, ok := w.(http.Flusher); !ok {
		http.Error(w, "no flush", 500)
		return
	}

	sessionID := q.Get("session")
	var sess *Session
	if sessionID != "" {
		sess = s.getSession(sessionID)
		if sess == nil || !sess.IsAlive() {
			if sess != nil {
				s.evictSession(sessionID, sess)
			}
			reason := fmt.Sprintf("session %q is not resumable", sessionID)
			if sse(w, "error", map[string]string{"message": reason}) == nil {
				_ = sendEnd(w, false, false, reason)
			}
			return
		}
	} else {
		dir := q.Get("dir")
		mode := q.Get("mode")
		engineName := q.Get("engine")
		var err error
		sess, err = s.newSession(dir, mode, engineName)
		if err != nil {
			if sse(w, "error", map[string]string{"message": err.Error()}) == nil {
				_ = sendEnd(w, false, false, err.Error())
			}
			return
		}
	}

	if err := sse(w, "session", sessionPayload(sess)); err != nil {
		_ = sess.Close()
		s.evictSession(sess.ID, sess)
		return
	}
	lastNativeID, _, _, _, _ := sess.status()
	result := sess.runTurn(r.Context(), prompt, func(event turnEvent) error {
		if event.normalized.SessionID != "" && event.normalized.SessionID != lastNativeID {
			lastNativeID = event.normalized.SessionID
			if err := sse(w, "session", sessionPayload(sess)); err != nil {
				return err
			}
		}
		return sse(w, "cc", event.payload)
	})
	if !result.ok {
		alive := sess.IsAlive()
		if !alive {
			s.evictSession(sess.ID, sess)
		}
		if result.emitFailed {
			return
		}
		if result.abnormal && !result.canceled {
			if err := sendProcessExit(w, sess, result.reason); err != nil {
				return
			}
		}
		_ = sendEnd(w, false, alive, result.reason)
		return
	}
	if !sess.IsAlive() {
		s.evictSession(sess.ID, sess)
		_ = sendEnd(w, false, false, "session stopped")
		return
	}
	if err := sse(w, "session", sessionPayload(sess)); err != nil {
		_ = sess.Close()
		s.evictSession(sess.ID, sess)
		return
	}
	_ = sendEnd(w, true, true, "")
}

func (s *Server) handleCloseSession(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id := q.Get("session")
	sess := s.getSession(id)
	if sess == nil {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "no such session"})
		return
	}
	if err := sess.Close(); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("close session: %v", err)})
		return
	}
	s.evictSession(id, sess)
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	environment := s.environmentSnapshot()
	config := make(map[string]string, len(configurableEnvironmentKeys))
	for _, key := range configurableEnvironmentKeys {
		value, _ := environmentValue(environment, key)
		config[key] = maskSecret(value)
	}
	respondJSON(w, http.StatusOK, config)
}

func (s *Server) handlePostConfig(w http.ResponseWriter, r *http.Request) {
	var envs map[string]string
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&envs); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	for key := range envs {
		if !configurableEnvironmentKey(key) {
			respondJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("unknown configuration key %q", key)})
			return
		}
	}
	if err := s.applyEnvironmentUpdates(envs); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func configurableEnvironmentKey(key string) bool {
	for _, allowed := range configurableEnvironmentKeys {
		if key == allowed {
			return true
		}
	}
	return false
}

func (s *Server) environmentSnapshot() []string {
	s.environmentMu.RLock()
	defer s.environmentMu.RUnlock()
	return append([]string(nil), s.environment...)
}

type processEnvironmentValue struct {
	key   string
	value string
	set   bool
}

func (s *Server) applyEnvironmentUpdates(updates map[string]string) error {
	s.environmentMu.Lock()
	defer s.environmentMu.Unlock()

	next := append([]string(nil), s.environment...)
	previous := make([]processEnvironmentValue, 0, len(updates))

	processEnvironmentMu.Lock()
	defer processEnvironmentMu.Unlock()
	for _, key := range configurableEnvironmentKeys {
		value, ok := updates[key]
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}

		oldValue, wasSet := os.LookupEnv(key)
		if err := os.Setenv(key, value); err != nil {
			if rollbackErr := restoreProcessEnvironment(previous); rollbackErr != nil {
				return fmt.Errorf("set configuration key %q: %v; rollback failed: %v", key, err, rollbackErr)
			}
			return fmt.Errorf("set configuration key %q: %w", key, err)
		}
		previous = append(previous, processEnvironmentValue{key: key, value: oldValue, set: wasSet})
		next = setEnvironmentValue(next, key, value)
	}

	s.environment = next
	return nil
}

func restoreProcessEnvironment(previous []processEnvironmentValue) error {
	for i := len(previous) - 1; i >= 0; i-- {
		item := previous[i]
		var err error
		if item.set {
			err = os.Setenv(item.key, item.value)
		} else {
			err = os.Unsetenv(item.key)
		}
		if err != nil {
			return fmt.Errorf("restore configuration key %q: %w", item.key, err)
		}
	}
	return nil
}

func setEnvironmentValue(environment []string, key, value string) []string {
	result := make([]string, 0, len(environment)+1)
	replaced := false
	for _, item := range environment {
		itemKey, _, ok := strings.Cut(item, "=")
		if ok && sameEnvironmentKey(itemKey, key) {
			if !replaced {
				result = append(result, key+"="+value)
				replaced = true
			}
			continue
		}
		result = append(result, item)
	}
	if !replaced {
		result = append(result, key+"="+value)
	}
	return result
}

func environmentValue(environment []string, key string) (string, bool) {
	for _, item := range environment {
		itemKey, value, ok := strings.Cut(item, "=")
		if ok && sameEnvironmentKey(itemKey, key) {
			return value, true
		}
	}
	return "", false
}

func sameEnvironmentKey(a, b string) bool {
	if os.PathSeparator == '\\' {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func maskSecret(s string) string {
	if len(s) < 8 {
		return ""
	}
	return s[:4] + "..." + s[len(s)-4:]
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
