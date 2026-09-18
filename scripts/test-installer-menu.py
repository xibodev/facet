"""Drive the real Bash installer with arrow/space keys through a terminal.

Cancel at the install summary: no downloads, package managers or project writes.
"""
import os
from pathlib import Path
import select
import struct
import tempfile
import time


def main():
    import fcntl
    import pty
    import termios

    repo = Path(__file__).resolve().parent.parent
    with tempfile.TemporaryDirectory(prefix="facet menu ") as tmp:
        project = Path(tmp) / "project"
        runtime = Path(tmp) / "runtime"
        pid, fd = pty.fork()
        if pid == 0:
            os.execve("/bin/bash", ["bash", str(repo / "install.sh"), "--install-dir", str(runtime)],
                      dict(os.environ, TERM="xterm-256color", FACET_PLAIN="0", NO_COLOR=""))
        fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
        output = b""
        # Host default depends on PATH. Explicitly walk to the first choice with
        # Home-independent wrapping isn't possible; observe the selected answer.
        prompts = [(b"Enter: confirm | Esc: cancel", b"\x1b[B\n"),
                   (b"Project directory", (str(project) + "\n").encode()),
                   (b"Space: toggle", b"\x1b[B \n"),
                   (b"Continue?", b"n\n")]
        index = 0
        deadline = time.monotonic() + 30
        try:
            while time.monotonic() < deadline:
                if select.select([fd], [], [], .1)[0]:
                    try:
                        output += os.read(fd, 65536)
                    except OSError:
                        pass
                if index < len(prompts) and prompts[index][0] in output:
                    os.write(fd, prompts[index][1])
                    index += 1
                done, status = os.waitpid(pid, os.WNOHANG)
                if done:
                    break
            else:
                os.kill(pid, 9)
                os.waitpid(pid, 0)
                raise AssertionError(f"menu stalled: {output!r}")
        finally:
            os.close(fd)
        assert os.waitstatus_to_exitcode(status) != 0 and index == 4, output
        assert b"Selected: remotion,piper" in output, output
        assert b"Installation cancelled" in output, output
        assert not project.exists() and not runtime.exists()
        assert b"\x1b[?25h" in output, "cursor visibility not restored"
        print("PASS: real terminal arrow selection, Space multi-select, answer summary, cancellation and cursor restoration.")


if __name__ == "__main__":
    main()
