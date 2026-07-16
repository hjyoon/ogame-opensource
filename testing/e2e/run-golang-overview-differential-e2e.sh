#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_OVERVIEW_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-overview-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/overview-differential.XXXXXX")"

actor_id="$(jq -r '.planet_context.owner.player_id // 0' "$FIXTURE")"
actor_login="$(jq -r '.planet_context.owner.login // empty' "$FIXTURE")"
home_id="$(jq -r '.planet_context.owner.home_planet_id // 0' "$FIXTURE")"
colony_id="$(jq -r '.planet_context.owner.colony_planet_id // 0' "$FIXTURE")"
moon_id="$(jq -r '.planet_context.owner.moon_id // 0' "$FIXTURE")"
foreign_id="$(jq -r '.planet_context.foreign.home_planet_id // 0' "$FIXTURE")"
[ "$actor_id" -gt 0 ] && [ "$home_id" -gt 0 ] && [ "$colony_id" -gt 0 ] && [ "$moon_id" -gt 0 ] && [ "$foreign_id" -gt 0 ] && [ -n "$actor_login" ]

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

users_backup="uni1_e2e_overviewdiff_users_$$"
planets_backup="uni1_e2e_overviewdiff_planets_$$"
fleet_backup="uni1_e2e_overviewdiff_fleet_$$"
queue_backup="uni1_e2e_overviewdiff_queue_$$"
build_backup="uni1_e2e_overviewdiff_build_$$"
db_query "DROP TABLE IF EXISTS $users_backup,$planets_backup,$fleet_backup,$queue_backup,$build_backup;
CREATE TABLE $users_backup AS SELECT * FROM uni1_users WHERE player_id=$actor_id;
CREATE TABLE $planets_backup AS SELECT * FROM uni1_planets WHERE planet_id IN ($home_id,$colony_id,$moon_id);
CREATE TABLE $fleet_backup AS SELECT * FROM uni1_fleet WHERE owner_id=$actor_id OR start_planet IN ($home_id,$colony_id,$moon_id) OR target_planet IN ($home_id,$colony_id,$moon_id);
CREATE TABLE $queue_backup AS SELECT * FROM uni1_queue WHERE owner_id=$actor_id OR sub_id IN ($home_id,$colony_id,$moon_id);
CREATE TABLE $build_backup AS SELECT * FROM uni1_buildqueue WHERE owner_id=$actor_id OR planet_id IN ($home_id,$colony_id,$moon_id)" >/dev/null

restore_user() {
  db_query "UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id SET
u.session=b.session,u.private_session=b.private_session,u.password=b.password,u.validated=b.validated,
u.vacation=b.vacation,u.vacation_until=b.vacation_until,u.disable=b.disable,u.disable_until=b.disable_until,
u.admin=b.admin,u.lang=b.lang,u.aktplanet=b.aktplanet,u.score1=b.score1,u.score2=b.score2,u.score3=b.score3" >/dev/null
}

restore_tables() {
  db_query "DELETE FROM uni1_queue WHERE owner_id=$actor_id OR sub_id IN ($home_id,$colony_id,$moon_id);
INSERT INTO uni1_queue SELECT * FROM $queue_backup;
DELETE FROM uni1_buildqueue WHERE owner_id=$actor_id OR planet_id IN ($home_id,$colony_id,$moon_id);
INSERT INTO uni1_buildqueue SELECT * FROM $build_backup;
DELETE FROM uni1_fleet WHERE owner_id=$actor_id OR start_planet IN ($home_id,$colony_id,$moon_id) OR target_planet IN ($home_id,$colony_id,$moon_id);
INSERT INTO uni1_fleet SELECT * FROM $fleet_backup;
DELETE FROM uni1_planets WHERE planet_id IN ($home_id,$colony_id,$moon_id);
INSERT INTO uni1_planets SELECT * FROM $planets_backup" >/dev/null
  restore_user
}

