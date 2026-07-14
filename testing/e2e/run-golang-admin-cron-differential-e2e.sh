#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_ADMIN_CRON_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-admin-cron-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/admin-cron-differential.XXXXXX")"

admin_id="$(jq -r '.player_id // 0' "$FIXTURE")"
operator_id="$(jq -r '.operator.player_id // 0' "$FIXTURE")"
target_id="$(jq -r '.admin_operations.target_player_id // 0' "$FIXTURE")"
[ "$admin_id" -gt 0 ] && [ "$operator_id" -gt 0 ] && [ "$target_id" -gt 0 ]

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

marker=$((910000000 + ($$ % 10000000)))
queue_backup="uni1_e2e_cron_queue_$$"
user_backup="uni1_e2e_cron_user_$$"
uni_backup="uni1_e2e_cron_uni_$$"
role_backup="uni1_e2e_cron_role_$$"

db_query "DROP TABLE IF EXISTS $queue_backup,$user_backup,$uni_backup,$role_backup;
CREATE TABLE $queue_backup AS SELECT * FROM uni1_queue WHERE owner_id=$target_id OR obj_id BETWEEN $marker AND $marker+100;
CREATE TABLE $user_backup AS SELECT player_id,name_changed,name_until,email,pemail,banned,banned_until,noattack,noattack_until FROM uni1_users WHERE player_id=$target_id;
CREATE TABLE $uni_backup AS SELECT num,freeze FROM uni1_uni;
CREATE TABLE $role_backup AS SELECT player_id,admin FROM uni1_users WHERE player_id IN ($admin_id,$operator_id)" >/dev/null

restore_case() {
  db_query "DELETE FROM uni1_queue WHERE owner_id=$target_id OR obj_id BETWEEN $marker AND $marker+100;
INSERT INTO uni1_queue SELECT * FROM $queue_backup;
UPDATE uni1_users u JOIN $user_backup b ON b.player_id=u.player_id SET u.name_changed=b.name_changed,u.name_until=b.name_until,u.email=b.email,u.pemail=b.pemail,u.banned=b.banned,u.banned_until=b.banned_until,u.noattack=b.noattack,u.noattack_until=b.noattack_until;
UPDATE uni1_uni u JOIN $uni_backup b ON b.num=u.num SET u.freeze=b.freeze;
DELETE FROM uni1_debug WHERE text LIKE 'queue: %E2ECronUnknown%';" >/dev/null
}

cleanup() {
  restore_case >/dev/null 2>&1 || true
  db_query "UPDATE uni1_users u JOIN $role_backup b ON b.player_id=u.player_id SET u.admin=b.admin;
DROP TABLE IF EXISTS $queue_backup,$user_backup,$uni_backup,$role_backup" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

admin_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$admin_id")"
admin_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$admin_id")"
operator_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$operator_id")"
operator_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$operator_id")"

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

configure_case() {
  case_name="$1"
  db_query "UPDATE uni1_uni SET freeze=0" >/dev/null
  case "$case_name" in
    debug)
      db_query "INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES($target_id,'Debug',0,$marker,0,UNIX_TIMESTAMP()-10,UNIX_TIMESTAMP()-1,9999)" >/dev/null
      ;;
    unknown)
      db_query "INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES($target_id,'E2ECronUnknown',0,$marker+1,0,UNIX_TIMESTAMP()-10,UNIX_TIMESTAMP()-1,9999)" >/dev/null
      ;;
    user-flags)
      db_query "UPDATE uni1_users SET name_changed=1,email='e2e-cron-new@example.local',pemail='e2e-cron-old@example.local',banned=1,banned_until=UNIX_TIMESTAMP()-1,noattack=1,noattack_until=UNIX_TIMESTAMP()-1 WHERE player_id=$target_id;
INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES
($target_id,'AllowName',0,$marker+2,0,UNIX_TIMESTAMP()-10,UNIX_TIMESTAMP()-1,0),
($target_id,'ChangeEmail',0,$marker+3,0,UNIX_TIMESTAMP()-10,UNIX_TIMESTAMP()-1,0),
($target_id,'UnbanPlayer',0,$marker+4,0,UNIX_TIMESTAMP()-10,UNIX_TIMESTAMP()-1,0),
($target_id,'AllowAttacks',0,$marker+5,0,UNIX_TIMESTAMP()-10,UNIX_TIMESTAMP()-1,0)" >/dev/null
      ;;
    task-freeze)
      db_query "INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio,freeze,frozen) VALUES($target_id,'Debug',0,$marker+6,0,UNIX_TIMESTAMP()-10,UNIX_TIMESTAMP()-1,9999,1,UNIX_TIMESTAMP()-5)" >/dev/null
      ;;
    universe-freeze)
      db_query "UPDATE uni1_uni SET freeze=1; INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES($target_id,'Debug',0,$marker+7,0,UNIX_TIMESTAMP()-10,UNIX_TIMESTAMP()-1,9999)" >/dev/null
      ;;
    operator-denied)
      db_query "INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES($target_id,'Debug',0,$marker+8,0,UNIX_TIMESTAMP()-10,UNIX_TIMESTAMP()-1,9999)" >/dev/null
      ;;
    batch-limit)
      values=""
      index=0
      while [ "$index" -lt 17 ]; do
        [ -z "$values" ] || values="$values,"
        values="$values($target_id,'Debug',0,$((marker+20+index)),0,UNIX_TIMESTAMP()-10,UNIX_TIMESTAMP()-1,9999)"
        index=$((index+1))
      done
      db_query "INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES $values" >/dev/null
      ;;
  esac
}

capture_state() {
  queues="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('type',type,'obj',obj_id,'freeze',freeze))) FROM (SELECT type,obj_id,freeze FROM uni1_queue WHERE obj_id BETWEEN $marker AND $marker+100 ORDER BY obj_id) q")"
  user="$(db_query "SELECT JSON_OBJECT('nameChanged',name_changed,'email',email,'pemail',pemail,'banned',banned,'bannedUntil',banned_until,'noattack',noattack,'noattackUntil',noattack_until) FROM uni1_users WHERE player_id=$target_id")"
  debug="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(text)) FROM (SELECT text FROM uni1_debug WHERE text LIKE 'queue: %E2ECronUnknown%' ORDER BY error_id) d")"
  frozen="$(db_query 'SELECT freeze FROM uni1_uni LIMIT 1')"
  jq -ncS --argjson queues "$queues" --argjson user "$user" --argjson debug "$debug" --argjson universeFreeze "$frozen" '{queues:$queues,user:$user,debug:$debug,universeFreeze:$universeFreeze}'
}

run_side() {
  side="$1"; case_name="$2"
  restore_case
  db_query "UPDATE uni1_users SET admin=2 WHERE player_id=$admin_id; UPDATE uni1_users SET admin=1 WHERE player_id=$operator_id" >/dev/null
  actor_login=$admin_login; actor_planet=$admin_planet
  [ "$case_name" = operator-denied ] && { actor_login=$operator_login; actor_planet=$operator_planet; }
  if [ "$side" = legacy ]; then login_legacy "$side-$case_name"; else login_go "$side-$case_name"; fi
  configure_case "$case_name"
  if [ "$side" = legacy ]; then
    http="$(curl --silent --show-error --max-time 30 --output "$TMP_DIR/$side-$case_name.body" --cookie "$TMP_DIR/$side-$case_name.cookies" \
      --request POST --data 'order_cron=1' --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$actor_planet&mode=Queue")"
    issue=""
  else
    http="$(curl --silent --show-error --max-time 30 --output "$TMP_DIR/$side-$case_name.body" --header 'Content-Type: application/json' \
      --cookie "$cookie" --data '{"action":"queue_cron"}' --write-out '%{http_code}' "$GO_BASE_URL/api/game/admin?session=$session&cp=$actor_planet&mode=Queue")"
    issue="$(jq -r '.actionIssue.code // empty' "$TMP_DIR/$side-$case_name.body" 2>/dev/null || true)"
  fi
  state="$(capture_state)"
  normalized="$(jq -ncS --argjson http "$http" --arg issue "$issue" --argjson state "$state" '{http:$http,issue:$issue,state:$state}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true
cases="debug unknown user-flags task-freeze universe-freeze operator-denied batch-limit"
for case_name in $cases; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.state')" = "$(printf '%s' "$go" | jq -cS '.state')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200 or .http==302' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200' >/dev/null || pass=false
  if [ "$case_name" = operator-denied ]; then
    [ "$(printf '%s' "$go" | jq -r '.issue')" = access_denied ] || pass=false
  else
    [ "$(printf '%s' "$go" | jq -r '.issue')" = action_saved ] || pass=false
  fi
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "admin-cron-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" '{pass:$pass,normalization:"generated task IDs and login session tokens are excluded; queue order, flags, debug text and freeze state remain exact",cases:.}' "$results" > "$REPORT"
[ "$all_pass" = true ]
printf 'Go/PHP admin CRON differential E2E: PASS (%s cases)\n' "$(printf '%s\n' $cases | wc -l | tr -d ' ')"
