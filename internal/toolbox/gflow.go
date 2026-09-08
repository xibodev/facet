package toolbox

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
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
	if r.Model == "" {
		r.Model = "veo-3.1"
	}
	if r.Duration == 0 {
		r.Duration = 6
	}
	if r.AspectRatio == "" {
		r.AspectRatio = "landscape"
	}
	if r.Resolution == "" {
		r.Resolution = "1080p"
	}
	if r.Model != "veo-3.1" || !slices.Contains([]float64{4, 6, 8, 10}, r.Duration) ||
		!slices.Contains([]string{"landscape", "portrait", "square"}, r.AspectRatio) ||
		!slices.Contains([]string{"720p", "1080p", "4k"}, r.Resolution) {
		return nil, nil, failure("invalid_request", "gflow video requires model veo-3.1, duration 4/6/8/10, aspect landscape/portrait/square, and resolution 720p/1080p/4k", nil)
	}
	timeout, err := cloudTimeout(r.TimeoutSeconds, 300)
	if err != nil {
		return nil, nil, err
	}
	if op == "estimate" {
		return gflowEstimate("gflow_video_generate", r.Mock), nil, nil
	}
	if r.OutputPath == "" {
		r.OutputPath = "gflow_video.mp4"
	}
	res := map[string]any{"provider": "google_flow", "model": r.Model, "prompt": r.Prompt,
		"duration": r.Duration, "aspect_ratio": r.AspectRatio, "resolution": r.Resolution,
		"output": r.OutputPath, "mock": r.Mock}
	if r.Mock {
		if err := gflowSafeOutput(r.OutputPath); err != nil {
			return nil, nil, err
		}
		if err := createMockVideo(r.OutputPath, 1920, 1080, r.Duration); err != nil {
			return nil, nil, failure("command_failed", "failed to create mock video", nil)
		}
		return res, nil, nil
	}
	args := []string{"video", "-d", fmt.Sprint(int(r.Duration)), "-a", r.AspectRatio, "-r", r.Resolution}
	if r.StartFrame != "" {
		args = append(args, "--start", r.StartFrame)
	}
	if r.EndFrame != "" {
		args = append(args, "--end", r.EndFrame)
	}
	outputs, err := generateGFlow(args, r.Prompt, "video", r.OutputPath, 1, timeout)
	if err != nil {
		return nil, nil, err
	}
	res["outputs"] = outputs
	return res, nil, nil
}

func doGFlowImage(op string, data []byte) (any, []string, error) {
	var r gflowImageRequest
	if err := decode(data, &r); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(r.Prompt) == "" {
		return nil, nil, failure("invalid_request", "prompt is required", nil)
	}
	if r.Model == "" {
		r.Model = "narwhal"
	}
	if r.AspectRatio == "" {
		r.AspectRatio = "landscape"
	}
	if r.Count == 0 {
		r.Count = 1
	}
	if !slices.Contains([]string{"narwhal", "harbor_seal", "gem_pix_2"}, r.Model) ||
		!slices.Contains([]string{"landscape", "portrait", "square", "4:3", "3:4"}, r.AspectRatio) || r.Count < 1 || r.Count > 4 {
		return nil, nil, failure("invalid_request", "gflow image requires model narwhal/harbor_seal/gem_pix_2, aspect landscape/portrait/square/4:3/3:4, and count 1-4", nil)
	}
	timeout, err := cloudTimeout(r.TimeoutSeconds, 180)
	if err != nil {
		return nil, nil, err
	}
	if op == "estimate" {
		return gflowEstimate("gflow_image_generate", r.Mock), nil, nil
	}
	if r.OutputPath == "" {
		r.OutputPath = "gflow_image.png"
	}
	res := map[string]any{"provider": "google_flow", "model": r.Model, "prompt": r.Prompt,
		"aspect_ratio": r.AspectRatio, "output": r.OutputPath, "mock": r.Mock}
	if r.Mock {
		if err := gflowSafeOutput(r.OutputPath); err != nil {
			return nil, nil, err
		}
		if err := createMockPNG(r.OutputPath, 1920, 1080, "Google Flow: "+r.Prompt); err != nil {
			return nil, nil, failure("command_failed", "failed to create mock image", nil)
		}
		return res, nil, nil
	}
	args := []string{"image", "-a", r.AspectRatio, "-m", r.Model, "-c", fmt.Sprint(r.Count)}
	if r.ReferenceImage != "" {
		args = append(args, "--ref", r.ReferenceImage)
	}
	outputs, err := generateGFlow(args, r.Prompt, "image", r.OutputPath, r.Count, timeout)
	if err != nil {
		return nil, nil, err
	}
	res["outputs"] = outputs
	return res, nil, nil
}

