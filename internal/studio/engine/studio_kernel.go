package engine

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// CompaKernelAdapter implements EngineAdapter for the external Compa kernel.
// This is the standalone composition adapter: Facet UI operates the separately
// installed Compa kernel process.
type CompaKernelAdapter struct {
	Executable string
}

// NewCompaKernelAdapter constructs a new CompaKernelAdapter.
func NewCompaKernelAdapter() *CompaKernelAdapter {
	return &CompaKernelAdapter{}
}

func (a *CompaKernelAdapter) Name() string {
	return "studio"
}

func (a *CompaKernelAdapter) DisplayName() string {
	return "Compa Kernel"
}

func (a *CompaKernelAdapter) ExecutableName() string {
	if a.Executable != "" {
		return a.Executable
	}
	if p := os.Getenv("FACET_COMPA_KERNEL"); p != "" {
		return p
	}

	exeName := "compa-kernel"
	fallbackName := "compa"
	if runtime.GOOS == "windows" {
		exeName += ".exe"
		fallbackName += ".exe"
	}

	// Check beside current executable
	if self, err := os.Executable(); err == nil {
		dir := filepath.Dir(self)
		cand := filepath.Join(dir, exeName)
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			return cand
		}
		cand = filepath.Join(dir, fallbackName)
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			return cand
		}
	}

	// Check shared dependency install location (%LOCALAPPDATA%\FacetCollection\dependencies\kernel\...)
	if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
		depPath := filepath.Join(localApp, "FacetCollection", "dependencies", "kernel", "0.0.1", exeName)
		if fi, err := os.Stat(depPath); err == nil && !fi.IsDir() {
			return depPath
		}
	}

	// LookPath
	if p, err := exec.LookPath(exeName); err == nil {
		return p
	}
	if p, err := exec.LookPath(fallbackName); err == nil {
		return p
	}

	return exeName
}

// BuildTurnArgs constructs command line arguments for the kernel agent.
func (a *CompaKernelAdapter) BuildTurnArgs(dir, mode, prompt, nativeID string, extraArgs []string) ([]string, error) {
	if err := validateAutonomousMode(mode); err != nil {
		return nil, err
	}
	args := []string{"agent", "-m", prompt}
	if dir != "" {
		args = append(args, "--dir", dir)
	}
	if nativeID != "" {
		args = append(args, "-s", nativeID)
	}
	args = append(args, extraArgs...)
	return args, nil
}

// NormalizeEvent parses output from the kernel agent.
func (a *CompaKernelAdapter) NormalizeEvent(line []byte) (*NormalizedEvent, error) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return nil, nil
	}

	var raw map[string]any
	if err := json.Unmarshal(trimmed, &raw); err == nil {
		if t, ok := raw["type"].(string); ok && t != "" {
			ev := &NormalizedEvent{
				Type: t,
				Raw:  raw,
			}
			if s, ok := raw["content"].(string); ok {
				ev.Content = s
			} else if s, ok := raw["text"].(string); ok {
				ev.Content = s
			}
			return ev, nil
		}
	}

	// Plain text line from agent
	text := string(trimmed)
	if strings.HasPrefix(strings.ToLower(text), "error:") {
		return &NormalizedEvent{
			Type:    EventError,
			Content: text,
			IsError: true,
		}, nil
	}

	return &NormalizedEvent{
		Type:    EventTextDelta,
		Content: text + "\n",
	}, nil
}
