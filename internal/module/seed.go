package module

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SeedSchemaID is the Midden content-seed contract Facet consumes.
//
// Facet consumes a seed by path plus digest only. It never invokes Midden,
// never imports a Midden client, and never reads Midden's database. The host
// agent decides when recovery is needed and hands the resulting seed to Facet.
const SeedSchemaID = "xibodev.midden.seed/v1"

// SeedManifestFile is the entry file inside a seed bundle directory.
const SeedManifestFile = "manifest.json"

// Seed is Facet's read model of a Midden content seed's manifest.json.
//
// It is deliberately permissive: Midden owns this contract, so Facet validates
// only what it must consume. Every field is optional except the schema — a seed
// carrying only a goal is valid.
//
// SuggestedOutputTypes is advisory. Facet owns the vocabulary
// (explainer, screen-demo, social-clip, talking-head, cinematic, localization)
// and an unrecognised value degrades to "no suggestion" with a warning rather
// than failing the seed, so a new Facet output type is never a breaking change
// in Midden's schema.
type Seed struct {
	Schema               string   `json:"schema"`
	Goal                 string   `json:"goal,omitempty"`
	Title                string   `json:"title,omitempty"`
	Summary              string   `json:"summary,omitempty"`
	KeyPoints            []string `json:"key_points,omitempty"`
	Attachments          []string `json:"attachments,omitempty"`
	SuggestedOutputTypes []string `json:"suggested_output_types,omitempty"`
	EvidenceDigest       string   `json:"evidence_digest,omitempty"`
}

// knownOutputTypes is Facet's vocabulary, mapped to real installed packs.
var knownOutputTypes = map[string]bool{
	"explainer": true, "screen-demo": true, "social-clip": true,
	"talking-head": true, "cinematic": true, "localization": true,
}

// OutputTypes returns the recognised suggestions and warnings for the rest.
// An unknown suggestion is never fatal.
func (s *Seed) OutputTypes() (accepted []string, warnings []string) {
	accepted = []string{}
	warnings = []string{}
	for _, t := range s.SuggestedOutputTypes {
		key := strings.ToLower(strings.TrimSpace(t))
		if knownOutputTypes[key] {
			accepted = append(accepted, key)
			continue
		}
		warnings = append(warnings,
			"seed suggested an unrecognised output type: "+t+" (ignored)")
	}
	return accepted, warnings
}

// SeedResolution is the verified outcome of loading a seed.
//
// EvidenceDigest is the identity of the evidence set, distinct from the digest
// of the manifest bytes: it stays stable when brief.md prose is regenerated.
type SeedResolution struct {
	Schema         string `json:"schema"`
	Path           string `json:"path"`
	DigestExpected string `json:"digest_expected"`
	DigestActual   string `json:"digest_actual"`
	EvidenceDigest string `json:"evidence_digest,omitempty"`
	Verified       bool   `json:"verified"`
	// Attachments the seed named and Facet actually resolved, each with the
	// digest of the bytes on disk.
	//
	// The manifest names attachments relative to the bundle, and the field was
	// parsed and never read: Midden's video_brief — the document written
	// specifically to brief this render — was dropped, and nothing downstream
	// could tell it had existed. An artifact claiming provenance from a seed
	// should say which of the seed's documents it actually had.
	Attachments []SeedAttachment `json:"attachments,omitempty"`
}

// SeedAttachment is one file the seed named, resolved against the bundle.
//
// Path stays RELATIVE to the seed bundle for the same reason SeedResolution's
// does: the staged location is the host's and differs between machines.
// Missing is set rather than failing the load — a seed naming a file that did
// not survive staging is worth reporting, not worth refusing, because the
// evidence and manifest may still be exactly what the caller wanted.
type SeedAttachment struct {
	Path    string `json:"path"`
	Digest  string `json:"digest,omitempty"`
	Bytes   int    `json:"bytes,omitempty"`
	Missing bool   `json:"missing,omitempty"`
}

