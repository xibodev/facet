// Package module projects Facet's existing mechanical toolbox onto the shared
// host module protocol. It is a strict adapter: it adds no execution system,
// duplicates no tool logic, and reaches the toolbox only through toolbox.CLI,
// the same entry point the human-facing `facet tools` command uses.
package module

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/xibodev/facet/internal/toolbox"
)

// Identity. ModuleID was chosen by the operator; Protocol is pending host ack.
const (
	ModuleID   = "xibodev.facet"
	ModuleName = "Facet"
	Protocol   = "xibodev.module/v1"

	// ContractVersion is the BEHAVIOURAL contract Facet implements, distinct
	// from Protocol, which versions only the wire format.
	//
	// Everything Facet believed about roots, deadline semantics, grant
	// exhaustiveness and confinement authority was inferred from what a host's
	// validator happened to accept. Nothing versioned those behaviours, so an
	// inference could not be pinned and could not fail a test when it drifted.
	// That is the defect this closes.
	//
	// EXACT PINNING, NOT NEGOTIATION (operator ruling). A module and a host
	// either agree on this identity or they do not interoperate. There are no
	// ranges, no highest-common selection, no downgrade and no fallback —
	// those are negotiation, and negotiation is deferred until several
	// behavioural versions genuinely coexist. A single-valued "negotiation" is
	// the shape that let ProtocolVersions look like a choice while being a
	// membership test against one constant.
	ContractVersion = "xibodev.module/v2"

	// Operation is the VERB, exactly one of these two — never a capability ID.
	// The capability travels in the request and is correlated by request_id.
	// Pinned to the host's modproto.OperationDescribe / OperationInvoke.
	OpDescribe = "describe"
	OpInvoke   = "invoke"
)

// ProviderVaries is declared by a capability that dispatches any tool, so the
// provider is not knowable until one is selected. It is a DECLARATION value,
// and an invocation replaces it with the provider actually used.
const ProviderVaries = "varies"

// ArtifactKindOutput is the kind every Facet artifact carries.
//
// Facet emits ONE kind: a file a tool wrote. The host validates an artifact's
// kind against the producing capability's artifact_schemas, so this must
// appear there or every artifact-producing run is refused.
const ArtifactKindOutput = "output"

// Capability IDs. The host addresses these; it never names a Facet tool.
const (
	CapToolsList       = "creative.tools.list"
	CapToolsDescribe   = "creative.tools.describe"
	CapToolsEstimate   = "creative.tools.estimate"
	CapToolsRun        = "creative.tools.run"
	CapOutputReview    = "creative.output.review"
	CapArtifactInspect = "creative.artifact.inspect"
	CapJobsStatus      = "creative.jobs.status"
)

// Envelope is the only thing written to stdout. Field order matches the
// cross-repo protocol exactly: protocol, module, operation, request_id, ok,
// result|error, warnings, execution.
type Envelope struct {
	Protocol  string    `json:"protocol"`
	Module    string    `json:"module"`
	Operation string    `json:"operation"`
	RequestID string    `json:"request_id"`
	OK        bool      `json:"ok"`
	Result    any       `json:"result,omitempty"`
	Error     *Error    `json:"error,omitempty"`
	Warnings  []string  `json:"warnings"`
	Execution Execution `json:"execution"`
}

// Error carries a stable code, a human message, a retryable flag, and bounded
// details. Codes pass through from the toolbox unchanged where one exists.
type Error struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details"`
}

