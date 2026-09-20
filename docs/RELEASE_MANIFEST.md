# Facet Release Manifest

**Status: authoritative.** This document records how the canonical Facet
contract is packaged. Product semantics come from Facet-owned executable
registries and assets; release shapes consume that truth rather than restating
it.

---

## 1. Product contract

Facet extends an existing reasoning driver with creative-production
capability. The driver owns conversation, planning, sequencing, permissions,
and model context. Facet owns:

- stateless media tools and their implementations;
- request and result contracts;
- requirements and execution effects, including `may_charge`;
- technical media inspection and artifact reporting; and
- portable production guidance.

Facet does not ship an autonomous agent loop, durable approval service,
required stage model, or provider chooser. Project files are optional
production records, not Facet-managed state.

## 2. Canonical Facet-owned assets

The current tracked product inventory is intentionally small:

| Asset | Canonical source | Current inventory | Purpose |
|---|---|---:|---|
| Core skill | `skills/facet/SKILL.md` | 1 file | Shared production contract for every reasoning driver |
| Creative boundary | `agents/facet-creative.md` | 1 file | Separates agent judgment from Facet execution |
| Guidance packs | `packs/` | 7 packs, 23 files | Compact method-specific guidance and package metadata |
| Tool schemas | `schemas/tools/` | 6 files | Stateless request contracts for tools that need shipped schemas |
| Public tools | `internal/toolbox/` registry | 33 canonical tools | Live executable capability vocabulary; compatibility aliases are invocation-only |
| Remotion composer | `remotion-composer/src/` | 5 files, 1 composition | Independently authored Facet explainer renderer |

The seven guidance packs are `character-animation`, `cinematic`, `explainer`,
`localization`, `screen-demo`, `social`, and `talking-head`. Packs describe
methods, constraints, and review concerns. They do not prescribe required
stages, hold execution state, or replace the reasoning driver's judgment.

The live tool registry remains authoritative for tool names, descriptions,
request and result schemas, cost estimates, requirements, and availability.
Shipped JSON schemas supplement that registry; they are not a second product
model.

## 3. Stateless tool guarantees

Every release shape preserves the same guarantees:

- `may_charge` is independent of whether a price is known;
- unknown cost is never treated as zero;
- paid work and publication require explicit human consent;
- the agent presents a production proposal and receives approval before
  narration synthesis, asset acquisition, provider generation, or the first
  render;
- approval covers ordinary local revisions, but a provider change, new or
  unknown cost, additional data exposure, identity or rights concern, or
  material quality reduction requires renewed approval;
- requirements retain their strength and resolution may be satisfied,
  unsatisfied, or unknown;
- deterministic behavior is measured rather than assumed;
- media bytes are inspected as media, not validated as JSON;
- mock output is test evidence only, never a production fallback; and
- successful execution is technical evidence, not creative acceptance.

Agents use the same interaction discipline in every shape:

```text
facet tools list
facet tools describe <tool>
facet tools estimate <tool> --input request.json
facet tools run <tool> --input request.json
facet routes list
facet routes describe <method>
facet routes assess --input request.json
```

No release-specific copy of tool names, effects, cost truth, or production
guidance may become authoritative.

## 4. Installed agent bundle: primary product path

The installed agent bundle is the supported product path. A standard install
places the canonical asset bundle at:

```text
$HOME/.facet/bundle
```

On Windows this is `%USERPROFILE%\.facet\bundle`. The canonical production
contract inside that bundle is `skills/facet/SKILL.md`, accompanied by
`agents/facet-creative.md`, the seven packs, the six shipped tool schemas, and
the runtime assets needed by Facet implementations.

Target adapters project those same assets into the native discovery shape for
Claude Code, Codex, GitHub Copilot CLI, and OpenCode. A target-shaped agent
bundle is a readable directory plus `facet-bundle.json`; it records Facet
version, adapter version, compatibility, tool names, entries, and digests.
Different target paths are allowed. Different product semantics are not.

The current tool transport is the `facet` CLI. Adapters must not advertise MCP
or another transport until Facet actually implements it.

## 5. Host projection

`xibodev.module/v2` is the machine-readable host boundary for consumers that
must inspect effects, route approval, issue grants, and check conformance
before invocation. It derives operations and effects from the same canonical
tool registry.

The host boundary is not required by an agentic CLI. An agent can use
`describe`, `estimate`, explicit consent, execution, and verification through
the ordinary Facet CLI. The installed agent bundle therefore has no dependency
on module-v2.

## 6. Standalone projection

Facet Standalone is experimental. Its browser experience must consume the same
embedded Facet tools, core skill, creative boundary, and guidance packs as the
installed agent bundle.

The current external-CLI subprocess adapters remain a development harness.
They are useful for end-to-end testing but are not the supported product path,
and new product architecture must not be built on them. A production
Standalone release must bind Facet capability through the supported embedded
runtime rather than inventing a separate product contract.

## 7. Packaging and verification

Release packaging includes canonical `skills/`, `agents/`, `packs/`,
`schemas/`, implementation runtimes, binaries, license, and notices. The
Remotion runtime ships only the five source paths allowlisted by
`remotion-composer/legacy-composer-manifest.json`. Removed donor surfaces and
workflow-oriented contracts must remain absent.

A release is valid only when:

1. shipped inventories match the tracked canonical sources;
2. target adapters preserve the shared product guarantees;
3. bundle manifests describe every projected file and verify its digest;
4. the running binary exposes the tool vocabulary recorded by the bundle; and
5. rendered artifacts pass content-aware inspection rather than exit-code-only
   checks.
