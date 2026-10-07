#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hashlib
import json
import shutil
import sys
import tempfile
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
LOCK_PATH = ROOT / "tools" / "mediapipe-runtime-lock.json"


def load_lock() -> dict:
    data = json.loads(LOCK_PATH.read_text(encoding="utf-8"))
    if data.get("schemaVersion") != 1:
        raise ValueError("unsupported MediaPipe runtime lock schema")
    files = data.get("files")
    if not isinstance(files, list) or not files:
        raise ValueError("MediaPipe runtime lock has no files")
    for item in files:
        if not isinstance(item, dict):
            raise ValueError("invalid MediaPipe lock entry")
        path = str(item.get("path", "")).strip()
        sha256 = str(item.get("sha256", "")).strip().lower()
        size = item.get("bytes")
        if not path or Path(path).is_absolute() or ".." in Path(path).parts:
            raise ValueError(f"invalid MediaPipe runtime path: {path!r}")
        if len(sha256) != 64 or any(char not in "0123456789abcdef" for char in sha256):
            raise ValueError(f"invalid SHA-256 for {path}")
        if not isinstance(size, int) or size <= 0:
            raise ValueError(f"invalid byte size for {path}")
        if not item.get("source") and not item.get("url"):
            raise ValueError(f"missing source for {path}")
    return data


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def verify_file(path: Path, expected_size: int, expected_sha256: str) -> None:
    if not path.is_file():
        raise FileNotFoundError(str(path))
    actual_size = path.stat().st_size
    if actual_size != expected_size:
        raise ValueError(
            f"{path}: size mismatch: expected {expected_size}, got {actual_size}"
        )
    actual_sha256 = sha256_file(path)
    if actual_sha256 != expected_sha256:
        raise ValueError(
            f"{path}: SHA-256 mismatch: expected {expected_sha256}, got {actual_sha256}"
        )


def source_path(item: dict) -> Path | None:
    relative = item.get("source")
    if not relative:
        return None
    if relative.startswith("node_modules/"):
        return ROOT / "apps" / "web-sdk" / relative
    return ROOT / relative


def verify_package(lock: dict) -> None:
    package_json = (
        ROOT
        / "apps"
        / "web-sdk"
        / "node_modules"
        / "@mediapipe"
        / "tasks-vision"
        / "package.json"
    )
    if not package_json.is_file():
        raise FileNotFoundError(
            "MediaPipe package is not installed. Run npm install in apps/web-sdk."
        )
    package = json.loads(package_json.read_text(encoding="utf-8"))
    expected_version = lock["package"]["version"]
    if package.get("version") != expected_version:
        raise ValueError(
            f"MediaPipe version mismatch: expected {expected_version}, got {package.get('version')}"
        )

    for item in lock["files"]:
        source = source_path(item)
        if source is None:
            continue
        verify_file(source, item["bytes"], item["sha256"])


def fetch_verified(url: str, destination: Path, expected_size: int, expected_sha256: str) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    if destination.is_file():
        try:
            verify_file(destination, expected_size, expected_sha256)
            return
        except (ValueError, FileNotFoundError):
            destination.unlink(missing_ok=True)

    with tempfile.NamedTemporaryFile(
        dir=destination.parent,
        prefix=destination.name + ".",
        suffix=".tmp",
        delete=False,
    ) as temporary:
        temporary_path = Path(temporary.name)

    try:
        request = urllib.request.Request(
            url,
            headers={"User-Agent": "FaceProof-MediaPipe-Vendor/1.0"},
        )
        with urllib.request.urlopen(request, timeout=120) as response, temporary_path.open("wb") as output:
            shutil.copyfileobj(response, output)
        verify_file(temporary_path, expected_size, expected_sha256)
        temporary_path.replace(destination)
    finally:
        temporary_path.unlink(missing_ok=True)


def prepare_runtime(lock: dict) -> None:
    verify_package(lock)
    runtime_root = ROOT / lock["runtimeRoot"]
    runtime_root.mkdir(parents=True, exist_ok=True)

    for item in lock["files"]:
        destination = runtime_root / item["path"]
        source = source_path(item)
        if source is not None:
            destination.parent.mkdir(parents=True, exist_ok=True)
            verify_file(source, item["bytes"], item["sha256"])
            if (
                not destination.is_file()
                or destination.stat().st_size != item["bytes"]
                or sha256_file(destination) != item["sha256"]
            ):
                shutil.copy2(source, destination)
            verify_file(destination, item["bytes"], item["sha256"])
            continue

        fetch_verified(
            str(item["url"]),
            destination,
            item["bytes"],
            item["sha256"],
        )

    manifest = {
        "schemaVersion": 1,
        "package": lock["package"],
        "files": [
            {
                "path": item["path"],
                "bytes": item["bytes"],
                "sha256": item["sha256"],
            }
            for item in lock["files"]
        ],
    }
    (runtime_root / "faceproof-runtime-manifest.json").write_text(
        json.dumps(manifest, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )


def verify_runtime(lock: dict) -> None:
    runtime_root = ROOT / lock["runtimeRoot"]
    for item in lock["files"]:
        verify_file(
            runtime_root / item["path"],
            item["bytes"],
            item["sha256"],
        )


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Prepare and verify the pinned FaceProof MediaPipe runtime."
    )
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--verify-lock", action="store_true")
    mode.add_argument("--verify-package", action="store_true")
    mode.add_argument("--verify-runtime", action="store_true")
    args = parser.parse_args()

    try:
        lock = load_lock()
        if args.verify_lock:
            print("[ok] MediaPipe lock is valid")
            return 0
        if args.verify_package:
            verify_package(lock)
            print("[ok] MediaPipe npm package matches pinned runtime hashes")
            return 0
        if args.verify_runtime:
            verify_runtime(lock)
            print("[ok] self-hosted MediaPipe runtime matches pinned hashes")
            return 0

        prepare_runtime(lock)
        verify_runtime(lock)
        print(f"[ok] MediaPipe runtime: {ROOT / lock['runtimeRoot']}")
        return 0
    except Exception as error:
        print(f"[error] {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