func gflowEstimate(operation string, mock bool) map[string]any {
	res := estimateResult([]string{operation})
	res["provider"] = "google_flow"
	res["network"] = !mock
	res["mock"] = mock
	res["cost_known"] = mock
	if !mock {
		res["estimated_cost"] = nil
	}
	return res
}

// Guard multiplication as well as negative values before filesystem/network work.
func cloudTimeout(seconds, fallback int) (time.Duration, error) {
	if seconds < 0 || int64(seconds) > int64((1<<63-1)/time.Second) {
		return 0, failure("invalid_request", "timeout_seconds is out of range", nil)
	}
	return positiveTimeout(seconds, fallback)
}

type gflowMedia struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	URL       string `json:"url"`
	LocalPath string `json:"local_path"`
	MIMEType  string `json:"mime_type"`
}

type gflowOutput struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	MIMEType   string `json:"mime_type"`
	Output     string `json:"output"`
	SourceFile string `json:"source_file"`
	SHA256     string `json:"sha256"`
}

const gflowStdoutLimit = 1 << 20

type gflowStdout struct {
	data     bytes.Buffer
	overflow bool
}

func (b *gflowStdout) Write(p []byte) (int, error) {
	n := len(p)
	remaining := gflowStdoutLimit - b.data.Len()
	if n > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	_, _ = b.data.Write(p)
	// Drain excess output without retaining it or disrupting a possibly paid download.
	return n, nil
}

