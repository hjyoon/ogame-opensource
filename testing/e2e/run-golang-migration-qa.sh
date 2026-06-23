#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
GO_BASE_URL="http://127.0.0.1:${OGAME_GO_PORT:-8890}"
MAILHOG_BASE_URL="http://127.0.0.1:${OGAME_MAILHOG_PORT:-8026}"
LEGACY_E2E_CONTAINER_DIR="${OGAME_E2E_CONTAINER_DIR:-/tmp/ogame-e2e}"

wait_for_url() {
  url="$1"
  attempts="${2:-30}"
  i=1
  while [ "$i" -le "$attempts" ]; do
    if curl --fail --silent "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    i=$((i + 1))
  done
  curl --fail --silent "$url" >/dev/null
}

if [ "${OGAME_RUN_LEGACY_E2E:-1}" = "1" ]; then
  "$SCRIPT_DIR/run-docker-e2e.sh"
  docker compose exec -T server php "$LEGACY_E2E_CONTAINER_DIR/prepare-golang-smoke-fixture.php" >/dev/null
fi

if command -v bun >/dev/null 2>&1; then
  (cd "$ROOT_DIR/frontend" && bun install && bun run build && bun run check && bun run test)
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
  docker compose -f "$ROOT_DIR/compose.golang.yaml" up -d --build --force-recreate goapp
  wait_for_url "$GO_BASE_URL/api/healthz"
  wait_for_url "$GO_BASE_URL/"
  if command -v bun >/dev/null 2>&1; then
    mkdir -p "$ROOT_DIR/.tmp"
    docker compose cp "$SCRIPT_DIR/prepare-golang-user-type-fixture.php" "server:$LEGACY_E2E_CONTAINER_DIR/prepare-golang-user-type-fixture.php" >/dev/null
    docker compose exec -T server php "$LEGACY_E2E_CONTAINER_DIR/prepare-golang-user-type-fixture.php" > "$ROOT_DIR/.tmp/golang-user-type-fixture.json"
    OGAME_GO_BASE_URL="$GO_BASE_URL" OGAME_MAILHOG_BASE_URL="$MAILHOG_BASE_URL" bun "$SCRIPT_DIR/golang-compat-smoke.mjs" > "$ROOT_DIR/.tmp/golang-compat-smoke.json"
    printf 'Go compatibility smoke: %s\n' "$ROOT_DIR/.tmp/golang-compat-smoke.json"
    OGAME_GO_BASE_URL="$GO_BASE_URL" OGAME_USER_TYPE_FIXTURE_FILE="$ROOT_DIR/.tmp/golang-user-type-fixture.json" bun "$SCRIPT_DIR/golang-user-type-qa.mjs" > "$ROOT_DIR/.tmp/golang-user-type-qa.json"
    printf 'Go user type QA: %s\n' "$ROOT_DIR/.tmp/golang-user-type-qa.json"
    for browser in ${OGAME_USER_TYPE_BROWSERS:-chromium firefox}; do
      printf 'Go user type Playwright QA (%s)\n' "$browser"
      (cd "$ROOT_DIR/frontend" && OGAME_PLAYWRIGHT_BROWSER="$browser" OGAME_GO_BASE_URL="$GO_BASE_URL" OGAME_USER_TYPE_FIXTURE_FILE="$ROOT_DIR/.tmp/golang-user-type-fixture.json" bun run e2e:user-types)
    done
  fi
fi
