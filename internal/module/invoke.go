package module

import (
	"encoding/json"
	"strings"

	"github.com/xibodev/facet/internal/toolbox"
)

// Request is the input body for estimate and invoke.
//
// Tool names the underlying Facet tool. Input is the tool-specific request body
// and is passed through to the toolbox unmodified: the module does not rewrite,
// enrich, or reinterpret a creative request.
type Request struct {
	RequestID string          `json:"request_id"`
	Tool      string          `json:"tool"`
	Input     json.RawMessage `json:"input"`
	Consent   *Consent        `json:"consent,omitempty"`
	Seed      *SeedRef        `json:"seed,omitempty"`
	// Binaries maps a declared subprocess name to the ABSOLUTE path the host
	// resolved for it. A module inherits no environment, so this is the only
	// way a binary becomes reachable.
	//
	// An absolute path is an identity; a PATH is a search, and a search can
	// resolve to something the host never authorized. A binary the host could
	// not resolve must be ABSENT rather than empty, so "not supplied" and
	// "supplied as nothing" stay distinguishable.
	Binaries map[string]string `json:"binaries,omitempty"`
}

// Consent records that a human approved a paid or billable operation. The
// module never fabricates this; the host agent must obtain it from a person.
type Consent struct {
	PaidGenerationApproved bool   `json:"paid_generation_approved"`
	ApprovedBy             string `json:"approved_by"`
	Note                   string `json:"note,omitempty"`
}

// SeedRef references a Midden content seed by path and digest only. Facet never
// invokes Midden and never reads its database.
type SeedRef struct {
	Schema string `json:"schema"`
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

// paidTools are the tools whose cost is genuinely unknown before a run. They
// must never be reported as free, and must never run without explicit human
// consent. This mirrors the nil-cost set in toolbox.executionFor.
var paidTools = map[string]bool{
	"gflow_video":    true,
	"gflow_image":    true,
	"openai_image":   true,
	"flux_image":     true,
	"kling_video":    true,
	"sora_video":     true,
	"openai_tts":     true,
	"elevenlabs_tts": true,
}

// writesOutput reports whether a run of this tool can write files outside the
// module's own process state.
//
// The toolbox now reports this honestly at source (internal/toolbox:
// externalWriteFor), so this is a SECOND, INDEPENDENT derivation rather than
// the only one. project() cross-checks the two and warns on disagreement: the
// host compares declared effects against reported ones, and that comparison is
// only meaningful if the two sides are independently derived. Two agreeing
// observations are evidence; one value copied twice is not.
//
// Both fail CLOSED on an unrecognised tool.
func writesOutput(op, tool string) bool {
	// Estimation validates a request and never writes output.
	if op != "run" {
		return false
	}
	switch tool {
	// Read-only inspection: reports facts about existing files.
	case "media_probe", "audio_probe", "music_library",
		"image_selector", "video_selector":
		return false
	}
	// Everything else may write. Unknown tools included, deliberately.
	return true
}

// capabilityOp maps a capability ID onto the toolbox operation that implements
// it, plus any tool the capability pins.
func capabilityOp(capability string) (op string, pinnedTool string, ok bool) {
	switch capability {
	case CapToolsList:
		return "list", "", true
	case CapToolsDescribe:
		return "describe", "", true
	case CapToolsEstimate:
		return "estimate", "", true
	case CapToolsRun:
		return "run", "", true
	case CapOutputReview:
		return "run", "output_review", true
	case CapArtifactInspect:
		return "run", "media_probe", true
	case CapJobsStatus:
		// Polling is answered directly; it never reaches the toolbox.
		return "status", "", true
	}
	return "", "", false
}

// Invoke executes one capability through the existing toolbox.
//
// It is a strict adapter. It does not implement tool behavior, does not retry,
// and does not soften an error into a success. Its only additions are protocol
// framing, consent enforcement, and honest execution facts.
//
// creative.tools.estimate is dispatched here like any other capability. It
// remains a separate, auditable operation that never bills and never generates;
// it is simply not a separate verb.
func Invoke(capability string, raw []byte) Envelope {
	if capability == CapToolsEstimate {
		return Estimate(capability, raw)
	}
	if capability == CapJobsStatus {
		return JobStatus(raw)
	}

	var req Request
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &req); err != nil {
			return fail(OpInvoke, newRequestID(), "invalid_request",
				"request body is not valid JSON",
				map[string]any{"error": bounded(err.Error())}, false)
		}
	}

	reqID := strings.TrimSpace(req.RequestID)
	if reqID == "" {
		reqID = newRequestID()
	}

	op, pinned, known := capabilityOp(capability)
	if !known {
		return fail(OpInvoke, reqID, "unknown_capability",
			"capability is not provided by this module",
			map[string]any{"capability": capability}, false)
	}

	// The binary map is Facet's entire execution authority under a host that
	// supplies no environment, so it is validated before anything runs and then
	// installed as the toolbox's resolution override. Validating it without
	// installing it would leave every subprocess resolving against an empty
	// PATH — declared, checked, and never actually used.
	if err := ValidateBinaries(req.Binaries); err != nil {
		return fail(OpInvoke, reqID, "invalid_request", err.Error(),
			map[string]any{"capability": capability}, false)
	}
	restore := useBinaries(req.Binaries)
	defer restore()

	tool := strings.TrimSpace(req.Tool)
	if pinned != "" {
		tool = pinned
	}

	// Consent gate. Paid generation requires explicit human approval, and an
	// unknown cost is never treated as free. A cross-agent agreement is not
	// consent; only a person can grant this.
	if op == "run" && paidTools[tool] {
		if req.Consent == nil || !req.Consent.PaidGenerationApproved {
			return fail(OpInvoke, reqID, "consent_required",
				"tool may incur real cost and requires explicit human consent before execution",
				map[string]any{
					"tool":       tool,
					"cost_known": false,
					"reason":     "cost is unknown before execution and is never assumed to be zero",
				}, false)
		}
	}

	args, err := toolboxArgs(op, tool, req.Input)
	if err != nil {
		return fail(OpInvoke, reqID, "invalid_request", err.Error(),
			map[string]any{"capability": capability, "tool": tool}, false)
	}

	env, ok := toolbox.CLI(args)
	return project(OpInvoke, op, reqID, capability, tool, env, ok)
}

