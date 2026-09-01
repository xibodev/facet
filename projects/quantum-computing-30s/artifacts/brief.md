# Brief — Quantum Computing (30s Explainer)

## Deliverable
- **Format:** 1920×1080, 30fps, H.264/yuv420p, AAC 48kHz stereo
- **Duration:** 30.0s exactly (final cut ends at 29.0s; Remotion `calculateMetadata` adds 1s padding → `(29+1)*30 = 900 frames`)
- **Output:** `projects/quantum-computing-30s/renders/final.mp4`

## Audience & Tone
Curious general audience — smart, non-physicist. Tone: clear, energetic, credible. No hype, no "quantum will break all encryption tomorrow" overclaiming.

## Core Message
A classical bit is one definite answer at a time. A qubit explores many possibilities at once — which makes quantum computers extraordinary at a *narrow* set of problems, not a faster laptop.

## Narrative Arc (hook → setup → build → climax → landing)
1. **Hook (0–4s):** Everyday computers are astonishingly good — but some problems break them.
2. **Setup (4–11s):** The bit: 0 or 1. One answer at a time.
3. **Build (11–19s):** The qubit: superposition + entanglement. Possibilities explored together.
4. **Climax (19–25.5s):** Real, bounded payoff — molecules, materials, optimization.
5. **Landing (25.5–29s):** Not a faster laptop. A different kind of machine.

## Accuracy Guardrails
- Do NOT say a quantum computer "tries every answer simultaneously" — that is the classic pop-science error. Interference amplifies correct answers and cancels wrong ones.
- Do NOT claim general-purpose superiority. Advantage is problem-specific.
- Avoid fabricated benchmark statistics. Use qualitative, defensible framing rather than invented numbers.

## Route (local / zero-cost)
- **Narration:** `edge_tts` — keyless Microsoft Edge neural TTS (local, free, no API key)
- **Visuals:** Remotion `Explainer` composition (installed at `remotion-composer/`, node_modules present)
- **Theme:** `flat-motion-graphics` (dark #0F172A, violet/pink accent, Space Grotesk) — high contrast, strong for science explainers
- **Render:** `video_compose` with `operation: remotion_render`
- **Total cost:** $0.00 (no paid APIs, no external mutations)

## Asset Path Rule (verified in source)
`remotion-composer/src/lib/*.ts` → `resolveAsset()`: absolute paths become `file://` URLs; **relative paths fall through to `staticFile()`** and would resolve into `public/`. Narration MUST therefore be passed as an **absolute path**.