// LoadSeed reads a seed and verifies its digest.
//
// A seed is a DIRECTORY bundle whose entry file is manifest.json:
//
//	<seed-root>/manifest.json     entry, the shape Seed reads
//	<seed-root>/brief.md          prose for the host agent
//	<seed-root>/evidence.jsonl    the evidence set
//	<seed-root>/provenance.json   source identities and revisions
//	<seed-root>/attachments/      referenced files
//
// Path may point either at the seed root or directly at manifest.json; a
// directory is resolved to its manifest. Facet reads only manifest.json and the
// attachments it names — brief.md, evidence.jsonl, and provenance.json are for
// the host agent, not for the mechanical toolbox.
//
// Digest verification is mandatory when the caller supplies one: an unverified
// seed is refused rather than consumed, so a Facet artifact can never claim
// provenance from bytes it did not actually read. Digests are sha256 hex,
// lowercase, over raw bytes; an optional "sha256:" prefix is accepted.
//
// The digest supplied in a SeedRef covers the file that is read. Evidence
// identity is carried separately by the manifest's own evidence_digest, so
// regenerating brief.md prose does not invalidate an evidence reference.
func LoadSeed(ref *SeedRef) (*Seed, *SeedResolution, error) {
	if ref == nil {
		return nil, nil, fmt.Errorf("no seed reference supplied")
	}
	path := strings.TrimSpace(ref.Path)
	if path == "" {
		return nil, nil, fmt.Errorf("seed path is required")
	}
	if ref.Schema != "" && ref.Schema != SeedSchemaID {
		return nil, nil, fmt.Errorf("unsupported seed schema %q, expected %s", ref.Schema, SeedSchemaID)
	}

	// Resolve a seed root to its entry file. The host stages the seed and names
	// the path; Facet never reconstructs a path of its own.
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		path = filepath.Join(path, SeedManifestFile)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("seed could not be read: %w", err)
	}

	sum := sha256.Sum256(raw)
	actual := hex.EncodeToString(sum[:])
	expected := strings.ToLower(strings.TrimSpace(ref.Digest))
	expected = strings.TrimPrefix(expected, "sha256:")

	res := &SeedResolution{
		Schema: SeedSchemaID,
		// The seed's location is the HOST's, and it is staged — the path Facet
		// receives is not the path Midden wrote, and it differs between
		// machines. Recording it verbatim put an absolute host path into every
		// artifact manifest, which leaks the host's filesystem layout and is a
		// protocol violation on any artefact field.
		//
		// Provenance keys on the DIGEST, which is stable across staging; the
		// location is recorded only as the entry file's name inside the bundle,
		// which is all a reader needs to know what was read.
		Path:           SeedManifestFile,
		DigestExpected: expected,
		DigestActual:   actual,
		Verified:       expected != "" && expected == actual,
	}
	if expected != "" && !res.Verified {
		return nil, res, fmt.Errorf("seed digest mismatch: expected %s, computed %s", expected, actual)
	}

	var seed Seed
	if err := json.Unmarshal(raw, &seed); err != nil {
		return nil, res, fmt.Errorf("seed is not valid JSON: %w", err)
	}
	res.EvidenceDigest = strings.TrimSpace(seed.EvidenceDigest)
	// Resolved only AFTER the digest verifies: reading files named by a seed
	// whose manifest failed verification would be acting on bytes Facet has
	// just refused to trust.
	res.Attachments = resolveAttachments(filepath.Dir(path), seed.Attachments)
	return &seed, res, nil
}

// ArtifactManifest is what Facet emits after producing creative output.
//
// It retains the seed reference verbatim so a finished asset can be traced back
// to the exact bytes that produced it.
type ArtifactManifest struct {
	Schema     string          `json:"schema"`
	Module     string          `json:"module"`
	Capability string          `json:"capability"`
	RequestID  string          `json:"request_id"`
	Source     *SeedResolution `json:"source,omitempty"`
	Artifacts  []Artifact      `json:"artifacts"`
	Execution  Execution       `json:"execution"`
	Review     ReviewState     `json:"review"`
}

// ReviewState separates technical QA from creative acceptance.
//
// A passing technical check is never editorial approval. HumanApproved stays
// false until a person says otherwise; no agent may set it.
type ReviewState struct {
	TechnicalQA   string `json:"technical_qa"`
	HumanApproved bool   `json:"human_approved"`
	Note          string `json:"note"`
}

// ManifestSchemaID identifies Facet's render/artifact manifest contract.
const ManifestSchemaID = "xibodev.facet.artifact/v1"

// resolveAttachments reads each file the seed named and records its digest.
//
// The names are relative to the bundle and are treated as such: an absolute
// path or one escaping the bundle is refused rather than followed, because a
// seed is data from another module and a path in it must not be able to reach
// arbitrary files.
func resolveAttachments(bundle string, names []string) []SeedAttachment {
	if len(names) == 0 {
		return nil
	}
	out := make([]SeedAttachment, 0, len(names))
	for _, name := range names {
		clean := filepath.ToSlash(strings.TrimSpace(name))
		if clean == "" {
			continue
		}
		att := SeedAttachment{Path: clean}
		if isAbsolutePath(clean) || escapesRoot(clean) {
			// Not resolved, and deliberately not read.
			att.Missing = true
			out = append(out, att)
			continue
		}
		raw, err := os.ReadFile(filepath.Join(bundle, filepath.FromSlash(clean)))
		if err != nil {
			att.Missing = true
			out = append(out, att)
			continue
		}
		sum := sha256.Sum256(raw)
		att.Digest = "sha256:" + hex.EncodeToString(sum[:])
		att.Bytes = len(raw)
		out = append(out, att)
	}
	return out
}

// NewManifest builds a manifest from a completed module envelope.
func NewManifest(capability string, env Envelope, source *SeedResolution) ArtifactManifest {
	return ArtifactManifest{
		Schema:     ManifestSchemaID,
		Module:     ModuleID,
		Capability: capability,
		RequestID:  env.RequestID,
		Source:     source,
		Artifacts:  env.Execution.Artifacts,
		Execution:  env.Execution,
		Review: ReviewState{
			TechnicalQA:   "not_run",
			HumanApproved: false,
			Note:          "Technical QA is not creative acceptance; human review is required before delivery.",
		},
	}
}
