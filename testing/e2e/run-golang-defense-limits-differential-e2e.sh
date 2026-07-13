#!/bin/sh
set -eu
[ "${OGAME_E2E_TRACE:-0}" != "1" ] && [ "${1:-}" != "--trace" ] || set -x

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_DEFENSE_LIMITS_REPORT:-$ROOT_DIR/.tmp/golang-defense-limits-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/defense-limits.XXXXXX")"

dome_login="$(jq -r '.concurrency_race.dome.login // empty' "$FIXTURE")"
dome_player="$(jq -r '.concurrency_race.dome.player_id // 0' "$FIXTURE")"
dome_planet="$(jq -r '.concurrency_race.dome.home_planet_id // 0' "$FIXTURE")"
dome_item="$(jq -r '.concurrency_race.dome.defense_id // 0' "$FIXTURE")"
missile_login="$(jq -r '.concurrency_race.missile.login // empty' "$FIXTURE")"
missile_player="$(jq -r '.concurrency_race.missile.player_id // 0' "$FIXTURE")"
missile_planet="$(jq -r '.concurrency_race.missile.home_planet_id // 0' "$FIXTURE")"
missile_item="$(jq -r '.concurrency_race.missile.defense_id // 0' "$FIXTURE")"
mixed_login="$(jq -r '.concurrency_race.defense.login // empty' "$FIXTURE")"
mixed_player="$(jq -r '.concurrency_race.defense.player_id // 0' "$FIXTURE")"
mixed_planet="$(jq -r '.concurrency_race.defense.home_planet_id // 0' "$FIXTURE")"
mixed_first="$(jq -r '.concurrency_race.defense.defense_id // 0' "$FIXTURE")"
mixed_second=402
[ -n "$dome_login" ] && [ -n "$missile_login" ] && [ -n "$mixed_login" ]
[ "$dome_item" -eq 407 ] && [ "$missile_item" -eq 502 ] && [ "$mixed_first" -eq 401 ]

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'exec mysql -N -B -uroot -p"$MYSQL_ROOT_PASSWORD" uni -e "$1"' sh "$1" 2>/dev/null
}

players="$dome_player,$missile_player,$mixed_player"
planets="$dome_planet,$missile_planet,$mixed_planet"
users_backup="uni1_e2e_deflimit_users_$$"
planets_backup="uni1_e2e_deflimit_planets_$$"
queue_backup="uni1_e2e_deflimit_queue_$$"
initial_log_id="$(db_query "SELECT COALESCE(MAX(id),0) FROM uni1_userlogs")"
db_query "DROP TABLE IF EXISTS $users_backup,$planets_backup,$queue_backup; CREATE TABLE $users_backup AS SELECT player_id,score1,score2,score3,place1,place2,place3,vacation,com_until FROM uni1_users; CREATE TABLE $planets_backup AS SELECT planet_id,\`700\`,\`701\`,\`702\`,\`44\`,\`401\`,\`402\`,\`407\`,\`502\`,\`503\`,lastpeek,prod1,prod2,prod3,prod4,prod12,prod212 FROM uni1_planets WHERE planet_id IN ($planets); CREATE TABLE $queue_backup AS SELECT * FROM uni1_queue WHERE type='Shipyard' AND (owner_id IN ($players) OR sub_id IN ($planets))" >/dev/null

restore_original() {
  db_query "DELETE FROM uni1_queue WHERE type='Shipyard' AND (owner_id IN ($players) OR sub_id IN ($planets)); INSERT INTO uni1_queue SELECT * FROM $queue_backup; DELETE FROM uni1_userlogs WHERE owner_id IN ($players) AND id>$initial_log_id; UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3,u.vacation=b.vacation,u.com_until=b.com_until; UPDATE uni1_planets p JOIN $planets_backup b ON b.planet_id=p.planet_id SET p.\`700\`=b.\`700\`,p.\`701\`=b.\`701\`,p.\`702\`=b.\`702\`,p.\`44\`=b.\`44\`,p.\`401\`=b.\`401\`,p.\`402\`=b.\`402\`,p.\`407\`=b.\`407\`,p.\`502\`=b.\`502\`,p.\`503\`=b.\`503\`,p.lastpeek=b.lastpeek,p.prod1=b.prod1,p.prod2=b.prod2,p.prod3=b.prod3,p.prod4=b.prod4,p.prod12=b.prod12,p.prod212=b.prod212; DROP TABLE IF EXISTS $users_backup,$planets_backup,$queue_backup" >/dev/null
}

