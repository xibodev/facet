# Facet Producer

Use `skills/facet/SKILL.md` as the core production contract from any supported agent CLI. The user's selected agent orchestrates; Facet is a stateless toolbox, not a Claude-only workflow.

## Produce
1. Understand the brief and supplied assets. Clarify only consequential questions, including rights or consent when unclear.
2. Explain the plan, renderer, provider/model choices, and material tradeoffs in ordinary conversation. Ask for explicit consent before paid generation or publication; unknown cost is not free.
3. Choose supplied-footage editing, the explainer pack, or another installed pack to fit the request. Preserve silent-video intent: narration, music, and captions are optional. No forced turn sequencing or mandatory artifact stages.
4. Read `facet tools describe <tool>` when needed. Estimate with the real request before consequential work; an estimate is not a credential check or render proof.
5. Execute locally through `facet tools run <tool> --input request.json`. `mock:true` is only for explicitly requested tests, never a production fallback.
6. Review technical output and sampled frames, revise meaningful defects, and deliver the inspected video with concise provenance and limitations.

## Requests And Rendering
```sh
facet tools run media_probe --input '{"input":"assets/source.mp4"}'
facet tools run frame_sample --input '{"input":"renders/final.mp4","output_dir":"artifacts/frames","strategy":{"type":"uniform","count":4}}'
facet tools run output_review --input '{"rendered_file":"renders/final.mp4"}'
```
`media_probe` accepts input or input_path, not file_path; frame_sample uses a strategy object, not video_path/interval_seconds shorthand.
For motion graphics use `facet tools run video_compose --input artifacts/explainer_props.json`; the explainer pack contains complete direct props. Nonempty cuts select Remotion; default Explainer is 1920x1080/30fps with one second after the last cut. Estimates do not render or prove runtime availability.
Use `gflow_image` or `gflow_video`, never generic gflow. The binary must be on PATH and authenticated; configured checks only the binary. Real estimates have null estimated_cost (unknown), not free generation. Real outputs[] include id/type/mime_type/output/source_file/sha256; retain output paths, not staging source_file paths.
Do not substitute manual-editor instructions, mock outputs, hidden provider choices, or unverified success for the requested video.
