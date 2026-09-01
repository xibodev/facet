package toolbox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type composeCut struct {
	ID         string  `json:"id,omitempty"`
	Source     string  `json:"source"`
	InSeconds  float64 `json:"in_seconds"`
	OutSeconds float64 `json:"out_seconds"`
	Speed      float64 `json:"speed,omitempty"`
	Type       string  `json:"type,omitempty"`
	Text       string  `json:"text,omitempty"`
	Title      string  `json:"title,omitempty"`
	Subtitle   string  `json:"subtitle,omitempty"`
}

type composeEditDecisions struct {
	RendererFamily string       `json:"renderer_family,omitempty"`
	RenderRuntime  string       `json:"render_runtime,omitempty"`
	Cuts           []composeCut `json:"cuts,omitempty"`
	Subtitles      struct {
		Enabled bool   `json:"enabled,omitempty"`
		Source  string `json:"source,omitempty"`
	} `json:"subtitles,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Bespoke  map[string]any `json:"bespoke,omitempty"`
}

type composeOverlay struct {
	AssetPath    string  `json:"asset_path"`
	X            float64 `json:"x,omitempty"`
	Y            float64 `json:"y,omitempty"`
	Width        float64 `json:"width,omitempty"`
	Height       float64 `json:"height,omitempty"`
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
	Opacity      float64 `json:"opacity,omitempty"`
}

type composeRequest struct {
	Operation         string                `json:"operation"`
	InputPath         string                `json:"input_path,omitempty"`
	OutputPath        string                `json:"output_path,omitempty"`
	EditDecisions     *composeEditDecisions `json:"edit_decisions,omitempty"`
	AssetManifest     map[string]any        `json:"asset_manifest,omitempty"`
	AudioPath         string                `json:"audio_path,omitempty"`
	SubtitlePath      string                `json:"subtitle_path,omitempty"`
	SubtitleStyle     map[string]any        `json:"subtitle_style,omitempty"`
	Overlays          []composeOverlay      `json:"overlays,omitempty"`
	Codec             string                `json:"codec,omitempty"`
	CRF               int                   `json:"crf,omitempty"`
	Preset            string                `json:"preset,omitempty"`
	Profile           string                `json:"profile,omitempty"`
	RemotionTimeoutMS int                   `json:"remotion_timeout_ms,omitempty"`
	TimeoutSeconds    int                   `json:"timeout_seconds,omitempty"`
}

func doVideoCompose(op string, data []byte) (any, []string, error) {
	var r composeRequest
	if err := decode(data, &r); err != nil {
		return nil, nil, err
	}
	operation := r.Operation
	if operation == "" {
		operation = "compose"
	}
	tmo, err := positiveTimeout(r.TimeoutSeconds, 600)
	if err != nil {
		return nil, nil, err
	}

	if op == "estimate" {
		return estimateResult([]string{"video_compose_" + operation}), nil, nil
	}

	switch operation {
	case "compose", "render":
		if r.EditDecisions == nil || len(r.EditDecisions.Cuts) == 0 {
			return nil, nil, failure("invalid_request", "edit_decisions with cuts required", nil)
		}
		outPath := r.OutputPath
		if outPath == "" {
			outPath = "composed_output.mp4"
		}
		if err := outputPath(outPath, true, false); err != nil {
			return nil, nil, err
		}

		runtime := strings.ToLower(r.EditDecisions.RenderRuntime)
		if runtime == "remotion" {
			return doRemotionRender(r, outPath, tmo)
		} else if runtime == "hyperframes" {
			// Delegate to hyperframes
			hfReq := map[string]any{
				"operation":      "render",
				"output_path":    outPath,
				"edit_decisions": r.EditDecisions,
				"asset_manifest": r.AssetManifest,
			}
			hfData, _ := json.Marshal(hfReq)
			return doHyperFramesCompose(op, hfData)
		}

		// FFmpeg compose implementation
		return doFFmpegCompose(r, outPath, tmo)

	case "remotion_render":
		outPath := r.OutputPath
		if outPath == "" {
			outPath = "remotion_output.mp4"
		}
		if err := outputPath(outPath, true, false); err != nil {
			return nil, nil, err
		}
		return doRemotionRender(r, outPath, tmo)

	case "burn_subtitles":
		if err := inputPath(r.InputPath); err != nil {
			return nil, nil, err
		}
		if err := inputPath(r.SubtitlePath); err != nil {
			return nil, nil, err
		}
		outPath := r.OutputPath
		if outPath == "" {
			outPath = "subtitled_output.mp4"
		}
		if err := outputPath(outPath, true, false); err != nil {
			return nil, nil, err
		}
		assStyle := buildSubtitleStyle(r.SubtitleStyle)
		subEscaped := strings.ReplaceAll(filepath.ToSlash(r.SubtitlePath), ":", `\:`)
		vf := fmt.Sprintf("subtitles='%s':force_style='%s'", subEscaped, assStyle)
		args := []string{"-hide_banner", "-loglevel", "error", "-y", "-i", r.InputPath, "-vf", vf, "-c:v", "libx264", "-c:a", "copy", outPath}
		if _, err := runCommand(tmo, "ffmpeg", args...); err != nil {
			return nil, nil, err
		}
		return map[string]any{
			"operation": "burn_subtitles",
			"input":     r.InputPath,
			"subtitles": r.SubtitlePath,
			"output":    outPath,
		}, nil, nil

	case "overlay":
		if err := inputPath(r.InputPath); err != nil {
			return nil, nil, err
		}
		outPath := r.OutputPath
		if outPath == "" {
			outPath = "overlay_output.mp4"
		}
		if err := outputPath(outPath, true, false); err != nil {
			return nil, nil, err
		}
		if len(r.Overlays) == 0 {
			return nil, nil, failure("invalid_request", "overlays array is required", nil)
		}
		cmdArgs := []string{"-hide_banner", "-loglevel", "error", "-y", "-i", r.InputPath}
		filters := []string{}
		for i, ov := range r.Overlays {
			if err := inputPath(ov.AssetPath); err != nil {
				return nil, nil, err
			}
			cmdArgs = append(cmdArgs, "-i", ov.AssetPath)
			vIn := "[0:v]"
			if i > 0 {
				vIn = fmt.Sprintf("[ov%d]", i-1)
			}
			vOut := fmt.Sprintf("[ov%d]", i)
			if i == len(r.Overlays)-1 {
				vOut = "[outv]"
			}
			filters = append(filters, fmt.Sprintf("%s[%d:v]overlay=x=%s:y=%s:enable='between(t,%s,%s)'%s", vIn, i+1, formatFloat(ov.X), formatFloat(ov.Y), formatFloat(ov.StartSeconds), formatFloat(ov.EndSeconds), vOut))
		}
		cmdArgs = append(cmdArgs, "-filter_complex", strings.Join(filters, ";"), "-map", "[outv]", "-map", "0:a?", "-c:v", "libx264", "-c:a", "copy", outPath)
		if _, err := runCommand(tmo, "ffmpeg", cmdArgs...); err != nil {
			return nil, nil, err
		}
		return map[string]any{
			"operation":     "overlay",
			"input":         r.InputPath,
			"overlay_count": len(r.Overlays),
			"output":        outPath,
		}, nil, nil

	case "encode":
		if err := inputPath(r.InputPath); err != nil {
			return nil, nil, err
		}
		outPath := r.OutputPath
		if outPath == "" {
			outPath = "encoded_output.mp4"
		}
		if err := outputPath(outPath, true, false); err != nil {
			return nil, nil, err
		}
		codec := r.Codec
		if codec == "" {
			codec = "libx264"
		}
		crf := r.CRF
		if crf == 0 {
			crf = 23
		}
		preset := r.Preset
		if preset == "" {
			preset = "medium"
		}
		args := []string{"-hide_banner", "-loglevel", "error", "-y", "-i", r.InputPath, "-c:v", codec, "-crf", strconv.Itoa(crf), "-preset", preset, "-c:a", "aac", outPath}
		if _, err := runCommand(tmo, "ffmpeg", args...); err != nil {
			return nil, nil, err
		}
		return map[string]any{
			"operation": "encode",
			"input":     r.InputPath,
			"output":    outPath,
			"codec":     codec,
		}, nil, nil

	default:
		return nil, nil, failure("invalid_request", "unknown operation: "+operation, nil)
	}
}

func doFFmpegCompose(r composeRequest, outPath string, tmo time.Duration) (any, []string, error) {
	tempDir, err := os.MkdirTemp(filepath.Dir(outPath), ".compose_tmp-*")
	if err != nil {
		return nil, nil, failure("command_failed", "unable to create temporary directory", nil)
	}
	defer os.RemoveAll(tempDir)

	cuts := r.EditDecisions.Cuts
	tempSegments := make([]string, len(cuts))
	targetW, targetH := 1920, 1080
	targetFPS := 30.0

	for i, cut := range cuts {
		src := cut.Source
		if err := inputPath(src); err != nil {
			return nil, nil, err
		}
		inS := cut.InSeconds
		outS := cut.OutSeconds
		dur := outS - inS
		if dur <= 0 {
			dur = 1.0
		}
		segFile := filepath.Join(tempDir, fmt.Sprintf("seg_%04d.mp4", i))
		vf := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=%s,format=yuv420p", targetW, targetH, targetW, targetH, formatFloat(targetFPS))

		p, _, err := probe(src, tmo)
		if err != nil {
			return nil, nil, err
		}
		var args []string
		if hasAudio(p) {
			args = []string{"-hide_banner", "-loglevel", "error", "-y", "-ss", formatFloat(inS), "-t", formatFloat(dur), "-i", src, "-vf", vf, "-c:v", "libx264", "-crf", "23", "-preset", "medium", "-c:a", "aac", "-ar", "48000", "-ac", "2", segFile}
		} else {
			args = []string{"-hide_banner", "-loglevel", "error", "-y", "-ss", formatFloat(inS), "-t", formatFloat(dur), "-i", src, "-f", "lavfi", "-t", formatFloat(dur), "-i", "anullsrc=r=48000:cl=stereo", "-vf", vf, "-map", "0:v:0", "-map", "1:a:0", "-c:v", "libx264", "-crf", "23", "-preset", "medium", "-c:a", "aac", "-ar", "48000", "-ac", "2", segFile}
		}
		if _, err := runCommand(tmo, "ffmpeg", args...); err != nil {
			return nil, nil, err
		}
		tempSegments[i] = segFile
	}

	concatList := filepath.Join(tempDir, "concat.txt")
	b := strings.Builder{}
	for _, ts := range tempSegments {
		abs, _ := filepath.Abs(ts)
		fmt.Fprintf(&b, "file '%s'\n", strings.ReplaceAll(abs, `\`, `/`))
	}
	if err := os.WriteFile(concatList, []byte(b.String()), 0644); err != nil {
		return nil, nil, failure("command_failed", "unable to write concat list", nil)
	}

	concatOut := filepath.Join(tempDir, "concat.mp4")
	if _, err := runCommand(tmo, "ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "concat", "-safe", "0", "-i", concatList, "-c", "copy", concatOut); err != nil {
		return nil, nil, err
	}

	finalIn := concatOut
	cmdArgs := []string{"-hide_banner", "-loglevel", "error", "-y", "-i", finalIn}
	subPath := r.SubtitlePath
	if subPath == "" && r.EditDecisions.Subtitles.Enabled && r.EditDecisions.Subtitles.Source != "" {
		subPath = r.EditDecisions.Subtitles.Source
	}

	hasSubs := subPath != "" && fileExists(subPath)
	if hasSubs {
		assStyle := buildSubtitleStyle(r.SubtitleStyle)
		subEscaped := strings.ReplaceAll(filepath.ToSlash(subPath), ":", `\:`)
		cmdArgs = append(cmdArgs, "-vf", fmt.Sprintf("subtitles='%s':force_style='%s'", subEscaped, assStyle))
	}

	if r.AudioPath != "" && fileExists(r.AudioPath) {
		cmdArgs = append(cmdArgs, "-i", r.AudioPath, "-map", "0:v:0", "-map", "1:a:0", "-c:v", "libx264", "-c:a", "aac", "-shortest", outPath)
	} else if hasSubs {
		cmdArgs = append(cmdArgs, "-c:v", "libx264", "-c:a", "copy", outPath)
	} else {
		cmdArgs = append(cmdArgs, "-c", "copy", outPath)
	}

	if _, err := runCommand(tmo, "ffmpeg", cmdArgs...); err != nil {
		return nil, nil, err
	}

	return map[string]any{
		"operation":       "compose",
		"cut_count":       len(cuts),
		"has_subtitles":   hasSubs,
		"has_mixed_audio": r.AudioPath != "",
		"output":          outPath,
	}, nil, nil
}

func doRemotionRender(r composeRequest, outPath string, tmo time.Duration) (any, []string, error) {
	composerDir := "remotion-composer"
	if _, err := os.Stat(filepath.Join(composerDir, "package.json")); err != nil {
		// check ../remotion-composer
		if _, err2 := os.Stat(filepath.Join("..", "remotion-composer", "package.json")); err2 == nil {
			composerDir = filepath.Join("..", "remotion-composer")
		}
	}

	compositionID := "Explainer"
	if r.EditDecisions != nil && r.EditDecisions.RendererFamily != "" {
		switch r.EditDecisions.RendererFamily {
		case "cinematic-trailer", "documentary-montage":
			compositionID = "CinematicRenderer"
		case "presenter":
			compositionID = "TalkingHead"
		default:
			compositionID = "Explainer"
		}
	}

	propsJSON, _ := json.Marshal(r.EditDecisions)
	propsPath := filepath.Join(filepath.Dir(outPath), ".remotion_props.json")
	if err := os.WriteFile(propsPath, propsJSON, 0644); err != nil {
		return nil, nil, failure("command_failed", "unable to write remotion props", nil)
	}
	defer os.Remove(propsPath)

	absOut, _ := filepath.Abs(outPath)
	absProps, _ := filepath.Abs(propsPath)
	entryFile := filepath.Join(composerDir, "src", "index.tsx")

	args := []string{"remotion", "render", entryFile, compositionID, absOut, "--props=" + absProps}
	if _, err := runCommand(tmo, "npx", args...); err != nil {
		return nil, nil, failure("command_failed", "remotion render failed: "+err.Error(), map[string]any{"composition_id": compositionID})
	}

	if r.AudioPath != "" && fileExists(r.AudioPath) {
		tempMux := filepath.Join(filepath.Dir(outPath), ".mux-"+filepath.Base(outPath))
		if _, err := runCommand(tmo, "ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", outPath, "-i", r.AudioPath, "-map", "0:v:0", "-map", "1:a:0", "-c:v", "copy", "-c:a", "aac", "-shortest", tempMux); err == nil {
			_ = os.Rename(tempMux, outPath)
		}
	}

	return map[string]any{
		"operation":      "remotion_render",
		"composition_id": compositionID,
		"output":         outPath,
	}, nil, nil
}

func buildSubtitleStyle(style map[string]any) string {
	if style == nil {
		style = map[string]any{}
	}
	font := "Segoe UI"
	if f, ok := style["font"].(string); ok && f != "" {
		font = f
	}
	fontSize := 24
	if fs, ok := style["font_size"].(float64); ok && fs > 0 {
		fontSize = int(fs)
	}
	marginV := 40
	if mv, ok := style["margin_v"].(float64); ok && mv > 0 {
		marginV = int(mv)
	}
	alignment := 2
	if al, ok := style["alignment"].(float64); ok && al > 0 {
		alignment = int(al)
	}
	return fmt.Sprintf("FontName=%s,FontSize=%d,Bold=1,PrimaryColour=&H00FFFFFF,OutlineColour=&H00000000,Outline=2,Shadow=1,Alignment=%d,MarginV=%d", font, fontSize, alignment, marginV)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
