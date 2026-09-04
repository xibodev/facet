package studio

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/xibodev/facet/internal/config"
)

var catalogMu sync.Mutex

// Catalog tracks recent and managed Facet projects.
type Catalog struct {
	Version     string           `json:"version"`
	DefaultRoot string           `json:"default_root"`
	Projects    []CatalogProject `json:"projects"`
}

// CatalogProject represents a single production entry in the catalog.
type CatalogProject struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Engine       string    `json:"engine"`
	Packs        []string  `json:"packs"`
	CreatedAt    time.Time `json:"created_at"`
	LastOpenedAt time.Time `json:"last_opened_at"`
	Exists       bool      `json:"exists"`
}

// PackInfo describes an available capability pack.
type PackInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Pipelines   []string `json:"pipelines"`
	Skills      []string `json:"skills"`
	Installed   bool     `json:"installed"`
}

// GetDefaultProductionsRoot returns the default path for new productions.
func GetDefaultProductionsRoot() string {
	if runtime.GOOS == "windows" {
		if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
			return filepath.Join(localApp, "Facet", "productions")
		}
	}
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, ".facet", "productions")
	}
	return filepath.Join(".", "productions")
}

// GetCatalogPath returns the absolute path to catalog.json.
func GetCatalogPath(rootDir ...string) string {
	if len(rootDir) > 0 && rootDir[0] != "" {
		localCat := filepath.Join(rootDir[0], ".facet", "catalog.json")
		if _, err := os.Stat(localCat); err == nil {
			return localCat
		}
		if isTempDirectory(rootDir[0]) {
			return localCat
		}
	}
	if runtime.GOOS == "windows" {
		if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
			return filepath.Join(localApp, "Facet", "catalog.json")
		}
	}
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, ".facet", "catalog.json")
	}
	return filepath.Join(".", ".facet", "catalog.json")
}

func isTempDirectory(dir string) bool {
	cleanDir := strings.ToLower(filepath.Clean(dir))
	cleanTemp := strings.ToLower(filepath.Clean(os.TempDir()))
	if strings.HasPrefix(cleanDir, cleanTemp) {
		return true
	}
	for _, envKey := range []string{"TEMP", "TMP"} {
		if val := os.Getenv(envKey); val != "" {
			cleanVal := strings.ToLower(filepath.Clean(val))
			if strings.HasPrefix(cleanDir, cleanVal) {
				return true
			}
		}
	}
	return false
}

// LoadCatalog reads the catalog from disk or initializes a default.
func LoadCatalog(rootDir ...string) (*Catalog, error) {
	catalogMu.Lock()
	defer catalogMu.Unlock()

	catPath := GetCatalogPath(rootDir...)
	cat := &Catalog{
		Version:     "1.0",
		DefaultRoot: GetDefaultProductionsRoot(),
		Projects:    make([]CatalogProject, 0),
	}

	data, err := os.ReadFile(catPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cat, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cat); err != nil {
		return cat, nil
	}

	// Update Exists flag for all projects
	for i := range cat.Projects {
		p := &cat.Projects[i]
		if fi, err := os.Stat(p.Path); err == nil && fi.IsDir() {
			p.Exists = true
		} else {
			p.Exists = false
		}
	}

	return cat, nil
}

// SaveCatalog writes the catalog atomically to disk.
func SaveCatalog(cat *Catalog, rootDir ...string) error {
	catalogMu.Lock()
	defer catalogMu.Unlock()

	catPath := GetCatalogPath(rootDir...)
	parentDir := filepath.Dir(catPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := catPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, catPath)
}

