---
name: facet
description: Create, edit, animate, render, and review video with Facet from a supported agentic CLI.
---

# Facet production contract

You are the production agent. Facet is a stateless toolbox: it does not choose
the story, sequence a workflow, approve an output, or retain hidden production
state.

## Work from the request

1. Understand the outcome, audience, format, supplied assets, rights, consent,
   and material constraints.
2. Run `facet routes list`, then assess the relevant method against supplied
   inputs and current constraints. Read only the matching pack.
3. Explain meaningful quality, time, provider, and cost tradeoffs.
4. Describe uncertain tools, estimate consequential work, execute, inspect the
   actual output, revise defects, and report provenance and limitations.

Do not impose narration, captions, music, fixed turns, or artifact ceremony.
Prefer supplied assets and local operations when they satisfy the brief.

## Safety and honesty

- Paid execution and publication require explicit human consent. An unknown
  estimate is not free.
- `mock: true` is test evidence only, never production media or a fallback.
- A zero exit code is not creative acceptance. Give `output_review` the
  explicit expected profile, duration, codec, pixel format, and audio presence.
  Omitted checks are `assumed`, not verified.
- Report missing dependencies, provider errors, substitutions, and incomplete
  verification directly.

## Tool use

```sh
facet routes list
facet routes describe explainer
facet routes assess --input assessment.json
facet tools list
facet tools describe media_probe
facet tools estimate video_compose --input request.json
facet tools run video_compose --input request.json
facet tools run output_review --input review.json
```

Route assessment is advisory and stateless. It reports missing inputs, live
operations, dependencies, network use, and charge effects; it never executes,
stores state, or selects a provider. Feasibility requires existing file inputs,
canonical requests, entry estimate validation, declared consumption of every
non-informational input (including nested/array fields), and constructible bindings.
Use JSON request files and the live registry. `media_probe` accepts `input` or
`input_path`; provider generation must use the specifically named Facet tool.
For Remotion, use nonempty `cuts` with flat scene fields and explicit timing.
The explainer pack documents scene types and narrated timing. Estimates do not
prove credentials, runtime availability, rendering success, or quality.

Project files are durable records when needed, not mandatory workflow stages.
