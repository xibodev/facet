package toolbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxDiagnostic = 8192

var names = []string{
	"audio_mix",
	"audio_mixer",
	"audio_probe",
	"color_grade",
	"direct_clip_search",
	"edge_tts",
	"elevenlabs_tts",
	"flux_image",
	"frame_sample",
	"frame_sampler",
	"gflow_image",
	"gflow_video",
	"hyperframes_compose",
	"image_selector",
	"kling_video",
	"media_probe",
	"music_library",
	"openai_image",
	"openai_tts",
	"output_review",
	"pexels_video",
	"piper_tts",
	"pixabay_video",
	"remotion_caption_burn",
	"scene_detect",
	"silence_cutter",
	"sora_video",
	"source_edit",
	"subtitle_gen",
	"video_compose",
	"video_selector",
	"video_stitch",
	"video_trimmer",
	"visual_qa",
	"wikimedia",
}

type Execution struct {
	Provider      string   `json:"provider"`
	Network       bool     `json:"network"`
	ExternalWrite bool     `json:"external_write"`
	EstimatedCost *float64 `json:"estimated_cost"`
	ActualCost    *float64 `json:"actual_cost"`
}

type Envelope struct {
	OK        bool       `json:"ok"`
	Tool      string     `json:"tool,omitempty"`
	Operation string     `json:"operation"`
	Result    any        `json:"result,omitempty"`
	Error     *ToolError `json:"error,omitempty"`
	Warnings  []string   `json:"warnings"`
	Execution Execution  `json:"execution"`
}

type ToolError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details"`
}

type toolFailure struct{ err *ToolError }

func (e *toolFailure) Error() string { return e.err.Message }

func failure(code, message string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	return &toolFailure{&ToolError{
		Code: code, Message: message, Retryable: retryableCode(code), Details: details,
	}}
}

// retryableCode says whether the SAME request could succeed if tried again.
//
// ToolError.Retryable was declared and never set anywhere, so every error told
// the host "do not retry" — including a timeout, which is the one failure a
// larger budget reliably fixes. A host that honours the flag would give up on
// a render that needed nothing but more time.
//
// Decided by code rather than per call site: there are 116 command_failed
// constructions alone, and a judgement repeated at every one of them is a
// judgement that drifts.
//
// Retryable means TRANSIENT, not "worth a second attempt by a human":
//   - a timeout may fit in a larger budget
//   - a provider returning something unusable is usually a transient upstream
//     fault rather than a wrong request
//
// Everything else is deliberately false. A malformed request, a missing
// credential, an absent dependency and an input that does not exist all fail
// identically on a retry, and telling a host otherwise invites a loop that
// burns time and, for a paid provider, money.
func retryableCode(code string) bool {
	switch code {
	case "command_timeout", "provider_response_invalid":
		return true
	}
	return false
}

func executionFor(tool string) Execution {
	provider := "local"
	network := false
	switch tool {
	case "media_probe", "audio_probe":
		provider = "ffprobe"
	case "color_grade":
		provider = "ffmpeg"
	case "direct_clip_search":
		provider = "openmontage"
		network = true
	case "edge_tts":
		provider = "microsoft_edge"
		network = true
	case "elevenlabs_tts":
		provider = "elevenlabs"
		network = true
	case "flux_image":
		provider = "fal"
		network = true
	case "image_selector":
		provider = "selector"
	case "kling_video":
		provider = "kling"
		network = true
	case "openai_image":
		provider = "openai"
		network = true
	case "openai_tts":
		provider = "openai"
		network = true
	case "pexels_video":
		provider = "pexels"
		network = true
	case "pixabay_video":
		provider = "pixabay"
		network = true
	case "piper_tts":
		provider = "piper"
	case "remotion_caption_burn":
		provider = "remotion"
	case "sora_video":
		provider = "openai"
		network = true
	case "subtitle_gen":
		provider = "openmontage"
	case "video_selector":
		provider = "selector"
	case "wikimedia":
		provider = "wikimedia"
		network = true
	case "hyperframes_compose":
		provider = "hyperframes"
	case "gflow_video", "gflow_image":
		provider = "google_flow"
		network = true
	}
	zero := 0.0
	e := Execution{Provider: provider, Network: network, EstimatedCost: &zero, ActualCost: &zero}
	switch tool {
	case "gflow_video", "gflow_image", "openai_image", "flux_image", "kling_video", "sora_video", "openai_tts", "elevenlabs_tts":
		e.EstimatedCost, e.ActualCost = nil, nil
	}
	return e
}

// readOnlyTools inspect existing files and report facts about them. Everything
// else may write, and an unrecognised tool is assumed to write.
//
// This exists because Execution.ExternalWrite was declared and never assigned,
// so every tool — including ones that demonstrably write media — reported
// external_write=false. A module host uses declared effects to decide whether
// approval is required, so a constant false routes write-performing tools
// around approval. It fails OPEN, which is why the unknown case here is true.
var readOnlyTools = map[string]bool{
	"media_probe": true, "audio_probe": true, "music_library": true,
	"image_selector": true, "video_selector": true,
}

// externalWriteFor reports whether an operation can write outside the process.
// Estimation validates a request and never writes output, by contract.
func externalWriteFor(tool, op string) bool {
	if op != "run" {
		return false
	}
	return !readOnlyTools[tool]
}

func canonicalToolName(tool string) string {
	tool = strings.ToLower(strings.TrimSpace(tool))
	for _, n := range names {
		if n == tool {
			return tool
		}
	}
	switch tool {
	case "edgetts", "edge-tts":
		return "edge_tts"
	case "edit", "source-edit":
		return "source_edit"
	case "probe", "media-probe":
		return "media_probe"
	case "audio-probe":
		return "audio_probe"
	case "output-review", "review":
		return "output_review"
	case "frame-sample":
		return "frame_sample"
	case "audio-mix":
		return "audio_mix"
	case "video-compose", "compose":
		return "video_compose"
	case "remotion_render", "remotion-render":
		return "video_compose"
	case "hyperframes", "hyperframes-compose":
		return "hyperframes_compose"
	case "gflow-video", "gflowvideo", "veo":
		return "gflow_video"
	case "gflow-image", "gflowimage", "imagen":
		return "gflow_image"
	case "openai-tts", "openaitts":
		return "openai_tts"
	case "elevenlabs-tts", "elevenlabs":
		return "elevenlabs_tts"
	case "piper-tts", "piper":
		return "piper_tts"
	case "subtitle-gen", "subtitles":
		return "subtitle_gen"
	case "scene-detect":
		return "scene_detect"
	case "silence-cutter":
		return "silence_cutter"
	case "color-grade":
		return "color_grade"
	case "video-trimmer", "trimmer":
		return "video_trimmer"
	case "video-stitch", "stitch":
		return "video_stitch"
	case "visual-qa", "vqa":
		return "visual_qa"
	case "clip_search", "direct-clip-search":
		return "direct_clip_search"
	default:
		return tool
	}
}

func CLI(args []string) (Envelope, bool) {
	op, tool := "", ""
	bad := func(message string) (Envelope, bool) {
		return errorEnvelope(tool, op, failure("invalid_request", message, nil)), false
	}
	if len(args) >= 1 && args[0] == "studio" {
		return success("", "studio", map[string]any{
			"command":      "studio",
			"description":  "Video Kit Studio web interface",
			"default_port": 8787,
			"default_dir":  ".",
		}, nil), true
	}
	if len(args) < 2 || args[0] != "tools" {
		return bad("usage: videokit tools <list|describe|estimate|run>")
	}
	op = args[1]
	switch op {
	case "list":
		if len(args) != 2 {
			return bad("tools list accepts no arguments")
		}
		items := make([]any, 0, len(names))
		for _, n := range names {
			items = append(items, summary(n))
		}
		return success("", op, map[string]any{"tools": items}, nil), true
	case "describe":
		if len(args) != 3 {
			return bad("usage: videokit tools describe <tool>")
		}
		tool = canonicalToolName(args[2])
		if !known(tool) {
			return bad("unknown tool: " + args[2])
		}
		return success(tool, op, description(tool), nil), true
	case "estimate", "run":
		if len(args) != 5 || args[3] != "--input" {
			return bad("usage: videokit tools " + op + " <tool> --input <request.json>")
		}
		tool = canonicalToolName(args[2])
		if !known(tool) {
			return bad("unknown tool: " + args[2])
		}
		var data []byte
		rawInput := strings.TrimSpace(args[4])
		unquoted := strings.Trim(rawInput, "'`\"")
		unquoted = strings.TrimSpace(unquoted)
		if (strings.HasPrefix(unquoted, "{") && strings.HasSuffix(unquoted, "}")) ||
			(strings.HasPrefix(unquoted, "[") && strings.HasSuffix(unquoted, "]")) {
			data = []byte(unquoted)
		} else {
			var err error
			data, err = os.ReadFile(args[4])
			if err != nil {
				return errorEnvelope(tool, op, failure("input_not_found", "request input could not be read", map[string]any{"path": args[4], "error": bounded(err.Error())})), false
			}
		}
		result, warnings, err := execute(tool, op, data)
		if err != nil {
			return errorEnvelope(tool, op, err), false
		}
		return success(tool, op, result, warnings), true
	default:
		return bad("unknown tools operation: " + op)
	}
}

