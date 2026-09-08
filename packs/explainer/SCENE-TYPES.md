# Explainer scene types

Every scene type the `Explainer` composition renders, with the fields each one
requires. **A cut whose required field is missing renders as an empty frame** —
the render still succeeds, so a silent typo produces a video of blank scenes
rather than an error. Check the type and its required field together.

Cut fields are FLAT. `type`, `text`, `title` and the rest sit directly on the
cut object beside `in_seconds`/`out_seconds`. There is no nested `scene` object.

```json
{"id":"c1","type":"text_card","text":"Hello","in_seconds":0,"out_seconds":3}
```

## Text and titles

| type | required | optional |
| --- | --- | --- |
| `text_card` | `text` | `fontSize` |
| `hero_title` | `text` | `heroSubtitle` or `subtitle` |
| `section_title` | `text` | `subtitle` |
| `callout` | `text` | `callout_type`, `title`, `backgroundColor` |

## Data and metrics

| type | required | optional |
| --- | --- | --- |
| `stat_card` | `stat` | `subtitle` |
| `stat_reveal` | `stat` | `subtitle` |
| `kpi_grid` | `chartData` | `title` |
| `progress_bar` | `progress` | `title` |
| `comparison` | `leftLabel`, `rightLabel`, `leftValue`, `rightValue` | `title`, `cardBackgroundColor` |

## Charts

| type | required | optional |
| --- | --- | --- |
| `bar_chart` | `chartData` | `title` |
| `pie_chart` | `chartData` | `title` |
| `line_chart` | `chartSeries` | `title` |

## Scenes with media or motion

| type | required | optional |
| --- | --- | --- |
| `terminal_scene` | `steps` | `terminalTitle`, `prompt` |
| `screenshot_scene` | `backgroundImage`, `screenshotSteps` | `screenshotSize`, `cursorStartAt` |
| `parallax` | `backgroundImage` | — |
| `anime_scene` | see the character-animation pack | — |
| `provider_chip` | `text` | — |

## Composition profile

Top-level props, beside `cuts`:

```json
{"width":1280,"height":720,"fps":30,"duration_seconds":9,"theme":"flat-motion-graphics","cuts":[…]}
```

Dimensions must be positive even integers; `duration_seconds * fps` must be a
whole frame count. Omitting `duration_seconds` extends to the last cut's
`out_seconds` plus one second. Cuts need `0 <= in_seconds < out_seconds` and
must fit inside an explicit duration — invalid timings are rejected rather than
truncated.

Narration is optional:

```json
{"audio":{"narration":{"src":"narration/voice.mp3","volume":1}}}
```

Audio longer than the video does not extend it; set `duration_seconds` to cover
the narration, or the tail is cut off.

## Verify the render, not the exit code

A successful render is not a correct one. Sample frames and confirm they differ:

```sh
facet tools run frame_sample --input '{"input":"renders/final.mp4","output_dir":"artifacts/frames","strategy":{"type":"uniform","count":4}}'
```

Identical frames mean the scenes did not render — usually a wrong `type`, or a
required field that is absent or misspelled.
