#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH='' cd -- "$SCRIPT_DIR/../.." && pwd)"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
CONTAINER_DIR="${OGAME_E2E_CONTAINER_DIR:-/tmp/ogame-e2e}"
QUEUE_DURATION="${OGAME_OVERVIEW_FLEET_REFRESH_DURATION:-8}"
FIXTURE_FILE="$ROOT_DIR/.tmp/overview-fleet-refresh-fixture.json"
BEFORE_FILE="$ROOT_DIR/.tmp/overview-fleet-refresh-before.json"
AFTER_FILE="$ROOT_DIR/.tmp/overview-fleet-refresh-after.json"
RELOAD_FILE="$ROOT_DIR/.tmp/overview-fleet-refresh-reload.json"

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

cleanup_fixture() {
  if [ "${OGAME_CLEAN_MIGRATION_FIXTURES:-1}" = "1" ]; then
    docker compose exec -T server php "$CONTAINER_DIR/cleanup-golang-migration-fixtures.php" >/dev/null 2>&1 || true
  fi
}

fetch_overview() {
  output="$1"
  curl --fail --silent --show-error \
    -H "Cookie: $COOKIE_NAME=$COOKIE_VALUE" \
    "$GO_BASE_URL/api/game/overview?session=$SESSION&cp=$PLANET_ID" \
    -o "$output"
}

mkdir -p "$ROOT_DIR/.tmp"
wait_for_url "$GO_BASE_URL/api/healthz"
docker compose exec -T server mkdir -p "$CONTAINER_DIR"
docker compose cp "$SCRIPT_DIR/cleanup-golang-migration-fixtures.php" "server:$CONTAINER_DIR/cleanup-golang-migration-fixtures.php" >/dev/null
docker compose cp "$SCRIPT_DIR/prepare-overview-fleet-fixture.php" "server:$CONTAINER_DIR/prepare-overview-fleet-fixture.php" >/dev/null
trap cleanup_fixture EXIT INT TERM

docker compose exec -T \
  -e "OGAME_OVERVIEW_FLEET_OWN_QUEUE_DURATION=$QUEUE_DURATION" \
  server php "$CONTAINER_DIR/prepare-overview-fleet-fixture.php" >"$FIXTURE_FILE"

SESSION="$(jq -er '.session' "$FIXTURE_FILE")"
PLANET_ID="$(jq -er '.planet_id' "$FIXTURE_FILE")"
PLAYER_ID="$(jq -er '.player_id' "$FIXTURE_FILE")"
FLEET_ID="$(jq -er '.own_fleet_id' "$FIXTURE_FILE")"
COOKIE_NAME="$(jq -er '.private_cookie_name' "$FIXTURE_FILE")"
COOKIE_VALUE="$(jq -er '.private_cookie_value' "$FIXTURE_FILE")"

fetch_overview "$BEFORE_FILE"
jq -e --argjson fleet "$FLEET_ID" '
  .authenticated == true and
  any(.overview.events[]; .id == $fleet and .mission == 3)
' "$BEFORE_FILE" >/dev/null

sleep $((QUEUE_DURATION + 2))
fetch_overview "$AFTER_FILE"
jq -e --argjson fleet "$FLEET_ID" --argjson owner "$PLAYER_ID" '
  .authenticated == true and
  (any(.overview.events[]; .id == $fleet and .mission == 3) | not) and
  any(.overview.events[]; .id > 0 and .ownerId == $owner and .mission == 103)
' "$AFTER_FILE" >/dev/null

RETURN_FLEET_ID="$(jq -er --argjson owner "$PLAYER_ID" '.overview.events[] | select(.id > 0 and .ownerId == $owner and .mission == 103) | .id' "$AFTER_FILE")"
fetch_overview "$RELOAD_FILE"
jq -e --argjson fleet "$RETURN_FLEET_ID" '
  .authenticated == true and
  any(.overview.events[]; .id == $fleet and .mission == 103)
' "$RELOAD_FILE" >/dev/null

printf 'Overview fleet refresh E2E passed: outbound=%s return=%s\n' "$FLEET_ID" "$RETURN_FLEET_ID"
