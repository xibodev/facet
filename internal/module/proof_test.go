package module

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const (
	seedBundle = "../../fixtures/module/seed-explainer"
	seedDigest = "e4a37c683730c1c6a156013ad9381ae2f33f359ded3a97135cf8239657fb93ce"
)

// A digest-bearing fixture must survive checkout on any platform.
//
// Git's autocrlf rewrites newlines on checkout by default, which changes the
// BYTES of a file whose digest was computed over those bytes. The seed manifest
// went from 526 to 540 bytes on a Windows checkout and its sha256 stopped
// matching — so the fixture was valid in the tree it was authored in and broken
// everywhere else. `.gitattributes` marks fixtures `-text` to prevent it; this
// test fails if that protection is ever removed.
func TestSeedFixtureBytesSurviveCheckout(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(seedBundle, SeedManifestFile))
	if err != nil {
		t.Skipf("seed fixture unavailable: %v", err)
	}
	if bytes.Contains(raw, []byte("\r\n")) {
		t.Error("seed manifest contains CRLF; its digest was computed over LF bytes " +
			"and newline translation has corrupted it (check .gitattributes)")
	}
	sum := sha256.Sum256(raw)
	if got := hex.EncodeToString(sum[:]); got != seedDigest {
		t.Errorf("seed manifest digest = %s, want %s; the fixture bytes changed",
			got, seedDigest)
	}
}

// The end-to-end proof named in the session prompt:
//
//	Midden seed -> Facet request -> deterministic local run -> artifact manifest
//
// It runs entirely on local deterministic tools. No provider is contacted and
// no paid generation occurs.
func TestSeedToArtifactManifestProof(t *testing.T) {
	if _, err := os.Stat(seedBundle); err != nil {
		t.Skipf("seed fixture unavailable: %v", err)
	}

	// 1. Consume the Midden seed bundle by path + digest. Facet never calls
	// Midden. The path names the seed ROOT; LoadSeed resolves the entry file.
	seed, res, err := LoadSeed(&SeedRef{
		Schema: SeedSchemaID, Path: seedBundle, Digest: seedDigest,
	})
	if err != nil {
		t.Fatalf("seed rejected: %v", err)
	}
	if !res.Verified {
		t.Fatal("seed digest did not verify")
	}
	if seed.Goal == "" || len(seed.KeyPoints) == 0 {
		t.Fatalf("seed carried no usable content: %+v", seed)
	}

	// The evidence set has its own identity, so regenerating brief.md prose
	// does not invalidate an evidence reference.
	if res.EvidenceDigest == "" {
		t.Error("seed resolution lost the evidence digest")
	}

	// Output-type suggestions are advisory and Facet owns the vocabulary.
	accepted, warns := seed.OutputTypes()
	if len(accepted) != 1 || accepted[0] != "explainer" {
		t.Errorf("output types = %v, want [explainer]", accepted)
	}
	if len(warns) != 0 {
		t.Errorf("unexpected vocabulary warnings: %v", warns)
	}

	// 2. Run a deterministic local capability.
	body, _ := json.Marshal(Request{
		RequestID: "req_proof_seed_to_manifest",
		Tool:      "media_probe",
		Input:     json.RawMessage(`{"input":"../../projects/cinematic-documentary/assets/video/shot1_raw.mp4"}`),
	})
	env := Invoke(CapToolsRun, body)
	if !env.OK {
		t.Skipf("probe unavailable in this environment: %+v", env.Error)
	}
	if env.Execution.Network {
		t.Error("deterministic proof must not touch the network")
	}

	// 3. Emit the artifact manifest and confirm it retains provenance.
	m := NewManifest(CapToolsRun, env, res)
	if m.Source == nil || m.Source.DigestActual != seedDigest {
		t.Fatal("manifest lost the seed reference")
	}
	if m.Source.Schema != SeedSchemaID {
		t.Errorf("manifest seed schema = %q", m.Source.Schema)
	}
	if m.RequestID != "req_proof_seed_to_manifest" {
		t.Errorf("manifest request_id = %q", m.RequestID)
	}
	if m.Review.HumanApproved {
		t.Error("manifest must not self-certify creative approval")
	}
	if m.Review.TechnicalQA != "not_run" {
		t.Errorf("technical QA state = %q, want not_run", m.Review.TechnicalQA)
	}
}

// The host stages a seed into its own store, so the path Facet receives is not
// the path Midden wrote. A seed must therefore load from an arbitrary copied
// location, with no Midden binary, index, or home directory present. Testing
// this from a COPY makes portability verified rather than asserted.
func TestSeedLoadsFromStagedCopy(t *testing.T) {
	if _, err := os.Stat(seedBundle); err != nil {
		t.Skipf("seed fixture unavailable: %v", err)
	}

	staged := filepath.Join(t.TempDir(), "staged-seed")
	if err := os.MkdirAll(staged, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(seedBundle)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(seedBundle, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(staged, e.Name()), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// Same digest, entirely different path.
	seed, res, err := LoadSeed(&SeedRef{
		Schema: SeedSchemaID, Path: staged, Digest: "sha256:" + seedDigest,
	})
	if err != nil {
		t.Fatalf("staged seed refused: %v", err)
	}
	if !res.Verified {
		t.Error("staged copy must verify against the same digest")
	}
	if seed.Goal == "" {
		t.Error("staged seed lost its content")
	}

	// Pointing directly at the entry file must work identically.
	_, res2, err := LoadSeed(&SeedRef{
		Schema: SeedSchemaID,
		Path:   filepath.Join(staged, SeedManifestFile),
		Digest: seedDigest,
	})
	if err != nil || !res2.Verified {
		t.Errorf("direct manifest path failed: %v", err)
	}
}
