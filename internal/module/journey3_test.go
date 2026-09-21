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
