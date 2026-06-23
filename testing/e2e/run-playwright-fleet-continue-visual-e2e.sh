#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
CONTAINER_DIR="${OGAME_E2E_CONTAINER_DIR:-/tmp/ogame-e2e}"
FIXTURE_FILE="${OGAME_FLEET_CONTINUE_FIXTURE_FILE:-$ROOT_DIR/.tmp/fleet-continue-fixture.json}"

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
docker compose cp "$SCRIPT_DIR/prepare-fleet-continue-fixture.php" "server:$CONTAINER_DIR/prepare-fleet-continue-fixture.php" >/dev/null
docker compose exec -T server php "$CONTAINER_DIR/prepare-fleet-continue-fixture.php" > "$FIXTURE_FILE"

cd "$ROOT_DIR/frontend"
for browser in ${OGAME_FLEET_CONTINUE_BROWSERS:-chromium firefox}; do
  printf 'Fleet continue visual E2E (%s)\n' "$browser"
  OGAME_PLAYWRIGHT_BROWSER="$browser" \
  OGAME_LEGACY_BASE_URL="$LEGACY_BASE_URL" \
  OGAME_GO_BASE_URL="$GO_BASE_URL" \
  OGAME_FLEET_CONTINUE_FIXTURE_FILE="$FIXTURE_FILE" \
  bun run e2e:visual:fleet-continue
done
