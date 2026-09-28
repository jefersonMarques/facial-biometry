#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
    echo "Usage: $0 <CPF> <YYYY-MM-DD> [expires-in-minutes]"
    exit 2
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CPF="$1"
MINIMUM_DATE="$2"
EXPIRES_MINUTES="${3:-60}"
API_URL="${FACEPROOF_API_URL:-http://localhost:8080}"

if [ -z "${FACEPROOF_IDENTITY_ISSUER_KEY:-}" ]; then
    KEY_FILE="$ROOT/.dev/identity-issuer-key"
    if [ ! -f "$KEY_FILE" ]; then
        echo "Identity issuer key not found. Run ./scripts/run-dev.sh once or set FACEPROOF_IDENTITY_ISSUER_KEY."
        exit 1
    fi
    FACEPROOF_IDENTITY_ISSUER_KEY="$(cat "$KEY_FILE")"
fi

curl --fail-with-body --silent --show-error     -X POST "$API_URL/v1/identity/checks"     -H "Authorization: Bearer $FACEPROOF_IDENTITY_ISSUER_KEY"     -H "Content-Type: application/json"     -d "{\"cpf\":\"$CPF\",\"minimumDocumentDate\":\"$MINIMUM_DATE\",\"expiresInMinutes\":$EXPIRES_MINUTES}"

echo
