# Docker UAT

## Boundaries

The image builds the actual Linux Go binaries with `bash install.sh`, installs
the user bundle and locked Remotion dependencies, and installs the published
`opencode-ai@1.18.29` CLI. No agent replacement, generated terminal transcript,
or pre-rendered production output is supplied by this harness.

Two independent result tracks are exported:

- `installation/checks.json`: deterministic installation, strict probe negative
  controls, a direct real Remotion fixture render, and an installed
  `facet tools run video_compose` render with real fixture audio from a fresh
  project outside the source checkout. Both fixture MP4s are NOT autonomous
  productions or real-agent success. A broken runtime pin must fail without output.
- `browser/result.json`: a fresh project created through Studio, OpenCode
  selected, prompt submitted, successful SSE termination, native-session resume
  for a revision, changed rendered bytes, strict media/FPS validation, and playback
  plus byte-matched browser download of EACH turn before the next prompt.
  Missing model opt-in fails this track with `REAL_AGENT_UNCONFIGURED` after
  testing project creation, engine propagation (without corrective selection),
  catalog details and media isolation. Pageerrors fail even without a model.
  Studio starts with an empty root, without a pre-created `projects` directory.
  This is an intentional failure, not a skip/pass.

No existing projects or host agent homes are mounted. Configuration credentials
are never automatically copied. Runtime HOME is disposable `/home/facet`.
The agent may write production files but the test scripts in `/opt/uat` are
root-owned. Do not expose the Docker socket inside the container.

## Default Local Run

Prerequisites: Docker Linux engine/Compose, Node, the project's installed
release-harness, and the Chromium revision required by host Playwright. The
container separately includes Chromium for its detailed browser journey.

From the repository root in PowerShell:

```powershell
$evidence = Join-Path ([System.IO.Path]::GetTempPath()) 'facet-uat'
node scripts/uat-run.mjs $evidence
```

The launcher invokes the installed CLI with `run-local --allow-dirty`, a unique
run ID, external evidence root and zero port offset. It removes credential-like
host variables and Compose/OpenCode overrides from the child environment. This
development run is not release certification. Inspect scenario results even
when core 1.2.0 changes the overall exit to 2 for dirty source.

For a future clean-source certification attempt, explicitly omit development
mode via the launcher flag (either argument order is accepted):

```powershell
node scripts/uat-run.mjs $evidence --certify
```

`--certify` omits `--allow-dirty`; core rejects dirty/untracked source. It does not
approve live model access, waive missing model configuration, or guarantee PASS.
It refuses `UAT_NETWORK_INTERNAL=false` before reading credentials or starting
Docker. The custom probe also inspects the actual Studio network attachments and
refuses certification if any is non-internal. Effective network facts are sealed
in `effective-network.json` and included in `docker-uat.json`.
On POSIX shells use an absolute external path, for example
`node scripts/uat-run.mjs "${TMPDIR:-/tmp}/facet-uat"`.

## Credential-Free Hosted Model

With approval for real outbound model usage, run:

```powershell
node scripts/uat-run.mjs $evidence --free-model
```

This selects the committed nonsecret `.release-harness/docker/opencode-free.json`
in the detached source, mounted read-only at `/run/uat/opencode.json`. Both `model`
and `small_model` are `opencode/big-pickle`, `enabled_providers` is `["opencode"]`,
and the built-in provider uses `{}`. OpenCode 1.18.29 can resolve this built-in
model with model fetching disabled; no custom SDK/model declaration or auth file
is required. No host credentials, HOME, auth stores or Docker socket are mounted.

`--free-model` sets `UAT_REAL_AGENT=1`, `UAT_NETWORK_INTERNAL=false` and a bounded
ten-minute timeout per turn. This is a **free hosted network service**, not a local
or offline model. Availability, latency and free access remain provider-controlled.
It enables open runtime egress, not an endpoint allowlist. `--certify --free-model`
is rejected before filesystem writes or Docker execution, regardless of inherited
environment. A simultaneous `UAT_OPENCODE_CONFIG_FILE` is rejected as ambiguous;
unset it rather than silently substituting a credential-bearing provider. The
default invocation remains offline and does not run a real agent.

## Explicit External Model Opt-In

Prepare a dedicated OpenCode JSON configuration OUTSIDE the repository, with
an explicit `model` and matching `provider` entry. Custom endpoints may require
provider SDK/options and a model declaration; built-in providers may use `{}`.
Use the real OpenCode configuration schema;
do not use a host auth-directory bind mount. The file is mounted read-only at
`/run/uat/opencode.json`, selected with `OPENCODE_CONFIG`. Do not print its contents.

