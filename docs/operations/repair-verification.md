# UAT Repair Verification

## 1.0.2 Candidate Local Gate

The candidate integration gate is local and non-certifying. Reproduce it with:

```powershell
go test -short ./... -count=1 -timeout=6m
go vet ./...
node --test "bin/*.test.js" "scripts/*.test.mjs"
npm --prefix remotion-composer test
pwsh -NoProfile -File scripts/test-install.ps1
node scripts/release-package-smoke.mjs
node scripts/studio-repair-smoke.mjs
```

Healthy results are normal exit 0 for every command. At candidate integration,
the full Go short suite and vet passed; all 88 launcher/UAT fixtures passed.
The checkout's incomplete composer dependencies initially prevented loading
TypeScript. The existing isolated clean install then passed all 13 composer
fixtures and typecheck again, after all 40 source/config/manifest files were
verified byte-identical to this candidate. This preserves the earlier isolated
renderer verification; it does not claim the checkout dependencies were repaired.

Installer preflight and 11 mock-COM shortcut cases passed without user Desktop
changes. npm dry-run content and both native source version checks passed for
1.0.2 (377 package files). The full synthetic Studio smoke passed: 13 recovery
checks, desktop/mobile geometry, playback and matching downloads, no page errors.
Evidence is retained externally in `facet-session-recovery-synthetic-AueekE`,
`facet-studio-repairs-IvV41O` and `facet-composer-clean-20260907` under the operator's
temporary opencode directory. No live model/provider calls were made by this gate.

Only reviewed source, tests, contracts, CI and documentation belong in the commit.
Verify `git check-ignore .quality-run/probe.log` matches the root ignore rule and
`git ls-files -- .quality-run/` returns nothing. Review the complete staged diff,
`git diff --cached --check`, file classes and secret shapes before committing;
retain local logs, caches, binaries and media outside the index. Installed agent
instruction copies are not part of the candidate changes.

Next is an appropriately scoped clean-source gate. Hosted-model open-egress UAT
cannot establish sealed-network certification even from clean source. The prior
dirty-source verdict below remains UNPROVEN, not PASS. No push, main merge, tag or
publication is part of this local candidate gate; see `CHANGELOG.md` for limitations.

## Latest Completed Acceptance

This section supersedes the historical blockers below. Run
`facet-uat-1788819989949` completed in 7m02s after restart recovery, real-CLI
receipt capture, local-media/profile support and small-viewport layout repairs.
Both scenarios passed; all seven installation checks passed. The actual free
OpenCode model completed creation and revision in the same native session, with
successful Facet rendering receipts, advancing browser playback and byte-matched
downloads. Independent sampled visual review confirmed the complete unclipped
titles on the requested blue and green backgrounds.

The deterministic verdict is UNPROVEN, exit 2, COMPLETE integrity, because this
was a dirty-source development run. It is not clean-source release certification.
All 34 sealed files and manifest/verdict bindings independently verified. All
1,659 source files remained unchanged during the run. Scoped cleanup confirmed
no run-owned containers, networks, volumes or detached workspace remained.

External evidence: `facet-final-sealed-20260907-e12c9/RESULT.md`,
`integrity-validation.json`, and `runs/facet-uat-1788819989949/verdict.json`
under the operator's temporary opencode directory. The run's `evidence/browser/`
contains both videos, downloads, sampled frames, receipts and transcript.
Video duration is 3.000 seconds, container duration 3.051 seconds with audio
padding, within the unchanged test tolerance. The agent corrected one invalid
review request within the same run; no gate retry concealed that error.

Receipt capture forwards the real installed Facet executable and retains its
unfiltered result before shell pipelines discard it. This is trusted local-test
telemetry, not malicious-agent-resistant attestation. The viewport regression
also passed 13 metadata/layout tests and the installed-image TypeScript check;
six default-1080p frames were unchanged. Portrait layouts use containment and
letterboxing rather than an unverified responsive redesign.

Remaining release work: review and explicitly approve the intended commit set,
then establish an appropriately scoped clean-source release verdict. The live
gflow wrapper still has replay verification rather than a new post-fix paid
generation; the separately verified exact-30-second forest composition uses the
previously generated genuine clip and Edge TTS narration. No release, commit,
tag, push or publication was performed in this verification.

