package studio

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	Slug         string        `json:"slug"`
	Name         string        `json:"name"`
	Path         string        `json:"path"`
	LastModified time.Time     `json:"last_modified"`
	Stages       StageStatuses `json:"stages"`
	ThumbnailURL string        `json:"thumbnail_url,omitempty"`
	VideoURL     string        `json:"video_url,omitempty"`
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
	Slug          string         `json:"slug"`
	Name          string         `json:"name"`
	Path          string         `json:"path"`
	LastModified  time.Time      `json:"last_modified"`
	Stages        StageStatuses  `json:"stages"`
	Brief         string         `json:"brief,omitempty"`
	Script        string         `json:"script,omitempty"`
	Beats         []BeatItem     `json:"beats,omitempty"`
	Narration     []MediaFile    `json:"narration,omitempty"`
	ReviewFrames  []MediaFile    `json:"review_frames,omitempty"`
	QAFrames      []MediaFile    `json:"qa_frames,omitempty"`
	ReviewReport  any            `json:"review_report,omitempty"`
	RemotionProps any            `json:"remotion_props,omitempty"`
	VideoPath     string         `json:"video_path,omitempty"`
	VideoURL      string         `json:"video_url,omitempty"`
	ThumbnailPath string         `json:"thumbnail_path,omitempty"`
	ThumbnailURL  string         `json:"thumbnail_url,omitempty"`
}

// ListProjects scans projects/ under rootDir and returns summaries.
func ListProjects(rootDir string) ([]ProjectSummary, error) {
	projectsDir := filepath.Join(rootDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			// If projects/ directory doesn't exist yet, return empty list
			return []ProjectSummary{}, nil
		}
		return nil, fmt.Errorf("failed to read projects directory: %w", err)
	}

	var list []ProjectSummary
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		slug := entry.Name()
		projPath := filepath.Join(projectsDir, slug)

		summary, err := scanProjectSummary(rootDir, slug, projPath)
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

