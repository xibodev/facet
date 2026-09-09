package module

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Journey 3: a REAL Midden seed, built from real recovered sessions and staged
// by the host, consumed by Facet and carried into an artifact manifest.
//
// Every piece of this has been proven separately — Midden builds seeds, Facet
// verifies digests, the host stages files — but the chain itself had never run
// with real data. A synthetic fixture cannot catch a field Midden actually
// emits that Facet silently ignores.
func TestJourney3RealSeedToManifest(t *testing.T) {
	const staged = "C:/Users/gafar/AppData/Local/Temp/staged-seed-xlane"
	const digest = "6b8673b74e0ed2020d6b53b7fb328bed5290670ed666f8d8fb229051054ff5e0"

	if _, err := os.Stat(staged); err != nil {
		t.Skipf("no staged Midden seed available: %v", err)
	}

	seed, res, err := LoadSeed(&SeedRef{Schema: SeedSchemaID, Path: staged, Digest: digest})
	if err != nil {
		t.Fatalf("a real Midden seed was refused: %v", err)
	}
	if !res.Verified {
		t.Fatal("digest did not verify against the real seed")
	}
	if seed.Goal == "" {
		t.Error("the seed carried no goal for a producer to work from")
	}

	// Evidence identity is separate from manifest integrity, so regenerating
	// brief.md prose does not invalidate an evidence reference.
	if res.EvidenceDigest == "" {
		t.Error("evidence digest lost; provenance cannot key on evidence identity")
	}

	// Midden owns the vocabulary check on its side; Facet warns rather than
	// failing on an unrecognised type, because a new Facet output type must
	// never be a breaking change in Midden's schema.
	accepted, warnings := seed.OutputTypes()
	t.Logf("suggested output types accepted=%v warnings=%v", accepted, warnings)

	// ASSERTED, not merely logged. This line previously computed the real
	// seed's vocabulary result and printed it, while every assertion in this
	// test checked something else -- so the one cross-lane fact it observed
	// could not fail.
	//
	// A LOG LINE IS NOT AN ASSERTION: an observation that cannot fail is
	// indistinguishable from one that was never made. facet-studio hit the
	// same shape in their real-binary test the same week, and it is the reason
	// their run reported a category nobody had checked.
	//
	// What must hold for a REAL Midden seed: every type it suggests is one
	// Facet can act on. A warning here means the two vocabularies have drifted
	// apart in production, which is the entire point of staging a real seed
	// rather than a fixture.
	if len(accepted) == 0 {
		t.Errorf("a real Midden seed suggested %v and Facet accepted NONE of them; "+
			"the producer named output types this consumer cannot act on",
			seed.SuggestedOutputTypes)
	}
	for _, w := range warnings {
		t.Errorf("a REAL cross-lane seed carries a type Facet does not recognise: %s", w)
	}

	// The manifest must retain the source verbatim so a finished video is
	// traceable to the exact bytes that produced it.
	env := Invoke(CapToolsList, []byte(`{"request_id":"req_journey3"}`))
	if !env.OK {
		t.Fatalf("list failed: %+v", env.Error)
	}
	m := NewManifest(CapToolsRun, env, res)
	if m.Source == nil {
		t.Fatal("the manifest dropped the seed reference")
	}
	if m.Source.DigestActual != digest {
		t.Errorf("manifest digest = %q, want the seed's %q", m.Source.DigestActual, digest)
	}
	if m.Source.EvidenceDigest != res.EvidenceDigest {
		t.Error("the manifest lost the evidence identity")
	}
	if m.Review.HumanApproved {
		t.Error("a manifest must never self-certify creative approval")
	}

	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("artifact manifest: %s", raw)
}

// An artifact manifest must never carry an absolute host path.
//
// The seed is staged by the host, so the path Facet receives is the host's
// filesystem layout and differs between machines. Recording it verbatim leaked
// that layout into every manifest and would be rejected as a protocol
// violation on any artefact field. Provenance keys on the digest, which is
// stable across staging.
func TestManifestCarriesNoAbsoluteHostPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, SeedManifestFile)
	content := []byte(`{"schema":"` + SeedSchemaID + `","goal":"check paths"}`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)

	_, res, err := LoadSeed(&SeedRef{
		Schema: SeedSchemaID, Path: dir, Digest: hex.EncodeToString(sum[:]),
	})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := json.Marshal(NewManifest(CapToolsRun, Describe("test"), res))
	if err != nil {
		t.Fatal(err)
	}
	// The temp directory is an absolute host path; none of it may appear.
	if strings.Contains(string(raw), filepath.ToSlash(dir)) {
		t.Errorf("manifest embeds the absolute staging path %q", dir)
	}
	if isAbsolutePath(res.Path) {
		t.Errorf("resolution path %q is absolute", res.Path)
	}
	if res.DigestActual == "" {
		t.Error("provenance must key on the digest, which is missing")
	}
}
