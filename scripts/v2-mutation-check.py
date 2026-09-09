#!/usr/bin/env python3
"""Mutation harness for the v2 projection (operator item 8).

Each mutant must:
  1. LAND -- the anchor text must be present, or the mutation silently did
     nothing and `ok` means nothing.
  2. COMPILE -- a build failure is an INVALID mutant, not a passing guarantee.
     Checked explicitly, because `go test` prints FAIL for a build error too
     and the two are indistinguishable from the exit code alone.
  3. Fail for the SEMANTIC reason -- the failure text is read, not just its
     presence, so an assertion firing for an unrelated reason is not counted.
"""
import subprocess, sys, io, os

ROOT = "E:/open-source-projects/facet"
MUTANTS = [
    ("may_charge weakened",
     "internal/toolbox/v2ops.go",
     "MayCharge:     MayCharge(n),",
     "MayCharge:     false,",
     "may_charge"),
    ("effect omitted (network)",
     "internal/toolbox/v2ops.go",
     "Network:       exec.Network,",
     "Network:       false,",
     "network"),
    ("determinism falsely asserted",
     "internal/toolbox/v2ops.go",
     "Deterministic: Deterministic(n),",
     "Deterministic: true,",
     "determinism"),
    ("cost_known inferred from may_charge",
     "internal/toolbox/v2ops.go",
     "CostKnown:     exec.EstimatedCost != nil,",
     "CostKnown:     !MayCharge(n),",
     "cost_known"),
    ("mandatory Requirement weakened",
     "internal/toolbox/v2ops.go",
     'strength := "required"',
     'strength := "preferred"',
     "strength"),
    ("Resolution collapsed",
     "internal/toolbox/toolbox.go",
     'if kind == "env" {\n\t\treturn ResolutionUnknown\n\t}',
     'if kind == "env" {\n\t\treturn ResolutionSatisfied\n\t}',
     "UNKNOWN"),
    ("produced artifact kind missing",
     "internal/toolbox/v2artifacts.go",
     '"frame":        {Kind: "media", MediaType: "image/jpeg"},',
     "",
     "not a declared artifact kind"),
    ("Operation references undeclared kind",
     "internal/toolbox/v2ops.go",
     '"frame_sample":          {"frame"},',
     '"frame_sample":          {"nonexistent_kind"},',
     "not a declared artifact kind"),
    ("emitted artifact misclassified",
     "internal/toolbox/v2artifacts.go",
     '"captions": {Kind: "text", MediaType: "text/plain"},',
     '"captions": {Kind: "document", MediaType: "text/plain"},',
     "names no validator"),
]

# NO shell quoting: subprocess with shell=True runs cmd.exe on Windows, where
# single quotes become part of the -run pattern and match nothing. The first
# version of this harness did exactly that: every mutant reported "caught for
# the wrong reason" because `go test` said "[no tests to run]" and exited 0.
#
# THE TESTS NEVER RAN AND THE HARNESS REPORTED ON THEM ANYWAY -- the same
# defect this whole audit is about, in the instrument built to detect it.
# Caught only because nine independent mutants failing identically is not a
# plausible shape.
RUN = ["go", "test", "./internal/module/", "-count=1", "-run",
       "SerializedV2|SerializedMayCharge|SerializedEffects|SerializedResolution|"
       "SerializedRequirement|SerializedProduces|SerializedArtifactKinds|"
       "AuthoringSchemas|VideoComposeIsMulti|PublicToolNamesUnchangedByV2|CostKnownComesFromTheEstimate|CostKnownProjectionReads"]

def sh(cmd):
    p = subprocess.run(cmd, shell=isinstance(cmd, str), cwd=ROOT,
                       capture_output=True, text=True)
    out = (p.stdout or "") + (p.stderr or "")
    # "[no tests to run]" is a PASS from go and a silent failure here: it means
    # the filter matched nothing, so the mutant was never exercised.
    if "no tests to run" in out:
        return -1, out + " HARNESS ERROR: the -run filter matched no tests"
    return p.returncode, out

fails = 0
for name, path, anchor, repl, reason in MUTANTS:
    full = os.path.join(ROOT, path)
    original = io.open(full, encoding="utf-8").read()
    if anchor not in original:
        print("  MUTANT DID NOT LAND (anchor absent): %s" % name); fails += 1; continue
    io.open(full, "w", encoding="utf-8", newline="\n").write(original.replace(anchor, repl, 1))

    rc, out = sh("go build ./...")
    if rc != 0:
        print("  INVALID MUTANT (does not compile): %s" % name)
        print("     %s" % out.strip().splitlines()[:1]); fails += 1
        io.open(full, "w", encoding="utf-8", newline="\n").write(original); continue

    rc, out = sh(RUN)
    if rc == 0:
        print("  NOT CAUGHT: %s" % name); fails += 1
    elif reason.lower() not in out.lower():
        print("  CAUGHT FOR THE WRONG REASON: %s (expected %r)" % (name, reason))
        for line in out.splitlines():
            if "_test.go:" in line:
                print("     %s" % line.strip()); break
        fails += 1
    else:
        line = next((l.strip() for l in out.splitlines() if "_test.go:" in l), "")
        print("  caught: %-38s | %s" % (name, line[:96]))

    io.open(full, "w", encoding="utf-8", newline="\n").write(original)

rc, _ = sh("go build ./...")
print("\nrestore builds clean: %s" % (rc == 0))
print("RESULT: %d mutant(s) not properly caught" % fails)
sys.exit(1 if fails else 0)
