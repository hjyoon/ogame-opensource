#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_COLONIZATION_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-colonization-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ]
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/colonization-differential.XXXXXX")"

login="$(jq -r '.fleet_lifecycle.attacker.login' "$FIXTURE")"
player_id="$(jq -r '.fleet_lifecycle.attacker.player_id' "$FIXTURE")"
home_planet="$(jq -r '.fleet_lifecycle.attacker.home_planet_id' "$FIXTURE")"

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'exec mysql -N -B -r -uroot -p"$MYSQL_ROOT_PASSWORD" uni -e "$1"' sh "$1" 2>/dev/null
}

target_g="$(db_query "SELECT g FROM uni1_planets WHERE planet_id=$home_planet")"
target_s="$(db_query "SELECT s FROM uni1_planets WHERE planet_id=$home_planet")"
target_p=15
while [ "$(db_query "SELECT COUNT(*) FROM uni1_planets WHERE g=$target_g AND s=$target_s AND p=$target_p")" != 0 ]; do
  target_s=$((target_s + 1))
  if [ "$target_s" -gt 499 ]; then
    target_s=1
    target_g=$((target_g + 1))
  fi
done

planet_backup="uni1_e2e_colony_planets_$$"
user_backup="uni1_e2e_colony_user_$$"
rank_backup="uni1_e2e_colony_ranks_$$"
fleet_backup="uni1_e2e_colony_fleet_$$"
queue_backup="uni1_e2e_colony_queue_$$"
message_backup="uni1_e2e_colony_messages_$$"
log_backup="uni1_e2e_colony_logs_$$"

db_query "DROP TABLE IF EXISTS $planet_backup,$user_backup,$rank_backup,$fleet_backup,$queue_backup,$message_backup,$log_backup; CREATE TABLE $planet_backup AS SELECT * FROM uni1_planets WHERE planet_id=$home_planet; CREATE TABLE $user_backup AS SELECT * FROM uni1_users WHERE player_id=$player_id; CREATE TABLE $rank_backup AS SELECT player_id,score1,score2,score3,place1,place2,place3,oldscore1,oldscore2,oldscore3,oldplace1,oldplace2,oldplace3 FROM uni1_users; CREATE TABLE $fleet_backup AS SELECT * FROM uni1_fleet WHERE owner_id=$player_id; CREATE TABLE $queue_backup AS SELECT q.* FROM uni1_queue q JOIN $fleet_backup f ON f.fleet_id=q.sub_id WHERE q.type='Fleet'; CREATE TABLE $message_backup AS SELECT * FROM uni1_messages WHERE owner_id=$player_id; CREATE TABLE $log_backup AS SELECT * FROM uni1_fleetlogs WHERE owner_id=$player_id" >/dev/null

delete_case_state() {
  db_query "DELETE q FROM uni1_queue q LEFT JOIN uni1_fleet f ON f.fleet_id=q.sub_id WHERE q.type='Fleet' AND (q.owner_id=$player_id OR f.owner_id=$player_id); DELETE FROM uni1_fleet WHERE owner_id=$player_id; DELETE FROM uni1_messages WHERE owner_id=$player_id; DELETE FROM uni1_fleetlogs WHERE owner_id=$player_id; DELETE FROM uni1_planets WHERE g=$target_g AND s=$target_s AND p=$target_p; DELETE FROM uni1_planets WHERE owner_id=$player_id AND name='E2E capacity'" >/dev/null
}

restore_rows() {
  delete_case_state
  db_query "DELETE FROM uni1_planets WHERE planet_id=$home_planet; INSERT INTO uni1_planets SELECT * FROM $planet_backup; INSERT INTO uni1_fleet SELECT * FROM $fleet_backup; INSERT INTO uni1_queue SELECT * FROM $queue_backup; INSERT INTO uni1_messages SELECT * FROM $message_backup; INSERT INTO uni1_fleetlogs SELECT * FROM $log_backup; UPDATE uni1_users u JOIN $user_backup b ON b.player_id=u.player_id SET u.session=b.session,u.private_session=b.private_session,u.lastlogin=b.lastlogin,u.lastclick=b.lastclick,u.ip_addr=b.ip_addr,u.aktplanet=b.aktplanet,u.vacation=b.vacation,u.vacation_until=b.vacation_until,u.banned=b.banned,u.banned_until=b.banned_until,u.noattack=b.noattack,u.noattack_until=b.noattack_until,u.disable=b.disable,u.disable_until=b.disable_until,u.validated=b.validated,u.deact_ip=b.deact_ip,u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3,u.oldscore1=b.oldscore1,u.oldscore2=b.oldscore2,u.oldscore3=b.oldscore3,u.oldplace1=b.oldplace1,u.oldplace2=b.oldplace2,u.oldplace3=b.oldplace3; UPDATE uni1_users u JOIN $rank_backup b ON b.player_id=u.player_id SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3,u.oldscore1=b.oldscore1,u.oldscore2=b.oldscore2,u.oldscore3=b.oldscore3,u.oldplace1=b.oldplace1,u.oldplace2=b.oldplace2,u.oldplace3=b.oldplace3" >/dev/null
}

