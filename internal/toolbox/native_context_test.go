package toolbox

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNativeRunRejectsCancelledContextBeforeToolExecution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := RunContext(ctx, "subtitle_gen", []byte(`{"segments":[{"text":"must not write","start":0,"end":1}],"output_path":"must-not-write.srt"}`))
	if result.OK || result.Error == nil || result.Error.Code != "cancelled" {
		t.Fatalf("cancelled tool accepted: %+v", result)
	}
}

func TestNativeCancellationStopsRunningMediaProcess(t *testing.T) {
	if os.Getenv("FACET_CONTEXT_HELPER") == "1" {
		time.Sleep(20 * time.Second)
		return
	}
	root := t.TempDir()
	input := filepath.Join(root, "input.mp4")
	if err := os.WriteFile(input, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	// A real child process holds open until cancelled; no provider access.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t.Setenv("FACET_CONTEXT_HELPER", "1")
	SetBinaryPaths(map[string]string{"ffmpeg": os.Args[0]})
	defer SetBinaryPaths(nil)
	started := time.Now()
	go func() { time.Sleep(150 * time.Millisecond); cancel() }()
	_, err := runCommandContext(ctx, "ffmpeg", "-test.run=TestNativeCancellationStopsRunningMediaProcess")
	if err == nil || time.Since(started) > 3*time.Second {
		t.Fatalf("child process did not cancel promptly: %v", err)
	}
}

func TestSavedCompositionPreservesProfileDuringEstimate(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "props.json")
	if err := os.WriteFile(file, []byte(`{"width":640,"height":360,"fps":24,"duration_seconds":2,"cuts":[{"id":"title","type":"hero_title","text":"TEST","in_seconds":0,"out_seconds":2}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	request, _ := json.Marshal(map[string]any{"input_path": file})
	result, _, err := doVideoComposeContext(context.Background(), "estimate", request)
	if err != nil {
		t.Fatal(err)
	}
	if result.(map[string]any)["estimated_duration_seconds"] == nil {
		t.Fatal("saved composition did not resolve to render estimate")
	}
}

func TestSilentGraphicPropsDoesNotMuteDeclaredMedia(t *testing.T) {
	for _, test := range []struct {
		body   string
		silent bool
	}{
		{`{"cuts":[{"type":"hero_title","text":"Silent"}]}`, true},
		{`{"cuts":[{"type":"video","source":"clip.mp4"}]}`, false},
		{`{"cuts":[{"type":"hero_title","text":"Narrated"}],"audio":{"narration":{"src":"voice.mp3"}}}`, false},
	} {
		if got := silentGraphicProps([]byte(test.body)); got != test.silent {
			t.Fatalf("silentGraphicProps(%s)=%v", test.body, got)
		}
	}
}
