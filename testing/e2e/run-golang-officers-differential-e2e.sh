#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_OFFICERS_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-officers-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/officers-differential.XXXXXX")"

actor_id="$(jq -r '.premium_dm.invalid.player_id // 0' "$FIXTURE")"
actor_login="$(jq -r '.premium_dm.invalid.login // empty' "$FIXTURE")"
planet_id="$(jq -r '.premium_dm.invalid.home_planet_id // 0' "$FIXTURE")"
[ "$actor_id" -gt 0 ] && [ "$planet_id" -gt 0 ] && [ -n "$actor_login" ]

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

backup="uni1_e2e_officersdiff_users_$$"
db_query "DROP TABLE IF EXISTS $backup; CREATE TABLE $backup AS SELECT * FROM uni1_users WHERE player_id=$actor_id" >/dev/null

restore_user() {
  db_query "UPDATE uni1_users u JOIN $backup b ON b.player_id=u.player_id SET
u.session=b.session,u.private_session=b.private_session,u.password=b.password,u.validated=b.validated,
u.vacation=b.vacation,u.vacation_until=b.vacation_until,u.disable=b.disable,u.disable_until=b.disable_until,
u.admin=b.admin,u.lang=b.lang,u.aktplanet=b.aktplanet,u.dm=b.dm,u.dmfree=b.dmfree,
u.com_until=b.com_until,u.adm_until=b.adm_until,u.eng_until=b.eng_until,u.geo_until=b.geo_until,u.tec_until=b.tec_until" >/dev/null
}

cleanup() {
  restore_user >/dev/null 2>&1 || true
  db_query "DROP TABLE IF EXISTS $backup" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

reset_case() {
  restore_user
  db_query "UPDATE uni1_users SET session='',private_session='',validated=1,vacation=0,vacation_until=0,
disable=0,disable_until=0,admin=0,lang='en',aktplanet=$planet_id,dm=500000,dmfree=0,
com_until=0,adm_until=0,eng_until=0,geo_until=0,tec_until=0 WHERE player_id=$actor_id" >/dev/null
}

login_legacy() {
  prefix="$1"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" \
    --output "$TMP_DIR/$prefix-login.body" --cookie-jar "$TMP_DIR/$prefix.cookies" --request POST \
    --data-urlencode "login=$actor_login" --data-urlencode "pass=$PASSWORD" --write-out '%{http_code}' \
    "$LEGACY_BASE_URL/game/reg/login2.php")"
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/$prefix-login.headers")"
  session="$(printf '%s' "$location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 302 ] && [ -n "$session" ]
}

login_go() {
  prefix="$1"
  payload="$(jq -nc --arg login "$actor_login" --arg pass "$PASSWORD" '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" \
    --output "$TMP_DIR/$prefix-login.body" --header 'Content-Type: application/json' --data "$payload" \
    --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/$prefix-login.headers" | cut -d';' -f1 | tr -d '\r')"
  redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/$prefix-login.body")"
  session="$(printf '%s' "$redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 200 ] && [ -n "$cookie" ] && [ -n "$session" ]
}

configure_case() {
  case_name="$1"; officer_id=1; days=7; dm=10000; dmfree=0
  case "$case_name" in
    commander-week) officer_id=1 ;;
    admiral-week) officer_id=2 ;;
    engineer-week) officer_id=3 ;;
    geologist-week) officer_id=4 ;;
    technocrat-week) officer_id=5 ;;
    commander-quarter) officer_id=1; days=90; dm=100000 ;;
    mixed-balances) officer_id=3; dm=4000; dmfree=7000 ;;
    insufficient) officer_id=2; dm=9999 ;;
    extend-active)
      officer_id=4
      db_query "UPDATE uni1_users SET geo_until=UNIX_TIMESTAMP()+200000 WHERE player_id=$actor_id" >/dev/null ;;
    invalid-type) officer_id=99 ;;
    invalid-days) officer_id=1; days=8 ;;
    *) printf 'Unknown officers differential case: %s\n' "$case_name" >&2; exit 1 ;;
  esac
  db_query "UPDATE uni1_users SET dm=$dm,dmfree=$dmfree WHERE player_id=$actor_id" >/dev/null
}

