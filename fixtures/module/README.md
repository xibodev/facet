# Facet module protocol fixtures

These checked-in examples are generated from real module calls or are
hand-authored deterministic inputs verified by tests.

| Fixture | Demonstrates |
| --- | --- |
| `requests/` | Supported synchronous request examples |
| `list-success.json` | Successful local tool catalog invocation |
| `run-local-deterministic.json` | Deterministic local execution and honest effects |
| `estimate-unknown-cost.json` | Unknown estimated cost remains `null` |
| `error-consent-required.json` | Paid work is refused without explicit consent |
| `error-unknown-capability.json` | Unknown capability rejection |
| `error-input-not-found.json` | Toolbox error codes pass through unchanged |
| `error-unknown-field.json` | Unsupported request fields are named explicitly |
| `seed-explainer/` | A portable `xibodev.midden.seed/v1` bundle |

Compatibility-only negative cases are isolated under `legacy/`. They are not
host guidance or supported capability examples.

## Normalization

`list-success.json` has runtime-specific dependency paths replaced with
`<resolved-at-runtime>` and availability normalized to `true`. No fixture
contains an absolute host path.

## Seed fixture

`seed-explainer/manifest.json` has this lowercase SHA-256 digest over its raw
bytes:

```
e4a37c683730c1c6a156013ad9381ae2f33f359ded3a97135cf8239657fb93ce
```

The seed may be loaded by bundle root or manifest path. Its evidence digest is
independent from regenerated brief prose; an empty evidence set is valid.

## Limits

These fixtures exercise protocol shapes. They do not certify rendering,
provider access, authentication, creative acceptance, or paid-provider UAT.
