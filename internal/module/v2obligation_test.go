package module

import (
	"encoding/json"
	"testing"
)

// Facet publishes contract_version: xibodev.module/v2 and, today, none of the
// v2 payload that declaration promises. Measured against facet-studio's real
// gate (pkg/modprotov2, commit 6d4b8e3):
//
//	Evaluate(facet descriptor) -> pin=v2, ACCEPTED, operations=0
//
// Their gate returns no error, because json.Unmarshal does not fail on a
// missing field. So the host admits Facet to the v2 path and cannot tell that
// it declares no Operations.
//
// That gap is DELIBERATE and currently correct: the v2 payload is Phase C step
// 6 and is held pending the operator. The declaration was published early, at
// facet-studio's request, so their contract pin could run against a real module
// instead of a fixture.
//
// This test exists so the gap stays deliberate. It does not implement the wire
// and does not require the payload to exist yet — it records the OBLIGATION the
// declaration creates, and fails the day someone believes it is already met.
func TestDeclaringV2CarriesAnObligation(t *testing.T) {
	env := Describe("test")
	if !env.OK {
		t.Fatalf("describe failed: %+v", env.Error)
	}
	raw, err := json.Marshal(env.Result)
	if err != nil {
		t.Fatal(err)
	}
	var d struct {
		ContractVersion string            `json:"contract_version"`
		Operations      []json.RawMessage `json:"operations"`
		ArtifactKinds   map[string]any    `json:"artifact_kinds"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}

	if d.ContractVersion != ContractVersion {
		t.Fatalf("contract_version = %q, want %q", d.ContractVersion, ContractVersion)
	}

	// The payload is not required YET. What must hold is that the two facts
	// stay in a state someone chose: either no v2 payload at all (today), or a
	// complete one. A descriptor carrying SOME v2 fields and not others is the
	// half-populated shape §10 forbids, and it is the state a partial
	// implementation lands in.
	hasOps := len(d.Operations) > 0
	hasKinds := len(d.ArtifactKinds) > 0
	if hasOps != hasKinds {
		t.Errorf("v2 payload is half-published: operations=%d artifact_kinds=%d; "+
			"a host decoding this reads the missing half as an affirmative zero",
			len(d.Operations), len(d.ArtifactKinds))
	}
}