```powershell
$env:UAT_REAL_AGENT = '1'
$env:UAT_OPENCODE_CONFIG_FILE = Join-Path $HOME 'private/uat-opencode.json'
# Only with explicit approval for outbound model access:
$env:UAT_NETWORK_INTERNAL = 'false'
node scripts/uat-run.mjs $evidence
```

`internal: true` is the default Compose runtime network. The false setting permits
outbound access; it is NOT a host allowlist. Use a controlled gateway/firewall
for strict endpoint restriction. Image build downloads packages before runtime
isolation. The browser itself only drives the loopback Studio URL.

Authentication injection is optional and limited to this deliberately selected,
read-only external config, never host HOME/auth-directory or Docker-socket mounts.
Missing opt-in or configuration blocks real-agent acceptance; credentials are not
a prerequisite for a provider that supports credential-free access. An actual CLI failure remains a
failure: the tests never replace it with canned events or an existing MP4.
Real acceptance proves execution, session continuity, media properties and
playback, not artistic quality. Review the captured frames/video for title,
background and semantic adherence before human sign-off.

### Test-Only Forwarding Receipts

The Docker image puts a root-owned executable script named `facet` on PATH at
`/opt/uat/bin/facet`. This is **instrumented REAL Facet execution**, not a fake
Facet fixture or alternate tool. It delegates to the exact installed executable
`/home/facet/.facet/bin/facet` with the original argv, cwd and environment,
inherited stdin, and byte-forwarded stdout/stderr. It does not generate media,
requests, artifacts, success envelopes, or replacement tool results. The ordinary
user prompts, model selection and turn timeouts are unchanged.

Before each prompt the browser opens a unique private receipt directory under
`/home/facet/uat-receipts`, recording a baseline index and wall-clock boundary.
The shim captures stdout and stderr before downstream filtering (including
`facet ... | tail`), up to 2 MiB per stream and 256 directory entries per turn;
collection additionally refuses more than 32 MiB total receipt data per turn.
Calls have UUIDs, start/completion timestamps, cwd, argv, executable SHA256,
real exit/signal, and complete stdout/stderr. A pending file becomes a completed
receipt only after the child closes. Overflow drops diagnostic text rather than
retaining a possibly secret partial prefix. Recorder errors/overflow return 74,
not silent success; child failures otherwise keep their exit. INT/TERM/HUP/QUIT
are forwarded to the direct child; child signal termination is re-raised.
There is no new child timeout or descendant/process-group supervisor.

Receipt acceptance requires the exact executable path and SHA256 matching the
root-owned install-time fingerprint, unchanged binary across execution, complete
capture, permitted final-producing `tools run` argv, exit zero, exact project cwd,
and a whole successful JSON envelope whose final path and SHA256 match independent
media inspection. The latest successful final receipt must match; an earlier hash
cannot stand in for changed media. A later failed producer invalidates prior
success until a subsequent successful final render. Receipt paths come only from
bounded directory enumeration, never a receipt-supplied `source`; symlinks,
pending files, baseline reuse and changed indices are rejected. The browser
rechecks indices after proof and before reporting completion.

The recorder cannot observe the native agent session ID. Supporting evidence must
therefore be an actual current-turn completed OpenCode bash record with consistent
raw/normalized fields and native session, an observed Facet producer command, and
tool timestamps enclosing the receipt timestamps within the browser turn. This
is time/cwd/command association, **not native-session attestation**. Shell text is
supporting context, not executable proof. No receipt path is read from agent text.
Receipt proof does not fall back on invalid receipts; the unchanged strict event
proof below is used only when no receipts were collected.

Receipts are trusted local-testing telemetry, **not a malicious-agent sandbox or
adversarial certification proof**. Root ownership protects the shim, not writable
receipt data or the user-owned installed binary. A same-user adversary can forge
data, change clocks or bypass PATH. Hash/index checks detect observed mutations,
not a forge completed before collection or a change-and-restore attack. Streaming
forwarding adds pipe/backpressure overhead; early-closing consumers may produce
EPIPE, and only direct-child signal propagation is covered. Known configured/env
credentials and credential-shaped fields are redacted in persisted diagnostics;
environment values are not recorded. Live child output is deliberately unchanged,
so a child that itself prints secrets still prints them to its caller. Unknown
secret formats cannot be guaranteed absent. Receipt files stay container-local;
sanitized receipt snapshots are exported inside the browser checkpoint/result.
Redaction that changes proof-critical content fails proof, not a repaired envelope.

