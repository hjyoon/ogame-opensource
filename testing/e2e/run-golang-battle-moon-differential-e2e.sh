#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_BATTLE_MOON_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-battle-moon-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"
MAX_ATTEMPTS="${OGAME_BATTLE_MOON_MAX_ATTEMPTS:-100}"

[ -f "$FIXTURE" ]
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/battle-moon-differential.XXXXXX")"

attacker_login="$(jq -r '.fleet_lifecycle.attacker.login' "$FIXTURE")"
attacker_id="$(jq -r '.fleet_lifecycle.attacker.player_id' "$FIXTURE")"
attacker_planet="$(jq -r '.fleet_lifecycle.attacker.home_planet_id' "$FIXTURE")"
target_id="$(jq -r '.fleet_lifecycle.target.player_id' "$FIXTURE")"
target_planet="$(jq -r '.fleet_lifecycle.target.home_planet_id' "$FIXTURE")"
players="$attacker_id,$target_id"

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'exec mysql -N -B -r -uroot -p"$MYSQL_ROOT_PASSWORD" uni -e "$1"' sh "$1" 2>/dev/null
}

target_g="$(db_query "SELECT g FROM uni1_planets WHERE planet_id=$target_planet")"
target_s="$(db_query "SELECT s FROM uni1_planets WHERE planet_id=$target_planet")"
target_p="$(db_query "SELECT p FROM uni1_planets WHERE planet_id=$target_planet")"
planet_backup="uni1_e2e_mooncreate_planets_$$"
user_backup="uni1_e2e_mooncreate_users_$$"
rank_backup="uni1_e2e_mooncreate_ranks_$$"
fleet_backup="uni1_e2e_mooncreate_fleet_$$"
queue_backup="uni1_e2e_mooncreate_queue_$$"
message_backup="uni1_e2e_mooncreate_messages_$$"
log_backup="uni1_e2e_mooncreate_logs_$$"
battle_before_max="$(db_query 'SELECT COALESCE(MAX(battle_id),0) FROM uni1_battledata')"

db_query "DROP TABLE IF EXISTS $planet_backup,$user_backup,$rank_backup,$fleet_backup,$queue_backup,$message_backup,$log_backup; CREATE TABLE $planet_backup AS SELECT * FROM uni1_planets WHERE planet_id=$attacker_planet OR (g=$target_g AND s=$target_s AND p=$target_p); CREATE TABLE $user_backup AS SELECT * FROM uni1_users WHERE player_id IN ($players); CREATE TABLE $rank_backup AS SELECT player_id,score1,score2,score3,place1,place2,place3,oldscore1,oldscore2,oldscore3,oldplace1,oldplace2,oldplace3 FROM uni1_users; CREATE TABLE $fleet_backup AS SELECT * FROM uni1_fleet WHERE owner_id IN ($players) OR start_planet IN (SELECT planet_id FROM $planet_backup) OR target_planet IN (SELECT planet_id FROM $planet_backup); CREATE TABLE $queue_backup AS SELECT * FROM uni1_queue WHERE type='Fleet' AND sub_id IN (SELECT fleet_id FROM $fleet_backup); CREATE TABLE $message_backup AS SELECT * FROM uni1_messages WHERE owner_id IN ($players); CREATE TABLE $log_backup AS SELECT * FROM uni1_fleetlogs WHERE owner_id IN ($players) OR target_id IN ($players)" >/dev/null

delete_case_state() {
  db_query "DELETE q FROM uni1_queue q LEFT JOIN uni1_fleet f ON f.fleet_id=q.sub_id WHERE q.type='Fleet' AND (q.owner_id IN ($players) OR f.owner_id IN ($players) OR f.start_planet=$attacker_planet OR f.target_planet IN (SELECT planet_id FROM uni1_planets WHERE g=$target_g AND s=$target_s AND p=$target_p)); DELETE FROM uni1_fleet WHERE owner_id IN ($players) OR start_planet=$attacker_planet OR target_planet IN (SELECT planet_id FROM uni1_planets WHERE g=$target_g AND s=$target_s AND p=$target_p); DELETE FROM uni1_messages WHERE owner_id IN ($players); DELETE FROM uni1_fleetlogs WHERE owner_id IN ($players) OR target_id IN ($players); DELETE FROM uni1_battledata WHERE battle_id>$battle_before_max; DELETE FROM uni1_planets WHERE planet_id=$attacker_planet OR (g=$target_g AND s=$target_s AND p=$target_p)" >/dev/null
}

