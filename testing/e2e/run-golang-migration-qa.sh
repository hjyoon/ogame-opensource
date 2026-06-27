#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
GO_BASE_URL="http://127.0.0.1:${OGAME_GO_PORT:-8890}"
MAILHOG_BASE_URL="http://127.0.0.1:${OGAME_MAILHOG_PORT:-8026}"
LEGACY_E2E_CONTAINER_DIR="${OGAME_E2E_CONTAINER_DIR:-/tmp/ogame-e2e}"
mkdir -p "$ROOT_DIR/.tmp"

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
  if [ "${OGAME_RUN_GO_DOCKER:-1}" = "1" ]; then
    docker compose -f "$ROOT_DIR/compose.golang.yaml" up -d mailhog >/dev/null
    wait_for_url "$MAILHOG_BASE_URL/api/v2/messages"
  fi
  "$SCRIPT_DIR/run-docker-e2e.sh"
  docker compose exec -T server php "$LEGACY_E2E_CONTAINER_DIR/prepare-golang-smoke-fixture.php" > "$ROOT_DIR/.tmp/golang-smoke-fixture.json"
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
    bun "$SCRIPT_DIR/golang-compat-smoke.mjs" --go-base-url "$GO_BASE_URL" --mailhog-base-url "$MAILHOG_BASE_URL" --fixture "$ROOT_DIR/.tmp/golang-smoke-fixture.json" > "$ROOT_DIR/.tmp/golang-compat-smoke.json"
    printf 'Go compatibility smoke: %s\n' "$ROOT_DIR/.tmp/golang-compat-smoke.json"
    docker compose exec -T server php "$LEGACY_E2E_CONTAINER_DIR/cleanup-golang-migration-fixtures.php" >/dev/null
    docker compose cp "$SCRIPT_DIR/prepare-golang-user-type-fixture.php" "server:$LEGACY_E2E_CONTAINER_DIR/prepare-golang-user-type-fixture.php" >/dev/null
    docker compose exec -T server php "$LEGACY_E2E_CONTAINER_DIR/prepare-golang-user-type-fixture.php" > "$ROOT_DIR/.tmp/golang-user-type-fixture.json"
    OGAME_GO_BASE_URL="$GO_BASE_URL" OGAME_USER_TYPE_FIXTURE_FILE="$ROOT_DIR/.tmp/golang-user-type-fixture.json" bun "$SCRIPT_DIR/golang-user-type-qa.mjs" > "$ROOT_DIR/.tmp/golang-user-type-qa.json"
    printf 'Go user type QA: %s\n' "$ROOT_DIR/.tmp/golang-user-type-qa.json"
    for browser in ${OGAME_USER_TYPE_BROWSERS:-chromium firefox}; do
      printf 'Go user type Playwright QA (%s)\n' "$browser"
      (cd "$ROOT_DIR/frontend" && OGAME_PLAYWRIGHT_BROWSER="$browser" OGAME_GO_BASE_URL="$GO_BASE_URL" OGAME_USER_TYPE_FIXTURE_FILE="$ROOT_DIR/.tmp/golang-user-type-fixture.json" bun run e2e:user-types)
    done
    if [ "${OGAME_RUN_AUTH_VISUAL:-1}" = "1" ]; then
      for browser in ${OGAME_AUTH_VISUAL_BROWSERS:-chromium firefox}; do
        printf 'Authenticated visual E2E (%s)\n' "$browser"
        OGAME_PLAYWRIGHT_BROWSER="$browser" \
        OGAME_GO_BASE_URL="$GO_BASE_URL" \
        OGAME_AUTH_VISUAL_OUTPUT_DIR="$ROOT_DIR/.tmp/playwright-auth-visual/auth/$browser" \
        OGAME_AUTH_VISUAL_ENFORCE_DIFF="${OGAME_AUTH_VISUAL_ENFORCE_DIFF:-1}" \
        OGAME_AUTH_VISUAL_ENFORCE_LAYOUT="${OGAME_AUTH_VISUAL_ENFORCE_LAYOUT:-1}" \
        OGAME_AUTH_VISUAL_MAX_DIFF_RATIO="${OGAME_AUTH_VISUAL_MAX_DIFF_RATIO:-0}" \
        OGAME_AUTH_VISUAL_MAX_BOX_DELTA="${OGAME_AUTH_VISUAL_MAX_BOX_DELTA:-0}" \
        "$SCRIPT_DIR/run-playwright-auth-visual-e2e.sh"
      done
    fi
    if [ "${OGAME_RUN_EMPIRE_VISUAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-playwright-empire-visual-e2e.sh"
    fi
    if [ "${OGAME_RUN_ALLIANCE_VISUAL:-1}" = "1" ]; then
      for browser in ${OGAME_ALLIANCE_VISUAL_BROWSERS:-chromium firefox}; do
        printf 'Alliance visual E2E (%s)\n' "$browser"
        OGAME_PLAYWRIGHT_BROWSER="$browser" \
        OGAME_GO_BASE_URL="$GO_BASE_URL" \
        OGAME_AUTH_VISUAL_OUTPUT_DIR="$ROOT_DIR/.tmp/playwright-auth-visual/alliance/$browser" \
        OGAME_AUTH_VISUAL_ENFORCE_DIFF="${OGAME_AUTH_VISUAL_ENFORCE_DIFF:-1}" \
        OGAME_AUTH_VISUAL_ENFORCE_LAYOUT="${OGAME_AUTH_VISUAL_ENFORCE_LAYOUT:-1}" \
        OGAME_AUTH_VISUAL_MAX_DIFF_RATIO="${OGAME_AUTH_VISUAL_MAX_DIFF_RATIO:-0}" \
        OGAME_AUTH_VISUAL_MAX_BOX_DELTA="${OGAME_AUTH_VISUAL_MAX_BOX_DELTA:-0}" \
        "$SCRIPT_DIR/run-playwright-alliance-visual-e2e.sh"
      done
    fi
    if [ "${OGAME_RUN_OVERVIEW_FLEET_VISUAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-playwright-overview-fleet-visual-e2e.sh"
    fi
    if [ "${OGAME_RUN_OVERVIEW_FLEET_COUNTDOWN:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-playwright-overview-fleet-countdown-e2e.sh"
    fi
    if [ "${OGAME_RUN_OVERVIEW_ALL_CASES:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-playwright-overview-all-cases-e2e.sh"
    fi
    if [ "${OGAME_RUN_FLEET_CONTINUE_VISUAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-playwright-fleet-continue-visual-e2e.sh"
    fi
    if [ "${OGAME_RUN_FLEET_ALL_CASES:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-playwright-fleet-all-cases-e2e.sh"
    fi
    if [ "${OGAME_RUN_NAVIGATION_VISUAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-playwright-navigation-visual-e2e.sh"
    fi
  fi
fi

if command -v bun >/dev/null 2>&1; then
  OGAME_GO_BASE_URL="$GO_BASE_URL" bun "$SCRIPT_DIR/golang-migration-qa-summary.mjs"
fi
