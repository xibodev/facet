package module

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/xibodev/facet/internal/toolbox"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The descriptor must declare exactly the twelve agreed fields, no more and no
// fewer. A drifting descriptor silently breaks every host that reads it.
func TestDescribeDeclaresExactlyTheAgreedFields(t *testing.T) {
	env := Describe("test")
	if !env.OK {
		t.Fatalf("describe failed: %+v", env.Error)
	}

	raw, err := json.Marshal(env.Result)
	if err != nil {
		t.Fatalf("marshal descriptor: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal descriptor: %v", err)
	}

	// The AGREED descriptor shape. Adding a field is a contract change, which
	// is why this asserts an exact count rather than a subset — it caught
	// contract_version being added, correctly, and the field stays only
	// because xibodev.module/v2 froze it as normative.
	want := []string{
		"module", "name", "version", "protocol_versions", "capabilities",
		"request_schemas", "result_schemas", "artifact_schemas",
		"agent_overlays", "skills", "permissions", "requirements",
		// v2: the BEHAVIOURAL contract, distinct from the wire protocol.
		"contract_version",
		// v2 SEMANTIC PAYLOAD. These ride in the same document as the v1
		// fields above, which §10 permits: a v1 host ignores what it does not
		// recognise, and facet-studio pinned that direction by test because
		// "works because the decoder is tolerant" and "works because the
		// contract requires it" produce the same observable.
		//
		// This test correctly refused them until this line was added. Adding a
		// descriptor field IS a contract change, and it is being made
		// deliberately under authorised Step 6 rather than absorbed silently.
		//
		// The v1 surface is unchanged: verified by diffing the serialized
		// descriptor before and after, which reported two ADDED keys and zero
		// changed or removed ones.
		"operations", "artifact_kinds",
	}
	for _, field := range want {
		if _, ok := got[field]; !ok {
			t.Errorf("descriptor missing required field %q", field)
		}
	}
	if len(got) != len(want) {
		t.Errorf("descriptor has %d fields, want exactly %d: %v", len(got), len(want), keys(got))
	}
}

// Every envelope must carry the protocol framing fields, including on failure.
func TestEnvelopeAlwaysCarriesProtocolFraming(t *testing.T) {
	cases := map[string]Envelope{
		"success":            Describe("test"),
		"unknown_capability": Invoke("creative.nope", []byte(`{}`)),
		"bad_json":           Invoke(CapToolsList, []byte(`{not json`)),
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			if env.Protocol != Protocol {
				t.Errorf("protocol = %q, want %q", env.Protocol, Protocol)
			}
			if env.Module != ModuleID {
				t.Errorf("module = %q, want %q", env.Module, ModuleID)
			}
			if env.RequestID == "" {
				t.Error("request_id must never be empty")
			}
			if env.Warnings == nil {
				t.Error("warnings must be [], never null")
			}
			if !env.OK && env.Error == nil {
				t.Error("failed envelope must carry an error")
			}
			if env.Execution.Artifacts == nil {
				t.Error("artifacts must be [], never null")
			}
		})
	}
}

// A host-supplied request_id must come back verbatim so the host can correlate
// a response without trusting the module to generate unique ids.
func TestHostRequestIDEchoedVerbatim(t *testing.T) {
	const id = "req_host_supplied_abc123"
	env := Invoke(CapToolsList, []byte(`{"request_id":"`+id+`"}`))
	if env.RequestID != id {
		t.Errorf("request_id = %q, want %q echoed verbatim", env.RequestID, id)
	}
}

// The consent gate is the safety property that matters most: a tool that can
// spend real money must refuse to run without explicit human approval.
func TestPaidToolRefusesWithoutConsent(t *testing.T) {
	for _, tool := range toolbox.ChargeableTools() {
		t.Run(tool, func(t *testing.T) {
			body := `{"tool":"` + tool + `","input":{"prompt":"x"}}`
			env := Invoke(CapToolsRun, []byte(body))
			if env.OK {
				t.Fatalf("paid tool %q ran without consent", tool)
			}
			if env.Error.Code != "consent_required" {
				t.Errorf("error code = %q, want consent_required", env.Error.Code)
			}
		})
	}
}