restore_rows() {
  delete_case_state
  db_query "INSERT INTO uni1_planets SELECT * FROM $planet_backup; INSERT INTO uni1_fleet SELECT * FROM $fleet_backup; INSERT INTO uni1_queue SELECT * FROM $queue_backup; INSERT INTO uni1_messages SELECT * FROM $message_backup; INSERT INTO uni1_fleetlogs SELECT * FROM $log_backup" >/dev/null
}

restore_original() {
  restore_rows >/dev/null 2>&1 || true
  db_query "UPDATE uni1_users u JOIN $user_backup b ON b.player_id=u.player_id SET u.session=b.session,u.private_session=b.private_session,u.lastlogin=b.lastlogin,u.lastclick=b.lastclick,u.ip_addr=b.ip_addr,u.aktplanet=b.aktplanet,u.vacation=b.vacation,u.vacation_until=b.vacation_until,u.banned=b.banned,u.banned_until=b.banned_until,u.noattack=b.noattack,u.noattack_until=b.noattack_until,u.disable=b.disable,u.disable_until=b.disable_until,u.validated=b.validated,u.deact_ip=b.deact_ip,u.\`109\`=b.\`109\`; UPDATE uni1_users u JOIN $rank_backup b ON b.player_id=u.player_id SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3,u.oldscore1=b.oldscore1,u.oldscore2=b.oldscore2,u.oldscore3=b.oldscore3,u.oldplace1=b.oldplace1,u.oldplace2=b.oldplace2,u.oldplace3=b.oldplace3; DROP TABLE IF EXISTS $planet_backup,$user_backup,$rank_backup,$fleet_backup,$queue_backup,$message_backup,$log_backup" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

reset_case() {
  restore_rows
  db_query "DELETE q FROM uni1_queue q JOIN uni1_fleet f ON f.fleet_id=q.sub_id WHERE q.type='Fleet' AND f.owner_id IN ($players); DELETE FROM uni1_fleet WHERE owner_id IN ($players); DELETE FROM uni1_messages WHERE owner_id IN ($players); DELETE FROM uni1_fleetlogs WHERE owner_id IN ($players) OR target_id IN ($players); DELETE FROM uni1_battledata WHERE battle_id>$battle_before_max; DELETE FROM uni1_planets WHERE g=$target_g AND s=$target_s AND p=$target_p AND planet_id<>$target_planet; UPDATE uni1_planets SET \`700\`=0,\`701\`=0,\`702\`=0,\`202\`=0,\`203\`=0,\`204\`=0,\`205\`=0,\`206\`=0,\`207\`=0,\`208\`=0,\`209\`=0,\`210\`=0,\`211\`=0,\`212\`=0,\`213\`=0,\`214\`=0,\`215\`=0,\`401\`=0,\`402\`=0,\`403\`=0,\`404\`=0,\`405\`=0,\`406\`=0,\`407\`=0,\`408\`=0,lastpeek=UNIX_TIMESTAMP(),lastakt=UNIX_TIMESTAMP(),prod1=0,prod2=0,prod3=0,prod4=0,prod12=0,prod212=0 WHERE planet_id IN ($attacker_planet,$target_planet); UPDATE uni1_planets SET \`214\`=1 WHERE planet_id=$target_planet; UPDATE uni1_users u JOIN $rank_backup b ON b.player_id=u.player_id SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3,u.oldscore1=b.oldscore1,u.oldscore2=b.oldscore2,u.oldscore3=b.oldscore3,u.oldplace1=b.oldplace1,u.oldplace2=b.oldplace2,u.oldplace3=b.oldplace3; UPDATE uni1_users u JOIN $user_backup b ON b.player_id=u.player_id SET u.session='',u.private_session='',u.vacation=0,u.vacation_until=0,u.banned=0,u.banned_until=0,u.noattack=0,u.noattack_until=0,u.disable=0,u.disable_until=0,u.validated=1,u.deact_ip=1,u.aktplanet=u.hplanetid,u.\`109\`=b.\`109\` WHERE u.player_id IN ($players); UPDATE uni1_users SET score1=20000000,score2=10 WHERE player_id=$attacker_id; UPDATE uni1_users SET \`109\`=100 WHERE player_id=$target_id; INSERT INTO uni1_fleet (owner_id,union_id,\`700\`,\`701\`,\`702\`,fuel,mission,start_planet,target_planet,flight_time,deploy_time,\`214\`) VALUES ($attacker_id,0,0,0,0,0,1,$attacker_planet,$target_planet,300,0,1); SET @attack=LAST_INSERT_ID(); INSERT INTO uni1_queue (owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES ($attacker_id,'Fleet',@attack,0,0,UNIX_TIMESTAMP()-301,UNIX_TIMESTAMP()-1,1201)" >/dev/null
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
  planets_json="$(db_query "SELECT JSON_ARRAYAGG(JSON_OBJECT('id',IF(type IN (0,10000),0,planet_id),'type',type,'name',name,'ownerId',owner_id,'maxfields',maxfields,'metal',IF(type=1,0,ROUND(\`700\`)),'crystal',IF(type=1,0,ROUND(\`701\`)),'deuterium',IF(type=1,0,ROUND(\`702\`)),'deathstar',\`214\`)) FROM (SELECT * FROM uni1_planets WHERE planet_id=$attacker_planet OR (g=$target_g AND s=$target_s AND p=$target_p) ORDER BY type,planet_id) p" | jq -c 'sort_by(.type,.id)')"
  fleets_json="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('ownerId',owner_id,'mission',mission,'startPlanet',start_planet,'targetPlanet',target_planet,'flightTime',flight_time,'deathstar',\`214\`))) FROM (SELECT * FROM uni1_fleet WHERE owner_id IN ($players) ORDER BY owner_id,mission) f" | jq -c 'sort_by(.ownerId,.mission)')"
  queues_json="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('ownerId',f.owner_id,'mission',f.mission,'duration',q.end-q.start,'prio',q.prio))) FROM uni1_queue q JOIN uni1_fleet f ON f.fleet_id=q.sub_id WHERE q.type='Fleet' AND f.owner_id IN ($players)" | jq -c 'sort_by(.ownerId,.mission)')"
  messages_json="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('ownerId',owner_id,'type',pm,'from',msgfrom,'subject',subj,'text',text,'shown',shown))) FROM (SELECT * FROM uni1_messages WHERE owner_id IN ($players) ORDER BY msg_id) m" | jq -c 'walk(if type=="string" then gsub("bericht=[0-9]+";"bericht=<id>") | gsub("At [0-9]{2}-[0-9]{2} [0-9:]{8}";"At <time>") else . end) | sort_by(.ownerId,.type,.subject,.text)')"
  users_json="$(db_query "SELECT JSON_ARRAYAGG(JSON_OBJECT('id',player_id,'score1',score1,'score2',score2,'score3',score3,'place1',place1,'place2',place2,'place3',place3,'activePlanet',aktplanet)) FROM (SELECT * FROM uni1_users WHERE player_id IN ($players) ORDER BY player_id) u" | jq -c 'sort_by(.id)')"
  battle_json="$(db_query "SELECT JSON_OBJECT('title',title,'report',report) FROM uni1_battledata WHERE battle_id>$battle_before_max ORDER BY battle_id DESC LIMIT 1")"
  [ -n "$battle_json" ] || battle_json=null
  battle_json="$(printf '%s' "$battle_json" | jq -c 'walk(if type=="string" then gsub("bericht=[0-9]+";"bericht=<id>") | gsub("At [0-9]{2}-[0-9]{2} [0-9:]{8}";"At <time>") else . end)')"
  jq -nc --argjson planets "$planets_json" --argjson fleets "$fleets_json" --argjson queues "$queues_json" --argjson messages "$messages_json" --argjson users "$users_json" --argjson battle "$battle_json" '{planets:$planets,fleets:$fleets,queues:$queues,messages:$messages,users:$users,battle:$battle}'
}

moon_is_valid() {
  db_query "SELECT COUNT(*) FROM uni1_planets m JOIN uni1_planets p ON p.planet_id=$target_planet WHERE m.g=$target_g AND m.s=$target_s AND m.p=$target_p AND m.type=0 AND m.owner_id=$target_id AND m.diameter BETWEEN 8366 AND 8944 AND m.temp BETWEEN p.temp-30 AND p.temp-20 AND m.fields=0 AND m.maxfields=1 AND m.\`700\`=0 AND m.\`701\`=0 AND m.\`702\`=0"
}

moon_exists() {
  db_query "SELECT COUNT(*) FROM uni1_planets WHERE g=$target_g AND s=$target_s AND p=$target_p AND type=0"
}

run_side() {
  side="$1"
  attempts=0
  while [ "$attempts" -lt "$MAX_ATTEMPTS" ]; do
    attempts=$((attempts + 1))
    reset_case
    if [ "$side" = legacy ]; then
      login_legacy
      action_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/legacy-action.body" --cookie "$TMP_DIR/legacy.cookies" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=flotten1&session=$session&cp=$attacker_planet")"
    else
      login_go
      action_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/go-action.body" --cookie "$cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/fleet?session=$session&cp=$attacker_planet")"
    fi
    [ "$(moon_exists)" = 1 ] && break
  done
  if [ "$(moon_is_valid)" != 1 ]; then
    db_query "SELECT 'moon',type,owner_id,diameter,temp,fields,maxfields,ROUND(\`700\`),ROUND(\`701\`),ROUND(\`702\`) FROM uni1_planets WHERE g=$target_g AND s=$target_s AND p=$target_p AND type=0; SELECT 'debris',ROUND(\`700\`),ROUND(\`701\`) FROM uni1_planets WHERE g=$target_g AND s=$target_s AND p=$target_p AND type=10000; SELECT 'fleet',mission,\`214\` FROM uni1_fleet WHERE owner_id IN ($players); SELECT 'battle',LEFT(report,240) FROM uni1_battledata WHERE battle_id>$battle_before_max ORDER BY battle_id DESC LIMIT 1" >&2
    return 1
  fi
  state="$(capture_state)"
}

run_side legacy
legacy_status="$action_status"; legacy_attempts="$attempts"; legacy_state="$state"
run_side go
go_status="$action_status"; go_attempts="$attempts"; go_state="$state"

pass=true
[ "$legacy_status" = 200 ] && [ "$go_status" = 200 ] || pass=false
[ "$legacy_state" = "$go_state" ] || pass=false
printf '%s' "$legacy_state" | jq -e --arg attacker "$attacker_id" --arg target "$target_id" --arg targetPlanet "$target_planet" '.fleets==[] and .queues==[] and (.messages|length)==4 and ([.messages[].ownerId]|unique|sort)==([($attacker|tonumber),($target|tonumber)]|sort) and ([.messages[]|select(.text|contains("form a moon around the planet"))]|length)==1 and ([.planets[]|select(.type==0 and .name=="Moon" and .ownerId==($target|tonumber) and .maxfields==1)]|length)==1 and ([.planets[]|select(.type==10000 and .name=="Debris Field" and .metal==1500000 and .crystal==1200000)]|length)==1 and ([.planets[]|select(.id==($targetPlanet|tonumber) and .deathstar==1)]|length)==1' >/dev/null || pass=false

jq -nc --argjson pass "$pass" --arg legacyStatus "$legacy_status" --arg goStatus "$go_status" --arg legacyAttempts "$legacy_attempts" --arg goAttempts "$go_attempts" --argjson legacy "$legacy_state" --argjson go "$go_state" '{pass:$pass,http:{legacy:$legacyStatus,go:$goStatus},attempts:{legacy:($legacyAttempts|tonumber),go:($goAttempts|tonumber)},db:{legacy:$legacy,go:$go}}' > "$REPORT"
[ "$pass" = true ]
printf 'Go/PHP battle-moon differential E2E: PASS\n'
