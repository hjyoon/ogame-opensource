#!/bin/sh
set -eu
[ "${OGAME_E2E_TRACE:-0}" != "1" ] && [ "${1:-}" != "--trace" ] || set -x

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_GRAVITON_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-graviton-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/graviton-differential.XXXXXX")"

wait_for_url() {
  url="$1"
  attempts=30
  while [ "$attempts" -gt 0 ]; do
    if curl --silent --fail --max-time 3 --output /dev/null "$url"; then
      return 0
    fi
    attempts=$((attempts - 1))
    sleep 1
  done
  printf 'Timed out waiting for %s\n' "$url" >&2
  return 1
}

wait_for_url "$LEGACY_BASE_URL/"
wait_for_url "$GO_BASE_URL/api/healthz"

login="$(jq -r '.queue_cancel.research.login // empty' "$FIXTURE")"
player_id="$(jq -r '.queue_cancel.research.player_id // 0' "$FIXTURE")"
planet_id="$(jq -r '.queue_cancel.research.home_planet_id // 0' "$FIXTURE")"
[ -n "$login" ] && [ "$player_id" -gt 0 ] && [ "$planet_id" -gt 0 ]

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'exec mysql -N -B -uroot -p"$MYSQL_ROOT_PASSWORD" uni -e "$1"' sh "$1" 2>/dev/null
}

planet_columns='`700`,`701`,`702`,fields,maxfields,lastpeek,temp,`1`,`2`,`3`,`4`,`12`,`31`,`212`,prod1,prod2,prod3,prod4,prod12,prod212'
user_columns='`199`,vacation,tec_until,eng_until'
original_planet="$(db_query "SELECT $planet_columns FROM uni1_planets WHERE planet_id=$planet_id AND owner_id=$player_id LIMIT 1")"
original_user="$(db_query "SELECT $user_columns FROM uni1_users WHERE player_id=$player_id LIMIT 1")"
[ -n "$original_planet" ] && [ -n "$original_user" ]
[ "$(db_query "SELECT COUNT(*) FROM uni1_queue WHERE owner_id=$player_id AND type='Research'")" = 0 ]
initial_log_id="$(db_query "SELECT COALESCE(MAX(id),0) FROM uni1_userlogs WHERE owner_id=$player_id")"
initial_debug_id="$(db_query "SELECT COALESCE(MAX(error_id),0) FROM uni1_debug")"
backup_table="uni1_e2e_graviton_users_$$"
db_query "DROP TABLE IF EXISTS $backup_table; CREATE TABLE $backup_table AS SELECT player_id,score1,score2,score3,place1,place2,place3 FROM uni1_users" >/dev/null

sql_assignments() {
  columns="$1"
  values="$2"
  old_ifs="$IFS"
  IFS=','
  set -- $columns
  IFS="$old_ifs"
  assignments=""
  index=1
  while [ "$#" -gt 0 ]; do
    column="$1"
    shift
    value="$(printf '%s' "$values" | cut -f"$index")"
    [ -z "$assignments" ] || assignments="$assignments,"
    assignments="$assignments$column=$value"
    index=$((index + 1))
  done
  printf '%s' "$assignments"
}

restore_ranks() {
  db_query "UPDATE uni1_users u JOIN $backup_table b ON b.player_id=u.player_id
SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3" >/dev/null
}

restore_original() {
  planet_assignments="$(sql_assignments "$planet_columns" "$original_planet")"
  user_assignments="$(sql_assignments "$user_columns" "$original_user")"
  db_query "DELETE FROM uni1_queue WHERE owner_id=$player_id AND type='Research';
DELETE FROM uni1_userlogs WHERE owner_id=$player_id AND id>$initial_log_id;
DELETE FROM uni1_debug WHERE error_id>$initial_debug_id;
UPDATE uni1_users u JOIN $backup_table b ON b.player_id=u.player_id
SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3;
UPDATE uni1_planets SET $planet_assignments WHERE planet_id=$planet_id AND owner_id=$player_id;
UPDATE uni1_users SET $user_assignments WHERE player_id=$player_id;
DROP TABLE IF EXISTS $backup_table" >/dev/null
}

