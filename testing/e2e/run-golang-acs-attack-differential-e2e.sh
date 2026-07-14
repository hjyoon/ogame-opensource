#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_ACS_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-acs-attack-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ]
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/acs-attack-differential.XXXXXX")"

head_login="$(jq -r '.fleet_lifecycle.attacker.login' "$FIXTURE")"
head_id="$(jq -r '.fleet_lifecycle.attacker.player_id' "$FIXTURE")"
head_planet="$(jq -r '.fleet_lifecycle.attacker.home_planet_id' "$FIXTURE")"
target_id="$(jq -r '.fleet_lifecycle.target.player_id' "$FIXTURE")"
target_planet="$(jq -r '.fleet_lifecycle.target.home_planet_id' "$FIXTURE")"
support_login="$(jq -r '.acs_hold.holder.login' "$FIXTURE")"
support_id="$(jq -r '.acs_hold.holder.player_id' "$FIXTURE")"
support_planet="$(jq -r '.acs_hold.holder.home_planet_id' "$FIXTURE")"
players="$head_id,$support_id,$target_id"
planets="$head_planet,$support_planet,$target_planet"

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'exec mysql -N -B -r -uroot -p"$MYSQL_ROOT_PASSWORD" uni -e "$1"' sh "$1" 2>/dev/null
}

planet_backup="uni1_e2e_acsdiff_planets_$$"
user_backup="uni1_e2e_acsdiff_users_$$"
rank_backup="uni1_e2e_acsdiff_ranks_$$"
fleet_backup="uni1_e2e_acsdiff_fleet_$$"
queue_backup="uni1_e2e_acsdiff_queue_$$"
message_backup="uni1_e2e_acsdiff_messages_$$"
log_backup="uni1_e2e_acsdiff_logs_$$"
union_backup="uni1_e2e_acsdiff_union_$$"
battle_before_max="$(db_query 'SELECT COALESCE(MAX(battle_id),0) FROM uni1_battledata')"
target_g="$(db_query "SELECT g FROM uni1_planets WHERE planet_id=$target_planet")"
target_s="$(db_query "SELECT s FROM uni1_planets WHERE planet_id=$target_planet")"
target_p="$(db_query "SELECT p FROM uni1_planets WHERE planet_id=$target_planet")"
support_location="$(db_query "WITH RECURSIVE systems(sys_num) AS (SELECT 1 UNION ALL SELECT sys_num+1 FROM systems WHERE sys_num<499), slots(slot_num) AS (SELECT 1 UNION ALL SELECT slot_num+1 FROM slots WHERE slot_num<15) SELECT JSON_OBJECT('system',systems.sys_num,'position',slots.slot_num) FROM systems CROSS JOIN slots LEFT JOIN uni1_planets p ON p.g=$target_g AND p.s=systems.sys_num AND p.p=slots.slot_num AND p.type=1 WHERE p.planet_id IS NULL ORDER BY ABS(systems.sys_num-$target_s),systems.sys_num,slots.slot_num LIMIT 1")"
if [ -z "$support_location" ]; then
  printf 'ACS differential: target galaxy has no empty support slot\n' >&2
  exit 1
fi
support_s="$(printf '%s' "$support_location" | jq -r '.system')"
support_p="$(printf '%s' "$support_location" | jq -r '.position')"