func success(tool, op string, result any, warnings []string) Envelope {
	if warnings == nil {
		warnings = []string{}
	}
	e := executionFor(tool)
	if facts, ok := result.(map[string]any); ok {
		if cost, ok := facts["estimated_cost"].(float64); ok {
			e.EstimatedCost = &cost
		}
		if mock, _ := facts["mock"].(bool); mock {
			zero := 0.0
			e.EstimatedCost, e.ActualCost, e.Network = &zero, &zero, false
		}
	}
	if op == "estimate" {
		zero := 0.0
		e.ActualCost = &zero // Estimation does not generate or bill media.
	}
	e.ExternalWrite = externalWriteFor(tool, op)
	return Envelope{OK: true, Tool: tool, Operation: op, Result: result, Warnings: warnings, Execution: e}
}

func errorEnvelope(tool, op string, err error) Envelope {
	te := &ToolError{Code: "command_failed", Message: bounded(err.Error()), Details: map[string]any{}}
	var tf *toolFailure
	if errors.As(err, &tf) {
		te = tf.err
	}
	e := executionFor(tool)
	e.ExternalWrite = externalWriteFor(tool, op)
	return Envelope{OK: false, Tool: tool, Operation: op, Error: te, Warnings: []string{}, Execution: e}
}

func known(name string) bool {
	cName := canonicalToolName(name)
	for _, n := range names {
		if n == cName {
			return true
		}
	}
	return false
}

func dependency(name string) map[string]any {
	path, err := lookPath(name)
	return map[string]any{"name": name, "available": err == nil, "path": path, "type": "binary"}
}

func envDependency(name string) map[string]any {
	val := os.Getenv(name)
	return map[string]any{"name": name, "available": val != "", "path": "", "type": "env"}
}

func summary(name string) map[string]any {
	deps := []any{}
	switch name {
	case "media_probe", "audio_probe":
		deps = append(deps, dependency("ffprobe"))
	case "frame_sample", "frame_sampler", "scene_detect", "visual_qa", "output_review", "source_edit", "video_trimmer", "video_stitch", "silence_cutter", "audio_mix", "audio_mixer":
		deps = append(deps, dependency("ffmpeg"), dependency("ffprobe"))
	case "video_compose", "remotion_caption_burn", "color_grade":
		deps = append(deps, dependency("ffmpeg"))
	case "hyperframes_compose":
		deps = append(deps, dependency("npx"), dependency("ffmpeg"))
	case "music_library":
		// optional ffprobe
		deps = append(deps, dependency("ffprobe"))
	case "direct_clip_search":
		deps = append(deps, dependency("ffmpeg"), dependency("ffprobe"))
	case "pexels_video":
		deps = append(deps, envDependency("PEXELS_API_KEY"))
	case "pixabay_video":
		deps = append(deps, envDependency("PIXABAY_API_KEY"))
	case "openai_tts", "openai_image", "sora_video":
		deps = append(deps, envDependency("OPENAI_API_KEY"))
	case "elevenlabs_tts":
		deps = append(deps, envDependency("ELEVENLABS_API_KEY"))
	case "flux_image", "kling_video":
		deps = append(deps, envDependency("FAL_KEY"))
	case "piper_tts":
		deps = append(deps, dependency("piper"))
	case "gflow_video", "gflow_image":
		deps = append(deps, dependency("gflow"))
	case "edge_tts", "subtitle_gen", "wikimedia", "image_selector", "video_selector":
		// pure go / public api / selector logic
	}

	configured := dependenciesAvailable(deps)
	if name == "music_library" {
		configured = true
	} else if name == "direct_clip_search" {
		configured = true // Wikimedia always available
	} else if name == "image_selector" || name == "video_selector" {
		configured = true
	}

	return map[string]any{
		"name":         name,
		"capability":   capabilities[name],
		"implemented":  true,
		"configured":   configured,
		"dependencies": deps,
	}
}

func dependenciesAvailable(deps []any) bool {
	for _, d := range deps {
		if m, ok := d.(map[string]any); ok {
			if avail, ok := m["available"].(bool); ok && !avail {
				return false
			}
		}
	}
	return true
}

var capabilities = map[string]string{
	"audio_mix":             "audio mixing",
	"audio_mixer":           "audio mixing",
	"audio_probe":           "audio metadata inspection",
	"color_grade":           "FFmpeg LUT and color grading tool",
	"direct_clip_search":    "stock clip search and download",
	"edge_tts":              "free keyless Microsoft Edge neural text-to-speech synthesis",
	"elevenlabs_tts":        "cloud text-to-speech synthesis",
	"flux_image":            "cloud AI image generation via FLUX",
	"frame_sample":          "review frame extraction",
	"frame_sampler":         "frame extraction and sampling",
	"gflow_image":           "Google Flow Imagen 4 / Nano Banana 2 image generation",
	"gflow_video":           "Google Flow Veo 3.1 cinematic video generation and 4K upsampling",
	"hyperframes_compose":   "HTML/CSS/GSAP video composition",
	"image_selector":        "image provider discovery, facts, and explainable ranking",
	"kling_video":           "cloud AI video generation via Kling",
	"media_probe":           "media inspection",
	"music_library":         "local music discovery and indexing",
	"openai_image":          "cloud AI image generation via OpenAI DALL-E / GPT Image",
	"openai_tts":            "cloud text-to-speech synthesis",
	"output_review":         "technical output review",
	"pexels_video":          "stock video search and download",
	"piper_tts":             "local text-to-speech synthesis",
	"pixabay_video":         "stock video search and download",
	"remotion_caption_burn": "animated caption burning",
	"scene_detect":          "scene cut and shot boundary detection",
	"silence_cutter":        "silence detection and jump cut editing",
	"sora_video":            "cloud AI video generation via Sora",
	"source_edit":           "supplied-footage editing",
	"subtitle_gen":          "subtitle generation (SRT/VTT/JSON)",
	"video_compose":         "video composition orchestration",
	"video_selector":        "video provider discovery, facts, duration limits, and explainable ranking",
	"video_stitch":          "multi-clip assembly and transitions",
	"video_trimmer":         "video trimming, speed, and concatenation",
	"visual_qa":             "visual quality assurance and inspection",
	"wikimedia":             "Wikimedia Commons stock search and download",
}

func description(name string) map[string]any {
	d := summary(name)
	exec := executionFor(name)
	d["provider"] = exec.Provider
	d["request_schema"] = schemas[name]
	d["result_schema"] = resultSchemas[name]
	d["cost"] = map[string]any{"currency": "USD", "amount": exec.EstimatedCost, "known": exec.EstimatedCost != nil}
	d["network"] = exec.Network
	// A `run` of this tool may write; describe reports the run behaviour, which
	// is what a caller is deciding about.
	d["external_write"] = externalWriteFor(name, "run")
	return d
}

