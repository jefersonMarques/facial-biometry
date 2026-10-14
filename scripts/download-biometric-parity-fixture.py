from __future__ import annotations

import hashlib
import shutil
import sys
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DESTINATION = ROOT / "build" / "biometric-parity" / "astronaut.png"
URL = (
    "https://raw.githubusercontent.com/scikit-image/scikit-image/"
    "533b7694d2004ae84e49e2cfd0bcfc5f8e562f22/"
    "src/_skimage2/data/astronaut.png"
)
EXPECTED_SHA256 = "88431cd9653ccd539741b555fb0a46b61558b301d4110412b5bc28b5e3ea6cb5"


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def verify(path: Path) -> bool:
    return path.is_file() and sha256(path) == EXPECTED_SHA256


def main() -> int:
    DESTINATION.parent.mkdir(parents=True, exist_ok=True)
    if verify(DESTINATION):
        print(f"[ok] parity fixture: {DESTINATION}")
        return 0

    DESTINATION.unlink(missing_ok=True)
    temporary = DESTINATION.with_suffix(".png.part")
    request = urllib.request.Request(URL, headers={"User-Agent": "FaceProof/0.1"})
    try:
        with urllib.request.urlopen(request, timeout=60) as response, temporary.open("wb") as output:
            shutil.copyfileobj(response, output)
        if not verify(temporary):
            raise RuntimeError(
                "fixture verification failed: "
                f"sha256={sha256(temporary) if temporary.exists() else 'missing'}"
            )
        temporary.replace(DESTINATION)
    except Exception as error:
        temporary.unlink(missing_ok=True)
        print(f"Fixture download failed: {error}", file=sys.stderr)
        return 1

    print(f"[ok] parity fixture: {DESTINATION}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