db_query "DROP TABLE IF EXISTS $planet_backup,$user_backup,$rank_backup,$fleet_backup,$queue_backup,$message_backup,$log_backup,$union_backup; CREATE TABLE $planet_backup AS SELECT * FROM uni1_planets WHERE planet_id IN ($planets); CREATE TABLE $user_backup AS SELECT * FROM uni1_users WHERE player_id IN ($players); CREATE TABLE $rank_backup AS SELECT player_id,score1,score2,score3,place1,place2,place3,oldscore1,oldscore2,oldscore3,oldplace1,oldplace2,oldplace3 FROM uni1_users; CREATE TABLE $fleet_backup AS SELECT * FROM uni1_fleet WHERE owner_id IN ($players) OR start_planet IN ($planets) OR target_planet IN ($planets); CREATE TABLE $queue_backup AS SELECT * FROM uni1_queue WHERE type='Fleet' AND sub_id IN (SELECT fleet_id FROM $fleet_backup); CREATE TABLE $message_backup AS SELECT * FROM uni1_messages WHERE owner_id IN ($players); CREATE TABLE $log_backup AS SELECT * FROM uni1_fleetlogs WHERE owner_id IN ($players) OR target_id IN ($players); CREATE TABLE $union_backup AS SELECT * FROM uni1_union WHERE fleet_id IN (SELECT fleet_id FROM $fleet_backup) OR target_player IN ($players)" >/dev/null

delete_case_state() {
  db_query "DELETE q FROM uni1_queue q LEFT JOIN uni1_fleet f ON f.fleet_id=q.sub_id WHERE q.type='Fleet' AND (q.owner_id IN ($players) OR f.owner_id IN ($players) OR f.start_planet IN ($planets) OR f.target_planet IN ($planets)); DELETE FROM uni1_union WHERE fleet_id IN (SELECT fleet_id FROM uni1_fleet WHERE owner_id IN ($players)) OR target_player IN ($players); DELETE FROM uni1_fleet WHERE owner_id IN ($players) OR start_planet IN ($planets) OR target_planet IN ($planets); DELETE FROM uni1_messages WHERE owner_id IN ($players); DELETE FROM uni1_fleetlogs WHERE owner_id IN ($players) OR target_id IN ($players); DELETE FROM uni1_battledata WHERE battle_id > $battle_before_max; DELETE FROM uni1_planets WHERE type=10000 AND g=$target_g AND s=$target_s AND p=$target_p" >/dev/null
}

reset_case() {
  delete_case_state
  db_query "UPDATE uni1_planets SET \`700\`=1000000,\`701\`=1000000,\`702\`=1000000,\`202\`=0,\`203\`=0,\`204\`=0,\`205\`=0,\`206\`=0,\`207\`=0,\`208\`=0,\`209\`=0,\`210\`=0,\`211\`=0,\`212\`=0,\`213\`=0,\`214\`=0,\`215\`=0,\`401\`=0,\`402\`=0,\`403\`=0,\`404\`=0,\`405\`=0,\`406\`=0,\`407\`=0,\`408\`=0,lastpeek=UNIX_TIMESTAMP(),lastakt=UNIX_TIMESTAMP(),prod1=0,prod2=0,prod3=0,prod4=0,prod12=0,prod212=0 WHERE planet_id IN ($planets); UPDATE uni1_planets SET \`204\`=10 WHERE planet_id IN ($head_planet,$support_planet); UPDATE uni1_planets SET g=$target_g,s=$support_s,p=$support_p WHERE planet_id=$support_planet; UPDATE uni1_users u JOIN $rank_backup b ON b.player_id=u.player_id SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3,u.oldscore1=b.oldscore1,u.oldscore2=b.oldscore2,u.oldscore3=b.oldscore3,u.oldplace1=b.oldplace1,u.oldplace2=b.oldplace2,u.oldplace3=b.oldplace3; UPDATE uni1_users u JOIN $user_backup b ON b.player_id=u.player_id SET u.session='',u.private_session='',u.vacation=0,u.vacation_until=0,u.banned=0,u.banned_until=0,u.noattack=0,u.noattack_until=0,u.disable=0,u.disable_until=0,u.validated=1,u.deact_ip=1,u.aktplanet=u.hplanetid WHERE u.player_id IN ($players)" >/dev/null
}

