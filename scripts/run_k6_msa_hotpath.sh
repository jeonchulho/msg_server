#!/usr/bin/env bash

set -euo pipefail

REPORT_DIR="${REPORT_DIR:-./loadtest_reports}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
SUMMARY_FILE="${REPORT_DIR}/k6_msa_hotpath_summary_${TIMESTAMP}.json"
CONSOLE_LOG="${REPORT_DIR}/k6_msa_hotpath_console_${TIMESTAMP}.log"
SUMMARY_MD="${REPORT_DIR}/k6_msa_hotpath_summary_${TIMESTAMP}.md"

mkdir -p "${REPORT_DIR}"

if ! command -v k6 >/dev/null 2>&1; then
  echo "[error] k6 is not installed"
  echo "[hint] run: make install-k6"
  exit 1
fi

echo "[info] running k6 MSA hotpath scenario"
echo "[info] summary file: ${SUMMARY_FILE}"
echo "[info] console log : ${CONSOLE_LOG}"
echo "[info] markdown   : ${SUMMARY_MD}"

set +e
k6 run --summary-export "${SUMMARY_FILE}" scripts/k6_msa_hotpath.js | tee "${CONSOLE_LOG}"
K6_EXIT=$?
set -e

if [[ ! -f "${SUMMARY_FILE}" ]]; then
  echo "[error] summary file was not generated"
  exit 1
fi

if command -v python3 >/dev/null 2>&1; then
  python3 scripts/render_k6_summary_md.py "${SUMMARY_FILE}" msa > "${SUMMARY_MD}"
  echo "[summary]"
  cat "${SUMMARY_MD}"
  if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
    cat "${SUMMARY_MD}" >> "${GITHUB_STEP_SUMMARY}"
  fi
else
  echo "[warn] python3 not found, skip summary parsing"
fi

if [[ ${K6_EXIT} -ne 0 ]]; then
  echo "[error] k6 finished with non-zero exit code: ${K6_EXIT}"
  exit ${K6_EXIT}
fi

echo "[ok] k6 run completed"
