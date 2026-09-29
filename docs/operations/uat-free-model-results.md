# Credential-Free Docker UAT Investigation

## Scope

Two bounded real runs on 2026-09-06 used OpenCode 1.18.29 with
`opencode/big-pickle`, no injected credentials, disposable container HOME, and
open runtime egress. This is a hosted service, not an offline model. No application
source was edited by the operator's UAT implementation session. Existing dirty
application changes were included in detached source and left untouched. No
commits, pushes, fake CLI or replacement event streams were used.

Reproduce from the repository root with an external evidence directory:

```powershell
$evidence = Join-Path ([System.IO.Path]::GetTempPath()) 'opencode/facet-uat-free'
node scripts/uat-run.mjs $evidence --free-model
```

## Actual Results

### Latest Full Run After Proof Reconciliation

Run `facet-uat-1788678491929` exercised the corrected parser with a normal
title-card prompt. Installation passed all seven checks, including rendering
through the installed Facet command. Studio smoke passed. The first agent turn
ended successfully and all 27 recorded model steps reported cost zero.

The required production gate still failed: the agent generated the final video
directly with FFmpeg, using Facet for discovery and inspection but not rendering.
There was no successful Facet rendering call for the parser to accept. The
diagnostic candidate fully decoded (3 seconds, 320x180, 24fps, H.264/AAC, mean
audio volume -15.7 dB), but is not an accepted production. Revision, playback and
download were not reached. The sealed verdict remains UNPROVEN, exit 2, with
COMPLETE integrity in development mode. No renderer timeout reproduced in this
run. All run-owned containers were removed.

This establishes credential-free model access, not reliable end-to-end agent
adherence. The next product investigation is why the agent bypasses Facet's
rendering contract; do not count direct FFmpeg output as proof of that contract.

### Earlier Runs

| Run ID | Core Verdict | Installation | Browser | Smoke |
| --- | --- | --- | --- | --- |
| facet-uat-1788675754452 | UNPROVEN, COMPLETE integrity, exit 2 | 6/7 checks passed | Turn 1 timed out at 600000 ms | PASS |
| facet-uat-1788677076660 | UNPROVEN, COMPLETE integrity, exit 2 | 6/7 checks passed | Turn 1 ended successfully but failed rendering proof | PASS |

Both installation and browser custom-probe processes exited 1. Dirty development
mode does not turn those failures into success. The effective-network evidence
reports `internal: false` in both runs. Certification is explicitly forbidden.

Each run's sealed evidence is under `$evidence/runs/<run-id>/evidence`.
Read `browser/result.json`, `browser/transcript.json`, `installation/checks.json`,
`installation/facet-compose.log` and `docker-uat.json`, plus sibling `verdict.json`.
Do not modify those sealed records. Prompts, sanitized SSE transcripts and trace
archives were exported; raw private traces were not. Neither run reached revision
submission or the accepted-turn playback/download checks.

## Root Causes And Limits

1. **Harness event assumption, corrected after these runs.** OpenCode emitted completed tool records
   (`raw.type=tool_use`, `part.state.status=completed`) containing command, call ID,
   output and exit status. Studio's `internal/studio/engine/opencode.go` maps these
   to one normalized `tool_result` (the completed-state branch), not a separate
   use/result pair. The second run recorded 50 normalized results and zero uses.
   The old proof parser incorrectly required a preceding normalized use, producing
   `No correlated successful Facet rendering call`. This was a harness mismatch,
   not an application adapter defect. At the user's direction, the parser now
   accepts the self-contained completed shape with rigorous raw/normalized identity,
   input/output, status and zero-exit checks. No events are invented, and duplicate
   or inconsistent records remain rejected. A verbatim sanitized completed-call
   fixture from the second run is retained in `scripts/uat-proof-fixtures.mjs`.
2. **Harness command/path restriction, corrected after these runs.** The second run rendered with
   `cd <project> && facet tools run video_compose --input artifacts/explainer_props.json`.
   The safe exact-project `cd` prefix and an absolute output naming that project's
   final file were previously rejected unnecessarily. Both are now supported with
   exact project binding; other shell compounds remain rejected. An explicit
   workdir must equal that project. The browser no longer appends shell-syntax
   instructions to normal user prompts. Media and revision assertions are unchanged.
3. **Installed compose timeout, repeated but underlying cause unresolved.** Both
   runs' `installed-facet-video-compose-with-audio` checks received actual Facet
   JSON `ok:false`, `error.code=command_timeout`, `node was cancelled or timed out`,
   with empty stderr, at the fixture's 180-second render limit. The direct Remotion
   fixture passed. These paths differ: the direct probe renders 30 frames with
   `--concurrency=1`; the installed Facet path renders the full two-second 1080p
   composition and uses Remotion's default concurrency. This suggests a runtime
   performance/concurrency investigation, but does not establish its cause. No
   longer timeout, smaller fixture, bypass renderer or weaker assertion was used
   to conceal the failure. Agent rendering later succeeded on other compositions.
4. **Agent discovery friction, observed.** The first turn spent most of its bound
   exploring tools, bundled files and source before successfully producing an
   intermediate render. `video_compose`'s advertised schema omits the direct
   top-level `cuts`/composition controls accepted in `internal/toolbox/compose.go`.
   This is an application discoverability issue, not missing model authentication.
   The second run authored a TitleCard in its disposable installed Remotion bundle;
   its candidate is not proof that a pristine built-in composition met the request.

The second run's actual failed candidate is retained as
`browser/failed-candidate.mp4`, explicitly `accepted:false`. It fully decodes,
contains 72 H.264 frames at 320x180/24fps and AAC audio, lasts 3.000 seconds, and has
decoded mean volume -27.1 dB (threshold -45 dB). SHA-256:
`7cf8f8ebfd470c9c617bf19afbbec0e102b6f2e2cb54ad09fcf4b0e3a004b935`.
These diagnostic facts do not bypass rendering proof, revision, playback,
download or semantic review. All 34 observed model steps in that run reported
cost 0; this is provider-reported usage, not an independent billing audit.

## Follow-Up Boundary

Real model access is established. End-to-end acceptance is not. The follow-up
proof reconciliation is unit-tested only; it does not amend the sealed historical
verdicts or establish revision/playback success. Renderer-timeout investigation is
owned by a parallel agent. Do not run another full Docker UAT until requested.
No application changes were needed for the proof correction. The two scoped
Docker stacks were removed; no global prune was used.

Read-only replay of the second run's complete sanitized `browser/transcript.json`
finds one successful final rendering call with the corrected parser. The checked-in
fixture was deep-compared with that original event and matches exactly. The local
regression suite plus this external replay passed 19 tests; browser script syntax
validation also passed. This is parser validation against historical evidence,
not another real-agent run or a replacement gate verdict.

Local regression probes:

```powershell
node --test bin/launchers.test.js scripts/uat-media.test.mjs scripts/uat-proof.test.mjs scripts/uat-run.test.mjs
go test -short ./...
```

Healthy local regression results are all tests passing; these synthetic/unit
checks do not constitute real browser acceptance or release certification.
