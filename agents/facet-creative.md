# Facet Creative Overlay

Load this when a turn involves creative production. It is guidance for the
host's agent, not a description of the toolbox API — the toolbox describes
itself through `facet module describe --json`.

## Division of labour

You own creative orchestration. Facet's toolbox is mechanical and stateless: it
executes what you ask and reports what happened. It will not infer intent,
choose a story, or decide that output is good enough. Those are yours.

## Project files are production truth

Scripts, scene plans, narration, renders, and review reports on disk are the
durable record. Chat history is not. When resuming work, read the files rather
than trusting recollection.

## Estimate before spending

Call `creative.tools.estimate` before any provider-backed or paid execution. An
estimate validates the concrete request and never bills, never generates, and
never writes output.

**Unknown cost is not free.** When `estimated_cost` is `null`, the price is
genuinely unknown — that is not permission to proceed cheaply. Paid tools refuse
to run until a human has explicitly approved the spend. Agreement from another
agent is not consent; only a person can grant it.

## Mock output is test-only

A result carrying `mock: true` is a contract test artifact. It is never a
production asset, never a substitute for a failed provider call, and must never
be presented as generated content. If a provider fails, report the failure.

## Command success is not creative acceptance

A zero exit code means the process ran. It does not mean the video is right.
Inspect the actual output: watch it, listen to it, check caption timing, factual
claims, pacing, and legibility.

Technical QA (`creative.output.review`) and human approval are separate states.
The artifact manifest records them separately for that reason, and
`human_approved` starts false. Never set it on a person's behalf.

## Disclose honestly

Report failures, substitutions, timing differences, and anything you could not
verify. An unverified claim presented as fact is worse than an admitted gap.

## Provenance travels with output

When a Midden seed produced the work, the artifact manifest retains its schema,
path, and digest, so a finished asset can be traced to the exact bytes behind
it. Facet consumes a seed by path and digest only — it never calls Midden and
never reads Midden's storage.
