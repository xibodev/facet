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
	Operation         string                `json:"operation,omitempty"`
	InputPath         string                `json:"input_path,omitempty"`
	OutputPath        string                `json:"output_path,omitempty"`
	Output            string                `json:"output,omitempty"`
	CompositionID     string                `json:"composition_id,omitempty"`
	Composition       string                `json:"composition,omitempty"`
	Theme             string                `json:"theme,omitempty"`
	Cuts              []map[string]any      `json:"cuts,omitempty"`
	Overlays          []composeOverlay      `json:"overlays,omitempty"`
	Captions          any                   `json:"captions,omitempty"`
	Audio             any                   `json:"audio,omitempty"`
	Scenes            []map[string]any      `json:"scenes,omitempty"`
	EditDecisions     *composeEditDecisions `json:"edit_decisions,omitempty"`
	AssetManifest     map[string]any        `json:"asset_manifest,omitempty"`
	AudioPath         string                `json:"audio_path,omitempty"`
	SubtitlePath      string                `json:"subtitle_path,omitempty"`
	SubtitleStyle     map[string]any        `json:"subtitle_style,omitempty"`
	Codec             string                `json:"codec,omitempty"`
	CRF               int                   `json:"crf,omitempty"`
	Preset            string                `json:"preset,omitempty"`
	Profile           string                `json:"profile,omitempty"`
	RemotionTimeoutMS int                   `json:"remotion_timeout_ms,omitempty"`
	TimeoutSeconds    int                   `json:"timeout_seconds,omitempty"`
	RawProps          map[string]any        `json:"-"`
}

func doVideoCompose(op string, data []byte) (any, []string, error) {
	// First check if payload is direct Remotion props or Scene Plan JSON
	var rawMap map[string]any
	if err := json.Unmarshal(data, &rawMap); err == nil && rawMap != nil {
		// 1. Direct Remotion Explainer props (contains top-level "cuts")
		if cutsRaw, hasCuts := rawMap["cuts"].([]any); hasCuts && len(cutsRaw) > 0 {
			outPath := "renders/final.mp4"
			if o, ok := rawMap["output"].(string); ok && strings.TrimSpace(o) != "" {
				outPath = strings.TrimSpace(o)
			} else if o, ok := rawMap["output_path"].(string); ok && strings.TrimSpace(o) != "" {
				outPath = strings.TrimSpace(o)
			}
			tmo := 600 * time.Second
			if t, ok := rawMap["timeout_seconds"].(float64); ok && t > 0 {
				tmo = time.Duration(t) * time.Second
			}
			if op == "estimate" {
				return estimateResult([]string{"video_compose_remotion_render"}), nil, nil
			}
			r := composeRequest{
				Operation:  "remotion_render",
				OutputPath: outPath,
				RawProps:   rawMap,
			}
			if comp, ok := rawMap["composition_id"].(string); ok && comp != "" {
				r.CompositionID = comp
			} else if comp, ok := rawMap["composition"].(string); ok && comp != "" {
				r.CompositionID = comp
			}
			if ap, ok := rawMap["audio_path"].(string); ok && ap != "" {
				r.AudioPath = ap
			}
			return doRemotionRender(r, outPath, tmo)
		}

		// 2. Direct Scene Plan JSON (contains top-level "scenes")
		if scenesRaw, hasScenes := rawMap["scenes"].([]any); hasScenes && len(scenesRaw) > 0 {
			outPath := "renders/final.mp4"
			if o, ok := rawMap["output"].(string); ok && strings.TrimSpace(o) != "" {
				outPath = strings.TrimSpace(o)
			} else if o, ok := rawMap["output_path"].(string); ok && strings.TrimSpace(o) != "" {
				outPath = strings.TrimSpace(o)
			}
			tmo := 600 * time.Second
			if t, ok := rawMap["timeout_seconds"].(float64); ok && t > 0 {
				tmo = time.Duration(t) * time.Second
			}
			if op == "estimate" {
				return estimateResult([]string{"video_compose_remotion_render"}), nil, nil
			}
			cuts := make([]map[string]any, 0, len(scenesRaw))
			for _, s := range scenesRaw {
				if sm, ok := s.(map[string]any); ok {
					cut := map[string]any{
						"id":         sm["id"],
						"type":       sm["type"],
						"in_seconds": sm["start_seconds"],
						"out_seconds": sm["end_seconds"],
					}
					if d, ok := sm["description"].(string); ok {
						cut["text"] = d
					}
					cuts = append(cuts, cut)
				}
			}
			theme := "flat-motion-graphics"
			if t, ok := rawMap["style_playbook"].(string); ok && t != "" {
				theme = t
			}
			remotionProps := map[string]any{
				"theme": theme,
				"cuts":  cuts,
			}
			if aud, ok := rawMap["audio"]; ok {
				remotionProps["audio"] = aud
			}
			if ov, ok := rawMap["overlays"]; ok {
				remotionProps["overlays"] = ov
			}
			r := composeRequest{
				Operation:  "remotion_render",
				OutputPath: outPath,
				RawProps:   remotionProps,
			}
			return doRemotionRender(r, outPath, tmo)
		}
	}

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
			outPath = r.Output
		}
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
			outPath = r.Output
		}
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

