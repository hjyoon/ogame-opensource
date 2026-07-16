#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
MAILHOG_BASE_URL="${OGAME_MAILHOG_BASE_URL:-http://127.0.0.1:${OGAME_MAILHOG_PORT:-8026}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_ADMIN_COUPON_CRON_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-admin-coupon-cron-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/admin-coupon-cron-differential.XXXXXX")"

admin_id="$(jq -r '.player_id // 0' "$FIXTURE")"
eligible_id="$(jq -r '.message_send.sender.player_id // 0' "$FIXTURE")"
other_id="$(jq -r '.message_send.recipient.player_id // 0' "$FIXTURE")"
if [ "$admin_id" -le 0 ] || [ "$eligible_id" -le 0 ] || [ "$other_id" -le 0 ]; then
  printf 'Invalid admin coupon CRON fixture IDs\n' >&2
  exit 1
fi

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

master_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot master -e "$1"' sh "$1"
}

marker=$((930000000 + ($$ % 1000000)))
users_backup="uni1_e2e_coupon_cron_users_$$"
queue_backup="uni1_e2e_coupon_cron_queue_$$"
role_backup="uni1_e2e_coupon_cron_role_$$"
coupon_backup="e2e_coupon_cron_coupons_$$"
legacy_log_backup="/tmp/ogame-e2e-coupon-mailto-$$.log"
legacy_log_missing="/tmp/ogame-e2e-coupon-mailto-$$.missing"

admin_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$admin_id")"
admin_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$admin_id")"
if [ "$admin_planet" -le 0 ] || [ -z "$admin_login" ]; then
  printf 'Invalid admin coupon CRON login fixture\n' >&2
  exit 1
fi

db_query "DROP TABLE IF EXISTS $users_backup,$queue_backup,$role_backup;
CREATE TABLE $users_backup AS SELECT * FROM uni1_users WHERE player_id IN ($eligible_id,$other_id);
CREATE TABLE $queue_backup AS SELECT * FROM uni1_queue WHERE type='Coupon';
CREATE TABLE $role_backup AS SELECT player_id,admin FROM uni1_users WHERE player_id=$admin_id" >/dev/null
master_query "DROP TABLE IF EXISTS $coupon_backup; CREATE TABLE $coupon_backup AS SELECT * FROM coupons" >/dev/null
docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T server sh -c "if [ -f /var/www/html/game/temp/mailto.log ]; then cp /var/www/html/game/temp/mailto.log '$legacy_log_backup'; else : > '$legacy_log_missing'; fi"

restore_legacy_log() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T server sh -c "if [ -f '$legacy_log_missing' ]; then rm -f /var/www/html/game/temp/mailto.log; else cp '$legacy_log_backup' /var/www/html/game/temp/mailto.log; fi" >/dev/null
}

restore_case() {
  db_query "DELETE FROM uni1_queue WHERE type='Coupon'; INSERT INTO uni1_queue SELECT * FROM $queue_backup;
DELETE FROM uni1_users WHERE player_id IN ($eligible_id,$other_id); INSERT INTO uni1_users SELECT * FROM $users_backup;
UPDATE uni1_users SET admin=2 WHERE player_id=$admin_id" >/dev/null
  master_query "DELETE FROM coupons; INSERT INTO coupons SELECT * FROM $coupon_backup" >/dev/null
  restore_legacy_log
  curl --fail --silent --show-error --request DELETE "$MAILHOG_BASE_URL/api/v1/messages" >/dev/null
}

cleanup() {
  restore_case >/dev/null 2>&1 || true
  db_query "UPDATE uni1_users u JOIN $role_backup b ON b.player_id=u.player_id SET u.admin=b.admin;
DROP TABLE IF EXISTS $users_backup,$queue_backup,$role_backup" >/dev/null 2>&1 || true
  master_query "DROP TABLE IF EXISTS $coupon_backup" >/dev/null 2>&1 || true
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T server sh -c "rm -f '$legacy_log_backup' '$legacy_log_missing'" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

login_legacy() {
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/legacy-login.headers" --output "$TMP_DIR/legacy-login.body" \
    --cookie-jar "$TMP_DIR/legacy.cookies" --request POST --data-urlencode "login=$admin_login" --data-urlencode "pass=$PASSWORD" \
    --write-out '%{http_code}' "$LEGACY_BASE_URL/game/reg/login2.php")"
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/legacy-login.headers")"
  session="$(printf '%s' "$location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 302 ] && [ -n "$session" ]
}

login_go() {
  payload="$(jq -nc --arg login "$admin_login" --arg pass "$PASSWORD" '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/go-login.headers" --output "$TMP_DIR/go-login.body" \
    --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/go-login.headers" | cut -d';' -f1 | tr -d '\r')"
  redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/go-login.body")"
  session="$(printf '%s' "$redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 200 ] && [ -n "$cookie" ] && [ -n "$session" ]
}

configure_case() {
  case_name="$1"
  restore_case
  master_query "DELETE FROM coupons" >/dev/null
  due="$(db_query 'SELECT UNIX_TIMESTAMP()-1')"
  case_tag=o; [ "$case_name" = periodic-frozen ] && case_tag=p; [ "$case_name" = registration-boundary ] && case_tag=b
  email="c${marker}${case_tag}@e.test"
  ingame_days=10000
  language=en; level=0; freeze=0; regdate=$((due-(ingame_days+1)*86400)); lastclick=$((due-3*86400))
  case "$case_name" in
    periodic-frozen)
      language=fr; level=2; freeze=1
      ;;
    registration-boundary)
      regdate=$((due-ingame_days*86400))
      ;;
  esac
  db_query "UPDATE uni1_users SET oname='Coupon Recipient',pemail='$email',lang='$language',regdate=$regdate,lastclick=$lastclick WHERE player_id=$eligible_id;
