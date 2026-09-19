// Package routes describes Facet production methods without executing them.
package routes

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/xibodev/facet/internal/toolbox"
)

const (
	StatusFeasible    = "feasible"
	StatusConditional = "conditional"
	StatusUnavailable = "unavailable"

	BindingTargetExact                   = "exact"
	BindingTargetArrayItem               = "array_item"
	BindingTargetVideoComposeMediaSource = "video_compose_media_cut_source"
	BindingTargetAnyValue                = "any_value"
	BindingTargetAllValues               = "all_values"
	BindingTargetTextSequence            = "text_sequence"

	InputConsumptionOperation     = "operation"
	InputConsumptionInformational = "informational"
)

type Input struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
	Consumption string `json:"consumption"`
}

type Binding struct {
	FromInput       string `json:"from_input,omitempty"`
	FromOperation   string `json:"from_operation,omitempty"`
	FromParameter   string `json:"from_parameter,omitempty"`
	ArtifactKind    string `json:"artifact_kind,omitempty"`
	ToOperation     string `json:"to_operation"`
	ToParameter     string `json:"to_parameter"`
	TargetSemantics string `json:"target_semantics"`
}

type RequestConstraint struct {
	Operation string `json:"operation"`
	Parameter string `json:"parameter"`
	Equals    string `json:"equals"`
}

type Route struct {
	ID                 string              `json:"id"`
	Title              string              `json:"title"`
	Summary            string              `json:"summary"`
	RequiredInputs     []Input             `json:"required_inputs"`
	EntryOperation     string              `json:"entry_operation"`
	Operations         []string            `json:"operations"`
	RequestConstraints []RequestConstraint `json:"request_constraints,omitempty"`
	Bindings           []Binding           `json:"bindings"`
}

type Method struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Summary string  `json:"summary"`
	Pack    string  `json:"pack"`
	Routes  []Route `json:"routes"`
}

