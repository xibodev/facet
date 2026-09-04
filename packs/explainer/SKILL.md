---
name: explainer
description: Produce 2D animated explainer videos using Facet and Remotion. Use for topic explainers, product walkthroughs, educational concepts, and motion graphics.
---

# Explainer Production Pack

Use this skill when producing an animated explainer, educational video, or motion graphics piece.

## Overview

The Explainer pipeline generates polished 2D explainer videos from a concept or script:
1. **Research & Concept:** Clarify core thesis, target audience, duration, and key data points.
2. **Script & Narration:** Write timed narrative beats and generate voiceover audio using `facet tools run edgetts`.
3. **Scene Plan:** Generate visual cards, comparisons, stats, and charts in `artifacts/scene_plan.json`.
4. **Remotion Composition:** Render visual elements and timed audio tracks into `renders/final.mp4`.
5. **Quality Review:** Sample frames with `facet tools run frame_sample` and verify AV sync with `facet tools run output_review`.

## Recommended Styles

- `clean-professional.yaml`: Modern corporate, subtle gradients, clean typography (Inter / Space Grotesk).
- `flat-motion-graphics.yaml`: High contrast, punchy primary accents, geometric shapes.
- `minimalist-diagram.yaml`: Dark background, mono typography, focused chart lines.

## Tool Commands

```powershell
# Generate voiceover for narrative beats
facet tools run edgetts --input artifacts/req_tts.json

# Probe generated audio for exact duration
facet tools run media_probe --input '{"file_path": "narration/beat1.mp3"}'

# Verify final render quality
facet tools run output_review --input '{"rendered_file": "renders/final.mp4"}'
```
