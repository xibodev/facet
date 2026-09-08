package module

import (
	"encoding/json"
	"strings"
	"testing"
)

// The host validates each artifact's `kind` against the producing capability's
// declared artifact_schemas, and refused every artifact-producing run:
//
//	capability "creative.tools.run" produced artifact kind "output",
//	which it does not declare in artifact_schemas
//
// The declaration named "render_report" and "asset_manifest" — JSON schema
// documents in schemas/artifacts/, describing things an AGENT authors. Facet
// emits one kind: a file a tool wrote. The two vocabularies were never the
// same, so no document schema could ever describe a rendered mp4.
func TestEveryEmittedArtifactKindIsDeclared(t *testing.T) {
	env := Describe("test")
	if !env.OK {
		t.Fatalf("describe failed: %+v", env.Error)
	}
	desc, ok := env.Result.(Descriptor)
	if !ok {
		t.Fatalf("result is %T, not a Descriptor", env.Result)
	}

	for _, id := range []string{CapToolsRun, CapOutputReview} {
		var found *Capability
		for i := range desc.Capabilities {
			if desc.Capabilities[i].ID == id {
				found = &desc.Capabilities[i]
			}
		}
		if found == nil {
			t.Fatalf("capability %s is not declared", id)
		}
		declares := false
		for _, k := range found.ArtifactSchemas {
			if k == ArtifactKindOutput {
				declares = true
			}
		}
		if !declares {
			t.Errorf("%s emits kind %q but declares %v; the host refuses every artifact it produces",
				id, ArtifactKindOutput, found.ArtifactSchemas)
		}
	}

	// A declared id must still resolve, or pruneReferences silently drops it
	// and the capability declares nothing at all — which is how the first
	// attempt at this fix failed.
	if _, present := desc.ArtifactSchemas[ArtifactKindOutput]; !present {
		t.Errorf("%q is declared by a capability but is not a key in artifact_schemas; "+
			"it will be pruned and the host will refuse artifacts again", ArtifactKindOutput)
	}
}

// The host generates request_id and requires it echoed verbatim. Polling has
// its own decoder and never received the protocol-field fix the main request
// path got, so a real host request failed strict decoding and the failure path
// minted a fresh id.
//
// Verified: {"request_id":"req_host_abc","job_id":...} echoed, but the same
// request carrying protocol and capability did not.
func TestJobStatusEchoesTheHostRequestID(t *testing.T) {
	for _, body := range []string{
		`{"request_id":"req_host","job_id":"job_missing"}`,
		`{"protocol":"xibodev.module/v1","capability":"creative.jobs.status",
		  "request_id":"req_host","job_id":"job_missing"}`,
		`{"protocol":"xibodev.module/v1","capability":"creative.jobs.status",
		  "request_id":"req_host","job_id":"job_missing","deadline_ms":60000,
		  "max_output_bytes":262144}`,
		// Even an unacceptable body must stay correlatable: the failure path
		// is exactly when the host needs to match error to call.
		`{"request_id":"req_host","job_id":"j","unknown_field":1}`,
	} {
		env := JobStatus([]byte(body))
		if env.RequestID != "req_host" {
			t.Errorf("request_id = %q, want it echoed verbatim; body was %s",
				env.RequestID, strings.Join(strings.Fields(body), " "))
		}
	}
}

// With no id supplied there is nothing to echo, so one is generated rather
// than left empty — an empty request_id fails the envelope contract.
func TestJobStatusGeneratesAnIDWhenNoneIsSupplied(t *testing.T) {
	env := JobStatus([]byte(`{"job_id":"job_missing"}`))
	if strings.TrimSpace(env.RequestID) == "" {
		t.Error("no request_id was produced; the envelope contract requires one")
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateEnvelopeBytes(raw); err != nil {
		t.Errorf("the polling envelope is invalid: %v", err)
	}
}
