from __future__ import annotations

import hashlib
import shutil
import sys
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DESTINATION = ROOT / "build" / "biometric-parity" / "opencv-zoo-sface-demo.jpg"
URL = (
    "https://media.githubusercontent.com/media/opencv/opencv_zoo/"
    "47534e27c9851bb1128ccc0102f1145e27f23f98/"
    "models/face_recognition_sface/example_outputs/demo.jpg"
)
EXPECTED_SHA256 = "0f879881a598fea6fec74e047e6a1d00e36d81de63bf0ed392b628e6ab6c2fc4"
EXPECTED_BYTES = 156282


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def verify(path: Path) -> bool:
    return (
        path.is_file()
        and path.stat().st_size == EXPECTED_BYTES
        and sha256(path) == EXPECTED_SHA256
    )


def main() -> int:
    DESTINATION.parent.mkdir(parents=True, exist_ok=True)
    if verify(DESTINATION):
        print(f"[ok] parity fixture: {DESTINATION}")
        return 0

    DESTINATION.unlink(missing_ok=True)
    temporary = DESTINATION.with_suffix(".jpg.part")
    request = urllib.request.Request(URL, headers={"User-Agent": "FaceProof/0.1"})
    try:
        with urllib.request.urlopen(request, timeout=60) as response, temporary.open("wb") as output:
            shutil.copyfileobj(response, output)
        if not verify(temporary):
            raise RuntimeError(
                f"fixture verification failed: size={temporary.stat().st_size if temporary.exists() else 0} "
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
