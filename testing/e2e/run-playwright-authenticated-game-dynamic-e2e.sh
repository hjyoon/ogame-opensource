#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:8890}"
LEGACY_E2E_CONTAINER_DIR="${OGAME_E2E_CONTAINER_DIR:-/tmp/ogame-e2e}"
FIXTURE_FILE="${OGAME_GAME_VISUAL_FIXTURE_FILE:-$ROOT_DIR/.tmp/authenticated-game-dynamic-fixture.json}"
BROWSERS="${OGAME_GAME_DYNAMIC_BROWSERS:-${OGAME_PLAYWRIGHT_BROWSER:-chromium firefox}}"
BROWSERS="$(printf '%s' "$BROWSERS" | tr ',' ' ')"

mkdir -p "$ROOT_DIR/.tmp"

wait_for_url() {
  url="$1"
  attempts="${2:-30}"
  i=1
  while [ "$i" -le "$attempts" ]; do
    if curl --fail --silent --output /dev/null "$url"; then
      return 0
    fi
    sleep 1
    i=$((i + 1))
  done
  curl --fail --silent --output /dev/null "$url"
}

wait_for_url "$LEGACY_BASE_URL/home.php"
wait_for_url "$GO_BASE_URL/api/healthz"

for browser in $BROWSERS; do
  if [ "${OGAME_GAME_DYNAMIC_PREPARE_FIXTURE:-1}" = "1" ]; then
    docker compose cp "$SCRIPT_DIR/prepare-authenticated-game-visual-fixture.php" "server:$LEGACY_E2E_CONTAINER_DIR/prepare-authenticated-game-visual-fixture.php" >/dev/null
    docker compose exec -T \
      -e OGAME_GAME_VISUAL_COMMANDER_FIXTURE="${OGAME_GAME_VISUAL_COMMANDER_FIXTURE:-1}" \
      -e OGAME_GAME_VISUAL_ALLIANCE_FIXTURE="${OGAME_GAME_VISUAL_ALLIANCE_FIXTURE:-1}" \
      -e OGAME_GAME_VISUAL_REPORT_FIXTURE="${OGAME_GAME_VISUAL_REPORT_FIXTURE:-0}" \
      -e OGAME_GAME_VISUAL_PHALANX_FIXTURE="${OGAME_GAME_VISUAL_PHALANX_FIXTURE:-0}" \
      -e OGAME_GAME_VISUAL_USER="${OGAME_GAME_VISUAL_USER:-}" \
      -e OGAME_GAME_VISUAL_PASS="${OGAME_GAME_VISUAL_PASS:-}" \
      -e OGAME_GAME_VISUAL_ADMIN="${OGAME_GAME_VISUAL_ADMIN:-}" \
      server php "$LEGACY_E2E_CONTAINER_DIR/prepare-authenticated-game-visual-fixture.php" > "$FIXTURE_FILE"
  fi
  printf 'Authenticated game dynamic behavior E2E (%s)\n' "$browser"
  (
    cd "$ROOT_DIR/frontend"
    OGAME_PLAYWRIGHT_BROWSER="$browser" \
    OGAME_LEGACY_BASE_URL="$LEGACY_BASE_URL" \
    OGAME_GO_BASE_URL="$GO_BASE_URL" \
    OGAME_GAME_VISUAL_FIXTURE_FILE="$FIXTURE_FILE" \
    OGAME_GAME_DYNAMIC_OUTPUT_DIR="$ROOT_DIR/.tmp/playwright-authenticated-game-dynamic/$browser" \
    bun run e2e:dynamic:game-auth
  )
done
