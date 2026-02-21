#!/usr/bin/env bash

set -euo pipefail

REPORT_DIR="${REPORT_DIR:-./loadtest_reports}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
SUMMARY_FILE="${REPORT_DIR}/k6_msa_hotpath_summary_${TIMESTAMP}.json"
CONSOLE_LOG="${REPORT_DIR}/k6_msa_hotpath_console_${TIMESTAMP}.log"

mkdir -p "${REPORT_DIR}"

if ! command -v k6 >/dev/null 2>&1; then
  echo "[error] k6 is not installed"
  echo "[hint] run: make install-k6"
  exit 1
fi

echo "[info] running k6 MSA hotpath scenario"
echo "[info] summary file: ${SUMMARY_FILE}"
echo "[info] console log : ${CONSOLE_LOG}"

set +e
k6 run --summary-export "${SUMMARY_FILE}" scripts/k6_msa_hotpath.js | tee "${CONSOLE_LOG}"
K6_EXIT=$?
set -e

if [[ ! -f "${SUMMARY_FILE}" ]]; then
  echo "[error] summary file was not generated"
  exit 1
fi

if command -v python3 >/dev/null 2>&1; then
  python3 - "${SUMMARY_FILE}" <<'PY'
import json
import sys

summary_file = sys.argv[1]

with open(summary_file, "r", encoding="utf-8") as f:
    data = json.load(f)

metrics = data.get("metrics", {})

dur = metrics.get("http_req_duration", {}).get("values", {})
failed = metrics.get("http_req_failed", {}).get("values", {})
reqs = metrics.get("http_reqs", {}).get("values", {})
chat = metrics.get("msa_chat_create_message_ms", {}).get("values", {})
session = metrics.get("msa_session_update_status_ms", {}).get("values", {})
tenant = metrics.get("msa_tenanthub_list_ms", {}).get("values", {})

print("[summary] k6 msa hotpath")
print(f"  - requests: {reqs.get('count', 'n/a')}")
print(f"  - failed_rate: {failed.get('rate', 'n/a')}")
print(f"  - http_p95_ms: {dur.get('p(95)', 'n/a')}")
print(f"  - http_p99_ms: {dur.get('p(99)', 'n/a')}")
print(f"  - chat_p95_ms: {chat.get('p(95)', 'n/a')}")
print(f"  - session_p95_ms: {session.get('p(95)', 'n/a')}")
print(f"  - tenanthub_p95_ms: {tenant.get('p(95)', 'n/a')}")
PY
else
  echo "[warn] python3 not found, skip summary parsing"
fi

if [[ ${K6_EXIT} -ne 0 ]]; then
  echo "[error] k6 finished with non-zero exit code: ${K6_EXIT}"
  exit ${K6_EXIT}
fi

echo "[ok] k6 run completed"
