#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_ADMIN_CLEANUP_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-admin-cleanup-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/admin-cleanup-differential.XXXXXX")"

admin_id="$(jq -r '.player_id // 0' "$FIXTURE")"
target_id="$(jq -r '.admin_operations.target_player_id // 0' "$FIXTURE")"
if [ "$admin_id" -le 0 ] || [ "$target_id" -le 0 ]; then
  printf 'Invalid admin cleanup fixture IDs\n' >&2
  exit 1
fi

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

marker=$((920000000 + ($$ % 1000000) * 10))
expired_planet=$((marker + 1)); future_planet=$((marker + 2))
disabled_user=$((marker + 10)); admin_user=$((marker + 11)); inactive_user=$((marker + 12)); dm_user=$((marker + 13)); bot_user=$((marker + 14))
disabled_planet=$((marker + 110)); admin_user_planet=$((marker + 111)); inactive_planet=$((marker + 112)); dm_planet=$((marker + 113)); bot_planet=$((marker + 114))
queue_backup="uni1_e2e_cleanup_queue_$$"
uni_backup="uni1_e2e_cleanup_uni_$$"
rank_backup="uni1_e2e_cleanup_rank_$$"
debug_backup="uni1_e2e_cleanup_debug_$$"
role_backup="uni1_e2e_cleanup_role_$$"

admin_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$admin_id")"
[ "$admin_planet" -gt 0 ]

db_query "DROP TABLE IF EXISTS $queue_backup,$uni_backup,$rank_backup,$debug_backup,$role_backup;
CREATE TABLE $queue_backup AS SELECT * FROM uni1_queue WHERE type IN ('CleanPlanets','CleanPlayers');
CREATE TABLE $uni_backup AS SELECT num,usercount FROM uni1_uni;
CREATE TABLE $rank_backup AS SELECT player_id,score1,score2,score3,place1,place2,place3 FROM uni1_users;
CREATE TABLE $debug_backup AS SELECT * FROM uni1_debug WHERE text LIKE 'Cleanup of destroyed planets (%' OR text LIKE 'Aufräumen zerstörter Planeten (%' OR text LIKE 'Чистка уничтоженных планет (%';
CREATE TABLE $role_backup AS SELECT player_id,admin FROM uni1_users WHERE player_id=$admin_id" >/dev/null

clear_fixture() {
  db_query "DELETE FROM uni1_queue WHERE type IN ('CleanPlanets','CleanPlayers') OR owner_id BETWEEN $marker AND $((marker+200));
DELETE FROM uni1_reports WHERE owner_id BETWEEN $marker AND $((marker+200));
DELETE FROM uni1_messages WHERE owner_id BETWEEN $marker AND $((marker+200));
DELETE FROM uni1_notes WHERE owner_id BETWEEN $marker AND $((marker+200));
DELETE FROM uni1_browse WHERE owner_id BETWEEN $marker AND $((marker+200));
DELETE FROM uni1_template WHERE owner_id BETWEEN $marker AND $((marker+200));
DELETE FROM uni1_botvars WHERE owner_id BETWEEN $marker AND $((marker+200));
DELETE FROM uni1_userlogs WHERE owner_id BETWEEN $marker AND $((marker+200));
DELETE FROM uni1_iplogs WHERE user_id BETWEEN $marker AND $((marker+200));
DELETE FROM uni1_union WHERE target_player BETWEEN $marker AND $((marker+200)) OR players REGEXP '(^|,)${marker}[0-9]*(,|$)';
DELETE FROM uni1_allyapps WHERE player_id BETWEEN $marker AND $((marker+200));
DELETE FROM uni1_buddy WHERE request_from BETWEEN $marker AND $((marker+200)) OR request_to BETWEEN $marker AND $((marker+200));
DELETE FROM uni1_buildqueue WHERE owner_id BETWEEN $marker AND $((marker+200)) OR planet_id BETWEEN $marker AND $((marker+200));
DELETE FROM uni1_fleet WHERE owner_id BETWEEN $marker AND $((marker+200)) OR start_planet BETWEEN $marker AND $((marker+200)) OR target_planet BETWEEN $marker AND $((marker+200));
DELETE FROM uni1_planets WHERE planet_id BETWEEN $marker AND $((marker+200)) OR owner_id BETWEEN $marker AND $((marker+200));
DELETE FROM uni1_users WHERE player_id BETWEEN $marker AND $((marker+200));
DELETE FROM uni1_debug WHERE text LIKE 'Cleanup of destroyed planets (%' OR text LIKE 'Aufräumen zerstörter Planeten (%' OR text LIKE 'Чистка уничтоженных планет (%';
INSERT INTO uni1_debug SELECT * FROM $debug_backup;
INSERT INTO uni1_queue SELECT * FROM $queue_backup;
UPDATE uni1_uni u JOIN $uni_backup b ON b.num=u.num SET u.usercount=b.usercount;
UPDATE uni1_users u JOIN $rank_backup b ON b.player_id=u.player_id SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3;
UPDATE uni1_users SET admin=2 WHERE player_id=$admin_id" >/dev/null
}

