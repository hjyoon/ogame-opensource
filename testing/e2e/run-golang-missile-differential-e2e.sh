#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_MISSILE_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-missile-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ]
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/missile-differential.XXXXXX")"

attacker_login="$(jq -r '.fleet_lifecycle.attacker.login' "$FIXTURE")"
attacker_id="$(jq -r '.fleet_lifecycle.attacker.player_id' "$FIXTURE")"
attacker_planet="$(jq -r '.fleet_lifecycle.attacker.home_planet_id' "$FIXTURE")"
target_id="$(jq -r '.fleet_lifecycle.target.player_id' "$FIXTURE")"
target_planet="$(jq -r '.fleet_lifecycle.target.home_planet_id' "$FIXTURE")"
players="$attacker_id,$target_id"

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'exec mysql -N -B -r -uroot -p"$MYSQL_ROOT_PASSWORD" uni -e "$1"' sh "$1" 2>/dev/null
}

target_g="$(db_query "SELECT g FROM uni1_planets WHERE planet_id=$target_planet")"
target_s="$(db_query "SELECT s FROM uni1_planets WHERE planet_id=$target_planet")"
target_p="$(db_query "SELECT p FROM uni1_planets WHERE planet_id=$target_planet")"
planet_backup="uni1_e2e_missile_planets_$$"
user_backup="uni1_e2e_missile_users_$$"
rank_backup="uni1_e2e_missile_ranks_$$"
fleet_backup="uni1_e2e_missile_fleet_$$"
queue_backup="uni1_e2e_missile_queue_$$"
message_backup="uni1_e2e_missile_messages_$$"
log_backup="uni1_e2e_missile_logs_$$"

db_query "DROP TABLE IF EXISTS $planet_backup,$user_backup,$rank_backup,$fleet_backup,$queue_backup,$message_backup,$log_backup; CREATE TABLE $planet_backup AS SELECT * FROM uni1_planets WHERE planet_id=$attacker_planet OR (g=$target_g AND s=$target_s AND p=$target_p); CREATE TABLE $user_backup AS SELECT * FROM uni1_users WHERE player_id IN ($players); CREATE TABLE $rank_backup AS SELECT player_id,score1,score2,score3,place1,place2,place3,oldscore1,oldscore2,oldscore3,oldplace1,oldplace2,oldplace3 FROM uni1_users; CREATE TABLE $fleet_backup AS SELECT * FROM uni1_fleet WHERE owner_id IN ($players); CREATE TABLE $queue_backup AS SELECT q.* FROM uni1_queue q JOIN $fleet_backup f ON f.fleet_id=q.sub_id WHERE q.type='Fleet'; CREATE TABLE $message_backup AS SELECT * FROM uni1_messages WHERE owner_id IN ($players); CREATE TABLE $log_backup AS SELECT * FROM uni1_fleetlogs WHERE owner_id IN ($players) OR target_id IN ($players)" >/dev/null

delete_case_state() {
  db_query "DELETE q FROM uni1_queue q LEFT JOIN uni1_fleet f ON f.fleet_id=q.sub_id WHERE q.type='Fleet' AND (q.owner_id IN ($players) OR f.owner_id IN ($players)); DELETE FROM uni1_fleet WHERE owner_id IN ($players); DELETE FROM uni1_messages WHERE owner_id IN ($players); DELETE FROM uni1_fleetlogs WHERE owner_id IN ($players) OR target_id IN ($players); DELETE FROM uni1_planets WHERE g=$target_g AND s=$target_s AND p=$target_p" >/dev/null
}

restore_rows() {
  delete_case_state
  db_query "DELETE FROM uni1_planets WHERE planet_id=$attacker_planet; INSERT INTO uni1_planets SELECT * FROM $planet_backup; INSERT INTO uni1_fleet SELECT * FROM $fleet_backup; INSERT INTO uni1_queue SELECT * FROM $queue_backup; INSERT INTO uni1_messages SELECT * FROM $message_backup; INSERT INTO uni1_fleetlogs SELECT * FROM $log_backup; UPDATE uni1_users u JOIN $user_backup b ON b.player_id=u.player_id SET u.session=b.session,u.private_session=b.private_session,u.lastlogin=b.lastlogin,u.lastclick=b.lastclick,u.ip_addr=b.ip_addr,u.aktplanet=b.aktplanet,u.lang=b.lang,u.\`109\`=b.\`109\`,u.\`111\`=b.\`111\`,u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3,u.oldscore1=b.oldscore1,u.oldscore2=b.oldscore2,u.oldscore3=b.oldscore3,u.oldplace1=b.oldplace1,u.oldplace2=b.oldplace2,u.oldplace3=b.oldplace3; UPDATE uni1_users u JOIN $rank_backup b ON b.player_id=u.player_id SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3,u.oldscore1=b.oldscore1,u.oldscore2=b.oldscore2,u.oldscore3=b.oldscore3,u.oldplace1=b.oldplace1,u.oldplace2=b.oldplace2,u.oldplace3=b.oldplace3" >/dev/null
}