Run the local regression gate with `node --test scripts/uat-*.test.mjs`. Forwarding
integration executes a real Node child to compare argv/cwd/env, output and exit;
its receipts are labeled `synthetic-forwarding-fixture` and cannot prove Facet
execution. It does not replace the required later installed Linux/Docker check.
Neither these tests nor receipt telemetry change the deterministic core seal or
establish a release verdict. No paid/model/full-Docker run is part of this repair.

### Strict Event-Only Fallback

Every agent turn must provide structured rendering execution evidence. Supported
forms are a normalized `tool_use`/`tool_result` pair, or OpenCode 1.18.29's
self-contained completed call: raw `type: tool_use`, `part.type: tool`,
`part.state.status: completed`, and `metadata.exit: 0`, normalized as `tool_result`.
For the latter, raw and normalized call ID, both raw session IDs, tool name, input
(including workdir), output and any metadata output must agree. The browser binds
the session to the observed native CLI session. Missing evidence, conflicting
fields, nonzero/missing exit, failure flags, duplicate results or reused completed
call IDs fail closed. A completed record is not a fabricated start event.

Accepted commands are a direct bash `facet tools run <rendering-tool> --input
<request-file>` invocation or one safe `cd <exact-project> &&` prefix. Quoted paths
are supported without shell expansion; `workdir`, if supplied, must equal the
exact project path. Echo, scripts, substitutions, redirects, pipelines and other
compound commands cannot prove execution. Facet stdout must be valid JSON with
`ok: true`, matching tool and `operation: run`. Relative or absolute output must
resolve to the expected project's `renders/final.mp4` to prove delivery. Successful
in-project intermediate renders may precede it but cannot substitute for it;
failed rendering calls still fail the proof. Path matching is lexical; independent
on-disk media inspection remains necessary.

The browser prompts request an ordinary title card and a title/background revision,
without instructions about shell syntax or manufacturing evidence. The harness
observes supported execution shapes rather than dictating command formatting.
Submitted prompts and selected model are recorded in
`browser/result.json`, including on a failed turn; SSE is in `transcript.json`.

Both real-agent videos require decoded audio mean volume at least -45 dB via
FFmpeg `volumedetect`. Revision must change the decoded RGB frame hash at one
second, not just MP4 metadata/container bytes. This is a sampled visual-change
check, not exact-title/OCR or semantic verification; those remain human review.
The sanitized real completed-call fixture in `scripts/uat-proof-fixtures.mjs`
records its source run and transcript lines. Mutations are explicitly synthetic
negative controls, never real UAT evidence. Tests cover successful/rejected event shapes, remux-only visual
identity, changed frames and silent audio; they never count as a user journey.

## Build Pin Limits

Go/Node image version tags and OpenCode/Playwright direct package versions are
explicit, but the base images are NOT digest-pinned, apt repositories/packages
are NOT snapshot-locked, and the UAT tools package has no transitive lockfile.
Only Remotion's install uses its committed lockfile. This is not a completely
reproducible dependency lock. Record the run image content IDs and installed
versions for each run. `installation/install-build.log` captures sanitized
installer build output (Go compilation and npm ci), not every Docker/apt step.

## Compose And Probe Semantics

Core 1.2.0 discovers root `docker-compose.test.yml` before `docker-compose.yml`;
no other filename or override flag is supported. Compose runs in detached source.
Host `127.0.0.1:31000` maps to a credential-free gateway connected to the ingress
and internal networks. It forwards to the Studio namespace TCP relay at
`0.0.0.0:8788`, which forwards to Studio's preserved `127.0.0.1:8787`.
The extra gateway is necessary because Docker Desktop does not publish ports on
an internal-only network. Studio has no ingress-network attachment. The gateway
is only a fixed-destination TCP relay, not an HTTP forward proxy.
Host/Origin and SSE bytes are not
rewritten. Do not publish Studio directly or weaken its same-origin checks.

Core's port block does not allocate ports. This integration deliberately runs
serially on 31000 with offset 0. Health checks and browser URL agree. If manually
changing ports, update `UAT_PORT`, `UAT_APP_URL` and the harness health offset
together; an offset does not rewrite Compose ports or origin URLs.

The custom probe executes `node scripts/uat-probe.mjs` on the HOST, cwd
`<evidence-root>/workspaces/<run-id>/source`. It derives the run/evidence location
from that layout, requires exactly one labeled running Studio container, then
uses `docker exec` for tests and `docker cp` to export results before sealing.
For direct/manual invocation, set both `UAT_RUN_ID` and `UAT_EVIDENCE_PATH`.
The container runs a real Playwright browser in addition to the harness's host
browser smoke. Thus the detailed journey is a project-owned custom-probe test,
not an unsupported extension to the declarative scenario language.