// Unknown cost must never be reported as zero. null means "we do not know";
// 0 means "genuinely free". Collapsing them turns an unknown charge into an
// implied promise that generation is free.
func TestUnknownCostIsNullNotZero(t *testing.T) {
	body := `{"tool":"gflow_image","input":{"prompt":"x","model":"narwhal",` +
		`"aspect_ratio":"landscape","count":1,"output_path":"out.png"}}`
	env := Estimate(CapToolsEstimate, []byte(body))
	if !env.OK {
		t.Skipf("estimate unavailable in this environment: %+v", env.Error)
	}
	if env.Execution.EstimatedCost != nil {
		t.Errorf("estimated_cost = %v, want null for an unpriced provider",
			*env.Execution.EstimatedCost)
	}
}

// A local deterministic tool must report itself as local, free, and offline.
func TestLocalToolReportsHonestExecution(t *testing.T) {
	env := Invoke(CapToolsList, []byte(`{}`))
	if !env.OK {
		t.Fatalf("list failed: %+v", env.Error)
	}
	e := env.Execution
	if !e.Local || e.Network {
		t.Errorf("local tool reported local=%v network=%v", e.Local, e.Network)
	}
	if e.EstimatedCost == nil || *e.EstimatedCost != 0 {
		t.Errorf("free local tool must report cost 0, got %v", e.EstimatedCost)
	}
}

// A seed whose bytes do not match its digest must be refused, so a Facet
// artifact can never claim provenance from bytes it did not read.
func TestSeedDigestMismatchIsRefused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, SeedManifestFile)
	content := []byte(`{"schema":"` + SeedSchemaID + `","goal":"explain rain"}`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	_, res, err := LoadSeed(&SeedRef{Schema: SeedSchemaID, Path: path,
		Digest: strings.Repeat("0", 64)})
	if err == nil {
		t.Fatal("seed with a mismatched digest was accepted")
	}
	if res == nil || res.Verified {
		t.Error("resolution must record the seed as unverified")
	}
}

// A seed whose digest matches must load, and the manifest must retain the
// source reference so output is traceable to its input.
func TestSeedVerifiesAndManifestRetainsSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, SeedManifestFile)
	content := []byte(`{"schema":"` + SeedSchemaID + `","goal":"explain rain",` +
		`"key_points":["evaporation","condensation"]}`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])

	seed, res, err := LoadSeed(&SeedRef{Schema: SeedSchemaID, Path: path, Digest: digest})
	if err != nil {
		t.Fatalf("valid seed refused: %v", err)
	}
	if !res.Verified {
		t.Error("matching digest must verify")
	}
	if seed.Goal != "explain rain" || len(seed.KeyPoints) != 2 {
		t.Errorf("seed content not parsed: %+v", seed)
	}

	m := NewManifest(CapToolsRun, Describe("test"), res)
	if m.Source == nil || m.Source.DigestActual != digest {
		t.Error("manifest must retain the seed digest for provenance")
	}
	if m.Review.HumanApproved {
		t.Error("human_approved must default to false; only a person can set it")
	}
}

// external_writes must fail CLOSED. The toolbox never assigns its own
// ExternalWrite field, so passing it through would declare every
// write-performing tool as side-effect-free and route it around host approval.
func TestExternalWritesFailsClosed(t *testing.T) {
	// A run that writes media must declare it, even for an unrecognised tool.
	for _, tool := range []string{"edge_tts", "video_compose", "source_edit", "not_a_real_tool"} {
		if !writesOutput("run", tool) {
			t.Errorf("run of %q declared no external writes; must fail closed", tool)
		}
	}
	// Read-only inspection genuinely writes nothing.
	for _, tool := range []string{"media_probe", "audio_probe"} {
		if writesOutput("run", tool) {
			t.Errorf("read-only tool %q declared an external write", tool)
		}
	}
	// Estimation never writes output, by contract.
	if writesOutput("estimate", "gflow_image") {
		t.Error("estimate declared an external write; estimation must never write")
	}

	// And the projection must carry it: a probe writes nothing.
	env := Invoke(CapToolsRun, []byte(
		`{"tool":"media_probe","input":{"input":"../../projects/cinematic-documentary/assets/video/shot1_raw.mp4"}}`))
	if env.OK && env.Execution.ExternalWrites {
		t.Error("media_probe run reported an external write")
	}
}

