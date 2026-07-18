#!/bin/sh
set -eu
[ "${OGAME_E2E_TRACE:-0}" != "1" ] && [ "${1:-}" != "--trace" ] || set -x

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_TERRAFORMER_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-terraformer-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/terraformer-differential.XXXXXX")"

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

login="$(jq -r '.queue_cancel.build.login // empty' "$FIXTURE")"
player_id="$(jq -r '.queue_cancel.build.player_id // 0' "$FIXTURE")"
planet_id="$(jq -r '.queue_cancel.build.home_planet_id // 0' "$FIXTURE")"
[ -n "$login" ] && [ "$player_id" -gt 0 ] && [ "$planet_id" -gt 0 ]

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'exec mysql -N -B -uroot -p"$MYSQL_ROOT_PASSWORD" uni -e "$1"' sh "$1" 2>/dev/null
}

planet_columns='`700`,`701`,`702`,fields,maxfields,lastpeek,temp,`1`,`2`,`3`,`4`,`12`,`15`,`33`,`212`,prod1,prod2,prod3,prod4,prod12,prod212'
user_columns='score1,score2,score3,vacation,com_until,eng_until,`113`'
original_planet="$(db_query "SELECT $planet_columns FROM uni1_planets WHERE planet_id=$planet_id AND owner_id=$player_id LIMIT 1")"
original_user="$(db_query "SELECT $user_columns FROM uni1_users WHERE player_id=$player_id LIMIT 1")"
[ -n "$original_planet" ] && [ -n "$original_user" ]
[ "$(db_query "SELECT COUNT(*) FROM uni1_buildqueue WHERE owner_id=$player_id OR planet_id=$planet_id")" = 0 ]
[ "$(db_query "SELECT COUNT(*) FROM uni1_queue WHERE owner_id=$player_id AND type IN ('Build','Demolish')")" = 0 ]
initial_log_id="$(db_query "SELECT COALESCE(MAX(id),0) FROM uni1_userlogs WHERE owner_id=$player_id")"

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

restore_original() {
  planet_assignments="$(sql_assignments "$planet_columns" "$original_planet")"
  user_assignments="$(sql_assignments "$user_columns" "$original_user")"
  db_query "DELETE FROM uni1_queue WHERE owner_id=$player_id AND type IN ('Build','Demolish');
DELETE FROM uni1_buildqueue WHERE owner_id=$player_id OR planet_id=$planet_id;
DELETE FROM uni1_userlogs WHERE owner_id=$player_id AND id>$initial_log_id;
UPDATE uni1_planets SET $planet_assignments WHERE planet_id=$planet_id AND owner_id=$player_id;
UPDATE uni1_users SET $user_assignments WHERE player_id=$player_id" >/dev/null
}