restore_original() {
  restore_rows >/dev/null 2>&1 || true
  db_query "DROP TABLE IF EXISTS $planet_backup,$user_backup,$rank_backup,$fleet_backup,$queue_backup,$message_backup,$log_backup" >/dev/null 2>&1 || true
}

cleanup() {
  restore_original
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

seed_case() {
  name="$1"
  restore_rows
  delete_case_state
  db_query "UPDATE uni1_planets SET \`700\`=0,\`701\`=0,\`702\`=0,\`202\`=0,\`208\`=0,lastpeek=UNIX_TIMESTAMP()+3600,lastakt=UNIX_TIMESTAMP(),prod1=0,prod2=0,prod3=0,prod4=0,prod12=0,prod212=0 WHERE planet_id=$home_planet; UPDATE uni1_users SET session='',private_session='',vacation=0,vacation_until=0,banned=0,banned_until=0,noattack=0,noattack_until=0,disable=0,disable_until=0,validated=1,deact_ip=1,aktplanet=hplanetid,score1=1000000,score2=10 WHERE player_id=$player_id" >/dev/null
  if [ "$name" = occupied ]; then
    db_query "INSERT INTO uni1_planets (name,type,g,s,p,owner_id,diameter,temp,fields,maxfields,date,\`700\`,\`701\`,\`702\`,lastpeek,lastakt,gate_until,remove) VALUES ('Occupied',1,$target_g,$target_s,$target_p,99999,12800,0,0,163,UNIX_TIMESTAMP(),0,0,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP(),0,0)" >/dev/null
  fi
  if [ "$name" = maximum ]; then
    db_query "INSERT INTO uni1_planets (name,type,g,s,p,owner_id,diameter,temp,fields,maxfields,date,\`700\`,\`701\`,\`702\`,lastpeek,lastakt,gate_until,remove) VALUES ('E2E capacity',1,9,490,1,$player_id,12800,0,0,163,UNIX_TIMESTAMP(),0,0,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP(),0,0),('E2E capacity',1,9,490,2,$player_id,12800,0,0,163,UNIX_TIMESTAMP(),0,0,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP(),0,0),('E2E capacity',1,9,490,3,$player_id,12800,0,0,163,UNIX_TIMESTAMP(),0,0,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP(),0,0),('E2E capacity',1,9,490,4,$player_id,12800,0,0,163,UNIX_TIMESTAMP(),0,0,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP(),0,0),('E2E capacity',1,9,490,5,$player_id,12800,0,0,163,UNIX_TIMESTAMP(),0,0,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP(),0,0),('E2E capacity',1,9,490,6,$player_id,12800,0,0,163,UNIX_TIMESTAMP(),0,0,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP(),0,0),('E2E capacity',1,9,490,7,$player_id,12800,0,0,163,UNIX_TIMESTAMP(),0,0,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP(),0,0),('E2E capacity',1,9,490,8,$player_id,12800,0,0,163,UNIX_TIMESTAMP(),0,0,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP(),0,0)" >/dev/null
  fi
  colony_ships=1
  small_cargo=2
  [ "$name" != consumed ] || small_cargo=0
  db_query "INSERT INTO uni1_planets (name,type,g,s,p,owner_id,diameter,temp,fields,maxfields,date,\`700\`,\`701\`,\`702\`,lastpeek,lastakt,gate_until,remove) VALUES ('Planet',10002,$target_g,$target_s,$target_p,99999,0,0,0,0,UNIX_TIMESTAMP(),0,0,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP(),0,0); SET @phantom=LAST_INSERT_ID(); INSERT INTO uni1_fleet (owner_id,union_id,\`700\`,\`701\`,\`702\`,fuel,mission,start_planet,target_planet,flight_time,deploy_time,\`202\`,\`208\`) VALUES ($player_id,0,100,200,300,100,7,$home_planet,@phantom,300,0,$small_cargo,$colony_ships); SET @fleet=LAST_INSERT_ID(); INSERT INTO uni1_queue (owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES ($player_id,'Fleet',@fleet,0,0,UNIX_TIMESTAMP()-301,UNIX_TIMESTAMP()-1,1207)" >/dev/null
}

login_legacy() {
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/legacy.headers" --output "$TMP_DIR/legacy.body" --cookie-jar "$TMP_DIR/legacy.cookies" --request POST --data-urlencode "login=$login" --data-urlencode "pass=$PASSWORD" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/reg/login2.php")"
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/legacy.headers")"
  session="$(printf '%s' "$location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 302 ] && [ -n "$session" ]
}

