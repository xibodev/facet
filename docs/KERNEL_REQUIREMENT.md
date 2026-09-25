# Kernel Consumer Requirement — Facet standalone (Release A)

**From:** Facet
**To:** facet-studio
**Status:** satisfied by `github.com/xibodev/facet-studio v1.0.0`.

Facet needs an embeddable reasoning kernel to build its standalone application.
Facet does not need — and does not want — a full Studio installation, a module
host, or a copy of Studio source.

Everything below is **derived from what four real agentic CLIs already had to
supply** for Facet's development harness (`internal/studio/engine`). It is the
measured minimum, not a wish list.

---

## What Facet supplies vs. what the kernel supplies

```
Facet Web UI
    ↓
KERNEL          <- conversation, model invocation, tool dispatch loop
    ↓
Facet agents/skills/packs      <- Facet supplies
Facet native tool bindings     <- Facet supplies
Facet core / implementations   <- Facet supplies
```

Facet owns the creative production chain and all product semantics. The kernel
owns the reasoning loop. **Facet is not asking the kernel to know anything about
video.**

---

## 1. Conversation / session runtime

- Start a session bound to a working directory
- Accept a user turn and run to completion
- Resume an existing session by id
- Report liveness, and terminate cleanly on request

Facet's harness already models this as `session` events carrying
`{alive, dir, engine, id, native_id}`.

## 2. Model / provider invocation

- The kernel owns model selection, credentials, retries and rate limits
- Facet does not want to configure providers per model, and will not proxy them

## 3. Generic tool registration and invocation

**The load-bearing item.** Facet must register its own tools with the kernel and
receive invocation callbacks:

- register a named tool with a description and an input schema
- receive an invocation with the arguments the model chose
- return a result, or a structured error the model can recover from

**In-process is the target.** Standalone Facet must bind natively:

```
Web UI → kernel → Facet native tools → Facet implementations
```

not

```
Web UI → kernel → Facet module-v2 subprocess → Facet
```

Facet must not host itself through its own external module projection. Module-v2
is the boundary for *external* hosts, and using it internally would make Facet a
consumer of its own published contract.

## 4. Skills / agent asset loading

- Load agent instructions, skills and guidance from a caller-supplied location
- Facet projects its canonical assets into whatever layout the kernel expects —
  the same target-adapter rule Facet already applies for Claude, Copilot, Codex
  and OpenCode

The kernel does not need to understand Facet's packs; it needs a defined place
to read assets from.

## 5. Streaming / events

Facet's UI needs an incremental stream. The event model its harness already
normalizes across four CLIs:

| Event | Carries |
|---|---|
| `session` | id, native id, liveness, working dir |
| `text_delta` | assistant output, incrementally |
| `think_delta` | reasoning, where a driver exposes it |
| `tool_use` | tool name, id, input |
| `tool_result` | tool id, output, `is_error` |
| `error` | failure the user must see |
| `end` | turn complete |

Optional but valuable, because Facet reports cost honestly: `cost_usd`,
`duration_ms`, `tokens`.

**A distinct terminal event matters.** Facet has already shipped a defect where
a successful exit was mistaken for a successful result; a stream that simply
stops is indistinguishable from one that failed silently.

## 6. Approval / execution hooks

Facet gates paid work: 8 of its 33 canonical tools may charge, and consent is required
before execution. The kernel must let Facet **interpose before a tool runs** —
inspect the pending call, and refuse or defer it pending human approval.

Without this hook, Facet's consent semantics cannot be honoured in standalone,
and that is a product guarantee rather than a preference.

## 7. Embedding and version compatibility

- A **consumable, versioned artifact** — Go module, published binary, or library
  Facet can pin. Not a source tree to copy.
- A stated compatibility contract, so Facet can pin a supported version and
  state which it ships
- **No dependency on a full Studio install or the module host**

---

## What Facet is NOT asking for

- Not a video, media or artifact model — that is Facet's
- Not Facet's tool vocabulary, Operations, effects or cost semantics
- Not project management, packs or creative guidance
- Not the Studio UI

---

## Open question for facet-studio

**Does a kernel in this shape already exist as a separable artifact, or would it
have to be extracted?**

Resolved: the released `pkg/agent` kernel is the versioned embeddable artifact.
Facet pins it as a Go dependency and registers the `facet-native` provider
through `agent.ToolProvider`. Facet does not launch the separately packaged
`facet-studio-kernel` executable and does not route itself through module-v2.

The installed Studio-target Facet bundle is the activation boundary. It
declares `xibodev.facet`, native transport, the provider binding, the compatible
v1 kernel API, the exact tool vocabulary, entry digests, and bundle digest.
Facet.UI verifies that contract before constructing an agent runtime. Guidance
and pack reads then come from the verified bundle; a missing, altered, stale, or
incompatible bundle leaves the generic kernel without Facet capability.
