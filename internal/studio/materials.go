package studio

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/xibodev/facet-studio/pkg/fileutil"
	"github.com/xibodev/facet/internal/toolbox"
)

func (s *Server) materialPath(dir, name string) (string, error) {
	root, err := s.resolveSessionDir(dir)
	if err != nil {
		return "", err
	}
	if !filepath.IsLocal(name) {
		return "", fmt.Errorf("choose a project-relative file")
	}
	full, err := filepath.EvalSymlinks(filepath.Join(root, name))
	if err != nil {
		return "", err
	}
	if !pathStrictlyWithin(root, full) {
		return "", fmt.Errorf("file leaves the project")
	}
	return full, nil
}

func (s *Server) reviewDirectory(dir string) (string, error) {
	root, err := s.resolveSessionDir(dir)
	if err != nil {
		return "", err
	}
	review := filepath.Join(root, "review")
	if err := os.MkdirAll(review, 0755); err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(review)
	if err != nil {
		return "", err
	}
	if !pathStrictlyWithin(root, resolved) {
		return "", fmt.Errorf("review directory leaves the project")
	}
	return resolved, nil
}

func (s *Server) handleSaveMaterial(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Dir      string `json:"dir"`
		Path     string `json:"path"`
		Content  string `json:"content"`
		Original string `json:"original"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondJSON(w, 400, map[string]any{"error": "invalid document"})
		return
	}
	name, err := s.materialPath(request.Dir, request.Path)
	if err != nil {
		respondJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	ext := strings.ToLower(filepath.Ext(name))
	if !containsExt([]string{".md", ".json", ".srt", ".vtt", ".txt"}, ext) {
		respondJSON(w, 400, map[string]any{"error": "only production text documents can be edited"})
		return
	}
	if ext == ".json" && !json.Valid([]byte(request.Content)) {
		respondJSON(w, 400, map[string]any{"error": "JSON is not valid; no changes saved."})
		return
	}
	current, err := os.ReadFile(name)
	if err != nil {
		respondJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	if string(current) != request.Original {
		respondJSON(w, 409, map[string]any{"error": "This file changed since you opened it. Reopen it before saving."})
		return
	}
	if err := fileutil.WriteFileAtomic(name, []byte(request.Content), 0644); err != nil {
		respondJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleReviewOutput(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Dir      string  `json:"dir"`
		Path     string  `json:"path"`
		Width    int     `json:"width"`
		Height   int     `json:"height"`
		FPS      float64 `json:"fps"`
		Duration float64 `json:"duration"`
		Audio    bool    `json:"audio"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondJSON(w, 400, map[string]any{"error": "invalid review request"})
		return
	}
	name, err := s.materialPath(request.Dir, request.Path)
	if err != nil {
		respondJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if request.Width <= 0 || request.Height <= 0 || request.FPS <= 0 || request.Duration <= 0 {
		respondJSON(w, 400, map[string]any{"error": "Enter the expected dimensions, frame rate and duration."})
		return
	}
	reviewDir, err := s.reviewDirectory(request.Dir)
	if err != nil {
		respondJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	input := map[string]any{"rendered_file": name, "profile": map[string]any{"width": request.Width, "height": request.Height, "fps": request.FPS}, "checks": map[string]any{"duration": map[string]any{"expected": request.Duration, "tolerance": 0.1}, "video_codec": "h264", "audio": map[string]any{"required": request.Audio, "codec": "aac", "sample_rate": 48000, "channels": 2}}, "evidence_dir": reviewDir}
	data, _ := json.Marshal(input)
	result := toolbox.RunContext(r.Context(), "output_review", data)
	if !result.OK {
		respondJSON(w, 422, result)
		return
	}
	encoded, _ := json.MarshalIndent(result, "", "  ")
	if err := os.MkdirAll(reviewDir, 0755); err != nil {
		respondJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	if err := fileutil.WriteFileAtomic(filepath.Join(reviewDir, "report.json"), encoded, 0644); err != nil {
		respondJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, 200, result)
}

func (s *Server) handleReviewDecision(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Dir      string `json:"dir"`
		Path     string `json:"path"`
		Decision string `json:"decision"`
		Note     string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondJSON(w, 400, map[string]any{"error": "invalid review decision"})
		return
	}
	decision := strings.ToLower(strings.TrimSpace(request.Decision))
	if decision == "" {
		decision = "accept"
	}
	switch decision {
	case "accept", "accepted":
		decision = "accepted"
	case "reject", "rejected":
		decision = "rejected"
	default:
		respondJSON(w, 400, map[string]any{"error": "review decision must be accept or reject"})
		return
	}
	name, err := s.materialPath(request.Dir, request.Path)
	if err != nil {
		respondJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	file, err := os.Open(name)
	if err != nil {
		respondJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		respondJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	approved := decision == "accepted"
	record := map[string]any{"decision": decision, "human_approved": approved, "artifact": request.Path, "sha256": hex.EncodeToString(digest.Sum(nil)), "note": request.Note, "decided_by": "user via Facet review view"}
	encoded, _ := json.MarshalIndent(record, "", "  ")
	dir, err := s.reviewDirectory(request.Dir)
	if err != nil {
		respondJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	acceptancePath := filepath.Join(dir, "acceptance.json")
	if !approved {
		if err := os.Remove(acceptancePath); err != nil && !os.IsNotExist(err) {
			respondJSON(w, 500, map[string]any{"error": err.Error()})
			return
		}
	}
	if err := fileutil.WriteFileAtomic(filepath.Join(dir, "decision.json"), encoded, 0644); err != nil {
		respondJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	if approved {
		if err := fileutil.WriteFileAtomic(acceptancePath, encoded, 0644); err != nil {
			respondJSON(w, 500, map[string]any{"error": err.Error()})
			return
		}
	}
	respondJSON(w, 200, map[string]any{"ok": true, "decision": decision, "message": "Your review decision is recorded against this exact file digest."})
}

func (s *Server) handleRenderMaterial(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Dir  string `json:"dir"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondJSON(w, 400, map[string]any{"error": "invalid composition selection"})
		return
	}
	name, err := s.materialPath(request.Dir, request.Path)
	if err != nil {
		respondJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if !strings.EqualFold(filepath.Ext(name), ".json") {
		respondJSON(w, 400, map[string]any{"error": "select a saved JSON composition"})
		return
	}
	data, err := os.ReadFile(name)
	if err != nil {
		respondJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	var props map[string]any
	if json.Unmarshal(data, &props) != nil {
		respondJSON(w, 400, map[string]any{"error": "invalid composition JSON"})
		return
	}
	if props["cuts"] == nil && props["scenes"] == nil && props["edit_decisions"] == nil {
		respondJSON(w, 400, map[string]any{"error": "this JSON file is not a composition"})
		return
	}
	args, _ := json.Marshal(map[string]any{"input_path": name})
	result := toolbox.RunContext(r.Context(), "video_compose", args)
	status := 200
	if !result.OK {
		status = 422
	}
	respondJSON(w, status, result)
}