// Catalog returns stable, Facet-owned production topology. Whether a route is
// usable is deliberately not stored here; Assess joins these definitions to
// the live canonical operation registry on every call.
func Catalog() []Method {
	methods := []Method{
		method("animation", "Animation", "Motion-graphics and asset-led animation.", "explainer",
			route("remotion-animation", "Local Remotion animation", "Animate supplied visual assets locally.",
				inputs("script", "visual_assets"), "video_compose", []string{"video_compose", "output_review"},
				inputTextSequence("script", "video_compose", "cuts[].text"),
				inputAllValues("visual_assets", "video_compose", "cuts[].source"),
				artifact("video_compose", "output", "render_video", "output_review", "input")),
			route("openai-image-animation", "OpenAI-assisted animation", "Generate still visuals with the explicitly named provider, then animate locally.",
				inputs("script"), "openai_image", []string{"openai_image", "video_compose", "output_review"},
				inputBinding("script", "openai_image", "prompt"),
				mediaCutArtifact("openai_image", "output_path", "image", "video_compose", "cuts"),
				artifact("video_compose", "output", "render_video", "output_review", "input"))),
		method("avatar", "Avatar", "Presenter-led and consented avatar productions.", "talking-head",
			route("supplied-avatar-edit", "Supplied avatar edit", "Edit supplied presenter or avatar footage without generating an identity.",
				inputs("presenter_media", "consent"), "source_edit", []string{"source_edit", "audio_mix", "output_review"},
				inputAnyValue("presenter_media", "source_edit", "segments[].input"),
				artifact("source_edit", "output", "render_video", "audio_mix", "video"),
				artifact("audio_mix", "output", "render_video", "output_review", "input")),
			route("edge-voice-avatar", "Edge voice with supplied avatar", "Synthesize speech with the explicitly named free network voice and edit supplied avatar footage.",
				inputs("presenter_media", "script", "consent"), "edge_tts", []string{"edge_tts", "source_edit", "output_review"},
				inputBinding("script", "edge_tts", "text"),
				inputAnyValue("presenter_media", "source_edit", "segments[].input"),
				artifact("edge_tts", "output_path", "narration", "source_edit", "replacement_audio"),
				artifact("source_edit", "output", "render_video", "output_review", "input"))),
		method("character-animation", "Character animation", "Character-led animation from supplied artwork, poses, or rigs.", "character-animation",
			route("character-remotion", "Local character composition", "Animate supplied character assets in the local composer.",
				inputs("character_assets", "action_plan"), "video_compose", []string{"video_compose", "audio_mix", "output_review"},
				inputAllValues("character_assets", "video_compose", "cuts[].source"),
				artifact("video_compose", "output", "render_video", "audio_mix", "video"),
				artifact("audio_mix", "output", "render_video", "output_review", "input"))),
		method("content-repurpose", "Content repurpose", "Short-form, vertical, and excerpted edits from supplied media.", "social",
			route("source-repurpose", "Local source repurpose", "Inspect, select, reframe, caption, and review supplied footage.",
				inputs("source_media", "transcript_segments"), "source_edit", []string{"source_edit", "subtitle_gen", "ffmpeg_caption_burn", "output_review"},
				inputAnyValue("source_media", "source_edit", "segments[].input"),
				inputBinding("transcript_segments", "subtitle_gen", "segments"),
				artifact("source_edit", "output", "render_video", "ffmpeg_caption_burn", "input_path"),
				artifact("subtitle_gen", "output_path", "captions", "ffmpeg_caption_burn", "srt_path"),
				artifact("ffmpeg_caption_burn", "output_path", "render_video", "output_review", "input"))),
		method("documentary-cinematic", "Documentary and cinematic", "Source-led documentary, archival, montage, and cinematic editing.", "cinematic",
			route("source-documentary", "Source-led documentary edit", "Assemble and grade supplied or licensed material locally.",
				inputs("source_media"), "video_stitch", []string{"video_stitch", "color_grade", "audio_mix", "output_review"},
				inputArrayItem("source_media", "video_stitch", "clips"),
				artifact("video_stitch", "output_path", "render_video", "color_grade", "input_path"),
				artifact("color_grade", "output_path", "render_video", "audio_mix", "video"),
				artifact("audio_mix", "output", "render_video", "output_review", "input")),
			requireRequestValue(
				route("wikimedia-documentary", "Wikimedia-augmented documentary", "Search the explicitly named public archive for video before local assembly.",
					inputs("research_query"), "wikimedia", []string{"wikimedia", "video_stitch", "color_grade", "audio_mix", "output_review"},
					inputBinding("research_query", "wikimedia", "query"),
					arrayItemArtifact("wikimedia", "output_path", "render_video", "video_stitch", "clips"),
					artifact("video_stitch", "output_path", "render_video", "color_grade", "input_path"),
					artifact("color_grade", "output_path", "render_video", "audio_mix", "video"),
					artifact("audio_mix", "output", "render_video", "output_review", "input")),
				"wikimedia", "kind", "video")),
		method("explainer", "Explainer", "Text-, metric-, and supplied-media-led explanation.", "explainer",
			route("local-explainer", "Local Remotion explainer", "Compose supplied script and visuals with the local renderer.",
				inputs("script", "visuals"), "video_compose", []string{"video_compose", "output_review"},
				inputTextSequence("script", "video_compose", "cuts[].text"),
				inputAllValues("visuals", "video_compose", "cuts[].source"),
				artifact("video_compose", "output", "render_video", "output_review", "input")),
			route("flux-image-explainer", "FLUX-assisted explainer", "Generate stills with the explicitly named provider, then compose locally.",
				inputs("script"), "flux_image", []string{"flux_image", "video_compose", "output_review"},
				inputBinding("script", "flux_image", "prompt"),
				mediaCutArtifact("flux_image", "output_path", "image", "video_compose", "cuts"),
				artifact("video_compose", "output", "render_video", "output_review", "input"))),
		method("localization", "Localization", "Subtitle, dubbing, and localized variants of existing video.", "localization",
			route("subtitle-localization", "Provided-translation subtitle route", "Create and burn supplied translated timed text.",
				inputs("source_media", "translated_segments"), "subtitle_gen", []string{"subtitle_gen", "ffmpeg_caption_burn", "output_review"},
				inputBinding("translated_segments", "subtitle_gen", "segments"),
				inputBinding("source_media", "ffmpeg_caption_burn", "input_path"),
				artifact("subtitle_gen", "output_path", "captions", "ffmpeg_caption_burn", "srt_path"),
				artifact("ffmpeg_caption_burn", "output_path", "render_video", "output_review", "input")),
			route("edge-dub-localization", "Edge voice dubbing route", "Synthesize the supplied translated script with the explicitly named voice service.",
				inputs("source_media", "translated_script"), "edge_tts", []string{"edge_tts", "source_edit", "output_review"},
				inputBinding("translated_script", "edge_tts", "text"),
				inputAnyValue("source_media", "source_edit", "segments[].input"),
				artifact("edge_tts", "output_path", "narration", "source_edit", "replacement_audio"),
				artifact("source_edit", "output", "render_video", "output_review", "input"))),
		method("music-led", "Music-led", "Edits paced around supplied or locally discovered music.", "cinematic",
			route("local-music-edit", "Local music-led edit", "Choose local music, edit supplied media, mix, and review.",
				inputs("source_media", "music"), "source_edit", []string{"source_edit", "audio_mix", "output_review"},
				inputAnyValue("source_media", "source_edit", "segments[].input"),
				artifact("source_edit", "output", "render_video", "audio_mix", "video"),
				inputBinding("music", "audio_mix", "music.input"),
				artifact("audio_mix", "output", "render_video", "output_review", "input"))),
		method("product-demo", "Product demo", "Recorded or synthetic software and product walkthroughs.", "screen-demo",
			route("recorded-product-demo", "Recorded product demo", "Edit a real product capture and add optional supplied captions.",
				inputs("screen_recording"), "source_edit", []string{"source_edit", "output_review"},
				inputAnyValue("screen_recording", "source_edit", "segments[].input"),
				artifact("source_edit", "output", "render_video", "output_review", "input")),
			route("synthetic-product-demo", "Synthetic product presentation", "Compose a clearly presentational product sequence locally.",
				inputs("script", "visual_assets"), "video_compose", []string{"video_compose", "output_review"},
				inputTextSequence("script", "video_compose", "cuts[].text"),
				inputAllValues("visual_assets", "video_compose", "cuts[].source"),
				artifact("video_compose", "output", "render_video", "output_review", "input"))),
		method("source-edit", "Source edit", "Mechanical editing of supplied footage and audio.", "cinematic",
			route("source-edit-local", "Local source edit", "Inspect, edit, mix, and review supplied media locally.",
				inputs("source_media"), "source_edit", []string{"source_edit", "output_review"},
				inputAnyValue("source_media", "source_edit", "segments[].input"),
				artifact("source_edit", "output", "render_video", "output_review", "input"))),
	}
	sort.Slice(methods, func(i, j int) bool { return methods[i].ID < methods[j].ID })
	return methods
}

