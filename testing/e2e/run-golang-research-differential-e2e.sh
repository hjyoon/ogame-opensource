#!/bin/sh
set -eu
[ "${OGAME_E2E_TRACE:-0}" != "1" ] && [ "${1:-}" != "--trace" ] || set -x

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_RESEARCH_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-research-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/research-differential.XXXXXX")"

login="$(jq -r '.queue_cancel.research.login // empty' "$FIXTURE")"
player_id="$(jq -r '.queue_cancel.research.player_id // 0' "$FIXTURE")"
planet_id="$(jq -r '.queue_cancel.research.home_planet_id // 0' "$FIXTURE")"
research_id="$(jq -r '.queue_cancel.research.tech_id // 0' "$FIXTURE")"
[ -n "$login" ] && [ "$player_id" -gt 0 ] && [ "$planet_id" -gt 0 ] && [ "$research_id" -gt 0 ]

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'exec mysql -N -B -uroot -p"$MYSQL_ROOT_PASSWORD" uni -e "$1"' sh "$1" 2>/dev/null
}

planet_sql="SELECT p.\`700\`,p.\`701\`,p.\`702\`,u.\`$research_id\`,p.fields,p.maxfields,p.lastpeek,u.score1,u.score2,u.score3,u.place1,u.place2,u.place3 FROM uni1_planets p JOIN uni1_users u ON u.player_id=p.owner_id WHERE p.planet_id=$planet_id AND p.owner_id=$player_id LIMIT 1"
initial="$(db_query "$planet_sql")"
[ -n "$initial" ]
old_ifs="$IFS"
IFS="$(printf '\t')"
set -- $initial
IFS="$old_ifs"
[ "$#" -eq 13 ]
original_metal="$1"
original_crystal="$2"
original_deuterium="$3"
original_level="$4"
original_fields="$5"
original_maxfields="$6"
original_lastpeek="$7"
original_score1="$8"
original_score2="$9"
shift 9
original_score3="$1"
original_place1="$2"
original_place2="$3"
original_place3="$4"
original_prod="$(db_query "SELECT prod1,prod2,prod3,prod4,prod12,prod212 FROM uni1_planets WHERE planet_id=$planet_id AND owner_id=$player_id LIMIT 1")"
IFS="$(printf '\t')"
set -- $original_prod
IFS="$old_ifs"
[ "$#" -eq 6 ]
original_prod1="$1" original_prod2="$2" original_prod3="$3" original_prod4="$4" original_prod12="$5" original_prod212="$6"
original_user_flags="$(db_query "SELECT vacation,tec_until FROM uni1_users WHERE player_id=$player_id LIMIT 1")"
IFS="$(printf '\t')"
set -- $original_user_flags
IFS="$old_ifs"
[ "$#" -eq 2 ]
original_vacation="$1" original_tec_until="$2"
initial_log_id="$(db_query "SELECT COALESCE(MAX(id),0) FROM uni1_userlogs WHERE owner_id=$player_id")"
initial_debug_id="$(db_query "SELECT COALESCE(MAX(error_id),0) FROM uni1_debug")"
backup_table="uni1_e2e_research_users_$$"
db_query "DROP TABLE IF EXISTS $backup_table; CREATE TABLE $backup_table AS SELECT player_id,score1,score2,score3,place1,place2,place3 FROM uni1_users" >/dev/null

restore_original() {
  db_query "DELETE FROM uni1_queue WHERE owner_id=$player_id AND type='Research'; DELETE FROM uni1_userlogs WHERE owner_id=$player_id AND id>$initial_log_id; DELETE FROM uni1_debug WHERE error_id>$initial_debug_id; UPDATE uni1_users u JOIN $backup_table b ON b.player_id=u.player_id SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3; UPDATE uni1_planets SET \`700\`=$original_metal,\`701\`=$original_crystal,\`702\`=$original_deuterium,fields=$original_fields,maxfields=$original_maxfields,lastpeek=$original_lastpeek,prod1=$original_prod1,prod2=$original_prod2,prod3=$original_prod3,prod4=$original_prod4,prod12=$original_prod12,prod212=$original_prod212 WHERE planet_id=$planet_id AND owner_id=$player_id; UPDATE uni1_users SET \`$research_id\`=$original_level,vacation=$original_vacation,tec_until=$original_tec_until WHERE player_id=$player_id; DROP TABLE IF EXISTS $backup_table" >/dev/null
}

