---
name: facet
description: Create, produce, edit, assemble, animate, or render videos in Facet. Use whenever asked to make a video, edit footage, generate an explainer, create shorts/clips, add captions, mix audio, or do any video production task.
---

# Facet Video Producer

Use this skill when producing, editing, generating, or refining videos in the Facet workspace. Claude Code operates as the sole creative producer and orchestrator. Personas are creative reasoning postures, and the Go toolbox (`facet`) provides stateless execution mechanics.

---

## 1. Operating Architecture & Principles

- **Claude Code is the Active Orchestrator:** Claude drives intake, direction, storytelling, asset planning, provider selection, tool invocation, and multi-pass review.
- **Go Toolbox is Stateless:** The `videokit` CLI exposes tool capabilities, runs validations, queries live provider status/costs, and executes deterministic operations. It does not own workflow state.
- **Project Files are Durable Records:** Productions live under `projects/<project-slug>/`. Artifacts (briefs, scripts, scene plans, edits, reviews) are ordinary JSON/Markdown files created as needed.
- **No Swarm / No Background Daemons:** Personas and stage directors are reasoning perspectives, not autonomous subagents or separate processes.
- **Honest Feasibility & Gated Actions:** Announce provider choices and costs before execution. Seek explicit approval before incurring paid API charges, external mutations, or irreversible creative downgrades.

---

## 2. Production Workflow Loop

```text
User Request / Assets
  │
  ├─► 1. Intake & Classification (intent, source of truth, deliverable, constraints)
  ├─► 2. Toolbox Discovery (`videokit tools list` & `videokit tools describe <tool>`)
  ├─► 3. Pipeline & Persona Selection (`pipeline_defs/<pipeline>.yaml`)
  ├─► 4. Route Formulation & Feasibility Check (local vs stock vs AI vs hybrid)
  ├─► 5. Progressive Skill Loading (`skills/`, `.agents/skills/`, `styles/`)
  ├─► 6. Artifact Construction & Schema Validation (`schemas/`)
  ├─► 7. Tool Execution (`videokit tools estimate` / `videokit tools run`)
  ├─► 8. Technical & Visual QA (`output_review`, `visual_qa`, frame sampling)
  └─► 9. Editorial Revision & Delivery Manifest (`projects/<project>/renders/`)
```

---

## 3. Step-by-Step Production Guide

### Step 1: Intake & Creative Discovery

1. **Classify the Request Source:**
   - **Supplied Footage:** User provided raw videos, screen recordings, or talking head clips $\rightarrow$ Prioritize source-edit / assembly workflows (`skills/methods/source-edit.md`).
   - **Reference / Inspiration Video:** User provided a reference link or file $\rightarrow$ Read `skills/meta/video-reference-analyst.md` and decompose structure, pacing, and style before proposing concepts.
   - **Concept / Topic from Scratch:** Explainer, cinematic trailer, marketing video $\rightarrow$ Read `skills/meta/creative-intake.md` or `skills/meta/onboarding.md`.
2. **Clarify Critical Parameters:**
   - Deliverable target (aspect ratio: 16:9, 9:16 vertical, 1:1 square; platform: YouTube, TikTok, Web, Broadcast).
   - Core message, target audience, tone, and pacing.
   - Budget constraints (free/local only vs commercial cloud AI generation).
   - Inspect any supplied assets immediately using `videokit tools run media_probe --input <req.json>`.

### Step 2: Live Toolbox Discovery

Never guess tool availability or assume credentials exist. Probe the live toolbox:

```bash
# List all registered tools and their basic capability summaries
videokit tools list

# Describe a specific tool (inputs, schema, provider requirements, cost model)
videokit tools describe media_probe
videokit tools describe frame_sample
videokit tools describe source_edit
videokit tools describe audio_mix
videokit tools describe output_review
```

Verify implementation status, missing environment variables/credentials, and required binary dependencies (e.g., `ffmpeg`, `ffprobe`, `remotion`).

### Step 3: Choose Pipeline & Persona

1. **Select the Primary Pipeline Definition:**
   Consult `pipeline_defs/` for the matching production method:
   - `pipeline_defs/source-edit.yaml` or `skills/methods/source-edit.md` — Footage-led assembly, cutting, trimming, audio balancing.
   - `pipeline_defs/animated-explainer.yaml` — Conceptual animated explainers using Remotion, SVGs, charts, and voiceover.
   - `pipeline_defs/cinematic.yaml` — Narrative, montage-heavy, dramatic visuals with rich soundscapes.
   - `pipeline_defs/screen-demo.yaml` — Software walkthroughs, UI captures, cursor highlights, feature tours.
   - `pipeline_defs/talking-head.yaml` / `avatar-spokesperson.yaml` — Presenter-driven speech, interviews, webinars.
   - `pipeline_defs/clip-factory.yaml` / `podcast-repurpose.yaml` — Extracting viral shorts and vertical clips from long-form content.
   - `pipeline_defs/hybrid.yaml` / `documentary-montage.yaml` — Multi-source documentary narratives combining archive, voice, and graphics.
