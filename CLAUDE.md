# Facet Repository Instructions

Facet equips the user's selected agent with video-production tools and a local Studio.

## Production Requests
- Follow `skills/facet/SKILL.md`, the canonical producer guidance, and the selected pack. That source takes precedence over stale installed skill copies in this checkout.
- Start from the user's idea; explain the proposed approach and providers, and ask only consequential questions. Respect requests to wait before execution.
- Narration, music, captions and creative artifacts depend on the request. Do not impose fixed turn numbers or synthesize unwanted audio.
- Obtain explicit consent before paid generation or publication. Unknown cost is not free. Never substitute mock output for a production asset.
- The agent operates the tools; the human should not have to author JSON requests or assemble the video manually.
- Inspect the actual output and disclose failures, substitutions, timing differences and unverified claims. A successful process exit is not creative acceptance.

## Development And Verification
- Questions, audits and software maintenance are not video-production requests. Do not start media generation in response to them.
- Keep the Go toolbox mechanical and stateless; preserve project files and user configuration.
- Edit canonical guidance and generation templates, not global installed instruction bundles. Project initialization must preserve unmanaged user instructions.
- `AGENTS.md` defines the release-harness authority boundary. Only deterministic sealed evidence can establish a release verdict; local tests and agent summaries cannot override it.
- Run targeted regressions and the documented checks in `docs/operations/repair-verification.md`. Distinguish synthetic integration, live-provider UAT, and certification.
- Do not commit, tag, push or publish without explicit authorization for those actions.
- Keep the local worktree clean. Once work is pushed, remove local scratch and
  stale worktrees rather than leaving them to drift: a dangling worktree holds an
  older state that later gets mistaken for current and regresses the work. Prefer
  one checkout per repository, and verify with `git worktree list` rather than
  assuming. Build output (`dist/`), run scratch (`.quality-run/`) and session
  scaffolding are ignored, not tracked — they are reproducible or local, and a
  tracked copy of either becomes a stale artifact someone trusts.
