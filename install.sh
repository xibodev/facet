#!/usr/bin/env bash
# Installer policy lives here, never in the Facet application.
set -euo pipefail
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
MANIFEST="$SCRIPT_DIR/installer/manifest.tsv"
die() { printf '%s\n' "$*" >&2; exit 1; }
definition() {
    awk -F '\t' -v id="$1" -v col="$2" '$2==id {sub(/\r$/, "", $NF); print $col; n++} END {if(n!=1)exit 1}' "$MANIFEST"
}
[[ -f "$MANIFEST" ]] || die 'Run install.sh from the complete installer package.'
[[ $(definition layout 3) == 1 ]] || die 'Unsupported installer manifest schema.'
VERSION=$(definition facet 3); TARGET=''; PROJECT=''; INSTALL=''; COMPONENTS=remotion
ARCHIVE=''; SUMS=''; YES=0; SKIP_VERIFY=0
while (($#)); do
    case "$1" in
        --version|--target|--project|--install-dir|--components|--archive|--checksums)
            (($# >= 2)) || die "Missing value for $1"
            case "$1" in
                --version) VERSION=$2;; --target) TARGET=$2;; --project) PROJECT=$2;;
                --install-dir) INSTALL=$2;; --components) COMPONENTS=$2;; --archive) ARCHIVE=$2;; --checksums) SUMS=$2;;
            esac; shift 2;;
        --yes) YES=1; shift;; --skip-verify) SKIP_VERIFY=1; shift;;
        --help|-h) printf '%s\n' 'Usage: bash install.sh [--target opencode|codex|claude|copilot] [--project DIR] [--install-dir DIR] [--version VERSION] [--components remotion,piper,gflow,hyperframes|none] [--archive ZIP --checksums FILE] [--yes] [--skip-verify]'; exit 0;;
        *) die "Unknown option: $1";;
    esac