normalized_remaining() {
  value="$1"
  if [ "$value" -le 0 ]; then printf '0'; return; fi
  for target in 604800 7776000 804800; do
    delta=$((value - target)); [ "$delta" -lt 0 ] && delta=$((-delta))
    if [ "$delta" -le 10 ]; then printf '%s' "$target"; return; fi
  done
  printf '%s' "$value"
}

capture_state() {
  row="$(db_query "SELECT dm,dmfree,
GREATEST(0,CAST(com_until AS SIGNED)-UNIX_TIMESTAMP()),GREATEST(0,CAST(adm_until AS SIGNED)-UNIX_TIMESTAMP()),
GREATEST(0,CAST(eng_until AS SIGNED)-UNIX_TIMESTAMP()),GREATEST(0,CAST(geo_until AS SIGNED)-UNIX_TIMESTAMP()),
GREATEST(0,CAST(tec_until AS SIGNED)-UNIX_TIMESTAMP()) FROM uni1_users WHERE player_id=$actor_id")"
  old_ifs="$IFS"; IFS="$(printf '\t')"
  # shellcheck disable=SC2086 # Split the tab-delimited SQL row into positional fields.
  set -- $row
  IFS="$old_ifs"
  state="$(jq -ncS --argjson dm "$1" --argjson free "$2" \
    --argjson commander "$(normalized_remaining "$3")" --argjson admiral "$(normalized_remaining "$4")" \
    --argjson engineer "$(normalized_remaining "$5")" --argjson geologist "$(normalized_remaining "$6")" \
    --argjson technocrat "$(normalized_remaining "$7")" \
    '{dm:$dm,free:$free,timers:{commander:$commander,admiral:$admiral,engineer:$engineer,geologist:$geologist,technocrat:$technocrat}}')"
}

run_legacy_action() {
  url="$LEGACY_BASE_URL/game/index.php?page=micropayment&session=$session&cp=$planet_id&buynow=1&type=$officer_id&days=$days"
  http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/legacy-$case_name.body" \
    --cookie "$TMP_DIR/legacy-$case_name.cookies" --write-out '%{http_code}' "$url")"
  issue=''
  grep -q 'Not enough dark matter' "$TMP_DIR/legacy-$case_name.body" && issue=not_enough_dark_matter || true
  grep -q 'renewal was successful' "$TMP_DIR/legacy-$case_name.body" && issue=recruited || true
}

run_go_action() {
  payload="$(jq -nc --argjson officer "$officer_id" --argjson days "$days" '{officerId:$officer,days:$days}')"
  http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/go-$case_name.body" \
    --header 'Content-Type: application/json' --cookie "$cookie" --data "$payload" --write-out '%{http_code}' \
    "$GO_BASE_URL/api/game/officers?session=$session&cp=$planet_id")"
  issue="$(jq -r '.actionIssue.code // empty' "$TMP_DIR/go-$case_name.body" 2>/dev/null || true)"
}

run_side() {
  side="$1"; case_name="$2"
  reset_case; configure_case "$case_name"
  if [ "$side" = legacy ]; then login_legacy "$side-$case_name"; run_legacy_action
  else login_go "$side-$case_name"; run_go_action; fi
  capture_state
  normalized="$(jq -ncS --argjson http "$http" --arg issue "$issue" --argjson state "$state" '{http:$http,issue:$issue,state:$state}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true; case_count=0
cases='commander-week admiral-week engineer-week geologist-week technocrat-week commander-quarter mixed-balances insufficient extend-active invalid-type invalid-days'
for case_name in $cases; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.state')" = "$(printf '%s' "$go" | jq -cS '.state')" ] || pass=false
  [ "$(printf '%s' "$legacy" | jq -r '.issue')" = "$(printf '%s' "$go" | jq -r '.issue')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http == 200' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http == 200' >/dev/null || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "officers-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
  case_count=$((case_count + 1))
done

jq -s --argjson pass "$all_pass" --argjson cases "$case_count" \
  '{pass:$pass,normalization:"session and absolute clock excluded; officer balances and timer durations exact within a 10-second request window",metrics:{cases:$cases},cases:.}' \
  "$results" > "$REPORT"
[ "$all_pass" = true ] || { jq '.cases[] | select(.pass == false)' "$REPORT" >&2; exit 1; }
printf 'Go/PHP officers differential E2E: PASS (%s cases)\n' "$case_count"
