package module

import (
	"encoding/json"
	"github.com/xibodev/facet/internal/toolbox"
	"os"
	"path/filepath"
	"strings"
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
	// A real host grants a root that EXISTS and that CONTAINS the media the
	// request names — paths leaving the granted root are refused, for reads as
	// well as writes. Granting the package directory while reading fixtures
	// from the repository root would be a request no real host would send.
	//
	// So the fixture grants the repository root and names the media relative
	// to it, which is what a host does.
	projectRoot := t.TempDir()
	wav := append([]byte{
		'R', 'I', 'F', 'F', 36, 0, 0, 0, 'W', 'A', 'V', 'E',
		'f', 'm', 't', ' ', 16, 0, 0, 0, 1, 0, 1, 0,
		0x40, 0x1f, 0, 0, 0x40, 0x1f, 0, 0, 1, 0, 8, 0,
		'd', 'a', 't', 'a', 0, 0, 0, 0,
	})
	if err := os.WriteFile(filepath.Join(projectRoot, "source.wav"), wav, 0600); err != nil {
		t.Fatal(err)
	}
	root, err := json.Marshal(projectRoot)
	if err != nil {
		t.Fatal(err)
	}

	body := []byte(`{
	  "protocol":"xibodev.module/v1",
	  "capability":"creative.tools.run",
	  "request_id":"req_host",
	  "tool":"media_probe",
	  "input":{"input":"source.wav"},
	  "roots":{"project_root":{"path":` + string(root) + `,"mode":"rw"}},
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
	// ESTIMATE rather than run: the grant path is identical and estimation
	// never contacts a provider. Using run here generated a real image and
	// spent real money on every suite run.
	env := Invoke(CapToolsEstimate, body)
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
	for _, tool := range toolbox.ChargeableTools() {
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

// The host's Grants is a value type with no omitempty, so `grants` is present
// on EVERY request it sends. These are the two shapes it actually produces,
// pinned so the gate is tested against the wire rather than against Facet's
// idea of it.
//
// The first shape was a real host bug: Grants was never populated, so it
// marshalled as present-with-null-fields. Present means host-mediated, null
// means nothing authorized, so it denied every paid call regardless of what a
// human approved. Denying it is correct — the module cannot tell an empty
// grant from a deliberate one, and must not assume authority it was not given.
func TestGateAgainstRealHostWireShapes(t *testing.T) {
	paid := `"tool":"gflow_image","input":{"prompt":"x","model":"narwhal","aspect_ratio":"landscape","count":1,"output_path":"a.png"},` +
		`"consent":{"paid_generation_approved":true,"approved_by":"operator"},`

	t.Run("present with null fields is denied", func(t *testing.T) {
		env := Invoke(CapToolsRun, []byte(`{`+paid+
			`"grants":{"network":null,"credentials":null,"paid_providers":null,"publish":false,"subprocess":null}}`))
		if env.OK {
			t.Fatal("an all-null grant authorized a paid provider")
		}
		if env.Error.Code != "permission_denied" {
			t.Errorf("code = %q, want permission_denied", env.Error.Code)
		}
	})

	t.Run("declared providers granted pass the gate", func(t *testing.T) {
		// ESTIMATE, not run. The gate is what is under test, and estimation
		// exercises the same grant path without contacting a provider.
		//
		// An earlier version of this test used `run` with consent and a full
		// grant, and it worked: it generated a real image and spent real money
		// every time the suite ran. A test that bills is a bug however well
		// authorized the spending is.
		env := Invoke(CapToolsEstimate, []byte(`{`+paid+
			`"grants":{"network":["labs.google"],"credentials":null,`+
			`"paid_providers":["google_flow","openai","elevenlabs","fal","kling"],`+
			`"publish":false,"subprocess":null}}`))
		if !env.OK && env.Error.Code == "permission_denied" {
			t.Errorf("a granted provider was denied: %s", env.Error.Message)
		}
	})

	// Binaries are granted as resolved absolute paths, not via Grants.Subprocess,
	// so an empty subprocess list must never deny a local tool.
	t.Run("empty subprocess does not deny a local tool", func(t *testing.T) {
		env := Invoke(CapToolsRun, []byte(`{"tool":"media_probe",`+
			`"input":{"input":"../../assets/source.mp4"},`+
			`"grants":{"network":null,"credentials":null,"paid_providers":null,"publish":false,"subprocess":null}}`))
		if !env.OK && env.Error.Code == "permission_denied" {
			t.Error("a local tool was denied by an empty subprocess grant")
		}
	})
}

// No test may invoke a paid tool through creative.tools.run.
//
// Two did, and both worked: they carried consent, reached the provider, and
// generated real images on every suite run. Authorized spending is still
// spending, and a test suite that bills is a bug — it also made the suite
// 15x slower and its result depend on a provider being reachable.
//
// creative.tools.estimate exercises the same validation and grant path without
// contacting anyone, so there is no reason for a test to use run for a paid
// tool. This reads the test sources rather than trusting the rule to be
// remembered.
func TestNoTestInvokesAPaidToolForReal(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		body, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, block := range strings.Split(string(body), "Invoke(CapToolsRun") {
			// Look only at the request that follows each run invocation.
			window := block
			if len(window) > 400 {
				window = window[:400]
			}
			// Consent is what makes a paid run actually reach the provider.
			// Without it the gate refuses before anything is contacted, which
			// is exactly what a consent test should assert.
			if !strings.Contains(window, "paid_generation_approved") &&
				!strings.Contains(window, "PaidGenerationApproved") {
				continue
			}
			for _, tool := range toolbox.ChargeableTools() {
				if strings.Contains(window, `"`+tool+`"`) {
					t.Errorf("%s invokes paid tool %q through CapToolsRun WITH consent; "+
						"use CapToolsEstimate — a test must never bill", e.Name(), tool)
				}
			}
		}
	}
}

// The request states which protocol it speaks and which capability it
// addresses. Both were accepted and ignored, which is how a confident wrong
// answer gets produced: a future protocol may mean something different by the
// same field names, and a capability disagreeing with the verb argument means
// host and module attribute one result to different capabilities.
func TestProtocolAndCapabilityMustAgree(t *testing.T) {
	t.Run("a protocol the module does not speak is refused", func(t *testing.T) {
		env := Invoke(CapToolsList, []byte(`{"protocol":"xibodev.module/v99"}`))
		if env.OK {
			t.Fatal("a future protocol was answered as if it were v1")
		}
		if env.Error.Code != "unsupported_protocol" {
			t.Errorf("code = %q, want unsupported_protocol", env.Error.Code)
		}
	})

	t.Run("a capability disagreeing with the invocation is refused", func(t *testing.T) {
		env := Invoke(CapToolsList, []byte(`{"capability":"creative.tools.run"}`))
		if env.OK {
			t.Fatal("a mismatched capability was silently resolved")
		}
		if env.Error.Code != "invalid_request" {
			t.Errorf("code = %q, want invalid_request", env.Error.Code)
		}
	})

	t.Run("agreement passes", func(t *testing.T) {
		env := Invoke(CapToolsList,
			[]byte(`{"protocol":"xibodev.module/v1","capability":"creative.tools.list"}`))
		if !env.OK {
			t.Errorf("an agreeing request was refused: %+v", env.Error)
		}
	})

	t.Run("absent fields are not required", func(t *testing.T) {
		// The CLI sends neither.
		if env := Invoke(CapToolsList, []byte(`{}`)); !env.OK {
			t.Errorf("a request without protocol or capability was refused: %+v", env.Error)
		}
	})
}
