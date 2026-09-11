from __future__ import annotations

import hashlib
import shutil
import sys
import urllib.request
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class Download:
    name: str
    url: str
    destination: Path
    sha256: str | None = None


ROOT = Path(__file__).resolve().parent

DOWNLOADS = [
    Download(
        name="YuNet",
        url="https://media.githubusercontent.com/media/opencv/opencv_zoo/main/models/face_detection_yunet/face_detection_yunet_2023mar.onnx",
        destination=ROOT / "yunet" / "face_detection_yunet_2023mar.onnx",
        sha256="8f2383e4dd3cfbb4553ea8718107fc0423210dc964f9f4280604804ed2552fa4",
    ),
    Download(
        name="YuNet license",
        url="https://raw.githubusercontent.com/opencv/opencv_zoo/main/models/face_detection_yunet/LICENSE",
        destination=ROOT / "yunet" / "LICENSE",
    ),
    Download(
        name="SFace",
        url="https://media.githubusercontent.com/media/opencv/opencv_zoo/main/models/face_recognition_sface/face_recognition_sface_2021dec.onnx",
        destination=ROOT / "sface" / "face_recognition_sface_2021dec.onnx",
        sha256="0ba9fbfa01b5270c96627c4ef784da859931e02f04419c829e83484087c34e79",
    ),
    Download(
        name="SFace license",
        url="https://raw.githubusercontent.com/opencv/opencv_zoo/main/models/face_recognition_sface/LICENSE",
        destination=ROOT / "sface" / "LICENSE",
    ),
    Download(
        name="MiniFASNet V2",
        url="https://github.com/yakhyo/face-anti-spoofing/releases/download/weights/MiniFASNetV2.onnx",
        destination=ROOT / "minifasnet" / "MiniFASNetV2.onnx",
        sha256="b32929adc2d9c34b9486f8c4c7bc97c1b69bc0ea9befefc380e4faae4e463907",
    ),
    Download(
        name="MiniFASNet license",
        url="https://raw.githubusercontent.com/yakhyo/face-anti-spoofing/main/LICENSE",
        destination=ROOT / "minifasnet" / "LICENSE",
    ),
]


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def download(item: Download) -> None:
    item.destination.parent.mkdir(parents=True, exist_ok=True)
    if item.destination.is_file():
        if item.sha256 is None or sha256(item.destination) == item.sha256:
            print(f"[ok] {item.name}: {item.destination.name}")
            return
        item.destination.unlink()

    temporary = item.destination.with_suffix(item.destination.suffix + ".part")
    print(f"[download] {item.name}")
    request = urllib.request.Request(item.url, headers={"User-Agent": "FaceProof/0.1"})
    with urllib.request.urlopen(request, timeout=60) as response, temporary.open("wb") as output:
        shutil.copyfileobj(response, output)

    if item.sha256 is not None:
        actual = sha256(temporary)
        if actual != item.sha256:
            temporary.unlink(missing_ok=True)
            raise RuntimeError(f"SHA-256 mismatch for {item.name}: {actual}")

    temporary.replace(item.destination)


def main() -> int:
    try:
        for item in DOWNLOADS:
            download(item)
    except Exception as error:
        print(f"Model download failed: {error}", file=sys.stderr)
        return 1

    print("Models are ready.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
