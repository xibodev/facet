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
	"path/filepath"
	"strings"
	"time"
)

type klingVideoRequest struct {
	Prompt         string  `json:"prompt"`
	Model          string  `json:"model,omitempty"`
	Duration       float64 `json:"duration,omitempty"`
	AspectRatio    string  `json:"aspect_ratio,omitempty"`
	Mode           string  `json:"mode,omitempty"`
	ImageURL       string  `json:"image_url,omitempty"`
	OutputPath     string  `json:"output_path,omitempty"`
	Mock           bool    `json:"mock,omitempty"`
	TimeoutSeconds int     `json:"timeout_seconds,omitempty"`
}

type soraVideoRequest struct {
	Prompt         string  `json:"prompt"`
	Model          string  `json:"model,omitempty"`
	Duration       float64 `json:"duration,omitempty"`
	AspectRatio    string  `json:"aspect_ratio,omitempty"`
	Resolution     string  `json:"resolution,omitempty"`
	OutputPath     string  `json:"output_path,omitempty"`
	Mock           bool    `json:"mock,omitempty"`
	TimeoutSeconds int     `json:"timeout_seconds,omitempty"`
}

func doKlingVideo(op string, data []byte) (any, []string, error) {
	var r klingVideoRequest
	if err := decode(data, &r); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(r.Prompt) == "" {
		return nil, nil, failure("invalid_request", "prompt is required", nil)
	}

	model := r.Model
	if model == "" {
		model = "fal-ai/kling-video"
	}

	duration := r.Duration
	if duration <= 0 {
		duration = 5.0
	}

	aspectRatio := r.AspectRatio
	if aspectRatio == "" {
		aspectRatio = "16:9"
	}

	mode := r.Mode
	if mode == "" {
		mode = "std"
	}

	cost := 0.10
	if duration > 5 {
		cost = 0.20
	}
	if mode == "pro" {
		cost *= 2.0
	}

	if op == "estimate" {
		res := estimateResult([]string{"kling_video_generate"})
		res["estimated_cost"] = cost
		res["network"] = true
		return res, nil, nil
	}

	outPath := r.OutputPath
	if outPath == "" {
		outPath = "kling_video.mp4"
	}
	if err := outputPath(outPath, true, false); err != nil {
		return nil, nil, err
	}

	apiKey := os.Getenv("FAL_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("KLING_API_KEY")
	}

	if apiKey == "" || r.Mock {
		// Mock contract when API key missing or mock requested
		if err := createMockVideo(outPath, 640, 360, duration); err != nil {
			return nil, nil, failure("command_failed", "failed to create mock video: "+err.Error(), nil)
		}
		return map[string]any{
			"provider":     "kling",
			"model":        model,
			"prompt":       r.Prompt,
			"duration":     duration,
			"aspect_ratio": aspectRatio,
			"mode":         mode,
			"output":       outPath,
			"mock":         true,
			"video_url":    "mock://fal/kling/" + filepath.Base(outPath),
		}, nil, nil
	}

	timeout, err := positiveTimeout(r.TimeoutSeconds, 300)
	if err != nil {
		return nil, nil, err
	}

	queueBase := os.Getenv("FAL_QUEUE_BASE_URL")
	if queueBase == "" {
		queueBase = "https://queue.fal.run"
	}
	endpoint := queueBase + "/fal-ai/kling-video/v1/standard/text-to-video"
	if mode == "pro" {
		endpoint = queueBase + "/fal-ai/kling-video/v1/pro/text-to-video"
	}
	if r.ImageURL != "" {
		endpoint = queueBase + "/fal-ai/kling-video/v1/standard/image-to-video"
		if mode == "pro" {
			endpoint = queueBase + "/fal-ai/kling-video/v1/pro/image-to-video"
		}
	}

	payload := map[string]any{
		"prompt":       r.Prompt,
		"duration":     fmt.Sprintf("%d", int(duration)),
		"aspect_ratio": aspectRatio,
	}
	if r.ImageURL != "" {
		payload["image_url"] = r.ImageURL
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, failure("command_failed", "failed to serialize Kling video request: "+err.Error(), nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, nil, failure("command_failed", "failed to create Kling request: "+err.Error(), nil)
	}
	httpReq.Header.Set("Authorization", "Key "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, nil, failure("command_failed", "Kling request failed: "+err.Error(), nil)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, failure("command_failed", "failed to read Kling response: "+err.Error(), nil)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, nil, failure("command_failed", fmt.Sprintf("Kling API error (HTTP %d): %s", resp.StatusCode, bounded(string(body))), nil)
	}

	var queueResp struct {
		RequestID   string `json:"request_id"`
		StatusURL   string `json:"status_url"`
		ResponseURL string `json:"response_url"`
		Video       struct {
			URL string `json:"url"`
		} `json:"video"`
	}
	if err := json.Unmarshal(body, &queueResp); err != nil {
		return nil, nil, failure("command_failed", "failed to parse Kling queue response: "+err.Error(), nil)
	}

	videoURL := queueResp.Video.URL
	if videoURL == "" && (queueResp.StatusURL != "" || queueResp.ResponseURL != "") {
		// Poll fal queue
		statusURL := queueResp.StatusURL
		if statusURL == "" {
			statusURL = fmt.Sprintf("https://queue.fal.run/fal-ai/kling-video/requests/%s/status", queueResp.RequestID)
		}
		responseURL := queueResp.ResponseURL
		if responseURL == "" {
			responseURL = fmt.Sprintf("https://queue.fal.run/fal-ai/kling-video/requests/%s", queueResp.RequestID)
		}

		pollTicker := time.NewTicker(3 * time.Second)
		defer pollTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return nil, nil, failure("command_timeout", "Kling video generation timed out while polling", nil)
			case <-pollTicker.C:
				statusReq, _ := http.NewRequestWithContext(ctx, "GET", statusURL, nil)
				statusReq.Header.Set("Authorization", "Key "+apiKey)
				sResp, sErr := client.Do(statusReq)
				if sErr != nil {
					continue
				}
				sBody, _ := io.ReadAll(sResp.Body)
				sResp.Body.Close()

				var statusResult struct {
					Status string `json:"status"`
				}
				_ = json.Unmarshal(sBody, &statusResult)
				if statusResult.Status == "COMPLETED" {
					// Fetch final result
					resReq, _ := http.NewRequestWithContext(ctx, "GET", responseURL, nil)
					resReq.Header.Set("Authorization", "Key "+apiKey)
					rResp, rErr := client.Do(resReq)
					if rErr == nil {
						rBody, _ := io.ReadAll(rResp.Body)
						rResp.Body.Close()
						var finalResult struct {
							Video struct {
								URL string `json:"url"`
							} `json:"video"`
						}
						_ = json.Unmarshal(rBody, &finalResult)
						videoURL = finalResult.Video.URL
					}
					goto DownloadVideo
				} else if statusResult.Status == "FAILED" {
					return nil, nil, failure("command_failed", "Kling video generation failed on server", nil)
				}
			}
		}
	}

