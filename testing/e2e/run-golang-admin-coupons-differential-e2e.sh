#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_ADMIN_COUPONS_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-admin-coupons-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/admin-coupons-differential.XXXXXX")"

admin_id="$(jq -r '.player_id // 0' "$FIXTURE")"
operator_id="$(jq -r '.operator.player_id // 0' "$FIXTURE")"
[ "$admin_id" -gt 0 ] && [ "$operator_id" -gt 0 ]

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot -e "$1"' sh "$1"
}

suffix="$$"
coupon_backup="e2e_admcoupon_coupons_$suffix"
queue_backup="e2e_admcoupon_queue_$suffix"
coupon_auto="$(db_query "SELECT COALESCE(AUTO_INCREMENT,1) FROM information_schema.TABLES WHERE TABLE_SCHEMA='master' AND TABLE_NAME='coupons'")"
queue_auto="$(db_query "SELECT COALESCE(AUTO_INCREMENT,1) FROM information_schema.TABLES WHERE TABLE_SCHEMA='uni' AND TABLE_NAME='uni1_queue'")"
db_query "DROP TABLE IF EXISTS master.$coupon_backup,uni.$queue_backup;
CREATE TABLE master.$coupon_backup AS SELECT * FROM master.coupons;
CREATE TABLE uni.$queue_backup AS SELECT * FROM uni.uni1_queue" >/dev/null

restore_case() {
  db_query "DELETE FROM master.coupons; INSERT INTO master.coupons SELECT * FROM master.$coupon_backup;
DELETE FROM uni.uni1_queue; INSERT INTO uni.uni1_queue SELECT * FROM uni.$queue_backup" >/dev/null
}

cleanup() {
  restore_case >/dev/null 2>&1 || true
  db_query "ALTER TABLE master.coupons AUTO_INCREMENT=$coupon_auto; ALTER TABLE uni.uni1_queue AUTO_INCREMENT=$queue_auto;
DROP TABLE IF EXISTS master.$coupon_backup,uni.$queue_backup" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

admin_login="$(db_query "SELECT name FROM uni.uni1_users WHERE player_id=$admin_id")"
admin_planet="$(db_query "SELECT hplanetid FROM uni.uni1_users WHERE player_id=$admin_id")"
operator_login="$(db_query "SELECT name FROM uni.uni1_users WHERE player_id=$operator_id")"
operator_planet="$(db_query "SELECT hplanetid FROM uni.uni1_users WHERE player_id=$operator_id")"

seed_base_coupons() {
  db_query "DELETE FROM master.coupons; ALTER TABLE master.coupons AUTO_INCREMENT=1000;
INSERT INTO master.coupons(id,code,amount,used,user_uni,user_id,user_name) VALUES
(101,'COUPON-DIFF-BASE-A',111,0,0,0,''),(102,'COUPON-DIFF-BASE-B',222,1,1,$admin_id,'$admin_login')" >/dev/null
}

seed_paged_coupons() {
  db_query "DELETE FROM master.coupons; ALTER TABLE master.coupons AUTO_INCREMENT=1000;
INSERT INTO master.coupons(id,code,amount,used,user_uni,user_id,user_name)
SELECT n,CONCAT('COUPON-DIFF-',LPAD(n,3,'0')),n*10,0,0,0,''
FROM (SELECT ROW_NUMBER() OVER () n FROM information_schema.columns LIMIT 20) x" >/dev/null
}

configure_case() {
  case_name="$1"
  restore_case
  db_query "UPDATE uni.uni1_users SET admin=2 WHERE player_id=$admin_id; UPDATE uni.uni1_users SET admin=1 WHERE player_id=$operator_id;
DELETE FROM uni.uni1_queue WHERE type='Coupon' OR (owner_id=99999 AND sub_id IN (9876501,9876502,9876503))" >/dev/null
  seed_base_coupons
  actor_login="$admin_login"; actor_planet="$admin_planet"; action=view; amount=0; item_id=0; coupon_from=0
  day_month=14.07; hour_minute=10:00; inactive_days=7; ingame_days=365; periodic_days=14
  case "$case_name" in
    coupon-list-page) action=view; coupon_from=15; seed_paged_coupons ;;
    coupon-add-positive) action=add_one; amount=987654321 ;;
    coupon-add-zero) action=add_one; amount=0 ;;
    coupon-add-negative) action=add_one; amount=-1 ;;
    coupon-remove-existing) action=remove_one; item_id=101 ;;
    coupon-remove-missing) action=remove_one; item_id=99999999 ;;
    coupon-add-date) action=add_date; amount=9876501 ;;
    coupon-add-date-signed)
      action=add_date; amount=-17; inactive_days=-2; ingame_days=-3; periodic_days=-4
      ;;
    coupon-remove-date)
      action=remove_date
      db_query "INSERT INTO uni.uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES(99999,'Coupon',9876502,7,14,2000000000,4000000000,520)" >/dev/null
      item_id="$(db_query "SELECT task_id FROM uni.uni1_queue WHERE type='Coupon' AND sub_id=9876502 ORDER BY task_id DESC LIMIT 1")"
      ;;
    coupon-remove-date-any-type)
      action=remove_date
      db_query "INSERT INTO uni.uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES(99999,'Build',9876503,0,0,2000000000,4000000000,0)" >/dev/null
      item_id="$(db_query "SELECT task_id FROM uni.uni1_queue WHERE type='Build' AND sub_id=9876503 ORDER BY task_id DESC LIMIT 1")"
      ;;
    coupon-operator-add-denied) action=add_one; amount=987654321; actor_login="$operator_login"; actor_planet="$operator_planet" ;;
    coupon-operator-remove-denied) action=remove_one; item_id=101; actor_login="$operator_login"; actor_planet="$operator_planet" ;;
  esac
}

