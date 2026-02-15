#!/usr/bin/env bash
set -euo pipefail

DSN=${POSTGRES_DSN:-postgres://msg:msg@localhost:5432/msg?sslmode=disable}

for file in migrations/*.sql; do
	echo "applying $file"
	psql "$DSN" -f "$file"
done

echo "all migrations applied"