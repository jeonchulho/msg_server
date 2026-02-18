up:
	docker compose up -d

down:
	docker compose down

migrate:
	./scripts/migrate.sh

seed-admin:
	TENANT_ID="$(TENANT_ID)" \
	ADMIN_EMAIL="$(ADMIN_EMAIL)" \
	ADMIN_PASSWORD="$(ADMIN_PASSWORD)" \
	ADMIN_NAME="$(ADMIN_NAME)" \
	ADMIN_TITLE="$(ADMIN_TITLE)" \
	./scripts/seed_admin.sh

run-chat:
	go run ./cmd/chat

run-session:
	go run ./cmd/session

run-fileman:
	go run ./cmd/fileman

run-dbman:
	go run ./cmd/dbman

run-vectorman:
	go run ./cmd/vectorman

run: run-chat

build:
	go build ./...

smoke:
	bash ./scripts/newman_smoke.sh

dbman-failover-smoke:
	bash ./scripts/dbman_failover_smoke.sh

chat-ws-only-smoke:
	bash ./scripts/chat_ws_only_smoke.sh

diag:
	bash ./scripts/quick_diag.sh

diag-report:
	DIAG_REPORT=./diag_report.txt bash ./scripts/quick_diag.sh

install-k6:
	bash ./scripts/install_k6.sh

load-chat-baseline:
	k6 run scripts/k6_chat_hotpath.js

load-chat:
	K6_VUS="$(K6_VUS)" K6_DURATION="$(K6_DURATION)" BASE_URL="$(BASE_URL)" TENANT_ID="$(TENANT_ID)" SMOKE_EMAIL="$(SMOKE_EMAIL)" SMOKE_PASSWORD="$(SMOKE_PASSWORD)" K6_SLEEP_MS="$(K6_SLEEP_MS)" k6 run scripts/k6_chat_hotpath.js
