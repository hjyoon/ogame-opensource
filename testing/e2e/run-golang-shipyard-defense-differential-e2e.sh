#!/bin/sh
set -eu
[ "${OGAME_E2E_TRACE:-0}" != "1" ] && [ "${1:-}" != "--trace" ] || set -x

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_SHIPYARD_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-shipyard-defense-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/shipyard-differential.XXXXXX")"

ship_login="$(jq -r '.concurrency_race.shipyard.login // empty' "$FIXTURE")"
ship_player="$(jq -r '.concurrency_race.shipyard.player_id // 0' "$FIXTURE")"
ship_planet="$(jq -r '.concurrency_race.shipyard.home_planet_id // 0' "$FIXTURE")"
ship_item="$(jq -r '.concurrency_race.shipyard.ship_id // 0' "$FIXTURE")"
ship_count="$(jq -r '.concurrency_race.shipyard.expected_count // 0' "$FIXTURE")"
def_login="$(jq -r '.concurrency_race.defense.login // empty' "$FIXTURE")"
def_player="$(jq -r '.concurrency_race.defense.player_id // 0' "$FIXTURE")"
def_planet="$(jq -r '.concurrency_race.defense.home_planet_id // 0' "$FIXTURE")"
def_item="$(jq -r '.concurrency_race.defense.defense_id // 0' "$FIXTURE")"
def_count="$(jq -r '.concurrency_race.defense.expected_count // 0' "$FIXTURE")"
[ -n "$ship_login" ] && [ -n "$def_login" ] && [ "$ship_player" -gt 0 ] && [ "$def_player" -gt 0 ]
[ "$ship_planet" -gt 0 ] && [ "$def_planet" -gt 0 ] && [ "$ship_item" -gt 0 ] && [ "$def_item" -gt 0 ]
[ "$ship_count" -eq 3 ] && [ "$def_count" -eq 3 ]

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'exec mysql -N -B -uroot -p"$MYSQL_ROOT_PASSWORD" uni -e "$1"' sh "$1" 2>/dev/null
}

users_backup="uni1_e2e_shipyard_users_$$"
planets_backup="uni1_e2e_shipyard_planets_$$"
queue_backup="uni1_e2e_shipyard_queue_$$"
initial_log_id="$(db_query "SELECT COALESCE(MAX(id),0) FROM uni1_userlogs")"
db_query "DROP TABLE IF EXISTS $users_backup,$planets_backup,$queue_backup; CREATE TABLE $users_backup AS SELECT player_id,score1,score2,score3,place1,place2,place3,vacation,com_until FROM uni1_users; CREATE TABLE $planets_backup AS SELECT planet_id,\`700\`,\`701\`,\`702\`,\`$ship_item\`,\`$def_item\`,lastpeek,prod1,prod2,prod3,prod4,prod12,prod212 FROM uni1_planets WHERE planet_id IN ($ship_planet,$def_planet); CREATE TABLE $queue_backup AS SELECT * FROM uni1_queue WHERE type='Shipyard' AND (owner_id IN ($ship_player,$def_player) OR sub_id IN ($ship_planet,$def_planet))" >/dev/null

restore_original() {
  db_query "DELETE FROM uni1_queue WHERE type='Shipyard' AND (owner_id IN ($ship_player,$def_player) OR sub_id IN ($ship_planet,$def_planet)); INSERT INTO uni1_queue SELECT * FROM $queue_backup; DELETE FROM uni1_userlogs WHERE owner_id IN ($ship_player,$def_player) AND id>$initial_log_id; UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3,u.vacation=b.vacation,u.com_until=b.com_until; UPDATE uni1_planets p JOIN $planets_backup b ON b.planet_id=p.planet_id SET p.\`700\`=b.\`700\`,p.\`701\`=b.\`701\`,p.\`702\`=b.\`702\`,p.\`$ship_item\`=b.\`$ship_item\`,p.\`$def_item\`=b.\`$def_item\`,p.lastpeek=b.lastpeek,p.prod1=b.prod1,p.prod2=b.prod2,p.prod3=b.prod3,p.prod4=b.prod4,p.prod12=b.prod12,p.prod212=b.prod212; DROP TABLE IF EXISTS $users_backup,$planets_backup,$queue_backup" >/dev/null
}

