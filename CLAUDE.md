# Facet — Agent Instructions

This repository is **Facet**, an autonomous video production system and agentic toolbox.

## Primary Directives
1. **Always produce a real rendered video file (`.mp4`)** using the local Go toolbox (`bin/facet.exe` or `facet`) and renderers.
2. **Never suggest external manual apps** (CapCut, Canva, Premiere, After Effects, DaVinci, or hiring freelancers). You produce and render the video here.
3. **Immediate Action on Turn 1:** Do NOT run file tree discovery (`ls`, `find`), do NOT read empty `brief.md` files, and do NOT run `facet tools list`. Draft `artifacts/script.json` and generate voiceover on Turn 1.

## Standard Tool Commands
- **Voiceover (Edge TTS):** `facet tools run edgetts --input artifacts/req_tts.json`
- **Probe Audio Duration:** `facet tools run media_probe --input '{"file_path": "narration/beat1.mp3"}'`
- **Remotion/Explainer Render:** `facet tools run video_compose --input artifacts/explainer_props.json`
- **Edit/Assembly:** `facet tools run edit --input artifacts/edit.json`
- **Quality Review:** `facet tools run output_review --input '{"rendered_file": "renders/final.mp4"}'`

## Execution Flow
1. **Turn 1 (Script & Narration):** Write `artifacts/script.json` & call `facet tools run edgetts --input artifacts/req_tts.json`.
2. **Turn 2 (Scene Plan & Composition):** Probe narration durations & generate `artifacts/scene_plan.json` / `artifacts/explainer_props.json`.
3. **Turn 3 (Render & QA):** Render to `renders/final.mp4` & review with `facet tools run output_review --input '{"rendered_file": "renders/final.mp4"}'`.