// Execution declares honest effect and cost facts.
//
// EstimatedCost and ActualCost are nullable on purpose: null means the cost is
// genuinely unknown, 0 means it is genuinely free. Never collapse the two.
type Execution struct {
	Local          bool       `json:"local"`
	Network        bool       `json:"network"`
	ExternalWrites bool       `json:"external_writes"`
	Provider       string     `json:"provider"`
	EstimatedCost  *float64   `json:"estimated_cost"`
	ActualCost     *float64   `json:"actual_cost"`
	Artifacts      []Artifact `json:"artifacts"`
	// ContractVersion names the behavioural contract that GOVERNED this run,
	// omitted when the caller named none.
	//
	// Without it, "the host pinned v2" and "the host said nothing and was
	// served under v1" are both ok:true and indistinguishable to a reader of
	// the envelope. That does not lie today, because no v2 guarantee yet
	// differs from v1 here — it becomes a lie the moment one does, and a
	// consumer cannot then tell a v2 guarantee from a v1 coincidence.
	//
	// facet-studio found the same two-states-one-value collapse in their own
	// gate (absent and wrong both reported not-ok) and split it into three
	// outcomes. This is the mirror on the module side: they report which
	// contract they may RELY on, this reports which contract a run was SERVED
	// under.
	//
	// Additive and omitted when absent, per §10: a v1 caller sees exactly what
	// it saw before.
	ContractVersion string `json:"contract_version,omitempty"`
}

// Artifact is a pointer to a file a run produced, never its contents.
//
// Path is relative to Root, a logical root the host named in the request. An
// absolute path is a protocol violation: it makes root confinement uncheckable.
// Digest is "sha256:" + lowercase hex.
type Artifact struct {
	ID        string `json:"id,omitempty"`
	Kind      string `json:"kind"`
	Path      string `json:"path"`
	Root      string `json:"root,omitempty"`
	MediaType string `json:"media_type,omitempty"`
	Bytes     int64  `json:"bytes,omitempty"`
	Digest    string `json:"digest,omitempty"`
	Title     string `json:"title,omitempty"`
	// Presentation names the host rendering primitive for this artefact.
	//
	// It exists for what a media type cannot express: a scene plan is
	// application/json and also a timeline, and only the module knows that.
	// The host honours a known value and ignores an unknown one, so a module
	// names a primitive the host owns rather than inventing one.
	Presentation string `json:"presentation,omitempty"`
}

// Descriptor declares exactly the twelve agreed fields, in order.
//
// The schema maps carry JSON Schema documents keyed by schema ID, so a
// capability can reference a schema by ID rather than inlining it.
type Descriptor struct {
	Module           string   `json:"module"`
	Name             string   `json:"name"`
	Version          string   `json:"version"`
	ProtocolVersions []string `json:"protocol_versions"`
	// ContractVersion is the single behavioural contract this module
	// implements. Singular by design: a list would read as negotiable, and
	// ProtocolVersions already proved that a list validated by a membership
	// test looks like a choice while being a constant.
	ContractVersion string         `json:"contract_version"`
	Capabilities    []Capability   `json:"capabilities"`
	RequestSchemas  map[string]any `json:"request_schemas"`
	ResultSchemas   map[string]any `json:"result_schemas"`
	ArtifactSchemas map[string]any `json:"artifact_schemas"`
	AgentOverlays   []Overlay      `json:"agent_overlays"`
	Skills          []Skill        `json:"skills"`
	Permissions     Permissions    `json:"permissions"`
	Requirements    []Requirement  `json:"requirements"`

	// --- xibodev.module/v2 semantic layer ---
	//
	// These ride in the SAME document as the v1 fields above. §10 requires a
	// v1 host to ignore fields it does not recognise, and facet-studio pinned
	// that direction by test (37835f6) precisely because "works because the
	// decoder is tolerant" and "works because the contract requires it"
	// produce the same observable.
	//
	// A v1 host therefore sees exactly the v1 descriptor it always saw; a v2
	// host reads contract_version, passes the gate, and finds the semantics.
	//
	// omitempty is deliberate: a module that has not built its v2 payload must
	// publish NOTHING rather than an empty list, because an empty `operations`
	// is an affirmative claim of having none.
	Operations    []toolbox.V2Operation             `json:"operations,omitempty"`
	ArtifactKinds map[string]toolbox.V2ArtifactKind `json:"artifact_kinds,omitempty"`
}

