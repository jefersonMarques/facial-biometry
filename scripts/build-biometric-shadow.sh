#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OPENCV_VERSION="4.14.0"
ONNXRUNTIME_VERSION="1.30.0"
OPENCV_PREFIX="$ROOT/build/opencv-$OPENCV_VERSION/install"
OPENCV_CONFIG="$OPENCV_PREFIX/lib/cmake/opencv4/OpenCVConfig.cmake"
ONNXRUNTIME_ROOT="$ROOT/build/onnxruntime-$ONNXRUNTIME_VERSION"
SHADOW_BUILD="$ROOT/build/biometric-shadow-linux"
MANIFEST="$ROOT/.dev/native-shadow.json"

for tool in git cmake ninja python3; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        echo "$tool not found." >&2
        exit 1
    fi
done

echo "Preparing pinned OpenCV $OPENCV_VERSION..."
bash scripts/build-opencv-parity.sh

if [ ! -f "$OPENCV_CONFIG" ]; then
    echo "OpenCVConfig.cmake not found after OpenCV build." >&2
    exit 1
fi

echo "Preparing pinned ONNX Runtime $ONNXRUNTIME_VERSION..."
python3 scripts/vendor-onnxruntime.py

if [ ! -f "$ONNXRUNTIME_ROOT/include/onnxruntime_cxx_api.h" ] ||
   [ ! -f "$ONNXRUNTIME_ROOT/lib/libonnxruntime.so" ]; then
    echo "ONNX Runtime native package is incomplete." >&2
    exit 1
fi

python3 models/download_models.py

echo "Configuring FaceProof native biometric shadow..."
rm -rf "$SHADOW_BUILD"

cmake     -S "$ROOT/faceproof-biometric-core"     -B "$SHADOW_BUILD"     -GNinja     -DCMAKE_BUILD_TYPE=Release     -DFACEPROOF_BIOMETRIC_BUILD_TESTS=OFF     -DFACEPROOF_BIOMETRIC_WITH_OPENCV=ON     -DFACEPROOF_BIOMETRIC_WITH_ONNXRUNTIME=ON     -DFACEPROOF_ONNXRUNTIME_ROOT="$ONNXRUNTIME_ROOT"     -DOpenCV_DIR="$OPENCV_CONFIG"

cmake --build "$SHADOW_BUILD" --parallel 2 --target     faceproof_biometric_vision_cli     faceproof_biometric_reference_cli     faceproof_biometric_pad_cli

VISION_CLI="$SHADOW_BUILD/faceproof_biometric_vision_cli"
REFERENCE_CLI="$SHADOW_BUILD/faceproof_biometric_reference_cli"
PAD_CLI="$SHADOW_BUILD/faceproof_biometric_pad_cli"

for executable in "$VISION_CLI" "$REFERENCE_CLI" "$PAD_CLI"; do
    if [ ! -x "$executable" ]; then
        echo "Native shadow executable not found: $executable" >&2
        exit 1
    fi
done

mkdir -p "$ROOT/.dev"

python3 - "$MANIFEST" "$VISION_CLI" "$REFERENCE_CLI" "$PAD_CLI" "$OPENCV_PREFIX/lib" "$ONNXRUNTIME_ROOT/lib" <<'PY'
import json
import sys
from pathlib import Path

(
    manifest_path,
    vision_cli,
    reference_cli,
    pad_cli,
    opencv_lib_dir,
    onnxruntime_lib_dir,
) = sys.argv[1:]

manifest = {
    "schemaVersion": 1,
    "mode": "vision-pad",
    "platform": "linux-x64",
    "openCvVersion": "4.14.0",
    "onnxRuntimeVersion": "1.30.0",
    "visionCli": str(Path(vision_cli).resolve()),
    "referenceCli": str(Path(reference_cli).resolve()),
    "padCli": str(Path(pad_cli).resolve()),
    "openCvLibDir": str(Path(opencv_lib_dir).resolve()),
    "onnxRuntimeLibDir": str(Path(onnxruntime_lib_dir).resolve()),
}
Path(manifest_path).write_text(
    json.dumps(manifest, indent=2) + "\n",
    encoding="utf-8",
)
PY

export LD_LIBRARY_PATH="$OPENCV_PREFIX/lib:$ONNXRUNTIME_ROOT/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"

echo ""
echo "[ok] FaceProof native shadow ready."
echo "[ok] OpenCV: $OPENCV_PREFIX"
echo "[ok] ONNX Runtime: $ONNXRUNTIME_ROOT"
echo "[ok] Vision CLI: $VISION_CLI"
echo "[ok] Reference CLI: $REFERENCE_CLI"
echo "[ok] PAD CLI: $PAD_CLI"
echo "[ok] Manifest: $MANIFEST"
echo ""
echo "Use: ./scripts/run-dev.sh --native-shadow"
