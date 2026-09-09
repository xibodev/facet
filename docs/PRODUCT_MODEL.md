# Facet Product Model

**Status: authoritative.** Companion to `RELEASE_MANIFEST.md`, which covers how
Facet is packaged. This covers **what Facet is and is not**.

---

## The core rule

> **Facet extends an existing agentic runtime with creative-production
> capability.**

Facet is not an agent. It does not reason, plan, converse, or drive a tool loop.
It supplies the domain knowledge and the tools that a reasoning driver uses.

## The split

| The reasoning driver owns | Facet owns |
|---|---|
| conversation | media tools and implementations |
| reasoning / planning | creative skills, agents, packs |
| tool calling | pipeline definitions |
| skills / agent loading | provider requirements |
| permissions and hooks | effects: `may_charge`, network, external writes |
| context and session handling | media verification and artifacts |
| the model powering the conversation | production workspace knowledge |

**Never build a second** planner, hook system, approval engine, agent loop, or
workflow runtime when the driver already supplies one.

### One clarification that has caused confusion

**"Provider requirements" means providers Facet's TOOLS need** — image, video,
voice, stock media, rendering runtimes. It does **not** mean the model powering
the conversation. That model is the driver's concern and Facet never configures,
proxies or bills it.

## Pipelines are instructions, not a runtime

The 16 pipeline and style YAML files under `packs/` are **instructions to the
reasoning driver**. The driver reads them and executes the sequence using Facet
tools.

**Verified:** nothing in Go parses or executes a pipeline. The two `yaml.Unmarshal`
call sites (`internal/config/config.go`, `internal/toolbox/compose.go`) both read
configuration, not pipelines. There is no second workflow runtime to retire.

---

## Audit: domain work vs. recreated driver capability

Every package measured against one question:

> Is this doing domain work Facet uniquely knows, or recreating something the
> reasoning driver already provides?

| Package | LOC | Verdict |
|---|---|---|
| `internal/toolbox` | 10331 | **DOMAIN.** 35 tools, Operations, effects, provider integrations, media implementations. The product. |
| `internal/module` | 3492 | **DOMAIN.** The v2 projection — how Facet declares its semantics to a host. |
| `internal/studio/projects.go` | 1211 | **DOMAIN.** Production workspace knowledge: renders, narration, artifact stages, evidence scanning. |
| `internal/studio/server.go` | 1011 | **MIXED.** Static UI serving and media endpoints are domain; session token plumbing exists only to drive subprocess CLIs. |
| `internal/studio/catalog.go` | 489 | **DOMAIN.** Project registry. |
| `internal/config` | 1390 | **DOMAIN.** Dependency detection, doctor, target projection. |
| `internal/bundle` | 440 | **DOMAIN.** Release C packaging. |
| `internal/studio/process.go` | 644 | **DRIVER CAPABILITY.** Session lifecycle, turn acquisition, process-tree reaping, stdout draining. |
| `internal/studio/engine/` | 1967 | **DRIVER CAPABILITY.** Adapters normalizing four external CLIs into one event model. |
| `internal/studio/process_tree_*` | 220 | **DRIVER CAPABILITY.** Win32 job objects and POSIX process groups to kill a subprocess tree. |

**~2,800 lines are driver capability.** Every one exists to spawn and supervise
an external agentic CLI as a reasoning runtime — the shape the product model
says standalone must not use.

### Disposition — bypass, do not delete

That code is **not deleted and not rewritten**:

- For **Release C** it is not shipped at all. The target CLI *is* the driver, and
  it supervises itself.
- For **Release A** it is superseded by the bundled Studio kernel. Standalone
  binds Facet tools natively and never spawns an external CLI.
- Until the kernel is consumable it remains the **development harness** — already
  classified in `internal/studio/engine`'s package doc — and it is the only way
  to exercise intent → tools → artifact end to end today.

**The rule: no new product architecture is built on it.** When the kernel lands,
the browser experience moves onto the kernel and these adapters survive only as
development tooling.

## What this re-orientation changed

Nothing was rewritten. The audit found:

1. **No duplicate planner, hook system, approval engine or workflow runtime
   exists.** Pipelines are already instructions; the driver already executes them.
2. **The one real duplication is the subprocess reasoning harness**, already
   classified as transitional and already excluded from both target releases.
3. **The domain/driver split is clean** everywhere else — `projects.go` knows
   about renders and narration; `process.go` knows about job objects. They are
   not entangled.

The correct action was to **confirm and record**, not to restart architecture
work.
