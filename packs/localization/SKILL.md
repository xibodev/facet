---
name: localization
description: Produce translated subtitles, narration, and localized video variants with Facet.
---

# Localization

Use this pack for an existing video's language variants. Confirm target locale,
terminology, reading level, subtitle format, voice requirements, and whether
timing may change.
Assess `localization` against the source and human-reviewed translated text;
Facet does not provide an implicit translation or voice provider.
Use the canonical `ffmpeg_caption_burn` operation for burned subtitles. The
legacy `remotion_caption_burn` name is only a compatibility alias.
For Edge voice dubbing, pass the synthesized narration to `source_edit` as
`replacement_audio`; `audio_mix.source` only controls the existing source audio.

Work from an accurate transcript and timestamps. Preserve meaning rather than
translating mechanically, fit subtitles to readable timing and safe regions,
and use only voices that support the target language. Voice cloning and lip
sync need separate provider support and consent. Review translation, timing,
pronunciation, mix, and requested delivery properties with a qualified human
when accuracy matters.
