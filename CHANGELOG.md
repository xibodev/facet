# Changelog

## 1.0.3

- Script-owned interactive installation for OpenCode, Codex, Claude Code, and GitHub Copilot CLI, using prebuilt Windows, Linux, and macOS binaries on x64 and ARM64.
- Shared dependency/host manifest and selectable Remotion, Piper, gflow, and HyperFrames runtimes with approximate download sizes.
- Checksummed product downloads, project-local launchers, preserved user instructions, and local media verification. Contributor source builds moved to `scripts/install-source.*`.
- Native script integration tests and clean Ubuntu Docker installation with real rendering are release-cutting CI gates.
- Agentic CLI use is recommended; Facet Standalone is experimental. Optional media providers report their own configuration requirements.

Scope: core integration checks cover the native platforms; complete optional-runtime installation and rendering are exercised on Windows x64 locally and Ubuntu x64 in Docker. External paid generation and every renderer/platform combination are not certified. Piper is unavailable on Windows ARM64 in this installer.

## 1.0.2 Candidate (Unreleased)

- Install the full production bundle and locked composer dependencies from source;
  preserve configuration and user-owned skills. Fix CLI help side effects, launch
  error propagation, npm launcher recursion and Windows install discovery. Existing
  Windows shortcuts remain unchanged unless recognized links are explicitly migrated.
- Fail explicitly on missing provider credentials or failed Remotion rendering,
  without substituting mock media. Preserve failed gflow downloads for recovery,
  validate CLI receipts and report unknown provider costs as unknown.
- Support project-local media staging and explicit Explainer export profiles;
  contain the authored canvas at smaller/portrait sizes, retain bounded timeout
  progress, and bind output facts to delivered bytes. Fix replacement-audio editing.
- Repair Studio catalog media links, scoped downloads, production scanning and
  completed-conversation recovery within the same tab and running Studio process.
- Add offline regressions, Linux/Windows CI and Docker UAT helpers with bounded,
  sanitized evidence capture. Clarify installation, provider and production guidance.

Known limitations: no in-flight or server-restart conversation recovery; portrait
Explainer output uses containment/letterboxing. Fresh post-fix paid gflow generation
has not been verified. Hosted-model UAT uses open egress and cannot certify the
sealed network policy. Local checks and prior dirty-source UAT are not a clean-source
release verdict; no sealed PASS or publication is claimed for this candidate.
