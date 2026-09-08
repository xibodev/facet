package module

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	// Async asks a long-running capability to return a job handle immediately
	// rather than blocking until the work completes. Opt-in: existing
	// consumers, including the human-facing CLI, expect a finished result.
	Async bool `json:"async,omitempty"`
	// Roots maps a logical root name declared in permissions.filesystem_* to a
	// canonicalized absolute path the host supplies per invocation. A module
	// resolves nothing itself: `facet_bundle` is how the Remotion composer is
	// found, and resolving it from the working directory made the renderer
	// depend on where the process happened to be launched.
	Roots map[string]Root `json:"roots,omitempty"`

	// The host sets these on every invocation. They are modelled so a strict
	// decoder accepts a real host request: rejecting a field the host always
	// sends would refuse every genuine call, and tolerating unknown fields
	// would silently discard a misspelled `consent`. Both are unacceptable, so
	// the protocol's own fields are named explicitly.
	Protocol   string `json:"protocol,omitempty"`
	Capability string `json:"capability,omitempty"`
	// Grants are the permissions actually authorized for THIS invocation. A
	// module must assume it has nothing that is not listed.
	Grants *Grants `json:"grants,omitempty"`
	// DeadlineMS is the host's wall-clock budget; it enforces it by killing the
	// process tree regardless, so this is a courtesy rather than a promise.
	DeadlineMS int `json:"deadline_ms,omitempty"`
	// MaxOutputBytes is the stdout ceiling. Facet returns artifact pointers
	// rather than inline payloads, so it is recorded rather than acted on.
	MaxOutputBytes int `json:"max_output_bytes,omitempty"`
}

// Grants is what the host authorized for one invocation.
type Grants struct {
	Network       []string `json:"network"`
	Credentials   []string `json:"credentials"`
	PaidProviders []string `json:"paid_providers"`
	Publish       bool     `json:"publish"`
	Subprocess    []string `json:"subprocess"`
}

