package module

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/xibodev/facet/internal/toolbox"
)

func TestCompatibilityAliasesProjectCanonicalToolIdentity(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input.mp4")
	if err := os.WriteFile(input, []byte("estimate only checks existence"), 0600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		alias     string
		canonical string
		input     map[string]any
	}{
		{
			alias: "audio_mixer", canonical: "audio_mix",
			input: map[string]any{
				"video": input, "source": map[string]any{"gain_db": -1},
				"duration": "video", "output": filepath.Join(root, "mixed.mp4"),
			},
		},
		{
			alias: "frame_sampler", canonical: "frame_sample",
			input: map[string]any{
				"input_path": input, "strategy": "count", "count": 2,
				"output_dir": filepath.Join(root, "frames"),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.alias, func(t *testing.T) {
			successBody, err := json.Marshal(map[string]any{"tool": tc.alias, "input": tc.input})
			if err != nil {
				t.Fatal(err)
			}
			estimated := Estimate(CapToolsEstimate, successBody)
			if !estimated.OK {
				t.Fatalf("alias estimate failed: %+v", estimated.Error)
			}
			result := estimated.Result.(map[string]any)
			if result["tool"] != tc.canonical {
				t.Fatalf("estimate projected tool %q, want canonical %q", result["tool"], tc.canonical)
			}

			runResult := project(
				OpInvoke, "run", "alias-run", CapToolsRun, tc.alias,
				toolbox.Envelope{
					OK: true, Tool: tc.canonical, Operation: "run",
					Result:    map[string]any{"status": "complete"},
					Warnings:  []string{},
					Execution: toolbox.Execution{Provider: "local"},
				},
				true,
			)
			if !runResult.OK {
				t.Fatalf("projected run result failed: %+v", runResult.Error)
			}
			if tool := runResult.Result.(map[string]any)["tool"]; tool != tc.canonical {
				t.Fatalf("run projected tool %q, want canonical %q", tool, tc.canonical)
			}

			errorBody := []byte(`{"tool":"` + tc.alias + `","input":{}}`)
			for name, env := range map[string]Envelope{
				"estimate": Estimate(CapToolsEstimate, errorBody),
				"run":      Invoke(CapToolsRun, errorBody),
			} {
				if env.OK || env.Error == nil {
					t.Fatalf("%s unexpectedly succeeded: %+v", name, env)
				}
				if env.Error.Details["tool"] != tc.canonical {
					t.Fatalf("%s error projected tool %q, want canonical %q",
						name, env.Error.Details["tool"], tc.canonical)
				}
			}
		})
	}
}
