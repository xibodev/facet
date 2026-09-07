# Facet

Video production tools and guidance for your AI coding agent, with a local browser Studio.

Facet combines a Go media toolbox, production packs, and a project workspace. Your agent plans and executes editing, narration, animation, and rendering; Studio lets you chat about a production and inspect the files it produces. Bring an installed, authenticated agent CLI: Claude Code, OpenCode, Codex, or GitHub Copilot. Facet does not supply model access.

[Overview](docs/index.html) | [Installation and Studio guide](docs/docs.html) | [License](LICENSE)

## Install From Source

Prerequisites: Git, **Go 1.25+**, **Node.js 18+ with npm**, and **FFmpeg plus FFprobe** on PATH. Linux/macOS also require Bash and tar. Browser rendering needs supported Chromium, system libraries, and fonts. Go module resolution and npm dependency installation may require network access.

### Windows

```powershell
git clone https://github.com/xibodev/facet.git
pwsh -File ./facet/install.ps1 -NonInteractive
```

The [Windows installer](install.ps1) builds both native executables, installs the full packs/Remotion bundle, and runs `npm ci` in the installed composer. Defaults are `%USERPROFILE%/.facet/bin` and its sibling `bundle` directory. Reopen your terminal for the updated user PATH.

- `-NonInteractive` skips prompts but still creates Desktop and Start Menu shortcuts. Use `-NoShortcuts` to skip them.
- Existing shortcuts are preserved. On a fresh side-by-side install, `-UpdateShortcuts` retargets only recognized Facet links without custom arguments; it cannot be combined with `-NoShortcuts` or `-Isolated`.
- `-NoPath` leaves PATH unchanged; invoke the executable by its full path in that case.
- `-Scope on-demand` is the default and does not change global agent skills. `-Scope global` registers the core skill for Claude Code and OpenCode only, preserving existing skills. Packs remain project-local.
- `-InstallDir` selects the binary directory, with a sibling `bundle`. Existing binaries or bundle cause the installer to refuse an overwrite. Choose a fresh destination instead of deleting a working installation.
- `-Isolated` requires explicit `-HomeDir`, `-InstallDir`, `-NoPath`, `-NoShortcuts`, and on-demand scope. The isolated home must differ from your real profile.

### Linux / macOS

```bash
git clone https://github.com/xibodev/facet.git
bash ./facet/install.sh
export PATH="$HOME/.facet/bin:$PATH"
```

The [Bash installer](install.sh) builds both executables and installs the same content layout under `~/.facet`. It writes into this fixed location; unlike the Windows installer, it does not require a fresh destination. It does not modify shell profiles, global agent skills, or custom Facet configuration. Add the PATH export to your shell profile for future terminals.

### Bundle and Distribution Boundaries

```text
~/.facet/
  bin/                 # facet and facet-ui (Windows: .exe)
  bundle/
    skills/
    packs/
    pipeline_defs/
    schemas/
    styles/
    remotion-composer/  # source, lockfile, installed npm dependencies
```

Both scripts run from a complete checkout, resolving source relative to the script file. You can call them by absolute path from any directory. They copy source without checkout `node_modules` or render/cache outputs and run `npm ci` once during installation, not per prompt.

**Piped installers are not supported.** The scripts do not clone/download source. `go install` builds the CLI alone, not the full production bundle. npm packages provide launchers and content, not guaranteed native binary delivery; `npx` is not a zero-install workflow. Install native Facet from source first. Launchers use a native binary beside themselves or in the user installation directory and report an error if none is available.

An existing `FACET_BIN` override takes precedence; set it to the full native executable path for the command being launched (CLI or Studio). A missing override falls back to normal discovery. Windows launchers prefer `~/.facet/bin` over the legacy LocalAppData installation; they never resolve themselves through PATH.

## Start a Production

First run diagnostics, then choose Studio or the terminal:

```text
facet version
facet doctor
```

