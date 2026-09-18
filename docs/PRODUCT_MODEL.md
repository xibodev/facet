# Facet Product Model

**Status: authoritative.** This covers **what Facet is and is not**.

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
| reasoning / planning | production guidance and capability descriptions |
| tool calling | stateless media operations |
| skills / agent loading | operation requirements |
| permissions and hooks | effects: `may_charge`, network, external writes |
| context and session handling | media verification and artifacts |
| the model powering the conversation | production workspace knowledge |

**Never build a second** planner, hook system, approval engine, agent loop, or
workflow runtime when the driver already supplies one.

### One clarification that has caused confusion

**"Operation requirements" means dependencies Facet's tools need** — image,
video, voice, stock media, and rendering runtimes. Facet reports those
requirements and effects but does not select a provider on the user's behalf.
The model powering the conversation is the driver's concern and Facet never
configures, proxies, or bills it.

## Packs are stateless guidance

Retained packs provide Facet-owned, method-specific guidance. They do not
declare required stages, choose providers, or encode an executable workflow.
The reasoning driver decides how to sequence available Facet operations for
the user's request.

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

The canonical contract keeps Facet stateless:

1. **Guidance describes production methods and capabilities.** It does not
   prescribe a required sequence or transfer planning responsibility from the
   reasoning driver.
2. **The subprocess reasoning harness is transitional development tooling.**
   It remains excluded from target releases and is not a product architecture
   to extend.
3. **Domain code remains separate from driver capability.** `projects.go`
   understands media records and evidence; `process.go` supervises development
   subprocesses.