cleanup() {
  clear_fixture >/dev/null 2>&1 || true
  db_query "UPDATE uni1_users u JOIN $role_backup b ON b.player_id=u.player_id SET u.admin=b.admin;
DROP TABLE IF EXISTS $queue_backup,$uni_backup,$rank_backup,$debug_backup,$role_backup" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

admin_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$admin_id")"

login_legacy() {
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/legacy-login.headers" --output "$TMP_DIR/legacy-login.body" \
    --cookie-jar "$TMP_DIR/legacy.cookies" --request POST --data-urlencode "login=$admin_login" --data-urlencode "pass=$PASSWORD" \
    --write-out '%{http_code}' "$LEGACY_BASE_URL/game/reg/login2.php")"
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/legacy-login.headers")"
  session="$(printf '%s' "$location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 302 ] && [ -n "$session" ]
}

login_go() {
  payload="$(jq -nc --arg login "$admin_login" --arg pass "$PASSWORD" '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/go-login.headers" --output "$TMP_DIR/go-login.body" \
    --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/go-login.headers" | cut -d';' -f1 | tr -d '\r')"
  redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/go-login.body")"
  session="$(printf '%s' "$redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 200 ] && [ -n "$cookie" ] && [ -n "$session" ]
}

clone_planet() {
  planet_id="$1"; owner_id="$2"; remove_at="$3"; planet_type="$4"
  table_name="uni1_e2e_cleanup_planet_${planet_id}"
  db_query "DROP TABLE IF EXISTS $table_name; CREATE TABLE $table_name AS SELECT * FROM uni1_planets WHERE planet_id=$admin_planet LIMIT 1;
UPDATE $table_name SET planet_id=$planet_id,owner_id=$owner_id,type=$planet_type,remove=$remove_at,name='e2e cleanup';
INSERT INTO uni1_planets SELECT * FROM $table_name; DROP TABLE $table_name" >/dev/null
}

clone_user() {
  player_id="$1"; planet_id="$2"; suffix="$3"
  table_name="uni1_e2e_cleanup_user_${player_id}"
  db_query "DROP TABLE IF EXISTS $table_name; CREATE TABLE $table_name AS SELECT * FROM uni1_users WHERE player_id=$target_id LIMIT 1;
UPDATE $table_name SET player_id=$player_id,name='e2ecu${suffix}',oname='e2ecu${suffix}',email='e2ecu${suffix}@example.local',pemail='e2ecu${suffix}@example.local',hplanetid=$planet_id,admin=0,ally_id=0,allyrank=0,disable=0,disable_until=0,lastclick=UNIX_TIMESTAMP(),dm=0,session='',private_session='';
INSERT INTO uni1_users SELECT * FROM $table_name; DROP TABLE $table_name" >/dev/null
  clone_planet "$planet_id" "$player_id" 0 1
}

setup_planets() {
  clear_fixture
  now="$(db_query 'SELECT UNIX_TIMESTAMP()')"
  clone_planet "$expired_planet" "$target_id" "$((now-20))" 3
  clone_planet "$future_planet" "$target_id" "$((now+86400))" 3
  db_query "INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES(99999,'CleanPlanets',0,$marker,0,$((now-10)),$((now-1)),700)" >/dev/null
  [ "$(db_query "SELECT COUNT(*) FROM uni1_planets WHERE planet_id IN ($expired_planet,$future_planet)")" = 2 ]
}

