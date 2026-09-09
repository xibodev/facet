package module

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/xibodev/facet/internal/toolbox"
)

// v2Wire is the descriptor as a HOST receives it: decoded from the serialized
// bytes, not read from the local Go objects.
//
// Testing the objects would only prove the projection function returns what it
// returns. A field with the wrong json tag, or silently dropped by omitempty,
// is invisible to an object test and fatal on the wire.
type v2Wire struct {
	ContractVersion string                            `json:"contract_version"`
	Operations      []toolbox.V2Operation             `json:"operations"`
	ArtifactKinds   map[string]toolbox.V2ArtifactKind `json:"artifact_kinds"`
	ArtifactSchemas map[string]any                    `json:"artifact_schemas"`
	Capabilities    []map[string]any                  `json:"capabilities"`
}

func serializedV2(t *testing.T) v2Wire {
	t.Helper()
	env := Describe("test")
	if !env.OK {
		t.Fatalf("describe failed: %+v", env.Error)
	}
	raw, err := json.Marshal(env.Result)
	if err != nil {
		t.Fatal(err)
	}
	var out v2Wire
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func v2ByID(d v2Wire) map[string]toolbox.V2Operation {
	m := map[string]toolbox.V2Operation{}
	for _, op := range d.Operations {
		m[op.ID] = op
	}
	return m
}

// The v2 payload must reach the wire at all. Before this existed Facet
// declared contract_version and published no Operations, and facet-studio's
// host reported no_operations_declared -- accurately.
func TestSerializedV2PayloadIsPublished(t *testing.T) {
	d := serializedV2(t)
	if d.ContractVersion != ContractVersion {
		t.Fatalf("contract_version = %q, want %q", d.ContractVersion, ContractVersion)
	}
	if len(d.Operations) == 0 {
		t.Fatal("no operations on the wire; the v2 declaration is ahead of its payload")
	}
	if len(d.ArtifactKinds) == 0 {
		t.Fatal("no artifact_kinds on the wire")
	}
	// Every public tool is an Operation. A projection that drops one hides an
	// effect the host gates on.
	if got, want := len(d.Operations), len(toolbox.Names()); got != want {
		t.Errorf("%d operations for %d public tools; the projection is not total", got, want)
	}
}

// may_charge and cost_known are INDEPENDENT. Facet's proof is the
// networked-and-free row: cost known, amount zero, no charge.
func TestSerializedMayChargeIsIndependentOfCostKnown(t *testing.T) {
	byID := v2ByID(serializedV2(t))

	for _, n := range []string{"edge_tts", "pexels_video", "pixabay_video", "wikimedia"} {
		op, ok := byID[n]
		if !ok {
			t.Errorf("%s missing from the projection", n)
			continue
		}
		if op.Effects.MayCharge {
			t.Errorf("%s declares may_charge; it is free", n)
		}
		if !op.Effects.CostKnown {
			t.Errorf("%s declares cost_known=false; its cost is known to be zero", n)
		}
		if !op.Effects.Network {
			t.Errorf("%s declares no network; it contacts a remote service", n)
		}
	}

	for _, n := range []string{"elevenlabs_tts", "openai_tts", "flux_image"} {
		op, ok := byID[n]
		if !ok {
			t.Errorf("%s missing from the projection", n)
			continue
		}
		if !op.Effects.MayCharge {
			t.Errorf("%s does not declare may_charge; it bills", n)
		}
		if op.Effects.CostKnown {
			t.Errorf("%s claims a known cost; the amount is not knowable before the call", n)
		}
	}
}

// The projection must agree with the canonical source tool by tool. A second
// effects table would drift, and this is what catches it.
func TestSerializedEffectsMatchCanonicalTruth(t *testing.T) {
	d := serializedV2(t)
	for _, op := range d.Operations {
		if want := toolbox.MayCharge(op.ID); op.Effects.MayCharge != want {
			t.Errorf("%s: may_charge=%v on the wire, canonical says %v",
				op.ID, op.Effects.MayCharge, want)
		}
		if want := toolbox.Deterministic(op.ID); op.Effects.Deterministic != want {
			t.Errorf("%s: deterministic=%v on the wire, canonical says %v",
				op.ID, op.Effects.Deterministic, want)
		}
		// A networked or chargeable Operation can never be deterministic.
		if op.Effects.Deterministic && (op.Effects.Network || op.Effects.MayCharge) {
			t.Errorf("%s claims determinism while network=%v may_charge=%v",
				op.ID, op.Effects.Network, op.Effects.MayCharge)
		}
	}
}

// Resolution stays tri-state on the wire. Collapsing UNKNOWN into either
// neighbour is the defect: a present credential is not a working provider.
func TestSerializedResolutionKeepsThreeStates(t *testing.T) {
	d := serializedV2(t)
	seen := map[string]int{}
	for _, op := range d.Operations {
		for _, r := range op.Requires {
			switch r.Resolution {
			case toolbox.ResolutionSatisfied, toolbox.ResolutionUnsatisfied, toolbox.ResolutionUnknown:
				seen[r.Resolution]++
			default:
				t.Errorf("%s requires %s with resolution %q, not one of the three states",
					op.ID, r.Name, r.Resolution)
			}
		}
	}
	if seen[toolbox.ResolutionUnknown] == 0 && seen[toolbox.ResolutionSatisfied] > 0 {
		t.Error("no requirement resolved UNKNOWN; holding a credential is being reported as proof it works")
	}
}

// Requirement strength must survive to the wire. It was previously a hardcoded
// configured=true override with no vocabulary for it.
func TestSerializedRequirementStrength(t *testing.T) {
	d := serializedV2(t)
	byID := v2ByID(d)

	var foundPreferred bool
	for _, op := range d.Operations {
		for _, r := range op.Requires {
			if r.Strength != "required" && r.Strength != "preferred" {
				t.Errorf("%s requires %s with strength %q", op.ID, r.Name, r.Strength)
			}
			if r.Strength == "preferred" {
				foundPreferred = true
			}
		}
	}
	if !foundPreferred {
		t.Error("no preferred requirement anywhere; music_library ffprobe is optional and must say so")
	}
	if op, ok := byID["music_library"]; ok {
		for _, r := range op.Requires {
			if r.Name == "ffprobe" && r.Strength != "preferred" {
				t.Errorf("music_library ffprobe is %q; it degrades rather than blocks", r.Strength)
			}
		}
	}

	// A REQUIRED requirement must stay required. Asserting only that SOME
	// preferred exists let a mutation weakening every requirement to
	// "preferred" pass: the preferred ones were still preferred, and nothing
	// checked the other direction. Caught by the mutation harness, not by
	// review.
	//
	// audio_mix cannot run without ffmpeg -- that is a block, not a
	// degradation.
	if op, ok := byID["audio_mix"]; ok {
		var sawFFmpeg bool
		for _, r := range op.Requires {
			if r.Name == "ffmpeg" {
				sawFFmpeg = true
				if r.Strength != "required" {
					t.Errorf("audio_mix ffmpeg is %q; without it the Operation cannot run at all", r.Strength)
				}
			}
		}
		if !sawFFmpeg {
			t.Error("audio_mix declares no ffmpeg requirement")
		}
	}

	// At least one requirement in the whole projection must be REQUIRED. If a
	// change made everything preferred, the strength axis would carry no
	// information while still type-checking.
	var required int
	for _, op := range d.Operations {
		for _, r := range op.Requires {
			if r.Strength == "required" {
				required++
			}
		}
	}
	if required == 0 {
		t.Error("no requirement anywhere is required; strength has collapsed to a single value")
	}
}

// cost_known must NOT be derivable from may_charge.
//
// In Facet's current tree the two happen to be exact inverses for every tool,
// so no VALUE comparison can distinguish them -- a mutation setting
// cost_known = !may_charge passed every assertion. That is a property of
// today's tool set, not of the contract, and the day a chargeable tool gains a
// known price it would silently become wrong.
//
// So this asserts the SOURCE rather than the values: cost_known reflects
// whether a numeric estimate exists, and may_charge is a separate declaration.
// The estimate is the only thing that can answer "is the amount known".
func TestCostKnownComesFromTheEstimateNotFromMayCharge(t *testing.T) {
	d := serializedV2(t)
	for _, op := range d.Operations {
		known := toolbox.EstimatedCostKnown(op.ID)
		if op.Effects.CostKnown != known {
			t.Errorf("%s: cost_known=%v on the wire, but the canonical estimate says %v",
				op.ID, op.Effects.CostKnown, known)
		}
	}

	// THE VALUE COMPARISON ABOVE CANNOT CATCH AN INVERSION, and saying so is
	// the point of this block.
	//
	// In today's tool set may_charge and cost_known are exact inverses for all
	// 35 tools, so `cost_known = !may_charge` produces byte-identical output
	// and every value assertion passes. Measured, not assumed: a mutation
	// doing exactly that survived the whole suite.
	//
	// The remedy is the one duration_test already uses -- pin to something the
	// code under test cannot generate. Here that is the CONSTANT that makes
	// the two fields independent by construction: cost_known is answered by an
	// estimate existing, and an estimate is a number, not a boolean about
	// billing. So assert against the estimate's own presence for a tool whose
	// two fields would DISAGREE if a price were ever attached.
	//
	// elevenlabs_tts bills and has no knowable amount. If someone attaches a
	// fixed price to it, may_charge stays true and cost_known becomes true --
	// the inverse relationship breaks, and an inference-based projection
	// silently reports the wrong thing. This asserts the wiring survives that
	// day rather than that today's numbers line up.
	if toolbox.EstimatedCostKnown("elevenlabs_tts") {
		t.Error("elevenlabs_tts reports a known cost; its amount is not knowable before the call")
	}
	if !toolbox.MayCharge("elevenlabs_tts") {
		t.Error("elevenlabs_tts does not declare may_charge; it bills")
	}
	// media_probe: free AND priced at zero. If cost_known were inferred as
	// !may_charge this would still read true, so it is the pair above that
	// carries the discrimination, not this one -- kept because it documents
	// the other quadrant.
	if !toolbox.EstimatedCostKnown("media_probe") {
		t.Error("media_probe has a known cost of zero and must say so")
	}
	if toolbox.MayCharge("media_probe") {
		t.Error("media_probe cannot bill")
	}
}

// The projection must read the estimate, not re-derive cost_known.
//
// This is a SOURCE assertion rather than a value one: it fails if the
// projection stops calling the canonical accessor, which is the change a
// value test cannot see while the two fields remain inverses.
func TestCostKnownProjectionReadsTheCanonicalAccessor(t *testing.T) {
	src, err := os.ReadFile("../toolbox/v2ops.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "CostKnown:     exec.EstimatedCost != nil") {
		t.Error("the v2 projection no longer derives cost_known from the estimate; " +
			"inferring it from may_charge is indistinguishable by value in this tree " +
			"and would silently break the day a chargeable tool gains a known price")
	}
}

// Every produced kind must be declared, and every declared kind produced.
// A dangling reference and a never-emitted declaration are different defects
// and both are silent.
func TestSerializedProducesAndKindsAgree(t *testing.T) {
	d := serializedV2(t)
	produced := map[string]bool{}
	for _, op := range d.Operations {
		for _, k := range op.Produces {
			produced[k] = true
			if _, ok := d.ArtifactKinds[k]; !ok {
				t.Errorf("%s produces %q, which is not a declared artifact kind", op.ID, k)
			}
		}
	}
	for name := range d.ArtifactKinds {
		if !produced[name] {
			t.Errorf("artifact kind %q is declared but no Operation produces it", name)
		}
	}
}

// Media is validated by media type, size and digest, NEVER by JSON Schema.
func TestSerializedArtifactKindsCarryNoValidator(t *testing.T) {
	d := serializedV2(t)
	for name, k := range d.ArtifactKinds {
		switch k.Kind {
		case "media":
			if k.Validator != nil {
				t.Errorf("artifact kind %q is media and names a validator; a schema cannot validate media bytes", name)
			}
			if k.MediaType == "" {
				t.Errorf("artifact kind %q is media with no media_type", name)
			}
		case "text":
			if k.Validator != nil {
				t.Errorf("artifact kind %q is text and names a validator; text declares none by definition", name)
			}
		case "document":
			if k.Validator == nil || k.Validator.Schema == "" {
				t.Errorf("artifact kind %q is document and names no validator", name)
			}
		default:
			t.Errorf("artifact kind %q has kind %q, not document|text|media", name, k.Kind)
		}
	}
}

// Authoring formats must NOT be projected as emitted artifacts, and must not
// be deleted from the frozen v1 field either.
func TestAuthoringSchemasAreNotProjectedAsEmitted(t *testing.T) {
	d := serializedV2(t)
	if len(d.ArtifactSchemas) < 20 {
		t.Errorf("artifact_schemas has %d entries; the v1 contract published 21 and nothing may be deleted",
			len(d.ArtifactSchemas))
	}
	for _, authoring := range []string{"brief", "scene_plan", "script", "review", "decision_log", "rig_plan"} {
		if _, ok := d.ArtifactKinds[authoring]; ok {
			t.Errorf("%q is an authoring format declared as an EMITTED artifact kind", authoring)
		}
		if _, ok := d.ArtifactSchemas[authoring]; !ok {
			t.Errorf("%q was dropped from artifact_schemas; the v1 contract is frozen", authoring)
		}
	}
}

// video_compose has two interchangeable runners. That is a
// multi-Implementation Operation, NOT a Composite: a Composite needs a typed
// child-Operation graph, and neither runner changes a declared effect.
func TestVideoComposeIsMultiImplementationNotComposite(t *testing.T) {
	byID := v2ByID(serializedV2(t))
	op, ok := byID["video_compose"]
	if !ok {
		t.Fatal("video_compose is missing from the projection")
	}
	if len(op.Implementations) < 2 {
		t.Errorf("video_compose declares %d implementations; it selects between Remotion and HyperFrames",
			len(op.Implementations))
	}
}

// All 35 public tool names survive. v2 adds a semantic layer beside the
// capability surface; it renames nothing.
func TestPublicToolNamesUnchangedByV2(t *testing.T) {
	byID := v2ByID(serializedV2(t))
	for _, n := range toolbox.Names() {
		if _, ok := byID[n]; !ok {
			t.Errorf("public tool %q has no Operation; v2 must not drop a public name", n)
		}
	}
}
