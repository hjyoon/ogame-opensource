#!/bin/sh
set -eu
[ "${OGAME_E2E_TRACE:-0}" != "1" ] && [ "${1:-}" != "--trace" ] || set -x

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_FLEET_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-fleet-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"
RECALL_ELAPSED_SECONDS="${OGAME_FLEET_RECALL_ELAPSED_SECONDS:-600}"
FLEET_CASES="${OGAME_FLEET_DIFFERENTIAL_CASES:-transport recall deploy recycle spy attack attack-defense attack-guarded-win attack-defense-repair acs-hold expedition}"

case "$RECALL_ELAPSED_SECONDS" in
  ''|*[!0-9]*) printf 'OGAME_FLEET_RECALL_ELAPSED_SECONDS must be a positive integer.\n' >&2; exit 1 ;;
  0) printf 'OGAME_FLEET_RECALL_ELAPSED_SECONDS must be greater than zero.\n' >&2; exit 1 ;;
esac

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/fleet-differential.XXXXXX")"

login="$(jq -r '.fleet_lifecycle.attacker.login // empty' "$FIXTURE")"
player_id="$(jq -r '.fleet_lifecycle.attacker.player_id // 0' "$FIXTURE")"
origin_id="$(jq -r '.fleet_lifecycle.attacker.home_planet_id // 0' "$FIXTURE")"
target_player_id="$(jq -r '.fleet_lifecycle.target.player_id // 0' "$FIXTURE")"
transport_target_id="$(jq -r '.fleet_lifecycle.target.home_planet_id // 0' "$FIXTURE")"
deploy_target_id="$(jq -r '.fleet_lifecycle.deploy_planet_id // 0' "$FIXTURE")"
[ -n "$login" ] && [ "$player_id" -gt 0 ] && [ "$origin_id" -gt 0 ]
[ "$target_player_id" -gt 0 ] && [ "$transport_target_id" -gt 0 ] && [ "$deploy_target_id" -gt 0 ]

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'exec mysql -N -B -r -uroot -p"$MYSQL_ROOT_PASSWORD" uni -e "$1"' sh "$1" 2>/dev/null
}

players="$player_id,$target_player_id"
planets="$origin_id,$transport_target_id,$deploy_target_id"
planet_backup="uni1_e2e_fleetdiff_planets_$$"
user_backup="uni1_e2e_fleetdiff_users_$$"
rank_backup="uni1_e2e_fleetdiff_ranks_$$"
fleet_backup="uni1_e2e_fleetdiff_fleet_$$"
queue_backup="uni1_e2e_fleetdiff_queue_$$"
message_backup="uni1_e2e_fleetdiff_messages_$$"
fleetlog_backup="uni1_e2e_fleetdiff_logs_$$"
debris_backup="uni1_e2e_fleetdiff_debris_$$"
battle_before_max="$(db_query 'SELECT COALESCE(MAX(battle_id),0) FROM uni1_battledata')"
attack_g="$(db_query "SELECT g FROM uni1_planets WHERE planet_id=$transport_target_id")"
attack_s="$(db_query "SELECT s FROM uni1_planets WHERE planet_id=$transport_target_id")"
attack_p="$(db_query "SELECT p FROM uni1_planets WHERE planet_id=$transport_target_id")"
uni_defrepair="$(db_query 'SELECT defrepair FROM uni1_uni LIMIT 1')"
uni_defrepair_delta="$(db_query 'SELECT defrepair_delta FROM uni1_uni LIMIT 1')"
original_uni_fspeed="$(db_query 'SELECT fspeed FROM uni1_uni LIMIT 1')"
uni_fspeed="${OGAME_FLEET_DIFFERENTIAL_FLEET_SPEED:-$original_uni_fspeed}"
case "$uni_fspeed" in
  ''|*[!0-9]*) printf 'OGAME_FLEET_DIFFERENTIAL_FLEET_SPEED must be a positive integer.\n' >&2; exit 1 ;;
  0) printf 'OGAME_FLEET_DIFFERENTIAL_FLEET_SPEED must be greater than zero.\n' >&2; exit 1 ;;
esac
expedition_target_id="$(db_query 'SELECT planet_id FROM uni1_planets WHERE type=20000 AND owner_id=99999 ORDER BY planet_id DESC LIMIT 1')"
[ "$expedition_target_id" -gt 0 ]
planets="$planets,$expedition_target_id"
expedition_hold_seconds=$(( (3600 + uni_fspeed / 2) / uni_fspeed ))
[ "$expedition_hold_seconds" -gt 0 ]
buddy_marker="golang-fleet-differential-hold-$$"

db_query "DROP TABLE IF EXISTS $planet_backup,$user_backup,$rank_backup,$fleet_backup,$queue_backup,$message_backup,$fleetlog_backup,$debris_backup; CREATE TABLE $planet_backup AS SELECT * FROM uni1_planets WHERE planet_id IN ($planets); CREATE TABLE $user_backup AS SELECT * FROM uni1_users WHERE player_id IN ($players); CREATE TABLE $rank_backup AS SELECT player_id,score1,score2,score3,place1,place2,place3,oldscore1,oldscore2,oldscore3,oldplace1,oldplace2,oldplace3 FROM uni1_users; CREATE TABLE $fleet_backup AS SELECT * FROM uni1_fleet WHERE owner_id IN ($players) OR start_planet IN ($planets) OR target_planet IN ($planets); CREATE TABLE $queue_backup AS SELECT * FROM uni1_queue WHERE type='Fleet' AND (owner_id IN ($players) OR sub_id IN (SELECT fleet_id FROM $fleet_backup)); CREATE TABLE $message_backup AS SELECT * FROM uni1_messages WHERE owner_id IN ($players); CREATE TABLE $fleetlog_backup AS SELECT * FROM uni1_fleetlogs WHERE owner_id IN ($players) OR target_id IN ($players); CREATE TABLE $debris_backup AS SELECT * FROM uni1_planets WHERE type=10000 AND g=$attack_g AND s=$attack_s AND p=$attack_p" >/dev/null

