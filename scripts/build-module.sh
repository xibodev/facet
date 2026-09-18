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
mkdir -p "$out/agents" "$out/skills" "$out/packs"

go build -o "$out/xibodev.facet.exe" ./cmd/facet-module

cp agents/facet-creative.md "$out/agents/"
cp -r skills/facet "$out/skills/"
cp -r packs/explainer "$out/packs/"

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
# package.json and package-lock.json are what make the composer installable:
# without them `npm ci` cannot run and the bundle cannot render. They were
# copied with `2>/dev/null || true`, which discards the error AND the exit
# status, so a failed copy produced a silently incomplete bundle.
#
# Verified: with package.json removed, `module describe` still succeeds and
# check-bundle-current.sh still reports "matches the repository". Both guards
# pass on a bundle that cannot render, because neither looks at these files.
#
# Required files fail the build. tsconfig.json is required too -- the composer
# is TypeScript.
cp remotion-composer/package.json remotion-composer/package-lock.json    remotion-composer/tsconfig.json "$out/remotion-composer/"

# public/ is genuinely optional: it holds staged media that a fresh checkout
# may not have. Optional stays optional, but the reason is now stated rather
# than implied by a silenced error.
if [ -d remotion-composer/public ]; then
  cp -r remotion-composer/public "$out/remotion-composer/"
fi

if [ ! -d "$out/remotion-composer/node_modules" ]; then
  echo "NOTE: $out/remotion-composer has no node_modules; run npm ci there before rendering"
fi

# A module that cannot describe itself is not installable, so fail here rather
# than let the host discover it.
"$out/xibodev.facet.exe" module describe --json > /dev/null

# A module that describes itself is not necessarily a module that can RENDER.
# describe succeeds with or without the composer manifests, so it cannot be the
# only check -- that is the same "a successful exit is not acceptance" mistake
# this repo has made before.
for required in package.json package-lock.json tsconfig.json; do
  if [ ! -f "$out/remotion-composer/$required" ]; then
    echo "FATAL: $out/remotion-composer/$required is missing; the bundle cannot render" >&2
    exit 1
  fi
done
echo "built $out/xibodev.facet.exe with declared content"
