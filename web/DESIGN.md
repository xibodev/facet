# Facet Studio Design System

Facet Standalone is an experimental projection of the canonical Facet contract.
The installed agent bundle is the supported product path. Studio presents real
project files, tool results, and embedded Facet guidance; it does not define a
separate workflow or product contract.

## Direction

Facet Studio is a calm production instrument centered on the real video output.
It uses Apple Human Interface Guidelines as an interaction and visual grammar,
not as marketing imitation. The interface should recede behind the project,
agent activity, production evidence, and media review.

## Visual System

- Use SF Pro system stacks, with compact utility type and clear display hierarchy.
- Build from white, `#f5f5f7`, near-black, and one action blue (`#0071e3`).
- Reserve semantic green, amber, and red for verified states.
- Use hairline separators, restrained shadows, and purposeful 8, 12, 18, and
  28px radii. Primary actions may use capsules or circles.
- Keep the video theater black and visually dominant. Avoid dashboard card soup.
- Support light and dark utility chrome without changing state semantics.

## Product Layout

- Desktop: compact titlebar, navigation rail, central production canvas, right
  inspector, and persistent bottom agent composer.
- Mobile: horizontal navigation, scrollable production canvas, sticky composer,
  and a modal inspector drawer with focus containment and a visible close action.
- Present only artifacts and evidence exposed by the current project.
- Labels may organize the interface, but they are not mandatory workflow
  stages and must not imply that absent files are required.

## State Rules

- Never show Passed, Master, Rendered, Ready, or Connected without direct runtime
  or file evidence.
- No project means no production evidence and no media source.
- Preview and Master are distinct labels. A preview never implies final delivery.
- A dead process immediately becomes Disconnected and offers Restart.
- Preserve the selected agentic CLI's permission model. Paid execution and
  publication always require explicit human consent; unknown cost is not free.

## Interaction Rules

- Minimum interactive target: 44px.
- Keep keyboard focus visible and preserve accessible names when labels collapse.
- Respect reduced motion. Prefer one restrained state transition over decorative
  animation.
- Project, engine, and settings changes must never imply that a running child
  received new configuration.
- Project artifacts and review status are derived from ordinary files, never
  fabricated frontend defaults.
- Facet remains stateless: Studio must not introduce a planner, workflow engine,
  durable approval service, or pipeline state machine.
- Technical success and measurable review results never imply creative or
  human acceptance.
