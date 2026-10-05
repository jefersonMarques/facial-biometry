#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 2 ] || [ "$#" -gt 6 ]; then
    echo "Usage: $0 <CPF> <YYYY-MM-DD> [expires-in-minutes] [campaign-id] [scenario] [expected-decision]"
    echo "Scenarios: unknown, genuine_live, impostor_live, printed_photo, screen_photo, replay_video"
    exit 2
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CPF="$1"
MINIMUM_DATE="$2"
EXPIRES_MINUTES="${3:-60}"
CAMPAIGN_ID="${4:-}"
SCENARIO="${5:-unknown}"
EXPECTED_DECISION="${6:-}"
API_URL="${FACEPROOF_API_URL:-http://localhost:8080}"

if [ -z "${FACEPROOF_IDENTITY_ISSUER_KEY:-}" ]; then
    KEY_FILE="$ROOT/.dev/identity-issuer-key"
    if [ ! -f "$KEY_FILE" ]; then
        echo "Identity issuer key not found. Run ./scripts/run-dev.sh once or set FACEPROOF_IDENTITY_ISSUER_KEY."
        exit 1
    fi
    FACEPROOF_IDENTITY_ISSUER_KEY="$(cat "$KEY_FILE")"
fi

PAYLOAD="$(python3 - "$CPF" "$MINIMUM_DATE" "$EXPIRES_MINUTES" "$CAMPAIGN_ID" "$SCENARIO" "$EXPECTED_DECISION" <<'PY'
import json
import sys

cpf, minimum_date, expires, campaign_id, scenario, expected = sys.argv[1:]
payload = {
    "cpf": cpf,
    "minimumDocumentDate": minimum_date,
    "expiresInMinutes": int(expires),
    "scenario": scenario,
}
if campaign_id:
    payload["campaignId"] = campaign_id
if expected:
    payload["expectedDecision"] = expected
print(json.dumps(payload, separators=(",", ":")))
PY
)"

curl --fail-with-body --silent --show-error     -X POST "$API_URL/v1/identity/checks"     -H "Authorization: Bearer $FACEPROOF_IDENTITY_ISSUER_KEY"     -H "Content-Type: application/json"     -d "$PAYLOAD"

echo
