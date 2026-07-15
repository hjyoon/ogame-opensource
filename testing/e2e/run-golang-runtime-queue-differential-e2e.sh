#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_RUNTIME_QUEUE_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-runtime-queue-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/runtime-queue-differential.XXXXXX")"

admin_id="$(jq -r '.player_id // 0' "$FIXTURE")"
admin_planet="$(jq -r '.planet_id // 0' "$FIXTURE")"
target_id="$(jq -r '.admin_operations.target_player_id // 0' "$FIXTURE")"
bot_id="$(jq -r '.bot_runtime.player_id // 0' "$FIXTURE")"
strategy_id="$(jq -r '.bot_runtime.start_strategy_id // 0' "$FIXTURE")"
[ "$admin_id" -gt 0 ] && [ "$admin_planet" -gt 0 ] && [ "$target_id" -gt 0 ] && [ "$bot_id" -gt 0 ] && [ "$strategy_id" -gt 0 ]

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

marker=$((940000000 + ($$ % 1000000) * 10))
queue_backup="uni1_e2e_runtime_queue_$$"
user_backup="uni1_e2e_runtime_users_$$"
ally_backup="uni1_e2e_runtime_ally_$$"
uni_backup="uni1_e2e_runtime_uni_$$"
planet_backup="uni1_e2e_runtime_planets_$$"
botvar_backup="uni1_e2e_runtime_botvars_$$"
debug_backup="uni1_e2e_runtime_debug_$$"
buildqueue_backup="uni1_e2e_runtime_buildqueue_$$"
userlog_backup="uni1_e2e_runtime_userlogs_$$"
queue_types="'UnloadAll','CleanDebris','UpdateStats','RecalcPoints','RecalcAllyPoints','AI'"

