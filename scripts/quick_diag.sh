#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
SMOKE_ADMIN_PASSWORD="${SMOKE_ADMIN_PASSWORD:-}"
POSTGRES_DSN="${POSTGRES_DSN:-postgres://msg:msg@localhost:5432/msg?sslmode=disable}"
DIAG_REPORT="${DIAG_REPORT:-}"

if [[ -n "$DIAG_REPORT" ]]; then
  mkdir -p "$(dirname "$DIAG_REPORT")"
  : > "$DIAG_REPORT"
  exec > >(tee -a "$DIAG_REPORT") 2>&1
  echo "diagnostic report file: $DIAG_REPORT"
fi

step() {
  echo
  echo "==> $1"
}

step "1) login check (${BASE_URL}/api/v1/auth/login)"
if [[ -z "$SMOKE_ADMIN_PASSWORD" ]]; then
  echo "SMOKE_ADMIN_PASSWORD is empty; skipping login body password check"
else
  code=$(curl -s -o /tmp/login.json -w "%{http_code}" -X POST "${BASE_URL}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"${ADMIN_EMAIL}\",\"password\":\"${SMOKE_ADMIN_PASSWORD}\"}")
  echo "login status: $code"
  cat /tmp/login.json || true
fi

step "2) seeded admin row check"
if command -v psql >/dev/null 2>&1; then
  psql "$POSTGRES_DSN" -c "select user_id,email,role,status,updated_at from users where email='${ADMIN_EMAIL}';" || true
else
  echo "psql not found; skip db check"
fi

step "3) health endpoint check"
curl -i "${BASE_URL}/health" || true

step "4) server log tail (/tmp/msg_server.log)"
if [[ -f /tmp/msg_server.log ]]; then
  tail -n 200 /tmp/msg_server.log || true
else
  echo "/tmp/msg_server.log not found"
fi

step "5) docker compose status"
if command -v docker >/dev/null 2>&1; then
  docker compose ps || true
else
  echo "docker not found; skip container status"
fi

echo
printf "quick diagnostics complete\n"
