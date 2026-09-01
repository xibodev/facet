# Provenance Classification

## Purpose

This file is the unit-level provenance mechanism for Video Kit. It complements
`DONORS.md`; it does not replace donor precedence, licensing obligations, commit
pins, or third-party notices. Add an entry before or with each imported,
adapted, or behaviorally ported unit.

## Classifications

- **Direct copy:** source text or code is reproduced substantially verbatim.
  Record donor repository, pinned commit, source path, destination path, license,
  notices, and any mechanical-only changes.
- **Adapted copy:** recognizable donor expression is modified, reorganized, or
  translated while retaining substantial source structure or wording. Record
  the same fields as a direct copy and summarize the adaptation.
- **Behavioral Go port:** Go code is newly expressed from observed donor
  behavior, contracts, schemas, tests, or fixtures rather than copied source
  expression. Record the behavioral evidence and parity tests; do not describe
  it as greenfield or original design.
- **Original:** the unit is independently authored for Video Kit without donor
  source expression or behavior as its basis. General architecture constraints
  and factual interoperability with FFmpeg or other runtimes do not alone make a
  unit a donor port.

Classification applies per reviewable unit, not automatically to a whole file
or directory. A file containing mixed origins must identify the relevant
symbols, sections, generated assets, or line ranges separately.

## Required Unit Record

| Field | Requirement |
|---|---|
| Unit | Stable destination path plus symbol, section, asset, or bounded range. |
| Classification | Exactly one of direct copy, adapted copy, behavioral Go port, or original. |
| Donor | Repository URL/name, or `none` for original work. |
| Revision | Exact commit or captured dirty-worktree state. |
| Source | Donor path(s), fixture(s), schema(s), or behavioral evidence; `none` for original work. |
| Destination | Video Kit path and unit. |
| License/notices | Applicable source license and required notice location, or `none`. |
| Changes/evidence | Adaptation summary, parity evidence, or concise originality basis. |

Use this table for future units:

| Unit | Classification | Donor | Revision | Source | Destination | License/notices | Changes/evidence |
|---|---|---|---|---|---|---|---|
| _Add before or with the unit_ |  |  |  |  |  |  |  |

## Current Phase 1 Records