func method(id, title, summary, pack string, routes ...Route) Method {
	return Method{ID: id, Title: title, Summary: summary, Pack: pack, Routes: routes}
}

func route(id, title, summary string, required []Input, entry string, operations []string, bindings ...Binding) Route {
	return Route{
		ID: id, Title: title, Summary: summary,
		RequiredInputs: required, EntryOperation: entry,
		Operations: operations, Bindings: bindings,
	}
}

func requireRequestValue(route Route, operation, parameter, value string) Route {
	route.RequestConstraints = append(route.RequestConstraints, RequestConstraint{
		Operation: operation,
		Parameter: parameter,
		Equals:    value,
	})
	return route
}

func artifact(fromOperation, fromParameter, kind, toOperation, toParameter string) Binding {
	return Binding{
		FromOperation: fromOperation, FromParameter: fromParameter, ArtifactKind: kind,
		ToOperation: toOperation, ToParameter: toParameter,
		TargetSemantics: BindingTargetExact,
	}
}

func arrayItemArtifact(fromOperation, fromParameter, kind, toOperation, toParameter string) Binding {
	binding := artifact(fromOperation, fromParameter, kind, toOperation, toParameter)
	binding.TargetSemantics = BindingTargetArrayItem
	return binding
}

func mediaCutArtifact(fromOperation, fromParameter, kind, toOperation, toParameter string) Binding {
	binding := artifact(fromOperation, fromParameter, kind, toOperation, toParameter)
	binding.TargetSemantics = BindingTargetVideoComposeMediaSource
	return binding
}

