#!/bin/sh
# Report guidance in an installed Facet runtime bundle that no longer matches
# this repository.
#
# A runtime's bundle is a COPY made when the release archive was packaged, and
# it is what the installed `facet` and every agentic CLI wired to it read: the
# skill digests describe the bundle, not this checkout. Installed guidance and
# the Remotion composer have drifted months behind the repository before (a
# stale padding rule, a composer that ignored duration_seconds) while every
# digest check passed, so compare the bytes.
#
# Exits non-zero when anything is missing, stale or left over, so this can gate
# a release rather than being something a person has to remember to look at.
#
# Usage: scripts/check-bundle-current.sh [BUNDLE_DIR]
# An explicit argument wins, and naming a directory that does not exist is an
# error. Otherwise $FACET_BUNDLE, else the active runtime's bundle under
# ~/.facet/current; only that default may be absent (nothing installed yet).
set -eu

if [ -n "${1:-}" ]; then
  if [ ! -d "$1" ]; then
    echo "FATAL: no bundle at $1 (named explicitly)" >&2
    exit 1
  fi
  bundle=$(cd "$1" && pwd)
else
  bundle=${FACET_BUNDLE:-$HOME/.facet/current/bundle}
  if [ ! -d "$bundle" ]; then
    echo "no installed bundle at $bundle; nothing to compare"
    exit 0
  fi
fi
cd "$(dirname "$0")/.."
echo "comparing against: $bundle"

# Everything a bundle ships: guidance trees, and the composer's source and
# manifests (never its installed node_modules).
list() (
  cd "$1"
  for tree in skills packs agents schemas remotion-composer/src; do
    [ ! -d "$tree" ] || find "$tree" -type f ! -path '*/node_modules/*'
  done
  for file in package.json package-lock.json tsconfig.json composer-manifest.json; do
    [ ! -f "remotion-composer/$file" ] || echo "remotion-composer/$file"
  done
)

repo_files=$(mktemp)
bundle_files=$(mktemp)
trap 'rm -f "$repo_files" "$bundle_files"' EXIT
list . | LC_ALL=C sort > "$repo_files"
list "$bundle" | LC_ALL=C sort > "$bundle_files"

status=0
while IFS= read -r rel; do
  if [ ! -f "$bundle/$rel" ]; then
    echo "MISSING in bundle: $rel"
    status=1
  elif ! cmp -s "$rel" "$bundle/$rel"; then
    echo "STALE in bundle: $rel"
    status=1
  fi
done < "$repo_files"
extra=$(LC_ALL=C comm -13 "$repo_files" "$bundle_files")
if [ -n "$extra" ]; then
  printf '%s\n' "$extra" | sed 's/^/EXTRA in bundle: /'
  status=1
fi

if [ "$status" -eq 0 ]; then
  echo "installed bundle matches the repository"
else
  echo
  echo "installed agents read the bundle copy, not this repository."
  echo "refresh: package a release archive (scripts/package-release.py) and reinstall it with"
  echo "  install.sh --archive <zip> --checksums <file>, or install the matching published release."
fi
exit "$status"
