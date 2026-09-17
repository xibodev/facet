"""Native package smoke test. No host authentication or media-provider calls."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import zipfile


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--os", required=True)
    parser.add_argument("--arch", required=True)
    parser.add_argument("--release-dir", required=True)
    args = parser.parse_args()
    version = json.loads(Path("package.json").read_text())["version"]
    release = Path(args.release_dir).resolve()
    suffix = ".exe" if args.os == "windows" else ""
    installer = release / f"facet-install-{version}-{args.os}-{args.arch}{suffix}"
    subprocess.run([str(installer), "--help"], check=True)
    archive = release / f"facet-{version}-{args.os}-{args.arch}.zip"
    with tempfile.TemporaryDirectory(prefix="facet package smoke ") as temp:
        temp = Path(temp)
        with zipfile.ZipFile(archive) as z:
            names = z.namelist()
            assert "bundle/skills/facet/SKILL.md" in names
            assert not any("node_modules/" in n or "/.env" in n for n in names)
            binary = "bin/facet" + suffix
            z.extract(binary, temp)
            (temp / binary).chmod(0o755)
        result = subprocess.check_output([str(temp / binary), "version"], text=True).strip()
        assert result == "facet v" + version, result
        # Reuse CI-provisioned FFmpeg; installer must not install system software.
        if os.environ.get("FACET_INSTALL_SMOKE") == "1":
            for host, config in {"opencode": ".opencode", "codex": ".agents", "claude": ".claude", "copilot": ".github"}.items():
                project = temp / host
                project.mkdir()
                (project / "AGENTS.md").write_text("Keep user instructions.")
                subprocess.run([str(installer), "--yes", "--target", host, "--project", str(project),
                                "--install-dir", str(temp / "release"), "--archive", str(archive),
                                "--checksums", str(release / f"checksums-{args.os}-{args.arch}.txt"),
                                "--components", "none"], check=True)
                assert (project / config / "skills/facet/SKILL.md").is_file()
                assert (project / "AGENTS.md").read_text() == "Keep user instructions."
                assert (project / ".facet-install/packs/explainer/SKILL.md").is_file()
                assert not (project / config / "skills/facet/packs").exists()
    for line in (release / f"checksums-{args.os}-{args.arch}.txt").read_text().splitlines():
        digest, name = line.split()
        assert hashlib.sha256((release / name).read_bytes()).hexdigest() == digest
    print("Native binary, installer help, bundle contents and checksums passed.")


if __name__ == "__main__":
    main()