login_go() {
  payload="$(jq -nc --arg login "$login" --arg pass "$PASSWORD" '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/go.headers" --output "$TMP_DIR/go.body" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/go.headers" | cut -d';' -f1 | tr -d '\r')"
  redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/go.body")"
  session="$(printf '%s' "$redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 200 ] && [ -n "$cookie" ] && [ -n "$session" ]
}

trigger() {
  side="$1"
  phase="$2"
  if [ "$side" = legacy ]; then
    curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$phase.body" --cookie "$TMP_DIR/legacy.cookies" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=flotten1&session=$session&cp=$home_planet"
  else
    curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$phase.body" --cookie "$cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/fleet?session=$session&cp=$home_planet"
  fi
}

capture_state() {
  planets="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('type',type,'name',name,'ownerId',owner_id,'fields',fields,'maxfields',IF(type=1 AND owner_id=$player_id AND g=$target_g AND s=$target_s AND p=$target_p,0,maxfields),'metal',ROUND(\`700\`),'crystal',ROUND(\`701\`),'deuterium',ROUND(\`702\`),'smallCargo',\`202\`,'colonyShip',\`208\`))) FROM (SELECT * FROM uni1_planets WHERE planet_id=$home_planet OR (g=$target_g AND s=$target_s AND p=$target_p) ORDER BY type,name,planet_id) p" | jq -c 'sort_by(.type,.name)')"
  fleets="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('mission',mission,'startPlanet',start_planet,'targetPlanet',IF(target_planet IN (SELECT planet_id FROM uni1_planets WHERE g=$target_g AND s=$target_s AND p=$target_p),0,target_planet),'flightTime',flight_time,'fuel',fuel,'metal',ROUND(\`700\`),'crystal',ROUND(\`701\`),'deuterium',ROUND(\`702\`),'smallCargo',\`202\`,'colonyShip',\`208\`))) FROM (SELECT * FROM uni1_fleet WHERE owner_id=$player_id ORDER BY mission,fleet_id) f" | jq -c 'sort_by(.mission,.targetPlanet)')"
  queues="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('mission',f.mission,'duration',q.end-q.start,'prio',q.prio))) FROM uni1_queue q JOIN uni1_fleet f ON f.fleet_id=q.sub_id WHERE q.type='Fleet' AND f.owner_id=$player_id" | jq -c 'sort_by(.mission)')"
  messages="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('type',pm,'from',msgfrom,'subject',subj,'text',text,'shown',shown))) FROM (SELECT * FROM uni1_messages WHERE owner_id=$player_id ORDER BY msg_id) m" | jq -c 'sort_by(.type,.subject,.text)')"
  logs="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('targetId',target_id,'mission',mission,'flightTime',flight_time,'originG',origin_g,'originS',origin_s,'originP',origin_p,'originType',origin_type,'targetG',target_g,'targetS',target_s,'targetP',target_p,'targetType',target_type,'fuel',fuel,'metal',ROUND(\`700\`),'crystal',ROUND(\`701\`),'deuterium',ROUND(\`702\`),'smallCargo',\`202\`,'colonyShip',\`208\`))) FROM (SELECT * FROM uni1_fleetlogs WHERE owner_id=$player_id ORDER BY mission,log_id) l" | jq -c 'sort_by(.mission,.targetType)')"
  user="$(db_query "SELECT JSON_OBJECT('score1',score1,'score2',score2,'score3',score3,'place1',place1,'place2',place2,'place3',place3,'activePlanet',aktplanet) FROM uni1_users WHERE player_id=$player_id" | jq -c '.')"
  jq -nc --argjson planets "$planets" --argjson fleets "$fleets" --argjson queues "$queues" --argjson messages "$messages" --argjson logs "$logs" --argjson user "$user" '{planets:$planets,fleets:$fleets,queues:$queues,messages:$messages,logs:$logs,user:$user}'
}

