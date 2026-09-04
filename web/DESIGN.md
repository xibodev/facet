# Facet Studio Design System

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
- The six evidence stages are Brief, Script, Voice, Compose, QA, and Master.

## State Rules

- Never show Passed, Master, Rendered, Ready, or Connected without direct runtime
  or file evidence.
- No project means six neutral Not run stages and no media source.
- Preview and Master are distinct labels. A preview never implies final delivery.
- A dead process immediately becomes Disconnected and offers Restart.
- CLI permission prompts are disabled in autonomous mode. Paid providers,
  publication, and external-account actions still require explicit consent.

## Interaction Rules

- Minimum interactive target: 44px.
- Keep keyboard focus visible and preserve accessible names when labels collapse.
- Respect reduced motion. Prefer one restrained state transition over decorative
  animation.
- Project, engine, and settings changes must preserve or reset session state
  explicitly; never imply that a running child received new configuration.
- Project artifacts and review status are derived from ordinary files, never
  fabricated frontend defaults.
