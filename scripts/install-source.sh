#!/usr/bin/env bash
# Contributor source build: bash /path/to/facet/scripts/install-source.sh
if [ -z "${BASH_VERSION:-}" ]; then
    printf '%s\n' 'Run this source installer with bash, not sh.' >&2
    exit 1
fi
set -euo pipefail

case "$(uname -s)" in
    Linux|Darwin) ;;
    *) printf '%s\n' 'This installer supports Linux/macOS. Use install.ps1 from the checkout on Windows.' >&2; exit 1 ;;
esac

source_help() {
    printf '%s\n' \
        'Facet requires a complete source checkout; this script does not download source code.' \
        'Run: git clone https://github.com/xibodev/facet.git' \
        'Then: bash /absolute/path/to/facet/scripts/install-source.sh' >&2
    exit 1
}

[ -n "${BASH_SOURCE[0]:-}" ] || source_help
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
for file in go.mod cmd/facet/main.go cmd/facet-ui/main.go skills/facet/SKILL.md remotion-composer/package-lock.json; do
    [ -f "${SCRIPT_DIR}/${file}" ] || source_help
done
for folder in skills packs pipeline_defs schemas styles remotion-composer agents; do
    [ -d "${SCRIPT_DIR}/${folder}" ] || source_help
done

for program in go node npm ffmpeg ffprobe tar; do
    command -v "$program" >/dev/null 2>&1 || {
        printf 'Missing prerequisite: %s. Install Go 1.25+, Node.js 18+ with npm, FFmpeg/FFprobe and tar first.\n' "$program" >&2
        exit 1
    }
done
GO_VERSION="$(go env GOVERSION)"
if [[ ! "$GO_VERSION" =~ ^go([0-9]+)\.([0-9]+) ]] || (( BASH_REMATCH[1] < 1 || (BASH_REMATCH[1] == 1 && BASH_REMATCH[2] < 25) )); then
    printf 'Go 1.25+ is required; found %s.\n' "$GO_VERSION" >&2
    exit 1
fi
node -e 'if (Number(process.versions.node.split(".")[0]) < 18) { console.error("Node.js 18+ is required"); process.exit(1); }'

INSTALL_ROOT="${HOME:?HOME must be set}/.facet"
USER_BIN_DIR="${INSTALL_ROOT}/bin"
USER_BUNDLE_DIR="${INSTALL_ROOT}/bundle"
mkdir -p "$USER_BIN_DIR" "$USER_BUNDLE_DIR"

printf '%s\n' 'Building Facet from source...'
(
    cd "$SCRIPT_DIR"
    go build -o "${USER_BIN_DIR}/facet" ./cmd/facet
    go build -o "${USER_BIN_DIR}/facet-ui" ./cmd/facet-ui
)

printf '%s\n' 'Installing skills, packs and Remotion source (excluding local dependencies and render outputs)...'
(
    cd "$SCRIPT_DIR"
    tar --exclude=node_modules --exclude=.git --exclude=out --exclude=dist --exclude=.cache \
        -cf - skills packs pipeline_defs schemas styles remotion-composer agents
) | tar -xf - -C "$USER_BUNDLE_DIR"

printf '%s\n' 'Installing locked Remotion dependencies with npm ci (network access may be required)...'
npm ci --prefix "${USER_BUNDLE_DIR}/remotion-composer" --no-audit --no-fund

printf '\nInstalled binaries: %s\nInstalled bundle: %s\n' "$USER_BIN_DIR" "$USER_BUNDLE_DIR"
printf '%s\n' 'No shell profiles, global agent skills or custom Facet configuration were changed.'
printf '%s\n' 'For this shell, run: export PATH="$HOME/.facet/bin:$PATH"'
printf '%s\n' 'Add that export to your shell profile for future terminals.'
printf '%s\n' 'Then: facet doctor'
printf '%s\n' '      facet init /absolute/path/to/my-video --engine opencode --no-launch'
printf '%s\n' 'Install/authenticate your agent separately before starting a production.'
