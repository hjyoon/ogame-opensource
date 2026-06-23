#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
CONTAINER_DIR="${OGAME_E2E_CONTAINER_DIR:-/tmp/ogame-e2e}"
FIXTURE_FILE="${OGAME_OVERVIEW_FLEET_FIXTURE_FILE:-$ROOT_DIR/.tmp/overview-fleet-fixture.json}"

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

mkdir -p "$ROOT_DIR/.tmp"
wait_for_url "$LEGACY_BASE_URL/home.php"
wait_for_url "$GO_BASE_URL/api/healthz"

docker compose exec -T server mkdir -p "$CONTAINER_DIR"
docker compose cp "$SCRIPT_DIR/prepare-overview-fleet-fixture.php" "server:$CONTAINER_DIR/prepare-overview-fleet-fixture.php" >/dev/null
docker compose exec -T server php "$CONTAINER_DIR/prepare-overview-fleet-fixture.php" > "$FIXTURE_FILE"

cd "$ROOT_DIR/frontend"
for browser in ${OGAME_OVERVIEW_FLEET_COUNTDOWN_BROWSERS:-chromium firefox}; do
  printf 'Overview fleet countdown E2E (%s)\n' "$browser"
  OGAME_PLAYWRIGHT_BROWSER="$browser" \
  OGAME_LEGACY_BASE_URL="$LEGACY_BASE_URL" \
  OGAME_GO_BASE_URL="$GO_BASE_URL" \
  OGAME_OVERVIEW_FLEET_FIXTURE_FILE="$FIXTURE_FILE" \
  bun run e2e:overview-fleet-countdown
done