UPDATE uni1_users SET pemail='other-$email',regdate=$((due-(ingame_days+1)*86400)),lastclick=$((due-3*86400-1)) WHERE player_id=$other_id;
INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio,freeze,frozen) VALUES(99999,'Coupon',4321,$(((3<<16)|ingame_days)),$level,$((due-10)),$due,520,$freeze,$((due-5)))" >/dev/null
}

capture_legacy_mail() {
  raw="$(docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T server sh -c 'cat /var/www/html/game/temp/mailto.log 2>/dev/null || true' | tr -d '\r')"
  block="$(printf '%s\n' "$raw" | awk -v target="To: $email" '$0==target {found=1; next} found && /^To: / {exit} found {print}')"
  subject="$(printf '%s\n' "$block" | awk '/^Subj: / {sub(/^Subj: /,""); print; exit}')"
  body="$(printf '%s\n' "$block" | awk 'body {print} /^Subj: / {body=1}' | sed -e '/^[[:space:]]*$/d' -E -e 's/[0-9A-Z]{4}(-[0-9A-Z]{4}){4}/<COUPON>/g')"
  count=0; [ -n "$subject$body" ] && count=1
  jq -ncS --argjson count "$count" --arg recipient "$email" --arg subject "$subject" --arg body "$body" \
    '{count:$count,messages:(if $count==1 then [{recipient:$recipient,subject:$subject,body:$body}] else [] end)}'
}

capture_go_mail() {
  attempt=0
  mail_json="$(curl --fail --silent --show-error "$MAILHOG_BASE_URL/api/v2/messages")"
  while [ "$expects_mail" = 1 ] && [ "$(printf '%s' "$mail_json" | jq -r '.total // 0')" -eq 0 ] && [ "$attempt" -lt 20 ]; do
    sleep 0.1
    mail_json="$(curl --fail --silent --show-error "$MAILHOG_BASE_URL/api/v2/messages")"
    attempt=$((attempt+1))
  done
  printf '%s' "$mail_json" | jq -cS --arg recipient "$email" '
    def decoded_subject:
      if test("^=\\?UTF-8\\?B\\?.*\\?=$"; "i") then capture("^=\\?UTF-8\\?B\\?(?<encoded>.*)\\?=$"; "i").encoded | @base64d else . end;
    {messages:[.items[] | select(any(.To[]; (.Mailbox+"@"+.Domain)==$recipient)) | {
      recipient:$recipient,subject:((.Content.Headers.Subject[0]//"")|decoded_subject),body:((.Content.Body//"")|gsub("\\r";"")|gsub("[0-9A-Z]{4}(-[0-9A-Z]{4}){4}";"<COUPON>")|gsub("[[:space:]]+$";""))
    }]} | .count=(.messages|length) | {count,messages}'
}

capture_state() {
  coupons="$(master_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('amount',amount,'used',used,'format',IF(code REGEXP '^[0-9A-Z]{4}(-[0-9A-Z]{4}){4}$',1,0)))) FROM coupons")"
  queue="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('level',level,'freeze',freeze,'extension',end-$due))) FROM uni1_queue WHERE type='Coupon'")"
  jq -ncS --argjson coupons "$coupons" --argjson queue "$queue" --argjson mail "$mail_state" '{coupons:$coupons,queue:$queue,mail:$mail}'
}

run_side() {
  side="$1"; case_name="$2"
  configure_case "$case_name"
  expects_mail=1; [ "$case_name" = registration-boundary ] && expects_mail=0
  if [ "$side" = legacy ]; then
    login_legacy
    http="$(curl --silent --show-error --max-time 60 --output "$TMP_DIR/$side-$case_name.body" --cookie "$TMP_DIR/legacy.cookies" \
      --request POST --data 'order_cron=1' --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$admin_planet&mode=Queue")"
    issue=""; mail_state="$(capture_legacy_mail)"
  else
    login_go
    http="$(curl --silent --show-error --max-time 60 --output "$TMP_DIR/$side-$case_name.body" --header 'Content-Type: application/json' \
      --cookie "$cookie" --data '{"action":"queue_cron"}' --write-out '%{http_code}' "$GO_BASE_URL/api/game/admin?session=$session&cp=$admin_planet&mode=Queue")"
    issue="$(jq -r '.actionIssue.code // empty' "$TMP_DIR/$side-$case_name.body")"; mail_state="$(capture_go_mail)"
  fi
  state="$(capture_state)"
  normalized="$(jq -ncS --argjson http "$http" --arg issue "$issue" --argjson state "$state" '{http:$http,issue:$issue,state:$state}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true
for case_name in one-off periodic-frozen registration-boundary; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.state')" = "$(printf '%s' "$go" | jq -cS '.state')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200 or .http==302' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200 and .issue=="action_saved"' >/dev/null || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "admin-coupon-cron-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" '{pass:$pass,normalization:"random coupon codes are reduced to their legacy format; recipients, localized subject/body, amount, queue removal/extension and frozen-task behavior remain exact",cases:.}' "$results" > "$REPORT"
[ "$all_pass" = true ]
printf 'Go/PHP admin coupon CRON differential E2E: PASS (3 cases)\n'
