---
name: facet
description: Create, edit, assemble, animate, and render reviewed videos with Facet from any supported agent CLI.
---

# Facet Video Producer

The user's selected agent (OpenCode, Codex, Copilot, Claude Code, or another supported engine) directs the production. Facet supplies stateless tools; project files preserve useful decisions and assets, not mandatory workflow stages.

## Normal Production Path
This core skill and the selected pack entry SKILL.md are authoritative for normal production and take precedence over deep legacy references. For simple tasks such as a supplied title card, start from the supplied intent and documented core tools: explain the plan, prepare the minimal request, estimate, render, and review. No source archaeology or persona/pipeline ceremony is required.
Consult deep legacy references (such as compose-director, runtime-selection guides, or Python APIs) only for a relevant specialized need or an actual error, never as a preflight requirement. Use targeted tool descriptions for uncertain contracts; do not scan sources or load every supporting skill before a normal render.

## Working Agreement
- Understand the outcome, supplied material, audience, format, and constraints. Inspect relevant files; clarify only consequential unknowns and rights/consent concerns.
- Briefly explain the plan, renderer, provider/model choices, and quality/time/cost tradeoffs. Ask for explicit consent before paid generation, publication, or material creative downgrades. Unknown cost is not free.
- Preserve the user's intent: silent videos need no narration or music; captions and scripts are optional. Work naturally without forced turn sequencing or unwanted narration.
- Prefer supplied/local assets when they meet the brief. Use source_edit for footage, the explainer pack for 2D motion, or another appropriate installed pack.
- Use `facet tools describe <tool>` when the contract is uncertain and `facet tools estimate <tool> --input request.json` before consequential calls. A render estimate reports `estimated_duration_seconds` and `exceeds_default_host_deadline`; when the latter is true the render will not finish inside a host's default 60s budget, so raise `deadline_ms` or invoke with `async:true`. Rendering costs roughly 7s of fixed startup plus ~80ms per 720p frame, so a 15s explainer takes about 43s. Estimates perform tool-specific checks, not deep renderer validation or proof of live availability or successful rendering.
- Execute and render here; do not hand the task off to a manual editor. `mock:true` is only for explicitly requested tests, never a production fallback for missing credentials or tools.

## Concise Requests
JSON files work across shells; these inline requests are also accepted. Replace example paths with existing project assets before probing or sampling.
```sh
facet tools run media_probe --input '{"input":"assets/source.mp4"}'
facet tools run frame_sample --input '{"input":"renders/final.mp4","output_dir":"artifacts/frames","strategy":{"type":"uniform","count":4}}'
facet tools run edge_tts --input '{"text":"Hello","output_path":"narration/voice.mp3"}'
facet tools estimate gflow_image --input '{"prompt":"Paper landscape","model":"narwhal","aspect_ratio":"landscape","count":1,"output_path":"assets/landscape.png"}'
facet tools estimate gflow_video --input '{"prompt":"Slow landscape pan","model":"veo-3.1","duration":6,"aspect_ratio":"landscape","resolution":"1080p","output_path":"assets/pan.mp4"}'
facet tools run output_review --input '{"rendered_file":"renders/final.mp4"}'
```
Narration is optional. `media_probe` accepts `input` or `input_path`, not `file_path`. `frame_sample` needs a strategy object: uniform/count, timestamps/timestamps array, or scenes/count with optional threshold. It does not accept video_path.

## Renderer Contract
`facet tools run video_compose --input artifacts/explainer_props.json` accepts direct Remotion props with nonempty cuts, or an operation envelope with edit_decisions. Direct cuts take precedence and select Remotion, not FFmpeg. See the explainer pack for a complete request.
For a narrated explainer end to end — script, narration, cuts matched to the audio, render, verification — follow `packs/explainer/NARRATED-WALKTHROUGH.md`, whose every step and timing was measured on a real run.
Cut fields are FLAT: `type`, `text`, `title` and the rest sit directly on the cut beside `in_seconds`/`out_seconds`. There is no nested `scene` object. The Explainer composition renders seventeen scene types (text_card, hero_title, section_title, callout, stat_card, stat_reveal, kpi_grid, progress_bar, comparison, bar_chart, pie_chart, line_chart, terminal_scene, screenshot_scene, parallax, anime_scene, provider_chip); `packs/explainer/SCENE-TYPES.md` lists the required field for each.
**A cut whose required field is missing or misspelled renders an EMPTY frame and the run still reports success.** A wrong `type`, or `txt` where `text` was meant, produces a video of blank scenes with no error. Always sample frames after rendering and confirm they differ; identical or unusually small frames mean the scenes did not render.
Default composition `Explainer` is 1920x1080 at 30 fps. Set top-level direct props `width`, `height`, `fps`, `duration_seconds` for an explicit profile, e.g. `"width":320,"height":180,"fps":24,"duration_seconds":3` renders 72 frames without padding. Dimensions must be positive even safe integers; fps/duration must be positive finite numbers and duration * fps a whole safe frame count. An omitted duration now renders exactly to the last cut's `out_seconds`, for both `cuts` and `scenes` plans — a 2-second plan renders 2.000s, not 3. An explicit `duration_seconds` still wins, so ask for one when you want a tail beyond the last cut. Empty cuts still fall back to 60 seconds in Remotion. Cuts must have finite 0 <= in_seconds < out_seconds, span at least one frame after boundary rounding, and fit within explicit duration; invalid timings are rejected, not truncated.
Use timeline in_seconds/out_seconds; use source_in_seconds to trim source media. Optional audio is `{"narration":{"src":"narration/voice.mp3","volume":1}}`; video_compose stages explicit media fields from the project directory, falling back to composer public/ for absent relative files. Explicit local absolute paths and file URLs are supported; unrelated files are not exposed. Omit audio for silence. Remotion needs the bundled composer, npm dependencies, Node, and Chromium; configure paths.remotion_composer or paths.bundle when needed. Estimates neither render nor run metadata validation; inspect the actual output profile.

## Provider And Delivery Contract
Only `gflow_image` and `gflow_video` are Facet tools, never generic `gflow`. The gflow CLI must be on PATH and authenticated; configured checks binary discovery only, not authentication. If installed in ~/go/bin, add that directory to PATH; there is no home-directory fallback.

gflow is optional and supports extension-free provider routes. Consult the installed gflow's help and returned diagnostics for its provider/session requirements; do not require an extension or run authentication setup preemptively. Status success does not prove generation access. Facet reports the provider's error details so the agent can help configure the specific missing requirement.
Real gflow estimates return estimated_cost:null and cost_known:false. Get paid consent before run, including unknown pricing. Images support models narwhal/harbor_seal/gem_pix_2, count 1-4; video uses veo-3.1 and duration 4/6/8/10. Use landscape/portrait/square aspect names (images also accept 4:3 and 3:4).
Real gflow results contain output plus outputs[] records with id, type, mime_type, output, source_file, sha256. Use output for the retained file; source_file is staging provenance, not a persistent asset path. Do not retry a failed generation blindly or claim estimates/mocks are real media.
Run output_review with expectations matching the brief, including optional audio for silence; inspect frames, pacing, legibility, and any audio. A gate reports `assumed` when you supplied no expectation for it — the file was compared against its own measured value, so that gate verified nothing. `profile`, `duration`, `video_codec` and `pixel_format` all behave this way; state what the brief asked for to turn them into real checks. Revise defects and deliver the verified file with provider/asset provenance, review outcome, and limitations.
