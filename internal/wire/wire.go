// Package wire installs the Facet bundle into an agentic CLI's native
// user-level or project-level locations and registers Facet's MCP server
// through that CLI's own configuration mechanism.
//
// Wiring is the user's decision, like installing a plugin: nothing here runs
// unless `facet wire` is invoked. The CLI keeps ownership of approvals and
// permissions, and its instruction files (AGENTS.md, CLAUDE.md, ...) are never
// edited. Everything facet wire writes is recorded with its digest in
// ~/.facet/wiring.json; only recorded, unmodified items are ever replaced or
// removed.
package wire

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/xibodev/facet/internal/bundle"
)

const usage = `Usage:
  facet wire <cli>[,<cli>...]|all [--scope user|project] [--project DIR] [--dry-run]
  facet wire --remove <cli>[,<cli>...]|all [--scope user|project] [--project DIR] [--dry-run]
  facet wire --status

Wires Facet into an agentic CLI: installs the facet skill, one facet-<pack>
skill per production-method pack, and the facet-creative agent where the CLI
supports agents, then registers the MCP server "facet" (the facet executable
started with "mcp") through the CLI's own configuration. Instruction files such
as AGENTS.md or CLAUDE.md are never edited.

CLIs: claude, codex, copilot, opencode, or all.

Options:
  --scope user|project  where the CLI should find Facet (default: user)
  --project DIR         the project for project scope (default: the current
                        directory); implies --scope project
  --dry-run             print the exact actions without changing anything
  --remove              reverse a wiring; only recorded, unmodified items are
                        removed
  --status              report every wiring and any drift

Everything written is recorded in ~/.facet/wiring.json. Existing files that
facet wire did not write, or that changed since it wrote them, are never
overwritten or deleted.
`

// commandTimeout bounds every CLI subprocess.
const commandTimeout = 60 * time.Second

// env is the process context wiring runs in.
type env struct {
	home    string
	cwd     string
	getenv  func(string) string
	exe     string // facet executable registered as the MCP server
	version string
	out     io.Writer
	errOut  io.Writer
	now     func() time.Time
}

