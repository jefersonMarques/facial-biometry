#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEV_DIRECTORY="$ROOT/.dev"
CONTAINER_NAME="${FACEPROOF_ANALYTICS_CONTAINER:-faceproof-postgres-dev}"
POSTGRES_PORT="${FACEPROOF_ANALYTICS_POSTGRES_PORT:-55432}"
PASSWORD_FILE="$DEV_DIRECTORY/postgres-password"

if [ -n "${FACEPROOF_ANALYTICS_DATABASE_URL:-}" ]; then
    echo "[ok] FaceProof analytics: external PostgreSQL configured."
    return 0 2>/dev/null || exit 0
fi

if ! command -v docker >/dev/null 2>&1; then
    echo "Docker not found. Install Docker or set FACEPROOF_ANALYTICS_DATABASE_URL." >&2
    return 1 2>/dev/null || exit 1
fi

mkdir -p "$DEV_DIRECTORY"
chmod 700 "$DEV_DIRECTORY"

if [ ! -f "$PASSWORD_FILE" ]; then
    python3 -c 'import secrets; print(secrets.token_urlsafe(32))' > "$PASSWORD_FILE"
    chmod 600 "$PASSWORD_FILE"
fi

POSTGRES_PASSWORD="$(cat "$PASSWORD_FILE")"

if docker container inspect "$CONTAINER_NAME" >/dev/null 2>&1; then
    if [ "$(docker inspect -f '{{.State.Running}}' "$CONTAINER_NAME")" != "true" ]; then
        echo "Starting existing FaceProof PostgreSQL container..."
        docker start "$CONTAINER_NAME" >/dev/null
    fi
else
    echo "Creating FaceProof PostgreSQL container..."
    docker run -d         --name "$CONTAINER_NAME"         --restart unless-stopped         -e POSTGRES_USER=faceproof         -e POSTGRES_PASSWORD="$POSTGRES_PASSWORD"         -e POSTGRES_DB=faceproof         -p "127.0.0.1:$POSTGRES_PORT:5432"         -v faceproof-postgres-data:/var/lib/postgresql/data         postgres:17-alpine >/dev/null
fi

READY=false
for _ in $(seq 1 40); do
    if docker exec "$CONTAINER_NAME" pg_isready -U faceproof -d faceproof >/dev/null 2>&1; then
        READY=true
        break
    fi
    sleep 1
done

if [ "$READY" != "true" ]; then
    echo "FaceProof PostgreSQL did not become ready." >&2
    return 1 2>/dev/null || exit 1
fi

export FACEPROOF_ANALYTICS_DATABASE_URL="postgres://faceproof:$POSTGRES_PASSWORD@127.0.0.1:$POSTGRES_PORT/faceproof?sslmode=disable"

echo "[ok] FaceProof analytics PostgreSQL: 127.0.0.1:$POSTGRES_PORT"
echo "[ok] Container: $CONTAINER_NAME"