reset_case() {
  db_query "DELETE FROM uni1_queue WHERE type='Shipyard' AND (owner_id IN ($players) OR sub_id IN ($planets)); DELETE FROM uni1_userlogs WHERE owner_id IN ($players) AND id>$initial_log_id; UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3,u.vacation=b.vacation,u.com_until=b.com_until; UPDATE uni1_planets p JOIN $planets_backup b ON b.planet_id=p.planet_id SET p.\`700\`=b.\`700\`,p.\`701\`=b.\`701\`,p.\`702\`=b.\`702\`,p.\`44\`=b.\`44\`,p.\`401\`=b.\`401\`,p.\`402\`=b.\`402\`,p.\`407\`=b.\`407\`,p.\`502\`=b.\`502\`,p.\`503\`=b.\`503\`,p.lastpeek=b.lastpeek,p.prod1=b.prod1,p.prod2=b.prod2,p.prod3=b.prod3,p.prod4=b.prod4,p.prod12=b.prod12,p.prod212=b.prod212; UPDATE uni1_planets SET \`700\`=100000000,\`701\`=100000000,\`702\`=100000000,\`$item_one\`=0,\`$item_two\`=0,\`44\`=$silo_level,\`502\`=0,\`503\`=0,lastpeek=UNIX_TIMESTAMP(),prod1=0,prod2=0,prod3=0,prod4=0,prod12=0,prod212=0 WHERE planet_id=$current_planet AND owner_id=$current_player; UPDATE uni1_users SET score1=0,score2=0,score3=0,vacation=0,com_until=0 WHERE player_id=$current_player" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

select_case() {
  case_name="$1"
  case "$case_name" in
    dome) current_login="$dome_login"; current_player="$dome_player"; current_planet="$dome_planet"; item_one=407; item_two=401; amount_one=4; amount_two=0; expected_one=1; expected_two=0; silo_level=0; repeat_order=1 ;;
    missile) current_login="$missile_login"; current_player="$missile_player"; current_planet="$missile_planet"; item_one=502; item_two=503; amount_one=99; amount_two=0; expected_one=20; expected_two=0; silo_level=2; repeat_order=0 ;;
    mixed) current_login="$mixed_login"; current_player="$mixed_player"; current_planet="$mixed_planet"; item_one=401; item_two=402; amount_one=2; amount_two=3; expected_one=2; expected_two=3; silo_level=0; repeat_order=0 ;;
  esac
  state_sql="SELECT p.\`700\`,p.\`701\`,p.\`702\`,p.\`$item_one\`,p.\`$item_two\`,u.score1,u.score2,u.score3,u.place1,u.place2,u.place3 FROM uni1_planets p JOIN uni1_users u ON u.player_id=p.owner_id WHERE p.planet_id=$current_planet AND p.owner_id=$current_player LIMIT 1"
}

