"""Package prebuilt Facet and its installer for one OS/architecture (CI only)."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import zipfile


def product_file(name):
    parts = Path(name).parts
    if any(p in {"node_modules", ".git", "out", ".cache"} or p.startswith(".env") for p in parts):
        return False
    return parts[0] in {"skills", "packs", "agents", "schemas", "styles", "pipeline_defs"} or (
        parts[0] == "remotion-composer" and (
            len(parts) > 2 and parts[1] in {"src", "public"}
            or name in {"remotion-composer/package.json", "remotion-composer/package-lock.json", "remotion-composer/tsconfig.json"}
        )
    )


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--os", required=True, choices=["windows", "linux", "darwin"])
    parser.add_argument("--arch", required=True, choices=["amd64", "arm64"])
    parser.add_argument("--out", required=True)
    args = parser.parse_args()
    repo = Path(__file__).resolve().parent.parent
    version = json.loads((repo / "package.json").read_text())["version"]
    out = Path(args.out).resolve()
    out.mkdir(parents=True, exist_ok=True)
    suffix = ".exe" if args.os == "windows" else ""
    env = dict(os.environ, GOOS=args.os, GOARCH=args.arch, CGO_ENABLED="0")
    tracked = subprocess.check_output(["git", "ls-files", "-z"], cwd=repo).decode().split("\0")
    # Include installer changes before committing when checking the package locally.
    with tempfile.TemporaryDirectory(prefix="facet-package-") as temp:
        temp = Path(temp)
        binaries = []
        for command in ["facet", "facet-ui", "facet-module", "facet-install"]:
            path = temp / (command + suffix)
            subprocess.run(["go", "build", "-trimpath", "-ldflags", f"-X main.version={version}", "-o", str(path), f"./cmd/{command}"], cwd=repo, env=env, check=True)
            binaries.append(path)
        installer = out / f"facet-install-{version}-{args.os}-{args.arch}{suffix}"
        installer.write_bytes(binaries[-1].read_bytes())
        installer.chmod(0o755)
        archive = out / f"facet-{version}-{args.os}-{args.arch}.zip"
        with zipfile.ZipFile(archive, "w", compression=zipfile.ZIP_DEFLATED) as z:
            for binary in binaries:
                info = zipfile.ZipInfo("bin/" + binary.name)
                info.create_system = 3
                info.external_attr = (0o100755 << 16)
                z.writestr(info, binary.read_bytes(), compress_type=zipfile.ZIP_DEFLATED)
            for name in sorted(filter(None, tracked)):
                if product_file(name):
                    source = repo / name
                    if source.is_symlink():
                        raise ValueError(f"Refusing bundle symlink: {name}")
                    z.write(source, "bundle/" + name)
            for name in ["LICENSE", "THIRD_PARTY_NOTICES.md", "PROVENANCE.md"]:
                z.write(repo / name, name)
        checksums = out / f"checksums-{args.os}-{args.arch}.txt"
        checksums.write_text("".join(f"{hashlib.sha256(p.read_bytes()).hexdigest()}  {p.name}\n" for p in [archive, installer]), encoding="utf-8")
        print(json.dumps({"archive": str(archive), "installer": str(installer), "checksums": str(checksums)}))


if __name__ == "__main__":
    main()