var schemas = map[string]any{
	"media_probe": map[string]any{"type": "object", "additionalProperties": false, "anyOf": []any{map[string]any{"required": []string{"input"}}, map[string]any{"required": []string{"input_path"}}}, "properties": map[string]any{
		"input": map[string]any{"type": "string", "minLength": 1}, "input_path": map[string]any{"type": "string", "minLength": 1}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "default": 30},
	}},
	"audio_probe": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"input_path"}, "properties": map[string]any{
		"input_path": map[string]any{"type": "string", "minLength": 1}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "default": 15},
	}},
	"frame_sample": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"output_dir", "strategy"}, "anyOf": []any{map[string]any{"required": []string{"input"}}, map[string]any{"required": []string{"input_path"}}}, "properties": map[string]any{
		"input": map[string]any{"type": "string", "minLength": 1}, "input_path": map[string]any{"type": "string", "minLength": 1}, "output_dir": map[string]any{"type": "string", "minLength": 1}, "format": map[string]any{"enum": []string{"jpg", "png"}},
		"strategy": map[string]any{"oneOf": []any{
			objectSchema([]string{"type", "timestamps"}, map[string]any{"type": map[string]any{"const": "timestamps"}, "timestamps": map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "number", "minimum": 0}}}),
			objectSchema([]string{"type", "count"}, map[string]any{"type": map[string]any{"const": "uniform"}, "count": map[string]any{"type": "integer", "minimum": 1}}),
			objectSchema([]string{"type", "count"}, map[string]any{"type": map[string]any{"const": "scenes"}, "count": map[string]any{"type": "integer", "minimum": 1}, "threshold": map[string]any{"type": "number", "exclusiveMinimum": 0, "exclusiveMaximum": 1}}),
		}},
		"image_format": map[string]any{"enum": []string{"jpg", "png"}, "default": "jpg"}, "overwrite": map[string]any{"type": "boolean", "default": false}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "default": 60},
	}},
	"frame_sampler": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"input_path", "strategy"}, "properties": map[string]any{
		"input_path": map[string]any{"type": "string", "minLength": 1}, "strategy": map[string]any{"enum": []string{"interval", "count", "timestamps", "scene_guided"}},
		"interval_seconds": map[string]any{"type": "number", "minimum": 0.1}, "count": map[string]any{"type": "integer", "minimum": 1}, "timestamps": map[string]any{"type": "array", "items": map[string]any{"type": "number"}},
		"output_dir": map[string]any{"type": "string"}, "format": map[string]any{"enum": []string{"jpg", "png"}, "default": "jpg"}, "quality": map[string]any{"type": "integer", "default": 2},
	}},
	"scene_detect": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"input_path"}, "properties": map[string]any{
		"input_path": map[string]any{"type": "string", "minLength": 1}, "method": map[string]any{"enum": []string{"content", "threshold", "adaptive"}, "default": "content"},
		"threshold": map[string]any{"type": "number", "default": 0.3}, "min_scene_length_seconds": map[string]any{"type": "number", "default": 1.0}, "output_path": map[string]any{"type": "string"},
	}},
	"visual_qa": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"operation", "input_path"}, "properties": map[string]any{
		"operation": map[string]any{"enum": []string{"review", "probe", "audio_levels"}}, "input_path": map[string]any{"type": "string", "minLength": 1},
		"timestamps": map[string]any{"type": "array", "items": map[string]any{"type": "number"}}, "output_dir": map[string]any{"type": "string"}, "expected": map[string]any{"type": "object"},
	}},
	"output_review": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
		"input": map[string]any{"type": "string", "minLength": 1}, "rendered_file": map[string]any{"type": "string", "minLength": 1}, "input_path": map[string]any{"type": "string", "minLength": 1}, "sample_count": map[string]any{"type": "integer", "minimum": 1},
		"profile": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"width", "height", "fps"}, "properties": map[string]any{"width": map[string]any{"type": "integer", "minimum": 1}, "height": map[string]any{"type": "integer", "minimum": 1}, "fps": map[string]any{"type": "number", "exclusiveMinimum": 0}}},
		"checks":  map[string]any{"type": "object", "additionalProperties": false, "required": []string{"duration", "video_codec", "pixel_format", "audio"}, "properties": map[string]any{"duration": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"expected", "tolerance"}, "properties": map[string]any{"expected": map[string]any{"type": "number", "exclusiveMinimum": 0}, "tolerance": map[string]any{"type": "number", "minimum": 0}}}, "video_codec": map[string]any{"type": "string", "minLength": 1}, "pixel_format": map[string]any{"type": "string", "minLength": 1}, "audio": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"required", "codec", "sample_rate", "channels"}, "properties": map[string]any{"required": map[string]any{"type": "boolean"}, "codec": map[string]any{"type": "string", "minLength": 1}, "sample_rate": map[string]any{"type": "integer", "minimum": 1}, "channels": map[string]any{"type": "integer", "minimum": 1}}}}},
		"samples": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"type": map[string]any{"const": "uniform"}, "count": map[string]any{"type": "integer", "minimum": 1}}}, "evidence_dir": map[string]any{"type": "string", "minLength": 1}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "default": 90},
	}},
	"source_edit": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"segments", "target", "output"}, "properties": map[string]any{
		"segments":          map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"input", "start", "end"}, "properties": map[string]any{"input": map[string]any{"type": "string", "minLength": 1}, "start": map[string]any{"type": "number", "minimum": 0}, "end": map[string]any{"type": "number", "exclusiveMinimum": 0}, "transition": map[string]any{"enum": []string{"cut"}}, "position": map[string]any{"enum": framePositions}, "focal_point": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"x", "y"}, "properties": map[string]any{"x": map[string]any{"type": "number", "minimum": 0, "maximum": 1}, "y": map[string]any{"type": "number", "minimum": 0, "maximum": 1}}}}}},
		"target":            map[string]any{"type": "object", "additionalProperties": false, "required": []string{"width", "height", "fps", "fit", "video_codec", "pixel_format", "audio_codec", "audio_sample_rate", "audio_channels"}, "properties": map[string]any{"width": map[string]any{"type": "integer", "minimum": 2, "multipleOf": 2}, "height": map[string]any{"type": "integer", "minimum": 2, "multipleOf": 2}, "fps": map[string]any{"type": "number", "exclusiveMinimum": 0}, "fit": map[string]any{"enum": []string{"contain", "cover"}}, "video_codec": map[string]any{"const": "h264"}, "pixel_format": map[string]any{"const": "yuv420p"}, "audio_codec": map[string]any{"const": "aac"}, "audio_sample_rate": map[string]any{"const": 48000}, "audio_channels": map[string]any{"const": 2}}},
		"replacement_audio": map[string]any{"type": "string", "minLength": 1}, "output": map[string]any{"type": "string", "minLength": 1}, "overwrite": map[string]any{"type": "boolean", "default": false}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "default": 300},
	}},
	"video_trimmer": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"operation", "input_path"}, "properties": map[string]any{
		"operation": map[string]any{"enum": []string{"cut", "speed", "concat"}}, "input_path": map[string]any{"type": "string"}, "output_path": map[string]any{"type": "string"},
		"start_seconds": map[string]any{"type": "number"}, "end_seconds": map[string]any{"type": "number"}, "speed_factor": map[string]any{"type": "number"}, "codec": map[string]any{"type": "string", "default": "copy"},
	}},
	"video_stitch": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"operation", "clips"}, "properties": map[string]any{
		"operation": map[string]any{"enum": []string{"validate", "stitch", "preview_stitch", "spatial"}}, "clips": map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string"}},
		"output_path": map[string]any{"type": "string"}, "transition": map[string]any{"enum": []string{"cut", "crossfade", "fade"}, "default": "cut"}, "transition_duration": map[string]any{"type": "number", "default": 0.5},
		"auto_normalize": map[string]any{"type": "boolean", "default": false}, "layout": map[string]any{"enum": []string{"side_by_side", "vertical_stack", "picture_in_picture"}},
	}},
	"video_compose": map[string]any{"type": "object", "additionalProperties": false, "description": "Accepts an operation envelope (default compose), direct Remotion props with nonempty cuts, or a scene plan with nonempty scenes. Direct cuts take precedence over scenes and operation. Direct Explainer props support width, height, fps and duration_seconds; defaults are 1920x1080 at 30 fps, with one second padding after the last cut only when duration_seconds is omitted. Metadata validation runs in Remotion, not in estimates; estimates do not deeply validate props or prove a render will succeed.", "properties": map[string]any{
		"operation": map[string]any{"enum": []string{"compose", "render", "remotion_render", "burn_subtitles", "overlay", "encode"}}, "input_path": map[string]any{"type": "string"}, "output_path": map[string]any{"type": "string"},
		"edit_decisions": map[string]any{"type": "object"}, "asset_manifest": map[string]any{"type": "object"}, "audio_path": map[string]any{"type": "string"}, "subtitle_path": map[string]any{"type": "string"},
		"output": stringSchema(), "composition_id": stringSchema(), "composition": stringSchema(), "theme": stringSchema(), "playbook": stringSchema(), "themeConfig": map[string]any{"type": "object"}, "style_playbook": stringSchema(),
		"width":            map[string]any{"type": "integer", "minimum": 2, "maximum": 9007199254740991, "multipleOf": 2, "default": 1920, "description": "Direct Explainer props: positive even safe integer pixels."},
		"height":           map[string]any{"type": "integer", "minimum": 2, "maximum": 9007199254740991, "multipleOf": 2, "default": 1080, "description": "Direct Explainer props: positive even safe integer pixels."},
		"fps":              map[string]any{"type": "number", "exclusiveMinimum": 0, "default": 30, "description": "Direct Explainer props: positive finite frame rate, used for metadata and scene timing."},
		"duration_seconds": map[string]any{"type": "number", "exclusiveMinimum": 0, "description": "Direct Explainer props: exact duration; duration_seconds * fps must be a positive safe integer frame count. Cuts must fit within it and span at least one frame after boundary rounding. Omit for last cut out_seconds + 1 second (60 seconds with no cuts in Remotion)."},
		"cuts":             map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "object", "required": []string{"in_seconds", "out_seconds"}, "properties": map[string]any{"in_seconds": map[string]any{"type": "number", "minimum": 0}, "out_seconds": map[string]any{"type": "number", "exclusiveMinimum": 0}}}},
		"scenes":           map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "object", "required": []string{"start_seconds", "end_seconds"}}},
		"overlays":         map[string]any{"type": "array", "items": map[string]any{"type": "object"}}, "captions": map[string]any{}, "audio": map[string]any{}, "subtitle_style": map[string]any{"type": "object"},
		"codec": stringSchema(), "crf": map[string]any{"type": "integer"}, "preset": stringSchema(), "profile": stringSchema(), "remotion_timeout_ms": map[string]any{"type": "integer"}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1},
	}},
	"subtitle_gen": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"segments"}, "properties": map[string]any{
		"segments": map[string]any{"type": "array"}, "format": map[string]any{"enum": []string{"srt", "vtt", "json"}, "default": "srt"},
		"output_path": map[string]any{"type": "string"}, "max_chars_per_line": map[string]any{"type": "integer", "default": 42}, "max_words_per_cue": map[string]any{"type": "integer", "default": 8},
		"highlight_style": map[string]any{"enum": []string{"none", "word_by_word", "karaoke"}, "default": "none"}, "corrections": map[string]any{"type": "object"},
	}},
	"remotion_caption_burn": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"input_path", "output_path"}, "properties": map[string]any{
		"input_path": map[string]any{"type": "string"}, "output_path": map[string]any{"type": "string"}, "segments": map[string]any{"type": "array"},
		"srt_path": map[string]any{"type": "string"}, "words_per_page": map[string]any{"type": "integer", "default": 4}, "font_size": map[string]any{"type": "integer", "default": 52},
		"highlight_color": map[string]any{"type": "string", "default": "#22D3EE"}, "force_ffmpeg": map[string]any{"type": "boolean", "default": false},
	}},
	"silence_cutter": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"input_path"}, "properties": map[string]any{
		"input_path": map[string]any{"type": "string"}, "output_path": map[string]any{"type": "string"}, "mode": map[string]any{"enum": []string{"remove", "speed_up", "mark"}, "default": "remove"},
		"silence_threshold_db": map[string]any{"type": "number", "default": -35}, "min_silence_duration": map[string]any{"type": "number", "default": 0.5}, "padding_seconds": map[string]any{"type": "number", "default": 0.08},
	}},
	"hyperframes_compose": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"operation"}, "properties": map[string]any{
		"operation":      map[string]any{"enum": []string{"doctor", "scaffold_workspace", "lint", "validate", "inspect", "check", "render", "render_existing", "add_block"}},
		"workspace_path": map[string]any{"type": "string"}, "output_path": map[string]any{"type": "string"}, "block_name": map[string]any{"type": "string"},
	}},
	"audio_mix": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"video", "duration", "output"}, "properties": map[string]any{
		"video": map[string]any{"type": "string", "minLength": 1}, "source": audioOperationSchema(false), "music": audioOperationSchema(true),
		"loudness": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"enabled"}, "properties": map[string]any{"enabled": map[string]any{"type": "boolean"}, "integrated_lufs": map[string]any{"type": "number", "minimum": -70, "maximum": -5}, "true_peak_db": map[string]any{"type": "number", "minimum": -9, "maximum": 0}}},
		"duration": map[string]any{"const": "video"}, "output": map[string]any{"type": "string", "minLength": 1}, "overwrite": map[string]any{"type": "boolean", "default": false}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "default": 300},
	}},
	"audio_mixer": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"video", "duration", "output"}, "properties": map[string]any{
		"video": map[string]any{"type": "string", "minLength": 1}, "source": audioOperationSchema(false), "music": audioOperationSchema(true),
		"loudness": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"enabled"}, "properties": map[string]any{"enabled": map[string]any{"type": "boolean"}, "integrated_lufs": map[string]any{"type": "number", "minimum": -70, "maximum": -5}, "true_peak_db": map[string]any{"type": "number", "minimum": -9, "maximum": 0}}},
		"duration": map[string]any{"const": "video"}, "output": map[string]any{"type": "string", "minLength": 1}, "overwrite": map[string]any{"type": "boolean", "default": false}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "default": 300},
	}},
	"music_library": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
		"library_dir": map[string]any{"type": "string"},
	}},
	"direct_clip_search": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"output_dir", "queries"}, "properties": map[string]any{
		"output_dir": map[string]any{"type": "string"}, "queries": map[string]any{"type": "array", "items": map[string]any{"type": "object", "required": []string{"query"}, "properties": map[string]any{"query": map[string]any{"type": "string"}, "slot_id": map[string]any{"type": "string"}, "kind": map[string]any{"enum": []string{"video", "image", "any"}}}}},
		"clips_per_query": map[string]any{"type": "integer", "default": 3}, "extract_thumbnails": map[string]any{"type": "boolean", "default": true}, "skip_existing": map[string]any{"type": "boolean", "default": true},
	}},
	"pexels_video": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"query"}, "properties": map[string]any{
		"query": map[string]any{"type": "string"}, "orientation": map[string]any{"enum": []string{"landscape", "portrait", "square"}}, "size": map[string]any{"enum": []string{"large", "medium", "small"}},
		"min_duration": map[string]any{"type": "integer"}, "max_duration": map[string]any{"type": "integer"}, "per_page": map[string]any{"type": "integer", "default": 5}, "output_path": map[string]any{"type": "string"},
	}},
	"pixabay_video": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"query"}, "properties": map[string]any{
		"query": map[string]any{"type": "string"}, "video_type": map[string]any{"enum": []string{"all", "film", "animation"}, "default": "all"},
		"category": map[string]any{"type": "string"}, "per_page": map[string]any{"type": "integer", "default": 5}, "output_path": map[string]any{"type": "string"},
	}},
	"wikimedia": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"query"}, "properties": map[string]any{
		"query": map[string]any{"type": "string"}, "kind": map[string]any{"enum": []string{"video", "image", "any"}, "default": "video"}, "per_page": map[string]any{"type": "integer", "default": 5}, "output_path": map[string]any{"type": "string"},
	}},
	"edge_tts": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"text"}, "properties": map[string]any{
		"text": map[string]any{"type": "string"}, "voice": map[string]any{"type": "string", "default": "en-US-ChristopherNeural"}, "rate": map[string]any{"type": "string", "default": "+0%"},
		"pitch": map[string]any{"type": "string", "default": "+0Hz"}, "volume": map[string]any{"type": "string", "default": "+0%"}, "output_path": map[string]any{"type": "string"}, "timeout_seconds": map[string]any{"type": "integer"},
	}},
	"openai_tts": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"text"}, "properties": map[string]any{
		"text": map[string]any{"type": "string"}, "voice": map[string]any{"type": "string", "default": "alloy"}, "model": map[string]any{"type": "string", "default": "gpt-4o-mini-tts"},
		"response_format": map[string]any{"enum": []string{"mp3", "opus", "aac", "flac", "wav", "pcm"}, "default": "mp3"}, "instructions": map[string]any{"type": "string"}, "speed": map[string]any{"type": "number", "default": 1.0}, "output_path": map[string]any{"type": "string"},
	}},
	"elevenlabs_tts": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"text"}, "properties": map[string]any{
		"text": map[string]any{"type": "string"}, "voice_id": map[string]any{"type": "string"}, "model_id": map[string]any{"type": "string", "default": "eleven_multilingual_v2"},
		"stability": map[string]any{"type": "number", "default": 0.5}, "similarity_boost": map[string]any{"type": "number", "default": 0.75}, "output_format": map[string]any{"type": "string", "default": "mp3_44100_128"}, "output_path": map[string]any{"type": "string"},
	}},
	"piper_tts": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"text"}, "properties": map[string]any{
		"text": map[string]any{"type": "string"}, "model": map[string]any{"type": "string", "default": "en_US-lessac-medium"}, "speaker_id": map[string]any{"type": "integer", "default": 0},
		"length_scale": map[string]any{"type": "number", "default": 1.0}, "sentence_silence": map[string]any{"type": "number", "default": 0.3}, "output_path": map[string]any{"type": "string"},
	}},
	"openai_image": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"prompt"}, "properties": map[string]any{
		"prompt": map[string]any{"type": "string", "minLength": 1}, "model": map[string]any{"type": "string", "default": "dall-e-3"},
		"size": map[string]any{"type": "string", "default": "1024x1024"}, "aspect_ratio": map[string]any{"enum": []string{"1:1", "16:9", "9:16", "4:3", "3:4"}},
		"quality": map[string]any{"enum": []string{"standard", "hd"}, "default": "standard"}, "style": map[string]any{"enum": []string{"vivid", "natural"}, "default": "vivid"},
		"output_path": map[string]any{"type": "string"}, "mock": map[string]any{"type": "boolean", "default": false}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "default": 120},
	}},
	"flux_image": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"prompt"}, "properties": map[string]any{
		"prompt": map[string]any{"type": "string", "minLength": 1}, "model": map[string]any{"type": "string", "default": "fal-ai/flux-pro"},
		"aspect_ratio": map[string]any{"enum": []string{"1:1", "16:9", "9:16", "4:3", "3:4", "21:9"}, "default": "16:9"}, "image_size": map[string]any{"type": "string"},
		"num_images": map[string]any{"type": "integer", "minimum": 1, "default": 1}, "guidance_scale": map[string]any{"type": "number"}, "seed": map[string]any{"type": "integer"},
		"output_path": map[string]any{"type": "string"}, "mock": map[string]any{"type": "boolean", "default": false}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "default": 120},
	}},
	"kling_video": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"prompt"}, "properties": map[string]any{
		"prompt": map[string]any{"type": "string", "minLength": 1}, "model": map[string]any{"type": "string", "default": "fal-ai/kling-video"},
		"duration": map[string]any{"type": "number", "default": 5}, "aspect_ratio": map[string]any{"enum": []string{"16:9", "9:16", "1:1"}, "default": "16:9"},
		"mode": map[string]any{"enum": []string{"std", "pro"}, "default": "std"}, "image_url": map[string]any{"type": "string"},
		"output_path": map[string]any{"type": "string"}, "mock": map[string]any{"type": "boolean", "default": false}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "default": 300},
	}},
	"sora_video": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"prompt"}, "properties": map[string]any{
		"prompt": map[string]any{"type": "string", "minLength": 1}, "model": map[string]any{"type": "string", "default": "sora-2"},
		"duration": map[string]any{"type": "number", "minimum": 1, "maximum": 20, "default": 5}, "aspect_ratio": map[string]any{"enum": []string{"16:9", "9:16", "1:1"}, "default": "16:9"},
		"resolution":  map[string]any{"enum": []string{"720p", "1080p"}, "default": "720p"},
		"output_path": map[string]any{"type": "string"}, "mock": map[string]any{"type": "boolean", "default": false}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "default": 300},
	}},
	"gflow_video": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"prompt"}, "properties": map[string]any{
		"prompt": map[string]any{"type": "string", "minLength": 1}, "model": map[string]any{"const": "veo-3.1", "default": "veo-3.1"},
		"duration": map[string]any{"enum": []int{4, 6, 8, 10}, "default": 6}, "aspect_ratio": map[string]any{"enum": []string{"landscape", "portrait", "square"}, "default": "landscape"},
		"resolution": map[string]any{"enum": []string{"720p", "1080p", "4k"}, "default": "1080p"}, "start_frame": map[string]any{"type": "string"}, "end_frame": map[string]any{"type": "string"},
		"output_path": map[string]any{"type": "string"}, "mock": map[string]any{"type": "boolean", "default": false}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "default": 300},
	}},
	"gflow_image": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"prompt"}, "properties": map[string]any{
		"prompt": map[string]any{"type": "string", "minLength": 1}, "model": map[string]any{"enum": []string{"narwhal", "harbor_seal", "gem_pix_2"}, "default": "narwhal"},
		"aspect_ratio": map[string]any{"enum": []string{"landscape", "portrait", "square", "4:3", "3:4"}, "default": "landscape"}, "count": map[string]any{"type": "integer", "minimum": 1, "maximum": 4, "default": 1},
		"reference_image": map[string]any{"type": "string"}, "output_path": map[string]any{"type": "string"}, "mock": map[string]any{"type": "boolean", "default": false}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "default": 180},
	}},
	"color_grade": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"input_path", "output_path"}, "properties": map[string]any{
		"input_path": map[string]any{"type": "string", "minLength": 1}, "output_path": map[string]any{"type": "string", "minLength": 1},
		"profile":   map[string]any{"enum": []string{"cinematic_warm", "cinematic_cool", "moody_dark", "bright_clean", "vintage_film", "high_contrast", "neutral", "custom"}, "default": "cinematic_warm"},
		"intensity": map[string]any{"type": "number", "minimum": 0, "maximum": 1, "default": 0.8}, "lut_path": map[string]any{"type": "string"}, "custom_vf": map[string]any{"type": "string"},
		"temperature": map[string]any{"type": "number"}, "contrast": map[string]any{"type": "number"}, "saturation": map[string]any{"type": "number"}, "brightness": map[string]any{"type": "number"}, "gamma": map[string]any{"type": "number"},
		"overwrite": map[string]any{"type": "boolean", "default": false}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "default": 300},
	}},
	"image_selector": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
		"prompt": map[string]any{"type": "string"}, "aspect_ratio": map[string]any{"type": "string"}, "style": map[string]any{"type": "string"}, "scene_type": map[string]any{"type": "string"},
		"budget_tier": map[string]any{"enum": []string{"free", "budget", "standard", "premium"}}, "max_cost": map[string]any{"type": "number", "minimum": 0},
		"preferred_provider": map[string]any{"type": "string"}, "allowed_providers": map[string]any{"type": "array", "items": stringSchema()},
		"generation_mode": map[string]any{"enum": []string{"create", "edit", "stock", "any"}}, "require_text_rendering": map[string]any{"type": "boolean"}, "require_vector": map[string]any{"type": "boolean"},
	}},
	"video_selector": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
		"prompt": map[string]any{"type": "string"}, "aspect_ratio": map[string]any{"type": "string"}, "duration": map[string]any{"type": "number", "minimum": 0.1},
		"style": map[string]any{"type": "string"}, "intent": map[string]any{"type": "string"}, "budget_tier": map[string]any{"enum": []string{"free", "budget", "standard", "premium"}},
		"max_cost": map[string]any{"type": "number", "minimum": 0}, "preferred_provider": map[string]any{"type": "string"}, "allowed_providers": map[string]any{"type": "array", "items": stringSchema()},
		"source_type": map[string]any{"enum": []string{"text_to_video", "image_to_video", "stock", "any"}}, "require_audio": map[string]any{"type": "boolean"},
	}},
}