`doctor` reports dependency and configuration state. It does not render media, prove a provider works, or certify a release. Authenticate your chosen agent separately and verify a real render from your intended production directory.

### Studio

```text
facet ui
```

The default address is `http://localhost:8787`. Use `--port 8788` for another port, `--no-open` to skip browser launch, and `--dir /absolute/path/to/productions` to select a working/project root.

1. Create a production or open an existing folder. The new-production form asks for its name, folder slug, engine, and packs. Check the location before choosing **Create & Launch**.
2. Choose an available agent engine. Executable detection is not proof of authentication, model access, or quota.
3. Send a simple prompt, such as "Make a short explainer about how rain forms" or "Turn my screen recording into a concise walkthrough."
4. Inspect available script, narration, and review artifacts. Play or download the rendered video when it exists. A new project does not contain a finished video.
5. Keep the same project selected to request revisions: "Shorten the opening and lower the background music."

**Session limits:** Reselecting the same project retains its active conversation. The same browser tab can restore a completed conversation on reload only during the current Studio process lifetime. In-flight turns, earlier activity replay, and recovery after a Studio restart are not supported. Changing project or engine starts separate context. Files persist on disk; ask a new conversation to inspect them rather than assume it remembers prior chat.

Studio binds to loopback and is not a hosted or multi-user service. Ordinary Docker `-p` publishing is insufficient to expose that listener.

### Terminal

```text
facet init my-video --engine opencode
```

This creates a production workspace, projects the core and Explainer skills, and launches the selected agent there. Engine choices are `claude`, `opencode`, `codex`, and `copilot`; the CLI defaults to `claude`. Use `--no-launch` to initialize without launching an agent. Use a separate folder for each production, and provide the real source file location when asking to edit existing footage.

## Production Packs

Packs supply guidance, pipeline definitions, and supporting content. They do not install every referenced tool or provider. Studio lists installed packs for selection; basic CLI initialization selects Explainer.

| Pack | Package | Use and Conditions |
| --- | --- | --- |
| [Explainer](packs/explainer/facet-pack.json) | `@xibodev/facet-pack-explainer` | Animated explainers, diagrams, charts, and text cards. Requires the composer, npm dependencies, browser, and a usable speech route when narrated. |
| [Cinematic](packs/cinematic/facet-pack.json) | `@xibodev/facet-pack-cinematic` | Cinematic/documentary montages, grading, and mixing. Requires available source assets and suitable rights; generated shots need a configured provider. |
| [Screen Demo](packs/screen-demo/facet-pack.json) | `@xibodev/facet-pack-screendemo` | Software walkthroughs and synthetic terminal scenes. Real capture needs app access and recording/browser tooling. Synthetic scenes are not evidence of real product behavior. |
| [Talking Head](packs/talking-head/facet-pack.json) | `@xibodev/facet-pack-talkinghead` | Presenter edits and avatar workflows. Needs presenter footage or separate avatar/lip-sync support and consent. TTS alone does not generate a talking face. |
| [Social](packs/social/facet-pack.json) | `@xibodev/facet-pack-social` | Vertical clips, podcast highlights, and captions. Needs source footage and accurate transcript/timing data; framing and moment selection require review. |
| [Character Animation](packs/character-animation/facet-pack.json) | `@xibodev/facet-pack-animation` | SVG rigs, poses, and action timelines. Requires assets, animation authoring, and a renderer; no character-generation model is bundled by the pack. |
| [Localization](packs/localization/facet-pack.json) | `@xibodev/facet-pack-localization` | Translated narration, subtitles, and dubbing. Requires transcription/translation and a supported target-language voice. Cloning/lip-sync needs separate provider support and consent; check language and timing. |

## Google Flow Prerequisites

Facet's `gflow_image` and `gflow_video` tools invoke the standalone [gflow CLI](https://github.com/xibodev/gflow-cli). Facet does not install or authenticate it for you.