func newEnv(version string, stdout, stderr io.Writer) (*env, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil, fmt.Errorf("cannot determine the home directory: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	exe, err := StableExecutable(home)
	if err != nil {
		return nil, fmt.Errorf("cannot determine the facet executable: %w", err)
	}
	return &env{
		home:    filepath.Clean(home),
		cwd:     cwd,
		getenv:  os.Getenv,
		exe:     exe,
		version: version,
		out:     stdout,
		errOut:  stderr,
		now:     func() time.Time { return time.Now().UTC() },
	}, nil
}

func (e *env) registryPath() string { return RegistryPath(e.home) }

// StableExecutable returns the facet executable to register with CLIs:
// ~/.facet/current/bin/facet[.exe] when it exists, so registrations survive
// upgrades of the active runtime, otherwise the running executable.
func StableExecutable(home string) (string, error) {
	if home != "" {
		if current := CurrentExecutable(home); current != "" {
			return current, nil
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Clean(exe), nil
}

// CurrentExecutable returns ~/.facet/current/bin/facet[.exe] when it exists.
func CurrentExecutable(home string) string {
	name := "facet"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(home, ".facet", "current", "bin", name)
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		return path
	}
	return ""
}

type mode int

const (
	modeInstall mode = iota
	modeRemove
	modeStatus
)

type request struct {
	mode     mode
	targets  []bundle.Target
	scope    bundle.Scope
	scopeSet bool
	project  string
	dryRun   bool
}

type usageError struct{ msg string }

func (u usageError) Error() string { return u.msg }

// parseArgs accepts options before or after the CLI names.
func parseArgs(args []string) (*request, bool, error) {
	req := &request{mode: modeInstall, scope: bundle.ScopeUser}
	var names []string
	projectSet := false
	value := func(i *int, flag string) (string, error) {
		if *i+1 >= len(args) {
			return "", usageError{flag + " requires a value"}
		}
		*i++
		return args[*i], nil
	}
	removeSet, statusSet := false, false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, inline, hasInline := strings.Cut(arg, "=")
		if !strings.HasPrefix(arg, "-") {
			names = append(names, arg)
			continue
		}
		switch strings.TrimLeft(name, "-") {
		case "h", "help":
			return nil, true, nil
		case "dry-run":
			if hasInline {
				return nil, false, usageError{"--dry-run takes no value"}
			}
			req.dryRun = true
		case "remove":
			if hasInline {
				return nil, false, usageError{"--remove takes no value"}
			}
			removeSet = true
		case "status":
			if hasInline {
				return nil, false, usageError{"--status takes no value"}
			}
			statusSet = true
		case "scope":
			v := inline
			if !hasInline {
				var err error
				if v, err = value(&i, "--scope"); err != nil {
					return nil, false, err
				}
			}
			scope, err := bundle.ParseScope(v)
			if err != nil {
				return nil, false, usageError{err.Error()}
			}
			req.scope, req.scopeSet = scope, true
		case "project":
			v := inline
			if !hasInline {
				var err error
				if v, err = value(&i, "--project"); err != nil {
					return nil, false, err
				}
			}
			if strings.TrimSpace(v) == "" {
				return nil, false, usageError{"--project requires a directory"}
			}
			req.project, projectSet = v, true
		default:
			return nil, false, usageError{fmt.Sprintf("unknown option %s", arg)}
		}
	}
	switch {
	case statusSet && (removeSet || req.dryRun || len(names) > 0 || req.scopeSet || projectSet):
		return nil, false, usageError{"--status takes no other arguments"}
	case statusSet:
		req.mode = modeStatus
		return req, false, nil
	case removeSet:
		req.mode = modeRemove
	}
	if len(names) == 0 {
		return nil, false, usageError{"name the CLI to wire: claude, codex, copilot, opencode, or all"}
	}
	targets, err := bundle.ParseTargets(names...)
	if err != nil {
		return nil, false, usageError{err.Error()}
	}
	req.targets = targets
	if projectSet {
		if req.scopeSet && req.scope != bundle.ScopeProject {
			return nil, false, usageError{"--project applies only to --scope project"}
		}
		req.scope = bundle.ScopeProject
	}
	return req, false, nil
}

// CLI implements `facet wire`. It returns the process exit code.
func CLI(args []string, stdout, stderr io.Writer, version string) int {
	req, help, err := parseArgs(args)
	if help {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "wire: %v\n\n%s", err, usage)
		return 2
	}
	e, err := newEnv(version, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "wire: %v\n", err)
		return 1
	}
	switch req.mode {
	case modeStatus:
		return e.status()
	case modeRemove:
		return e.remove(req)
	default:
		return e.install(req)
	}
}

