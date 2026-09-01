# Video Agent Bundle Provenance Snapshot

- Capture timestamp (UTC): `2026-08-30T15:21:29.5182762Z`
- Donor checkout: `C:\Users\gafar\AppData\Local\Temp\opencode\video-agent-bundle`
- Snapshot destination: `E:\open-source-projects\video-kit\provenance\video-agent-bundle`
- Repository URL: `https://github.com/xibodev/video-agent-bundle.git`
- Captured HEAD: `ff272c2812008e6563eddd6bab981513c15c460a`

## Evidence Commands

- `repository-url.txt`: `git -C "C:\Users\gafar\AppData\Local\Temp\opencode\video-agent-bundle" remote get-url origin`
- `remotes.txt`: `git -C "C:\Users\gafar\AppData\Local\Temp\opencode\video-agent-bundle" remote -v`
- `head.txt`: `git -C "C:\Users\gafar\AppData\Local\Temp\opencode\video-agent-bundle" rev-parse HEAD`
- `status-v2.txt`: `git -C "C:\Users\gafar\AppData\Local\Temp\opencode\video-agent-bundle" status --porcelain=v2 --untracked-files=all`
- `tracked.diff`: `git -C "C:\Users\gafar\AppData\Local\Temp\opencode\video-agent-bundle" diff --binary HEAD --`
- `tracked.stat`: `git -C "C:\Users\gafar\AppData\Local\Temp\opencode\video-agent-bundle" diff --stat HEAD --`
- `tracked.numstat`: `git -C "C:\Users\gafar\AppData\Local\Temp\opencode\video-agent-bundle" diff --numstat HEAD --`
- `tracked.summary`: `git -C "C:\Users\gafar\AppData\Local\Temp\opencode\video-agent-bundle" diff --summary HEAD --`
- `untracked-paths.txt`: `git -C "C:\Users\gafar\AppData\Local\Temp\opencode\video-agent-bundle" ls-files --others --exclude-standard`
- `hashes.tsv` hashes: PowerShell `Get-FileHash -Algorithm SHA256 -LiteralPath <donor-candidate>`; state determined with `git -C "C:\Users\gafar\AppData\Local\Temp\opencode\video-agent-bundle" ls-files --error-unmatch -- <path>`, `git -C "C:\Users\gafar\AppData\Local\Temp\opencode\video-agent-bundle" cat-file -e HEAD:<path>`, and `git -C "C:\Users\gafar\AppData\Local\Temp\opencode\video-agent-bundle" diff --quiet HEAD -- <path>`.
- All Git evidence commands ran with `GIT_OPTIONAL_LOCKS=0` and their stdout was redirected into the named snapshot file.
- `checksums.sha256`: PowerShell `Get-FileHash -Algorithm SHA256`, sorted by relative filename, excluding `checksums.sha256`.

## Donor Immutability

- This was a read-only capture. No donor file was edited, cleaned, reset, staged, or committed.
- Candidate contents were not opened for review; present candidates were read only by SHA-256 hashing.
- `E:\testgrounds\product-video-factory` was not inspected or accessed.
- Retained evidence records and checksums the pre/post HEAD, complete
  porcelain-v2 status output, tracked diff evidence, untracked paths, and
  candidate file hashes. No retained Git index hash or recursive donor inventory
  evidence is claimed.

## Candidate Classifications

- `committed HEAD`: tracked at HEAD and unchanged in the current worktree/index.
- `tracked modification`: tracked now and different from HEAD, including additions or staged/unstaged changes.
- `untracked`: not tracked by Git.
- `behavioral reference` and `test reference` are intended review classifications only; no salvage decision has been made.

## Counts

- Status lines: 26
- Tracked diff lines: 47930
- Tracked stat lines: 19
- Tracked numstat lines: 18
- Tracked summary lines: 0
- Untracked paths: 8
- Candidates present: 12
- Candidates missing: 0
- Source states: committed HEAD=0; tracked modification=8; untracked=4
- Review classifications: behavioral reference=6; test reference=6

## Pre/Post Verification

- Pre HEAD: `ff272c2812008e6563eddd6bab981513c15c460a`
- Post HEAD: `ff272c2812008e6563eddd6bab981513c15c460a`
- HEAD unchanged: True
- Full status unchanged: True
- Retained tracked diff, untracked-path, and candidate-hash evidence is listed
  above and covered by `checksums.sha256`.
- No conclusion is recorded here from unretained index or recursive inventory
  evidence.
