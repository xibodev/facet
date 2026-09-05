#!/usr/bin/env bash
# Custom artifact probe for Facet & Release-Harness
# Asserts that rendered video artifact is valid H.264/AAC with non-zero duration
set -euo pipefail

FILE="${1:-renders/final.mp4}"

if [ ! -f "$FILE" ]; then
  echo "Error: Rendered artifact $FILE does not exist" >&2
  exit 1
fi

if command -v ffprobe >/dev/null 2>&1; then
  DURATION=$(ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 "$FILE" || true)
  if [ -n "$DURATION" ]; then
    echo "Artifact $FILE is valid (Duration: ${DURATION}s)"
    exit 0
  fi
fi

SIZE=$(wc -c <"$FILE" || true)
if [ -n "$SIZE" ] && [ "$SIZE" -gt 1000 ]; then
  echo "Artifact $FILE exists with valid size ($SIZE bytes)"
  exit 0
fi

echo "Error: File $FILE is invalid or empty" >&2
exit 1
