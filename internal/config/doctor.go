package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/xibodev/facet/internal/toolbox"
)

// CheckStatus represents the health/discovery status of a component.
type CheckStatus string

const (
	StatusOK       CheckStatus = "OK"
	StatusFound    CheckStatus = "Found"
	StatusNotFound CheckStatus = "Not found"
	StatusWarning  CheckStatus = "Warning"
	StatusError    CheckStatus = "Error"
)

// RuntimeCheck represents the status of an external or built-in runtime.
type RuntimeCheck struct {
	Name      string      `json:"name"`
	Status    CheckStatus `json:"status"`
	Path      string      `json:"path,omitempty"`
	Version   string      `json:"version,omitempty"`
	Details   string      `json:"details,omitempty"`
	Available bool        `json:"available"`
}

// AgentCLICheck represents the discovery status of an agent CLI.
type AgentCLICheck struct {
	Name      string      `json:"name"`
	Status    CheckStatus `json:"status"`
	Path      string      `json:"path,omitempty"`
	Available bool        `json:"available"`
}

// EnvVarCheck represents the masked presence of an environment variable.
type EnvVarCheck struct {
	Name    string `json:"name"`
	IsSet   bool   `json:"is_set"`
	Display string `json:"display"` // "set" or "not set"
}

// ToolCheck represents the configuration status of a toolbox tool.
type ToolCheck struct {
	Name         string   `json:"name"`
	Capability   string   `json:"capability"`
	Configured   bool     `json:"configured"`
	MissingDeps  []string `json:"missing_dependencies,omitempty"`
}

// DoctorReport contains the aggregated discovery and health report.
type DoctorReport struct {
	Runtimes []RuntimeCheck  `json:"runtimes"`
	CLIs     []AgentCLICheck `json:"agent_clis"`
	EnvVars  []EnvVarCheck   `json:"env_vars"`
	Tools    []ToolCheck     `json:"tools"`
}

// RunDoctor executes all system discovery checks and prints the formatted report to stdout.
func RunDoctor(cfg *Config) error {
	_, err := RunDoctorWithWriter(cfg, os.Stdout)
	return err
}

// RunDoctorWithWriter executes all discovery checks, formats and prints the output to w, and returns the report.
func RunDoctorWithWriter(cfg *Config, w io.Writer) (*DoctorReport, error) {
	if cfg == nil {
		var err error
		cfg, err = Load()
		if err != nil {
			cfg = DefaultConfig()
		}
	} else {
		cfg.AutoDetect()
	}

	report := GenerateDoctorReport(cfg)
	formatDoctorReport(report, w)
	return report, nil
}

// GenerateDoctorReport inspects the environment and produces a structured DoctorReport.
func GenerateDoctorReport(cfg *Config) *DoctorReport {
	report := &DoctorReport{
		Runtimes: make([]RuntimeCheck, 0),
		CLIs:     make([]AgentCLICheck, 0),
		EnvVars:  make([]EnvVarCheck, 0),
		Tools:    make([]ToolCheck, 0),
	}

	// 1. System Runtimes
	report.Runtimes = append(report.Runtimes, probeBinaryRuntime("FFmpeg", cfg.Paths.FFmpeg, "ffmpeg", "-version"))
	report.Runtimes = append(report.Runtimes, probeBinaryRuntime("FFprobe", cfg.Paths.FFprobe, "ffprobe", "-version"))
	report.Runtimes = append(report.Runtimes, probeBinaryRuntime("Node", cfg.Paths.Node, "node", "-v"))
	report.Runtimes = append(report.Runtimes, probeRemotionComposer(cfg.Paths.RemotionComposer))
	report.Runtimes = append(report.Runtimes, probeEdgeTTS())

	// 2. Agent CLIs
	report.CLIs = append(report.CLIs, probeAgentCLI("Claude Code", cfg.Paths.Claude, "claude", "claude-code"))
	report.CLIs = append(report.CLIs, probeAgentCLI("OpenCode", cfg.Paths.OpenCode, "opencode"))
	report.CLIs = append(report.CLIs, probeAgentCLI("GitHub Copilot", cfg.Paths.Copilot, "copilot", "github-copilot-cli", "gh-copilot"))
	report.CLIs = append(report.CLIs, probeAgentCLI("OpenAI Codex", cfg.Paths.Codex, "codex", "openai-codex"))

	// 3. Environment Variables
	probes := cfg.EnvProbes
	if len(probes) == 0 {
		probes = DefaultEnvProbes()
	}
	for _, envVar := range probes {
		val := os.Getenv(envVar)
		isSet := strings.TrimSpace(val) != ""
		display := "not set"
		if isSet {
			display = "set"
		}
		report.EnvVars = append(report.EnvVars, EnvVarCheck{
			Name:    envVar,
			IsSet:   isSet,
			Display: display,
		})
	}

	// 4. 33 Toolbox Tools
	envList, ok := toolbox.CLI([]string{"tools", "list"})
	if ok && envList.Result != nil {
		if resMap, ok := envList.Result.(map[string]any); ok {
			if toolList, ok := resMap["tools"].([]any); ok {
				for _, item := range toolList {
					if tMap, ok := item.(map[string]any); ok {
						name, _ := tMap["name"].(string)
						capab, _ := tMap["capability"].(string)
						conf, _ := tMap["configured"].(bool)
						var missing []string
						if deps, ok := tMap["dependencies"].([]any); ok {
							for _, d := range deps {
								if dm, ok := d.(map[string]any); ok {
									if avail, ok := dm["available"].(bool); ok && !avail {
										depName, _ := dm["name"].(string)
										depType, _ := dm["type"].(string)
										if depType == "env" {
											missing = append(missing, fmt.Sprintf("env:%s", depName))
										} else {
											missing = append(missing, depName)
										}
									}
								}
							}
						}
						report.Tools = append(report.Tools, ToolCheck{
							Name:        name,
							Capability:  capab,
							Configured:  conf,
							MissingDeps: missing,
						})
					}
				}
			}
		}
	}

	return report
}