// Estimate validates a request and reports expected effects and cost. It never
// generates media and never bills.
func Estimate(capability string, raw []byte) Envelope {
	var req Request
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &req); err != nil {
			return fail(OpInvoke, newRequestID(), "invalid_request",
				"request body is not valid JSON",
				map[string]any{"error": bounded(err.Error())}, false)
		}
	}

	reqID := strings.TrimSpace(req.RequestID)
	if reqID == "" {
		reqID = newRequestID()
	}

	_, pinned, known := capabilityOp(capability)
	if !known {
		return fail(OpInvoke, reqID, "unknown_capability",
			"capability is not provided by this module",
			map[string]any{"capability": capability}, false)
	}

	tool := strings.TrimSpace(req.Tool)
	if pinned != "" {
		tool = pinned
	}

	args, err := toolboxArgs("estimate", tool, req.Input)
	if err != nil {
		return fail(OpInvoke, reqID, "invalid_request", err.Error(),
			map[string]any{"capability": capability, "tool": tool}, false)
	}

	env, ok := toolbox.CLI(args)
	return project(OpInvoke, "estimate", reqID, capability, tool, env, ok)
}

func toolboxArgs(op, tool string, input json.RawMessage) ([]string, error) {
	switch op {
	case "list":
		return []string{"tools", "list"}, nil
	case "describe":
		if tool == "" {
			return nil, &toolboxErr{missingToolMessage(input)}
		}
		return []string{"tools", "describe", tool}, nil
	case "estimate", "run":
		if tool == "" {
			return nil, &toolboxErr{missingToolMessage(input)}
		}
		if len(input) == 0 {
			return nil, &toolboxErr{"input is required for this capability"}
		}
		return []string{"tools", op, tool, "--input", string(input)}, nil
	}
	return nil, &toolboxErr{"unsupported operation: " + op}
}

// missingToolMessage names the likely mistake instead of only the symptom.
//
// `tool` is a field of the protocol REQUEST, a sibling of `input`, and never a
// field inside `input`. That is what the declared request schemas say, but the
// distinction is easy to miss, and an integrator who nests it gets a bare
// "tool is required" that does not say where to put it. If the misplaced value
// is visible, say so — a diagnosable error is worth more than a terse one.
func missingToolMessage(input json.RawMessage) string {
	const base = "tool is required for this capability"
	if len(input) == 0 {
		return base
	}
	var probe struct {
		Tool string `json:"tool"`
	}
	if err := json.Unmarshal(input, &probe); err == nil && strings.TrimSpace(probe.Tool) != "" {
		return base + ": found \"tool\":\"" + probe.Tool + "\" inside \"input\", but tool is a " +
			"field of the request itself, a sibling of input"
	}
	return base
}

