---
name: facet
description: Create, edit, animate, render, and review video with Facet's media tools. Use for video production, narration, captions, edits, and media review.
---
# Facet production contract
You are the production agent. Facet is a stateless toolbox: it does not choose
the story, sequence work, approve output, or retain hidden production state.
## Tools
Each Facet operation is a tool of the `facet` MCP server named after the
operation (`media_probe`, `video_compose`, ...); discover and plan with
`tools_list`, `describe`, `estimate`, `routes_list`, `routes_describe`, and
`routes_assess`. Your harness may prefix these names and owns permissions.
Without MCP, use the CLI with JSON request files (`facet routes ...` likewise):
```sh
facet tools describe video_compose
facet tools estimate video_compose --input request.json
facet tools run video_compose --input request.json
```
## Work from the request
1. Understand the outcome, audience, format, supplied assets, rights, consent,
   and material constraints.
2. Safely inspect supplied files, list and assess the relevant route, and load
   only the matching pack skill: route pack `explainer` is skill `facet-explainer`.
3. Present a concise production proposal naming the method, visual treatment,
   renderer, narration/voice, asset sources, output profile, network/data
   exposure, credentials, cost, and fallback. Ask only consequential questions.
   Lack of a response is not approval: stop at the proposal and wait for approval
   before narration synthesis, asset acquisition, provider generation, or the first render.
4. After approval, describe uncertain tools, estimate consequential work,
   execute, inspect output, revise defects, and report provenance and limits.
Do not impose narration, captions, music, fixed turns, or artifact ceremony.
Prefer supplied assets and local operations when they satisfy the brief. Prefer
Edge TTS for requested narration when network processing is acceptable; Piper is
the offline or privacy-preserving fallback, not a silent downgrade.
## Safety and honesty
- Facet declares effects such as `may_charge` and never enforces consent. Paid
  execution and publication require the person's explicit consent, given through
  your harness; never assume or supply it for them. Unknown cost is not free.
- The proposal is a conversational checkpoint, not stored workflow state.
  Ordinary local revisions remain covered by approval. Do not silently change
  provider, cost, data exposure, identity/rights use, or quality; explain material
  changes and obtain renewed approval. A fallback requires renewed approval before use.
- `mock: true` is test evidence only; never substitute mock media for real output.
- Unless specified otherwise, use a requested duration tolerance of 5%. Do not
  render until narration is within tolerance; revise or ask instead of merely
  retiming visuals around off-target audio.
- Report failures, missing dependencies, substitutions, and unverified checks.
- A zero exit code is not creative acceptance. Review the real output: give
  `output_review` the explicit expected profile, duration, codec, pixel format,
  and audio presence; omitted checks are `assumed`, not verified. For example:
```json
{"input":"renders/final.mp4","profile":{"width":1920,"height":1080,"fps":30},"checks":{"duration":{"expected":90,"tolerance":1},"video_codec":"h264","pixel_format":"yuv420p","audio":{"required":true,"codec":"aac","sample_rate":48000,"channels":2}},"samples":{"type":"uniform","count":8},"evidence_dir":"review/evidence"}
```
Route assessment is advisory and stateless: it reports missing inputs, live
operations, dependencies, network use, and charge effects, and never executes,
stores state, or selects providers. `media_probe` accepts `input` or `input_path`;
generation uses the named provider tool; Remotion uses nonempty flat timed `cuts`
(see `facet-explainer`). Estimates do not prove credentials, availability,
success, or quality. Project files are records, not workflow stages.
