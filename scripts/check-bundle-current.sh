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
# Exits non-zero when they differ, so this can gate a release rather than being
# something a person has to remember to look at.
set -e
bundle="${FACET_BUNDLE:-$HOME/.facet/bundle}"
if [ ! -d "$bundle" ]; then
  echo "no installed bundle at $bundle; nothing to compare"
  exit 0
fi

status=0
for rel in \
  agents/facet-creative.md \
  skills/facet/SKILL.md \
  packs/explainer/SCENE-TYPES.md \
  packs/explainer/NARRATED-WALKTHROUGH.md
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
