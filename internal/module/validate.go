package module

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// ValidateEnvelopeBytes enforces the module-boundary rules on raw stdout bytes.
//
// It mirrors the host's modproto.ValidateEnvelope. Facet cannot import that
// package (Go forbids importing another module's internal/, and facet-studio is
// unpublished), so this is a second independent implementation of one contract.
// That is exactly why both lanes cross-parse each other's fixtures: a rule that
// drifts here breaks a test rather than production.
//
// Facet validates its OWN output with this, not just the host's fixtures. A
// validator only used on someone else's data proves nothing about your own.
func ValidateEnvelopeBytes(raw []byte) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return fmt.Errorf("stdout is empty; expected exactly one JSON envelope")
	}

	// stdout purity. The host found an inversion in its own version of this
	// check worth guarding against here: detecting trailing content by
	// attempting a SECOND json.Decode only catches a trailing valid DOCUMENT.
	// A trailing log line makes that decode fail, and the failure reads as
	// "nothing follows" — so the likeliest real violation (diagnostics merged
	// into stdout) sails through. Decode one value, then require the remainder
	// to be whitespace only.
	dec := json.NewDecoder(bytes.NewReader(raw))
	var probe json.RawMessage
	if err := dec.Decode(&probe); err != nil {
		return fmt.Errorf("stdout is not a JSON envelope: %w", err)
	}
	rest := new(bytes.Buffer)
	if _, err := rest.ReadFrom(dec.Buffered()); err != nil {
		return fmt.Errorf("reading trailing bytes: %w", err)
	}
	trailing := bytes.TrimSpace(rest.Bytes())
	if len(trailing) > 0 {
		return fmt.Errorf("stdout carries %d trailing non-whitespace bytes after the envelope: %s",
			len(trailing), boundedBytes(trailing))
	}

	// Decode strictly so an unknown field is a signal rather than silence.
	var env Envelope
	if err := json.Unmarshal(probe, &env); err != nil {
		return fmt.Errorf("envelope does not decode: %w", err)
	}

	if env.Protocol != Protocol {
		return fmt.Errorf("protocol = %q, want %q", env.Protocol, Protocol)
	}
	if strings.TrimSpace(env.Module) == "" {
		return fmt.Errorf("module is empty")
	}
	if strings.TrimSpace(env.RequestID) == "" {
		return fmt.Errorf("request_id is empty")
	}
	if env.OK && env.Error != nil {
		return fmt.Errorf("ok:true carries an error")
	}
	if !env.OK && env.Error == nil {
		return fmt.Errorf("ok:false carries no error")
	}
	if !env.OK && env.Result != nil {
		return fmt.Errorf("ok:false carries a result")
	}
	// ok:true with NEITHER result nor error is the case a real module reaches
	// by forgetting to set Result: it decodes to a typed zero value rather than
	// an obvious error, so it has to be checked explicitly.
	if env.OK && env.Result == nil {
		return fmt.Errorf("ok:true carries neither result nor error")
	}

	// operation names the VERB, never the capability. The capability travels in
	// the request and is correlated by request_id, so an operation carrying a
	// capability ID means the module confused the two.
	//
	// The set is CLOSED. An earlier version of this check also accepted
	// "module.describe"/"module.invoke", which let Facet's own non-canonical
	// values pass its own validator — a validator that accepts your mistakes
	// is worse than none, because it certifies them.
	if env.Operation != OpDescribe && env.Operation != OpInvoke {
		return fmt.Errorf(
			"operation = %q; must be exactly %q or %q, never a capability ID",
			env.Operation, OpDescribe, OpInvoke)
	}

	// Collections are [] or {}, never null: a null collection forces every
	// consumer into a nil check the contract says is unnecessary.
	if err := requireNonNull(probe, "warnings"); err != nil {
		return err
	}
	if env.Error != nil && env.Error.Details == nil {
		return fmt.Errorf("error.details is null")
	}
	if env.Execution.Artifacts == nil {
		return fmt.Errorf("execution.artifacts is null")
	}

	for i, a := range env.Execution.Artifacts {
		if isAbsolutePath(a.Path) {
			return fmt.Errorf("artifact[%d] path is absolute: %q", i, a.Path)
		}
		if a.Digest != "" && !ValidDigest(a.Digest) {
			return fmt.Errorf("artifact[%d] digest is not sha256:<64 lowercase hex>: %q", i, a.Digest)
		}
	}
	return nil
}

