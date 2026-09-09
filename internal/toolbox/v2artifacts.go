package toolbox

// Artifact kinds for xibodev.module/v2 — the Value Contract layer.
//
// The three kinds are NOT interchangeable labels:
//
//	media     validated by media type, size and digest. NEVER by JSON Schema.
//	          Naming a validator here is an error, not an omission: a schema
//	          cannot validate an mp4, so the declaration would promise a check
//	          nothing can perform.
//	text      no validation contract beyond media type and digest. An HONEST
//	          statement that no validator exists.
//	document  MUST name a validator that validates the artifact's OWN CONTENT.
//
// Facet emits six media and two text kinds, and declares ZERO validators.
// captions is `text` rather than `document` because Facet WRITES SRT and never
// parses it — there is no validator to name, and declaring one that validates
// only content Facet produced itself would be a check pointed at its own
// output.
//
// This set was validated against facet-studio's real §7 implementation during
// cross-lane review before being published here: all eight accepted with no
// findings, captions-as-document correctly refused, and mp4-with-JSON-Schema
// correctly refused as an ERROR.

// V2ArtifactKind is one declared output type.
type V2ArtifactKind struct {
	Kind      string `json:"kind"`
	MediaType string `json:"media_type"`
	// Validator is deliberately ABSENT for every Facet kind. The field exists
	// so a `document` kind could name one; Facet has no document artifacts.
	Validator *V2Validator `json:"validator,omitempty"`
}

// V2Validator names a check over an artifact's own content.
type V2Validator struct {
	Type   string `json:"type"`
	Schema string `json:"schema"`
}

// THE RELOCATION, stated because the two fields now look similar.
//
// The v1 `artifact_schemas` map holds 21 entries, of which 20 describe things
// an AGENT AUTHORS — a brief, a scene plan, a review, a rig plan — and exactly
// one describes what a tool writes. It was the module's index of every schema
// it knows, and the host reads it as "artifact kinds this module emits", which
// was never true of the other twenty.
//
// v2 does not fix that by deleting anything: `artifact_schemas` is a published
// v1 contract and stays exactly as it was. Instead `artifact_kinds` is the
// EMITTED surface and carries only the eight real outputs. The authoring
// formats remain available where they were always published.
//
// So the same document now says two different true things: here is every
// schema I know (v1), and here is what I actually produce (v2).

// V2ArtifactKinds returns the artifact kinds Facet actually emits.
//
// Every kind here is referenced by at least one Operation's `produces`, and
// every `produces` entry names a kind here. A kind nothing produces is the
// declared-but-never-emitted defect; a produces entry naming no kind is a
// dangling reference. Both are asserted in tests.
func V2ArtifactKinds() map[string]V2ArtifactKind {
	return map[string]V2ArtifactKind{
		"render_video": {Kind: "media", MediaType: "video/mp4"},
		"frame":        {Kind: "media", MediaType: "image/jpeg"},
		"narration":    {Kind: "media", MediaType: "audio/mpeg"},
		"image":        {Kind: "media", MediaType: "image/png"},
		// SRT is written and never parsed here, so no validator is declared.
		"captions": {Kind: "text", MediaType: "text/plain"},

		// NOT DECLARED, deliberately: audio/flac, application/pdf and a
		// tool_log kind. Facet's media-type DETECTION table recognises flac
		// and PDF because it READS them; it emits mp3 and writes no PDF. A
		// kind declared here that no Operation produces is a promise the host
		// indexes and nothing fulfils -- the declared-but-never-produced class
		// this repo has already paid for nine times.
		//
		// Caught by TestSerializedProducesAndKindsAgree on its first run, in
		// the same hour it was written, against a declaration I had just made.
	}
}
