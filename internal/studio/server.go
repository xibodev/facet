package studio

import (
	"bufio"
	"encoding/json"
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/xibodev/facet/internal/studio/engine"
	"github.com/xibodev/facet/web"
)

// Server coordinates the Studio web UI, REST API, and CLI session manager.
type Server struct {
	rootDir    string
	mux        *http.ServeMux
	sessions   map[string]*Session
	sessionsMu sync.Mutex
}

// NewServer constructs a new Studio Server.
func NewServer(rootDir string) *Server {
	if rootDir == "" {
		rootDir = "."
	}
	absRoot, err := filepath.Abs(rootDir)
	if err == nil {
		rootDir = absRoot
	}

	s := &Server{
		rootDir:  rootDir,
		mux:      http.NewServeMux(),
		sessions: make(map[string]*Session),
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	// Web UI
	s.mux.HandleFunc("GET /", s.handleIndex)

	// Projects API
	s.mux.HandleFunc("GET /api/projects", s.handleListProjects)
	s.mux.HandleFunc("GET /api/projects/{slug}", s.handleGetProject)

	// Media File Serving
	s.mux.HandleFunc("GET /api/media/{path...}", s.handleServeMedia)

	// CLI Chat / SSE Stream
	s.mux.HandleFunc("GET /api/chat", s.handleChatSSE)

	// CLI Session Close
	s.mux.HandleFunc("GET /api/close", s.handleCloseSession)
	s.mux.HandleFunc("POST /api/close", s.handleCloseSession)

	// Config / Environment
	s.mux.HandleFunc("GET /api/config", s.handleGetConfig)
	s.mux.HandleFunc("POST /api/config", s.handlePostConfig)
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
	host := "localhost"
	startPort := 8787
	if addr != "" {
		if strings.Contains(addr, ":") {
			parts := strings.Split(addr, ":")
			if parts[0] != "" && parts[0] != "0.0.0.0" && parts[0] != "[::]" {
				host = parts[0]
			}
			if p, err := fmt.Sscanf(parts[1], "%d", &startPort); err == nil && p == 1 {
				// startPort parsed
			}
		}
	}

	var listener net.Listener
	var err error
	var boundPort int

	for port := startPort; port < startPort+20; port++ {
		tryAddr := fmt.Sprintf(":%d", port)
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

	studioURL := fmt.Sprintf("http://%s:%d", host, boundPort)

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
	projects, err := ListProjects(s.rootDir)
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
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, filepath.FromSlash("/../")) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	fullPath := filepath.Join(s.rootDir, cleaned)
	rel, err := filepath.Rel(s.rootDir, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
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
		contentType = mime.TypeByExtension(ext)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}

	w.Header().Set("Content-Type", contentType)
	http.ServeFile(w, r, fullPath)
}

func sse(w http.ResponseWriter, fl http.Flusher, event string, payload any) {
	b, _ := json.Marshal(payload)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
	fl.Flush()
}

func (s *Server) getSession(id string) *Session {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	return s.sessions[id]
}

func (s *Server) newSession(dir, mode, engineName string) (*Session, error) {
	id := fmt.Sprintf("s%d", time.Now().UnixNano())
	if dir == "" {
		dir = s.rootDir
	} else {
		abs, err := filepath.Abs(dir)
		if err == nil {
			dir = abs
		}
	}
	if engineName == "" {
		engineName = "claude"
	}
	switch mode {
	case "rw", "ask", "ro":
	default:
		mode = "rw"
	}

	adapter := engine.GetAdapter(engineName)
	sess := &Session{
		ID:      id,
		Dir:     dir,
		Mode:    mode,
		Engine:  engineName,
		adapter: adapter,
		events:  make(chan map[string]any, 512),
	}

	args, err := adapter.BuildArgs(dir, mode, nil)
	if err != nil {
		return nil, fmt.Errorf("build args for engine (%s): %w", engineName, err)
	}

	execName := adapter.ExecutableName()
	if engineName != "" && engineName != adapter.Name() && !strings.Contains(adapter.Name(), engineName) {
		execName = engineName
	}

	cmd := exec.Command(execName, args...)
	cmd.Dir = dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start engine (%s): %w", engineName, err)
	}

	sess.cmd = cmd
	sess.stdin = stdin
	sess.alive = true

	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 1024*1024), 32*1024*1024)
		for sc.Scan() {
			line := sc.Bytes()
			if len(line) == 0 || line[0] != '{' {
				continue
			}
			var ev map[string]any
			if json.Unmarshal(line, &ev) != nil {
				continue
			}
			if ev["type"] == "system" && ev["subtype"] == "init" {
				if cid, ok := ev["session_id"].(string); ok && cid != "" {
					sess.mu.Lock()
					sess.ClaudeID = cid
					sess.mu.Unlock()
				}
			} else if ev["type"] == "session" || ev["type"] == "conversation.create" || ev["type"] == "init" {
				if cid, ok := ev["session_id"].(string); ok && cid != "" {
					sess.mu.Lock()
					sess.ClaudeID = cid
					sess.mu.Unlock()
				} else if cid, ok := ev["conversation_id"].(string); ok && cid != "" {
					sess.mu.Lock()
					sess.ClaudeID = cid
					sess.mu.Unlock()
				}
			}
			select {
			case sess.events <- ev:
			default:
			}
		}
		sess.mu.Lock()
		sess.alive = false
		sess.mu.Unlock()
		close(sess.events)
	}()

	s.sessionsMu.Lock()
	s.sessions[id] = sess
	s.sessionsMu.Unlock()
	return sess, nil
}

