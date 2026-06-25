#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:8890}"
CONTAINER_DIR="/tmp/ogame-alliance-visual-e2e"
FIXTURE_FILE="$ROOT_DIR/.tmp/alliance-visual-fixture.json"

wait_for_url() {
  url="$1"
  attempts="${2:-30}"
  i=1
  while [ "$i" -le "$attempts" ]; do
    code="$(curl --silent --show-error --output /dev/null --write-out '%{http_code}' "$url" || printf '000')"
    case "$code" in
      2*|3*)
      return 0
      ;;
    esac
    sleep 1
    i=$((i + 1))
  done
  code="$(curl --silent --show-error --output /dev/null --write-out '%{http_code}' "$url" || printf '000')"
  case "$code" in
    2*|3*) return 0 ;;
  esac
  printf 'URL not ready: %s (status %s)\n' "$url" "$code" >&2
  return 1
}

mkdir -p "$ROOT_DIR/.tmp"
wait_for_url "$LEGACY_BASE_URL/home.php"
wait_for_url "$GO_BASE_URL/api/healthz"

cleanup_fixture() {
  if [ "${OGAME_CLEAN_MIGRATION_FIXTURES:-1}" = "1" ]; then
    docker compose exec -T server php "$CONTAINER_DIR/cleanup-golang-migration-fixtures.php" >/dev/null 2>&1 || true
  fi
}

cd "$ROOT_DIR"
docker compose exec -T server mkdir -p "$CONTAINER_DIR" >/dev/null
docker compose cp "$SCRIPT_DIR/cleanup-golang-migration-fixtures.php" "server:$CONTAINER_DIR/cleanup-golang-migration-fixtures.php" >/dev/null
docker compose cp "$SCRIPT_DIR/prepare-alliance-visual-fixture.php" "server:$CONTAINER_DIR/prepare-alliance-visual-fixture.php" >/dev/null
trap cleanup_fixture EXIT INT TERM
docker compose exec -T server php "$CONTAINER_DIR/prepare-alliance-visual-fixture.php" > "$FIXTURE_FILE"

LOGIN="$(jq -r '.login' "$FIXTURE_FILE")"
PASSWORD="$(jq -r '.password' "$FIXTURE_FILE")"

cd "$ROOT_DIR/frontend"
AUTH_VISUAL_BROWSER="${OGAME_PLAYWRIGHT_BROWSER:-chromium}"
OGAME_LEGACY_BASE_URL="$LEGACY_BASE_URL" \
  OGAME_GO_BASE_URL="$GO_BASE_URL" \
  OGAME_AUTH_VISUAL_USER="$LOGIN" \
  OGAME_AUTH_VISUAL_PASS="$PASSWORD" \
  OGAME_AUTH_VISUAL_OUTPUT_DIR="${OGAME_AUTH_VISUAL_OUTPUT_DIR:-$ROOT_DIR/.tmp/playwright-auth-visual/alliance/$AUTH_VISUAL_BROWSER}" \
  OGAME_AUTH_VISUAL_PAGE="${OGAME_AUTH_VISUAL_PAGE:-game-alliance-owned-home,game-alliance-management,game-alliance-members,game-alliance-applications,game-alliance-circular,game-alliance-application-text,game-alliance-settings,game-alliance-ranks}" \
  bun run e2e:visual:auth
