from __future__ import annotations

import hashlib
import shutil
import sys
import tarfile
import tempfile
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
VERSION = "1.30.0"
ARCHIVE_NAME = f"onnxruntime-linux-x64-{VERSION}.tgz"
URL = (
    f"https://github.com/microsoft/onnxruntime/releases/download/v{VERSION}/"
    f"{ARCHIVE_NAME}"
)
EXPECTED_BYTES = 11306877
EXPECTED_SHA256 = "a5ed5a3cac51fbb2e90da632ae43d19212faaa20e76484e62bcb7c23ddb3b3fd"
DOWNLOAD_DIR = ROOT / "build" / "downloads"
ARCHIVE = DOWNLOAD_DIR / ARCHIVE_NAME
DESTINATION = ROOT / "build" / f"onnxruntime-{VERSION}"


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def verify_archive(path: Path) -> bool:
    return (
        path.is_file()
        and path.stat().st_size == EXPECTED_BYTES
        and sha256(path) == EXPECTED_SHA256
    )


def verify_runtime(path: Path) -> bool:
    return (
        (path / "include" / "onnxruntime_cxx_api.h").is_file()
        and (path / "lib" / "libonnxruntime.so").is_file()
    )


def safe_extract(archive: tarfile.TarFile, destination: Path) -> None:
    root = destination.resolve()
    for member in archive.getmembers():
        target = (destination / member.name).resolve()
        if target != root and root not in target.parents:
            raise RuntimeError(f"unsafe archive path: {member.name}")
    archive.extractall(destination)


def download() -> None:
    DOWNLOAD_DIR.mkdir(parents=True, exist_ok=True)
    if verify_archive(ARCHIVE):
        return

    ARCHIVE.unlink(missing_ok=True)
    temporary = ARCHIVE.with_suffix(".tgz.part")
    request = urllib.request.Request(
        URL,
        headers={"User-Agent": "FaceProof-ONNXRuntime-Vendor/1.0"},
    )
    try:
        with urllib.request.urlopen(request, timeout=120) as response, temporary.open("wb") as output:
            shutil.copyfileobj(response, output)
        if not verify_archive(temporary):
            actual_size = temporary.stat().st_size if temporary.exists() else 0
            actual_sha = sha256(temporary) if temporary.exists() else "missing"
            raise RuntimeError(
                "ONNX Runtime archive verification failed: "
                f"size={actual_size} sha256={actual_sha}"
            )
        temporary.replace(ARCHIVE)
    finally:
        temporary.unlink(missing_ok=True)


def extract() -> None:
    if verify_runtime(DESTINATION):
        return

    shutil.rmtree(DESTINATION, ignore_errors=True)
    DESTINATION.parent.mkdir(parents=True, exist_ok=True)

    with tempfile.TemporaryDirectory(
        prefix="faceproof-ort-",
        dir=DESTINATION.parent,
    ) as temporary_name:
        temporary = Path(temporary_name)
        with tarfile.open(ARCHIVE, "r:gz") as archive:
            safe_extract(archive, temporary)

        roots = [
            item
            for item in temporary.iterdir()
            if item.is_dir() and item.name.startswith("onnxruntime-")
        ]
        if len(roots) != 1:
            raise RuntimeError("unexpected ONNX Runtime archive layout")

        shutil.move(str(roots[0]), str(DESTINATION))

    if not verify_runtime(DESTINATION):
        raise RuntimeError("ONNX Runtime native runtime is incomplete")


def main() -> int:
    try:
        download()
        extract()
    except Exception as error:
        print(f"ONNX Runtime setup failed: {error}", file=sys.stderr)
        return 1

    print(f"[ok] ONNX Runtime {VERSION}: {DESTINATION}")
    print(f"[ok] archive SHA-256: {EXPECTED_SHA256}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
