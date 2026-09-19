# Narrated explainer walkthrough

Narration is optional. When requested, write and synthesize it first, then use
`media_probe` to measure its actual duration. Unknown provider cost is not free;
estimate and obtain consent before paid generation.

Create a frame-aligned request whose timeline covers the narration:

```json
{
  "composition_id": "Explainer",
  "width": 1280,
  "height": 720,
  "fps": 30,
  "duration_seconds": 9,
  "audio": {
    "narration": {"src": "narration/voice.mp3", "volume": 1},
    "music": {"src": "audio/bed.mp3", "volume": 0.15, "loop": true}
  },
  "cuts": [
    {
      "id": "opening",
      "type": "hero_title",
      "text": "How Rain Forms",
      "subtitle": "A short explainer",
      "in_seconds": 0,
      "out_seconds": 3
    },
    {
      "id": "process",
      "type": "text_card",
      "text": "Water evaporates, cools, and condenses",
      "in_seconds": 3,
      "out_seconds": 6
    },
    {
      "id": "result",
      "type": "stat_card",
      "stat": "3 steps",
      "label": "evaporation, condensation, precipitation",
      "in_seconds": 6,
      "out_seconds": 9
    }
  ],
  "output": "renders/final.mp4"
}
```

Run `facet tools estimate video_compose --input <request-file>`, then render
with `facet tools run video_compose --input <request-file>`. Audio longer than
the video is truncated rather than extending the timeline, so raise
`duration_seconds` and the last cut end when needed.

Verify the delivered MP4 with `media_probe`, `frame_sample`, and
`output_review`. Check the video stream duration, visible frame variation,
requested dimensions and frame rate, expected audio presence, and the actual
spoken pacing. Technical success is not creative acceptance.