DownloadVideo:
	if videoURL == "" {
		return nil, nil, failure("command_failed", "no video URL received from Kling", nil)
	}

	if err := downloadHTTPFile(ctx, videoURL, outPath); err != nil {
		return nil, nil, failure("command_failed", "failed to download Kling video: "+err.Error(), nil)
	}

	return map[string]any{
		"provider":     "kling",
		"model":        model,
		"prompt":       r.Prompt,
		"duration":     duration,
		"aspect_ratio": aspectRatio,
		"mode":         mode,
		"output":       outPath,
		"mock":         false,
		"video_url":    videoURL,
	}, nil, nil
}

func doSoraVideo(op string, data []byte) (any, []string, error) {
	var r soraVideoRequest
	if err := decode(data, &r); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(r.Prompt) == "" {
		return nil, nil, failure("invalid_request", "prompt is required", nil)
	}

	model := r.Model
	if model == "" {
		model = "sora-2"
	}

	duration := r.Duration
	if duration <= 0 {
		duration = 5.0
	}

	aspectRatio := r.AspectRatio
	if aspectRatio == "" {
		aspectRatio = "16:9"
	}

	resolution := r.Resolution
	if resolution == "" {
		resolution = "720p"
	}

	cost := 0.20
	if duration > 5 {
		cost = 0.40
	}
	if resolution == "1080p" {
		cost += 0.20
	}

	if op == "estimate" {
		res := estimateResult([]string{"sora_video_generate"})
		res["estimated_cost"] = cost
		res["network"] = true
		return res, nil, nil
	}

	outPath := r.OutputPath
	if outPath == "" {
		outPath = "sora_video.mp4"
	}
	if err := outputPath(outPath, true, false); err != nil {
		return nil, nil, err
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" || r.Mock {
		// Mock contract when API key is missing or mock explicitly requested
		if err := createMockVideo(outPath, 1280, 720, duration); err != nil {
			return nil, nil, failure("command_failed", "failed to create mock video: "+err.Error(), nil)
		}
		return map[string]any{
			"provider":     "sora",
			"model":        model,
			"prompt":       r.Prompt,
			"duration":     duration,
			"aspect_ratio": aspectRatio,
			"resolution":   resolution,
			"output":       outPath,
			"mock":         true,
			"video_url":    "mock://openai/sora/" + filepath.Base(outPath),
		}, nil, nil
	}

	timeout, err := positiveTimeout(r.TimeoutSeconds, 300)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}

	payload := map[string]any{
		"model":        model,
		"prompt":       r.Prompt,
		"duration":     duration,
		"aspect_ratio": aspectRatio,
		"resolution":   resolution,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, failure("command_failed", "failed to serialize Sora request: "+err.Error(), nil)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/v1/videos/generations", bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, nil, failure("command_failed", "failed to create Sora request: "+err.Error(), nil)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, nil, failure("command_failed", "Sora request failed: "+err.Error(), nil)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, failure("command_failed", "failed to read Sora response: "+err.Error(), nil)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, nil, failure("command_failed", fmt.Sprintf("OpenAI Sora API error (HTTP %d): %s", resp.StatusCode, bounded(string(body))), nil)
	}

	var soraResp struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		VideoURL string `json:"video_url"`
		Data     []struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	_ = json.Unmarshal(body, &soraResp)

	videoURL := soraResp.VideoURL
	if videoURL == "" && len(soraResp.Data) > 0 {
		videoURL = soraResp.Data[0].URL
	}

	// If async task polling needed
	if videoURL == "" && soraResp.ID != "" {
		pollTicker := time.NewTicker(3 * time.Second)
		defer pollTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return nil, nil, failure("command_timeout", "Sora video generation timed out while polling", nil)
			case <-pollTicker.C:
				statusReq, _ := http.NewRequestWithContext(ctx, "GET", "https://api.openai.com/v1/videos/generations/"+soraResp.ID, nil)
				statusReq.Header.Set("Authorization", "Bearer "+apiKey)
				sResp, sErr := client.Do(statusReq)
				if sErr != nil {
					continue
				}
				sBody, _ := io.ReadAll(sResp.Body)
				sResp.Body.Close()

				var pollResult struct {
					Status   string `json:"status"`
					VideoURL string `json:"video_url"`
					Data     []struct {
						URL string `json:"url"`
					} `json:"data"`
				}
				_ = json.Unmarshal(sBody, &pollResult)
				if pollResult.Status == "completed" || pollResult.Status == "succeeded" {
					videoURL = pollResult.VideoURL
					if videoURL == "" && len(pollResult.Data) > 0 {
						videoURL = pollResult.Data[0].URL
					}
					goto DownloadSoraVideo
				} else if pollResult.Status == "failed" {
					return nil, nil, failure("command_failed", "Sora video generation failed on server", nil)
				}
			}
		}
	}

