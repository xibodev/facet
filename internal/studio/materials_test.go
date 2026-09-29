package studio

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestMaterialEditorRejectsStaleAndInvalidWrites(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "props.json")
	if err := os.WriteFile(file, []byte(`{"title":"original"}`), 0600); err != nil {
		t.Fatal(err)
	}
	s := NewServer(root)
	for _, test := range []struct {
		original, content string
		status            int
	}{{"stale", `{}`, 409}, {`{"title":"original"}`, `{broken`, 400}, {`{"title":"original"}`, `{"title":"revised"}`, 200}} {
		body, _ := json.Marshal(map[string]any{"dir": root, "path": "props.json", "original": test.original, "content": test.content})
		r := newSecurityRequest("POST", "/api/materials", string(body))
		r.Header.Set(sessionTokenHeader, s.sessionToken)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != test.status {
			t.Fatalf("status %d want %d: %s", w.Code, test.status, w.Body.String())
		}
	}
	data, _ := os.ReadFile(file)
	if string(data) != `{"title":"revised"}` {
		t.Fatalf("unexpected document %s", data)
	}
}