func findComposerDir() string {
	candidates := []string{
		"remotion-composer",
		filepath.Join("..", "remotion-composer"),
		filepath.Join("..", "..", "remotion-composer"),
		filepath.Join("..", "..", "..", "remotion-composer"),
		filepath.Join("packs", "explainer", "runtime"),
		filepath.Join("..", "packs", "explainer", "runtime"),
		filepath.Join("..", "..", "packs", "explainer", "runtime"),
	}
	for _, cand := range candidates {
		if _, err := os.Stat(filepath.Join(cand, "package.json")); err == nil {
			if abs, err := filepath.Abs(cand); err == nil {
				return abs
			}
			return cand
		}
	}

	curr, err := os.Getwd()
	if err == nil {
		for {
			cand := filepath.Join(curr, "remotion-composer")
			if _, err := os.Stat(filepath.Join(cand, "package.json")); err == nil {
				if abs, err := filepath.Abs(cand); err == nil {
					return abs
				}
				return cand
			}
			parent := filepath.Dir(curr)
			if parent == curr || parent == "." {
				break
			}
			curr = parent
		}
	}

	if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
		appCandidates := []string{
			filepath.Join(localApp, "Facet", "runtimes", "remotion", "current"),
			filepath.Join(localApp, "Facet", "runtimes", "remotion"),
		}
		for _, cand := range appCandidates {
			if _, err := os.Stat(filepath.Join(cand, "package.json")); err == nil {
				if abs, err := filepath.Abs(cand); err == nil {
					return abs
				}
				return cand
			}
		}
	}

	return "remotion-composer"
}

