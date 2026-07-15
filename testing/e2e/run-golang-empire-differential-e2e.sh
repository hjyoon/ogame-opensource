#!/bin/sh
set -eu
[ "${OGAME_E2E_TRACE:-0}" != "1" ] && [ "${1:-}" != "--trace" ] || set -x

ROOT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_EMPIRE_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-empire-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/empire-differential.XXXXXX")"

actor_id="$(jq -r '.queue_cancel.build.player_id // 0' "$FIXTURE")"
actor_login="$(jq -r '.queue_cancel.build.login // empty' "$FIXTURE")"
planet_id="$(jq -r '.queue_cancel.build.home_planet_id // 0' "$FIXTURE")"
building_id="$(jq -r '.queue_cancel.build.building_id // 0' "$FIXTURE")"
foreign_id="$(jq -r '.planet_context.foreign.home_planet_id // 0' "$FIXTURE")"
[ "$actor_id" -gt 0 ] && [ "$planet_id" -gt 0 ] && [ "$building_id" -gt 0 ] && [ "$foreign_id" -gt 0 ] && [ -n "$actor_login" ]

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

users_backup="uni1_e2e_empirediff_users_$$"
planets_backup="uni1_e2e_empirediff_planets_$$"
queue_backup="uni1_e2e_empirediff_queue_$$"
build_backup="uni1_e2e_empirediff_build_$$"
log_max="$(db_query "SELECT COALESCE(MAX(id),0) FROM uni1_userlogs WHERE owner_id=$actor_id")"
db_query "DROP TABLE IF EXISTS $users_backup,$planets_backup,$queue_backup,$build_backup;
CREATE TABLE $users_backup AS SELECT * FROM uni1_users WHERE player_id=$actor_id;
CREATE TABLE $planets_backup AS SELECT * FROM uni1_planets WHERE planet_id IN ($planet_id,$foreign_id);
CREATE TABLE $queue_backup AS SELECT * FROM uni1_queue WHERE owner_id=$actor_id;
CREATE TABLE $build_backup AS SELECT * FROM uni1_buildqueue WHERE owner_id=$actor_id OR planet_id=$planet_id" >/dev/null

