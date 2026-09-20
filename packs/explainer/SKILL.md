---
name: explainer
description: Produce reviewed 2D explainers and motion graphics with Facet and Remotion.
---

# Explainer

Use this pack for concise text-, metric-, and supplied-media-led explanation.
Confirm the route with `facet routes describe explainer` and assess it against
the actual script and visuals before choosing any optional provider.
Confirm the thesis, audience, duration, format, and visual direction.
Narration and music are optional.
Prefer Edge TTS for natural narration when network processing is acceptable.
Use Piper as the offline or privacy-preserving fallback. Name the voice and
provider in the production proposal, and never switch silently.

For a silent direct composition, create a request with explicit output profile
and a nonempty flat `cuts` array:

```json
{"composition_id":"Explainer","width":1920,"height":1080,"fps":30,"duration_seconds":4,"cuts":[{"id":"intro","type":"text_card","in_seconds":0,"out_seconds":4,"text":"A clearer explanation"}],"output":"renders/final.mp4"}
```

Use `packs/explainer/SCENE-TYPES.md` for the four supported primitives. Unknown
types, blank required content, overlapping cuts, and invalid metadata are
rejected. For requested narration, follow
`packs/explainer/NARRATED-WALKTHROUGH.md`, measure the synthesized audio, and
confirm it remains within the requested duration tolerance (5% by default)
before timing the visuals to it. Do not render until narration is within tolerance;
revise the script or obtain approval for a material duration difference.
Estimate before rendering, do not silently downgrade the renderer, sample
frames, and review the final MP4 against the requested profile.
