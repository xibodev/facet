package module

import (
	"encoding/json"
	"testing"
)

// A host request carries protocol fields the module does not act on. They must
// be MODELLED, not tolerated: rejecting a field the host always sends would
// refuse every genuine call, and ignoring unknown fields would silently discard
// a misspelled `consent`.
//
// This is the check for a regression I shipped and caught one tick later:
// making the request strict without modelling protocol/capability/grants/
// deadline_ms/max_output_bytes rejected every real host invocation.
func TestRealHostRequestIsAccepted(t *testing.T) {
	body := []byte(`{
	  "protocol":"xibodev.module/v1",
	  "capability":"creative.tools.run",
	  "request_id":"req_host",
	  "tool":"media_probe",
	  "input":{"input":"../../projects/cinematic-documentary/assets/video/shot1_raw.mp4"},
	  "roots":{"project_root":{"path":"/abs/project","mode":"rw"}},
	  "grants":{"network":[],"credentials":[],"paid_providers":[],"publish":false,"subprocess":["ffprobe"]},
	  "deadline_ms":600000,
	  "max_output_bytes":262144
	}`)
	env := Invoke(CapToolsRun, body)
	if !env.OK {
		t.Fatalf("a realistic host request was refused: %+v", env.Error)
	}
}

// A field the module does not know is still refused, because a misspelled
// consent must never be silently discarded.
func TestUnknownRequestFieldStillRefused(t *testing.T) {
	env := Invoke(CapToolsRun, []byte(`{"tool":"media_probe","input":{},"binarys":{}}`))
	if env.OK {
		t.Fatal("an unknown request field was accepted")
	}
	if env.Error.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", env.Error.Code)
	}
}

// Grants are what the host authorized for THIS invocation, and consent is a
// human approving the spend. They are different things: running on consent
// alone would let a module reach a provider the host deliberately withheld.
func TestGrantGateRefusesAnUngrantedProvider(t *testing.T) {
	body, _ := json.Marshal(Request{
		Tool:    "gflow_image",
		Input:   json.RawMessage(`{"prompt":"x","model":"narwhal","aspect_ratio":"landscape","count":1,"output_path":"a.png"}`),
		Consent: &Consent{PaidGenerationApproved: true, ApprovedBy: "operator"},
		Grants:  &Grants{PaidProviders: []string{}},
	})
	env := Invoke(CapToolsRun, body)
	if env.OK {
		t.Fatal("a paid tool ran with human consent but no host grant")
	}
	if env.Error.Code != "permission_denied" {
		t.Errorf("code = %q, want permission_denied", env.Error.Code)
	}
}

// The grant is checked BEFORE consent, so a granted provider still requires a
// human. Neither gate substitutes for the other.
func TestGrantDoesNotSubstituteForConsent(t *testing.T) {
	body, _ := json.Marshal(Request{
		Tool:   "gflow_image",
		Input:  json.RawMessage(`{"prompt":"x","model":"narwhal","aspect_ratio":"landscape","count":1,"output_path":"a.png"}`),
		Grants: &Grants{PaidProviders: []string{"google_flow"}},
	})
	env := Invoke(CapToolsRun, body)
	if env.OK {
		t.Fatal("a granted provider ran without human consent")
	}
	if env.Error.Code != "consent_required" {
		t.Errorf("code = %q, want consent_required", env.Error.Code)
	}
}

// A request with no grants block at all is the CLI and every existing consumer.
// Absent grants must not be read as "nothing granted", or the human-facing tool
// would stop working.
func TestAbsentGrantsDoNotDenyTheCLI(t *testing.T) {
	body, _ := json.Marshal(Request{
		Tool:    "gflow_image",
		Input:   json.RawMessage(`{"prompt":"x","model":"narwhal","aspect_ratio":"landscape","count":1,"output_path":"a.png"}`),
		Consent: &Consent{PaidGenerationApproved: true, ApprovedBy: "operator"},
	})
	env := Invoke(CapToolsRun, body)
	if !env.OK && env.Error.Code == "permission_denied" {
		t.Error("absent grants were treated as an empty grant list, denying the CLI")
	}
}

// Every paid tool must map to a provider Facet actually declares, or the gate
// would deny a tool the host correctly granted.
func TestEveryPaidToolMapsToADeclaredProvider(t *testing.T) {
	env := Describe("test")
	desc, ok := env.Result.(Descriptor)
	if !ok {
		t.Fatalf("result is not a Descriptor: %T", env.Result)
	}
	for tool := range paidTools {
		provider := paidProviderFor(tool)
		if provider == "" {
			t.Errorf("paid tool %q maps to no provider; it would be denied always", tool)
			continue
		}
		if !contains(desc.Permissions.PaidProviders, provider) {
			t.Errorf("paid tool %q needs provider %q which is not declared in permissions",
				tool, provider)
		}
	}
}
