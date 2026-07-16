#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
BASE_URL="${OGAME_SQLITE_BASE_URL:-http://127.0.0.1:${OGAME_SQLITE_PORT:-8891}}"
UNIVERSE_URL="${OGAME_SQLITE_PUBLIC_BASE_URL:-http://localhost:${OGAME_SQLITE_PORT:-8891}}"
COMPOSE="docker compose -f $ROOT_DIR/docker-compose.yml"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

$COMPOSE up -d --build goapp-sqlite

attempt=0
until curl --silent --fail --max-time 5 "$BASE_URL/api/healthz" >"$TMP_DIR/ready.json"; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    $COMPOSE logs --tail=100 goapp-sqlite
    exit 1
  fi
  sleep 1
done

jq -e '.status == "ok" and .masterDbReady == true and .universeDbReady == true' "$TMP_DIR/ready.json" >/dev/null
curl --silent --fail --max-time 10 "$BASE_URL/" | grep -q '<div id="root">'

payload="$(jq -nc --arg universe "$UNIVERSE_URL" '{login:"legor",pass:"admin",universe:$universe}')"
curl --silent --fail --max-time 10 --dump-header "$TMP_DIR/login.headers" \
  --output "$TMP_DIR/login.json" --header 'Content-Type: application/json' --data "$payload" \
  "$BASE_URL/api/public/login"
jq -e '.valid == true and (.session.redirectTo | startswith("/game/overview"))' "$TMP_DIR/login.json" >/dev/null
session="$(jq -r '.session.redirectTo' "$TMP_DIR/login.json" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/login.headers" | cut -d';' -f1 | tr -d '\r')"
[ -n "$cookie" ] && [ -n "$session" ]

for endpoint in overview buildings resources research shipyard defense fleet galaxy messages options; do
  query="session=$session&cp=1"
  [ "$endpoint" = galaxy ] && query="$query&galaxy=1&system=1"
  curl --silent --fail --max-time 10 --header "Cookie: $cookie" "$BASE_URL/api/game/$endpoint?$query" >"$TMP_DIR/$endpoint.json"
  jq -e 'type == "object"' "$TMP_DIR/$endpoint.json" >/dev/null
done

registration_name="s$(date +%s)$$"
registration_payload="$(jq -nc --arg character "$registration_name" --arg email "$registration_name@example.local" --arg universe "$UNIVERSE_URL" '{character:$character,password:"Sqlite123!",email:$email,universe:$universe,agb:true}')"
curl --silent --fail --max-time 10 --dump-header "$TMP_DIR/registration.headers" \
  --output "$TMP_DIR/registration.json" --header 'Content-Type: application/json' --data "$registration_payload" \
  "$BASE_URL/api/public/registration"
jq -e '.valid == true and .created == true and .account.playerId > 1 and .account.homePlanetId >= 10000' "$TMP_DIR/registration.json" >/dev/null
registration_session="$(jq -r '.session.redirectTo' "$TMP_DIR/registration.json" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
registration_cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/registration.headers" | cut -d';' -f1 | tr -d '\r')"
registration_planet="$(jq -r '.account.homePlanetId' "$TMP_DIR/registration.json")"
[ -n "$registration_session" ] && [ -n "$registration_cookie" ]

sleep 2
curl --silent --fail --max-time 10 --header "Cookie: $registration_cookie" \
  --header 'Content-Type: application/json' --data '{"action":"add","techId":1,"listId":0}' \
  "$BASE_URL/api/game/buildings?session=$registration_session&cp=$registration_planet" >"$TMP_DIR/building-add.json"
jq -e '.authenticated == true and .actionIssue == null and (.buildings.queue | length) == 1' "$TMP_DIR/building-add.json" >/dev/null

printf 'SQLite E2E passed: %s (bootstrap, registration, build mutation, and 10 authenticated APIs)\n' "$BASE_URL"