cleanup() {
  restore_rows >/dev/null 2>&1 || true
  db_query "DROP TABLE IF EXISTS $planet_backup,$user_backup,$rank_backup,$fleet_backup,$queue_backup,$message_backup,$log_backup" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

seed_case() {
  name="$1"
  restore_rows
  delete_case_state
  db_query "UPDATE uni1_planets SET \`401\`=0,\`402\`=0,\`403\`=0,\`404\`=0,\`405\`=0,\`406\`=0,\`407\`=0,\`408\`=0,\`502\`=0,\`503\`=0,lastpeek=UNIX_TIMESTAMP()+3600,lastakt=UNIX_TIMESTAMP() WHERE planet_id=$attacker_planet; UPDATE uni1_users SET session='',private_session='',lang='en',\`109\`=0,\`111\`=0 WHERE player_id IN ($players); INSERT INTO uni1_planets SELECT * FROM $planet_backup WHERE planet_id=$target_planet; UPDATE uni1_planets SET name='MissileTarget',\`401\`=0,\`402\`=0,\`403\`=0,\`404\`=0,\`405\`=0,\`406\`=0,\`407\`=0,\`408\`=0,\`502\`=0,\`503\`=0,lastpeek=UNIX_TIMESTAMP()+3600,lastakt=UNIX_TIMESTAMP() WHERE planet_id=$target_planet" >/dev/null
  amount=1
  primary=0
  destination=$target_planet
  case "$name" in
    full)
      amount=3; primary=401
      db_query "UPDATE uni1_planets SET \`401\`=10,\`402\`=5,\`502\`=5 WHERE planet_id=$target_planet" >/dev/null
      ;;
    partial)
      amount=3; primary=401
      db_query "UPDATE uni1_planets SET \`401\`=100,\`402\`=10,\`502\`=2 WHERE planet_id=$target_planet" >/dev/null
      ;;
    plasma)
      amount=1; primary=406
      db_query "UPDATE uni1_planets SET \`406\`=3 WHERE planet_id=$target_planet" >/dev/null
      ;;
    sweep)
      amount=1; primary=0
      db_query "UPDATE uni1_planets SET \`401\`=20,\`402\`=20 WHERE planet_id=$target_planet" >/dev/null
      ;;
    moon)
      amount=2; primary=401
      db_query "UPDATE uni1_planets SET \`502\`=1 WHERE planet_id=$target_planet; INSERT INTO uni1_planets (name,type,g,s,p,owner_id,diameter,temp,fields,maxfields,date,\`700\`,\`701\`,\`702\`,lastpeek,lastakt,gate_until,remove,\`401\`) VALUES ('MissileMoon',0,$target_g,$target_s,$target_p,$target_id,8000,-20,0,1,UNIX_TIMESTAMP(),0,0,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP(),0,0,20); SET @moon=LAST_INSERT_ID()" >/dev/null
      destination="$(db_query "SELECT planet_id FROM uni1_planets WHERE g=$target_g AND s=$target_s AND p=$target_p AND type=0 ORDER BY planet_id DESC LIMIT 1")"
      ;;
  esac
  db_query "INSERT INTO uni1_fleet (owner_id,union_id,fuel,mission,start_planet,target_planet,flight_time,deploy_time,ipm_amount,ipm_target) VALUES ($attacker_id,0,0,20,$attacker_planet,$destination,300,0,$amount,$primary); SET @fleet=LAST_INSERT_ID(); INSERT INTO uni1_queue (owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES ($attacker_id,'Fleet',@fleet,0,0,UNIX_TIMESTAMP()-301,UNIX_TIMESTAMP()-1,1520)" >/dev/null
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
  planets="$(db_query "SELECT JSON_ARRAYAGG(JSON_OBJECT('type',type,'name',name,'ownerId',owner_id,'rocket',\`401\`,'light',\`402\`,'heavy',\`403\`,'gauss',\`404\`,'ion',\`405\`,'plasma',\`406\`,'smallDome',\`407\`,'largeDome',\`408\`,'abm',\`502\`,'ipm',\`503\`)) FROM (SELECT * FROM uni1_planets WHERE planet_id=$attacker_planet OR (g=$target_g AND s=$target_s AND p=$target_p) ORDER BY type,planet_id) p" | jq -c 'sort_by(.type,.name)')"
  messages="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('ownerId',owner_id,'type',pm,'from',msgfrom,'subject',subj,'text',text,'shown',shown))) FROM (SELECT * FROM uni1_messages WHERE owner_id IN ($players) ORDER BY msg_id) m" | jq -c 'sort_by(.ownerId,.type,.subject,.text)')"
  users="$(db_query "SELECT JSON_ARRAYAGG(JSON_OBJECT('id',player_id,'score1',score1,'score2',score2,'score3',score3,'place1',place1,'place2',place2,'place3',place3)) FROM (SELECT * FROM uni1_users WHERE player_id IN ($players) ORDER BY player_id) u" | jq -c 'sort_by(.id)')"
  fleets="$(db_query "SELECT COUNT(*) FROM uni1_fleet WHERE owner_id IN ($players)")"
  queues="$(db_query "SELECT COUNT(*) FROM uni1_queue WHERE type='Fleet' AND owner_id IN ($players)")"
  jq -nc --argjson planets "$planets" --argjson messages "$messages" --argjson users "$users" --arg fleets "$fleets" --arg queues "$queues" '{planets:$planets,messages:$messages,users:$users,fleets:($fleets|tonumber),queues:($queues|tonumber)}'
}

run_side() {
  side="$1"; case_name="$2"
  seed_case "$case_name"
  if [ "$side" = legacy ]; then
    login_legacy
    action_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/legacy-$case_name.body" --cookie "$TMP_DIR/legacy.cookies" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=flotten1&session=$session&cp=$attacker_planet")"
  else
    login_go
    action_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/go-$case_name.body" --cookie "$cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/fleet?session=$session&cp=$attacker_planet")"
  fi
  state="$(capture_state)"
  jq -nc --arg status "$action_status" --argjson state "$state" '{http:$status,state:$state}'
}

results='[]'; pass=true
for case_name in full partial plasma sweep moon; do
  legacy="$(run_side legacy "$case_name")"
  go="$(run_side go "$case_name")"
  case_pass=true
  [ "$legacy" = "$go" ] || case_pass=false
  printf '%s' "$legacy" | jq -e '.http=="200" and .state.fleets==0 and .state.queues==0 and (.state.messages|length)==2 and ([.state.messages[].type]|unique)==[2]' >/dev/null || case_pass=false
  case "$case_name" in
    full) printf '%s' "$legacy" | jq -e '(.state.planets|map(select(.name=="MissileTarget"))[0]) as $p | $p.abm==2 and $p.rocket==10 and $p.light==5' >/dev/null || case_pass=false ;;
    partial) printf '%s' "$legacy" | jq -e '(.state.planets|map(select(.name=="MissileTarget"))[0]) as $p | $p.abm==0 and $p.rocket==40 and $p.light==10' >/dev/null || case_pass=false ;;
    plasma) printf '%s' "$legacy" | jq -e '(.state.planets|map(select(.name=="MissileTarget"))[0].plasma)==2' >/dev/null || case_pass=false ;;
    sweep) printf '%s' "$legacy" | jq -e '(.state.planets|map(select(.name=="MissileTarget"))[0]) as $p | $p.rocket==0 and $p.light==0' >/dev/null || case_pass=false ;;
    moon) printf '%s' "$legacy" | jq -e '(.state.planets|map(select(.name=="MissileTarget"))[0].abm)==0 and (.state.planets|map(select(.name=="MissileMoon"))[0].rocket)==0' >/dev/null || case_pass=false ;;
  esac
  [ "$case_pass" = true ] || pass=false
  results="$(printf '%s' "$results" | jq -c --arg name "$case_name" --argjson pass "$case_pass" --argjson legacy "$legacy" --argjson go "$go" '. + [{name:$name,pass:$pass,legacy:$legacy,go:$go}]')"
done

jq -nc --argjson pass "$pass" --argjson cases "$results" '{pass:$pass,cases:$cases}' > "$REPORT"
[ "$pass" = true ]
printf 'Go/PHP missile differential E2E: PASS\n'