DownloadSoraVideo:
	if videoURL == "" {
		return nil, nil, failure("command_failed", "no video URL received from Sora", nil)
	}

	if err := downloadHTTPFile(ctx, videoURL, outPath); err != nil {
		return nil, nil, failure("command_failed", "failed to download Sora video: "+err.Error(), nil)
	}

	return map[string]any{
		"provider":     "sora",
		"model":        model,
		"prompt":       r.Prompt,
		"duration":     duration,
		"aspect_ratio": aspectRatio,
		"resolution":   resolution,
		"output":       outPath,
		"mock":         false,
		"video_url":    videoURL,
	}, nil, nil
}

func createMockVideo(path string, width, height int, duration float64) error {
	if width <= 0 {
		width = 640
	}
	if height <= 0 {
		height = 360
	}
	if duration <= 0 {
		duration = 1.0
	}
	parent := filepath.Dir(path)
	if parent != "." {
		if err := os.MkdirAll(parent, 0755); err != nil {
			return err
		}
	}

	if _, err := exec.LookPath("ffmpeg"); err == nil {
		durStr := fmt.Sprintf("%.2f", duration)
		sizeStr := fmt.Sprintf("%dx%d", width, height)
		args := []string{
			"-hide_banner", "-loglevel", "error", "-y",
			"-f", "lavfi", "-i", fmt.Sprintf("color=c=navy:size=%s:rate=24:duration=%s", sizeStr, durStr),
			"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=440:sample_rate=48000:duration=%s", durStr),
			"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest",
			path,
		}
		if _, err := runCommand(30*time.Second, "ffmpeg", args...); err == nil {
			return nil
		}
	}

	return os.WriteFile(path, []byte("mock video payload for "+path), 0644)
}
