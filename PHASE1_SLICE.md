# FINALIZED MINIMUM PHASE 2 CONTRACT

## Decision

Select a **supplied-footage source edit** as the first real-video vertical
slice. Claude inspects user-supplied media, chooses moments and edit structure,
authors ordinary project records, invokes a stateless Go toolbox, composes with
FFmpeg, and reviews the rendered output.

This is preferred to a topic explainer for the first slice because it:

- requires no paid credentials, generated media, stock acquisition, or speech
  provider;
- produces a real video through a renderer already expected by the architecture;
- exercises genuine editorial decisions rather than reducing the slice to a
  synthetic contract test;
- tests discovery, source inspection, timeline decisions, composition, audio,
  and review with a short reproducible dependency path;
- establishes reusable media mechanics needed by later source edit, content
  repurpose, documentary, localization, and cinematic work;
- avoids making early visual-acquisition, TTS, or Remotion assumptions merely
  to complete an explainer.

A topic explainer remains valuable after these media foundations exist. It has a
wider dependency fan-out: research, script, narration, visual asset creation or
acquisition, timing, and a composition runtime.

## Exact Public Tool Set

The Phase 2 slice exposes exactly five public tools:

| Tool | Mechanical responsibility | Minimum result |
|---|---|---|
| `media_probe` | Inspect source and output streams and container facts. | Duration, dimensions, frame rate, codecs, audio streams, rotation, and probe warnings. |
| `frame_sample` | Extract deterministic review frames by explicit timestamp, uniform interval, or scene-derived timestamp. | Sample paths, requested strategy, resolved timestamps, dimensions, and extraction warnings. |
| `source_edit` | Validate and execute an ordered cut map over supplied media, including trim, sequence, per-segment crop placement, and crop/scale/pad. Phase 2 transitions are cut-only. | Output path, realized segments, duration, normalized media facts, and requested-versus-realized operation summary. |
| `audio_mix` | Apply deterministic source-audio gain and fades, optional supplied music mixing and ducking, and output normalization. | Output path, stream facts, requested versus realized operations, and measured loudness when available. |
| `output_review` | Run deterministic technical acceptance checks and produce evidence for Claude's judgment. | Execution status, separate review status, gate results, sampled evidence, failures/warnings, and output facts. |

No public dependency, registry, process, FFmpeg, schema, or validation helper is
added. The five tools do not select moments, decide pacing, choose a story,
approve an output, or progress a workflow. Claude owns those decisions.

## Composition Route

FFmpeg is the sole composition route for this slice. `media_probe` uses ffprobe;
`frame_sample`, `source_edit`, and `audio_mix` invoke FFmpeg; `output_review`
coordinates deterministic probes and samples without becoming a creative
reviewer. Intermediate files are ordinary project artifacts, and the final
render is written under the project's render directory.

The minimum production record is a brief, source-review facts, an edit map, the
render, review evidence, and a delivery record. No hidden stage or session state
is introduced.

## Required Knowledge

- **Shared:** conversational intake, consequential-choice handling, editorial
  direction, file and material provenance, output profiles, sound fundamentals,
  technical versus judgment review, and delivery records.
- **Method:** source inspection, selects and moment rationale, continuity,
  cut-map authoring, pacing, aspect-ratio adaptation, source-audio decisions,
  edit revision, and supplied-footage rights/consent prompts.
- **Route:** talking-head and hybrid guidance is source-edit route knowledge;
  video-editing and sound guidance supplies cut, continuity, mix, ducking,
  loudness, and review practice without becoming a workflow controller.
- **Tool/runtime:** ffprobe JSON interpretation; FFmpeg trim/concat,
  timestamp/time-base handling, filter graphs, crop/scale/pad, encoding, audio
  gain/fades/mixing/loudness, deterministic frame extraction, and failure
  diagnosis.

## Completed Deep Inventory

The selected slice and its immediate dependencies were deeply inventoried from
clean OpenMontage at `cd9f3c1f03368be87b140af494914b8ee4e3c7a4`. These are
behavioral evidence paths, not copied Video Kit code:

