#!/bin/sh
set -eu
[ "${OGAME_E2E_TRACE:-0}" != "1" ] && [ "${1:-}" != "--trace" ] || set -x

ROOT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_GALAXY_ACTIONS_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-galaxy-actions-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/galaxy-actions-differential.XXXXXX")"

instant_id="$(jq -r '.fleet_restrictions.attacker.player_id // 0' "$FIXTURE")"
instant_login="$(jq -r '.fleet_restrictions.attacker.login // empty' "$FIXTURE")"
instant_planet="$(jq -r '.fleet_restrictions.attacker.home_planet_id // 0' "$FIXTURE")"
instant_target_id="$(jq -r '.fleet_restrictions.comparable.player_id // 0' "$FIXTURE")"
instant_target_planet="$(jq -r '.fleet_restrictions.comparable.home_planet_id // 0' "$FIXTURE")"
target_g="$(jq -r '.fleet_restrictions.comparable.coordinates.galaxy // 0' "$FIXTURE")"
target_s="$(jq -r '.fleet_restrictions.comparable.coordinates.system // 0' "$FIXTURE")"
target_p="$(jq -r '.fleet_restrictions.comparable.coordinates.position // 0' "$FIXTURE")"
missile_id="$(jq -r '.galaxy_missile.attacker.player_id // 0' "$FIXTURE")"
missile_login="$(jq -r '.galaxy_missile.attacker.login // empty' "$FIXTURE")"
missile_planet="$(jq -r '.galaxy_missile.attacker.home_planet_id // 0' "$FIXTURE")"
missile_target_id="$(jq -r '.galaxy_missile.target.player_id // 0' "$FIXTURE")"
missile_target_planet="$(jq -r '.galaxy_missile.target.home_planet_id // 0' "$FIXTURE")"
missile_target_g="$(jq -r '.galaxy_missile.target.coordinates.galaxy // 0' "$FIXTURE")"
missile_target_s="$(jq -r '.galaxy_missile.target.coordinates.system // 0' "$FIXTURE")"
[ "$instant_id" -gt 0 ] && [ "$instant_planet" -gt 0 ] && [ "$instant_target_id" -gt 0 ] && [ "$instant_target_planet" -gt 0 ]
[ "$target_g" -gt 0 ] && [ "$target_s" -gt 0 ] && [ "$target_p" -gt 0 ] && [ -n "$instant_login" ]
[ "$missile_id" -gt 0 ] && [ "$missile_planet" -gt 0 ] && [ "$missile_target_id" -gt 0 ] && [ "$missile_target_planet" -gt 0 ] && [ -n "$missile_login" ]

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

users_backup="uni1_e2e_galactdiff_users_$$"
planets_backup="uni1_e2e_galactdiff_planets_$$"
fleet_backup="uni1_e2e_galactdiff_fleet_$$"
queue_backup="uni1_e2e_galactdiff_queue_$$"
logs_backup="uni1_e2e_galactdiff_fleetlogs_$$"
userlog_max="$(db_query "SELECT COALESCE(MAX(id),0) FROM uni1_userlogs WHERE owner_id IN ($instant_id,$missile_id)")"
db_query "DROP TABLE IF EXISTS $users_backup,$planets_backup,$fleet_backup,$queue_backup,$logs_backup;
CREATE TABLE $users_backup AS SELECT * FROM uni1_users WHERE player_id IN ($instant_id,$instant_target_id,$missile_id,$missile_target_id);
CREATE TABLE $planets_backup AS SELECT * FROM uni1_planets WHERE planet_id IN ($instant_planet,$instant_target_planet,$missile_planet,$missile_target_planet)
OR (g=$target_g AND s=$target_s AND p=$target_p AND type=2);
CREATE TABLE $fleet_backup AS SELECT * FROM uni1_fleet WHERE owner_id IN ($instant_id,$missile_id);
CREATE TABLE $queue_backup AS SELECT * FROM uni1_queue WHERE owner_id IN ($instant_id,$missile_id);
CREATE TABLE $logs_backup AS SELECT * FROM uni1_fleetlogs WHERE owner_id IN ($instant_id,$missile_id)" >/dev/null