// Capability describes one addressable operation and its honest effects.
//
// RequestSchema and ResultSchema are IDs INTO the descriptor's schema maps. A
// reference that resolves to nothing means the host cannot validate anything
// the capability sends or returns, so every ID here must be a present key.
type Capability struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Summary         string   `json:"summary"`
	RequestSchema   string   `json:"request_schema"`
	ResultSchema    string   `json:"result_schema"`
	ArtifactSchemas []string `json:"artifact_schemas"`
	Effects         Effects  `json:"effects"`
	Skills          []string `json:"skills"`
	LongRunning     bool     `json:"long_running"`
	PollCapability  string   `json:"poll_capability,omitempty"`
}

// Effects declares what invoking a capability does, BEFORE it runs. The host
// uses it to decide whether approval is required, so it must be honest even
// when the answer is inconvenient.
//
// CostKnown distinguishes "free" from "unknown". When false the host treats the
// capability as unpriced and requires approval regardless of any number present.
type Effects struct {
	Local          bool   `json:"local"`
	Network        bool   `json:"network"`
	ExternalWrites bool   `json:"external_writes"`
	Provider       string `json:"provider"`
	CostKnown      bool   `json:"cost_known"`
}

// Overlay points at agent guidance the host may load. The host decides.
//
// Digest is "sha256:<lowercase-hex>" over the file contents. The host refuses
// to fold content into agent context without verifiable provenance, so an empty
// digest is a hard rejection rather than a missing nicety.
type Overlay struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Path   string `json:"path"`
	Digest string `json:"digest"`
	Tokens int    `json:"tokens"`
}

// Skill is progressively selected; the host must not load them all every turn.
type Skill struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Path    string `json:"path"`
	Digest  string `json:"digest"`
	Tokens  int    `json:"tokens"`
}

// Permissions states what the module needs to do its job.
//
// Every field except Publish is a LIST of named grants, not a boolean. The host
// intersects a declaration with policy and grants per invocation, which a
// boolean cannot express: "network: true" says nothing about which hosts, and
// "subprocess: true" nothing about which binaries.
type Permissions struct {
	FilesystemRead  []string `json:"filesystem_read"`
	FilesystemWrite []string `json:"filesystem_write"`
	Network         []string `json:"network"`
	Credentials     []string `json:"credentials"`
	PaidProviders   []string `json:"paid_providers"`
	Publish         bool     `json:"publish"`
	Subprocess      []string `json:"subprocess"`
}

// Requirement is an external dependency the module cannot install itself.
type Requirement struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Required bool   `json:"required"`
	For      string `json:"for"`
}

func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "req_unavailable"
	}
	return "req_" + hex.EncodeToString(b)
}

// localExec describes a purely local, free, side-effect-free operation.
func localExec(provider string) Execution {
	zero := 0.0
	return Execution{
		Local:         true,
		Network:       false,
		Provider:      provider,
		EstimatedCost: &zero,
		ActualCost:    &zero,
		Artifacts:     []Artifact{},
	}
}

func fail(op, reqID, code, message string, details map[string]any, retryable bool) Envelope {
	if details == nil {
		details = map[string]any{}
	}
	return Envelope{
		Protocol:  Protocol,
		Module:    ModuleID,
		Operation: op,
		RequestID: reqID,
		OK:        false,
		Error:     &Error{Code: code, Message: message, Retryable: retryable, Details: details},
		Warnings:  []string{},
		Execution: localExec("facet"),
	}
}

// Usage reports a malformed command as a protocol envelope rather than as bare
// text, so the host never has to parse stderr to learn that a call was wrong.
func Usage(message string) Envelope {
	return fail(OpInvoke, newRequestID(), "invalid_request", message, nil, false)
}

// InputError reports an unreadable --input argument.
func InputError(op, path string, err error) Envelope {
	verb := OpInvoke
	if op == OpDescribe {
		verb = OpDescribe
	}
	return fail(verb, newRequestID(), "input_not_found",
		"request input could not be read",
		map[string]any{"path": path, "error": err.Error()}, false)
}