var resultSchemas = map[string]any{
	"media_probe": objectSchema([]string{"input", "sha256", "format", "video_streams", "audio_streams", "warnings"}, map[string]any{
		"input": stringSchema(), "sha256": map[string]any{"type": "string", "pattern": "^[0-9a-f]{64}$"}, "format": map[string]any{"type": "object", "required": []string{"duration", "format_name", "size", "bit_rate"}}, "video": map[string]any{"type": "object"}, "video_streams": map[string]any{"type": "array", "items": map[string]any{"type": "object", "required": []string{"index", "codec", "width", "height", "pixel_format", "fps", "reported_frame_rate", "rotation"}}}, "audio": map[string]any{"type": "array"}, "audio_streams": map[string]any{"type": "array", "items": map[string]any{"type": "object", "required": []string{"index", "codec", "sample_rate", "channels", "channel_layout"}}}, "warnings": stringArraySchema(),
	}),
	"audio_probe": objectSchema([]string{"file", "duration_seconds", "format_name", "format_long_name", "size_bytes", "bit_rate", "stream_count"}, map[string]any{
		"file": stringSchema(), "duration_seconds": map[string]any{"type": "number"}, "format_name": stringSchema(), "format_long_name": stringSchema(), "size_bytes": map[string]any{"type": "integer"}, "bit_rate": map[string]any{"type": "integer"}, "stream_count": map[string]any{"type": "integer"}, "audio": map[string]any{"type": "object"},
	}),
	"frame_sample": objectSchema([]string{"input", "strategy", "resolved_timestamps", "samples"}, map[string]any{
		"input": stringSchema(), "strategy": map[string]any{"enum": []string{"timestamps", "uniform", "scenes"}}, "resolved_timestamps": numberArraySchema(), "samples": map[string]any{"type": "array", "items": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"path", "timestamp", "width", "height"}, "properties": map[string]any{"path": stringSchema(), "timestamp": map[string]any{"type": "number"}, "width": map[string]any{"type": "integer"}, "height": map[string]any{"type": "integer"}}}},
	}),
	"frame_sampler": objectSchema([]string{"strategy", "frame_count", "frames", "output_dir"}, map[string]any{
		"strategy": stringSchema(), "frame_count": map[string]any{"type": "integer"}, "frames": map[string]any{"type": "array"}, "output_dir": stringSchema(),
	}),
	"scene_detect": objectSchema([]string{"scene_count", "scenes", "method"}, map[string]any{
		"scene_count": map[string]any{"type": "integer"}, "scenes": map[string]any{"type": "array"}, "method": stringSchema(), "output": stringSchema(),
	}),
	"visual_qa": objectSchema([]string{"operation", "input"}, map[string]any{
		"operation": stringSchema(), "input": stringSchema(), "duration": map[string]any{"type": "number"}, "frames": map[string]any{"type": "array"},
	}),
	"output_review": objectSchema([]string{"execution_status", "review_status", "gates", "samples", "volume", "output_facts"}, map[string]any{
		"execution_status": map[string]any{"const": "succeeded"}, "review_status": map[string]any{"enum": []string{"pass", "warn", "fail"}}, "gates": map[string]any{"type": "array", "items": map[string]any{"type": "object", "required": []string{"name", "status"}}}, "samples": map[string]any{"type": "array"}, "volume": map[string]any{"type": "object"}, "output_facts": map[string]any{"type": "object"},
	}),
	"source_edit":   mediaOutputResultSchema([]string{"realized_segments", "silent_inputs_filled"}),
	"video_trimmer": objectSchema([]string{"operation", "output"}, map[string]any{"operation": stringSchema(), "output": stringSchema()}),
	"video_stitch":  objectSchema([]string{"operation"}, map[string]any{"operation": stringSchema(), "output": stringSchema()}),
	"video_compose": objectSchema([]string{"operation"}, map[string]any{"operation": stringSchema(), "composition_id": stringSchema(), "output": stringSchema(), "output_facts": map[string]any{"type": "object"}}),
	"subtitle_gen": objectSchema([]string{"format", "cue_count", "output"}, map[string]any{
		"format": stringSchema(), "cue_count": map[string]any{"type": "integer"}, "output": stringSchema(),
	}),
	"remotion_caption_burn": objectSchema([]string{"method", "output"}, map[string]any{"method": stringSchema(), "output": stringSchema()}),
	"silence_cutter":        objectSchema([]string{"mode"}, map[string]any{"mode": stringSchema(), "output": stringSchema()}),
	"hyperframes_compose":   objectSchema([]string{"operation"}, map[string]any{"operation": stringSchema()}),
	"audio_mix":             mediaOutputResultSchema([]string{"loudnorm"}),
	"audio_mixer":           mediaOutputResultSchema([]string{"loudnorm"}),
	"music_library": objectSchema([]string{"library_dir", "exists", "track_count", "tracks"}, map[string]any{
		"library_dir": stringSchema(), "exists": map[string]any{"type": "boolean"}, "track_count": map[string]any{"type": "integer"}, "total_duration_seconds": map[string]any{"type": "number"}, "tracks": map[string]any{"type": "array"},
	}),
	"direct_clip_search": objectSchema([]string{"output_dir", "clips_downloaded", "total_clips", "clips"}, map[string]any{
		"output_dir": stringSchema(), "clips_downloaded": map[string]any{"type": "integer"}, "total_clips": map[string]any{"type": "integer"}, "clips": map[string]any{"type": "array"},
	}),
	"pexels_video":   objectSchema([]string{"provider", "video_id", "query", "output"}, map[string]any{"provider": stringSchema(), "video_id": map[string]any{"type": "integer"}, "query": stringSchema(), "output": stringSchema()}),
	"pixabay_video":  objectSchema([]string{"provider", "video_id", "query", "output"}, map[string]any{"provider": stringSchema(), "video_id": map[string]any{"type": "integer"}, "query": stringSchema(), "output": stringSchema()}),
	"wikimedia":      objectSchema([]string{"provider", "source_id", "query", "output"}, map[string]any{"provider": stringSchema(), "source_id": stringSchema(), "query": stringSchema(), "output": stringSchema()}),
	"edge_tts":       objectSchema([]string{"output", "voice", "format", "provider"}, map[string]any{"output": stringSchema(), "voice": stringSchema(), "format": stringSchema(), "provider": stringSchema(), "size_bytes": map[string]any{"type": "integer"}, "duration_seconds": map[string]any{"type": "number"}}),
	"openai_tts":     objectSchema([]string{"provider", "model", "voice", "output"}, map[string]any{"provider": stringSchema(), "model": stringSchema(), "voice": stringSchema(), "output": stringSchema()}),
	"elevenlabs_tts": objectSchema([]string{"provider", "model", "voice_id", "output"}, map[string]any{"provider": stringSchema(), "model": stringSchema(), "voice_id": stringSchema(), "output": stringSchema()}),
	"piper_tts":      objectSchema([]string{"provider", "model", "output"}, map[string]any{"provider": stringSchema(), "model": stringSchema(), "output": stringSchema()}),
	"openai_image":   objectSchema([]string{"provider", "model", "prompt", "output"}, map[string]any{"provider": stringSchema(), "model": stringSchema(), "prompt": stringSchema(), "size": stringSchema(), "quality": stringSchema(), "output": stringSchema(), "mock": map[string]any{"type": "boolean"}, "url": stringSchema()}),
	"flux_image":     objectSchema([]string{"provider", "model", "prompt", "output"}, map[string]any{"provider": stringSchema(), "model": stringSchema(), "prompt": stringSchema(), "aspect_ratio": stringSchema(), "output": stringSchema(), "mock": map[string]any{"type": "boolean"}, "url": stringSchema(), "seed": map[string]any{"type": "integer"}}),
	"kling_video":    objectSchema([]string{"provider", "model", "prompt", "output"}, map[string]any{"provider": stringSchema(), "model": stringSchema(), "prompt": stringSchema(), "duration": map[string]any{"type": "number"}, "aspect_ratio": stringSchema(), "mode": stringSchema(), "output": stringSchema(), "mock": map[string]any{"type": "boolean"}, "video_url": stringSchema()}),
	"sora_video":     objectSchema([]string{"provider", "model", "prompt", "output"}, map[string]any{"provider": stringSchema(), "model": stringSchema(), "prompt": stringSchema(), "duration": map[string]any{"type": "number"}, "aspect_ratio": stringSchema(), "resolution": stringSchema(), "output": stringSchema(), "mock": map[string]any{"type": "boolean"}, "video_url": stringSchema()}),
	"gflow_video":    gflowResultSchema(true),
	"gflow_image":    gflowResultSchema(false),
	"color_grade":    objectSchema([]string{"input", "output", "profile", "intensity", "filter_graph"}, map[string]any{"input": stringSchema(), "output": stringSchema(), "profile": stringSchema(), "intensity": map[string]any{"type": "number"}, "lut_path": stringSchema(), "filter_graph": stringSchema(), "duration": map[string]any{"type": "number"}, "output_facts": map[string]any{"type": "object"}}),
	"image_selector": objectSchema([]string{"selected_recommendation", "rationale", "candidates", "total_candidates", "configured_candidates"}, map[string]any{"selected_recommendation": stringSchema(), "rationale": stringSchema(), "candidates": map[string]any{"type": "array"}, "total_candidates": map[string]any{"type": "integer"}, "configured_candidates": map[string]any{"type": "integer"}, "requested_aspect_ratio": stringSchema(), "requested_style": stringSchema()}),
	"video_selector": objectSchema([]string{"selected_recommendation", "rationale", "candidates", "total_candidates", "configured_candidates"}, map[string]any{"selected_recommendation": stringSchema(), "rationale": stringSchema(), "candidates": map[string]any{"type": "array"}, "total_candidates": map[string]any{"type": "integer"}, "configured_candidates": map[string]any{"type": "integer"}, "requested_duration": map[string]any{"type": "number"}, "requested_aspect_ratio": stringSchema()}),
}