## Current Status

### Latest targeted remediation

Matched-input Docker probes did not reproduce the sealed installed-render
timeout: installed Facet completed in 38.125 seconds and direct Remotion in
27.689 seconds with equivalent inputs/runtime controls. The demonstrated defect
was lost timeout diagnostics; the command runner now retains bounded recognized
renderer progress, with other output redacted. No rendering timeout or default
concurrency was increased. Evidence: `facet-compose-targeted-20260907/RESULT.md`
under the external temporary opencode directory.

The sealed agent transcript demonstrated more than four minutes of legacy
guidance exploration before its render announcement, followed by no observed
events. It does not prove a model outage. Generated guidance now points to the
engine-local core skill and gives core/pack entry guidance precedence over
need-specific legacy references. Seven-pack instructions remain under 50 lines;
contract tests passed. Timeout evidence now records receipt timing and a bounded,
sanitized process inventory without command lines, environment or credentials.
Checkpoint/proof/runner regression tests passed (32 checks).

The updated image was built, but Docker control commands subsequently stalled
before the targeted browser test started. Scoped cleanup was eventually verified
for that build attempt (`facet-targeted-1788809666667/RESULT.md`). A follow-up
using the retained image stopped at a single timed-out image inspection and
created no container (`facet-focused-1788811015265-e7692e/RESULT.md`). No model
turn ran in either attempt. These are environmental/setup failures, not passing
evidence for the updated guidance or a new deterministic verdict. Retain the
image/source fingerprint for the next targeted test instead of rebuilding.

The latest budget-corrected Docker run completed adjudication. Docker availability,
scoped cleanup and missing timeout evidence are no longer the current blockers.
Run `facet-uat-1788801139352` returned sealed UNPROVEN, exit 2, with COMPLETE
integrity against dirty source. Studio smoke passed; the production scenario
failed. Six of seven installation checks passed. The installed Facet composition
request hit its 180-second Node timeout; the direct Remotion fixture passed.
The real OpenCode first turn hit its separate 600-second limit without an end
event or completed render. Revision, playback and download were not reached.

The runner completed in 20m31s, within its 50-minute bound and the independent
55-minute supervisor bound. Sanitized checkpoints, transcript, trace and failure
screenshots were exported; checkpoint/export error flags were false. All 23
sealed files and verdict/run-manifest bindings were independently verified.
All 1,648 source files matched the recorded fingerprint. Scoped cleanup succeeded
and no run-owned containers, networks, volumes or detached workspace remained.

External evidence root: `facet-uat-budget-corrected-20260907-7c93a1` under the
operator's temporary opencode directory. Read the run's `verdict.json`,
`evidence/installation/facet-compose.log`, `evidence/browser/result.json`, and
root `integrity-validation.json` before further remediation. Earlier failed and
incomplete runs below remain historical evidence, not the current Docker status.

Next: diagnose the installed composition timeout and the real agent's missing
completion from this evidence. Do not weaken media/provenance assertions or
declare release readiness from the separate supplied-clip success. No new paid
generation, repository edits, or publishing actions were part of this run.

## Scope and decision

The approved repair targets failures observed while installing the public
v1.0.1 release and using real agent CLIs and Studio. The changes are a working
candidate, not a published release. Earlier claims of certified GREEN were not
supported by Level 2 evidence and must not be used as release approval.

## Repairs

- Provider failures require explicit errors. Missing credentials do not select
  mocks. Only an explicit testing request may select mock generation.
- Gflow uses one CLI invocation, reads its actual media-array response, handles
  the observed upscale progress-dot prefix, and publishes the returned artifact
  to the requested file. It does not retry through another transport.
- Gflow stdout and pipe-draining waits are bounded. Attempted generation errors
  retain staging in quarantine with a recovery path rather than deleting a
  potentially paid download. Unknown estimated cost is not advertised as free.
- Source installers validate prerequisites and install packs and the Remotion
  bundle/dependencies. Unsupported piped source installation gives actionable
  guidance. CLI help no longer initializes a project; launch errors propagate.
- Studio project listing avoids dependency-tree scans. Catalog media is scoped
  correctly, with range/download regressions. Completed native conversations can
  resume after same-tab reload within the running Studio's lifetime.
