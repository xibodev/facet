package module

import "testing"

// Journey C: a REAL Midden seed, staged elsewhere, consumed by Facet.
func TestJourneyCRealSeed(t *testing.T) {
	seed, res, err := LoadSeed(&SeedRef{Schema: SeedSchemaID, Path: `E:\open-source-projects\midden\.tmp-jc\staged`, Digest: "e3e1dc2c397fa1647fe13f7cd083aef840e2c9db250e7641f0dfa851d5a99ecf"})
	if err != nil {
		t.Fatalf("Facet REJECTED a real Midden seed: %v", err)
	}
	t.Logf("verified=%v", res.Verified)
	t.Logf("goal=%q", seed.Goal)
	t.Logf("title=%q", seed.Title)
	t.Logf("output_types=%v", seed.OutputTypes)
	if !res.Verified {
		t.Error("digest did not verify")
	}
}
