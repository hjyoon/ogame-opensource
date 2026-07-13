#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_EXPEDITION_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-expedition-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ]
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/expedition-differential.XXXXXX")"

login="$(jq -r '.fleet_lifecycle.attacker.login' "$FIXTURE")"
player_id="$(jq -r '.fleet_lifecycle.attacker.player_id' "$FIXTURE")"
home_planet="$(jq -r '.fleet_lifecycle.attacker.home_planet_id' "$FIXTURE")"

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'exec mysql -N -B -r -uroot -p"$MYSQL_ROOT_PASSWORD" uni -e "$1"' sh "$1" 2>/dev/null
}

target_g="$(db_query "SELECT g FROM uni1_planets WHERE planet_id=$home_planet")"
target_s=499
while [ "$(db_query "SELECT COUNT(*) FROM uni1_planets WHERE g=$target_g AND s=$target_s AND p=16")" != 0 ]; do
  target_s=$((target_s - 1))
  [ "$target_s" -gt 400 ]
done

exp_backup="uni1_e2e_exp_settings_$$"
user_backup="uni1_e2e_exp_user_$$"
rank_backup="uni1_e2e_exp_ranks_$$"
fleet_backup="uni1_e2e_exp_fleet_$$"
queue_backup="uni1_e2e_exp_queue_$$"
message_backup="uni1_e2e_exp_messages_$$"
log_backup="uni1_e2e_exp_logs_$$"
battle_max="$(db_query 'SELECT COALESCE(MAX(battle_id),0) FROM uni1_battledata')"

db_query "DROP TABLE IF EXISTS $exp_backup,$user_backup,$rank_backup,$fleet_backup,$queue_backup,$message_backup,$log_backup; CREATE TABLE $exp_backup AS SELECT * FROM uni1_exptab; CREATE TABLE $user_backup AS SELECT * FROM uni1_users WHERE player_id=$player_id; CREATE TABLE $rank_backup AS SELECT player_id,score1,score2,score3,place1,place2,place3,oldscore1,oldscore2,oldscore3,oldplace1,oldplace2,oldplace3 FROM uni1_users; CREATE TABLE $fleet_backup AS SELECT * FROM uni1_fleet WHERE owner_id=$player_id; CREATE TABLE $queue_backup AS SELECT q.* FROM uni1_queue q JOIN $fleet_backup f ON f.fleet_id=q.sub_id WHERE q.type='Fleet'; CREATE TABLE $message_backup AS SELECT * FROM uni1_messages WHERE owner_id=$player_id; CREATE TABLE $log_backup AS SELECT * FROM uni1_fleetlogs WHERE owner_id=$player_id" >/dev/null

delete_case_state() {
  db_query "DELETE q FROM uni1_queue q LEFT JOIN uni1_fleet f ON f.fleet_id=q.sub_id WHERE q.type='Fleet' AND (q.owner_id=$player_id OR f.owner_id=$player_id); DELETE FROM uni1_fleet WHERE owner_id=$player_id; DELETE FROM uni1_messages WHERE owner_id=$player_id; DELETE FROM uni1_fleetlogs WHERE owner_id=$player_id; DELETE FROM uni1_planets WHERE name='E2E Exp Farspace' AND g=$target_g AND s=$target_s AND p=16; DELETE FROM uni1_battledata WHERE battle_id>$battle_max" >/dev/null
}

restore_rows() {
  delete_case_state
  db_query "DELETE FROM uni1_users WHERE player_id=$player_id; INSERT INTO uni1_users SELECT * FROM $user_backup; UPDATE uni1_users u JOIN $rank_backup b ON b.player_id=u.player_id SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3,u.oldscore1=b.oldscore1,u.oldscore2=b.oldscore2,u.oldscore3=b.oldscore3,u.oldplace1=b.oldplace1,u.oldplace2=b.oldplace2,u.oldplace3=b.oldplace3; DELETE FROM uni1_exptab; INSERT INTO uni1_exptab SELECT * FROM $exp_backup; INSERT INTO uni1_fleet SELECT * FROM $fleet_backup; INSERT INTO uni1_queue SELECT * FROM $queue_backup; INSERT INTO uni1_messages SELECT * FROM $message_backup; INSERT INTO uni1_fleetlogs SELECT * FROM $log_backup" >/dev/null
}