| Unit | Classification | Donor | Revision | Source | Destination | License/notices | Changes/evidence |
|---|---|---|---|---|---|---|---|
| Phase 1 pipeline classification document | Original | none | n/a | Shallow inspection of all 13 `pipeline_defs/*.yaml` definitions at clean upstream OpenMontage commit `cd9f3c1f03368be87b140af494914b8ee4e3c7a4` | `PIPELINE_MAP.md` | none | Original Video Kit taxonomy and rationale informed by the complete shallow definition census; no donor expression copied. |
| First-slice selection and finalized contract | Original | none | n/a | Behavioral evidence from clean upstream OpenMontage commit `cd9f3c1f03368be87b140af494914b8ee4e3c7a4`: `tools/base_tool.py`, `tools/tool_registry.py`, `tools/analysis/audio_probe.py`, `tools/analysis/composition_validator.py`, `tools/analysis/frame_sampler.py`, `tools/analysis/scene_detect.py`, `tools/analysis/visual_qa.py`, `tools/audio/audio_mixer.py`, `tools/video/video_compose.py`, `tools/video/video_stitch.py`, and `tools/video/video_trimmer.py` | `PHASE1_SLICE.md` | none | Original normalized Video Kit contract combined with behavioral evidence from the named selected-slice paths; no implementation included. |
| Provenance classification mechanism | Original | none | n/a | `DONORS.md` provenance requirements | `PROVENANCE.md` | none | Product-owned unit-record format. |
| Repository license | Direct copy | OpenMontage | `cd9f3c1f03368be87b140af494914b8ee4e3c7a4` | `LICENSE` | `LICENSE` | GNU AGPL v3; `THIRD_PARTY_NOTICES.md` | Direct copy of the upstream `LICENSE`; identical SHA-256 `0D96A4FF68AD6D4B6F1F30F713B18D5184912BA8DD389F86AA7710DB079ABCB0`. |
| Third-party notices document | Original | none | n/a | Notices and licensing facts in the pinned donor | `THIRD_PARTY_NOTICES.md` | `LICENSE`; future component-specific notices must be added here | Original Video Kit notice structure informed by donor notices; it does not claim unimported components. |
| Toolbox contract, registry, envelopes, validation, and process execution | Behavioral Go port | OpenMontage | `cd9f3c1f03368be87b140af494914b8ee4e3c7a4` | `tools/base_tool.py`; `tools/tool_registry.py` | `internal/toolbox/toolbox.go`: public catalog, CLI dispatch, envelopes, validation, dependency facts, and `runCommand` | GNU AGPL v3; `LICENSE` | Newly expressed Go behavior using only the Phase 1 recorded contract; strict JSON, bounded diagnostics, and direct `exec.CommandContext` invocation have focused Go tests. No donor source expression was copied. |
| Media probe and deterministic frame sampling | Behavioral Go port | OpenMontage | `cd9f3c1f03368be87b140af494914b8ee4e3c7a4` | `tools/analysis/audio_probe.py`; `tools/analysis/frame_sampler.py`; `tools/analysis/scene_detect.py` | `internal/toolbox/toolbox.go`: `probe`, `doMediaProbe`, `resolveSamples`, `sceneSamples`, and `doFrameSample` | GNU AGPL v3; `LICENSE` | Newly expressed ffprobe normalization, SHA-256 identity, timestamp validation, FFmpeg scene detection, deterministic representative fallback, and frame extraction behavior; integration tests use generated media. |
| Supplied-footage source editing | Behavioral Go port | OpenMontage | `cd9f3c1f03368be87b140af494914b8ee4e3c7a4` | `tools/video/video_compose.py`; `tools/video/video_trimmer.py`; `tools/video/video_stitch.py` | `internal/toolbox/toolbox.go`: `doSourceEdit` and target/timeline validation | GNU AGPL v3; `LICENSE` | Newly expressed single-process FFmpeg filter graph for ordered trims, contain/cover normalization, silent-audio synthesis, safe concat, and optional replacement audio. Real integration test covers mixed source-audio presence. |
| Audio mixing and normalization | Behavioral Go port | OpenMontage | `cd9f3c1f03368be87b140af494914b8ee4e3c7a4` | `tools/audio/audio_mixer.py` | `internal/toolbox/toolbox.go`: `doAudioMix`, filter construction, and loudnorm parsing | GNU AGPL v3; `LICENSE` | Newly expressed deterministic source/music gains and fades, optional sidechain ducking, AAC normalization, and loudnorm fact reporting. |
| Technical output review | Behavioral Go port | OpenMontage | `cd9f3c1f03368be87b140af494914b8ee4e3c7a4` | `tools/analysis/visual_qa.py`; `tools/analysis/composition_validator.py` | `internal/toolbox/toolbox.go`: `doOutputReview`, gates, sampling, and volumedetect parsing | GNU AGPL v3; `LICENSE` | Newly expressed mechanical gates with four inspectable frames and volume facts; execution status remains separate from review acceptance. |
| Go CLI entry point and Phase 2 tests | Original | none | n/a | Finalized Video Kit CLI and test requirements in `DESIGN.md` and `PHASE1_SLICE.md` | `cmd/videokit/main.go`; `internal/toolbox/toolbox_test.go`; `go.mod` | FFmpeg runtime fact in `THIRD_PARTY_NOTICES.md` | Independently authored wiring and focused contract/integration tests. FFmpeg and ffprobe are invoked system dependencies and are not copied or distributed. |

These records do not classify future pipeline guidance, schemas, toolbox code,
composer files, tests, or assets. Classify those only after their actual source
and treatment are known.

## Dirty Selective-Salvage Donor

The reproducible snapshot for the current `video-agent-bundle` working tree is
at [`provenance/video-agent-bundle/`](provenance/video-agent-bundle/). Its
`MANIFEST.md`, Git evidence, candidate hashes, and snapshot checksums establish
the captured revision and source state. A snapshot candidate is not approved
salvage and has no direct/adapted/behavioral classification until a specific
unit is reviewed and entered above.

For a unit derived from that donor, cite the snapshot manifest, captured HEAD
`ff272c2812008e6563eddd6bab981513c15c460a`, the candidate's recorded source
state, and its SHA-256 entry. Never silently substitute the published HEAD for a
tracked modification or untracked candidate.

