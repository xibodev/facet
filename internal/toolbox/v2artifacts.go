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

// The v1 `artifact_schemas` field declares the generic output kind needed by
// legacy hosts. This v2 map adds precise media/text kinds for actual tool
// outputs. Workflow authoring documents are not emitted tool contracts and
// are intentionally absent from both surfaces.

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