func gflowSafeOutput(path string) error {
	if strings.TrimSpace(path) == "" {
		return failure("invalid_request", "output path is required", nil)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return failure("invalid_request", "output path cannot be resolved", nil)
	}
	for p := filepath.Clean(path); ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil && !os.IsNotExist(err) {
			return failure("invalid_request", "output path cannot be inspected", nil)
		}
		if err == nil && (info.Mode()&os.ModeSymlink != 0 || (p == filepath.Clean(path) && !info.Mode().IsRegular()) || (p != filepath.Clean(path) && !info.IsDir())) {
			return failure("invalid_request", "output must be a regular file without symlink ancestors", nil)
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return nil
}

func generateGFlow(args []string, prompt, kind, output string, count int, timeout time.Duration) (_ []gflowOutput, resultErr error) {
	targets := make([]string, count)
	for i := range targets {
		targets[i] = output
		if i > 0 {
			ext := filepath.Ext(output)
			targets[i] = strings.TrimSuffix(output, ext) + fmt.Sprintf("-%d", i+1) + ext
		}
		if err := gflowSafeOutput(targets[i]); err != nil {
			return nil, err
		}
	}
	bin, err := lookPath("gflow")
	if err != nil {
		return nil, failure("dependency_missing", "gflow CLI is required; install and authenticate it or explicitly set mock=true", nil)
	}
	if err := outputPath(output, true, false); err != nil {
		return nil, err
	}
	parent, err := filepath.Abs(filepath.Dir(output))
	if err != nil {
		return nil, failure("invalid_request", "invalid output directory", nil)
	}
	stage, err := os.MkdirTemp(parent, ".gflow-*")
	if err != nil {
		return nil, failure("command_failed", "could not create gflow staging directory", nil)
	}
	quarantine := false
	defer func() {
		if !quarantine {
			_ = os.RemoveAll(stage)
			return
		}
		// A failed or timed-out CLI may already have downloaded paid media.
		var toolErr *toolFailure
		if errors.As(resultErr, &toolErr) {
			toolErr.err.Details["recovery_path"] = stage
			toolErr.err.Details["quarantined"] = true
		}
	}()
	// Resolve both sides of containment, including Windows short-name ancestors.
	canonicalStage, err := filepath.EvalSymlinks(stage)
	if err != nil {
		return nil, failure("command_failed", "could not resolve gflow staging directory", nil)
	}
	root, err := os.OpenRoot(stage)
	if err != nil {
		return nil, failure("command_failed", "could not open gflow staging directory", nil)
	}
	defer root.Close()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// -o is a directory in the public CLI. Never retry through a second transport.
	args = append(args, "-o", stage, "--json", "--", prompt)
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.WaitDelay = time.Second
	var stdout gflowStdout
	cmd.Stdout = &stdout
	// Provider diagnostics may contain credentials or signed URLs.
	cmd.Stderr = io.Discard
	quarantine = true
	runErr := cmd.Run()
	if stdout.overflow {
		return nil, failure("provider_response_invalid", "gflow stdout exceeded the 1 MiB limit; no retry was attempted", nil)
	}
	if runErr != nil {
		if ctx.Err() != nil {
			return nil, failure("command_timeout", "gflow timed out; generation may still be running; no retry was attempted", nil)
		}
		if errors.Is(runErr, exec.ErrWaitDelay) {
			return nil, failure("command_timeout", "gflow output pipes did not close after CLI exit; no retry was attempted", nil)
		}
		return nil, failure("command_failed", "gflow CLI failed; check provider authentication and availability; no retry was attempted", nil)
	}
	var media []gflowMedia
	// gflow v1.0.0 prints polling dots on stdout during video upscaling, even --json.
	// Strip only that known prefix; Unmarshal still rejects prose and trailing JSON.
	response := bytes.TrimLeft(stdout.data.Bytes(), ". \t\r\n")
	if err := json.Unmarshal(response, &media); err != nil || len(media) != count {
		return nil, failure("provider_response_invalid", "gflow must return a JSON media array matching the requested count; no retry was attempted", nil)
	}
	outputs := make([]gflowOutput, count)
	seen := make(map[string]bool, count)
	// Validate every artifact before replacing any requested output.
	for i, item := range media {
		if item.ID == "" || item.Type != kind || !strings.HasPrefix(item.MIMEType, kind+"/") || item.LocalPath == "" || seen[item.ID] {
			return nil, failure("provider_response_invalid", "gflow returned incomplete or inconsistent media metadata", nil)
		}
		seen[item.ID] = true
		source := item.LocalPath
		if !filepath.IsAbs(source) {
			source = filepath.Join(stage, source)
		}
		resolved, err := filepath.EvalSymlinks(source)
		if err != nil {
			return nil, failure("provider_response_invalid", "gflow returned an inaccessible local_path", nil)
		}
		rel, err := filepath.Rel(canonicalStage, resolved)
		if err != nil || !filepath.IsLocal(rel) {
			return nil, failure("provider_response_invalid", "gflow local_path escapes its staging directory", nil)
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return nil, failure("provider_response_invalid", "gflow local_path must name a nonempty regular file", nil)
		}
		media[i].LocalPath = rel
		outputs[i] = gflowOutput{ID: item.ID, Type: item.Type, MIMEType: item.MIMEType, Output: targets[i], SourceFile: filepath.ToSlash(rel)}
	}
	copies := make([]string, count)
	for i, item := range media {
		source, err := root.Open(item.LocalPath)
		if err != nil {
			return nil, failure("command_failed", "could not open gflow artifact", nil)
		}
		temp, err := os.CreateTemp(parent, ".gflow-copy-*")
		if err != nil {
			source.Close()
			return nil, failure("command_failed", "could not stage gflow artifact copy", nil)
		}
		defer os.Remove(temp.Name())
		copies[i] = temp.Name()
		hash := sha256.New()
		_, copyErr := io.Copy(io.MultiWriter(temp, hash), source)
		source.Close()
		closeErr := temp.Close()
		if copyErr != nil || closeErr != nil {
			return nil, failure("command_failed", "could not copy gflow artifact", nil)
		}
		if err := gflowSafeOutput(targets[i]); err != nil {
			return nil, err
		}
		outputs[i].SHA256 = hex.EncodeToString(hash.Sum(nil))
	}
	if err := publishFileSet(copies, targets, true); err != nil {
		return nil, err
	}
	quarantine = false
	return outputs, nil
}
