#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_ADMIN_QUEUE_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-admin-queue-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/admin-queue-differential.XXXXXX")"

admin_id="$(jq -r '.player_id // 0' "$FIXTURE")"
operator_id="$(jq -r '.operator.player_id // 0' "$FIXTURE")"
target_id="$(jq -r '.admin_operations.target_player_id // 0' "$FIXTURE")"
[ "$admin_id" -gt 0 ] && [ "$operator_id" -gt 0 ] && [ "$target_id" -gt 0 ]

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

queue_backup="uni1_e2e_admq_queue_$$"
fleet_backup="uni1_e2e_admq_fleet_$$"
fleetlog_owner_backup="uni1_e2e_admq_flog_owner_$$"
fleetlog_old_backup="uni1_e2e_admq_flog_old_$$"
userlog_owner_backup="uni1_e2e_admq_ulog_owner_$$"
userlog_old_backup="uni1_e2e_admq_ulog_old_$$"
uni_backup="uni1_e2e_admq_uni_$$"

db_query "DROP TABLE IF EXISTS $queue_backup,$fleet_backup,$fleetlog_owner_backup,$fleetlog_old_backup,$userlog_owner_backup,$userlog_old_backup,$uni_backup;
CREATE TABLE $queue_backup AS SELECT * FROM uni1_queue WHERE owner_id=$target_id;
CREATE TABLE $fleet_backup AS SELECT * FROM uni1_fleet WHERE owner_id=$target_id;
CREATE TABLE $fleetlog_owner_backup AS SELECT * FROM uni1_fleetlogs WHERE owner_id=$target_id;
CREATE TABLE $fleetlog_old_backup AS SELECT * FROM uni1_fleetlogs WHERE start < UNIX_TIMESTAMP()-4*7*24*60*60;
CREATE TABLE $userlog_owner_backup AS SELECT * FROM uni1_userlogs WHERE owner_id=$target_id;
CREATE TABLE $userlog_old_backup AS SELECT * FROM uni1_userlogs WHERE date < UNIX_TIMESTAMP()-2*7*24*60*60;
CREATE TABLE $uni_backup AS SELECT num,freeze FROM uni1_uni" >/dev/null

restore_case() {
  db_query "DELETE FROM uni1_queue WHERE owner_id=$target_id; INSERT INTO uni1_queue SELECT * FROM $queue_backup;
DELETE FROM uni1_fleet WHERE owner_id=$target_id; INSERT INTO uni1_fleet SELECT * FROM $fleet_backup;
DELETE FROM uni1_fleetlogs WHERE owner_id=$target_id; INSERT INTO uni1_fleetlogs SELECT * FROM $fleetlog_owner_backup;
INSERT IGNORE INTO uni1_fleetlogs SELECT * FROM $fleetlog_old_backup;
DELETE FROM uni1_userlogs WHERE owner_id=$target_id; INSERT INTO uni1_userlogs SELECT * FROM $userlog_owner_backup;
INSERT IGNORE INTO uni1_userlogs SELECT * FROM $userlog_old_backup;
UPDATE uni1_uni u JOIN $uni_backup b ON b.num=u.num SET u.freeze=b.freeze" >/dev/null
}

cleanup() {
  restore_case >/dev/null 2>&1 || true
  db_query "DROP TABLE IF EXISTS $queue_backup,$fleet_backup,$fleetlog_owner_backup,$fleetlog_old_backup,$userlog_owner_backup,$userlog_old_backup,$uni_backup" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

admin_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$admin_id")"
admin_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$admin_id")"
operator_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$operator_id")"
operator_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$operator_id")"
target_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$target_id")"
union_marker=$((900000000 + ($$ % 10000000)))

configure_case() {
  case_name="$1"
  restore_case
  db_query "UPDATE uni1_uni SET freeze=0" >/dev/null
  actor_login=$admin_login; actor_planet=$admin_planet; mode=Queue
  action=queue_end; post_key=order_end; task_id=0; secondary_task_id=0
  case_epoch="$(db_query 'SELECT UNIX_TIMESTAMP()')"

  case "$case_name" in
    queue-operator-denied) actor_login=$operator_login; actor_planet=$operator_planet ;;
    queue-end) ;;
    queue-remove) action=queue_remove; post_key=order_remove ;;
    queue-freeze) action=queue_freeze; post_key=order_freeze ;;
    queue-unfreeze) action=queue_unfreeze; post_key=order_unfreeze ;;
    queue-missing) task_id=2147483000; return ;;
    fleet-operator-denied) actor_login=$operator_login; actor_planet=$operator_planet; mode=Fleetlogs; action=fleetlogs_end; post_key=order_end ;;
    fleet-2min-single) mode=Fleetlogs; action=fleetlogs_2min; post_key=order_2min ;;
    fleet-end-single) mode=Fleetlogs; action=fleetlogs_end; post_key=order_end ;;
    fleet-2min-union) mode=Fleetlogs; action=fleetlogs_2min; post_key=order_2min ;;
    fleet-end-union) mode=Fleetlogs; action=fleetlogs_end; post_key=order_end ;;
    fleet-return) mode=Fleetlogs; action=fleetlogs_return; post_key=order_return ;;
    fleet-missing) mode=Fleetlogs; action=fleetlogs_end; post_key=order_end; task_id=2147483000; return ;;
  esac

  case "$case_name" in
    queue-*)
      freeze=0; frozen=0
      if [ "$case_name" = queue-unfreeze ]; then freeze=1; frozen='UNIX_TIMESTAMP()-120'; fi
      task_id="$(db_query "INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio,freeze,frozen)
