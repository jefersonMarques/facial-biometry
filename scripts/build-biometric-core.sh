#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OPENCV_VERSION="4.14.0"
ONNXRUNTIME_VERSION="1.30.0"
CORE_VERSION="0.2.0"

OPENCV_PREFIX="$ROOT/build/opencv-$OPENCV_VERSION/install"
OPENCV_CMAKE_DIR="$OPENCV_PREFIX/lib/cmake/opencv4"
ONNXRUNTIME_ROOT="$ROOT/build/onnxruntime-$ONNXRUNTIME_VERSION"
CORE_BUILD="$ROOT/build/faceproof-core-linux"
MANIFEST="$ROOT/.dev/biometric-core.json"

for tool in cmake ninja python3 nm; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        echo "$tool not found." >&2
        exit 1
    fi
done

echo "Preparing pinned OpenCV $OPENCV_VERSION..."
bash scripts/build-opencv-parity.sh

echo "Preparing pinned ONNX Runtime $ONNXRUNTIME_VERSION..."
python3 scripts/vendor-onnxruntime.py

python3 models/download_models.py

rm -rf "$CORE_BUILD"

cmake     -S "$ROOT/faceproof-biometric-core"     -B "$CORE_BUILD"     -GNinja     -DCMAKE_BUILD_TYPE=Release     -DFACEPROOF_BIOMETRIC_BUILD_TESTS=OFF     -DFACEPROOF_BIOMETRIC_BUILD_CLI=OFF     -DFACEPROOF_BIOMETRIC_WITH_OPENCV=ON     -DFACEPROOF_BIOMETRIC_WITH_ONNXRUNTIME=ON     -DFACEPROOF_ONNXRUNTIME_ROOT="$ONNXRUNTIME_ROOT"     -DOpenCV_DIR="$OPENCV_CMAKE_DIR"

cmake --build "$CORE_BUILD" --parallel 2 --target faceproof_secure_core

CORE_LIBRARY="$CORE_BUILD/libfaceproof_core.so"
if [ ! -f "$CORE_LIBRARY" ]; then
    echo "libfaceproof_core.so was not produced." >&2
    exit 1
fi

for symbol in     fp_secure_core_abi_version     fp_secure_core_create     fp_secure_core_analyze_identity     fp_secure_core_reference_jpeg     fp_secure_core_guide_jpeg; do
    if ! nm -D "$CORE_LIBRARY" | grep -q " $symbol$"; then
        echo "Required Secure Core symbol not exported: $symbol" >&2
        exit 1
    fi
done

mkdir -p "$ROOT/.dev"
python3 - "$MANIFEST" "$CORE_LIBRARY" "$OPENCV_PREFIX/lib" "$ONNXRUNTIME_ROOT/lib" "$CORE_VERSION" <<'PY'
import hashlib
import json
import sys
from pathlib import Path

manifest_path, library, opencv_lib, onnx_lib, version = sys.argv[1:]
library_path = Path(library).resolve()
digest = hashlib.sha256(library_path.read_bytes()).hexdigest()

manifest = {
    "schemaVersion": 1,
    "platform": "linux-x64",
    "coreVersion": version,
    "abiVersion": 1,
    "libraryPath": str(library_path),
    "librarySha256": digest,
    "openCvVersion": "4.14.0",
    "onnxRuntimeVersion": "1.30.0",
    "openCvLibDir": str(Path(opencv_lib).resolve()),
    "onnxRuntimeLibDir": str(Path(onnx_lib).resolve()),
}

Path(manifest_path).write_text(
    json.dumps(manifest, indent=2) + "\n",
    encoding="utf-8",
)
PY

echo
echo "[ok] FaceProof Secure Core built."
echo "[ok] Library: $CORE_LIBRARY"
echo "[ok] Manifest: $MANIFEST"
echo "[ok] ABI: 1"
echo "[ok] Version: $CORE_VERSION"
