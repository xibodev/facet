# Cinematic Documentary Brief: The Voyager Golden Record

## Outcome

Produce the 3rd materially different video project under `projects/cinematic-documentary/` to fulfill Phase 4 First-Release readiness:
A 16:9 landscape 1920x1080 cinematic documentary montage (~18.5s) exploring humanity's message to the stars on the Voyager Golden Record.

## Pipeline & Topic

- **Pipeline**: Cinematic / Documentary montage
- **Topic**: "The Voyager Golden Record — Humanity's Message to the Stars"
- **Aspect Ratio**: 16:9 Landscape (1920x1080)
- **Target Duration**: 15–20 seconds (Delivered: 18.52 seconds)
- **Frame Rate**: 30.0 fps

## Production Method & Tool Orchestration

1. **Voiceover Generation**: Synthesize neural voiceover using `edge_tts` (`en-US-ChristopherNeural`) through the Video Kit CLI.
2. **Media Acquisition**: Acquire high-resolution NASA / public domain visual assets via `wikimedia` stock search tool.
3. **Cinematic Motion & Color Grading**: Create slow cinematic camera motion for each scene and apply `color_grade` (`cinematic_warm` LUT profile) across all shots.
4. **Assembly**: Sequence and assemble shots using `video_stitch` with exact target resolution and frame rate normalization.
5. **Sound Design & Audio Mixing**: Mix voiceover narration and ambient space drone soundtrack using `audio_mix` with ducking (-14 dB gain, 4:1 ratio, 0.15 threshold) and EBU R128 loudness normalization (-16 LUFS, -1.5 dBTP).
6. **Technical Output Review**: Execute `output_review` with 5 validation gates and 4 uniform review frame samples.

## Delivery Profile

- **Output**: `renders/final.mp4`
- **Resolution**: 1920x1080 landscape
- **Frame Rate**: 30 fps
- **Video Codec**: H.264
- **Audio Codec**: AAC, 48 kHz stereo
- **Expected Duration**: 18.52 seconds (tolerance: ±0.20s)
- **Estimated Cost**: USD 0.00
- **External Network Writes**: None
