package toolbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
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
						"id":          sm["id"],
						"type":        sm["type"],
						"in_seconds":  sm["start_seconds"],
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

// bundleRoot is a host-supplied read-only bundle location. When set it wins
// over discovery: the host knows where it installed the module's content, and
// the working directory does not. Under a module host cwd is not promised at
// all, which made composer discovery depend on where the process was launched.
var (
	bundleMu   sync.RWMutex
	bundleRoot string
)

// SetBundleRoot installs the host-supplied bundle location for one invocation.
// An empty value restores ordinary discovery, which is what the CLI uses.
func SetBundleRoot(path string) {
	bundleMu.Lock()
	bundleRoot = strings.TrimSpace(path)
	bundleMu.Unlock()
}

func hostBundleRoot() string {
	bundleMu.RLock()
	defer bundleMu.RUnlock()
	return bundleRoot
}

func findComposerDir() (string, error) {
	// A host-supplied bundle is authoritative and checked before anything else.
	if root := hostBundleRoot(); root != "" {
		candidate := filepath.Join(root, "remotion-composer")
		if fileExists(filepath.Join(candidate, "package.json")) {
			return filepath.Abs(candidate)
		}
	}
	home, _ := os.UserHomeDir()
	configPaths := []string{".facet.yaml"}
	if home != "" {
		configPaths = append(configPaths, filepath.Join(home, ".config", "facet", "config.yaml"))
	}
	// Read only runtime paths here: config imports toolbox, so importing it would cycle.
	var cfg struct {
		Paths struct {
			RemotionComposer string `yaml:"remotion_composer"`
			Bundle           string `yaml:"bundle"`
		} `yaml:"paths"`
	}
	for _, path := range configPaths {
		if !fileExists(path) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", failure("invalid_request", "unable to read runtime config: "+err.Error(), nil)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return "", failure("invalid_request", "unable to parse runtime config: "+err.Error(), nil)
		}
		break
	}
	if cfg.Paths.RemotionComposer != "" {
		if !fileExists(filepath.Join(cfg.Paths.RemotionComposer, "package.json")) {
			return "", failure("dependency_missing", "configured Remotion Composer package.json not found", map[string]any{"path": cfg.Paths.RemotionComposer})
		}
		return filepath.Abs(cfg.Paths.RemotionComposer)
	}
	candidates := []string{
		"remotion-composer",
		filepath.Join("..", "remotion-composer"),
		filepath.Join("..", "..", "remotion-composer"),
		filepath.Join("..", "..", "..", "remotion-composer"),
		filepath.Join("packs", "explainer", "runtime"),
		filepath.Join("..", "packs", "explainer", "runtime"),
		filepath.Join("..", "..", "packs", "explainer", "runtime"),
	}
	if cfg.Paths.Bundle != "" {
		candidates = append([]string{filepath.Join(cfg.Paths.Bundle, "remotion-composer")}, candidates...)
	}
	for _, cand := range candidates {
		if fileExists(filepath.Join(cand, "package.json")) {
			return filepath.Abs(cand)
		}
	}

	curr, err := os.Getwd()
	if err == nil {
		for {
			cand := filepath.Join(curr, "remotion-composer")
			if fileExists(filepath.Join(cand, "package.json")) {
				return filepath.Abs(cand)
			}
			parent := filepath.Dir(curr)
			if parent == curr || parent == "." {
				break
			}
			curr = parent
		}
	}

	if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
		candidates = []string{
			filepath.Join(localApp, "Facet", "runtimes", "remotion", "current"),
			filepath.Join(localApp, "Facet", "runtimes", "remotion"),
		}
	} else {
		candidates = nil
	}
	if executable, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(executable); err == nil {
			executable = resolved
		}
		root := filepath.Dir(filepath.Dir(executable))
		candidates = append(candidates, filepath.Join(root, "bundle", "remotion-composer"), filepath.Join(root, "remotion-composer"))
	}
	if home != "" {
		candidates = append(candidates, filepath.Join(home, ".facet", "bundle", "remotion-composer"))
	}
	for _, cand := range candidates {
		if fileExists(filepath.Join(cand, "package.json")) {
			return filepath.Abs(cand)
		}
	}

	return "", failure("dependency_missing", "Remotion Composer runtime not found; install the Facet bundle or configure paths.remotion_composer", nil)
}

