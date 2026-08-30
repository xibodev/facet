# Video Kit Donors And Migration Matrix

## Donor Precedence

1. Clean upstream OpenMontage is the primary behavioral and toolbox donor.
2. The current video-agent-bundle is a selective salvage donor.
3. The Facet-modified product-video-factory checkout is quarantined during the
   headless phase.

No donor overrides `DESIGN.md`.

## Primary Donor: Upstream OpenMontage

- Repository: <https://github.com/calesthio/OpenMontage>
- Pinned commit: `cd9f3c1f03368be87b140af494914b8ee4e3c7a4`
- License: GNU AGPL v3. Video Kit will use a GNU AGPL v3-compatible repository
  license because it will directly copy, adapt, and behaviorally port substantial
  upstream material. Preserve required notices and source obligations.
- The URL and commit pin are authoritative. Materialize a clean local reference
  from this exact commit when Phase 1 donor inventory begins; do not depend on a
  temporary checkout path.

Do not silently update this commit. Any upstream rebase or selective adoption of
a later change must be recorded here before code is copied.

### Upstream Migration Matrix

| Upstream material | Treatment | Reason |
|---|---|---|
| `tools/` provider clients | Port behavior to Go | This is the proven toolbox we need, not greenfield client design. |
| Tool names and capability identities | Preserve where useful | Keeps upstream knowledge, schemas, and examples coherent. |
| Tool input/output schemas | Preserve or compatibly normalize | Enables behavior-parity fixtures and predictable agent use. |
| Base tool contract | Reimplement once in Go | Preserve useful semantics without Python reflection or inheritance. |
| Tool registry and capability menu | Reimplement simply in Go | Claude needs live implemented/configured facts; no host required. |
| Provider capability facts and ranking signals | Port only when transparent and explainable | Go may expose candidate facts, reject impossible candidates, or provide requested mechanical fallback, but Claude makes the creative and commercial choice. |
| Shared HTTP/upload/poll/download logic | Port once, then reuse | Avoid repeated provider-client reinvention. |
| `pipeline_defs/` | Curate as production-method donor | Pipelines exist upstream; ours may simplify format and taxonomy. |
| Root governing instructions | Rewrite | The agentic surface is intentionally product-owned and Claude-first. |
| Pipeline director skills | Curate and consolidate | Preserve expertise without copying repetition or mandatory ceremony. |
| Creative/core/meta skills | Selectively retain and edit | Keep proven craft knowledge relevant to the new surface. |
| `.agents/skills/` technology packs | Retain selectively | Load only after selecting a relevant tool or runtime. |
| `schemas/` | Reuse or normalize | Preserve useful artifact/tool contracts without host state schemas. |
| `styles/` | Reuse and curate | Keep production language; simplify only for a concrete need. |
| `remotion-composer/` | Import from the clean pin | Reuse the proven React composition engine. |
| HyperFrames integration and skills | Reuse/invoke | HyperFrames remains an existing runtime, not a Go rewrite target. |
| FFmpeg behavior | Invoke and port wrappers | FFmpeg remains the media engine. |
| Checkpoint files | Optional ordinary-file guidance, no Go controller | Claude can resume natively or inspect project artifacts. |
| Backlot | Exclude | UI is deferred; it is not required for headless production. |
| Setup/install scripts | Exclude initially | Packaging and broad environment setup are deferred. |
| Local GPU implementations | Defer | First pass targets stock, supplied media, existing binaries, and cloud clients. |
| Tests and fixtures | Use as behavioral evidence | Port focused contracts; do not copy irrelevant host or UI tests. |

Any ported ranking must return candidates, inputs, signals, and rationale. It
must not silently execute the top-ranked provider. Existing upstream selector
code may be ported only where it exposes transparent candidate facts, rejects
mechanically impossible candidates, provides explainable ranking signals, or
implements an explicitly requested mechanical fallback.

## Selective Salvage Donor: Current Video Agent Bundle

- Repository: <https://github.com/xibodev/video-agent-bundle>
- Checkout: `C:\Users\gafar\AppData\Local\Temp\opencode\video-agent-bundle`
- Published HEAD: `ff272c2812008e6563eddd6bab981513c15c460a`
- Important note: useful later work is currently uncommitted. Treat the exact
  working tree as evidence; do not reset, clean, modify, or assume HEAD contains
  all salvage candidates.

