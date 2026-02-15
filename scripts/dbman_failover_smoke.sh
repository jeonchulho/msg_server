#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

GOOD_DBMAN_ENDPOINT="${GOOD_DBMAN_ENDPOINT:-http://localhost:8082}"
BAD_DBMAN_ENDPOINT="${BAD_DBMAN_ENDPOINT:-http://127.0.0.1:65535}"

export DBMAN_HTTP_TIMEOUT_MS="${DBMAN_HTTP_TIMEOUT_MS:-1500}"
export DBMAN_FAIL_THRESHOLD="${DBMAN_FAIL_THRESHOLD:-1}"
export DBMAN_COOLDOWN_MS="${DBMAN_COOLDOWN_MS:-3000}"

echo "[info] failover smoke start"
echo "[info] bad endpoint:  ${BAD_DBMAN_ENDPOINT}"
echo "[info] good endpoint: ${GOOD_DBMAN_ENDPOINT}"

if ! curl -fsS "${GOOD_DBMAN_ENDPOINT}/health/ready" >/dev/null; then
  echo "[error] good dbman endpoint is not ready: ${GOOD_DBMAN_ENDPOINT}"
  echo "[hint] run: make run-dbman"
  exit 1
fi

tmp_file="$(mktemp /tmp/dbman_failover_smoke_XXXXXX.go)"
trap 'rm -f "${tmp_file}"' EXIT

cat >"${tmp_file}" <<'EOF'
package main

import (
  "context"
  "fmt"
  "os"
  "time"

  commondbman "msg_server/server/common/infra/dbman"
)

func main() {
  bad := os.Getenv("BAD_DBMAN_ENDPOINT")
  good := os.Getenv("GOOD_DBMAN_ENDPOINT")
  client := commondbman.NewClientWithEndpoints(bad, good)

  ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
  defer cancel()

  var out []map[string]any
  if err := client.Post(ctx, commondbman.BasePath+"/tenants/list", map[string]any{}, &out); err != nil {
    fmt.Printf("FAILOVER_SMOKE_FAILED: %v\n", err)
    os.Exit(1)
  }

  fmt.Printf("FAILOVER_SMOKE_OK: tenants=%d\n", len(out))
}
EOF

BAD_DBMAN_ENDPOINT="${BAD_DBMAN_ENDPOINT}" \
GOOD_DBMAN_ENDPOINT="${GOOD_DBMAN_ENDPOINT}" \
go run "${tmp_file}"

echo "[ok] failover smoke passed"
