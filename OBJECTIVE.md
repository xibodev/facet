# Facet Objective

## What This App Must Be

Facet is a standalone local-first video-production application. It owns the
video domain: projects, source media, scripts, narration, editing, composition,
renderers, creative guidance, artifacts, provenance, technical review, and a
video-focused user interface.

Standalone Facet must embed the complete supported Facet Studio kernel for
conversation, providers, authentication, models, tool dispatch, sessions,
events, cancellation, and approvals. Facet must register its video tools
natively and must not use an external agentic CLI or its own detached module as
the standalone brain.

The same domain capability must also ship as:

- a Facet Agent Bundle for a named agentic CLI, with GitHub Copilot acceptable
  as the first supported target; and
- an installable Facet capability module for full Facet Studio.

## Required Order

1. Inspect the current code and tests to recover the real video-domain surface.
2. Keep one canonical operation, tool, effect, requirement, and artifact truth
   across all delivery forms.
3. Make the agent bundle and Studio module independently buildable and testable.
4. Wait for Facet Studio to publish a supported tagged kernel contract.
5. Pin that release without a sibling `replace`, embed it, and migrate the
   standalone UI to the native runtime.
6. Verify clean installs and real video journeys for all three forms before
   release.

## Start Clean

1. Read only this file for intent. Do not reconstruct plans from deleted
   documentation.
2. Run `git status` and preserve all existing work unless the user explicitly
   asks to replace it.
3. Inspect `internal/toolbox`, application entry points, the current workbench,
   bundle/module code, `go.mod`, build scripts, and tests.
4. Derive tool counts, supported packs, effects, dependencies, and release state
   from executable code and tests. Do not trust comments or generated fixtures
   without checking them.
5. Run focused baseline checks before editing and verify actual media output,
   not only process success or file size.
6. Do not create new Markdown plans, status logs, prompts, handoffs, skills, or
   architecture essays. Put durable contracts in code and tests. Use Git history
   only when provenance is necessary.

## Boundaries

- Facet owns video-service adapters; it does not own conversational provider or
  model management.
- Do not build a second agent loop, session engine, planner, workflow runtime,
  hook system, or approval engine.
- Pipelines are guidance for the active agent, not a Go workflow state machine.
- The Studio module contains domain capability, not the standalone UI or an
  embedded kernel.
- The Agent Bundle relies on its target CLI for the generic runtime.
- Standalone completion is blocked until the Studio kernel is tagged, supported,
  pinned, and proven from a clean clone.
- Do not commit, tag, push, publish, or release without explicit user approval.
