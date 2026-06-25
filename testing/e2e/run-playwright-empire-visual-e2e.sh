#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
CONTAINER_DIR="${OGAME_E2E_CONTAINER_DIR:-/tmp/ogame-e2e}"
FIXTURE_FILE="${OGAME_EMPIRE_VISUAL_FIXTURE_FILE:-$ROOT_DIR/.tmp/empire-visual-fixture.json}"

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

mkdir -p "$ROOT_DIR/.tmp"
wait_for_url "$LEGACY_BASE_URL/home.php"
wait_for_url "$GO_BASE_URL/api/healthz"

cleanup_fixture() {
  if [ "${OGAME_CLEAN_MIGRATION_FIXTURES:-1}" = "1" ]; then
    docker compose exec -T server php "$CONTAINER_DIR/cleanup-golang-migration-fixtures.php" >/dev/null 2>&1 || true
  fi
}

docker compose exec -T server mkdir -p "$CONTAINER_DIR"
docker compose cp "$SCRIPT_DIR/cleanup-golang-migration-fixtures.php" "server:$CONTAINER_DIR/cleanup-golang-migration-fixtures.php" >/dev/null
docker compose cp "$SCRIPT_DIR/prepare-empire-visual-fixture.php" "server:$CONTAINER_DIR/prepare-empire-visual-fixture.php" >/dev/null
trap cleanup_fixture EXIT INT TERM
docker compose exec -T server php "$CONTAINER_DIR/prepare-empire-visual-fixture.php" > "$FIXTURE_FILE"

LOGIN="$(jq -r '.login' "$FIXTURE_FILE")"
PASSWORD="$(jq -r '.password' "$FIXTURE_FILE")"

cd "$ROOT_DIR/frontend"
for browser in ${OGAME_EMPIRE_VISUAL_BROWSERS:-chromium firefox}; do
  printf 'Empire visual E2E (%s)\n' "$browser"
  OGAME_PLAYWRIGHT_BROWSER="$browser" \
  OGAME_LEGACY_BASE_URL="$LEGACY_BASE_URL" \
  OGAME_GO_BASE_URL="$GO_BASE_URL" \
  OGAME_AUTH_VISUAL_USER="$LOGIN" \
  OGAME_AUTH_VISUAL_PASS="$PASSWORD" \
  OGAME_AUTH_VISUAL_PAGE="${OGAME_AUTH_VISUAL_PAGE:-game-empire}" \
  OGAME_AUTH_VISUAL_ENFORCE_DIFF="${OGAME_AUTH_VISUAL_ENFORCE_DIFF:-1}" \
  OGAME_AUTH_VISUAL_ENFORCE_LAYOUT="${OGAME_AUTH_VISUAL_ENFORCE_LAYOUT:-1}" \
  OGAME_AUTH_VISUAL_MAX_DIFF_RATIO="${OGAME_AUTH_VISUAL_MAX_DIFF_RATIO:-0}" \
  OGAME_AUTH_VISUAL_MAX_BOX_DELTA="${OGAME_AUTH_VISUAL_MAX_BOX_DELTA:-0}" \
  bun run e2e:visual:auth
done