## Primary And Third-Party Sources

For OpenMontage-derived units, cite the authoritative pin in `DONORS.md`, source
path, and applicable GNU AGPL v3 obligations. Record Remotion, HyperFrames,
FFmpeg, npm packages, provider SDKs, media, and fonts under their own terms;
OpenMontage provenance does not relicense independent dependencies or assets.

`LICENSE`, `THIRD_PARTY_NOTICES.md`, and this provenance mechanism are present,
so the repository-level licensing gate is established. Before or with every
future copied, adapted, or behaviorally ported unit, update the unit record above
and add any component-specific license, attribution, and notice required by
`THIRD_PARTY_NOTICES.md` and `DONORS.md`.

<!-- phase-2-original-agent-surface:start -->
## Phase 2 Original Agent Surface

| Unit | Classification | Donor | Revision | Source | Destination | License/notices | Changes/evidence |
|---|---|---|---|---|---|---|---|
| Root Claude producer skill | Original | none | n/a | `DESIGN.md`; `PLAN.md`; finalized first-slice decision in `PHASE1_SLICE.md` | `SKILL.md` | none | Independently authored concise routing and production guidance; no donor governing Markdown inspected or copied. |
| Source-edit production method | Original | none | n/a | Video Kit architecture and selected source-edit requirements in `DESIGN.md` and `PHASE1_SLICE.md` | `skills/methods/source-edit.md` | none | Independently authored method guidance for the selected local FFmpeg route; no donor expression copied. |
| Shared review and editing guidance | Original | none | n/a | Video Kit review responsibilities and selected-slice requirements in `DESIGN.md` and `PHASE1_SLICE.md` | `skills/shared/review-editing.md` | none | Independently authored editorial and output-review guidance; no donor expression copied. |
<!-- phase-2-original-agent-surface:end -->

## Phase 2 Original Production Fixture

| Unit | Classification | Donor | Revision | Source | Destination | License/notices | Changes/evidence |
|---|---|---|---|---|---|---|---|
| First-real-video fixture and reproduction record | Original | none | n/a | Independently specified FFmpeg synthetic test sources and sine tones; no user, stock, generated-provider, donor, or third-party media | `projects/phase2-source-edit/assets/source/source.mp4`; `projects/phase2-source-edit/reproduce.ps1`; production records under `projects/phase2-source-edit/` | none; FFmpeg is an invoked system runtime recorded in `THIRD_PARTY_NOTICES.md` | The script reproducibly generates the 16-second landscape audiovisual fixture, then invokes Video Kit for discovery, inspection, editorial composition, audio finishing, and review. The fixture carries no claim of real-world footage. |

## Phase 2 Toolbox Hardening

| Unit | Classification | Donor | Revision | Source | Destination | License/notices | Changes/evidence |
|---|---|---|---|---|---|---|---|
| Atomic media and frame publication, cancellable hashing, structured schemas, strict estimates, crop placement, audio validation, and review partial-result semantics | Original | none | n/a | Defects observed during Phase 2 implementation review and requirements in `DESIGN.md` and `PHASE1_SLICE.md`; no donor or Facet source inspected | `internal/toolbox/toolbox.go` | none; FFmpeg remains an invoked runtime recorded in `THIRD_PARTY_NOTICES.md` | Independently authored hardening: validate temporary media before rollback-safe replacement, stage frame sets, expose concrete schema objects, separate estimate eligibility from run decoding, validate audio operations, add focal crop placement, remove contradictory pass messages, and make hashing deadline-aware. |
| Phase 2 hardening regressions | Original | none | n/a | Phase 2 toolbox findings and public CLI contract | `internal/toolbox/toolbox_test.go` | none | Independently authored tests for structured descriptions, estimate validation and non-mutation, rollback, frame-set publication, no-op/ducking rejection, cut-only framing, gate wording, hash cancellation, operation reporting, and CLI error JSON. |
| Cut-only and validation contract clarification | Original | none | n/a | Approved selected-slice scope and implemented toolbox behavior | `PHASE1_SLICE.md`; `skills/methods/source-edit.md` | none | Documents per-segment position/focal point, cut-only transitions, strict audio ranges, side-effect-free estimate boundaries, and coherent frame/review partial semantics. |
