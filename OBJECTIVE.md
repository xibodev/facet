# Facet Objective

Facet is an agent-first video-production toolbox. The user's selected agentic
CLI owns intent, creative decisions, sequencing, and communication. Facet owns
stateless media operations, request validation, capability discovery,
technical review, and portable guidance.

## Canonical product path

The installed agent bundle is the primary product. The native Go embedding,
target-shaped bundles, the capability module, and experimental Standalone must
consume the same Facet-owned assets:

- `skills/facet/SKILL.md` for the shared production contract;
- `agents/facet-creative.md` for the creative responsibility boundary;
- retained pack entry skills for method-specific guidance; and
- live schemas and tool descriptions for mechanical request contracts.

No delivery form may maintain a competing copy of product guidance.

## Boundaries

- Keep Facet mechanical and stateless. Do not add an agent loop, planner,
  workflow engine, durable approval service, or pipeline state machine.
- Project files are optional production records, not required stages.
- Paid execution and publication require explicit human consent. Unknown cost
  is not free.
- Mock media is test-only and never a production fallback.
- Tool success is technical evidence, not creative or human acceptance.
- Optional providers and runtimes must report missing dependencies and access
  honestly.
- Standalone is experimental and must use the same embedded capability rather
  than inventing a separate contract.

## Engineering rule

Derive discovery and shipped content from executable registries and canonical
assets. Tests must fail when donor terminology, removed legacy surfaces,
dangling pack references, or target-specific semantic drift returns.
