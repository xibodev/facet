package toolbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
)

type gflowVideoRequest struct {
	Prompt         string  `json:"prompt"`
	Model          string  `json:"model,omitempty"`
	Duration       float64 `json:"duration,omitempty"`
	AspectRatio    string  `json:"aspect_ratio,omitempty"`
	Resolution     string  `json:"resolution,omitempty"`
	StartFrame     string  `json:"start_frame,omitempty"`
	EndFrame       string  `json:"end_frame,omitempty"`
	OutputPath     string  `json:"output_path,omitempty"`
	Mock           bool    `json:"mock,omitempty"`
	TimeoutSeconds int     `json:"timeout_seconds,omitempty"`
}

type gflowImageRequest struct {
	Prompt         string `json:"prompt"`
	Model          string `json:"model,omitempty"`
	AspectRatio    string `json:"aspect_ratio,omitempty"`
	Count          int    `json:"count,omitempty"`
	ReferenceImage string `json:"reference_image,omitempty"`
	OutputPath     string `json:"output_path,omitempty"`
	Mock           bool   `json:"mock,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func doGFlowVideo(op string, data []byte) (any, []string, error) {
	var r gflowVideoRequest
	if err := decode(data, &r); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(r.Prompt) == "" {
		return nil, nil, failure("invalid_request", "prompt is required", nil)
	}

	model := r.Model
	if model == "" {
		model = "veo-3.1"
	}
	duration := r.Duration
	if duration <= 0 {
		duration = 6.0
	}
	aspectRatio := r.AspectRatio
	if aspectRatio == "" {
		aspectRatio = "landscape"
	}
	resolution := r.Resolution
	if resolution == "" {
		resolution = "1080p"
	}

	if op == "estimate" {
		res := estimateResult([]string{"gflow_video_generate"})
		res["estimated_cost"] = 0.0
		res["network"] = true
		res["provider"] = "google_flow"
		return res, nil, nil
	}

	outPath := r.OutputPath
	if outPath == "" {
		outPath = "gflow_video.mp4"
	}
	if err := outputPath(outPath, true, false); err != nil {
		return nil, nil, err
	}

	timeout, err := positiveTimeout(r.TimeoutSeconds, 300)
	if err != nil {
		return nil, nil, err
	}

	// 1. Check if gflow CLI executable exists
	if gflowBin, err := exec.LookPath("gflow"); err == nil && !r.Mock {
		durArg := fmt.Sprintf("%d", int(duration))
		args := []string{"video", r.Prompt, "-d", durArg, "-a", aspectRatio, "-r", resolution, "-o", outPath, "--json"}
		if r.StartFrame != "" {
			args = append(args, "--start", r.StartFrame)
		}
		if r.EndFrame != "" {
			args = append(args, "--end", r.EndFrame)
		}
		if _, err := runCommand(timeout, gflowBin, args...); err == nil && fileExists(outPath) {
			return map[string]any{
				"provider":     "google_flow",
				"model":        model,
				"prompt":       r.Prompt,
				"duration":     duration,
				"aspect_ratio": aspectRatio,
				"resolution":   resolution,
				"output":       outPath,
				"mock":         false,
			}, nil, nil
		}
	}

	// 2. Check if local gflow server is running at http://127.0.0.1:8001
	if !r.Mock {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		payload := map[string]any{
			"prompt":   r.Prompt,
			"duration": int(duration),
			"aspect":   aspectRatio,
		}
		payloadBytes, _ := json.Marshal(payload)
		httpReq, reqErr := http.NewRequestWithContext(ctx, "POST", "http://127.0.0.1:8001/v1/videos/generations", bytes.NewReader(payloadBytes))
		if reqErr == nil {
			httpReq.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: timeout}
			if resp, err := client.Do(httpReq); err == nil && (resp.StatusCode == 200 || resp.StatusCode == 201) {
				defer resp.Body.Close()
				body, _ := io.ReadAll(resp.Body)
				var soraLikeResp struct {
					VideoURL string `json:"video_url"`
					Data     []struct {
						URL string `json:"url"`
					} `json:"data"`
				}
				_ = json.Unmarshal(body, &soraLikeResp)
				vURL := soraLikeResp.VideoURL
				if vURL == "" && len(soraLikeResp.Data) > 0 {
					vURL = soraLikeResp.Data[0].URL
				}
				if vURL != "" {
					if err := downloadHTTPFile(ctx, vURL, outPath); err == nil {
						return map[string]any{
							"provider":     "google_flow",
							"model":        model,
							"prompt":       r.Prompt,
							"duration":     duration,
							"aspect_ratio": aspectRatio,
							"resolution":   resolution,
							"output":       outPath,
							"mock":         false,
						}, nil, nil
					}
				}
			}
		}
	}

	// 3. Fallback to mock video
	if err := createMockVideo(outPath, 1920, 1080, duration); err != nil {
		return nil, nil, failure("command_failed", "failed to create video: "+err.Error(), nil)
	}

	return map[string]any{
		"provider":     "google_flow",
		"model":        model,
		"prompt":       r.Prompt,
		"duration":     duration,
		"aspect_ratio": aspectRatio,
		"resolution":   resolution,
		"output":       outPath,
		"mock":         true,
	}, nil, nil
}

func doGFlowImage(op string, data []byte) (any, []string, error) {
	var r gflowImageRequest
	if err := decode(data, &r); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(r.Prompt) == "" {
		return nil, nil, failure("invalid_request", "prompt is required", nil)
	}

	model := r.Model
	if model == "" {
		model = "narwhal"
	}
	aspectRatio := r.AspectRatio
	if aspectRatio == "" {
		aspectRatio = "landscape"
	}
	count := r.Count
	if count <= 0 {
		count = 1
	}

	if op == "estimate" {
		res := estimateResult([]string{"gflow_image_generate"})
		res["estimated_cost"] = 0.0
		res["network"] = true
		res["provider"] = "google_flow"
		return res, nil, nil
	}

	outPath := r.OutputPath
	if outPath == "" {
		outPath = "gflow_image.png"
	}
	if err := outputPath(outPath, true, false); err != nil {
		return nil, nil, err
	}

	timeout, err := positiveTimeout(r.TimeoutSeconds, 180)
	if err != nil {
		return nil, nil, err
	}

	// 1. Check if gflow CLI executable exists
	if gflowBin, err := exec.LookPath("gflow"); err == nil && !r.Mock {
		args := []string{"image", r.Prompt, "-a", aspectRatio, "-m", model, "-c", fmt.Sprintf("%d", count), "-o", outPath, "--json"}
		if r.ReferenceImage != "" {
			args = append(args, "--ref", r.ReferenceImage)
		}
		if _, err := runCommand(timeout, gflowBin, args...); err == nil && fileExists(outPath) {
			return map[string]any{
				"provider":     "google_flow",
				"model":        model,
				"prompt":       r.Prompt,
				"aspect_ratio": aspectRatio,
				"output":       outPath,
				"mock":         false,
			}, nil, nil
		}
	}

	// 2. Check if local gflow server is running at http://127.0.0.1:8001
	if !r.Mock {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		payload := map[string]any{
			"prompt": r.Prompt,
			"n":      count,
			"size":   "1024x1024",
		}
		payloadBytes, _ := json.Marshal(payload)
		httpReq, reqErr := http.NewRequestWithContext(ctx, "POST", "http://127.0.0.1:8001/v1/images/generations", bytes.NewReader(payloadBytes))
		if reqErr == nil {
			httpReq.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: timeout}
			if resp, err := client.Do(httpReq); err == nil && resp.StatusCode == 200 {
				defer resp.Body.Close()
				body, _ := io.ReadAll(resp.Body)
				var openAIResp struct {
					Data []struct {
						URL     string `json:"url"`
						B64JSON string `json:"b64_json"`
					} `json:"data"`
				}
				_ = json.Unmarshal(body, &openAIResp)
				if len(openAIResp.Data) > 0 && openAIResp.Data[0].URL != "" {
					if err := downloadHTTPFile(ctx, openAIResp.Data[0].URL, outPath); err == nil {
						return map[string]any{
							"provider":     "google_flow",
							"model":        model,
							"prompt":       r.Prompt,
							"aspect_ratio": aspectRatio,
							"output":       outPath,
							"mock":         false,
						}, nil, nil
					}
				}
			}
		}
	}

	// 3. Fallback to mock image
	if err := createMockPNG(outPath, 1920, 1080, "Google Flow: "+r.Prompt); err != nil {
		return nil, nil, failure("command_failed", "failed to create mock image: "+err.Error(), nil)
	}

	return map[string]any{
		"provider":     "google_flow",
		"model":        model,
		"prompt":       r.Prompt,
		"aspect_ratio": aspectRatio,
		"output":       outPath,
		"mock":         true,
	}, nil, nil
}
