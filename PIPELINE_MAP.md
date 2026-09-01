# Phase 1 Pipeline Map

## Basis And Criteria

This map is based on a shallow, comprehensive inspection of all 13 definitions
under clean upstream OpenMontage `pipeline_defs/*.yaml` at pinned commit
`cd9f3c1f03368be87b140af494914b8ee4e3c7a4`. It covers every pipeline definition
at that pin; the inspection was definition-level rather than a deep dependency
or implementation review.

Each manifest is normalized by asking:

- what source of truth grounds the production;
- what dominant transformation produces the result;
- which irreversible creative decisions, artifacts, and review criteria are
  distinct;
- whether unique tool mechanics justify a production method rather than a
  route, treatment, output profile, or reusable skill;
- whether the result belongs under one primary pipeline without creating a
  pipeline chain.

## Normalized Taxonomy

The normalized pipelines are **explainer**, **animation**, **avatar
spokesperson**, **character animation**, **cinematic**, **content repurpose**,
**documentary**, **source edit**, **localization**, and **product demo**.
**Music-led production** is a required gap to add when its method is defined.

| Upstream manifest | Disposition | Normalized destination | Rationale |
|---|---|---|---|
| `animated-explainer` | Rename | `explainer` | Explanation is the dominant method; animation is a treatment or supporting capability unless animation itself carries the production method. |
| `animation` | Retain | `animation` | Motion design and animation decisions, artifacts, continuity, and review are materially distinct. |
| `avatar-spokesperson` | Retain | `avatar-spokesperson` | Generated-presenter performance, consent, speech, and synchronization form a distinct method. |
| `character-animation` | Retain | `character-animation` | Character identity, performance, continuity, and animation review require distinct guidance. |
| `cinematic` | Retain | `cinematic` | Shot-led montage and cinematic image-making are the dominant transformation. |
| `clip-factory` | Merge | `content-repurpose` | Selecting and reshaping moments from existing long-form content is repurposing; volume is an execution route, not a method. |
| `documentary-montage` | Rename | `documentary` | Evidence, sourcing, rights, and factual editorial construction define the method; montage is a treatment. |
| `framework-smoke` | Exclude | None | A framework verification fixture is not a user production method. |
| `hybrid` | Convert to route | `source-edit` | Mixed treatment does not establish a separate source of truth or dominant method; supplied source material remains primary. |
| `localization-dub` | Rename | `localization` | Translation and cultural/linguistic adaptation are primary; dubbing is one route alongside captions, graphics, and other localized elements. |
| `podcast-repurpose` | Merge | `content-repurpose` | Podcast is a source/deliverable context; moment selection and transformation are the same repurposing method. |
| `screen-demo` | Rename | `product-demo` | Observable product behavior and product truth carry the proof; screen capture is one route. |
| `talking-head` | Convert to route | `source-edit` | Supplied presenter footage is inspected and edited; presenter framing is a treatment, not a separate production method. |

## Extracted Skills

The manifest distinctions should be preserved as progressively loaded knowledge,
not duplicated pipelines:

- **Shared production:** intake, direction, storytelling, visual planning, asset
  planning, editing, sound, delivery, and output review.
- **Explainer:** concept decomposition, explanatory structure, visual analogy,
  narration planning, and comprehension review.
- **Animation:** motion language, timing, transitions, graphic continuity, and
  animation review.
- **Avatar spokesperson:** consent, presenter selection, performance direction,
  speech alignment, lip-sync evaluation, and disclosure/provenance.
- **Character animation:** character bible, identity and scene continuity,
  performance direction, pose/motion planning, and continuity review.
- **Cinematic:** shot language, visual motif, coverage, montage rhythm, and
  cinematic sound design.
- **Content repurpose:** source review, transcript navigation, moment selection,
  derivative cut mapping, reframing, hook construction, and multi-output review.
- **Documentary:** evidence-led research, source logging, rights, factual
  integrity, interview/archive construction, and documentary review.
- **Source edit:** media inspection, selects, continuity, cut maps, source-audio
  treatment, reframing, and edit review. Hybrid and talking-head guidance lives
  here as route knowledge.
- **Localization:** translation intent, terminology, cultural adaptation,
  dubbing, subtitle timing, localized graphics, sync, and linguistic review.
- **Product demo:** product truth, reproducible capture, task flow, proof,
  annotation, and behavioral accuracy review.
- **Tool/runtime:** FFmpeg and ffprobe mechanics are loaded after route
  selection; Remotion, HyperFrames, capture, stock, and provider guidance remain
  separately loadable when selected.

## Music-Led Gap

None of the 13 manifests represents production in which a supplied music track,
beat structure, and musical progression are the source of truth and dominant
editorial clock. Add a `music-led` pipeline rather than treating music as merely
background audio. Its distinct knowledge should cover track and beat analysis,
phrase structure, synchronization, visual development around musical events,
rights, loudness, and audiovisual rhythm review. Route and artifact details
remain to be defined from a real production need.
