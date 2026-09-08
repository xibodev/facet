---
name: explainer
description: Produce reviewed 2D explainers and motion graphics with Facet and the bundled Remotion renderer.
---

# Explainer Production Pack

This pack entry and the core Facet skill are authoritative for normal production and take precedence over deep legacy references. The minimal normal path for a simple title card is supplied intent, a brief plan, the direct request below, estimate, render, and review; no separate script or narration is required.
Supporting legacy skills under this pack (including compose-director, runtime-selection guides, and Python API workflows) are references only for a relevant specialized need or an actual error, never a preflight requirement. They do not impose source archaeology or persona/pipeline ceremony on normal production.

Understand the thesis, audience, duration, format, and visual direction. Explain the plan and provider choices; ask for explicit consent before paid generation. Narration, music, and captions are optional: honor silent-video requests. No forced turn sequencing.
Choose clean-professional, flat-motion-graphics, or minimalist-diagram as a theme identifier (no .yaml suffix). Plan meaningful visuals rather than filling every scene with a text card.

## Narrated Production
For a video that speaks, follow `NARRATED-WALKTHROUGH.md`: narration first because it sets the timing, then cuts matched to it, then render and verify. Scene types and their required fields are in `SCENE-TYPES.md`.

## Direct Renderer Request
Write `artifacts/explainer_props.json` with a nonempty cuts array, then estimate and run video_compose. This minimal example is silent:
```json
{"composition_id":"Explainer","theme":"clean-professional","width":1920,"height":1080,"fps":30,"duration_seconds":4,"cuts":[{"id":"intro","type":"text_card","source":"","in_seconds":0,"out_seconds":4,"text":"A clearer explanation"}],"output":"renders/final.mp4"}
```
```sh
facet tools estimate video_compose --input artifacts/explainer_props.json
facet tools run video_compose --input artifacts/explainer_props.json
```
Direct cuts take precedence over operation and select Remotion. Use in_seconds/out_seconds for timeline placement and source_in_seconds for source video trim. Set top-level width/height/fps/duration_seconds explicitly; the example is exactly four seconds (120 frames), without padding. A 320x180, 24 fps, three-second profile uses `"width":320,"height":180,"fps":24,"duration_seconds":3` and cuts ending by three seconds. Dimensions must be positive even safe integers, fps/duration positive finite numbers, and duration * fps a whole safe frame count. Cuts need finite 0 <= in_seconds < out_seconds, at least one frame after boundary rounding, and must fit explicit duration; invalid timings fail instead of truncating. Omitted width/height/fps default to 1920/1080/30; only omitted duration adds one second after the last cut (60 seconds for empty cuts in Remotion).
An alternative operation envelope uses `{"operation":"compose","edit_decisions":{"render_runtime":"remotion","cuts":[...]},"output_path":"renders/final.mp4"}`. Direct scene plans are converted by renaming start_seconds/end_seconds to in_seconds/out_seconds and carrying every other field through, so the scene-type fields in SCENE-TYPES.md work in a scenes array too. `description` becomes `text` only when no explicit `text` is given. Prefer `cuts` when authoring precisely; the conversion no longer drops fields.
Remotion requires the composer source, npm dependencies, Node, and Chromium; configure paths.remotion_composer or paths.bundle if discovery fails. Estimates do not run metadata validation, render, or verify installed runtime dependencies. Do not silently downgrade to an FFmpeg slideshow.

## Optional Audio And Assets
```sh
facet tools run edge_tts --input '{"text":"A clearer explanation.","output_path":"narration/voice.mp3"}'
facet tools run media_probe --input '{"input":"narration/voice.mp3"}'
```
If narration is requested, probe it and time cuts to the real duration. Add `"audio":{"narration":{"src":"narration/voice.mp3","volume":1}}` to direct props; music uses audio.music.src and volume. Omit audio for silence. video_compose stages explicit media fields from the project directory, with composer public/ as fallback for absent relative files; local absolute paths and file URLs are also supported. It does not expose the entire project or public tree.
For generated assets use `gflow_image` or `gflow_video`, never generic gflow. Check the core skill for exact requests, PATH/authentication requirements, null/unknown real estimates, and outputs[] provenance. Obtain paid consent first. `mock:true` is only for explicitly requested tests, never production.

## Review And Deliver
```sh
facet tools run frame_sample --input '{"input":"renders/final.mp4","output_dir":"artifacts/frames","strategy":{"type":"uniform","count":4}}'
facet tools run output_review --input '{"rendered_file":"renders/final.mp4"}'
```
media_probe also accepts input_path, not file_path. Configure review expectations to match resolution, timing, and intentional silence; inspect sampled frames for clipping, hierarchy, legibility, continuity, and any audio sync. Deliver the verified file with provenance and remaining limitations, not just a successful tool exit.
