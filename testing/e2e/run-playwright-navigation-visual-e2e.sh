#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
URL_CHECK_FILE="$ROOT_DIR/.tmp/navigation-url-check"
LEGACY_E2E_CONTAINER_DIR="${OGAME_E2E_CONTAINER_DIR:-/tmp/ogame-e2e}"
FIXTURE_FILE="${OGAME_NAV_VISUAL_FIXTURE_FILE:-$ROOT_DIR/.tmp/navigation-visual-fixture.json}"
mkdir -p "$ROOT_DIR/.tmp"

wait_for_url() {
  url="$1"
  attempts="${2:-30}"
  i=1
  while [ "$i" -le "$attempts" ]; do
    if curl --fail --silent --output "$URL_CHECK_FILE" "$url"; then
      return 0
    fi
    sleep 1
    i=$((i + 1))
  done
  curl --fail --silent --output "$URL_CHECK_FILE" "$url"
}

wait_for_url "$LEGACY_BASE_URL/home.php"
wait_for_url "$GO_BASE_URL/api/healthz"

prepare_navigation_fixture() {
  if [ "${OGAME_NAV_VISUAL_PREPARE_FIXTURE:-1}" = "0" ]; then
    return 0
  fi

  docker compose cp "$SCRIPT_DIR/prepare-authenticated-game-visual-fixture.php" "server:$LEGACY_E2E_CONTAINER_DIR/prepare-authenticated-game-visual-fixture.php" >/dev/null
  docker compose exec -T \
    -e OGAME_GAME_VISUAL_COMMANDER_FIXTURE="${OGAME_GAME_VISUAL_COMMANDER_FIXTURE:-1}" \
    -e OGAME_GAME_VISUAL_ALLIANCE_FIXTURE="${OGAME_GAME_VISUAL_ALLIANCE_FIXTURE:-1}" \
    -e OGAME_GAME_VISUAL_REPORT_FIXTURE="${OGAME_GAME_VISUAL_REPORT_FIXTURE:-1}" \
    -e OGAME_GAME_VISUAL_PHALANX_FIXTURE="${OGAME_GAME_VISUAL_PHALANX_FIXTURE:-1}" \
    -e OGAME_GAME_VISUAL_USER="${OGAME_NAV_VISUAL_USER:-legor}" \
    -e OGAME_GAME_VISUAL_PASS="${OGAME_NAV_VISUAL_PASS:-admin}" \
    server php "$LEGACY_E2E_CONTAINER_DIR/prepare-authenticated-game-visual-fixture.php" > "$FIXTURE_FILE"
}

sync_docker_visual_fixtures() {
  if [ "${OGAME_NAV_VISUAL_SYNC_DOCKER_FIXTURES:-1}" = "0" ]; then
    return 0
  fi
  if ! command -v docker >/dev/null 2>&1; then
    return 0
  fi

  legacy_container="${OGAME_LEGACY_DOCKER_CONTAINER:-ogame-opensource-server-1}"
  go_container="${OGAME_GO_DOCKER_CONTAINER:-ogame-opensource-goapp-1}"
  legacy_temp="${OGAME_LEGACY_DOCKER_GAME_TEMP:-/var/www/html/game/temp}"
  go_temp="${OGAME_GO_DOCKER_GAME_TEMP:-/srv/ogame/game/temp}"
  fixture_dir="$ROOT_DIR/.tmp/navigation-visual-db-backups"

  if ! docker inspect "$legacy_container" >/dev/null 2>&1 || ! docker inspect "$go_container" >/dev/null 2>&1; then
    return 0
  fi
  if ! docker exec "$legacy_container" sh -lc "test -d '$legacy_temp'" >/dev/null 2>&1; then
    return 0
  fi

  rm -rf "$fixture_dir"
  mkdir -p "$fixture_dir"
  docker exec "$go_container" sh -lc "mkdir -p '$go_temp' && find '$go_temp' -maxdepth 1 -type f -name 'backup_*.json' -delete"
  docker exec "$legacy_container" sh -lc "cd '$legacy_temp' && ls backup_*.json 2>/dev/null || true" | while IFS= read -r name; do
    [ -n "$name" ] || continue
    docker cp "$legacy_container:$legacy_temp/$name" "$fixture_dir/$name"
    docker cp "$fixture_dir/$name" "$go_container:$go_temp/$name"
  done
}

prepare_navigation_fixture
sync_docker_visual_fixtures

cd "$ROOT_DIR/frontend"
status=0
for browser in ${OGAME_NAV_VISUAL_BROWSERS:-chromium firefox}; do
  printf 'Navigation visual E2E (%s)\n' "$browser"
  if ! OGAME_PLAYWRIGHT_BROWSER="$browser" \
    OGAME_LEGACY_BASE_URL="$LEGACY_BASE_URL" \
    OGAME_GO_BASE_URL="$GO_BASE_URL" \
    bun run e2e:visual:navigation; then
    status=1
  fi
done
if ! bun run scripts/render-navigation-visual-coverage.ts; then
  status=1
fi
exit "$status"