func gflowResultSchema(video bool) map[string]any {
	properties := map[string]any{"provider": stringSchema(), "model": stringSchema(), "prompt": stringSchema(), "aspect_ratio": stringSchema(), "output": stringSchema(), "mock": map[string]any{"type": "boolean"},
		"outputs": map[string]any{"type": "array", "minItems": 1, "items": objectSchema([]string{"id", "type", "mime_type", "output", "source_file", "sha256"}, map[string]any{
			"id": stringSchema(), "type": map[string]any{"enum": []string{"image", "video"}}, "mime_type": stringSchema(), "output": stringSchema(), "source_file": stringSchema(), "sha256": map[string]any{"type": "string", "pattern": "^[0-9a-f]{64}$"},
		})}}
	if video {
		properties["duration"], properties["resolution"] = map[string]any{"type": "number"}, stringSchema()
	}
	schema := objectSchema([]string{"provider", "model", "prompt", "output", "mock"}, properties)
	schema["if"] = map[string]any{"properties": map[string]any{"mock": map[string]any{"const": false}}}
	schema["then"] = map[string]any{"required": []string{"outputs"}}
	return schema
}

func objectSchema(required []string, properties map[string]any) map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": properties}
}
func stringSchema() map[string]any { return map[string]any{"type": "string"} }
func stringArraySchema() map[string]any {
	return map[string]any{"type": "array", "items": stringSchema()}
}
func numberArraySchema() map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "number"}}
}
func mediaOutputResultSchema(extra []string) map[string]any {
	required := []string{"output", "duration", "requested_operations", "realized_operations", "output_facts"}
	required = append(required, extra...)
	properties := map[string]any{"output": stringSchema(), "duration": map[string]any{"type": "number"}, "requested_operations": stringArraySchema(), "realized_operations": stringArraySchema(), "output_facts": map[string]any{"type": "object"}}
	for _, name := range extra {
		if name == "loudnorm" {
			properties[name] = map[string]any{"type": "object"}
		} else {
			properties[name] = map[string]any{"type": "integer"}
		}
	}
	return objectSchema(required, properties)
}