func doRemotionRender(r composeRequest, outPath string, tmo time.Duration) (any, []string, error) {
	absComposer, err := findComposerDir()
	if err != nil {
		return nil, nil, err
	}
	cliPath := filepath.Join(absComposer, "node_modules", "@remotion", "cli", "remotion-cli.js")
	if !fileExists(cliPath) {
		return nil, nil, failure("dependency_missing", "Remotion render CLI not found; install the composer dependencies with npm ci", map[string]any{"path": cliPath})
	}
	entryFile := filepath.Join(absComposer, "src", "index.tsx")
	if !fileExists(entryFile) {
		return nil, nil, failure("dependency_missing", "Remotion composition entry point not found", map[string]any{"path": entryFile})
	}
	if r.AudioPath != "" {
		if err := inputPath(r.AudioPath); err != nil {
			return nil, nil, err
		}
	}
	if err := outputPath(outPath, true, false); err != nil {
		return nil, nil, err
	}
	renderPath, cleanup, err := temporaryOutput(outPath)
	if err != nil {
		return nil, nil, err
	}
	defer cleanup()

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
		propsJSON, err = json.Marshal(r.RawProps)
	} else if r.EditDecisions != nil {
		propsJSON, err = json.Marshal(r.EditDecisions)
	} else {
		propsJSON = []byte("{}")
	}
	if err != nil {
		return nil, nil, failure("invalid_request", "unable to encode remotion props", nil)
	}
	renderDir, err := os.MkdirTemp("", "facet-remotion-*")
	if err != nil {
		return nil, nil, failure("command_failed", "unable to create remotion staging directory", nil)
	}
	defer os.RemoveAll(renderDir)
	publicDir := filepath.Join(renderDir, "public")
	if err := os.Mkdir(publicDir, 0700); err != nil {
		return nil, nil, err
	}
	propsJSON, err = stageRemotionMedia(propsJSON, absComposer, publicDir)
	if err != nil {
		return nil, nil, err
	}
	propsPath := filepath.Join(renderDir, "props.json")
	if err := os.WriteFile(propsPath, propsJSON, 0600); err != nil {
		return nil, nil, failure("command_failed", "unable to write remotion props: "+err.Error(), nil)
	}

	absOut, _ := filepath.Abs(renderPath)
	absProps, _ := filepath.Abs(propsPath)

	args := []string{cliPath, "render", entryFile, compositionID, absOut, "--props=" + absProps, "--public-dir=" + publicDir}

	// Honour an explicitly requested output profile.
	//
	// Direct Remotion props carry width/height at the top level, but they were
	// only ever written into the props file — Remotion takes the output
	// dimensions as CLI overrides, not props — so a caller asking for 1280x720
	// silently received the composition's 1920x1080 default. The render
	// succeeded and quietly ignored the request, which is worse than failing.
	if r.RawProps != nil {
		dim := func(key string) int {
			v, ok := r.RawProps[key].(float64)
			if !ok || v <= 0 {
				return 0
			}
			return int(v)
		}
		w, h := dim("width"), dim("height")
		if w > 0 && h > 0 {
			args = append(args, "--width="+strconv.Itoa(w), "--height="+strconv.Itoa(h))
		}
	}

	if browser := findBrowserExecutable(); browser != "" {
		args = append(args, "--browser-executable="+browser)
	}
	if stdout, err := runCommandDir(tmo, absComposer, "node", args...); err != nil {
		var failed *toolFailure
		if errors.As(err, &failed) {
			if failed.err.Code == "command_timeout" {
				failed.err.Details["output"] = remotionTimeoutDiagnostic(string(stdout))
				stderr, _ := failed.err.Details["stderr"].(string)
				failed.err.Details["stderr"] = remotionTimeoutDiagnostic(stderr)
			} else {
				// "node failed" names the binary, not the cause, and an agent
				// that cannot tell whether the module is broken or its request
				// was wrong abandons the module and works around it. Lift the
				// renderer's own first error line into the message so the
				// failure is diagnosable without digging through details.
				stderr, _ := failed.err.Details["stderr"].(string)
				if reason := remotionFailureReason(stderr, string(stdout)); reason != "" {
					failed.err.Message = "remotion render failed: " + reason
				}
				failed.err.Details["composer_dir"] = absComposer
			}
		}
		return nil, nil, err
	}

	if r.AudioPath != "" {
		tempMux, cleanupMux, err := temporaryOutput(outPath)
		if err != nil {
			return nil, nil, err
		}
		defer cleanupMux()
		if _, err := runCommand(tmo, "ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", renderPath, "-i", r.AudioPath, "-map", "0:v:0", "-map", "1:a:0", "-c:v", "copy", "-c:a", "aac", "-shortest", tempMux); err != nil {
			return nil, nil, err
		}
		renderPath = tempMux
	}

	// Validate the completed artifact before replacing an existing delivery.
	if _, _, err := probe(renderPath, tmo); err != nil {
		return nil, nil, err
	}
	if err := os.Rename(renderPath, outPath); err != nil {
		return nil, nil, failure("command_failed", "remotion output could not be published", map[string]any{"error": bounded(err.Error())})
	}
	// Bind evidence to the actual published bytes, including any post-render mux.
	facts, warnings, err := probe(outPath, tmo)
	if err != nil {
		return nil, nil, err
	}

	return map[string]any{
		"operation":      "remotion_render",
		"composition_id": compositionID,
		"output":         outPath,
		"output_facts":   facts,
	}, warnings, nil
}

