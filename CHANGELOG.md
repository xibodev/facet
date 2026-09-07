# Changelog

## 1.0.2 Candidate (Unreleased)

- Install the full production bundle and locked composer dependencies from source;
  preserve configuration and user-owned skills. Fix CLI help side effects, launch
  error propagation, npm launcher recursion and Windows install discovery. Existing
  Windows shortcuts remain unchanged unless recognized links are explicitly migrated.
- Fail explicitly on missing provider credentials or failed Remotion rendering,
  without substituting mock media. Preserve failed gflow downloads for recovery,
  validate CLI receipts and report unknown provider costs as unknown.
- Support project-local media staging and explicit Explainer export profiles;
  contain the authored canvas at smaller/portrait sizes, retain bounded timeout
  progress, and bind output facts to delivered bytes. Fix replacement-audio editing.
- Repair Studio catalog media links, scoped downloads, production scanning and
  completed-conversation recovery within the same tab and running Studio process.
- Add offline regressions, Linux/Windows CI and Docker UAT helpers with bounded,
  sanitized evidence capture. Clarify installation, provider and production guidance.

Known limitations: no in-flight or server-restart conversation recovery; portrait
Explainer output uses containment/letterboxing. Fresh post-fix paid gflow generation
has not been verified. Hosted-model UAT uses open egress and cannot certify the
sealed network policy. Local checks and prior dirty-source UAT are not a clean-source
release verdict; no sealed PASS or publication is claimed for this candidate.