reset_case() {
  db_query "DELETE FROM uni1_queue WHERE type='Shipyard' AND (owner_id IN ($ship_player,$def_player) OR sub_id IN ($ship_planet,$def_planet)); DELETE FROM uni1_userlogs WHERE owner_id IN ($ship_player,$def_player) AND id>$initial_log_id; UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3,u.vacation=b.vacation,u.com_until=b.com_until; UPDATE uni1_planets p JOIN $planets_backup b ON b.planet_id=p.planet_id SET p.\`700\`=b.\`700\`,p.\`701\`=b.\`701\`,p.\`702\`=b.\`702\`,p.\`$ship_item\`=b.\`$ship_item\`,p.\`$def_item\`=b.\`$def_item\`,p.lastpeek=b.lastpeek,p.prod1=b.prod1,p.prod2=b.prod2,p.prod3=b.prod3,p.prod4=b.prod4,p.prod12=b.prod12,p.prod212=b.prod212; UPDATE uni1_planets SET \`700\`=10000000,\`701\`=10000000,\`702\`=10000000,\`$current_item\`=0,lastpeek=UNIX_TIMESTAMP(),prod1=0,prod2=0,prod3=0,prod4=0,prod12=0,prod212=0 WHERE planet_id=$current_planet AND owner_id=$current_player; UPDATE uni1_users SET score1=0,score2=0,score3=0,vacation=0,com_until=0 WHERE player_id=$current_player" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

select_case() {
  current_kind="$1"
  if [ "$current_kind" = shipyard ]; then
    current_login="$ship_login" current_player="$ship_player" current_planet="$ship_planet"
    current_item="$ship_item" current_count="$ship_count" legacy_mode=Flotte go_api=shipyard fleet_score=1
  else
    current_login="$def_login" current_player="$def_player" current_planet="$def_planet"
    current_item="$def_item" current_count="$def_count" legacy_mode=Verteidigung go_api=defense fleet_score=0
  fi
  state_sql="SELECT p.\`700\`,p.\`701\`,p.\`702\`,p.\`$current_item\`,u.score1,u.score2,u.score3,u.place1,u.place2,u.place3 FROM uni1_planets p JOIN uni1_users u ON u.player_id=p.owner_id WHERE p.planet_id=$current_planet AND p.owner_id=$current_player LIMIT 1"
}

capture_state() {
  state="$(db_query "$state_sql")"
  old_ifs="$IFS"; IFS="$(printf '\t')"; set -- $state; IFS="$old_ifs"
  [ "$#" -eq 10 ]
  metal="$1" crystal="$2" deuterium="$3" item_count="$4" score1="$5" score2="$6" score3="$7" place1="$8" place2="$9"
  shift 9
  place3="$1"
  task="$(db_query "SELECT type,sub_id,obj_id,level,end-start,prio,freeze,frozen FROM uni1_queue WHERE owner_id=$current_player AND type='Shipyard' AND sub_id=$current_planet ORDER BY task_id LIMIT 1")"
  task_json=null
  if [ -n "$task" ]; then
    IFS="$(printf '\t')"; set -- $task; IFS="$old_ifs"
    task_json="$(jq -nc --arg type "$1" --arg planet "$2" --arg obj "$3" --arg level "$4" --arg duration "$5" --arg prio "$6" --arg freeze "$7" --arg frozen "$8" '{type:$type,planetId:($planet|tonumber),objId:($obj|tonumber),count:($level|tonumber),duration:($duration|tonumber),prio:($prio|tonumber),freeze:($freeze|tonumber),frozen:($frozen|tonumber)}')"
  fi
  jq -nc --arg metal "$metal" --arg crystal "$crystal" --arg deuterium "$deuterium" --arg count "$item_count" \
    --arg score1 "$score1" --arg score2 "$score2" --arg score3 "$score3" --arg place1 "$place1" --arg place2 "$place2" --arg place3 "$place3" --argjson task "$task_json" \
    '{planet:{metal:($metal|tonumber),crystal:($crystal|tonumber),deuterium:($deuterium|tonumber),itemCount:($count|tonumber)},user:{score1:($score1|tonumber),score2:($score2|tonumber),score3:($score3|tonumber),place1:($place1|tonumber),place2:($place2|tonumber),place3:($place3|tonumber)},queueTask:$task}'
}

login_legacy() {
  prefix="$1"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" --output "$TMP_DIR/$prefix-login.body" --cookie-jar "$TMP_DIR/$prefix.cookies" --request POST --data-urlencode "login=$current_login" --data-urlencode "pass=$PASSWORD" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/reg/login2.php")"
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/$prefix-login.headers")"
  legacy_session="$(printf '%s' "$location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 302 ] && [ -n "$legacy_session" ]
}

login_go() {
  prefix="$1"
  payload="$(jq -nc --arg login "$current_login" --arg pass "$PASSWORD" '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" --output "$TMP_DIR/$prefix-login.body" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  go_cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/$prefix-login.headers" | cut -d';' -f1 | tr -d '\r')"
  redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/$prefix-login.body")"
  go_session="$(printf '%s' "$redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 200 ] && [ -n "$go_cookie" ] && [ -n "$go_session" ]
}

force_partial_due() {
  db_query "UPDATE uni1_queue SET start=UNIX_TIMESTAMP()-100,end=UNIX_TIMESTAMP() WHERE owner_id=$current_player AND type='Shipyard' AND sub_id=$current_planet" >/dev/null
}

force_all_due() {
  db_query "UPDATE uni1_queue SET start=UNIX_TIMESTAMP()-200,end=UNIX_TIMESTAMP()-100 WHERE owner_id=$current_player AND type='Shipyard' AND sub_id=$current_planet" >/dev/null
}