2. **Adopt the Appropriate Producer Persona:**
   - **Editor-Producer:** Precision pacing, source truth preservation, continuity, seamless transitions.
   - **Story Director:** Narrative arc, emotional beats, hook strength, script polish.
   - **Visual Stylist:** Brand consistency, typography, color grading, visual rhythm.

### Step 4: Formulate Feasible Routes & Announce Costs

- Propose a concrete route matching user constraints (e.g. Local FFmpeg + Remotion vs Cloud TTS + AI Video Gen).
- If an operation incurs cost or calls external APIs, provide an estimate:
  ```bash
  videokit tools estimate <tool> --input <request.json>
  ```
- Clearly communicate the provider, model, estimated latency, and cost before execution.

### Step 5: Load Progressive Craft Skills

Load only the knowledge needed for the active stage to preserve context:

1. **Stage Directors & Pipelines (`skills/pipelines/`):**
   - Read stage-specific instructions (e.g., `skills/pipelines/explainer/01-concept-director.md`, `skills/pipelines/cinematic/03-scene-director.md`).
2. **Core Technical Skills (`skills/core/`):**
   - `skills/core/ffmpeg.md` — Command patterns, filtering, encoding standards.
   - `skills/core/remotion.md` — React composition, props, sequences, audio syncing.
   - `skills/core/hyperframes.md` — Procedural web animation and motion graphics.
   - `skills/core/color-grading.md`, `skills/core/subtitle-sync.md`, `skills/core/whisperx.md`.
3. **Creative Guidance (`skills/creative/`):**
   - `skills/creative/storytelling.md`, `skills/creative/broll-planning.md`, `skills/creative/sound-design.md`, `skills/creative/video-editing.md`, `skills/creative/typography.md`.
4. **Technology Packs (`.agents/skills/`):**
   - AI generation & TTS: `elevenlabs`, `text-to-speech`, `kling-official`, `flux-best-practices`, `heygen`, `minimax-h3`.
   - Motion & Animation: `remotion-best-practices`, `gsap-core`, `threejs-animation`, `manim-composer`.
   - Video engineering: `video-edit`, `video-toolkit`, `video-understand`, `sound-effects`.
5. **Style Playbooks (`styles/`):**
   - Consult style rules from `styles/clean-professional.yaml`, `styles/premium-minimalist.yaml`, `styles/flat-motion-graphics.yaml`, `styles/anime-ghibli.yaml`, or `styles/minimalist-diagram.yaml`.

### Step 6: Validate Schemas & Create Artifacts

- Project artifacts reside in `projects/<project-slug>/artifacts/`.
- Validate data structure against `schemas/`:
  - `schemas/artifacts/` — `brief.json`, `script.json`, `scene_plan.json`, `asset_manifest.json`, `edit_plan.json`.
  - `schemas/tools/` — Tool-specific input/output payloads.
  - `schemas/styles/` — Style configurations.

### Step 7: Execute Tools via Go CLI

Execute tools with explicit JSON inputs conforming to the tool's schema:

```bash
videokit tools run <tool> --input <request.json>
```

#### Standard Execution Protocol:
1. **Probe Source Media:**
   ```json
   {
     "file_path": "projects/demo/assets/raw_clip.mp4"
   }
   ```
   $\rightarrow$ `videokit tools run media_probe --input req.json`
2. **Sample Frames for Analysis:**
   ```json
   {
     "video_path": "projects/demo/assets/raw_clip.mp4",
     "output_dir": "projects/demo/artifacts/frames",
     "interval_seconds": 2.0
   }
   ```
   $\rightarrow$ `videokit tools run frame_sample --input req.json`
3. **Assemble Edit & Audio:**
   ```json
   {
     "edit_plan_path": "projects/demo/artifacts/edit_plan.json",
     "output_path": "projects/demo/renders/v1.mp4"
   }
   ```
   $\rightarrow$ `videokit tools run source_edit --input req.json` / `audio_mix`

### Step 8: Quality Assurance & Multi-Pass Review

Review is mandatory for every production:

1. **Automated & Technical QA:**
   Run `output_review` to inspect container, stream parameters, audio levels, black frames, silence, and freeze frames:
   ```json
   {
     "video_path": "projects/demo/renders/v1.mp4",
     "sample_count": 8,
     "visual_qa": true
   }
   ```
   $\rightarrow$ `videokit tools run output_review --input req.json`
2. **Visual & Creative Critique:**
   - Inspect sampled frames from `output_review` / `visual_qa`.
   - Read `skills/meta/reviewer.md` and `skills/shared/review-editing.md`.
   - Evaluate pacing, hook retention, text legibility, audio balance (dialogue vs BGM), and brand alignment.
3. **Targeted Revision:**
   - Diagnose root causes of any defects.
   - Modify the edit plan, composition code, or assets.
   - Re-render and re-verify only the changed segments.

### Step 9: Final Delivery & Provenance

When the video passes review:
1. Place final deliverables in `projects/<project-slug>/renders/final.mp4`.
2. Generate a compact delivery summary:
   - Output path and technical specs (resolution, duration, fps, size).
   - Provenance of all assets (supplied footage, stock, generated audio/video).
   - Executed route, tools, and total actual cost.
   - Review verdict and any known trade-offs or limitations.
