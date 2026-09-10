# Facet Release Manifest

**Status: authoritative.** This is the source-of-truth model for how one Facet
codebase projects into three product shapes. It supersedes any shape inferred
from the current web UI.

---

## 1. Product definition

Facet is a **harness-agnostic creative-production product** that transforms
creative intent into verified media artifacts through reusable Operations,
tools, implementations, and domain intelligence.

Facet owns the production chain:

```
source → generate → edit → compose → inspect/QA → select → deliver
```

**Facet does NOT own a generic conversational reasoning loop.** Conversation is
supplied by the target driver. Facet supplies the creative intelligence loaded
into that driver.

---

## 2. Canonical product truth

One-way projection. Truth lives here; every release shape reads it.

```
Operation / product truth
          ↓
tool / module / target projections
```

| Canonical asset | Location | What it is |
|---|---|---|
| Operations + effects | `internal/toolbox/toolbox.go`, `v2ops.go` | `MayCharge`, `Deterministic`, `executionFor`, requirements, Resolution |
| Artifact kinds | `internal/toolbox/v2artifacts.go` | document / text / media, honest validators |
| 35 public tools | `internal/toolbox/` | the stable public vocabulary |
| Skills | `skills/` (160 files) | canonical creative guidance |
| Packs | `packs/` (112 files) | production styles |
| Authoring schemas | `schemas/` (34 files) | what an agent authors — NOT emitted artifacts |

**Invariants that survive every release shape:**

- `may_charge` is independent of `cost_known`; neither is derived from the other
- deterministic truth is measured, never assumed
- Requirements carry strength; Resolution stays tri-state (satisfied /
  unsatisfied / **unknown**)
- artifact kinds stay honest — media bytes are never JSON-Schema-validated
- 35 public tool names are stable
- **no release-mode-specific effect/cost/tool truth table may exist**

---

## 3. The three release pipelines

Distinct package structures with different assets. **Not one binary with
runtime flags** — each mode implies a different folder layout, different agent
instruction files, and a different tool-exposure mechanism.

### Release A — standalone Facet application

```
Facet Web UI
    ↓
bundled/pinned facet-studio kernel        <- reasoning driver, IN THE BOX
    ↓
Facet agents/skills/packs
    ↓
native Facet tool bindings                <- in-process, NOT module-v2
    ↓
Facet core / implementations
```

Ships: Facet core, Web UI, creative agents/skills/packs, a pinned supported
Studio kernel, native capability binding, dependency installer UX.

**Does not** spawn Claude Code / Copilot / Codex / OpenCode as its conversational
runtime. **Does not** host Facet through its own module-v2 projection — that
boundary is for external hosts only.

Third-party implementation dependencies remain Facet's concern: ffmpeg,
Remotion, HyperFrames, edge-tts, provider tooling. The installer detects
**declared** requirements, explains them, reuses compatible installs, and helps
install what is missing. Detection validates declared requirements; it never
infers capability from an arbitrary binary on PATH.

**Status: does not exist.** No kernel integration anywhere in the tree.
Blocked on Phase 3.

### Release B — facet-studio module

```
full facet-studio → Studio kernel/host → xibodev.module/v2 → Facet module
```

Ships: Facet core, the v2 descriptor/invoke surface, host-appropriate product
assets, implementation/dependency declarations, honest effects and artifact
semantics. **No standalone UI, no bundled kernel.**

**Status: BUILT AND VERIFIED.** `scripts/build-module.sh` +
`scripts/check-bundle-current.sh`. 35 operations, 5 artifact kinds, 7
capabilities, accepted by the real host with zero findings.

Ships independently of Release A.

#### Why v2 exists here and nowhere else

`xibodev.module/v2` is **Release B's host boundary, not a product-wide
contract.** Releases A and C contain zero references to it, measured.

The distinction is NOT "structured schemas versus prose" — that reading is
wrong and worth stating so it is not repeated. `facet tools describe` already
returns `request_schema`, `result_schema`, `may_charge` and `cost.known` over
plain CLI, so `describe → estimate → consent → run → verify` is Release C's
loop too. **That discipline is a product property, carried by every projection.**