reset_case() {
  solar_level="$1"
  lab_level="$2"
  graviton_level="$3"
  restore_ranks
  db_query "DELETE FROM uni1_queue WHERE owner_id=$player_id AND type='Research';
DELETE FROM uni1_userlogs WHERE owner_id=$player_id AND id>$initial_log_id;
DELETE FROM uni1_debug WHERE error_id>$initial_debug_id;
UPDATE uni1_planets SET \`700\`=1234,\`701\`=2345,\`702\`=3456,fields=$((solar_level + lab_level)),maxfields=300,lastpeek=UNIX_TIMESTAMP()+60,temp=19,
\`1\`=0,\`2\`=0,\`3\`=0,\`4\`=$solar_level,\`12\`=0,\`31\`=$lab_level,\`212\`=0,
prod1=0,prod2=0,prod3=0,prod4=1,prod12=0,prod212=0
WHERE planet_id=$planet_id AND owner_id=$player_id;
UPDATE uni1_users SET \`199\`=$graviton_level,score1=0,score2=0,score3=0,vacation=0,tec_until=0,eng_until=0
WHERE player_id=$player_id" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

capture_state() {
  planet="$(db_query "SELECT JSON_OBJECT('metal',FLOOR(\`700\`),'crystal',FLOOR(\`701\`),'deuterium',FLOOR(\`702\`),'solarPlant',\`4\`,'researchLab',\`31\`) FROM uni1_planets WHERE planet_id=$planet_id AND owner_id=$player_id LIMIT 1")"
  user="$(db_query "SELECT JSON_OBJECT('graviton',\`199\`,'score1',score1,'score2',score2,'score3',score3) FROM uni1_users WHERE player_id=$player_id LIMIT 1")"
  task="$(db_query "SELECT COALESCE(CONCAT('[',GROUP_CONCAT(JSON_OBJECT('type',type,'planetId',sub_id,'techId',obj_id,'level',level,'duration',end-start) ORDER BY task_id SEPARATOR ','),']'),'[]') FROM uni1_queue WHERE owner_id=$player_id AND type='Research'")"
  jq -ncS --argjson planet "$planet" --argjson user "$user" --argjson task "$task" \
    '{planet:$planet,user:$user,queue:$task}'
}

login_legacy() {
  prefix="$1"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" \
    --output "$TMP_DIR/$prefix-login.body" --cookie-jar "$TMP_DIR/$prefix.cookies" --request POST \
    --data-urlencode "login=$login" --data-urlencode "pass=$PASSWORD" --write-out '%{http_code}' \
    "$LEGACY_BASE_URL/game/reg/login2.php")"
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/$prefix-login.headers")"
  session="$(printf '%s' "$location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 302 ] && [ -n "$session" ]
  cookie="$TMP_DIR/$prefix.cookies"
}

login_go() {
  prefix="$1"
  payload="$(jq -nc --arg login "$login" --arg pass "$PASSWORD" '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" \
    --output "$TMP_DIR/$prefix-login.body" --header 'Content-Type: application/json' --data "$payload" \
    --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/$prefix-login.headers" | cut -d';' -f1 | tr -d '\r')"
  redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/$prefix-login.body")"
  session="$(printf '%s' "$redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 200 ] && [ -n "$cookie" ] && [ -n "$session" ]
}

start_research() {
  side="$1"
  if [ "$side" = legacy ]; then
    curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$case_name-start.body" \
      --cookie "$cookie" --write-out '%{http_code}' \
      "$LEGACY_BASE_URL/game/index.php?page=buildings&mode=Forschung&session=$session&cp=$planet_id&bau=199"
  else
    curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$case_name-start.body" \
      --cookie "$cookie" --header 'Content-Type: application/json' --data '{"action":"start","techId":199}' \
      --write-out '%{http_code}' "$GO_BASE_URL/api/game/research?session=$session&cp=$planet_id"
  fi
}

finish_research() {
  side="$1"
  if [ "$side" = legacy ]; then
    curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$case_name-finish.body" \
      --cookie "$cookie" --write-out '%{http_code}' \
      "$LEGACY_BASE_URL/game/index.php?page=buildings&mode=Forschung&session=$session&cp=$planet_id"
  else
    curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$case_name-finish.body" \
      --cookie "$cookie" --write-out '%{http_code}' \
      "$GO_BASE_URL/api/game/research?session=$session&cp=$planet_id"
  fi
}

cancel_research() {
  side="$1"
  if [ "$side" = legacy ]; then
    curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$case_name-cancel.body" \
      --cookie "$cookie" --write-out '%{http_code}' \
      "$LEGACY_BASE_URL/game/index.php?page=buildings&mode=Forschung&session=$session&cp=$planet_id&unbau=199"
  else
    curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$case_name-cancel.body" \
      --cookie "$cookie" --header 'Content-Type: application/json' --data '{"action":"cancel","techId":199}' \
      --write-out '%{http_code}' "$GO_BASE_URL/api/game/research?session=$session&cp=$planet_id"
  fi
}

run_side() {
  side="$1"
  solar_level="$2"
  lab_level="$3"
  graviton_level="$4"
  final_action="$5"
  reset_case "$solar_level" "$lab_level" "$graviton_level"
  if [ "$side" = legacy ]; then
    login_legacy "$side-$case_name"
  else
    login_go "$side-$case_name"
  fi
  before="$(capture_state)"
  start_status="$(start_research "$side")"
  started="$(capture_state)"
  issue=""
  [ "$side" != go ] || issue="$(jq -r '.actionIssue.code // empty' "$TMP_DIR/$side-$case_name-start.body")"
  final_status=0
  final="$started"
  if [ "$final_action" = finish ]; then
    db_query "UPDATE uni1_queue SET start=UNIX_TIMESTAMP()-2,end=UNIX_TIMESTAMP()-1 WHERE owner_id=$player_id AND type='Research' AND obj_id=199" >/dev/null
    final_status="$(finish_research "$side")"
    final="$(capture_state)"
  elif [ "$final_action" = cancel ]; then
    db_query "UPDATE uni1_queue SET start=UNIX_TIMESTAMP(),end=UNIX_TIMESTAMP()+60 WHERE owner_id=$player_id AND type='Research' AND obj_id=199" >/dev/null
    final_status="$(cancel_research "$side")"
    final="$(capture_state)"
  fi
  result="$(jq -ncS --arg start "$start_status" --arg finalStatus "$final_status" --arg issue "$issue" \
    --argjson before "$before" --argjson started "$started" --argjson final "$final" \
    '{http:{start:$start,final:$finalStatus},issue:$issue,before:$before,started:$started,final:$final}')"
}

all_pass=true
cases="$TMP_DIR/cases.jsonl"
: > "$cases"

record_case() {
  pass="$1"
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$cases"
}

case_name=level-one-completion
run_side legacy 59 12 0 finish
legacy="$result"
run_side go 59 12 0 finish
go="$result"
pass=true
[ "$(printf '%s' "$legacy" | jq -cS 'del(.issue)')" = "$(printf '%s' "$go" | jq -cS 'del(.issue)')" ] || pass=false
printf '%s' "$legacy" | jq -e '
  .http.start=="200" and .http.final=="200" and
  .started.planet==.before.planet and (.started.queue|length)==1 and
  .started.queue[0].techId==199 and .started.queue[0].level==1 and .started.queue[0].duration==1 and
  .final.planet==.before.planet and (.final.queue|length)==0 and
  .final.user.graviton==1 and .final.user.score1==0 and .final.user.score2==0 and .final.user.score3==1
' >/dev/null || pass=false
record_case "$pass"

case_name=level-two-completion
run_side legacy 69 12 1 finish
legacy="$result"
run_side go 69 12 1 finish
go="$result"
pass=true
[ "$(printf '%s' "$legacy" | jq -cS 'del(.issue)')" = "$(printf '%s' "$go" | jq -cS 'del(.issue)')" ] || pass=false
printf '%s' "$legacy" | jq -e '
  .started.planet==.before.planet and (.started.queue|length)==1 and
  .started.queue[0].level==2 and .started.queue[0].duration==1 and
  .final.planet==.before.planet and .final.user.graviton==2 and .final.user.score1==0 and .final.user.score3==1
' >/dev/null || pass=false
record_case "$pass"

case_name=cancel
run_side legacy 59 12 0 cancel
legacy="$result"
run_side go 59 12 0 cancel
go="$result"
pass=true
[ "$(printf '%s' "$legacy" | jq -cS 'del(.issue)')" = "$(printf '%s' "$go" | jq -cS 'del(.issue)')" ] || pass=false
printf '%s' "$legacy" | jq -e '
  (.started.queue|length)==1 and .final==.before and .http.start=="200" and .http.final=="200"
' >/dev/null || pass=false
record_case "$pass"

case_name=insufficient-energy
run_side legacy 58 12 0 none
legacy="$result"
run_side go 58 12 0 none
go="$result"
pass=true
[ "$(printf '%s' "$legacy" | jq -cS 'del(.issue)')" = "$(printf '%s' "$go" | jq -cS 'del(.issue)')" ] || pass=false
printf '%s' "$legacy" | jq -e '.http.start=="200" and .started==.before and (.started.queue|length)==0' >/dev/null || pass=false
[ "$(printf '%s' "$go" | jq -r '.issue')" = no_resources ] || pass=false
record_case "$pass"

case_name=research-lab-requirement
run_side legacy 69 11 0 none
legacy="$result"
run_side go 69 11 0 none
go="$result"
pass=true
[ "$(printf '%s' "$legacy" | jq -cS 'del(.issue)')" = "$(printf '%s' "$go" | jq -cS 'del(.issue)')" ] || pass=false
printf '%s' "$legacy" | jq -e '.http.start=="200" and .started==.before and (.started.queue|length)==0' >/dev/null || pass=false
[ "$(printf '%s' "$go" | jq -r '.issue')" = requirements ] || pass=false
record_case "$pass"

restore_original
jq -s --argjson pass "$all_pass" --arg login "$login" --argjson playerId "$player_id" --argjson planetId "$planet_id" \
  '{pass:$pass,fixture:{login:$login,playerId:$playerId,planetId:$planetId},cases:.}' "$cases" > "$REPORT"
[ "$all_pass" = true ]
printf 'Go/PHP Graviton differential E2E: PASS (5 cases, planet=%s)\n' "$planet_id"