VALUES($target_id,'Debug',0,7777,3,UNIX_TIMESTAMP()-600,UNIX_TIMESTAMP()+1800,17,$freeze,$frozen); SELECT LAST_INSERT_ID()")"
      ;;
    fleet-2min-union|fleet-end-union)
      fleet_id="$(db_query "INSERT INTO uni1_fleet(owner_id,union_id,\`700\`,\`701\`,\`702\`,fuel,mission,start_planet,target_planet,flight_time,deploy_time,\`202\`,\`203\`)
VALUES($target_id,$union_marker,12345,2345,345,40,3,$target_planet,$admin_planet,1800,0,7654,4321); SELECT LAST_INSERT_ID()")"
      second_fleet_id="$(db_query "INSERT INTO uni1_fleet(owner_id,union_id,\`700\`,\`701\`,\`702\`,fuel,mission,start_planet,target_planet,flight_time,deploy_time,\`202\`,\`203\`)
VALUES($target_id,$union_marker,22345,3345,445,60,3,$target_planet,$admin_planet,1800,0,7655,4322); SELECT LAST_INSERT_ID()")"
      task_id="$(db_query "INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES($target_id,'Fleet',$fleet_id,0,0,UNIX_TIMESTAMP()-600,UNIX_TIMESTAMP()+1800,503); SELECT LAST_INSERT_ID()")"
      secondary_task_id="$(db_query "INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES($target_id,'Fleet',$second_fleet_id,0,0,UNIX_TIMESTAMP()-600,UNIX_TIMESTAMP()+1700,503); SELECT LAST_INSERT_ID()")"
      ;;
    fleet-*)
      fleet_id="$(db_query "INSERT INTO uni1_fleet(owner_id,union_id,\`700\`,\`701\`,\`702\`,fuel,mission,start_planet,target_planet,flight_time,deploy_time,\`202\`,\`203\`)
VALUES($target_id,0,12345,2345,345,40,3,$target_planet,$admin_planet,1800,0,7654,4321); SELECT LAST_INSERT_ID()")"
      task_id="$(db_query "INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES($target_id,'Fleet',$fleet_id,0,0,UNIX_TIMESTAMP()-600,UNIX_TIMESTAMP()+1800,503); SELECT LAST_INSERT_ID()")"
      ;;
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

capture_state() {
  queues="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT(
'role',role,'type',type,'endDelay',end_delay,'startAge',start_age,'freeze',freeze,'frozenAge',frozen_age,'prio',prio)))
FROM (SELECT CASE WHEN q.type='Debug' THEN 'queue' WHEN f.\`202\`=7655 THEN 'secondary' ELSE 'primary' END role,
q.type,CAST(q.end AS SIGNED)-CAST(UNIX_TIMESTAMP() AS SIGNED) end_delay,CAST(UNIX_TIMESTAMP() AS SIGNED)-CAST(q.start AS SIGNED) start_age,
q.freeze,IF(q.frozen=0,0,CAST(UNIX_TIMESTAMP() AS SIGNED)-CAST(q.frozen AS SIGNED)) frozen_age,q.prio
FROM uni1_queue q LEFT JOIN uni1_fleet f ON q.type='Fleet' AND f.fleet_id=q.sub_id
WHERE q.task_id IN ($task_id,$secondary_task_id) OR (f.owner_id=$target_id AND f.\`202\` IN (7654,7655)) ORDER BY role) x")"
  fleets="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('role',role,'owner',owner_id,'union',has_union,
'metal',metal,'crystal',crystal,'deuterium',deuterium,'fuel',fuel,'mission',mission,'startPlanet',start_planet,'targetPlanet',target_planet,
'flightTime',flight_time,'deployTime',deploy_time,'ship202',ship202,'ship203',ship203)))
FROM (SELECT IF(\`202\`=7655,'secondary','primary') role,owner_id,IF(union_id>0,1,0) has_union,\`700\` metal,\`701\` crystal,\`702\` deuterium,
fuel,mission,start_planet,target_planet,flight_time,deploy_time,\`202\` ship202,\`203\` ship203
FROM uni1_fleet WHERE owner_id=$target_id AND \`202\` IN (7654,7655) ORDER BY role) x")"
  fleetlogs="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('role',role,'owner',owner_id,'target',target_id,'union',has_union,
'metal',metal,'crystal',crystal,'deuterium',deuterium,'fuel',fuel,'mission',mission,'flightTime',flight_time,'deployTime',deploy_time,
'startAge',start_age,'endDelay',end_delay,'origin',origin,'targetCoord',target_coord,'ship202',ship202,'ship203',ship203)))
FROM (SELECT IF(\`202\`=7655,'secondary','primary') role,owner_id,target_id,IF(union_id>0,1,0) has_union,\`700\` metal,\`701\` crystal,\`702\` deuterium,
fuel,mission,flight_time,deploy_time,CAST(UNIX_TIMESTAMP() AS SIGNED)-CAST(start AS SIGNED) start_age,
CAST(end AS SIGNED)-CAST(UNIX_TIMESTAMP() AS SIGNED) end_delay,CONCAT_WS(':',origin_g,origin_s,origin_p,origin_type) origin,
CONCAT_WS(':',target_g,target_s,target_p,target_type) target_coord,\`202\` ship202,\`203\` ship203
FROM uni1_fleetlogs WHERE owner_id=$target_id AND \`202\` IN (7654,7655) AND start >= $case_epoch-5 ORDER BY log_id) x")"
  userlogs="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('type',type,'text',text)))
FROM (SELECT type,text FROM uni1_userlogs WHERE owner_id=$target_id AND type='FLEET' AND date >= $case_epoch-5 ORDER BY id) x")"
  old_fleetlog_count="$(db_query 'SELECT COUNT(*) FROM uni1_fleetlogs WHERE start < UNIX_TIMESTAMP()-4*7*24*60*60')"
  old_userlog_count="$(db_query 'SELECT COUNT(*) FROM uni1_userlogs WHERE date < UNIX_TIMESTAMP()-2*7*24*60*60')"
  jq -ncS --argjson queues "$queues" --argjson fleets "$fleets" --argjson fleetlogs "$fleetlogs" --argjson userlogs "$userlogs" \
    --argjson oldFleetlogCount "$old_fleetlog_count" --argjson oldUserlogCount "$old_userlog_count" '
    def stable_time:
      if . >= -5 and . <= 5 then 0
      elif . >= 110 and . <= 130 then 120
      elif . >= 585 and . <= 615 then 600
      elif . >= 1685 and . <= 1715 then 1700
      elif . >= 1785 and . <= 1815 then 1800
      elif . >= 1905 and . <= 1935 then 1920
      else . end;
    {queues:($queues | map(.endDelay |= stable_time | .startAge |= stable_time | .frozenAge |= stable_time)),
     fleets:($fleets | map(.flightTime |= stable_time)),
     fleetlogs:($fleetlogs | map(.flightTime |= stable_time | .startAge |= stable_time | .endDelay |= stable_time)),
     userlogs:($userlogs | map(.text |= gsub("Fleet Recall [0-9]+:"; "Fleet Recall ID:"))),
     oldFleetlogCount:$oldFleetlogCount,oldUserlogCount:$oldUserlogCount}'
}

run_side() {
  side="$1"; case_name="$2"; configure_case "$case_name"
  if [ "$side" = legacy ]; then
    login_legacy "$side-$case_name"
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/$side-$case_name.body" --cookie "$TMP_DIR/$side-$case_name.cookies" \
      --request POST --data "$post_key=$task_id" --write-out '%{http_code}' \
      "$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$actor_planet&mode=$mode")"
    issue=""
  else
    login_go "$side-$case_name"
    payload="$(jq -nc --arg action "$action" --argjson taskId "$task_id" '{action:$action,taskId:$taskId}')"
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/$side-$case_name.body" --header 'Content-Type: application/json' \
      --cookie "$cookie" --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/game/admin?session=$session&cp=$actor_planet&mode=$mode")"
    issue="$(jq -r '.actionIssue.code // empty' "$TMP_DIR/$side-$case_name.body" 2>/dev/null || true)"
  fi
  state="$(capture_state)"
  normalized="$(jq -ncS --argjson http "$http" --arg issue "$issue" --argjson state "$state" '{http:$http,issue:$issue,state:$state}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true
cases="queue-operator-denied queue-end queue-remove queue-freeze queue-unfreeze queue-missing fleet-operator-denied fleet-2min-single fleet-end-single fleet-2min-union fleet-end-union fleet-return fleet-missing"
for case_name in $cases; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.state')" = "$(printf '%s' "$go" | jq -cS '.state')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200 or .http==302' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200' >/dev/null || pass=false
  case "$case_name" in
    *-operator-denied) [ "$(printf '%s' "$go" | jq -r '.issue')" = access_denied ] || pass=false ;;
    *) [ "$(printf '%s' "$go" | jq -r '.issue')" = action_saved ] || pass=false ;;
  esac
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "admin-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" \
  '{pass:$pass,normalization:"generated IDs and wall-clock deltas reduce to stable roles/buckets; queue, fleet, fleetlog and userlog effects remain exact",cases:.}' \
  "$results" > "$REPORT"
[ "$all_pass" = true ]
case_count="$(printf '%s\n' $cases | wc -l | tr -d ' ')"
printf 'Go/PHP admin queue and fleetlogs differential E2E: PASS (%s cases)\n' "$case_count"
