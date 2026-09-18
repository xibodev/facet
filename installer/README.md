# Script installer contract

Installation policy belongs to the root `install.ps1` (Windows, PowerShell 7)
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

Windows:

```powershell
pwsh -File ./install.ps1
pwsh -File ./install.ps1 -NonInteractive -Target codex -ProjectDir ./video -Components none
```

Linux/macOS:

```bash
bash ./install.sh
bash ./install.sh --yes --target codex --project ./video --components none
```

Use `-ArchivePath` and `-ChecksumPath` on Windows, or `--archive` and
`--checksums` in Bash, to supply an already-downloaded product archive. Optional
dependencies can still require network access. `-SkipVerify` / `--skip-verify`
explicitly reports media readiness as unverified.

Both scripts leave existing instruction/config files alone, refuse occupied
skill/state locations, and allow a verified product installation to serve another
project. They create a project-local launcher that selects the installed binary
and runtime paths without changing the user's shell profile or persistent PATH.
System package-manager dependency installs may update PATH themselves.

Pack resources live in `.facet-install/packs/`, outside the host's recursive
skill discovery tree. Only the core `facet` skill is registered. Existing
projects with `.facet-install/` are not automatically updated or uninstalled in
this first version; those operations are not advertised as implemented.

## Tests

Installer acceptance is a required release-cutting CI gate. The native platform
matrix runs script integration tests, and a clean Ubuntu Docker container installs
all optional components and verifies real Remotion/HyperFrames renders and Piper
speech before the workflow may create a draft release. These run on GitHub-hosted
runners; local probes supplement rather than replace the release gates.

`scripts/test-prebuilt-install.py` runs the platform's real script against a
real native archive. Set `FACET_INSTALL_SMOKE=1` with FFmpeg installed to exercise
all four host placements, interactive selection, repeat/conflict refusal,
preserved instructions, invalid checksums, traversal, duplicate entries, links,
modified installations, and local encode/probe/decode checks.

`scripts/test-installer-wsl.sh` is a narrower local adapter: native Linux Facet,
Windows FFmpeg version probes, and explicitly skipped media verification. It is
not Linux rendering acceptance.

## Contributor source builds

The previous source installers are preserved as `scripts/install-source.ps1`
and `scripts/install-source.sh`. They require a complete checkout and the build
toolchain declared by the repository. They are not shipped in the release
installer package. Application-owned legacy initialization/diagnostics remain
unchanged pending the separately planned cleanup.
