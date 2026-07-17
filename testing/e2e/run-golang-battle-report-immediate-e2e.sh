#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH='' cd -- "$SCRIPT_DIR/../.." && pwd)"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
CONTAINER_DIR="${OGAME_E2E_CONTAINER_DIR:-/tmp/ogame-e2e}"
QUEUE_DURATION="${OGAME_BATTLE_REPORT_QUEUE_DURATION:-8}"
FIXTURE_FILE="$ROOT_DIR/.tmp/battle-report-immediate-fixture.json"
FIRST_FILE="$ROOT_DIR/.tmp/battle-report-immediate-first.json"
RELOAD_FILE="$ROOT_DIR/.tmp/battle-report-immediate-reload.json"
REPORT_FILE="$ROOT_DIR/.tmp/battle-report-immediate-report.json"

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

fetch_messages() {
  output="$1"
  curl --fail --silent --show-error \
    -H "Cookie: $COOKIE_NAME=$COOKIE_VALUE" \
    "$GO_BASE_URL/api/game/messages?session=$SESSION&cp=$PLANET_ID" \
    -o "$output"
}

mkdir -p "$ROOT_DIR/.tmp"
wait_for_url "$GO_BASE_URL/api/healthz"
docker compose exec -T server mkdir -p "$CONTAINER_DIR"
docker compose cp "$SCRIPT_DIR/cleanup-golang-migration-fixtures.php" "server:$CONTAINER_DIR/cleanup-golang-migration-fixtures.php" >/dev/null
docker compose cp "$SCRIPT_DIR/prepare-overview-fleet-fixture.php" "server:$CONTAINER_DIR/prepare-overview-fleet-fixture.php" >/dev/null
trap cleanup_fixture EXIT INT TERM

docker compose exec -T \
  -e "OGAME_OVERVIEW_FLEET_ENEMY_QUEUE_DURATION=$QUEUE_DURATION" \
  server php "$CONTAINER_DIR/prepare-overview-fleet-fixture.php" >"$FIXTURE_FILE"

SESSION="$(jq -er '.session' "$FIXTURE_FILE")"
PLANET_ID="$(jq -er '.planet_id' "$FIXTURE_FILE")"
PREPARED_AT="$(jq -er '.prepared_at' "$FIXTURE_FILE")"
COOKIE_NAME="$(jq -er '.private_cookie_name' "$FIXTURE_FILE")"
COOKIE_VALUE="$(jq -er '.private_cookie_value' "$FIXTURE_FILE")"

sleep $((QUEUE_DURATION + 2))
fetch_messages "$FIRST_FILE"
jq -e --argjson prepared "$PREPARED_AT" '
  .authenticated == true and
  ([.messages.rows[] | select(.type == 2 and .date >= $prepared and (.subject | contains("combatreport_")))] | length) == 1
' "$FIRST_FILE" >/dev/null

REPORT_ID="$(jq -er --argjson prepared "$PREPARED_AT" '.messages.rows[] | select(.type == 2 and .date >= $prepared and (.subject | contains("combatreport_"))) | .id' "$FIRST_FILE")"
TEXT_REPORT_ID="$(jq -er --argjson prepared "$PREPARED_AT" '
  .messages.rows[]
  | select(.type == 2 and .date >= $prepared and (.subject | contains("combatreport_")))
  | .subject
  | capture("bericht=(?<id>[0-9]+)")
  | .id
' "$FIRST_FILE")"
fetch_messages "$RELOAD_FILE"
jq -e --argjson report "$REPORT_ID" --argjson prepared "$PREPARED_AT" '
  .authenticated == true and
  any(.messages.rows[]; .id == $report and .type == 2) and
  ([.messages.rows[] | select(.type == 2 and .date >= $prepared and (.subject | contains("combatreport_")))] | length) == 1
' "$RELOAD_FILE" >/dev/null

curl --fail --silent --show-error \
  -H "Cookie: $COOKIE_NAME=$COOKIE_VALUE" \
  "$GO_BASE_URL/api/game/report?session=$SESSION&bericht=$TEXT_REPORT_ID" \
  -o "$REPORT_FILE"
jq -e '
  .authenticated == true and
  .report.allowed == true and
  (.report.text | test("The (attacker|defender) has won the battle!|The battle ended in a draw")) and
  (.report.text | contains("The attacker lost a total")) and
  (.report.text | contains("The defender lost a total")) and
  (.report.text | contains("At these space coordinates now float")) and
  (.report.text | contains("</table>"))
' "$REPORT_FILE" >/dev/null

for browser in ${OGAME_BATTLE_REPORT_BROWSERS:-chromium firefox}; do
  (
    cd "$ROOT_DIR/frontend"
    OGAME_PLAYWRIGHT_BROWSER="$browser" \
    OGAME_GO_BASE_URL="$GO_BASE_URL" \
    OGAME_BATTLE_REPORT_SESSION="$SESSION" \
    OGAME_BATTLE_REPORT_ID="$TEXT_REPORT_ID" \
    OGAME_BATTLE_REPORT_COOKIE_NAME="$COOKIE_NAME" \
    OGAME_BATTLE_REPORT_COOKIE_VALUE="$COOKIE_VALUE" \
      bun run e2e:battle-report-completeness
  )
done

printf 'Immediate battle report E2E passed: link=%s report=%s\n' "$REPORT_ID" "$TEXT_REPORT_ID"