func (s *Server) handleChatSSE(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	prompt := q.Get("prompt")
	if prompt == "" {
		http.Error(w, "prompt required", 400)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no flush", 500)
		return
	}

	sessionID := q.Get("session")
	sess := s.getSession(sessionID)
	if sess == nil {
		dir := q.Get("dir")
		mode := q.Get("mode")
		engine := q.Get("engine")
		var err error
		sess, err = s.newSession(dir, mode, engine)
		if err != nil {
			sse(w, fl, "error", map[string]string{"message": err.Error()})
			sse(w, fl, "end", map[string]string{"ok": "0"})
			return
		}
	}

	sess.turnMu.Lock()
	defer sess.turnMu.Unlock()

	sess.mu.Lock()
	claudeID, dir, mode := sess.ClaudeID, sess.Dir, sess.Mode
	sess.mu.Unlock()
	sse(w, fl, "session", map[string]string{
		"id": sess.ID, "claude_id": claudeID, "dir": dir, "mode": mode,
	})

	if err := sess.send(prompt); err != nil {
		sse(w, fl, "error", map[string]string{"message": err.Error()})
		sse(w, fl, "end", map[string]string{"ok": "0"})
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-sess.events:
			if !ok {
				sse(w, fl, "end", map[string]string{"ok": "0"})
				return
			}
			if ev["type"] == "system" && ev["subtype"] == "init" {
				if id, _ := ev["session_id"].(string); id != "" {
					sess.mu.Lock()
					sess.ClaudeID = id
					sess.mu.Unlock()
					sse(w, fl, "session", map[string]string{
						"id": sess.ID, "claude_id": id, "dir": dir, "mode": mode,
					})
				}
			} else if ev["type"] == "session" || ev["type"] == "conversation.create" {
				var id string
				if sID, ok := ev["session_id"].(string); ok && sID != "" {
					id = sID
				} else if sID, ok := ev["conversation_id"].(string); ok && sID != "" {
					id = sID
				}
				if id != "" {
					sess.mu.Lock()
					sess.ClaudeID = id
					sess.mu.Unlock()
					sse(w, fl, "session", map[string]string{
						"id": sess.ID, "claude_id": id, "dir": dir, "mode": mode,
					})
				}
			}
			sse(w, fl, "cc", ev)
			if ev["type"] == "result" || ev["type"] == "done" || ev["type"] == "finish" || ev["type"] == "turn.finish" {
				sse(w, fl, "end", map[string]string{"ok": "1"})
				return
			}
		}
	}
}

func (s *Server) handleCloseSession(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id := q.Get("session")
	sess := s.getSession(id)
	if sess == nil {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "no such session"})
		return
	}
	_ = sess.Close()
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{
		"OPENAI_API_KEY":     maskSecret(os.Getenv("OPENAI_API_KEY")),
		"ELEVENLABS_API_KEY": maskSecret(os.Getenv("ELEVENLABS_API_KEY")),
		"FAL_KEY":            maskSecret(os.Getenv("FAL_KEY")),
		"PEXELS_API_KEY":     maskSecret(os.Getenv("PEXELS_API_KEY")),
		"PIXABAY_API_KEY":    maskSecret(os.Getenv("PIXABAY_API_KEY")),
		"ANTHROPIC_API_KEY":  maskSecret(os.Getenv("ANTHROPIC_API_KEY")),
	})
}

func (s *Server) handlePostConfig(w http.ResponseWriter, r *http.Request) {
	var envs map[string]string
	if err := json.NewDecoder(r.Body).Decode(&envs); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	for k, v := range envs {
		if strings.TrimSpace(v) != "" {
			_ = os.Setenv(k, v)
		}
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
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
