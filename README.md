# Facet

Facet equips the user's selected agentic CLI with stateless video-production
tools, compact production packs, and technical review. The agent owns creative
reasoning and orchestration; Facet validates requests, executes media
operations, and reports what happened.

[Documentation](docs/index.html) | [Installation guide](docs/docs.html) |
[License](LICENSE)

## Install

The interactive installer supports OpenCode, Codex, Claude Code, and GitHub
Copilot CLI, plus Facet Studio projects. It preserves unmanaged project
instructions and adds one ownership-verified Facet section to only the selected
target's governing file. Codex project skills use the current `.agents/skills`
convention. Setup is core-only unless production methods are explicitly selected
with the repeatable `--pack` / `--production-method` options (`-Pack` on PowerShell).
`facet init` uses the same bounded-section ownership model: user text outside the
section is preserved, managed-section edits stop a rewrite, and changing adapters
removes only the prior Facet section. Init also reconciles owned skill projections:
adapter switches and pack deselection remove only projections whose recorded type,
link target, or copied-tree hash still matches. Replaced or modified paths stop with
an explicit preservation error.

To remove a script-installed project integration without deleting a shared
runtime, rerun the matching installer with `--action uninstall` or
`-Action uninstall`. Uninstall verifies ownership before removing managed skills,
state, and the Facet instruction section; modified and unmanaged content is kept.
A later reinstall accepts preserved, non-colliding `.facet-install` content while
rejecting unsafe links and stale files that conflict with managed state.

```powershell
irm https://xibodev.github.io/facet/install.ps1 | iex
```

```bash
curl -fsSL https://xibodev.github.io/facet/install.sh | sh
```

The bootstrap downloads and verifies the v1.0.4 installer package. For offline
installation, download the
[installer ZIP](https://github.com/xibodev/facet/releases/download/v1.0.4/facet-installer-1.0.4.zip)
and keep its `installer` directory beside the extracted scripts.

The recommended setup installs FFmpeg/FFprobe, Remotion with its browser, and
Piper as the offline narration fallback. Edge TTS support is built into Facet
and is preferred when network processing is acceptable. gflow and HyperFrames
remain optional. Provider-backed operations can require credentials, account
access, quota, or payment; configuration checks do not prove live access.

## Agent contract

Start a new session in the configured project and ask the agent to use Facet.
The installed `skills/facet/SKILL.md` is the canonical production contract.
Production packs add method-specific constraints without creating workflow
state or replacing the agent's judgment.

```text
facet version
facet routes list
facet routes describe <method>
facet routes assess --input request.json
facet tools list
facet tools describe <tool>
facet tools estimate <tool> --input request.json
facet tools run <tool> --input request.json
```

The operating rules are:

- inspect the request and supplied assets before choosing an operation;
- estimate consequential or provider-backed work before execution;
- require explicit human consent before paid work or publication;
- treat unknown cost as unknown, never as free;
- never substitute mock media for a failed production call; and
- inspect the rendered result because process success is not creative
  acceptance.

Facet Standalone is experimental and consumes the same embedded Facet guidance
and pack assets. The supported product path is the installed agent bundle.

## Production packs

| Pack | Purpose |
| --- | --- |
| Explainer | 2D motion, diagrams, charts, and text-led explanation |
| Cinematic | Source-led documentary, montage, grading, and mixing |
| Screen Demo | Recorded or synthetic software walkthroughs |
| Talking Head | Presenter footage and consented avatar workflows |
| Social | Short-form source repurposing, reframing, and captions |
| Character Animation | Character assets, poses, motion, and assembly |
| Localization | Translation, subtitles, dubbing, and timing review |

Packs describe methods and requirements; they do not install providers or
promise that every optional route is available. `facet routes assess` joins
the selected method to live canonical operations, input availability,
dependencies, network use, and charge effects. It never executes, stores
workflow state, or chooses a provider. A route is feasible only when concrete
file inputs exist, every operation request has a valid canonical shape, the
entry request passes normal estimate validation, every consumable required
input matches its declared operation request field, and intermediate artifact
bindings are constructible. Inputs used only for policy or planning are
explicitly marked informational in the route catalog.

## Project and delivery boundaries

Facet stores no mandatory workflow state. Scripts, requests, assets, renders,
and review reports are ordinary project files created only when the work needs
them. Pinned paths in `.facet.yaml` or the user config are preserved.

Local editing does not require provider keys. gflow generation is optional,
must be invoked through the named Facet tools, and returns unknown cost when a
reliable price is unavailable. A `mock: true` result is test evidence only.

Technical review can verify measurable properties, but it cannot approve
story, taste, factual claims, rights, consent, or publication. Report
unverified conditions and provider failures plainly.

## License

GNU Affero General Public License, version 3 or later:
[AGPL-3.0-or-later](LICENSE). Notices for distributed dependencies and assets
remain in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
