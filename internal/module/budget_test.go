package module

import (
	"encoding/json"
	"strings"
	"testing"
)

// The host truncates at max_output_bytes and treats a truncated envelope as
// unusable, because a partial JSON document cannot be trusted even when it
// happens to parse. A module that ignores the budget turns a successful call
// into a corrupt response, and the host cannot tell whether the work succeeded.
//
// Verified before this fix: creative.tools.list returned 12,565 bytes against a
// 2,048-byte budget and reported ok:true.
func TestOutputBudgetIsEnforced(t *testing.T) {
	big := Envelope{
		Protocol: Protocol, Module: ModuleID, Operation: OpInvoke,
		RequestID: "req_budget", OK: true,
		Result:    map[string]any{"payload": strings.Repeat("x", 4096)},
		Warnings:  []string{},
		Execution: localExec("facet"),
	}

	t.Run("an oversized success becomes an error that fits", func(t *testing.T) {
		out := EnforceOutputBudget(big, 1024)
		if out.OK {
			t.Fatal("an oversized response was returned as a success")
		}
		if out.Error.Code != "output_too_large" {
			t.Errorf("code = %q, want output_too_large", out.Error.Code)
		}
		raw, err := json.Marshal(out)
		if err != nil {
			t.Fatal(err)
		}
		// The replacement must itself fit, or it solves nothing.
		if len(raw) > 1024 {
			t.Errorf("the replacement is %d bytes, still over the 1024 budget", len(raw))
		}
		// It must stay a valid envelope: framing intact, no result, warnings
		// and artifacts still collections rather than null.
		if err := ValidateEnvelopeBytes(raw); err != nil {
			t.Errorf("the replacement is not a valid envelope: %v", err)
		}
		if out.RequestID != big.RequestID {
			t.Error("the request id was lost, so the host cannot correlate the failure")
		}
	})

	t.Run("a response that fits is untouched", func(t *testing.T) {
		out := EnforceOutputBudget(big, 1<<20)
		if !out.OK || out.Result == nil {
			t.Error("a response inside the budget was replaced")
		}
	})

	t.Run("no budget means no enforcement", func(t *testing.T) {
		// The CLI and every existing consumer set none.
		if out := EnforceOutputBudget(big, 0); !out.OK {
			t.Error("a response was refused although no budget was set")
		}
	})

	t.Run("an oversized failure is left alone", func(t *testing.T) {
		// A failure carries no result. If that is still over budget the
		// caller's limit is smaller than any answer, and replacing an error
		// with another error would lose the real reason.
		bigFail := Envelope{
			Protocol: Protocol, Module: ModuleID, Operation: OpInvoke,
			RequestID: "req_fail", OK: false,
			Error: &Error{
				Code: "command_failed", Message: strings.Repeat("y", 4096),
				Details: map[string]any{},
			},
			Warnings:  []string{},
			Execution: localExec("facet"),
		}
		out := EnforceOutputBudget(bigFail, 512)
		if out.Error.Code != "command_failed" {
			t.Errorf("the original failure was replaced: %q", out.Error.Code)
		}
	})
}
