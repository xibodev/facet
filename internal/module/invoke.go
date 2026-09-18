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
	Protocol string `json:"protocol,omitempty"`
	// ContractVersion is the BEHAVIOURAL contract the host expects, echoed
	// from what it read in the descriptor. Exact match or refusal — see
	// module.ContractVersion.
	ContractVersion string `json:"contract_version,omitempty"`
	Capability      string `json:"capability,omitempty"`
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

// paidTools is DERIVED from the Operation layer, never restated here.
//
// It previously enumerated the same eight tools as toolbox.executionFor, by
// hand, in two layers. Chargeability is a property of the Operation; this
// layer is a Projection and must not hold a second opinion about it.
//
// Kept as a function rather than a map so there is no copy to drift.
func paidTool(tool string) bool { return toolbox.MayCharge(tool) }

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

	// The BEHAVIOURAL contract must match exactly.
	//
	// Checked before any work because a behavioural mismatch means the two
	// sides disagree about what the guarantees MEAN — not about a field's
	// shape. Running first and discovering that afterwards is how every
	// inference in this module got made.
	//
	// Absent is permitted: a v1 host does not know this field exists, and
	// refusing it would break every existing caller. Present-and-wrong is
	// refused, because a host that names a contract has made a claim.
	if v := strings.TrimSpace(req.ContractVersion); v != "" && v != ContractVersion {
		// Naming the WIRE identity here is a distinct mistake and gets a
		// distinct remedy.
		//
		// xibodev.module/v1 is a wire-format identity, not a behavioural
		// contract — v1 behaviour was never versioned, which is the premise of
		// the successor. So an author writing it here means "I am a v1
		// caller", which is TRUE, and the honest answer is that a v1 caller
		// sends nothing at all.
		//
		// Still refused rather than served: serving it would report a run as
		// governed by a contract with no guarantees and no conformance suite,
		// which a consumer could not distinguish from a real one. That is the
		// two-states-one-value defect again, arriving through a permissive
		// default — and §10 says absent falls back to v1 behaviour "never to a
		// more permissive default".
		//
		// The old message told such an author to "install a module
		// implementing xibodev.module/v1". No such module can exist. A
		// refusal whose remedy is impossible is worse than a blunt one.
		remedy := "match the contract_version exactly on both sides; " +
			"ranges, downgrade and fallback are deliberately not supported"
		detail := "the host expects behavioural contract " + v +
			" but this module implements " + ContractVersion +
			"; there is no negotiation between behavioural contracts"
		if v == Protocol {
			remedy = "omit contract_version entirely: it names a BEHAVIOURAL " +
				"contract, and " + Protocol + " is the wire-format identity. " +
				"A v1 caller sends no contract_version and is served under v1"
			detail = Protocol + " is a wire-format identity, not a behavioural " +
				"contract — v1 behaviour was never versioned. This module " +
				"implements " + ContractVersion
		}
		return fail(OpInvoke, reqID, "contract_incompatible", detail,
			map[string]any{
				"requested_contract_version": v,
				"module_contract_version":    ContractVersion,
				"remedy":                     remedy,
			}, false)
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
	projectRoot, releaseGrants, grantFailure := applyGrants(&req, reqID)
	if grantFailure != nil {
		return *grantFailure
	}
	defer releaseGrants()

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
	if op == "run" && paidTool(tool) && req.Grants != nil {
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
	if op == "run" && paidTool(tool) {
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

	// A supplied seed is LOADED and VERIFIED before any work begins.
	//
	// SeedRef was declared on the request, documented, and read by nothing:
	// LoadSeed existed and was tested, but no invocation ever called it, so a
	// host staging a Midden seed got a silent no-op — journey C's Facet half
	// was a library function no caller could reach. The eighth instance of a
	// field accepted and ignored.
	//
	// Verified first so a bad digest fails before spending a render, and so a
	// Facet artifact can never claim provenance from bytes it did not read.
	var seedRes *SeedResolution
	var seedWarnings []string
	if req.Seed != nil {
		seed, res, err := LoadSeed(req.Seed)
		if err != nil {
			return fail(OpInvoke, reqID, "invalid_request",
				"seed could not be loaded: "+bounded(err.Error()),
				map[string]any{"schema": SeedSchemaID}, false)
		}
		if !res.Verified && strings.TrimSpace(req.Seed.Digest) != "" {
			return fail(OpInvoke, reqID, "invalid_request",
				"seed digest did not verify; refusing to consume unverified evidence",
				map[string]any{"expected": req.Seed.Digest, "actual": res.DigestActual}, false)
		}
		seedRes = res

		// A seed's suggested output types are CHECKED against Facet's
		// vocabulary, and an unrecognised one is reported.
		//
		// OutputTypes was written, tested, and called by nothing -- the ninth
		// instance of the declared-but-never-wired class, and the second in
		// this seed path alone after SeedRef itself. A producer naming
		// "explainer-video" instead of "explainer" got silence, and the run
		// proceeded as if the seed had suggested nothing at all.
		//
		// A warning rather than a refusal, deliberately: Midden's vocabulary
		// is its own and may legitimately grow past Facet's. The caller learns
		// the suggestion was ignored instead of assuming it was honoured.
		_, seedWarnings = seed.OutputTypes()
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

	// Async work cannot depend on the working directory: it is process-global
	// and restored when Invoke returns, which is BEFORE a job finishes. So the
	// caller's relative paths are resolved against the granted root here,
	// while that root is still current, and the job runs on absolute paths.
	//
	// Refusing the combination (the previous behaviour) would have left the
	// cockpit unable to poll a render, which is the whole reason async exists:
	// an 83s 1080p render otherwise shows nothing until it completes.
	if req.Async && isLongRunning(capability) && projectRoot != "" {
		input = absolutizeRequestPaths(input, projectRoot)
	}

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
	out := project(OpInvoke, op, reqID, capability, tool, env, ok)

	// Seed warnings reach the CALLER, not just the log. Collecting a warning
	// and dropping it would be the same defect one layer along: the check
	// would run, find the problem, and tell nobody.
	if len(seedWarnings) > 0 {
		out.Warnings = append(out.Warnings, seedWarnings...)
	}
	// Which contract governed this run. Omitted when the caller named none,
	// so a v1 host sees exactly what it saw before.
	out.Execution.ContractVersion = strings.TrimSpace(req.ContractVersion)

	// A run that consumed a seed reports where its content came from.
	//
	// This is the point of loading one: without it the host stages evidence,
	// Facet reads it, and the resulting artifact carries no trace of what it
	// was made from. The manifest keys provenance on the seed DIGEST rather
	// than its staged path, which differs between machines.
	// Reported whenever a seed was consumed, ARTIFACTS OR NOT.
	//
	// This required artifacts, so a seeded run that produced none — a probe, a
	// review, a listing — returned no manifest at all. The seed had been read
	// and its digest verified, and that fact vanished silently: a caller
	// asking "did Facet actually use my seed?" got no answer.
	//
	// A manifest with an empty artifact list is the honest answer. It says the
	// seed was consumed and verified, and that this particular run produced
	// nothing to attribute.
	if seedRes != nil && out.OK {
		out.Result = map[string]any{
			"output":   out.Result,
			"manifest": NewManifest(capability, out, seedRes),
		}
	}

	// The host truncates past its budget and a truncated envelope is unusable,
	// so an oversized success is replaced by an error that fits rather than
	// left to become corrupt JSON.
	return EnforceOutputBudget(out, req.MaxOutputBytes)
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
// applyGrants installs every per-invocation grant the host supplied and
// returns the resolved project root plus a restore function.
//
// It exists because Estimate had NONE of this. It is a separate entry point
// that never entered the granted project_root, so an identical request
// succeeded through run and failed through estimate with input_not_found:
//
//	creative.tools.run      -> ok
//	creative.tools.estimate -> input_not_found
//
// The skill instructs an agent to estimate before anything consequential, so
// the check meant to prevent a wasted call was the one that could not resolve
// the caller's paths. Binary grants and the bundle root were missing there
// too, which would make an estimate report a dependency as absent when the
// host had supplied it.
//
// Sharing one implementation is the point: two paths that must agree about
// grants will not stay in agreement if each installs its own.
func applyGrants(req *Request, reqID string) (projectRoot string, restore func(), failure *Envelope) {
	var undo []func()
	release := func() {
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
	}

	undo = append(undo, useBinaries(req.Binaries))

	// A host-supplied bundle root is authoritative over discovery: the host
	// knows where it installed the module's content, and cwd does not.
	if b, ok := req.Roots["facet_bundle"]; ok && strings.TrimSpace(b.Path) != "" {
		undo = append(undo, useBundleRoot(b.Path))
	}

	// Relative paths are measured from the project root the host granted.
	//
	// project_root was declared, used to label artifacts, and never resolved
	// against: the host sets the working directory to the module's install
	// directory, so a user-supplied "source.mp4" named nothing findable and
	// every relative path failed input_not_found.
	if pr, ok := req.Roots["project_root"]; ok && strings.TrimSpace(pr.Path) != "" {
		restoreDir, err := useWorkingRoot(pr.Path)
		if err != nil {
			release()
			env := fail(OpInvoke, reqID, "invalid_request",
				"the granted project_root could not be entered",
				map[string]any{"root": "project_root", "error": bounded(err.Error())}, false)
			return "", func() {}, &env
		}
		undo = append(undo, restoreDir)
		if abs, err := os.Getwd(); err == nil {
			projectRoot = abs
		}

		// A granted root is a CONFINEMENT, not just a base for resolution.
		// Verified: output_path "../escaped.mp4" wrote a real file outside the
		// granted root and reported it back as a relative path, which the host
		// validator accepts because it is not absolute.
		//
		// Refused before the tool runs: after the write the file already
		// exists outside the root, and no envelope can undo that.
		if bad, ok := requestEscapesRoot(req.Input); ok {
			release()
			env := fail(OpInvoke, reqID, "invalid_request",
				"a request path leaves the granted project_root: "+bad,
				map[string]any{"root": "project_root", "path": bad}, false)
			return "", func() {}, &env
		}
	}

	return projectRoot, release, nil
}

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

	if err := ValidateBinaries(req.Binaries); err != nil {
		return fail(OpInvoke, reqID, "invalid_request", err.Error(),
			map[string]any{"capability": capability}, false)
	}
	_, releaseGrants, grantFailure := applyGrants(&req, reqID)
	if grantFailure != nil {
		return *grantFailure
	}
	defer releaseGrants()

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
	est := project(OpInvoke, "estimate", reqID, capability, tool, env, ok)
	est.Execution.ContractVersion = strings.TrimSpace(req.ContractVersion)
	return EnforceOutputBudget(est, req.MaxOutputBytes)
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
			Kind: ArtifactKindOutput,
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
// An unknown value is ignored by the host, so naming a primitive it does not
// own is harmless — but pointless, and it would misrepresent the artefact to
// anything that did honour it.
func presentationFor(path, mediaType string) string {
	switch {
	case strings.HasSuffix(path, ".md"):
		return "markdown"
	case strings.HasSuffix(path, ".srt"), strings.HasSuffix(path, ".vtt"):
		// Captions arrive as text/plain, which tells a host to render prose.
		// They are timed cues: shown as a paragraph the timings become noise,
		// and the one question a reviewer has — does this line up with the
		// video — cannot be answered.
		return "timeline"
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
//
// ALIASES toolbox.DeadlineSafetyMargin rather than restating it. The estimate
// derives "does this render fit the default deadline" from the same value, and
// when these were two literals the estimate went stale by four seconds without
// anything failing: the margin here dropped 5s -> 1s and the estimate's
// hardcoded 55 did not follow.
const deadlineSafetyMargin = toolbox.DeadlineSafetyMargin

// minimumToolBudget is the least time worth handing a tool.
//
// Below this the clamp is not protecting work, it is guaranteeing a timeout
// with extra steps. Leaving the request untouched lets the host's own kill be
// the thing that stops it, which at least reports the caller's real intent.
const minimumToolBudget = 2 * time.Second

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
	if budget < minimumToolBudget {
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
	if escapesRoot(rel) {
		return slashed
	}
	return rel
}

// requestEscapesRoot finds the first path in a tool request that leaves the
// granted root, so the write can be refused rather than described afterwards.
//
// It walks the request generically instead of naming output_path: every tool
// spells its paths differently (input/input_path/source/output_path/segments),
// and a confinement check that has to be remembered per tool is one a new tool
// will be added without.
func requestEscapesRoot(raw json.RawMessage) (string, bool) {
	var input any
	if len(raw) == 0 || json.Unmarshal(raw, &input) != nil {
		// A body that does not decode is refused later by the tool's own
		// strict decoding, which reports the problem far better than this can.
		return "", false
	}

	// Only values under a path-BEARING key are treated as paths.
	//
	// The first version checked every string in the request, which refused
	// narration: `{"title":"../../ explained"}` had the whole render rejected
	// because a caption mentioned a parent directory. A confinement check that
	// fires on prose teaches callers to work around it, which is worse than
	// the hole it closes.
	//
	// Keying on the field name is sound because a path only reaches the
	// filesystem through a field the tool reads AS a path; a string in a
	// caption is never opened.
	var walk func(key string, v any) (string, bool)
	walk = func(key string, v any) (string, bool) {
		switch t := v.(type) {
		case string:
			if !isPathKey(key) || t == "" || isAbsolutePath(t) {
				// Absolute paths are a separate concern: they are visible to
				// the host validator, which refuses them outright.
				return "", false
			}
			if escapesRoot(filepath.ToSlash(t)) {
				return t, true
			}
		case []any:
			// An array inherits its key: `segments: ["a.mp4", "../b.mp4"]`.
			for _, e := range t {
				if bad, ok := walk(key, e); ok {
					return bad, true
				}
			}
		case map[string]any:
			for k, e := range t {
				if bad, ok := walk(k, e); ok {
					return bad, true
				}
			}
		}
		return "", false
	}
	return walk("", input)
}

// absolutizeRequestPaths rewrites every path-bearing field to an absolute path
// under root, so the work no longer depends on a working directory.
//
// This is what lets async work honour a granted project_root. The working
// directory is process-global and restored when Invoke returns — before a job
// finishes — so a goroutine could not safely rely on it. Resolving the paths
// while still inside the root removes the dependency entirely rather than
// racing on it.
//
// Confinement is checked before this runs, so a path that escapes the root is
// already refused and never reaches here.
func absolutizeRequestPaths(raw json.RawMessage, root string) json.RawMessage {
	var input any
	if len(raw) == 0 || json.Unmarshal(raw, &input) != nil {
		return raw
	}

	var walk func(key string, v any) any
	walk = func(key string, v any) any {
		switch t := v.(type) {
		case string:
			if !isPathKey(key) || t == "" || isAbsolutePath(t) {
				return t
			}
			return filepath.ToSlash(filepath.Join(root, filepath.FromSlash(t)))
		case []any:
			out := make([]any, len(t))
			for i, e := range t {
				out[i] = walk(key, e)
			}
			return out
		case map[string]any:
			out := make(map[string]any, len(t))
			for k, e := range t {
				out[k] = walk(k, e)
			}
			return out
		}
		return v
	}

	rewritten, err := json.Marshal(walk("", input))
	if err != nil {
		return raw
	}
	return rewritten
}

// requestIDFrom recovers the host's request_id from a body that failed strict
// decoding.
//
// A refusal must still be correlatable: minting a fresh id on the failure path
// leaves the host unable to match the error to the call that caused it, which
// is exactly when it needs to. Decoded loosely on purpose — the body is
// already known to be unacceptable, and the id is the one thing worth
// salvaging from it.
func requestIDFrom(raw []byte) string {
	var probe struct {
		RequestID string `json:"request_id"`
	}
	if json.Unmarshal(raw, &probe) == nil {
		if id := strings.TrimSpace(probe.RequestID); id != "" {
			return id
		}
	}
	return newRequestID()
}

// isPathKey reports whether a request field carries a filesystem path.
//
// Derived from the tools' own schemas rather than guessed: the substrings
// cover every path-bearing field across the toolbox (input_path, output_dir,
// audio_path, srt_path, lut_path, library_dir, evidence_dir, source,
// rendered_file, workspace_path, and the bare input/output pair).
func isPathKey(key string) bool {
	if key == "" {
		return false
	}
	k := strings.ToLower(key)
	switch k {
	case "input", "output", "source", "segments":
		return true
	// output_format and source_type name a codec and a kind, not a location.
	case "output_format", "source_type", "profile":
		return false
	}
	for _, frag := range []string{"_path", "path_", "_dir", "_file", "filename"} {
		if strings.Contains(k, frag) {
			return true
		}
	}
	return strings.HasSuffix(k, "path") || strings.HasSuffix(k, "dir")
}

// escapesRoot reports whether a slash-separated relative path leaves the root
// it is measured from.
//
// A leading "../" is the obvious case; a later one matters just as much,
// because "renders/../../x" lands outside too. Checked segment by segment
// rather than by prefix so the deeper form cannot slip through.
func escapesRoot(rel string) bool {
	depth := 0
	for _, seg := range strings.Split(rel, "/") {
		switch seg {
		case "", ".":
		case "..":
			depth--
			if depth < 0 {
				return true
			}
		default:
			depth++
		}
	}
	return false
}