// Stage only explicit component media fields, never a project/public tree or arbitrary
// strings in metadata. JSON decoding gives us a private copy of the caller's props.
func stageRemotionMedia(data []byte, composer, publicDir string) ([]byte, error) {
	var props map[string]any
	if err := json.Unmarshal(data, &props); err != nil {
		return nil, failure("invalid_request", "invalid remotion props", nil)
	}
	const maxFileSize int64 = 2 << 30
	const maxTotalSize int64 = 8 << 30
	var total int64
	staged := map[string]string{}
	stage := func(src string) (string, error) {
		if src == "" || strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") || strings.HasPrefix(src, "data:") {
			return src, nil // Match resolveAsset's existing URL semantics without fetching.
		}
		if name, ok := staged[src]; ok {
			return name, nil
		}
		invalid := func(message string) (string, error) {
			return "", failure("invalid_request", "remotion media: "+message, nil)
		}
		path := src
		if strings.HasPrefix(strings.ToLower(path), "file:") {
			u, err := url.Parse(path)
			if err != nil || u.Opaque != "" || u.User != nil || (u.Host != "" && u.Host != "localhost") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
				return invalid("file URL must identify a local file without query or fragment")
			}
			path = u.Path
			if os.PathSeparator == '\\' && len(path) >= 3 && path[0] == '/' && path[2] == ':' {
				path = path[1:]
			}
			if !filepath.IsAbs(path) {
				return invalid("file URL must be absolute")
			}
		}
		path = filepath.FromSlash(path)
		if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") || strings.Contains(strings.TrimPrefix(path, filepath.VolumeName(path)), ":") {
			return invalid("unsupported scheme, network path or alternate stream")
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".wav", ".mp3", ".m4a", ".aac", ".flac", ".ogg", ".opus", ".mp4", ".mov", ".webm", ".avi", ".mkv", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tif", ".tiff":
		default:
			return invalid("unsupported media extension (active documents and non-media files are not public assets)")
		}
		var input *os.File
		var err error
		if filepath.IsAbs(path) {
			// Explicit absolute paths may name media outside the project, but not devices.
			info, statErr := os.Stat(path)
			if statErr != nil || !info.Mode().IsRegular() {
				return invalid("absolute source must be an accessible regular file")
			}
			input, err = os.Open(path)
		} else {
			if !filepath.IsLocal(path) {
				return invalid("relative source must stay inside the project")
			}
			// Root.Open also confines symlinks, including during path resolution.
			for _, rootPath := range []string{".", filepath.Join(composer, "public")} {
				root, openErr := os.OpenRoot(rootPath)
				if openErr != nil {
					err = openErr
				} else {
					info, statErr := root.Stat(path)
					if statErr == nil && !info.Mode().IsRegular() {
						root.Close()
						return invalid("source must be a regular file")
					}
					if statErr != nil {
						err = statErr
					} else {
						input, err = root.Open(path)
					}
					root.Close()
				}
				if !errors.Is(err, os.ErrNotExist) {
					break
				}
			}
		}
		if err != nil {
			return invalid("source is missing, inaccessible or escapes its root")
		}
		defer input.Close()
		info, err := input.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxFileSize || total+info.Size() > maxTotalSize || len(staged) >= 256 {
			return invalid("source must be nonempty regular media within staging limits (256 files, 2 GiB each, 8 GiB total)")
		}
		var header [512]byte
		n, err := input.ReadAt(header[:], 0)
		if err != nil && err != io.EOF {
			return invalid("cannot inspect source media")
		}
		mime := http.DetectContentType(header[:n])
		media := strings.HasPrefix(mime, "audio/") || strings.HasPrefix(mime, "video/") || strings.HasPrefix(mime, "image/") || mime == "application/ogg"
		// Containers not recognized by net/http's bounded signature table.
		//
		// The MPEG audio check matches an 11-bit frame sync (0xFF followed by
		// three set bits), which is what the spec actually defines. A tighter
		// mask of 0xf6==0xf0 rejected the most common real headers — 0xf3 and
		// 0xfb, MPEG-1 Layer III — so Facet refused MP3s that its own edge_tts
		// tool had just produced. Verified against a generated narration file
		// whose header is ff f3.
		media = media || (n >= 12 && (string(header[4:8]) == "ftyp" || string(header[4:8]) == "moov" || string(header[4:8]) == "mdat")) ||
			(n >= 4 && (string(header[:4]) == "fLaC" || string(header[:4]) == "\x1a\x45\xdf\xa3" || string(header[:4]) == "II*\x00" || string(header[:4]) == "MM\x00*")) ||
			(n >= 2 && header[0] == 0xff && header[1]&0xe0 == 0xe0)
		if !media {
			return invalid("source does not have a supported media signature")
		}
		name := fmt.Sprintf("asset-%03d%s", len(staged), ext)
		output, err := os.OpenFile(filepath.Join(publicDir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return "", err
		}
		written, copyErr := io.Copy(output, io.LimitReader(input, info.Size()+1))
		closeErr := output.Close()
		if copyErr != nil || closeErr != nil || written != info.Size() {
			return invalid("source changed or could not be staged")
		}
		total += written
		staged[src] = name
		return name, nil
	}
	rewrite := func(object map[string]any, keys []string) error {
		for _, key := range keys {
			if src, ok := object[key].(string); ok {
				value, err := stage(src)
				if err != nil {
					return err
				}
				object[key] = value
			}
		}
		return nil
	}
	if err := rewrite(props, []string{"videoSrc", "backgroundSrc", "productImage"}); err != nil {
		return nil, err
	}
	for _, key := range []string{"cuts", "scenes", "clips"} {
		items, _ := props[key].([]any)
		for _, item := range items {
			object, _ := item.(map[string]any)
			if err := rewrite(object, []string{"source", "src", "backgroundImage", "backgroundVideo", "backgroundSrc"}); err != nil {
				return nil, err
			}
			images, _ := object["images"].([]any)
			for i, image := range images {
				if src, ok := image.(string); ok {
					value, err := stage(src)
					if err != nil {
						return nil, err
					}
					images[i] = value
				}
			}
		}
	}
	audio, _ := props["audio"].(map[string]any)
	for _, layer := range []any{audio["narration"], audio["music"], props["soundtrack"], props["music"]} {
		object, _ := layer.(map[string]any)
		if err := rewrite(object, []string{"src"}); err != nil {
			return nil, err
		}
	}
	return json.Marshal(props)
}