// Root is one host-supplied filesystem grant.
type Root struct {
	Path string `json:"path"`
	// Mode is "ro" or "rw"; facet_bundle is always read-only.
	Mode string `json:"mode"`
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
		if err := decodeRequest(raw, &req); err != nil {
			return fail(OpInvoke, newRequestID(), "invalid_request",
				requestDecodeMessage(err),
				map[string]any{"error": bounded(err.Error())}, false)
		}
	}

	reqID := strings.TrimSpace(req.RequestID)
	if reqID == "" {
		reqID = newRequestID()
	}

	// The request states which protocol it speaks and which capability it
	// addresses. Both were accepted and ignored.
	//
	// A protocol Facet does not speak is refused rather than guessed at: a
	// future version may mean something different by the same field names, and
	// answering it as if it were v1 produces a confident wrong result.
	if v := strings.TrimSpace(req.Protocol); v != "" && v != Protocol {
		return fail(OpInvoke, reqID, "unsupported_protocol",
			"this module speaks "+Protocol+" and cannot answer a request in "+v,
			map[string]any{"requested": v, "supported": []string{Protocol}}, false)
	}

	// A capability disagreeing with the verb argument means the host and the
	// module would attribute the same result to different capabilities. Facet
	// ran the argument and reported it, so a mismatch was silently resolved in
	// favour of one side.
	if c := strings.TrimSpace(req.Capability); c != "" && c != capability {
		return fail(OpInvoke, reqID, "invalid_request",
			"the request addresses "+c+" but the invocation names "+capability,
			map[string]any{"request_capability": c, "invoked": capability}, false)
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

	// A host-supplied bundle root is authoritative over discovery: the host
	// knows where it installed the module's content, and cwd does not.
	if b, ok := req.Roots["facet_bundle"]; ok && strings.TrimSpace(b.Path) != "" {
		restoreBundle := useBundleRoot(b.Path)
		defer restoreBundle()
	}

	// Relative paths are measured from the project root the host granted.
	//
	// project_root was declared, used to label artifacts, and never resolved
	// against: the host sets the working directory to the module's install
	// directory, so a user-supplied "source.mp4" named nothing findable and
	// every relative path failed input_not_found. Working inside the granted
	// root is what makes a caller's relative path mean what they wrote.
	if pr, ok := req.Roots["project_root"]; ok && strings.TrimSpace(pr.Path) != "" {
		restoreDir, err := useWorkingRoot(pr.Path)
		if err != nil {
			return fail(OpInvoke, reqID, "invalid_request",
				"the granted project_root could not be entered",
				map[string]any{"root": "project_root", "error": bounded(err.Error())}, false)
		}
		defer restoreDir()
	}

	tool := strings.TrimSpace(req.Tool)
	if pinned != "" {
		tool = pinned
	}

	// Grant gate, checked BEFORE consent.
	//
	// Grants are what the host authorized for THIS invocation, and a module
	// must assume it has nothing that is not listed. Consent and a grant are
	// different things: a human approving the spend does not mean the host
	// authorized the provider, and running on consent alone would let a module
	// reach a provider the host deliberately withheld.
	if op == "run" && paidTools[tool] && req.Grants != nil {
		provider := paidProviderFor(tool)
		if !contains(req.Grants.PaidProviders, provider) {
			return fail(OpInvoke, reqID, "permission_denied",
				"the host did not grant the paid provider this tool requires",
				map[string]any{
					"tool": tool, "provider": provider,
					"granted_paid_providers": req.Grants.PaidProviders,
				}, false)
		}
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

	// Finish inside the host's budget rather than being killed by it.
	//
	// The host enforces deadline_ms by killing the process tree, and its
	// default is 60s while a render's own timeout is 600s. Verified: a 30s
	// 1080p render exceeded 60s, the process was killed, and the work was lost
	// with no envelope — the host saw a dead process rather than a failure it
	// could report. Clamping the tool's timeout means Facet returns a real
	// error inside the budget instead.
	input := applyDeadline(req.Input, req.DeadlineMS)

	args, err := toolboxArgs(op, tool, input)
	if err != nil {
		return fail(OpInvoke, reqID, "invalid_request", err.Error(),
			map[string]any{"capability": capability, "tool": tool}, false)
	}

	// Long-running work may return a handle instead of blocking.
	//
	// This is OPT-IN via `async`, deliberately. A render takes 30s at 720p and
	// 83s at 1080p, which leaves a cockpit with nothing to show; but the
	// human-facing CLI and every existing consumer expect a finished result,
	// and silently changing that for everyone would break them. The host asks
	// for a handle when it wants one.
	//
	// The binary grant is captured for the goroutine because `restore` runs
	// when this function returns, which is BEFORE the work finishes. Without
	// that the async path would resolve against an empty PATH — the same
	// declared-but-not-wired failure the grant itself was added to fix.
	if req.Async && isLongRunning(capability) {
		job := startJob(capability, tool)
		grants := req.Binaries
		go func() {
			restore := useBinaries(grants)
			defer restore()
			env, ok := toolbox.CLI(args)
			finishJob(job.JobID, project(OpInvoke, op, job.JobID, capability, tool, env, ok))
		}()
		return jobHandleEnvelope(reqID, capability, tool, job)
	}

	env, ok := toolbox.CLI(args)
	// The host truncates past its budget and a truncated envelope is unusable,
	// so an oversized success is replaced by an error that fits rather than
	// left to become corrupt JSON.
	return EnforceOutputBudget(
		project(OpInvoke, op, reqID, capability, tool, env, ok), req.MaxOutputBytes)
}

// isLongRunning reports whether a capability may return a job handle. It must
// agree with the descriptor: a capability that declares long_running but
// refuses to produce a handle would be a contract violation the host cannot
// see until it asks.
func isLongRunning(capability string) bool {
	return capability == CapToolsRun
}

// Estimate validates a request and reports expected effects and cost. It never
// generates media and never bills.
func Estimate(capability string, raw []byte) Envelope {
	var req Request
	if len(raw) > 0 {
		if err := decodeRequest(raw, &req); err != nil {
			return fail(OpInvoke, newRequestID(), "invalid_request",
				requestDecodeMessage(err),
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
	return EnforceOutputBudget(
		project(OpInvoke, "estimate", reqID, capability, tool, env, ok), req.MaxOutputBytes)
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
		// A host chooses how to present an artefact from its media type, so an
		// artefact without one falls back to a download link instead of the
		// player or viewer it deserves. Most tools report a mime_type; the
		// renderer does not, so its mp4 arrived untyped and would have been
		// offered as a file to save rather than a video to watch.
		//
		// Detect from the file's own bytes when the tool did not say. The
		// bytes are evidence and a declared type is a claim — the same reason
		// the provider's "image/png" for JPEG data is not trusted.
		if strings.TrimSpace(media) == "" {
			media = detectArtifactMediaType(path)
		}

		// The host validates every artifact and REJECTS one missing an id, a
		// root, a relative path or a valid digest. Facet emitted none of them,
		// so every artifact-producing run would have been refused with the
		// envelope otherwise correct.
		//
		// Separators are normalized because a Windows path with backslashes
		// cannot be checked against a root the host declared with forward
		// slashes: confinement would be uncheckable rather than merely ugly.
		rel := relativeArtifactPath(path)

		// A digest the tool did not report is computed from the bytes on disk.
		// The host uses it as provenance for anything it shows or stores, and
		// an artifact it cannot verify is one it will not accept.
		digest := protocolDigest(sha)
		if digest == "" || !ValidDigest(digest) {
			digest = fileDigestOf(path)
		}

		out = append(out, Artifact{
			// The id identifies this artifact within the response. The path is
			// already unique per run and is what a reader recognises.
			ID:   rel,
			Kind: "output",
			// project_root is where every tool writes; the bundle is read-only.
			Root:         "project_root",
			Path:         rel,
			Bytes:        fileBytes(path),
			MediaType:    media,
			Digest:       digest,
			Presentation: presentationFor(rel, media),
		})
	}

	if p, ok := m["output"].(string); ok {
		add(p, stringField(m, "sha256"), stringField(m, "mime_type"))
	}
	if p, ok := m["output_path"].(string); ok {
		add(p, stringField(m, "sha256"), stringField(m, "mime_type"))
	}

	// Frame extraction reports its files under `samples`, not `outputs`, so a
	// QA frame set produced no artifacts at all and the host had nothing to
	// build an image grid from.
	for _, sm := range mapSlice(m["samples"]) {
		add(stringField(sm, "path"), stringField(sm, "sha256"), stringField(sm, "mime_type"))
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

// detectArtifactMediaType reports the media type of a produced file from its
// own bytes, or "" when it cannot be read or does not identify itself.
//
// It never guesses from the extension. An mp4 written with a .png name is an
// mp4, and a host that trusted the name would offer the wrong viewer — the
// same failure as trusting a provider's declared type.
func detectArtifactMediaType(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	var header [512]byte
	n, err := f.Read(header[:])
	if err != nil && n == 0 {
		return ""
	}
	detected := http.DetectContentType(header[:n])
	switch {
	case strings.HasPrefix(detected, "image/"),
		strings.HasPrefix(detected, "video/"),
		strings.HasPrefix(detected, "audio/"),
		strings.HasPrefix(detected, "text/"),
		detected == "application/pdf",
		detected == "application/ogg":
		return detected
	}

	// net/http's signature table is deliberately small and misses formats a
	// media toolbox produces constantly. An MP3 without an ID3 tag — which is
	// exactly what edge_tts emits — detects as application/octet-stream, so
	// narration arrived untyped and a host would have offered it as a file to
	// download rather than audio to play.
	//
	// These are signature checks on the bytes, not extension guesses.
	if n >= 2 && header[0] == 0xff && header[1]&0xe0 == 0xe0 {
		return "audio/mpeg" // MPEG audio frame sync
	}
	if n >= 12 && string(header[4:8]) == "ftyp" {
		return "video/mp4"
	}
	if n >= 4 && string(header[:4]) == "fLaC" {
		return "audio/flac"
	}
	if n >= 4 && string(header[:4]) == "\x1a\x45\xdf\xa3" {
		return "video/webm" // Matroska/WebM
	}

	// A generic octet-stream tells the host nothing it did not already know,
	// so report nothing rather than something meaningless.
	return ""
}

// mapSlice normalizes a result field that may be []any or []map[string]any.
//
// A tool builds its result with a concrete slice type, so a single []any type
// assertion silently yields nothing for half of them — the field is present,
// the assertion fails, and the artefacts vanish without an error. frame_sample
// returns []map[string]any and produced zero artifacts for exactly that reason.
func mapSlice(v any) []map[string]any {
	switch t := v.(type) {
	case []map[string]any:
		return t
	case []any:
		out := make([]map[string]any, 0, len(t))
		for _, item := range t {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

// presentationFor names the host primitive that should render an artefact.
//
// It is deliberately sparse. A media type already tells the host that an mp4 is
// video and a jpeg is an image, so repeating that adds nothing and risks
// disagreeing with the bytes. The hint is for what a media type CANNOT express:
// a scene plan is application/json and also a timeline, and only Facet knows
// which JSON documents carry a time axis.
//
// An unknown value is ignored by the host, so naming a primitive it does not
// own is harmless — but pointless, and it would misrepresent the artefact to
// anything that did honour it.
func presentationFor(path, mediaType string) string {
	switch {
	case strings.HasSuffix(path, "scene_plan.json"),
		strings.HasSuffix(path, "edit_decisions.json"):
		// Time-ranged data: every item carries start/end seconds, and a table
		// of numbers answers "is the pacing sane" badly.
		return "timeline"
	case strings.HasSuffix(path, ".md"):
		return "markdown"
	}
	// Media types speak for themselves; do not second-guess them.
	return ""
}

// requestDecodeMessage distinguishes a malformed request from a well-formed one
// this module does not accept.
//
// The protocol request is the first thing a host constructs, and telling it the
// JSON is invalid when the JSON is fine sends it to re-serialize rather than to
// the schema. The same conflation inside the toolbox led an agent to abandon
// the module entirely.
func requestDecodeMessage(err error) string {
	text := err.Error()
	if strings.Contains(text, "unknown field") {
		field := text
		if i := strings.Index(text, "unknown field "); i >= 0 {
			field = strings.TrimSpace(text[i+len("unknown field "):])
		}
		return "the request carries the field " + field +
			" which this capability does not accept; see request_schemas in `module describe`"
	}
	if strings.Contains(text, "cannot unmarshal") {
		return "a request field has the wrong type: " + bounded(text)
	}
	return "request body is not valid JSON"
}

// decodeRequest rejects a request field this module does not know.
//
// json.Unmarshal ignores unknown fields, so a host typo was silently dropped: a
// request carrying "binarys" ran with no binaries granted and failed later as a
// missing dependency, and a misspelled "consent" would have been ignored while
// the run proceeded. Both look like a module fault and neither is.
//
// The consent case is the reason this is strict rather than lenient. A field
// that gates spending must never be silently discarded.
func decodeRequest(raw []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(dst)
}

// paidProviderFor names the billing provider behind a paid tool, matching the
// names declared in permissions.paid_providers.
func paidProviderFor(tool string) string {
	switch tool {
	case "gflow_video", "gflow_image":
		return "google_flow"
	case "openai_image", "openai_tts", "sora_video":
		return "openai"
	case "elevenlabs_tts":
		return "elevenlabs"
	case "flux_image":
		return "fal"
	case "kling_video":
		return "kling"
	}
	// An unrecognised paid tool fails closed: an empty provider matches no
	// grant, so it is refused rather than allowed by omission.
	return ""
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// fileDigestOf returns "sha256:<lowercase hex>" over a produced file, or "" if
// it cannot be read. A digest is provenance the host relies on, so it is
// computed rather than omitted when a tool did not report one.
func fileDigestOf(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// fileBytes reports a produced file's size, or 0 when it cannot be read.
func fileBytes(path string) int64 {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return 0
	}
	return info.Size()
}

// deadlineSafetyMargin is how much of the host's budget is reserved for Facet
// to write its envelope after the tool gives up. A tool that returns exactly at
// the deadline is still killed before its error can be reported.
const deadlineSafetyMargin = 5 * time.Second

// applyDeadline clamps a tool's own timeout to the host's remaining budget.
//
// It never EXTENDS a timeout: a caller asking for 30 seconds gets 30 seconds
// even when the host allows 600. It only prevents a tool from outliving the
// budget its caller has, which is the case that loses work.
func applyDeadline(input json.RawMessage, deadlineMS int) json.RawMessage {
	if deadlineMS <= 0 || len(input) == 0 {
		return input
	}
	budget := time.Duration(deadlineMS)*time.Millisecond - deadlineSafetyMargin
	if budget <= 0 {
		// Too small to reserve a margin from; leave the request untouched
		// rather than fabricate a timeout the caller did not ask for.
		return input
	}
	seconds := int(budget.Seconds())
	if seconds <= 0 {
		return input
	}

	var body map[string]any
	if err := json.Unmarshal(input, &body); err != nil {
		// Not an object; the tool will reject it with its own message.
		return input
	}
	if existing, ok := body["timeout_seconds"].(float64); ok && existing > 0 {
		if int(existing) <= seconds {
			return input // the caller already asked for less
		}
	}
	body["timeout_seconds"] = seconds

	clamped, err := json.Marshal(body)
	if err != nil {
		return input
	}
	return clamped
}

// relativeArtifactPath makes an artifact path relative to the root it is
// declared under.
//
// A tool that is handed an absolute output_path reports it back verbatim, so
// the artifact carried the host's own filesystem layout: the host refuses an
// absolute path because confinement cannot be checked against a root, and an
// id like "E:/.../.local/state/xibodev.facet/project_root/tb.mp4" leaks that
// layout into anything that stores or displays it.
//
// The working directory is the project root under a host, which is what a
// relative path is measured against. A path that cannot be made relative — one
// on another volume, say — is returned as given rather than mangled, so the
// host still refuses it visibly instead of receiving something plausible and
// wrong.
func relativeArtifactPath(path string) string {
	slashed := filepath.ToSlash(path)
	if !isAbsolutePath(slashed) {
		return slashed
	}
	cwd, err := os.Getwd()
	if err != nil {
		return slashed
	}
	rel, err := filepath.Rel(cwd, filepath.FromSlash(path))
	if err != nil {
		return slashed
	}
	rel = filepath.ToSlash(rel)
	// A path outside the root is not made to look like one inside it.
	if strings.HasPrefix(rel, "../") || rel == ".." {
		return slashed
	}
	return rel
}