// GetProjectDetails reads deep project artifacts and metadata.
func GetProjectDetails(rootDir, slug string) (*ProjectDetails, error) {
	projPath := filepath.Join(rootDir, "projects", slug)
	info, err := os.Stat(projPath)
	if err != nil {
		return nil, fmt.Errorf("project %q not found: %w", slug, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("project %q is not a directory", slug)
	}

	details := &ProjectDetails{
		Slug:         slug,
		Name:         slugToTitle(slug),
		Path:         projPath,
		LastModified: info.ModTime(),
	}

	// 1. Read Brief
	briefPath := filepath.Join(projPath, "brief.md")
	if briefBytes, err := os.ReadFile(briefPath); err == nil {
		details.Brief = string(briefBytes)
		details.Stages.Brief = true
		if title := extractMarkdownTitle(details.Brief); title != "" {
			details.Name = title
		}
	}

	// 2. Read Script & Parse Beats
	scriptPath := filepath.Join(projPath, "script.md")
	if scriptBytes, err := os.ReadFile(scriptPath); err == nil {
		details.Script = string(scriptBytes)
		details.Stages.Script = true
		if details.Name == slugToTitle(slug) {
			if title := extractMarkdownTitle(details.Script); title != "" {
				details.Name = title
			}
		}
		details.Beats = parseMarkdownTableBeats(details.Script)
	}

	// 3. Scan Narration & Voice Audio
	narrationFiles := scanMediaDir(rootDir, projPath, "narration", []string{".mp3", ".wav", ".aac", ".m4a", ".ogg"})
	voiceSamples := scanMediaDir(rootDir, projPath, "voice-samples", []string{".mp3", ".wav", ".aac", ".m4a", ".ogg"})
	assetsAudio := scanMediaDir(rootDir, projPath, filepath.Join("assets", "audio"), []string{".mp3", ".wav", ".aac", ".m4a", ".ogg"})

	allAudio := append([]MediaFile(nil), narrationFiles...)
	allAudio = append(allAudio, voiceSamples...)
	allAudio = append(allAudio, assetsAudio...)
	details.Narration = allAudio
	if len(allAudio) > 0 {
		details.Stages.Voiceover = true
	}

	// 4. Read Remotion Props
	remotionPath := filepath.Join(projPath, "remotion_props.json")
	if remotionBytes, err := os.ReadFile(remotionPath); err == nil {
		var props any
		if err := json.Unmarshal(remotionBytes, &props); err == nil {
			details.RemotionProps = props
			details.Stages.Composition = true
			if len(details.Beats) == 0 {
				details.Beats = extractBeatsFromRemotionProps(props)
			}
		}
	}

	// 5. Scan QA and Review Frames
	details.QAFrames = scanMediaDir(rootDir, projPath, "qa", []string{".png", ".jpg", ".jpeg", ".webp"})
	reviewFinal := scanMediaDir(rootDir, projPath, filepath.Join("review", "final-frames"), []string{".png", ".jpg", ".jpeg", ".webp"})
	reviewSource := scanMediaDir(rootDir, projPath, filepath.Join("review", "source-frames"), []string{".png", ".jpg", ".jpeg", ".webp"})
	details.ReviewFrames = append(reviewFinal, reviewSource...)

	// 6. Read Review Report
	reportPath := filepath.Join(projPath, "review", "report.json")
	if reportBytes, err := os.ReadFile(reportPath); err == nil {
		var report any
		if err := json.Unmarshal(reportBytes, &report); err == nil {
			details.ReviewReport = report
			details.Stages.Review = true
		}
	}
	if len(details.ReviewFrames) > 0 || len(details.QAFrames) > 0 {
		details.Stages.Review = true
	}

	// 7. Locate Master Video & Thumbnail
	details.VideoPath, details.VideoURL = findMasterVideo(rootDir, projPath, slug)
	if details.VideoPath != "" {
		if strings.HasSuffix(details.VideoPath, "final.mp4") || strings.HasSuffix(details.VideoPath, "video.mp4") {
			details.Stages.Master = true
		}
		details.Stages.Composition = true
	}

	details.ThumbnailPath, details.ThumbnailURL = findThumbnail(rootDir, projPath, slug)

	// Update LastModified from latest file
	if latest := findLatestModTime(projPath); !latest.IsZero() && latest.After(details.LastModified) {
		details.LastModified = latest
	}

	return details, nil
}

func scanProjectSummary(rootDir, slug, projPath string) (ProjectSummary, error) {
	info, err := os.Stat(projPath)
	if err != nil {
		return ProjectSummary{}, err
	}

	summary := ProjectSummary{
		Slug:         slug,
		Name:         slugToTitle(slug),
		Path:         projPath,
		LastModified: info.ModTime(),
	}

	// Check Brief
	briefPath := filepath.Join(projPath, "brief.md")
	if b, err := os.ReadFile(briefPath); err == nil {
		summary.Stages.Brief = true
		if title := extractMarkdownTitle(string(b)); title != "" {
			summary.Name = title
		}
	}

	// Check Script
	scriptPath := filepath.Join(projPath, "script.md")
	if s, err := os.ReadFile(scriptPath); err == nil {
		summary.Stages.Script = true
		if summary.Name == slugToTitle(slug) {
			if title := extractMarkdownTitle(string(s)); title != "" {
				summary.Name = title
			}
		}
	}

	// Check Voiceover Audio
	if hasFilesWithExt(filepath.Join(projPath, "narration"), []string{".mp3", ".wav", ".aac", ".m4a"}) ||
		hasFilesWithExt(filepath.Join(projPath, "voice-samples"), []string{".mp3", ".wav", ".aac", ".m4a"}) ||
		hasFilesWithExt(filepath.Join(projPath, "assets", "audio"), []string{".mp3", ".wav", ".aac", ".m4a"}) {
		summary.Stages.Voiceover = true
	}

	// Check Composition
	if fileExists(filepath.Join(projPath, "remotion_props.json")) ||
		fileExists(filepath.Join(projPath, "artifacts", "edit.json")) ||
		hasFilesWithExt(filepath.Join(projPath, "renders"), []string{".mp4"}) {
		summary.Stages.Composition = true
	}

	// Check Review
	if fileExists(filepath.Join(projPath, "review", "report.json")) ||
		hasFilesWithExt(filepath.Join(projPath, "qa"), []string{".png", ".jpg", ".jpeg", ".webp"}) ||
		hasFilesWithExt(filepath.Join(projPath, "review", "final-frames"), []string{".png", ".jpg", ".jpeg", ".webp"}) ||
		hasFilesWithExt(filepath.Join(projPath, "review", "source-frames"), []string{".png", ".jpg", ".jpeg", ".webp"}) {
		summary.Stages.Review = true
	}

	// Check Master Render
	finalVideo, videoURL := findMasterVideo(rootDir, projPath, slug)
	if finalVideo != "" {
		summary.VideoURL = videoURL
		if strings.HasSuffix(finalVideo, "final.mp4") || strings.HasSuffix(finalVideo, "video.mp4") {
			summary.Stages.Master = true
		}
		summary.Stages.Composition = true
	}

	_, thumbURL := findThumbnail(rootDir, projPath, slug)
	summary.ThumbnailURL = thumbURL

	if latest := findLatestModTime(projPath); !latest.IsZero() && latest.After(summary.LastModified) {
		summary.LastModified = latest
	}

	return summary, nil
}

func scanMediaDir(rootDir, projPath, subDir string, exts []string) []MediaFile {
	dirPath := filepath.Join(projPath, subDir)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil
	}

	var results []MediaFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !containsExt(exts, ext) {
			continue
		}
		fullPath := filepath.Join(dirPath, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}

		relPath, _ := filepath.Rel(rootDir, fullPath)
		urlPath := "/api/media/" + filepath.ToSlash(relPath)

		results = append(results, MediaFile{
			Name:         e.Name(),
			Path:         fullPath,
			RelativePath: filepath.ToSlash(relPath),
			URL:          urlPath,
			Size:         info.Size(),
			LastModified: info.ModTime(),
		})
	}
	return results
}