restore_original() {
  db_query "DELETE FROM uni1_buddy WHERE text='$buddy_marker'; DELETE FROM uni1_queue WHERE type='Fleet' AND (owner_id IN ($players) OR sub_id IN (SELECT fleet_id FROM uni1_fleet WHERE owner_id IN ($players) OR start_planet IN ($planets) OR target_planet IN ($planets))); DELETE FROM uni1_fleet WHERE owner_id IN ($players) OR start_planet IN ($planets) OR target_planet IN ($planets); DELETE FROM uni1_messages WHERE owner_id IN ($players); DELETE FROM uni1_fleetlogs WHERE owner_id IN ($players) OR target_id IN ($players); DELETE FROM uni1_battledata WHERE battle_id > $battle_before_max; DELETE FROM uni1_planets WHERE type=10000 AND g=$attack_g AND s=$attack_s AND p=$attack_p; UPDATE uni1_uni SET defrepair=$uni_defrepair,defrepair_delta=$uni_defrepair_delta,fspeed=$original_uni_fspeed; UPDATE uni1_users u JOIN $user_backup b ON b.player_id=u.player_id SET u.session=b.session,u.private_session=b.private_session,u.lastlogin=b.lastlogin,u.lastclick=b.lastclick,u.ip_addr=b.ip_addr,u.aktplanet=b.aktplanet,u.vacation=b.vacation,u.vacation_until=b.vacation_until,u.banned=b.banned,u.banned_until=b.banned_until,u.noattack=b.noattack,u.noattack_until=b.noattack_until,u.disable=b.disable,u.disable_until=b.disable_until,u.validated=b.validated,u.deact_ip=b.deact_ip,u.lang=b.lang,u.\`106\`=b.\`106\`,u.\`124\`=b.\`124\`,u.tec_until=b.tec_until; UPDATE uni1_users u JOIN $rank_backup b ON b.player_id=u.player_id SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3,u.oldscore1=b.oldscore1,u.oldscore2=b.oldscore2,u.oldscore3=b.oldscore3,u.oldplace1=b.oldplace1,u.oldplace2=b.oldplace2,u.oldplace3=b.oldplace3; UPDATE uni1_planets p JOIN $planet_backup b ON b.planet_id=p.planet_id SET p.owner_id=b.owner_id,p.type=b.type,p.name=b.name,p.\`700\`=b.\`700\`,p.\`701\`=b.\`701\`,p.\`702\`=b.\`702\`,p.\`202\`=b.\`202\`,p.\`203\`=b.\`203\`,p.\`204\`=b.\`204\`,p.\`205\`=b.\`205\`,p.\`206\`=b.\`206\`,p.\`207\`=b.\`207\`,p.\`208\`=b.\`208\`,p.\`209\`=b.\`209\`,p.\`210\`=b.\`210\`,p.\`211\`=b.\`211\`,p.\`212\`=b.\`212\`,p.\`213\`=b.\`213\`,p.\`214\`=b.\`214\`,p.\`215\`=b.\`215\`,p.\`401\`=b.\`401\`,p.\`402\`=b.\`402\`,p.\`403\`=b.\`403\`,p.\`404\`=b.\`404\`,p.\`405\`=b.\`405\`,p.\`406\`=b.\`406\`,p.\`407\`=b.\`407\`,p.\`408\`=b.\`408\`,p.lastpeek=b.lastpeek,p.lastakt=b.lastakt,p.prod1=b.prod1,p.prod2=b.prod2,p.prod3=b.prod3,p.prod4=b.prod4,p.prod12=b.prod12,p.prod212=b.prod212; INSERT INTO uni1_planets SELECT * FROM $debris_backup; INSERT INTO uni1_fleet SELECT * FROM $fleet_backup; INSERT INTO uni1_queue SELECT * FROM $queue_backup; INSERT INTO uni1_messages SELECT * FROM $message_backup; INSERT INTO uni1_fleetlogs SELECT * FROM $fleetlog_backup; DROP TABLE IF EXISTS $planet_backup,$user_backup,$rank_backup,$fleet_backup,$queue_backup,$message_backup,$fleetlog_backup,$debris_backup" >/dev/null
}

