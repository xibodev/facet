# Facet Explainer contract

The Facet-owned `Explainer` composition uses a nonempty, ordered `cuts` array.
Cuts are direct: there are no implicit transitions, padding, or generated
intermediate scenes.

## Scene primitives

| `type` | Required fields | Optional fields |
| --- | --- | --- |
| `text_card` | `text` | `fontSize`, `backgroundColor`, `color` |
| `hero_title` | `text` | `subtitle`, `backgroundColor`, `color` |
| `stat_card` | `stat` | `label`, `backgroundColor`, `color` |
| `media` | `source`, `media_kind` (`image` or `video`) | `fit` (`contain` or `cover`), `title`, `muted`, `backgroundColor`, `color` |

Every cut also requires `in_seconds` and `out_seconds`, with
`0 <= in_seconds < out_seconds`. Cuts must be ordered, must not overlap, and
must fit within the composition duration. Blank required strings and unknown
types are rejected before rendering.

## Composition profile

```json
{
  "composition_id": "Explainer",
  "width": 1280,
  "height": 720,
  "fps": 30,
  "duration_seconds": 4,
  "cuts": [
    {
      "id": "intro",
      "type": "text_card",
      "text": "A clear explanation",
      "in_seconds": 0,
      "out_seconds": 4
    }
  ],
  "output": "renders/final.mp4"
}
```

Width and height default to 1920x1080 and must be positive even safe integers.
FPS defaults to 30. `duration_seconds * fps` must be a positive whole frame
count. When duration is omitted, the composition ends at the last cut.

## Audio and local assets

Narration and music are optional:

```json
{
  "audio": {
    "narration": {"src": "narration/voice.mp3", "volume": 1},
    "music": {"src": "audio/bed.mp3", "volume": 0.2, "loop": true}
  }
}
```

Volume is between 0 and 1. Audio does not extend the video; narration longer
than `duration_seconds` is truncated. A composition with no audio declaration
and no media source audio renders without an audio stream.

Local `media.source` and audio `src` paths are copied into an isolated,
temporary Remotion public directory. Facet stages only those explicit media
fields, rejects non-media or escaping paths, and removes the staging directory
after the render.
