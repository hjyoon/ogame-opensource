#!/bin/sh
set -eu
[ "${OGAME_E2E_TRACE:-0}" != "1" ] && [ "${1:-}" != "--trace" ] || set -x

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_BUILDING_ADVANCED_REPORT:-$ROOT_DIR/.tmp/golang-building-advanced-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/building-advanced.XXXXXX")"

login="$(jq -r '.queue_cancel.build.login // empty' "$FIXTURE")"
player_id="$(jq -r '.queue_cancel.build.player_id // 0' "$FIXTURE")"
planet_id="$(jq -r '.queue_cancel.build.home_planet_id // 0' "$FIXTURE")"
building_id="$(jq -r '.queue_cancel.build.building_id // 0' "$FIXTURE")"
[ -n "$login" ] && [ "$player_id" -gt 0 ] && [ "$planet_id" -gt 0 ] && [ "$building_id" -gt 0 ]

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'exec mysql -N -B -uroot -p"$MYSQL_ROOT_PASSWORD" uni -e "$1"' sh "$1" 2>/dev/null
}

planet_sql="SELECT p.\`700\`,p.\`701\`,p.\`702\`,p.\`$building_id\`,p.fields,p.maxfields,p.lastpeek,u.score1,u.score2,u.score3 FROM uni1_planets p JOIN uni1_users u ON u.player_id=p.owner_id WHERE p.planet_id=$planet_id AND p.owner_id=$player_id LIMIT 1"
initial="$(db_query "$planet_sql")"
old_ifs="$IFS"; IFS="$(printf '\t')"; set -- $initial; IFS="$old_ifs"
[ "$#" -eq 10 ]
original_metal="$1" original_crystal="$2" original_deuterium="$3" original_level="$4" original_fields="$5" original_maxfields="$6" original_lastpeek="$7" original_score1="$8" original_score2="$9"
shift 9
original_score3="$1"
original_prod="$(db_query "SELECT prod1,prod2,prod3,prod4,prod12,prod212 FROM uni1_planets WHERE planet_id=$planet_id LIMIT 1")"
IFS="$(printf '\t')"; set -- $original_prod; IFS="$old_ifs"
[ "$#" -eq 6 ]
original_prod1="$1" original_prod2="$2" original_prod3="$3" original_prod4="$4" original_prod12="$5" original_prod212="$6"
original_flags="$(db_query "SELECT vacation,com_until FROM uni1_users WHERE player_id=$player_id LIMIT 1")"
IFS="$(printf '\t')"; set -- $original_flags; IFS="$old_ifs"
[ "$#" -eq 2 ]
original_vacation="$1" original_commander="$2"
initial_log_id="$(db_query "SELECT COALESCE(MAX(id),0) FROM uni1_userlogs WHERE owner_id=$player_id")"

restore_original() {
  db_query "DELETE FROM uni1_queue WHERE owner_id=$player_id AND type IN ('Build','Demolish'); DELETE FROM uni1_buildqueue WHERE owner_id=$player_id OR planet_id=$planet_id; DELETE FROM uni1_userlogs WHERE owner_id=$player_id AND id>$initial_log_id; UPDATE uni1_planets SET \`700\`=$original_metal,\`701\`=$original_crystal,\`702\`=$original_deuterium,\`$building_id\`=$original_level,fields=$original_fields,maxfields=$original_maxfields,lastpeek=$original_lastpeek,prod1=$original_prod1,prod2=$original_prod2,prod3=$original_prod3,prod4=$original_prod4,prod12=$original_prod12,prod212=$original_prod212 WHERE planet_id=$planet_id AND owner_id=$player_id; UPDATE uni1_users SET score1=$original_score1,score2=$original_score2,score3=$original_score3,vacation=$original_vacation,com_until=$original_commander WHERE player_id=$player_id" >/dev/null
}

