package studio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// StageStatuses indicates which production stages are completed or active.
type StageStatuses struct {
	Brief       bool `json:"brief"`
	Script      bool `json:"script"`
	Voiceover   bool `json:"voiceover"`
	Composition bool `json:"composition"`
	Review      bool `json:"review"`
	Master      bool `json:"master"`
}

// ProjectSummary provides lightweight metadata for list views.
type ProjectSummary struct {
	Slug             string        `json:"slug"`
	Name             string        `json:"name"`
	Path             string        `json:"path"`
	LastModified     time.Time     `json:"last_modified"`
	Stages           StageStatuses `json:"stages"`
	BriefPath        string        `json:"brief_path,omitempty"`
	BriefURL         string        `json:"brief_url,omitempty"`
	ScriptPath       string        `json:"script_path,omitempty"`
	ScriptURL        string        `json:"script_url,omitempty"`
	CompositionPath  string        `json:"composition_path,omitempty"`
	CompositionURL   string        `json:"composition_url,omitempty"`
	ThumbnailURL     string        `json:"thumbnail_url,omitempty"`
	VideoURL         string        `json:"video_url,omitempty"`
	PreviewVideoPath string        `json:"preview_video_path,omitempty"`
	PreviewVideoURL  string        `json:"preview_video_url,omitempty"`
	VideoVersion     string        `json:"video_version,omitempty"`
}

// BeatItem represents a single editorial or visual beat from script or composition.
type BeatItem struct {
	Index     string            `json:"index,omitempty"`
	TimeRange string            `json:"time_range,omitempty"`
	Title     string            `json:"title,omitempty"`
	Visual    string            `json:"visual,omitempty"`
	Narration string            `json:"narration,omitempty"`
	Type      string            `json:"type,omitempty"`
	Raw       map[string]string `json:"raw,omitempty"`
}

