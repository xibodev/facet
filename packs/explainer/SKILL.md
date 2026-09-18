---
name: explainer
description: Produce reviewed 2D explainers and motion graphics with Facet and Remotion.
---

# Explainer

Use this pack for text-led explanation, diagrams, charts, product concepts, and
other 2D motion work. Confirm the thesis, audience, duration, format, and visual
direction. Narration, music, and captions are optional.

For a silent direct composition, create a request with explicit output profile
and a nonempty flat `cuts` array:

```json
{"composition_id":"Explainer","width":1920,"height":1080,"fps":30,"duration_seconds":4,"cuts":[{"id":"intro","type":"text_card","in_seconds":0,"out_seconds":4,"text":"A clearer explanation"}],"output":"renders/final.mp4"}
```

Use `packs/explainer/SCENE-TYPES.md` for supported scene fields. For requested
narration, follow `packs/explainer/NARRATED-WALKTHROUGH.md` and time visuals to
the measured audio. Estimate before rendering, do not silently downgrade the
renderer, sample frames, and review the final file against the requested
profile.
