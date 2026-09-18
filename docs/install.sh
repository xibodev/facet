#!/usr/bin/env bash
# Website bootstrap. Installation policy remains in the release's install.sh.
# The function is parsed in full before executing, so a piped script cannot be
# mistaken for prompt input. Interactive input is explicitly taken from the TTY.
facet_bootstrap() (
    set -euo pipefail
    version=1.0.3
    expected=108bcf2353c5b20e09b81189ad7006689464f23083fc5dd88e5bc948c15edc2c
    url="https://github.com/xibodev/facet/releases/download/v$version/facet-installer-$version.zip"
    case "$(uname -s)" in Linux|Darwin) ;; *) printf '%s\n' 'Use the PowerShell command on Windows.' >&2; exit 1;; esac
    for command in curl unzip zipinfo shasum; do
        command -v "$command" >/dev/null || { printf 'Required download utility: %s\n' "$command" >&2; exit 1; }
    done
    noninteractive=0
    for argument in "$@"; do [[ "$argument" != --yes ]] || noninteractive=1; done
    if [[ $noninteractive == 0 ]]; then
        if ! { exec 3</dev/tty; } 2>/dev/null; then
            printf '%s\n' 'Interactive setup needs a terminal. For automation, download the script and pass --yes --target <cli> --project <dir>.' >&2
            exit 1
        fi
    fi
    temp=$(mktemp -d "${TMPDIR:-/tmp}/facet-bootstrap.XXXXXX")
    trap 'rm -rf -- "$temp"' EXIT
    printf 'Downloading Facet %s installer…\n' "$version"
    curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 --max-time 180 --max-filesize 1048576 "$url" -o "$temp/installer.zip"
    actual=$(shasum -a 256 "$temp/installer.zip")
    [[ "${actual%% *}" == "$expected" ]] || { printf '%s\n' 'Installer checksum mismatch; nothing executed.' >&2; exit 1; }
    # Accept only the small, known installer package, never arbitrary ZIP paths.
    printf '%s\n' install.ps1 install.sh installer/README.md installer/manifest.tsv installer/verify.html | LC_ALL=C sort > "$temp/expected"
    zipinfo -1 "$temp/installer.zip" | LC_ALL=C sort > "$temp/actual"
    cmp -s "$temp/expected" "$temp/actual" || { printf '%s\n' 'Unexpected installer package layout.' >&2; exit 1; }
    mkdir "$temp/package"
    unzip -q "$temp/installer.zip" -d "$temp/package"
    if [[ $noninteractive == 1 ]]; then
        bash "$temp/package/install.sh" "$@" </dev/null
    else
        bash "$temp/package/install.sh" "$@" <&3
    fi
)
facet_bootstrap "$@"
