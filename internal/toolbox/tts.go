package toolbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type openAITTSRequest struct {
	Text           string  `json:"text"`
	Voice          string  `json:"voice,omitempty"`
	Model          string  `json:"model,omitempty"`
	Format         string  `json:"format,omitempty"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Instructions   string  `json:"instructions,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
	OutputPath     string  `json:"output_path,omitempty"`
	TimeoutSeconds int     `json:"timeout_seconds,omitempty"`
}

type elevenLabsTTSRequest struct {
	Text            string  `json:"text"`
	VoiceID         string  `json:"voice_id,omitempty"`
	ModelID         string  `json:"model_id,omitempty"`
	Stability       float64 `json:"stability,omitempty"`
	SimilarityBoost float64 `json:"similarity_boost,omitempty"`
	Style           float64 `json:"style,omitempty"`
	Speed           float64 `json:"speed,omitempty"`
	UseSpeakerBoost *bool   `json:"use_speaker_boost,omitempty"`
	OutputFormat    string  `json:"output_format,omitempty"`
	OutputPath      string  `json:"output_path,omitempty"`
	TimeoutSeconds  int     `json:"timeout_seconds,omitempty"`
}

type piperTTSRequest struct {
	Text            string  `json:"text"`
	Model           string  `json:"model,omitempty"`
	SpeakerID       int     `json:"speaker_id,omitempty"`
	LengthScale     float64 `json:"length_scale,omitempty"`
	SentenceSilence float64 `json:"sentence_silence,omitempty"`
	OutputPath      string  `json:"output_path,omitempty"`
	TimeoutSeconds  int     `json:"timeout_seconds,omitempty"`
}

func doOpenAITTS(op string, data []byte) (any, []string, error) {
	var r openAITTSRequest
	if err := decode(data, &r); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(r.Text) == "" {
		return nil, nil, failure("invalid_request", "text is required", nil)
	}
	if op == "estimate" {
		cost := roundFloat(float64(len(r.Text))*0.000015, 4)
		res := estimateResult([]string{"openai_tts_generate"})
		res["estimated_cost"] = cost
		res["network"] = true
		return res, nil, nil
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, nil, failure("unconfigured", "OPENAI_API_KEY environment variable is not set", nil)
	}

	model := r.Model
	if model == "" {
		model = "gpt-4o-mini-tts"
	}
	voice := r.Voice
	if voice == "" {
		voice = "alloy"
	}
	fmtType := r.ResponseFormat
	if fmtType == "" {
		fmtType = r.Format
	}
	if fmtType == "" {
		fmtType = "mp3"
	}
	speed := r.Speed
	if speed <= 0 {
		speed = 1.0
	}

	outPath := r.OutputPath
	if outPath == "" {
		outPath = fmt.Sprintf("openai_tts.%s", fmtType)
	}
	if err := outputPath(outPath, true, false); err != nil {
		return nil, nil, err
	}

	bodyMap := map[string]any{
		"model":           model,
		"voice":           voice,
		"input":           r.Text,
		"response_format": fmtType,
		"speed":           speed,
	}
	if r.Instructions != "" {
		bodyMap["instructions"] = r.Instructions
	}
	reqBody, _ := json.Marshal(bodyMap)

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/audio/speech", bytes.NewReader(reqBody))
	if err != nil {
		return nil, nil, failure("command_failed", "unable to create HTTP request", nil)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	tmo, err := positiveTimeout(r.TimeoutSeconds, 120)
	if err != nil {
		return nil, nil, err
	}
	client := &http.Client{Timeout: tmo}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, failure("command_failed", "OpenAI TTS request failed: "+err.Error(), nil)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errResp, _ := io.ReadAll(resp.Body)
		return nil, nil, failure("command_failed", fmt.Sprintf("OpenAI API error (HTTP %d): %s", resp.StatusCode, string(errResp)), nil)
	}

	outFile, err := os.Create(outPath)
	if err != nil {
		return nil, nil, failure("command_failed", "unable to create output audio file", nil)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return nil, nil, failure("command_failed", "unable to save audio content", nil)
	}

	dur, _ := probeDuration(outPath, 5*time.Second)

	return map[string]any{
		"provider":               "openai",
		"model":                  model,
		"voice":                  voice,
		"format":                 fmtType,
		"text_length":            len(r.Text),
		"audio_duration_seconds": dur,
		"output":                 outPath,
	}, nil, nil
}

