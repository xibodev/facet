# Video Kit Design

## Product Definition

Video Kit is a Claude-first agent bundle that uses curated Markdown/YAML
production guidance and a Go toolbox derived from OpenMontage to turn
conversational requests into completed videos.

The first product runs headlessly inside Claude Code. A future UI may wrap the
proven headless behavior, but no UI or host architecture is part of the current
product.

## User Experience

A user describes the video they want in ordinary conversation. Claude then:

1. understands the intent and relevant source material;
2. chooses an appropriate production pipeline and producer persona;
3. discovers currently implemented and configured tools;
4. explains consequential free, paid, stock, supplied, and generated options;
5. develops the direction, story, script, scenes, assets, edit, and sound plan;
6. asks before paid work, publication, or material creative downgrades;
7. invokes tools to acquire, generate, analyze, edit, and compose media;
8. reviews the resulting video and revises defects;
9. returns the completed video and its material provenance.

Claude may adapt stage ordering when the request or source material warrants it.
Pipeline guidance constrains quality and required decisions; it is not a code-
owned workflow state machine.

## Architecture

```text
User
  -> Claude Code
     -> root producer skill
        -> pipeline + persona + progressively loaded production knowledge
        -> live Go toolbox catalog
           -> stock and open-media clients
           -> image/video/audio/provider clients
           -> analysis and post-production tools
           -> FFmpeg / browsers / other existing binaries
           -> Remotion / HyperFrames
         -> ordinary project files and rendered media
```

## Production Architecture

Video Kit separates what the user wants from how the production is made:

- **User intent:** the outcome the user wants.
- **Video type or deliverable:** user-facing language such as UGC ad, tutorial,
  trailer, documentary, Short, or product film.
- **Source mode or source of truth:** the material that grounds the production,
  such as a topic, product URL, supplied footage, reference video, open-media
  corpus, music track, existing film, or character assets.
- **Pipeline:** the dominant production method that turns the source into the
  desired result.
- **Route:** one concrete, capability-dependent way to execute that pipeline
  with the currently available inputs, tools, providers, runtimes, and budget.
- **Persona:** a reusable creative reasoning posture adopted by Claude, not a
  separate running process.
- **Skill:** progressively loaded production or tool knowledge.
- **Capability:** an abstract mechanical need such as speech, motion footage,
  source inspection, image generation, or composition.
- **Tool:** an executable implementation of a capability.
- **Provider:** the local runtime, stock source, or remote service used by a
  tool.
- **Style or treatment:** the visual, editorial, motion, and sonic language.
- **Output profile:** delivery constraints such as dimensions, frame rate,
  codec, duration, and platform.
- **Artifact:** a useful project-local record created when that production
  needs it.
- **Review:** technical and judgment evaluation of a particular produced
  output.

Claude reasons about a production using this model:

```text
user outcome
+ source of truth
+ requested deliverable
+ explicit constraints
-> primary production pipeline
+ producer persona
-> feasible route from live capabilities, providers, inputs, and budget
-> progressively loaded shared, method, and tool knowledge
-> request-appropriate artifacts
-> tool execution and composition
-> output-specific review and revision
```

### Pipeline Selection Rules

- Claude Code is the sole active orchestrator.
- Personas are reasoning postures, not processes, subagents, services, or state
  owners. There is no swarm or lifecycle of producer and director agents.
- A user-facing video type is not automatically a pipeline.
- A pipeline represents a materially distinct production method.
- A route represents one feasible implementation of that method.
- One primary pipeline leads a production. Supporting capabilities do not
  create a pipeline chain.
- Reference-driven production is normally an intake modifier, not a pipeline.
- Style, genre, orientation, duration, and output platform are orthogonal to
  pipeline selection.
- Pipeline definitions reference capabilities, not hard-coded providers.
- A shipped pipeline is selectable. Request feasibility depends on current
  inputs and configured tools.
- There is no planned/development/operational promotion system.
- Review applies to the produced output, not to an abstract pipeline.

For example, "Make a vertical UGC-style ad for my app" does not identify a
pipeline. Product demo is appropriate when observable product behavior carries
the proof; source edit when the user supplies creator footage; avatar
spokesperson when the presenter is generated; and cinematic when montage with
generated or stock imagery carries the message. "UGC-style," "vertical," and
"ad" describe treatment, output profile, and deliverable rather than production
method.

A documentary with animated maps remains primarily documentary; animation is a
supporting capability. An English film dubbed into Spanish is primarily
localization; translation, TTS, lip sync, and localized graphics are supporting
capabilities.

### Working Pipeline Hypothesis

The final pipeline list is intentionally not locked before donor inventory.
Likely production methods include explainer, product demo, source edit, content
repurpose, cinematic, documentary, animation, character animation, avatar
spokesperson, localization, and music-led production.

