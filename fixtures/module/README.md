# Facet module protocol fixtures

Generated from real runs of `facet module ...` at the commit noted below. These
are the shapes a host can code against; they are not hand-written.

Regenerate with the commands in each entry below.

| Fixture | Command | Demonstrates |
| --- | --- | --- |
| `describe.json` | `facet module describe --json` | The twelve descriptor fields, six capabilities, live per-tool schemas |
| `list-success.json` | `module invoke creative.tools.list` | Successful local invocation, tool catalog shape |
| `run-local-deterministic.json` | `module invoke creative.tools.run` (media_probe) | Deterministic local run, honest `local=true network=false` execution |
| `estimate-unknown-cost.json` | `module estimate creative.tools.estimate` (gflow_image) | `estimated_cost: null` — unknown cost is never reported as zero |
| `error-consent-required.json` | `module invoke creative.tools.run` (gflow_image, no consent) | The consent gate refusing a paid tool |
| `error-unknown-capability.json` | `module invoke creative.bogus` | Unknown capability rejection |
| `error-input-not-found.json` | `module invoke creative.tools.run` (missing file) | Toolbox error code passed through unchanged |
| `seed-explainer/` | hand-authored synthetic | A `xibodev.midden.seed/v1` bundle Facet can consume |

## Normalization

`list-success.json` had its dependency `path` values replaced with
`<resolved-at-runtime>` and `available` forced to `true`. Those fields are
properties of the machine that generated the fixture, not of the contract. Every
other fixture is byte-for-byte as emitted.

No fixture contains an absolute host path.

## Seed fixture

`seed-explainer/` is a DIRECTORY bundle, matching Midden's contract:

```
seed-explainer/
  manifest.json     entry file — the shape Facet reads
  brief.md          prose for the host agent
  evidence.jsonl    the evidence set (empty here; a valid state)
  provenance.json   source identities
  attachments/      referenced files
```

`manifest.json` — sha256, hex, lowercase, over the raw file bytes:

```
e4a37c683730c1c6a156013ad9381ae2f33f359ded3a97135cf8239657fb93ce
```

`LoadSeed` accepts a path to either the bundle root or `manifest.json`, and
accepts the digest with or without a `sha256:` prefix.

The manifest carries `evidence_digest` separately: evidence identity stays
stable when `brief.md` prose is regenerated. An empty `evidence.jsonl` digests
to `e3b0c442...b855` (sha256 of zero bytes) and is a valid seed, not an error.

## What these fixtures do not prove

They exercise the protocol surface only. They do not prove a renderer works, do
not prove a provider is reachable or authenticated, and do not constitute
creative acceptance of any output. No paid provider was called to produce any
fixture here.
