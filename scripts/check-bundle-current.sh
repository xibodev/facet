#!/bin/sh
# Report guidance in the installed bundle that no longer matches the repository.
#
# The bundle under ~/.facet/bundle is a COPY made at install time, and it is
# what a real installation reads — the descriptor's skill digests describe the
# bundle, not this checkout. Both were honest and the content was months apart:
# the installed SKILL.md still taught the `out_seconds + 1 second` padding rule
# that was corrected here, and knew nothing about the estimate or QA fields
# added since.
#
# Nothing detected that. The digests matched what was installed, so every check
# passed while an agent read stale instructions.
#
# The Remotion composer is checked too, and it drifted worse than the guidance:
# the installed copy hardcoded `(lastEnd + 1) * 30` and ignored duration_seconds
# entirely, so a 2-second plan rendered 3.000s through the installed path while
# the same request through a fresh build rendered 2.000s. Identical binaries,
# identical request, different videos.
#
# Exits non-zero when they differ, so this can gate a release rather than being
# something a person has to remember to look at.
set -e
# An explicit argument WINS. This script silently ignored $1 and always
# checked $HOME/.facet/bundle, so `check-bundle-current.sh dist` reported on a
# directory it was never asked about. Every "bundle current" claim made with an
# argument was true only because the two happened to agree -- which is not a
# property of the measurement.
#
# A nonexistent path passed explicitly is an ERROR, not "nothing to compare":
# the caller named a bundle and it is not there. Only the DEFAULT location may
# be legitimately absent, because then nothing is installed yet.
if [ -n "${1:-}" ]; then
  bundle="$1"
  if [ ! -d "$bundle" ]; then
    echo "FATAL: no bundle at $bundle (named explicitly)" >&2
    exit 1
  fi
else
  bundle="${FACET_BUNDLE:-$HOME/.facet/bundle}"
  if [ ! -d "$bundle" ]; then
    echo "no installed bundle at $bundle; nothing to compare"
    exit 0
  fi
fi
echo "comparing against: $bundle"

status=0
for rel in \
  agents/facet-creative.md \
  skills/facet/SKILL.md \
  packs/explainer/SCENE-TYPES.md \
  packs/explainer/NARRATED-WALKTHROUGH.md   remotion-composer/src/Root.tsx   remotion-composer/src/Explainer.tsx   remotion-composer/src/explainerMetadata.ts   remotion-composer/package.json
do
  if [ ! -f "$bundle/$rel" ]; then
    echo "MISSING in bundle: $rel"
    status=1
    continue
  fi
  if ! cmp -s "$rel" "$bundle/$rel"; then
    echo "STALE in bundle: $rel"
    echo "    installed agents read the bundle copy, not this repository"
    status=1
  fi
done

if [ "$status" -eq 0 ]; then
  echo "installed bundle matches the repository"
else
  echo
  echo "refresh with: ./scripts/build-module.sh \"$bundle\""
fi
exit "$status"
