# Facet

Video production tools and guidance for your agentic CLI.

Facet combines media tools, production packs, and a project workspace. Your agent plans and executes editing, narration, animation, and rendering.

[Overview](docs/index.html) | [Installation guide](docs/docs.html) | [License](LICENSE)

## Recommended Installation

Use Facet with your agentic CLI. The interactive installer supports OpenCode, Codex, Claude Code, and GitHub Copilot CLI, and lets you choose the optional production tools you need.

Run the command for your platform in a terminal:

```powershell
# Windows (PowerShell 7)
irm https://xibodev.github.io/facet/install.ps1 | iex
```

```bash
# Linux / macOS
curl -fsSL https://xibodev.github.io/facet/install.sh | bash
```

The website bootstrap downloads and checksum-verifies the v1.0.3 installer package, then launches its interactive installer. That installer selects the matching prebuilt Windows, Linux, or macOS bundle for x64 or ARM64.

**Manual/offline download:** [Download the installer ZIP](https://github.com/xibodev/facet/releases/download/v1.0.3/facet-installer-1.0.3.zip), extract it, and run `pwsh -File ./install.ps1` or `bash ./install.sh` from the extracted folder. Keep its `installer/` directory beside the scripts. [Release assets and SHA-256 checksums](https://github.com/xibodev/facet/releases/tag/v1.0.3) are also available. Only the website bootstrap is designed for piping; the extracted installer requires its companion files.

The installer lets you select your CLI, project folder, and optional dependencies. It installs prebuilt Facet binaries and registers the Facet skill in your CLI's project directory, preserving existing user instructions and configuration. Start a new agent session in that project after setup.

### Optional Dependencies

Local editing and rendering do not require media-provider API keys. Optional generation services may require separate credentials, account access, or credits. The installer explains optional downloads, and tools report missing configuration when you use them.

- **FFmpeg and FFprobe:** core media editing and inspection.
- **Remotion:** animated compositions and rendered captions, with its browser and Node.js/npm dependencies.
- **Piper:** local speech synthesis, with an optional runtime and voice-model download.
- **gflow:** optional image/video generation through its supported providers.
- **HyperFrames:** HTML-based video rendering.

The dependency selection lists approximate download sizes and what each component adds.

## Using Facet with Another Agent

Using a different agentic CLI? Ask your agent to inspect this repository and adapt Facet's skills and command-line tools to its own integration conventions. The installer currently supports the four CLIs listed above; other integrations are community-adapted and may need additional setup.

## Start a Production

Open your selected CLI in the project configured by the installer and ask it to use Facet:

> Use Facet to make a short explainer about how rain forms.

Provide actual file locations when editing existing footage. Review the rendered video and continue the conversation to request changes.

## Standalone Status

Facet Standalone is experimental. For now, we recommend using Facet through a supported agentic CLI.

## Production Packs

Packs supply guidance, pipeline definitions, and supporting content. They do not install every referenced tool or provider. The Facet skill directs your agent to the relevant pack for the task.

| Pack | Package | Use and Conditions |
| --- | --- | --- |
| [Explainer](packs/explainer/facet-pack.json) | `@xibodev/facet-pack-explainer` | Animated explainers, diagrams, charts, and text cards. Requires the composer, npm dependencies, browser, and a usable speech route when narrated. |
| [Cinematic](packs/cinematic/facet-pack.json) | `@xibodev/facet-pack-cinematic` | Cinematic/documentary montages, grading, and mixing. Requires available source assets and suitable rights; generated shots need a configured provider. |
| [Screen Demo](packs/screen-demo/facet-pack.json) | `@xibodev/facet-pack-screendemo` | Software walkthroughs and synthetic terminal scenes. Real capture needs app access and recording/browser tooling. Synthetic scenes are not evidence of real product behavior. |
| [Talking Head](packs/talking-head/facet-pack.json) | `@xibodev/facet-pack-talkinghead` | Presenter edits and avatar workflows. Needs presenter footage or separate avatar/lip-sync support and consent. TTS alone does not generate a talking face. |
| [Social](packs/social/facet-pack.json) | `@xibodev/facet-pack-social` | Vertical clips, podcast highlights, and captions. Needs source footage and accurate transcript/timing data; framing and moment selection require review. |
| [Character Animation](packs/character-animation/facet-pack.json) | `@xibodev/facet-pack-animation` | SVG rigs, poses, and action timelines. Requires assets, animation authoring, and a renderer; no character-generation model is bundled by the pack. |
| [Localization](packs/localization/facet-pack.json) | `@xibodev/facet-pack-localization` | Translated narration, subtitles, and dubbing. Requires transcription/translation and a supported target-language voice. Cloning/lip-sync needs separate provider support and consent; check language and timing. |

## Optional gflow Integration

Facet's `gflow_image` and `gflow_video` tools invoke the optional [gflow CLI](https://github.com/xibodev/gflow-cli). Select it during dependency setup if you need it. gflow supports extension-free provider routes; its help and error diagnostics explain the requirements of the provider you choose.

**Setup/status success is not generation evidence.** Account eligibility, credits, quotas, models, and service availability still apply. There is no zero-cost guarantee; an unknown estimate or zero-valued generic metadata is not a billing promise.

Provider failures are errors, not an invitation to substitute mock media. Explicit `mock: true` is for contract tests and must never be presented as real generated content. After a timeout, check whether the remote job is still running before retrying.

## Project Files and Configuration

The CLI-only installer registers a project skill (for example `.opencode/skills/facet/SKILL.md`) and creates `.facet-install/` for its launcher, installation record, and pack references. Existing project configuration is preserved. Scripts, scene plans, narration, videos, and review reports are created only when that work runs. `renders/final.mp4` is a convention, not an output guaranteed by installation.

Configuration discovery preserves pinned paths in `.facet.yaml` or `~/.config/facet/config.yaml`. Unpinned bundle discovery checks the current directory and two parents, then executable-relative `../bundle` and the source root, then `~/.facet/bundle`. Pack resolution prefers `paths.bundle`; Remotion discovery prefers that bundle's composer. Existing project config is loaded before reinitialization so pinned paths/defaults survive unless explicitly changed.

Verify the renderer from the actual production directory, not only from the source checkout. A discovered configuration path alone does not prove a successful render.

## Known Limitations

- Installation and dependency diagnostics do not prove end-to-end production. Browser libraries, fonts, runtime paths, agent logins, and provider access can still fail.
- Pack guidance can reference optional capabilities outside the native toolbox. Check the installed registry and implementation before promising a route.
- Edge TTS is keyless but network-dependent. Other media services can require credentials and payment; quotas and model availability vary.
- Facet Standalone is experimental; the recommended workflow uses a supported agentic CLI.
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
