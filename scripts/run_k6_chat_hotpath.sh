#!/usr/bin/env bash

set -euo pipefail

REPORT_DIR="${REPORT_DIR:-./loadtest_reports}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
SUMMARY_FILE="${REPORT_DIR}/k6_chat_hotpath_summary_${TIMESTAMP}.json"
CONSOLE_LOG="${REPORT_DIR}/k6_chat_hotpath_console_${TIMESTAMP}.log"

mkdir -p "${REPORT_DIR}"

if ! command -v k6 >/dev/null 2>&1; then
  echo "[error] k6 is not installed"
  echo "[hint] run: make install-k6"
  exit 1
fi

echo "[info] running k6 hotpath scenario"
echo "[info] summary file: ${SUMMARY_FILE}"
echo "[info] console log : ${CONSOLE_LOG}"

set +e
k6 run --summary-export "${SUMMARY_FILE}" scripts/k6_chat_hotpath.js | tee "${CONSOLE_LOG}"
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

def v(path, default="n/a"):
    cur = metrics
    for key in path:
        if not isinstance(cur, dict) or key not in cur:
            return default
        cur = cur[key]
    return cur

dur = metrics.get("http_req_duration", {}).get("values", {})
failed = metrics.get("http_req_failed", {}).get("values", {})
reqs = metrics.get("http_reqs", {}).get("values", {})

print("[summary] k6 hotpath")
print(f"  - requests: {reqs.get('count', 'n/a')}")
print(f"  - failed_rate: {failed.get('rate', 'n/a')}")
print(f"  - p95_ms: {dur.get('p(95)', 'n/a')}")
print(f"  - p99_ms: {dur.get('p(99)', 'n/a')}")
print(f"  - avg_ms: {dur.get('avg', 'n/a')}")
PY
else
  echo "[warn] python3 not found, skip summary parsing"
fi

if [[ ${K6_EXIT} -ne 0 ]]; then
  echo "[error] k6 finished with non-zero exit code: ${K6_EXIT}"
  exit ${K6_EXIT}
fi

echo "[ok] k6 run completed"
