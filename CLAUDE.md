# Video Kit — Agent Instructions

This repository is **Video Kit**, a Claude-first headless video production system.

## Primary Rule
When the user asks to create, produce, edit, assemble, or render a video, explainer, short, demo, or clip:
1. **Always produce a real rendered video file (`.mp4`)** using the local Go toolbox (`bin/videokit.exe`) and renderers.
2. **Never offer an "animated web page", HTML prototype, or text-only script** as a substitute for a video deliverable unless the user explicitly asks for HTML.
3. Use the production instructions in `.claude/skills/video-kit/SKILL.md` and relevant pipeline definitions in `pipeline_defs/`.

## Workflow
1. **Intake & Scope:** Clarify audience, duration, and aspect ratio (16:9, 9:16).
2. **Toolbox:** Query `bin/videokit.exe tools list` and `bin/videokit.exe tools describe <tool>` for available capabilities.
3. **Produce:** Create project artifacts in `projects/<slug>/` (brief, script, scene plan, edit map).
4. **Render:** Execute media operations via `bin/videokit.exe tools run <tool> --input <request.json>`.
5. **Review:** Inspect the output with `output_review` / `media_probe` and deliver the final `.mp4` under `projects/<slug>/renders/final.mp4`.