// project converts a toolbox envelope into a module envelope.
//
// Cost pointers are carried across by reference semantics, not defaulted: a nil
// estimated_cost stays nil, because null means "unknown" and 0 means "free".
// Collapsing them would turn an unknown charge into an implied promise.
// toolboxOp is "run" or "estimate" and is passed explicitly rather than derived
// from the verb: since estimation is a capability rather than a verb, the
// envelope's operation is "invoke" for both, and inferring from it would
// declare an estimate as if it were a run.
func project(op, toolboxOp, reqID, capability, tool string, env toolbox.Envelope, ok bool) Envelope {
	warnings := env.Warnings
	if warnings == nil {
		warnings = []string{}
	}

	// external_writes is derived twice, independently: once by the toolbox at
	// source, once here. They should always agree; a disagreement means one of
	// the two is wrong and the host should not be handed a confident answer.
	// Fail closed on disagreement — either side claiming a write wins.
	adapterSays := writesOutput(toolboxOp, tool)
	toolboxSays := env.Execution.ExternalWrite
	if adapterSays != toolboxSays {
		warnings = append(warnings,
			"external_writes disagreement for "+tool+": toolbox reported a different "+
				"value than the module adapter derived; declaring the write-performing "+
				"value so approval is not bypassed")
	}

	exec := Execution{
		Local:          !env.Execution.Network,
		Network:        env.Execution.Network,
		ExternalWrites: adapterSays || toolboxSays,
		Provider:       env.Execution.Provider,
		EstimatedCost:  env.Execution.EstimatedCost,
		ActualCost:     env.Execution.ActualCost,
		Artifacts:      artifactsFrom(env.Result),
	}

	out := Envelope{
		Protocol:  Protocol,
		Module:    ModuleID,
		Operation: op,
		RequestID: reqID,
		OK:        ok && env.OK,
		Warnings:  warnings,
		Execution: exec,
	}

	if out.OK {
		out.Result = map[string]any{
			"capability": capability,
			"tool":       tool,
			"output":     env.Result,
		}
		return out
	}

	code, message, retryable := "command_failed", "tool execution failed", false
	details := map[string]any{}
	if env.Error != nil {
		code = env.Error.Code
		message = env.Error.Message
		retryable = env.Error.Retryable
		if env.Error.Details != nil {
			details = env.Error.Details
		}
	}
	details["capability"] = capability
	if tool != "" {
		details["tool"] = tool
	}
	out.Error = &Error{Code: code, Message: message, Retryable: retryable, Details: details}
	return out
}

// artifactsFrom extracts artifact pointers a tool reported. It reports only
// files the tool actually named; it never invents a path or claims a digest it
// was not given.
func artifactsFrom(result any) []Artifact {
	out := []Artifact{}
	m, ok := result.(map[string]any)
	if !ok {
		return out
	}

	add := func(path string, sha string, media string) {
		if strings.TrimSpace(path) == "" {
			return
		}
		out = append(out, Artifact{
			Kind:      "output",
			Path:      path,
			MediaType: media,
			Digest:    protocolDigest(sha),
		})
	}

	if p, ok := m["output"].(string); ok {
		add(p, stringField(m, "sha256"), stringField(m, "mime_type"))
	}
	if p, ok := m["output_path"].(string); ok {
		add(p, stringField(m, "sha256"), stringField(m, "mime_type"))
	}

	if raw, ok := m["outputs"].([]any); ok {
		for _, item := range raw {
			om, ok := item.(map[string]any)
			if !ok {
				continue
			}
			add(stringField(om, "output"), stringField(om, "sha256"), stringField(om, "mime_type"))
		}
	}
	return out
}

func stringField(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

// protocolDigest normalizes a digest to the module-boundary form agreed across
// lanes: "sha256:" + lowercase hex.
//
// Only the protocol-level Artifact.SHA256 is prefixed. Digests inside a tool's
// own result payload are tool output rather than protocol framing and are left
// byte-identical to what `facet tools` returns, so both surfaces stay
// self-consistent. An unrecognised value is returned unchanged rather than
// decorated, so this never manufactures a well-formed digest from a malformed
// one.
func protocolDigest(sha string) string {
	s := strings.ToLower(strings.TrimSpace(sha))
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "sha256:") {
		s = strings.TrimPrefix(s, "sha256:")
	}
	if len(s) != 64 {
		return sha
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return sha
		}
	}
	return "sha256:" + s
}

func bounded(s string) string {
	const limit = 512
	if len(s) > limit {
		return s[:limit] + "..."
	}
	return s
}
