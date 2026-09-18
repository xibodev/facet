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
    parser.add_argument("--updated-installer", action="store_true", help="use installer-only build checksum while retaining product archive checksum")
    args = parser.parse_args()
    version = json.loads((Path(__file__).resolve().parent.parent / "package.json").read_text())["version"]
    release = Path(args.release_dir).resolve()
    suffix = ".exe" if args.os == "windows" else ""
    installer = release / f"facet-installer-{version}.zip"
    archive = release / f"facet-{version}-{args.os}-{args.arch}.zip"
    with tempfile.TemporaryDirectory(prefix="facet package smoke ") as temp:
        temp = Path(temp)
        with zipfile.ZipFile(installer) as z:
            z.extractall(temp / "installer")
        def install(project, destination, payload=archive, checksums=None, expect_success=True, interactive=None, action="add", components="none", extra_env=None, migrate=False):
            checksums = checksums or release / f"checksums-{args.os}-{args.arch}.txt"
            if args.os == "windows":
                command = [os.environ.get("FACET_TEST_POWERSHELL", "pwsh"), "-NoProfile", "-File", str(temp / "installer/install.ps1"),
                           "-Target", project.name, "-ProjectDir", str(project), "-InstallDir", str(destination),
                           "-ArchivePath", str(payload), "-ChecksumPath", str(checksums), "-Components", components, "-Action", action]
                if interactive is None: command += ["-NonInteractive"]
            else:
                command = ["bash", str(temp / "installer/install.sh"), "--target", project.name,
                           "--project", str(project), "--install-dir", str(destination), "--archive", str(payload),
                           "--checksums", str(checksums), "--components", components, "--action", action]
                if interactive is None: command += ["--yes"]
            if os.environ.get("FACET_INSTALL_SKIP_MEDIA") == "1":
                command += ["-SkipVerify" if args.os == "windows" else "--skip-verify"]
            if migrate: command += ["-MigrateLegacy" if args.os == "windows" else "--migrate-legacy"]
            result = subprocess.run(command, input=interactive, text=True, capture_output=True, env=dict(os.environ, FACET_LOG_DIR=str(temp / "logs"), **(extra_env or {})))
            if (result.returncode == 0) != expect_success:
                raise AssertionError(f"Unexpected installer result {result.returncode}:\n{result.stdout}\n{result.stderr}")
            return result
        with zipfile.ZipFile(archive) as z:
            names = z.namelist()
            assert "bundle/skills/facet/SKILL.md" in names
            assert not any("facet-install" in n for n in names), "Product archive must not ship a compiled installer"
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
                install(project, temp / "release", interactive="none\ny\n" if host == "opencode" else None)
                assert (project / config / "skills/facet/SKILL.md").is_file()
                assert (project / "AGENTS.md").read_text() == "Keep user instructions."
                assert (project / ".facet-install/packs/explainer/SKILL.md").is_file()
                assert not (project / config / "skills/facet/packs").exists()
                rerun = install(project, temp / "release")
                assert "Reusing configured dependencies" in rerun.stdout
                assert "configuration: --" not in rerun.stdout and "[STREAM]" not in rerun.stdout
                launcher = project / ".facet-install" / ("run-facet.ps1" if args.os == "windows" else "run-facet.sh")
                before = launcher.read_bytes()
                repair = install(project, temp / "release", action="repair")
                assert launcher.read_bytes() != before, "Repair did not rebind to isolated runtime"
                assert (temp / "release/bin" / ("facet" + suffix)).exists(), "Repair destroyed shared runtime"
                if host == "opencode":
                    install(project, temp / "release", action="update")
                    assert (project / "AGENTS.md").read_text() == "Keep user instructions."
                skill = project / config / "skills/facet/SKILL.md"
                original = skill.read_bytes()
                skill.write_bytes(original + b"\nUser customization\n")
                install(project, temp / "release", expect_success=False)
                assert skill.read_bytes() == original + b"\nUser customization\n"
                skill.write_bytes(original)
                if host == "claude":
                    ownership = project / ".facet-install" / ("managed-files.json" if args.os == "windows" else "managed-files.sha256")
                    ownership.unlink()
                    (project / ".facet-install/custom-note.txt").write_text("retain legacy customization")
                    install(project, temp / "release", expect_success=False)
                    install(project, temp / "release", migrate=True)
                    backups = list(project.glob(".facet-backup-*/state/custom-note.txt"))
                    assert len(backups) == 1 and backups[0].read_text() == "retain legacy customization"
                if host == "codex":
                    # Fail the dependency subprocess after selecting an addition.
                    # A new runtime generation must be discarded, with the old
                    # project's launcher and working runtime still usable.
                    if os.environ.get("FACET_INSTALL_SKIP_MEDIA") != "1":
                        fake = temp / "failed-npm"
                        fake.mkdir()
                        shim = fake / ("npm.cmd" if args.os == "windows" else "npm")
                        shim.write_text("@echo off\r\nexit /b 23\r\n" if args.os == "windows" else "#!/bin/sh\nexit 23\n")
                        shim.chmod(0o755)
                        before = launcher.read_bytes()
                        generations = set(temp.glob("release-generation-*"))
                        failure = install(project, temp / "release", components="remotion", expect_success=False,
                                          extra_env={"PATH": str(fake) + os.pathsep + os.environ["PATH"]})
                        assert "failed" in (failure.stdout + failure.stderr).lower()
                        assert launcher.read_bytes() == before, "Failed addition replaced project binding"
                        assert set(temp.glob("release-generation-*")) == generations, "Failed addition left a partial generation"
                        install(project, temp / "release")
            bad_sums = temp / "bad-sums.txt"
            bad_sums.write_text("0" * 64 + "  " + archive.name + "\n")
            bad_project = temp / "bad-project" / "codex"
            install(bad_project, temp / "not-created", checksums=bad_sums, expect_success=False)
            assert not (temp / "not-created").exists()
            assert not bad_project.exists()
            for entry in ["../escaped.txt", "/absolute.txt", "C:/escape.txt", "symlink", "duplicate"]:
                unsafe = temp / "unsafe.zip"
                with zipfile.ZipFile(unsafe, "w") as z:
                    if entry == "symlink":
                        info = zipfile.ZipInfo("link")
                        info.create_system = 3
                        info.external_attr = 0o120777 << 16
                        z.writestr(info, "../escaped.txt")
                    elif entry == "duplicate":
                        z.writestr("a.txt", "first")
                        z.writestr("A.txt", "second")
                    else: z.writestr(entry, "must not escape")
                bad_sums.write_text(hashlib.sha256(unsafe.read_bytes()).hexdigest() + "  " + archive.name + "\n")
                install(bad_project, temp / "not-created", payload=unsafe, checksums=bad_sums, expect_success=False)
                assert not (temp / "not-created").exists()
            unmanaged = temp / "unmanaged"
            unmanaged.mkdir()
            (unmanaged / "keep.txt").write_text("preserve")
            install(bad_project, unmanaged, expect_success=False)
            assert (unmanaged / "keep.txt").read_text() == "preserve"
            with (temp / "release/bin" / ("facet" + suffix)).open("ab") as f: f.write(b"changed")
            install(bad_project, temp / "release", expect_success=False)
    for line in (release / f"checksums-{args.os}-{args.arch}.txt").read_text().splitlines():
        digest, name = line.split()
        if args.updated_installer and name == installer.name:
            digest, name = (release / "installer-checksums.txt").read_text().split()
        assert hashlib.sha256((release / name).read_bytes()).hexdigest() == digest
    print("Native binary, script installer package, bundle contents and checksums passed.")


if __name__ == "__main__":
    main()
