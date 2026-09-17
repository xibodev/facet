package studio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xibodev/facet-studio/pkg/config"
	"github.com/xibodev/facet/internal/studio/engine"
)

func configureTestModel(t *testing.T, server *Server, endpoint string) {
	t.Helper()
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
	configureTestModel(t, server, endpoint.URL)
	sess, err := server.newSession("", "rw", "studio")
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	var events []*engine.NormalizedEvent
	result := sess.runTurn(context.Background(), "Hello", func(event turnEvent) error { events = append(events, event.normalized); return nil })
	if !result.ok {
		t.Fatal(result.reason)
	}
	found := false
	for _, e := range events {
		if e.Content == "Visible Facet response" {
			found = true
		}
	}
	if !found {
		t.Fatalf("response not rendered: %#v", events)
	}
	joined := strings.Join(prompts, "\n")
	if !strings.Contains(joined, "Facet Video Producer") || !strings.Contains(joined, "facet_guidance") {
		t.Fatal("canonical capability guidance missing from actual model request")
	}
}

func TestMissingModelNeverCreatesAnUnusableLoop(t *testing.T) {
	server := NewServer(t.TempDir())
	server.modelConfigPath = filepath.Join(t.TempDir(), "config.json")
	loop, b, err := server.buildRuntime(server.rootDir)
	if err == nil || loop != nil || b != nil {
		t.Fatal("missing configuration was reported as a ready runtime")
	}
}

func TestStandaloneRejectsExternalRuntimeSelection(t *testing.T) {
	s := NewServer(t.TempDir())
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
