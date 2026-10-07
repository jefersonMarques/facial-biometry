#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="4.14.0"
PREFIX="${FACEPROOF_OPENCV_PREFIX:-$ROOT/build/opencv-$VERSION/install}"
SOURCE_DIR="$ROOT/build/opencv-$VERSION/src"
BUILD_DIR="$ROOT/build/opencv-$VERSION/build"

CONFIG="$PREFIX/lib/cmake/opencv4/OpenCVConfig.cmake"
if [ -f "$CONFIG" ]; then
    echo "[ok] OpenCV $VERSION: $PREFIX"
    exit 0
fi

rm -rf "$SOURCE_DIR" "$BUILD_DIR"
mkdir -p "$(dirname "$SOURCE_DIR")"

git clone     --depth 1     --branch "$VERSION"     https://github.com/opencv/opencv.git     "$SOURCE_DIR"

cmake -S "$SOURCE_DIR" -B "$BUILD_DIR" -GNinja     -DCMAKE_BUILD_TYPE=Release     -DCMAKE_INSTALL_PREFIX="$PREFIX"     -DBUILD_LIST=core,imgproc,imgcodecs,dnn,objdetect     -DBUILD_SHARED_LIBS=ON     -DBUILD_TESTS=OFF     -DBUILD_PERF_TESTS=OFF     -DBUILD_EXAMPLES=OFF     -DBUILD_opencv_apps=OFF     -DBUILD_opencv_python3=OFF     -DBUILD_JAVA=OFF     -DBUILD_opencv_java=OFF     -DWITH_FFMPEG=OFF     -DWITH_GSTREAMER=OFF     -DWITH_GTK=OFF     -DWITH_QT=OFF     -DWITH_OPENCL=OFF     -DWITH_IPP=OFF     -DWITH_TIFF=OFF     -DWITH_OPENEXR=OFF     -DWITH_WEBP=OFF

cmake --build "$BUILD_DIR" --target install --parallel 2

test -f "$CONFIG"
echo "[ok] OpenCV $VERSION: $PREFIX"