| Concern | Clean donor evidence | Phase 2 treatment |
|---|---|---|
| Base execution and dependency facts | `tools/base_tool.py`; `tools/tool_registry.py` | Re-express once as a small Go contract and registry. Dependencies are reported facts, not extra tools. |
| Probe and frame evidence | `tools/analysis/audio_probe.py`; `tools/analysis/frame_sampler.py` | Fold into `media_probe` and `frame_sample`. |
| Edit and composition mechanics | `tools/video/video_compose.py`; `tools/video/video_trimmer.py`; `tools/video/video_stitch.py` | Fold into `source_edit`; retain one FFmpeg route. |
| Audio mechanics | `tools/audio/audio_mixer.py` | Fold into `audio_mix`. |
| Technical review | `tools/analysis/visual_qa.py`; `tools/analysis/composition_validator.py` | Fold deterministic evidence and gates into `output_review`; Claude retains judgment. |
| Selected artifact contracts | `schemas/` | Normalize only source/edit, output-profile, review, and delivery fields used by this slice. Do not import host, stage, or session state. |
| Existing composer and tests | `remotion-composer/`; `remotion-composer/tests/` | Evidence inventoried, but Remotion execution and parity are explicitly deferred from this FFmpeg slice. |
| Source-edit production knowledge | `pipeline_defs/talking-head.yaml`; `pipeline_defs/hybrid.yaml`; `.agents/skills/video-editing/SKILL.md`; `.agents/skills/sound/SKILL.md` | Curate talking-head and hybrid as source-edit routes and progressively load editing and sound knowledge. Do not copy governing Markdown wholesale. |

Verified behavior to preserve or compatibly normalize:

- Probe execution consumes ffprobe JSON and normalizes container and stream
  facts while preserving actionable bounded stderr on failure.
- Frame sampling is deterministic for explicit timestamps, uniform sampling,
  and scene-derived sampling; results report the strategy and resolved times.
- `source_edit` normalizes each segment to one requested output target, frame
  rate, `yuv420p`, AAC at 48 kHz stereo, and generated silent audio where an
  input has no audio, then concatenates compatible normalized segments.
- Audio filters execute in declared order: per-input trim/gain/fades, optional
  sidechain ducking, mix, then final loudness normalization. `loudnorm` facts
  and realized ducking/fade operations are reported rather than implied.
- Output review reports command/execution success separately from review
  `pass`, `warn`, or `fail`. A technically completed command cannot turn failed
  gates into execution failure, and execution success cannot imply acceptance.
- Generic aggregate requests are rejected. Every operation names concrete
  inputs, timeline segments or audio tracks, target profile, output, and checks.

Shared immediate mechanics are executable discovery, process invocation,
timeouts, path validation, structured JSON, bounded stderr, temporary-output
cleanup, output-conflict protection, and atomic finalization where supported.
They remain internal library mechanics.

## Finalized Request Schemas

All paths are project-local or explicitly supplied filesystem paths. Unknown
fields are rejected. Times and durations are finite, non-negative seconds;
end times must be greater than start times.

### `media_probe`

```json
{
  "input": "projects/demo/assets/source.mp4",
  "timeout_seconds": 30
}
```

Required: `input`. Optional: positive `timeout_seconds`. The result contains raw
identity facts needed for traceability plus normalized format, video, and audio
facts derived from ffprobe JSON.

### `frame_sample`

```json
{
  "input": "projects/demo/assets/source.mp4",
  "output_dir": "projects/demo/review/source-frames",
  "strategy": {"type": "timestamps", "timestamps": [0.5, 3.0, 7.25]},
  "image_format": "jpg",
  "overwrite": false,
  "timeout_seconds": 60
}
```

Required: `input`, `output_dir`, and exactly one strategy: `timestamps` with a
non-empty timestamp list, `uniform` with a positive `count`, or `scenes` with a
positive `count` and optional deterministic threshold. Optional image format is
`jpg` or `png`. Requests outside probed duration are rejected.

### `source_edit`

```json
{
  "segments": [
    {"input": "projects/demo/assets/a.mp4", "start": 1.2, "end": 5.7, "position": "top"},
    {"input": "projects/demo/assets/b.mp4", "start": 0.0, "end": 3.5, "focal_point": {"x": 0.65, "y": 0.4}, "transition": "cut"}
  ],
  "target": {
    "width": 1080,
    "height": 1920,
    "fps": 30,
    "fit": "cover",
    "video_codec": "h264",
    "pixel_format": "yuv420p",
    "audio_codec": "aac",
    "audio_sample_rate": 48000,
    "audio_channels": 2
  },
  "output": "projects/demo/renders/edit.mp4",
  "overwrite": false,
  "timeout_seconds": 300
}
```

Required: non-empty ordered `segments`, fixed `target`, and `output`. A segment
may additionally declare either a named `position` (`center`, cardinal edge, or
corner) or a normalized `focal_point` with `x`/`y` from 0 to 1. These fields
place a `cover` crop; they are mutually exclusive. The only accepted transition
is `cut`: dissolves and other transition effects are outside the selected Phase
2 slice. Phase 2 accepts only the normalized H.264/`yuv420p`/AAC 48 kHz stereo
target; missing input audio is replaced by duration-matched silence.

### `audio_mix`