restore_original() {
  restore_tables
  db_query "DROP TABLE IF EXISTS $users_backup,$planets_backup,$fleet_backup,$queue_backup,$build_backup" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

reset_case() {
  restore_tables
  db_query "UPDATE uni1_users SET session='',private_session='',validated=1,vacation=0,vacation_until=0,
disable=0,disable_until=0,admin=0,lang='en',aktplanet=$home_id,score1=500000,score2=200,score3=300 WHERE player_id=$actor_id;
UPDATE uni1_planets SET remove=0 WHERE planet_id IN ($home_id,$colony_id,$moon_id);
UPDATE uni1_planets SET name='Overview Home',type=1,\`1\`=0 WHERE planet_id=$home_id;
UPDATE uni1_planets SET name='Overview Colony',type=1,\`1\`=2 WHERE planet_id=$colony_id;
UPDATE uni1_planets SET name='Overview Moon',type=0,\`41\`=1 WHERE planet_id=$moon_id;
DELETE FROM uni1_queue WHERE owner_id=$actor_id OR sub_id IN ($home_id,$colony_id,$moon_id);
DELETE FROM uni1_buildqueue WHERE owner_id=$actor_id OR planet_id IN ($home_id,$colony_id,$moon_id);
DELETE FROM uni1_fleet WHERE owner_id=$actor_id OR start_planet IN ($home_id,$colony_id,$moon_id) OR target_planet IN ($home_id,$colony_id,$moon_id)" >/dev/null
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

capture_state() {
  planets="$(db_query "SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT(
'role',role_name,'owner',owner_name,'name',name,'type',type,'removeDelay',remove_delay)),JSON_ARRAY())
FROM (SELECT CASE planet_id WHEN $home_id THEN 'home' WHEN $colony_id THEN 'colony' ELSE 'moon' END role_name,
IF(owner_id=$actor_id,'actor','space') owner_name,name,type,
CASE WHEN remove=0 THEN 0 WHEN ABS(CAST(remove AS SIGNED)-CAST(UNIX_TIMESTAMP() AS SIGNED)-86400)<=10 THEN 86400 ELSE -1 END remove_delay
FROM uni1_planets WHERE planet_id IN ($home_id,$colony_id,$moon_id) ORDER BY planet_id) p")"
  user="$(db_query "SELECT JSON_OBJECT('activePlanet',aktplanet,'score1',score1,'score2',score2,'score3',score3)
FROM uni1_users WHERE player_id=$actor_id")"
  queues="$(db_query "SELECT JSON_OBJECT(
'queue',SUM(CASE WHEN source='queue' THEN amount ELSE 0 END),
'buildQueue',SUM(CASE WHEN source='build' THEN amount ELSE 0 END))
FROM (SELECT 'queue' source,COUNT(*) amount FROM uni1_queue WHERE type <> 'RecalcPoints' AND (owner_id=$actor_id OR sub_id IN ($home_id,$colony_id,$moon_id))
UNION ALL SELECT 'build',COUNT(*) FROM uni1_buildqueue WHERE owner_id=$actor_id OR planet_id IN ($home_id,$colony_id,$moon_id)) q")"
  state="$(jq -ncS --argjson planets "$planets" --argjson user "$user" --argjson queues "$queues" '{planets:$planets,user:$user,queues:$queues}')"
}

configure_case() {
  case_name="$1"; action=rename; selected_id="$home_id"; target_id="$home_id"; name='Renamed World'; password="$PASSWORD"; expected_issue=''
  case "$case_name" in
    rename-valid) ;;
    rename-empty) name='' ;;
    rename-long) name='abcdefghijklmnopqrstuvwxyz0123456789' ;;
    rename-forbidden) name='Bad;Name' ;;
    rename-moon) selected_id="$moon_id"; target_id="$moon_id"; name='Luna' ;;
    delete-password) action=delete; selected_id="$colony_id"; target_id="$colony_id"; password='wrong'; expected_issue=password_invalid ;;
    delete-home) action=delete; target_id="$home_id"; expected_issue=home_planet ;;
    delete-foreign) action=delete; target_id="$foreign_id" ;;
    delete-incoming)
      action=delete; selected_id="$colony_id"; target_id="$colony_id"; expected_issue=fleet_incoming
      db_query "INSERT INTO uni1_fleet(owner_id,mission,start_planet,target_planet,flight_time,deploy_time)
VALUES($actor_id,3,$home_id,$colony_id,UNIX_TIMESTAMP()+600,UNIX_TIMESTAMP()+1200)" >/dev/null ;;
    delete-outgoing)
      action=delete; selected_id="$colony_id"; target_id="$colony_id"; expected_issue=fleet_outgoing
      db_query "INSERT INTO uni1_fleet(owner_id,mission,start_planet,target_planet,flight_time,deploy_time)
VALUES($actor_id,3,$colony_id,$home_id,UNIX_TIMESTAMP()+600,UNIX_TIMESTAMP()+1200)" >/dev/null ;;
    delete-colony)
      action=delete; selected_id="$colony_id"; target_id="$colony_id"
      db_query "INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES
($actor_id,'Build',$colony_id,1,3,UNIX_TIMESTAMP(),UNIX_TIMESTAMP()+600,10);
INSERT INTO uni1_buildqueue(owner_id,planet_id,list_id,tech_id,level,destroy,start,end) VALUES
($actor_id,$colony_id,1,1,3,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP()+600)" >/dev/null ;;
    delete-moon)
      action=delete; selected_id="$moon_id"; target_id="$moon_id"
      db_query "INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES
($actor_id,'Build',$moon_id,41,2,UNIX_TIMESTAMP(),UNIX_TIMESTAMP()+600,10);
INSERT INTO uni1_buildqueue(owner_id,planet_id,list_id,tech_id,level,destroy,start,end) VALUES
($actor_id,$moon_id,1,41,2,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP()+600)" >/dev/null ;;
    *) printf 'Unknown overview differential case: %s\n' "$case_name" >&2; exit 1 ;;
  esac
  db_query "UPDATE uni1_users SET aktplanet=$selected_id WHERE player_id=$actor_id" >/dev/null
}

legacy_issue() {
  issue=''
  grep -q 'password is wrong' "$TMP_DIR/legacy-$case_name.body" && issue=password_invalid || true
  grep -q "can't abandon the home planet" "$TMP_DIR/legacy-$case_name.body" && issue=home_planet || true
  grep -q 'still on their way to this planet' "$TMP_DIR/legacy-$case_name.body" && issue=fleet_incoming || true
  grep -q 'have not yet returned' "$TMP_DIR/legacy-$case_name.body" && issue=fleet_outgoing || true
}

run_legacy_action() {
  url="$LEGACY_BASE_URL/game/index.php?page=renameplanet&session=$session&cp=$selected_id&pl=$selected_id"
  if [ "$action" = rename ]; then
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/legacy-$case_name.body" \
      --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --data-urlencode 'aktion=Rename' \
      --data-urlencode "newname=$name" --write-out '%{http_code}' "$url")"
  else
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/legacy-$case_name.body" \
      --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --data-urlencode 'aktion=Delete the planet!' \
      --data-urlencode "deleteid=$target_id" --data-urlencode "pw=$password" --write-out '%{http_code}' "$url")"
  fi
  legacy_issue
}

run_go_action() {
  if [ "$action" = rename ]; then
    payload="$(jq -nc --arg name "$name" '{action:"rename",name:$name}')"
  else
    payload="$(jq -nc --argjson id "$target_id" --arg password "$password" '{action:"delete",deleteId:$id,password:$password}')"
  fi
  http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/go-$case_name.body" \
    --header 'Content-Type: application/json' --cookie "$cookie" --data "$payload" --write-out '%{http_code}' \
    "$GO_BASE_URL/api/game/overview?session=$session&cp=$selected_id")"
  issue="$(jq -r '.actionIssue.code // empty' "$TMP_DIR/go-$case_name.body" 2>/dev/null || true)"
}

run_side() {
  side="$1"; case_name="$2"
  reset_case; configure_case "$case_name"
  if [ "$side" = legacy ]; then
    login_legacy "$side-$case_name"
    db_query "UPDATE uni1_users SET aktplanet=$selected_id WHERE player_id=$actor_id" >/dev/null
    run_legacy_action
  else
    login_go "$side-$case_name"
    db_query "UPDATE uni1_users SET aktplanet=$selected_id WHERE player_id=$actor_id" >/dev/null
    run_go_action
  fi
  capture_state
  normalized="$(jq -ncS --argjson http "$http" --arg issue "$issue" --argjson state "$state" '{http:$http,issue:$issue,state:$state}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true; case_count=0
cases='rename-valid rename-empty rename-long rename-forbidden rename-moon delete-password delete-home delete-foreign delete-incoming delete-outgoing delete-colony delete-moon'
for case_name in $cases; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.state')" = "$(printf '%s' "$go" | jq -cS '.state')" ] || pass=false
  [ "$(printf '%s' "$legacy" | jq -r '.issue')" = "$(printf '%s' "$go" | jq -r '.issue')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http == 200 or .http == 302' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http == 200' >/dev/null || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "overview-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
  case_count=$((case_count + 1))
done

jq -s --argjson pass "$all_pass" --argjson cases "$case_count" \
  '{pass:$pass,normalization:"session secrets, resource ticks and absolute destroy times excluded; names/ownership/types/score/active planet/queue effects exact",metrics:{cases:$cases},cases:.}' \
  "$results" > "$REPORT"
[ "$all_pass" = true ] || { jq '.cases[] | select(.pass == false)' "$REPORT" >&2; exit 1; }
printf 'Go/PHP overview differential E2E: PASS (%s cases)\n' "$case_count"
