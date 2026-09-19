package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Run the actual entry point in a child so exit codes and accidental writes are tested.
func TestCLIProcess(t *testing.T) {
	if os.Getenv("FACET_CLI_TEST_PROCESS") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"facet"}, os.Args[i+1:]...)
			main()
			os.Exit(0)
		}
	}
	os.Exit(99)
}

func runCLI(t *testing.T, dir, home, path string, args ...string) (string, int) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, append([]string{"-test.run=^TestCLIProcess$", "--"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "FACET_CLI_TEST_PROCESS=1", "HOME="+home, "USERPROFILE="+home, "APPDATA="+home, "LOCALAPPDATA="+home, "PATH="+path)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(out), exitErr.ExitCode()
	}
	t.Fatal(err)
	return "", -1
}

func TestHelpAndInvalidInputsDoNotWrite(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		ok   bool
		want string
	}{
		{"help", []string{"--help"}, true, "Usage:"},
		{"init-help", []string{"init", "--help"}, true, "Usage: facet init"},
		{"init-short-help", []string{"init", "-h"}, true, "Usage: facet init"},
		{"init-target-help", []string{"init", "new-project", "--help"}, true, "Usage: facet init"},
		{"doctor-help", []string{"doctor", "--help"}, true, "Usage of doctor"},
		{"ui-help", []string{"ui", "--help"}, true, "Usage of ui"},
		{"command", []string{"bogus"}, false, "Unknown command"},
		{"init-flag", []string{"init", "new-project", "--bogus"}, false, "unknown argument"},
		{"init-engine", []string{"init", "new-project", "--engine", "bogus"}, false, "unknown engine"},
		{"init-empty-engine", []string{"init", "--engine="}, false, "unknown engine"},
		{"init-missing-engine", []string{"init", "--engine"}, false, "requires"},
		{"init-flag-engine", []string{"init", "--engine", "--no-launch"}, false, "requires"},
		{"init-extra-target", []string{"init", "one", "two"}, false, "unknown argument"},
		{"doctor-flag", []string{"doctor", "--bogus"}, false, "flag provided but not defined"},
		{"ui-flag", []string{"ui", "--bogus"}, false, "flag provided but not defined"},
		{"ui-argument", []string{"ui", "bogus"}, false, "unexpected arguments"},
		{"version-flag", []string{"version", "--bogus"}, false, "unexpected arguments"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, home := t.TempDir(), t.TempDir()
			out, code := runCLI(t, dir, home, "", tc.args...)
			if (code == 0) != tc.ok || !strings.Contains(out, tc.want) {
				t.Fatalf("exit=%d output=%s", code, out)
			}
			for _, root := range []string{dir, home} {
				entries, err := os.ReadDir(root)
				if err != nil || len(entries) != 0 {
					t.Fatalf("unexpected writes in %s: %v (%v)", root, entries, err)
				}
			}
		})
	}
}

func TestLaunchErrorsPropagate(t *testing.T) {
	for _, missing := range []bool{true, false} {
		t.Run(map[bool]string{true: "missing", false: "exit-code"}[missing], func(t *testing.T) {
			dir, home, bin := t.TempDir(), t.TempDir(), t.TempDir()
			bin = filepath.Join(bin, "agent bin")
			if err := os.Mkdir(bin, 0755); err != nil {
				t.Fatal(err)
			}
			want := 1
			if !missing {
				name, body := "opencode", "#!/bin/sh\nexit 7\n"
				if runtime.GOOS == "windows" {
					name, body = "opencode.cmd", "@exit /b 7\r\n"
				}
				if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0755); err != nil {
					t.Fatal(err)
				}
				want = 7
			}
			out, code := runCLI(t, dir, home, bin, "init", "project", "--engine", "opencode")
			if code != want || !strings.Contains(out, "Launch error:") {
				t.Fatalf("wanted exit %d; got %d: %s", want, code, out)
			}
		})
	}
}

func TestModuleJobStateDoesNotCrossProcesses(t *testing.T) {
	dir, home := t.TempDir(), t.TempDir()
	out, code := runCLI(t, dir, home, "", "module", "invoke", "creative.jobs.status", "--input",
		`{"request_id":"req_poll","job_id":"job_from_another_process"}`)
	if code == 0 {
		t.Fatalf("fresh process unexpectedly knew job: %s", out)
	}
	var polled struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &polled); err != nil {
		t.Fatalf("decode poll refusal: %v\n%s", err, out)
	}
	if polled.Error.Code != "unknown_job" {
		t.Fatalf("poll code=%q, want unknown_job: %s", polled.Error.Code, out)
	}
}