The probe exits nonzero if EITHER track fails. Core's dirty-source mode can turn
that product failure into an overall UNPROVEN/exit 2: inspect `verdict.json`,
`raw-results.json`, `docker-uat.json` and each track's JSON, not just process exit.

## Evidence And Cleanup

### Budgets And Interrupted Runs

Allow **55 minutes** for an external supervisor, never 25 minutes. The launcher
retains its 50-minute hard child bound plus up to one minute of scoped cleanup.
Plan for setup (source enumeration/copy, image build, health, installation track
and its export) to finish within approximately 25 minutes, two hosted agent waits
of up to 10 minutes each, and 5 minutes reserved for checks/export/evaluation.
The setup target is not a separate enforced deadline or a guarantee: a slow
setup consumes the same 50-minute total and can still leave acceptance incomplete.
The installation child is bounded at 10 minutes and the browser child at 22.5
minutes (20 minutes of turns plus checks/finalization). The custom probe's existing
33-minute scenario limit covers both children and export; individual worst-case
bounds are not additive guarantees. External-config turns retain their existing
four-minute default; `--free-model` explicitly selects ten minutes per turn.

The launcher prints timestamped elapsed-time markers every 30 seconds, including
during slow source setup. The probe records `docker-uat.partial.json` with track,
elapsed time, image ID and completed child exits. Installation files are exported
immediately after that child closes, before the browser starts.

The browser writes atomic `progress.json` and `checkpoint.json` every five seconds
and before each turn/check group. The checkpoint includes the report, buffered
transcript and console, sanitized at write time. Content is withheld until session
token capture completes, and during pending token requests; safe progress remains
available even if a request hangs. All partial records say `status: running`,
`final: false`, `adjudication: false`; the checkpoint report cannot claim a pass.
The last checkpoint remains a historical nonfinal snapshot even after completion.

While the browser child runs, the asynchronous host probe polls only these two
safe atomic files every 30 seconds. Exports never overlap: a slow Docker operation
can delay a poll, each Docker call has a 30-second bound, and unchanged files are
not recopied. No repeated media copies, trace processing or per-file existence
processes run in this loop. Final allowlisted artifacts are exported after child
exit and publication of the browser's complete final result, with the last export
awaited before returning control to core sealing. Killing `docker exec` alone does
not prove its container process stopped; without the final marker only atomic
partial JSON is copied, never a potentially unfinished trace or media file.
`trace-private.zip`, staging files and arbitrary agent files are never exported.
Final JSON is published only after pending-token validation and trace sanitation.
Historical sealed evidence is never reopened or amended by checkpoint timers.

An interrupted run's partial files are diagnostics, **not adjudication**, accepted
turns or certification. A supervisor timeout without a completed core verdict and
seal must not be relabeled PASS, FAIL or UNPROVEN. Inspect final `result.json`, child
statuses and the core manifest/verdict before claiming any completed acceptance.

Validate checkpointing locally, without Docker or model calls:

```powershell
node --test bin/launchers.test.js scripts/uat-media.test.mjs scripts/uat-proof.test.mjs scripts/uat-run.test.mjs scripts/uat-checkpoint.test.mjs
```

### Timeout Diagnostic Notes

Each observed SSE event now has `receipt.receivedAt` (UTC) and monotonic
`receipt.elapsedMs` from the Playwright binding receipt, not the model's embedded
timestamp and not a checkpoint write. `turnElapsedMs` starts just before the send
click. `report.telemetry` records the current turn's event count, last receipt,
silence since that receipt, and unmatched tool starts observed in actual events.
An empty `activeTools` means no unmatched start was observed, not that no tool is
running. OpenCode completed-only calls never manufacture starts. `lastToolResult`
references the actual transcript index and retains at most 8 KiB of stdout tail,
sanitized before truncation and withheld while token capture is pending. Full
sanitized output remains in the existing checkpoint/transcript, not raw logs.

A turn-end wait timeout snapshots the configured bound, actual wait elapsed time,
receipt telemetry and a bounded process inventory in `report.timeout_diagnostics`,
written to the existing atomic sanitized checkpoint before screenshots or trace
finalization, and retained in the final result if finalization completes. Safe
progress remains content-free when token capture fails closed. The host also
writes `<track>-runner-diagnostics.json` on a nonzero child exit or runner timeout,
including timeout-vs-exit evidence and a fresh container process inventory, before
final export and return to core sealing. This host fallback does not require a
browser final marker; it contains no transcript, tool input, stdout or credentials.
Existing nonfinal media/trace export restrictions remain unchanged.

