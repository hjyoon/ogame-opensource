#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_HOLDING_DEFENSE_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-holding-defense-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ]
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/holding-defense-differential.XXXXXX")"

attacker_login="$(jq -r '.fleet_lifecycle.attacker.login' "$FIXTURE")"
attacker_id="$(jq -r '.fleet_lifecycle.attacker.player_id' "$FIXTURE")"
attacker_planet="$(jq -r '.fleet_lifecycle.attacker.home_planet_id' "$FIXTURE")"
target_id="$(jq -r '.fleet_lifecycle.target.player_id' "$FIXTURE")"
target_planet="$(jq -r '.fleet_lifecycle.target.home_planet_id' "$FIXTURE")"
holder_id="$(jq -r '.acs_hold.holder.player_id' "$FIXTURE")"
holder_planet="$(jq -r '.acs_hold.holder.home_planet_id' "$FIXTURE")"
players="$attacker_id,$target_id,$holder_id"
planets="$attacker_planet,$target_planet,$holder_planet"

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'exec mysql -N -B -r -uroot -p"$MYSQL_ROOT_PASSWORD" uni -e "$1"' sh "$1" 2>/dev/null
}

planet_backup="uni1_e2e_holddef_planets_$$"
user_backup="uni1_e2e_holddef_users_$$"
rank_backup="uni1_e2e_holddef_ranks_$$"
fleet_backup="uni1_e2e_holddef_fleet_$$"
queue_backup="uni1_e2e_holddef_queue_$$"
message_backup="uni1_e2e_holddef_messages_$$"
log_backup="uni1_e2e_holddef_logs_$$"
battle_before_max="$(db_query 'SELECT COALESCE(MAX(battle_id),0) FROM uni1_battledata')"
target_g="$(db_query "SELECT g FROM uni1_planets WHERE planet_id=$target_planet")"
target_s="$(db_query "SELECT s FROM uni1_planets WHERE planet_id=$target_planet")"
target_p="$(db_query "SELECT p FROM uni1_planets WHERE planet_id=$target_planet")"

db_query "DROP TABLE IF EXISTS $planet_backup,$user_backup,$rank_backup,$fleet_backup,$queue_backup,$message_backup,$log_backup; CREATE TABLE $planet_backup AS SELECT * FROM uni1_planets WHERE planet_id IN ($planets); CREATE TABLE $user_backup AS SELECT * FROM uni1_users WHERE player_id IN ($players); CREATE TABLE $rank_backup AS SELECT player_id,score1,score2,score3,place1,place2,place3,oldscore1,oldscore2,oldscore3,oldplace1,oldplace2,oldplace3 FROM uni1_users; CREATE TABLE $fleet_backup AS SELECT * FROM uni1_fleet WHERE owner_id IN ($players) OR start_planet IN ($planets) OR target_planet IN ($planets); CREATE TABLE $queue_backup AS SELECT * FROM uni1_queue WHERE type='Fleet' AND (owner_id IN ($players) OR sub_id IN (SELECT fleet_id FROM $fleet_backup)); CREATE TABLE $message_backup AS SELECT * FROM uni1_messages WHERE owner_id IN ($players); CREATE TABLE $log_backup AS SELECT * FROM uni1_fleetlogs WHERE owner_id IN ($players) OR target_id IN ($players)" >/dev/null

delete_case_state() {
  db_query "DELETE q FROM uni1_queue q LEFT JOIN uni1_fleet f ON f.fleet_id=q.sub_id WHERE q.type='Fleet' AND (q.owner_id IN ($players) OR f.owner_id IN ($players) OR f.start_planet IN ($planets) OR f.target_planet IN ($planets)); DELETE FROM uni1_fleet WHERE owner_id IN ($players) OR start_planet IN ($planets) OR target_planet IN ($planets); DELETE FROM uni1_messages WHERE owner_id IN ($players); DELETE FROM uni1_fleetlogs WHERE owner_id IN ($players) OR target_id IN ($players); DELETE FROM uni1_battledata WHERE battle_id>$battle_before_max; DELETE FROM uni1_planets WHERE type=10000 AND g=$target_g AND s=$target_s AND p=$target_p" >/dev/null
}