func audioOperationSchema(music bool) map[string]any {
	properties := map[string]any{"gain_db": map[string]any{"type": "number"}, "fade_in": map[string]any{"type": "number", "minimum": 0}, "fade_out": map[string]any{"type": "number", "minimum": 0}}
	if music {
		properties["input"] = map[string]any{"type": "string", "minLength": 1}
		properties["ducking"] = map[string]any{"type": "object", "additionalProperties": false, "required": []string{"enabled"}, "properties": map[string]any{"enabled": map[string]any{"type": "boolean"}, "threshold": map[string]any{"type": "number", "exclusiveMinimum": 0, "maximum": 1}, "ratio": map[string]any{"type": "number", "exclusiveMinimum": 1, "maximum": 20}}}
	}
	return map[string]any{"type": "object", "additionalProperties": false, "properties": properties}
}

func execute(tool, op string, data []byte) (any, []string, error) {
	switch tool {
	case "media_probe":
		return doMediaProbe(op, data)
	case "audio_probe":
		return doAudioProbe(op, data)
	case "frame_sample":
		return doFrameSample(op, data)
	case "frame_sampler":
		return doFrameSampler(op, data)
	case "scene_detect":
		return doSceneDetect(op, data)
	case "visual_qa":
		return doVisualQA(op, data)
	case "output_review":
		return doOutputReview(op, data)
	case "source_edit":
		return doSourceEdit(op, data)
	case "video_trimmer":
		return doVideoTrimmer(op, data)
	case "video_stitch":
		return doVideoStitch(op, data)
	case "video_compose":
		return doVideoCompose(op, data)
	case "subtitle_gen":
		return doSubtitleGen(op, data)
	case "remotion_caption_burn":
		return doRemotionCaptionBurn(op, data)
	case "silence_cutter":
		return doSilenceCutter(op, data)
	case "hyperframes_compose":
		return doHyperFramesCompose(op, data)
	case "audio_mix", "audio_mixer":
		return doAudioMix(op, data)
	case "music_library":
		return doMusicLibrary(op, data)
	case "direct_clip_search":
		return doDirectClipSearch(op, data)
	case "pexels_video":
		return doPexelsVideo(op, data)
	case "pixabay_video":
		return doPixabayVideo(op, data)
	case "wikimedia":
		return doWikimedia(op, data)
	case "edge_tts":
		return doEdgeTTS(op, data)
	case "openai_tts":
		return doOpenAITTS(op, data)
	case "elevenlabs_tts":
		return doElevenLabsTTS(op, data)
	case "piper_tts":
		return doPiperTTS(op, data)
	case "openai_image":
		return doOpenAIImage(op, data)
	case "flux_image":
		return doFluxImage(op, data)
	case "kling_video":
		return doKlingVideo(op, data)
	case "sora_video":
		return doSoraVideo(op, data)
	case "gflow_video":
		return doGFlowVideo(op, data)
	case "gflow_image":
		return doGFlowImage(op, data)
	case "color_grade":
		return doColorGrade(op, data)
	case "image_selector":
		return doImageSelector(op, data)
	case "video_selector":
		return doVideoSelector(op, data)
	}
	return nil, nil, failure("invalid_request", "unknown tool: "+tool, nil)
}

