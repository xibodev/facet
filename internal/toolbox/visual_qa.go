package toolbox

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type reviewRequest struct {
	Input   string `json:"input,omitempty"`
	Profile struct {
		Width  int     `json:"width"`
		Height int     `json:"height"`
		FPS    float64 `json:"fps"`
	} `json:"profile"`
	Checks struct {
		Duration *struct {
			Expected  float64 `json:"expected"`
			Tolerance float64 `json:"tolerance"`
		} `json:"duration"`
		VideoCodec  string `json:"video_codec"`
		PixelFormat string `json:"pixel_format"`
		Audio       *struct {
			Required   bool   `json:"required"`
			Codec      string `json:"codec"`
			SampleRate int    `json:"sample_rate"`
			Channels   int    `json:"channels"`
		} `json:"audio"`
	} `json:"checks"`
	Samples        sampleStrategy `json:"samples"`
	EvidenceDir    string         `json:"evidence_dir"`
	TimeoutSeconds int            `json:"timeout_seconds,omitempty"`
}

type visualQARequest struct {
	Operation      string                 `json:"operation,omitempty"`
	InputPath      string                 `json:"input_path,omitempty"`
	Input          string                 `json:"input,omitempty"`
	Timestamps     []float64              `json:"timestamps,omitempty"`
	OutputDir      string                 `json:"output_dir,omitempty"`
	Checks         []string               `json:"checks,omitempty"`
	Expected       map[string]any         `json:"expected,omitempty"`
	TimeoutSeconds int                    `json:"timeout_seconds,omitempty"`
}

var volumeRE = regexp.MustCompile(`(?m)(mean_volume|max_volume):\s*([-+\w.]+)\s*dB`)

func parseVolume(s string) map[string]any {
	m := map[string]any{}
	for _, v := range volumeRE.FindAllStringSubmatch(s, -1) {
		m[v[1]+"_db"] = v[2]
	}
	return m
}

func gate(name string, pass bool, message string) map[string]any {
	status := "fail"
	if pass {
		status = "pass"
	}
	g := map[string]any{"name": name, "status": status}
	if !pass && message != "" {
		g["message"] = message
	}
	return g
}

func firstVideo(p map[string]any) map[string]any {
	v, _ := p["video"].(map[string]any)
	return v
}

func hasAudio(p map[string]any) bool {
	if auds, ok := p["audio_streams"].([]map[string]any); ok {
		return len(auds) > 0
	}
	return false
}