restore_case() {
  db_query "DELETE FROM uni1_queue WHERE owner_id=$player_id AND type='Research'; DELETE FROM uni1_userlogs WHERE owner_id=$player_id AND id>$initial_log_id; DELETE FROM uni1_debug WHERE error_id>$initial_debug_id; UPDATE uni1_users u JOIN $backup_table b ON b.player_id=u.player_id SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3; UPDATE uni1_planets SET \`700\`=10000000,\`701\`=10000000,\`702\`=10000000,fields=0,maxfields=200,lastpeek=UNIX_TIMESTAMP(),prod1=0,prod2=0,prod3=0,prod4=0,prod12=0,prod212=0 WHERE planet_id=$planet_id AND owner_id=$player_id; UPDATE uni1_users SET \`$research_id\`=0,score1=0,score2=0,score3=0,vacation=0,tec_until=0 WHERE player_id=$player_id" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

capture_state() {
  planet="$(db_query "$planet_sql")"
  old_ifs="$IFS"
  IFS="$(printf '\t')"
  set -- $planet
  IFS="$old_ifs"
  [ "$#" -eq 13 ]
  metal="$1" crystal="$2" deuterium="$3" level="$4" fields="$5" maxfields="$6" lastpeek="$7" score1="$8" score2="$9"
  shift 9
  score3="$1"
  place1="$2"
  place2="$3"
  place3="$4"
  task="$(db_query "SELECT type,sub_id,obj_id,level,end-start,prio,freeze,frozen FROM uni1_queue WHERE owner_id=$player_id AND type='Research' ORDER BY task_id LIMIT 1")"
  task_json=null
  if [ -n "$task" ]; then
    IFS="$(printf '\t')"; set -- $task; IFS="$old_ifs"
    task_json="$(jq -nc --arg type "$1" --arg planet "$2" --arg obj "$3" --arg level "$4" --arg duration "$5" --arg prio "$6" --arg freeze "$7" --arg frozen "$8" '{type:$type,planetId:($planet|tonumber),objId:($obj|tonumber),level:($level|tonumber),duration:($duration|tonumber),prio:($prio|tonumber),freeze:($freeze|tonumber),frozen:($frozen|tonumber)}')"
  fi
  jq -nc --arg metal "$metal" --arg crystal "$crystal" --arg deuterium "$deuterium" \
    --arg level "$level" --arg fields "$fields" --arg maxfields "$maxfields" \
    --arg score1 "$score1" --arg score2 "$score2" --arg score3 "$score3" --arg place1 "$place1" --arg place2 "$place2" --arg place3 "$place3" \
    --argjson task "$task_json" \
    '{planet:{metal:($metal|tonumber),crystal:($crystal|tonumber),deuterium:($deuterium|tonumber),fields:($fields|tonumber),maxFields:($maxfields|tonumber)},user:{researchLevel:($level|tonumber),score1:($score1|tonumber),score2:($score2|tonumber),score3:($score3|tonumber),place1:($place1|tonumber),place2:($place2|tonumber),place3:($place3|tonumber)},queueTask:$task}'
}

login_legacy() {
  prefix="$1"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" \
    --output "$TMP_DIR/$prefix-login.body" --cookie-jar "$TMP_DIR/$prefix.cookies" --request POST \
    --data-urlencode "login=$login" --data-urlencode "pass=$PASSWORD" --write-out '%{http_code}' \
    "$LEGACY_BASE_URL/game/reg/login2.php")"
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/$prefix-login.headers")"
  legacy_session="$(printf '%s' "$location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 302 ] && [ -n "$legacy_session" ]
}

login_go() {
  prefix="$1"
  payload="$(jq -nc --arg login "$login" --arg pass "$PASSWORD" '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" \
    --output "$TMP_DIR/$prefix-login.body" --header 'Content-Type: application/json' --data "$payload" \
    --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  go_cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/$prefix-login.headers" | cut -d';' -f1 | tr -d '\r')"
  redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/$prefix-login.body")"
  go_session="$(printf '%s' "$redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 200 ] && [ -n "$go_cookie" ] && [ -n "$go_session" ]
}

run_legacy() {
  mode="$1"
  restore_case
  login_legacy "legacy-$mode"
  legacy_before="$(capture_state)"
  legacy_enqueue_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/legacy-$mode-enqueue.body" \
    --cookie "$TMP_DIR/legacy-$mode.cookies" --write-out '%{http_code}' \
    "$LEGACY_BASE_URL/game/index.php?page=buildings&mode=Forschung&session=$legacy_session&cp=$planet_id&bau=$research_id")"
  legacy_enqueued="$(capture_state)"
  if [ "$mode" = cancel ]; then
    legacy_final_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/legacy-$mode-final.body" \
      --cookie "$TMP_DIR/legacy-$mode.cookies" --write-out '%{http_code}' \
      "$LEGACY_BASE_URL/game/index.php?page=buildings&mode=Forschung&session=$legacy_session&cp=$planet_id&unbau=$research_id")"
  else
    db_query "UPDATE uni1_queue SET start=UNIX_TIMESTAMP()-2,end=UNIX_TIMESTAMP()-1 WHERE owner_id=$player_id AND type='Research'" >/dev/null
    legacy_final_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/legacy-$mode-final.body" \
      --cookie "$TMP_DIR/legacy-$mode.cookies" --write-out '%{http_code}' \
      "$LEGACY_BASE_URL/game/index.php?page=buildings&mode=Forschung&session=$legacy_session&cp=$planet_id")"
  fi
  legacy_final="$(capture_state)"
}

