# Video Kit Plan

## Objective

Build the simplest useful Claude-first video production bundle by redesigning
OpenMontage's agentic surface and porting its relevant non-GPU Python toolbox to
Go, while retaining proven renderers and salvaging only useful current Go work.

## Authority

1. `DESIGN.md` defines the intended product and non-negotiable boundaries.
2. This file defines phase order and current work.
3. `DONORS.md` defines upstream provenance and migration treatment.
4. The pinned clean upstream OpenMontage snapshot defines donor behavior.
5. Existing repositories are evidence and salvage sources, not architecture
   authorities.

## Phase 0: Architecture Lock

- Approve the product, production, toolbox, and donor architecture.
- Record the GNU AGPL v3-compatible repository licensing posture.
- Keep the repository documentation-only.
- Stop before donor inventory or implementation.

Current state: Phase 0 was explicitly approved in commit `f4abdd5`.

## Phase 1: Donor Inventory And First-Slice Selection

After explicit architecture approval:

1. Materialize or inspect a clean copy of upstream OpenMontage at pinned commit
   `cd9f3c1f03368be87b140af494914b8ee4e3c7a4`.
2. Perform a complete shallow census of upstream pipelines, tools, schemas,
   styles, renderers, skills, provider clients, and related knowledge.
3. Classify capabilities and tools as first vertical slice, first release,
   later, or exclude using only the detail needed for prioritization.
4. Produce `PIPELINE_MAP.md` by inspecting every upstream pipeline definition
   and mapping it as required by `DESIGN.md`.
5. Produce an initially shallow `TOOL_PORT_MATRIX.md` covering every discovered
   tool by name, capability family, source path, rough dependencies, proposed
   treatment, and priority.
6. Capture the reproducible read-only provenance snapshot of the dirty current
   video-agent-bundle required by `DONORS.md` before reviewing individual files
   or selecting salvage candidates.
7. Add the repository `LICENSE`, third-party notices, and a provenance mechanism
   before the first direct copy or adaptation.
8. Select the first real-video vertical slice.
9. Deeply inventory only the selected slice's tools and immediate dependencies,
   including schemas, behavior, cost semantics, representative inputs, outputs,
   and parity fixtures.
10. Enrich `TOOL_PORT_MATRIX.md` incrementally as later capability batches
    approach implementation.
11. Finalize only the minimum Go tool contract needed by the selected slice.

Phase 1 permits only `DONORS.md`- and this-plan-approved inspection and
documentation. No implementation, copying, porting, generation, or salvage may
begin until the Phase 1 gate is complete.

Completed: the Phase 1 gate passed with `UPSTREAM_SURFACE_CENSUS.md`,
`PIPELINE_MAP.md`, `TOOL_PORT_MATRIX.md`, the immutable selective-donor snapshot
under `provenance/video-agent-bundle/`, `LICENSE`, `THIRD_PARTY_NOTICES.md`,
`PROVENANCE.md`, and the finalized supplied-footage source-edit decision and
minimum contract in `PHASE1_SLICE.md`.

Select the slice using these criteria:

- no paid credentials required;
- creates a real video;
- exercises live toolbox discovery;
- requires genuine Claude production decisions;
- uses ordinary project files;
- exercises one real renderer;
- can be reproduced quickly;
- is broad enough to expose integration defects;
- does not depend on deferred local GPU support.

Likely candidates are a supplied-footage edit using inspection, editing,
composition, and review, or a topic explainer using readily available speech,
code-native or open visuals, one renderer, and FFmpeg inspection. Phase 1 makes
the final choice after clean upstream inventory and salvage-cost comparison.

## Phase 2: Minimal Agent Surface And First Real Video

Implement only what the selected slice requires:

- a minimal root Claude producer/router skill;
- one or two relevant production methods;
- a minimal persona and progressively loaded production guidance;
- Go tool listing, description, estimation, and execution;
- required source, media, analysis, speech, composition, and review mechanics;
- one existing composition route;
- ordinary project files;
- one real, reproducible, non-paid production through Claude Code.

Phase 2 ends with an actual reviewed video, not only unit tests, schemas, or
mocked provider contracts. The agentic surface is a redesign; do not copy
upstream governing Markdown wholesale.

Completed: Phase 2 delivered the minimal Claude surface (`.claude/skills/video-kit/SKILL.md`),
the compiled Go toolbox, and the first reviewed source-edit production in `projects/phase2-source-edit/`.

## Phase 3: Production-Driven Expansion

Completed: Phase 3 expanded the Go toolbox to 33 tools across media analysis, FFmpeg editing,
audio mastering, open media, Edge/neural TTS, cloud image/video provider contracts with mocks,
color grading, and explainable selectors. All tools pass unit and integration tests.

## Phase 4: First-Release Readiness

Completed: Phase 4 validated the complete system across three materially different productions:
1. `projects/finetuning-explainer/` — Animated explainer (30s 16:9, Remotion React composition, 6 narrative beats, neural voiceover, contact sheets).
2. `projects/phase2-source-edit/` — Supplied-footage vertical source edit (9s 9:16, normalized FFmpeg cutting, audio mixing, technical gate review).
3. `projects/cinematic-documentary/` — Cinematic documentary montage (18.5s 16:9, NASA/Wikimedia public domain media acquisition, cinematic color grade, video stitch, voiceover, music ducking, loudness normalization, output review).

All productions passed output review with 0 external paid cost, durable project records, and clean technical gates.

Current state: Phases 0 through 4 are complete. The headless Claude-first Video Kit bundle is functional, self-contained, and ready for use.

## Phase 5: Deferred Expansion

Only after the Claude headless product works, consider other agentic CLIs;
compatibility beyond Markdown only where testing proves necessary; packaging and
installers; additional operating systems; web or desktop UI; Facet integration;
local GPU model execution; and any daemon, database, MCP bridge, workflow host,
or other host architecture.

## Test Strategy

- Go unit tests for shared mechanics.
- Mocked provider request/response tests.
- Upstream behavior-parity fixtures.
- Remotion, HyperFrames, and FFmpeg smoke tests.
- One Claude discovery and toolbox-use integration path.
- One real production as the Phase 2 integration gate.
- Real productions after meaningful capability batches and as first-release
  integration and quality tests.

Do not require live credentials for implementation status. Do not require one
acceptance film per pipeline. Do not block broad porting on perceptual review of
an unrelated output.

## Drift Gate

Before adding a subsystem, answer all four questions:

1. Which `DESIGN.md` section requires it?
2. Which named upstream capability or observed production defect motivates it?
3. Why are Claude's existing capabilities and the current toolbox insufficient?
4. Why is it needed in the current phase?

If the answers cite future UI, future CLIs, hypothetical scale, possible security
needs, or architectural elegance, defer the subsystem.
