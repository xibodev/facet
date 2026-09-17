package studio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/xibodev/facet-studio/pkg/agent"
	"github.com/xibodev/facet-studio/pkg/bus"
	"github.com/xibodev/facet-studio/pkg/fileutil"
	"github.com/xibodev/facet/internal/studio/engine"
	"github.com/xibodev/facet/internal/toolbox"
)

// nativeOutput implements the kernel's channel and tool-observation contracts.
// Writes are serialized; nothing may write the HTTP response after runTurn ends.
type nativeOutput struct {
	mu           sync.Mutex
	emit         func(turnEvent) error
	err          error
	text         string
	toolSequence int
	toolIDs      map[string][]string
	session      *Session
}

func (o *nativeOutput) BeforeLLM(_ context.Context, req *agent.LLMHookRequest) (*agent.LLMHookRequest, agent.HookDecision, error) {
	o.session.mu.Lock()
	o.session.NativeID = req.Meta.SessionKey
	o.session.mu.Unlock()
	return req, agent.HookDecision{}, nil
}
func (o *nativeOutput) AfterLLM(_ context.Context, res *agent.LLMHookResponse) (*agent.LLMHookResponse, agent.HookDecision, error) {
	return res, agent.HookDecision{}, nil
}

// The kernel invokes this approver and enforces its timeout/decision. Facet only
// presents the request and returns the user's answer; it does not run approvals.
func (o *nativeOutput) ApproveTool(ctx context.Context, req *agent.ToolApprovalRequest) (agent.ApprovalDecision, error) {
	if !toolbox.MayCharge(req.Tool) {
		return agent.ApprovalDecision{Approved: true}, nil
	}
	o.mu.Lock()
	o.toolSequence++
	id := fmt.Sprintf("%s:%d", req.Meta.TurnID, o.toolSequence)
	o.mu.Unlock()
	answer := make(chan bool, 1)
	o.session.mu.Lock()
	if o.session.approvals == nil {
		o.session.approvals = map[string]chan bool{}
	}
	o.session.approvals[id] = answer
	o.session.mu.Unlock()
	defer func() { o.session.mu.Lock(); delete(o.session.approvals, id); o.session.mu.Unlock() }()
	o.mu.Lock()
	e := &engine.NormalizedEvent{Type: "approval", ToolName: req.Tool, ToolID: id, Content: "This operation may incur a charge. Allow this specific operation?"}
	err := o.send(e)
	o.mu.Unlock()
	if err != nil {
		return agent.ApprovalDecision{}, err
	}
	select {
	case <-ctx.Done():
		return agent.ApprovalDecision{Reason: "Approval cancelled or timed out"}, ctx.Err()
	case allowed := <-answer:
		if !allowed {
			o.mu.Lock()
			toolID := ""
			if ids := o.toolIDs[req.Tool]; len(ids) > 0 {
				toolID = ids[0]
				o.toolIDs[req.Tool] = ids[1:]
			}
			_ = o.send(&engine.NormalizedEvent{Type: engine.EventToolResult, ToolID: toolID, ToolName: req.Tool, ToolOutput: "Not executed: you declined this operation.", IsError: true})
			o.mu.Unlock()
		}
		return agent.ApprovalDecision{Approved: allowed, Reason: "User decision in Facet"}, nil
	}
}

