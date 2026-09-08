# A narrated explainer, end to end

The path from "make me a short video about X" to a delivered file. Every step
here was run; the durations are measured on a real machine, not estimated.

Nothing about this is mandatory. A silent video skips step 2, a video with no
generated imagery skips step 4, and a title card needs only steps 3 and 5.

## 1. Agree the brief before producing anything

Thesis, audience, rough duration, and whether it should speak. Say which
renderer and providers you intend to use, and get explicit consent before any
paid generation. Unknown cost is not free.

## 2. Narration first, because it sets the timing

Write the script, then synthesize it. `edge_tts` is keyless and free.

```sh
facet tools run edge_tts --input '{"text":"Sunlight evaporates water. The vapour cools and condenses. Droplets merge until they fall.","output_path":"narration/voice.mp3"}'
```

Then measure it, because the audio decides how long the video must be:

```sh
facet tools run media_probe --input '{"input":"narration/voice.mp3"}'
```

Measured for the script above: 8.16 seconds. **Audio longer than the video is
cut off** — it does not extend the timeline — so set `duration_seconds` to cover
the narration.

## 3. Cuts that match the narration

Write `artifacts/explainer_props.json`. Cut fields are flat, and every scene
type is listed in `SCENE-TYPES.md` with the field it requires.

```json
{
  "width": 1280, "height": 720, "fps": 30, "duration_seconds": 9,
  "theme": "flat-motion-graphics",
  "audio": {"narration": {"src": "narration/voice.mp3", "volume": 1}},
  "cuts": [
    {"id": "c1", "type": "hero_title", "text": "How Rain Forms",
     "heroSubtitle": "A short explainer", "in_seconds": 0, "out_seconds": 3},
    {"id": "c2", "type": "text_card",
     "text": "Sunlight evaporates water", "in_seconds": 3, "out_seconds": 6},
    {"id": "c3", "type": "hero_title", "text": "That is rain",
     "heroSubtitle": "Evaporation to precipitation", "in_seconds": 6, "out_seconds": 9}
  ]
}
```

Beats should follow the narration rather than divide the duration evenly.

## 4. Generated imagery, only with consent

`gflow_image` and `gflow_video` cost real money and their price is unknown
before the call. Estimate first, get explicit human approval, then run. Use the
returned file as a cut source:

```json
{"id": "c2", "type": "image", "source": "assets/cloud.png",
 "title": "Condensation", "in_seconds": 3, "out_seconds": 6}
```

## 5. Render

```sh
facet tools estimate video_compose --input artifacts/explainer_props.json
facet tools run video_compose --input artifacts/explainer_props.json
```

Measured: 28 seconds at 720p and 83 seconds at 1080p. Under a module host, send
`"async": true` to get a job handle immediately and poll
`creative.jobs.status` rather than blocking.

The finished file here is 9.000 seconds, matching the last cut exactly. That
was not always true: the composition pads to `lastEnd + 1` when no duration is
stated, so this step used to deliver 10.05 seconds. Facet now sends the plan's
own end, and an explicit `duration_seconds` still wins when you want a tail
past the last cut.

Check the delivered duration anyway. The container reads slightly longer than
the video stream — a silent AAC track rounds up to whole audio frames — so read
the video stream's duration, which is the figure that matches the request.

## 6. Verify the file, not the exit code

```sh
facet tools run output_review --input '{"rendered_file":"renders/final.mp4","sample_count":4}'
```

`review_status: pass` means the technical gates passed — profile, duration,
codec, pixel format, audio, and that the frames are not empty. **It is not
creative acceptance.** Watch it, listen to it, and check the pacing and the
claims before delivering.

If frames come back identical or unusually small, the scenes did not render:
almost always a wrong `type` or a missing required field, which succeeds with a
blank frame rather than failing.

## 7. Deliver honestly

Say what was generated, what it cost, what you could not verify, and what a
person still needs to check.
