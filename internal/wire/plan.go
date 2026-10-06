package wire

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xibodev/facet/internal/bundle"
)

// fileStep is one projected file and what wiring does with it.
type fileStep struct {
	path    string
	kind    string
	op      string // create | update | unchanged | adopt
	content []byte
	digest  string
	prev    []string // digests that were Facet's before this change
}

// staleStep is a recorded file that is no longer projected, or is being
// removed.
type staleStep struct {
	file OwnedFile
	op   string // remove | keep | gone
}

// plan is everything one wiring change will do, computed before anything is
// written so conflicts are reported with nothing changed.
type plan struct {
	cli      bundle.Target
	scope    bundle.Scope
	project  string
	root     string
	removing bool
	prev     *Wiring
	files    []fileStep
	stale    []staleStep
	mkdirs   []string // directories to create, outermost first
	rmdirs   []string // recorded directories to delete when empty, deepest first
	mcp      mcpStep
	problems []string
	notes    []string
	warnings []string
}

func (p *plan) label() string {
	return (&Wiring{CLI: string(p.cli), Scope: string(p.scope), Project: p.project}).label()
}

// changed reports whether applying the plan modifies anything.
func (p *plan) changed(e *env) bool {
	if p.prev == nil || p.prev.Pending || p.removing {
		return true
	}
	if p.prev.FacetVersion != e.version || !samePath(p.prev.Executable, e.exe) || len(p.mkdirs) > 0 {
		return true
	}
	for _, f := range p.files {
		if f.op != "unchanged" {
			return true
		}
	}
	for _, s := range p.stale {
		if s.op != "" {
			return true
		}
	}
	return p.mcp.op != "unchanged"
}

func fileDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// dirSet collects directories that must be created, outermost first.
type dirSet struct {
	seen map[string]bool
	list []string
}

