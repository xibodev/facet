package studio

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMediaRangeAndDownloadCatalogAndLocal(t *testing.T) {
	for _, external := range []bool{false, true} {
		root := t.TempDir()
		project := filepath.Join(root, "projects", "movie")
		if external {
			project = t.TempDir()
		}
		content := "0123456789abcdefghijklmnopqrstuvwxyz"
		name := "clip 100% #1.mp4"
		mustWriteTestFile(t, filepath.Join(project, "renders", name), content)
		slug := "movie"
		if external {
			p, err := RegisterOrUpdateProject("Catalog Movie", project, "codex", nil, root)
			if err != nil {
				t.Fatal(err)
			}
			slug = p.ID
		}
		details, err := GetProjectDetails(root, slug)
		if err != nil {
			t.Fatal(err)
		}
		server := NewServer(root)
		for _, tc := range []struct {
			rangeHeader, body, contentRange string
			status                          int
		}{
			{"", content, "", 200},
			{"bytes=0-9", content[:10], "bytes 0-9/36", 206},
			{"bytes=-5", content[len(content)-5:], "bytes 31-35/36", 206},
			{"bytes=36-", "", "bytes */36", 416},
		} {
			req := newSecurityRequest(http.MethodGet, details.PreviewVideoURL, "")
			req.Header.Set("Range", tc.rangeHeader)
			rec := httptest.NewRecorder()
			server.Handler().ServeHTTP(rec, req)
			if rec.Code != tc.status || rec.Header().Get("Content-Range") != tc.contentRange {
				t.Fatalf("range %q: %d %v", tc.rangeHeader, rec.Code, rec.Header())
			}
			if tc.status != 416 && (rec.Body.String() != tc.body || rec.Header().Get("Content-Type") != "video/mp4") {
				t.Fatalf("incorrect media response: %v %q", rec.Header(), rec.Body.String())
			}
			if tc.rangeHeader == "" && sha256.Sum256(rec.Body.Bytes()) != sha256.Sum256([]byte(content)) {
				t.Fatal("download hash mismatch")
			}
		}
	}
}

func TestProductionScanIgnoresDependencyTreeAndSeesUpdates(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "projects", "movie")
	mustWriteTestFile(t, filepath.Join(project, "renders", "final.mp4"), "first")
	unrelated := filepath.Join(project, "node_modules", "dependency", "index.js")
	mustWriteTestFile(t, unrelated, "not production evidence")
	future := time.Now().Add(24 * time.Hour)
	if err := os.Chtimes(unrelated, future, future); err != nil {
		t.Fatal(err)
	}
	list, err := ListProjects(root)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}
	if list[0].LastModified.After(time.Now().Add(time.Hour)) {
		t.Fatal("dependency tree influenced production scan")
	}
	version := list[0].VideoVersion
	mustWriteTestFile(t, filepath.Join(project, "renders", "final.mp4"), "replacement video")
	list, err = ListProjects(root)
	if err != nil || list[0].VideoVersion == version {
		t.Fatal("replacement not discovered")
	}
	if err := os.Remove(filepath.Join(project, "renders", "final.mp4")); err != nil {
		t.Fatal(err)
	}
	list, err = ListProjects(root)
	if err != nil || list[0].Stages.Master {
		t.Fatal("deleted output still present")
	}
}

func TestMediaRejectsCrossProjectSymlinkAndEncodedTraversal(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "projects", "one")
	other := filepath.Join(root, "projects", "two", "renders")
	mustWriteTestFile(t, filepath.Join(project, "brief.md"), "one")
	mustWriteTestFile(t, filepath.Join(other, "final.mp4"), "other project")
	server := NewServer(root)
	paths := []string{"/api/media/projects/one/%2e%2e/two/renders/final.mp4"}
	if makeTestSymlink(t, other, filepath.Join(project, "renders")) {
		paths = append(paths, "/api/media/projects/one/renders/final.mp4")
	}
	for _, path := range paths {
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, newSecurityRequest(http.MethodGet, path, ""))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("GET %s = %d", path, rec.Code)
		}
	}
}

func TestReviewToolDefaultFramesAndResolvedReportURL(t *testing.T) {
	root, project := t.TempDir(), t.TempDir()
	mustWriteTestFile(t, filepath.Join(project, "renders", "review_frames", "sample_001.jpg"), "fixture")
	mustWriteTestFile(t, filepath.Join(project, "review", "report.json"), `{"result":{"review_status":"warn"}}`)
	p, err := RegisterOrUpdateProject("Review", project, "codex", nil, root)
	if err != nil {
		t.Fatal(err)
	}
	d, err := GetProjectDetails(root, p.ID)
	if err != nil || len(d.ReviewFrames) != 1 || d.ReviewReportURL != "/api/media/catalog/review/review/report.json" {
		t.Fatalf("review: %#v %v", d, err)
	}
	rec := httptest.NewRecorder()
	NewServer(root).Handler().ServeHTTP(rec, newSecurityRequest(http.MethodGet, d.ReviewReportURL, ""))
	if rec.Code != 200 || !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("report link: %d %s", rec.Code, rec.Body)
	}
}

func TestSessionRecoveryIsBoundToProjectAndEngine(t *testing.T) {
	adapter := newStudioHelperAdapter(t, "success")
	sess := newStudioHelperSession(t, adapter)
	// Production newSession stores the canonical path, including Windows long names.
	dir, err := canonicalExistingPath(sess.Dir)
	if err != nil {
		t.Fatal(err)
	}
	sess.Dir = dir
	result := sess.runTurn(context.Background(), "first", func(turnEvent) error { return nil })
	if !result.ok {
		t.Fatal(result.reason)
	}
	server := NewServer(sess.Dir)
	server.sessions[sess.ID] = sess
	for _, tc := range []struct {
		dir, engine string
		status      int
	}{
		{sess.Dir, sess.Engine, 200}, {t.TempDir(), sess.Engine, 409}, {sess.Dir, "wrong", 409},
	} {
		q := url.Values{"session": {sess.ID}, "dir": {tc.dir}, "engine": {tc.engine}}
		req := newSecurityRequest(http.MethodGet, "/api/session?"+q.Encode(), "")
		req.Header.Set(sessionTokenHeader, server.sessionToken)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("recovery %v: %d %s", tc, rec.Code, rec.Body)
		}
		if tc.status == 200 && !strings.Contains(rec.Body.String(), "helper-native-id") {
			t.Fatal("native context lost")
		}
	}
	result = sess.runTurn(context.Background(), "second", func(turnEvent) error { return nil })
	if !result.ok || adapter.buildHistory()[1][3] != "helper-native-id" {
		t.Fatal("native conversation not resumed")
	}
}
