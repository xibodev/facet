package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// PathsConfig holds resolved or pinned executable and directory paths.
type PathsConfig struct {
	FFmpeg           string `yaml:"ffmpeg,omitempty" json:"ffmpeg,omitempty"`
	FFprobe          string `yaml:"ffprobe,omitempty" json:"ffprobe,omitempty"`
	Node             string `yaml:"node,omitempty" json:"node,omitempty"`
	NPM              string `yaml:"npm,omitempty" json:"npm,omitempty"`
	Bundle           string `yaml:"bundle,omitempty" json:"bundle,omitempty"`
	RemotionComposer string `yaml:"remotion_composer,omitempty" json:"remotion_composer,omitempty"`
	Claude           string `yaml:"claude,omitempty" json:"claude,omitempty"`
	OpenCode         string `yaml:"opencode,omitempty" json:"opencode,omitempty"`
	Copilot          string `yaml:"copilot,omitempty" json:"copilot,omitempty"`
	Codex            string `yaml:"codex,omitempty" json:"codex,omitempty"`
}

// DefaultsConfig holds default production and engine parameters.
type DefaultsConfig struct {
	Engine         string `yaml:"engine" json:"engine"`
	Voice          string `yaml:"voice" json:"voice"`
	Resolution     string `yaml:"resolution" json:"resolution"`
	FPS            int    `yaml:"fps" json:"fps"`
	AspectRatio    string `yaml:"aspect_ratio" json:"aspect_ratio"`
	PermissionMode string `yaml:"permission_mode" json:"permission_mode"`
}

// Config represents the complete Facet configuration.
type Config struct {
	Project    string         `yaml:"project,omitempty" json:"project,omitempty"`
	Paths      PathsConfig    `yaml:"paths" json:"paths"`
	Defaults   DefaultsConfig `yaml:"defaults" json:"defaults"`
	EnvProbes  []string       `yaml:"env_probes" json:"env_probes"`
	LoadedFrom string         `yaml:"-" json:"-"`
}

// DefaultEnvProbes returns the standard list of environment variable names probed.
func DefaultEnvProbes() []string {
	return []string{
		"OPENAI_API_KEY",
		"ELEVENLABS_API_KEY",
		"FAL_KEY",
		"PEXELS_API_KEY",
		"PIXABAY_API_KEY",
		"ANTHROPIC_API_KEY",
	}
}

// DefaultConfig returns a new Config with standard default values.
func DefaultConfig() *Config {
	return &Config{
		Paths: PathsConfig{},
		Defaults: DefaultsConfig{
			Engine:         "claude",
			Voice:          "en-US-ChristopherNeural",
			Resolution:     "1920x1080",
			FPS:            30,
			AspectRatio:    "16:9",
			PermissionMode: "rw",
		},
		EnvProbes: DefaultEnvProbes(),
	}
}

// FindConfigFile locates the configuration file in standard precedence order:
// 1. ./.facet.yaml (or .facet.yaml in current working directory)
// 2. ~/.config/facet/config.yaml
func FindConfigFile() string {
	// 1. Check current directory .facet.yaml
	if fi, err := os.Stat(".facet.yaml"); err == nil && !fi.IsDir() {
		return ".facet.yaml"
	}

	// 2. Check ~/.config/facet/config.yaml
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		globalPath := filepath.Join(home, ".config", "facet", "config.yaml")
		if fi, err := os.Stat(globalPath); err == nil && !fi.IsDir() {
			return globalPath
		}
	}

	return ""
}

// Load loads the configuration. If a path is specified, it loads from that path.
// Otherwise, it checks ./.facet.yaml then ~/.config/facet/config.yaml.
// If neither exists, it returns DefaultConfig with AutoDetect populated.
func Load(paths ...string) (*Config, error) {
	var targetPath string
	if len(paths) > 0 && strings.TrimSpace(paths[0]) != "" {
		targetPath = paths[0]
	} else {
		targetPath = FindConfigFile()
	}

	cfg := DefaultConfig()

	if targetPath == "" {
		cfg.AutoDetect()
		return cfg, nil
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) && (len(paths) == 0 || paths[0] == "") {
			cfg.AutoDetect()
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file at %s: %w", targetPath, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file at %s: %w", targetPath, err)
	}

	cfg.LoadedFrom = targetPath

	// Ensure defaults for any empty fields
	if cfg.Defaults.Engine == "" {
		cfg.Defaults.Engine = "claude"
	}
	if cfg.Defaults.Voice == "" {
		cfg.Defaults.Voice = "en-US-ChristopherNeural"
	}
	if cfg.Defaults.Resolution == "" {
		cfg.Defaults.Resolution = "1920x1080"
	}
	if cfg.Defaults.FPS <= 0 {
		cfg.Defaults.FPS = 30
	}
	if cfg.Defaults.AspectRatio == "" {
		cfg.Defaults.AspectRatio = "16:9"
	}
	if cfg.Defaults.PermissionMode == "" {
		cfg.Defaults.PermissionMode = "rw"
	}
	if len(cfg.EnvProbes) == 0 {
		cfg.EnvProbes = DefaultEnvProbes()
	}

	cfg.AutoDetect()
	return cfg, nil
}