func probeBinaryRuntime(name, pinnedPath, defaultBinary string, versionArg string) RuntimeCheck {
	target := pinnedPath
	if target == "" {
		target = FindExecutable(defaultBinary)
	}

	if target == "" {
		return RuntimeCheck{
			Name:      name,
			Status:    StatusNotFound,
			Available: false,
			Details:   fmt.Sprintf("%s not found in PATH or config", defaultBinary),
		}
	}

	versionStr := ""
	if versionArg != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, target, versionArg)
		out, err := cmd.Output()
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) > 0 {
				versionStr = strings.TrimSpace(lines[0])
			}
		}
	}

	return RuntimeCheck{
		Name:      name,
		Status:    StatusOK,
		Path:      target,
		Version:   versionStr,
		Available: true,
	}
}

func probeRemotionComposer(composerPath string) RuntimeCheck {
	candidates := []string{composerPath, "remotion-composer", filepath.Join("..", "remotion-composer")}
	for _, cand := range candidates {
		if cand == "" {
			continue
		}
		pkgJson := filepath.Join(cand, "package.json")
		if fi, err := os.Stat(pkgJson); err == nil && !fi.IsDir() {
			abs, _ := filepath.Abs(cand)
			nodeModules := filepath.Join(cand, "node_modules")
			hasModules := false
			if nmFi, err := os.Stat(nodeModules); err == nil && nmFi.IsDir() {
				hasModules = true
			}
			details := abs
			if !hasModules {
				details += " (node_modules missing - run npm install)"
			}
			return RuntimeCheck{
				Name:      "Remotion Composer",
				Status:    StatusOK,
				Path:      abs,
				Details:   details,
				Available: true,
			}
		}
	}

	return RuntimeCheck{
		Name:      "Remotion Composer",
		Status:    StatusNotFound,
		Available: false,
		Details:   "remotion-composer/package.json not found",
	}
}

func probeEdgeTTS() RuntimeCheck {
	return RuntimeCheck{
		Name:      "Edge-TTS",
		Status:    StatusOK,
		Details:   "built-in keyless neural TTS runtime ready",
		Available: true,
	}
}

func probeAgentCLI(displayName, pinnedPath string, binaryNames ...string) AgentCLICheck {
	target := pinnedPath
	if target == "" {
		target = FindExecutable(binaryNames...)
	}

	if target != "" {
		return AgentCLICheck{
			Name:      displayName,
			Status:    StatusFound,
			Path:      target,
			Available: true,
		}
	}

	return AgentCLICheck{
		Name:      displayName,
		Status:    StatusNotFound,
		Available: false,
	}
}

func formatDoctorReport(report *DoctorReport, w io.Writer) {
	fmt.Fprintln(w, "=== Facet System Doctor ===")
	fmt.Fprintln(w)

	// 1. Runtimes
	fmt.Fprintln(w, "[System Runtimes]")
	for _, r := range report.Runtimes {
		symbol := "✓"
		if !r.Available {
			symbol = "✗"
		}
		info := string(r.Status)
		if r.Version != "" {
			info = fmt.Sprintf("%s (%s)", r.Status, r.Version)
		} else if r.Details != "" {
			info = fmt.Sprintf("%s (%s)", r.Status, r.Details)
		} else if r.Path != "" {
			info = fmt.Sprintf("%s (%s)", r.Status, r.Path)
		}
		fmt.Fprintf(w, "  %s %-18s : %s\n", symbol, r.Name, info)
	}
	fmt.Fprintln(w)

	// 2. Agent CLIs
	fmt.Fprintln(w, "[Agent CLIs]")
	for _, cli := range report.CLIs {
		symbol := "✓"
		if !cli.Available {
			symbol = "-"
		}
		info := string(cli.Status)
		if cli.Path != "" {
			info = fmt.Sprintf("%s (%s)", cli.Status, cli.Path)
		}
		fmt.Fprintf(w, "  %s %-18s : %s\n", symbol, cli.Name, info)
	}
	fmt.Fprintln(w)

	// 3. Environment Variables
	fmt.Fprintln(w, "[Environment Variables]")
	for _, ev := range report.EnvVars {
		symbol := "✓"
		if !ev.IsSet {
			symbol = "-"
		}
		fmt.Fprintf(w, "  %s %-20s : %s\n", symbol, ev.Name, ev.Display)
	}
	fmt.Fprintln(w)

	// 4. Toolbox Tools
	fmt.Fprintf(w, "[Toolbox Tools (%d tools)]\n", len(report.Tools))
	for _, t := range report.Tools {
		symbol := "✓"
		statusStr := "[configured]"
		if !t.Configured {
			symbol = "-"
			if len(t.MissingDeps) > 0 {
				statusStr = fmt.Sprintf("[missing: %s]", strings.Join(t.MissingDeps, ", "))
			} else {
				statusStr = "[unconfigured]"
			}
		}
		fmt.Fprintf(w, "  %s %-22s %-25s (%s)\n", symbol, t.Name, statusStr, t.Capability)
	}
	fmt.Fprintln(w)
}

// JSON serialization helper for report
func (r *DoctorReport) JSON() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
