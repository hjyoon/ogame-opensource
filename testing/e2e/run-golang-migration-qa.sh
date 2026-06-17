#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
GO_BASE_URL="http://127.0.0.1:${OGAME_GO_PORT:-8890}"

if [ "${OGAME_RUN_LEGACY_E2E:-1}" = "1" ]; then
  "$SCRIPT_DIR/run-docker-e2e.sh"
fi

if command -v bun >/dev/null 2>&1; then
  (cd "$ROOT_DIR/frontend" && bun install && bun run build && bun run check)
else
  printf 'SKIP frontend build: bun was not found\n'
fi

if command -v go >/dev/null 2>&1 && go version >/dev/null 2>&1; then
  "$ROOT_DIR/backend/scripts/test-coverage.sh"
else
  printf 'SKIP backend tests: go was not available\n'
fi

if [ "${OGAME_RUN_GO_DOCKER:-1}" = "1" ]; then
  if [ "${OGAME_KEEP_GO_DOCKER:-0}" != "1" ]; then
    trap 'docker compose -f "$ROOT_DIR/compose.golang.yaml" down >/dev/null 2>&1 || true' EXIT INT TERM
  fi
  docker compose -f "$ROOT_DIR/compose.golang.yaml" up -d --build goapp
  curl --fail --silent "$GO_BASE_URL/api/healthz" >/dev/null
  curl --fail --silent "$GO_BASE_URL/" >/dev/null
  if command -v bun >/dev/null 2>&1; then
    mkdir -p "$ROOT_DIR/.tmp"
    OGAME_GO_BASE_URL="$GO_BASE_URL" bun "$SCRIPT_DIR/golang-compat-smoke.mjs" > "$ROOT_DIR/.tmp/golang-compat-smoke.json"
    printf 'Go compatibility smoke: %s\n' "$ROOT_DIR/.tmp/golang-compat-smoke.json"
  fi
fi