setup_players() {
  clear_fixture
  now="$(db_query 'SELECT UNIX_TIMESTAMP()')"
  clone_user "$disabled_user" "$disabled_planet" disabled
  clone_user "$admin_user" "$admin_user_planet" admin
  clone_user "$inactive_user" "$inactive_planet" inactive
  clone_user "$dm_user" "$dm_planet" dm
  clone_user "$bot_user" "$bot_planet" bot
  db_query "UPDATE uni1_users SET disable=1,disable_until=$((now-20)),lastclick=$now WHERE player_id=$disabled_user;
UPDATE uni1_users SET disable=1,disable_until=$((now-20)),lastclick=$now,admin=1 WHERE player_id=$admin_user;
UPDATE uni1_users SET lastclick=$((now-36*86400)),dm=0 WHERE player_id=$inactive_user;
UPDATE uni1_users SET lastclick=$((now-36*86400)),dm=100 WHERE player_id=$dm_user;
UPDATE uni1_users SET lastclick=$((now-36*86400)),dm=0 WHERE player_id=$bot_user;
INSERT INTO uni1_messages(owner_id,pm,msgfrom,subj,text,shown,date,planet_id) VALUES($disabled_user,0,'e2e','e2e','e2e',0,$now,$disabled_planet);
SET @msg=LAST_INSERT_ID(); INSERT INTO uni1_reports(owner_id,msg_id,msgfrom,subj,text,date) VALUES($disabled_user,@msg,'e2e','e2e','e2e',$now);
INSERT INTO uni1_notes(owner_id,subj,text,textsize,prio,date) VALUES($disabled_user,'e2e','e2e',3,0,$now);
INSERT INTO uni1_browse(owner_id,url,method,getdata,postdata,date) VALUES($disabled_user,'/e2e','GET','','',$now);
INSERT INTO uni1_template(owner_id,name,date) VALUES($disabled_user,'e2e',$now);
INSERT INTO uni1_botvars(owner_id,var,value) VALUES($disabled_user,'e2e','1');
INSERT INTO uni1_userlogs(owner_id,date,type,text) VALUES($disabled_user,$now,'E2E','e2e');
INSERT INTO uni1_iplogs(ip,user_id,reg,date) VALUES('127.0.0.1',$disabled_user,0,$now);
INSERT INTO uni1_union(fleet_id,target_player,name,players) VALUES(0,$disabled_user,'e2e','$disabled_user,$target_id');
INSERT INTO uni1_allyapps(ally_id,player_id,text,date) VALUES(0,$disabled_user,'e2e',$now);
INSERT INTO uni1_buddy(request_from,request_to,text,accepted) VALUES($disabled_user,$target_id,'e2e',0);
INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES
($bot_user,'AI',0,0,0,$now,$((now+86400)),100),
(99999,'CleanPlayers',0,$((marker+1)),0,$((now-10)),$((now-1)),900);
UPDATE uni1_uni SET usercount=usercount+5" >/dev/null
  [ "$(db_query "SELECT COUNT(*) FROM uni1_users WHERE player_id IN ($disabled_user,$admin_user,$inactive_user,$dm_user,$bot_user)")" = 5 ]
  [ "$(db_query "SELECT COUNT(*) FROM uni1_planets WHERE planet_id IN ($disabled_planet,$admin_user_planet,$inactive_planet,$dm_planet,$bot_planet)")" = 5 ]
}

capture_planets() {
  expired="$(db_query "SELECT COUNT(*) FROM uni1_planets WHERE planet_id=$expired_planet")"
  future="$(db_query "SELECT COUNT(*) FROM uni1_planets WHERE planet_id=$future_planet")"
  scheduled="$(db_query "SELECT COUNT(*) FROM uni1_queue WHERE type='CleanPlanets' AND end>UNIX_TIMESTAMP()")"
  slot="$(db_query "SELECT CONCAT(DAYOFWEEK(FROM_UNIXTIME(end)),':',HOUR(FROM_UNIXTIME(end)),':',MINUTE(FROM_UNIXTIME(end))) FROM uni1_queue WHERE type='CleanPlanets' LIMIT 1")"
  debug="$(db_query "SELECT COALESCE((SELECT text FROM uni1_debug WHERE text LIKE '%destroyed planets (%' OR text LIKE '%zerstörter Planeten (%' OR text LIKE '%уничтоженных планет (%' ORDER BY error_id DESC LIMIT 1),'')")"
  jq -ncS --argjson expired "$expired" --argjson future "$future" --argjson scheduled "$scheduled" --arg slot "$slot" --arg debug "$debug" '{expired:$expired,future:$future,scheduled:$scheduled,slot:$slot,debug:$debug}'
}

