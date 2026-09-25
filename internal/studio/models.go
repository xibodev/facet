package studio

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/xibodev/facet-studio/pkg/agent"
	"github.com/xibodev/facet-studio/pkg/auth"
	"github.com/xibodev/facet-studio/pkg/bus"
	"github.com/xibodev/facet-studio/pkg/config"
	"github.com/xibodev/facet-studio/pkg/modelservice"
	"github.com/xibodev/facet-studio/pkg/providers"
	"github.com/xibodev/facet/internal/bundle"
	"github.com/xibodev/facet/internal/toolbox"
	"github.com/xibodev/facet/pkg/provider"
)

func applicationHome() string {
	if home := os.Getenv("FACET_HOME"); home != "" {
		return home
	}
	home, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".facet", "runtime")
	}
	return filepath.Join(home, "Facet", "runtime")
}

// InitializeApplication is called once by the standalone composition root.
// Kernel config, catalogs and auth all agree on Facet's home; full Studio state
// is neither read nor modified implicitly.
func InitializeApplication() error {
	home, err := filepath.Abs(applicationHome())
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		return err
	}
	return os.Setenv(config.EnvHome, home)
}

func (s *Server) loadModels() (*config.Config, error) {
	if _, err := os.Stat(s.modelConfigPath); os.IsNotExist(err) {
		cfg := config.DefaultConfig()
		cfg.ModelList = nil // upstream templates are examples, not configured accounts
		cfg.Agents.Defaults.ModelName = ""
		return cfg, nil
	}
	return config.LoadConfig(s.modelConfigPath)
}

func modelResolution(cfg *config.Config, selection string) (*providers.InstanceResolution, error) {
	store, err := modelservice.LoadCatalogs()
	if err != nil {
		return nil, err
	}
	catalogs := map[string]providers.InstanceCatalog{}
	for _, entry := range store.Entries {
		if entry == nil {
			continue
		}
		c := providers.InstanceCatalog{InstanceID: entry.InstanceID}
		for _, model := range entry.Models {
			c.Models = append(c.Models, model.ID)
		}
		catalogs[c.InstanceID] = c
	}
	return providers.ResolveInstanceTargetOrRoute(cfg, catalogs, selection, modelservice.ResolveProviderCredentialReference, nil)
}

func selectedProvider(cfg *config.Config, selection string) (providers.LLMProvider, *config.ModelConfig, error) {
	if selection == "" {
		return nil, nil, fmt.Errorf("choose a model in Settings; discover free models or connect a provider")
	}
	// Exact instance models use the kernel's catalog-backed resolver.
	if target, err := config.ParseExactModelTarget(selection); err == nil {
		for _, instance := range cfg.ProviderInstances {
			if instance == nil || instance.ID != target.InstanceID {
				continue
			}
			resolved, err := modelResolution(cfg, selection)
			if err != nil {
				return nil, nil, err
			}
			candidate := resolved.Candidates[0]
			llm, err := resolved.ProviderForCandidate(candidate)
			if err != nil {
				return nil, nil, err
			}
			mc, err := resolved.ModelConfigForCandidate(candidate)
			if err == nil && instance.AuthConnectionRef != "" {
				secret, secretErr := modelservice.ResolveProviderCredentialReference(instance.AuthConnectionRef)
				if secretErr != nil {
					return nil, nil, secretErr
				}
				mc.SetAPIKey(secret)
			}
			return llm, mc, err
		}
	}
	for _, mc := range cfg.ModelList {
		if mc != nil && mc.ModelName == selection && mc.Enabled {
			llm, _, err := providers.CreateProviderFromConfig(mc)
			return llm, mc, err
		}
	}
	return nil, nil, fmt.Errorf("selected model %q is not configured; refresh its provider catalog in Settings", selection)
}

func (s *Server) buildRuntime(workspace string) (*agent.AgentLoop, *bus.MessageBus, error) {
	manifest, err := s.loadCapability()
	if err != nil {
		return nil, nil, err
	}
	s.modelMu.Lock()
	cfg, err := s.loadModels()
	s.modelMu.Unlock()
	if err != nil {
		return nil, nil, fmt.Errorf("load Facet settings: %w", err)
	}
	llm, mc, err := selectedProvider(cfg, cfg.Agents.Defaults.ModelName)
	if err != nil {
		return nil, nil, err
	}
	// Runtime projection, not a second provider catalog. Preserve exact selection.
	copyModel := *mc
	copyModel.ModelName = cfg.Agents.Defaults.ModelName
	copyModel.Streaming.Enabled = true
	if copyModel.RequestTimeout == 0 {
		copyModel.RequestTimeout = 60
	}
	cfg.ModelList = []*config.ModelConfig{&copyModel}
	cfg.Agents.List = nil
	cfg.Agents.Defaults.Workspace = workspace
	channel := &config.Channel{Type: "facet", Enabled: true, Settings: config.RawNode(`{"streaming":{"enabled":true}}`)}
	if err := channel.Decode(&struct {
		Streaming config.StreamingConfig `json:"streaming"`
	}{}); err != nil {
		return nil, nil, err
	}
	cfg.Channels = config.ChannelsConfig{"facet": channel}
	entries := make([]string, 0, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		entries = append(entries, entry.Path)
	}
	msgBus := bus.NewMessageBus()
	loop := agent.NewAgentLoop(cfg, msgBus, llm, agent.WithToolProviders(provider.NewBundleToolProvider(s.bundleDir, entries...)))
	if err := provider.MountBundleGuidance(loop, s.bundleDir, entries...); err != nil {
		loop.Close()
		msgBus.Close()
		return nil, nil, err
	}
	return loop, msgBus, nil
}

