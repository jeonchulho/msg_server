#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COLLECTION_FILE="$ROOT_DIR/postman/msg_server.smoke.postman_collection.json"
ENV_FILE="${1:-$ROOT_DIR/postman/msg_server.local.postman_environment.json}"

if [[ ! -f "$COLLECTION_FILE" ]]; then
  echo "collection not found: $COLLECTION_FILE" >&2
  exit 1
fi

if [[ ! -f "$ENV_FILE" ]]; then
  echo "environment file not found: $ENV_FILE" >&2
  exit 1
fi

NEWMAN_ARGS=(run "$COLLECTION_FILE" -e "$ENV_FILE" --bail)

if [[ -n "${SMOKE_BASE_URL:-}" ]]; then
  NEWMAN_ARGS+=(--env-var "baseUrl=${SMOKE_BASE_URL}")
fi

if [[ -n "${SMOKE_TENANT_ID:-}" ]]; then
  NEWMAN_ARGS+=(--env-var "tenantId=${SMOKE_TENANT_ID}")
fi

if [[ -n "${SMOKE_EMAIL:-}" ]]; then
  NEWMAN_ARGS+=(--env-var "email=${SMOKE_EMAIL}")
fi

if [[ -n "${SMOKE_PASSWORD:-}" ]]; then
  NEWMAN_ARGS+=(--env-var "password=${SMOKE_PASSWORD}")
fi

if command -v newman >/dev/null 2>&1; then
  newman "${NEWMAN_ARGS[@]}"
  exit 0
fi

if command -v npx >/dev/null 2>&1; then
  npx --yes newman "${NEWMAN_ARGS[@]}"
  exit 0
fi

echo "newman is required. install with: npm i -g newman" >&2
exit 1