func findMasterVideo(rootDir, projPath, slug string) (string, string) {
	candidates := []string{
		filepath.Join(projPath, "renders", "final.mp4"),
		filepath.Join(projPath, "renders", "video.mp4"),
		filepath.Join(projPath, "renders", "montage_vo.mp4"),
		filepath.Join(projPath, "renders", "montage.mp4"),
		filepath.Join(projPath, "renders", "edit.mp4"),
	}

	for _, c := range candidates {
		if fileExists(c) {
			rel, _ := filepath.Rel(rootDir, c)
			return c, "/api/media/" + filepath.ToSlash(rel)
		}
	}

	// Fallback to any .mp4 in renders/
	rendersDir := filepath.Join(projPath, "renders")
	if entries, err := os.ReadDir(rendersDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".mp4") {
				p := filepath.Join(rendersDir, e.Name())
				rel, _ := filepath.Rel(rootDir, p)
				return p, "/api/media/" + filepath.ToSlash(rel)
			}
		}
	}

	return "", ""
}

func findThumbnail(rootDir, projPath, slug string) (string, string) {
	searchDirs := []string{
		filepath.Join(projPath, "review", "final-frames"),
		filepath.Join(projPath, "qa"),
		filepath.Join(projPath, "review", "source-frames"),
		filepath.Join(projPath, "assets", "raw"),
	}

	imgExts := []string{".jpg", ".jpeg", ".png", ".webp"}

	for _, dir := range searchDirs {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && containsExt(imgExts, strings.ToLower(filepath.Ext(e.Name()))) {
					p := filepath.Join(dir, e.Name())
					rel, _ := filepath.Rel(rootDir, p)
					return p, "/api/media/" + filepath.ToSlash(rel)
				}
			}
		}
	}

	return "", ""
}

func findLatestModTime(dir string) time.Time {
	var latest time.Time
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && d != nil {
			if info, err := d.Info(); err == nil {
				if info.ModTime().After(latest) {
					latest = info.ModTime()
				}
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

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func hasFilesWithExt(dir string, exts []string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && containsExt(exts, strings.ToLower(filepath.Ext(e.Name()))) {
			return true
		}
	}
	return false
}

func containsExt(exts []string, ext string) bool {
	for _, e := range exts {
		if e == ext {
			return true
		}
	}
	return false
}
