package toolbox

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

type hyperframesRequest struct {
	Operation      string         `json:"operation"`
	WorkspacePath  string         `json:"workspace_path,omitempty"`
	OutputPath     string         `json:"output_path,omitempty"`
	BlockName      string         `json:"block_name,omitempty"`
	EditDecisions  map[string]any `json:"edit_decisions,omitempty"`
	AssetManifest  map[string]any `json:"asset_manifest,omitempty"`
	Playbook       map[string]any `json:"playbook,omitempty"`
	Profile        string         `json:"profile,omitempty"`
	Quality        string         `json:"quality,omitempty"`
	FPS            int            `json:"fps,omitempty"`
	Strict         bool           `json:"strict,omitempty"`
	SkipContrast   bool           `json:"skip_contrast,omitempty"`
	StrictCheck    bool           `json:"strict_check,omitempty"`
	Snapshots      bool           `json:"snapshots,omitempty"`
	TimeoutSeconds int            `json:"timeout_seconds,omitempty"`
}

func doHyperFramesCompose(op string, data []byte) (any, []string, error) {
	var r hyperframesRequest
	if err := decode(data, &r); err != nil {
		return nil, nil, err
	}
	operation := r.Operation
	if operation == "" {
		operation = "render"
	}
	tmo, err := positiveTimeout(r.TimeoutSeconds, 1800)
	if err != nil {
		return nil, nil, err
	}

	if op == "estimate" {
		return estimateResult([]string{"hyperframes_" + operation}), nil, nil
	}

	// Check if npx and ffmpeg are on PATH
	npxPath, npxErr := exec.LookPath("npx")
	ffmpegPath, ffmpegErr := exec.LookPath("ffmpeg")

	if operation == "doctor" {
		return map[string]any{
			"operation": "doctor",
			"runtime_check": map[string]any{
				"runtime_available": npxErr == nil && ffmpegErr == nil,
				"npx_available":     npxErr == nil,
				"npx_path":          npxPath,
				"ffmpeg_available":  ffmpegErr == nil,
				"ffmpeg_path":       ffmpegPath,
			},
		}, nil, nil
	}

	if npxErr != nil || ffmpegErr != nil {
		return nil, nil, failure("dependency_missing", "hyperframes runtime requires npx and ffmpeg on PATH", map[string]any{
			"npx_available":    npxErr == nil,
			"ffmpeg_available": ffmpegErr == nil,
		})
	}

	workspace := r.WorkspacePath
	if workspace == "" {
		workspace = "hyperframes_workspace"
	}

	switch operation {
	case "scaffold_workspace":
		if err := os.MkdirAll(workspace, 0755); err != nil {
			return nil, nil, failure("command_failed", "unable to create workspace directory", nil)
		}
		_ = os.MkdirAll(filepath.Join(workspace, "assets"), 0755)
		_ = os.MkdirAll(filepath.Join(workspace, "compositions"), 0755)
		config := map[string]any{
			"registry": "https://raw.githubusercontent.com/heygen-com/hyperframes/main/registry",
			"paths": map[string]string{
				"blocks":     "compositions",
				"components": "compositions/components",
				"assets":     "assets",
			},
		}
		cfgB, _ := json.MarshalIndent(config, "", "  ")
		_ = os.WriteFile(filepath.Join(workspace, "hyperframes.json"), cfgB, 0644)
		htmlSkeleton := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>HyperFrames Composition</title>
  <script src="https://cdn.jsdelivr.net/npm/gsap@3.14.2/dist/gsap.min.js"></script>
</head>
<body>
  <div data-composition-id="root" data-start="0" data-duration="10" data-width="1920" data-height="1080">
  </div>
</body>
</html>`
		_ = os.WriteFile(filepath.Join(workspace, "index.html"), []byte(htmlSkeleton), 0644)
		cutCount := 0
		if cuts, ok := r.EditDecisions["cuts"].([]any); ok {
			cutCount = len(cuts)
		}
		return map[string]any{
			"operation": "scaffold_workspace",
			"workspace": workspace,
			"cut_count": cutCount,
		}, nil, nil

	case "lint", "validate", "inspect", "check":
		args := []string{"hyperframes", operation, "--json"}
		if r.SkipContrast {
			args = append(args, "--no-contrast")
		}
		out, err := runCommandDir(tmo, workspace, "npx", args...)
		if err != nil {
			return nil, nil, failure("command_failed", "hyperframes "+operation+" failed: "+err.Error(), map[string]any{"output": string(out)})
		}
		var parsed any
		if json.Unmarshal(out, &parsed) == nil {
			return parsed, nil, nil
		}
		return map[string]any{
			"operation": operation,
			"raw":       string(out),
		}, nil, nil

	case "add_block":
		if r.BlockName == "" {
			return nil, nil, failure("invalid_request", "block_name is required for add_block", nil)
		}
		args := []string{"hyperframes", "add", r.BlockName, "--json", "--no-clipboard"}
		out, err := runCommandDir(tmo, workspace, "npx", args...)
		if err != nil {
			return nil, nil, failure("command_failed", "hyperframes add failed: "+err.Error(), map[string]any{"output": string(out)})
		}
		return map[string]any{
			"operation":  "add_block",
			"block_name": r.BlockName,
			"output":     string(out),
		}, nil, nil

	case "render", "render_existing":
		outPath := r.OutputPath
		if outPath == "" {
			outPath = filepath.Join(workspace, "renders", "final.mp4")
		}
		_ = os.MkdirAll(filepath.Dir(outPath), 0755)
		fps := r.FPS
		if fps <= 0 {
			fps = 30
		}
		quality := r.Quality
		if quality == "" {
			quality = "standard"
		}
		absOut, _ := filepath.Abs(outPath)
		args := []string{"hyperframes", "render", "--output", absOut, "--fps", strconv.Itoa(fps), "--quality", quality}
		out, err := runCommandDir(tmo, workspace, "npx", args...)
		if err != nil {
			return nil, nil, failure("command_failed", "hyperframes render failed: "+err.Error(), map[string]any{"output": string(out)})
		}
		return map[string]any{
			"operation": operation,
			"output":    outPath,
			"workspace": workspace,
		}, nil, nil

	default:
		return nil, nil, failure("invalid_request", "unknown operation: "+operation, nil)
	}
}