func inputBinding(fromInput, toOperation, toParameter string) Binding {
	return Binding{
		FromInput: fromInput, ToOperation: toOperation, ToParameter: toParameter,
		TargetSemantics: BindingTargetExact,
	}
}

func inputArrayItem(fromInput, toOperation, toParameter string) Binding {
	binding := inputBinding(fromInput, toOperation, toParameter)
	binding.TargetSemantics = BindingTargetArrayItem
	return binding
}

func inputAnyValue(fromInput, toOperation, toParameter string) Binding {
	binding := inputBinding(fromInput, toOperation, toParameter)
	binding.TargetSemantics = BindingTargetAnyValue
	return binding
}

func inputAllValues(fromInput, toOperation, toParameter string) Binding {
	binding := inputBinding(fromInput, toOperation, toParameter)
	binding.TargetSemantics = BindingTargetAllValues
	return binding
}

func inputTextSequence(fromInput, toOperation, toParameter string) Binding {
	binding := inputBinding(fromInput, toOperation, toParameter)
	binding.TargetSemantics = BindingTargetTextSequence
	return binding
}

func inputs(names ...string) []Input {
	out := make([]Input, 0, len(names))
	for _, name := range names {
		out = append(out, Input{
			Name: name, Description: inputDescription(name), Kind: inputKind(name),
			Consumption: inputConsumption(name),
		})
	}
	return out
}

func inputConsumption(name string) string {
	switch name {
	case "action_plan", "consent":
		return InputConsumptionInformational
	default:
		return InputConsumptionOperation
	}
}

func inputKind(name string) string {
	switch name {
	case "source_media", "presenter_media", "screen_recording", "music":
		return "file"
	case "visual_assets", "visuals", "character_assets":
		return "files"
	case "translated_segments", "transcript_segments":
		return "segments"
	case "consent":
		return "consent"
	default:
		return "text"
	}
}

func inputDescription(name string) string {
	descriptions := map[string]string{
		"action_plan":         "Actions, poses, timing, or shot beats for the character",
		"character_assets":    "Supplied or licensed character artwork, poses, or rig assets",
		"consent":             "Recorded subject consent for identity, voice, and avatar use",
		"music":               "A supplied track or an explicit request to choose from the local library",
		"presenter_media":     "Supplied presenter or avatar footage",
		"research_query":      "Specific archive search terms and rights constraints",
		"screen_recording":    "A real product capture when actual behavior must be demonstrated",
		"script":              "Approved script, copy, or narrative beats",
		"source_media":        "Supplied or licensed source footage and audio",
		"translated_script":   "Human-reviewed translated narration text",
		"translated_segments": "Human-reviewed translated text with timing",
		"transcript_segments": "Transcript text with start and end timing",
		"visual_assets":       "Supplied or licensed images, graphics, or UI captures",
		"visuals":             "Supplied or licensed visuals, or an explicit generation choice",
	}
	if description := descriptions[name]; description != "" {
		return description
	}
	return strings.ReplaceAll(name, "_", " ")
}

func Find(methodID string) (Method, bool) {
	for _, method := range Catalog() {
		if method.ID == strings.ToLower(strings.TrimSpace(methodID)) {
			return method, true
		}
	}
	return Method{}, false
}