restore_state() {
  db_query "DELETE FROM uni1_fleet WHERE owner_id IN ($instant_id,$missile_id); INSERT INTO uni1_fleet SELECT * FROM $fleet_backup;
DELETE FROM uni1_queue WHERE owner_id IN ($instant_id,$missile_id); INSERT INTO uni1_queue SELECT * FROM $queue_backup;
DELETE FROM uni1_fleetlogs WHERE owner_id IN ($instant_id,$missile_id); INSERT INTO uni1_fleetlogs SELECT * FROM $logs_backup;
DELETE FROM uni1_userlogs WHERE owner_id IN ($instant_id,$missile_id) AND id>$userlog_max;
DELETE FROM uni1_planets WHERE g=$target_g AND s=$target_s AND p=$target_p AND type=2;
INSERT INTO uni1_planets SELECT * FROM $planets_backup WHERE type=2;
UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id SET
u.session=b.session,u.private_session=b.private_session,u.password=b.password,u.validated=b.validated,u.admin=b.admin,u.lang=b.lang,
u.aktplanet=b.aktplanet,u.vacation=b.vacation,u.vacation_until=b.vacation_until,u.noattack=b.noattack,u.noattack_until=b.noattack_until,
u.banned=b.banned,u.banned_until=b.banned_until,u.disable=b.disable,u.disable_until=b.disable_until,u.ally_id=b.ally_id,
u.deact_ip=b.deact_ip,u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.\`108\`=b.\`108\`,u.\`115\`=b.\`115\`,u.\`117\`=b.\`117\`;
UPDATE uni1_planets p JOIN $planets_backup b ON b.planet_id=p.planet_id SET
p.\`209\`=b.\`209\`,p.\`210\`=b.\`210\`,p.\`503\`=b.\`503\`,p.\`700\`=b.\`700\`,p.\`701\`=b.\`701\`,p.\`702\`=b.\`702\`,
p.prod1=b.prod1,p.prod2=b.prod2,p.prod3=b.prod3,p.prod4=b.prod4,p.prod12=b.prod12,p.prod212=b.prod212,p.lastpeek=b.lastpeek,p.lastakt=b.lastakt" >/dev/null
}

cleanup() {
  restore_state >/dev/null 2>&1 || true
  db_query "DROP TABLE IF EXISTS $users_backup,$planets_backup,$fleet_backup,$queue_backup,$logs_backup" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

reset_instant() {
  restore_state
  actor_id="$instant_id"; actor_login="$instant_login"; actor_planet="$instant_planet"
  db_query "DELETE FROM uni1_fleet WHERE owner_id=$instant_id; DELETE FROM uni1_queue WHERE owner_id=$instant_id;
DELETE FROM uni1_fleetlogs WHERE owner_id=$instant_id; DELETE FROM uni1_userlogs WHERE owner_id=$instant_id AND id>$userlog_max;
DELETE FROM uni1_planets WHERE g=$target_g AND s=$target_s AND p=$target_p AND type=2;
INSERT INTO uni1_planets(name,type,g,s,p,owner_id,diameter,temp,fields,maxfields,date,\`700\`,\`701\`,\`702\`,lastpeek,lastakt,gate_until,remove)
VALUES('E2E Debris',2,$target_g,$target_s,$target_p,$instant_target_id,0,0,0,0,UNIX_TIMESTAMP(),500000,250000,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP(),0,0);
UPDATE uni1_users SET session='',private_session='',validated=1,admin=0,vacation=0,vacation_until=0,noattack=0,noattack_until=0,
banned=0,banned_until=0,disable=0,disable_until=0,ally_id=0,deact_ip=1,score1=100000,score2=0,score3=0,
aktplanet=$instant_planet,\`108\`=12,\`115\`=12 WHERE player_id=$instant_id;
UPDATE uni1_users SET admin=0,vacation=0,noattack=0,banned=0,disable=0,ally_id=0,deact_ip=1,score1=100000,lastclick=UNIX_TIMESTAMP()
WHERE player_id=$instant_target_id;
UPDATE uni1_planets SET \`209\`=5,\`210\`=10,\`700\`=1000000,\`701\`=1000000,\`702\`=1000000,
prod1=0,prod2=0,prod3=0,prod4=0,prod12=0,prod212=0,lastpeek=UNIX_TIMESTAMP() WHERE planet_id=$instant_planet" >/dev/null
}

reset_missile() {
  restore_state
  actor_id="$missile_id"; actor_login="$missile_login"; actor_planet="$missile_planet"
  db_query "DELETE FROM uni1_fleet WHERE owner_id=$missile_id; DELETE FROM uni1_queue WHERE owner_id=$missile_id;
DELETE FROM uni1_fleetlogs WHERE owner_id=$missile_id; DELETE FROM uni1_userlogs WHERE owner_id=$missile_id AND id>$userlog_max;
UPDATE uni1_users SET session='',private_session='',validated=1,admin=0,vacation=0,vacation_until=0,noattack=0,noattack_until=0,
banned=0,banned_until=0,disable=0,disable_until=0,ally_id=0,deact_ip=1,score1=100000,score2=0,score3=0,
aktplanet=$missile_planet,\`117\`=10 WHERE player_id=$missile_id;
UPDATE uni1_users SET admin=0,vacation=0,noattack=0,banned=0,disable=0,ally_id=0,deact_ip=1,score1=100000,lastclick=UNIX_TIMESTAMP()
WHERE player_id=$missile_target_id;
UPDATE uni1_planets SET \`503\`=5,prod1=0,prod2=0,prod3=0,prod4=0,prod12=0,prod212=0,lastpeek=UNIX_TIMESTAMP()
WHERE planet_id=$missile_planet" >/dev/null
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
  case_name="$1"; group=instant; mission=6; amount=2; selected_target_type=1; selected_target_g="$target_g"; selected_target_s="$target_s"; selected_target_p="$target_p"; selected_target_id="$instant_target_planet"; defense_id=401
  case "$case_name" in
    spy-success) ;;
    recycle-success) mission=8; selected_target_type=2 ;;
    spy-zero) amount=0 ;;
    recycle-zero) mission=8; selected_target_type=2; amount=0 ;;
    spy-vacation) db_query "UPDATE uni1_users SET vacation=1,vacation_until=UNIX_TIMESTAMP()+86400 WHERE player_id=$instant_id" >/dev/null ;;
    spy-invalid-target) selected_target_p=16 ;;
    missile-success) group=missile; amount=2; selected_target_id="$missile_target_planet" ;;
    missile-zero) group=missile; amount=0; selected_target_id="$missile_target_planet" ;;
    missile-insufficient) group=missile; amount=9; selected_target_id="$missile_target_planet" ;;
    missile-no-target) group=missile; amount=1; selected_target_id=999999999 ;;
    missile-self) group=missile; amount=1; selected_target_id="$missile_planet" ;;
    missile-vacation)
      group=missile; amount=1; selected_target_id="$missile_target_planet"
      db_query "UPDATE uni1_users SET vacation=1,vacation_until=UNIX_TIMESTAMP()+86400 WHERE player_id=$missile_id" >/dev/null ;;
    *) printf 'Unknown galaxy actions differential case: %s\n' "$case_name" >&2; exit 1 ;;
  esac
}

capture_state() {
  origin="$(db_query "SELECT JSON_OBJECT('probe',\`210\`,'recycler',\`209\`,'missile',\`503\`,'deuterium',FLOOR(\`702\`))
FROM uni1_planets WHERE planet_id=$actor_planet")"
  fleets="$(db_query "SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT('mission',f.mission,'target',CASE WHEN f.target_planet=$instant_target_planet THEN 'instantTarget'
WHEN f.target_planet=$missile_target_planet THEN 'missileTarget' WHEN f.target_planet=$missile_planet THEN 'self'
WHEN tp.type=2 THEN 'debris' ELSE 'other' END,'probe',f.\`210\`,'recycler',f.\`209\`,'ipm',f.ipm_amount,'ipmTarget',f.ipm_target,'fuel',f.fuel)),JSON_ARRAY())
FROM (SELECT * FROM uni1_fleet WHERE owner_id=$actor_id ORDER BY fleet_id) f LEFT JOIN uni1_planets tp ON tp.planet_id=f.target_planet")"
  state="$(jq -ncS --argjson origin "$origin" --argjson fleets "$fleets" '{origin:$origin,fleets:$fleets}')"
}

run_legacy_action() {
  if [ "$group" = instant ]; then
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/legacy-$case_name.body" \
      --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --data-urlencode "order=$mission" \
      --data-urlencode "session=$session" \
      --data-urlencode "galaxy=$selected_target_g" --data-urlencode "system=$selected_target_s" --data-urlencode "planet=$selected_target_p" \
      --data-urlencode "planettype=$selected_target_type" --data-urlencode "shipcount=$amount" --write-out '%{http_code}' \
      "$LEGACY_BASE_URL/game/index.php?ajax=1&page=flottenversand&session=$session&cp=$actor_planet")"
  else
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/legacy-$case_name.body" \
      --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --data-urlencode 'aktion=1' --data-urlencode "anz=$amount" \
      --data-urlencode "pziel=$defense_id" --write-out '%{http_code}' \
      "$LEGACY_BASE_URL/game/index.php?page=galaxy&session=$session&cp=$actor_planet&pdd=$selected_target_id&galaxy=$missile_target_g&system=$missile_target_s")"
  fi
  [ "$http" = 200 ]
  action_result="$(tr -d '\r\n' < "$TMP_DIR/legacy-$case_name.body" | head -c 200)"
}

run_go_action() {
  if [ "$group" = instant ]; then
    action=dispatch-spy; [ "$mission" -eq 6 ] || action=dispatch-recycle
    payload="$(jq -nc --arg action "$action" --argjson g "$selected_target_g" --argjson s "$selected_target_s" --argjson p "$selected_target_p" \
      --argjson type "$selected_target_type" --argjson amount "$amount" '{action:$action,targetGalaxy:$g,targetSystem:$s,targetPosition:$p,targetType:$type,amount:$amount}')"
    query_g="$selected_target_g"; query_s="$selected_target_s"
  else
    payload="$(jq -nc --argjson id "$selected_target_id" --argjson amount "$amount" --argjson defense "$defense_id" \
      '{action:"launch-missile",targetPlanetId:$id,amount:$amount,targetDefenseId:$defense}')"
    query_g="$missile_target_g"; query_s="$missile_target_s"
  fi
  http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/go-$case_name.body" \
    --header 'Content-Type: application/json' --cookie "$cookie" --data "$payload" --write-out '%{http_code}' \
    "$GO_BASE_URL/api/game/galaxy?session=$session&cp=$actor_planet&galaxy=$query_g&system=$query_s")"
  [ "$http" = 200 ]
  action_result="$(jq -r '.actionIssue.code // empty' "$TMP_DIR/go-$case_name.body" 2>/dev/null || true)"
}

run_side() {
  side="$1"; case_name="$2"
  case "$case_name" in missile-*) reset_missile ;; *) reset_instant ;; esac
  configure_case "$case_name"
  if [ "$side" = legacy ]; then login_legacy "$side-$case_name"; run_legacy_action
  else login_go "$side-$case_name"; run_go_action; fi
  capture_state; normalized="$state"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true; case_count=0
cases='spy-success recycle-success spy-zero recycle-zero spy-vacation spy-invalid-target missile-success missile-zero missile-insufficient missile-no-target missile-self missile-vacation'
for case_name in $cases; do
  run_side legacy "$case_name"; legacy="$normalized"; legacy_result="$action_result"
  run_side go "$case_name"; go="$normalized"; go_result="$action_result"
  pass=true; [ "$legacy" = "$go" ] || pass=false; [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "galaxy-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    --arg legacyResult "$legacy_result" --arg goResult "$go_result" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go,transport:{legacy:$legacyResult,go:$goResult}}' >> "$results"
  case_count=$((case_count + 1))
done

jq -s --argjson pass "$all_pass" --argjson cases "$case_count" \
  '{pass:$pass,normalization:"session, random fleet IDs and absolute flight clocks excluded; origin units/fuel and fleet mission payload exact",metrics:{cases:$cases},cases:.}' \
  "$results" > "$REPORT"
[ "$all_pass" = true ] || { jq '.cases[] | select(.pass == false)' "$REPORT" >&2; exit 1; }
printf 'Go/PHP galaxy actions differential E2E: PASS (%s cases)\n' "$case_count"