- Review-frame discovery matches actual tool output. Mobile and desktop clipping
  found during fixture testing was corrected.
- Source editing with replacement audio consumes the unused concat audio output;
  the original unconnected-filter failure has a real FFmpeg regression test.
- Producer instructions and schemas use actual tool names/fields, preserve
  optional narration, require paid consent, and reject production placeholders.
- Public documentation describes supported installation and real human/agent
  workflows, with limitations instead of fixed-turn or certification promises.

## Reproducible local checks

Test prerequisites: Go 1.25+, Node.js 20+ with npm (required by the pinned
Playwright), FFmpeg and FFprobe on PATH; PowerShell 7+ for the installer checks.
Install the locked root dev dependencies, then the Chromium revision matched
to that Playwright version, from the repository root:

```powershell
npm ci
npx --no-install playwright install chromium
```

Do not omit dev dependencies for these tests. Browser installation requires
network access unless the matching revision is already cached; Linux also needs
the Chromium system libraries. This test requirement does not change Facet's
Node.js 18+ runtime minimum.

Run from the repository root:

```powershell
go test -short ./... -count=1 -timeout=6m
go vet ./...
node --test bin/launchers.test.js scripts/uat-media.test.mjs scripts/uat-proof.test.mjs scripts/uat-run.test.mjs
node scripts/studio-repair-smoke.mjs
pwsh -File scripts/test-install.ps1
```

The Windows installer integration has a separate isolated test:

```powershell
pwsh -File scripts/test-install-windows.ps1
```

The Studio smoke uses an explicitly synthetic adapter, not a real model. It
tests project selection, session recovery/isolation, playback, byte-matching
downloads and responsive layout. It cannot establish autonomous production.

## Verification evidence from this repair session

The final local regression passed all Go packages, 23 Node tests and go vet.
The real isolated Windows install and synthetic Studio browser checks passed.
Browser fixture playback advanced and downloaded bytes matched the original.

A transport replay used the existing manually generated gflow forest clip.
Progress-prefixed receipt parsing preserved identical bytes and provenance.
A nonzero exit after download returned an error and preserved quarantine without
overwriting the final output. This was a MOCK TRANSPORT replay, not a live
provider success, regardless of wrapper metadata describing its normal branch.

The real Copilot Studio retest created a project, respected a proposal/approval
conversation and generated Edge TTS narration. Its one gflow request failed on
the progress-prefixed response. The subsequent framing fix was tested without
spending another generation. Live gflow-to-Studio final delivery is still unproven.

## Docker status

The recovered Docker run `facet-uat-1788744802494` passed Studio smoke and all
seven installation checks. It exposed the replacement-audio defect and overly
restricted proof grammar, which were repaired afterward. Its historical failed
production verdict is preserved; a valid diagnostic MP4 is not a passing journey.

The final run `facet-uat-1788747270495` returned UNPROVEN (exit 2), with COMPLETE
evidence integrity in dirty development mode. Docker Desktop/WSL failed before
artifact export. Both scenarios failed and production, revision, playback and
download were not verified. Container cleanup could not be confirmed while the
engine was unavailable. Do not infer successful cleanup or release readiness.

Evidence roots are external under the operator-selected temporary opencode
directory: `facet-repairs-docker-rerun`, `facet-repairs-final`,
`gflow-regression-replay`, and `facet-studio-repairs-*`. Keep original manifests,
verdicts and raw evidence unchanged. Logs may be sensitive; do not publish them
without review.

After Docker is healthy, inspect only this run's resources and use its scoped
cleanup command from `facet-repairs-final/FINAL-REPORT.md`. Never use a global
prune or terminate unrelated containers. Then rerun the affected integration:

```powershell
$evidence = Join-Path ([System.IO.Path]::GetTempPath()) 'opencode/facet-repairs-retest'
node scripts/uat-run.mjs $evidence --free-model
```

Free hosted inference needs network access. This development mode is not sealed
offline certification. A dirty source tree is not certification-eligible.

## Remaining gates

### Latest execution blocker

A follow-up supplied-asset acceptance attempt used the existing genuine gflow
forest clip, with no new paid generation authorized. Both candidate Go binaries
built, but isolated Studio startup and the headless fallback timed out before
usable startup evidence. No finished Facet composition or Edge TTS audio was
produced by that attempt. Its external evidence folder is
`facet-forest-acceptance-2113` (ATTEMPT.md, status.json, prompt.txt).