// Protocol-level digests carry the mandatory "sha256:" prefix, and a malformed
// value is never decorated into a well-formed-looking one.
func TestProtocolDigestPrefix(t *testing.T) {
	const hexDigest = "8f612e6db25d7ee58024a51af18b76a2ecaefafc94a506ec7d000f6c47153ada"
	if got := protocolDigest(hexDigest); got != "sha256:"+hexDigest {
		t.Errorf("bare hex not prefixed: %q", got)
	}
	if got := protocolDigest("sha256:" + hexDigest); got != "sha256:"+hexDigest {
		t.Errorf("already-prefixed digest altered: %q", got)
	}
	if got := protocolDigest(""); got != "" {
		t.Errorf("empty digest became %q", got)
	}
	// Too short, and non-hex: returned unchanged rather than falsely prefixed.
	for _, bad := range []string{"abc123", strings.Repeat("z", 64)} {
		if got := protocolDigest(bad); got != bad {
			t.Errorf("malformed digest %q was decorated into %q", bad, got)
		}
	}
}

// Declared subprocess binaries must match what the toolbox actually invokes.
//
// Under the host a module inherits NO environment, so it can run only the
// binaries it declared and the host resolved. An omission is therefore not
// cosmetic: it starves a tool at runtime. `node` was missing from the first
// version of this list because the list was derived from the dependency probe
// table, and node is invoked (compose.go, to run remotion-cli.js) without ever
// being probed — a declared-vs-actual gap in Facet's own descriptor.
func TestDeclaredSubprocessMatchesInvocations(t *testing.T) {
	env := Describe("test")
	if !env.OK {
		t.Fatalf("describe failed: %+v", env.Error)
	}
	desc, ok := env.Result.(Descriptor)
	if !ok {
		t.Fatalf("result is not a Descriptor: %T", env.Result)
	}

	declared := map[string]bool{}
	for _, b := range desc.Permissions.Subprocess {
		declared[b] = true
	}

	// Verified by grepping invocation sites, not the probe table:
	//   ffmpeg/ffprobe/node/npx via runCommand/runCommandDir
	//   gflow  (gflow.go: LookPath then exec.CommandContext)
	//   piper  (tts.go:   LookPath then exec.CommandContext)
	for _, bin := range []string{"ffmpeg", "ffprobe", "gflow", "node", "npx", "piper"} {
		if !declared[bin] {
			t.Errorf("binary %q is invoked by the toolbox but not declared in "+
				"Permissions.Subprocess; the host would not grant it", bin)
		}
	}

	// Over-declaring is also a defect: it requests authority Facet does not use.
	for b := range declared {
		switch b {
		case "ffmpeg", "ffprobe", "gflow", "node", "npx", "piper":
		default:
			t.Errorf("binary %q is declared but no invocation site is known; "+
				"do not request authority that is not used", b)
		}
	}
}

// Permissions must be enumerated grants, never blanket booleans: the host
// intersects a declaration with policy per invocation, and "network: true"
// says nothing about which hosts.
func TestPermissionsAreEnumeratedNotBlanket(t *testing.T) {
	env := Describe("test")
	desc, ok := env.Result.(Descriptor)
	if !ok {
		t.Fatalf("result is not a Descriptor: %T", env.Result)
	}
	p := desc.Permissions
	if len(p.Network) == 0 {
		t.Error("network grants empty; Facet contacts named providers")
	}
	for _, h := range p.Network {
		if strings.ContainsAny(h, "*/") {
			t.Errorf("network grant %q is a pattern, not a named host", h)
		}
	}
	if len(p.PaidProviders) == 0 {
		t.Error("paid_providers empty although Facet has billable tools")
	}
	if p.Publish {
		t.Error("Facet renders and reviews; it must never declare publish")
	}
}