reset_case() {
  solar_level="$1"
  commander_until="${2:-0}"
  db_query "DELETE FROM uni1_queue WHERE owner_id=$player_id AND type IN ('Build','Demolish');
DELETE FROM uni1_buildqueue WHERE owner_id=$player_id OR planet_id=$planet_id;
DELETE FROM uni1_userlogs WHERE owner_id=$player_id AND id>$initial_log_id;
UPDATE uni1_planets SET \`700\`=10000,\`701\`=50000,\`702\`=100000,fields=$((solar_level + 1)),maxfields=200,lastpeek=UNIX_TIMESTAMP()+60,temp=19,
\`1\`=0,\`2\`=0,\`3\`=0,\`4\`=$solar_level,\`12\`=0,\`15\`=1,\`33\`=0,\`212\`=0,
prod1=0,prod2=0,prod3=0,prod4=1,prod12=0,prod212=0
WHERE planet_id=$planet_id AND owner_id=$player_id;
UPDATE uni1_users SET score1=0,score2=0,score3=0,vacation=0,com_until=$commander_until,eng_until=0,\`113\`=12
WHERE player_id=$player_id" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

capture_state() {
  planet="$(db_query "SELECT JSON_OBJECT('metal',FLOOR(\`700\`),'crystal',FLOOR(\`701\`),'deuterium',FLOOR(\`702\`),'level',\`33\`,'fields',fields,'maxFields',maxfields) FROM uni1_planets WHERE planet_id=$planet_id AND owner_id=$player_id LIMIT 1")"
  score="$(db_query "SELECT score1 FROM uni1_users WHERE player_id=$player_id LIMIT 1")"
  builds="$(db_query "SELECT COALESCE(CONCAT('[',GROUP_CONCAT(JSON_OBJECT('listId',list_id,'techId',tech_id,'level',level,'destroy',destroy,'duration',end-start) ORDER BY list_id SEPARATOR ','),']'),'[]') FROM uni1_buildqueue WHERE owner_id=$player_id AND planet_id=$planet_id")"
  tasks="$(db_query "SELECT COALESCE(CONCAT('[',GROUP_CONCAT(JSON_OBJECT('type',type,'objId',obj_id,'level',level,'duration',end-start) ORDER BY task_id SEPARATOR ','),']'),'[]') FROM uni1_queue WHERE owner_id=$player_id AND type IN ('Build','Demolish')")"
  jq -ncS --argjson planet "$planet" --arg score "$score" --argjson builds "$builds" --argjson tasks "$tasks" \
    '{planet:$planet,score:($score|tonumber),builds:$builds,tasks:$tasks}'
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

enqueue_building() {
  side="$1"
  tech_id="$2"
  suffix="$3"
  if [ "$side" = legacy ]; then
    curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$case_name-$suffix.body" \
      --cookie "$cookie" --write-out '%{http_code}' \
      "$LEGACY_BASE_URL/game/index.php?page=b_building&session=$session&cp=$planet_id&modus=add&techid=$tech_id&planet=$planet_id"
  else
    curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$case_name-$suffix.body" \
      --cookie "$cookie" --header 'Content-Type: application/json' --data "{\"action\":\"add\",\"techId\":$tech_id}" \
      --write-out '%{http_code}' "$GO_BASE_URL/api/game/buildings?session=$session&cp=$planet_id"
  fi
}

force_building_due() {
  tech_id="$1"
  db_query "UPDATE uni1_buildqueue SET start=UNIX_TIMESTAMP()-2,end=UNIX_TIMESTAMP()-1 WHERE owner_id=$player_id AND planet_id=$planet_id AND tech_id=$tech_id;
UPDATE uni1_queue SET start=UNIX_TIMESTAMP()-2,end=UNIX_TIMESTAMP()-1 WHERE owner_id=$player_id AND type='Build' AND obj_id=$tech_id" >/dev/null
}

finish_due_building() {
  side="$1"
  suffix="$2"
  if [ "$side" = legacy ]; then
    curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$case_name-$suffix.body" \
      --cookie "$cookie" --write-out '%{http_code}' \
      "$LEGACY_BASE_URL/game/index.php?page=b_building&session=$session&cp=$planet_id"
  else
    curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$case_name-$suffix.body" \
      --cookie "$cookie" --write-out '%{http_code}' \
      "$GO_BASE_URL/api/game/buildings?session=$session&cp=$planet_id"
  fi
}

run_side() {
  side="$1"
  solar_level="$2"
  reset_case "$solar_level"
  if [ "$side" = legacy ]; then
    login_legacy "$side-$case_name"
  else
    login_go "$side-$case_name"
  fi
  before="$(capture_state)"
  enqueue_status="$(enqueue_building "$side" 33 enqueue)"
  enqueued="$(capture_state)"
  finish_status=0
  final="$enqueued"
  issue=""
  if [ "$case_name" = enough-energy ]; then
    force_building_due 33
    finish_status="$(finish_due_building "$side" finish)"
    final="$(capture_state)"
  elif [ "$side" = go ]; then
    issue="$(jq -r '.actionIssue.code // empty' "$TMP_DIR/$side-$case_name-enqueue.body")"
  fi
  result="$(jq -ncS --arg enqueue "$enqueue_status" --arg finish "$finish_status" --arg issue "$issue" \
    --argjson before "$before" --argjson enqueued "$enqueued" --argjson final "$final" \
    '{http:{enqueue:$enqueue,finish:$finish},issue:$issue,before:$before,enqueued:$enqueued,final:$final}')"
}

run_commander_side() {
  side="$1"
  reset_case 25 "UNIX_TIMESTAMP()+3600"
  db_query "UPDATE uni1_planets SET \`700\`=10060,\`701\`=50015 WHERE planet_id=$planet_id AND owner_id=$player_id" >/dev/null
  if [ "$side" = legacy ]; then
    login_legacy "$side-$case_name"
  else
    login_go "$side-$case_name"
  fi
  before="$(capture_state)"
  first_status="$(enqueue_building "$side" 1 first)"
  sleep 1
  second_status="$(enqueue_building "$side" 33 second)"
  queued="$(capture_state)"
  force_building_due 1
  propagate_status="$(finish_due_building "$side" propagate)"
  propagated="$(capture_state)"
  force_building_due 33
  finish_status="$(finish_due_building "$side" finish)"
  final="$(capture_state)"
  result="$(jq -ncS --arg first "$first_status" --arg second "$second_status" \
    --arg propagate "$propagate_status" --arg finish "$finish_status" \
    --argjson before "$before" --argjson queued "$queued" --argjson propagated "$propagated" --argjson final "$final" \
    '{http:{first:$first,second:$second,propagate:$propagate,finish:$finish},before:$before,queued:$queued,propagated:$propagated,final:$final}')"
}

all_pass=true
cases="$TMP_DIR/cases.jsonl"
: > "$cases"

case_name=enough-energy
run_side legacy 25
legacy="$result"
run_side go 25
go="$result"
pass=true
[ "$legacy" = "$go" ] || pass=false
printf '%s' "$legacy" | jq -e '
  .http.enqueue=="200" and .http.finish=="200" and
  .enqueued.planet.metal==10000 and .enqueued.planet.crystal==0 and .enqueued.planet.deuterium==0 and
  (.enqueued.builds|length)==1 and .enqueued.builds[0].techId==33 and
  (.enqueued.tasks|length)==1 and .enqueued.tasks[0].objId==33 and
  .final.planet.level==1 and .final.planet.fields==27 and .final.planet.maxFields==205 and
  (.final.builds|length)==0 and (.final.tasks|length)==0
' >/dev/null || pass=false
[ "$pass" = true ] || all_pass=false
jq -nc --arg name "$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
  '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$cases"

case_name=commander-propagation
run_commander_side legacy
legacy="$result"
run_commander_side go
go="$result"
pass=true
[ "$legacy" = "$go" ] || pass=false
printf '%s' "$legacy" | jq -e '
  .http.first=="200" and .http.second=="200" and .http.propagate=="200" and .http.finish=="200" and
  (.queued.builds|length)==2 and .queued.builds[0].techId==1 and .queued.builds[1].techId==33 and
  (.queued.tasks|length)==1 and .queued.tasks[0].objId==1 and
  .propagated.planet.level==0 and .propagated.planet.fields==27 and .propagated.planet.maxFields==200 and
  .propagated.planet.crystal==0 and .propagated.planet.deuterium==0 and
  (.propagated.builds|length)==1 and .propagated.builds[0].techId==33 and
  (.propagated.tasks|length)==1 and .propagated.tasks[0].objId==33 and
  .final.planet.level==1 and .final.planet.fields==28 and .final.planet.maxFields==205 and
  .final.score==150075 and (.final.builds|length)==0 and (.final.tasks|length)==0
' >/dev/null || pass=false
[ "$pass" = true ] || all_pass=false
jq -nc --arg name "$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
  '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$cases"

case_name=insufficient-energy
run_side legacy 0
legacy="$result"
run_side go 0
go="$result"
pass=true
[ "$(printf '%s' "$legacy" | jq -cS '{http:.http,before:.before,enqueued:.enqueued,final:.final}')" = \
  "$(printf '%s' "$go" | jq -cS '{http:.http,before:.before,enqueued:.enqueued,final:.final}')" ] || pass=false
printf '%s' "$legacy" | jq -e '
  .http.enqueue=="200" and .before==.enqueued and (.enqueued.builds|length)==0 and (.enqueued.tasks|length)==0
' >/dev/null || pass=false
[ "$(printf '%s' "$go" | jq -r '.issue')" = no_resources ] || pass=false
[ "$pass" = true ] || all_pass=false
jq -nc --arg name "$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
  '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$cases"

restore_original
jq -s --argjson pass "$all_pass" --arg login "$login" --argjson playerId "$player_id" --argjson planetId "$planet_id" \
  '{pass:$pass,fixture:{login:$login,playerId:$playerId,planetId:$planetId},cases:.}' "$cases" > "$REPORT"
[ "$all_pass" = true ]
printf 'Go/PHP Terraformer differential E2E: PASS (3 cases, planet=%s)\n' "$planet_id"