db_query "DROP TABLE IF EXISTS $queue_backup,$user_backup,$ally_backup,$uni_backup,$planet_backup,$botvar_backup,$debug_backup,$buildqueue_backup,$userlog_backup;
CREATE TABLE $queue_backup AS SELECT * FROM uni1_queue WHERE type IN ($queue_types) OR owner_id=$bot_id;
CREATE TABLE $user_backup AS SELECT player_id,session,private_session,lastlogin,lastclick,ip_addr,score1,score2,score3,place1,place2,place3,oldscore1,oldscore2,oldscore3,oldplace1,oldplace2,oldplace3,scoredate,admin,aktplanet,vacation,vacation_until,banned,banned_until,disable,disable_until,\`113\`,\`115\` FROM uni1_users;
CREATE TABLE $ally_backup AS SELECT ally_id,score1,score2,score3,place1,place2,place3,oldscore1,oldscore2,oldscore3,oldplace1,oldplace2,oldplace3,scoredate FROM uni1_ally;
CREATE TABLE $uni_backup AS SELECT num,freeze,hacks FROM uni1_uni;
CREATE TABLE $planet_backup AS SELECT * FROM uni1_planets WHERE type IN (10000,20000) OR owner_id=$bot_id;
CREATE TABLE $botvar_backup AS SELECT * FROM uni1_botvars WHERE owner_id=$bot_id;
CREATE TABLE $debug_backup AS SELECT * FROM uni1_debug;
CREATE TABLE $buildqueue_backup AS SELECT * FROM uni1_buildqueue WHERE owner_id=$bot_id;
CREATE TABLE $userlog_backup AS SELECT * FROM uni1_userlogs WHERE owner_id=$bot_id" >/dev/null

restore_case() {
  db_query "DELETE FROM uni1_queue WHERE type IN ($queue_types) OR owner_id=$bot_id; INSERT INTO uni1_queue SELECT * FROM $queue_backup;
UPDATE uni1_users u JOIN $user_backup b ON b.player_id=u.player_id SET u.session=b.session,u.private_session=b.private_session,
u.lastlogin=b.lastlogin,u.lastclick=b.lastclick,u.ip_addr=b.ip_addr,u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,
u.place1=b.place1,u.place2=b.place2,u.place3=b.place3,u.oldscore1=b.oldscore1,u.oldscore2=b.oldscore2,u.oldscore3=b.oldscore3,
u.oldplace1=b.oldplace1,u.oldplace2=b.oldplace2,u.oldplace3=b.oldplace3,u.scoredate=b.scoredate,u.admin=b.admin,
u.aktplanet=b.aktplanet,u.vacation=b.vacation,u.vacation_until=b.vacation_until,u.banned=b.banned,u.banned_until=b.banned_until,
u.disable=b.disable,u.disable_until=b.disable_until,u.\`113\`=b.\`113\`,u.\`115\`=b.\`115\`;
UPDATE uni1_ally a JOIN $ally_backup b ON b.ally_id=a.ally_id SET a.score1=b.score1,a.score2=b.score2,a.score3=b.score3,
a.place1=b.place1,a.place2=b.place2,a.place3=b.place3,a.oldscore1=b.oldscore1,a.oldscore2=b.oldscore2,a.oldscore3=b.oldscore3,
a.oldplace1=b.oldplace1,a.oldplace2=b.oldplace2,a.oldplace3=b.oldplace3,a.scoredate=b.scoredate;
UPDATE uni1_uni u JOIN $uni_backup b ON b.num=u.num SET u.freeze=b.freeze,u.hacks=b.hacks;
DELETE FROM uni1_fleet WHERE fuel=$marker;
DELETE FROM uni1_planets WHERE type IN (10000,20000) OR owner_id=$bot_id; INSERT INTO uni1_planets SELECT * FROM $planet_backup;
DELETE FROM uni1_botvars WHERE owner_id=$bot_id; INSERT INTO uni1_botvars SELECT * FROM $botvar_backup;
DELETE FROM uni1_debug; INSERT INTO uni1_debug SELECT * FROM $debug_backup;
DELETE FROM uni1_buildqueue WHERE owner_id=$bot_id; INSERT INTO uni1_buildqueue SELECT * FROM $buildqueue_backup;
DELETE FROM uni1_userlogs WHERE owner_id=$bot_id; INSERT INTO uni1_userlogs SELECT * FROM $userlog_backup" >/dev/null
}

cleanup() {
  restore_case >/dev/null 2>&1 || true
  db_query "DROP TABLE IF EXISTS $queue_backup,$user_backup,$ally_backup,$uni_backup,$planet_backup,$botvar_backup,$debug_backup,$buildqueue_backup,$userlog_backup" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

admin_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$admin_id")"
[ -n "$admin_login" ]

login_legacy() {
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/legacy-login.headers" --output "$TMP_DIR/legacy-login.body" \
    --cookie-jar "$TMP_DIR/legacy.cookies" --request POST --data-urlencode "login=$admin_login" --data-urlencode "pass=$PASSWORD" \
    --write-out '%{http_code}' "$LEGACY_BASE_URL/game/reg/login2.php")"
  session="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/legacy-login.headers" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
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

insert_planet() {
  id="$1"; name="$2"; type="$3"; metal="$4"; crystal="$5"
  db_query "INSERT INTO uni1_planets (planet_id,name,type,g,s,p,owner_id,diameter,temp,fields,maxfields,date,\`700\`,\`701\`,\`702\`,lastpeek,lastakt,gate_until,remove)
VALUES ($id,'$name',$type,9,$(((id % 400)+1)),$(((id % 15)+1)),99999,0,0,0,0,UNIX_TIMESTAMP(),$metal,$crystal,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP(),0,0)" >/dev/null
}

prepare_case() {
  case_name="$1"
  due="$(db_query 'SELECT UNIX_TIMESTAMP()-1')"
  db_query "DELETE FROM uni1_queue WHERE type IN ($queue_types); UPDATE uni1_uni SET freeze=0" >/dev/null
  case "$case_name" in
    unload-all)
      insert_planet "$((marker+1))" 'rt unused farspace' 20000 0 0
      insert_planet "$((marker+2))" 'rt active farspace' 20000 0 0
      db_query "UPDATE uni1_users SET session='runtimesess',private_session='runtime-private' WHERE player_id=$target_id;
UPDATE uni1_uni SET hacks=7;
INSERT INTO uni1_fleet(owner_id,union_id,\`700\`,\`701\`,\`702\`,fuel,mission,start_planet,target_planet,flight_time,deploy_time,\`202\`)
VALUES($admin_id,0,0,0,0,$marker,15,$admin_planet,$((marker+2)),3600,0,1);
INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES(99999,'UnloadAll',0,$marker,0,$((due-10)),$due,777)" >/dev/null
      ;;
    clean-debris)
      insert_planet "$((marker+3))" 'rt empty debris' 10000 0 0
      insert_planet "$((marker+4))" 'rt active debris' 10000 0 0
      insert_planet "$((marker+5))" 'rt resource debris' 10000 25 10
      db_query "INSERT INTO uni1_fleet(owner_id,union_id,\`700\`,\`701\`,\`702\`,fuel,mission,start_planet,target_planet,flight_time,deploy_time,\`209\`)
VALUES($admin_id,0,0,0,0,$marker,8,$admin_planet,$((marker+4)),3600,0,1);
INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES(99999,'CleanDebris',0,$marker,0,$((due-10)),$due,600)" >/dev/null
      ;;
    update-stats)
      ally_id="$(db_query 'SELECT COALESCE(MIN(ally_id),0) FROM uni1_ally')"; [ "$ally_id" -gt 0 ]
      db_query "UPDATE uni1_users SET score1=111,score2=222,score3=333,place1=4,place2=5,place3=6,
oldscore1=1,oldscore2=2,oldscore3=3,oldplace1=1,oldplace2=1,oldplace3=1,scoredate=0 WHERE player_id=$target_id;
UPDATE uni1_ally SET score1=444,score2=555,score3=666,place1=7,place2=8,place3=9,
oldscore1=1,oldscore2=2,oldscore3=3,oldplace1=1,oldplace2=1,oldplace3=1,scoredate=0 WHERE ally_id=$ally_id;
INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES(99999,'UpdateStats',0,$marker,0,$((due-10)),$due,510)" >/dev/null
      ;;
    recalc-points)
      db_query "UPDATE uni1_users SET score1=987654321,score2=123456789,score3=777777,place1=999,place2=999,place3=999 WHERE player_id=$target_id;
INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES($target_id,'RecalcPoints',0,$marker,0,$((due-10)),$due,500)" >/dev/null
      ;;
    recalc-alliance)
      ally_id="$(db_query 'SELECT COALESCE(MIN(ally_id),0) FROM uni1_ally')"; [ "$ally_id" -gt 0 ]
      db_query "UPDATE uni1_ally SET score1=987654321,score2=123456789,score3=777777,place1=999,place2=999,place3=999 WHERE ally_id=$ally_id;
INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES(99999,'RecalcAllyPoints',0,$marker,0,$((due-10)),$due,400)" >/dev/null
      ;;
    ai)
      block="$(db_query "SELECT CAST(JSON_UNQUOTE(JSON_EXTRACT(source,'$.nodeDataArray[0].key')) AS UNSIGNED) FROM uni1_botstrat WHERE id=$strategy_id")"
      [ "$block" -gt 0 ]
      db_query "INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES($bot_id,'AI',$strategy_id,$block,0,$due,$due,1000)" >/dev/null
      ;;
    ai-full)
      db_query "DELETE FROM uni1_queue WHERE owner_id=$bot_id;
DELETE FROM uni1_buildqueue WHERE owner_id=$bot_id;
DELETE FROM uni1_userlogs WHERE owner_id=$bot_id;
UPDATE uni1_users SET admin=0,vacation=0,vacation_until=0,banned=0,banned_until=0,disable=0,disable_until=0,
aktplanet=(SELECT planet_id FROM $planet_backup WHERE owner_id=$bot_id ORDER BY planet_id LIMIT 1),
\`113\`=2,\`115\`=2 WHERE player_id=$bot_id;
UPDATE uni1_planets SET \`700\`=10000000,\`701\`=10000000,\`702\`=10000000,
\`1\`=0,\`2\`=1,\`3\`=1,\`4\`=20,\`12\`=1,\`14\`=10,\`15\`=0,\`21\`=10,\`31\`=10,
\`202\`=0,\`212\`=5,prod1=0,prod2=0,prod3=0,prod4=0,prod12=0,prod212=0,
fields=0,maxfields=200,lastpeek=$due,lastakt=$due WHERE owner_id=$bot_id AND type=1;
DELETE FROM uni1_botvars WHERE owner_id=$bot_id;
INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES($bot_id,'AI',$strategy_id,1,0,$due,$due,1000)" >/dev/null
      ;;
  esac
}

capture_state() {
  case "$case_name" in
    unload-all)
      db_query "SELECT JSON_OBJECT('unused',(SELECT COUNT(*) FROM uni1_planets WHERE planet_id=$((marker+1))),
'active',(SELECT COUNT(*) FROM uni1_planets WHERE planet_id=$((marker+2))),
'publicSessions',(SELECT COUNT(*) FROM uni1_users WHERE session<>''),'privatePreserved',(SELECT private_session='runtime-private' FROM uni1_users WHERE player_id=$target_id),
'hacks',(SELECT hacks FROM uni1_uni LIMIT 1),'queue',(SELECT COUNT(*) FROM uni1_queue WHERE type='UnloadAll'))"
      ;;
    clean-debris)
      db_query "SELECT JSON_OBJECT('empty',(SELECT COUNT(*) FROM uni1_planets WHERE planet_id=$((marker+3))),
'active',(SELECT COUNT(*) FROM uni1_planets WHERE planet_id=$((marker+4))),'resource',(SELECT COUNT(*) FROM uni1_planets WHERE planet_id=$((marker+5))),
'future',(SELECT COUNT(*) FROM uni1_queue WHERE type='CleanDebris' AND end>$due),
'weekday',(SELECT DAYOFWEEK(FROM_UNIXTIME(end)) FROM uni1_queue WHERE type='CleanDebris' LIMIT 1),
'hour',(SELECT HOUR(FROM_UNIXTIME(end)) FROM uni1_queue WHERE type='CleanDebris' LIMIT 1),
'minute',(SELECT MINUTE(FROM_UNIXTIME(end)) FROM uni1_queue WHERE type='CleanDebris' LIMIT 1))"
      ;;
    update-stats)
      db_query "SELECT JSON_OBJECT('user',(SELECT JSON_OBJECT('old1',oldscore1,'old2',oldscore2,'old3',oldscore3,'place1',oldplace1,'place2',oldplace2,'place3',oldplace3,'date',scoredate=$due) FROM uni1_users WHERE player_id=$target_id),
'ally',(SELECT JSON_OBJECT('old1',oldscore1,'old2',oldscore2,'old3',oldscore3,'place1',oldplace1,'place2',oldplace2,'place3',oldplace3,'date',scoredate=$due) FROM uni1_ally WHERE ally_id=$ally_id),
'future',(SELECT COUNT(*) FROM uni1_queue WHERE type='UpdateStats' AND start=$due AND end>$due),
'weekday',(SELECT DAYOFWEEK(FROM_UNIXTIME(end)) FROM uni1_queue WHERE type='UpdateStats' LIMIT 1),
'hour',(SELECT HOUR(FROM_UNIXTIME(end)) FROM uni1_queue WHERE type='UpdateStats' LIMIT 1),
'minute',(SELECT MINUTE(FROM_UNIXTIME(end)) FROM uni1_queue WHERE type='UpdateStats' LIMIT 1),
'debug',(SELECT COUNT(*) FROM uni1_debug WHERE date BETWEEN $due AND $((due+10)) AND text LIKE '%points saved%'))"
      ;;
    recalc-points)
      db_query "SELECT JSON_OBJECT('score1',score1,'score2',score2,'score3',score3,'place1',place1,'place2',place2,'place3',place3,
'queue',(SELECT COUNT(*) FROM uni1_queue WHERE type='RecalcPoints' AND owner_id=$target_id)) FROM uni1_users WHERE player_id=$target_id"
      ;;
    recalc-alliance)
      db_query "SELECT JSON_OBJECT('score1',score1,'score2',score2,'score3',score3,'place1',place1,'place2',place2,'place3',place3,
'queue',(SELECT COUNT(*) FROM uni1_queue WHERE type='RecalcAllyPoints')) FROM uni1_ally WHERE ally_id=$ally_id"
      ;;
    ai)
      db_query "SELECT JSON_OBJECT('count',COUNT(*),'sub',COALESCE(MAX(sub_id),0),'obj',COALESCE(MAX(obj_id),0),
'startAtDue',COALESCE(MAX(start=$due),0),'endAtDue',COALESCE(MAX(end=$due),0),'prio',COALESCE(MAX(prio),0))
FROM uni1_queue WHERE type='AI' AND owner_id=$bot_id"
      ;;
    ai-full)
      db_query "SELECT JSON_OBJECT(
'phase',COALESCE((SELECT value FROM uni1_botvars WHERE owner_id=$bot_id AND var='phase' LIMIT 1),''),
'production',(SELECT JSON_OBJECT('metal',prod1,'crystal',prod2,'deuterium',prod3,'solar',prod4,'fusion',prod12,'satellite',prod212) FROM uni1_planets WHERE owner_id=$bot_id AND type=1 LIMIT 1),
'resources',(SELECT JSON_OBJECT('metal',\`700\`,'crystal',\`701\`,'deuterium',\`702\`) FROM uni1_planets WHERE owner_id=$bot_id AND type=1 LIMIT 1),
'buildQueue',COALESCE((SELECT CONCAT(tech_id,':',level,':',destroy) FROM uni1_buildqueue WHERE owner_id=$bot_id ORDER BY id LIMIT 1),''),
'buildTask',COALESCE((SELECT CONCAT(obj_id,':',level,':',CAST(start AS SIGNED)-$due,':',end-start) FROM uni1_queue WHERE owner_id=$bot_id AND type='Build' LIMIT 1),''),
'researchTask',COALESCE((SELECT CONCAT(obj_id,':',level,':',CAST(start AS SIGNED)-$due,':',end-start) FROM uni1_queue WHERE owner_id=$bot_id AND type='Research' LIMIT 1),''),
'shipyardTask',COALESCE((SELECT CONCAT(obj_id,':',level,':',CAST(start AS SIGNED)-$due,':',end-start) FROM uni1_queue WHERE owner_id=$bot_id AND type='Shipyard' LIMIT 1),''),
'aiTasks',COALESCE((SELECT GROUP_CONCAT(CONCAT(sub_id,':',obj_id,':',CAST(start AS SIGNED)-$due,':',end-start) ORDER BY sub_id,obj_id SEPARATOR ',') FROM uni1_queue WHERE owner_id=$bot_id AND type='AI'),''))"
      ;;
  esac
}

run_side() {
  side="$1"; case_name="$2"
  restore_case
  db_query "UPDATE uni1_users SET admin=2 WHERE player_id=$admin_id" >/dev/null
  if [ "$side" = legacy ]; then login_legacy; else login_go; fi
  prepare_case "$case_name"
  request_count=1
  [ "$case_name" = ai-full ] && request_count="${OGAME_RUNTIME_AI_REQUEST_COUNT:-8}"
  if [ "$side" = legacy ]; then
    request_index=1
    while [ "$request_index" -le "$request_count" ]; do
      http="$(curl --silent --show-error --max-time 90 --output "$TMP_DIR/$side-$case_name-$request_index.body" --cookie "$TMP_DIR/legacy.cookies" \
        --request POST --data 'order_cron=1' --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$admin_planet&mode=Queue")"
      request_index=$((request_index+1))
    done
    issue=""
  else
    request_index=1
    while [ "$request_index" -le "$request_count" ]; do
      http="$(curl --silent --show-error --max-time 90 --output "$TMP_DIR/$side-$case_name-$request_index.body" --header 'Content-Type: application/json' \
        --cookie "$cookie" --data '{"action":"queue_cron"}' --write-out '%{http_code}' "$GO_BASE_URL/api/game/admin?session=$session&cp=$admin_planet&mode=Queue")"
      request_index=$((request_index+1))
    done
    issue="$(jq -r '.actionIssue.code // empty' "$TMP_DIR/$side-$case_name-$request_count.body")"
  fi
  state="$(capture_state | jq -cS '.')"
  fatal=false
  if grep -Eqi 'Fatal error|Parse error|Error-ID:' "$TMP_DIR/$side-$case_name-"*.body 2>/dev/null; then fatal=true; fi
  normalized="$(jq -ncS --argjson http "$http" --arg issue "$issue" --argjson fatal "$fatal" --argjson state "$state" '{http:$http,issue:$issue,fatal:$fatal,state:$state}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true; case_count=0
cases="${OGAME_RUNTIME_QUEUE_CASES:-unload-all clean-debris update-stats recalc-points recalc-alliance ai ai-full}"
for case_name in $cases; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.state')" = "$(printf '%s' "$go" | jq -cS '.state')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200 or .http==302' >/dev/null || pass=false
  printf '%s' "$legacy" | jq -e '.fatal==false' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200 and .issue=="action_saved" and .fatal==false' >/dev/null || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "runtime-queue-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
  case_count=$((case_count+1))
done

jq -s --argjson pass "$all_pass" --argjson cases "$case_count" \
  '{pass:$pass,normalization:"generated queue, fleet and session IDs are excluded; persisted effects, protected rows, ranks, recurring schedules and bot transitions remain exact",metrics:{cases:$cases},cases:.}' \
  "$results" > "$REPORT"
[ "$all_pass" = true ] || { jq '.cases[] | select(.pass == false)' "$REPORT" >&2; exit 1; }
printf 'Go/PHP runtime queue differential E2E: PASS (%s cases)\n' "$case_count"