func (s *Server) loadCapability() (*bundle.Manifest, error) {
	manifest, err := bundle.VerifyTarget(s.bundleDir, bundle.TargetStudio)
	if err != nil {
		return nil, fmt.Errorf(
			"Facet bundle at %s is not installed or compatible: %w",
			s.bundleDir,
			err,
		)
	}
	if err := verifyBundleTools(manifest.Tools); err != nil {
		return nil, err
	}
	return manifest, nil
}

func verifyBundleTools(got []string) error {
	want := toolbox.Names()
	got = append([]string(nil), got...)
	sort.Strings(got)
	if len(got) != len(want) {
		return fmt.Errorf("Facet bundle tool vocabulary has %d entries, runtime has %d; reinstall the matching bundle", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			return fmt.Errorf("Facet bundle tool vocabulary does not match this runtime; reinstall the matching bundle")
		}
	}
	return nil
}

func (s *Server) handleCapabilityStatus(w http.ResponseWriter, _ *http.Request) {
	manifest, err := s.loadCapability()
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]any{
			"ready": false,
			"error": err.Error(),
		})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"ready":          true,
		"capability_id":  manifest.CapabilityID,
		"facet_version":  manifest.FacetVersion,
		"bundle_digest":  manifest.BundleDigest,
		"tool_count":     len(manifest.Tools),
		"native_binding": manifest.Compatibility.NativeProvider,
	})
}

type modelOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Server) handleGetModels(w http.ResponseWriter, r *http.Request) {
	s.modelMu.Lock()
	defer s.modelMu.Unlock()
	cfg, err := s.loadModels()
	if err != nil {
		respondJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	options := []modelOption{}
	seen := map[string]bool{}
	add := func(id string) {
		if !seen[id] {
			options = append(options, modelOption{ID: id, Name: id})
			seen[id] = true
		}
	}
	store, err := modelservice.LoadCatalogs()
	if err != nil {
		respondJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	for _, instance := range cfg.ProviderInstances {
		if instance == nil || instance.State != config.ProviderInstanceStateEnabled {
			continue
		}
		if catalog := store.Entries[instance.ID]; catalog != nil {
			for _, model := range catalog.Models {
				add(instance.ID + "/" + model.ID)
			}
		}
	}
	for _, mc := range cfg.ModelList {
		if mc != nil && mc.Enabled {
			add(mc.ModelName)
		}
	}
	sort.Slice(options, func(i, j int) bool { return options[i].ID < options[j].ID })
	respondJSON(w, 200, map[string]any{"active_model": cfg.Agents.Defaults.ModelName, "models": options, "providers": modelservice.ListRoster(cfg), "configured": seen[cfg.Agents.Defaults.ModelName], "config_path": s.modelConfigPath})
}

func (s *Server) handleSelectModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]any{"error": "invalid model selection"})
		return
	}
	s.modelMu.Lock()
	defer s.modelMu.Unlock()
	cfg, err := s.loadModels()
	if err == nil {
		var llm providers.LLMProvider
		llm, _, err = selectedProvider(cfg, strings.TrimSpace(req.Model))
		if closer, ok := llm.(interface{ Close() }); ok {
			closer.Close()
		}
	}
	if err != nil {
		respondJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	cfg.Agents.Defaults.ModelName = strings.TrimSpace(req.Model)
	if err := config.SaveConfig(s.modelConfigPath, cfg); err != nil {
		respondJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, 200, map[string]any{"ok": true, "active_model": req.Model, "message": "Saved. Start a new conversation to use this model; existing conversations keep their model."})
}

