#!/bin/bash
set -e

_SHUTDOWN=0
shutdown() {
  if [ $_SHUTDOWN -eq 1 ]; then return; fi
  _SHUTDOWN=1
  echo ""
  echo "[dev] shutting down..."
  kill 0
}

trap shutdown EXIT INT TERM

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# nvm is a shell function, not a binary — must be sourced explicitly
export NVM_DIR="$HOME/.nvm"
if [ -s "$NVM_DIR/nvm.sh" ]; then
  source "$NVM_DIR/nvm.sh"
  nvm use 24
else
  echo "[dev] warning: nvm not found, skipping 'nvm use 24'"
fi

check_dep() {
  if ! command -v "$1" &>/dev/null; then
    echo "[dev] error: '$1' is not installed or not in PATH"
    echo "[dev] $2"
    exit 1
  fi
}

check_dep air  "install with: go install github.com/air-verse/air@latest"
check_dep bun  "install with: https://bun.sh"
check_dep go   "install with: https://go.dev/dl"

echo "[dev] installing Go dependencies..."
cd "$ROOT/api" && go mod download

echo "[dev] installing web dependencies..."
cd "$ROOT/web" && bun install

echo "[dev] starting api (air)..."
cd "$ROOT/api" && air &
API_PID=$!

echo "[dev] starting web (bun)..."
cd "$ROOT/web" && bun --env-file="$ROOT/.env" run dev &
WEB_PID=$!

echo "[dev] running — api pid=$API_PID  web pid=$WEB_PID"
echo "[dev] ctrl+c to stop both"

wait