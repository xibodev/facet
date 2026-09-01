# Upstream Surface Census

## Scope

This is the complete shallow Phase 1 census of clean upstream OpenMontage at
commit `cd9f3c1f03368be87b140af494914b8ee4e3c7a4`. It records structure and
prioritization facts, not deep behavior for every tool. See `PIPELINE_MAP.md` for
the comprehensive pipeline mapping and `TOOL_PORT_MATRIX.md` for all tool
identities. Deep analysis is limited to the selected slice in `PHASE1_SLICE.md`.

The pin contains 2,115 files.

| Surface | Count | Treatment |
|---|---:|---|
| `.agents/skills` | 994 files; 90 directories; 89 `SKILL.md` | Select tool/provider knowledge progressively. |
| `skills` | 157 Markdown files | Curate shared and method knowledge; do not copy governance wholesale. |
| `tools` | 174 files | Shallow identities are in `TOOL_PORT_MATRIX.md`. |
| `tests` | 126 files; 109 `test_*.py` | Use focused behavior and parity evidence. |
| `remotion-composer` | 41 files | Existing composition runtime; introduce when a production requires it. |
| `schemas` | 34 files; 29 JSON schemas | Reuse or normalize only request-appropriate contracts. |
| `pipeline_defs` | 13 YAML files | All definitions inspected and mapped. |
| `styles` | 5 YAML files plus loader | Treatment knowledge orthogonal to pipelines. |
| `lib` | 20 files | Inspect incrementally with consuming capability families. |
| `ink-theater` | 30 files | Later animation/runtime candidate with separate notices. |
| `backlot` | 11 files | Exclude host and UI surface. |
| `.claude` | 432 files | Exclude other governing projections; redesign Claude surface. |

## Pipelines

The 13 definitions are `animated-explainer`, `animation`,
`avatar-spokesperson`, `character-animation`, `cinematic`, `clip-factory`,
`documentary-montage`, `framework-smoke`, `hybrid`, `localization-dub`,
`podcast-repurpose`, `screen-demo`, and `talking-head`.

## Production Knowledge

The 157 `skills/` files comprise 6 core, 36 creative, 11 meta, 103
pipeline-director files, and the root `INDEX.md`.

Core files are `color-grading.md`, `ffmpeg.md`, `hyperframes.md`, `remotion.md`,
`subtitle-sync.md`, and `whisperx.md`.

Creative files cover B-roll planning, cinematic treatment, enhancement, image
generation use, long- and short-form work, screen recording, sound design,
stock sourcing, storytelling, typography, editing, stitching, video generation
prompting, video understanding, 3D, drawing, animation, data visualization, Ink
Theater, talking-head generation, background removal, diagrams, face restoration,
provider use, lip sync, Manim, music, scene detection, and upscaling. Provider
prompting files cover Grok, Hunyuan, LTX, Seedance, Sora, and Veo.

Meta files are `animation-runtime-selector.md`, `bespoke-composition.md`,
`capability-extension.md`, `checkpoint-protocol.md`, `creative-intake.md`,
`onboarding.md`, `reviewer.md`, `skill-creator.md`, `taste-direction.md`,
`video-reference-analyst.md`, and `voice-performance-director.md`. Intake,
review, taste, reference, and voice guidance are useful; checkpoint, onboarding,
skill-creation, and capability-extension governance must not become host state.

Pipeline-director knowledge spans animation, avatar spokesperson, character
animation, cinematic, clip factory, documentary montage, explainer, hybrid,
localization, podcast repurpose, screen demo, and talking head. Its repeated
stage/director structure is evidence for consolidation, not a target hierarchy.

## Technology Packs

The `.agents/skills` tree groups runtime/composition packs (FFmpeg, Playwright,
synthetic capture, Remotion, and HyperFrames), production packs (media use,
video edit/understand/translate, motion graphics, character work, music-to-video,
and website-to-video), provider packs, and graphics technology packs. Claude
loads selected packs only after choosing a method and route.

HyperFrames material records provenance from
`https://github.com/heygen-com/hyperframes` at commit `3351fb1a`, tag `v0.7.17`.
It remains an independent runtime, not a Go rewrite target.

## Schemas

The 20 artifact schemas are action timeline, asset manifest, brief, character
design, character QA report, cost log, decision log, edit decisions, final
review, pose library, proposal packet, publish log, render report, research
brief, review, rig plan, scene plan, script, source media review, and video
analysis brief.

The 6 tool schemas cover Atlas 3D, Blender world, FAL 3D, Three.js asset catalog,
Three.js world, and video stitch. The remaining JSON schemas define pipeline
manifests, style playbooks, and checkpoints. Schemas constrain artifacts when
present and never require mandatory production stages.

## Styles

The five playbooks are `anime-ghibli`, `clean-professional`,
`flat-motion-graphics`, `minimalist-diagram`, and `premium-minimalist`.

## Runtimes And Tests

Remotion has a bounded composer, sample props, and focused staging, diagnostics,
audio, transition, and theme tests. HyperFrames has an adapter, style bridge,
progressively loadable skills, and a Remotion-conversion fixture corpus. FFmpeg
and ffprobe underpin source inspection, editing, audio, capture, enhancement,
and output review. Browser integration is limited to reproducible capture and
browser-rendered media.

Test files group as: tools 58, contracts 34, QA 7, library 6, Backlot 6, eval 8,
styles 2, fixtures 1, pipelines 1, and root 3. Backlot tests are excluded;
selected tool and runtime fixtures are enriched only as their capability family
approaches implementation.

## Exclusions

Backlot and host/UI behavior, local GPU tools and requirements, setup/install
and packaging machinery, checkpoint-owned workflow concepts, generic agent
orchestration, and other CLI projections are excluded or deferred by the
authority documents.