// RegisterOrUpdateProject adds or touches a project in the catalog.
func RegisterOrUpdateProject(name, targetPath, engine string, packs []string, rootDir ...string) (*CatalogProject, error) {
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		absPath = targetPath
	}

	cat, err := LoadCatalog(rootDir...)
	if err != nil {
		cat = &Catalog{
			Version:     "1.0",
			DefaultRoot: GetDefaultProductionsRoot(),
			Projects:    make([]CatalogProject, 0),
		}
	}

	now := time.Now().UTC()
	var found *CatalogProject

	for i := range cat.Projects {
		p := &cat.Projects[i]
		if strings.EqualFold(p.Path, absPath) {
			p.LastOpenedAt = now
			if name != "" {
				p.Name = name
			}
			if engine != "" {
				p.Engine = engine
			}
			if len(packs) > 0 {
				p.Packs = packs
			}
			p.Exists = true
			found = p
			break
		}
	}

	if found == nil {
		if name == "" {
			name = filepath.Base(absPath)
		}
		id := strings.ToLower(name)
		id = strings.ReplaceAll(id, " ", "-")

		newProj := CatalogProject{
			ID:           id,
			Name:         name,
			Path:         absPath,
			Engine:       engine,
			Packs:        packs,
			CreatedAt:    now,
			LastOpenedAt: now,
			Exists:       true,
		}
		cat.Projects = append([]CatalogProject{newProj}, cat.Projects...)
		found = &cat.Projects[0]
	}

	_ = SaveCatalog(cat, rootDir...)
	return found, nil
}

// DiscoverAvailablePacks inspects both repository packs and central app data.
func DiscoverAvailablePacks(rootDir string) []PackInfo {
	packsMap := make(map[string]PackInfo)

	// 1. Check local repo packs directory
	repoPacksDir := filepath.Join(rootDir, "packs")
	scanPacksDir(repoPacksDir, packsMap)

	// 2. Check parent repo packs directory if running from subfolder
	parentPacksDir := filepath.Join(rootDir, "..", "packs")
	scanPacksDir(parentPacksDir, packsMap)

	// 3. Check central app data
	if runtime.GOOS == "windows" {
		if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
			centralPacksDir := filepath.Join(localApp, "Facet", "packs")
			scanPacksDir(centralPacksDir, packsMap)
		}
	}

	// 4. Ensure known official packs exist in list even if not yet installed
	knownPacks := []PackInfo{
		{
			ID:          "explainer",
			Name:        "Animated Explainer",
			Version:     "1.0.0",
			Description: "2D animated explainer pipeline with motion graphics, text cards, charts, and Remotion composition.",
			Pipelines:   []string{"animated-explainer"},
			Skills:      []string{"explainer", "explainer-producer"},
			Installed:   true,
		},
		{
			ID:          "cinematic",
			Name:        "Cinematic & Documentary",
			Version:     "1.0.0",
			Description: "Cinematic documentary montage, Wikimedia public domain sourcing, color grading, and video stitching.",
			Pipelines:   []string{"cinematic", "documentary-montage"},
			Skills:      []string{"cinematic"},
			Installed:   true,
		},
		{
			ID:          "screen-demo",
			Name:        "Screen Demo & Walkthrough",
			Version:     "1.0.0",
			Description: "Software walkthroughs, synthetic terminal screen recording, UI highlights, and feature demos.",
			Pipelines:   []string{"screen-demo"},
			Skills:      []string{"screen-demo"},
			Installed:   true,
		},
		{
			ID:          "talking-head",
			Name:        "Talking Head & Avatar",
			Version:     "1.0.0",
			Description: "AI presenter videos, lip-sync, teleprompter scripts, and Edge/Azure neural narration.",
			Pipelines:   []string{"talking-head", "avatar-spokesperson"},
			Skills:      []string{"talking-head"},
			Installed:   true,
		},
		{
			ID:          "social",
			Name:        "Social & Short-Form",
			Version:     "1.0.0",
			Description: "Short-form vertical video, viral clips, podcast repurposing, and caption burn-in pipeline.",
			Pipelines:   []string{"clip-factory", "podcast-repurpose"},
			Skills:      []string{"social"},
			Installed:   true,
		},
		{
			ID:          "character-animation",
			Name:        "2D Character Animation",
			Version:     "1.0.0",
			Description: "2D character animation, SVG rigging, pose libraries, and action cycle pipelines.",
			Pipelines:   []string{"character-animation", "animation"},
			Skills:      []string{"animation"},
			Installed:   true,
		},
		{
			ID:          "localization",
			Name:        "Video Localization & Dubbing",
			Version:     "1.0.0",
			Description: "Multilingual dubbing, subtitle translation, and voice localization pipeline.",
			Pipelines:   []string{"localization-dub"},
			Skills:      []string{"localization"},
			Installed:   true,
		},
	}

	for _, kp := range knownPacks {
		if existing, exists := packsMap[kp.ID]; exists {
			existing.Installed = true
			packsMap[kp.ID] = existing
		} else {
			packsMap[kp.ID] = kp
		}
	}

	result := make([]PackInfo, 0, len(packsMap))
	order := []string{"explainer", "cinematic", "screen-demo", "talking-head", "social", "character-animation", "localization"}
	for _, id := range order {
		if p, ok := packsMap[id]; ok {
			result = append(result, p)
			delete(packsMap, id)
		}
	}
	for _, p := range packsMap {
		result = append(result, p)
	}

	return result
}

