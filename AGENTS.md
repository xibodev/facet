# Video Kit Agent Contract

Read these files, in order, before doing any work:

1. `DESIGN.md`
2. `PLAN.md`
3. `DONORS.md`

`DESIGN.md` is the product and architecture authority. `PLAN.md` controls the
current phase. `DONORS.md` controls provenance and what may be copied or ported.

## Current Stop

This repository is at Phase 0: Architecture Lock. During architecture review,
editing `AGENTS.md`, `DESIGN.md`, `PLAN.md`, and `DONORS.md` is allowed. Keep the
repository documentation-only and stop before donor inventory or implementation.

Do not inspect donor implementation, copy, port, generate, or salvage
implementation code until the user explicitly approves the product, production,
toolbox, and donor architecture. After approval, Phase 1 may inspect only the
donors allowed by `DONORS.md`. Clean upstream OpenMontage at the pinned commit is
the only source for upstream behavior; never infer it from another checkout.

## Drift Rules

Video Kit is a Claude-first, headless agent bundle plus a Go toolbox. It is not
a host application, workflow engine, or rewrite of an agentic CLI.

Do not add any of the following during the headless phase:

- UI, server, daemon, database, queue, scheduler, or background service;
- Facet host, Production Kernel, Backlot host, MCP bridge, or capability packs;
- cross-CLI abstraction, universal agent runtime, or compatibility SDK;
- host-owned stage progression, retries, approval state, or session state;
- pipeline promotion statuses that prevent a shipped pipeline from being used;
- local GPU model installation or execution;
- packaging, installers, or operating-system portability work;
- speculative security, scalability, or multi-user infrastructure.

Future CLI, packaging, UI, local GPU, host, portability, and scale concerns must
not shape the initial headless implementation unless the user explicitly changes
the approved scope.

Every implementation task after Phase 0 must satisfy at least one condition:

1. Port a named capability from the pinned upstream OpenMontage donor.
2. Salvage a named and proven mechanic from the current video-agent-bundle.
3. Fix a defect observed while producing a real video.
4. Implement a requirement explicitly approved in `DESIGN.md` or `PLAN.md`.

If none applies, do not build it.

## Donor Boundaries

- Clean upstream OpenMontage is the primary behavioral and tool donor.
- The current video-agent-bundle is a selective salvage donor only.
- The Facet-modified checkout is quarantined and must not be read or copied
  during headless implementation unless the user explicitly requests one named
  artifact.
- Never infer upstream OpenMontage behavior from Facet-modified code.

## Working Method

- Claude Code is the only reference agentic CLI until the headless product
  works end to end.
- Trust Claude's existing abilities to read files, reason, use the shell, use
  browser/web tools, invoke binaries, diagnose errors, and resume sessions.
- Keep the Go layer mechanical and stateless. It must not choose stories,
  pipelines, providers, stages, or creative outcomes. It may expose transparent
  provider facts, eligibility, estimates, and ranking signals for Claude.
- Preserve ordinary project files as the production record.
- Treat provider credentials and runtime dependencies as configuration facts,
  not as pipeline implementation or maturity statuses.
- Preserve useful upstream behavior, prioritize real-video production, and
  expand parity incrementally according to the current phase and production
  needs.
- Keep changes minimal. Do not invent an abstraction before multiple ported
  tools demonstrate that it is needed.
- Treat personas as Claude reasoning postures, not processes, subagents,
  services, or state owners. Claude Code is the sole active orchestrator.