A separate bounded host probe timed out before returning even its initial date
output. This does not establish a Facet defect, nor prove that candidate version
commands started. Stop further launches until basic terminal execution works;
do not repeatedly start renderers or restart shared infrastructure to conceal it.
The newest session-recovery changes also still need runtime verification.
File-only review subsequently added a ten-second timeout for the session lookup,
with a hung-response regression case, so an unavailable recovery endpoint cannot
keep the composer disabled indefinitely. These additions are not runtime-verified.
The same bound now applies to session-close requests during context changes,
with a regression for switching projects after a hung close. This prevents an
unresolved close promise from indefinitely blocking the context-change chain;
it does not guarantee the unreachable server has stopped the old process.

### Subsequent recovery and acceptance results

Terminal execution subsequently recovered. The recovery-only synthetic browser
regression passed all 11 checks, recording 12 chat requests and zero page errors.
Evidence: `facet-session-recovery-synthetic-8AdZhH/results.json` under the external
temporary opencode directory. The initial test failed because it submitted before
the creation/selection continuation settled; the test now waits for readiness.
This supersedes the unverified status above for recovery and close timeout tests.

The subsequent real-Claude Studio supplied-clip attempt built and launched the
candidate, created its isolated project through Playwright, and submitted the
natural-language request. Edge TTS and intermediate processing succeeded, but
the overall bounded attempt ended before any final render. Evidence:
`facet-forest-recovered-2246/RESULT.md`, screenshots and `native-stream.jsonl`.
The original source hash remained unchanged. No new paid clip was generated.
This is an agent-production completion failure within the test budget, not a
host-startup failure, synthetic success, or verified final-video delivery.

### Supplied-clip follow-up delivered

The subsequent `facet-forest-followup-2303` attempt opened the same existing
production through Studio and asked real Claude to finish using the retained
clip and Edge TTS narration. Because Studio had restarted, this was a fresh
native conversation with existing artifacts, not native-session recovery.

The agent produced `renders/final.mp4`: 30.700 seconds, 1920x1080 at 30fps,
H.264/yuv420p and AAC stereo 48kHz. Full decode passed. Visible browser playback
advanced beyond one second without a media error, and the downloaded master
matched the final's SHA-256. Sampled frames show genuine forest footage; prior
execution and edit artifacts trace it to the original gflow clip. Existing
Edge TTS narration was reused unchanged and the ambience asset was excluded.
No new media generation was requested or observed.

The strict 30 +/- 0.6 second duration assertion failed; the original failed
status is preserved. This proves a delivered supplied-asset composition, not
all-checks-pass, exact timing, or fresh generation through the gflow wrapper.
Full-motion/editorial listening review remains separate. Studio still showed
Script not run and QA status unknown rather than a full production verdict.
The producer encountered disk-space exhaustion and used external scratch;
its edit recipe records those intermediate dependencies.

External evidence: `facet-forest-followup-2303/RESULT.md`, native transcript,
playback screenshot, downloaded master and verification logs. Production finished
in approximately 11m30s. Owned browser and server cleanup was verified.

### Exact-duration revision verified

The agent subsequently revised the existing master through a natural Studio
request to exactly 30 seconds, retaining the prior 30.700-second master in
evidence. No new media was generated. The production observer timed out during
agent QA; that attempt's incomplete status remains unchanged.

A separate verification-only browser run independently checked the revised
file: video/audio/container each report 30.000000 seconds; counted video frames
are 900 at 30/1 fps. Full FFmpeg decode passed. Studio playback advanced beyond
one second, seeking to 29 seconds succeeded without a media error, and the
downloaded master matched the final hash. Source and narration hashes remain
unchanged. Final SHA-256:

`e332328df8bd897e2e87d0a07467523318e91ddb5d9097b44242a568b327d5e2`

External evidence: `facet-forest-exact30-2322/RESULT.md` and
`facet-forest-verification-1788759609291/RESULT.json`, including screenshots,
decode logs, representative forest frame and browser download. These are
ordinary unsealed UAT records, not a release-harness certification. This resolves
the supplied-clip timing and delivery checks, not fresh provider generation.