capture_state() {
  state="$(db_query "$state_sql")"
  old_ifs="$IFS"; IFS="$(printf '\t')"; set -- $state; IFS="$old_ifs"
  [ "$#" -eq 11 ]
  metal="$1" crystal="$2" deuterium="$3" count_one="$4" count_two="$5" score1="$6" score2="$7" score3="$8" place1="$9"
  shift 9
  place2="$1" place3="$2"
  tasks="$(db_query "SELECT COALESCE(CONCAT('[',GROUP_CONCAT(JSON_OBJECT('objId',obj_id,'count',level,'duration',end-start,'startOffset',start-base_start,'endOffset',end-base_start,'prio',prio) ORDER BY start,task_id SEPARATOR ','),']'),'[]') FROM (SELECT q.*,MIN(start) OVER () AS base_start FROM uni1_queue q WHERE owner_id=$current_player AND type='Shipyard' AND sub_id=$current_planet) x")"
  tasks="$(printf '%s' "$tasks" | jq -cS '.')"
  jq -nc --arg metal "$metal" --arg crystal "$crystal" --arg deuterium "$deuterium" --arg one "$count_one" --arg two "$count_two" --arg score1 "$score1" --arg score2 "$score2" --arg score3 "$score3" --arg place1 "$place1" --arg place2 "$place2" --arg place3 "$place3" --argjson tasks "$tasks" '{planet:{metal:($metal|tonumber),crystal:($crystal|tonumber),deuterium:($deuterium|tonumber),countOne:($one|tonumber),countTwo:($two|tonumber)},user:{score1:($score1|tonumber),score2:($score2|tonumber),score3:($score3|tonumber),place1:($place1|tonumber),place2:($place2|tonumber),place3:($place3|tonumber)},queueTasks:$tasks}'
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

legacy_order() {
  prefix="$1"
  if [ "$amount_two" -gt 0 ]; then
    curl --silent --show-error --max-time 15 --output "$TMP_DIR/$prefix.body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --data-urlencode "fmenge[$item_one]=$amount_one" --data-urlencode "fmenge[$item_two]=$amount_two" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=buildings&mode=Verteidigung&session=$legacy_session&cp=$current_planet"
  else
    curl --silent --show-error --max-time 15 --output "$TMP_DIR/$prefix.body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --data-urlencode "fmenge[$item_one]=$amount_one" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=buildings&mode=Verteidigung&session=$legacy_session&cp=$current_planet"
  fi
}

go_order() {
  prefix="$1"
  if [ "$amount_two" -gt 0 ]; then body="{\"orders\":{\"$item_one\":$amount_one,\"$item_two\":$amount_two}}"; else body="{\"orders\":{\"$item_one\":$amount_one}}"; fi
  curl --silent --show-error --max-time 15 --output "$TMP_DIR/$prefix.body" --cookie "$go_cookie" --header 'Content-Type: application/json' --data "$body" --write-out '%{http_code}' "$GO_BASE_URL/api/game/defense?session=$go_session&cp=$current_planet"
}

force_all_due() {
  db_query "UPDATE uni1_queue SET start=UNIX_TIMESTAMP()-(level*100),end=UNIX_TIMESTAMP()-((level-1)*100) WHERE owner_id=$current_player AND type='Shipyard' AND sub_id=$current_planet" >/dev/null
}

trigger_legacy() {
  curl --silent --show-error --max-time 15 --output "$TMP_DIR/legacy-$case_name-final.body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=buildings&mode=Verteidigung&session=$legacy_session&cp=$current_planet"
}

trigger_go() {
  curl --silent --show-error --max-time 15 --output "$TMP_DIR/go-$case_name-final.body" --cookie "$go_cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/defense?session=$go_session&cp=$current_planet"
}

run_case() {
  select_case "$1"
  reset_case; login_legacy "legacy-$case_name"
  legacy_before="$(capture_state)"; legacy_order_status="$(legacy_order legacy-$case_name-order)"; legacy_ordered="$(capture_state)"
  legacy_repeat_status=200; legacy_repeated="$legacy_ordered"
  if [ "$repeat_order" -eq 1 ]; then legacy_repeat_status="$(legacy_order legacy-$case_name-repeat)"; legacy_repeated="$(capture_state)"; fi
  force_all_due; legacy_final_status="$(trigger_legacy)"; legacy_final="$(capture_state)"
  reset_case; login_go "go-$case_name"
  go_before="$(capture_state)"; go_order_status="$(go_order go-$case_name-order)"; go_ordered="$(capture_state)"
  go_repeat_status=200; go_repeated="$go_ordered"
  if [ "$repeat_order" -eq 1 ]; then go_repeat_status="$(go_order go-$case_name-repeat)"; go_repeated="$(capture_state)"; fi
  force_all_due; go_final_status="$(trigger_go)"; go_final="$(capture_state)"
  case_pass=true
  [ "$legacy_order_status" = 200 ] && [ "$go_order_status" = 200 ] && [ "$legacy_repeat_status" = 200 ] && [ "$go_repeat_status" = 200 ] && [ "$legacy_final_status" = 200 ] && [ "$go_final_status" = 200 ] || case_pass=false
  [ "$legacy_before" = "$go_before" ] && [ "$legacy_ordered" = "$go_ordered" ] && [ "$legacy_repeated" = "$go_repeated" ] && [ "$legacy_final" = "$go_final" ] || case_pass=false
  jq -e --argjson one "$item_one" --argjson expectedOne "$expected_one" --argjson two "$item_two" --argjson expectedTwo "$expected_two" '(([.queueTasks[] | select(.objId == $one) | .count] | add // 0) == $expectedOne) and (([.queueTasks[] | select(.objId == $two) | .count] | add // 0) == $expectedTwo)' >/dev/null <<EOF || case_pass=false
$legacy_ordered
EOF
  if [ "$repeat_order" -eq 1 ]; then [ "$legacy_repeated" = "$legacy_ordered" ] || case_pass=false; fi
  jq -e --argjson one "$expected_one" --argjson two "$expected_two" '.planet.countOne==$one and .planet.countTwo==$two and (.queueTasks|length)==0 and .user.score1>0' >/dev/null <<EOF || case_pass=false
$legacy_final
EOF
  [ "$case_pass" = true ] || all_pass=false
  jq -nc --arg name "defense-$case_name" --argjson pass "$case_pass" --arg legacyOrder "$legacy_order_status" --arg goOrder "$go_order_status" --arg legacyRepeat "$legacy_repeat_status" --arg goRepeat "$go_repeat_status" --arg legacyFinalStatus "$legacy_final_status" --arg goFinalStatus "$go_final_status" --argjson before "$legacy_before" --argjson legacyOrdered "$legacy_ordered" --argjson goOrdered "$go_ordered" --argjson legacyRepeated "$legacy_repeated" --argjson goRepeated "$go_repeated" --argjson legacyFinal "$legacy_final" --argjson goFinal "$go_final" '{name:$name,pass:$pass,http:{order:{legacy:$legacyOrder,go:$goOrder},repeat:{legacy:$legacyRepeat,go:$goRepeat},final:{legacy:$legacyFinalStatus,go:$goFinalStatus}},db:{before:$before,ordered:{legacy:$legacyOrdered,go:$goOrdered},repeated:{legacy:$legacyRepeated,go:$goRepeated},final:{legacy:$legacyFinal,go:$goFinal}}}' >> "$case_results"
}

all_pass=true
case_results="$TMP_DIR/cases.jsonl"
: > "$case_results"
run_case dome
run_case missile
run_case mixed
jq -s --argjson pass "$all_pass" '{pass:$pass,cases:.}' "$case_results" > "$REPORT"
[ "$all_pass" = true ]
printf 'Go/PHP defense limits differential E2E: PASS (3 cases)\n'