restore_state() {
  db_query "DELETE FROM uni1_queue WHERE owner_id=$actor_id; INSERT INTO uni1_queue SELECT * FROM $queue_backup;
DELETE FROM uni1_buildqueue WHERE owner_id=$actor_id OR planet_id=$planet_id; INSERT INTO uni1_buildqueue SELECT * FROM $build_backup;
DELETE FROM uni1_userlogs WHERE owner_id=$actor_id AND id>$log_max;
UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id SET
u.session=b.session,u.private_session=b.private_session,u.password=b.password,u.validated=b.validated,
u.vacation=b.vacation,u.vacation_until=b.vacation_until,u.disable=b.disable,u.disable_until=b.disable_until,
u.admin=b.admin,u.lang=b.lang,u.aktplanet=b.aktplanet,u.com_until=b.com_until,u.score1=b.score1,u.score2=b.score2,u.score3=b.score3;
UPDATE uni1_planets p JOIN $planets_backup b ON b.planet_id=p.planet_id SET
p.\`700\`=b.\`700\`,p.\`701\`=b.\`701\`,p.\`702\`=b.\`702\`,p.\`$building_id\`=b.\`$building_id\`,
p.fields=b.fields,p.maxfields=b.maxfields,p.prod1=b.prod1,p.prod2=b.prod2,p.prod3=b.prod3,p.prod4=b.prod4,
p.prod12=b.prod12,p.prod212=b.prod212,p.lastpeek=b.lastpeek" >/dev/null
}

cleanup() {
  restore_state >/dev/null 2>&1 || true
  db_query "DROP TABLE IF EXISTS $users_backup,$planets_backup,$queue_backup,$build_backup" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

reset_case() {
  restore_state
  db_query "DELETE FROM uni1_queue WHERE owner_id=$actor_id; DELETE FROM uni1_buildqueue WHERE owner_id=$actor_id OR planet_id=$planet_id;
DELETE FROM uni1_userlogs WHERE owner_id=$actor_id AND id>$log_max;
UPDATE uni1_users SET session='',private_session='',validated=1,vacation=0,vacation_until=0,disable=0,disable_until=0,
admin=0,lang='en',aktplanet=$planet_id,com_until=UNIX_TIMESTAMP()+2592000,score1=0,score2=0,score3=0 WHERE player_id=$actor_id;
UPDATE uni1_planets SET \`700\`=10000000,\`701\`=10000000,\`702\`=10000000,\`$building_id\`=0,
fields=0,maxfields=200,prod1=0,prod2=0,prod3=0,prod4=0,prod12=0,prod212=0,lastpeek=UNIX_TIMESTAMP() WHERE planet_id=$planet_id" >/dev/null
}

login_legacy() {
  prefix="$1"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" \
    --output "$TMP_DIR/$prefix-login.body" --cookie-jar "$TMP_DIR/$prefix.cookies" --request POST \
    --data-urlencode "login=$actor_login" --data-urlencode "pass=$PASSWORD" --write-out '%{http_code}' \
    "$LEGACY_BASE_URL/game/reg/login2.php")"
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/$prefix-login.headers")"
  session="$(printf '%s' "$location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 302 ] && [ -n "$session" ]
}

login_go() {
  prefix="$1"
  payload="$(jq -nc --arg login "$actor_login" --arg pass "$PASSWORD" '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" \
    --output "$TMP_DIR/$prefix-login.body" --header 'Content-Type: application/json' --data "$payload" \
    --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/$prefix-login.headers" | cut -d';' -f1 | tr -d '\r')"
  redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/$prefix-login.body")"
  session="$(printf '%s' "$redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 200 ] && [ -n "$cookie" ] && [ -n "$session" ]
}

configure_case() {
  case_name="$1"; action=add; target_id="$planet_id"; tech_id="$building_id"; list_id=0; second_action=''
  case "$case_name" in
    add-valid) ;;
    add-remove) second_action=remove ;;
    destroy-valid)
      action=destroy
      db_query "UPDATE uni1_planets SET \`$building_id\`=2,fields=2 WHERE planet_id=$planet_id" >/dev/null ;;
    invalid-tech) tech_id=999999 ;;
    foreign-planet) target_id="$foreign_id" ;;
    vacation-blocked) db_query "UPDATE uni1_users SET vacation=1,vacation_until=UNIX_TIMESTAMP()+86400 WHERE player_id=$actor_id" >/dev/null ;;
    commander-inactive) db_query "UPDATE uni1_users SET com_until=0 WHERE player_id=$actor_id" >/dev/null ;;
    remove-missing) action=remove; list_id=999999 ;;
    *) printf 'Unknown empire differential case: %s\n' "$case_name" >&2; exit 1 ;;
  esac
}

capture_state() {
  planet="$(db_query "SELECT JSON_OBJECT('metal',FLOOR(\`700\`),'crystal',FLOOR(\`701\`),'deuterium',FLOOR(\`702\`),
'level',\`$building_id\`,'fields',fields) FROM uni1_planets WHERE planet_id=$planet_id")"
  foreign="$(db_query "SELECT JSON_OBJECT('metal',FLOOR(\`700\`),'crystal',FLOOR(\`701\`),'deuterium',FLOOR(\`702\`),
'level',\`$building_id\`,'fields',fields) FROM uni1_planets WHERE planet_id=$foreign_id")"
  build="$(db_query "SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT('list',list_id,'tech',tech_id,'level',level,'destroy',destroy,'duration',end-start)),JSON_ARRAY())
FROM (SELECT * FROM uni1_buildqueue WHERE owner_id=$actor_id OR planet_id=$planet_id ORDER BY list_id) b")"
  queue="$(db_query "SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT('type',type,'sub',IF(sub_id=$planet_id,'actor','other'),'obj',obj_id,'level',level,
'duration',end-start,'prio',prio,'freeze',freeze,'frozen',frozen)),JSON_ARRAY())
FROM (SELECT * FROM uni1_queue WHERE owner_id=$actor_id AND type IN ('Build','Demolish') ORDER BY task_id) q")"
  user="$(db_query "SELECT JSON_OBJECT('score1',score1,'score2',score2,'score3',score3) FROM uni1_users WHERE player_id=$actor_id")"
  state="$(jq -ncS --argjson planet "$planet" --argjson foreign "$foreign" --argjson build "$build" --argjson queue "$queue" --argjson user "$user" \
    '{planet:$planet,foreignPlanet:$foreign,buildQueue:$build,queue:$queue,user:$user}')"
}

legacy_request() {
  request_action="$1"; request_list="$2"; suffix="$3"
  url="$LEGACY_BASE_URL/game/index.php?page=imperium&no_header=1&planettype=1&session=$session&cp=$planet_id&modus=$request_action&planet=$target_id&techid=$tech_id&listid=$request_list"
  http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/legacy-$case_name-$suffix.body" \
    --cookie "$TMP_DIR/legacy-$case_name.cookies" --write-out '%{http_code}' "$url")"
}

go_request() {
  request_action="$1"; request_list="$2"; suffix="$3"
  url="$GO_BASE_URL/api/game/empire?planettype=1&session=$session&cp=$planet_id&modus=$request_action&planet=$target_id&techid=$tech_id&listid=$request_list"
  http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/go-$case_name-$suffix.body" \
    --cookie "$cookie" --write-out '%{http_code}' "$url")"
}

run_side() {
  side="$1"; case_name="$2"
  reset_case; configure_case "$case_name"
  if [ "$side" = legacy ]; then login_legacy "$side-$case_name"; legacy_request "$action" "$list_id" first
  else login_go "$side-$case_name"; go_request "$action" "$list_id" first; fi
  if [ "$side" = legacy ]; then [ "$http" = 200 ] || [ "$http" = 302 ]; else [ "$http" = 200 ]; fi
  capture_state; intermediate="$state"
  if [ "$second_action" = remove ]; then
    if [ "$side" = legacy ]; then legacy_request remove 1 second; else go_request remove 1 second; fi
    if [ "$side" = legacy ]; then [ "$http" = 200 ] || [ "$http" = 302 ]; else [ "$http" = 200 ]; fi
    capture_state
  fi
  normalized="$(jq -ncS --argjson intermediate "$intermediate" --argjson final "$state" '{intermediate:$intermediate,final:$final}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true; case_count=0
cases='add-valid add-remove destroy-valid invalid-tech foreign-planet vacation-blocked commander-inactive remove-missing'
for case_name in $cases; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$legacy" = "$go" ] || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "empire-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
  case_count=$((case_count + 1))
done

jq -s --argjson pass "$all_pass" --argjson cases "$case_count" \
  '{pass:$pass,normalization:"session, task IDs and absolute queue clock excluded; shortcut resource, queue, level, field and score effects exact",metrics:{cases:$cases},cases:.}' \
  "$results" > "$REPORT"
[ "$all_pass" = true ] || { jq '.cases[] | select(.pass == false)' "$REPORT" >&2; exit 1; }
printf 'Go/PHP empire differential E2E: PASS (%s cases)\n' "$case_count"