valid_random_colony() {
  db_query "SELECT COUNT(*) FROM uni1_planets WHERE g=$target_g AND s=$target_s AND p=$target_p AND type=1 AND owner_id=$player_id AND name='Colony' AND diameter BETWEEN 4800 AND 14400 AND MOD(diameter,96)=0 AND temp BETWEEN -90 AND -81 AND fields=0 AND maxfields=FLOOR(POW(diameter/1000,2)) AND ROUND(\`700\`)=500 AND ROUND(\`701\`)=500 AND ROUND(\`702\`)=0"
}

run_side() {
  side="$1"
  case_name="$2"
  seed_case "$case_name"
  if [ "$side" = legacy ]; then login_legacy; else login_go; fi
  arrival_status="$(trigger "$side" arrival)"
  random_valid=true
  if [ "$case_name" = success ] || [ "$case_name" = consumed ]; then
    [ "$(valid_random_colony)" = 1 ] || random_valid=false
  fi
  arrival_state="$(capture_state)"
  if [ "$(db_query "SELECT COUNT(*) FROM uni1_fleet WHERE owner_id=$player_id")" -gt 0 ]; then
    db_query "UPDATE uni1_planets SET \`700\`=500,\`701\`=500,\`702\`=0,lastpeek=UNIX_TIMESTAMP()-301 WHERE g=$target_g AND s=$target_s AND p=$target_p AND type=1 AND owner_id=$player_id; UPDATE uni1_queue q JOIN uni1_fleet f ON f.fleet_id=q.sub_id SET q.start=UNIX_TIMESTAMP()-301,q.end=UNIX_TIMESTAMP()-1 WHERE q.type='Fleet' AND f.owner_id=$player_id" >/dev/null
    return_status="$(trigger "$side" return)"
  else
    return_status=none
  fi
  final_state="$(capture_state)"
  jq -nc --arg status "$arrival_status" --arg returnStatus "$return_status" --argjson randomValid "$random_valid" --argjson arrival "$arrival_state" --argjson final "$final_state" '{http:{arrival:$status,return:$returnStatus},randomColonyValid:$randomValid,arrival:$arrival,final:$final}'
}

results='[]'
pass=true
for case_name in success consumed occupied maximum; do
  legacy="$(run_side legacy "$case_name")"
  go="$(run_side go "$case_name")"
  case_pass=true
  [ "$legacy" = "$go" ] || case_pass=false
  printf '%s' "$legacy" | jq -e '.http.arrival=="200" and (.http.return=="200" or .http.return=="none") and .randomColonyValid==true' >/dev/null || case_pass=false
  if [ "$case_name" = success ]; then
    printf '%s' "$legacy" | jq -e '(.arrival.fleets|length)==1 and .arrival.fleets[0].mission==107 and .arrival.fleets[0].colonyShip==0 and .arrival.fleets[0].smallCargo==2 and (.arrival.messages|map(select(.subject=="Settlers\u0027 report"))|length)==1 and .final.fleets==[] and .final.queues==[] and (.final.messages|length)==2' >/dev/null || case_pass=false
  elif [ "$case_name" = consumed ]; then
    printf '%s' "$legacy" | jq -e '.arrival.fleets==[] and .arrival.queues==[] and .arrival.user.score1==960000 and .arrival.user.score2==9 and (.arrival.messages|length)==1 and .final==.arrival' >/dev/null || case_pass=false
  elif [ "$case_name" = occupied ]; then
    printf '%s' "$legacy" | jq -e '.arrival.fleets[0].mission==107 and .arrival.fleets[0].colonyShip==1 and (.arrival.planets|map(select(.type==10002))|length)==1 and (.arrival.messages[0].text|contains("finds no planet suitable")) and .final.fleets==[] and (.final.planets|map(select(.type==10002))|length)==0' >/dev/null || case_pass=false
  else
    printf '%s' "$legacy" | jq -e '.arrival.fleets[0].mission==107 and .arrival.fleets[0].colonyShip==1 and (.arrival.planets|map(select(.type==10004 and .name=="Planet abandoned"))|length)==1 and (.arrival.messages[0].text|contains("empire becomes too large")) and .final.fleets==[]' >/dev/null || case_pass=false
  fi
  [ "$case_pass" = true ] || pass=false
  results="$(printf '%s' "$results" | jq -c --arg name "$case_name" --argjson pass "$case_pass" --argjson legacy "$legacy" --argjson go "$go" '. + [{name:$name,pass:$pass,legacy:$legacy,go:$go}]')"
done

jq -nc --argjson pass "$pass" --argjson cases "$results" '{pass:$pass,cases:$cases}' > "$REPORT"
[ "$pass" = true ]
printf 'Go/PHP colonization differential E2E: PASS\n'