func ValidateCatalog() error {
	live := operationMap(toolbox.V2Operations())
	for _, method := range Catalog() {
		if method.Pack == "" {
			return fmt.Errorf("method %q has no pack", method.ID)
		}
		for _, route := range method.Routes {
			if len(route.Operations) == 0 || route.EntryOperation != route.Operations[0] {
				return fmt.Errorf("method %q route %q has invalid entry operation %q", method.ID, route.ID, route.EntryOperation)
			}
			for _, operation := range route.Operations {
				if _, ok := live[operation]; !ok {
					return fmt.Errorf("method %q route %q references unavailable operation %q", method.ID, route.ID, operation)
				}
			}
			routeOperations := map[string]int{}
			for index, operation := range route.Operations {
				routeOperations[operation] = index
			}
			for _, constraint := range route.RequestConstraints {
				if _, ok := routeOperations[constraint.Operation]; !ok {
					return fmt.Errorf("method %q route %q constrains operation %q outside the route", method.ID, route.ID, constraint.Operation)
				}
				if strings.TrimSpace(constraint.Parameter) == "" || strings.TrimSpace(constraint.Equals) == "" {
					return fmt.Errorf("method %q route %q has incomplete request constraint for %q", method.ID, route.ID, constraint.Operation)
				}
			}
			requiredInputs := map[string]Input{}
			for _, input := range route.RequiredInputs {
				switch input.Consumption {
				case InputConsumptionOperation, InputConsumptionInformational:
				default:
					return fmt.Errorf("method %q route %q input %q has unknown consumption %q", method.ID, route.ID, input.Name, input.Consumption)
				}
				requiredInputs[input.Name] = input
			}
			targeted := map[string]bool{}
			consumedInputs := map[string]bool{}
			for _, binding := range route.Bindings {
				if (binding.FromInput == "") == (binding.FromOperation == "") {
					return fmt.Errorf("method %q route %q binding must have exactly one source", method.ID, route.ID)
				}
				if binding.ToParameter == "" || (binding.FromOperation != "" && binding.FromParameter == "") {
					return fmt.Errorf("method %q route %q has incomplete binding parameters", method.ID, route.ID)
				}
				switch binding.TargetSemantics {
				case BindingTargetExact:
				case BindingTargetArrayItem:
					if binding.ToOperation != "video_stitch" || binding.ToParameter != "clips" {
						return fmt.Errorf("method %q route %q uses array-item semantics for unsupported target %s.%s", method.ID, route.ID, binding.ToOperation, binding.ToParameter)
					}
				case BindingTargetAnyValue:
					if binding.FromInput == "" || !strings.Contains(binding.ToParameter, "[]") {
						return fmt.Errorf("method %q route %q uses any-value semantics without an array path for %s.%s", method.ID, route.ID, binding.ToOperation, binding.ToParameter)
					}
				case BindingTargetAllValues:
					input, ok := requiredInputs[binding.FromInput]
					if !ok || input.Kind != "files" || !strings.Contains(binding.ToParameter, "[]") {
						return fmt.Errorf("method %q route %q has invalid all-values input binding to %s.%s", method.ID, route.ID, binding.ToOperation, binding.ToParameter)
					}
				case BindingTargetTextSequence:
					input, ok := requiredInputs[binding.FromInput]
					if !ok || input.Kind != "text" || binding.ToOperation != "video_compose" || binding.ToParameter != "cuts[].text" {
						return fmt.Errorf("method %q route %q has invalid text-sequence input binding to %s.%s", method.ID, route.ID, binding.ToOperation, binding.ToParameter)
					}
				case BindingTargetVideoComposeMediaSource:
					if binding.ToOperation != "video_compose" || binding.ToParameter != "cuts" {
						return fmt.Errorf("method %q route %q uses media-cut semantics for unsupported target %s.%s", method.ID, route.ID, binding.ToOperation, binding.ToParameter)
					}
					if binding.ArtifactKind != "image" && binding.ArtifactKind != "video" && binding.ArtifactKind != "render_video" {
						return fmt.Errorf("method %q route %q uses media-cut semantics for unsupported artifact kind %q", method.ID, route.ID, binding.ArtifactKind)
					}
				default:
					return fmt.Errorf("method %q route %q has unknown binding target semantics %q", method.ID, route.ID, binding.TargetSemantics)
				}
				targetIndex, targetExists := routeOperations[binding.ToOperation]
				if !targetExists {
					return fmt.Errorf("method %q route %q binds to operation %q outside the route", method.ID, route.ID, binding.ToOperation)
				}
				targeted[binding.ToOperation] = true
				if binding.FromInput != "" {
					input, ok := requiredInputs[binding.FromInput]
					if !ok {
						return fmt.Errorf("method %q route %q binds undeclared input %q", method.ID, route.ID, binding.FromInput)
					}
					if input.Consumption != InputConsumptionOperation {
						return fmt.Errorf("method %q route %q binds informational input %q", method.ID, route.ID, binding.FromInput)
					}
					consumedInputs[binding.FromInput] = true
				}
				if binding.FromOperation != "" {
					source, ok := live[binding.FromOperation]
					if !ok || !stringIn(source.Produces, binding.ArtifactKind) {
						return fmt.Errorf("method %q route %q has unconstructible %q binding from %q", method.ID, route.ID, binding.ArtifactKind, binding.FromOperation)
					}
					sourceIndex, sourceExists := routeOperations[binding.FromOperation]
					if !sourceExists || sourceIndex >= targetIndex {
						return fmt.Errorf("method %q route %q binding from %q does not precede %q", method.ID, route.ID, binding.FromOperation, binding.ToOperation)
					}
				}
			}
			for _, operation := range route.Operations[1:] {
				if !targeted[operation] {
					return fmt.Errorf("method %q route %q has no binding for downstream operation %q", method.ID, route.ID, operation)
				}
			}
			for _, input := range route.RequiredInputs {
				if input.Consumption == InputConsumptionOperation && !consumedInputs[input.Name] {
					return fmt.Errorf("method %q route %q consumable input %q has no operation binding", method.ID, route.ID, input.Name)
				}
			}
		}
	}
	return nil
}

