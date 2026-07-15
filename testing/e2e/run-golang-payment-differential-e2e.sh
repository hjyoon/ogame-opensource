#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_PAYMENT_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-payment-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"
VALID_CODE='E2E1-AAAA-BBBB-CCCC-DDDD'
USED_CODE='E2E2-AAAA-BBBB-CCCC-DDDD'
MISSING_CODE='E2E3-AAAA-BBBB-CCCC-DDDD'

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/payment-differential.XXXXXX")"

actor_id="$(jq -r '.planet_context.owner.player_id // 0' "$FIXTURE")"
actor_login="$(jq -r '.planet_context.owner.login // empty' "$FIXTURE")"
planet_id="$(jq -r '.planet_context.owner.home_planet_id // 0' "$FIXTURE")"
[ "$actor_id" -gt 0 ] && [ "$planet_id" -gt 0 ] && [ -n "$actor_login" ]

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

users_backup="uni1_e2e_paymentdiff_users_$$"
coupon_backup="e2e_paymentdiff_coupons_$$"
db_query "DROP TABLE IF EXISTS uni.$users_backup,master.$coupon_backup;
CREATE TABLE uni.$users_backup AS SELECT * FROM uni.uni1_users WHERE player_id=$actor_id;
CREATE TABLE master.$coupon_backup AS SELECT * FROM master.coupons WHERE code IN ('$VALID_CODE','$USED_CODE','$MISSING_CODE')" >/dev/null

restore_state() {
  db_query "UPDATE uni.uni1_users u JOIN uni.$users_backup b ON b.player_id=u.player_id SET
u.session=b.session,u.private_session=b.private_session,u.password=b.password,u.validated=b.validated,
u.vacation=b.vacation,u.vacation_until=b.vacation_until,u.disable=b.disable,u.disable_until=b.disable_until,
u.admin=b.admin,u.lang=b.lang,u.aktplanet=b.aktplanet,u.dm=b.dm,u.dmfree=b.dmfree;
DELETE FROM master.coupons WHERE code IN ('$VALID_CODE','$USED_CODE','$MISSING_CODE');
INSERT INTO master.coupons SELECT * FROM master.$coupon_backup" >/dev/null
}

cleanup() {
  restore_state >/dev/null 2>&1 || true
  db_query "DROP TABLE IF EXISTS uni.$users_backup,master.$coupon_backup" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

reset_case() {
  restore_state
  db_query "UPDATE uni.uni1_users SET session='',private_session='',validated=1,vacation=0,vacation_until=0,
disable=0,disable_until=0,admin=0,lang='en',aktplanet=$planet_id,dm=12345,dmfree=678 WHERE player_id=$actor_id;
DELETE FROM master.coupons WHERE code IN ('$VALID_CODE','$USED_CODE','$MISSING_CODE');
INSERT INTO master.coupons(code,amount,used,user_uni,user_id,user_name) VALUES
('$VALID_CODE',76543,0,0,0,''),('$USED_CODE',43210,1,1,99999,'already-used')" >/dev/null
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
  case_name="$1"; action=check; code="$VALID_CODE"; requests=1
  case "$case_name" in
    check-valid) ;;
    check-missing) code="$MISSING_CODE" ;;
    check-used) code="$USED_CODE" ;;
    check-empty) code='' ;;
    activate-valid) action=activate ;;
    activate-missing) action=activate; code="$MISSING_CODE" ;;
    activate-used) action=activate; code="$USED_CODE" ;;
    activate-twice) action=activate; requests=2 ;;
    *) printf 'Unknown payment differential case: %s\n' "$case_name" >&2; exit 1 ;;
  esac
}

capture_state() {
  user="$(db_query "SELECT JSON_OBJECT('dm',dm,'free',dmfree) FROM uni.uni1_users WHERE player_id=$actor_id")"
  coupons="$(db_query "SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT('code',code,'amount',amount,'used',used,
'universe',user_uni,'user',IF(user_id=$actor_id,'actor',CAST(user_id AS CHAR)),'name',user_name)),JSON_ARRAY())
FROM (SELECT * FROM master.coupons WHERE code IN ('$VALID_CODE','$USED_CODE','$MISSING_CODE') ORDER BY code) c")"
  state="$(jq -ncS --argjson user "$user" --argjson coupons "$coupons" '{user:$user,coupons:$coupons}')"
}

legacy_request() {
  index="$1"
  url="$LEGACY_BASE_URL/game/index.php?page=payment&session=$session&cp=$planet_id"
  http="$(curl --silent --show-error --max-time 20 --dump-header "$TMP_DIR/legacy-$case_name-$index.headers" \
    --output "$TMP_DIR/legacy-$case_name-$index.body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST \
    --data-urlencode "action=$action" --data-urlencode "couponcode=$code" --write-out '%{http_code}' "$url")"
}

run_legacy_action() {
  legacy_request 1
  [ "$requests" -eq 1 ] || legacy_request 2
  if [ "$action" = check ]; then [ "$http" = 200 ]; else [ "$http" = 302 ]; fi
  capture_state
  if [ "$action" = check ]; then
    if grep -q "$VALID_CODE" "$TMP_DIR/legacy-$case_name-1.body"; then issue=coupon_valid; else issue=invalid_coupon; fi
  elif printf '%s' "$state" | jq -e --arg code "$VALID_CODE" '.coupons[] | select(.code == $code and .used == 1 and .user == "actor")' >/dev/null; then
    [ "$requests" -eq 1 ] && issue=coupon_activated || issue=invalid_coupon
  else
    issue=invalid_coupon
  fi
}

go_request() {
  index="$1"
  payload="$(jq -nc --arg action "$action" --arg code "$code" '{action:$action,couponCode:$code}')"
  http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/go-$case_name-$index.body" \
    --header 'Content-Type: application/json' --cookie "$cookie" --data "$payload" --write-out '%{http_code}' \
    "$GO_BASE_URL/api/game/payment?session=$session&cp=$planet_id")"
  issue="$(jq -r '.actionIssue.code // empty' "$TMP_DIR/go-$case_name-$index.body" 2>/dev/null || true)"
}

run_go_action() {
  go_request 1
  [ "$requests" -eq 1 ] || go_request 2
  [ "$http" = 200 ]
  capture_state
}

run_side() {
  side="$1"; case_name="$2"
  reset_case; configure_case "$case_name"
  if [ "$side" = legacy ]; then login_legacy "$side-$case_name"; run_legacy_action
  else login_go "$side-$case_name"; run_go_action; fi
  normalized="$(jq -ncS --arg issue "$issue" --argjson state "$state" '{issue:$issue,state:$state}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true; case_count=0
cases='check-valid check-missing check-used check-empty activate-valid activate-missing activate-used activate-twice'
for case_name in $cases; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$legacy" = "$go" ] || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "payment-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
  case_count=$((case_count + 1))
done

jq -s --argjson pass "$all_pass" --argjson cases "$case_count" \
  '{pass:$pass,normalization:"legacy redirect and Go JSON transport are reduced to payment semantics; coupon ownership and paid/free DM effects remain exact",metrics:{cases:$cases},cases:.}' \
  "$results" > "$REPORT"
[ "$all_pass" = true ] || { jq '.cases[] | select(.pass == false)' "$REPORT" >&2; exit 1; }
printf 'Go/PHP payment differential E2E: PASS (%s cases)\n' "$case_count"