// MediaFile represents an audio, image, or video file inside a project.
type MediaFile struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	RelativePath string    `json:"relative_path"`
	URL          string    `json:"url"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
}

// ProjectDetails provides deep inspection data for a project.
type ProjectDetails struct {
	Slug             string        `json:"slug"`
	Name             string        `json:"name"`
	Path             string        `json:"path"`
	LastModified     time.Time     `json:"last_modified"`
	Stages           StageStatuses `json:"stages"`
	Brief            string        `json:"brief,omitempty"`
	BriefPath        string        `json:"brief_path,omitempty"`
	BriefURL         string        `json:"brief_url,omitempty"`
	Script           string        `json:"script,omitempty"`
	ScriptPath       string        `json:"script_path,omitempty"`
	ScriptURL        string        `json:"script_url,omitempty"`
	Beats            []BeatItem    `json:"beats,omitempty"`
	Narration        []MediaFile   `json:"narration,omitempty"`
	ReviewFrames     []MediaFile   `json:"review_frames,omitempty"`
	QAFrames         []MediaFile   `json:"qa_frames,omitempty"`
	ReviewReport     any           `json:"review_report,omitempty"`
	RemotionProps    any           `json:"remotion_props,omitempty"`
	CompositionPath  string        `json:"composition_path,omitempty"`
	CompositionURL   string        `json:"composition_url,omitempty"`
	VideoPath        string        `json:"video_path,omitempty"`
	VideoURL         string        `json:"video_url,omitempty"`
	PreviewVideoPath string        `json:"preview_video_path,omitempty"`
	PreviewVideoURL  string        `json:"preview_video_url,omitempty"`
	VideoVersion     string        `json:"video_version,omitempty"`
	ThumbnailPath    string        `json:"thumbnail_path,omitempty"`
	ThumbnailURL     string        `json:"thumbnail_url,omitempty"`
}

type artifactEvidence struct {
	FullPath string
	Path     string
	URL      string
	Text     string
	Title    string
	Value    any
}

type videoEvidence struct {
	Path    string
	URL     string
	Version string
}

type projectEvidence struct {
	Brief              artifactEvidence
	Script             artifactEvidence
	ScriptBeats        []BeatItem
	Narration          []MediaFile
	Composition        artifactEvidence
	CompositionIsProps bool
	QAFrames           []MediaFile
	ReviewFrames       []MediaFile
	ReviewReport       any
	HasReviewReport    bool
	Master             videoEvidence
	Preview            videoEvidence
	Thumbnail          videoEvidence
}

type projectsScope struct {
	rootPath          string
	projectsPath      string
	canonicalProjects string
}

type projectScope struct {
	projectsScope
	slug             string
	projectPath      string
	canonicalProject string
}

type projectFile struct {
	canonicalPath string
	relativePath  string
	url           string
	info          os.FileInfo
}

func (e projectEvidence) stages() StageStatuses {
	return StageStatuses{
		Brief:       e.Brief.Path != "",
		Script:      e.Script.Path != "",
		Voiceover:   len(e.Narration) > 0,
		Composition: e.Composition.Path != "" || e.Master.Path != "" || e.Preview.Path != "",
		Review:      e.HasReviewReport || len(e.QAFrames) > 0 || len(e.ReviewFrames) > 0,
		Master:      e.Master.Path != "",
	}
}

func resolveProjectsScope(rootDir string) (projectsScope, error) {
	rootPath := filepath.Clean(rootDir)
	canonicalRoot, err := canonicalExistingPath(rootPath)
	if err != nil {
		return projectsScope{}, fmt.Errorf("resolve Studio root: %w", err)
	}

	projectsPath := filepath.Join(rootPath, "projects")
	canonicalProjects, err := canonicalExistingPath(projectsPath)
	if err != nil {
		return projectsScope{}, fmt.Errorf("resolve projects directory: %w", err)
	}
	if !pathStrictlyWithin(canonicalRoot, canonicalProjects) {
		return projectsScope{}, fmt.Errorf("projects directory resolves outside the Studio root")
	}
	info, err := os.Stat(canonicalProjects)
	if err != nil {
		return projectsScope{}, fmt.Errorf("stat projects directory: %w", err)
	}
	if !info.IsDir() {
		return projectsScope{}, fmt.Errorf("projects path is not a directory")
	}

	return projectsScope{
		rootPath:          rootPath,
		projectsPath:      projectsPath,
		canonicalProjects: canonicalProjects,
	}, nil
}

func validateProjectSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("project slug is empty")
	}
	for candidate := slug; ; {
		if candidate == "" || candidate == "." || candidate == ".." || strings.ContainsAny(candidate, `/\`) || filepath.IsAbs(candidate) || filepath.VolumeName(candidate) != "" {
			return fmt.Errorf("invalid project slug %q", slug)
		}
		decoded, err := url.PathUnescape(candidate)
		if err != nil {
			return fmt.Errorf("invalid project slug %q", slug)
		}
		if decoded == candidate {
			return nil
		}
		candidate = decoded
	}
}

func (scope projectsScope) selectProject(slug string) (projectScope, os.FileInfo, error) {
	if err := validateProjectSlug(slug); err != nil {
		return projectScope{}, nil, err
	}
	entries, err := os.ReadDir(scope.canonicalProjects)
	if err != nil {
		return projectScope{}, nil, fmt.Errorf("project %q not found: %w", slug, err)
	}
	for _, entry := range entries {
		if entry.Name() == slug {
			return scope.projectEntry(slug)
		}
	}
	// Fallback to catalog lookup
	if cat, err := LoadCatalog(scope.rootPath); err == nil {
		for _, cp := range cat.Projects {
			if strings.EqualFold(cp.ID, slug) || strings.EqualFold(filepath.Base(cp.Path), slug) {
				return makeCustomProjectScope(slug, cp.Path)
			}
		}
	}
	return projectScope{}, nil, fmt.Errorf("project %q is not an actual direct child directory of %q", slug, scope.projectsPath)
}

func makeCustomProjectScope(slug, dirPath string) (projectScope, os.FileInfo, error) {
	canonicalProject, err := canonicalExistingPath(dirPath)
	if err != nil {
		return projectScope{}, nil, fmt.Errorf("resolve project %q: %w", slug, err)
	}
	info, err := os.Stat(canonicalProject)
	if err != nil {
		return projectScope{}, nil, fmt.Errorf("stat project %q: %w", slug, err)
	}
	if !info.IsDir() {
		return projectScope{}, nil, fmt.Errorf("project %q is not a directory", slug)
	}
	return projectScope{
		projectsScope: projectsScope{
			rootPath:          dirPath,
			projectsPath:      filepath.Dir(dirPath),
			canonicalProjects: filepath.Dir(canonicalProject),
		},
		slug:             slug,
		projectPath:      dirPath,
		canonicalProject: canonicalProject,
	}, info, nil
}

func (scope projectsScope) projectEntry(slug string) (projectScope, os.FileInfo, error) {
	projectCandidate := filepath.Join(scope.canonicalProjects, slug)
	canonicalProject, err := canonicalExistingPath(projectCandidate)
	if err != nil {
		return projectScope{}, nil, fmt.Errorf("resolve project %q: %w", slug, err)
	}
	if !pathStrictlyWithin(scope.canonicalProjects, canonicalProject) || !samePath(projectCandidate, canonicalProject) {
		return projectScope{}, nil, fmt.Errorf("project %q is not an actual direct child directory of %q", slug, scope.projectsPath)
	}
	info, err := os.Stat(canonicalProject)
	if err != nil {
		return projectScope{}, nil, fmt.Errorf("stat project %q: %w", slug, err)
	}
	if !info.IsDir() {
		return projectScope{}, nil, fmt.Errorf("project %q is not a directory", slug)
	}

	return projectScope{
		projectsScope:    scope,
		slug:             slug,
		projectPath:      filepath.Join(scope.projectsPath, slug),
		canonicalProject: canonicalProject,
	}, info, nil
}

func (scope projectScope) resolveEntry(path string) (string, os.FileInfo, bool) {
	if strings.TrimSpace(path) == "" {
		return "", nil, false
	}
	projectPath, err := filepath.Abs(scope.projectPath)
	if err != nil {
		return "", nil, false
	}
	candidatePath, err := filepath.Abs(path)
	if err != nil {
		return "", nil, false
	}
	if !pathStrictlyWithin(projectPath, candidatePath) {
		return "", nil, false
	}

	canonicalPath, err := canonicalExistingPath(candidatePath)
	if err != nil {
		return "", nil, false
	}
	if !pathStrictlyWithin(scope.canonicalProject, canonicalPath) {
		return "", nil, false
	}

	info, err := os.Stat(canonicalPath)
	if err != nil {
		return "", nil, false
	}
	return canonicalPath, info, true
}

func (scope projectScope) resolveFile(path string) (projectFile, bool) {
	canonicalPath, info, ok := scope.resolveEntry(path)
	if !ok || !info.Mode().IsRegular() {
		return projectFile{}, false
	}
	relativePath, mediaURL := formatEvidenceLocation(scope.rootPath, path)
	if relativePath == "" || mediaURL == "" {
		return projectFile{}, false
	}
	return projectFile{
		canonicalPath: canonicalPath,
		relativePath:  relativePath,
		url:           mediaURL,
		info:          info,
	}, true
}

func (scope projectScope) readDir(path string) ([]os.DirEntry, bool) {
	canonicalPath, info, ok := scope.resolveEntry(path)
	if !ok || !info.IsDir() {
		return nil, false
	}
	entries, err := os.ReadDir(canonicalPath)
	if err != nil {
		return nil, false
	}
	return entries, true
}

// ListProjects scans projects/ under rootDir and returns summaries.
func ListProjects(rootDir string) ([]ProjectSummary, error) {
	projects, err := resolveProjectsScope(rootDir)
	if err != nil {
		if os.IsNotExist(err) || errorsIsNotExist(err) {
			return []ProjectSummary{}, nil
		}
		return nil, fmt.Errorf("failed to read projects directory: %w", err)
	}
	entries, err := os.ReadDir(projects.canonicalProjects)
	if err != nil {
		return nil, fmt.Errorf("failed to read projects directory: %w", err)
	}

	list := make([]ProjectSummary, 0, len(entries))
	for _, entry := range entries {
		slug := entry.Name()
		project, info, err := projects.projectEntry(slug)
		if err != nil {
			continue
		}
		summary, err := scanProjectSummary(project, info)
		if err != nil {
			continue
		}
		list = append(list, summary)
	}

	// Sort by LastModified descending
	sort.Slice(list, func(i, j int) bool {
		return list[i].LastModified.After(list[j].LastModified)
	})

	return list, nil
}

func errorsIsNotExist(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "cannot find") || strings.Contains(err.Error(), "no such file") || strings.Contains(err.Error(), "does not exist")
}

// ListProjectsWithCatalog scans rootDir projects and merges catalog projects.
func ListProjectsWithCatalog(rootDir string) ([]ProjectSummary, error) {
	list, err := ListProjects(rootDir)
	if err != nil {
		list = make([]ProjectSummary, 0)
	}
	seen := make(map[string]bool)
	for _, item := range list {
		seen[strings.ToLower(item.Slug)] = true
	}

	// Merge registered projects from catalog
	if cat, err := LoadCatalog(rootDir); err == nil {
		for _, cp := range cat.Projects {
			slug := cp.ID
			if slug == "" {
				slug = filepath.Base(cp.Path)
			}
			if seen[strings.ToLower(slug)] {
				continue
			}
			if !cp.Exists {
				continue
			}
			customScope, info, err := makeCustomProjectScope(slug, cp.Path)
			if err != nil {
				continue
			}
			summary, err := scanProjectSummary(customScope, info)
			if err != nil {
				continue
			}
			if cp.Name != "" {
				summary.Name = cp.Name
			}
			list = append(list, summary)
			seen[strings.ToLower(slug)] = true
		}
	}

	// Sort by LastModified descending
	sort.Slice(list, func(i, j int) bool {
		return list[i].LastModified.After(list[j].LastModified)
	})

	return list, nil
}

// GetProjectDetails reads deep project artifacts and metadata.
func GetProjectDetails(rootDir, slug string) (*ProjectDetails, error) {
	projects, err := resolveProjectsScope(rootDir)
	if err != nil {
		return nil, err
	}
	project, info, err := projects.selectProject(slug)
	if err != nil {
		return nil, err
	}
	evidence := scanProjectEvidence(project)

	details := &ProjectDetails{
		Slug:             slug,
		Name:             slugToTitle(slug),
		Path:             project.projectPath,
		LastModified:     info.ModTime(),
		Stages:           evidence.stages(),
		Brief:            evidence.Brief.Text,
		BriefPath:        evidence.Brief.Path,
		BriefURL:         evidence.Brief.URL,
		Script:           evidence.Script.Text,
		ScriptPath:       evidence.Script.Path,
		ScriptURL:        evidence.Script.URL,
		Beats:            evidence.ScriptBeats,
		Narration:        evidence.Narration,
		ReviewFrames:     evidence.ReviewFrames,
		QAFrames:         evidence.QAFrames,
		ReviewReport:     evidence.ReviewReport,
		CompositionPath:  evidence.Composition.Path,
		CompositionURL:   evidence.Composition.URL,
		VideoPath:        evidence.Master.Path,
		VideoURL:         evidence.Master.URL,
		PreviewVideoPath: evidence.Preview.Path,
		PreviewVideoURL:  evidence.Preview.URL,
		ThumbnailPath:    evidence.Thumbnail.Path,
		ThumbnailURL:     evidence.Thumbnail.URL,
	}
	if evidence.Master.Version != "" {
		details.VideoVersion = evidence.Master.Version
	} else {
		details.VideoVersion = evidence.Preview.Version
	}
	if evidence.Brief.Title != "" {
		details.Name = evidence.Brief.Title
	} else if evidence.Script.Title != "" {
		details.Name = evidence.Script.Title
	}
	if evidence.CompositionIsProps {
		details.RemotionProps = evidence.Composition.Value
		if len(details.Beats) == 0 {
			details.Beats = extractBeatsFromRemotionProps(evidence.Composition.Value)
		}
	}

	// Update LastModified from latest file
	if latest := findLatestModTime(project); !latest.IsZero() && latest.After(details.LastModified) {
		details.LastModified = latest
	}

	return details, nil
}

func scanProjectSummary(project projectScope, info os.FileInfo) (ProjectSummary, error) {
	evidence := scanProjectEvidence(project)

	summary := ProjectSummary{
		Slug:             project.slug,
		Name:             slugToTitle(project.slug),
		Path:             project.projectPath,
		LastModified:     info.ModTime(),
		Stages:           evidence.stages(),
		BriefPath:        evidence.Brief.Path,
		BriefURL:         evidence.Brief.URL,
		ScriptPath:       evidence.Script.Path,
		ScriptURL:        evidence.Script.URL,
		CompositionPath:  evidence.Composition.Path,
		CompositionURL:   evidence.Composition.URL,
		ThumbnailURL:     evidence.Thumbnail.URL,
		VideoURL:         evidence.Master.URL,
		PreviewVideoPath: evidence.Preview.Path,
		PreviewVideoURL:  evidence.Preview.URL,
	}
	if evidence.Master.Version != "" {
		summary.VideoVersion = evidence.Master.Version
	} else {
		summary.VideoVersion = evidence.Preview.Version
	}
	if evidence.Brief.Title != "" {
		summary.Name = evidence.Brief.Title
	} else if evidence.Script.Title != "" {
		summary.Name = evidence.Script.Title
	}

	if latest := findLatestModTime(project); !latest.IsZero() && latest.After(summary.LastModified) {
		summary.LastModified = latest
	}

	return summary, nil
}

func scanProjectEvidence(project projectScope) projectEvidence {
	brief := resolveTextArtifact(project, "brief")
	script := resolveTextArtifact(project, "script")
	evidence := projectEvidence{
		Brief:       brief,
		Script:      script,
		ScriptBeats: extractJSONBeats(script.Value),
	}
	if len(evidence.ScriptBeats) == 0 && strings.EqualFold(filepath.Ext(script.FullPath), ".md") {
		evidence.ScriptBeats = parseMarkdownTableBeats(script.Text)
	}

	audioExts := []string{".mp3", ".wav", ".aac", ".m4a", ".ogg"}
	evidence.Narration = append(evidence.Narration, scanMediaDir(project, "narration", audioExts)...)
	evidence.Narration = append(evidence.Narration, scanMediaDir(project, "voice-samples", audioExts)...)
	evidence.Narration = append(evidence.Narration, scanMediaDir(project, filepath.Join("assets", "audio"), audioExts)...)

	evidence.Composition, evidence.CompositionIsProps = resolveCompositionArtifact(project)
	imageExts := []string{".png", ".jpg", ".jpeg", ".webp"}
	evidence.QAFrames = scanMediaDir(project, "qa", imageExts)
	evidence.ReviewFrames = append(evidence.ReviewFrames, scanMediaDir(project, filepath.Join("review", "final-frames"), imageExts)...)
	evidence.ReviewFrames = append(evidence.ReviewFrames, scanMediaDir(project, filepath.Join("review", "source-frames"), imageExts)...)

	if report, ok := readJSONArtifact(project, filepath.Join(project.projectPath, "review", "report.json")); ok {
		evidence.ReviewReport = normalizeReviewReport(report.Value)
		evidence.HasReviewReport = true
	}
	evidence.Master = findNamedVideo(project, []string{"final.mp4", "video.mp4"}, false)
	if evidence.Master.Path == "" {
		evidence.Preview = findNamedVideo(project, []string{"edit.mp4", "montage_vo.mp4", "montage.mp4"}, true)
	}
	evidence.Thumbnail = findThumbnailEvidence(project)
	return evidence
}

func resolveTextArtifact(project projectScope, base string) artifactEvidence {
	for _, candidate := range []string{
		filepath.Join(project.projectPath, base+".md"),
		filepath.Join(project.projectPath, base+".json"),
		filepath.Join(project.projectPath, "artifacts", base+".md"),
		filepath.Join(project.projectPath, "artifacts", base+".json"),
	} {
		file, ok := project.resolveFile(candidate)
		if !ok {
			continue
		}
		contents, err := os.ReadFile(file.canonicalPath)
		if err != nil {
			continue
		}
		artifact := artifactEvidence{FullPath: candidate, Path: file.relativePath, URL: file.url}
		if strings.EqualFold(filepath.Ext(candidate), ".json") {
			pretty, value, ok := decodePrettyJSON(contents)
			if !ok {
				continue
			}
			artifact.Text = pretty
			artifact.Value = value
			artifact.Title = extractJSONTitle(value)
			return artifact
		}
		artifact.Text = string(contents)
		artifact.Title = extractMarkdownTitle(artifact.Text)
		return artifact
	}
	return artifactEvidence{}
}

func resolveCompositionArtifact(project projectScope) (artifactEvidence, bool) {
	propsCandidates := []string{filepath.Join(project.projectPath, "remotion_props.json")}
	artifactsDir := filepath.Join(project.projectPath, "artifacts")
	if entries, ok := project.readDir(artifactsDir); ok {
		for _, entry := range entries {
			name := strings.ToLower(entry.Name())
			if strings.HasSuffix(name, "props.json") {
				propsCandidates = append(propsCandidates, filepath.Join(artifactsDir, entry.Name()))
			}
		}
	}
	for _, candidate := range propsCandidates {
		if artifact, ok := readJSONArtifact(project, candidate); ok {
			return artifact, true
		}
	}
	if artifact, ok := readJSONArtifact(project, filepath.Join(artifactsDir, "edit.json")); ok {
		return artifact, false
	}
	return artifactEvidence{}, false
}

func readJSONArtifact(project projectScope, path string) (artifactEvidence, bool) {
	file, ok := project.resolveFile(path)
	if !ok {
		return artifactEvidence{}, false
	}
	contents, err := os.ReadFile(file.canonicalPath)
	if err != nil {
		return artifactEvidence{}, false
	}
	pretty, value, ok := decodePrettyJSON(contents)
	if !ok {
		return artifactEvidence{}, false
	}
	return artifactEvidence{FullPath: path, Path: file.relativePath, URL: file.url, Text: pretty, Value: value}, true
}

func decodePrettyJSON(contents []byte) (string, any, bool) {
	trimmed := bytes.TrimSpace(contents)
	if !json.Valid(trimmed) {
		return "", nil, false
	}
	var value any
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return "", nil, false
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, trimmed, "", "  "); err != nil {
		return "", nil, false
	}
	pretty.WriteByte('\n')
	return pretty.String(), value, true
}

func extractJSONTitle(value any) string {
	root, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range []string{"title", "name", "project_title", "project_name", "video_title"} {
		if title, ok := root[key].(string); ok && strings.TrimSpace(title) != "" {
			return strings.TrimSpace(title)
		}
	}
	for _, key := range []string{"project", "metadata"} {
		if nested, ok := root[key].(map[string]any); ok {
			for _, titleKey := range []string{"title", "name"} {
				if title, ok := nested[titleKey].(string); ok && strings.TrimSpace(title) != "" {
					return strings.TrimSpace(title)
				}
			}
		}
	}
	return ""
}

func extractJSONBeats(value any) []BeatItem {
	root, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	containers := []map[string]any{root}
	if nested, ok := root["script"].(map[string]any); ok {
		containers = append(containers, nested)
	}
	for _, container := range containers {
		for _, key := range []string{"beats", "scenes", "sections"} {
			items, ok := container[key].([]any)
			if !ok {
				continue
			}
			beats := make([]BeatItem, 0, len(items))
			for i, item := range items {
				beatMap, ok := item.(map[string]any)
				if !ok {
					continue
				}
				beat := BeatItem{
					Index:     firstJSONText(beatMap, "index", "id", "number", "no"),
					TimeRange: firstJSONText(beatMap, "time_range", "time"),
					Title:     firstJSONText(beatMap, "title", "name", "label", "beat", "scene"),
					Visual:    firstJSONText(beatMap, "visual", "visual_description", "action", "shot"),
					Narration: firstJSONText(beatMap, "narration", "script", "dialogue", "text", "voiceover"),
					Type:      firstJSONText(beatMap, "type", "intent"),
				}
				if beat.Index == "" {
					beat.Index = strconv.Itoa(i + 1)
				}
				if beat.TimeRange == "" {
					start := firstJSONTime(beatMap, "start", "start_seconds", "in_seconds")
					end := firstJSONTime(beatMap, "end", "end_seconds", "out_seconds")
					if start != "" && end != "" {
						beat.TimeRange = start + " - " + end
					}
				}
				for field, raw := range beatMap {
					if text := jsonText(raw); text != "" {
						if beat.Raw == nil {
							beat.Raw = make(map[string]string)
						}
						beat.Raw[field] = text
					}
				}
				beats = append(beats, beat)
			}
			return beats
		}
	}
	return nil
}

func firstJSONText(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if text := jsonText(values[key]); text != "" {
			return text
		}
	}
	return ""
}

func firstJSONTime(values map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := values[key].(type) {
		case string:
			if value = strings.TrimSpace(value); value != "" {
				return value
			}
		case float64:
			return strconv.FormatFloat(value, 'f', -1, 64) + "s"
		}
	}
	return ""
}

func jsonText(value any) string {
	switch value := value.(type) {
	case string:
		return strings.TrimSpace(value)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(value)
	default:
		return ""
	}
}

func scanMediaDir(project projectScope, subDir string, exts []string) []MediaFile {
	dirPath := filepath.Join(project.projectPath, subDir)
	entries, ok := project.readDir(dirPath)
	if !ok {
		return nil
	}

	var results []MediaFile
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !containsExt(exts, ext) {
			continue
		}
		fullPath := filepath.Join(dirPath, e.Name())
		file, ok := project.resolveFile(fullPath)
		if !ok || file.info.Size() == 0 {
			continue
		}

		results = append(results, MediaFile{
			Name:         e.Name(),
			Path:         fullPath,
			RelativePath: filepath.ToSlash(file.relativePath),
			URL:          file.url,
			Size:         file.info.Size(),
			LastModified: file.info.ModTime(),
		})
	}
	return results
}

func findNamedVideo(project projectScope, names []string, fallback bool) videoEvidence {
	rendersDir := filepath.Join(project.projectPath, "renders")
	for _, name := range names {
		if video := videoFileEvidence(project, filepath.Join(rendersDir, name)); video.Path != "" {
			return video
		}
	}
	if !fallback {
		return videoEvidence{}
	}
	reserved := map[string]bool{
		"final.mp4":      true,
		"video.mp4":      true,
		"edit.mp4":       true,
		"montage_vo.mp4": true,
		"montage.mp4":    true,
	}
	entries, ok := project.readDir(rendersDir)
	if !ok {
		return videoEvidence{}
	}
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		if reserved[name] || filepath.Ext(name) != ".mp4" {
			continue
		}
		if video := videoFileEvidence(project, filepath.Join(rendersDir, entry.Name())); video.Path != "" {
			return video
		}
	}
	return videoEvidence{}
}

func videoFileEvidence(project projectScope, path string) videoEvidence {
	file, ok := project.resolveFile(path)
	if !ok || file.info.Size() == 0 {
		return videoEvidence{}
	}
	return videoEvidence{
		Path:    file.relativePath,
		URL:     file.url,
		Version: strconv.FormatInt(file.info.Size(), 10) + "-" + strconv.FormatInt(file.info.ModTime().UnixNano(), 10),
	}
}

func normalizeReviewReport(report any) any {
	reportMap, ok := report.(map[string]any)
	if !ok {
		return map[string]any{"status": "unknown", "report": report}
	}
	if result, ok := reportMap["result"].(map[string]any); ok {
		reportMap = result
	}

	status, _ := reportMap["review_status"].(string)
	if status == "" {
		status, _ = reportMap["status"].(string)
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		status = "unknown"
	}
	reportMap["status"] = status
	return reportMap
}

func findThumbnailEvidence(project projectScope) videoEvidence {
	searchDirs := []string{
		filepath.Join(project.projectPath, "review", "final-frames"),
		filepath.Join(project.projectPath, "qa"),
		filepath.Join(project.projectPath, "review", "source-frames"),
		filepath.Join(project.projectPath, "assets", "raw"),
	}

	imgExts := []string{".jpg", ".jpeg", ".png", ".webp"}

	for _, dir := range searchDirs {
		if entries, ok := project.readDir(dir); ok {
			for _, e := range entries {
				if containsExt(imgExts, strings.ToLower(filepath.Ext(e.Name()))) {
					p := filepath.Join(dir, e.Name())
					if thumbnail := videoFileEvidence(project, p); thumbnail.Path != "" {
						thumbnail.Version = ""
						return thumbnail
					}
				}
			}
		}
	}

	return videoEvidence{}
}

func evidenceLocation(rootDir, path string) (string, string) {
	if strings.TrimSpace(path) == "" {
		return "", ""
	}
	projects, err := resolveProjectsScope(rootDir)
	if err != nil {
		return "", ""
	}
	projectsPath, err := filepath.Abs(projects.projectsPath)
	if err != nil {
		return "", ""
	}
	candidatePath, err := filepath.Abs(path)
	if err != nil {
		return "", ""
	}
	relativeToProjects, err := filepath.Rel(projectsPath, candidatePath)
	if err != nil || relativeToProjects == "." || relativeToProjects == ".." || strings.HasPrefix(relativeToProjects, ".."+string(filepath.Separator)) {
		return "", ""
	}
	parts := strings.Split(relativeToProjects, string(filepath.Separator))
	if len(parts) < 2 {
		return "", ""
	}
	project, _, err := projects.selectProject(parts[0])
	if err != nil {
		return "", ""
	}
	file, ok := project.resolveFile(candidatePath)
	if !ok {
		return "", ""
	}
	return file.relativePath, file.url
}

func formatEvidenceLocation(rootDir, path string) (string, string) {
	rel, err := filepath.Rel(rootDir, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ""
	}
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return rel, "/api/media/" + strings.Join(parts, "/")
}

func findLatestModTime(project projectScope) time.Time {
	var latest time.Time
	_ = filepath.WalkDir(project.canonicalProject, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil {
			return nil
		}
		canonicalPath, err := canonicalExistingPath(path)
		if err != nil || !pathWithin(project.canonicalProject, canonicalPath) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !samePath(path, canonicalPath) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := os.Stat(canonicalPath)
		if err != nil {
			return nil
		}
		if info.IsDir() || info.Mode().IsRegular() {
			if info.ModTime().After(latest) {
				latest = info.ModTime()
			}
		}
		return nil
	})
	return latest
}

func parseMarkdownTableBeats(script string) []BeatItem {
	lines := strings.Split(script, "\n")
	var beats []BeatItem

	inTable := false
	var headers []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") {
			inTable = false
			headers = nil
			continue
		}

		// Split columns
		cols := splitMarkdownTableRow(trimmed)
		if len(cols) < 2 {
			continue
		}

		if !inTable {
			// Check if next row is divider
			headers = make([]string, len(cols))
			for i, c := range cols {
				headers[i] = cleanMarkdownCell(c)
			}
			inTable = true
			continue
		}

		// Check if this row is a divider (---)
		if isTableDivider(cols) {
			continue
		}

		// Parse data row
		beat := BeatItem{
			Raw: make(map[string]string),
		}

		hasIndexHeader := false
		for _, h := range headers {
			lh := strings.ToLower(h)
			if lh == "#" || lh == "no" || lh == "id" || lh == "index" {
				hasIndexHeader = true
				break
			}
		}

		for i, colVal := range cols {
			cleanVal := cleanMarkdownCell(colVal)
			header := ""
			if i < len(headers) {
				header = strings.ToLower(headers[i])
				beat.Raw[headers[i]] = cleanVal
			}

			switch {
			case header == "#" || header == "index" || header == "no" || header == "id" || (!hasIndexHeader && header == "beat"):
				if beat.Index == "" {
					beat.Index = cleanVal
				}
			case header == "t" || strings.Contains(header, "time") || strings.Contains(header, "duration"):
				beat.TimeRange = cleanVal
			case header == "shot" || (hasIndexHeader && header == "beat") || strings.Contains(header, "title") || strings.Contains(header, "name") || strings.Contains(header, "section"):
				if beat.Title == "" {
					beat.Title = cleanVal
				}
			case strings.Contains(header, "visual") || strings.Contains(header, "scene") || strings.Contains(header, "action"):
				beat.Visual = cleanVal
			case strings.Contains(header, "narration") || strings.Contains(header, "audio") || strings.Contains(header, "script") || strings.Contains(header, "dialogue"):
				beat.Narration = cleanVal
			}
		}

		// Default title if empty
		if beat.Title == "" && beat.Index != "" {
			beat.Title = "Beat " + beat.Index
		}

		beats = append(beats, beat)
	}

	return beats
}

func splitMarkdownTableRow(row string) []string {
	trimmed := strings.Trim(row, "|")
	parts := strings.Split(trimmed, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isTableDivider(cols []string) bool {
	for _, c := range cols {
		clean := strings.ReplaceAll(strings.ReplaceAll(c, "-", ""), ":", "")
		if clean != "" {
			return false
		}
	}
	return true
}

var mdFormatRegex = regexp.MustCompile(`[*_` + "`" + `]`)

func cleanMarkdownCell(s string) string {
	res := mdFormatRegex.ReplaceAllString(s, "")
	res = strings.TrimSpace(res)
	res = strings.Trim(res, "\"")
	return res
}

func extractMarkdownTitle(doc string) string {
	lines := strings.Split(doc, "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "# ") {
			title := strings.TrimPrefix(trimmed, "# ")
			return strings.TrimSpace(title)
		}
	}
	return ""
}