func doOutputReview(op string, data []byte) (any, []string, error) {
	var r reviewRequest
	if err := decode(data, &r); err != nil {
		return nil, nil, err
	}
	tmo, err := positiveTimeout(r.TimeoutSeconds, 90)
	if err != nil {
		return nil, nil, err
	}
	if err = inputPath(r.Input); err != nil {
		return nil, nil, err
	}
	if r.Profile.Width <= 0 || r.Profile.Height <= 0 || !finite(r.Profile.FPS) || r.Profile.FPS <= 0 || r.Checks.VideoCodec == "" || r.Checks.PixelFormat == "" || r.Checks.Audio == nil || r.Checks.Duration == nil {
		return nil, nil, failure("invalid_request", "profile and concrete duration, video, pixel format, and audio checks are required", nil)
	}
	if !finite(r.Checks.Duration.Expected) || !finite(r.Checks.Duration.Tolerance) || r.Checks.Duration.Expected <= 0 || r.Checks.Duration.Tolerance < 0 || r.Checks.Audio.SampleRate <= 0 || r.Checks.Audio.Channels <= 0 || strings.TrimSpace(r.Checks.Audio.Codec) == "" {
		return nil, nil, failure("invalid_request", "review duration and audio checks contain invalid values", nil)
	}
	if r.Samples.Type != "uniform" || r.Samples.Count != 4 {
		return nil, nil, failure("invalid_request", "output_review requires exactly 4 uniform samples", nil)
	}
	if strings.TrimSpace(r.EvidenceDir) == "" {
		return nil, nil, failure("invalid_request", "evidence_dir is required", nil)
	}
	if op == "estimate" {
		return map[string]any{"estimated_cost": 0, "network": false, "external_write": false, "side_effect_free": true, "operations": []string{"validate_checks", "ffprobe", "extract_4_frames", "volumedetect"}, "validation_scope": "request shape, concrete gate constraints, exact uniform sample strategy, and paths; media decoding and gate evaluation occur during run"}, nil, nil
	}
	p, w, err := probe(r.Input, tmo)
	if err != nil {
		return nil, nil, err
	}
	v := firstVideo(p)
	if v == nil {
		return nil, nil, failure("output_validation_failed", "output has no video stream", nil)
	}
	dur := p["format"].(map[string]any)["duration"].(float64)
	audios, _ := p["audio_streams"].([]map[string]any)
	gates := []map[string]any{}
	gates = append(gates, gate("profile", v["width"].(int) == r.Profile.Width && v["height"].(int) == r.Profile.Height && math.Abs(v["fps"].(float64)-r.Profile.FPS) < .02, "dimensions or fps differ"))
	gates = append(gates, gate("duration", math.Abs(dur-r.Checks.Duration.Expected) <= r.Checks.Duration.Tolerance, "duration outside tolerance"))
	gates = append(gates, gate("video_codec", v["codec"] == r.Checks.VideoCodec, "video codec differs"))
	gates = append(gates, gate("pixel_format", v["pixel_format"] == r.Checks.PixelFormat, "pixel format differs"))
	audioPass := !r.Checks.Audio.Required
	if len(audios) > 0 {
		a := audios[0]
		audioPass = a["codec"] == r.Checks.Audio.Codec && a["sample_rate"] == r.Checks.Audio.SampleRate && a["channels"] == r.Checks.Audio.Channels
	}
	gates = append(gates, gate("audio", audioPass, "audio expectation differs"))
	sampleJSON, _ := json.Marshal(sampleRequest{Input: r.Input, OutputDir: r.EvidenceDir, Strategy: r.Samples, ImageFormat: "jpg", Overwrite: true, TimeoutSeconds: r.TimeoutSeconds})
	samples, sw, err := doFrameSample("run", sampleJSON)
	if err != nil {
		return nil, nil, failure("partial_result", "technical review completed but evidence extraction failed", map[string]any{"execution_status": "partial", "review_status": "revise", "completed_artifacts": map[string]any{"gates": gates, "output_facts": p}, "failures": []map[string]any{{"operation": "sample_extraction", "error": errorEnvelope("frame_sample", "run", err).Error}}})
	}
	w = append(w, sw...)
	vol, verr := runCommand(tmo, "ffmpeg", "-hide_banner", "-i", r.Input, "-map", "0:a:0", "-af", "volumedetect", "-f", "null", "-")
	volume := map[string]any{}
	if verr != nil {
		w = append(w, "volumedetect unavailable: "+bounded(verr.Error()))
	} else {
		volume = parseVolume(string(vol))
	}
	status := "pass"
	for _, g := range gates {
		if g["status"] == "fail" {
			status = "fail"
		}
	}
	if status == "pass" && len(w) > 0 {
		status = "warn"
	}
	return map[string]any{"execution_status": "succeeded", "review_status": status, "gates": gates, "samples": samples.(map[string]any)["samples"], "volume": volume, "output_facts": p}, w, nil
}