```json
{
  "video": "projects/demo/renders/edit.mp4",
  "source": {"gain_db": -2, "fade_in": 0.2, "fade_out": 0.4},
  "music": {
    "input": "projects/demo/assets/music.wav",
    "gain_db": -16,
    "fade_in": 1.0,
    "fade_out": 2.0,
    "ducking": {"enabled": true, "threshold": 0.05, "ratio": 8}
  },
  "loudness": {"enabled": true, "integrated_lufs": -14, "true_peak_db": -1},
  "duration": "video",
  "output": "projects/demo/renders/final.mp4",
  "overwrite": false,
  "timeout_seconds": 300
}
```

Required: `video`, `duration: "video"`, and `output`. `source`, `music`, and
`loudness` are optional concrete operations, but at least one enabled or non-zero
operation must be present. Combined fade durations must fit the video. Enabled
ducking requires source audio, music audio, threshold in `(0, 1]`, and ratio in
`(1, 20]`. Enabled loudness targets are limited to integrated LUFS `[-70, -5]`
and true peak dB `[-9, 0]`.

### `output_review`

```json
{
  "input": "projects/demo/renders/final.mp4",
  "profile": {"width": 1080, "height": 1920, "fps": 30},
  "checks": {
    "duration": {"expected": 9.0, "tolerance": 0.1},
    "video_codec": "h264",
    "pixel_format": "yuv420p",
    "audio": {"required": true, "codec": "aac", "sample_rate": 48000, "channels": 2}
  },
  "samples": {"type": "uniform", "count": 5},
  "evidence_dir": "projects/demo/review/final",
  "timeout_seconds": 90
}
```

Required: `input`, concrete `profile`, concrete `checks`, sample strategy, and
`evidence_dir`. Empty or generic check aggregates are invalid. The result keeps
`execution_status` separate from `review_status` and includes each gate and
sample as inspectable evidence.

## Execution Envelope

`list`, `describe`, `estimate`, and `run` use the architecture's finalized
minimum invocation shape:

```text
videokit tools list
videokit tools describe <tool>
videokit tools estimate <tool> --input <request.json>
videokit tools run <tool> --input <request.json>
```

`list` returns only the five public tools and honest dependency/configuration
facts. `describe` returns identity, capability, dependencies, structured request
and result schemas, cost, network, and side-effect facts. `estimate` validates
request shape, cross-field rules, ranges, timelines, paths, and basic eligibility
without rendering or creating outputs. Because estimate is side-effect free, it
does not decode media: `media_probe` estimate checks file existence and extension,
and tools defer stream presence, decoded duration bounds, and codec readability
to `run`. `run` performs one requested operation.

Success envelope:

```json
{
  "ok": true,
  "tool": "media_probe",
  "operation": "run",
  "result": {},
  "warnings": [],
  "execution": {
    "provider": "local",
    "network": false,
    "external_write": false,
    "estimated_cost": 0,
    "actual_cost": 0
  }
}
```

Error envelope:

```json
{
  "ok": false,
  "tool": "media_probe",
  "operation": "run",
  "error": {
    "code": "dependency_missing",
    "message": "ffprobe is not available",
    "retryable": false,
    "details": {}
  },
  "warnings": [],
  "execution": {
    "provider": "local",
    "network": false,
    "external_write": false,
    "estimated_cost": 0,
    "actual_cost": 0
  }
}
```

The stable errors are `invalid_request`, `dependency_missing`,
`input_not_found`, `input_probe_failed`, `timestamp_out_of_bounds`,
`invalid_timeline`, `output_conflict`, `command_failed`, `command_timeout`,
`output_missing`, `output_validation_failed`, and `partial_result`. Errors use
the most specific code, preserve bounded actionable stderr in `details`, and do
not leak credentials or invent workflow state. Exit status is zero only for a
success envelope. `partial_result` is an error envelope with inspectable
completed artifacts and failures in `details`; it never silently reports full
success.

`frame_sample` extracts and validates every requested frame in an invocation
temporary directory, then publishes the set. Extraction or publication failure
restores any overwritten set and leaves no newly published partial set.
`output_review` converts a sampling failure after completed probe/gates into
`partial_result` with `execution_status: "partial"`, `review_status: "revise"`,
completed gate/output facts, and the sampling failure.

This slice is local, non-networked, non-paid, and performs no external writes,
so its execution facts are always provider `local`, network `false`,
external-write `false`, and estimated/actual provider cost `0`.

## Representative Results

Representative `media_probe` success result:

```json
{
  "format": {"duration": 12.48, "format_name": "mov,mp4,m4a,3gp,3g2,mj2"},
  "video": {"width": 1920, "height": 1080, "fps": 30, "codec": "h264", "pixel_format": "yuv420p", "rotation": 0},
  "audio": [{"codec": "aac", "sample_rate": 48000, "channels": 2}],
  "warnings": []
}
```