cleanup() {
  restore_rows >/dev/null 2>&1 || true
  db_query "DROP TABLE IF EXISTS $exp_backup,$user_backup,$rank_backup,$fleet_backup,$queue_backup,$message_backup,$log_backup" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

seed_case() {
  event="$1"
  restore_rows
  delete_case_state
  db_query "UPDATE uni1_users SET session='',private_session='',lang='en',admin=0,banned=0,validated=1,deact_ip=1,dmfree=0,trader=0,rate_m=0,rate_k=0,rate_d=0,score1=100000000,score2=1000,score3=1000,\`109\`=10,\`110\`=10,\`111\`=10 WHERE player_id=$player_id; UPDATE uni1_exptab SET chance_success=100,depleted_min=999,depleted_med=1000,depleted_max=1001,chance_depleted_min=0,chance_depleted_med=0,chance_depleted_max=0,chance_alien=100,chance_pirates=100,chance_dm=100,chance_lost=100,chance_delay=100,chance_accel=100,chance_res=100,chance_fleet=100,dm_factor=3" >/dev/null
  case "$event" in
    nothing) db_query 'UPDATE uni1_exptab SET chance_success=0' >/dev/null ;;
    aliens) db_query 'UPDATE uni1_exptab SET chance_alien=0' >/dev/null ;;
    pirates) db_query 'UPDATE uni1_exptab SET chance_pirates=0' >/dev/null ;;
    dark_matter) db_query 'UPDATE uni1_exptab SET chance_dm=0' >/dev/null ;;
    black_hole) db_query 'UPDATE uni1_exptab SET chance_lost=0' >/dev/null ;;
    delay) db_query 'UPDATE uni1_exptab SET chance_delay=0' >/dev/null ;;
    accel) db_query 'UPDATE uni1_exptab SET chance_accel=0' >/dev/null ;;
    resources) db_query 'UPDATE uni1_exptab SET chance_res=0' >/dev/null ;;
    fleet) db_query 'UPDATE uni1_exptab SET chance_fleet=0' >/dev/null ;;
    trader) : ;;
  esac

  hold=3600
  [ "$event" != nothing ] || hold=0
  small=10; large=0; probe=1; deathstar=0
  case "$event" in
    resources|fleet) small=0; large=20 ;;
    aliens|pirates) small=0; deathstar=20 ;;
  esac
  db_query "INSERT INTO uni1_planets (name,type,g,s,p,owner_id,diameter,temp,fields,maxfields,date,\`700\`,\`701\`,\`702\`,lastpeek,lastakt,gate_until,remove) VALUES ('E2E Exp Farspace',20000,$target_g,$target_s,16,99999,0,0,0,0,UNIX_TIMESTAMP(),0,0,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP(),0,0); SET @target=LAST_INSERT_ID(); INSERT INTO uni1_fleet (owner_id,union_id,\`700\`,\`701\`,\`702\`,fuel,mission,start_planet,target_planet,flight_time,deploy_time,\`202\`,\`203\`,\`210\`,\`214\`) VALUES ($player_id,0,0,0,0,100,15,$home_planet,@target,120,$hold,$small,$large,$probe,$deathstar); SET @fleet=LAST_INSERT_ID(); INSERT INTO uni1_queue (owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES ($player_id,'Fleet',@fleet,0,0,UNIX_TIMESTAMP()-121,UNIX_TIMESTAMP()-1,1515)" >/dev/null
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

