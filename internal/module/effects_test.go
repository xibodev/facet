package module

import (
	"encoding/json"
	"testing"
)

// A capability's DECLARED effects and what an invocation REPORTS must agree.
//
// The host compares the two on every call and shows the reported value to the
// operator, so a disagreement is visible to a human as a warning about a run
// that otherwise succeeded. It caught one I had missed — a job handle
// reporting provider "facet" against a declared "varies" — which is why this
// is a standing test rather than a spot check.
//
// Effects are a promise made before the work runs. A capability that declares
// local:true and then reaches the network has misled the host about what it
// authorised, which matters more than any single wrong value.
func TestDeclaredEffectsAreSelfConsistent(t *testing.T) {
	env := Describe("test")
	if !env.OK {
		t.Fatalf("describe failed: %+v", env.Error)
	}
	desc, ok := env.Result.(Descriptor)
	if !ok {
		t.Fatalf("result is %T, not a Descriptor", env.Result)
	}

	for _, c := range desc.Capabilities {
		e := c.Effects

		// local and network are opposites in the sense that matters: work
		// that reaches the network is not local, whatever else it does.
		if e.Local && e.Network {
			t.Errorf("%s declares both local and network; the host cannot tell "+
				"which authorisation it is granting", c.ID)
		}

		// A capability that can bill must not claim its cost is known unless
		// it really is. cost_known=true with a paid provider is the shape that
		// hides spend: the host skips approval on a known cost.
		if e.CostKnown && e.Provider == ProviderVaries {
			t.Errorf("%s declares cost_known with provider %q; a capability that "+
				"dispatches any tool cannot know its cost in advance",
				c.ID, ProviderVaries)
		}

		// A provider must be named. An empty one tells the host nothing and
		// shows the operator nothing.
		if e.Provider == "" {
			t.Errorf("%s declares no provider", c.ID)
		}
	}
}

// A read-only capability must report no artifacts and no external writes: it
// is the declaration a host relies on to skip a write approval.
func TestReadOnlyCapabilitiesWriteNothing(t *testing.T) {
	env := Invoke(CapToolsList, []byte(`{}`))
	if !env.OK {
		t.Skipf("tool listing unavailable: %+v", env.Error)
	}
	if env.Execution.ExternalWrites {
		t.Error("listing tools reported an external write")
	}
	if len(env.Execution.Artifacts) != 0 {
		raw, _ := json.Marshal(env.Execution.Artifacts)
		t.Errorf("listing tools reported artifacts: %s", raw)
	}
	if env.Execution.Network {
		t.Error("listing tools reported network use")
	}
}