// Every schema ID a capability references must resolve to a present key.
//
// This is the check that would have caught 11 dangling references shipping in
// describe.json: the capabilities named `creative.tools.run.request` while the
// schema maps were keyed by TOOL name, so the intersection was empty and the
// host could not validate a single request or result. A reference that resolves
// to nothing is worse than an absent one — it looks like a contract.
func TestEveryCapabilitySchemaReferenceResolves(t *testing.T) {
	env := Describe("test")
	if !env.OK {
		t.Fatalf("describe failed: %+v", env.Error)
	}
	desc, ok := env.Result.(Descriptor)
	if !ok {
		t.Fatalf("result is not a Descriptor: %T", env.Result)
	}

	for _, c := range desc.Capabilities {
		if c.RequestSchema != "" {
			if _, present := desc.RequestSchemas[c.RequestSchema]; !present {
				t.Errorf("capability %s references request_schema %q which is not a key in request_schemas",
					c.ID, c.RequestSchema)
			}
		}
		if c.ResultSchema != "" {
			if _, present := desc.ResultSchemas[c.ResultSchema]; !present {
				t.Errorf("capability %s references result_schema %q which is not a key in result_schemas",
					c.ID, c.ResultSchema)
			}
		}
		for _, id := range c.ArtifactSchemas {
			if _, present := desc.ArtifactSchemas[id]; !present {
				t.Errorf("capability %s references artifact schema %q which is not declared",
					c.ID, id)
			}
		}
		// Nested collections must be [] and never null. The top level was
		// clean while these were null: checking the outer document is not
		// enough, the nesting is where it hides.
		if c.ArtifactSchemas == nil {
			t.Errorf("capability %s artifact_schemas is null; must be []", c.ID)
		}
		if c.Skills == nil {
			t.Errorf("capability %s skills is null; must be []", c.ID)
		}
		for _, id := range c.Skills {
			found := false
			for _, s := range desc.Skills {
				if s.ID == id {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("capability %s references skill %q which is not declared", c.ID, id)
			}
		}
	}
}

// Anything the host may fold into agent context must carry verifiable
// provenance. The host hard-rejects an empty digest, so declaring a path
// without one only produces a rejection later.
func TestOverlayAndSkillDigestsAreVerifiable(t *testing.T) {
	env := Describe("test")
	desc, ok := env.Result.(Descriptor)
	if !ok {
		t.Fatalf("result is not a Descriptor: %T", env.Result)
	}
	if len(desc.AgentOverlays) == 0 && len(desc.Skills) == 0 {
		t.Skip("no overlays or skills declared in this environment")
	}
	for _, o := range desc.AgentOverlays {
		if !ValidDigest(o.Digest) {
			t.Errorf("overlay %s digest %q is not sha256:<64 lowercase hex>", o.ID, o.Digest)
		}
		if o.Tokens <= 0 {
			t.Errorf("overlay %s declares %d tokens; the host budgets context with this", o.ID, o.Tokens)
		}
	}
	for _, s := range desc.Skills {
		if !ValidDigest(s.Digest) {
			t.Errorf("skill %s digest %q is not sha256:<64 lowercase hex>", s.ID, s.Digest)
		}
		if s.Tokens <= 0 {
			t.Errorf("skill %s declares %d tokens", s.ID, s.Tokens)
		}
	}
}

// A declared digest must match the file's ACTUAL bytes. A digest that is
// well-formed but wrong is worse than an absent one: it asserts provenance the
// module never verified.
func TestDeclaredDigestsMatchFileContents(t *testing.T) {
	env := Describe("test")
	desc, _ := env.Result.(Descriptor)

	check := func(id, path, declared string) {
		// Declared paths are MODULE-relative by contract — a module never hands
		// the host an absolute path — so they must be resolved against the
		// module root, not the test's working directory.
		raw, err := os.ReadFile(filepath.Join(moduleRoot(), path))
		if err != nil {
			t.Errorf("%s declares path %q which cannot be read: %v", id, path, err)
			return
		}
		sum := sha256.Sum256(raw)
		want := "sha256:" + hex.EncodeToString(sum[:])
		if declared != want {
			t.Errorf("%s digest %q does not match file contents %q", id, declared, want)
		}
	}
	for _, o := range desc.AgentOverlays {
		check(o.ID, o.Path, o.Digest)
	}
	for _, s := range desc.Skills {
		check(s.ID, s.Path, s.Digest)
	}
}

// The descriptor must be identical wherever the binary is invoked.
//
// The host runs a module as a detached process and promises no particular cwd.
// Resolving content against cwd made describe return ok:true with EMPTY
// artifact schemas, overlays and skills when run from anywhere but the repo
// root — while capabilities still referenced them. That is the dangling
// reference class again, produced by the environment rather than by code.
func TestDescriptorIsIndependentOfWorkingDirectory(t *testing.T) {
	from := func(dir string) Descriptor {
		t.Helper()
		orig, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(dir); err != nil {
			t.Skipf("cannot chdir to %s: %v", dir, err)
		}
		defer func() { _ = os.Chdir(orig) }()

		env := Describe("test")
		if !env.OK {
			t.Fatalf("describe failed in %s: %+v", dir, env.Error)
		}
		desc, ok := env.Result.(Descriptor)
		if !ok {
			t.Fatalf("result is not a Descriptor: %T", env.Result)
		}
		return desc
	}

	here := from(".")
	away := from(t.TempDir())

	if len(here.ArtifactSchemas) != len(away.ArtifactSchemas) {
		t.Errorf("artifact_schemas differ by cwd: %d here, %d elsewhere",
			len(here.ArtifactSchemas), len(away.ArtifactSchemas))
	}
	if len(here.Skills) != len(away.Skills) {
		t.Errorf("skills differ by cwd: %d here, %d elsewhere",
			len(here.Skills), len(away.Skills))
	}
	if len(here.AgentOverlays) != len(away.AgentOverlays) {
		t.Errorf("agent_overlays differ by cwd: %d here, %d elsewhere",
			len(here.AgentOverlays), len(away.AgentOverlays))
	}

	// And whatever it declares must be self-consistent in both places.
	for _, d := range []Descriptor{here, away} {
		haveSkill := map[string]bool{}
		for _, s := range d.Skills {
			haveSkill[s.ID] = true
		}
		for _, c := range d.Capabilities {
			for _, id := range c.ArtifactSchemas {
				if _, ok := d.ArtifactSchemas[id]; !ok {
					t.Errorf("capability %s references undeclared artifact schema %q", c.ID, id)
				}
			}
			for _, id := range c.Skills {
				if !haveSkill[id] {
					t.Errorf("capability %s references undeclared skill %q", c.ID, id)
				}
			}
		}
	}
}

// The declared request schema and the code that reads a request must agree on
// WHERE `tool` lives. They disagreeing is invisible to both a schema check and
// a unit test: each side is self-consistent, and only a request built from the
// schema and fed to the reader proves it.
//
// `tool` is a field of the protocol Request, a SIBLING of `input`, never a
// field inside `input`.
func TestToolPlacementMatchesDeclaredSchema(t *testing.T) {
	env := Describe("test")
	desc, ok := env.Result.(Descriptor)
	if !ok {
		t.Fatalf("result is not a Descriptor: %T", env.Result)
	}

	// Every capability that needs a tool must declare it at the request root.
	for _, id := range []string{
		CapToolsDescribe, CapToolsEstimate, CapToolsRun,
	} {
		var cap Capability
		for _, c := range desc.Capabilities {
			if c.ID == id {
				cap = c
				break
			}
		}
		schema, present := desc.RequestSchemas[cap.RequestSchema]
		if !present {
			t.Fatalf("%s references missing schema %q", id, cap.RequestSchema)
		}
		m, _ := schema.(map[string]any)
		props, _ := m["properties"].(map[string]any)
		if _, hasTool := props["tool"]; !hasTool {
			t.Errorf("%s schema does not declare `tool` at the request root", id)
		}
		// If the schema also declares `input`, `tool` must not be inside it.
		if inner, ok := props["input"].(map[string]any); ok {
			if ip, ok := inner["properties"].(map[string]any); ok {
				if _, nested := ip["tool"]; nested {
					t.Errorf("%s schema declares `tool` inside `input`; it is a sibling", id)
				}
			}
		}
	}

	// And the reader must accept exactly that shape, and reject the nested one.
	root := Invoke(CapToolsDescribe, []byte(`{"request_id":"r","tool":"media_probe"}`))
	if !root.OK {
		t.Errorf("schema-conformant request (tool at root) was rejected: %+v", root.Error)
	}

	nested := Invoke(CapToolsDescribe, []byte(`{"request_id":"r","input":{"tool":"media_probe"}}`))
	if nested.OK {
		t.Error("tool nested inside input was accepted; the schema says it is a sibling")
	}
	// A rejection must say WHERE the value belongs, not just that it is absent.
	if !strings.Contains(nested.Error.Message, "sibling") {
		t.Errorf("misplaced-tool error does not explain the placement: %q",
			nested.Error.Message)
	}
}

func keys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