func doVisualQA(op string, data []byte) (any, []string, error) {
	var r visualQARequest
	if err := decode(data, &r); err != nil {
		return nil, nil, err
	}
	input := r.InputPath
	if input == "" {
		input = r.Input
	}
	if input == "" {
		return nil, nil, failure("invalid_request", "input_path is required", nil)
	}
	operation := r.Operation
	if operation == "" {
		operation = "probe"
	}
	tmo, err := positiveTimeout(r.TimeoutSeconds, 90)
	if err != nil {
		return nil, nil, err
	}
	if err = inputPath(input); err != nil {
		return nil, nil, err
	}

	if op == "estimate" {
		return estimateResult([]string{"visual_qa_" + operation, "ffprobe"}), nil, nil
	}

	p, _, err := probe(input, tmo)
	if err != nil {
		return nil, nil, err
	}
	duration := p["format"].(map[string]any)["duration"].(float64)

	switch operation {
	case "review":
		outDir := r.OutputDir
		if outDir == "" {
			outDir = filepath.Join(filepath.Dir(input), "review_frames")
		}
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return nil, nil, failure("command_failed", "unable to create output directory", nil)
		}
		ts := r.Timestamps
		if len(ts) == 0 {
			ts = []float64{1.0, duration * 0.25, duration * 0.50, duration * 0.75, math.Max(0, duration-1.0)}
		}
		frames := []map[string]any{}
		for _, tVal := range ts {
			label := strings.ReplaceAll(fmt.Sprintf("%.1f", tVal), ".", "_")
			frameFile := filepath.Join(outDir, fmt.Sprintf("frame_%ss.jpg", label))
			args := []string{"-hide_banner", "-loglevel", "error", "-y", "-ss", formatFloat(tVal), "-i", input, "-frames:v", "1", "-q:v", "2", frameFile}
			if _, err := runCommand(tmo, "ffmpeg", args...); err == nil {
				frames = append(frames, map[string]any{
					"timestamp": tVal,
					"path":      frameFile,
				})
			}
		}
		return map[string]any{
			"operation":   "review",
			"input":       input,
			"frame_count": len(frames),
			"frames":      frames,
		}, nil, nil

	case "probe":
		v := firstVideo(p)
		audios, _ := p["audio_streams"].([]map[string]any)
		info := map[string]any{
			"operation":    "probe",
			"input":        input,
			"duration":     duration,
			"file_size_mb": roundFloat(float64(p["format"].(map[string]any)["size"].(int64))/1048576.0, 1),
			"has_audio":    len(audios) > 0,
		}
		if v != nil {
			info["width"] = v["width"]
			info["height"] = v["height"]
			info["pixel_format"] = v["pixel_format"]
			info["video_codec"] = v["codec"]
			info["fps"] = v["fps"]
		}
		if len(audios) > 0 {
			a := audios[0]
			info["audio_codec"] = a["codec"]
			info["sample_rate"] = a["sample_rate"]
			info["channels"] = a["channels"]
		}
		issues := []string{}
		if exp := r.Expected; exp != nil {
			if ew, ok := exp["width"].(float64); ok && v != nil && v["width"].(int) != int(ew) {
				issues = append(issues, fmt.Sprintf("Width: expected %d, got %d", int(ew), v["width"].(int)))
			}
			if eh, ok := exp["height"].(float64); ok && v != nil && v["height"].(int) != int(eh) {
				issues = append(issues, fmt.Sprintf("Height: expected %d, got %d", int(eh), v["height"].(int)))
			}
			if minD, ok := exp["min_duration"].(float64); ok && duration < minD {
				issues = append(issues, fmt.Sprintf("Duration too short: %.1fs < %.1fs", duration, minD))
			}
			if maxD, ok := exp["max_duration"].(float64); ok && duration > maxD {
				issues = append(issues, fmt.Sprintf("Duration too long: %.1fs > %.1fs", duration, maxD))
			}
			if epf, ok := exp["pixel_format"].(string); ok && v != nil && v["pixel_format"].(string) != epf {
				issues = append(issues, fmt.Sprintf("Pixel format: expected %s, got %s", epf, v["pixel_format"].(string)))
			}
			if ha, ok := exp["has_audio"].(bool); ok && (len(audios) > 0) != ha {
				issues = append(issues, fmt.Sprintf("Audio: expected %v, got %v", ha, len(audios) > 0))
			}
		}
		info["validation_issues"] = issues
		info["validation_passed"] = len(issues) == 0
		return info, nil, nil

	case "audio_levels":
		ts := r.Timestamps
		if len(ts) == 0 {
			ts = []float64{1.0, duration * 0.5, math.Max(0, duration-2.0)}
		}
		levels := []map[string]any{}
		for _, tVal := range ts {
			cmdOut, err := runCommand(tmo, "ffmpeg", "-hide_banner", "-ss", formatFloat(tVal), "-t", "3", "-i", input, "-vn", "-af", "volumedetect", "-f", "null", "-")
			if err != nil {
				levels = append(levels, map[string]any{"timestamp": tVal, "error": err.Error()})
			} else {
				vol := parseVolume(string(cmdOut))
				levels = append(levels, map[string]any{
					"timestamp":      tVal,
					"mean_volume_db": vol["mean_volume_db"],
					"max_volume_db":  vol["max_volume_db"],
				})
			}
		}
		return map[string]any{
			"operation": "audio_levels",
			"input":     input,
			"levels":    levels,
		}, nil, nil

	default:
		return nil, nil, failure("invalid_request", "unknown operation: "+operation, nil)
	}
}