### Salvage Candidates

| Current material | Treatment |
|---|---|
| Go process execution and structured JSON results | Review and salvage. |
| FFmpeg/ffprobe inspection | Review and salvage. |
| Browser capture | Review and salvage. |
| Source ingestion, transcript validation, and hashes | Review and salvage. |
| Speech invocation and timing mechanics | Review and salvage where compatible with broader upstream TTS contracts. |
| Remotion render invocation | Review and salvage. |
| Derivative cut-map validation | Review and salvage as a media/edit tool, not a pipeline framework. |
| Output profiles and machine media gates | Review and salvage. |
| Source-audio and source-vignette fixes | Apply to the clean upstream composer if still needed. |
| Caption contrast controls | Apply if still needed against the pinned composer. |
| Added diagram primitives | Review as optional composer improvements. |
| Renderer dependency checks | Review and salvage minimally. |
| Focused Go and composer tests | Port only with the mechanics they verify. |

### Dirty-Worktree Provenance

Before reviewing or salvaging individual files, Phase 1 must capture a read-only
provenance snapshot outside the donor checkout containing:

- repository URL and HEAD commit;
- full `git status`;
- tracked working-tree diff, including binary metadata where appropriate;
- untracked paths;
- hashes of every file selected for review;
- whether each selected file came from committed HEAD, tracked modifications, or
  an untracked state.

For each salvaged mechanic, record whether it is direct code salvage, adapted
code, behavioral reference, or test/fixture reference. Do not modify, clean,
reset, stage, or commit the donor checkout, and do not treat unrelated dirty
files as donor material automatically.

### Reject From Current Bundle

- The assumption that OpenMontage donated only its composer.
- The reduced hand-written capability catalog as the complete toolbox.
- Global `planned`, `development`, and `operational` pipeline statuses.
- Pipeline-by-pipeline promotion and acceptance campaigns.
- Human audio approval as a global pipeline availability gate.
- Cross-CLI design work before Claude is complete.
- Any host, daemon, state machine, or future UI architecture.
- The current taxonomy as an unquestioned replacement for upstream methods.

## Quarantined Donor: Facet-Modified Checkout

- Checkout: `E:\testgrounds\product-video-factory`
- Recorded HEAD: `bd66e4fdb1f16a1d645322eda562a7ae7fc2d48a`
- Status: extensively modified and dirty.

Do not read or copy this checkout during the headless implementation. It is not
an authority for upstream OpenMontage behavior. Its Production Kernel, agent
gateway, MCP bridge, capability packs, application state, database, process
supervision, packaging, and UI host are explicitly excluded.

Later, after the headless product works, the user may name specific UI concepts
or components for separate evaluation. That future permission does not make
Facet an architecture donor now.

## Existing Clean OpenMontage Checkout

`E:\open-source-projects\OpenMontage` is a clean upstream checkout at commit
`4eab34c5cfcccaa4f1970554928feccce73ee930`. It is not the pinned primary donor
for this project because it differs from `cd9f3c1...`. Leave it untouched unless
the user explicitly changes the donor pin or requests a comparison.

## Provenance Rules

- Record the source path and pinned commit for every copied upstream tree.
- Preserve the upstream license and required notices.
- Distinguish direct copies, adapted copies, behavioral ports, and original code.
- Do not describe a Go behavioral port as an original greenfield design.
- Do not read secrets, credentials, tokens, or unrelated generated projects from
  any donor.
- Never modify donor worktrees while salvaging from them.

## Licensing And Notices

Before the first direct copy or adaptation, Phase 1 must add the repository
`LICENSE`, third-party notices, and a provenance mechanism for imported and
ported units. Classify each unit as direct copy, adapted copy, behavioral Go
port, or original Video Kit work.

Preserve separate terms for Remotion, HyperFrames, npm dependencies, provider
SDKs, media assets, fonts, and other third-party components. Importing
OpenMontage's composer does not relicense Remotion packages under the GNU AGPL.
