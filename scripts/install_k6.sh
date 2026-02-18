#!/usr/bin/env bash

set -euo pipefail

run_as_root() {
  if [[ "${EUID}" -eq 0 ]]; then
    "$@"
    return
  fi
  if command -v sudo >/dev/null 2>&1; then
    sudo "$@"
    return
  fi
  echo "[error] root 권한이 필요합니다. root로 실행하거나 sudo를 설치하세요."
  exit 1
}

if command -v k6 >/dev/null 2>&1; then
  echo "[info] k6 is already installed"
  k6 version
  exit 0
fi

echo "[info] installing k6 (apt repository)"

run_as_root apt-get update
run_as_root apt-get install -y ca-certificates curl gnupg

run_as_root mkdir -p /etc/apt/keyrings
curl -fsSL https://dl.k6.io/key.gpg | run_as_root gpg --dearmor -o /etc/apt/keyrings/k6.gpg
echo "deb [signed-by=/etc/apt/keyrings/k6.gpg] https://dl.k6.io/deb stable main" | run_as_root tee /etc/apt/sources.list.d/k6.list >/dev/null

run_as_root apt-get update
run_as_root apt-get install -y k6

echo "[ok] k6 installed"
k6 version