Phase 1 must map every pinned upstream pipeline to retain as a pipeline, merge
with another pipeline, convert to a route, convert to a reusable skill, rename,
exclude, or replace/add where a real method is missing. Decisions must consider
source of truth, dominant transformation, irreversible creative decisions,
unique tool mechanics, useful artifacts, and distinct review criteria.

Possible normalization candidates may guide investigation but are not decisions:
talking head, interview, webinar, testimonial, and some hybrid work may become
source-edit routes; clip factory, podcast repurpose, long-form-to-shorts, and
event highlights may become content-repurpose routes; YouTube Shorts, TikTok,
UGC style, tutorials, trailers, and vertical video are generally deliverables,
profiles, genres, or treatments rather than pipelines.

## Agentic Information Architecture

The intended agent surface is approximately:

```text
SKILL.md
-> understand the request and inspect supplied material
-> select one primary pipeline
-> adopt one producer persona
-> query the live Go toolbox
-> propose feasible routes, costs, and material tradeoffs
-> load relevant production and provider skills
-> create only useful artifacts
-> invoke tools and existing runtimes
-> review and revise the output
```

Claude progressively loads a likely knowledge hierarchy:

- **Shared production skills:** research, direction, storytelling, visual
  planning, asset planning, editing, sound, and review.
- **Method-specific skills:** product truth, source review, moment selection,
  documentary sourcing and rights, character continuity, avatar performance and
  consent, localization, and music and beat analysis.
- **Tool and provider skills:** Remotion, HyperFrames, FFmpeg, Kling, Pexels,
  ElevenLabs, and other selected tool-specific guidance.

The upstream OpenMontage knowledge inventory informs the final hierarchy, but
its governing Markdown is not copied wholesale.

## Responsibilities

### Claude Code

Claude owns all semantic and creative work:

- conversational intake and clarification;
- intent and pipeline selection;
- producer/director posture;
- research and creative reasoning;
- direction, story, script, scene, asset, edit, sound, and tool choices;
- provider selection using live toolbox facts;
- cost and consequential-choice conversation;
- stage progression and native session continuation;
- artifact authoring;
- visual and editorial review;
- diagnosis and repair after tool failures.

Claude uses native browser and web capabilities for research, inspection,
interactive reasoning, and ordinary web investigation.

### Markdown, YAML, And Schemas

The agent surface teaches Claude how to produce videos:

- one root producer/router skill;
- producer personas;
- pipeline definitions for materially different production methods;
- shared creative and production skills;
- pipeline-specific skills only where the method is genuinely unique;
- tool/provider skills loaded after a tool is selected;
- style playbooks;
- artifact and tool schemas.

This surface is intentionally redesigned from OpenMontage. It should preserve
useful production expertise without duplicating every director or forcing Claude
to load hundreds of files for every request.

### Go Toolbox

Go owns executable mechanics only:

- tool registration and description;
- dependency and credential status;
- request validation;
- provider HTTP clients, uploads, polling, and downloads;
- cost estimates supplied to Claude;
- deterministic media processing and analysis;
- invocation of FFmpeg, browsers, Remotion, HyperFrames, and other binaries;
- structured JSON results and useful errors.

Go does not own creative orchestration, pipeline progression, approval dialogue,
session continuation, or project workflow state.

Go may report implementation and configuration status, deterministic
eligibility, supported operations, input/output constraints, expected duration
or latency, estimated cost, provider limitations, relevant quality or
continuity signals, and rejection reasons. It may reject mechanically impossible
candidates. Any ranking must return candidates, inputs, signals, and rationale;
it must not silently execute the top-ranked provider. Claude chooses the
provider and explains consequential creative and commercial tradeoffs.

Go invokes a browser only for reproducible production mechanics such as
deterministic page capture, screen recording, or browser-rendered media. Video
Kit does not implement a browser host, browser service, general automation
framework, or remote browser platform.

### Consequential-Action Guards

Tool metadata declares whether an operation may incur cost, publish externally,
or mutate an external account or service. Estimation is always side-effect free.

A potentially paid invocation requires an explicit request-local authorization
signal containing at least the provider, operation, permission to incur cost,
and maximum authorized estimated cost. External publication or mutation
requires its own explicit request-local authorization signal. Go validates the
signal before execution but stores no approval state.

Results record provider, operation, estimate, actual cost when available,
external task or asset identifiers, and whether an external write occurred.
This is an accidental-execution guard, not a durable approval service,
authorization server, cryptographic consent system, governance subsystem, or
workflow state machine. Claude remains responsible for obtaining consent in
conversation.

### Existing Runtimes

Remotion, HyperFrames, FFmpeg, browsers, and similar tools remain independent
runtimes. Video Kit invokes and integrates them; it does not recreate them.