run_go() {
  mode="$1"
  restore_case
  login_go "go-$mode"
  go_before="$(capture_state)"
  go_enqueue_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/go-$mode-enqueue.body" \
    --cookie "$go_cookie" --header 'Content-Type: application/json' --data "{\"action\":\"start\",\"techId\":$research_id}" \
    --write-out '%{http_code}' "$GO_BASE_URL/api/game/research?session=$go_session&cp=$planet_id")"
  go_enqueued="$(capture_state)"
  if [ "$mode" = cancel ]; then
    go_final_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/go-$mode-final.body" \
      --cookie "$go_cookie" --header 'Content-Type: application/json' --data "{\"action\":\"cancel\",\"techId\":$research_id}" \
      --write-out '%{http_code}' "$GO_BASE_URL/api/game/research?session=$go_session&cp=$planet_id")"
  else
    db_query "UPDATE uni1_queue SET start=UNIX_TIMESTAMP()-2,end=UNIX_TIMESTAMP()-1 WHERE owner_id=$player_id AND type='Research'" >/dev/null
    go_final_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/go-$mode-final.body" \
      --cookie "$go_cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/research?session=$go_session&cp=$planet_id")"
  fi
  go_final="$(capture_state)"
}

all_pass=true
case_results="$TMP_DIR/cases.jsonl"
: > "$case_results"

run_case() {
  mode="$1"
  run_legacy "$mode"
  run_go "$mode"
  case_pass=true
  [ "$legacy_enqueue_status" = 200 ] && [ "$go_enqueue_status" = 200 ] || case_pass=false
  [ "$legacy_final_status" = 200 ] && [ "$go_final_status" = 200 ] || case_pass=false
  [ "$legacy_before" = "$go_before" ] || case_pass=false
  [ "$legacy_enqueued" = "$go_enqueued" ] || case_pass=false
  [ "$legacy_final" = "$go_final" ] || case_pass=false
  jq -e '.queueTask != null and .planet.crystal < $before.planet.crystal and .planet.deuterium < $before.planet.deuterium' \
    --argjson before "$legacy_before" >/dev/null <<EOF || case_pass=false
$legacy_enqueued
EOF
  if [ "$mode" = cancel ]; then
    [ "$legacy_final" = "$legacy_before" ] || case_pass=false
  else
    jq -e '.queueTask == null and .user.researchLevel == ($before.user.researchLevel + 1) and .planet.fields == $before.planet.fields and .user.score1 > $before.user.score1 and .user.score3 == ($before.user.score3 + 1)' \
      --argjson before "$legacy_before" >/dev/null <<EOF || case_pass=false
$legacy_final
EOF
  fi
  [ "$case_pass" = true ] || all_pass=false
  jq -nc --arg name "research-$mode" --argjson pass "$case_pass" \
    --arg legacyEnqueueStatus "$legacy_enqueue_status" --arg goEnqueueStatus "$go_enqueue_status" \
    --arg legacyFinalStatus "$legacy_final_status" --arg goFinalStatus "$go_final_status" \
    --argjson before "$legacy_before" --argjson legacyEnqueued "$legacy_enqueued" --argjson goEnqueued "$go_enqueued" \
    --argjson legacyFinal "$legacy_final" --argjson goFinal "$go_final" \
    '{name:$name,pass:$pass,http:{enqueue:{legacy:$legacyEnqueueStatus,go:$goEnqueueStatus},final:{legacy:$legacyFinalStatus,go:$goFinalStatus}},db:{before:$before,enqueued:{legacy:$legacyEnqueued,go:$goEnqueued},final:{legacy:$legacyFinal,go:$goFinal}}}' >> "$case_results"
}

run_case cancel
run_case completion
jq -s --argjson pass "$all_pass" --arg login "$login" --argjson playerId "$player_id" --argjson planetId "$planet_id" \
  '{pass:$pass,fixture:{login:$login,playerId:$playerId,planetId:$planetId},cases:.}' "$case_results" > "$REPORT"
[ "$all_pass" = true ]
printf 'Go/PHP research differential E2E: PASS (2 cases, planet=%s)\n' "$planet_id"
