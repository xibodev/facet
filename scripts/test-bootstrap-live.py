"""Download the published installer through the bootstrap, then cancel at its
confirmation. No product/dependency install or host configuration takes place.
"""
import argparse
import os
from pathlib import Path
import subprocess
import tempfile
import time

REPO = Path(__file__).resolve().parent.parent


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--website", action="store_true", help="exercise the deployed one-liner instead of local bootstrap source")
    args = parser.parse_args()
    with tempfile.TemporaryDirectory(prefix="facet bootstrap live ") as root:
        root = Path(root)
        project = root / "project"
        if os.name == "nt":
            # A local Read-Host replacement supplies only cancellation answers;
            # the bootstrap's Invoke-WebRequest uses the real published URL.
            script = root / "verify.ps1"
            script.write_text("""$ErrorActionPreference = 'Stop'
$answers = [Collections.Generic.Queue[string]]::new()
foreach ($answer in @('codex', $env:FACET_TEST_PROJECT, 'none', 'n')) { $answers.Enqueue($answer) }
function Read-Host { param($Prompt); if (-not $answers.Count) { throw 'Unexpected prompt' }; $answers.Dequeue() }
try {
    if ($env:FACET_TEST_WEBSITE -eq '1') {
        irm https://xibodev.github.io/facet/install.ps1 | iex
    } else {
        Get-Content -LiteralPath $env:FACET_TEST_BOOTSTRAP -Raw | iex
    }
    throw 'Expected cancellation'
} catch {
    if ($_.Exception.Message -notmatch 'Installation cancelled') { throw }
}
if ($answers.Count -or (Test-Path -LiteralPath $env:FACET_TEST_PROJECT)) { throw 'Cancellation contract failed' }
""", encoding="utf-8")
            subprocess.run(["pwsh", "-NoProfile", "-File", str(script)], env=dict(os.environ, FACET_TEST_PROJECT=str(project), FACET_TEST_BOOTSTRAP=str(REPO / "docs/install.ps1"), FACET_TEST_WEBSITE="1" if args.website else "0"), check=True, timeout=240)
        else:
            import pty
            import select
            import shlex
            pid, fd = pty.fork()
            if pid == 0:
                os.chdir(root)
                command = "set -o pipefail; " + ("curl -fsSL https://xibodev.github.io/facet/install.sh" if args.website else "cat " + shlex.quote(str(REPO / "docs/install.sh"))) + (" | bash" if args.website else " | sh")
                os.execve("/bin/bash", ["bash", "-c", command], dict(os.environ, FACET_PLAIN="1"))
            prompts = [(b"Which CLI should use Facet?", b"codex\n"), (b"Project directory", (str(project)+"\n").encode()), (b"Optional production tools", b"none\n"), (b"Continue?", b"n\n")]
            output = b""
            index = 0
            deadline = time.monotonic() + 240
            try:
                while time.monotonic() < deadline:
                    if select.select([fd], [], [], .1)[0]:
                        try: output += os.read(fd, 65536)
                        except OSError: pass
                    if index < len(prompts) and prompts[index][0] in output:
                        os.write(fd, prompts[index][1]); index += 1
                    done, status = os.waitpid(pid, os.WNOHANG)
                    if done: break
                else:
                    os.kill(pid, 9); os.waitpid(pid, 0)
                    raise AssertionError(f"live bootstrap timed out: {output!r}")
            finally:
                os.close(fd)
            assert os.waitstatus_to_exitcode(status) != 0 and b"Installation cancelled" in output and index == 4, output
            assert not project.exists()
        print("PASS: published installer checksum, extraction, interactive handoff, and cancellation without project writes.")


if __name__ == "__main__":
    main()
