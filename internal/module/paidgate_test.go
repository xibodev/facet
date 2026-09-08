package module

import (
	"encoding/json"
	"testing"
)

// The module's paidTools set and the toolbox's cost reporting must agree.
//
// They are two hand-maintained lists of the same fact — which tools can bill —
// and the comment on paidTools says only that it "mirrors" the toolbox. Every
// other pair of lists in this repo that had to agree eventually did not: the
// network hosts, the credentials, the scene-to-cut fields, the two duration
// paths, the estimate and run grant handling.
//
// The consequence here is not a wrong duration. A tool missing from paidTools
// runs without consent and spends someone's money; a tool wrongly in it is
// gated forever and can never run.
func TestPaidToolsMatchesReportedCost(t *testing.T) {
	env := Invoke(CapToolsList, []byte(`{}`))
	if !env.OK {
		t.Skipf("tool listing unavailable: %+v", env.Error)
	}
	raw, err := json.Marshal(env.Result)
	if err != nil {
		t.Fatal(err)
	}
	var probe struct {
		Output struct {
			Tools []struct {
				Name string `json:"name"`
				Cost *struct {
					Known bool `json:"known"`
				} `json:"cost"`
			} `json:"tools"`
		} `json:"output"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
	if len(probe.Output.Tools) == 0 {
		t.Fatal("the listing carries no tools; nothing was verified")
	}

	for _, tool := range probe.Output.Tools {
		if tool.Cost == nil {
			continue
		}
		costUnknown := !tool.Cost.Known
		gated := paidTools[tool.Name]

		if costUnknown && !gated {
			t.Errorf("%s reports an unknown cost but is not in paidTools; "+
				"it would run without human consent and spend real money", tool.Name)
		}
		if gated && !costUnknown {
			t.Errorf("%s is gated as paid but reports a known cost; "+
				"it requires consent it can never need", tool.Name)
		}
	}
}

// Every paid tool must map to a provider the host can grant. An unrecognised
// tool yields an empty provider, which matches no grant — so it fails closed,
// but silently and permanently.
func TestEveryPaidToolNamesAGrantableProvider(t *testing.T) {
	for tool := range paidTools {
		if provider := paidProviderFor(tool); provider == "" {
			t.Errorf("%s is gated as paid but names no provider; no grant can ever "+
				"authorise it, so it is refused forever rather than gated", tool)
		}
	}
}

// Consent and a grant are different authorisations, and neither substitutes
// for the other. A human approving the spend does not mean the host permitted
// the provider; running on consent alone would reach a provider the host
// deliberately withheld.
//
// Verified against the real binary: all four refusals below were observed
// before this test was written.
func TestConsentDoesNotSubstituteForAGrant(t *testing.T) {
	body := func(extra string) []byte {
		return []byte(`{"tool":"gflow_image","input":{"prompt":"x","output_path":"o.png"}` + extra + `}`)
	}

	// No consent at all.
	if env := Invoke(CapToolsRun, body("")); env.OK {
		t.Error("a paid tool ran with no consent")
	} else if env.Error.Code != "consent_required" {
		t.Errorf("code = %q, want consent_required", env.Error.Code)
	}

	// Consent explicitly refused.
	if env := Invoke(CapToolsRun, body(`,"consent":{"paid_generation_approved":false}`)); env.OK {
		t.Error("a paid tool ran with consent explicitly refused")
	}

	// Consent given, provider NOT granted: the host withheld it.
	env := Invoke(CapToolsRun, body(
		`,"consent":{"paid_generation_approved":true},"grants":{"paid_providers":[]}`))
	if env.OK {
		t.Fatal("consent alone reached a provider the host did not grant")
	}
	if env.Error.Code != "permission_denied" {
		t.Errorf("code = %q, want permission_denied; consent is not a grant", env.Error.Code)
	}
}
