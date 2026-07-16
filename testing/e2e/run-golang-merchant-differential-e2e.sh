#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_MERCHANT_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-merchant-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/merchant-differential.XXXXXX")"

actor_id="$(jq -r '.merchant.call.player_id // 0' "$FIXTURE")"
actor_login="$(jq -r '.merchant.call.login // empty' "$FIXTURE")"
planet_id="$(jq -r '.merchant.call.home_planet_id // 0' "$FIXTURE")"
[ "$actor_id" -gt 0 ] && [ "$planet_id" -gt 0 ] && [ -n "$actor_login" ]

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

users_backup="uni1_e2e_merchantdiff_users_$$"
planets_backup="uni1_e2e_merchantdiff_planets_$$"
db_query "DROP TABLE IF EXISTS $users_backup,$planets_backup;
CREATE TABLE $users_backup AS SELECT * FROM uni1_users WHERE player_id=$actor_id;
CREATE TABLE $planets_backup AS SELECT * FROM uni1_planets WHERE planet_id=$planet_id" >/dev/null

restore_state() {
  db_query "UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id SET
u.session=b.session,u.private_session=b.private_session,u.password=b.password,u.validated=b.validated,
u.vacation=b.vacation,u.vacation_until=b.vacation_until,u.disable=b.disable,u.disable_until=b.disable_until,
u.admin=b.admin,u.lang=b.lang,u.aktplanet=b.aktplanet,u.dm=b.dm,u.dmfree=b.dmfree,u.trader=b.trader,
u.rate_m=b.rate_m,u.rate_k=b.rate_k,u.rate_d=b.rate_d;
UPDATE uni1_planets p JOIN $planets_backup b ON b.planet_id=p.planet_id SET
p.\`700\`=b.\`700\`,p.\`701\`=b.\`701\`,p.\`702\`=b.\`702\`,p.\`22\`=b.\`22\`,p.\`23\`=b.\`23\`,p.\`24\`=b.\`24\`,
p.prod1=b.prod1,p.prod2=b.prod2,p.prod3=b.prod3,p.prod4=b.prod4,p.prod12=b.prod12,p.prod212=b.prod212,p.lastpeek=b.lastpeek" >/dev/null
}

cleanup() {
  restore_state >/dev/null 2>&1 || true
  db_query "DROP TABLE IF EXISTS $users_backup,$planets_backup" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

reset_case() {
  restore_state
  db_query "UPDATE uni1_users SET session='',private_session='',validated=1,vacation=0,vacation_until=0,
disable=0,disable_until=0,admin=0,lang='en',aktplanet=$planet_id,dm=100000,dmfree=0,
trader=0,rate_m=3,rate_k=2,rate_d=1 WHERE player_id=$actor_id;
UPDATE uni1_planets SET \`700\`=100000,\`701\`=100000,\`702\`=100000,\`22\`=10,\`23\`=10,\`24\`=10,
prod1=0,prod2=0,prod3=0,prod4=0,prod12=0,prod212=0,lastpeek=UNIX_TIMESTAMP() WHERE planet_id=$planet_id" >/dev/null
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
  case_name="$1"; action=call; offer_id=1; metal=0; crystal=0; deuterium=0; random_rates=false
  case "$case_name" in
    call-metal) random_rates=true; db_query "UPDATE uni1_users SET dm=1000,dmfree=2000 WHERE player_id=$actor_id" >/dev/null ;;
    call-crystal) offer_id=2; random_rates=true ;;
    call-deuterium) offer_id=3; random_rates=true ;;
    call-replace) offer_id=3; random_rates=true; db_query "UPDATE uni1_users SET trader=2 WHERE player_id=$actor_id" >/dev/null ;;
    call-insufficient) db_query "UPDATE uni1_users SET dm=2499,dmfree=0 WHERE player_id=$actor_id" >/dev/null ;;
    call-invalid) offer_id=0 ;;
    trade-metal) action=trade; offer_id=1; crystal=2000; deuterium=1000 ;;
    trade-crystal) action=trade; offer_id=2; metal=3000; deuterium=1000 ;;
    trade-deuterium) action=trade; offer_id=3; metal=3000; crystal=2000 ;;
    trade-resource-reject)
      action=trade; offer_id=1; crystal=2000; deuterium=1000
      db_query "UPDATE uni1_planets SET \`700\`=1000 WHERE planet_id=$planet_id" >/dev/null ;;
    trade-storage-reject)
      action=trade; offer_id=1; crystal=2000; deuterium=1000
      db_query "UPDATE uni1_planets SET \`700\`=9000,\`701\`=9000,\`702\`=9000,\`22\`=0,\`23\`=0,\`24\`=0 WHERE planet_id=$planet_id" >/dev/null ;;
    trade-zero) action=trade; offer_id=1 ;;
    *) printf 'Unknown merchant differential case: %s\n' "$case_name" >&2; exit 1 ;;
  esac
  [ "$action" = call ] || db_query "UPDATE uni1_users SET dm=0,dmfree=0,trader=$offer_id,rate_m=3,rate_k=2,rate_d=1 WHERE player_id=$actor_id" >/dev/null
}

capture_state() {
  row="$(db_query "SELECT dm,dmfree,trader,rate_m,rate_k,rate_d FROM uni1_users WHERE player_id=$actor_id")"
  old_ifs="$IFS"; IFS="$(printf '\t')"
  # shellcheck disable=SC2086 # Split the tab-delimited SQL row into positional fields.
  set -- $row
  IFS="$old_ifs"
  dm_value="$1"; free_value="$2"; trader_value="$3"; rate_m="$4"; rate_k="$5"; rate_d="$6"
  resources="$(db_query "SELECT JSON_OBJECT('metal',FLOOR(\`700\`),'crystal',FLOOR(\`701\`),'deuterium',FLOOR(\`702\`)) FROM uni1_planets WHERE planet_id=$planet_id")"
  rates="$(jq -nc --argjson metal "$rate_m" --argjson crystal "$rate_k" --argjson deuterium "$rate_d" '{metal:$metal,crystal:$crystal,deuterium:$deuterium}')"
  if [ "$random_rates" = true ]; then
    rates="$(printf '%s' "$rates" | jq -c --argjson offer "$offer_id" '
      if $offer == 1 then {valid:(.metal == 3 and .crystal >= 1.4 and .crystal <= 2 and .deuterium >= 0.7 and .deuterium <= 1)}
      elif $offer == 2 then {valid:(.metal >= 2.1 and .metal <= 3 and .crystal == 2 and .deuterium >= 0.7 and .deuterium <= 1)}
      else {valid:(.metal >= 2.1 and .metal <= 3 and .crystal >= 1.4 and .crystal <= 2 and .deuterium == 1)} end')"
  fi
  state="$(jq -ncS --argjson dm "$dm_value" --argjson free "$free_value" --argjson trader "$trader_value" \
    --argjson rates "$rates" --argjson resources "$resources" '{dm:$dm,free:$free,trader:$trader,rates:$rates,resources:$resources}')"
}

run_legacy_action() {
  url="$LEGACY_BASE_URL/game/index.php?page=trader&session=$session&cp=$planet_id"
  if [ "$action" = call ]; then
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/legacy-$case_name.body" \
      --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --data-urlencode 'call_trader=1' \
      --data-urlencode "offer_id=$offer_id" --write-out '%{http_code}' "$url")"
  else
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/legacy-$case_name.body" \
      --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --data-urlencode 'trade=1' \
      --data-urlencode "1_value=$metal" --data-urlencode "2_value=$crystal" --data-urlencode "3_value=$deuterium" \
      --write-out '%{http_code}' "$url")"
  fi
  issue=''
  grep -q 'Not enough dark matter' "$TMP_DIR/legacy-$case_name.body" && issue=not_enough_dark_matter || true
  grep -q 'Not enough material to trade' "$TMP_DIR/legacy-$case_name.body" && issue=not_enough_resource || true
  grep -q 'Not enough storage space' "$TMP_DIR/legacy-$case_name.body" && issue=not_enough_storage || true
}

run_go_action() {
  payload="$(jq -nc --arg action "$action" --argjson offer "$offer_id" --argjson metal "$metal" \
    --argjson crystal "$crystal" --argjson deuterium "$deuterium" \
    '{action:$action,offerId:$offer,values:{metal:$metal,crystal:$crystal,deuterium:$deuterium}}')"
  http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/go-$case_name.body" \
    --header 'Content-Type: application/json' --cookie "$cookie" --data "$payload" --write-out '%{http_code}' \
    "$GO_BASE_URL/api/game/merchant?session=$session&cp=$planet_id")"
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
cases='call-metal call-crystal call-deuterium call-replace call-insufficient call-invalid trade-metal trade-crystal trade-deuterium trade-resource-reject trade-storage-reject trade-zero'
for case_name in $cases; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.state')" = "$(printf '%s' "$go" | jq -cS '.state')" ] || pass=false
  [ "$(printf '%s' "$legacy" | jq -r '.issue')" = "$(printf '%s' "$go" | jq -r '.issue')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http == 200' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http == 200' >/dev/null || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "merchant-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
  case_count=$((case_count + 1))
done

jq -s --argjson pass "$all_pass" --argjson cases "$case_count" \
  '{pass:$pass,normalization:"random call rates are reduced to legacy offer-specific validity ranges; DM, offer lifecycle and traded resources remain exact",metrics:{cases:$cases},cases:.}' \
  "$results" > "$REPORT"
[ "$all_pass" = true ] || { jq '.cases[] | select(.pass == false)' "$REPORT" >&2; exit 1; }
printf 'Go/PHP merchant differential E2E: PASS (%s cases)\n' "$case_count"
