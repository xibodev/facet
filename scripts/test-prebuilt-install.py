"""Release package and installer lifecycle test. No host authentication,
media-provider calls, system package installs or PATH changes.

Always: checks the platform archive and the installer archive against the 2.0
release contract (one `facet` binary, the allowlisted composer sources under
dependencies/remotion-composer, generated third-party notices, the four
installer files, checksums) and
runs the binary when the archive targets this machine.

With FACET_INSTALL_SMOKE=1 it also runs the platform's real installer, taken
from the installer archive, against the native archive. Every installer run
gets a throwaway user profile: HOME, USERPROFILE, APPDATA, LOCALAPPDATA and the
CLI configuration variables point into a temporary directory, for the child
process only. Covered: install, reuse, repair of a modified runtime, update to
a second build and rollback (when Go is available to build it), the
interactive plain menus (Windows), wiring through `facet wire` into a scratch
project, uninstall with and without --purge, a v1 runtime left untouched, and
rejection of bad checksums, unsafe archives and invalid options. Every run
uses --no-path and no optional components. FFmpeg must be on PATH unless
FACET_INSTALL_SKIP_MEDIA=1, which passes --skip-verify.

On Windows, FACET_TEST_POWERSHELL selects the host (default pwsh; CI also runs
powershell.exe, Windows PowerShell 5.1).
"""
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import platform
import shutil
import stat
import subprocess
import sys
import tempfile
import zipfile

REPO = Path(__file__).resolve().parent.parent
RUN_TIMEOUT = 600
INSTALLER_FILES = {"install.ps1", "install.sh", "installer/manifest.tsv", "installer/verify.html"}
COMPOSER_METADATA = ("package.json", "package-lock.json", "tsconfig.json", "composer-manifest.json")
ISOLATED_VARIABLES = (
    "FACET_ACTION", "FACET_VERSION", "FACET_COMPONENTS", "FACET_WIRE", "FACET_SCOPE", "FACET_PROJECT",
    "FACET_YES", "FACET_NO_PATH", "FACET_SKIP_VERIFY", "FACET_PURGE", "FACET_PLAIN", "FACET_LOG_DIR",
    "FACET_HOME", "FACET_REMOTION_COMPOSER",
    "CLAUDE_CONFIG_DIR", "CODEX_HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME",
)
VALUE_FLAGS = {
    "action": ("-Action", "--action"),
    "version": ("-Version", "--version"),
    "components": ("-Components", "--components"),
    "archive": ("-ArchivePath", "--archive"),
    "checksums": ("-ChecksumPath", "--checksums"),
    "wire": ("-Wire", "--wire"),
    "scope": ("-Scope", "--scope"),
    "project": ("-ProjectDir", "--project"),
}
SWITCH_FLAGS = {
    "yes": ("-NonInteractive", "--yes"),
    "no_path": ("-NoPath", "--no-path"),
    "skip_verify": ("-SkipVerify", "--skip-verify"),
    "purge": ("-Purge", "--purge"),
}