func (o *nativeOutput) send(event *engine.NormalizedEvent) error {
	if o.err != nil {
		return o.err
	}
	payload, err := normalizedEventMap(event)
	if err == nil {
		err = o.emit(turnEvent{normalized: event, payload: payload})
	}
	o.err = err
	return err
}
func (o *nativeOutput) GetStreamer(context.Context, string, string, string) (bus.Streamer, bool) {
	return o, true
}
func (o *nativeOutput) Update(_ context.Context, content string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.text = content
	return o.send(&engine.NormalizedEvent{Type: "text_replace", Content: content})
}
func (o *nativeOutput) Finalize(ctx context.Context, content string) error {
	return o.Update(ctx, content)
}
func (o *nativeOutput) Cancel(context.Context) {}
func (o *nativeOutput) BeforeTool(_ context.Context, call *agent.ToolCallHookRequest) (*agent.ToolCallHookRequest, agent.HookDecision, error) {
	if call.Tool == "exec" {
		command, _ := call.Arguments["command"].(string)
		if selfInvocation.MatchString(command) {
			return call, agent.HookDecision{Action: agent.HookActionDenyTool, Reason: "Facet tools are registered natively. Read the request JSON file and invoke the named native tool directly; do not launch another Facet binary."}, nil
		}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.toolSequence++
	id := fmt.Sprintf("native-tool-%d", o.toolSequence)
	o.toolIDs[call.Tool] = append(o.toolIDs[call.Tool], id)
	err := o.send(&engine.NormalizedEvent{Type: engine.EventToolUse, ToolID: id, ToolName: call.Tool, ToolInput: call.Arguments})
	return call, agent.HookDecision{}, err
}

var selfInvocation = regexp.MustCompile(`(?i)\bfacet(?:\.exe)?[\s"']+tools\b`)

func (o *nativeOutput) AfterTool(_ context.Context, result *agent.ToolResultHookResponse) (*agent.ToolResultHookResponse, agent.HookDecision, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	id := ""
	if ids := o.toolIDs[result.Tool]; len(ids) > 0 {
		id = ids[0]
		o.toolIDs[result.Tool] = ids[1:]
	}
	e := &engine.NormalizedEvent{Type: engine.EventToolResult, ToolID: id, ToolName: result.Tool, DurationMs: result.Duration.Milliseconds()}
	if result.Result != nil {
		e.ToolOutput = result.Result.ForUser
		if e.ToolOutput == "" {
			e.ToolOutput = result.Result.ForLLM
		}
		e.IsError = result.Result.IsError
	}
	return result, agent.HookDecision{}, o.send(e)
}

func (s *Session) runNativeTurn(ctx context.Context, prompt string, emit func(turnEvent) error) turnResult {
	o := &nativeOutput{emit: emit, toolIDs: map[string][]string{}, session: s}
	if s.nativeBus != nil {
		s.nativeBus.SetStreamDelegate(o)
	}
	if err := s.nativeLoop.MountHook(agent.NamedHook("facet.presentation", o)); err != nil {
		return turnResult{reason: err.Error()}
	}
	defer s.nativeLoop.UnmountHook("facet.presentation")
	response, err := s.nativeLoop.ProcessDirectWithChannel(ctx, prompt, s.ID, "facet", s.ID)
	s.mu.Lock()
	nativeID := s.NativeID
	s.mu.Unlock()
	if nativeID != "" {
		data, _ := json.Marshal(map[string]string{"id": s.ID, "key": nativeID})
		saveErr := os.MkdirAll(filepath.Join(s.Dir, ".facet"), 0700)
		if saveErr == nil {
			saveErr = fileutil.WriteFileAtomic(filepath.Join(s.Dir, ".facet", "conversation.json"), data, 0600)
		}
		if saveErr != nil && err == nil {
			err = fmt.Errorf("response completed but conversation reference could not be saved: %w", saveErr)
		}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.err != nil {
		return turnResult{reason: o.err.Error(), emitFailed: true}
	}
	if err != nil {
		return turnResult{reason: err.Error(), canceled: errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)}
	}
	if response != "" && response != o.text {
		if err := o.send(&engine.NormalizedEvent{Type: "text_replace", Content: response}); err != nil {
			return turnResult{reason: err.Error(), emitFailed: true}
		}
	}
	if response == "" && o.text == "" && o.toolSequence == 0 {
		return turnResult{reason: "The model returned no response. Retry or choose another model in Settings."}
	}
	return turnResult{ok: true}
}
