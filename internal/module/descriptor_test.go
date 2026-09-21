package module

import (
	"encoding/json"
	"strings"
	"testing"
)

// Facet's descriptor must satisfy the host's ValidateDescriptor, not Facet's
// idea of it.
//
// Two bugs this session came from matching field NAMES and assuming that meant
// matching the contract: the request rejected fields the host always sends, and
// artifacts omitted three fields the host requires. The validator is where the
// contract actually lives, so these rules are transcribed from
// facet-studio/pkg/modproto/validate.go rather than inferred.
func TestDescriptorSatisfiesHostRules(t *testing.T) {
	env := Describe("test")
	if !env.OK {
		t.Fatalf("describe failed: %+v", env.Error)
	}
	d, ok := env.Result.(Descriptor)
	if !ok {
		t.Fatalf("result is not a Descriptor: %T", env.Result)
	}

	if d.Module == "" || d.Name == "" || d.Version == "" {
		t.Error("module, name and version must all be set")
	}
	if len(d.ProtocolVersions) == 0 {
		t.Error("protocol_versions must list at least one version")
	}
	spoken := false
	for _, v := range d.ProtocolVersions {
		if v == Protocol {
			spoken = true
		}
	}
	if !spoken {
		t.Errorf("protocol_versions %v does not include %q, so the host refuses the module",
			d.ProtocolVersions, Protocol)
	}
	if d.Capabilities == nil {
		t.Error("capabilities must be [] rather than null")
	}

	ids := map[string]bool{}
	for _, c := range d.Capabilities {
		if c.ID == "" {
			t.Error("a capability has no id")
			continue
		}
		if ids[c.ID] {
			t.Errorf("duplicate capability id %q", c.ID)
		}
		ids[c.ID] = true

		// The summary is the one line the agent sees per enabled capability.
		if strings.TrimSpace(c.Summary) == "" {
			t.Errorf("capability %s has no summary", c.ID)
		}
		if c.ArtifactSchemas == nil {
			t.Errorf("capability %s artifact_schemas is null; must be []", c.ID)
		}
		if c.Skills == nil {
			t.Errorf("capability %s skills is null; must be []", c.ID)
		}

		if c.LongRunning || c.PollCapability != "" {
			t.Errorf("per-invocation module advertises unsupported lifecycle for %s", c.ID)
		}
	}

	// Every schema map value must decode, and a declared $id must agree with
	// the key it is filed under, or the host validates requests against a
	// document describing something else.
	for field, schemas := range map[string]map[string]any{
		"request_schemas":  d.RequestSchemas,
		"result_schemas":   d.ResultSchemas,
		"artifact_schemas": d.ArtifactSchemas,
	} {
		for key, doc := range schemas {
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Errorf("%s[%q] does not marshal: %v", field, key, err)
				continue
			}
			var probe struct {
				ID string `json:"$id"`
			}
			if err := json.Unmarshal(raw, &probe); err != nil {
				t.Errorf("%s[%q] is not a decodable JSON Schema document: %v", field, key, err)
				continue
			}
			if probe.ID != "" && !schemaIDAgrees(probe.ID, key) {
				t.Errorf("%s[%q] declares $id %q, which does not agree with its key",
					field, key, probe.ID)
			}
		}
	}
}

// schemaIDAgrees compares the final path segment, matching the host's rule: a
// document's own identity and the descriptor's reference to it are two naming
// systems, and only the last segment has to line up.
func schemaIDAgrees(id, key string) bool {
	last := func(s string) string {
		s = strings.SplitN(s, "#", 2)[0]
		if i := strings.LastIndex(s, "/"); i >= 0 {
			s = s[i+1:]
		}
		return strings.TrimSuffix(s, ".schema.json")
	}
	return last(id) == last(key)
}