The latest complete local regression passed the Go short suite, go vet and all
26 Node tests. Docker's subsequent health probe timed out again; run-scoped
cleanup and final Docker adjudication remain unverified.

### Outstanding release gates

### Latest local regression

After guidance/schema alignment, the full local Go short suite passed all five
tested packages (three others have no tests), go vet passed, all 26 Node tests
passed, and the recovery-only synthetic browser test passed all 11 checks with
12 requests and zero page errors. These are local checks, not release adjudication.

The first run exposed a brittle quarantine test comparing Windows path strings
with different slash spellings. It was reproduced and repaired with
`filepath.Rel` requiring an exact matching parent, preserving escape negatives.
All eight top-level GFlow tests then passed under both forward-slash and native
backslash TMP/TEMP paths. The Windows symlink branch could not run without the
required OS privilege; do not report that branch verified on Windows.

Local evidence is recorded in the external
`facet-final-local-verification-20260907-MQmujF.md` and the follow-up test output.
The root `.quality-run/` fixture/evidence tree is untracked and ignored as generated
local verification output; its files remain on disk, including failed attempts.
Do not use blanket staging when preparing a reviewed release commit.

The advertised `video_compose` result schema now accepts its optional
`output_facts`, including the final post-mux hash. The delivery regression
reproduced the mismatch before the fix and passed afterward. Host browser-test
dependencies are pinned in the root manifest and lockfile; use the documented
`npm ci` and matching Playwright Chromium installation rather than an unversioned
no-save install. Lock consistency and all 26 Node regressions passed; a clean
host dependency installation was not performed during this check.

The subsequent clean-install attempt was stopped at the disk-space safety gate:
the external verification record `facet-release-verification-20260906-disk-gate-7b9a41/RESULT.md`
records 406,552,576 free bytes on C:, below the required 2 GiB. No dependencies
or browser binaries were installed and no evidence was deleted. This is a
verification precondition failure, not a failed npm install. Re-probe free space
with `Get-PSDrive -Name C` before retrying; require at least 2 GiB for the isolated
dependency check and additional capacity for rendering and Docker. The lack of
space may contribute to host failures but does not establish their root cause.

Verification continued on E: without deleting C: evidence. Native Node loaded
the isolated harness evaluator and Playwright, and launched/closed the existing
cached Chromium successfully. The original isolated npm install still lacks an
observed successful exit and must not be retroactively marked passed.

Manifest-only resolution in `.quality-run/lock-rebuild-20260907T062453-5e859342`
generated a replacement root lock with 11 resolved URLs and 11 integrity hashes.
Each was independently matched to public npm metadata. All dependency versions
and the package manifest stayed unchanged; npm hoisted the same yaml version.
The old lock and metadata checks are retained in that fixture's evidence folder.
Lock generation printed success but exceeded its process bound; the generated
lock was validated separately before transfer. Clean npm ci attempts still timed
out without an observed exit 0. Registry integrity metadata is now present;
clean-install completion remains a distinct outstanding check.

The subsequent diagnostic (`evidence/DIAGNOSIS.md` in that lock-rebuild fixture)
found that both ci runs stopped during tarball fetch/extraction/cache population,
without completion markers. Ten cached tarballs verified; the missing AJV
tarball downloaded independently and matched the locked SHA-512. This rules out
neither filesystem contention nor cache-write stalls, but does not justify
changing package versions or claiming the install finished. A native execution
of all 26 Node tests passed again. The host C: free-space probe had fallen to
51,982,336 bytes. Stop installation/container retries until host capacity is
restored; preserve the existing media and test evidence rather than deleting it
to force the gate forward. No further npm install was performed by this diagnostic.

### Capacity recovered; install stall isolated further

After a fresh probe showed C: had recovered above 8 GiB free, one new isolated
online npm ci still exceeded its 180-second bound. Docker's separate ten-second
version probe also timed out, so no run-scoped cleanup was attempted.

