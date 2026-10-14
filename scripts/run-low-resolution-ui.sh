#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

OPENCV_ROOT="$ROOT/build/opencv-4.14.0/install"
OPENCV_CMAKE="$OPENCV_ROOT/lib/cmake/opencv4"
BUILD_DIR="$ROOT/build/low-resolution-benchmark"
BENCHMARK_BIN="$BUILD_DIR/faceproof_low_resolution_benchmark"

if ! command -v go >/dev/null 2>&1 || ! command -v cmake >/dev/null 2>&1; then
    echo "Go and CMake are required." >&2
    exit 1
fi
if [ ! -f "$OPENCV_CMAKE/OpenCVConfig.cmake" ]; then
    echo "OpenCV 4.14 unavailable. Run: bash scripts/build-opencv-parity.sh" >&2
    exit 1
fi
if [ ! -x "$BENCHMARK_BIN" ]; then
    cmake -S "$ROOT/faceproof-biometric-core" -B "$BUILD_DIR" \
        -DCMAKE_BUILD_TYPE=Release \
        -DFACEPROOF_BIOMETRIC_WITH_OPENCV=ON \
        -DFACEPROOF_BIOMETRIC_BUILD_CLI=ON \
        -DOpenCV_DIR="$OPENCV_CMAKE"
    cmake --build "$BUILD_DIR" --target faceproof_low_resolution_benchmark --parallel 2
fi

export LD_LIBRARY_PATH="$OPENCV_ROOT/lib:$(printenv LD_LIBRARY_PATH || true)"
cd "$ROOT/services/api"
exec go run ./cmd/lowresolutionui \
    -binary "$BENCHMARK_BIN" \
    -yunet "$ROOT/models/yunet/face_detection_yunet_2023mar.onnx" \
    -sface "$ROOT/models/sface/face_recognition_sface_2021dec.onnx" \
    "$@"
