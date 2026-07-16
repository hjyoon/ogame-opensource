#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_RESOURCE_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-resource-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/resource-differential.XXXXXX")"

login="$(jq -r '.resource_scope.owner.login // empty' "$FIXTURE")"
player_id="$(jq -r '.resource_scope.owner.player_id // 0' "$FIXTURE")"
planet_id="$(jq -r '.resource_scope.owner.home_planet_id // 0' "$FIXTURE")"
[ -n "$login" ] && [ "$player_id" -gt 0 ] && [ "$planet_id" -gt 0 ]

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'exec mysql -N -B -uroot -p"$MYSQL_ROOT_PASSWORD" uni -e "$1"' sh "$1" 2>/dev/null
}

snapshot_sql="SELECT prod1,prod2,prod3,prod4,prod12,prod212 FROM uni1_planets WHERE planet_id=$planet_id AND owner_id=$player_id LIMIT 1"
initial="$(db_query "$snapshot_sql")"
[ -n "$initial" ]
old_ifs="$IFS"
IFS="$(printf '\t')"
set -- $initial
IFS="$old_ifs"
[ "$#" -eq 6 ]
restore_sql="UPDATE uni1_planets SET prod1=$1,prod2=$2,prod3=$3,prod4=$4,prod12=$5,prod212=$6 WHERE planet_id=$planet_id AND owner_id=$player_id"
restore() { db_query "$restore_sql" >/dev/null 2>&1 || true; }
cleanup() { restore; rm -rf "$TMP_DIR"; }
trap cleanup EXIT INT TERM

all_pass=true
case_results="$TMP_DIR/cases.jsonl"
: > "$case_results"

run_case() {
  case_name="$1"
  legacy_form="$2"
  go_json="$3"
  expected_row="$(printf '%b' "$4")"
  restore
  legacy_status="$(curl --silent --show-error --max-time 15 \
    --dump-header "$TMP_DIR/$case_name-legacy-login.headers" --output "$TMP_DIR/$case_name-legacy-login.body" \
    --cookie-jar "$TMP_DIR/$case_name-legacy.cookies" --request POST \
    --data-urlencode "login=$login" --data-urlencode "pass=$PASSWORD" \
    --write-out '%{http_code}' "$LEGACY_BASE_URL/game/reg/login2.php")"
  legacy_location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/$case_name-legacy-login.headers")"
  legacy_session="$(printf '%s' "$legacy_location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$legacy_status" = "302" ] && [ -n "$legacy_session" ]
  legacy_update_status="$(curl --silent --show-error --max-time 15 \
    --output "$TMP_DIR/$case_name-legacy.body" --cookie "$TMP_DIR/$case_name-legacy.cookies" --request POST \
    --data "$legacy_form" --write-out '%{http_code}' \
    "$LEGACY_BASE_URL/game/index.php?page=resources&session=$legacy_session&cp=$planet_id")"
  legacy_row="$(db_query "$snapshot_sql")"
  restore
  go_login_payload="$(jq -nc --arg login "$login" --arg pass "$PASSWORD" \
    '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  go_login_status="$(curl --silent --show-error --max-time 15 \
    --dump-header "$TMP_DIR/$case_name-go-login.headers" --output "$TMP_DIR/$case_name-go-login.body" \
    --header 'Content-Type: application/json' --data "$go_login_payload" \
    --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  go_cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/$case_name-go-login.headers" | cut -d';' -f1 | tr -d '\r')"
  go_redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/$case_name-go-login.body")"
  go_session="$(printf '%s' "$go_redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$go_login_status" = "200" ] && [ -n "$go_cookie" ] && [ -n "$go_session" ]
  go_update_status="$(curl --silent --show-error --max-time 15 \
    --output "$TMP_DIR/$case_name-go.body" --cookie "$go_cookie" \
    --header 'Content-Type: application/json' --data "$go_json" --write-out '%{http_code}' \
    "$GO_BASE_URL/api/game/resources?session=$go_session&cp=$planet_id")"
  go_row="$(db_query "$snapshot_sql")"
  restore

  case_pass=true
  [ "$legacy_update_status" = "200" ] || case_pass=false
  [ "$go_update_status" = "200" ] || case_pass=false
  [ "$legacy_row" = "$go_row" ] || case_pass=false
  [ "$go_row" = "$expected_row" ] || case_pass=false
  [ "$case_pass" = true ] || all_pass=false
  jq -nc --arg name "$case_name" --argjson pass "$case_pass" \
    --arg legacyStatus "$legacy_update_status" --arg goStatus "$go_update_status" \
    --arg expected "$expected_row" --arg legacy "$legacy_row" --arg go "$go_row" \
    '{name:$name,pass:$pass,http:{legacy:$legacyStatus,go:$goStatus},db:{expected:$expected,legacy:$legacy,go:$go}}' \
    >> "$case_results"
}

run_case resource-production-partial \
  'last1=80&last2=70&last3=60&last4=100&last12=90&last212=50&action=Recalculate' \
  '{"production":{"1":80,"2":70,"3":60,"4":100,"12":90,"212":50}}' \
  '0.8\t0.7\t0.6\t1\t0.9\t0.5'
run_case resource-production-boundaries \
  'last1=0&last2=10&last3=20&last4=30&last12=40&last212=100&action=Recalculate' \
  '{"production":{"1":0,"2":10,"3":20,"4":30,"12":40,"212":100}}' \
  '0\t0.1\t0.2\t0.3\t0.4\t1'
run_case resource-production-full \
  'last1=100&last2=100&last3=100&last4=100&last12=100&last212=100&action=Recalculate' \
  '{"production":{"1":100,"2":100,"3":100,"4":100,"12":100,"212":100}}' \
  '1\t1\t1\t1\t1\t1'

jq -s --argjson pass "$all_pass" --arg initial "$initial" \
  '{pass:$pass,initial:$initial,cases:.}' "$case_results" > "$REPORT"
[ "$all_pass" = true ]
printf 'Go/PHP resource differential E2E: PASS (3 cases, planet=%s)\n' "$planet_id"