restore_original() {
  delete_case_state >/dev/null 2>&1 || true
  db_query "UPDATE uni1_users u JOIN $user_backup b ON b.player_id=u.player_id SET u.session=b.session,u.private_session=b.private_session,u.lastlogin=b.lastlogin,u.lastclick=b.lastclick,u.ip_addr=b.ip_addr,u.aktplanet=b.aktplanet,u.vacation=b.vacation,u.vacation_until=b.vacation_until,u.banned=b.banned,u.banned_until=b.banned_until,u.noattack=b.noattack,u.noattack_until=b.noattack_until,u.disable=b.disable,u.disable_until=b.disable_until,u.validated=b.validated,u.deact_ip=b.deact_ip; UPDATE uni1_users u JOIN $rank_backup b ON b.player_id=u.player_id SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3,u.oldscore1=b.oldscore1,u.oldscore2=b.oldscore2,u.oldscore3=b.oldscore3,u.oldplace1=b.oldplace1,u.oldplace2=b.oldplace2,u.oldplace3=b.oldplace3; DELETE FROM uni1_planets WHERE planet_id IN ($planets); INSERT INTO uni1_planets SELECT * FROM $planet_backup; INSERT INTO uni1_fleet SELECT * FROM $fleet_backup; INSERT INTO uni1_queue SELECT * FROM $queue_backup; INSERT INTO uni1_messages SELECT * FROM $message_backup; INSERT INTO uni1_fleetlogs SELECT * FROM $log_backup; INSERT INTO uni1_union SELECT * FROM $union_backup; DROP TABLE IF EXISTS $planet_backup,$user_backup,$rank_backup,$fleet_backup,$queue_backup,$message_backup,$log_backup,$union_backup" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

coordinates() {
  db_query "SELECT g,s,p FROM uni1_planets WHERE planet_id=$1 LIMIT 1"
}

login_legacy() {
  key="$1" login="$2"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$key.headers" --output "$TMP_DIR/$key.body" --cookie-jar "$TMP_DIR/$key.cookies" --request POST --data-urlencode "login=$login" --data-urlencode "pass=$PASSWORD" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/reg/login2.php")"
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/$key.headers")"
  session="$(printf '%s' "$location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 302 ] && [ -n "$session" ]
}

login_go() {
  key="$1" login="$2"
  payload="$(jq -nc --arg login "$login" --arg pass "$PASSWORD" '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$key.headers" --output "$TMP_DIR/$key.body" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/$key.headers" | cut -d';' -f1 | tr -d '\r')"
  redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/$key.body")"
  session="$(printf '%s' "$redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 200 ] && [ -n "$cookie" ] && [ -n "$session" ]
}

legacy_launch() {
  key="$1" cookie_file="$2" public_session="$3" origin="$4" mission="$5" union_id="$6"
  origin_coords="$(coordinates "$origin")"; target_coords="$(coordinates "$target_planet")"
  old_ifs="$IFS"; IFS="$(printf '\t')"; set -- $origin_coords; IFS="$old_ifs"; og="$1" os="$2" op="$3"
  IFS="$(printf '\t')"; set -- $target_coords; IFS="$old_ifs"; tg="$1" ts="$2" tp="$3"
  fspeed="$(db_query 'SELECT fspeed FROM uni1_uni LIMIT 1')"
  curl --silent --show-error --max-time 15 --output "$TMP_DIR/$key-launch.body" --cookie "$cookie_file" --data-urlencode "thisgalaxy=$og" --data-urlencode "thissystem=$os" --data-urlencode "thisplanet=$op" --data-urlencode 'thisplanettype=1' --data-urlencode "speedfactor=$fspeed" --data-urlencode "galaxy=$tg" --data-urlencode "system=$ts" --data-urlencode "planet=$tp" --data-urlencode 'planettype=1' --data-urlencode 'speed=10' --data-urlencode "order=$mission" --data-urlencode 'ship204=1' --data-urlencode 'resource1=0' --data-urlencode 'resource2=0' --data-urlencode 'resource3=0' --data-urlencode "union2=$union_id" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=flottenversand&session=$public_session&cp=$origin"
}