func decode(data []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return failure("invalid_request", decodeMessage(err, dst),
			map[string]any{"error": bounded(err.Error())})
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return failure("invalid_request", "request must contain one JSON object", nil)
	}
	return nil
}

// decodeMessage says which of three different problems occurred.
//
// "invalid request JSON" covered all of them, and for two it was actively
// wrong: a request with an unknown field is perfectly valid JSON that this tool
// does not accept, and a type mismatch is valid JSON with a wrong value. An
// agent told its JSON is invalid re-serializes correct JSON, fails again, and
// concludes the module is broken — which was observed: one abandoned Facet
// after this error and fell back to raw ffmpeg, producing a blank video it
// reported as a success.
// It also NAMES the fields the tool does accept. Telling a caller to go run
// `tools describe` costs a round trip and assumes it can; the accepted names
// are already in the destination struct, and one of them is usually the one
// meant. This matters because the input field is not spelled the same across
// tools — media_probe takes `input`, video_trimmer takes `input_path`,
// audio_mix takes `source` — so a caller that learned one tool guesses wrong
// on the next. Observed: two consecutive wrong guesses on a tool I own.
func decodeMessage(err error, dst any) string {
	text := err.Error()
	switch {
	case strings.Contains(text, "unknown field"):
		field := text
		if i := strings.Index(text, "unknown field "); i >= 0 {
			field = strings.TrimSpace(text[i+len("unknown field "):])
		}
		msg := "this tool does not accept the field " + field
		if accepted := acceptedFields(dst); accepted != "" {
			msg += "; it accepts " + accepted
		}
		// The field list is the likely fix; describe stays as the authority
		// on types, enums and which combinations are valid.
		return msg + " (see `facet tools describe <tool>` for the full schema)"
	case strings.Contains(text, "cannot unmarshal"):
		return "a field has the wrong type: " + bounded(text)
	}
	return "request body is not valid JSON"
}

// acceptedFields lists the JSON names a request struct accepts, so a rejection
// carries the answer rather than only the complaint.
func acceptedFields(dst any) string {
	t := reflect.TypeOf(dst)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return ""
	}
	var names []string
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		if name, _, _ := strings.Cut(tag, ","); name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	return strings.Join(names, ", ")
}

func positiveTimeout(v int, fallback int) (time.Duration, error) {
	if v < 0 {
		return 0, failure("invalid_request", "timeout_seconds must be positive", nil)
	}
	if v == 0 {
		v = fallback
	}
	return time.Duration(v) * time.Second, nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func inputPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return failure("invalid_request", "input path is required", nil)
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return failure("input_not_found", "input does not exist", map[string]any{"path": path})
	}
	if err != nil {
		return failure("invalid_request", "input cannot be accessed", map[string]any{"path": path, "error": bounded(err.Error())})
	}
	if info.IsDir() {
		return failure("invalid_request", "input must be a file", map[string]any{"path": path})
	}
	return nil
}