func (s *Server) handleDiscoverModels(w http.ResponseWriter, r *http.Request) {
	s.modelMu.Lock()
	defer s.modelMu.Unlock()
	cfg, err := s.loadModels()
	if err != nil {
		respondJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := modelservice.AutoConnectFree(ctx, cfg, &http.Client{Timeout: 8 * time.Second}, nil)
	if err == nil {
		err = config.SaveConfig(s.modelConfigPath, cfg)
	}
	if err != nil {
		respondJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, 200, map[string]any{"ok": true, "discovery": result, "message": "Catalog discovery finished. Select and test a model; discovery alone does not verify a conversation or tool use."})
}

func (s *Server) handleTestModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]any{"error": "model is required"})
		return
	}
	s.modelMu.Lock()
	cfg, err := s.loadModels()
	s.modelMu.Unlock()
	var llm providers.LLMProvider
	var mc *config.ModelConfig
	if err == nil {
		llm, mc, err = selectedProvider(cfg, req.Model)
	}
	if err != nil {
		respondJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if closer, ok := llm.(interface{ Close() }); ok {
		defer closer.Close()
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	start := time.Now()
	response, err := llm.Chat(ctx, []providers.Message{{Role: "user", Content: "Reply with OK."}}, nil, mc.Model, map[string]any{"max_tokens": 256})
	if err == nil && (response == nil || strings.TrimSpace(response.Content) == "") {
		err = fmt.Errorf("model returned no visible response")
	}
	if err != nil {
		respondJSON(w, 200, map[string]any{"ok": false, "error": err.Error(), "model": req.Model, "latency_ms": time.Since(start).Milliseconds()})
		return
	}
	respondJSON(w, 200, map[string]any{"ok": true, "model": req.Model, "latency_ms": time.Since(start).Milliseconds(), "message": "Model replied successfully. Video tool execution has not been tested by this check."})
}

func (s *Server) handleConnectProvider(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Provider string `json:"provider"`
		Endpoint string `json:"endpoint"`
		Secret   string `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondJSON(w, 400, map[string]any{"error": "invalid provider settings"})
		return
	}
	s.modelMu.Lock()
	defer s.modelMu.Unlock()
	cfg, err := s.loadModels()
	if err != nil {
		respondJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	var entry *modelservice.ProviderRosterItem
	for _, item := range modelservice.ListRoster(cfg) {
		if item.ID == request.Provider {
			copy := item
			entry = &copy
			break
		}
	}
	if entry == nil || !modelservice.CatalogSyncAdapterSupported(entry.Adapter) {
		respondJSON(w, 400, map[string]any{"error": "This provider has no supported catalog connection in the embedded kernel."})
		return
	}
	endpoint := strings.TrimSpace(request.Endpoint)
	if endpoint == "" {
		endpoint = entry.DefaultEndpoint
	}
	instance := &config.ProviderInstanceConfig{ID: strings.ReplaceAll(entry.ID, "_", "-"), ProviderKind: entry.ID, Adapter: entry.Adapter, Protocol: entry.Protocol, Endpoint: endpoint, State: config.ProviderInstanceStateEnabled}
	if err := instance.Validate(); err != nil {
		respondJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	for _, existing := range cfg.ProviderInstances {
		if existing != nil && existing.ID == instance.ID {
			instance.AuthConnectionRef = existing.AuthConnectionRef
		}
	}
	secret := strings.TrimSpace(request.Secret)
	if secret == "" && instance.AuthConnectionRef != "" {
		secret, err = modelservice.ResolveProviderCredentialReference(instance.AuthConnectionRef)
	}
	if err != nil {
		respondJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	input := modelservice.CatalogSyncInputFromInstance(instance)
	input.Secret = secret
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	models, err := modelservice.SyncCompatibleCatalog(ctx, input, nil)
	if err != nil {
		respondJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	if request.Secret != "" {
		key := "facet-" + instance.ID
		if err := auth.SetCredential(key, &auth.AuthCredential{Provider: instance.ProviderKind, AuthMethod: "token", AccessToken: secret}); err != nil {
			respondJSON(w, 500, map[string]any{"error": err.Error()})
			return
		}
		instance.AuthConnectionRef = "credential:" + key
	}
	found := false
	for i, existing := range cfg.ProviderInstances {
		if existing != nil && existing.ID == instance.ID {
			cfg.ProviderInstances[i] = instance
			found = true
			break
		}
	}
	if !found {
		cfg.ProviderInstances = append(cfg.ProviderInstances, instance)
	}
	if err := modelservice.SaveProviderInstanceCatalog(instance, models); err != nil {
		respondJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	if err := config.SaveConfig(s.modelConfigPath, cfg); err != nil {
		respondJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	respondJSON(w, 200, map[string]any{"ok": true, "message": fmt.Sprintf("Connected %s; %d catalog models available. Select and test a model before use.", entry.DisplayName, len(models))})
}
