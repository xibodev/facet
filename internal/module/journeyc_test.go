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