def sha256(path):
    digest = hashlib.sha256()
    with open(path, "rb") as stream:
        for block in iter(lambda: stream.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()


def host_platform():
    machine = platform.machine().lower()
    arch = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(machine, machine)
    return {"win32": "windows", "darwin": "darwin"}.get(sys.platform, "linux"), arch


def check_archives(release, version, os_name, arch, updated_installer):
    suffix = ".exe" if os_name == "windows" else ""
    archive = release / f"facet-{version}-{os_name}-{arch}.zip"
    installer = release / f"facet-installer-{version}.zip"
    manifest = json.loads((REPO / "remotion-composer" / "composer-manifest.json").read_text(encoding="utf-8"))
    composer = {f"dependencies/remotion-composer/{name}" for name in (*COMPOSER_METADATA, *manifest["allowedSourcePaths"])}
    expected = {"bin/facet" + suffix, "LICENSE", "THIRD_PARTY_NOTICES.md"} | composer
    with zipfile.ZipFile(archive) as z:
        names = {name for name in z.namelist() if not name.endswith("/")}
        assert names == expected, f"{archive.name}: unexpected {sorted(names - expected)}, missing {sorted(expected - names)}"
        heading = f"## Go modules linked into the `facet` binary ({os_name}/{arch})"
        assert heading in z.read("THIRD_PARTY_NOTICES.md").decode("utf-8"), "notices were not generated for this platform"
        assert z.getinfo("bin/facet" + suffix).external_attr >> 16 & 0o111, "binary is not executable"
    with zipfile.ZipFile(installer) as z:
        names = {name for name in z.namelist() if not name.endswith("/")}
        assert names == INSTALLER_FILES, f"{installer.name} ships {sorted(names)}"
        rows = [line.split("\t") for line in z.read("installer/manifest.tsv").decode("utf-8").splitlines()]
        assert [row[2] for row in rows if row[:2] == ["release", "facet"]] == [version], "installer manifest is not stamped with the release version"
        assert z.read("install.ps1").isascii(), "install.ps1 must stay ASCII for Windows PowerShell 5.1"
        assert b"\r\n" not in z.read("install.sh"), "install.sh must use LF line endings"
    for line in (release / f"checksums-{os_name}-{arch}.txt").read_text(encoding="utf-8").splitlines():
        digest, name = line.split()
        if updated_installer and name == installer.name:
            digest, name = (release / "installer-checksums.txt").read_text(encoding="utf-8").split()
        assert sha256(release / name) == digest, f"checksum mismatch: {name}"
    return archive, installer


class Lifecycle:
    def __init__(self, os_name, arch, version, archive, checksums, scripts, root):
        self.os_name, self.arch, self.version = os_name, arch, version
        self.windows = os_name == "windows"
        self.suffix = ".exe" if self.windows else ""
        self.archive, self.checksums, self.scripts, self.root = archive, checksums, scripts, root
        self.skip_media = os.environ.get("FACET_INSTALL_SKIP_MEDIA") == "1"
        self.shell = os.environ.get("FACET_TEST_POWERSHELL", "pwsh")
        self.home = None
        # FACET_HOME for the installer, when a test moves Facet's home folder.
        self.moved_home = None

    def new_home(self, name):
        self.home = self.root / name
        self.moved_home = None
        for directory in (self.home / "AppData" / "Roaming", self.home / "AppData" / "Local", self.home / ".config"):
            directory.mkdir(parents=True)
        return self.home

    @property
    def facet_home(self):
        return self.moved_home or self.home / ".facet"

    def runtime(self, version=None):
        return self.facet_home / "runtimes" / f"{version or self.version}-{self.os_name}-{self.arch}"

    def environment(self):
        env = {key: value for key, value in os.environ.items() if key not in ISOLATED_VARIABLES}
        env.update(HOME=str(self.home), USERPROFILE=str(self.home),
                   APPDATA=str(self.home / "AppData" / "Roaming"), LOCALAPPDATA=str(self.home / "AppData" / "Local"))
        if self.moved_home:
            env.update(FACET_HOME=str(self.moved_home))
        if not self.windows:
            env.update(XDG_CONFIG_HOME=str(self.home / ".config"), XDG_DATA_HOME=str(self.home / ".local" / "share"),
                       XDG_CACHE_HOME=str(self.home / ".cache"))
        return env

    def install(self, expect_success=True, interactive=None, **options):
        options.setdefault("archive", self.archive)
        options.setdefault("checksums", self.checksums)
        if options.get("archive") is None:
            options.pop("archive")
            options.pop("checksums", None)
        if options.get("action") in ("rollback", "uninstall"):
            options.pop("archive", None)
            options.pop("checksums", None)
        else:
            options.setdefault("components", "none")
            options.setdefault("skip_verify", self.skip_media)
        options.setdefault("no_path", True)
        options.setdefault("yes", interactive is None)
        index = 0 if self.windows else 1
        if self.windows:
            command = [self.shell, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", str(self.scripts / "install.ps1")]
        else:
            command = ["bash", str(self.scripts / "install.sh")]
        for key, value in options.items():
            if key in SWITCH_FLAGS:
                if value:
                    command.append(SWITCH_FLAGS[key][index])
            elif value is not None:
                command += [VALUE_FLAGS[key][index], str(value)]
        result = subprocess.run(command, input=interactive or "", text=True, capture_output=True, env=self.environment(),
                                cwd=self.root, timeout=RUN_TIMEOUT)
        output = result.stdout + result.stderr
        if (result.returncode == 0) != expect_success:
            raise AssertionError(f"installer exited {result.returncode} for {command[-12:]}:\n{output}")
        return output

    def current(self):
        path = self.facet_home / "current"
        if not os.path.lexists(path):
            return None
        info = os.lstat(path)
        if self.windows:
            assert getattr(info, "st_reparse_tag", 0) == stat.IO_REPARSE_TAG_MOUNT_POINT, "~/.facet/current is not a junction"
        else:
            assert stat.S_ISLNK(info.st_mode), "~/.facet/current is not a symbolic link"
        return Path(os.path.realpath(path))

    def assert_active(self, version):
        runtime = self.runtime(version)
        assert self.current() == Path(os.path.realpath(runtime)), f"current is {self.current()}, expected {runtime}"
        facet = self.facet_home / "current" / "bin" / ("facet" + self.suffix)
        reported = subprocess.run([str(facet), "version"], capture_output=True, text=True, timeout=60, env=self.environment())
        assert reported.returncode == 0 and reported.stdout.strip() == f"facet v{version}", reported
        record = json.loads((runtime / "components.json").read_text(encoding="utf-8"))
        assert record["version"] == version and record["components"] == [], record
        # The composer is a media dependency of the runtime, beside the others.
        assert (runtime / "dependencies" / "remotion-composer" / "package.json").is_file(), "composer not under dependencies/"
        for retired in ("bin", "bundle"):
            assert not (self.facet_home / retired).exists(), f"retired ~/.facet/{retired} layout created"
            assert not (runtime / "bundle").exists(), "retired <runtime>/bundle layout created"

    def wired_versions(self):
        registry = self.facet_home / "wiring.json"
        if not registry.exists():
            return {}
        return {(w["cli"], w["scope"]): w["facet_version"] for w in json.loads(registry.read_text(encoding="utf-8"))["wirings"]}

    def assert_no_profile_edits(self):
        if self.windows:
            return
        for profile in (".profile", ".bashrc", ".bash_profile", ".bash_login", ".zshrc", ".zprofile", ".zshenv",
                        ".config/fish/config.fish", ".config/fish/conf.d/facet.fish"):
            assert not (self.home / profile).exists(), f"--no-path wrote {profile}"


def user_path():
    """The Windows user PATH, read only, to prove -NoPath leaves it alone."""
    import winreg
    try:
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, "Environment") as key:
            return winreg.QueryValueEx(key, "Path")[0]
    except FileNotFoundError:
        return None


def build_variant(life, version, out):
    """A second release build of this platform, differing in its version."""
    if not shutil.which("go"):
        return None
    binary = out / ("facet" + life.suffix)
    subprocess.run(["go", "build", "-trimpath", "-ldflags", f"-X main.Version={version}", "-o", str(binary), "./cmd/facet"],
                   cwd=REPO, env=dict(os.environ, GOOS=life.os_name, GOARCH=life.arch, CGO_ENABLED="0"), check=True, timeout=RUN_TIMEOUT)
    variant = out / f"facet-{version}-{life.os_name}-{life.arch}.zip"
    with zipfile.ZipFile(life.archive) as source, zipfile.ZipFile(variant, "w") as target:
        for info in source.infolist():
            target.writestr(info, binary.read_bytes() if info.filename == "bin/facet" + life.suffix else source.read(info))
    sums = out / f"checksums-{version}.txt"
    sums.write_text(f"{sha256(variant)}  {variant.name}\n", encoding="utf-8")
    return variant, sums


def lifecycle(life, temp):
    path_before = user_path() if life.windows else None
    home = life.new_home("home")
    v1 = home / ".facet" / "releases" / f"1.1.0-{life.os_name}-{life.arch}"
    v1.mkdir(parents=True)
    (v1 / "marker.txt").write_text("v1 runtime", encoding="utf-8")

    output = life.install()
    life.assert_active(life.version)
    assert "v1 project integrations are separate" in output and "--action uninstall" in output, output
    life.assert_no_profile_edits()
    doctor = subprocess.run([str(life.facet_home / "current" / "bin" / ("facet" + life.suffix)), "doctor"], cwd=temp,
                            capture_output=True, text=True, timeout=120, env=life.environment())
    assert doctor.returncode == 0, doctor

    output = life.install()
    assert "Reusing installed runtime" in output or "reusing" in output.lower(), output
    assert sorted(p.name for p in (life.facet_home / "runtimes").iterdir() if not p.name.startswith(".")) == [life.runtime().name]

    if life.windows:
        # Plain menus with numbered answers: 1 selects install in the action
        # menu (the active runtime exists), then no components, confirmation,
        # and no wiring when CLIs are detected.
        output = life.install(interactive="1\nnone\ny\nnone\n", components=None)
        assert "What should the installer do?" in output and "Action:      install" in output, output
        life.assert_active(life.version)

    binary = life.runtime() / "bin" / ("facet" + life.suffix)
    original = sha256(binary)
    with open(binary, "ab") as stream:
        stream.write(b"modified")
    life.install()
    assert sha256(binary) == original, "a modified runtime was not repaired"
    life.assert_active(life.version)

    variant = build_variant(life, f"{life.version}-rollback.1", temp)
    if variant:
        other = f"{life.version}-rollback.1"
        life.install(action="update", version=other, archive=variant[0], checksums=variant[1])
        life.assert_active(other)
        assert life.runtime().is_dir(), "update removed the previous runtime"
        life.install(action="rollback")
        life.assert_active(life.version)
        life.install(action="rollback")
        life.assert_active(other)
        life.install(action="rollback", version=life.version)
        life.assert_active(life.version)
    else:
        print("Go is unavailable: update and rollback between two builds were not exercised.")
    life.install(action="rollback", version="9.9.9", expect_success=False)

    project = temp / "wired project"
    project.mkdir()
    output = life.install(wire="opencode", scope="project", project=project)
    skills = list((project / ".opencode").rglob("SKILL.md"))
    assert skills, f"facet wire installed no skills:\n{output}"
    assert '"facet"' in (project / "opencode.json").read_text(encoding="utf-8"), output
    registry = json.loads((life.facet_home / "wiring.json").read_text(encoding="utf-8"))
    assert any(w["cli"] == "opencode" and w["scope"] == "project" for w in registry["wirings"]), registry

    if variant:
        # Every install, update and rollback refreshes the recorded wirings,
        # so a CLI's Facet guidance always matches the active runtime.
        life.install(action="update", version=other, archive=variant[0], checksums=variant[1])
        life.assert_active(other)
        assert life.wired_versions() == {("opencode", "project"): other}, life.wired_versions()
        life.install(action="rollback")
        life.assert_active(life.version)
        assert life.wired_versions() == {("opencode", "project"): life.version}, life.wired_versions()
        # A third build: only the active runtime and the one before it are kept.
        third = build_variant(life, f"{life.version}-prune.1", temp)
        newest = f"{life.version}-prune.1"
        life.install(action="update", version=newest, archive=third[0], checksums=third[1])
        kept = sorted(p.name for p in (life.facet_home / "runtimes").iterdir() if not p.name.startswith("."))
        assert kept == sorted([life.runtime().name, life.runtime(newest).name]), f"runtimes kept: {kept}"
        life.install(action="rollback")
        life.assert_active(life.version)

    output = life.install(action="uninstall")
    assert not os.path.lexists(life.facet_home / "current"), "uninstall left ~/.facet/current"
    assert life.runtime().is_dir(), "uninstall without --purge removed the runtime"
    assert not list((project / ".opencode").rglob("SKILL.md")), f"uninstall left wired skills:\n{output}"
    config = project / "opencode.json"
    assert not config.exists() or '"facet"' not in config.read_text(encoding="utf-8"), output
    assert "v1 project integrations are separate" in output, output

    life.install()
    life.assert_active(life.version)
    life.install(action="uninstall", purge=True)
    assert not (life.facet_home / "runtimes").exists(), "--purge kept the runtimes"
    assert not os.path.lexists(life.facet_home / "current")
    assert (v1 / "marker.txt").read_text(encoding="utf-8") == "v1 runtime", "the v1 runtime was touched"
    life.assert_no_profile_edits()
    if life.windows:
        assert user_path() == path_before, "-NoPath changed the user PATH"


def moved_home(life, temp):
    """FACET_HOME moves the whole installation: nothing lands in ~/.facet."""
    home = life.new_home("moved")
    life.moved_home = temp / "moved facet home"
    life.install()
    life.assert_active(life.version)
    assert not (home / ".facet").exists(), "the installer wrote ~/.facet although FACET_HOME is set"
    life.install(action="uninstall", purge=True)
    assert not (life.facet_home / "runtimes").exists() and not os.path.lexists(life.facet_home / "current")


def rejections(life, temp):
    life.new_home("rejections")
    runtimes = life.facet_home / "runtimes"
    bad_sums = temp / "bad-sums.txt"
    bad_sums.write_text("0" * 64 + "  " + life.archive.name + "\n", encoding="utf-8")
    life.install(checksums=bad_sums, expect_success=False)
    assert not life.runtime().exists() and not os.path.lexists(life.facet_home / "current")
    for entry in ["../escaped.txt", "/absolute.txt", "C:/escape.txt", "symlink", "duplicate"]:
        unsafe = temp / "unsafe.zip"
        with zipfile.ZipFile(unsafe, "w") as z:
            if entry == "symlink":
                info = zipfile.ZipInfo("bin/link")
                info.create_system = 3
                info.external_attr = 0o120777 << 16
                z.writestr(info, "../../escaped.txt")
            elif entry == "duplicate":
                z.writestr("a.txt", "first")
                z.writestr("A.txt", "second")
            else:
                z.writestr(entry, "must not escape")
        sums = temp / "unsafe-sums.txt"
        sums.write_text(f"{sha256(unsafe)}  {life.archive.name}\n", encoding="utf-8")
        life.install(archive=unsafe, checksums=sums, expect_success=False)
        assert not life.runtime().exists() and not os.path.lexists(life.facet_home / "current"), entry
        leftovers = [p.name for p in runtimes.iterdir()] if runtimes.exists() else []
        assert not leftovers, f"{entry}: left {leftovers}"
        assert not (runtimes.parent / "escaped.txt").exists() and not (temp / "escaped.txt").exists(), entry
    life.install(checksums=None, expect_success=False)
    life.install(components="bogus", expect_success=False)
    life.install(action="rollback", expect_success=False)
    assert not os.path.lexists(life.facet_home / "current")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--os", required=True)
    parser.add_argument("--arch", required=True)
    parser.add_argument("--release-dir", required=True)
    parser.add_argument("--updated-installer", action="store_true", help="use the installer-only build checksum while keeping the product archive checksum")
    args = parser.parse_args()
    version = json.loads((REPO / "package.json").read_text(encoding="utf-8"))["version"]
    release = Path(args.release_dir).resolve()
    archive, installer = check_archives(release, version, args.os, args.arch, args.updated_installer)
    native = host_platform() == (args.os, args.arch)
    with tempfile.TemporaryDirectory(prefix="facet package test ") as temp:
        temp = Path(temp)
        suffix = ".exe" if args.os == "windows" else ""
        if native:
            with zipfile.ZipFile(archive) as z:
                z.extract("bin/facet" + suffix, temp / "extracted")
            binary = temp / "extracted" / "bin" / ("facet" + suffix)
            binary.chmod(0o755)
            reported = subprocess.check_output([str(binary), "version"], text=True, timeout=60).strip()
            assert reported == "facet v" + version, reported
        if os.environ.get("FACET_INSTALL_SMOKE") == "1":
            if not native:
                raise SystemExit(f"FACET_INSTALL_SMOKE needs a {args.os}/{args.arch} host")
            if os.environ.get("FACET_INSTALL_SKIP_MEDIA") != "1" and not (shutil.which("ffmpeg") and shutil.which("ffprobe")):
                raise SystemExit("FFmpeg and FFprobe must be on PATH for the lifecycle test (or set FACET_INSTALL_SKIP_MEDIA=1)")
            with zipfile.ZipFile(installer) as z:
                z.extractall(temp / "installer")
            life = Lifecycle(args.os, args.arch, version, archive, release / f"checksums-{args.os}-{args.arch}.txt", temp / "installer", temp)
            lifecycle(life, temp)
            moved_home(life, temp)
            rejections(life, temp)
            print("Installer lifecycle passed: install, reuse, repair, update/rollback, wiring refresh, pruning, "
                  "FACET_HOME, uninstall, purge and rejections.")
    print("Native binary, script installer package, composer contents and checksums passed.")


if __name__ == "__main__":
    main()
