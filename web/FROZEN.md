# Frozen: Facet Studio (standalone UI)

**Status: frozen. Working, supported, not under development.**

`facet ui`, `web/`, `internal/studio/` and `cmd/facet-ui` still build, still run,
and their tests still pass. Nothing here has been removed and nothing is
deprecated for users who rely on it today.

## What frozen means

- Bug fixes that keep existing behaviour working: yes.
- Security fixes: yes.
- New viewers, new panels, new UI features: no.
- Changes required by the module protocol: no — those belong in
  `internal/module/`, which is independent of this package.

## Why

Facet's role is a headless creative toolbox: a CLI a person can drive directly,
and a module surface (`facet module describe` / `facet module invoke`) that an
agentic host invokes. Presentation of Facet's artefacts is moving to the host,
which owns the rendering primitives and composes module-declared views against
them.

That makes this UI donor surface. It is the reference for what artefact viewers
Facet's output actually needs, and the requirement it establishes is small:
every viewer here is a plain HTML element chosen by media type — `<video>` with
native controls, `<img>` in a grid, `<audio controls>`, and `<pre>` for JSON.
There is no `<canvas>` anywhere in `web/index.html`, and no frame-stepping,
side-by-side comparison, or timeline editing. A host does not need module-
supplied view code to render any of it.

The one requirement that is not a single element: QA review frames are shown as
a GRID. Sampled frames are close to useless one at a time; scanning several
together is how a person catches a bad cut or unreadable text.

## If you are looking for the module surface

`internal/module/` implements the host protocol and shares nothing with this
package. Freezing the UI does not affect it.