capture_state() {
  event="$1"; score_before=100000000
  raw="$(db_query "SELECT JSON_OBJECT('visit',(SELECT \`700\` FROM uni1_planets WHERE name='E2E Exp Farspace' AND g=$target_g AND s=$target_s AND p=16 LIMIT 1),'fleetCount',(SELECT COUNT(*) FROM uni1_fleet WHERE owner_id=$player_id),'returnCount',(SELECT COUNT(*) FROM uni1_fleet WHERE owner_id=$player_id AND mission=115),'returnFlight',COALESCE((SELECT flight_time FROM uni1_fleet WHERE owner_id=$player_id AND mission=115 ORDER BY fleet_id DESC LIMIT 1),0),'returnFuel',COALESCE((SELECT fuel FROM uni1_fleet WHERE owner_id=$player_id AND mission=115 ORDER BY fleet_id DESC LIMIT 1),0),'returnShips',COALESCE((SELECT \`202\`+\`203\`+\`204\`+\`205\`+\`206\`+\`207\`+\`208\`+\`209\`+\`210\`+\`211\`+\`212\`+\`213\`+\`214\`+\`215\` FROM uni1_fleet WHERE owner_id=$player_id AND mission=115 ORDER BY fleet_id DESC LIMIT 1),0),'returnResources',COALESCE((SELECT \`700\`+\`701\`+\`702\` FROM uni1_fleet WHERE owner_id=$player_id AND mission=115 ORDER BY fleet_id DESC LIMIT 1),0),'dmfree',(SELECT dmfree FROM uni1_users WHERE player_id=$player_id),'trader',(SELECT trader FROM uni1_users WHERE player_id=$player_id),'score1',(SELECT score1 FROM uni1_users WHERE player_id=$player_id),'expMessages',(SELECT COUNT(*) FROM uni1_messages WHERE owner_id=$player_id AND pm=3 AND subj LIKE 'Expedition result%'),'battleText',(SELECT COUNT(*) FROM uni1_messages WHERE owner_id=$player_id AND pm=6),'battleLink',(SELECT COUNT(*) FROM uni1_messages WHERE owner_id=$player_id AND pm=2),'battleData',(SELECT COUNT(*) FROM uni1_battledata WHERE battle_id>$battle_max),'logs',(SELECT COUNT(*) FROM uni1_fleetlogs WHERE owner_id=$player_id),'orbitLogs',(SELECT COUNT(*) FROM uni1_fleetlogs WHERE owner_id=$player_id AND mission=215),'returnLogs',(SELECT COUNT(*) FROM uni1_fleetlogs WHERE owner_id=$player_id AND mission=115))")"
  sent=11
  case "$event" in resources|fleet) sent=21 ;; aliens|pirates) sent=21 ;; esac
  printf '%s' "$raw" | jq -cS --arg event "$event" --argjson sent "$sent" --argjson scoreBefore "$score_before" '
    . + {timing:(if .returnCount==0 then "none" elif .returnFlight>120 then "delayed" elif .returnFlight<120 then "accelerated" else "base" end)} |
    {processed:(.fleetCount==.returnCount),visit:(.visit==1),expeditionMessage:(.expMessages==1),returnPresent:(.returnCount==1),timing:.timing,returnFuel:.returnFuel,
     reward:(if $event=="dark_matter" then .dmfree>0 elif $event=="resources" then .returnResources>0 elif $event=="fleet" then .returnShips>$sent elif $event=="trader" then .trader>0 else true end),
     scoreContract:(if $event=="black_hole" then .score1<$scoreBefore elif ($event=="aliens" or $event=="pirates") then .score1<=$scoreBefore else true end),
     battleText:(.battleText==(if ($event=="aliens" or $event=="pirates") then 1 else 0 end)),battleLink:(.battleLink==(if ($event=="aliens" or $event=="pirates") then 1 else 0 end)),battleData:(.battleData==(if ($event=="aliens" or $event=="pirates") then 1 else 0 end)),
     orbitLog:(.orbitLogs==1),returnLog:(.returnLogs==(if $event=="black_hole" then 0 else 1 end)),logCount:(.logs==(if $event=="black_hole" then 1 else 2 end))}'
}

run_side() {
  side="$1"; event="$2"
  seed_case "$event"
  if [ "$side" = legacy ]; then
    login_legacy
    first="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/legacy-$event-arrive.body" --cookie "$TMP_DIR/legacy.cookies" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=flotten1&session=$session&cp=$home_planet")"
  else
    login_go
    first="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/go-$event-arrive.body" --cookie "$cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/fleet?session=$session&cp=$home_planet")"
  fi
  db_query "UPDATE uni1_queue q JOIN uni1_fleet f ON f.fleet_id=q.sub_id SET q.end=UNIX_TIMESTAMP()-1 WHERE q.type='Fleet' AND f.owner_id=$player_id AND f.mission=215" >/dev/null
  if [ "$side" = legacy ]; then
    second="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/legacy-$event-hold.body" --cookie "$TMP_DIR/legacy.cookies" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=flotten1&session=$session&cp=$home_planet")"
  else
    second="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/go-$event-hold.body" --cookie "$cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/fleet?session=$session&cp=$home_planet")"
  fi
  normalized="$(capture_state "$event")"
  jq -nc --arg first "$first" --arg second "$second" --argjson state "$normalized" '{http:{arrival:$first,hold:$second},state:$state}'
}

run_side_with_retries() {
  side="$1"; event="$2"; attempts=1
  result="$(run_side "$side" "$event")"
  while [ "$event" = fleet ] && [ "$(printf '%s' "$result" | jq -r '.state.reward')" != true ] && [ "$attempts" -lt 12 ]; do
    attempts=$((attempts + 1))
    result="$(run_side "$side" "$event")"
  done
  printf '%s' "$result" | jq -c --argjson attempts "$attempts" '. + {attempts:$attempts}'
}

results='[]'; pass=true
for event in nothing dark_matter resources fleet trader delay accel aliens pirates black_hole; do
  legacy="$(run_side_with_retries legacy "$event")"
  go="$(run_side_with_retries go "$event")"
  case_pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.state')" = "$(printf '%s' "$go" | jq -cS '.state')" ] || case_pass=false
  printf '%s' "$legacy" | jq -e --arg event "$event" '.http.arrival=="200" and .http.hold=="200" and (.state|to_entries|all(if (.key=="returnPresent" and $event=="black_hole") then (.value==false) elif (.key=="timing") then (.value==(if $event=="black_hole" then "none" elif $event=="delay" then "delayed" elif $event=="accel" then "accelerated" else "base" end)) elif (.key=="returnFuel") then (.value==0) else (.value==true) end))' >/dev/null || case_pass=false
  [ "$case_pass" = true ] || pass=false
  results="$(printf '%s' "$results" | jq -c --arg event "$event" --argjson pass "$case_pass" --argjson legacy "$legacy" --argjson go "$go" '. + [{event:$event,pass:$pass,legacy:$legacy,go:$go}]')"
done

jq -nc --argjson pass "$pass" --arg normalization "random amounts, variants, combat rounds, and survivor counts are reduced to stable event contracts" --argjson cases "$results" '{pass:$pass,normalization:$normalization,cases:$cases}' > "$REPORT"
[ "$pass" = true ]
printf 'Go/PHP expedition differential E2E: PASS (10 lifecycle cases)\n'