The E: dependency fixture's cache was subsequently verified complete: all 11
tarballs matched locked SHA-512 values, including AJV. Direct calls through npm's
installed pacote fetched/read AJV successfully. One manifest-only offline ci
using that verified cache also timed out after recording all 11 cache hits.
This establishes that the install stall is not solely an online AJV download or
missing-cache issue. It does not establish extraction, filesystem contention or
npm shutdown as the root cause. No version changes or successful-install claim
were made. Evidence: `.quality-run/dependency-final-20260907-9c82b6/evidence/RESULT.md`
and `AJV-DIAGNOSIS.md`. Further identical install retries are not useful without
additional diagnostic evidence.

### Clean offline dependency installation completed

Instrumented cached AJV extraction showed forward progress rather than a proven
deadlock: Windows tar serialized per-file operations and wrote 80 of 466 files
within the initial 30-second probe. Based on that measured throughput, a new
manifest-only offline ci received a 600-second child bound instead of repeating
the unchanged two-minute limit. It completed with an actual exit 0 in 175.517s,
with completely drained output and empty stderr. No retry was made in this final
fixture. Earlier timeout records remain failures/incomplete attempts.

All 11 packages' versions, dependency declarations, resolved URLs and integrity
fields matched the lock. Guarded module resolution confirmed no host fallback;
the harness evaluator loaded and cached Chromium launched, checked a page and
closed. No browser download, provider call or root node_modules modification
occurred. Source manifest hashes remained unchanged.

Reproducible evidence: `.quality-run/dependency-slowhost-final-20260907-b7e40c/RESULT.md`
and its native process/timing records. This closes the clean OFFLINE installation
gate using verified cached public tarballs; it is not a claim that prior online
ci attempts succeeded or that Docker release adjudication passed.

The repository-authored root `CLAUDE.md` now points to the canonical
`skills/facet/SKILL.md` and explicitly gives that source precedence over stale
installed copies. Fixed turns and unsupported request samples were removed;
production rules are scoped to production requests rather than software audits.
The root pointer and conditional-narration contract regression passed, along with
generated-instruction and example-estimate tests. The separate
`.claude/skills/facet/SKILL.md` copy was not overwritten: no managed ownership
record or targeted safe refresh mechanism was established for it. `facet init .`
is not a targeted repair because it skips unowned skills and changes other
scaffolding. Do not overwrite global instruction bundles as a workaround.

1. Restore healthy Docker and verify scoped cleanup, then complete the actual
   agent generation/revision/playback/download run.
2. Retest live gflow through the installed candidate when another generation is
   authorized, including independent original-clip/final provenance and playback.
3. Review the candidate diff and documentation, commit only approved changes,
   and run a clean, appropriately scoped release gate before publication.

### Generated evidence and source provenance

Reserve root `.quality-run/` for disposable dependency-install fixtures, caches and
retained local evidence, never product source, contracts or authored tests. The
anchored `/.quality-run/` ignore rule excludes only that generated untracked tree;
it neither deletes evidence nor changes the inclusion of tracked files. Keep
`.release-harness/` contracts, source, tests and this verification note in scope.

Installed harness 1.2.0 calls `getSourceInfo()` immediately after the toolchain
print in the single-repository `check-pr` path. It queries Git metadata/status,
then enumerates source and hashes each file before printing the source SHA.
Enumeration includes tracked and non-ignored untracked files. `.quality-run` is
already tooling metadata in the harness filesystem fallback and warning scan;
Git-backed enumeration requires the repository ignore rule instead.

Probe the boundary with `git check-ignore -v .quality-run/` (expect the anchored
root rule), and `git ls-files -- .quality-run/` (expect no tracked entries).
Count `git ls-files -z --others --exclude-standard` paths by prefix in Node rather
than dumping fixture contents. Product contracts/source/tests must remain listed
when untracked, and nested product paths named `.quality-run` must not match the
root-only rule. Rerun with native Node:

```powershell
node node_modules/@xibodev/release-harness/bin/release-harness.js check-pr --allow-dirty
```

This installed CLI returns UNPROVEN, exit 2, for a dirty tree before validating
`harness.config.json` or running configured PR commands. Completion at that point
is not a passing test suite or certification.

#### 2026-09-07 boundary repair and bounded rerun

