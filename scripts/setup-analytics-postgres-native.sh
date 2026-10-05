#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEV_DIRECTORY="$ROOT/.dev"
PASSWORD_FILE="$DEV_DIRECTORY/postgres-native-password"
DATABASE_URL_FILE="$DEV_DIRECTORY/analytics-database-url"

if ! command -v psql >/dev/null 2>&1; then
    echo "Installing PostgreSQL in WSL..."
    sudo apt-get update
    sudo apt-get install -y postgresql postgresql-contrib
fi

echo "Starting PostgreSQL..."
sudo service postgresql start >/dev/null

mkdir -p "$DEV_DIRECTORY"
chmod 700 "$DEV_DIRECTORY"

if [ ! -f "$PASSWORD_FILE" ]; then
    python3 -c 'import secrets; print(secrets.token_urlsafe(32))' > "$PASSWORD_FILE"
    chmod 600 "$PASSWORD_FILE"
fi

POSTGRES_PASSWORD="$(cat "$PASSWORD_FILE")"

if sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='faceproof'" | grep -q 1; then
    sudo -u postgres psql -v ON_ERROR_STOP=1         -c "ALTER ROLE faceproof WITH LOGIN PASSWORD '$POSTGRES_PASSWORD';" >/dev/null
else
    sudo -u postgres psql -v ON_ERROR_STOP=1         -c "CREATE ROLE faceproof LOGIN PASSWORD '$POSTGRES_PASSWORD';" >/dev/null
fi

if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='faceproof'" | grep -q 1; then
    sudo -u postgres createdb -O faceproof faceproof
fi

DATABASE_URL="postgres://faceproof:$POSTGRES_PASSWORD@127.0.0.1:5432/faceproof?sslmode=disable"
printf '%s\n' "$DATABASE_URL" > "$DATABASE_URL_FILE"
chmod 600 "$DATABASE_URL_FILE"

PGPASSWORD="$POSTGRES_PASSWORD"     psql -h 127.0.0.1 -U faceproof -d faceproof -tAc "SELECT 1" >/dev/null

echo "[ok] PostgreSQL nativo pronto."
echo "[ok] Database: faceproof"
echo "[ok] URL salva em: $DATABASE_URL_FILE"
echo
echo "Agora execute:"
echo "  ./scripts/run-dev.sh --native-shadow --analytics"