func scanPacksDir(dir string, out map[string]PackInfo) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		packPath := filepath.Join(dir, entry.Name())
		manifestPath := filepath.Join(packPath, "facet-pack.json")

		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}

		var raw struct {
			Name        string `json:"name"`
			Version     string `json:"version"`
			Description string `json:"description"`
			Exports     struct {
				Skills []struct {
					ID string `json:"id"`
				} `json:"skills"`
				Pipelines []struct {
					ID string `json:"id"`
				} `json:"pipelines"`
			} `json:"exports"`
		}

		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}

		id := entry.Name()
		name := raw.Name
		if name == "" {
			name = id
		}

		skills := make([]string, 0, len(raw.Exports.Skills))
		for _, s := range raw.Exports.Skills {
			skills = append(skills, s.ID)
		}

		pipelines := make([]string, 0, len(raw.Exports.Pipelines))
		for _, p := range raw.Exports.Pipelines {
			pipelines = append(pipelines, p.ID)
		}

		out[id] = PackInfo{
			ID:          id,
			Name:        name,
			Version:     raw.Version,
			Description: raw.Description,
			Pipelines:   pipelines,
			Skills:      skills,
			Installed:   true,
		}
	}
}

// CreateNewProject handles the backend creation and initialization of a new project.
func CreateNewProject(name, slug, baseDir, engine string, packs []string, rootDir ...string) (*CatalogProject, error) {
	if name == "" {
		name = "Untitled Production"
	}
	if slug == "" {
		slug = strings.ToLower(name)
		slug = strings.ReplaceAll(slug, " ", "-")
	}

	if baseDir == "" {
		baseDir = GetDefaultProductionsRoot()
	}

	targetDir := filepath.Join(baseDir, slug)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create project directory: %w", err)
	}

	if engine == "" {
		engine = "claude"
	}
	if len(packs) == 0 {
		packs = []string{"explainer"}
	}

	opts := config.InitOptions{
		ProjectDir: targetDir,
		Engine:     engine,
		Packs:      packs,
	}

	cfg := config.DefaultConfig()
	if _, err := config.RunInitWithOptions(opts, cfg, nil); err != nil {
		return nil, fmt.Errorf("project initialization failed: %w", err)
	}

	return RegisterOrUpdateProject(name, targetDir, engine, packs, rootDir...)
}

// OpenExistingProject registers an existing folder and refreshes its projections.
func OpenExistingProject(targetPath, engine string, rootDir ...string) (*CatalogProject, error) {
	fi, err := os.Stat(targetPath)
	if err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("directory does not exist: %s", targetPath)
	}

	// Try reading facet.lock.json to discover engine and packs
	packs := []string{"explainer"}
	lockPath := filepath.Join(targetPath, "facet.lock.json")
	if data, err := os.ReadFile(lockPath); err == nil {
		var lock config.ProjectLock
		if err := json.Unmarshal(data, &lock); err == nil {
			if lock.Engine != "" {
				engine = lock.Engine
			}
			if len(lock.Packs) > 0 {
				packs = lock.Packs
			}
		}
	}

	if engine == "" {
		engine = "claude"
	}

	opts := config.InitOptions{
		ProjectDir: targetPath,
		Engine:     engine,
		Packs:      packs,
	}
	cfg := config.DefaultConfig()
	_, _ = config.RunInitWithOptions(opts, cfg, nil)

	name := filepath.Base(targetPath)
	return RegisterOrUpdateProject(name, targetPath, engine, packs, rootDir...)
}