Git output was counted in native Node via `spawnSync`, NUL-delimited paths and a
55-second Git deadline under a 60-second outer bound; no fixture contents were
printed. Before the ignore edit: 1,437 untracked candidates, including 1,397 under
`.quality-run/` (96 ms). After: 40 candidates, zero under `.quality-run/`; the
combined after-count and boundary assertions took 232 ms. Other prefix counts
were unchanged: root files 3, `.release-harness/` 4, `bin/` 1, `cmd/` 1, `docs/` 3,
`internal/` 10 and `scripts/` 18. Existing dependency ignore rules already excluded
`node_modules`; this repair removes the remaining generated fixture/cache/evidence
candidates, not product dependencies or authored source.

Boundary assertions passed: no tracked `.quality-run/` entries; `check-ignore
--no-index` matched `.quality-run/evidence.json` but not
`internal/.quality-run/authored-test.json`, `.release-harness/topology.json`,
`internal/toolbox/contracts_test.go` or `scripts/uat-proof.test.mjs`.
`git diff --check` exited 0 with only LF/CRLF warnings.

The native CLI command above ran once with a 115,000 ms child deadline and
120,000 ms terminal bound. Complete stdout (200 bytes; stderr was empty):

```text
=== Level 1: PR Integration Gate ===
✓ Topology valid (facet, monorepo)
✓ Origins contract valid (1 origins defined)
✓ Toolchain detected:
    Node: v24.18.0
    Git: 2.51.1.
    Docker: 28.5.1
```

Actual result: elapsed 115,073 ms, child status null, signal SIGTERM, spawn error
ETIMEDOUT; supervisor exit 124. There was NO normal harness exit or verdict, not
even the expected dirty-tree UNPROVEN. Only the CLI's own toolchain detection ran;
no independent Docker probe, restart, provider call or commit was performed.

A separate read-only source diagnostic wrapped Git subprocess timings in memory,
then called the installed `enumerateSource()` and `computeTreeDigest()` without
changing installed code. Enumeration completed in 2,220 ms: Git strategy, 1,646
source files, zero `.quality-run/` paths, zero warnings. Its Git probes completed
in 89 ms (root), 112 ms (source candidates), 88 ms (index modes), and 1,749 ms
(ignored-file warning scan). It reached `START digest` but not digest completion
before its 55-second child deadline (55,062 ms total, status null, SIGTERM,
ETIMEDOUT, supervisor exit 124; 60-second outer bound). This isolates a remaining
delay to content hashing in that diagnostic; it does not identify the slow file
or prove the precise stall location of the uninstrumented CLI run. The generated
evidence exclusion is correct but insufficient to resolve the timeout. Keep all
remaining source in scope and diagnose hashing separately rather than expanding
ignore rules to obtain a verdict. All prior evidence remains in place.

No new release, tag, push or publication was performed during this repair.

## Whole-recovery timeout regression

The last code review found that the recovery deadline started only after project
selection completed. A hung first project-detail response could therefore retain
the pending-recovery flag even when subsequent polling loaded the project.
One ten-second deadline now covers selection, session fetch and response parsing.
Owner-only cleanup and context guards prevent late responses from restoring an
obsolete session or clearing a newer recovery operation.

The recovery-only synthetic browser regression passed all 13 checks with 17
chat requests and zero browser errors. It holds the initial detail response,
allows later polls to load the project, verifies the pending state clears, and
releases the old response after a fresh conversation starts. Existing hung-close,
hung-session and context-isolation tests remain covered. Evidence:
`facet-session-recovery-synthetic-RduU2c/results.json` in the external temporary
opencode directory. Script and inline JavaScript syntax checks also passed.
This is synthetic regression evidence, not Docker or real-provider adjudication.

## Level 1 development evidence

After excluding only generated root `.quality-run/` artifacts and diagnosing
source-read latency, the installed Level 1 command completed normally:
`check-pr --allow-dirty` returned exit 2 with
`UNPROVEN (NON-CERTIFYING DEVELOPMENT MODE: dirty working tree)` in 6.016 seconds.
Stdout/stderr were captured directly, avoiding the earlier diagnostic observer's
IPC loss. No retries were performed for this invocation and stderr was empty.
The unchanged dirty worktree is intentionally not certified.

Evidence: `.quality-run/level1-check-pr-20260907T114720Z-f7f92599.result.json`
and its stdout/stderr logs. Full-source hashing was retained; no product source,
contracts or test files were excluded to obtain this result. Docker Level 2 and
run-scoped cleanup remain separate outstanding gates.
