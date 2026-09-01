# Phase 2 Source Edit Delivery

## Delivered Output

- Video: `renders/final.mp4`
- Source: `assets/source/source.mp4`
- Reproduction: `reproduce.ps1`
- Technical report: `review/report.json`
- Final review frames: `review/final-frames/frame-0001.jpg` through
  `frame-0004.jpg`
- Source inspection frames: `review/source-frames/frame-0001.jpg` through
  `frame-0004.jpg`

## Material Provenance

No user footage was provided. The source is an original synthetic fixture made
locally from FFmpeg `testsrc2`, `smptebars`, `yuvtestsrc`, and `rgbtestsrc`
visual sources plus four sine tones. It depicts no people, brands, places, or
events and makes no documentary claim. The exact generation command is retained
in `reproduce.ps1`; `PROVENANCE.md` classifies the fixture and reproduction
record as original Video Kit work.

## Editorial Result

The 16-second landscape source contains four four-second sections. The final
nine-second portrait edit uses three three-second selections in source order
4, 1, 3 and deliberately omits section 2. This opens on the strongest simple
color pattern, moves to the most active pattern, and closes on a contrasting
technical pattern rather than preserving the fixture's original sequence.

Center-cover reframing converts 1280x720 to 720x1280. This is intentionally
aggressive, but the fixture's central test-pattern content remains readable in
all four final review frames. Source tones are retained. `audio_mix` applies
-4 dB gain, 0.2-second fade-in, 0.4-second fade-out, and a -16 LUFS/-1.5 dBTP
loudness request; no additional music was needed.

## Review Outcome

Status: **pass**

Video Kit's `output_review` completed successfully and passed all five gates:

- 720x1280 at 30 fps
- 9.0-second duration within 0.15-second tolerance
- H.264 video
- yuv420p pixel format
- AAC audio at 48 kHz stereo

Four uniform evidence frames were produced at 1.125, 3.375, 5.625, and 7.875
seconds. Visual inspection confirms the intended sequence, deliberate central
crop, distinct sections, and no blank, corrupt, stretched, or misoriented
frames. Audio evidence reports mean volume `-18.7 dB` and maximum volume
`-15.1 dB`; the tones are present and comfortably below clipping.

## Execution Facts

- Provider: local
- Network access: none
- Estimated provider cost: USD 0
- Actual provider cost: USD 0
- External write/publication: none
- Editing, audio finishing, probing, sampling, and review: Video Kit CLI
- Direct FFmpeg use: fixture generation only

## Known Limitations

- The source is a synthetic audiovisual fixture, not user or real-world footage.
- The portrait crop intentionally discards the landscape edges.
- Editorial review is limited to the fixture's pattern motion, cut sequence,
  representative frames, and audible tones; it does not establish behavior for
  speech, faces, captions, or narrative footage.
