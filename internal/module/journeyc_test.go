package module

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Journey C is mined content becoming a video: a Midden seed carries the
// material, Facet renders it, and the artifact manifest retains where it came
// from.
//
// Every earlier verification used a single title card carrying the seed's
// title. That proved the handoff but not the point of it — a real seed carries
// key_points, and an explainer is one scene per point. This asserts the shape
// the journey actually takes.
func TestSeedContentDrivesAMultiSceneRender(t *testing.T) {
	root := t.TempDir()
	points := []string{
		"Sunlight evaporates water from oceans and lakes",
		"Rising vapour cools and condenses into droplets",
		"Droplets merge until they fall as rain",
	}
	manifest := map[string]any{
		"schema": SeedSchemaID,
		"goal":   "Explain how rain forms",
		"title":  "How Rain Forms",
		// The field a composer actually builds scenes from.
		"key_points": points,
	}
	raw, err := json.MarshalIndent(manifest, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, SeedManifestFile), raw, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)

	seed, res, err := LoadSeed(&SeedRef{
		Schema: SeedSchemaID, Path: root, Digest: hex.EncodeToString(sum[:]),
	})
	if err != nil {
		t.Fatalf("a seed carrying key_points failed to load: %v", err)
	}
	if !res.Verified {
		t.Fatal("a matching digest did not verify")
	}

	// The material a composer needs must survive the load. Dropping key_points
	// would leave a title card and nothing to say — the shape every earlier
	// test happened to accept.
	if len(seed.KeyPoints) != len(points) {
		t.Fatalf("key_points: got %d, want %d; there is nothing to build scenes from",
			len(seed.KeyPoints), len(points))
	}
	for i, want := range points {
		if seed.KeyPoints[i] != want {
			t.Errorf("key_points[%d] = %q, want %q", i, seed.KeyPoints[i], want)
		}
	}
	if seed.Title == "" || seed.Goal == "" {
		t.Error("title or goal was lost; the opening scene has no subject")
	}
}

// evidence_digest is the PRODUCER's claim about its evidence set, and Facet
// reports it verbatim rather than recomputing it.
//
// That is deliberate: Facet reads manifest.json and the attachments it names,
// never evidence.jsonl, so it has not seen the bytes the digest describes.
// Recomputing would mean either reading evidence it does not consume or
// inventing a value — and a provenance field a consumer cannot trust is worse
// than one that is honestly second-hand.
func TestEvidenceDigestIsReportedNotRecomputed(t *testing.T) {
	root := t.TempDir()
	const claimed = "1111111111111111111111111111111111111111111111111111111111111111"
	manifest := map[string]any{
		"schema": SeedSchemaID, "title": "T", "evidence_digest": claimed,
	}
	raw, err := json.MarshalIndent(manifest, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, SeedManifestFile), raw, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)

	_, res, err := LoadSeed(&SeedRef{
		Schema: SeedSchemaID, Path: root, Digest: hex.EncodeToString(sum[:]),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.EvidenceDigest != claimed {
		t.Errorf("evidence_digest = %q, want the producer's claim %q reported verbatim",
			res.EvidenceDigest, claimed)
	}
	// It must not be confused with the digest Facet DID verify.
	if res.EvidenceDigest == res.DigestActual {
		t.Error("evidence_digest was replaced by the manifest digest; they identify different things")
	}
}

// A seeded run reports its provenance even when it produces NO artifact.
//
// The manifest was attached only when artifacts existed, so a seeded probe,
// review or listing returned no manifest at all. The seed had been read and
// its digest verified, and that fact vanished silently — a caller asking "did
// Facet actually use my seed?" got no answer from a successful run.
//
// An empty artifact list is the honest answer: the seed was consumed and
// verified, and this run produced nothing to attribute.
func TestSeededRunReportsProvenanceWithoutArtifacts(t *testing.T) {
	root := t.TempDir()
	evidence := []byte("{\"id\":\"e1\"}\n")
	if err := os.WriteFile(filepath.Join(root, "evidence.jsonl"), evidence, 0600); err != nil {
		t.Fatal(err)
	}
	evSum := sha256.Sum256(evidence)
	manifest := map[string]any{
		"schema": SeedSchemaID, "title": "T",
		"evidence_digest": hex.EncodeToString(evSum[:]),
	}
	raw, err := json.MarshalIndent(manifest, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, SeedManifestFile), raw, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)

	rootJSON, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	digestJSON, err := json.Marshal(hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatal(err)
	}
	// music_library succeeds and writes nothing.
	body := []byte(`{"tool":"music_library","input":{},
	  "seed":{"schema":"` + SeedSchemaID + `","path":` + string(rootJSON) +
		`,"digest":` + string(digestJSON) + `}}`)

	env := Invoke(CapToolsRun, body)
	if !env.OK {
		t.Skipf("music_library unavailable: %+v", env.Error)
	}
	result, ok := env.Result.(map[string]any)
	if !ok {
		t.Fatalf("result is %T, not a manifest wrapper", env.Result)
	}
	m, ok := result["manifest"].(ArtifactManifest)
	if !ok {
		t.Fatal("a seeded run reported no manifest; the caller cannot tell the seed was used")
	}
	if m.Source == nil || !m.Source.Verified {
		t.Error("the manifest does not record that the seed verified")
	}

	// The two digests identify DIFFERENT things and must not be conflated.
	if m.Source.DigestActual != hex.EncodeToString(sum[:]) {
		t.Errorf("digest_actual = %q, want the manifest.json hash", m.Source.DigestActual)
	}
	if m.Source.EvidenceDigest != hex.EncodeToString(evSum[:]) {
		t.Errorf("evidence_digest = %q, want the evidence.jsonl hash reported verbatim",
			m.Source.EvidenceDigest)
	}
	if m.Source.DigestActual == m.Source.EvidenceDigest {
		t.Error("the two digests are identical; they identify different things")
	}
}

// SeedRef.digest covers the FILE Facet reads, which is manifest.json — not the
// evidence set. Evidence identity travels separately in evidence_digest so
// regenerating brief.md prose does not invalidate an evidence reference.
//
// A producer supplying the evidence hash as SeedRef.digest is refused, and the
// refusal is what tells them the field means something else.
func TestSeedRefDigestCoversTheManifestNotTheEvidence(t *testing.T) {
	root := t.TempDir()
	evidence := []byte("{\"id\":\"e1\"}\n")
	if err := os.WriteFile(filepath.Join(root, "evidence.jsonl"), evidence, 0600); err != nil {
		t.Fatal(err)
	}
	evSum := sha256.Sum256(evidence)
	raw, err := json.MarshalIndent(map[string]any{
		"schema": SeedSchemaID, "title": "T",
		"evidence_digest": hex.EncodeToString(evSum[:]),
	}, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, SeedManifestFile), raw, 0600); err != nil {
		t.Fatal(err)
	}

	// The evidence hash is the WRONG value for this field.
	if _, _, err := LoadSeed(&SeedRef{
		Schema: SeedSchemaID, Path: root, Digest: hex.EncodeToString(evSum[:]),
	}); err == nil {
		t.Error("the evidence digest was accepted as the seed reference digest")
	}

	// The manifest hash is the right one.
	sum := sha256.Sum256(raw)
	if _, res, err := LoadSeed(&SeedRef{
		Schema: SeedSchemaID, Path: root, Digest: hex.EncodeToString(sum[:]),
	}); err != nil || !res.Verified {
		t.Errorf("the manifest digest was not accepted: %v", err)
	}
}