capture_players() {
  disabled="$(db_query "SELECT COUNT(*) FROM uni1_users WHERE player_id=$disabled_user")"
  disabled_planet_left="$(db_query "SELECT COUNT(*) FROM uni1_planets WHERE planet_id=$disabled_planet")"
  admin_left="$(db_query "SELECT COUNT(*) FROM uni1_users WHERE player_id=$admin_user AND admin=1")"
  inactive="$(db_query "SELECT COUNT(*) FROM uni1_users WHERE player_id=$inactive_user")"
  dm="$(db_query "SELECT COUNT(*) FROM uni1_users WHERE player_id=$dm_user AND dm=100")"
  bot="$(db_query "SELECT COUNT(*) FROM uni1_users WHERE player_id=$bot_user")"
  bot_ai="$(db_query "SELECT COUNT(*) FROM uni1_queue WHERE owner_id=$bot_user AND type='AI'")"
  scheduled="$(db_query "SELECT COUNT(*) FROM uni1_queue WHERE type='CleanPlayers' AND end>UNIX_TIMESTAMP()")"
  slot="$(db_query "SELECT CONCAT(DAYOFWEEK(FROM_UNIXTIME(end)),':',HOUR(FROM_UNIXTIME(end)),':',MINUTE(FROM_UNIXTIME(end))) FROM uni1_queue WHERE type='CleanPlayers' LIMIT 1")"
  aux="$(db_query "SELECT
(SELECT COUNT(*) FROM uni1_reports WHERE owner_id=$disabled_user)+(SELECT COUNT(*) FROM uni1_messages WHERE owner_id=$disabled_user)+(SELECT COUNT(*) FROM uni1_notes WHERE owner_id=$disabled_user)+(SELECT COUNT(*) FROM uni1_browse WHERE owner_id=$disabled_user)+(SELECT COUNT(*) FROM uni1_template WHERE owner_id=$disabled_user)+(SELECT COUNT(*) FROM uni1_botvars WHERE owner_id=$disabled_user)+(SELECT COUNT(*) FROM uni1_userlogs WHERE owner_id=$disabled_user)+(SELECT COUNT(*) FROM uni1_iplogs WHERE user_id=$disabled_user)+(SELECT COUNT(*) FROM uni1_union WHERE target_player=$disabled_user OR players REGEXP '(^|,)$disabled_user(,|$)')+(SELECT COUNT(*) FROM uni1_allyapps WHERE player_id=$disabled_user)+(SELECT COUNT(*) FROM uni1_buddy WHERE request_from=$disabled_user OR request_to=$disabled_user)")"
  jq -ncS --argjson disabled "$disabled" --argjson disabledPlanet "$disabled_planet_left" --argjson admin "$admin_left" --argjson inactive "$inactive" --argjson dm "$dm" --argjson bot "$bot" --argjson botAI "$bot_ai" --argjson scheduled "$scheduled" --arg slot "$slot" --argjson auxiliary "$aux" '{disabled:$disabled,disabledPlanet:$disabledPlanet,admin:$admin,inactive:$inactive,dm:$dm,bot:$bot,botAI:$botAI,scheduled:$scheduled,slot:$slot,auxiliary:$auxiliary}'
}

run_side() {
  side="$1"; case_name="$2"
  if [ "$case_name" = planets ]; then setup_planets; else setup_players; fi
  if [ "$side" = legacy ]; then
    login_legacy
    http="$(curl --silent --show-error --max-time 60 --output "$TMP_DIR/$side-$case_name.body" --cookie "$TMP_DIR/legacy.cookies" \
      --request POST --data 'order_cron=1' --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$admin_planet&mode=Queue")"
    issue=""
  else
    login_go
    http="$(curl --silent --show-error --max-time 60 --output "$TMP_DIR/$side-$case_name.body" --header 'Content-Type: application/json' \
      --cookie "$cookie" --data '{"action":"queue_cron"}' --write-out '%{http_code}' "$GO_BASE_URL/api/game/admin?session=$session&cp=$admin_planet&mode=Queue")"
    issue="$(jq -r '.actionIssue.code // empty' "$TMP_DIR/$side-$case_name.body")"
  fi
  if [ "$case_name" = planets ]; then state="$(capture_planets)"; else state="$(capture_players)"; fi
  normalized="$(jq -ncS --argjson http "$http" --arg issue "$issue" --argjson state "$state" '{http:$http,issue:$issue,state:$state}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true
for case_name in planets players; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.state')" = "$(printf '%s' "$go" | jq -cS '.state')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200 or .http==302' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200 and .issue=="action_saved"' >/dev/null || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "admin-cleanup-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" '{pass:$pass,normalization:"generated IDs and dates are excluded; cleanup outcomes, retained exceptions, auxiliary rows, localized debug text and recurring UTC schedule slots remain exact",cases:.}' "$results" > "$REPORT"
[ "$all_pass" = true ]
printf 'Go/PHP admin cleanup differential E2E: PASS (2 cases)\n'