restore_original() {
  delete_case_state >/dev/null 2>&1 || true
  db_query "UPDATE uni1_users u JOIN $user_backup b ON b.player_id=u.player_id SET u.session=b.session,u.private_session=b.private_session,u.lastlogin=b.lastlogin,u.lastclick=b.lastclick,u.ip_addr=b.ip_addr,u.aktplanet=b.aktplanet,u.vacation=b.vacation,u.vacation_until=b.vacation_until,u.banned=b.banned,u.banned_until=b.banned_until,u.noattack=b.noattack,u.noattack_until=b.noattack_until,u.disable=b.disable,u.disable_until=b.disable_until,u.validated=b.validated,u.deact_ip=b.deact_ip; UPDATE uni1_users u JOIN $rank_backup b ON b.player_id=u.player_id SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3,u.oldscore1=b.oldscore1,u.oldscore2=b.oldscore2,u.oldscore3=b.oldscore3,u.oldplace1=b.oldplace1,u.oldplace2=b.oldplace2,u.oldplace3=b.oldplace3; DELETE FROM uni1_planets WHERE planet_id IN ($planets); INSERT INTO uni1_planets SELECT * FROM $planet_backup; INSERT INTO uni1_fleet SELECT * FROM $fleet_backup; INSERT INTO uni1_queue SELECT * FROM $queue_backup; INSERT INTO uni1_messages SELECT * FROM $message_backup; INSERT INTO uni1_fleetlogs SELECT * FROM $log_backup; DROP TABLE IF EXISTS $planet_backup,$user_backup,$rank_backup,$fleet_backup,$queue_backup,$message_backup,$log_backup" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

reset_case() {
  delete_case_state
  db_query "UPDATE uni1_planets SET \
    \`700\`=0,\`701\`=0,\`702\`=0,\`202\`=0,\`203\`=0,\`204\`=0,\`205\`=0,\`206\`=0,\`207\`=0,\`208\`=0,\`209\`=0,\`210\`=0,\`211\`=0,\`212\`=0,\`213\`=0,\`214\`=0,\`215\`=0,\`401\`=0,\`402\`=0,\`403\`=0,\`404\`=0,\`405\`=0,\`406\`=0,\`407\`=0,\`408\`=0,lastpeek=UNIX_TIMESTAMP(),lastakt=UNIX_TIMESTAMP(),prod1=0,prod2=0,prod3=0,prod4=0,prod12=0,prod212=0 WHERE planet_id IN ($planets); UPDATE uni1_users u JOIN $rank_backup b ON b.player_id=u.player_id SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3,u.oldscore1=b.oldscore1,u.oldscore2=b.oldscore2,u.oldscore3=b.oldscore3,u.oldplace1=b.oldplace1,u.oldplace2=b.oldplace2,u.oldplace3=b.oldplace3; UPDATE uni1_users u JOIN $user_backup b ON b.player_id=u.player_id SET u.session='',u.private_session='',u.vacation=0,u.vacation_until=0,u.banned=0,u.banned_until=0,u.noattack=0,u.noattack_until=0,u.disable=0,u.disable_until=0,u.validated=1,u.deact_ip=1,u.aktplanet=u.hplanetid; INSERT INTO uni1_fleet (owner_id,union_id,\`700\`,\`701\`,\`702\`,fuel,mission,start_planet,target_planet,flight_time,deploy_time,\`202\`) VALUES ($holder_id,0,0,0,0,0,205,$holder_planet,$target_planet,300,3600,1); SET @held=LAST_INSERT_ID(); INSERT INTO uni1_queue (owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES ($holder_id,'Fleet',@held,0,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP()+3600,405); INSERT INTO uni1_fleet (owner_id,union_id,\`700\`,\`701\`,\`702\`,fuel,mission,start_planet,target_planet,flight_time,deploy_time,\`213\`) VALUES ($attacker_id,0,0,0,0,0,1,$attacker_planet,$target_planet,300,0,1); SET @attack=LAST_INSERT_ID(); INSERT INTO uni1_queue (owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES ($attacker_id,'Fleet',@attack,0,0,UNIX_TIMESTAMP()-301,UNIX_TIMESTAMP()-1,1201)" >/dev/null
}

login_legacy() {
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/legacy.headers" --output "$TMP_DIR/legacy.body" --cookie-jar "$TMP_DIR/legacy.cookies" --request POST --data-urlencode "login=$attacker_login" --data-urlencode "pass=$PASSWORD" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/reg/login2.php")"
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/legacy.headers")"
  session="$(printf '%s' "$location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 302 ] && [ -n "$session" ]
}