login_legacy() {
  prefix="$1"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" --output "$TMP_DIR/$prefix-login.body" \
    --cookie-jar "$TMP_DIR/$prefix.cookies" --request POST --data-urlencode "login=$actor_login" --data-urlencode "pass=$PASSWORD" \
    --write-out '%{http_code}' "$LEGACY_BASE_URL/game/reg/login2.php")"
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/$prefix-login.headers")"
  session="$(printf '%s' "$location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 302 ] && [ -n "$session" ]
}

login_go() {
  prefix="$1"
  payload="$(jq -nc --arg login "$actor_login" --arg pass "$PASSWORD" '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" --output "$TMP_DIR/$prefix-login.body" \
    --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/$prefix-login.headers" | cut -d';' -f1 | tr -d '\r')"
  redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/$prefix-login.body")"
  session="$(printf '%s' "$redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 200 ] && [ -n "$cookie" ] && [ -n "$session" ]
}

coupon_state() {
  db_query "SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT('id',id,'code',code,'amount',amount,'used',used,'userUni',user_uni,'userId',user_id,'userName',user_name)),JSON_ARRAY())
FROM (SELECT * FROM master.coupons ORDER BY id) x" | jq -cS 'map(.id=0 | if (.code|startswith("COUPON-DIFF-")) then . else .code="GENERATED" end)'
}

queue_state() {
  db_query "SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT('id',task_id,'type',type,'amount',sub_id,'criteria',obj_id,'periodic',level,'start',start,'end',end,'priority',prio)),JSON_ARRAY())
FROM (SELECT * FROM uni.uni1_queue WHERE type='Coupon' OR (owner_id=99999 AND sub_id IN (9876501,9876502,9876503)) ORDER BY task_id) x" |
    jq -cS 'map(.id=0 | .endDelta=(.end-.start) | .start=0 | del(.end))'
}

