#!/usr/bin/env bash
set -euo pipefail

DSN="${POSTGRES_DSN:-postgres://msg:msg@localhost:5432/msg?sslmode=disable}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-pass1234}"
ADMIN_NAME="${ADMIN_NAME:-Admin}"
ADMIN_TITLE="${ADMIN_TITLE:-Administrator}"
TENANT_ID="${TENANT_ID:-default}"

if ! command -v psql >/dev/null 2>&1; then
  echo "psql is required" >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "go is required" >&2
  exit 1
fi

PASSWORD_HASH="$({
  cat <<'EOF'
package main
import (
  "fmt"
  "os"
  "golang.org/x/crypto/bcrypt"
)
func main() {
  if len(os.Args) < 2 {
    panic("password arg required")
  }
  hash, err := bcrypt.GenerateFromPassword([]byte(os.Args[1]), bcrypt.DefaultCost)
  if err != nil {
    panic(err)
  }
  fmt.Print(string(hash))
}
EOF
} | go run /dev/stdin "$ADMIN_PASSWORD")"

ORG_ID="$(psql "$DSN" -tA -c "
WITH existing AS (
  SELECT org_id FROM org_units WHERE tenant_id='${TENANT_ID}' ORDER BY org_id LIMIT 1
), inserted AS (
  INSERT INTO org_units (tenant_id, org_parent_id, name)
  SELECT '${TENANT_ID}', NULL, 'Root'
  WHERE NOT EXISTS (SELECT 1 FROM existing)
  RETURNING org_id
)
SELECT org_id FROM inserted
UNION ALL
SELECT org_id FROM existing
LIMIT 1;
")"

psql "$DSN" -v ON_ERROR_STOP=1 -c "
INSERT INTO users (tenant_id, org_id, email, name, title, status, status_note, role, password_hash)
VALUES ('${TENANT_ID}', '${ORG_ID}', '${ADMIN_EMAIL}', '${ADMIN_NAME}', '${ADMIN_TITLE}', 'online', 'seeded admin', 'admin', '${PASSWORD_HASH}')
ON CONFLICT (tenant_id, email)
DO UPDATE SET
  tenant_id = EXCLUDED.tenant_id,
  org_id = EXCLUDED.org_id,
  name = EXCLUDED.name,
  title = EXCLUDED.title,
  status = EXCLUDED.status,
  status_note = EXCLUDED.status_note,
  role = EXCLUDED.role,
  password_hash = EXCLUDED.password_hash,
  updated_at = NOW();
"

echo "seeded admin user: ${ADMIN_EMAIL} (tenant: ${TENANT_ID})"
