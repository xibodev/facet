package module

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xibodev/facet/internal/config"
	"github.com/xibodev/facet/internal/toolbox"
)

// toolCatalog asks the live toolbox for its catalog. The module keeps no second
// list, so drift between the `module` and `tools` surfaces is structurally
// impossible: both read the same registry.
func toolCatalog() ([]map[string]any, error) {
	env, ok := toolbox.CLI([]string{"tools", "list"})
	if !ok {
		return nil, errFromToolbox(env)
	}
	res, _ := env.Result.(map[string]any)
	raw, _ := res["tools"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// Describe builds the descriptor from live toolbox state.
func Describe(version string) Envelope {
	reqID := newRequestID()
	tools, err := toolCatalog()
	if err != nil {
		return fail(OpDescribe, reqID, "toolbox_unavailable",
			"tool catalog could not be read",
			map[string]any{"error": err.Error()}, true)
	}

	reqSchemas := map[string]any{}
	resSchemas := map[string]any{}
	warnings := []string{}

	// Capability-level schemas first: every ID a capability references must
	// resolve to a present key, or the host cannot validate a single request or
	// result. Tool-keyed schemas are added alongside them below and describe
	// the `input` payload rather than the capability envelope.
	capReq, capRes := capabilitySchemas()
	for id, doc := range capReq {
		reqSchemas[id] = doc
	}
	for id, doc := range capRes {
		resSchemas[id] = doc
	}

	// Per-tool request/result schemas are NOT inlined here. They are served by
	// creative.tools.describe, which returns one tool's schemas for ~1.7KB on
	// demand — and it is the same toolbox describe this loop used to call, so
	// there is no second source to drift.
	//
	// Inlining all of them cost 27KB of a 107KB descriptor that every agent
	// pays at session start to learn about tools it will mostly not call. The
	// host confirmed nothing reads them from the descriptor.
	//
	// Capability-level schemas above STAY: the host validates requests and
	// results against those, and a capability referencing a schema the host
	// does not have means it can validate nothing.
	artifacts := artifactSchemas(&warnings)
	declaredOverlays := overlays(&warnings)
	declaredSkills := skills(&warnings)

	// A capability must never reference content that is not declared. When a
	// file cannot be read the reference is DROPPED rather than left dangling:
	// a reference resolving to nothing looks like a contract and is worse than
	// an absent one, because the host cannot tell the difference until it
	// tries to load it.
	capabilities := pruneReferences(capabilityList(), artifacts, declaredSkills, &warnings)

	desc := Descriptor{
		Module:           ModuleID,
		Name:             ModuleName,
		Version:          version,
		ProtocolVersions: []string{Protocol},
		Capabilities:     capabilities,
		RequestSchemas:   reqSchemas,
		ResultSchemas:    resSchemas,
		ArtifactSchemas:  artifacts,
		AgentOverlays:    declaredOverlays,
		Skills:           declaredSkills,
		Permissions: Permissions{
			// project_root is the production workspace Facet reads and writes.
			// facet_bundle is READ-ONLY and holds the content that ships with
			// the module: the Remotion composer and its installed npm
			// dependencies, packs, schemas and styles. It is declared
			// separately because a host cannot otherwise supply it, and
			// resolving the composer from the working directory made the
			// renderer depend on where the process happened to be launched.
			FilesystemRead:  []string{"project_root", "facet_bundle"},
			FilesystemWrite: []string{"project_root"},
			// Named grants, not a blanket boolean: only these providers are
			// ever contacted, and only these binaries are ever executed.
			Network: []string{
				"labs.google", "api.openai.com", "api.elevenlabs.io",
				"fal.run", "api.pexels.com", "pixabay.com",
				"commons.wikimedia.org", "speech.platform.bing.com",
			},
			Credentials: []string{
				"OPENAI_API_KEY", "ELEVENLABS_API_KEY", "FAL_KEY",
				"PEXELS_API_KEY", "PIXABAY_API_KEY",
			},
			// Every provider here can bill. Each requires explicit human
			// consent per invocation; unknown cost is never treated as free.
			PaidProviders: []string{
				"google_flow", "openai", "elevenlabs", "fal", "kling",
			},
			// Facet renders and reviews; it never publishes.
			Publish: false,
			// Derived from actual invocation sites, NOT from the dependency
			// probe table. `node` is invoked directly (compose.go runs
			// remotion-cli.js through it) but is never probed as a dependency,
			// so a probe-derived list omitted it and would have starved the
			// renderer under a host that grants only declared binaries.
			Subprocess: []string{"ffmpeg", "ffprobe", "gflow", "node", "npx", "piper"},
		},
		Requirements: requirements(tools),
	}

	return Envelope{
		Protocol:  Protocol,
		Module:    ModuleID,
		Operation: OpDescribe,
		RequestID: reqID,
		OK:        true,
		Result:    desc,
		Warnings:  warnings,
		Execution: localExec("facet"),
	}
}

// capabilitySchemas are the schema documents the CAPABILITIES reference.
//
// These are distinct from the per-tool schemas: a capability is the host's
// addressable unit, and every ID a capability names must resolve to a present
// key or the host cannot validate anything it sends or receives. The per-tool
// schemas remain in the same maps, keyed by tool name, because they describe
// the `input` payload a run carries.
func capabilitySchemas() (req map[string]any, res map[string]any) {
	str := map[string]any{"type": "string", "minLength": 1}
	obj := func(required []string, props map[string]any) map[string]any {
		return map[string]any{
			"type": "object", "required": required, "properties": props,
		}
	}
	// Every capability request shares this envelope; `input` is the
	// tool-specific body validated by the per-tool schema for `tool`.
	toolCall := obj([]string{"tool", "input"}, map[string]any{
		"request_id": str,
		"tool":       str,
		"input":      map[string]any{"type": "object"},
		"consent": obj([]string{"paid_generation_approved", "approved_by"}, map[string]any{
			"paid_generation_approved": map[string]any{"type": "boolean"},
			"approved_by":              str,
			"note":                     map[string]any{"type": "string"},
		}),
		"seed": obj([]string{"schema", "path", "digest"}, map[string]any{
			"schema": str, "path": str, "digest": str,
		}),
		"binaries": map[string]any{
			"type": "object", "additionalProperties": str,
		},
		// Opt-in handle for long-running work. Absent or false means block
		// until the work completes, which is what every existing consumer
		// expects.
		"async": map[string]any{"type": "boolean"},
	})
	passthrough := obj([]string{"capability", "tool", "output"}, map[string]any{
		"capability": str, "tool": map[string]any{"type": "string"},
		"output": map[string]any{},
	})

	req = map[string]any{
		"creative.jobs.status.request/v1": obj([]string{"job_id"}, map[string]any{
			"request_id": str, "job_id": str,
		}),
		"creative.tools.list.request/v1": obj(nil, map[string]any{"request_id": str}),
		"creative.tools.describe.request/v1": obj([]string{"tool"}, map[string]any{
			"request_id": str, "tool": str,
		}),
		"creative.tools.estimate.request/v1":   toolCall,
		"creative.tools.run.request/v1":        toolCall,
		"creative.output.review.request/v1":    toolCall,
		"creative.artifact.inspect.request/v1": toolCall,
	}
	res = map[string]any{
		"creative.jobs.status.result/v1": obj([]string{"capability", "job"}, map[string]any{
			"capability": str,
			"job": obj([]string{"job_id", "state"}, map[string]any{
				"job_id":  str,
				"state":   map[string]any{"enum": []string{"running", "succeeded", "failed"}},
				"percent": map[string]any{"type": []string{"number", "null"}},
				"detail":  map[string]any{"type": "string"},
			}),
		}),
		"creative.tools.list.result/v1":       passthrough,
		"creative.tools.describe.result/v1":   passthrough,
		"creative.tools.estimate.result/v1":   passthrough,
		"creative.tools.run.result/v1":        passthrough,
		"creative.output.review.result/v1":    passthrough,
		"creative.artifact.inspect.result/v1": passthrough,
	}
	return req, res
}

// moduleRoot resolves the directory holding the module's own content.
//
// The host runs a module as a detached process and promises no particular cwd,
// so content must never be resolved against the working directory: run from the
// repo root the descriptor declared 20 artifact schemas, one overlay and one
// skill; run from anywhere else it returned ok:true with all three EMPTY.
//
// It must also work from a real INSTALL, where the layout is not the repo
// layout: the binary lives in ~/.facet/bin/ and its content in the sibling
// ~/.facet/bundle/. Walking up from the executable finds a repo checkout but
// never a bundle, so an installed Facet declared no skills at all — the exact
// failure a host would hit after `install.ps1`.
//
// Discovery is delegated to internal/config, which already resolves both
// layouts (executable-relative ../bundle, ~/.facet/bundle, cwd and two parents)
// and honours a pinned paths.bundle. Duplicating that here is what produced the
// install-layout gap in the first place.
func moduleRoot() string {
	// A candidate only counts if it actually holds Facet's content. Config's
	// own bundle discovery treats any directory containing a `skills` folder as
	// a bundle root, which matched the user's HOME because an unrelated
	// ~/skills existed there — so a loose heuristic silently won over the real
	// bundle. Every candidate here is verified by the file the module needs.
	holdsContent := func(dir string) bool {
		if strings.TrimSpace(dir) == "" {
			return false
		}
		_, err := os.Stat(filepath.Join(dir, "skills", "facet", "SKILL.md"))
		return err == nil
	}

	// A pinned or discovered bundle wins when it is genuinely one.
	if cfg, err := config.Load(); err == nil && holdsContent(cfg.Paths.Bundle) {
		return cfg.Paths.Bundle
	}

	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	// Installed layout first: the binary is in <root>/bin and content in the
	// sibling <root>/bundle. Then a source checkout, where content sits beside
	// the binary or a few directories above it.
	binDir := filepath.Dir(exe)
	if root := filepath.Dir(binDir); holdsContent(filepath.Join(root, "bundle")) {
		return filepath.Join(root, "bundle")
	}
	if home, err := os.UserHomeDir(); err == nil {
		if b := filepath.Join(home, ".facet", "bundle"); holdsContent(b) {
			return b
		}
	}
	dir := binDir
	for i := 0; i < 4; i++ {
		if holdsContent(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// modulePath joins a module-relative path onto the module root. An empty root
// yields the original relative path, preserving the previous behaviour.
func modulePath(rel ...string) string {
	root := moduleRoot()
	if root == "" {
		return filepath.Join(rel...)
	}
	return filepath.Join(append([]string{root}, rel...)...)
}

// fileDigest returns "sha256:<lowercase-hex>" over a file's contents, plus a
// rough token estimate so the host can budget context before loading it.
//
// A file that cannot be read yields an empty digest and is reported by the
// caller as a warning rather than declared: the host hard-rejects content with
// no verifiable provenance, so declaring an unverifiable path would only
// produce a rejection later.
func fileDigest(path string) (digest string, tokens int, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", 0, err
	}
	sum := sha256.Sum256(raw)
	// ~4 bytes per token is the usual rough estimate for English prose.
	return "sha256:" + hex.EncodeToString(sum[:]), len(raw) / 4, nil
}

// pruneReferences drops capability references to content that is not declared,
// so the descriptor is internally consistent wherever it is invoked.
func pruneReferences(caps []Capability, artifacts map[string]any,
	declared []Skill, warnings *[]string) []Capability {

	haveSkill := map[string]bool{}
	for _, s := range declared {
		haveSkill[s.ID] = true
	}

	out := make([]Capability, 0, len(caps))
	for _, c := range caps {
		kept := make([]string, 0, len(c.ArtifactSchemas))
		for _, id := range c.ArtifactSchemas {
			if _, present := artifacts[id]; present {
				kept = append(kept, id)
				continue
			}
			*warnings = append(*warnings,
				"capability "+c.ID+" no longer references undeclared artifact schema "+id)
		}
		c.ArtifactSchemas = kept

		keptSkills := make([]string, 0, len(c.Skills))
		for _, id := range c.Skills {
			if haveSkill[id] {
				keptSkills = append(keptSkills, id)
				continue
			}
			*warnings = append(*warnings,
				"capability "+c.ID+" no longer references undeclared skill "+id)
		}
		c.Skills = keptSkills

		out = append(out, c)
	}
	return out
}

func capabilityList() []Capability {
	return []Capability{
		{
			ID:              CapToolsList,
			Title:           "List creative tools",
			Summary:         "Enumerate installed creative tools with dependency and configuration state.",
			RequestSchema:   "creative.tools.list.request/v1",
			ResultSchema:    "creative.tools.list.result/v1",
			ArtifactSchemas: []string{},
			Skills:          []string{"facet-core"},
			Effects: Effects{
				Local: true, Network: false, ExternalWrites: false,
				Provider: "local", CostKnown: true,
			},
		},
		{
			ID:              CapToolsDescribe,
			Title:           "Describe a creative tool",
			Summary:         "Return the request/result schema, provider, effects, and cost behavior of one tool.",
			RequestSchema:   "creative.tools.describe.request/v1",
			ResultSchema:    "creative.tools.describe.result/v1",
			ArtifactSchemas: []string{},
			Skills:          []string{"facet-core"},
			Effects: Effects{
				Local: true, Network: false, ExternalWrites: false,
				Provider: "local", CostKnown: true,
			},
		},
		{
			ID:    CapToolsEstimate,
			Title: "Estimate a creative tool call",
			Summary: "Validate a concrete request and report expected effects and cost. " +
				"Never generates media, never bills, and never writes output.",
			RequestSchema:   "creative.tools.estimate.request/v1",
			ResultSchema:    "creative.tools.estimate.result/v1",
			ArtifactSchemas: []string{},
			Skills:          []string{"facet-core"},
			// Estimation never writes and never bills. CostKnown is false
			// because the estimate may legitimately return an unknown cost,
			// which the host must treat as unpriced rather than free.
			Effects: Effects{
				Local: true, Network: false, ExternalWrites: false,
				Provider: "local", CostKnown: false,
			},
		},
		{
			ID:    CapToolsRun,
			Title: "Run a creative tool",
			Summary: "Execute one tool. May reach the network, write files, and incur real cost " +
				"depending on the selected tool.",
			RequestSchema: "creative.tools.run.request/v1",
			ResultSchema:  "creative.tools.run.result/v1",
			// A run may produce any of these; the host validates what arrives.
			ArtifactSchemas: []string{"render_report", "asset_manifest"},
			Skills:          []string{"facet-core"},
			// A render takes 30s at 720p and 83s at 1080p, so this capability
			// may return a job handle rather than a finished result and the
			// host polls it instead of showing a blank cockpit.
			LongRunning:    true,
			PollCapability: CapJobsStatus,
			// Declared honestly and pessimistically: this capability dispatches
			// any tool, including paid provider-backed ones, so it declares the
			// worst case. CostKnown false forces host approval.
			Effects: Effects{
				Local: false, Network: true, ExternalWrites: true,
				Provider: "varies", CostKnown: false,
			},
		},
		{
			ID:    CapOutputReview,
			Title: "Review rendered output",
			Summary: "Technical QA of a rendered file. This is not creative acceptance and never " +
				"substitutes for human review.",
			RequestSchema:   "creative.output.review.request/v1",
			ResultSchema:    "creative.output.review.result/v1",
			ArtifactSchemas: []string{"review", "final_review"},
			Skills:          []string{"facet-core"},
			Effects: Effects{
				Local: true, Network: false, ExternalWrites: true,
				Provider: "ffmpeg", CostKnown: true,
			},
		},
		{
			ID:    CapJobsStatus,
			Title: "Poll a long-running job",
			Summary: "Report the state of a long-running invocation. Deterministic, local, " +
				"free, and never runs work or bills.",
			RequestSchema:   "creative.jobs.status.request/v1",
			ResultSchema:    "creative.jobs.status.result/v1",
			ArtifactSchemas: []string{},
			Skills:          []string{"facet-core"},
			Effects: Effects{
				Local: true, Network: false, ExternalWrites: false,
				Provider: "local", CostKnown: true,
			},
		},
		{
			ID:              CapArtifactInspect,
			Title:           "Inspect a Facet artifact",
			Summary:         "Read a Facet artifact manifest and report its schema, provenance, and source references.",
			RequestSchema:   "creative.artifact.inspect.request/v1",
			ResultSchema:    "creative.artifact.inspect.result/v1",
			ArtifactSchemas: []string{},
			Skills:          []string{"facet-core"},
			Effects: Effects{
				Local: true, Network: false, ExternalWrites: false,
				Provider: "ffprobe", CostKnown: true,
			},
		},
	}
}

// unreferencedArtifactSchemas are schema files present on disk that no tool,
// pack, pipeline, or skill references. They are still declared — the host may
// legitimately read an existing artifact written against one — but they are
// flagged so the host does not present them as current output contracts.
//
// Verified by reference count across tracked files at commit e476bc3.
var unreferencedArtifactSchemas = map[string]bool{
	"cost_log": true,
}

// artifactSchemas loads the real JSON Schema documents from disk, keyed by
// schema ID.
//
// The host receives schemas through `describe`, not by reading a module's files
// off disk, so the documents are inlined rather than referenced by path: a path
// would only be resolvable if the host could see into this repository.
//
// A missing directory or unreadable file is reported as a warning rather than
// declared, so the host is never promised a schema that is not there. Files
// with no known producer or consumer are declared but warned about.
func artifactSchemas(warnings *[]string) map[string]any {
	out := map[string]any{}
	dir := modulePath("schemas", "artifacts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		*warnings = append(*warnings, "artifact schema directory unavailable: "+filepath.ToSlash(dir))
		return out
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".schema.json") {
			continue
		}
		id := strings.TrimSuffix(name, ".schema.json")

		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			*warnings = append(*warnings, "artifact schema unreadable: "+id)
			continue
		}
		var doc any
		if err := json.Unmarshal(raw, &doc); err != nil {
			*warnings = append(*warnings, "artifact schema is not valid JSON: "+id)
			continue
		}
		// The schema BODY is deliberately not inlined. It was 67KB of a 107KB
		// descriptor — 58% — that every agent paid at session start, and the
		// host confirmed nothing reads it: the cockpit renders from the
		// host-resolved primitive, computed from media type plus the
		// artifact's own presentation hint.
		//
		// What stays is an index entry: the ID a capability references, so
		// every reference still resolves, plus a digest and size so a host
		// that DOES want a schema can verify the file it reads is the one this
		// descriptor described. An index of IDs alone would make the reference
		// checkable but the content unverifiable.
		out[id] = artifactSchemaRef(doc, raw)

		if unreferencedArtifactSchemas[id] {
			*warnings = append(*warnings,
				"artifact schema '"+id+"' has no known producer or consumer; "+
					"readable for existing artifacts, not a current output contract")
		}
	}
	return out
}

// digestOfBytes renders a sha256 digest in the module-boundary form.
func digestOfBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// artifactSchemaRef describes an artifact schema without carrying it.
//
// The description and title come from the schema itself, so an agent can tell
// what an artifact IS without fetching the document — which is what the
// summary was actually used for. The digest identifies the exact bytes.
func artifactSchemaRef(doc any, raw []byte) map[string]any {
	ref := map[string]any{
		"digest": digestOfBytes(raw),
		"bytes":  len(raw),
	}
	if m, ok := doc.(map[string]any); ok {
		for _, k := range []string{"title", "description"} {
			if v, present := m[k]; present {
				ref[k] = v
			}
		}
	}
	return ref
}

func overlays(warnings *[]string) []Overlay {
	const rel = "agents/facet-creative.md"
	path := modulePath("agents", "facet-creative.md")
	digest, tokens, err := fileDigest(path)
	if err != nil {
		*warnings = append(*warnings, "agent overlay unreadable, not declared: "+rel)
		return []Overlay{}
	}
	return []Overlay{{
		ID:     "facet.creative",
		Title:  "Facet creative overlay",
		Path:   rel,
		Digest: digest,
		Tokens: tokens,
	}}
}

// skills declares the progressively loadable knowledge the host may fold into
// agent context when Facet is selected.
//
// Every file the core skill REFERENCES must be declared here. The host installs
// exactly what the descriptor declares and verifies each digest, so an
// undeclared file does not travel with the module: an agent then reads "the
// path is in NARRATED-WALKTHROUGH.md", finds nothing, and sequences by guess.
// Documenting knowledge and failing to declare it is worse than not writing it,
// because the reference implies the content is available.
func skills(warnings *[]string) []Skill {
	declared := []struct {
		id, title, summary string
		parts              []string
	}{
		{
			id: "facet-core", title: "Facet video producer",
			summary: "Canonical producer guidance: plan, estimate, render, review, disclose.",
			parts:   []string{"skills", "facet", "SKILL.md"},
		},
		{
			id: "facet-explainer-walkthrough", title: "Narrated explainer, end to end",
			summary: "The order a narrated video is produced in: narration first because " +
				"the audio decides the length, then cuts matched to it, render, verify.",
			parts: []string{"packs", "explainer", "NARRATED-WALKTHROUGH.md"},
		},
		{
			id: "facet-explainer-scene-types", title: "Explainer scene types",
			summary: "Every scene type the Explainer composition renders and the field " +
				"each one requires; a missing field renders an empty frame.",
			parts: []string{"packs", "explainer", "SCENE-TYPES.md"},
		},
	}

	out := make([]Skill, 0, len(declared))
	for _, d := range declared {
		rel := strings.Join(d.parts, "/")
		digest, tokens, err := fileDigest(modulePath(d.parts...))
		if err != nil {
			*warnings = append(*warnings, "skill unreadable, not declared: "+rel)
			continue
		}
		out = append(out, Skill{
			ID: d.id, Title: d.title, Summary: d.summary,
			Path: rel, Digest: digest, Tokens: tokens,
		})
	}
	return out
}

// requirements reports external dependencies derived from the toolbox's own
// dependency probes rather than a static list.
func requirements(tools []map[string]any) []Requirement {
	seen := map[string]Requirement{}
	for _, t := range tools {
		name, _ := t["name"].(string)
		deps, _ := t["dependencies"].([]any)
		for _, d := range deps {
			m, ok := d.(map[string]any)
			if !ok {
				continue
			}
			dn, _ := m["name"].(string)
			dk, _ := m["type"].(string)
			if dn == "" {
				continue
			}
			if _, exists := seen[dn]; !exists {
				seen[dn] = Requirement{Name: dn, Kind: dk, Required: false, For: name}
			}
		}
	}
	out := make([]Requirement, 0, len(seen))
	for _, r := range seen {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func errFromToolbox(env toolbox.Envelope) error {
	if env.Error != nil {
		return &toolboxErr{env.Error.Message}
	}
	return &toolboxErr{"toolbox returned an unsuccessful envelope"}
}

type toolboxErr struct{ msg string }

func (e *toolboxErr) Error() string { return e.msg }