// resolveProject returns the absolute project directory for project scope.
// Removal accepts a project directory that no longer exists, so a wiring can
// always be reversed.
func (e *env) resolveProject(req *request, mustExist bool) (string, error) {
	if req.scope != bundle.ScopeProject {
		return "", nil
	}
	dir := req.project
	if dir == "" {
		dir = e.cwd
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	switch {
	case err != nil && (mustExist || !errors.Is(err, fs.ErrNotExist)):
		return "", fmt.Errorf("project directory %s: %w", abs, err)
	case err == nil && !info.IsDir():
		return "", fmt.Errorf("project directory %s is not a directory", abs)
	}
	if samePath(abs, e.home) {
		return "", errors.New("the project directory is your home directory, where project-scope files would act user-wide; use --scope user instead")
	}
	return abs, nil
}

func (e *env) install(req *request) int {
	project, err := e.resolveProject(req, true)
	if err != nil {
		fmt.Fprintf(e.errOut, "wire: %v\n", err)
		return 1
	}
	reg, err := LoadRegistry(e.registryPath())
	if err != nil {
		fmt.Fprintf(e.errOut, "wire: %v\n", err)
		return 1
	}
	var plans []*plan
	failed := false
	for _, t := range req.targets {
		p := e.planInstall(reg, t, req.scope, project)
		plans = append(plans, p)
		if len(p.problems) > 0 {
			failed = true
		}
	}
	if req.dryRun {
		fmt.Fprintln(e.out, "Dry run: nothing will be changed.")
	}
	if e.temporaryExecutable() {
		fmt.Fprintf(e.errOut, "wire: warning: %s looks temporary; the CLI will lose the MCP server when it is deleted. Install Facet and rerun facet wire from the installed copy.\n", e.exe)
	}
	if failed {
		for _, p := range plans {
			if len(p.problems) == 0 {
				continue
			}
			fmt.Fprintf(e.errOut, "%s: cannot wire:\n", p.label())
			for _, problem := range p.problems {
				fmt.Fprintf(e.errOut, "  - %s\n", problem)
			}
		}
		fmt.Fprintln(e.errOut, "Nothing was changed.")
		return 1
	}
	for _, p := range plans {
		e.printPlan(p)
		if req.dryRun {
			for _, note := range p.notes {
				fmt.Fprintf(e.out, "  note:      %s\n", note)
			}
			continue
		}
		before := len(p.mcp.warnings)
		if err := e.applyInstall(reg, p); err != nil {
			fmt.Fprintf(e.errOut, "%s: %v\n", p.label(), err)
			return 1
		}
		for _, warning := range p.mcp.warnings[before:] {
			fmt.Fprintf(e.errOut, "  warning: %s\n", warning)
		}
		fmt.Fprintf(e.out, "%s: wired; MCP server %q runs %s\n", p.label(), bundle.MCPServerName, commandLine(append([]string{e.exe}, bundle.MCPServerArgs()...)))
		for _, note := range p.notes {
			fmt.Fprintf(e.out, "  note:      %s\n", note)
		}
	}
	return 0
}

func (e *env) remove(req *request) int {
	project, err := e.resolveProject(req, false)
	if err != nil {
		fmt.Fprintf(e.errOut, "wire: %v\n", err)
		return 1
	}
	reg, err := LoadRegistry(e.registryPath())
	if err != nil {
		fmt.Fprintf(e.errOut, "wire: %v\n", err)
		return 1
	}
	if req.dryRun {
		fmt.Fprintln(e.out, "Dry run: nothing will be changed.")
	}
	code := 0
	for _, t := range req.targets {
		w := reg.find(string(t), string(req.scope), project)
		label := (&Wiring{CLI: string(t), Scope: string(req.scope), Project: project}).label()
		if w == nil {
			fmt.Fprintf(e.out, "%s: not wired by facet wire; nothing to remove\n", label)
			continue
		}
		p := e.planRemove(w)
		e.printPlan(p)
		if req.dryRun {
			continue
		}
		if err := e.applyRemove(reg, p); err != nil {
			fmt.Fprintf(e.errOut, "%s: %v\n", label, err)
			code = 1
			continue
		}
		fmt.Fprintf(e.out, "%s: removed\n", label)
	}
	return code
}

// temporaryExecutable reports whether the registered executable lives in the
// temporary directory, as a `go run` build does.
func (e *env) temporaryExecutable() bool {
	tmp := os.TempDir()
	if tmp == "" {
		return false
	}
	rel, err := filepath.Rel(pathKey(tmp), pathKey(e.exe))
	return err == nil && !strings.HasPrefix(rel, "..") && rel != "." && CurrentExecutable(e.home) == ""
}

// commandLine renders argv for people, quoting arguments with spaces.
func commandLine(argv []string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		if a == "" || strings.ContainsAny(a, " \t\"'") {
			parts[i] = `"` + strings.ReplaceAll(a, `"`, `\"`) + `"`
		} else {
			parts[i] = a
		}
	}
	return strings.Join(parts, " ")
}
