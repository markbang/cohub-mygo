#!/usr/bin/env python3
"""Assemble a complete MyGo release, checking all six signed archives first."""

import base64
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile


def prepare(source: Path, dest: Path) -> None:
    config = json.loads(Path("mygo.json").read_text())
    slug, version = config["slug"], config["version"]
    repository = config["updates"]["github"]
    base_url = f"https://github.com/{repository}/releases/download/v{version}/"
    targets = (
        "darwin-arm64", "darwin-amd64", "windows-amd64",
        "windows-arm64", "linux-amd64", "linux-arm64",
    )
    expected = {"install.sh", "install.ps1"}
    for target in targets:
        expected.add(f"update-{target}.json")
        expected.add(f"{slug}-{version}-{target}.tar.gz")
    for arch in ("amd64", "arm64"):
        expected.add(f"{slug}-{version}-{arch}.dmg")
        expected.add(f"{slug}-setup-{version}-{arch}.exe")
        expected.add(f"{slug}_{version}_{arch}.deb")

    files = {}
    for item in sorted(source.rglob("*")):
        if not item.is_file() or item.name not in expected:
            continue
        if item.name in files:
            if item.read_bytes() != files[item.name].read_bytes():
                raise ValueError(f"Conflicting release asset: {item.name}")
        else:
            files[item.name] = item
    missing = expected - files.keys()
    if missing:
        raise ValueError(f"Missing release assets: {', '.join(sorted(missing))}")

    # Ed25519 SubjectPublicKeyInfo, as used by the generated installer.
    public_key = base64.b64decode(config["updates"]["publicKey"], validate=True)
    if len(public_key) != 32:
        raise ValueError("Expected a 32-byte Ed25519 public key")
    with tempfile.TemporaryDirectory() as work:
        key = Path(work) / "public.der"
        key.write_bytes(bytes.fromhex("302a300506032b6570032100") + public_key)
        signature = Path(work) / "signature"
        digest = Path(work) / "digest"
        for target in targets:
            manifest = json.loads(files[f"update-{target}.json"].read_text())
            name = f"{slug}-{version}-{target}.tar.gz"
            archive = files[name]
            if manifest["version"] != version or manifest["url"] != base_url + name:
                raise ValueError(f"Wrong version or URL in {target} manifest")
            if archive.stat().st_size != manifest["size"]:
                raise ValueError(f"Wrong archive size for {target}")
            signature.write_bytes(base64.b64decode(manifest["signature"], validate=True))
            # MyGo signs the archive's SHA-256 digest with Ed25519.
            digest.write_bytes(hashlib.sha256(archive.read_bytes()).digest())
            subprocess.run(
                ["openssl", "pkeyutl", "-verify", "-pubin", "-keyform", "DER",
                 "-inkey", str(key), "-rawin", "-in", str(digest),
                 "-sigfile", str(signature)],
                check=True, capture_output=True,
            )
            print(f"Verified {target}: {archive.stat().st_size:,} bytes")

    dest.mkdir(parents=True, exist_ok=True)
    sums = []
    for name, source_file in sorted(files.items()):
        shutil.copyfile(source_file, dest / name)
        digest = hashlib.sha256(source_file.read_bytes()).hexdigest()
        sums.append(f"{digest}  {name}\n")
    (dest / "SHA256SUMS").write_text("".join(sums))
    print(f"Prepared {len(files)} assets and SHA256SUMS in {dest}")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        sys.exit("Usage: prepare-release.py ARTIFACT_DIRECTORY OUTPUT_DIRECTORY")
    prepare(Path(sys.argv[1]), Path(sys.argv[2]))
