#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
ACCELERATED_FLEET_SPEED="${OGAME_ACCELERATED_FLEET_SPEED:-128}"
RECALL_ELAPSED_SECONDS="${OGAME_ACCELERATED_RECALL_ELAPSED_SECONDS:-2}"
REPORT="${OGAME_ACCELERATED_RECALL_REPORT:-$ROOT_DIR/.tmp/golang-fleet-speed-recall-differential.json}"

case "$ACCELERATED_FLEET_SPEED:$RECALL_ELAPSED_SECONDS" in
  *[!0-9:]*) printf 'Accelerated fleet speed and Recall elapsed time must be positive integers.\n' >&2; exit 1 ;;
  0:*|*:0) printf 'Accelerated fleet speed and Recall elapsed time must be greater than zero.\n' >&2; exit 1 ;;
esac

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'exec mysql -N -B -r -uroot -p"$MYSQL_ROOT_PASSWORD" uni -e "$1"' sh "$1" 2>/dev/null
}

original_fleet_speed="$(db_query 'SELECT fspeed FROM uni1_uni LIMIT 1')"
restore_fleet_speed() {
  db_query "UPDATE uni1_uni SET fspeed=$original_fleet_speed" >/dev/null 2>&1 || true
}
trap restore_fleet_speed EXIT INT TERM

db_query "UPDATE uni1_uni SET fspeed=$ACCELERATED_FLEET_SPEED" >/dev/null
OGAME_FLEET_DIFFERENTIAL_CASES=recall \
OGAME_FLEET_RECALL_ELAPSED_SECONDS="$RECALL_ELAPSED_SECONDS" \
OGAME_FLEET_DIFFERENTIAL_REPORT="$REPORT" \
"$SCRIPT_DIR/run-golang-fleet-differential-e2e.sh"

jq -e \
  --argjson speed "$ACCELERATED_FLEET_SPEED" \
  --argjson elapsed "$RECALL_ELAPSED_SECONDS" \
  '.pass and .fixture.fleetSpeed == $speed and .fixture.recallElapsedSeconds == $elapsed and (.cases | length) == 1 and .cases[0].name == "fleet-recall" and .cases[0].pass' \
  "$REPORT" >/dev/null
printf 'Accelerated fleet Recall differential E2E: PASS (fspeed=%s, elapsed=%ss)\n' "$ACCELERATED_FLEET_SPEED" "$RECALL_ELAPSED_SECONDS"