1. Install the standalone `gflow` CLI using its own instructions and put it on PATH.
2. Install its Chrome extension and grant the required site permissions for `labs.google`.
3. Open [Google Flow](https://labs.google/fx/tools/flow) in that Chrome profile and log in with an eligible account. Keep the required browser/extension connection available.
4. Check setup/status diagnostics, then test actual generation only after approving any account credits or charges. Inspect the returned local media file.

**Setup/status success is not generation evidence.** Account eligibility, credits, quotas, models, and service availability still apply. There is no zero-cost guarantee; an unknown estimate or zero-valued generic metadata is not a billing promise.

Provider failures are errors, not an invitation to substitute mock media. Explicit `mock: true` is for contract tests and must never be presented as real generated content. After a timeout, check whether the remote job is still running before retrying.

## Project Files and Configuration

Initialization creates `assets/`, `artifacts/`, `narration/`, `renders/`, `.facet/`, `.facet.yaml`, and `facet.lock.json`, plus agent-specific skill projections (for example `.opencode/skills/`). Scripts, scene plans, narration, videos, and review reports are created only when that work runs. `renders/final.mp4` is a convention, not an output guaranteed by initialization.

Configuration discovery preserves pinned paths in `.facet.yaml` or `~/.config/facet/config.yaml`. Unpinned bundle discovery checks the current directory and two parents, then executable-relative `../bundle` and the source root, then `~/.facet/bundle`. Pack resolution prefers `paths.bundle`; Remotion discovery prefers that bundle's composer. Existing project config is loaded before reinitialization so pinned paths/defaults survive unless explicitly changed.

Verify the renderer from the actual production directory, not only from the source checkout. A discovered configuration path alone does not prove a successful render.

## Known Limitations

- Installation and dependency diagnostics do not prove end-to-end production. Browser libraries, fonts, runtime paths, agent logins, and provider access can still fail.
- Pack guidance can reference optional capabilities outside the native toolbox. Check the installed registry and implementation before promising a route.
- Edge TTS is keyless but network-dependent. Other media services can require credentials and payment; quotas and model availability vary.
- Studio reload recovery covers completed conversations only in the same tab and current Studio lifetime. No in-flight recovery, activity replay, or restart persistence is provided.
- Automated review is partial. Watch and listen to the output, check caption timing, factual claims, consent, and asset rights. A file or sampled frame is not editorial acceptance.
- No fixed-turn, completion-time, broadcast-quality, or release-certification claim is made here.

## Technical Reference

Most users can work through natural-language prompts. Integrators can expand the source-linked reference below.

<details>
<summary>CLI discovery and source contracts</summary>

```text
facet help
facet init --help
facet ui --help
facet tools list
facet tools describe video_compose
facet tools describe output_review
facet tools estimate <tool-name> --input request.json
facet tools run <tool-name> --input request.json
```

Use the installed catalog instead of a fixed tool count. `request.json` must match the selected tool and operation. Descriptions and standalone schemas cover different surfaces and can lag the implementation; these links are not a claim of exhaustive API coverage.

- [CLI commands and flags](cmd/facet/main.go)
- [Tool registry, aliases, and request descriptions](internal/toolbox/toolbox.go)
- [Composition requests and runtime invocation](internal/toolbox/compose.go)
- [Remotion composition registration, default props, and themes](remotion-composer/src/Root.tsx)
- [Explainer scene types and rendering](remotion-composer/src/Explainer.tsx)
- [Artifact JSON schemas](schemas/artifacts) and [standalone tool JSON schemas](schemas/tools)
- [Technical output review](internal/toolbox/visual_qa.go)
- [Google Flow request validation and errors](internal/toolbox/gflow.go)
- [Core producer guidance](skills/facet/SKILL.md) and [pipeline definitions](pipeline_defs)

Use source at your installed revision when debugging. The repository branch may contain changes not present in your binary.

</details>

## License

GNU Affero General Public License, version 3 or later: [AGPL-3.0-or-later](LICENSE).