type Request struct {
	Method            string                     `json:"method,omitempty"`
	Methods           []string                   `json:"methods,omitempty"`
	Inputs            map[string]json.RawMessage `json:"inputs,omitempty"`
	OperationRequests map[string]json.RawMessage `json:"operation_requests,omitempty"`
	AllowNetwork      *bool                      `json:"allow_network,omitempty"`
	AllowCharges      *bool                      `json:"allow_charges,omitempty"`
}

func (r *Request) UnmarshalJSON(data []byte) error {
	var wire struct {
		Method            string                     `json:"method"`
		Methods           []string                   `json:"methods"`
		Inputs            json.RawMessage            `json:"inputs"`
		OperationRequests map[string]json.RawMessage `json:"operation_requests"`
		AvailableInputs   []string                   `json:"available_inputs"`
		AllowNetwork      *bool                      `json:"allow_network"`
		AllowCharges      *bool                      `json:"allow_charges"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	r.Method, r.Methods = wire.Method, wire.Methods
	r.AllowNetwork, r.AllowCharges = wire.AllowNetwork, wire.AllowCharges
	r.OperationRequests = wire.OperationRequests
	r.Inputs = map[string]json.RawMessage{}
	if len(wire.Inputs) != 0 && string(wire.Inputs) != "null" {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(wire.Inputs, &object); err == nil {
			for name, value := range object {
				r.Inputs[name] = value
			}
		} else {
			var names []string
			if err := json.Unmarshal(wire.Inputs, &names); err != nil {
				return fmt.Errorf("inputs must be an object or an array of names")
			}
			for _, name := range names {
				r.Inputs[name] = json.RawMessage(`true`)
			}
		}
	}
	for _, name := range wire.AvailableInputs {
		r.Inputs[name] = json.RawMessage(`true`)
	}
	return nil
}

func packOperations(pack string) []string {
	seen := map[string]bool{}
	var out []string
	for _, method := range Catalog() {
		if method.Pack != pack {
			continue
		}
		for _, route := range method.Routes {
			for _, operation := range route.Operations {
				if !seen[operation] {
					seen[operation] = true
					out = append(out, operation)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

func stringIn(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
