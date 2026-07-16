#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
LOGIN_USER="${OGAME_RECOVERY_LOGIN_USER:-legor}"
LOGIN_PASS="${OGAME_RECOVERY_LOGIN_PASS:-admin}"
UNIVERSE="${OGAME_RECOVERY_UNIVERSE:-http://localhost:8888}"
REPORT="${OGAME_RECOVERY_REPORT:-$ROOT_DIR/.tmp/golang-db-recovery.json}"

login() {
  curl --fail --silent -H 'Content-Type: application/json' \
    --data "{\"login\":\"$LOGIN_USER\",\"pass\":\"$LOGIN_PASS\",\"universe\":\"$UNIVERSE\"}" \
    "$BASE_URL/api/public/login"
}

wait_ready() {
  i=0
  while [ "$i" -lt 60 ]; do
    if curl --fail --silent "$BASE_URL/api/healthz" >/dev/null 2>&1; then return 0; fi
    i=$((i + 1)); sleep 1
  done
  return 1
}

restore_db() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" start mysql >/dev/null 2>&1 || true
}
trap restore_db EXIT INT TERM

login | jq -e '.valid == true' >/dev/null
docker compose -f "$ROOT_DIR/docker-compose.yml" stop mysql >/dev/null
curl --fail --silent "$BASE_URL/api/livez" | jq -e '.status == "ok"' >/dev/null
status="$(curl --silent --output /tmp/ogame-recovery-health.json --write-out '%{http_code}' "$BASE_URL/api/healthz")"
[ "$status" = "503" ]
jq -e '.status == "unavailable" and .masterDbReady == false and .universeDbReady == false' /tmp/ogame-recovery-health.json >/dev/null
restore_db
wait_ready
login | jq -e '.valid == true' >/dev/null

# Reproduce a process starting while its database is unavailable. `restart`
# does not start dependencies, so recovery must come from the retained pools.
docker compose -f "$ROOT_DIR/docker-compose.yml" stop mysql >/dev/null
docker compose -f "$ROOT_DIR/docker-compose.yml" restart goapp >/dev/null
curl --retry 30 --retry-delay 1 --retry-all-errors --fail --silent "$BASE_URL/api/livez" >/dev/null
status="$(curl --silent --output /tmp/ogame-startup-recovery-health.json --write-out '%{http_code}' "$BASE_URL/api/healthz")"
[ "$status" = "503" ]
restore_db
wait_ready
login | jq -e '.valid == true' >/dev/null
trap - EXIT INT TERM
mkdir -p "$(dirname -- "$REPORT")"
printf '%s\n' '{"pass":true,"cases":[{"name":"mysql-stop-readiness-recovery","pass":true},{"name":"go-starts-before-mysql-recovery","pass":true}]}' > "$REPORT"
printf 'Go DB recovery E2E: PASS\n'