## Project Record

Productions use ordinary files that Claude can inspect and edit, for example:

```text
projects/<project>/
  brief.md
  artifacts/
    research.json
    proposal.json
    script.json
    scene-plan.json
    assets.json
    edit.json
  assets/
  compositions/
  renders/
  review/
```

This layout is a convention, not hidden workflow state. Claude may resume from
its native session or reconstruct context by reading these files.

A production creates only the artifacts useful to its request. Schemas constrain
artifacts when they exist; they do not force every production to instantiate
research, proposal, script, scene plan, assets, and edit files. Claude may adapt
the work sequence to the request and source material.

For example, a source trim may need only a brief, source review, edit map,
output, and review. A topic explainer may need research, script, scene plan,
assets, composition, and review. A music visualizer may need track analysis,
visual direction, composition, and review.

Every completed production has a small final delivery record containing the
output location, material provenance, consequential creative and provider
choices, cost and network facts, review outcome, and known limitations.

## Tool Surface

The expected toolbox shape is a single Go binary with a discoverable contract:

```text
videokit tools list
videokit tools describe <tool>
videokit tools estimate <tool> --input <request.json>
videokit tools run <tool> --input <request.json>
```

Long-running providers may additionally expose status and cancellation if real
ported tools require them. Existing direct commands may remain when they are
useful; uniformity is not a reason to rewrite working mechanics.

Tool descriptions should preserve useful OpenMontage concepts such as name,
capability, provider, dependencies, configuration status, input/output schema,
cost behavior, supported operations, and relevant skills.

## Pipeline And Capability Semantics

A shipped pipeline definition is available for Claude to select. It is not
globally blocked by a maturity label.

The system reports separate facts:

- whether the pipeline instructions exist;
- whether a tool client is implemented;
- whether its credentials or runtime dependencies are configured;
- whether available inputs and tools can satisfy this specific request;
- whether this produced video passed review.

For example, cinematic production may be feasible with supplied or stock
footage while Kling is unconfigured. Kling's configuration does not determine
whether the cinematic pipeline exists.

There is no planned/development/operational promotion system for pipelines.
Review and acceptance apply to produced videos and toolbox behavior, not to an
abstract pipeline label.

## First-Release Scope

The first release targets only Claude Code in a source checkout and includes:

- redesigned producer, persona, pipeline, skill, and style guidance;
- a useful production-driven non-GPU Go toolbox port derived from upstream
  OpenMontage;
- stock/open media, cloud image/video/audio providers, source media, analysis,
  subtitles, enhancement, capture, and post-production as porting permits;
- OpenMontage-derived Remotion composition;
- HyperFrames integration;
- FFmpeg media work and inspection;
- ordinary project artifacts and real video production.

Broad non-GPU parity is a donor-preservation target, not necessarily a
first-release gate. Video Kit need not port every upstream provider wrapper
before it is useful.

Provider clients may ship as implemented but unconfigured. Mock contract tests
are sufficient before credentials are available. Live use hardens them later.

## Deferred Scope

- Other agentic CLIs and compatibility projections.
- Web or desktop UI.
- Facet integration or reuse.
- Daemons, servers, databases, and durable workflow hosts.
- Installers and distribution packaging.
- Multi-user, high-scale, enterprise, and cross-platform architecture.
- Local GPU model execution and installation.
- Broad publication integrations.

## Design Invariants

1. Claude Code is the sole reference CLI until the headless product works.
2. The agentic CLI is the orchestrator.
3. Go is a stateless toolbox, not a workflow controller.
4. The clean upstream OpenMontage repository is the primary tool-behavior donor.
5. The agentic surface is intentionally redesigned rather than lift-shifted.
6. Provider configuration is runtime state, not pipeline maturity.
7. Shipped pipelines are selectable; route feasibility is request-specific.
8. Review belongs to each produced video.
9. Remotion and HyperFrames are reused, not recreated.
10. Real productions are the primary integration and hardening loop.
11. No future UI or compatibility requirement may shape the first headless
    implementation without explicit user approval.
12. No new subsystem is justified merely because it could be useful later.

## Early Success

The Phase 2 milestone is one real, reproducible, non-paid video produced through
Claude Code using live toolbox discovery, ordinary project files, and one
existing composition route. It must be an actual reviewed video, not only unit
tests, schemas, or mocked contracts.

## First-Release Success

Video Kit first succeeds when Claude can discover the bundle, understand varied
video requests, inspect the live toolbox, offer honest available routes, produce
ordinary artifacts, invoke multiple acquisition and composition tools, create
several materially different videos, review them, and repair observed defects.

Success does not require every provider to be configured, every operating system
to be supported, every pipeline to have a certification film, or a UI to exist.
