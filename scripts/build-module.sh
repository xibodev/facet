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

# A module that cannot describe itself is not installable, so fail here rather
# than let the host discover it.
"$out/xibodev.facet.exe" module describe --json > /dev/null
echo "built $out/xibodev.facet.exe with declared content"