go_launch() {
  key="$1" private_cookie="$2" public_session="$3" origin="$4" mission="$5" union_id="$6"
  target_coords="$(coordinates "$target_planet")"
  old_ifs="$IFS"; IFS="$(printf '\t')"; set -- $target_coords; IFS="$old_ifs"
  payload="$(jq -nc --arg g "$1" --arg s "$2" --arg p "$3" --arg mission "$mission" --arg union "$union_id" '{action:"launch-dispatch",ships:{"204":1},resources:{"700":0,"701":0,"702":0},target:{galaxy:($g|tonumber),system:($s|tonumber),position:($p|tonumber)},targetType:1,mission:($mission|tonumber),speed:10,unionId:($union|tonumber)}')"
  curl --silent --show-error --max-time 15 --output "$TMP_DIR/$key-launch.body" --cookie "$private_cookie" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/game/fleet?session=$public_session&cp=$origin"
}

legacy_recall() {
  key="$1" cookie_file="$2" public_session="$3" origin="$4" recall_id="$5"
  curl --silent --show-error --max-time 15 --output "$TMP_DIR/$key-recall.body" --cookie "$cookie_file" --data-urlencode "order_return=$recall_id" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=flotten1&session=$public_session&cp=$origin"
}

go_recall() {
  key="$1" private_cookie="$2" public_session="$3" origin="$4" recall_id="$5"
  payload="$(jq -nc --arg id "$recall_id" '{action:"recall",fleetId:($id|tonumber)}')"
  curl --silent --show-error --max-time 15 --output "$TMP_DIR/$key-recall.body" --cookie "$private_cookie" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/game/fleet?session=$public_session&cp=$origin"
}

create_union() {
  fleet_id="$(db_query "SELECT fleet_id FROM uni1_fleet WHERE owner_id=$head_id AND mission=1 ORDER BY fleet_id DESC LIMIT 1")"
  if [ -z "$fleet_id" ]; then
    printf 'ACS differential: head fleet was not created\n' >&2
    return 1
  fi
  union_id="$(db_query "INSERT INTO uni1_union (fleet_id,target_player,name,players) VALUES ($fleet_id,$target_id,'E2EACS','$head_id,$support_id'); SELECT LAST_INSERT_ID();")"
  # Keep the join window independent of universe speed and fixture coordinates.
  db_query "UPDATE uni1_fleet SET union_id=$union_id,mission=21 WHERE fleet_id=$fleet_id; UPDATE uni1_queue SET start=UNIX_TIMESTAMP(),end=UNIX_TIMESTAMP()+604800 WHERE type='Fleet' AND sub_id=$fleet_id" >/dev/null
}

require_support_fleet() {
  support_fleet_id="$(db_query "SELECT fleet_id FROM uni1_fleet WHERE owner_id=$support_id AND mission=2 ORDER BY fleet_id DESC LIMIT 1")"
  if [ -z "$support_fleet_id" ]; then
    printf 'ACS differential: %s support fleet was not created\n' "$1" >&2
    return 1
  fi
}