Representative `source_edit` and `audio_mix` result facts:

```json
{
  "output": "projects/demo/renders/final.mp4",
  "duration": 9.2,
  "realized_segments": 2,
  "video": {"width": 1080, "height": 1920, "fps": 30, "codec": "h264", "pixel_format": "yuv420p"},
  "audio": {"codec": "aac", "sample_rate": 48000, "channels": 2, "silent_inputs_filled": 1},
  "operations": ["trim", "scale_crop", "concat", "gain", "fade", "duck", "mix", "loudnorm"]
}
```

Representative review result:

```json
{
  "execution_status": "succeeded",
  "review_status": "warn",
  "gates": [
    {"name": "profile", "status": "pass"},
    {"name": "duration", "status": "pass"},
    {"name": "audio", "status": "pass"},
    {"name": "sample_extraction", "status": "warn", "message": "one duplicate scene timestamp was removed"}
  ],
  "samples": [
    {"path": "projects/demo/review/final/frame-0001.jpg", "timestamp": 0.5}
  ]
}
```

## Parity Fixtures

Focused parity fixtures are derived from selected clean-upstream tests and
remain behavioral evidence under the pinned commit:

- base-tool dependency reporting: selected tests for `tools/base_tool.py` and
  `tools/tool_registry.py`;
- vertical composition: selected `tools/video/video_compose.py` tests covering
  portrait target normalization and output validation;
- mixer behavior: selected `tools/audio/audio_mixer.py` tests covering fades,
  sidechain ducking, `loudnorm`, and video-bounded duration;
- frame behavior: selected `tools/analysis/frame_sampler.py` tests covering
  explicit, uniform, and deterministic frame extraction;
- scene behavior: selected scene-sampling tests associated with
  `tools/analysis/frame_sampler.py` and `tools/analysis/scene_detect.py`;
- review behavior: selected `tools/analysis/visual_qa.py` and
  `tools/analysis/composition_validator.py` tests covering evidence and gate
  status independently of command success;
- Remotion composer and its tests under `remotion-composer/` are inventoried but
  explicitly deferred; no Remotion parity fixture gates this FFmpeg slice.

Phase 2 adds compact Go fixtures for these cases plus missing-audio synthesis,
timestamp bounds, invalid timelines, existing-output conflicts, command timeout,
missing output, and failed final validation. The integration gate remains one
real, reproducible, reviewed non-paid video through Claude Code.

## Dirty Donor Candidate Status

The current `video-agent-bundle` remains a selective salvage donor, not an
authority. The read-only snapshot at
`provenance/video-agent-bundle/MANIFEST.md` records HEAD
`ff272c2812008e6563eddd6bab981513c15c460a`; exact candidate SHA-256 values and
source states are in `provenance/video-agent-bundle/hashes.tsv`.

Selected candidates relevant to this slice are classified only for later review:

| Candidates | Captured state | Current classification |
|---|---|---|
| `videokit/cmd/videokit/main.go`, `videokit/internal/environment/check.go`, `videokit/internal/media/composer.go` | Tracked modifications | Behavioral references; no salvage decision. |
| Corresponding `main_test.go`, `check_test.go`, and `composer_test.go` | Tracked modifications | Test references; no salvage decision. |
| `videokit/internal/media/source.go`, `videokit/internal/media/derivatives.go` | Untracked | Behavioral references; no salvage decision. |
| Corresponding `source_test.go` and `derivatives_test.go` | Untracked | Test references; no salvage decision. |
| `third_party/remotion-composer/src/Explainer.tsx` | Tracked modification | Deferred behavioral reference; no salvage decision. |
| `third_party/remotion-composer/scripts/contract-tests.mjs` | Tracked modification | Deferred test reference; no salvage decision. |

The hashes point to the immutable snapshot evidence; they do not approve copying
or adaptation. No donor code has been copied, adapted, or salvaged into Video
Kit.

## Explicit Exclusions

- Topic research, script generation, TTS, stock/open-media search, generated
  imagery or video, subtitles, localization, publication, and paid providers.
- Remotion and HyperFrames composition for this slice.
- Browser capture, screen recording, local GPU execution, enhancement, and
  provider ranking.
- A daemon, server, database, queue, scheduler, workflow state machine, durable
  approvals, pipeline maturity status, UI, packaging, installers, and
  cross-CLI/operating-system portability work.
- Creative auto-editing, automatic highlight selection, hidden retries, or
  toolbox-owned review acceptance.
- Generic aggregate operations, additional convenience tools, renderer
  abstractions, or public helpers beyond the five named tools.

Remotion remains an approved existing composition runtime. Its inventory is
complete for slice selection, but implementation and parity are deferred until a
concrete production after this FFmpeg source-edit slice justifies it.
