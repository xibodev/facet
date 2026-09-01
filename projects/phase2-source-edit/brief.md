# Phase 2 Source Edit Brief

## Outcome

Produce the first durable Phase 2 video through the implemented Video Kit CLI:
a concise, playable vertical source edit with four review frames and a complete
ordinary-file production record.

## Source Truth

No user footage was supplied. `assets/source/source.mp4` is therefore an
original, reproducible local fixture rather than documentary or user footage.
It is a 16-second, 1280x720 audiovisual source made from four visually distinct
four-second test-pattern sections with audible tones. The fixture is generated
only by the narrowly scoped FFmpeg command in `reproduce.ps1`.

## Editorial Promise

Turn the source's pattern sequence into a brisk vertical rhythm that starts
with the most visually forceful section, resets to the opening pattern, and
closes on a third contrasting texture without repeating every available idea.

## Delivery Profile

- Output: `renders/final.mp4`
- Profile: 720x1280 portrait, 30 fps
- Media: H.264, yuv420p, AAC, 48 kHz stereo
- Expected duration: 9 seconds, tolerance 0.15 seconds
- Route: supplied-source edit through local Video Kit tools
- Cost: USD 0
- Network: none
- Publication or external writes: none

## Decisions

- Reorder source sections to 4, 1, 3.
- Select three seconds from each chosen section, avoiding source boundaries.
- Omit section 2 so the result is selective rather than a reformatted copy.
- Use center-cover reframing because the synthetic patterns keep their useful
  detail centrally and can tolerate the strong landscape-to-portrait crop.
- Retain source tones, then use `audio_mix` for a restrained gain adjustment,
  short head/tail fades, and loudness normalization. No extra music is needed.