// DefaultMaxOutputBytes bounds a module response. A large payload belongs in an
// artifact pointer, not inlined in result: inlining floods the agent's context
// and defeats the artifact model. The host enforces its own limit from the
// request; this is Facet's own default when none is supplied.
const DefaultMaxOutputBytes = 256 * 1024

// ValidateEnvelopeSized applies the structural rules plus an output-size bound.
//
// Size is deliberately separate from ValidateEnvelopeBytes: an oversized
// payload is not malformed, so it cannot be caught by shape checks. It is a
// read-time bound, which is why the host enforces it via max_output_bytes
// rather than in the validator.
func ValidateEnvelopeSized(raw []byte, maxBytes int) error {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxOutputBytes
	}
	if len(raw) > maxBytes {
		return fmt.Errorf(
			"module output is %d bytes, exceeding the %d-byte bound; "+
				"large results must be returned as artifact pointers, not inlined",
			len(raw), maxBytes)
	}
	return ValidateEnvelopeBytes(raw)
}

// Expectation carries what the DECLARED contract said, so a response can be
// checked against it. Two of the protocol's violations are not structurally
// detectable from a single envelope — they need this comparison.
type Expectation struct {
	// RequestID the host generated. A module must echo it verbatim.
	RequestID string
	// CostKnown is the capability's declared Effects.CostKnown. When false, a
	// numeric cost in the response is under-reporting, not information.
	CostKnown bool
	// CostKnownSet distinguishes "declared false" from "not supplied", so an
	// absent expectation never silently asserts one.
	CostKnownSet bool
}

// ValidateEnvelopeAgainst applies the structural rules and then the
// contextual ones that need the declared contract.
//
// Both extra checks catch under-reporting rather than malformation, which is
// why they cannot live in the structural pass: the bytes are well-formed and
// only a comparison reveals the lie.
func ValidateEnvelopeAgainst(raw []byte, exp Expectation) error {
	if err := ValidateEnvelopeBytes(raw); err != nil {
		return err
	}
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("envelope does not decode: %w", err)
	}

	if exp.RequestID != "" && env.RequestID != exp.RequestID {
		return fmt.Errorf("request_id %q was not echoed verbatim; module returned %q",
			exp.RequestID, env.RequestID)
	}

	// An unpriced capability reporting a number understates real spend. null is
	// the honest answer when the cost is genuinely unknown.
	if exp.CostKnownSet && !exp.CostKnown {
		if c := env.Execution.EstimatedCost; c != nil {
			return fmt.Errorf(
				"capability declared cost_known=false but reported estimated_cost %v; unknown cost must stay null", *c)
		}
		if c := env.Execution.ActualCost; c != nil {
			return fmt.Errorf(
				"capability declared cost_known=false but reported actual_cost %v; unknown cost must stay null", *c)
		}
	}
	return nil
}

// requireNonNull reports a JSON null for a field the contract says is always a
// collection. Go decodes null into a nil slice, which is indistinguishable from
// an absent field after unmarshalling, so this inspects the raw bytes.
func requireNonNull(raw json.RawMessage, field string) error {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return fmt.Errorf("envelope is not an object: %w", err)
	}
	v, present := probe[field]
	if !present {
		return fmt.Errorf("%s is absent; must be [] never omitted", field)
	}
	if string(bytes.TrimSpace(v)) == "null" {
		return fmt.Errorf("%s is null; must be []", field)
	}
	return nil
}

// ValidDigest reports whether s is "sha256:" + exactly 64 lowercase hex digits.
// Bare hex, uppercase hex, a truncated value, and a wrong algorithm are all
// rejected rather than repaired.
func ValidDigest(s string) bool {
	const prefix = "sha256:"
	if !strings.HasPrefix(s, prefix) {
		return false
	}
	hex := s[len(prefix):]
	if len(hex) != 64 {
		return false
	}
	for _, r := range hex {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// isAbsolutePath checks BOTH POSIX and Windows shapes regardless of the host
// OS, because a module may run on a different platform than the host.
func isAbsolutePath(p string) bool {
	if p == "" {
		return false
	}
	if p[0] == '/' || p[0] == '\\' {
		return true
	}
	// Drive-letter form: C:\ or C:/
	if len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		c := p[0]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			return true
		}
	}
	return false
}

func boundedBytes(b []byte) string {
	const limit = 120
	if len(b) > limit {
		return string(b[:limit]) + "..."
	}
	return string(b)
}
