package toolbox

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kolonist/edgetts"
)

type edgeTTSRequest struct {
	Text           string `json:"text"`
	Voice          string `json:"voice,omitempty"`
	Rate           string `json:"rate,omitempty"`
	Pitch          string `json:"pitch,omitempty"`
	Volume         string `json:"volume,omitempty"`
	OutputPath     string `json:"output_path,omitempty"`
	Output         string `json:"output,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func doEdgeTTS(op string, data []byte) (any, []string, error) {
	var r edgeTTSRequest
	if err := decode(data, &r); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(r.Text) == "" {
		return nil, nil, failure("invalid_request", "text is required", nil)
	}

	voice := r.Voice
	if voice == "" {
		voice = "en-US-ChristopherNeural"
	}
	rate := r.Rate
	if rate == "" {
		rate = "+0%"
	}
	pitch := r.Pitch
	if pitch == "" {
		pitch = "+0Hz"
	}
	volume := r.Volume
	if volume == "" {
		volume = "+0%"
	}

	outPath := r.OutputPath
	if outPath == "" {
		outPath = r.Output
	}
	if outPath == "" {
		outPath = "edge_tts.mp3"
	}

	if op == "estimate" {
		res := estimateResult([]string{"edge_tts_synthesize"})
		res["voice"] = voice
		res["output"] = outPath
		res["estimated_cost"] = 0.0
		res["network"] = true
		return res, nil, nil
	}

	if err := outputPath(outPath, true, false); err != nil {
		return nil, nil, err
	}

	timeoutSec := r.TimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = 30
	}

	audioBytes, err := synthesizeEdgeTTS(r.Text, voice, rate, pitch, volume, time.Duration(timeoutSec)*time.Second)
	if err != nil {
		return nil, nil, failure("tts_failed", fmt.Sprintf("edge-tts synthesis failed: %v", err), nil)
	}

	if err := os.WriteFile(outPath, audioBytes, 0o644); err != nil {
		return nil, nil, failure("output_failed", fmt.Sprintf("failed to write output: %v", err), nil)
	}

	res := map[string]any{
		"output":     outPath,
		"voice":      voice,
		"size_bytes": len(audioBytes),
		"format":     "mp3",
		"provider":   "microsoft_edge",
	}

	// Try to get duration from ffprobe if available
	if probeRes, _, err := doMediaProbe("run", []byte(fmt.Sprintf(`{"input_path":%q}`, outPath))); err == nil {
		if pm, ok := probeRes.(map[string]any); ok {
			if dur, ok := pm["duration_seconds"]; ok {
				res["duration_seconds"] = dur
			}
		}
	}

	return res, nil, nil
}

func synthesizeEdgeTTS(text, voice, rate, pitch, volume string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := edgetts.Args{
		Voice:  voice,
		Rate:   rate,
		Volume: volume,
	}
	client := edgetts.New(args)
	speaker := client.Speak(text)
	return speaker.GetSound(ctx, edgetts.OutputFormatMp3)
}
