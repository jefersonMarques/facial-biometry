#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PACK_VERSION="${FACEPROOF_MODEL_PACK_VERSION:-0.1.0}"
PACK_DIR="${FACEPROOF_MODEL_PACK_DIR:-$ROOT/.dev/model-packs}"
PRIVATE_KEY="${FACEPROOF_MODEL_PACK_PRIVATE_KEY_PATH:-$ROOT/.dev/model-pack-private.key}"
PUBLIC_KEY="${FACEPROOF_MODEL_PACK_PUBLIC_KEY_PATH:-$ROOT/.dev/model-pack-public.key}"
OUTPUT="${FACEPROOF_MODEL_PACK_OUTPUT:-$PACK_DIR/faceproof-standard-$PACK_VERSION.fpmp}"
CACHE_DIR="${FACEPROOF_MODEL_PACK_CACHE_DIR:-$ROOT/data/model-cache}"

YUNET="$ROOT/models/yunet/face_detection_yunet_2023mar.onnx"
SFACE="$ROOT/models/sface/face_recognition_sface_2021dec.onnx"
MINIFASNET="$ROOT/models/minifasnet/MiniFASNetV2.onnx"

for model in "$YUNET" "$SFACE" "$MINIFASNET"; do
    if [ ! -s "$model" ]; then
        echo "Required model is missing: $model" >&2
        echo "Prepare the model sources before building the signed Model Pack." >&2
        exit 1
    fi
done

mkdir -p "$PACK_DIR" "$CACHE_DIR" "$ROOT/.dev"
chmod 700 "$ROOT/.dev" "$PACK_DIR" "$CACHE_DIR"

if [ ! -f "$PRIVATE_KEY" ] || [ ! -f "$PUBLIC_KEY" ]; then
    if [ -f "$PRIVATE_KEY" ] || [ -f "$PUBLIC_KEY" ]; then
        echo "Model Pack signing key pair is incomplete; refusing to overwrite it." >&2
        exit 1
    fi

    echo "Generating DEVELOPMENT Ed25519 Model Pack signing key..."
    (
        cd "$ROOT/services/api"
        go run ./cmd/modelpack keygen             --private "$PRIVATE_KEY"             --public "$PUBLIC_KEY"
    )
    chmod 600 "$PRIVATE_KEY"
    chmod 644 "$PUBLIC_KEY"
    echo "[warning] Development signing key created in .dev."
    echo "[warning] Production signing keys must be generated and stored offline."
fi

echo "Building signed FaceProof Model Pack $PACK_VERSION..."
(
    cd "$ROOT/services/api"
    go run ./cmd/modelpack create         --output "$OUTPUT"         --private-key "$PRIVATE_KEY"         --pack-id "faceproof-standard"         --pack-version "$PACK_VERSION"         --secure-core-min-version "0.2.0"         --yunet "$YUNET"         --sface "$SFACE"         --minifasnet "$MINIFASNET"
)

echo "Verifying freshly built Model Pack..."
(
    cd "$ROOT/services/api"
    go run ./cmd/modelpack verify         --pack "$OUTPUT"         --public-key "$PUBLIC_KEY"         --cache-dir "$CACHE_DIR"
)

chmod 600 "$OUTPUT"

echo
echo "[ok] FaceProof Model Pack built."
echo "[ok] Pack: $OUTPUT"
echo "[ok] Public key: $PUBLIC_KEY"
echo "[ok] Private key: stored locally for development only"
