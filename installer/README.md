# Script installer contract

Installation policy belongs to the root `install.ps1` (Windows PowerShell 5.1 or PowerShell 7)
and `install.sh` (Linux/macOS, Bash). Neither invokes `facet init`, `facet doctor`,
or an application-owned setup wizard. No compiled installer is distributed.

## Distribution

`scripts/package-release.py` builds product binaries and creates:

- `facet-<version>-<os>-<arch>.zip`: `bin/` and the canonical `bundle/`.
- `facet-installer-<version>.zip`: both scripts, `installer/manifest.tsv`, and
  the local renderer verification fixture. Unpack this archive before running.
- Per-platform checksum lists, combined into `SHA256SUMS.txt` at release time.

Installer archives are platform-independent and deterministic. The package
step stamps the product version into the manifest without rewriting scripts.
The manifest is tab-separated data, not sourced or evaluated shell code.

Host directories, dependency versions, platform availability, descriptions,
and approximate download sizes have one source: `manifest.tsv`.

## Invocation

The website's `docs/install.ps1` and `docs/install.sh` are small bootstrap
entry points for `irm ... | iex` and `curl ... | bash`. They pin a published
installer version and SHA-256, extract the verified package temporarily, and
invoke these canonical release scripts. They contain no host/dependency setup
policy. Update their pins only after the matching release asset is published.
Bash prompts use `/dev/tty`, never the download pipe. Bootstrap tests run in
normal CI and as a required release-cutting job.

Windows:

```powershell
pwsh -File ./install.ps1
pwsh -File ./install.ps1 -NonInteractive -Target codex -ProjectDir ./video -Components none -Pack cinematic,localization
```

Linux/macOS:

```bash
bash ./install.sh
bash ./install.sh --yes --target codex --project ./video --components none --pack cinematic --production-method localization
```

Use `-ArchivePath` and `-ChecksumPath` on Windows, or `--archive` and
`--checksums` in Bash, to supply an already-downloaded product archive. Optional
dependencies can still require network access. `-SkipVerify` / `--skip-verify`
explicitly reports media readiness as unverified.

Both scripts preserve existing instruction content and merge one bounded,
ownership-verified Facet section into only the selected agent's governing file.
Changes outside that section remain user-owned; a modified managed section stops
repair/update instead of being overwritten. Codex project skills use
`.agents/skills`. The scripts allow a verified product installation to serve
another project and create a project-local launcher that selects the installed
binary and runtime paths without changing the user's shell profile or persistent PATH.
System package-manager dependency installs may update PATH themselves.

Pack resources live in `.facet-install/packs/`, outside the host's recursive
skill discovery tree. Only the core `facet` skill is registered; repeat
`--pack` / `--production-method` (or pass `-Pack` on PowerShell) to activate
named methods in its installed guidance. With no selection, setup is core-only.
Existing projects with ownership hashes can be rerun with `--action add|repair|update|uninstall`
(`-Action` on Windows). Add preserves existing component choices and adds the
selected ones. Repair builds a separate runtime generation; update can rebind to
another explicit release version. A project is rebound only after setup checks
pass, and earlier runtime generations remain available to other projects.
Uninstall verifies every recorded file and the bounded instruction-section hash,
then removes only that project's managed skill files, state files, ownership
record, and Facet section. Modified managed content blocks uninstall; unmanaged
files and instruction text remain in place. The shared runtime is never removed
because another project may still use it.
Reinstall detection is based on a valid project installation receipt, not merely
the presence of `.facet-install`. Non-colliding unmanaged residue is preserved and
merged back transactionally; linked residue or stale entries that collide with
managed state stop setup without deleting the preserved files.

```powershell
pwsh -File ./install.ps1 -NonInteractive -Target codex -ProjectDir ./video -Action uninstall
```

```bash
bash ./install.sh --yes --target codex --project ./video --action uninstall
```

Older v1.0.3 integrations lack file ownership hashes. Interactive setup offers a
separate opt-in migration, or automation can pass `--migrate-legacy` /
`-MigrateLegacy`. The entire old skill and state directory are retained in a
`.facet-backup-*` project folder, including customizations. Review that backup
before deleting it. No account authentication is attempted.

Normal output is concise progress. Detailed, URL-redacted subprocess output is
retained under `~/.facet/logs`; use `--verbose` / `-Verbose` for live detail or
`FACET_LOG_DIR` to select a log directory. Downloads retry transient failures;
verified product archives are cached under `~/.facet/cache`. Configured runtimes
are reused without running npm ci again. Adding dependencies to an existing
runtime builds a sibling generation instead of modifying one used by another
project. System package-manager actions cannot be rolled back by Facet.

Interactive terminals use Up/Down and Enter for CLI/action selection, and Space
to toggle optional components. Completed answers remain visible above numbered
installation stages. Use `--plain` / `-Plain`, `FACET_PLAIN=1`, or `NO_COLOR` for
plain prompts. Redirected terminals also use plain prompts; noninteractive flags
do not invoke the menu. The Unix menu is driven by a real pseudo-terminal test
in release CI, including cursor restoration after cancellation.

Pipe-friendly configuration variables are `FACET_TARGET`, `FACET_PROJECT`,
`FACET_INSTALL_DIR`, `FACET_COMPONENTS`, `FACET_PACKS`, `FACET_ACTION`, and `FACET_YES=1`.
The website bootstraps remain pinned to the last published installer until a
new release is validated and published; local changes do not upgrade that pin.

Supported automatic system-dependency setup is Windows with winget, macOS with
Homebrew, and apt-based Linux distributions. Other Linux distributions can use
preinstalled dependencies; automatic system provisioning for them is not yet
implemented. Download sizes are planning ranges, not exact disk-space budgets.

## Tests

Installer acceptance is a required release-cutting CI gate. The native platform
matrix runs script integration tests, and a clean Ubuntu Docker container installs
all optional components and verifies real Remotion/HyperFrames renders and Piper
speech before the workflow may create a draft release. These run on GitHub-hosted
runners; local probes supplement rather than replace the release gates.

`scripts/test-prebuilt-install.py` runs the platform's real script against a
real native archive. Set `FACET_INSTALL_SMOKE=1` with FFmpeg installed to exercise
all five host placements, interactive selection, repeat/reuse, isolated repair,
explicit legacy migration with backups, failed dependency-add rollback, modified
file preservation, ownership-verified uninstall, shared-runtime retention,
invalid checksums, traversal, duplicate entries, links, modified installations,
and local encode/probe/decode checks. Windows CI runs
this lifecycle suite under both Windows PowerShell 5.1 and PowerShell 7.

`scripts/test-installer-wsl.sh` is a narrower local adapter: native Linux Facet,
Windows FFmpeg version probes, and explicitly skipped media verification. It is
not Linux rendering acceptance.

## Contributor source builds

The previous source installers are preserved as `scripts/install-source.ps1`
and `scripts/install-source.sh`. They require a complete checkout and the build
toolchain declared by the repository. They are not shipped in the release
installer package. Application-owned legacy initialization/diagnostics remain
unchanged pending the separately planned cleanup.