// Only retain known progress lines: browser logs can echo props, source code,
// credentials and signed media URLs. Free-form timeout output is not safe to expose.
var remotionProgressLine = regexp.MustCompile(`^(Bundling [0-9]+%|Getting composition|Concurrency +[0-9]+x|Rendered [0-9]+/[0-9]+(, time remaining: [0-9hms .]+)?|Encoded [0-9]+/[0-9]+)$`)

func remotionTimeoutDiagnostic(output string) string {
	var progress strings.Builder
	omitted := false
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if remotionProgressLine.MatchString(line) {
			progress.WriteString(line + "\n")
		} else if line != "" {
			omitted = true
		}
	}
	text := progress.String()
	if omitted {
		text += "[REDACTED non-progress renderer output]\n"
	}
	return bounded(text)
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

// ansiEscape strips terminal colouring so a renderer's own error text is
// readable in a JSON envelope and in a cockpit.
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// remotionFailureReason extracts the renderer's own first error line.
//
// A message naming only the binary — "node failed" — leaves a caller unable to
// tell whether the module is broken or its request was wrong, and an agent that
// cannot tell abandons the module and works around it. That was observed: a
// render failure led an agent to fall back to raw ffmpeg and report a blank
// video as a success.
//
// Only lines the renderer itself emitted are lifted, bounded, and stripped of
// colour codes. Everything else stays in details rather than being guessed at.
func remotionFailureReason(stderr, stdout string) string {
	for _, source := range []string{stderr, stdout} {
		for _, line := range strings.Split(strings.ReplaceAll(source, "\r", "\n"), "\n") {
			line = strings.TrimSpace(ansiEscape.ReplaceAllString(line, ""))
			if line == "" {
				continue
			}
			// Remotion labels its errors and then repeats the type, so a real
			// line reads "Error  Error: Could not find composition with ID X".
			// Match on the embedded type rather than a prefix, which is what
			// the actual output looks like once colour codes are stripped.
			for _, marker := range []string{
				"Error:", "TypeError:", "ReferenceError:", "SyntaxError:",
				"Cannot find module", "ENOENT",
			} {
				if i := strings.Index(line, marker); i >= 0 {
					// Puppeteer teardown noise is a symptom of the real
					// failure, not the failure, and naming it would send a
					// caller after the wrong thing.
					if strings.Contains(line, "Was not able to close puppeteer page") {
						break
					}
					return boundedReason(strings.TrimSpace(line[i:]))
				}
			}
		}
	}
	return ""
}

func boundedReason(s string) string {
	const limit = 300
	if len(s) > limit {
		return s[:limit] + "..."
	}
	return s
}
