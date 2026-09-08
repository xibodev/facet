// Package module projects Facet's existing mechanical toolbox onto the shared
// host module protocol. It is a strict adapter: it adds no execution system,
// duplicates no tool logic, and reaches the toolbox only through toolbox.CLI,
// the same entry point the human-facing `facet tools` command uses.
package module

import (
	"crypto/rand"
	"encoding/hex"
)

// Identity. ModuleID was chosen by the operator; Protocol is pending host ack.
const (
	ModuleID   = "xibodev.facet"
	ModuleName = "Facet"
	Protocol   = "xibodev.module/v1"

	// Operation is the VERB, exactly one of these two — never a capability ID.
	// The capability travels in the request and is correlated by request_id.
	// Pinned to the host's modproto.OperationDescribe / OperationInvoke.
	OpDescribe = "describe"
	OpInvoke   = "invoke"
)

// Capability IDs. The host addresses these; it never names a Facet tool.
const (
	CapToolsList       = "creative.tools.list"
	CapToolsDescribe   = "creative.tools.describe"
	CapToolsEstimate   = "creative.tools.estimate"
	CapToolsRun        = "creative.tools.run"
	CapOutputReview    = "creative.output.review"
	CapArtifactInspect = "creative.artifact.inspect"
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
}

// Descriptor declares exactly the twelve agreed fields, in order.
//
// The schema maps carry JSON Schema documents keyed by schema ID, so a
// capability can reference a schema by ID rather than inlining it.
type Descriptor struct {
	Module           string         `json:"module"`
	Name             string         `json:"name"`
	Version          string         `json:"version"`
	ProtocolVersions []string       `json:"protocol_versions"`
	Capabilities     []Capability   `json:"capabilities"`
	RequestSchemas   map[string]any `json:"request_schemas"`
	ResultSchemas    map[string]any `json:"result_schemas"`
	ArtifactSchemas  map[string]any `json:"artifact_schemas"`
	AgentOverlays    []Overlay      `json:"agent_overlays"`
	Skills           []Skill        `json:"skills"`
	Permissions      Permissions    `json:"permissions"`
	Requirements     []Requirement  `json:"requirements"`
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
