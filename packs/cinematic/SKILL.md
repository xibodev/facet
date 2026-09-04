---
name: cinematic
description: Produce cinematic documentary montages, historical deep dives, and archival videos using Facet, public domain assets, color grading, and audio mastering.
---

# Cinematic & Documentary Production Pack

Use this skill when producing a cinematic documentary, historical narrative, or atmospheric montage.

## Overview

The Cinematic / Documentary Montage pipeline transforms archival footage, photos, and generated clips into a cohesive film:
1. **Asset Sourcing:** Acquire high-resolution source images/clips (e.g. Wikimedia, NASA, or generated media) into `assets/raw/`.
2. **Narration & Script:** Write script beats and synthesize narration with `facet tools run edgetts`.
3. **Motion Montage:** Animate still photographs using subtle camera motion (slow zoom/pan) into graded clips.
4. **Color Grading:** Apply cinematic LUT or grading profiles with `facet tools run color_grade`.
5. **Video Stitching:** Seamlessly concatenate shots into `renders/montage.mp4` with `facet tools run video_stitch`.
6. **Audio Mastering:** Mix narration, background music, audio ducking, and loudness normalization via `facet tools run audio_mix`.
7. **Final Review:** Validate broadcast metrics and audio sync with `facet tools run output_review`.

## Recommended Styles

- `premium-minimalist.yaml`: Elegant typography, rich dark contrast, restrained motion.
- `anime-ghibli.yaml`: Warm palette, painterly aesthetic, lush soundscape.

## Tool Commands

```powershell
# Synthesize narration
facet tools run edgetts --input artifacts/requests/edge-tts.json

# Apply color grading to a shot
facet tools run color_grade --input artifacts/requests/color-grade-shot1.json

# Stitch multiple graded shots
facet tools run video_stitch --input artifacts/requests/video-stitch.json

# Mix narration and background music with auto-ducking
facet tools run audio_mix --input artifacts/requests/audio-mix.json

# Review final output against quality gates
facet tools run output_review --input artifacts/requests/output-review.json
```
