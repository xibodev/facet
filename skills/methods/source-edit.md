# Source Edit Method

Load this method when supplied footage is the production's source of truth and
selecting, arranging, reframing, and mixing that footage is the dominant work.
Then load `../shared/review-editing.md` for editing and review guidance.

## Route

1. Inventory the supplied media and preserve original paths. Use `media_probe`
   and targeted `frame_sample` calls to establish duration, streams,
   orientation, visual content, and defects. Record source-review facts only
   when they will support selection, execution, or provenance.
2. Define the editorial promise in one sentence. Choose moments for relevance,
   intelligibility, performance, continuity, and pacing; retain brief rationale
   for non-obvious inclusions or omissions.
3. Choose an output profile from the request. Explain consequential crop,
   padding, duration, quality, source-audio, or music compromises before making
   them. Do not invent missing visual or spoken claims.
4. Author an explicit ordered cut map only when it is useful for execution or
   revision. Invoke `source_edit` for trims, ordering, normalization,
    crop/scale/pad, per-segment focal placement, and cut transitions. Phase 2 is
    deliberately cut-only; it does not implement dissolves or other transition
    effects because the selected slice does not require them. Use `audio_mix` only when
   gain, fades, supplied music, ducking, or loudness normalization is needed.
5. Use the local FFmpeg route exposed by the live tools. Do not add generated or
   stock media, paid services, publication, automatic highlight selection, or a
   different renderer unless the user changes the request and authorizes the
   consequences.
6. Invoke `output_review` with concrete profile and acceptance checks, inspect
   its sampled evidence, then apply the shared judgment review. Revise the edit
   map or mix for material defects and rerun only affected work.

A simple trim may need only a short brief, an edit description, the render,
review evidence, and the final delivery record. Add separate source-review or
cut-map files only when their reuse or audit value justifies them.