func doRemotionRender(r composeRequest, outPath string, tmo time.Duration) (any, []string, error) {
	outDir := filepath.Dir(outPath)
	if outDir != "" && outDir != "." {
		_ = os.MkdirAll(outDir, 0755)
	}

	composerDir := findComposerDir()
	absComposer, err := filepath.Abs(composerDir)
	if err != nil {
		absComposer = composerDir
	}

	compositionID := "Explainer"
	if r.CompositionID != "" {
		compositionID = r.CompositionID
	} else if r.EditDecisions != nil && r.EditDecisions.RendererFamily != "" {
		switch r.EditDecisions.RendererFamily {
		case "cinematic-trailer", "documentary-montage":
			compositionID = "CinematicRenderer"
		case "presenter":
			compositionID = "TalkingHead"
		default:
			compositionID = "Explainer"
		}
	}

	var propsJSON []byte
	if r.RawProps != nil {
		propsJSON, _ = json.Marshal(r.RawProps)
	} else if r.EditDecisions != nil {
		propsJSON, _ = json.Marshal(r.EditDecisions)
	} else {
		propsJSON = []byte("{}")
	}

	propsDir := filepath.Dir(outPath)
	if propsDir == "" || propsDir == "." {
		propsDir = "."
	}
	propsPath := filepath.Join(propsDir, ".remotion_props.json")
	if err := os.WriteFile(propsPath, propsJSON, 0644); err != nil {
		return nil, nil, failure("command_failed", "unable to write remotion props: "+err.Error(), nil)
	}
	defer os.Remove(propsPath)

	absOut, _ := filepath.Abs(outPath)
	absProps, _ := filepath.Abs(propsPath)
	entryFile := filepath.Join(absComposer, "src", "index.tsx")

	args := []string{"--prefix", absComposer, "remotion", "render", entryFile, compositionID, absOut, "--props=" + absProps}
	if browser := findBrowserExecutable(); browser != "" {
		args = append(args, "--browser-executable="+browser)
	}
	if _, err := runCommand(tmo, "npx", args...); err != nil {
		return renderExplainerWithFFmpeg(r, outPath, tmo)
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

func findDefaultFontFile() string {
	candidates := []string{
		`C:\Windows\Fonts\segoeui.ttf`,
		`C:\Windows\Fonts\arial.ttf`,
		`/System/Library/Fonts/Helvetica.ttc`,
		`/Library/Fonts/Arial.ttf`,
		`/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf`,
		`/usr/share/fonts/TTF/DejaVuSans.ttf`,
	}
	for _, c := range candidates {
		if fileExists(c) {
			escaped := strings.ReplaceAll(filepath.ToSlash(c), ":", `\:`)
			return fmt.Sprintf("fontfile='%s':", escaped)
		}
	}
	return ""
}

func escapeDrawtext(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	s = strings.ReplaceAll(s, `:`, `\:`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	return s
}

func renderExplainerWithFFmpeg(r composeRequest, outPath string, tmo time.Duration) (any, []string, error) {
	tempDir, err := os.MkdirTemp(filepath.Dir(outPath), ".explainer_tmp-*")
	if err != nil {
		return nil, nil, failure("command_failed", "unable to create temporary directory", nil)
	}
	defer os.RemoveAll(tempDir)

	var cuts []map[string]any
	if r.RawProps != nil {
		if c, ok := r.RawProps["cuts"].([]any); ok {
			for _, item := range c {
				if m, ok := item.(map[string]any); ok {
					cuts = append(cuts, m)
				}
			}
		}
	} else if r.EditDecisions != nil {
		for _, c := range r.EditDecisions.Cuts {
			cuts = append(cuts, map[string]any{
				"id":          c.ID,
				"source":      c.Source,
				"in_seconds":  c.InSeconds,
				"out_seconds": c.OutSeconds,
				"type":        c.Type,
				"text":        c.Text,
				"title":       c.Title,
				"subtitle":    c.Subtitle,
			})
		}
	}

	if len(cuts) == 0 {
		return nil, nil, failure("invalid_request", "no cuts provided for explainer render", nil)
	}

	fontOpt := findDefaultFontFile()
	tempSegments := make([]string, len(cuts))
	for i, cut := range cuts {
		inS := 0.0
		if v, ok := cut["in_seconds"].(float64); ok {
			inS = v
		}
		outS := 0.0
		if v, ok := cut["out_seconds"].(float64); ok {
			outS = v
		}
		dur := outS - inS
		if dur <= 0 {
			dur = 3.0
		}
		cutType, _ := cut["type"].(string)
		text, _ := cut["text"].(string)
		sub, _ := cut["subtitle"].(string)
		title, _ := cut["title"].(string)
		stat, _ := cut["stat"].(string)
		src, _ := cut["source"].(string)

		segFile := filepath.Join(tempDir, fmt.Sprintf("seg_%04d.mp4", i))

		if src != "" && fileExists(src) {
			vf := "scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=30,format=yuv420p"
			args := []string{"-hide_banner", "-loglevel", "error", "-y", "-ss", formatFloat(inS), "-t", formatFloat(dur), "-i", src, "-vf", vf, "-c:v", "libx264", "-crf", "23", "-preset", "medium", "-an", segFile}
			if _, err := runCommand(tmo, "ffmpeg", args...); err != nil {
				return nil, nil, err
			}
		} else {
			var vf string
			switch cutType {
			case "hero_title":
				vf = fmt.Sprintf("drawbox=x=160:y=120:w=1600:h=840:color=0x1E293B@0.6:t=fill,drawbox=x=160:y=120:w=1600:h=840:color=0x7C3AED@0.8:t=4,drawtext=%stext='%s':fontsize=68:fontcolor=0xF8FAFC:x=(w-text_w)/2:y=(h-text_h)/2-60,drawtext=%stext='%s':fontsize=38:fontcolor=0x22D3EE:x=(w-text_w)/2:y=(h-text_h)/2+60,fps=30,format=yuv420p", fontOpt, escapeDrawtext(text), fontOpt, escapeDrawtext(sub))
			case "stat_card":
				vf = fmt.Sprintf("drawbox=x=200:y=150:w=1520:h=780:color=0x1E293B@0.7:t=fill,drawbox=x=200:y=150:w=1520:h=780:color=0xEC4899@0.9:t=4,drawtext=%stext='%s':fontsize=120:fontcolor=0xEC4899:x=(w-text_w)/2:y=(h-text_h)/2-70,drawtext=%stext='%s':fontsize=42:fontcolor=0xF8FAFC:x=(w-text_w)/2:y=(h-text_h)/2+80,fps=30,format=yuv420p", fontOpt, escapeDrawtext(stat), fontOpt, escapeDrawtext(sub))
			case "callout":
				vf = fmt.Sprintf("drawbox=x=240:y=180:w=1440:h=720:color=0x1E293B@0.8:t=fill,drawbox=x=240:y=180:w=1440:h=720:color=0x22D3EE@0.9:t=4,drawtext=%stext='%s':fontsize=56:fontcolor=0x22D3EE:x=(w-text_w)/2:y=300,drawtext=%stext='%s':fontsize=36:fontcolor=0xF8FAFC:x=(w-text_w)/2:y=480,fps=30,format=yuv420p", fontOpt, escapeDrawtext(title), fontOpt, escapeDrawtext(text))
			default:
				displayText := text
				if displayText == "" {
					displayText = title
				}
				if displayText == "" {
					displayText = "Scene"
				}
				vf = fmt.Sprintf("drawbox=x=200:y=150:w=1520:h=780:color=0x1E293B@0.6:t=fill,drawtext=%stext='%s':fontsize=52:fontcolor=0xF8FAFC:x=(w-text_w)/2:y=(h-text_h)/2,fps=30,format=yuv420p", fontOpt, escapeDrawtext(displayText))
			}
			args := []string{"-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", fmt.Sprintf("color=c=0x0F172A:s=1920x1080:d=%s", formatFloat(dur)), "-vf", vf, "-c:v", "libx264", "-crf", "23", "-preset", "medium", "-an", segFile}
			if _, err := runCommand(tmo, "ffmpeg", args...); err != nil {
				return nil, nil, err
			}
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

	// Resolve audio file
	audioFile := r.AudioPath
	if audioFile == "" && r.RawProps != nil {
		if ap, ok := r.RawProps["audio_path"].(string); ok && ap != "" {
			audioFile = ap
		} else if aud, ok := r.RawProps["audio"].(map[string]any); ok {
			if narr, ok := aud["narration"].(map[string]any); ok {
				if src, ok := narr["src"].(string); ok && src != "" {
					audioFile = src
				}
			}
		}
	}

	if audioFile != "" && fileExists(audioFile) {
		args := []string{"-hide_banner", "-loglevel", "error", "-y", "-i", concatOut, "-i", audioFile, "-map", "0:v:0", "-map", "1:a:0", "-c:v", "copy", "-c:a", "aac", "-ar", "48000", "-ac", "2", "-shortest", outPath}
		if _, err := runCommand(tmo, "ffmpeg", args...); err != nil {
			return nil, nil, err
		}
	} else {
		// Synthesize silent stereo audio track
		args := []string{"-hide_banner", "-loglevel", "error", "-y", "-i", concatOut, "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo", "-map", "0:v:0", "-map", "1:a:0", "-c:v", "copy", "-c:a", "aac", "-shortest", outPath}
		if _, err := runCommand(tmo, "ffmpeg", args...); err != nil {
			return nil, nil, err
		}
	}

	return map[string]any{
		"operation":      "remotion_render",
		"composition_id": "Explainer",
		"cut_count":      len(cuts),
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

func findBrowserExecutable() string {
	if env := os.Getenv("REMOTION_BROWSER_EXECUTABLE"); env != "" && fileExists(env) {
		return env
	}
	if env := os.Getenv("PUPPETEER_EXECUTABLE_PATH"); env != "" && fileExists(env) {
		return env
	}
	if env := os.Getenv("CHROME_PATH"); env != "" && fileExists(env) {
		return env
	}
	candidates := []string{
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		`/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`,
		`/usr/bin/google-chrome`,
		`/usr/bin/chromium-browser`,
		`/usr/bin/chromium`,
	}
	for _, c := range candidates {
		if fileExists(c) {
			return c
		}
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
