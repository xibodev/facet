package studio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	facet "github.com/xibodev/facet"
	"github.com/xibodev/facet-studio/pkg/config"
	"github.com/xibodev/facet/internal/bundle"
	"github.com/xibodev/facet/internal/toolbox"
)

func installTestStudioBundle(t *testing.T, marker string) string {
	t.Helper()
	source := t.TempDir()
	skills := filepath.Join(source, "skills", "facet")
	if err := os.MkdirAll(skills, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skills, "SKILL.md"), []byte("# Facet production contract\n\n"+marker), 0o644); err != nil {
		t.Fatal(err)
	}
	packs := filepath.Join(source, "packs")
	for _, pack := range facet.RetainedPacks() {
		dir := filepath.Join(packs, pack.ID)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+pack.Title), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := t.TempDir()
	if _, err := bundle.Build(bundle.Source{
		SkillsDir: skills, PacksDir: packs, Tools: toolbox.Names(), FacetVersion: "test",
	}, bundle.TargetStudio, out); err != nil {
		t.Fatal(err)
	}
	return out
}

func configureTestModel(t *testing.T, server *Server, endpoint string) string {
	t.Helper()
	marker := "installed-bundle-marker-" + strings.ReplaceAll(t.Name(), "/", "-")
	server.bundleDir = installTestStudioBundle(t, marker)
	home := t.TempDir()
	t.Setenv(config.EnvHome, home)
	server.modelConfigPath = filepath.Join(home, "config.json")
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.ModelName = "fixture/model"
	cfg.Agents.Defaults.MaxLLMRetries = 0
	cfg.ModelList = []*config.ModelConfig{{ModelName: "fixture/model", Provider: "openai", Model: "model", APIBase: endpoint, Enabled: true}}
	if err := config.SaveConfig(server.modelConfigPath, cfg); err != nil {
		t.Fatal(err)
	}
	return marker
}

func TestNativeConversationVisibleContentAndGuidance(t *testing.T) {
	var prompts []string
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		for _, m := range input.Messages {
			prompts = append(prompts, m.Content)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Visible Facet response\"},\"finish_reason\":null}]}\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"))
	}))
	defer endpoint.Close()
	server := NewServer(t.TempDir())
	marker := configureTestModel(t, server, endpoint.URL)
	sess, err := server.newSession("", "rw", "studio")
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	var events []map[string]any
	result := sess.runTurn(context.Background(), "Hello", func(event turnEvent) error { events = append(events, event.payload); return nil })
	if !result.ok {
		t.Fatal(result.reason)
	}
	found := false
	for _, e := range events {
		if e["content"] == "Visible Facet response" {
			found = true
		}
	}
	if !found {
		t.Fatalf("response not rendered: %#v", events)
	}
	joined := strings.Join(prompts, "\n")
	if !strings.Contains(joined, marker) || !strings.Contains(joined, "facet_guidance") {
		t.Fatal("canonical capability guidance missing from actual model request")
	}
}

func TestMissingBundleNeverCreatesAnAgentRuntime(t *testing.T) {
	server := NewServer(t.TempDir())
	server.bundleDir = filepath.Join(t.TempDir(), "missing")
	home := t.TempDir()
	t.Setenv(config.EnvHome, home)
	server.modelConfigPath = filepath.Join(home, "config.json")
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.ModelName = "fixture/model"
	cfg.ModelList = []*config.ModelConfig{{ModelName: "fixture/model", Provider: "openai", Model: "model", APIBase: "http://127.0.0.1:1", Enabled: true}}
	if err := config.SaveConfig(server.modelConfigPath, cfg); err != nil {
		t.Fatal(err)
	}
	loop, b, err := server.buildRuntime(server.rootDir)
	if err == nil || loop != nil || b != nil {
		t.Fatal("missing capability bundle was reported as a ready runtime")
	}
	if !strings.Contains(err.Error(), "Facet bundle") {
		t.Fatalf("missing bundle error is not actionable: %v", err)
	}
}

