package module

import (
	"encoding/json"
	"strings"
	"testing"
)

// The operator ruling requires a FALSIFICATION test: a wrong contract_version
// must be refused before any v2 behavioural guarantee is relied upon.
//
// This is the test that would have prevented every inference in this module.
// Confinement authority, deadline semantics and grant exhaustiveness were all
// inferred from what a host's validator happened to accept, and none could be
// pinned or fail when it drifted, because nothing versioned behaviour.
func TestWrongContractVersionIsRefused(t *testing.T) {
	body := []byte(`{
	  "tool":"media_probe",
	  "input":{"input":"x.mp4"},
	  "contract_version":"xibodev.module/v99"
	}`)

	env := Invoke(CapToolsRun, body)
	if env.OK {
		t.Fatal("a host naming a different behavioural contract was served anyway")
	}
	if env.Error.Code != "contract_incompatible" {
		t.Errorf("code = %q, want contract_incompatible", env.Error.Code)
	}

	// The refusal must name BOTH sides and a remedy. "Incompatible" alone
	// leaves an operator with nothing to act on.
	msg := env.Error.Message
	for _, want := range []string{"xibodev.module/v99", ContractVersion} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal does not name %q: %q", want, msg)
		}
	}
	if _, present := env.Error.Details["remedy"]; !present {
		t.Error("the refusal carries no remedy")
	}

	// Refused BEFORE the tool runs: a behavioural mismatch means the two sides
	// disagree about what the guarantees mean, and discovering that after the
	// work is how the inferences got made in the first place.
	if len(env.Execution.Artifacts) != 0 {
		t.Error("a refused invocation produced artifacts")
	}
}

// The matching version proceeds. A pin that refuses everything is not a pin.
func TestMatchingContractVersionProceeds(t *testing.T) {
	body := []byte(`{
	  "tool":"media_probe",
	  "input":{"input":"definitely-not-here.mp4"},
	  "contract_version":"` + ContractVersion + `"
	}`)

	env := Invoke(CapToolsRun, body)
	// The probe fails because the file is absent — that is the tool talking,
	// which proves the contract check passed and execution was reached.
	if env.Error != nil && env.Error.Code == "contract_incompatible" {
		t.Errorf("the module refused its own contract version: %s", env.Error.Message)
	}
}

// An ABSENT contract_version is permitted. A v1 host does not know the field
// exists, and refusing it would break every existing caller — the public
// compatibility the operator required preserved.
func TestAbsentContractVersionIsPermitted(t *testing.T) {
	env := Invoke(CapToolsRun, []byte(`{"tool":"media_probe","input":{"input":"nope.mp4"}}`))
	if env.Error != nil && env.Error.Code == "contract_incompatible" {
		t.Error("a request omitting contract_version was refused; v1 hosts do not send it")
	}
}

// Pinning is NOT negotiation. The descriptor must publish exactly one
// behavioural contract: a list validated by a membership test is what let
// ProtocolVersions look like a choice while being a constant, and the ruling
// forbids negotiation-shaped semantics containing one value.
func TestContractVersionIsSingularNotAList(t *testing.T) {
	env := Describe("test")
	if !env.OK {
		t.Fatalf("describe failed: %+v", env.Error)
	}
	raw, err := json.Marshal(env.Result)
	if err != nil {
		t.Fatal(err)
	}
	var probe map[string]any
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
	v, present := probe["contract_version"]
	if !present {
		t.Fatal("the descriptor publishes no contract_version; a host cannot pin what it cannot read")
	}
	if _, isList := v.([]any); isList {
		t.Error("contract_version is a list; that reads as negotiable and the ruling forbids it")
	}
	if s, _ := v.(string); s != ContractVersion {
		t.Errorf("contract_version = %v, want %q", v, ContractVersion)
	}

	// And the wire protocol stays distinct: versioning behaviour is not
	// versioning the wire format.
	if probe["protocol_versions"] == nil {
		t.Error("protocol_versions disappeared; the wire identity is separate and still required")
	}
}

// A run must say WHICH contract governed it.
//
// Before this, "the host pinned v2" and "the host said nothing and was served
// under v1" were both ok:true with identical envelopes. Two states, one
// observable value — the shape this whole phase kept finding, and the one
// facet-studio found in their own gate where absent and wrong both reported
// not-ok.
//
// It does not lie today, because no v2 guarantee yet differs from v1 here. It
// becomes a lie the moment one does, and a consumer cannot then tell a v2
// guarantee from a v1 coincidence. Fixed before it is load-bearing rather
// than after.
func TestARunReportsWhichContractGovernedIt(t *testing.T) {
	pinned := Invoke(CapToolsRun, []byte(`{
	  "tool":"media_probe","input":{"input":"nope.mp4"},
	  "contract_version":"`+ContractVersion+`"}`))
	if got := pinned.Execution.ContractVersion; got != ContractVersion {
		t.Errorf("a v2-pinned run reports contract_version %q, want %q", got, ContractVersion)
	}

	v1 := Invoke(CapToolsRun, []byte(`{"tool":"media_probe","input":{"input":"nope.mp4"}}`))
	if got := v1.Execution.ContractVersion; got != "" {
		t.Errorf("a run with no contract named reports %q; it must be omitted so a "+
			"v1 caller sees what it saw before", got)
	}

	// The point of the field: the two must be DISTINGUISHABLE. Both are ok,
	// and before this they were identical.
	if pinned.Execution.ContractVersion == v1.Execution.ContractVersion {
		t.Error("a v2-pinned run and a v1 fallback are indistinguishable in the envelope")
	}
}

// Three outcomes, three observable states. Absent is served under v1, matching
// is served under v2, wrong is refused — and no two of them look alike.
func TestThreeContractOutcomesAreDistinct(t *testing.T) {
	body := func(extra string) []byte {
		return []byte(`{"tool":"media_probe","input":{"input":"nope.mp4"}` + extra + `}`)
	}
	absent := Invoke(CapToolsRun, body(""))
	matched := Invoke(CapToolsRun, body(`,"contract_version":"`+ContractVersion+`"`))
	wrong := Invoke(CapToolsRun, body(`,"contract_version":"xibodev.module/v99"`))

	if wrong.OK {
		t.Error("a wrong contract was served")
	}
	if wrong.Error.Code != "contract_incompatible" {
		t.Errorf("wrong contract code = %q", wrong.Error.Code)
	}
	// Absent and matched are both SERVED — refusing absent would break every
	// v1 caller, which §10 forbids.
	for name, env := range map[string]Envelope{"absent": absent, "matched": matched} {
		if env.Error != nil && env.Error.Code == "contract_incompatible" {
			t.Errorf("%s contract was refused; §10 requires v1 modules keep working", name)
		}
	}
	// And served-under-v1 is not served-under-v2.
	if absent.Execution.ContractVersion == matched.Execution.ContractVersion {
		t.Error("served-under-v1 and served-under-v2 report the same thing")
	}
}