func (d *dirSet) need(dir string) {
	if d.seen == nil {
		d.seen = map[string]bool{}
	}
	var chain []string
	for cur := filepath.Clean(dir); ; {
		if d.seen[pathKey(cur)] {
			break
		}
		if _, err := os.Lstat(cur); err == nil {
			break
		}
		chain = append(chain, cur)
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	for i := len(chain) - 1; i >= 0; i-- {
		d.seen[pathKey(chain[i])] = true
		d.list = append(d.list, chain[i])
	}
}

// ownersExcept indexes files owned by wirings other than self.
func ownersExcept(reg *Registry, self *Wiring) map[string]string {
	owners := map[string]string{}
	for _, w := range reg.Wirings {
		if w == self {
			continue
		}
		for _, f := range w.Files {
			owners[pathKey(f.Path)] = w.label()
		}
	}
	return owners
}

func (e *env) planInstall(reg *Registry, t bundle.Target, scope bundle.Scope, project string) *plan {
	p := &plan{cli: t, scope: scope, project: project}
	base := e.home
	if scope == bundle.ScopeProject {
		base = project
	}
	p.root = bundle.Root(t, scope, base, e.getenv)
	p.prev = reg.find(string(t), string(scope), project)
	files, err := bundle.Files(t)
	if err != nil {
		p.problems = append(p.problems, err.Error())
		return p
	}
	owners := ownersExcept(reg, p.prev)
	recorded := map[string]OwnedFile{}
	if p.prev != nil {
		for _, f := range p.prev.Files {
			recorded[pathKey(f.Path)] = f
		}
	}
	wanted := map[string]bool{}
	var dirs dirSet
	for _, f := range files {
		path := filepath.Join(p.root, filepath.FromSlash(f.Path))
		key := pathKey(path)
		wanted[key] = true
		step := fileStep{path: path, kind: f.Kind, content: f.Content, digest: f.Digest()}
		if owner, ok := owners[key]; ok {
			p.problems = append(p.problems, fmt.Sprintf("%s is recorded for the %s wiring", path, owner))
			continue
		}
		own, wasOwned := recorded[key]
		info, err := os.Lstat(path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			step.op = "create"
			dirs.need(filepath.Dir(path))
		case err != nil:
			p.problems = append(p.problems, err.Error())
			continue
		case !info.Mode().IsRegular():
			p.problems = append(p.problems, fmt.Sprintf("%s exists and is not a regular file", path))
			continue
		default:
			got, err := fileDigest(path)
			if err != nil {
				p.problems = append(p.problems, err.Error())
				continue
			}
			switch {
			case got == step.digest && wasOwned:
				step.op = "unchanged"
			case got == step.digest:
				// Byte-identical to what Facet installs: nothing a person
				// wrote can be lost by managing it.
				step.op = "adopt"
			case wasOwned && own.owns(got):
				step.op = "update"
				step.prev = []string{got}
			case wasOwned:
				p.problems = append(p.problems, fmt.Sprintf("%s changed after facet wire installed it; move your version aside or restore it, then rerun", path))
				continue
			default:
				problem := fmt.Sprintf("%s exists and was not installed by facet wire; move it aside, then rerun", path)
				if hint := legacyHint(path); hint != "" {
					problem += "\n    " + hint
				}
				p.problems = append(p.problems, problem)
				continue
			}
		}
		p.files = append(p.files, step)
	}
	if p.prev != nil {
		for _, f := range p.prev.Files {
			if wanted[pathKey(f.Path)] {
				continue
			}
			st := staleStep{file: f}
			got, err := fileDigest(f.Path)
			switch {
			case errors.Is(err, fs.ErrNotExist):
				st.op = "gone"
			case err == nil && f.owns(got):
				st.op = "remove"
			default:
				st.op = "keep"
				p.warnings = append(p.warnings, fmt.Sprintf("%s is no longer installed by Facet but changed after facet wire wrote it; it is left in place and no longer managed", f.Path))
			}
			p.stale = append(p.stale, st)
		}
	}

	var prevMCP *MCPRecord
	if p.prev != nil {
		prevMCP = p.prev.MCP
	}
	step, problems := e.planMCPInstall(t, scope, project, p.root, prevMCP)
	p.mcp = step
	p.problems = append(p.problems, problems...)
	if step.file != "" && step.after != nil && !step.existed {
		dirs.need(filepath.Dir(step.file))
	}
	p.mkdirs = dirs.list
	p.notes = append(p.notes, e.wiringNotes(t, scope, project)...)
	return p
}

func (e *env) wiringNotes(t bundle.Target, scope bundle.Scope, project string) []string {
	var notes []string
	switch {
	case t == bundle.TargetClaude && scope == bundle.ScopeProject:
		notes = append(notes, "Claude Code asks you to approve project MCP servers from .mcp.json the first time it starts in this project.")
	case t == bundle.TargetCodex && scope == bundle.ScopeProject:
		notes = append(notes, fmt.Sprintf("Codex reads %s only for projects you have marked as trusted.", filepath.Join(project, ".codex", "config.toml")))
	}
	notes = append(notes, fmt.Sprintf("Start a new %s session to load the skills and the MCP server.", t))
	return notes
}

// pendingRecord is saved before anything is written: it accepts both the
// previous and the new digest of every file, so an interrupted run never
// turns Facet's own files into "modified" ones.
func (p *plan) pendingRecord(e *env) *Wiring {
	w := &Wiring{CLI: string(p.cli), Scope: string(p.scope), Project: p.project, Root: p.root,
		FacetVersion: e.version, Executable: e.exe, WiredAt: e.now().Format("2006-01-02T15:04:05Z"), Pending: true}
	if p.prev != nil {
		w.FacetVersion, w.Executable, w.WiredAt = p.prev.FacetVersion, p.prev.Executable, p.prev.WiredAt
		w.MCP = p.prev.MCP
		w.Dirs = append(w.Dirs, p.prev.Dirs...)
	}
	recorded := map[string]OwnedFile{}
	if p.prev != nil {
		for _, f := range p.prev.Files {
			recorded[pathKey(f.Path)] = f
		}
	}
	for _, f := range p.files {
		own := OwnedFile{Path: f.path, Kind: f.kind, Digest: f.digest}
		if old, ok := recorded[pathKey(f.path)]; ok && f.op == "update" {
			own.Alt = uniqueStrings(append(append(append([]string(nil), f.prev...), old.Digest), old.Alt...))
		}
		w.Files = append(w.Files, own)
	}
	for _, s := range p.stale {
		if s.op == "remove" {
			w.Files = append(w.Files, s.file)
		}
	}
	w.Dirs = uniquePaths(append(w.Dirs, p.mkdirs...))
	return w
}

// finalRecord describes the wiring once applied.
func (p *plan) finalRecord(e *env, mcp *MCPRecord) *Wiring {
	w := &Wiring{CLI: string(p.cli), Scope: string(p.scope), Project: p.project, Root: p.root,
		FacetVersion: e.version, Executable: e.exe, WiredAt: e.now().Format("2006-01-02T15:04:05Z"), MCP: mcp}
	for _, f := range p.files {
		w.Files = append(w.Files, OwnedFile{Path: f.path, Kind: f.kind, Digest: f.digest})
	}
	if p.prev != nil {
		w.Dirs = append(w.Dirs, p.prev.Dirs...)
	}
	w.Dirs = uniquePaths(append(w.Dirs, p.mkdirs...))
	return w
}

func (e *env) applyInstall(reg *Registry, p *plan) error {
	if !p.changed(e) {
		return nil
	}
	regPath := e.registryPath()
	reg.put(p.pendingRecord(e))
	if err := reg.Save(regPath); err != nil {
		return fmt.Errorf("recording the wiring in %s: %w", regPath, err)
	}
	for _, dir := range p.mkdirs {
		if err := os.Mkdir(dir, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
	}
	for _, f := range p.files {
		// Check again immediately before writing: never overwrite a file
		// that appeared or changed since the plan was made.
		switch f.op {
		case "create":
			if _, err := os.Lstat(f.path); err == nil {
				return fmt.Errorf("%s appeared while facet wire was running; nothing of it was changed, rerun", f.path)
			}
		case "update":
			got, err := fileDigest(f.path)
			if err != nil || !containsString(f.prev, got) {
				return fmt.Errorf("%s changed while facet wire was running; nothing of it was changed, rerun", f.path)
			}
		default:
			continue
		}
		if err := writeFileAtomic(f.path, f.content, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", f.path, err)
		}
	}
	for _, s := range p.stale {
		if s.op != "remove" {
			continue
		}
		if got, err := fileDigest(s.file.Path); err == nil && s.file.owns(got) {
			if err := os.Remove(s.file.Path); err != nil {
				return err
			}
		}
	}
	mcpErr := e.applyMCP(&p.mcp)
	mcp := p.mcp.record
	if mcpErr != nil && p.prev != nil {
		mcp = p.prev.MCP
	} else if mcpErr != nil {
		mcp = nil
	}
	reg.put(p.finalRecord(e, mcp))
	if err := reg.Save(regPath); err != nil {
		return fmt.Errorf("recording the wiring in %s: %w", regPath, err)
	}
	if mcpErr != nil {
		return fmt.Errorf("the skills are installed, but registering the MCP server failed: %w", mcpErr)
	}
	return nil
}

func (e *env) planRemove(w *Wiring) *plan {
	p := &plan{cli: bundle.Target(w.CLI), scope: bundle.Scope(w.Scope), project: w.Project, root: w.Root, removing: true, prev: w}
	for _, f := range w.Files {
		st := staleStep{file: f}
		got, err := fileDigest(f.Path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			st.op = "gone"
		case err == nil && f.owns(got):
			st.op = "remove"
		default:
			st.op = "keep"
			p.warnings = append(p.warnings, fmt.Sprintf("%s changed after facet wire installed it; it is left in place", f.Path))
		}
		p.stale = append(p.stale, st)
	}
	p.rmdirs = append([]string(nil), w.Dirs...)
	sort.Slice(p.rmdirs, func(i, j int) bool { return len(p.rmdirs[i]) > len(p.rmdirs[j]) })
	if w.MCP != nil {
		p.mcp = e.planMCPRemove(w.MCP)
	}
	return p
}

func (e *env) applyRemove(reg *Registry, p *plan) error {
	for _, s := range p.stale {
		if s.op != "remove" {
			continue
		}
		// Check again immediately before deleting.
		if got, err := fileDigest(s.file.Path); err == nil && s.file.owns(got) {
			if err := os.Remove(s.file.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
	}
	mcpErr := e.applyMCP(&p.mcp)
	if mcpErr == nil && p.mcp.op == "keep" && p.mcp.cliMissing {
		mcpErr = errors.New(strings.Join(p.mcp.warnings, "; "))
	}
	for _, dir := range p.rmdirs {
		_ = os.Remove(dir) // only empty directories are removed
	}
	if mcpErr != nil {
		// Keep what is needed to finish the removal later.
		w := *p.prev
		w.Files = nil
		var kept []string
		for _, dir := range p.prev.Dirs {
			if _, err := os.Lstat(dir); err == nil {
				kept = append(kept, dir)
			}
		}
		w.Dirs = kept
		reg.put(&w)
		if err := reg.Save(e.registryPath()); err != nil {
			return err
		}
		return fmt.Errorf("the files are removed, but unregistering the MCP server failed: %w", mcpErr)
	}
	reg.drop(p.prev.CLI, p.prev.Scope, p.prev.Project)
	return reg.Save(e.registryPath())
}

// printPlan lists every action in the order it runs. A dry run prints the
// same list and stops.
func (e *env) printPlan(p *plan) {
	w := e.out
	fmt.Fprintf(w, "%s -> %s\n", p.label(), p.root)
	line := func(verb, target string) { fmt.Fprintf(w, "  %-10s %s\n", verb, target) }
	for _, dir := range p.mkdirs {
		line("mkdir", dir)
	}
	unchanged := 0
	for _, f := range p.files {
		if f.op == "unchanged" {
			unchanged++
			continue
		}
		line(f.op, f.path)
	}
	if unchanged > 0 {
		line("unchanged", fmt.Sprintf("%d file(s) already installed", unchanged))
	}
	for _, s := range p.stale {
		switch s.op {
		case "remove":
			line("remove", s.file.Path)
		case "keep":
			line("keep", s.file.Path+" (changed since facet wire wrote it)")
		}
	}
	e.printMCP(p.mcp, line)
	for _, dir := range p.rmdirs {
		line("rmdir", dir+" (if empty)")
	}
	if p.changed(e) {
		line("record", e.registryPath())
	}
	for _, warning := range append(append([]string(nil), p.warnings...), p.mcp.warnings...) {
		fmt.Fprintf(w, "  warning: %s\n", warning)
	}
}

func containsString(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func uniquePaths(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s != "" && !seen[pathKey(s)] {
			seen[pathKey(s)] = true
			out = append(out, s)
		}
	}
	return out
}