func doElevenLabsTTS(op string, data []byte) (any, []string, error) {
	var r elevenLabsTTSRequest
	if err := decode(data, &r); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(r.Text) == "" {
		return nil, nil, failure("invalid_request", "text is required", nil)
	}
	if op == "estimate" {
		cost := roundFloat(float64(len(r.Text))*0.0003, 4)
		res := estimateResult([]string{"elevenlabs_tts_generate"})
		res["estimated_cost"] = cost
		res["network"] = true
		return res, nil, nil
	}

	apiKey := os.Getenv("ELEVENLABS_API_KEY")
	if apiKey == "" {
		return nil, nil, failure("unconfigured", "ELEVENLABS_API_KEY environment variable is not set", nil)
	}

	voiceID := r.VoiceID
	if voiceID == "" {
		voiceID = "21m00Tcm4TlvDq8ikWAM"
	}
	modelID := r.ModelID
	if modelID == "" {
		modelID = "eleven_multilingual_v2"
	}
	outFormat := r.OutputFormat
	if outFormat == "" {
		outFormat = "mp3_44100_128"
	}

	stability := r.Stability
	if stability == 0 {
		stability = 0.5
	}
	similarity := r.SimilarityBoost
	if similarity == 0 {
		similarity = 0.75
	}
	speakerBoost := true
	if r.UseSpeakerBoost != nil {
		speakerBoost = *r.UseSpeakerBoost
	}

	speed := r.Speed
	if speed <= 0 {
		speed = 1.0
	}

	outPath := r.OutputPath
	if outPath == "" {
		ext := "mp3"
		if strings.Contains(outFormat, "pcm") || strings.Contains(outFormat, "wav") {
			ext = "wav"
		}
		outPath = fmt.Sprintf("elevenlabs_tts.%s", ext)
	}
	if err := outputPath(outPath, true, false); err != nil {
		return nil, nil, err
	}

	bodyMap := map[string]any{
		"text":     r.Text,
		"model_id": modelID,
		"voice_settings": map[string]any{
			"stability":         stability,
			"similarity_boost":  similarity,
			"style":             r.Style,
			"speed":             speed,
			"use_speaker_boost": speakerBoost,
		},
	}
	reqBody, _ := json.Marshal(bodyMap)

	urlStr := fmt.Sprintf("https://api.elevenlabs.io/v1/text-to-speech/%s?output_format=%s", voiceID, outFormat)
	req, err := http.NewRequest("POST", urlStr, bytes.NewReader(reqBody))
	if err != nil {
		return nil, nil, failure("command_failed", "unable to create HTTP request", nil)
	}
	req.Header.Set("xi-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")

	tmo, err := positiveTimeout(r.TimeoutSeconds, 120)
	if err != nil {
		return nil, nil, err
	}
	client := &http.Client{Timeout: tmo}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, failure("command_failed", "ElevenLabs TTS request failed: "+err.Error(), nil)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errResp, _ := io.ReadAll(resp.Body)
		return nil, nil, failure("command_failed", fmt.Sprintf("ElevenLabs API error (HTTP %d): %s", resp.StatusCode, string(errResp)), nil)
	}

	outFile, err := os.Create(outPath)
	if err != nil {
		return nil, nil, failure("command_failed", "unable to create output audio file", nil)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return nil, nil, failure("command_failed", "unable to save audio content", nil)
	}

	dur, _ := probeDuration(outPath, 5*time.Second)

	return map[string]any{
		"provider":               "elevenlabs",
		"model":                  modelID,
		"voice_id":               voiceID,
		"format":                 outFormat,
		"text_length":            len(r.Text),
		"audio_duration_seconds": dur,
		"output":                 outPath,
	}, nil, nil
}

func doPiperTTS(op string, data []byte) (any, []string, error) {
	var r piperTTSRequest
	if err := decode(data, &r); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(r.Text) == "" {
		return nil, nil, failure("invalid_request", "text is required", nil)
	}
	if op == "estimate" {
		return estimateResult([]string{"piper_tts_generate"}), nil, nil
	}

	piperPath, err := exec.LookPath("piper")
	if err != nil {
		return nil, nil, failure("unconfigured", "piper binary is not available on PATH", nil)
	}

	model := r.Model
	if model == "" {
		model = "en_US-lessac-medium"
	}
	lengthScale := r.LengthScale
	if lengthScale <= 0 {
		lengthScale = 1.0
	}
	sentenceSilence := r.SentenceSilence
	if sentenceSilence <= 0 {
		sentenceSilence = 0.3
	}

	outPath := r.OutputPath
	if outPath == "" {
		outPath = "piper_tts.wav"
	}
	if err := outputPath(outPath, true, false); err != nil {
		return nil, nil, err
	}

	tmo, err := positiveTimeout(r.TimeoutSeconds, 120)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), tmo)
	defer cancel()

	cmd := exec.CommandContext(ctx, piperPath,
		"--model", model,
		"--speaker", strconv.Itoa(r.SpeakerID),
		"--length-scale", formatFloat(lengthScale),
		"--sentence-silence", formatFloat(sentenceSilence),
		"--output_file", outPath,
	)
	cmd.Stdin = strings.NewReader(r.Text)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if runErr := cmd.Run(); runErr != nil {
		return nil, nil, failure("command_failed", "piper failed: "+runErr.Error(), map[string]any{"stderr": stderr.String()})
	}

	dur, _ := probeDuration(outPath, 5*time.Second)

	return map[string]any{
		"provider":               "piper",
		"model":                  model,
		"speaker_id":             r.SpeakerID,
		"text_length":            len(r.Text),
		"audio_duration_seconds": dur,
		"output":                 outPath,
		"format":                 "wav",
	}, nil, nil
}
