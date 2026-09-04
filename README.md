# Facet

> Autonomous Video Production Engine & Studio for Agentic CLIs (Claude Code, OpenCode, OpenAI Codex, GitHub Copilot).

[![npm version](https://img.shields.io/npm/v/@xibodev/facet.svg?color=blue)](https://www.npmjs.com/package/@xibodev/facet)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](LICENSE)

Facet turns conversational requests in your terminal or browser into fully produced, rendered `.mp4` videos. It combines a stateless Go media engine, pre-loaded production guidance for AI coding agents, and an optional reactive web studio.

---

## ⚡ Quick Start

### Option 1: NPX (Zero Install)

Run instantly on Windows, macOS, or Linux:

```bash
# Verify system dependencies and tools
npx @xibodev/facet doctor

# Create a project and launch your agent
npx @xibodev/facet init my-video --engine claude

# Or launch the visual studio
npx @xibodev/facet ui
```

### Option 2: Global NPM Install

```bash
npm install -g @xibodev/facet
```

### Option 3: Direct Script Install

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/xibodev/facet/main/install.ps1 | iex
```

**macOS / Linux (Bash):**
```bash
curl -fsSL https://raw.githubusercontent.com/xibodev/facet/main/install.sh | sh
```

---

## 🎬 How It Works

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                             FACET STUDIO (UI)                               │
├──────────────────────────────────────┬──────────────────────────────────────┤
│          LEFT: AGENT STREAM          │        RIGHT: ARTIFACT CANVAS        │
│                                      │                                      │
│  • Spawns your CLI (Claude/OpenCode) │  • 6-Stage Reactive Progress Stepper │
│  • Bidirectional stream-json SSE     │  • Script & Narrative Beat Table     │
│  • Tool execution pills & status     │  • Synthesized Audio Beat Players    │
│  • Structured question choices       │  • Visual QA Contact Sheet Gallery   │
│  • Zero server context state         │  • Master Video Player (1080p, H264) │
└──────────────────▲───────────────────┴───────────────────▲──────────────────┘
                   │ (Pipes stdin/stdout)                  │ (Watches projects/)
┌──────────────────┴───────────────────────────────────────┴──────────────────┐
│                             FACET ENGINE (Go)                               │
│  • 33 Media & Analysis Tools (`facet tools list/describe/run`)              │
│  • Keyless Neural Speech (Edge-TTS built-in)                                │
│  • FFmpeg Trimming, Concat, Normalization & Loudness Mastering              │
│  • Remotion React Motion Graphics & Captions Composer                       │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 🛠️ CLI Commands

```bash
# Inspect system runtimes, CLIs, and 33 media tools
facet doctor

# Scaffold a project workspace and launch the agent CLI
facet init <project-slug> [--engine claude|opencode|codex|copilot] [--no-launch]

# Launch the visual web studio (auto-opens browser)
facet ui [--port 8787] [--dir .] [--no-open]

# Run toolbox operations directly from scripts or agent harnesses
facet tools list
facet tools describe <tool-name>
facet tools estimate <tool-name> --input request.json
facet tools run <tool-name> --input request.json
```

---

## 📦 Toolbox Capabilities (33 Tools)

Facet ships with 33 stateless media tools implemented in pure Go:

| Category | Tools | Capabilities |
| :--- | :--- | :--- |
| **Voice & Speech** | `edge_tts`, `openai_tts`, `elevenlabs_tts`, `piper_tts` | Keyless Microsoft Edge neural TTS, ElevenLabs, OpenAI voice, local Piper. |
| **Video Editing** | `source_edit`, `video_trimmer`, `video_stitch`, `video_compose`, `silence_cutter` | Ordered trims, 16:9 / 9:16 normalization, transitions, silence detection, Remotion React rendering. |
| **Audio Mastering** | `audio_mix`, `audio_mixer`, `music_library`, `audio_probe` | Multi-track speech/music mixing, sidechain ducking, EBU R128 loudness normalization. |
| **Visual QA** | `output_review`, `visual_qa`, `frame_sample`, `frame_sampler`, `scene_detect`, `media_probe` | Container inspection, 5-gate technical verification, keyframe extraction, scene cut detection. |
| **Open & Stock Media** | `direct_clip_search`, `wikimedia`, `pexels_video`, `pixabay_video` | Public domain & CC media acquisition with download and probing. |
| **AI Generation** | `openai_image`, `flux_image`, `kling_video`, `sora_video`, `color_grade` | Cloud generation APIs with mock contracts and LUT color grading. |
| **Selectors** | `image_selector`, `video_selector` | Explainable candidate ranking and cost estimation. |

---

## 🚀 Modular Production Packs

Facet organizes production capabilities into modular content packs under `packs/` that provide focused skills, styles, and pipeline definitions:

| Pack | Package | Pipelines Included | Capabilities |
| :--- | :--- | :--- | :--- |
| **Explainer** | `@xibodev/facet-pack-explainer` | `animated-explainer` | 2D motion graphics, charts, text cards, Remotion runtime |
| **Cinematic** | `@xibodev/facet-pack-cinematic` | `cinematic`, `documentary-montage` | Archival montage, Wikimedia public domain sourcing, color grading |
| **Screen Demo** | `@xibodev/facet-pack-screendemo` | `screen-demo` | Software walkthroughs, synthetic terminal screen recording |
| **Talking Head** | `@xibodev/facet-pack-talkinghead` | `talking-head`, `avatar-spokesperson` | AI presenter videos, lip-sync, teleprompter scripts |
| **Social** | `@xibodev/facet-pack-social` | `clip-factory`, `podcast-repurpose` | 9:16 vertical shorts, viral hooks, animated captions |
| **Animation** | `@xibodev/facet-pack-animation` | `character-animation`, `animation` | 2D character rigging, pose libraries, action timelines |
| **Localization** | `@xibodev/facet-pack-localization` | `localization-dub` | Multilingual dubbing, subtitle translation, voice cloning |

---

## 📄 License

GNU Affero General Public License, version 3 ([AGPL-3.0-or-later](LICENSE)).