func outputPath(path string, overwrite bool, estimate bool) error {
	if strings.TrimSpace(path) == "" {
		return failure("invalid_request", "output path is required", nil)
	}
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return failure("output_conflict", "output already exists", map[string]any{"path": path})
		}
	}
	if !estimate {
		parent := filepath.Dir(path)
		if parent == "." {
			return nil
		}
		if err := os.MkdirAll(parent, 0755); err != nil {
			return failure("command_failed", "output directory could not be created", map[string]any{"error": bounded(err.Error())})
		}
	}
	return nil
}

func samePath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	return errA == nil && errB == nil && strings.EqualFold(filepath.Clean(aa), filepath.Clean(bb))
}

func temporaryOutput(path string) (string, func(), error) {
	ext := filepath.Ext(path)
	file, err := os.CreateTemp(filepath.Dir(path), ".videokit-*"+ext)
	if err != nil {
		return "", nil, failure("command_failed", "temporary output could not be created", map[string]any{"error": bounded(err.Error())})
	}
	temp := file.Name()
	if err := file.Close(); err != nil {
		os.Remove(temp)
		return "", nil, failure("command_failed", "temporary output could not be closed", map[string]any{"error": bounded(err.Error())})
	}
	if err := os.Remove(temp); err != nil {
		return "", nil, failure("command_failed", "temporary output could not be prepared", map[string]any{"error": bounded(err.Error())})
	}
	return temp, func() { _ = os.Remove(temp) }, nil
}

func finalizeOutput(temp, output string, overwrite bool) error {
	if !overwrite {
		if err := os.Rename(temp, output); err != nil {
			return failure("command_failed", "temporary output could not be finalized", map[string]any{"error": bounded(err.Error())})
		}
		return nil
	}
	backupFile, err := os.CreateTemp(filepath.Dir(output), ".videokit-backup-*")
	if err != nil {
		return failure("command_failed", "replacement backup could not be prepared", map[string]any{"error": bounded(err.Error())})
	}
	backup := backupFile.Name()
	if err = backupFile.Close(); err != nil {
		_ = os.Remove(backup)
		return failure("command_failed", "replacement backup could not be closed", map[string]any{"error": bounded(err.Error())})
	}
	if err = os.Remove(backup); err != nil {
		return failure("command_failed", "replacement backup could not be reserved", map[string]any{"error": bounded(err.Error())})
	}
	hadOutput := false
	if _, err = os.Stat(output); err == nil {
		hadOutput = true
		if err = os.Rename(output, backup); err != nil {
			return failure("command_failed", "existing output could not be preserved", map[string]any{"error": bounded(err.Error())})
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return failure("command_failed", "existing output could not be inspected", map[string]any{"error": bounded(err.Error())})
	}
	if err := os.Rename(temp, output); err != nil {
		if hadOutput {
			if rollbackErr := os.Rename(backup, output); rollbackErr != nil {
				return failure("command_failed", "temporary output could not be finalized and preserved output rollback failed", map[string]any{"error": bounded(err.Error()), "rollback_error": bounded(rollbackErr.Error()), "backup": backup})
			}
		}
		return failure("command_failed", "temporary output could not be finalized; preserved output was restored", map[string]any{"error": bounded(err.Error())})
	}
	if hadOutput {
		_ = os.Remove(backup)
	}
	return nil
}

func publishFileSet(staged, outputs []string, overwrite bool) error {
	backups := make([]string, len(outputs))
	published := 0
	rollback := func() {
		for i := 0; i < published; i++ {
			_ = os.Remove(outputs[i])
		}
		for i, backup := range backups {
			if backup != "" {
				_ = os.Rename(backup, outputs[i])
			}
		}
	}
	if overwrite {
		for i, output := range outputs {
			if _, err := os.Stat(output); err == nil {
				backupFile, createErr := os.CreateTemp(filepath.Dir(output), ".videokit-frame-backup-*")
				if createErr != nil {
					rollback()
					return failure("command_failed", "frame backup could not be prepared", map[string]any{"path": output, "error": bounded(createErr.Error())})
				}
				backup := backupFile.Name()
				closeErr := backupFile.Close()
				removeErr := os.Remove(backup)
				if closeErr != nil || removeErr != nil {
					rollback()
					return failure("command_failed", "frame backup could not be reserved", map[string]any{"path": output})
				}
				if err = os.Rename(output, backup); err != nil {
					rollback()
					return failure("command_failed", "existing frame set could not be preserved", map[string]any{"path": output, "error": bounded(err.Error())})
				}
				backups[i] = backup
			} else if !errors.Is(err, os.ErrNotExist) {
				rollback()
				return failure("command_failed", "existing frame could not be inspected", map[string]any{"path": output, "error": bounded(err.Error())})
			}
		}
	}
	for i := range staged {
		if err := os.Rename(staged[i], outputs[i]); err != nil {
			rollback()
			return failure("command_failed", "frame set could not be published", map[string]any{"path": outputs[i], "error": bounded(err.Error())})
		}
		published++
	}
	for _, backup := range backups {
		if backup != "" {
			_ = os.Remove(backup)
		}
	}
	return nil
}

func bounded(s string) string {
	if len(s) <= maxDiagnostic {
		return s
	}
	return s[len(s)-maxDiagnostic:]
}

func runCommand(timeout time.Duration, program string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return runCommandContext(ctx, program, args...)
}

// timeoutMessage says what ran out of time and what to do about it.
//
// "node was cancelled or timed out" names the binary rather than the work and
// suggests nothing. An agent reading it cannot tell whether the render was
// impossible or merely given four seconds too few — and the honest answer is
// usually the latter, because the module clamps a tool's timeout to the host's
// remaining budget.
func timeoutMessage(program string, budget time.Duration) string {
	if budget > 0 {
		return fmt.Sprintf(
			"%s did not finish within %s; raise timeout_seconds, or the host's "+
				"deadline_ms if that is the smaller budget", program, budget)
	}
	return program + " did not finish in the time allowed; raise timeout_seconds, " +
		"or the host's deadline_ms if that is the smaller budget"
}

func runCommandContext(ctx context.Context, program string, args ...string) ([]byte, error) {
	resolved, err := lookPath(program)
	if err != nil {
		return nil, failure("dependency_missing", program+" is not available", nil)
	}
	// Recorded before the run so the message can name the budget that expired.
	var budget time.Duration
	if deadline, ok := ctx.Deadline(); ok {
		budget = time.Until(deadline)
	}
	cmd := exec.CommandContext(ctx, resolved, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if ctx.Err() != nil {
		return nil, failure("command_timeout", timeoutMessage(program, budget.Round(time.Second)), map[string]any{"stderr": bounded(stderr.String())})
	}
	if err != nil {
		return nil, failure("command_failed", program+" failed", map[string]any{"stderr": bounded(stderr.String()), "error": bounded(err.Error())})
	}
	return append(stdout.Bytes(), stderr.Bytes()...), nil
}

func runCommandDir(timeout time.Duration, dir, program string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	resolved, err := lookPath(program)
	if err != nil {
		return nil, failure("dependency_missing", program+" is not available", nil)
	}
	cmd := exec.CommandContext(ctx, resolved, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if ctx.Err() != nil {
		return []byte(bounded(stdout.String())), failure("command_timeout", timeoutMessage(program, timeout), map[string]any{"stderr": bounded(stderr.String())})
	}
	if err != nil {
		return nil, failure("command_failed", program+" failed", map[string]any{"stderr": bounded(stderr.String()), "error": bounded(err.Error()), "output": bounded(stdout.String())})
	}
	return append(stdout.Bytes(), stderr.Bytes()...), nil
}

func parseFloat(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }

func estimateResult(ops []string) map[string]any {
	return map[string]any{"estimated_cost": 0.0, "network": false, "external_write": false, "side_effect_free": true, "operations": ops}
}

func formatFloat(v float64) string { return strconv.FormatFloat(v, 'f', 6, 64) }

func parseLoudnorm(s string) map[string]any {
	start := strings.LastIndex(s, "{\n")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		var m map[string]any
		if json.Unmarshal([]byte(s[start:end+1]), &m) == nil {
			return m
		}
	}
	return map[string]any{}
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

// Names returns a defensive copy of the exact public tool catalog.
func Names() []string { out := append([]string(nil), names...); sort.Strings(out); return out }