// Save writes the configuration to a YAML file. If path is provided, it writes there;
// otherwise it uses LoadedFrom if set, or defaults to ./.facet.yaml.
func (c *Config) Save(paths ...string) error {
	var targetPath string
	if len(paths) > 0 && strings.TrimSpace(paths[0]) != "" {
		targetPath = paths[0]
	} else if c.LoadedFrom != "" {
		targetPath = c.LoadedFrom
	} else {
		targetPath = ".facet.yaml"
	}

	dir := filepath.Dir(targetPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory %s: %w", dir, err)
		}
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config to %s: %w", targetPath, err)
	}

	c.LoadedFrom = targetPath
	return nil
}

// AutoDetect checks exec.LookPath for ffmpeg, ffprobe, node, npm, claude, opencode, copilot,
// as well as bundle and remotion composer directories, populating missing paths without
// overwriting already-pinned values.
func (c *Config) AutoDetect() map[string]string {
	detected := make(map[string]string)

	lookup := func(key string, target *string, names ...string) {
		if *target != "" {
			return
		}
		if path := FindExecutable(names...); path != "" {
			*target = path
			detected[key] = path
		}
	}

	lookup("ffmpeg", &c.Paths.FFmpeg, "ffmpeg")
	lookup("ffprobe", &c.Paths.FFprobe, "ffprobe")
	lookup("node", &c.Paths.Node, "node")
	lookup("npm", &c.Paths.NPM, "npm")
	lookup("claude", &c.Paths.Claude, "claude", "claude-code")
	lookup("opencode", &c.Paths.OpenCode, "opencode")
	lookup("copilot", &c.Paths.Copilot, "copilot", "github-copilot-cli", "gh-copilot")
	lookup("codex", &c.Paths.Codex, "codex", "openai-codex")

	// Auto-detect bundle root directory if not pinned
	if c.Paths.Bundle == "" {
		candidates := []string{".", "..", filepath.Join("..", "..")}
		for _, cand := range candidates {
			if isBundleRoot(cand) {
				if abs, err := filepath.Abs(cand); err == nil {
					c.Paths.Bundle = abs
					detected["bundle"] = abs
					break
				}
			}
		}
	}

	// Auto-detect Remotion Composer directory if not pinned
	if c.Paths.RemotionComposer == "" {
		candidates := []string{
			"remotion-composer",
			filepath.Join("..", "remotion-composer"),
			filepath.Join("..", "..", "remotion-composer"),
		}
		if c.Paths.Bundle != "" {
			candidates = append([]string{filepath.Join(c.Paths.Bundle, "remotion-composer")}, candidates...)
		}
		for _, cand := range candidates {
			if fi, err := os.Stat(filepath.Join(cand, "package.json")); err == nil && !fi.IsDir() {
				if abs, err := filepath.Abs(cand); err == nil {
					c.Paths.RemotionComposer = abs
					detected["remotion_composer"] = abs
					break
				}
			}
		}
	}

	return detected
}

func isBundleRoot(dir string) bool {
	if fi, err := os.Stat(filepath.Join(dir, "skills")); err == nil && fi.IsDir() {
		return true
	}
	if fi, err := os.Stat(filepath.Join(dir, "pipeline_defs")); err == nil && fi.IsDir() {
		return true
	}
	return false
}

// FindExecutable searches for executables across system PATH, extensions, and standard directories.
func FindExecutable(names ...string) string {
	exts := []string{""}
	if os.PathSeparator == '\\' {
		exts = []string{"", ".cmd", ".exe", ".bat", ".ps1"}
	}

	for _, name := range names {
		for _, ext := range exts {
			candidate := name + ext
			if path, err := exec.LookPath(candidate); err == nil && path != "" {
				if abs, err := filepath.Abs(path); err == nil {
					return abs
				}
				return path
			}
		}
	}

	// Search common install directories on Windows & Unix
	home, _ := os.UserHomeDir()
	appData := os.Getenv("APPDATA")
	localAppData := os.Getenv("LOCALAPPDATA")

	searchDirs := []string{}
	if home != "" {
		searchDirs = append(searchDirs,
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, ".facet", "bin"),
			filepath.Join(home, "bin"),
		)
	}
	if appData != "" {
		searchDirs = append(searchDirs, filepath.Join(appData, "npm"))
	}
	if localAppData != "" {
		searchDirs = append(searchDirs,
			filepath.Join(localAppData, "Programs"),
			filepath.Join(localAppData, "Programs", "Facet", "bin"),
		)
		// Check fnm/nvm subdirectories
		matches, _ := filepath.Glob(filepath.Join(localAppData, "fnm_multishells", "*"))
		searchDirs = append(searchDirs, matches...)
	}

	for _, dir := range searchDirs {
		for _, name := range names {
			for _, ext := range exts {
				full := filepath.Join(dir, name+ext)
				if fi, err := os.Stat(full); err == nil && !fi.IsDir() {
					return full
				}
			}
		}
	}

	return ""
}