The Linux collector reads only bounded `/proc/uptime`, numeric-PID `status` and
`stat` files. It selects same-UID active `node`, `ffmpeg`, `opencode` processes and
their same-UID descendants, omitting zombies. Rows contain only PID, PPID, canonical
name (other descendant names become `other`), process age in ms, cumulative CPU ms,
and RSS bytes. It never reads arguments, environment, auth files or Docker sockets.
This is a user-scoped snapshot, not proof a PID belongs to a specific tool call.
Limits are 4096 directory entries, 512 examined PIDs, 128 output rows, 1 second of
scan time, bounded metadata reads, and a 2.5-second browser collector subprocess
deadline. Host Docker capture retains its existing 30-second command bound.
Transient exits and permission/read failures are counted; unavailable or partial
captures are explicit. No matching rows does not prove there were no processes.

For the next **authorized** first-turn investigation, use a newly prepared isolated
stack containing these scripts and the unchanged selected model/configuration.
No new full run, inference, or paid call is implied by the local checks. For an
already prepared run-owned Studio container, browser-only invocation is:

```powershell
docker exec $container node /opt/uat/uat-browser.mjs /home/facet/uat-evidence/browser
```

Resolve `$container` from that stack's exact run label, never a remembered ID.
Preserve the existing turn timeout and both prompts/assertions. This command stops
on first-turn failure; if the first turn succeeds, it still verifies the revision.
Use the normal host probe when exported/sealed harness evidence is required;
manual browser-only execution does not seal evidence. Inspect first-turn
`timeout_diagnostics`, receipt timestamps, process metrics and sanitized stdout
before deciding on another attempt. No observed events, observed events followed
by silence, an unmatched observed tool start, and a present renderer process are
distinct observations. None alone proves a model stall or establishes acceptance.

Targeted deterministic regression and syntax commands (expect normal exit 0):

```powershell
node --test scripts/uat-checkpoint.test.mjs scripts/uat-proof.test.mjs scripts/uat-run.test.mjs
node --check scripts/uat-browser.mjs
node --check scripts/uat-probe.mjs
node --check scripts/uat-checkpoint.mjs
node --check scripts/uat-checkpoint-process.mjs
node --check scripts/uat-checkpoint.test.mjs
```

Keep `node --test scripts/uat-media.test.mjs` as a separate verification gate.
A wrapper timeout or missing normal exit is a failure, not evidence that media
validation passed. Do not increase its bound or weaken assertions to clear it.

Under `<root>/runs/<run-id>/evidence`, before core sealing:

- `installation/`: check facts, sanitized installer build log, direct renderer
  props/log/fixture MP4 and installed Facet compose props/log/audio-bearing MP4.
- `browser/`: result JSON, sanitized console/transcript, sanitized `trace.zip`,
  screenshots, and `turn-1.mp4`/`turn-2.mp4` only when actual agent renders pass,
  plus each turn's browser download and sampled frame.
- `browser/failed-candidate.mp4`, if a failed journey left an actual final file:
  diagnostic output only, explicitly not an accepted turn. Result JSON records
  available media facts, observed event counts and provider-reported step costs;
  these observations never substitute for the acceptance assertions.
- `docker-uat.json`: run ID, image content ID and both child exit statuses.
- Core adds `probes/RENDER-GATE-001-custom-custom.json`, smoke screenshots,
  `raw-results.json`, `execution.log`, policy snapshot and evidence manifest.

Sibling `verdict.json` and `run.manifest.json` bind the final verdict and source.
No raw container logs or credential configuration are exported. Trace text
resources, URLs, token response bodies and transcript strings are scrubbed before
the raw trace archive is deleted; screenshots never visit credential settings.
Known secret literals, URL-encoded forms and repeatedly JSON-escaped forms are
scrubbed longest-first. Synthetic quote/backslash/newline fixtures test eight
nested JSON serialization levels without reading real credentials.
Treat evidence as potentially sensitive nonetheless, especially agent-authored
text. Do not upload it automatically.

Core always destroys its stack and workspace after evaluation. The launcher
also runs scoped `down -v --remove-orphans` for partial-start failure cleanup.
Do not use a global Docker prune. A separate human inspection stack must use
the same image/source, its own run ID/port, and explicit cleanup.

## Strict Artifact Probe

`scripts/probe-artifact.sh` and `.ps1` invoke the same Node validator. Node,
ffprobe AND ffmpeg are mandatory. It requires a regular file, finite positive
duration within bounds, H.264 video with dimensions, AAC audio, and successful
full decode with `-xerror`. There is no file-size fallback. Deterministic tests
reject absent, arbitrary >1000-byte, truncated, audio-less, wrong-codec and
out-of-range files. These fixtures never enter the real production directory.
