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
)

type Input struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Route struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Summary        string   `json:"summary"`
	RequiredInputs []Input  `json:"required_inputs"`
	Operations     []string `json:"operations"`
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
				inputs("script", "visual_assets"), "video_compose", "output_review"),
			route("openai-image-animation", "OpenAI-assisted animation", "Generate still visuals with the explicitly named provider, then animate locally.",
				inputs("script"), "openai_image", "video_compose", "output_review")),
		method("avatar", "Avatar", "Presenter-led and consented avatar productions.", "talking-head",
			route("supplied-avatar-edit", "Supplied avatar edit", "Edit supplied presenter or avatar footage without generating an identity.",
				inputs("presenter_media", "consent"), "source_edit", "audio_mix", "output_review"),
			route("edge-voice-avatar", "Edge voice with supplied avatar", "Synthesize speech with the explicitly named free network voice and edit supplied avatar footage.",
				inputs("presenter_media", "script", "consent"), "edge_tts", "source_edit", "audio_mix", "output_review")),
		method("character-animation", "Character animation", "Character-led animation from supplied artwork, poses, or rigs.", "character-animation",
			route("character-remotion", "Local character composition", "Animate supplied character assets in the local composer.",
				inputs("character_assets", "action_plan"), "video_compose", "audio_mix", "output_review")),
		method("content-repurpose", "Content repurpose", "Short-form, vertical, and excerpted edits from supplied media.", "social",
			route("source-repurpose", "Local source repurpose", "Inspect, select, reframe, caption, and review supplied footage.",
				inputs("source_media"), "media_probe", "scene_detect", "source_edit", "subtitle_gen", "output_review")),
		method("documentary-cinematic", "Documentary and cinematic", "Source-led documentary, archival, montage, and cinematic editing.", "cinematic",
			route("source-documentary", "Source-led documentary edit", "Assemble and grade supplied or licensed material locally.",
				inputs("source_media"), "media_probe", "video_stitch", "color_grade", "audio_mix", "output_review"),
			route("wikimedia-documentary", "Wikimedia-augmented documentary", "Search the explicitly named public archive before local assembly.",
				inputs("research_query"), "wikimedia", "video_stitch", "color_grade", "audio_mix", "output_review")),
		method("explainer", "Explainer", "Text-, metric-, and supplied-media-led explanation.", "explainer",
			route("local-explainer", "Local Remotion explainer", "Compose supplied script and visuals with the local renderer.",
				inputs("script", "visuals"), "video_compose", "output_review"),
			route("flux-image-explainer", "FLUX-assisted explainer", "Generate stills with the explicitly named provider, then compose locally.",
				inputs("script"), "flux_image", "video_compose", "output_review")),
		method("localization", "Localization", "Subtitle, dubbing, and localized variants of existing video.", "localization",
			route("subtitle-localization", "Provided-translation subtitle route", "Create and burn supplied translated timed text.",
				inputs("source_media", "translated_segments"), "subtitle_gen", "remotion_caption_burn", "output_review"),
			route("edge-dub-localization", "Edge voice dubbing route", "Synthesize the supplied translated script with the explicitly named voice service.",
				inputs("source_media", "translated_script"), "edge_tts", "audio_mix", "source_edit", "output_review")),
		method("music-led", "Music-led", "Edits paced around supplied or locally discovered music.", "cinematic",
			route("local-music-edit", "Local music-led edit", "Choose local music, edit supplied media, mix, and review.",
				inputs("source_media", "music"), "music_library", "source_edit", "audio_mix", "output_review")),
		method("product-demo", "Product demo", "Recorded or synthetic software and product walkthroughs.", "screen-demo",
			route("recorded-product-demo", "Recorded product demo", "Edit a real product capture and add optional supplied captions.",
				inputs("screen_recording"), "media_probe", "source_edit", "remotion_caption_burn", "output_review"),
			route("synthetic-product-demo", "Synthetic product presentation", "Compose a clearly presentational product sequence locally.",
				inputs("script", "visual_assets"), "video_compose", "output_review")),
		method("source-edit", "Source edit", "Mechanical editing of supplied footage and audio.", "cinematic",
			route("source-edit-local", "Local source edit", "Inspect, edit, mix, and review supplied media locally.",
				inputs("source_media"), "media_probe", "source_edit", "audio_mix", "output_review")),
	}
	sort.Slice(methods, func(i, j int) bool { return methods[i].ID < methods[j].ID })
	return methods
}

func method(id, title, summary, pack string, routes ...Route) Method {
	return Method{ID: id, Title: title, Summary: summary, Pack: pack, Routes: routes}
}

func route(id, title, summary string, required []Input, operations ...string) Route {
	return Route{
		ID: id, Title: title, Summary: summary,
		RequiredInputs: required, Operations: operations,
	}
}

func inputs(names ...string) []Input {
	out := make([]Input, 0, len(names))
	for _, name := range names {
		out = append(out, Input{Name: name, Description: inputDescription(name)})
	}
	return out
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
			for _, operation := range route.Operations {
				if _, ok := live[operation]; !ok {
					return fmt.Errorf("method %q route %q references unavailable operation %q", method.ID, route.ID, operation)
				}
			}
		}
	}
	return nil
}

type Request struct {
	Method       string                     `json:"method,omitempty"`
	Methods      []string                   `json:"methods,omitempty"`
	Inputs       map[string]json.RawMessage `json:"inputs,omitempty"`
	AllowNetwork *bool                      `json:"allow_network,omitempty"`
	AllowCharges *bool                      `json:"allow_charges,omitempty"`
}

func (r *Request) UnmarshalJSON(data []byte) error {
	var wire struct {
		Method          string          `json:"method"`
		Methods         []string        `json:"methods"`
		Inputs          json.RawMessage `json:"inputs"`
		AvailableInputs []string        `json:"available_inputs"`
		AllowNetwork    *bool           `json:"allow_network"`
		AllowCharges    *bool           `json:"allow_charges"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	r.Method, r.Methods = wire.Method, wire.Methods
	r.AllowNetwork, r.AllowCharges = wire.AllowNetwork, wire.AllowCharges
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