run_legacy() {
  reset_case
  login_legacy "legacy-$current_kind"
  legacy_before="$(capture_state)"
  legacy_enqueue_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/legacy-$current_kind-enqueue.body" --cookie "$TMP_DIR/legacy-$current_kind.cookies" --data-urlencode "fmenge[$current_item]=$current_count" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=buildings&mode=$legacy_mode&session=$legacy_session&cp=$current_planet")"
  legacy_enqueued="$(capture_state)"
  force_partial_due
  legacy_partial_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/legacy-$current_kind-partial.body" --cookie "$TMP_DIR/legacy-$current_kind.cookies" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=buildings&mode=$legacy_mode&session=$legacy_session&cp=$current_planet")"
  legacy_partial="$(capture_state)"
  force_all_due
  legacy_final_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/legacy-$current_kind-final.body" --cookie "$TMP_DIR/legacy-$current_kind.cookies" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=buildings&mode=$legacy_mode&session=$legacy_session&cp=$current_planet")"
  legacy_final="$(capture_state)"
}

run_go() {
  reset_case
  login_go "go-$current_kind"
  go_before="$(capture_state)"
  go_enqueue_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/go-$current_kind-enqueue.body" --cookie "$go_cookie" --header 'Content-Type: application/json' --data "{\"orders\":{\"$current_item\":$current_count}}" --write-out '%{http_code}' "$GO_BASE_URL/api/game/$go_api?session=$go_session&cp=$current_planet")"
  go_enqueued="$(capture_state)"
  force_partial_due
  go_partial_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/go-$current_kind-partial.body" --cookie "$go_cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/$go_api?session=$go_session&cp=$current_planet")"
  go_partial="$(capture_state)"
  force_all_due
  go_final_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/go-$current_kind-final.body" --cookie "$go_cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/$go_api?session=$go_session&cp=$current_planet")"
  go_final="$(capture_state)"
}

all_pass=true
case_results="$TMP_DIR/cases.jsonl"
: > "$case_results"

run_case() {
  select_case "$1"
  run_legacy
  run_go
  case_pass=true
  [ "$legacy_enqueue_status" = 200 ] && [ "$go_enqueue_status" = 200 ] || case_pass=false
  [ "$legacy_partial_status" = 200 ] && [ "$go_partial_status" = 200 ] || case_pass=false
  [ "$legacy_final_status" = 200 ] && [ "$go_final_status" = 200 ] || case_pass=false
  [ "$legacy_before" = "$go_before" ] || case_pass=false
  [ "$legacy_enqueued" = "$go_enqueued" ] || case_pass=false
  [ "$legacy_partial" = "$go_partial" ] || case_pass=false
  [ "$legacy_final" = "$go_final" ] || case_pass=false
  jq -e --argjson item "$current_item" --argjson count "$current_count" '.queueTask.objId == $item and .queueTask.count == $count and .queueTask.prio == 0 and .planet.metal < $before.planet.metal' --argjson before "$legacy_before" >/dev/null <<EOF || case_pass=false
$legacy_enqueued
EOF
  jq -e --argjson remaining "$((current_count - 1))" '.planet.itemCount == 1 and .queueTask.count == $remaining and .queueTask.duration == 100 and .user.score1 > $before.user.score1' --argjson before "$legacy_before" >/dev/null <<EOF || case_pass=false
$legacy_partial
EOF
  jq -e --argjson count "$current_count" --argjson fleet "$fleet_score" '.planet.itemCount == $count and .queueTask == null and .user.score1 > $before.user.score1 and .user.score2 == ($before.user.score2 + ($count * $fleet))' --argjson before "$legacy_before" >/dev/null <<EOF || case_pass=false
$legacy_final
EOF
  [ "$case_pass" = true ] || all_pass=false
  jq -nc --arg name "$current_kind-lifecycle" --argjson pass "$case_pass" --arg legacyEnqueueStatus "$legacy_enqueue_status" --arg goEnqueueStatus "$go_enqueue_status" --arg legacyPartialStatus "$legacy_partial_status" --arg goPartialStatus "$go_partial_status" --arg legacyFinalStatus "$legacy_final_status" --arg goFinalStatus "$go_final_status" --argjson before "$legacy_before" --argjson legacyEnqueued "$legacy_enqueued" --argjson goEnqueued "$go_enqueued" --argjson legacyPartial "$legacy_partial" --argjson goPartial "$go_partial" --argjson legacyFinal "$legacy_final" --argjson goFinal "$go_final" '{name:$name,pass:$pass,http:{enqueue:{legacy:$legacyEnqueueStatus,go:$goEnqueueStatus},partial:{legacy:$legacyPartialStatus,go:$goPartialStatus},final:{legacy:$legacyFinalStatus,go:$goFinalStatus}},db:{before:$before,enqueued:{legacy:$legacyEnqueued,go:$goEnqueued},partial:{legacy:$legacyPartial,go:$goPartial},final:{legacy:$legacyFinal,go:$goFinal}}}' >> "$case_results"
}

run_case shipyard
run_case defense
jq -s --argjson pass "$all_pass" '{pass:$pass,cases:.}' "$case_results" > "$REPORT"
[ "$all_pass" = true ]
printf 'Go/PHP shipyard-defense differential E2E: PASS (2 lifecycle cases)\n'
