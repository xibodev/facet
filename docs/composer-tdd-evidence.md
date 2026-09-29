# Composer clean-room TDD evidence

Recorded on 2026-09-18 for `feat/agent-contract-cleanup`.

## Red

The imported `remotion-composer/src` tree and donor-only demos were deleted
before replacement source was authored. The replacement tests were then run:

```text
npm run test:contract
TS18003: No inputs were found ... include paths were ["src"].
exit 2

npm run test:legacy
ENOENT: no such file or directory, scandir '...\remotion-composer\src'
tests 1, pass 0, fail 1
```

This established that the contract tests and source allowlist could not pass
against an absent replacement.

## Green

After independently authoring the five allowlisted source files:

```text
npm test
contract: 5 passed
legacy guard: 1 passed
real renders: 3 passed
```

The real-render tests produced H.264 MP4s and verified direct frame changes,
exact video-stream duration, no audio stream for silent graphics, and AAC audio
for staged narration plus looping music without extending the video.

Additional green gates:

```text
FACET_REMOTION_MEDIA_INTEGRATION=1 go test ./internal/toolbox \
  -run TestRemotionLocalMediaOfflineRender -count=1 -v
PASS

go test -p 1 ./... -count=1
PASS

node scripts/release-package-smoke.mjs
PASS: @xibodev/facet@1.0.4 ... 50 npm files

python scripts/package-release.py --os windows --arch amd64 --out <temp>
PASS; native archive contains exactly the five allowlisted composer source files

node scripts/package-windows.cjs <temp>
PASS; Windows stage contains exactly the five allowlisted composer source files
```
