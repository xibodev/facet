package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestRoutesCLIIsMachineReadableAndReadOnly(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "list", args: []string{"routes", "list"}},
		{name: "describe", args: []string{"routes", "describe", "explainer"}},
		{name: "assess-inline", args: []string{"routes", "assess", "--input", `{"method":"source-edit","inputs":{"source_media":"clip.mp4"}}`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, home := t.TempDir(), t.TempDir()
			out, code := runCLI(t, dir, home, "", tc.args...)
			if code != 0 {
				t.Fatalf("exit=%d output=%s", code, out)
			}
			var envelope map[string]any
			if err := json.Unmarshal([]byte(out), &envelope); err != nil {
				t.Fatalf("not JSON: %v\n%s", err, out)
			}
			if envelope["ok"] != true {
				t.Fatalf("unexpected envelope: %#v", envelope)
			}
			for _, root := range []string{dir, home} {
				entries, err := os.ReadDir(root)
				if err != nil || len(entries) != 0 {
					t.Fatalf("routes command wrote in %s: %v (%v)", root, entries, err)
				}
			}
		})
	}
}

func TestRoutesCLIRejectsUnknownMethodsAndBadRequests(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{args: []string{"routes", "describe", "unknown"}, want: "unknown method"},
		{args: []string{"routes", "assess"}, want: "usage:"},
		{args: []string{"routes", "assess", "--input", `{"method":"unknown"}`}, want: "unknown method"},
	} {
		out, code := runCLI(t, t.TempDir(), t.TempDir(), "", tc.args...)
		if code == 0 || !strings.Contains(out, tc.want) {
			t.Fatalf("args=%v exit=%d output=%s", tc.args, code, out)
		}
		var envelope map[string]any
		if err := json.Unmarshal([]byte(out), &envelope); err != nil {
			t.Fatalf("error output is not JSON: %v\n%s", err, out)
		}
	}
}
