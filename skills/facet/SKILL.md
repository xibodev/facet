---
name: facet
description: Create, produce, edit, assemble, animate, or render videos in Facet. Use whenever asked to make a video, edit footage, generate an explainer, create shorts/clips, add captions, mix audio, or do any video production task.
---

# Facet Video Producer

Use this skill when producing, editing, generating, or refining videos in a Facet workspace. Claude Code operates as the sole creative producer and orchestrator. Personas are creative reasoning postures, and the Go toolbox (`facet`) provides stateless execution mechanics.

---

## 1. Operating Architecture & Principles

- **Claude Code is the Active Orchestrator:** Claude drives intake, direction, storytelling, asset planning, provider selection, tool invocation, and multi-pass review.
- **Go Toolbox is Stateless:** The `facet` CLI exposes tool capabilities, runs validations, queries live provider status/costs, and executes deterministic operations. It does not own workflow state.
- **Project Files are Durable Records:** Productions live under `projects/<project-slug>/` or dedicated project directories. Artifacts (briefs, scripts, scene plans, edits, reviews) are ordinary JSON/Markdown files created as needed.
- **Modular Production Packs:** Specialized capabilities (Explainers, Cinematic Montages, Screen Demos) are supplied by installed Facet packs providing focused skills, styles, and pipeline definitions.
- **Honest Feasibility & Gated Actions:** Announce provider choices and costs before execution. Seek explicit approval before incurring paid API charges, external mutations, or irreversible creative downgrades.

---

## 2. Production Workflow Loop

```text
User Request / Assets
  │
  ├─► 1. Intake & Classification (intent, source of truth, deliverable, constraints)
  ├─► 2. Toolbox Discovery (`facet tools list` & `facet tools describe <tool>`)
  ├─► 3. Pipeline Selection (from core or active pack, e.g. explainer, cinematic, source-edit)
  ├─► 4. Route Formulation & Feasibility Check (local vs stock vs AI vs hybrid)
  ├─► 5. Progressive Skill Loading (core skills and pack skills)
  ├─► 6. Artifact Construction & Schema Validation (`schemas/`)
  ├─► 7. Tool Execution (`facet tools estimate` / `facet tools run`)
  ├─► 8. Technical & Visual QA (`output_review`, `visual_qa`, frame sampling)
  └─► 9. Editorial Revision & Delivery Manifest (`renders/final.mp4`)
```

---

## 3. Step-by-Step Production Guide

### Step 1: Intake & Creative Discovery

1. **Classify the Request Source:**
   - **Supplied Footage:** User provided raw videos, screen recordings, or talking head clips $\rightarrow$ Prioritize source-edit / assembly workflows.
   - **Animated Explainer / 2D Motion:** User wants concepts, diagrams, stats, charts $\rightarrow$ Use Explainer Pack (`@xibodev/facet-pack-explainer`).
   - **Cinematic / Archival Montage:** Historical, dramatic, narrative, public domain stills $\rightarrow$ Use Cinematic Pack (`@xibodev/facet-pack-cinematic`).
2. **Clarify Critical Parameters:**
   - Deliverable target (aspect ratio: 16:9, 9:16 vertical, 1:1 square; platform: YouTube, TikTok, Web).
   - Core message, target audience, tone, and pacing.
   - Inspect supplied assets immediately using `facet tools run media_probe --input <req.json>`.

### Step 2: Standard Tool Commands

Use the standard tools directly without running discovery scans:

```bash
# Voice synthesis via keyless neural Edge TTS
facet tools run edgetts --input artifacts/req_tts.json

# Probe audio duration to lock scene plan timestamps
facet tools run media_probe --input '{"file_path": "narration/beat1.mp3"}'

# Sample frames for inspection
facet tools run frame_sample --input '{"video_path": "assets/clip.mp4", "output_dir": "artifacts/frames", "interval_seconds": 2.0}'

# Assemble edit cuts
facet tools run edit --input artifacts/edit.json

# Mix audio tracks and duck background music
facet tools run audio_mix --input artifacts/audio_mix.json

# Review final output against quality gates
facet tools run output_review --input '{"rendered_file": "renders/final.mp4"}'
```

### Step 3: Formulate Feasible Routes & Announce Costs

- Propose a concrete route matching user constraints (e.g. Local FFmpeg + Remotion vs Edge TTS vs Cloud Video Gen).
- If an operation incurs cost or calls external APIs, provide an estimate:
  ```bash
  facet tools estimate <tool> --input <request.json>
  ```
- Clearly communicate the provider, model, estimated latency, and cost before execution.

### Step 4: Execute Tools via Go CLI

Execute tools with explicit JSON inputs conforming to the tool's schema:

```bash
facet tools run <tool> --input <request.json>
```

#### Standard Execution Protocol:
1. **Probe Source Media:**
   ```bash
   facet tools run media_probe --input '{"file_path": "assets/source.mp4"}'
   ```
2. **Sample Frames for Analysis:**
   ```bash
   facet tools run frame_sample --input '{"video_path": "assets/source.mp4", "output_dir": "artifacts/frames", "interval_seconds": 2.0}'
   ```
3. **Assemble Edit & Audio:**
   ```bash
   facet tools run edit --input artifacts/edit.json
   facet tools run audio_mix --input artifacts/audio_mix.json
   ```

### Step 5: Quality Assurance & Multi-Pass Review

Review is mandatory for every production:

1. **Automated & Technical QA:**
   Run `output_review` to inspect container, stream parameters, audio levels, black frames, silence, and freeze frames:
   ```bash
   facet tools run output_review --input '{"rendered_file": "renders/final.mp4", "sample_count": 8}'
   ```
2. **Visual & Creative Critique:**
   - Inspect sampled frames from `output_review`.
   - Evaluate pacing, hook retention, text legibility, audio balance (dialogue vs BGM), and brand alignment.
3. **Targeted Revision:**
   - Modify the edit plan, composition code, or assets.
   - Re-render and re-verify only the changed segments.

### Step 6: Final Delivery & Provenance

When the video passes review:
1. Place final deliverables in `renders/final.mp4`.
2. Generate a compact delivery summary with resolution, duration, fps, size, provenance of assets, tools used, and review verdict.