func extractBeatsFromRemotionProps(props any) []BeatItem {
	propsMap, ok := props.(map[string]any)
	if !ok {
		return nil
	}

	cuts, ok := propsMap["cuts"].([]any)
	if !ok {
		return nil
	}

	var beats []BeatItem
	for i, c := range cuts {
		cutMap, ok := c.(map[string]any)
		if !ok {
			continue
		}

		itemType, _ := cutMap["type"].(string)
		inSec, _ := cutMap["in_seconds"].(float64)
		outSec, _ := cutMap["out_seconds"].(float64)
		text, _ := cutMap["text"].(string)
		title, _ := cutMap["title"].(string)

		timeRange := fmt.Sprintf("%.1fs - %.1fs", inSec, outSec)
		if title == "" {
			title = text
		}
		if title == "" {
			title = fmt.Sprintf("Cut %d (%s)", i+1, itemType)
		}

		beats = append(beats, BeatItem{
			Index:     fmt.Sprintf("%d", i+1),
			TimeRange: timeRange,
			Title:     title,
			Visual:    text,
			Type:      itemType,
		})
	}

	return beats
}

func slugToTitle(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func containsExt(exts []string, ext string) bool {
	for _, e := range exts {
		if e == ext {
			return true
		}
	}
	return false
}