login_go() {
  payload="$(jq -nc --arg login "$attacker_login" --arg pass "$PASSWORD" '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/go.headers" --output "$TMP_DIR/go.body" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/go.headers" | cut -d';' -f1 | tr -d '\r')"
  redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/go.body")"
  session="$(printf '%s' "$redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 200 ] && [ -n "$cookie" ] && [ -n "$session" ]
}

capture_state() {
  planets_json="$(db_query "SELECT JSON_ARRAYAGG(JSON_OBJECT('id',planet_id,'metal',ROUND(\`700\`),'crystal',ROUND(\`701\`),'deuterium',ROUND(\`702\`),'smallCargo',\`202\`,'destroyer',\`213\`)) FROM (SELECT * FROM uni1_planets WHERE planet_id IN ($planets) ORDER BY planet_id) p" | jq -c 'sort_by(.id)')"
  fleets_json="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('ownerId',owner_id,'mission',mission,'startPlanet',start_planet,'targetPlanet',target_planet,'flightTime',flight_time,'deployTime',deploy_time,'smallCargo',\`202\`,'destroyer',\`213\`))) FROM (SELECT * FROM uni1_fleet WHERE owner_id IN ($players) ORDER BY owner_id,mission) f" | jq -c 'sort_by(.ownerId,.mission)')"
  queues_json="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('ownerId',f.owner_id,'mission',f.mission,'duration',q.end-q.start,'prio',q.prio))) FROM uni1_queue q JOIN uni1_fleet f ON f.fleet_id=q.sub_id WHERE q.type='Fleet' AND f.owner_id IN ($players)" | jq -c 'sort_by(.ownerId,.mission)')"
  messages_json="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('ownerId',owner_id,'type',pm,'from',msgfrom,'subject',subj,'text',text,'shown',shown))) FROM (SELECT * FROM uni1_messages WHERE owner_id IN ($players) ORDER BY msg_id) m" | jq -c 'walk(if type=="string" then gsub("bericht=[0-9]+";"bericht=<id>") | gsub("At [0-9]{2}-[0-9]{2} [0-9:]{8}";"At <time>") else . end) | sort_by(.ownerId,.subject,.text)')"
  logs_json="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('ownerId',owner_id,'targetId',target_id,'mission',mission,'duration',end-start,'flightTime',flight_time,'smallCargo',\`202\`,'destroyer',\`213\`))) FROM (SELECT * FROM uni1_fleetlogs WHERE owner_id IN ($players) OR target_id IN ($players) ORDER BY log_id) l" | jq -c 'sort_by(.ownerId,.mission,.duration)')"
  users_json="$(db_query "SELECT JSON_ARRAYAGG(JSON_OBJECT('id',player_id,'score1',score1,'score2',score2,'score3',score3)) FROM (SELECT * FROM uni1_users WHERE player_id IN ($players) ORDER BY player_id) u" | jq -c 'sort_by(.id)')"
  battle_json="$(db_query "SELECT JSON_OBJECT('title',title,'report',report) FROM uni1_battledata WHERE battle_id>$battle_before_max ORDER BY battle_id DESC LIMIT 1")"
  [ -n "$battle_json" ] || battle_json=null
  battle_json="$(printf '%s' "$battle_json" | jq -c 'walk(if type=="string" then gsub("bericht=[0-9]+";"bericht=<id>") | gsub("At [0-9]{2}-[0-9]{2} [0-9:]{8}";"At <time>") else . end)')"
  debris_json="$(db_query "SELECT JSON_OBJECT('metal',ROUND(\`700\`),'crystal',ROUND(\`701\`)) FROM uni1_planets WHERE type=10000 AND g=$target_g AND s=$target_s AND p=$target_p LIMIT 1")"
  [ -n "$debris_json" ] || debris_json=null
  jq -nc --argjson planets "$planets_json" --argjson fleets "$fleets_json" --argjson queues "$queues_json" --argjson messages "$messages_json" --argjson logs "$logs_json" --argjson users "$users_json" --argjson battle "$battle_json" --argjson debris "$debris_json" '{planets:$planets,fleets:$fleets,queues:$queues,messages:$messages,fleetLogs:$logs,users:$users,battle:$battle,debris:$debris}'
}

force_return_due() {
  db_query "UPDATE uni1_queue q JOIN uni1_fleet f ON f.fleet_id=q.sub_id SET q.start=UNIX_TIMESTAMP()-301,q.end=UNIX_TIMESTAMP()-1 WHERE q.type='Fleet' AND f.owner_id=$attacker_id AND f.mission=101" >/dev/null
}

run_side() {
  side="$1"
  reset_case
  if [ "$side" = legacy ]; then
    login_legacy
    battle_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/legacy-battle.body" --cookie "$TMP_DIR/legacy.cookies" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=flotten1&session=$session&cp=$attacker_planet")"
  else
    login_go
    battle_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/go-battle.body" --cookie "$cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/fleet?session=$session&cp=$attacker_planet")"
  fi
  battle="$(capture_state)"
  force_return_due
  if [ "$side" = legacy ]; then
    final_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/legacy-final.body" --cookie "$TMP_DIR/legacy.cookies" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=flotten1&session=$session&cp=$attacker_planet")"
  else
    final_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/go-final.body" --cookie "$cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/fleet?session=$session&cp=$attacker_planet")"
  fi
  final="$(capture_state)"
}

run_side legacy
legacy_battle_status="$battle_status"; legacy_final_status="$final_status"; legacy_battle="$battle"; legacy_final="$final"
run_side go
go_battle_status="$battle_status"; go_final_status="$final_status"; go_battle="$battle"; go_final="$final"

pass=true
[ "$legacy_battle_status" = 200 ] && [ "$go_battle_status" = 200 ] || pass=false
[ "$legacy_final_status" = 200 ] && [ "$go_final_status" = 200 ] || pass=false
[ "$legacy_battle" = "$go_battle" ] || pass=false
[ "$legacy_final" = "$go_final" ] || pass=false
printf '%s' "$legacy_battle" | jq -e --arg attacker "$attacker_id" --arg target "$target_id" --arg holder "$holder_id" '.battle.report!="" and .debris.metal==600 and .debris.crystal==600 and (.messages|length)==6 and ([.messages[].ownerId]|unique|sort)==([($attacker|tonumber),($target|tonumber),($holder|tonumber)]|sort) and (.fleets|length)==1 and .fleets[0].ownerId==($attacker|tonumber) and .fleets[0].mission==101' >/dev/null || pass=false
printf '%s' "$legacy_final" | jq -e --arg attackerPlanet "$attacker_planet" '.fleets==[] and .queues==[] and (.messages|length)==7 and ([.planets[]|select(.id==($attackerPlanet|tonumber))|.destroyer]|first)==1' >/dev/null || pass=false

jq -nc --argjson pass "$pass" --arg legacyBattleStatus "$legacy_battle_status" --arg goBattleStatus "$go_battle_status" --arg legacyFinalStatus "$legacy_final_status" --arg goFinalStatus "$go_final_status" --argjson legacyBattle "$legacy_battle" --argjson goBattle "$go_battle" --argjson legacyFinal "$legacy_final" --argjson goFinal "$go_final" '{pass:$pass,http:{battle:{legacy:$legacyBattleStatus,go:$goBattleStatus},final:{legacy:$legacyFinalStatus,go:$goFinalStatus}},db:{battle:{legacy:$legacyBattle,go:$goBattle},final:{legacy:$legacyFinal,go:$goFinal}}}' > "$REPORT"
[ "$pass" = true ]
printf 'Go/PHP holding-defense differential E2E: PASS\n'