What v2 actually buys is one thing: **a program, rather than an agent, must
decide whether an invocation may proceed — before Facet is asked.**

| Release | Who gates the call | What they need |
|---|---|---|
| A, C | the agent, inline with a human | readable schemas and cost, per call |
| B | a **host**, before invocation | machine-checkable effects it can gate on |

A host routes approval, issues grants, and runs no-weakening conformance
*without executing the tool*. It cannot read help text to do that. An agent can.

**This is why a sibling product with no host has no reason to adopt v2.** The
cost is real — a frozen contract, conformance tests, a projection to maintain —
and with nothing gating it programmatically, the benefit is zero. Not
divergence; different consumers.

### Release C — external agentic-CLI bundle

```
target CLI (Claude Code | Copilot | Codex | OpenCode | future)
    ↓
Facet bundle: agents, skills, instructions, packs, tool wiring
```

The target CLI is the reasoning driver. Facet ships a **target-shaped
installable package** — not a command that mutates a project in place.

**Status: PARTIAL.** `facet init --engine <e>` performs the projection today
but produces no distributable artifact. Gap analysis in §5.

---

## 4. The target-adapter rule

The same rule already used for Operations, applied to product assets:

```
canonical Facet asset / skill / pipeline
              ↓
        target adapter
              ↓
    target-shaped release asset
```

**Different packaging is allowed. Different product semantics are not.**

Four hand-maintained copies of one skill is the failure this rule prevents —
the same class as a second effects table, one layer up.

---

## 5. Inventory: canonical vs. projection

| Concern | Canonical source | Adapter today | Gap |
|---|---|---|---|
| Skill placement | `skills/` | `getSkillsTargetPath(dir, engine, name)` — init.go:447 | none for the 4 known engines |
| Agent instructions | `skills/`, `packs/` | `scaffoldAgentInstructions(dir, engine, packs)` — init.go:278 | none |
| Tool exposure | 35 tools | **NONE** | **no MCP/CLI tool wiring is written for any target** |
| Packaging | — | **NONE** | init mutates a project; it produces no installable artifact |

**The two real gaps for Release C are tool wiring and packaging.** The asset
projection already exists and already follows the adapter rule.

---

## 6. Current web UI — classification

```
browser → process adapter → Claude/Codex/Copilot/OpenCode → Facet
```

**This is a DEVELOPMENT HARNESS, not the standalone product.** It spawns a
user-installed external CLI as its reasoning runtime, which Release A
explicitly must not do.

It is retained because it works and is useful for testing the
intent → tools → artifact chain end to end. It is **not** deleted.

**Rule: do not build new product architecture on top of it.** When the pinned
kernel lands, the browser experience moves onto that kernel and these adapters
survive only as development tooling.

---

## 7. Artifact quality — non-negotiable

> **Verify the output, not the process exit code.**

For media, validate actual content: duration, frame count, resolution, audio
presence, visible content, integrity, digest.

This exists because Facet once shipped four renders that returned `ok:true`
with nonzero bytes on disk — all byte-identical and blank. **`ok:true` plus
nonzero bytes is not success**, in any release shape.

---

## 8. Escalation boundary

Ordinary engineering — folder naming, package layout, build scripts, local
refactors — is decided here, not escalated.

Escalate only when:

- Studio cannot expose a generic kernel suitable for embedding
- canonical Facet semantics cannot project to a target driver without
  **semantic** divergence
- a target requires a genuinely new shared capability or contract

---

## 9. Roadmap state

| Phase | Scope | Status |
|---|---|---|
| 1 | Release architecture + inventory | **this document** |
| 2 | Release C — installable CLI bundles | next; unblocked |
| 3 | Release A — standalone kernel integration | blocked on Studio publishing a versioned minimal kernel |
| 4 | Release B — stays independently shippable | done; keep verifying |
