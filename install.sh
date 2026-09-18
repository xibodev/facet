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
VERSION=${FACET_VERSION:-$(definition facet 3)}; TARGET=${FACET_TARGET:-}; PROJECT=${FACET_PROJECT:-}; INSTALL=${FACET_INSTALL_DIR:-}; COMPONENTS=${FACET_COMPONENTS:-}
ACTION=${FACET_ACTION:-add}; ACTION_EXPLICIT=${FACET_ACTION:-}; DETAIL=0; MIGRATE_LEGACY=0; PLAIN=${FACET_PLAIN:-0}
ARCHIVE=''; SUMS=''; YES=${FACET_YES:-0}; SKIP_VERIFY=0
while (($#)); do
    case "$1" in
        --version|--target|--project|--install-dir|--components|--archive|--checksums|--action)
            (($# >= 2)) || die "Missing value for $1"
            case "$1" in
                --version) VERSION=$2;; --target) TARGET=$2;; --project) PROJECT=$2;;
                --install-dir) INSTALL=$2;; --components) COMPONENTS=$2;; --archive) ARCHIVE=$2;; --checksums) SUMS=$2;;
                --action) ACTION=$2; ACTION_EXPLICIT=1;;
            esac; shift 2;;
        --yes) YES=1; shift;; --skip-verify) SKIP_VERIFY=1; shift;;
        --verbose) DETAIL=1; shift;;
        --plain) PLAIN=1; shift;;
        --migrate-legacy) MIGRATE_LEGACY=1; shift;;
        --help|-h) printf '%s\n' 'Usage: bash install.sh [--target opencode|codex|claude|copilot] [--project DIR] [--install-dir DIR] [--version VERSION] [--components remotion,piper,gflow,hyperframes|none] [--action add|repair|update] [--archive ZIP --checksums FILE] [--yes] [--skip-verify] [--verbose]'; exit 0;;
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
confirm() { printf '  -> %s\n' "$*"; }
RICH=0
if [[ $YES != 1 && $PLAIN != 1 && -z ${NO_COLOR:-} && -t 0 && -t 2 && ${TERM:-dumb} != dumb ]]; then RICH=1; fi
section() { if [[ $RICH == 1 ]]; then printf '\n\033[36;1m%s\033[0m\n' "$1"; else printf '\n%s\n' "$1"; fi; }
# UI only: values come from the same manifest as the plain/noninteractive path.
# Use stderr for drawing, stdout solely for the selected value.
choose() (
    title=$1; multiple=$2; defaults=$3; shift 3
    values=(); labels=(); checked=(); index=0
    while (($#)); do values+=("$1"); labels+=("$2"); shift 2; done
    if [[ $RICH != 1 ]]; then ask "$title" "$defaults"; exit; fi
    for ((i=0;i<${#values[@]};i++)); do
        checked+=(0)
        if [[ ",$defaults," == *",${values[i]},"* ]]; then checked[i]=1; [[ $multiple == 1 ]] || index=$i; fi
    done
    old_tty=$(stty -g)
    trap 'stty "$old_tty"; printf "\033[?25h" >&2' EXIT
    trap 'exit 130' INT TERM
    stty -echo -icanon min 1 time 0
    printf '\n\033[36;1m? %s\033[0m\n\033[?25l' "$title" >&2
    first=1
    while :; do
        if [[ $first == 0 ]]; then printf '\033[%dA' "$((${#values[@]}+1))" >&2; fi
        first=0
        width=$(tput cols 2>/dev/null || printf 80); ((width>20)) || width=80
        for ((i=0;i<${#values[@]};i++)); do
            pointer=' '; mark='( )'; color='\033[0m'
            if [[ $multiple == 1 ]]; then mark='[ ]'; [[ ${checked[i]} == 0 ]] || mark='[x]'; fi
            if [[ $i == "$index" ]]; then pointer='>'; color='\033[32m'; [[ $multiple == 1 ]] || mark='(*)'; fi
            line="  $pointer $mark ${labels[i]}"
            printf '\033[2K%b%s\033[0m\n' "$color" "${line:0:$((width-2))}" >&2
        done
        if [[ $multiple == 1 ]]; then hint='Up/Down: move | Space: toggle | Enter: confirm | Esc: cancel'; else hint='Up/Down: move | Enter: confirm | Esc: cancel'; fi
        printf '\033[2K\033[90m%s\033[0m\n' "${hint:0:$((width-2))}" >&2
        IFS= read -rsn1 key || exit 1
        case "$key" in
            $'\033')
                # macOS ships Bash 3.2, whose read timeout must be an integer.
                suffix=''; IFS= read -rsn2 -t 1 suffix || true
                case "$suffix" in '[A') index=$(((index+${#values[@]}-1)%${#values[@]}));; '[B') index=$(((index+1)%${#values[@]}));; *) printf '\nInstallation cancelled.\n' >&2; exit 1;; esac;;
            ' ') if [[ $multiple == 1 ]]; then checked[index]=$((1-checked[index])); fi;;
            '')
                result=''
                for ((i=0;i<${#values[@]};i++)); do
                    if [[ $multiple == 1 && ${checked[i]} == 1 || $multiple == 0 && $i == "$index" ]]; then result="${result:+$result,}${values[i]}"; fi
                done
                result=${result:-none}
                printf '\033[%dA' "$((${#values[@]}+1))" >&2
                for ((i=0;i<=${#values[@]};i++)); do printf '\033[2K\n' >&2; done
                printf '\033[%dA\033[32m  Selected: %s\033[0m\n' "$((${#values[@]}+1))" "$result" >&2
                printf '%s' "$result"; exit 0;;
        esac
    done
)
[[ $YES != 1 || -n "$TARGET" && -n "$PROJECT" ]] || die '--yes requires --target and --project.'
default_host=opencode
section 'FACET'
printf 'Video production for your agent.\nInstaller %s | %s/%s\n' "$VERSION" "$OS" "$ARCH"
section '[1/3] Choose your setup'
detected=''
host_options=()
while IFS=$'\t' read -r kind id rest; do
    if [[ "$kind" == host ]]; then
        status='not detected'
        if command -v "$id" >/dev/null; then [[ -n "$detected" ]] || default_host=$id; detected="$detected $id"; status=detected; fi
        host_options+=("$id" "$id - $status")
    fi
done < "$MANIFEST"
printf '  Detected CLIs:%s\n' "${detected:- none (choose one to configure)}"
[[ -n "$TARGET" ]] || TARGET=$(choose 'Which CLI should use Facet? (opencode, codex, claude, copilot)' 0 "$default_host" "${host_options[@]}")
[[ $(definition "$TARGET" 1) == host ]] || die 'Unsupported CLI.'
HOST_PATH=$(definition "$TARGET" 4)
[[ -n "$PROJECT" ]] || PROJECT=$(ask 'Project directory' .)
absolute() { case "$1" in /*) printf '%s' "$1";; *) printf '%s/%s' "$PWD" "$1";; esac; }
PROJECT=$(absolute "$PROJECT"); INSTALL=$(absolute "${INSTALL:-$HOME/.facet/releases/$VERSION-$OS-$ARCH}")
if [[ -z "$COMPONENTS" && -f "$PROJECT/.facet-install/installation.tsv" ]]; then COMPONENTS=$(awk -F '\t' '$1=="components" {gsub(/ /,",",$2); print $2}' "$PROJECT/.facet-install/installation.tsv"); fi
COMPONENTS=${COMPONENTS:-remotion}
[[ "$PROJECT$INSTALL" != *$'\n'* && "$PROJECT$INSTALL" != *$'\r'* && "$PROJECT$INSTALL" != *$'\t'* ]] || die 'Control characters are unsupported in installation paths.'
if [[ -n "$ARCHIVE" ]]; then ARCHIVE=$(absolute "$ARCHIVE"); SUMS=$(absolute "$SUMS"); fi
printf '%s\n' 'Core: FFmpeg and FFprobe. Optional downloads (approximate; platform/cache dependent):'
awk -F '\t' -v selected=",$COMPONENTS," '$1=="component" {printf "%s %d. %s: %s — %s\n",index(selected,","$2",")?"[x]":"[ ]",++n,$2,$8,$9}' "$MANIFEST"
printf '%s\n' 'Media providers are optional and may require credentials/account access. CLI authentication is assumed.'
if [[ $YES != 1 ]]; then
    component_options=()
    while IFS=$'\t' read -r kind id version value windows linux darwin size capability; do
        [[ "$kind" != component ]] || component_options+=("$id" "$id | $size | $capability")
    done < "$MANIFEST"
    COMPONENTS=$(choose 'Optional production tools (uncheck all for core editing; plain input: 1,2 or none)' 1 "$COMPONENTS" "${component_options[@]}")
fi
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
PREVIOUS=0
LEGACY_MIGRATION=0
if [[ -d "$STATE" ]]; then
    real_ancestors "$STATE"
    [[ $(awk -F '\t' '$1=="host" {print $2}' "$STATE/installation.tsv") == "$TARGET" ]] || die 'Project is configured for another CLI.'
    if [[ ! -f "$STATE/managed-files.sha256" ]]; then
        if [[ $YES != 1 ]]; then [[ $(ask 'Migrate older integration? Complete original files will be kept in a project backup' n) =~ ^(y|yes)$ ]] && MIGRATE_LEGACY=1; fi
        [[ $MIGRATE_LEGACY == 1 ]] || die 'Older integration has no ownership hashes; use --migrate-legacy to preserve a full backup before replacing it.'
        LEGACY_MIGRATION=1
        real_ancestors "$SKILL"
        [[ -z $(find "$STATE" "$SKILL" -type l -print) ]] || die 'Refusing legacy migration containing links.'
    else
    while IFS= read -r line; do
        file=${line#*  }; [[ "$file" != /* && "$file" != *../* && "$file" != *\\* ]] || die 'Invalid ownership record.'
        [[ ! -L "$PROJECT/$file" ]] || die 'Preserving replaced project file.'
        real_ancestors "$(dirname "$PROJECT/$file")"
    done < "$STATE/managed-files.sha256"
    (cd "$PROJECT" && shasum -a 256 -c "$STATE/managed-files.sha256") >/dev/null || die 'Preserving modified project files.'
    expected_count=$(wc -l < "$STATE/managed-files.sha256" | tr -d ' ')
    actual_count=$(find "$STATE" "$SKILL" -type f ! -path "$STATE/managed-files.sha256" | wc -l | tr -d ' ')
    [[ "$actual_count" == "$expected_count" ]] || die 'Preserving extra files in managed project directories.'
    fi
    PREVIOUS=1
    if [[ $YES != 1 && -z "$ACTION_EXPLICIT" ]]; then ACTION=$(choose 'What should setup do?' 0 add add 'Add components - keep existing tools' repair 'Repair - verify a replacement' update 'Update - switch to selected version'); fi
    if [[ "$ACTION" == add ]]; then
        [[ $(awk -F '\t' '$1=="version" {print $2}' "$STATE/installation.tsv") == "$VERSION" ]] || die 'Choose update to change product version.'
        INSTALL=$(awk -F '\t' '$1=="installation" {print $2}' "$STATE/installation.tsv")
        for item in $(awk -F '\t' '$1=="components" {print $2}' "$STATE/installation.tsv"); do
            if [[ "$item" != none ]] && ! has "$item"; then SELECTED+=("$item"); fi
        done
        if ((${#SELECTED[@]} > 1)) && has none; then filtered=(); for item in "${SELECTED[@]}"; do [[ "$item" == none ]] || filtered+=("$item"); done; SELECTED=("${filtered[@]}"); fi
    fi
else new_path "$SKILL"; new_path "$STATE"; fi
case "$ACTION" in add|repair|update) ;; *) die 'Choose add, repair, or update.';; esac
real_ancestors "$INSTALL"
REUSE=0
if [[ -f "$INSTALL/components.tsv" && "$ACTION" != repair ]]; then
    REUSE=1
    for item in "${SELECTED[@]}"; do
        [[ "$item" == none ]] || grep -qx "$item" "$INSTALL/components.tsv" || REUSE=0
    done
fi
if [[ -d "$INSTALL" && $REUSE == 0 ]]; then
    [[ -f "$INSTALL/.facet-receipt" ]] || die 'Existing installation is not managed by these scripts.'
    INSTALL="$INSTALL-generation-$(date +%s)-$$"
fi
printf '\n  CLI: %s\n  Project: %s\n  Action: %s\n  Components: %s\n  Runtime: %s\n' "$TARGET" "$PROJECT" "$ACTION" "${SELECTED[*]}" "$INSTALL"
printf '%s\n' '  Existing runtimes are retained until a verified replacement is ready.'
[[ $YES == 1 ]] || [[ $(ask 'Continue?' y) =~ ^(y|yes)$ ]] || die 'Installation cancelled.'
section '[2/3] Install and verify capabilities'
for program in curl unzip zipinfo awk shasum; do command -v "$program" >/dev/null || die "Install required archive utility: $program"; done
LOG_DIR=${FACET_LOG_DIR:-$HOME/.facet/logs}; mkdir -p "$LOG_DIR"
LOG="$LOG_DIR/install-$(date +%Y%m%d-%H%M%S)-$$.log"
TEMP=$(mktemp -d); STAGE=''; PROJECT_STAGE=''; COMMITTED=0; NEW_RUNTIME=1
[[ ! -d "$INSTALL" ]] || NEW_RUNTIME=0
cleanup() {
    rc=$?
    if [[ $COMMITTED == 0 && -n "$PROJECT_STAGE" ]]; then
        if [[ -d "$PROJECT_STAGE/old-state" ]]; then rm -rf "$STATE"; mv "$PROJECT_STAGE/old-state" "$STATE"; fi
        if [[ -d "$PROJECT_STAGE/old-skill" ]]; then rm -rf "$SKILL"; mv "$PROJECT_STAGE/old-skill" "$SKILL"; fi
    fi
    [[ -z "$STAGE" ]] || rm -rf -- "$STAGE"; [[ -z "$PROJECT_STAGE" ]] || rm -rf -- "$PROJECT_STAGE"; rm -rf -- "$TEMP"
    if [[ $COMMITTED == 0 && $NEW_RUNTIME == 1 ]]; then rm -rf -- "$INSTALL"; fi
    if [[ $rc != 0 ]]; then printf '\nSetup incomplete. Previous project bindings preserved. Log: %s\nRerun setup to retry.\n' "$LOG" >&2; fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
redact() { sed -E 's#(https?://)[^/@[:space:]]+@#\1<redacted>@#g; s#([?&][^=[:space:]&]+)=[^&[:space:]]+#\1=<redacted>#g'; }
step() {
    local label=$1 rc; shift
    printf '  -> %s\n' "$label"
    set +e
    if [[ $DETAIL == 1 ]]; then (set -e; "$@") 2>&1 | redact | tee -a "$LOG"; rc=${PIPESTATUS[0]}; else (set -e; "$@") 2>&1 | redact >> "$LOG"; rc=${PIPESTATUS[0]}; fi
    set -e
    if [[ $rc != 0 ]]; then printf '  FAILED %s (see %s)\n' "$label" "$LOG" >&2; return "$rc"; fi
    printf '  OK %s\n' "$label"
}
fetch() { curl --fail --silent --show-error --location --retry 3 --retry-delay 2 --connect-timeout 20 --proto '=https' --tlsv1.2 --max-time 900 --output "$2" "$1"; }
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
if [[ -z "$ARCHIVE" && $REUSE == 0 ]]; then
    base="https://github.com/$(definition facet 4)/releases/download/v$VERSION"
    CACHE="$HOME/.facet/cache"; mkdir -p "$CACHE"
    ARCHIVE="$CACHE/$NAME"; SUMS="$TEMP/SHA256SUMS.txt"
    step 'Check release download' fetch "$base/SHA256SUMS.txt" "$SUMS"
    expected=$(awk -v name="$NAME" '{sub(/\r$/,"")} $2==name{print $1}' "$SUMS")
    if [[ -f "$ARCHIVE" && $(hash_file "$ARCHIVE") == "$expected" ]]; then printf '  OK Reusing verified download\n'; else
        step 'Download Facet' fetch "$base/$NAME" "$ARCHIVE.partial"
        verify_checksum "$ARCHIVE.partial" "$SUMS" "$NAME"; mv "$ARCHIVE.partial" "$ARCHIVE"
    fi
fi
archive_hash=''
if [[ -n "$ARCHIVE" ]]; then verify_checksum "$ARCHIVE" "$SUMS" "$NAME"; archive_hash=$(hash_file "$ARCHIVE"); fi
if [[ -d "$INSTALL" ]]; then
    [[ -f "$INSTALL/.facet-receipt" && -f "$INSTALL/.facet-files.sha256" ]] || die 'Existing directory is not managed by these scripts.'
    [[ -z "$archive_hash" || $(cat "$INSTALL/.facet-receipt") == "$VERSION $archive_hash" ]] || die 'Installation receipt mismatch; choose Repair.'
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
        if [[ "$id" == browser-libs ]] && ! apt-cache show libasound2t64 >/dev/null 2>&1; then
            package=${package//libasound2t64/libasound2}; read -r -a packages <<< "$package"
        fi
        confirm "sudo apt-get install ${packages[*]}"
        if [[ $EUID == 0 ]]; then DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "${packages[@]}"; else sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "${packages[@]}"; fi
    fi
}
if ! command -v ffmpeg >/dev/null || ! command -v ffprobe >/dev/null; then step 'Install media tools' system_package ffmpeg; fi
step 'Check FFmpeg' ffmpeg -version; step 'Check FFprobe' ffprobe -version
node_compatible() { command -v node >/dev/null && [[ $(node -p "process.versions.node.split('.')[0]") -ge $(definition node 4) ]]; }
    if [[ $REUSE == 1 ]]; then printf '  OK Reusing configured dependencies (no package reinstall)\n'; fi
if has remotion || has hyperframes; then
    if ! node_compatible; then
        step 'Install compatible Node runtime' system_package node
        if [[ "$OS" == darwin ]]; then export PATH="$(brew --prefix "$(definition node 7)")/bin:$PATH"; fi
    fi
    node_compatible || die "Node.js $(definition node 4)+ is required; update Node using your distribution's supported method and rerun."
    command -v npm >/dev/null || die 'npm is missing; install it and rerun.'
    if [[ "$OS" == linux && $REUSE == 0 ]]; then step 'Install browser system libraries' system_package browser-libs; fi
fi
COMPOSER="$INSTALL/$(definition remotion 4)"; HF_ENTRY="$DEPS/hyperframes/node_modules/hyperframes/bin/hyperframes.mjs"
PYTHON="$DEPS/piper/bin/python"; VOICES="$DEPS/voices"
if has remotion && [[ $REUSE == 0 ]]; then
    confirm "Install locked Remotion packages and browser: $COMPOSER"
    (cd "$COMPOSER" && step 'Install Remotion packages' npm ci --no-audit --no-fund && step 'Prepare Remotion browser' node node_modules/@remotion/cli/remotion-cli.js browser ensure)
fi
if has hyperframes && [[ $REUSE == 0 ]]; then
    HF="$DEPS/hyperframes"; mkdir -p "$HF"
    [[ -f "$HF/package.json" ]] || printf '{"private":true,"dependencies":{"hyperframes":"%s"}}\n' "$(definition hyperframes 3)" > "$HF/package.json"
    confirm "Install HyperFrames $(definition hyperframes 3) and browser"
    action=install; [[ ! -f "$HF/package-lock.json" ]] || action=ci
    (cd "$HF" && step 'Install HyperFrames packages' npm "$action" --no-audit --no-fund)
    step 'Prepare HyperFrames browser' node "$HF_ENTRY" browser ensure
fi
if has gflow && [[ $REUSE == 0 ]]; then
    gn="gflow_$(definition gflow 3)_${OS}_$ARCH.tar.gz"
    base="https://github.com/$(definition gflow 4)/releases/download/v$(definition gflow 3)"
    step 'Download gflow' fetch "$base/$gn" "$TEMP/$gn"; fetch "$base/checksums.txt" "$TEMP/gflow-sums"
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
if has piper && [[ $REUSE == 0 ]]; then
    command -v python3 >/dev/null || step 'Install Python for Piper' system_package python
    if [[ "$OS" == darwin ]] && command -v brew >/dev/null; then
        python_prefix=$(brew --prefix "$(definition python 7)" 2>/dev/null || true)
        if [[ -d "$python_prefix/bin" ]]; then export PATH="$python_prefix/bin:$PATH"; fi
    fi
    if [[ ! -x "$DEPS/piper/bin/python" ]]; then
        python3 -m venv "$DEPS/piper" || { system_package python; python3 -m venv "$DEPS/piper"; }
    fi
    PYTHON="$DEPS/piper/bin/python"; VOICES="$DEPS/voices"; mkdir -p "$VOICES"
    confirm "Install Piper $(definition piper 3) and voice $(definition piper 4)"
    step 'Install Piper' "$PYTHON" -m pip install --only-binary=:all: "piper-tts==$(definition piper 3)"
    step 'Download speech model' "$PYTHON" -m piper.download_voices --download-dir "$VOICES" "$(definition piper 4)"
fi
verify_media() (
    set -e
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
)
if [[ $SKIP_VERIFY == 0 ]]; then step 'Verify selected local capabilities' verify_media; else printf '%s\n' 'Verification skipped: media readiness is unverified.'; fi
if [[ $PREVIOUS == 0 ]]; then new_path "$SKILL"; new_path "$STATE"; fi
section '[3/3] Connect your CLI'
printf '%s\n' "${SELECTED[@]}" > "$INSTALL/components.tsv"
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
(
    cd "$PROJECT_STAGE"
    find state skill -type f | while IFS= read -r file; do
        case "$file" in state/*) dest=".facet-install/${file#state/}";; skill/*) dest="$HOST_PATH/facet/${file#skill/}";; esac
        printf '%s  %s\n' "$(hash_file "$file")" "$dest"
    done
) > "$PROJECT_STAGE/managed-files.sha256"
mv "$PROJECT_STAGE/managed-files.sha256" "$PROJECT_STAGE/state/managed-files.sha256"
mkdir -p "$(dirname "$SKILL")"
if [[ $PREVIOUS == 1 ]]; then mv "$STATE" "$PROJECT_STAGE/old-state"; mv "$SKILL" "$PROJECT_STAGE/old-skill"; fi
mv "$PROJECT_STAGE/state" "$STATE"
mv "$PROJECT_STAGE/skill" "$SKILL"
if [[ $LEGACY_MIGRATION == 1 ]]; then
    backup=$(mktemp -d "$PROJECT/.facet-backup-XXXXXX")
    mv "$PROJECT_STAGE/old-state" "$backup/state"; mv "$PROJECT_STAGE/old-skill" "$backup/skill"
    printf 'Original integration preserved at: %s\n' "$backup"
fi
COMMITTED=1
printf '\nFacet is ready for %s.\n  Open a new %s session in: %s\n  Try: Use Facet to make a short title card.\n' "$TARGET" "$TARGET" "$PROJECT"
printf 'Setup log: %s\nRerun setup to add components, repair, or update this project.\n' "$LOG"