done
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9.-]+)?$ ]] || die 'Invalid release version.'
[[ -z "$ARCHIVE" && -z "$SUMS" || -n "$ARCHIVE" && -n "$SUMS" ]] || die 'Supply both --archive and --checksums.'
case $(uname -s) in Linux) OS=linux;; Darwin) OS=darwin;; *) die 'Use install.ps1 on Windows.';; esac
case $(uname -m) in x86_64|amd64) ARCH=amd64;; arm64|aarch64) ARCH=arm64;; *) die 'Unsupported architecture.';; esac
ask() {
    local answer
    printf '%s [%s]: ' "$1" "$2" >&2
    IFS= read -r answer || die 'Interactive input ended; use --yes with explicit options.'
    printf '%s' "${answer:-$2}"
}
confirm() { printf '%s\n' "$*"; [[ $YES == 1 ]] || [[ $(ask 'Continue?' y) =~ ^(y|yes)$ ]] || die 'Installation cancelled.'; }
[[ $YES != 1 || -n "$TARGET" && -n "$PROJECT" ]] || die '--yes requires --target and --project.'
[[ -n "$TARGET" ]] || TARGET=$(ask 'CLI: opencode, codex, claude, copilot' opencode)
[[ $(definition "$TARGET" 1) == host ]] || die 'Unsupported CLI.'
HOST_PATH=$(definition "$TARGET" 4)
[[ -n "$PROJECT" ]] || PROJECT=$(ask 'Project directory' .)
absolute() { case "$1" in /*) printf '%s' "$1";; *) printf '%s/%s' "$PWD" "$1";; esac; }
PROJECT=$(absolute "$PROJECT"); INSTALL=$(absolute "${INSTALL:-$HOME/.facet/releases/$VERSION-$OS-$ARCH}")
[[ "$PROJECT$INSTALL" != *$'\n'* && "$PROJECT$INSTALL" != *$'\r'* && "$PROJECT$INSTALL" != *$'\t'* ]] || die 'Control characters are unsupported in installation paths.'
if [[ -n "$ARCHIVE" ]]; then ARCHIVE=$(absolute "$ARCHIVE"); SUMS=$(absolute "$SUMS"); fi
printf '%s\n' 'Core: FFmpeg and FFprobe. Optional downloads (approximate; platform/cache dependent):'
awk -F '\t' '$1=="component" {printf "%d. %s: %s — %s\n",++n,$2,$8,$9}' "$MANIFEST"
printf '%s\n' 'Media providers are optional and may require credentials/account access. CLI authentication is assumed.'
[[ $YES == 1 ]] || COMPONENTS=$(ask 'Select numbers or names separated by commas; none for core only' "$COMPONENTS")
SELECTED=()
IFS=',' read -r -a raw <<< "$COMPONENTS"
for id in "${raw[@]}"; do
    id=${id// /}
    if [[ "$id" =~ ^[1-4]$ ]]; then id=$(awk -F '\t' -v i="$id" '$1=="component" && ++n==i {print $2}' "$MANIFEST"); fi
    if [[ "$id" != none ]]; then [[ $(definition "$id" 1) == component ]] || die "Unknown component: $id"; fi
    for existing in "${SELECTED[@]:-}"; do [[ "$existing" != "$id" ]] || die 'Duplicate component.'; done
    SELECTED+=("$id")
done
has() { local item; for item in "${SELECTED[@]}"; do [[ "$item" != "$1" ]] || return 0; done; return 1; }
if has none && ((${#SELECTED[@]} != 1)); then die 'none cannot be combined with components.'; fi
real_ancestors() {
    local path=$1
    while [[ "$path" != / && -n "$path" ]]; do
        [[ ! -L "$path" ]] || die "Unsafe directory link: $path"
        [[ ! -e "$path" || -d "$path" ]] || die "Not a directory: $path"
        path=$(dirname -- "$path")
    done
}
# macOS /var and /tmp are OS-owned aliases; resolve the existing project parent
# before checking paths, without following a user-owned host-config link.
canonical_parent() {
    local path=$1 suffix='' part
    while [[ ! -d "$path" ]]; do part=$(basename -- "$path"); suffix="/$part$suffix"; path=$(dirname -- "$path"); done
    printf '%s%s' "$(cd -- "$path" && pwd -P)" "$suffix"
}
PROJECT=$(canonical_parent "$PROJECT"); INSTALL=$(canonical_parent "$INSTALL")
SKILL="$PROJECT/$HOST_PATH/facet"; STATE="$PROJECT/.facet-install"
new_path() { [[ ! -e "$1" && ! -L "$1" ]] || die "Preserving existing entry: $1"; real_ancestors "$(dirname -- "$1")"; }
new_path "$SKILL"; new_path "$STATE"; real_ancestors "$INSTALL"
confirm "Facet $VERSION -> $INSTALL; CLI $TARGET -> $PROJECT; optional: ${SELECTED[*]}"
for program in curl unzip zipinfo awk shasum; do command -v "$program" >/dev/null || die "Install required archive utility: $program"; done
TEMP=$(mktemp -d); STAGE=''; PROJECT_STAGE=''
cleanup() { [[ -z "$STAGE" ]] || rm -rf -- "$STAGE"; [[ -z "$PROJECT_STAGE" ]] || rm -rf -- "$PROJECT_STAGE"; rm -rf -- "$TEMP"; }
trap cleanup EXIT
fetch() { curl --fail --location --proto '=https' --tlsv1.2 --max-time 900 --output "$2" "$1"; }
hash_file() { shasum -a 256 "$1" | awk '{print $1}'; }
verify_checksum() {
    local expected
    expected=$(awk -v name="$3" '{sub(/\r$/, "")} $2==name || $2=="*"name {print tolower($1);n++} END{if(n!=1)exit 1}' "$2") || die "Checksum entry missing or duplicated: $3"
    [[ "$expected" =~ ^[a-f0-9]{64}$ && $(hash_file "$1") == "$expected" ]] || die "Checksum mismatch: $3"
}
safe_name() {
    local name=$1 part
    while [[ "$name" == ./* ]]; do name=${name#./}; done
    [[ -n "$name" ]] || return 0
    [[ "$name" != /* && "$name" != *\\* && "$name" != *:* && "$name" != *$'\r'* && "$name" != *$'\t'* ]] || die "Unsafe archive path: $name"
    local parts
    IFS='/' read -r -a parts <<< "$name"
    for part in "${parts[@]}"; do [[ "$part" != .. && "$part" != . && -n "$part" && "$part" != *'.' && "$part" != *' ' ]] || die "Unsafe archive path: $name"; done
}
expand_zip() {
    local archive=$1 dest=$2 name mode
    zipinfo -1 "$archive" > "$TEMP/zip-names"
    while IFS= read -r name; do safe_name "${name%/}"; done < "$TEMP/zip-names"
    [[ $(sort -f "$TEMP/zip-names" | uniq -di | wc -l | tr -d ' ') == 0 ]] || die 'Duplicate archive entries.'
    zipinfo -l "$archive" > "$TEMP/zip-modes"
    awk 'length($1)==10 && $2 ~ /^[0-9]/ {if(substr($1,1,1)!="-" && substr($1,1,1)!="d")bad=1;total+=$4} END {if(bad || total>1073741824)exit 1}' "$TEMP/zip-modes" || die 'Archive links/special files or excessive size.'
    unzip -q "$archive" -d "$dest"
    [[ -z $(find "$dest" -type l -print) ]] || die 'Archive contains links.'
}
NAME="facet-$VERSION-$OS-$ARCH.zip"
if [[ -z "$ARCHIVE" ]]; then
    base="https://github.com/$(definition facet 4)/releases/download/v$VERSION"
    ARCHIVE="$TEMP/$NAME"; SUMS="$TEMP/SHA256SUMS.txt"
    fetch "$base/$NAME" "$ARCHIVE"; fetch "$base/SHA256SUMS.txt" "$SUMS"
fi
verify_checksum "$ARCHIVE" "$SUMS" "$NAME"
archive_hash=$(hash_file "$ARCHIVE")
if [[ -d "$INSTALL" ]]; then
    [[ -f "$INSTALL/.facet-receipt" && -f "$INSTALL/.facet-files.sha256" ]] || die 'Existing directory is not managed by these scripts.'
    [[ $(cat "$INSTALL/.facet-receipt") == "$VERSION $archive_hash" ]] || die 'Installation receipt mismatch; choose a fresh directory.'
    awk '$2=="bin/facet" {found=1} END{if(!found)exit 1}' "$INSTALL/.facet-files.sha256" || die 'Receipt is missing product binary.'
    while IFS= read -r line; do
        file=${line#*  }; safe_name "$file"
        [[ ! -L "$INSTALL/$file" ]] || die 'Installed file replaced by link.'
        real_ancestors "$(dirname "$INSTALL/$file")"
    done < "$INSTALL/.facet-files.sha256"
    [[ -z $(find "$INSTALL/bin" "$INSTALL/bundle/skills" "$INSTALL/bundle/packs" -type l -print) ]] || die 'Installed product replaced by links.'
    (cd "$INSTALL" && shasum -a 256 -c .facet-files.sha256) >/dev/null || die 'Installed files changed.'
else
    mkdir -p -- "$(dirname -- "$INSTALL")"
    STAGE=$(mktemp -d "$(dirname -- "$INSTALL")/.facet-stage-XXXXXX")
    expand_zip "$ARCHIVE" "$STAGE"
    for required in bin/facet bundle/skills/facet/SKILL.md bundle/packs/explainer/SKILL.md bundle/remotion-composer/package-lock.json; do [[ -f "$STAGE/$required" ]] || die "Release missing $required"; done
    chmod +x "$STAGE/bin/facet"
    [[ $("$STAGE/bin/facet" version) == "facet v$VERSION" ]] || die 'Binary version mismatch.'
    (cd "$STAGE" && find bin bundle -type f -exec shasum -a 256 '{}' \;) > "$STAGE/.facet-files.sha256"
    printf '%s %s\n' "$VERSION" "$archive_hash" > "$STAGE/.facet-receipt"
    mv -- "$STAGE" "$INSTALL"; STAGE=''
fi
FACET="$INSTALL/bin/facet"; DEPS="$INSTALL/dependencies"; mkdir -p "$DEPS"
export PATH="$DEPS/node/bin:$DEPS/gflow:$DEPS/piper/bin:$PATH"
install_linux_node() {
    local version arch stem base name nd entry
    version=$(definition node 3); arch=x64; [[ "$ARCH" != arm64 ]] || arch=arm64
    stem="node-v$version-linux-$arch"; name="$stem.tar.gz"; base="https://nodejs.org/dist/v$version"
    confirm "Download official Node.js $version runtime (about 30-60 MB) into $DEPS/node"
    fetch "$base/$name" "$TEMP/$name"; fetch "$base/SHASUMS256.txt" "$TEMP/node-sums"
    verify_checksum "$TEMP/$name" "$TEMP/node-sums" "$name"
    new_path "$DEPS/node"
    tar -tzf "$TEMP/$name" > "$TEMP/node-names"
    while IFS= read -r entry; do safe_name "${entry%/}"; [[ "$entry" == "$stem/"* ]] || die 'Unexpected Node archive root.'; done < "$TEMP/node-names"
    # Skip npm/npx/corepack links and replace npm/npx with explicit launchers.
    # No other special files are allowed in the extracted runtime.
    tar -tvzf "$TEMP/$name" --exclude="$stem/bin/npm" --exclude="$stem/bin/npx" --exclude="$stem/bin/corepack" > "$TEMP/node-modes"
    awk 'substr($1,1,1)!="-" && substr($1,1,1)!="d" {bad=1} END {exit bad}' "$TEMP/node-modes" || die 'Unexpected links/special files in Node runtime.'
    nd="$TEMP/node-unpack"; mkdir "$nd"
    tar -xzf "$TEMP/$name" -C "$nd" --exclude="$stem/bin/npm" --exclude="$stem/bin/npx" --exclude="$stem/bin/corepack"
    for entry in npm npx; do
        printf '#!/usr/bin/env bash\nexec %q %q "$@"\n' "$DEPS/node/bin/node" "$DEPS/node/lib/node_modules/npm/bin/$entry-cli.js" > "$nd/$stem/bin/$entry"
        chmod +x "$nd/$stem/bin/$entry"
    done
    mv "$nd/$stem" "$DEPS/node"
    hash -r
}
system_package() {
    local id=$1 package
    if [[ "$OS" == darwin ]]; then
        package=$(definition "$id" 7); command -v brew >/dev/null || die "Install Homebrew or $id manually and rerun."
        confirm "brew install $package"; brew install "$package"
        if [[ "$id" == node || "$id" == python ]]; then export PATH="$(brew --prefix "$package")/bin:$PATH"; fi
    else
        if [[ "$id" == node ]]; then install_linux_node; return; fi
        command -v apt-get >/dev/null || die "Install $id with your distribution's package manager and rerun."
        package=$(definition "$id" 6); local packages; read -r -a packages <<< "$package"
        confirm "sudo apt-get install ${packages[*]}"
        if [[ $EUID == 0 ]]; then DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "${packages[@]}"; else sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "${packages[@]}"; fi
    fi
}
if ! command -v ffmpeg >/dev/null || ! command -v ffprobe >/dev/null; then system_package ffmpeg; fi
ffmpeg -version >/dev/null; ffprobe -version >/dev/null
node_compatible() { command -v node >/dev/null && [[ $(node -p "process.versions.node.split('.')[0]") -ge $(definition node 4) ]]; }
if has remotion || has hyperframes; then
    if ! node_compatible; then system_package node; fi
    node_compatible || die "Node.js $(definition node 4)+ is required; update Node using your distribution's supported method and rerun."
    command -v npm >/dev/null || die 'npm is missing; install it and rerun.'
    if [[ "$OS" == linux ]]; then system_package browser-libs; fi
fi
if has remotion; then
    COMPOSER="$INSTALL/$(definition remotion 4)"
    confirm "Install locked Remotion packages and browser: $COMPOSER"
    (cd "$COMPOSER" && npm ci --no-audit --no-fund && node node_modules/@remotion/cli/remotion-cli.js browser ensure)
fi
if has hyperframes; then
    HF="$DEPS/hyperframes"; mkdir -p "$HF"
    [[ -f "$HF/package.json" ]] || printf '{"private":true,"dependencies":{"hyperframes":"%s"}}\n' "$(definition hyperframes 3)" > "$HF/package.json"
    confirm "Install HyperFrames $(definition hyperframes 3) and browser"
    action=install; [[ ! -f "$HF/package-lock.json" ]] || action=ci
    (cd "$HF" && npm "$action" --no-audit --no-fund)
    HF_ENTRY="$HF/node_modules/hyperframes/bin/hyperframes.mjs"; node "$HF_ENTRY" browser ensure
fi
if has gflow; then
    gn="gflow_$(definition gflow 3)_${OS}_$ARCH.tar.gz"
    base="https://github.com/$(definition gflow 4)/releases/download/v$(definition gflow 3)"
    fetch "$base/$gn" "$TEMP/$gn"; fetch "$base/checksums.txt" "$TEMP/gflow-sums"
    verify_checksum "$TEMP/$gn" "$TEMP/gflow-sums" "$gn"
    if [[ ! -d "$DEPS/gflow" ]]; then
        tar -tzf "$TEMP/$gn" > "$TEMP/tar-names"
        while IFS= read -r name; do safe_name "${name%/}"; done < "$TEMP/tar-names"
        tar -tvzf "$TEMP/$gn" > "$TEMP/tar-modes"
        awk 'substr($1,1,1)!="-" && substr($1,1,1)!="d" {bad=1} END {exit bad}' "$TEMP/tar-modes" || die 'gflow archive contains links/special files.'
        mkdir "$DEPS/gflow"; tar -xzf "$TEMP/$gn" -C "$DEPS/gflow"
    fi
    "$DEPS/gflow/gflow" --help >/dev/null
fi
if has piper; then
    command -v python3 >/dev/null || system_package python
    if [[ ! -x "$DEPS/piper/bin/python" ]]; then
        python3 -m venv "$DEPS/piper" || { system_package python; python3 -m venv "$DEPS/piper"; }
    fi
    PYTHON="$DEPS/piper/bin/python"; VOICES="$DEPS/voices"; mkdir -p "$VOICES"
    confirm "Install Piper $(definition piper 3) and voice $(definition piper 4)"
    "$PYTHON" -m pip install --only-binary=:all: "piper-tts==$(definition piper 3)"
    "$PYTHON" -m piper.download_voices --download-dir "$VOICES" "$(definition piper 4)"
fi
if [[ $SKIP_VERIFY == 0 ]]; then
    VERIFY="$TEMP/verify"; mkdir "$VERIFY"; VIDEO="$VERIFY/test.mp4"
    ffmpeg -v error -f lavfi -i color=c=blue:s=320x180:r=24:d=1 -c:v libx264 -pix_fmt yuv420p "$VIDEO"
    if has remotion; then
        # JSON is valid YAML; Node is already a selected renderer dependency.
        node -e 'require("fs").writeFileSync(process.argv[1],JSON.stringify({paths:{remotion_composer:process.argv[2]}}))' "$VERIFY/.facet.yaml" "$COMPOSER"
        VIDEO="$VERIFY/render.mp4"
        node -e 'require("fs").writeFileSync(process.argv[1],JSON.stringify({width:320,height:180,fps:24,duration_seconds:1,output_path:process.argv[2],cuts:[{type:"text_card",text:"Facet setup",in_seconds:0,out_seconds:1}]}))' "$VERIFY/render.json" "$VIDEO"
        (cd "$VERIFY" && "$FACET" tools run video_compose --input "$VERIFY/render.json")
    fi
    ffprobe -v error -show_streams "$VIDEO"; ffmpeg -v error -i "$VIDEO" -f null -
    if has piper; then
        printf 'Facet setup verification.\n' | "$PYTHON" -m piper --model "$VOICES/$(definition piper 4).onnx" --output_file "$VERIFY/voice.wav"
        ffmpeg -v error -i "$VERIFY/voice.wav" -f null -
    fi
    if has hyperframes; then
        mkdir "$VERIFY/html"; cp "$SCRIPT_DIR/installer/verify.html" "$VERIFY/html/index.html"
        (cd "$VERIFY/html" && node "$HF_ENTRY" render --output "$VERIFY/html/render.mp4" --fps 24)
        ffmpeg -v error -i "$VERIFY/html/render.mp4" -f null -
    fi
    printf '%s\n' 'Local media verification passed; external providers and host invocation were not tested.'
else printf '%s\n' 'Verification skipped: media readiness is unverified.'; fi
new_path "$SKILL"; new_path "$STATE"
mkdir -p "$PROJECT"; PROJECT_STAGE=$(mktemp -d "$PROJECT/.facet-stage-XXXXXX")
mkdir "$PROJECT_STAGE/state"; cp -R "$INSTALL/bundle/skills/facet" "$PROJECT_STAGE/skill"
cp -R "$INSTALL/bundle/packs" "$PROJECT_STAGE/state/packs"
{
    printf '#!/usr/bin/env bash\n'
    printf 'export PATH=%q:"$PATH"\n' "$DEPS/node/bin:$DEPS/gflow:$DEPS/piper/bin:$(dirname "$(command -v ffmpeg)")"
    if command -v node >/dev/null; then printf 'export PATH=%q:"$PATH"\n' "$(dirname "$(command -v node)")"; fi
    printf 'exec %q "$@"\n' "$FACET"
} > "$PROJECT_STAGE/state/run-facet.sh"
chmod +x "$PROJECT_STAGE/state/run-facet.sh"
{
    printf '\n## This installation\n'
    printf -- '- Invoke Facet through `%s` followed by the normal arguments; use this launcher instead of bare facet in examples.\n' "$STATE/run-facet.sh"
    printf -- '- Resolve packs/... under `%s`. Read a relevant pack SKILL.md on demand.\n' "$STATE"
    printf -- '- Optional components: %s. Tools report missing media-provider configuration when used.\n' "${SELECTED[*]}"
    if has piper; then printf -- '- Piper model: `%s`.\n' "$VOICES/$(definition piper 4).onnx"; fi
    if has hyperframes; then printf -- '- Use `node "%s"` for pinned HyperFrames; avoid unpinned npx.\n' "$HF_ENTRY"; fi
} >> "$PROJECT_STAGE/skill/SKILL.md"
printf 'version\t%s\ninstallation\t%s\nhost\t%s\ncomponents\t%s\n' "$VERSION" "$INSTALL" "$TARGET" "${SELECTED[*]}" > "$PROJECT_STAGE/state/installation.tsv"
mkdir -p "$(dirname "$SKILL")"
mv "$PROJECT_STAGE/state" "$STATE"
if ! mv "$PROJECT_STAGE/skill" "$SKILL"; then rm -rf "$STATE"; die 'Failed to register skill.'; fi
printf 'Installed. Start a new %s session in %s and ask it to use Facet.\n' "$TARGET" "$PROJECT"