reset_case() {
  db_query "DELETE FROM uni1_buddy WHERE text='$buddy_marker'; INSERT INTO uni1_buddy (request_from,request_to,text,accepted) VALUES ($player_id,$target_player_id,'$buddy_marker',1); DELETE FROM uni1_queue WHERE type='Fleet' AND (owner_id IN ($players) OR sub_id IN (SELECT fleet_id FROM uni1_fleet WHERE owner_id IN ($players) OR start_planet IN ($planets) OR target_planet IN ($planets))); DELETE FROM uni1_fleet WHERE owner_id IN ($players) OR start_planet IN ($planets) OR target_planet IN ($planets); DELETE FROM uni1_messages WHERE owner_id IN ($players); DELETE FROM uni1_fleetlogs WHERE owner_id IN ($players) OR target_id IN ($players); DELETE FROM uni1_battledata WHERE battle_id > $battle_before_max; DELETE FROM uni1_planets WHERE type=10000 AND g=$attack_g AND s=$attack_s AND p=$attack_p; UPDATE uni1_uni SET defrepair=$current_defrepair,defrepair_delta=$current_defrepair_delta,fspeed=$uni_fspeed; UPDATE uni1_planets SET \`700\`=1000000,\`701\`=1000000,\`702\`=1000000,\`202\`=0,\`203\`=0,\`204\`=0,\`205\`=0,\`206\`=0,\`207\`=0,\`208\`=0,\`209\`=0,\`210\`=0,\`211\`=0,\`212\`=0,\`213\`=0,\`214\`=0,\`215\`=0,\`401\`=0,\`402\`=0,\`403\`=0,\`404\`=0,\`405\`=0,\`406\`=0,\`407\`=0,\`408\`=0,lastpeek=UNIX_TIMESTAMP(),lastakt=UNIX_TIMESTAMP(),prod1=0,prod2=0,prod3=0,prod4=0,prod12=0,prod212=0 WHERE planet_id IN ($planets); UPDATE uni1_planets SET \`$current_ship_id\`=$current_origin_ships WHERE planet_id=$origin_id; UPDATE uni1_planets SET type=$current_target_type,owner_id=$current_target_owner,\`700\`=$current_target_metal,\`701\`=$current_target_crystal,\`702\`=$current_target_deuterium,\`204\`=$current_target_light_fighter,\`401\`=$current_target_rocket,\`406\`=$current_target_plasma WHERE planet_id=$current_target_id; UPDATE uni1_users u JOIN $rank_backup b ON b.player_id=u.player_id SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3,u.oldscore1=b.oldscore1,u.oldscore2=b.oldscore2,u.oldscore3=b.oldscore3,u.oldplace1=b.oldplace1,u.oldplace2=b.oldplace2,u.oldplace3=b.oldplace3; UPDATE uni1_users u JOIN $user_backup b ON b.player_id=u.player_id SET u.session='',u.private_session='',u.vacation=0,u.vacation_until=0,u.banned=0,u.banned_until=0,u.noattack=0,u.noattack_until=0,u.disable=0,u.disable_until=0,u.validated=1,u.deact_ip=1,u.aktplanet=u.hplanetid,u.lang='en',u.tec_until=0,u.\`106\`=CASE WHEN u.player_id=$player_id THEN 8 ELSE 3 END,u.\`124\`=CASE WHEN u.player_id=$player_id THEN 16 ELSE b.\`124\` END WHERE u.player_id IN ($players)" >/dev/null
  db_query "UPDATE uni1_users SET score1=10000000 WHERE player_id IN ($players)" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

coordinates() {
  db_query "SELECT g,s,p FROM uni1_planets WHERE planet_id=$1 LIMIT 1"
}

planet_json() {
  row="$(db_query "SELECT ROUND(\`700\`),ROUND(\`701\`),ROUND(\`702\`),\`$current_ship_id\`,\`204\`,\`401\`,\`406\` FROM uni1_planets WHERE planet_id=$1 LIMIT 1")"
  old_ifs="$IFS"; IFS="$(printf '\t')"; set -- $row; IFS="$old_ifs"
  [ "$#" -eq 7 ]
  jq -nc --arg m "$1" --arg c "$2" --arg d "$3" --arg sc "$4" --arg lf "$5" --arg rocket "$6" --arg plasma "$7" \
    '{metal:($m|tonumber),crystal:($c|tonumber),deuterium:($d|tonumber),shipCount:($sc|tonumber),lightFighter:($lf|tonumber),rocketLauncher:($rocket|tonumber),plasma:($plasma|tonumber)}'
}

capture_state() {
  origin="$(planet_json "$origin_id")"
  target="$(planet_json "$current_target_id")"
  debris_row="$(db_query "SELECT ROUND(\`700\`),ROUND(\`701\`),ROUND(\`702\`) FROM uni1_planets WHERE g=(SELECT g FROM uni1_planets WHERE planet_id=$current_target_id) AND s=(SELECT s FROM uni1_planets WHERE planet_id=$current_target_id) AND p=(SELECT p FROM uni1_planets WHERE planet_id=$current_target_id) AND type=10000 LIMIT 1")"
  debris_json=null
  if [ -n "$debris_row" ]; then
    old_ifs="$IFS"; IFS="$(printf '\t')"; set -- $debris_row; IFS="$old_ifs"
    debris_json="$(jq -nc --arg m "$1" --arg c "$2" --arg d "$3" '{metal:($m|tonumber),crystal:($c|tonumber),deuterium:($d|tonumber)}')"
  fi
  fleet="$(db_query "SELECT mission,start_planet,target_planet,flight_time,deploy_time,fuel,ROUND(\`700\`),ROUND(\`701\`),ROUND(\`702\`),\`$current_ship_id\` FROM uni1_fleet WHERE owner_id=$player_id ORDER BY fleet_id DESC LIMIT 1")"
  fleet_json=null
  queue_json=null
  if [ -n "$fleet" ]; then
    old_ifs="$IFS"; IFS="$(printf '\t')"; set -- $fleet; IFS="$old_ifs"
    [ "$#" -eq 10 ]
    fleet_json="$(jq -nc --arg mission "$1" --arg start "$2" --arg target "$3" --arg flight "$4" --arg deploy "$5" --arg fuel "$6" --arg m "$7" --arg c "$8" --arg d "$9" --arg sc "${10}" '{mission:($mission|tonumber),startPlanet:($start|tonumber),targetPlanet:($target|tonumber),flightTime:($flight|tonumber),deployTime:($deploy|tonumber),fuel:($fuel|tonumber),metal:($m|tonumber),crystal:($c|tonumber),deuterium:($d|tonumber),shipCount:($sc|tonumber)}')"
    queue="$(db_query "SELECT q.end-q.start,q.prio,q.freeze,q.frozen,q.level FROM uni1_queue q JOIN uni1_fleet f ON f.fleet_id=q.sub_id WHERE q.type='Fleet' AND f.owner_id=$player_id ORDER BY q.task_id DESC LIMIT 1")"
    if [ -n "$queue" ]; then
      IFS="$(printf '\t')"; set -- $queue; IFS="$old_ifs"
      queue_json="$(jq -nc --arg duration "$1" --arg prio "$2" --arg freeze "$3" --arg frozen "$4" --arg level "$5" '{duration:($duration|tonumber),prio:($prio|tonumber),freeze:($freeze|tonumber),frozen:($frozen|tonumber),level:($level|tonumber)}')"
    fi
  fi
  fleet_count="$(db_query "SELECT COUNT(*) FROM uni1_fleet WHERE owner_id=$player_id")"
  queue_count="$(db_query "SELECT COUNT(*) FROM uni1_queue WHERE type='Fleet' AND owner_id=$player_id")"
  message_count="$(db_query "SELECT COUNT(*) FROM uni1_messages WHERE owner_id IN ($players)")"
  fleetlog_count="$(db_query "SELECT COUNT(*) FROM uni1_fleetlogs WHERE owner_id IN ($players) OR target_id IN ($players)")"
  battle_count="$(db_query "SELECT COUNT(*) FROM uni1_battledata WHERE battle_id > $battle_before_max")"
  messages="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('ownerId',owner_id,'type',pm,'from',msgfrom,'subject',subj,'text',text,'shown',shown,'planetId',planet_id))) FROM (SELECT owner_id,pm,msgfrom,subj,text,shown,planet_id FROM uni1_messages WHERE owner_id IN ($players) ORDER BY msg_id) rows_ordered")"
  logs="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('ownerId',owner_id,'targetId',target_id,'unionId',union_id,'mission',mission,'flightTime',flight_time,'deployTime',deploy_time,'duration',end-start,'fuel',fuel,'origin',CONCAT(origin_g,':',origin_s,':',origin_p,':',origin_type),'target',CONCAT(target_g,':',target_s,':',target_p,':',target_type),'metal',ROUND(\`700\`),'crystal',ROUND(\`701\`),'deuterium',ROUND(\`702\`),'shipCount',\`$current_ship_id\`))) FROM (SELECT * FROM uni1_fleetlogs WHERE owner_id IN ($players) OR target_id IN ($players) ORDER BY log_id) rows_ordered")"
  battle="$(db_query "SELECT JSON_OBJECT('title',title,'report',report) FROM uni1_battledata WHERE battle_id > $battle_before_max ORDER BY battle_id DESC LIMIT 1")"
  users="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('playerId',player_id,'score1',score1,'score2',score2,'score3',score3))) FROM (SELECT player_id,score1,score2,score3 FROM uni1_users WHERE player_id IN ($players) ORDER BY player_id) rows_ordered")"
  [ -n "$battle" ] || battle=null
  messages="$(printf '%s' "$messages" | jq -c 'walk(if type == "string" then gsub("bericht=[0-9]+";"bericht=<id>") | gsub("At [0-9]{2}-[0-9]{2} [0-9:]{8}";"At <time>") | gsub("at [0-9]{2}-[0-9]{2} [0-9:]{8}";"at <time>") else . end) | sort_by(.ownerId,.subject,.text)')"
  battle="$(printf '%s' "$battle" | jq -c 'walk(if type == "string" then gsub("bericht=[0-9]+";"bericht=<id>") | gsub("At [0-9]{2}-[0-9]{2} [0-9:]{8}";"At <time>") else . end)')"
  logs="$(printf '%s' "$logs" | jq -c 'sort_by(.ownerId,.mission,.duration)')"
  jq -nc --argjson origin "$origin" --argjson target "$target" --argjson debris "$debris_json" --argjson fleet "$fleet_json" --argjson queue "$queue_json" \
    --arg fleets "$fleet_count" --arg tasks "$queue_count" --arg messageCount "$message_count" --arg fleetlogCount "$fleetlog_count" --arg battleCount "$battle_count" \
    --argjson messages "$messages" --argjson logs "$logs" --argjson battle "$battle" --argjson users "$users" \
    '{origin:$origin,target:$target,debris:$debris,fleet:$fleet,queue:$queue,messages:$messages,fleetLogs:$logs,battle:$battle,users:$users,counts:{fleets:($fleets|tonumber),tasks:($tasks|tonumber),messages:($messageCount|tonumber),logs:($fleetlogCount|tonumber),battles:($battleCount|tonumber)}}'
}

normalize_recall_clock() {
  jq -c --argjson expected "$RECALL_ELAPSED_SECONDS" '
    if .fleet != null and .fleet.mission == 103 then
      if (.fleet.flightTime < ($expected - 1) or .fleet.flightTime > ($expected + 10) or .queue.duration < ($expected - 1) or .queue.duration > ($expected + 10)) then error("recall duration outside deterministic window") else .fleet.flightTime=$expected | .queue.duration=$expected end
    else . end |
    .fleetLogs |= map(if .mission == 103 then if (.flightTime < ($expected - 1) or .flightTime > ($expected + 10) or .duration < ($expected - 1) or .duration > ($expected + 10)) then error("recall log duration outside deterministic window") else .flightTime=$expected | .duration=$expected end else . end)'
}

login_legacy() {
  prefix="$1"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" --output "$TMP_DIR/$prefix-login.body" --cookie-jar "$TMP_DIR/$prefix.cookies" --request POST --data-urlencode "login=$login" --data-urlencode "pass=$PASSWORD" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/reg/login2.php")"
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/$prefix-login.headers")"
  legacy_session="$(printf '%s' "$location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 302 ] && [ -n "$legacy_session" ]
}

login_go() {
  prefix="$1"
  payload="$(jq -nc --arg login "$login" --arg pass "$PASSWORD" '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" --output "$TMP_DIR/$prefix-login.body" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  go_cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/$prefix-login.headers" | cut -d';' -f1 | tr -d '\r')"
  redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/$prefix-login.body")"
  go_session="$(printf '%s' "$redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 200 ] && [ -n "$go_cookie" ] && [ -n "$go_session" ]
}

legacy_launch() {
  prefix="$1"
  origin_coords="$(coordinates "$origin_id")"; target_coords="$(coordinates "$current_target_id")"
  old_ifs="$IFS"; IFS="$(printf '\t')"; set -- $origin_coords; IFS="$old_ifs"
  og="$1" os="$2" op="$3"
  IFS="$(printf '\t')"; set -- $target_coords; IFS="$old_ifs"
  tg="$1" ts="$2" tp="$3"
  fspeed="$(db_query 'SELECT fspeed FROM uni1_uni LIMIT 1')"
  ship202=0; ship204=0; ship209=0; ship210=0; ship213=0
  case "$current_ship_id" in 202) ship202="$current_ships" ;; 204) ship204="$current_ships" ;; 209) ship209="$current_ships" ;; 210) ship210="$current_ships" ;; 213) ship213="$current_ships" ;; esac
  legacy_launch_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$prefix-launch.body" --cookie "$TMP_DIR/$prefix.cookies" --write-out '%{http_code}' \
    --data-urlencode "thisgalaxy=$og" --data-urlencode "thissystem=$os" --data-urlencode "thisplanet=$op" --data-urlencode 'thisplanettype=1' --data-urlencode "speedfactor=$fspeed" \
    --data-urlencode "galaxy=$tg" --data-urlencode "system=$ts" --data-urlencode "planet=$tp" --data-urlencode "planettype=$current_game_target_type" --data-urlencode 'speed=10' --data-urlencode "order=$current_mission" \
    --data-urlencode "ship202=$ship202" --data-urlencode 'ship203=0' --data-urlencode "ship204=$ship204" --data-urlencode 'ship205=0' --data-urlencode 'ship206=0' --data-urlencode 'ship207=0' --data-urlencode 'ship208=0' --data-urlencode "ship209=$ship209" --data-urlencode "ship210=$ship210" --data-urlencode 'ship211=0' --data-urlencode 'ship212=0' --data-urlencode "ship213=$ship213" --data-urlencode 'ship214=0' --data-urlencode 'ship215=0' \
    --data-urlencode "resource1=$current_metal" --data-urlencode "resource2=$current_crystal" --data-urlencode "resource3=$current_deuterium" \
    --data-urlencode "holdingtime=$current_hold_hours" --data-urlencode "expeditiontime=$current_expedition_hours" \
    "$LEGACY_BASE_URL/game/index.php?page=flottenversand&session=$legacy_session&cp=$origin_id")"
}

go_launch() {
  prefix="$1"
  target_coords="$(coordinates "$current_target_id")"
  old_ifs="$IFS"; IFS="$(printf '\t')"; set -- $target_coords; IFS="$old_ifs"
  payload="$(jq -nc --arg g "$1" --arg s "$2" --arg p "$3" --arg mission "$current_mission" --arg shipID "$current_ship_id" --arg ships "$current_ships" --arg m "$current_metal" --arg c "$current_crystal" --arg d "$current_deuterium" --arg targetType "$current_game_target_type" --arg hold "$current_hold_hours" --arg expedition "$current_expedition_hours" '{action:"launch-dispatch",ships:{($shipID):($ships|tonumber)},resources:{"700":($m|tonumber),"701":($c|tonumber),"702":($d|tonumber)},target:{galaxy:($g|tonumber),system:($s|tonumber),position:($p|tonumber)},targetType:($targetType|tonumber),mission:($mission|tonumber),speed:10,holdHours:($hold|tonumber),expeditionHours:($expedition|tonumber)}')"
  go_launch_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$prefix-launch.body" --cookie "$go_cookie" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/game/fleet?session=$go_session&cp=$origin_id")"
}

force_latest_due() {
  db_query "UPDATE uni1_queue q JOIN uni1_fleet f ON f.fleet_id=q.sub_id SET q.start=UNIX_TIMESTAMP()-2,q.end=UNIX_TIMESTAMP()-1 WHERE q.type='Fleet' AND f.owner_id=$player_id" >/dev/null
}

run_side() {
  side="$1"; mode="$2"
  reset_case
  if [ "$side" = legacy ]; then login_legacy "$side-$mode"; else login_go "$side-$mode"; fi
  before="$(capture_state)"
  if [ "$side" = legacy ]; then legacy_launch "$side-$mode"; launch_status="$legacy_launch_status"; else go_launch "$side-$mode"; launch_status="$go_launch_status"; fi
  launched="$(capture_state)"
  if [ "$mode" = recall ]; then
    launched_duration="$(printf '%s' "$launched" | jq -r '.queue.duration // 0')"
    [ "$launched_duration" -gt "$RECALL_ELAPSED_SECONDS" ] || {
      printf 'Recall elapsed time (%ss) must be shorter than outbound flight time (%ss).\n' "$RECALL_ELAPSED_SECONDS" "$launched_duration" >&2
      return 1
    }
    db_query "UPDATE uni1_queue q JOIN uni1_fleet f ON f.fleet_id=q.sub_id SET q.start=UNIX_TIMESTAMP()-$RECALL_ELAPSED_SECONDS,q.end=UNIX_TIMESTAMP()+GREATEST(f.flight_time-$RECALL_ELAPSED_SECONDS,1) WHERE q.type='Fleet' AND f.owner_id=$player_id" >/dev/null
    fleet_id="$(db_query "SELECT fleet_id FROM uni1_fleet WHERE owner_id=$player_id ORDER BY fleet_id DESC LIMIT 1")"
    if [ "$side" = legacy ]; then
      action_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$mode-action.body" --cookie "$TMP_DIR/$side-$mode.cookies" --data-urlencode "order_return=$fleet_id" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=flotten1&session=$legacy_session&cp=$origin_id")"
    else
      action_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$mode-action.body" --cookie "$go_cookie" --header 'Content-Type: application/json' --data "{\"action\":\"recall\",\"fleetId\":$fleet_id}" --write-out '%{http_code}' "$GO_BASE_URL/api/game/fleet?session=$go_session&cp=$origin_id")"
    fi
    transitioned="$(capture_state)"
    transitioned="$(printf '%s' "$transitioned" | normalize_recall_clock)"
    force_latest_due
  else
    force_latest_due
    if [ "$side" = legacy ]; then
      action_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$mode-action.body" --cookie "$TMP_DIR/$side-$mode.cookies" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=flotten1&session=$legacy_session&cp=$origin_id")"
    else
      action_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$mode-action.body" --cookie "$go_cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/fleet?session=$go_session&cp=$origin_id")"
    fi
    transitioned="$(capture_state)"
    case "$mode" in transport|recycle|spy|attack|attack-defense|attack-guarded-win|attack-defense-repair|acs-hold) force_latest_due ;; esac
  fi
  hold_status=200
  returning="$transitioned"
  if [ "$mode" = acs-hold ]; then
    if [ "$side" = legacy ]; then
      hold_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$mode-hold.body" --cookie "$TMP_DIR/$side-$mode.cookies" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=flotten1&session=$legacy_session&cp=$origin_id")"
    else
      hold_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$mode-hold.body" --cookie "$go_cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/fleet?session=$go_session&cp=$origin_id")"
    fi
    returning="$(capture_state)"
    force_latest_due
  fi
  if [ "$side" = legacy ]; then
    final_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$mode-final.body" --cookie "$TMP_DIR/$side-$mode.cookies" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=flotten1&session=$legacy_session&cp=$origin_id")"
  else
    final_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$mode-final.body" --cookie "$go_cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/fleet?session=$go_session&cp=$origin_id")"
  fi
  final="$(capture_state)"
  if [ "$mode" = recall ]; then
    final="$(printf '%s' "$final" | normalize_recall_clock)"
  fi
}

all_pass=true
case_results="$TMP_DIR/cases.jsonl"
: > "$case_results"

run_case() {
  mode="$1"
  current_defrepair="$uni_defrepair"
  current_defrepair_delta="$uni_defrepair_delta"
  current_target_rocket=0
  current_hold_hours=0
  current_expedition_hours=0
  case "$mode" in
    transport) current_target_id="$transport_target_id"; current_mission=3; current_ship_id=202; current_ships=1; current_origin_ships=10; current_metal=123; current_crystal=45; current_deuterium=0; current_target_type=1; current_game_target_type=1; current_target_owner="$target_player_id"; current_target_metal=1000000; current_target_crystal=1000000; current_target_deuterium=1000000; current_target_light_fighter=0; current_target_plasma=0 ;;
    recall) current_target_id="$transport_target_id"; current_mission=3; current_ship_id=202; current_ships=1; current_origin_ships=10; current_metal=88; current_crystal=33; current_deuterium=0; current_target_type=1; current_game_target_type=1; current_target_owner="$target_player_id"; current_target_metal=1000000; current_target_crystal=1000000; current_target_deuterium=1000000; current_target_light_fighter=0; current_target_plasma=0 ;;
    deploy) current_target_id="$deploy_target_id"; current_mission=4; current_ship_id=202; current_ships=2; current_origin_ships=10; current_metal=77; current_crystal=22; current_deuterium=11; current_target_type=1; current_game_target_type=1; current_target_owner="$player_id"; current_target_metal=1000000; current_target_crystal=1000000; current_target_deuterium=1000000; current_target_light_fighter=0; current_target_plasma=0 ;;
    recycle) current_target_id="$deploy_target_id"; current_mission=8; current_ship_id=209; current_ships=5; current_origin_ships=5; current_metal=0; current_crystal=0; current_deuterium=0; current_target_type=10000; current_game_target_type=2; current_target_owner="$player_id"; current_target_metal=120000; current_target_crystal=80000; current_target_deuterium=0; current_target_light_fighter=0; current_target_plasma=0 ;;
    spy) current_target_id="$transport_target_id"; current_mission=6; current_ship_id=210; current_ships=2; current_origin_ships=10; current_metal=0; current_crystal=0; current_deuterium=0; current_target_type=1; current_game_target_type=1; current_target_owner="$target_player_id"; current_target_metal=1000000; current_target_crystal=1000000; current_target_deuterium=1000000; current_target_light_fighter=0; current_target_plasma=0 ;;
    attack) current_target_id="$transport_target_id"; current_mission=1; current_ship_id=202; current_ships=2; current_origin_ships=10; current_metal=0; current_crystal=0; current_deuterium=0; current_target_type=1; current_game_target_type=1; current_target_owner="$target_player_id"; current_target_metal=1000000; current_target_crystal=1000000; current_target_deuterium=1000000; current_target_light_fighter=0; current_target_plasma=0 ;;
    attack-defense) current_target_id="$transport_target_id"; current_mission=1; current_ship_id=204; current_ships=1; current_origin_ships=10; current_metal=0; current_crystal=0; current_deuterium=0; current_target_type=1; current_game_target_type=1; current_target_owner="$target_player_id"; current_target_metal=1000000; current_target_crystal=1000000; current_target_deuterium=1000000; current_target_light_fighter=0; current_target_plasma=10 ;;
    attack-guarded-win) current_target_id="$transport_target_id"; current_mission=1; current_ship_id=213; current_ships=1; current_origin_ships=10; current_metal=0; current_crystal=0; current_deuterium=0; current_target_type=1; current_game_target_type=1; current_target_owner="$target_player_id"; current_target_metal=1000000; current_target_crystal=1000000; current_target_deuterium=1000000; current_target_light_fighter=1; current_target_plasma=0 ;;
    attack-defense-repair) current_target_id="$transport_target_id"; current_mission=1; current_ship_id=213; current_ships=1; current_origin_ships=10; current_metal=0; current_crystal=0; current_deuterium=0; current_target_type=1; current_game_target_type=1; current_target_owner="$target_player_id"; current_target_metal=1000000; current_target_crystal=1000000; current_target_deuterium=1000000; current_target_light_fighter=0; current_target_rocket=1; current_target_plasma=0; current_defrepair=100; current_defrepair_delta=0 ;;
    acs-hold) current_target_id="$transport_target_id"; current_mission=5; current_ship_id=204; current_ships=1; current_origin_ships=10; current_metal=0; current_crystal=0; current_deuterium=0; current_target_type=1; current_game_target_type=1; current_target_owner="$target_player_id"; current_target_metal=1000000; current_target_crystal=1000000; current_target_deuterium=1000000; current_target_light_fighter=0; current_target_plasma=0; current_hold_hours=1 ;;
    expedition) current_target_id="$expedition_target_id"; current_mission=15; current_ship_id=202; current_ships=1; current_origin_ships=10; current_metal=0; current_crystal=0; current_deuterium=0; current_target_type=20000; current_game_target_type=1; current_target_owner=99999; current_target_metal=0; current_target_crystal=0; current_target_deuterium=0; current_target_light_fighter=0; current_target_plasma=0; current_expedition_hours=1 ;;
  esac
  run_side legacy "$mode"
  legacy_launch_status="$launch_status"; legacy_action_status="$action_status"; legacy_hold_status="$hold_status"; legacy_final_status="$final_status"
  legacy_before="$before"; legacy_launched="$launched"; legacy_transitioned="$transitioned"; legacy_returning="$returning"; legacy_final="$final"
  run_side go "$mode"
  go_launch_status="$launch_status"; go_action_status="$action_status"; go_hold_status="$hold_status"; go_final_status="$final_status"
  go_before="$before"; go_launched="$launched"; go_transitioned="$transitioned"; go_returning="$returning"; go_final="$final"
  pass=true
  [ "$legacy_launch_status" = 200 ] && [ "$go_launch_status" = 200 ] || pass=false
  [ "$legacy_action_status" = 200 ] && [ "$go_action_status" = 200 ] || pass=false
  [ "$legacy_hold_status" = 200 ] && [ "$go_hold_status" = 200 ] || pass=false
  [ "$legacy_final_status" = 200 ] && [ "$go_final_status" = 200 ] || pass=false
  [ "$legacy_before" = "$go_before" ] || pass=false
  [ "$legacy_launched" = "$go_launched" ] || pass=false
  [ "$legacy_transitioned" = "$go_transitioned" ] || pass=false
  [ "$legacy_returning" = "$go_returning" ] || pass=false
  [ "$legacy_final" = "$go_final" ] || pass=false
  jq -e '.fleet != null and .queue != null and .counts.fleets == 1 and .counts.tasks == 1 and .origin.shipCount < $before.origin.shipCount and (.origin.metal <= $before.origin.metal)' --argjson before "$legacy_before" >/dev/null <<EOF || pass=false
$legacy_launched
EOF
  if [ "$mode" != expedition ]; then
    jq -e '.fleet == null and .queue == null and .counts.fleets == 0 and .counts.tasks == 0' >/dev/null <<EOF || pass=false
$legacy_final
EOF
  fi
  if [ "$mode" = deploy ]; then
    jq -e '.target.shipCount == 2 and .target.metal >= 1000077 and .target.crystal >= 1000022 and .target.deuterium >= 1000011' >/dev/null <<EOF || pass=false
$legacy_final
EOF
  elif [ "$mode" = recycle ]; then
    jq -e '.origin.shipCount == 5 and .origin.metal == 1050000 and .origin.crystal == 1050000 and .target.metal == 70000 and .target.crystal == 30000' >/dev/null <<EOF || pass=false
$legacy_final
EOF
  elif [ "$mode" = spy ]; then
    jq -e --argjson outbound "$legacy_launched" '.fleet.mission == 106 and .fleet.flightTime == $outbound.fleet.flightTime and .fleet.fuel == (($outbound.fleet.fuel / 2) | floor) and .queue.duration == $outbound.queue.duration and .counts.messages == 2 and .counts.logs == 2 and (.messages | map(.type) | sort) == [1,5] and (.messages[] | select(.type == 1) | .text | contains("Resources on"))' >/dev/null <<EOF || pass=false
$legacy_transitioned
EOF
    jq -e '.origin.shipCount == 10 and .counts.messages == 2 and .counts.logs == 2' >/dev/null <<EOF || pass=false
$legacy_final
EOF
  elif [ "$mode" = attack ]; then
    jq -e '.origin.shipCount == 10 and .target.metal < 1000000 and .target.crystal < 1000000 and .target.deuterium < 1000000 and .debris != null and .debris.metal == 0 and .debris.crystal == 0 and .counts.messages == 5 and .counts.battles == 1 and .battle.report != ""' >/dev/null <<EOF || pass=false
$legacy_final
EOF
  elif [ "$mode" = attack-defense ]; then
    jq -e --argjson before "$legacy_before" '.origin.shipCount == 9 and .target.plasma == 10 and .target.metal == 1000000 and .target.crystal == 1000000 and .target.deuterium == 1000000 and .debris.metal == 900 and .debris.crystal == 300 and .counts.messages == 4 and .counts.battles == 1 and .battle.report != "" and .users[0].score1 == ($before.users[0].score1 - 4000) and .users[0].score2 == ($before.users[0].score2 - 1)' >/dev/null <<EOF || pass=false
$legacy_final
EOF
  elif [ "$mode" = attack-guarded-win ]; then
    jq -e --argjson before "$legacy_before" '.origin.shipCount == 10 and .target.lightFighter == 0 and .target.metal < 1000000 and .target.crystal < 1000000 and .target.deuterium < 1000000 and .debris.metal == 900 and .debris.crystal == 300 and .counts.messages == 5 and .counts.battles == 1 and .battle.report != "" and .users[1].score1 == ($before.users[1].score1 - 4000) and .users[1].score2 == ($before.users[1].score2 - 1)' >/dev/null <<EOF || pass=false
$legacy_final
EOF
  elif [ "$mode" = attack-defense-repair ]; then
    jq -e --argjson before "$legacy_before" '.origin.shipCount == 10 and .target.rocketLauncher == 1 and .target.metal < 1000000 and .target.crystal < 1000000 and .target.deuterium < 1000000 and .debris.metal == 0 and .debris.crystal == 0 and .counts.messages == 5 and .counts.battles == 1 and (.battle.report | contains("1 Rocket Launchercould be repaired.")) and .users[1].score1 == ($before.users[1].score1 - 2000) and .users[1].score2 == $before.users[1].score2' >/dev/null <<EOF || pass=false
$legacy_final
EOF
  elif [ "$mode" = acs-hold ]; then
    jq -e '.fleet.mission == 205 and .fleet.flightTime == 3600 and .fleet.deployTime > 0 and .fleet.fuel == 0 and .queue.duration == 3600 and .counts.fleets == 1 and .counts.tasks == 1 and .counts.logs == 2' >/dev/null <<EOF || pass=false
$legacy_transitioned
EOF
    jq -e --argjson outbound "$legacy_launched" '.fleet.mission == 105 and .fleet.flightTime == $outbound.fleet.flightTime and .fleet.deployTime == 0 and .fleet.fuel == 0 and .queue.duration == $outbound.queue.duration and .counts.fleets == 1 and .counts.tasks == 1 and .counts.logs == 3' >/dev/null <<EOF || pass=false
$legacy_returning
EOF
    jq -e '.origin.shipCount == 10 and .counts.messages == 1 and .counts.logs == 3' >/dev/null <<EOF || pass=false
$legacy_final
EOF
  elif [ "$mode" = expedition ]; then
    jq -e --argjson hold "$expedition_hold_seconds" '.fleet.mission == 15 and .fleet.deployTime == $hold and .queue.level == 1 and .counts.fleets == 1 and .counts.tasks == 1 and .counts.logs == 1' >/dev/null <<EOF || pass=false
$legacy_launched
EOF
    jq -e --argjson hold "$expedition_hold_seconds" --argjson outbound "$legacy_launched" '.fleet.mission == 215 and .fleet.flightTime == $hold and .fleet.deployTime == $outbound.fleet.flightTime and .queue.duration == $hold and .queue.level == 1 and .counts.fleets == 1 and .counts.tasks == 1 and .counts.logs == 2' >/dev/null <<EOF || pass=false
$legacy_transitioned
EOF
    [ "$legacy_transitioned" = "$legacy_final" ] || pass=false
  else
    jq -e '.origin.shipCount == 10' >/dev/null <<EOF || pass=false
$legacy_final
EOF
  fi
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "fleet-$mode" --argjson pass "$pass" --arg legacyLaunch "$legacy_launch_status" --arg goLaunch "$go_launch_status" --arg legacyAction "$legacy_action_status" --arg goAction "$go_action_status" --arg legacyHold "$legacy_hold_status" --arg goHold "$go_hold_status" --arg legacyFinalStatus "$legacy_final_status" --arg goFinalStatus "$go_final_status" --argjson before "$legacy_before" --argjson legacyLaunched "$legacy_launched" --argjson goLaunched "$go_launched" --argjson legacyTransitioned "$legacy_transitioned" --argjson goTransitioned "$go_transitioned" --argjson legacyReturning "$legacy_returning" --argjson goReturning "$go_returning" --argjson legacyFinal "$legacy_final" --argjson goFinal "$go_final" '{name:$name,pass:$pass,http:{launch:{legacy:$legacyLaunch,go:$goLaunch},action:{legacy:$legacyAction,go:$goAction},hold:{legacy:$legacyHold,go:$goHold},final:{legacy:$legacyFinalStatus,go:$goFinalStatus}},db:{before:$before,launched:{legacy:$legacyLaunched,go:$goLaunched},transitioned:{legacy:$legacyTransitioned,go:$goTransitioned},returning:{legacy:$legacyReturning,go:$goReturning},final:{legacy:$legacyFinal,go:$goFinal}}}' >> "$case_results"
}

case_count=0
for fleet_case in $FLEET_CASES; do
  case "$fleet_case" in
    transport|recall|deploy|recycle|spy|attack|attack-defense|attack-guarded-win|attack-defense-repair|acs-hold|expedition) ;;
    *) printf 'Unknown fleet differential case: %s\n' "$fleet_case" >&2; exit 1 ;;
  esac
  run_case "$fleet_case"
  case_count=$((case_count + 1))
done
[ "$case_count" -gt 0 ]
restore_original
jq -s --argjson pass "$all_pass" --arg login "$login" --argjson playerId "$player_id" --argjson originId "$origin_id" --argjson fleetSpeed "$uni_fspeed" --argjson recallElapsedSeconds "$RECALL_ELAPSED_SECONDS" '{pass:$pass,fixture:{login:$login,playerId:$playerId,originId:$originId,fleetSpeed:$fleetSpeed,recallElapsedSeconds:$recallElapsedSeconds},cases:.}' "$case_results" > "$REPORT"
[ "$all_pass" = true ]
printf 'Go/PHP fleet differential E2E: PASS (%s cases, fspeed=%s)\n' "$case_count" "$uni_fspeed"
