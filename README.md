# Facet

Video-production tools and guidance for agentic CLIs. Facet gives the agent in
Claude Code, Codex, GitHub Copilot CLI, OpenCode, or Compa a set of media tools
and the guidance to use them for editing, narration, animation, captions,
rendering, and review. Your CLI stays in charge of the model, tool calls, and
approvals; Facet never calls a reasoning model and keeps no workflow state.

- **Facet Toolkit:** the `facet` command. 33 media tools that probe, edit,
  stitch, compose, narrate, caption, generate, and review video; an advisory
  catalog of production routes; and an MCP server, `facet mcp`.
- **Facet Bundle:** the `facet` skill, a `facet-<pack>` skill for each of seven
  production packs, and the `facet-creative` agent, installed into your CLI by
  `facet wire`.

## Install

```sh
# Linux / macOS
curl -fsSL https://xibodev.github.io/facet/install.sh | sh
```

```powershell
# Windows (PowerShell)
irm https://xibodev.github.io/facet/install.ps1 | iex
```

The installer keeps one runtime per user under `~/.facet` (`FACET_HOME` moves
it), verifies it with short test renders, adds `~/.facet/current/bin` to your
PATH, and offers to wire your CLIs. FFmpeg is required and is installed when
missing. Remotion and Piper are default components; HyperFrames is optional.

Coming from Facet 1.x, which installed into each project? See
[Upgrading from 1.x](https://xibodev.github.io/facet/docs.html#upgrade).

## Use

```sh
facet wire claude    # or codex, copilot, opencode, compa, a comma-separated list, or all
facet doctor
```

Start a new session in your CLI from a production folder and ask:

> Use Facet to make a short explainer about how rain forms.

`facet wire` installs the skills, registers the `facet` MCP server, and adds a
rule to ask before each of the eight tools that may charge (Claude Code, Codex,
OpenCode, and Compa; Copilot CLI already asks before every tool that is not
read-only). Facet declares each tool's effects; your CLI enforces consent.

Claude Code, Codex, and Copilot CLI can instead add Facet as a plugin from this
repository's marketplace, once the Toolkit is installed:

```sh
claude plugin marketplace add xibodev/facet && claude plugin install facet@facet
codex plugin marketplace add xibodev/facet && codex plugin add facet@facet
copilot plugin marketplace add xibodev/facet && copilot plugin install facet@facet
```

Use the plugin or `facet wire` for a CLI, not both. See
[Install as a Plugin](https://xibodev.github.io/facet/docs.html#plugins).

People and scripts can run the same tools directly:

```sh
facet tools list
facet tools describe video_compose
facet tools run video_compose --input request.json
```

The npm package `@xibodev/facet` is only a launcher for a runtime the installer
put in place.

Documentation: <https://xibodev.github.io/facet/docs.html>

## Development

Go 1.26, Node.js, and FFmpeg.

```sh
go build ./cmd/facet
go test ./...
node --test "bin/*.test.js" "scripts/*.test.mjs"
```

## License

Facet is licensed under
[AGPL-3.0-or-later](https://github.com/xibodev/facet/blob/main/LICENSE). See
[THIRD_PARTY_NOTICES.md](https://github.com/xibodev/facet/blob/main/THIRD_PARTY_NOTICES.md)
for third-party notices.
