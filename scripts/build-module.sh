#!/bin/sh
# Build the module binary WITH the content its descriptor declares.
#
# The host copies declared content relative to the BINARY, not the repository.
# A binary built into dist/ alone reported four "not readable" warnings and
# would ship a module with no overlay and no skills — an agent that knows
# nothing about Facet. The previous install only worked because an earlier
# copy's content happened to still be in place.
#
# Everything here is a path the descriptor declares. Adding a declared skill
# or overlay without adding it here reintroduces the same silent gap.
set -e
out="${1:-dist}"
# Not rm -rf: the previous binary may be running (a host holds it open).
# Overwriting in place is enough and does not fail on a busy directory.
mkdir -p "$out/agents" "$out/skills" "$out/packs" "$out/schemas"

go build -o "$out/xibodev.facet.exe" ./cmd/facet

cp agents/facet-creative.md "$out/agents/"
cp -r skills/facet "$out/skills/"
cp -r packs/explainer "$out/packs/"
cp -r schemas/artifacts "$out/schemas/"

# The Remotion composer SOURCE, which the descriptor declares facet_bundle as
# holding. Omitting it left an installed bundle running a composer from a week
# earlier: it hardcoded `(lastEnd + 1) * 30` and ignored duration_seconds
# entirely, so a 2-second plan rendered 3.000s through the installed path while
# the same request through this build rendered 2.000s.
#
# Identical binaries, identical request, different videos — the difference was
# a composer nothing kept current.
#
# node_modules is NOT copied: it is ~130 packages the install already has, and
# package.json is compared instead so a dependency change is visible rather
# than silently reused.
# Copied ALWAYS, not only when node_modules is already present. The previous
# condition meant a FRESH package never received the composer at all — only an
# existing install did — so the first install of a new target had no renderer
# source and the check that should have caught it was skipped for the same
# reason.
mkdir -p "$out/remotion-composer"
cp -r remotion-composer/src "$out/remotion-composer/"
cp remotion-composer/package.json remotion-composer/package-lock.json    remotion-composer/tsconfig.json "$out/remotion-composer/" 2>/dev/null || true
cp -r remotion-composer/public "$out/remotion-composer/" 2>/dev/null || true

if [ ! -d "$out/remotion-composer/node_modules" ]; then
  echo "NOTE: $out/remotion-composer has no node_modules; run npm ci there before rendering"
fi

# A module that cannot describe itself is not installable, so fail here rather
# than let the host discover it.
"$out/xibodev.facet.exe" module describe --json > /dev/null
echo "built $out/xibodev.facet.exe with declared content"