capture_state() {
  planets_json="$(db_query "SELECT JSON_ARRAYAGG(JSON_OBJECT('id',planet_id,'metal',ROUND(\`700\`),'crystal',ROUND(\`701\`),'deuterium',ROUND(\`702\`),'fighters',\`204\`)) FROM (SELECT * FROM uni1_planets WHERE planet_id IN ($planets) ORDER BY planet_id) p" | jq -c 'sort_by(.id)')"
  fleets_json="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('ownerId',owner_id,'mission',mission,'startPlanet',start_planet,'targetPlanet',target_planet,'flightTime',flight_time,'deployTime',deploy_time,'fuel',fuel,'metal',ROUND(\`700\`),'crystal',ROUND(\`701\`),'deuterium',ROUND(\`702\`),'fighters',\`204\`,'inUnion',IF(union_id>0,1,0)))) FROM (SELECT * FROM uni1_fleet WHERE owner_id IN ($head_id,$support_id) ORDER BY owner_id,mission) f" | jq -c 'sort_by(.ownerId,.mission)')"
  queues_json="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('ownerId',f.owner_id,'mission',f.mission,'arrivalSkew',q.end-(SELECT MIN(q2.end) FROM uni1_queue q2 JOIN uni1_fleet f2 ON f2.fleet_id=q2.sub_id WHERE q2.type='Fleet' AND f2.owner_id IN ($head_id,$support_id)),'prio',q.prio))) FROM (SELECT q.* FROM uni1_queue q WHERE q.type='Fleet') q JOIN uni1_fleet f ON f.fleet_id=q.sub_id WHERE f.owner_id IN ($head_id,$support_id)" | jq -c 'sort_by(.ownerId,.mission)')"
  messages_json="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('ownerId',owner_id,'type',pm,'from',msgfrom,'subject',subj,'text',text,'shown',shown))) FROM (SELECT * FROM uni1_messages WHERE owner_id IN ($players) ORDER BY msg_id) m" | jq -c 'walk(if type=="string" then gsub("bericht=[0-9]+";"bericht=<id>") | gsub("At [0-9]{2}-[0-9]{2} [0-9:]{8}";"At <time>") else . end)')"
  logs_json="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('ownerId',owner_id,'targetId',target_id,'mission',mission,'duration',end-start,'flightTime',flight_time,'deployTime',deploy_time,'fuel',fuel,'metal',ROUND(\`700\`),'crystal',ROUND(\`701\`),'deuterium',ROUND(\`702\`),'fighters',\`204\`,'inUnion',IF(union_id>0,1,0)))) FROM (SELECT * FROM uni1_fleetlogs WHERE owner_id IN ($players) OR target_id IN ($players) ORDER BY log_id) l" | jq -c 'sort_by(.ownerId,.mission,.duration)')"
  battle_json="$(db_query "SELECT JSON_OBJECT('title',title,'report',report) FROM uni1_battledata WHERE battle_id>$battle_before_max ORDER BY battle_id DESC LIMIT 1")"
  [ -n "$battle_json" ] || battle_json=null
  battle_json="$(printf '%s' "$battle_json" | jq -c 'walk(if type=="string" then gsub("bericht=[0-9]+";"bericht=<id>") | gsub("At [0-9]{2}-[0-9]{2} [0-9:]{8}";"At <time>") else . end)')"
  union_count="$(db_query "SELECT COUNT(*) FROM uni1_union WHERE target_player=$target_id AND FIND_IN_SET('$head_id',players)")"
  debris_json="$(db_query "SELECT JSON_OBJECT('metal',ROUND(\`700\`),'crystal',ROUND(\`701\`)) FROM uni1_planets WHERE type=10000 AND g=$target_g AND s=$target_s AND p=$target_p LIMIT 1")"
  [ -n "$debris_json" ] || debris_json=null
  jq -nc --argjson planets "$planets_json" --argjson fleets "$fleets_json" --argjson queues "$queues_json" --argjson messages "$messages_json" --argjson logs "$logs_json" --argjson battle "$battle_json" --argjson debris "$debris_json" --arg unions "$union_count" '{planets:$planets,fleets:$fleets,queues:$queues,messages:$messages,fleetLogs:$logs,battle:$battle,debris:$debris,unionCount:($unions|tonumber)}'
}

capture_recall_state() {
  capture_state | jq -c '(.fleets[]? |= del(.flightTime)) | (.queues[]? |= del(.arrivalSkew)) | (.fleetLogs[]? |= del(.duration,.flightTime))'
}

force_due() {
  db_query "UPDATE uni1_queue q JOIN uni1_fleet f ON f.fleet_id=q.sub_id SET q.start=UNIX_TIMESTAMP()-2,q.end=UNIX_TIMESTAMP()-1 WHERE q.type='Fleet' AND f.owner_id IN ($head_id,$support_id)" >/dev/null
}

run_side() {
  side="$1"
  reset_case
  if [ "$side" = legacy ]; then
    login_legacy legacy-head "$head_login"; head_session="$session"; head_cookie="$TMP_DIR/legacy-head.cookies"
    login_legacy legacy-support "$support_login"; support_session="$session"; support_cookie="$TMP_DIR/legacy-support.cookies"
    head_status="$(legacy_launch legacy-head "$head_cookie" "$head_session" "$head_planet" 1 0)"
  else
    login_go go-head "$head_login"; head_session="$session"; head_cookie="$cookie"
    login_go go-support "$support_login"; support_session="$session"; support_cookie="$cookie"
    head_status="$(go_launch go-head "$head_cookie" "$head_session" "$head_planet" 1 0)"
  fi
  create_union
  if [ "$side" = legacy ]; then
    support_status="$(legacy_launch legacy-support "$support_cookie" "$support_session" "$support_planet" 2 "$union_id")"
  else
    support_status="$(go_launch go-support "$support_cookie" "$support_session" "$support_planet" 2 "$union_id")"
  fi
  require_support_fleet "$side"
  joined="$(capture_state)"
  force_due
  if [ "$side" = legacy ]; then
    battle_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-battle.body" --cookie "$head_cookie" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=flotten1&session=$head_session&cp=$head_planet")"
  else
    battle_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-battle.body" --cookie "$head_cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/fleet?session=$head_session&cp=$head_planet")"
  fi
  returning="$(capture_state)"
  force_due
  if [ "$side" = legacy ]; then
    final_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-final.body" --cookie "$head_cookie" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=flotten1&session=$head_session&cp=$head_planet")"
  else
    final_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-final.body" --cookie "$head_cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/fleet?session=$head_session&cp=$head_planet")"
  fi
  final="$(capture_state)"
}

run_recall_side() {
  side="$1"
  reset_case
  if [ "$side" = legacy ]; then
    login_legacy legacy-recall-head "$head_login"; head_session="$session"; head_cookie="$TMP_DIR/legacy-recall-head.cookies"
    login_legacy legacy-recall-support "$support_login"; support_session="$session"; support_cookie="$TMP_DIR/legacy-recall-support.cookies"
    recall_head_launch_status="$(legacy_launch legacy-recall-head "$head_cookie" "$head_session" "$head_planet" 1 0)"
  else
    login_go go-recall-head "$head_login"; head_session="$session"; head_cookie="$cookie"
    login_go go-recall-support "$support_login"; support_session="$session"; support_cookie="$cookie"
    recall_head_launch_status="$(go_launch go-recall-head "$head_cookie" "$head_session" "$head_planet" 1 0)"
  fi
  create_union
  if [ "$side" = legacy ]; then
    recall_support_launch_status="$(legacy_launch legacy-recall-support "$support_cookie" "$support_session" "$support_planet" 2 "$union_id")"
  else
    recall_support_launch_status="$(go_launch go-recall-support "$support_cookie" "$support_session" "$support_planet" 2 "$union_id")"
  fi
  require_support_fleet "$side recall"
  db_query "UPDATE uni1_queue q JOIN uni1_fleet f ON f.fleet_id=q.sub_id SET q.start=UNIX_TIMESTAMP()-600,q.end=UNIX_TIMESTAMP()+3000 WHERE q.type='Fleet' AND f.owner_id IN ($head_id,$support_id) AND f.mission IN (2,21)" >/dev/null
  if [ "$side" = legacy ]; then
    support_recall_status="$(legacy_recall legacy-recall-support "$support_cookie" "$support_session" "$support_planet" "$support_fleet_id")"
  else
    support_recall_status="$(go_recall go-recall-support "$support_cookie" "$support_session" "$support_planet" "$support_fleet_id")"
  fi
  support_recalled="$(capture_recall_state)"
  head_fleet_id="$(db_query "SELECT fleet_id FROM uni1_fleet WHERE owner_id=$head_id AND mission=21 ORDER BY fleet_id DESC LIMIT 1")"
  if [ "$side" = legacy ]; then
    head_recall_status="$(legacy_recall legacy-recall-head "$head_cookie" "$head_session" "$head_planet" "$head_fleet_id")"
  else
    head_recall_status="$(go_recall go-recall-head "$head_cookie" "$head_session" "$head_planet" "$head_fleet_id")"
  fi
  all_recalled="$(capture_recall_state)"
  force_due
  if [ "$side" = legacy ]; then
    recall_final_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-recall-final.body" --cookie "$head_cookie" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=flotten1&session=$head_session&cp=$head_planet")"
  else
    recall_final_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-recall-final.body" --cookie "$head_cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/fleet?session=$head_session&cp=$head_planet")"
  fi
  recall_final="$(capture_recall_state)"
}

run_side legacy
legacy_head_status="$head_status"; legacy_support_status="$support_status"; legacy_battle_status="$battle_status"; legacy_final_status="$final_status"
legacy_joined="$joined"; legacy_returning="$returning"; legacy_final="$final"
run_side go
go_head_status="$head_status"; go_support_status="$support_status"; go_battle_status="$battle_status"; go_final_status="$final_status"
go_joined="$joined"; go_returning="$returning"; go_final="$final"
run_recall_side legacy
legacy_recall_head_launch_status="$recall_head_launch_status"; legacy_recall_support_launch_status="$recall_support_launch_status"; legacy_support_recall_status="$support_recall_status"; legacy_head_recall_status="$head_recall_status"; legacy_recall_final_status="$recall_final_status"
legacy_support_recalled="$support_recalled"; legacy_all_recalled="$all_recalled"; legacy_recall_final="$recall_final"
run_recall_side go
go_recall_head_launch_status="$recall_head_launch_status"; go_recall_support_launch_status="$recall_support_launch_status"; go_support_recall_status="$support_recall_status"; go_head_recall_status="$head_recall_status"; go_recall_final_status="$recall_final_status"
go_support_recalled="$support_recalled"; go_all_recalled="$all_recalled"; go_recall_final="$recall_final"

pass=true
[ "$legacy_head_status" = 200 ] && [ "$go_head_status" = 200 ] || pass=false
[ "$legacy_support_status" = 200 ] && [ "$go_support_status" = 200 ] || pass=false
[ "$legacy_battle_status" = 200 ] && [ "$go_battle_status" = 200 ] || pass=false
[ "$legacy_final_status" = 200 ] && [ "$go_final_status" = 200 ] || pass=false
[ "$legacy_joined" = "$go_joined" ] || pass=false
[ "$legacy_returning" = "$go_returning" ] || pass=false
[ "$legacy_final" = "$go_final" ] || pass=false
[ "$legacy_recall_head_launch_status" = 200 ] && [ "$go_recall_head_launch_status" = 200 ] || pass=false
[ "$legacy_recall_support_launch_status" = 200 ] && [ "$go_recall_support_launch_status" = 200 ] || pass=false
[ "$legacy_support_recall_status" = 200 ] && [ "$go_support_recall_status" = 200 ] || pass=false
[ "$legacy_head_recall_status" = 200 ] && [ "$go_head_recall_status" = 200 ] || pass=false
[ "$legacy_recall_final_status" = 200 ] && [ "$go_recall_final_status" = 200 ] || pass=false
[ "$legacy_support_recalled" = "$go_support_recalled" ] || pass=false
[ "$legacy_all_recalled" = "$go_all_recalled" ] || pass=false
[ "$legacy_recall_final" = "$go_recall_final" ] || pass=false
printf '%s' "$legacy_joined" | jq -e '.unionCount==1 and (.fleets|length)==2 and ([.fleets[].mission]|sort)==[2,21] and ([.queues[].arrivalSkew]|unique)==[0]' >/dev/null || pass=false
printf '%s' "$legacy_returning" | jq -e '.unionCount==0 and (.fleets|length)==2 and ([.fleets[].mission]|sort)==[102,121] and (.messages|length)==6 and .battle.report!=""' >/dev/null || pass=false
printf '%s' "$legacy_final" | jq -e '.unionCount==0 and (.fleets|length)==0 and (.queues|length)==0 and (.messages|length)==8 and ([.planets[]|select(.id=='"$head_planet"' or .id=='"$support_planet"')|.fighters]|sort)==[10,10]' >/dev/null || pass=false
printf '%s' "$legacy_support_recalled" | jq -e '.unionCount==1 and ([.fleets[].mission]|sort)==[21,102] and ([.fleets[].inUnion]|sort)==[0,1]' >/dev/null || pass=false
printf '%s' "$legacy_all_recalled" | jq -e '.unionCount==0 and ([.fleets[].mission]|sort)==[102,121] and ([.fleets[].inUnion]|unique)==[0]' >/dev/null || pass=false
printf '%s' "$legacy_recall_final" | jq -e '.unionCount==0 and (.fleets|length)==0 and (.queues|length)==0 and (.messages|length)==2 and ([.planets[]|select(.id=='"$head_planet"' or .id=='"$support_planet"')|.fighters]|sort)==[10,10]' >/dev/null || pass=false

jq -nc --argjson pass "$pass" --arg legacyHead "$legacy_head_status" --arg goHead "$go_head_status" --arg legacySupport "$legacy_support_status" --arg goSupport "$go_support_status" --arg legacyBattle "$legacy_battle_status" --arg goBattle "$go_battle_status" --arg legacyFinalStatus "$legacy_final_status" --arg goFinalStatus "$go_final_status" --arg legacySupportRecall "$legacy_support_recall_status" --arg goSupportRecall "$go_support_recall_status" --arg legacyHeadRecall "$legacy_head_recall_status" --arg goHeadRecall "$go_head_recall_status" --argjson legacyJoined "$legacy_joined" --argjson goJoined "$go_joined" --argjson legacyReturning "$legacy_returning" --argjson goReturning "$go_returning" --argjson legacyFinal "$legacy_final" --argjson goFinal "$go_final" --argjson legacySupportRecalled "$legacy_support_recalled" --argjson goSupportRecalled "$go_support_recalled" --argjson legacyAllRecalled "$legacy_all_recalled" --argjson goAllRecalled "$go_all_recalled" --argjson legacyRecallFinal "$legacy_recall_final" --argjson goRecallFinal "$go_recall_final" '{pass:$pass,http:{head:{legacy:$legacyHead,go:$goHead},support:{legacy:$legacySupport,go:$goSupport},battle:{legacy:$legacyBattle,go:$goBattle},final:{legacy:$legacyFinalStatus,go:$goFinalStatus},supportRecall:{legacy:$legacySupportRecall,go:$goSupportRecall},headRecall:{legacy:$legacyHeadRecall,go:$goHeadRecall}},db:{joined:{legacy:$legacyJoined,go:$goJoined},returning:{legacy:$legacyReturning,go:$goReturning},final:{legacy:$legacyFinal,go:$goFinal},supportRecalled:{legacy:$legacySupportRecalled,go:$goSupportRecalled},allRecalled:{legacy:$legacyAllRecalled,go:$goAllRecalled},recallFinal:{legacy:$legacyRecallFinal,go:$goRecallFinal}}}' > "$REPORT"
[ "$pass" = true ]
printf 'Go/PHP ACS attack differential E2E: PASS\n'
