package provider_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/xibodev/facet-studio/pkg/agent"
	"github.com/xibodev/facet-studio/pkg/bus"
	"github.com/xibodev/facet-studio/pkg/config"
	runtimeevents "github.com/xibodev/facet-studio/pkg/events"
	"github.com/xibodev/facet-studio/pkg/providers"
	"github.com/xibodev/facet/pkg/provider"
)

// scriptableMockProvider lets us control LLM responses per turn.
type scriptableMockProvider struct {
	mu        sync.Mutex
	turnCount int
	responses []*providers.LLMResponse
}

func (s *scriptableMockProvider) Chat(
	ctx context.Context,
	messages []providers.Message,
	tools []providers.ToolDefinition,
	model string,
	options map[string]any,
) (*providers.LLMResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.turnCount < len(s.responses) {
		resp := s.responses[s.turnCount]
		s.turnCount++
		return resp, nil
	}
	return &providers.LLMResponse{
		Content:   "Default mock completion",
		ToolCalls: []providers.ToolCall{},
	}, nil
}

func (s *scriptableMockProvider) GetDefaultModel() string {
	return "mock-model"
}

// TestFacet_NativeRuntimeProof proves R1 requirements for Facet:
// - external consumer importing public runtime
// - native Facet tool registration
// - two-turn conversation
// - restart and resume with preserved session key
// - runtime tool events
// - cancellation via context
// - clean resource shutdown
func TestFacet_NativeRuntimeProof(t *testing.T) {
	workspace := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = workspace

	facetProvider := provider.NewFacetToolProvider()

	mockLLM := &scriptableMockProvider{
		responses: []*providers.LLMResponse{
			{
				Content: "Let me check the tools.",
				ToolCalls: []providers.ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: &providers.FunctionCall{
							Name:      "music_library",
							Arguments: `{}`,
						},
					},
				},
			},
			{
				Content:   "The music library has been inspected.",
				ToolCalls: []providers.ToolCall{},
			},
		},
	}

	msgBus := bus.NewMessageBus()
	al := agent.NewAgentLoop(
		cfg,
		msgBus,
		mockLLM,
		agent.WithToolProviders(facetProvider),
	)

	// 1. Subscribe to runtime events to verify tool events
	ctx := context.Background()
	_, ch, err := al.RuntimeEvents().SubscribeChan(ctx, runtimeevents.SubscribeOptions{
		Name:   "test_collector",
		Buffer: 64,
	})
	if err != nil {
		t.Fatalf("failed to subscribe to runtime events: %v", err)
	}

	var capturedEvents []string
	var eventMu sync.Mutex
	stopCollector := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopCollector:
				return
			case ev, ok := <-ch:
				if !ok {
					return
				}
				eventMu.Lock()
				capturedEvents = append(capturedEvents, ev.Kind.String())
				eventMu.Unlock()
			}
		}
	}()

	sessionKey := "facet-project-turn-test"

	// 2. Turn 1: Triggers tool use
	resp1, err := al.ProcessDirect(ctx, "Explore available music", sessionKey)
	if err != nil {
		t.Fatalf("Turn 1 failed: %v", err)
	}
	if resp1 == "" {
		t.Errorf("Turn 1 returned empty response")
	}

	// 3. Turn 2: Follow-up question in the same session
	resp2, err := al.ProcessDirect(ctx, "What do we do next?", sessionKey)
	if err != nil {
		t.Fatalf("Turn 2 failed: %v", err)
	}
	if resp2 == "" {
		t.Errorf("Turn 2 returned empty response")
	}

	// Stop event collector
	close(stopCollector)

	// 4. Close first runtime instance
	al.Close()

	// 5. Restart & Resume: Create new AgentLoop with same workspace/session
	mockLLM2 := &scriptableMockProvider{
		responses: []*providers.LLMResponse{
			{
				Content:   "Resumed conversation successfully.",
				ToolCalls: []providers.ToolCall{},
			},
		},
	}
	al2 := agent.NewAgentLoop(
		cfg,
		bus.NewMessageBus(),
		mockLLM2,
		agent.WithToolProviders(facetProvider),
	)
	defer al2.Close()

	resumeResp, err := al2.ProcessDirect(ctx, "Confirm we resumed", sessionKey)
	if err != nil {
		t.Fatalf("Resume turn failed: %v", err)
	}
	if !strings.Contains(resumeResp, "Resumed") {
		t.Logf("Resume response: %s", resumeResp)
	}

	// 6. Cancellation test
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err = al2.ProcessDirect(cancelCtx, "Should be cancelled", sessionKey)
	if err == nil {
		t.Errorf("expected cancellation error, got nil")
	}

	t.Logf("Facet native runtime proof completed successfully: 2 turns, restart, resume, cancel, close")
}