legacy_request() {
  case_name="$1"; body="$TMP_DIR/legacy-$case_name.body"
  url="$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$actor_planet&mode=Coupons"
  case "$action" in
    view)
      http="$(curl --silent --show-error --max-time 20 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" \
        --write-out '%{http_code}' "$url&from=$coupon_from")"
      output="$(grep -o 'COUPON-DIFF-[0-9][0-9][0-9]' "$body" | jq -Rsc 'split("\n")|map(select(length>0))')"
      ;;
    add_one)
      http="$(curl --silent --show-error --max-time 20 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" \
        --request POST --data-urlencode "dm=$amount" --write-out '%{http_code}' "$url&action=add_one")"
      output='[]'
      ;;
    remove_one)
      http="$(curl --silent --show-error --max-time 20 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" \
        --write-out '%{http_code}' "$url&action=remove_one&item_id=$item_id")"
      output='[]'
      ;;
    add_date)
      http="$(curl --silent --show-error --max-time 20 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST \
        --data-urlencode "ddmm=$day_month" --data-urlencode "hhmm=$hour_minute" --data-urlencode "darkmatter=$amount" \
        --data-urlencode "inactive_days=$inactive_days" --data-urlencode "ingame_days=$ingame_days" --data-urlencode "periodic=$periodic_days" \
        --write-out '%{http_code}' "$url&action=add_date")"
      output='[]'
      ;;
    remove_date)
      http="$(curl --silent --show-error --max-time 20 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" \
        --write-out '%{http_code}' "$url&action=remove_date&item_id=$item_id")"
      output='[]'
      ;;
  esac
  issue=""
}

go_request() {
  case_name="$1"; body="$TMP_DIR/go-$case_name.body"
  url="$GO_BASE_URL/api/game/admin?session=$session&cp=$actor_planet&mode=Coupons&from=$coupon_from"
  if [ "$action" = view ]; then
    http="$(curl --silent --show-error --max-time 20 --output "$body" --cookie "$cookie" --write-out '%{http_code}' "$url")"
    output="$(jq -c '[.admin.couponRows[]?.code | select(test("^COUPON-DIFF-[0-9]{3}$"))]' "$body")"
    issue=""
    return
  fi
  payload="$(jq -nc --arg action "$action" --argjson amount "$amount" --argjson item "$item_id" \
    --arg day "$day_month" --arg hour "$hour_minute" --argjson inactive "$inactive_days" --argjson ingame "$ingame_days" --argjson periodic "$periodic_days" \
    '{action:$action,amount:$amount,itemId:$item,dayMonth:$day,hourMinute:$hour,inactiveDays:$inactive,ingameDays:$ingame,periodicDays:$periodic}')"
  http="$(curl --silent --show-error --max-time 20 --output "$body" --cookie "$cookie" --header 'Content-Type: application/json' \
    --data "$payload" --write-out '%{http_code}' "$url")"
  output='[]'
  issue="$(jq -r '.actionIssue.code // empty' "$body" 2>/dev/null || true)"
}

run_side() {
  side="$1"; case_name="$2"; configure_case "$case_name"
  if [ "$side" = legacy ]; then login_legacy "$side-$case_name"; legacy_request "$case_name"; else login_go "$side-$case_name"; go_request "$case_name"; fi
  coupons="$(coupon_state)"; queue="$(queue_state)"
  normalized="$(jq -ncS --argjson http "$http" --arg issue "$issue" --argjson output "$output" --argjson coupons "$coupons" --argjson queue "$queue" \
    '{http:$http,issue:$issue,output:$output,state:{coupons:$coupons,queue:$queue}}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true
cases="coupon-list-page coupon-add-positive coupon-add-zero coupon-add-negative coupon-remove-existing coupon-remove-missing coupon-add-date coupon-add-date-signed coupon-remove-date coupon-remove-date-any-type coupon-operator-add-denied coupon-operator-remove-denied"
for case_name in $cases; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.output,.state')" = "$(printf '%s' "$go" | jq -cS '.output,.state')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200 or .http==302' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200' >/dev/null || pass=false
  case "$case_name" in
    coupon-operator-*) [ "$(printf '%s' "$go" | jq -r '.issue')" = access_denied ] || pass=false ;;
    coupon-list-page) ;;
    *) [ "$(printf '%s' "$go" | jq -r '.issue')" = action_saved ] || pass=false ;;
  esac
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "admin-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" \
  '{pass:$pass,normalization:"generated coupon code/id and queue id/start are normalized; coupon values, ordering, packed criteria and end-start remain exact",cases:.}' \
  "$results" > "$REPORT"
[ "$all_pass" = true ]
case_count="$(printf '%s\n' $cases | wc -l | tr -d ' ')"
printf 'Go/PHP admin coupons differential E2E: PASS (%s cases)\n' "$case_count"