reset_case() {
  db_query "DELETE FROM uni1_queue WHERE owner_id=$player_id AND type IN ('Build','Demolish'); DELETE FROM uni1_buildqueue WHERE owner_id=$player_id OR planet_id=$planet_id; DELETE FROM uni1_userlogs WHERE owner_id=$player_id AND id>$initial_log_id; UPDATE uni1_planets SET \`700\`=100000000,\`701\`=100000000,\`702\`=100000000,\`$building_id\`=$baseline_level,fields=$baseline_fields,maxfields=200,lastpeek=UNIX_TIMESTAMP(),prod1=0,prod2=0,prod3=0,prod4=0,prod12=0,prod212=0 WHERE planet_id=$planet_id AND owner_id=$player_id; UPDATE uni1_users SET score1=$baseline_score,score2=0,score3=0,vacation=0,com_until=$commander_until WHERE player_id=$player_id" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

capture_state() {
  state="$(db_query "$planet_sql")"
  IFS="$(printf '\t')"; set -- $state; IFS="$old_ifs"
  [ "$#" -eq 10 ]
  metal="$1" crystal="$2" deuterium="$3" level="$4" fields="$5" maxfields="$6" score1="$8" score2="$9"
  shift 9
  score3="$1"
  builds="$(db_query "SELECT COALESCE(CONCAT('[',GROUP_CONCAT(JSON_OBJECT('listId',list_id,'techId',tech_id,'level',level,'destroy',destroy,'duration',end-start) ORDER BY list_id SEPARATOR ','),']'),'[]') FROM uni1_buildqueue WHERE owner_id=$player_id AND planet_id=$planet_id")"
  tasks="$(db_query "SELECT COALESCE(CONCAT('[',GROUP_CONCAT(JSON_OBJECT('type',type,'objId',obj_id,'level',level,'duration',end-start,'prio',prio,'freeze',freeze,'frozen',frozen) ORDER BY task_id SEPARATOR ','),']'),'[]') FROM uni1_queue WHERE owner_id=$player_id AND type IN ('Build','Demolish')")"
  builds="$(printf '%s' "$builds" | jq -cS '.')"
  tasks="$(printf '%s' "$tasks" | jq -cS '.')"
  jq -nc --arg metal "$metal" --arg crystal "$crystal" --arg deuterium "$deuterium" --arg level "$level" --arg fields "$fields" --arg maxfields "$maxfields" --arg score1 "$score1" --arg score2 "$score2" --arg score3 "$score3" --argjson builds "$builds" --argjson tasks "$tasks" '{planet:{metal:($metal|tonumber),crystal:($crystal|tonumber),deuterium:($deuterium|tonumber),buildingLevel:($level|tonumber),fields:($fields|tonumber),maxFields:($maxfields|tonumber)},user:{score1:($score1|tonumber),score2:($score2|tonumber),score3:($score3|tonumber)},buildRows:$builds,queueTasks:$tasks}'
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

legacy_action() {
  prefix="$1" action="$2" list_id="${3:-0}"
  extra="modus=$action&techid=$building_id&planet=$planet_id"
  [ "$list_id" -le 0 ] || extra="modus=$action&listid=$list_id&planet=$planet_id"
  curl --silent --show-error --max-time 15 --output "$TMP_DIR/$prefix.body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=b_building&session=$legacy_session&cp=$planet_id&$extra"
}

go_action() {
  prefix="$1" action="$2" list_id="${3:-0}"
  body="{\"action\":\"$action\",\"techId\":$building_id,\"listId\":$list_id}"
  curl --silent --show-error --max-time 15 --output "$TMP_DIR/$prefix.body" --cookie "$go_cookie" --header 'Content-Type: application/json' --data "$body" --write-out '%{http_code}' "$GO_BASE_URL/api/game/buildings?session=$go_session&cp=$planet_id"
}

force_due() {
  db_query "UPDATE uni1_buildqueue SET start=UNIX_TIMESTAMP()-2,end=UNIX_TIMESTAMP()-1 WHERE owner_id=$player_id AND planet_id=$planet_id; UPDATE uni1_queue SET start=UNIX_TIMESTAMP()-2,end=UNIX_TIMESTAMP()-1 WHERE owner_id=$player_id AND type IN ('Build','Demolish')" >/dev/null
}

run_demolition() {
  case_name=demolition baseline_level=1 baseline_fields=1 baseline_score=1600000 commander_until=0
  reset_case; login_legacy legacy-demolition
  legacy_before="$(capture_state)"; legacy_first_status="$(legacy_action legacy-demolition-enqueue destroy)"; legacy_enqueued="$(capture_state)"
  force_due; legacy_final_status="$(legacy_action legacy-demolition-final noop)"; legacy_final="$(capture_state)"
  reset_case; login_go go-demolition
  go_before="$(capture_state)"; go_first_status="$(go_action go-demolition-enqueue destroy)"; go_enqueued="$(capture_state)"
  force_due; go_final_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/go-demolition-final.body" --cookie "$go_cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/buildings?session=$go_session&cp=$planet_id")"; go_final="$(capture_state)"
  record_case demolition
}

run_commander() {
  case_name=commander baseline_level=0 baseline_fields=0 baseline_score=0 commander_until="UNIX_TIMESTAMP()+3600"
  reset_case; login_legacy legacy-commander
  legacy_before="$(capture_state)"; legacy_first_status="$(legacy_action legacy-commander-first add)"; sleep 1
  legacy_second_status="$(legacy_action legacy-commander-second add)"; legacy_enqueued="$(capture_state)"
  legacy_propagate_status="$(legacy_action legacy-commander-propagate remove 1)"; legacy_propagated="$(capture_state)"
  force_due; legacy_final_status="$(legacy_action legacy-commander-final noop)"; legacy_final="$(capture_state)"
  reset_case; login_go go-commander
  go_before="$(capture_state)"; go_first_status="$(go_action go-commander-first add)"; sleep 1
  go_second_status="$(go_action go-commander-second add)"; go_enqueued="$(capture_state)"
  go_propagate_status="$(go_action go-commander-propagate remove 1)"; go_propagated="$(capture_state)"
  force_due; go_final_status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/go-commander-final.body" --cookie "$go_cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/buildings?session=$go_session&cp=$planet_id")"; go_final="$(capture_state)"
  record_case commander
}

record_case() {
  kind="$1" case_pass=true
  [ "$legacy_first_status" = 200 ] && [ "$go_first_status" = 200 ] && [ "$legacy_final_status" = 200 ] && [ "$go_final_status" = 200 ] || case_pass=false
  [ "$legacy_before" = "$go_before" ] && [ "$legacy_enqueued" = "$go_enqueued" ] && [ "$legacy_final" = "$go_final" ] || case_pass=false
  if [ "$kind" = demolition ]; then
    jq -e '.buildRows|length==1' >/dev/null <<EOF || case_pass=false
$legacy_enqueued
EOF
    jq -e '.buildRows[0].destroy==1 and .buildRows[0].level==0 and .queueTasks[0].type=="Demolish"' >/dev/null <<EOF || case_pass=false
$legacy_enqueued
EOF
    jq -e '.planet.buildingLevel==0 and .planet.fields==0 and .user.score1==0 and (.buildRows|length)==0 and (.queueTasks|length)==0' >/dev/null <<EOF || case_pass=false
$legacy_final
EOF
    extra_json=null
  else
    [ "$legacy_second_status" = 200 ] && [ "$go_second_status" = 200 ] && [ "$legacy_propagate_status" = 200 ] && [ "$go_propagate_status" = 200 ] || case_pass=false
    [ "$legacy_propagated" = "$go_propagated" ] || case_pass=false
    jq -e '(.buildRows|length)==2 and .buildRows[0].level==1 and .buildRows[1].level==2 and (.queueTasks|length)==1' >/dev/null <<EOF || case_pass=false
$legacy_enqueued
EOF
    jq -e '(.buildRows|length)==1 and .buildRows[0].listId==2 and .buildRows[0].level==1 and .queueTasks[0].level==1' >/dev/null <<EOF || case_pass=false
$legacy_propagated
EOF
    jq -e '.planet.buildingLevel==1 and .planet.fields==1 and .user.score1==1600000 and (.buildRows|length)==0 and (.queueTasks|length)==0' >/dev/null <<EOF || case_pass=false
$legacy_final
EOF
    extra_json="$(jq -nc --arg legacySecond "$legacy_second_status" --arg goSecond "$go_second_status" --arg legacyPropagate "$legacy_propagate_status" --arg goPropagate "$go_propagate_status" --argjson legacy "$legacy_propagated" --argjson go "$go_propagated" '{http:{second:{legacy:$legacySecond,go:$goSecond},propagate:{legacy:$legacyPropagate,go:$goPropagate}},propagated:{legacy:$legacy,go:$go}}')"
  fi
  [ "$case_pass" = true ] || all_pass=false
  jq -nc --arg name "building-$kind" --argjson pass "$case_pass" --arg legacyFirst "$legacy_first_status" --arg goFirst "$go_first_status" --arg legacyFinalStatus "$legacy_final_status" --arg goFinalStatus "$go_final_status" --argjson before "$legacy_before" --argjson legacyEnqueued "$legacy_enqueued" --argjson goEnqueued "$go_enqueued" --argjson legacyFinal "$legacy_final" --argjson goFinal "$go_final" --argjson extra "$extra_json" '{name:$name,pass:$pass,http:{first:{legacy:$legacyFirst,go:$goFirst},final:{legacy:$legacyFinalStatus,go:$goFinalStatus}},db:{before:$before,enqueued:{legacy:$legacyEnqueued,go:$goEnqueued},final:{legacy:$legacyFinal,go:$goFinal}},extra:$extra}' >> "$case_results"
}

all_pass=true
case_results="$TMP_DIR/cases.jsonl"
: > "$case_results"
run_demolition
run_commander
jq -s --argjson pass "$all_pass" '{pass:$pass,cases:.}' "$case_results" > "$REPORT"
[ "$all_pass" = true ]
printf 'Go/PHP advanced building differential E2E: PASS (2 cases)\n'