func TestCapabilityStatusReportsVerifiedBundle(t *testing.T) {
	server := NewServer(t.TempDir())
	server.bundleDir = installTestStudioBundle(t, "capability-status")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, newSecurityRequest(http.MethodGet, "/api/capability", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var result struct {
		Ready         bool   `json:"ready"`
		CapabilityID  string `json:"capability_id"`
		FacetVersion  string `json:"facet_version"`
		BundleDigest  string `json:"bundle_digest"`
		ToolCount     int    `json:"tool_count"`
		NativeBinding string `json:"native_binding"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Ready || result.CapabilityID != bundle.CapabilityID || result.FacetVersion != "test" ||
		!strings.HasPrefix(result.BundleDigest, "sha256:") || result.ToolCount != len(toolbox.Names()) ||
		result.NativeBinding != "facet-native" {
		t.Fatalf("unexpected capability status: %#v", result)
	}
}

func TestCapabilityStatusExplainsMissingBundle(t *testing.T) {
	server := NewServer(t.TempDir())
	server.bundleDir = filepath.Join(t.TempDir(), "missing")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, newSecurityRequest(http.MethodGet, "/api/capability", ""))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "not installed or compatible") {
		t.Fatalf("missing bundle status is not actionable: %s", rec.Body.String())
	}
}

func TestCapabilityStatusRejectsMismatchedToolVocabulary(t *testing.T) {
	server := NewServer(t.TempDir())
	server.bundleDir = installTestStudioBundle(t, "tool-mismatch")
	path := filepath.Join(server.bundleDir, "facet-bundle.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest bundle.Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Tools = manifest.Tools[:len(manifest.Tools)-1]
	raw, err = json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, newSecurityRequest(http.MethodGet, "/api/capability", ""))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "tool vocabulary") {
		t.Fatalf("mismatched vocabulary status = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMissingModelNeverCreatesAnUnusableLoop(t *testing.T) {
	server := NewServer(t.TempDir())
	server.bundleDir = installTestStudioBundle(t, "missing-model-bundle")
	server.modelConfigPath = filepath.Join(t.TempDir(), "config.json")
	loop, b, err := server.buildRuntime(server.rootDir)
	if err == nil || loop != nil || b != nil {
		t.Fatal("missing configuration was reported as a ready runtime")
	}
}

func TestStandaloneRejectsExternalRuntimeSelection(t *testing.T) {
	s := NewServer(t.TempDir())
	s.bundleDir = installTestStudioBundle(t, "external-runtime-selection")
	s.modelConfigPath = filepath.Join(t.TempDir(), "config.json")
	request := newSecurityRequest("GET", "/api/chat?prompt=hello&engine=claude", "")
	query := request.URL.Query()
	query.Set(sessionTokenQuery, s.sessionToken)
	request.URL.RawQuery = query.Encode()
	rec := newDeadlineResponseRecorder()
	s.Handler().ServeHTTP(rec, request)
	if !strings.Contains(rec.Body.String(), "choose a model in Settings") {
		t.Fatalf("external CLI was selected instead of native runtime: %s", rec.Body.String())
	}
	if len(s.sessions) != 0 {
		t.Fatal("unconfigured runtime created a session")
	}
}

func TestModelTestRejectsHTTPFailure(t *testing.T) {
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "not found", 404) }))
	defer endpoint.Close()
	server := NewServer(t.TempDir())
	configureTestModel(t, server, endpoint.URL)
	req := newSecurityRequest("POST", "/api/models/test", `{"model":"fixture/model"}`)
	req.Header.Set(sessionTokenHeader, server.sessionToken)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	if result.OK || result.Error == "" {
		t.Fatalf("HTTP failure reported as healthy: %s", rec.Body.String())
	}
}

func TestNativeProjectConversationSurvivesRuntimeRestart(t *testing.T) {
	var history [][]string
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&input)
		var texts []string
		for _, m := range input.Messages {
			texts = append(texts, m.Content)
		}
		history = append(history, texts)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Remembered\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n"))
	}))
	defer endpoint.Close()
	server := NewServer(t.TempDir())
	configureTestModel(t, server, endpoint.URL)
	first, err := server.newSession("", "rw", "studio")
	if err != nil {
		t.Fatal(err)
	}
	if result := first.runTurn(context.Background(), "My project color is blue", func(turnEvent) error { return nil }); !result.ok {
		t.Fatal(result.reason)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := server.newSession("", "rw", "studio")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if result := second.runTurn(context.Background(), "What was my color?", func(turnEvent) error { return nil }); !result.ok {
		t.Fatal(result.reason)
	}
	if len(history) != 2 || !strings.Contains(strings.Join(history[1], "\n"), "My project color is blue") {
		t.Fatal("kernel history lost on runtime restart")
	}
}

func TestNativePresentationDoesNotDependOnExternalCLIEventModel(t *testing.T) {
	raw, err := os.ReadFile("native.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if strings.Contains(body, "internal/studio/engine") || strings.Contains(body, "engine.NormalizedEvent") {
		t.Fatal("native kernel presentation still depends on the transitional external-CLI event model")
	}
}
