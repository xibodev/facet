package toolbox

import "sort"

// Operation projection for xibodev.module/v2.
//
// DERIVED, never a second table. Every field here reads the same canonical
// source the v1 surface reads: MayCharge, Deterministic, executionFor and
// summary's dependency list. A second authoritative effects table is the
// duplicated-constant defect this repo has already paid for twice — in
// chargeability and in determinism — and a projection is exactly where it
// would reappear, because the two layers look independent.
//
// The tool NAME is the Operation ID. All 35 public names are unchanged: v2 adds
// a semantic layer beside the v1 surface, it does not rename anything.

// V2Effects is an Operation's declared effects.
//
// may_charge and cost_known are INDEPENDENT and neither is inferred from the
// other. Five Facet tools are networked and free (edge_tts, pexels_video,
// pixabay_video, wikimedia, direct_clip_search): cost known, amount zero, no
// charge. Eight may charge and their amount is unknowable before the call.
type V2Effects struct {
	Network       bool `json:"network"`
	ExternalWrite bool `json:"external_writes"`
	MayCharge     bool `json:"may_charge"`
	CostKnown     bool `json:"cost_known"`
	Deterministic bool `json:"deterministic"`
}

// V2Requirement is one thing an Operation needs, with how badly it needs it.
type V2Requirement struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	// Strength is "mandatory" or "preferred" -- the frozen vocabulary. A
	// preferred requirement that is
	// unsatisfied degrades the Operation rather than blocking it.
	Strength string `json:"strength"`
	// Resolution is SATISFIED, UNSATISFIED or UNKNOWN — never collapsed to a
	// boolean. A present credential resolves UNKNOWN because holding a key is
	// not proof the provider will serve you.
	Resolution string `json:"resolution"`
}

// V2Operation is the semantic unit: what this Operation MEANS, as opposed to
// what the capability surface exposes.
type V2Operation struct {
	ID           string          `json:"id"`
	Title        string          `json:"title"`
	Effects      V2Effects       `json:"effects"`
	Requirements []V2Requirement `json:"requirements"`
	// Produces names artifact kinds declared in the descriptor's
	// artifact_kinds map. An Operation naming a kind that is not declared is a
	// dangling reference the host reports.
	Produces []string `json:"produces,omitempty"`
	// Implementations are the interchangeable ways this Operation runs. More
	// than one is a multi-Implementation Operation, NOT a Composite: a
	// Composite would require a typed child-Operation graph, and selecting
	// between renderers is not that.
	Implementations []string `json:"implementations,omitempty"`
}

// preferredRequirements are dependencies whose absence degrades an Operation
// instead of blocking it.
//
// These were previously expressed as hardcoded overrides — music_library set
// configured=true after declaring ffprobe, and direct_clip_search did the same
// — written before there was a vocabulary for strength. The override WAS the
// strength; v2 gives it a name.
var preferredRequirements = map[string]map[string]bool{
	"music_library":      {"ffprobe": true},
	"direct_clip_search": {"ffmpeg": true, "ffprobe": true},
}

// operationImplementations records where one Operation has several
// interchangeable runners. Both are local and neither bills, so the choice
// changes no declared effect — which is why it is an Implementation list
// rather than separate Operations.
var operationImplementations = map[string][]string{
	"video_compose": {"remotion", "hyperframes"},
}

// operationProduces maps an Operation to the artifact kinds it emits.
//
// Only Operations that actually write a file appear. A read-only Operation
// produces nothing, and declaring a kind it never emits would be the
// declared-but-never-produced defect from the other direction.
var operationProduces = map[string][]string{
	"video_compose":         {"render_video"},
	"hyperframes_compose":   {"render_video"},
	"remotion_caption_burn": {"render_video"},
	"video_trimmer":         {"render_video"},
	"video_stitch":          {"render_video"},
	"silence_cutter":        {"render_video"},
	"source_edit":           {"render_video"},
	"color_grade":           {"render_video"},
	"frame_sample":          {"frame"},
	"frame_sampler":         {"frame"},
	"scene_detect":          {"frame"},
	"visual_qa":             {"frame"},
	"output_review":         {"frame"},
	"edge_tts":              {"narration"},
	"openai_tts":            {"narration"},
	"elevenlabs_tts":        {"narration"},
	"piper_tts":             {"narration"},
	"audio_mix":             {"narration"},
	"audio_mixer":           {"narration"},
	"music_library":         {"narration"},
	"subtitle_gen":          {"captions"},
	"openai_image":          {"image"},
	"flux_image":            {"image"},
	"gflow_image":           {"image"},
	"image_selector":        {"image"},
	"sora_video":            {"render_video"},
	"kling_video":           {"render_video"},
	"gflow_video":           {"render_video"},
	"pexels_video":          {"render_video"},
	"pixabay_video":         {"render_video"},
	"wikimedia":             {"render_video"},
	"direct_clip_search":    {"render_video"},
	"video_selector":        {"render_video"},
}

// V2Operations projects every public tool as an Operation.
//
// Sorted so the descriptor is byte-stable: an unordered map would make two
// identical modules produce different bytes and defeat digest comparison.
func V2Operations() []V2Operation {
	names := Names()
	sort.Strings(names)

	out := make([]V2Operation, 0, len(names))
	for _, n := range names {
		exec := executionFor(n)
		out = append(out, V2Operation{
			ID:    n,
			Title: capabilities[n],
			Effects: V2Effects{
				Network:       exec.Network,
				ExternalWrite: externalWriteFor(n, "run"),
				MayCharge:     MayCharge(n),
				// CostKnown asks whether a NUMERIC AMOUNT is known, never
				// whether money may be spent. executionFor supplies a non-nil
				// estimate exactly when the amount is known.
				CostKnown:     exec.EstimatedCost != nil,
				Deterministic: Deterministic(n),
			},
			Requirements:    v2RequirementsFor(n),
			Produces:        operationProduces[n],
			Implementations: operationImplementations[n],
		})
	}
	return out
}

// v2RequirementsFor derives requirements from the SAME dependency list the v1
// surface publishes, so the two can never disagree.
func v2RequirementsFor(tool string) []V2Requirement {
	raw, _ := summary(tool)["dependencies"].([]any)
	out := make([]V2Requirement, 0, len(raw))
	pref := preferredRequirements[tool]
	for _, d := range raw {
		m, ok := d.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		typ, _ := m["type"].(string)
		res, _ := m["resolution"].(string)
		strength := "mandatory"
		if pref[name] {
			strength = "preferred"
		}
		out = append(out, V2Requirement{
			Name: name, Kind: typ, Strength: strength, Resolution: res,
		})
	}
	return out
}

// EstimatedCostKnown reports whether a NUMERIC cost amount is known for a
// tool, which is the only thing cost_known means.
//
// Exported so conformance can assert the projection reads THIS rather than
// inferring from may_charge. In Facet's current tool set the two are exact
// inverses for every tool, so a value comparison cannot tell them apart -- a
// mutation setting cost_known = !may_charge passed every value assertion. The
// inverse relationship is an accident of today's tools, not the contract.
func EstimatedCostKnown(tool string) bool { return executionFor(tool).EstimatedCost != nil }

// NetworkFor and ExternalWriteFor expose the canonical per-tool effects so a
// PROJECTION can derive its pessimistic union instead of restating it.
//
// Exported for exactly that reason: a capability that dispatches any of 35
// tools must declare the worst case any of them can do, and a hardcoded
// literal there is a second effects table waiting to drift.
func NetworkFor(tool string) bool       { return executionFor(tool).Network }
func ExternalWriteFor(tool string) bool { return externalWriteFor(tool, "run") }
