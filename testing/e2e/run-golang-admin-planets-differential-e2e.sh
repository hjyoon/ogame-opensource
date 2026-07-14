#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_ADMIN_PLANETS_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-admin-planets-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/admin-planets-differential.XXXXXX")"

admin_id="$(jq -r '.player_id // 0' "$FIXTURE")"
target_id="$(jq -r '.message_send.recipient.player_id // 0' "$FIXTURE")"
[ "$admin_id" -gt 0 ] && [ "$target_id" -gt 0 ] || { printf 'Invalid admin planets fixture\n' >&2; exit 1; }

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

read_action_issue() {
  body="$1"
  if ! jq -e . "$body" >/dev/null 2>&1; then
    printf 'Invalid Go JSON response for %s: ' "$case_name" >&2
    tr '\n' ' ' < "$body" | cut -c1-500 >&2
    printf '\n' >&2
    exit 1
  fi
  jq -r '.actionIssue.code // empty' "$body"
}

normalize_result() {
  if ! printf '%s' "$state" | jq -e . >/dev/null 2>&1; then
    printf 'Invalid database state JSON for %s/%s: <%s>\n' "$side" "$case_name" "$state" >&2
    exit 1
  fi
  case "$http" in
    ''|*[!0-9]*) printf 'Invalid HTTP status for %s/%s: <%s>\n' "$side" "$case_name" "$http" >&2; exit 1 ;;
  esac
  jq -ncS --argjson http "$http" --arg issue "$issue" --argjson state "$state" '{http:$http,issue:$issue,state:$state}'
}

planet_backup="uni1_e2e_admin_planets_p_$$"
build_backup="uni1_e2e_admin_planets_b_$$"
queue_backup="uni1_e2e_admin_planets_q_$$"
user_backup="uni1_e2e_admin_planets_u_$$"
coltab_backup="uni1_e2e_admin_planets_c_$$"
admin_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$admin_id")"
target_home="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$target_id")"
admin_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$admin_id")"
[ "$admin_planet" -gt 0 ] && [ "$target_home" -gt 0 ] && [ -n "$admin_login" ] || { printf 'Invalid admin planet login fixture\n' >&2; exit 1; }

db_query "DROP TABLE IF EXISTS $planet_backup,$build_backup,$queue_backup,$user_backup,$coltab_backup;
CREATE TABLE $planet_backup AS SELECT * FROM uni1_planets WHERE owner_id=$target_id;
CREATE TABLE $build_backup AS SELECT * FROM uni1_buildqueue WHERE owner_id=$target_id;
CREATE TABLE $queue_backup AS SELECT * FROM uni1_queue WHERE owner_id=$target_id;
CREATE TABLE $user_backup AS SELECT * FROM uni1_users WHERE player_id IN ($admin_id,$target_id);
CREATE TABLE $coltab_backup AS SELECT * FROM uni1_coltab" >/dev/null

restore_case() {
  db_query "DELETE FROM uni1_queue WHERE owner_id=$target_id; INSERT INTO uni1_queue SELECT * FROM $queue_backup;
DELETE FROM uni1_buildqueue WHERE owner_id=$target_id; INSERT INTO uni1_buildqueue SELECT * FROM $build_backup;
DELETE FROM uni1_planets WHERE owner_id=$target_id; INSERT INTO uni1_planets SELECT * FROM $planet_backup;
DELETE FROM uni1_users WHERE player_id IN ($admin_id,$target_id); INSERT INTO uni1_users SELECT * FROM $user_backup;
DELETE FROM uni1_coltab; INSERT INTO uni1_coltab SELECT * FROM $coltab_backup;
UPDATE uni1_users SET admin=2 WHERE player_id=$admin_id" >/dev/null
}

cleanup() {
  restore_case >/dev/null 2>&1 || true
  db_query "DROP TABLE IF EXISTS $planet_backup,$build_backup,$queue_backup,$user_backup,$coltab_backup" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

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

insert_planet() {
  db_query "INSERT INTO uni1_planets(name,type,g,s,p,owner_id,diameter,temp,fields,maxfields,date,\`700\`,\`701\`,\`702\`,prod1,prod2,prod3,prod4,prod12,prod212,lastpeek,lastakt,gate_until,remove)
VALUES('E2EAdminPlanet',1,8,$system,8,$target_id,15000,50,0,225,$anchor,10,20,30,1,1,1,1,1,1,$anchor,$anchor,0,0)" >/dev/null
  planet_id="$(db_query "SELECT planet_id FROM uni1_planets WHERE owner_id=$target_id AND name='E2EAdminPlanet' AND s=$system LIMIT 1")"
}

insert_moon() {
  db_query "INSERT INTO uni1_planets(name,type,g,s,p,owner_id,diameter,temp,fields,maxfields,date,\`700\`,\`701\`,\`702\`,lastpeek,lastakt,gate_until,remove)
VALUES('E2EAdminMoon',0,8,$system,8,$target_id,8500,25,0,1,$anchor,0,0,0,$anchor,$anchor,$gate_until,0)" >/dev/null
  moon_id="$(db_query "SELECT planet_id FROM uni1_planets WHERE owner_id=$target_id AND name='E2EAdminMoon' AND s=$system LIMIT 1")"
}

configure_case() {
  case_name="$1"
  restore_case
  anchor="$(db_query 'SELECT UNIX_TIMESTAMP()')"
  system=$((300 + ($$ % 100)))
  gate_until=0
  db_query "UPDATE uni1_coltab SET t3_a=12,t3_b=12,t3_c=1000; UPDATE uni1_users SET lang='en' WHERE player_id IN ($admin_id,$target_id)" >/dev/null
  case "$case_name" in
    delete-home) planet_id="$target_home" ;;
    *) insert_planet ;;
  esac
  case "$case_name" in
    update-full) insert_moon ;;
    delete-colony)
      db_query "INSERT INTO uni1_buildqueue(owner_id,planet_id,list_id,tech_id,level,start,end) VALUES($target_id,$planet_id,1,1,2,$anchor,$((anchor+100)));
SET @bid=LAST_INSERT_ID(); INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES($target_id,'Build',@bid,1,2,$anchor,$((anchor+100)),600),($target_id,'Shipyard',$planet_id,0,0,$anchor,$((anchor+100)),600)" >/dev/null
      ;;
    create-moon-existing) insert_moon ;;
    gate-warm) insert_moon; planet_id="$moon_id" ;;
    gate-cool) gate_until=$((anchor+1200)); insert_moon; planet_id="$moon_id" ;;
    recalc-moon) insert_moon; planet_id="$moon_id"; db_query "UPDATE uni1_planets SET \`14\`=2,\`41\`=3,fields=0,maxfields=0 WHERE planet_id=$planet_id" >/dev/null ;;
    recalc-planet) db_query "UPDATE uni1_planets SET \`1\`=9,\`33\`=2,fields=0,maxfields=0 WHERE planet_id=$planet_id" >/dev/null ;;
  esac
}

legacy_update() {
  curl --silent --show-error --max-time 30 --output "$TMP_DIR/legacy-update.body" --cookie "$TMP_DIR/legacy.cookies" --request POST \
    --data g=-2 --data s=33 --data p=-8 --data diameter=-15000 --data type=-1 --data temp=-90 \
    --data 700=-100 --data 701=200 --data 702=-300 --data 1=120 --data 33=-2 --data 202=-4 --data 401=-5 \
    --data prod1=0.5 --data prod212=0.7 --write-out '%{http_code}' \
    "$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$planet_id&mode=Planets&action=update"
}

go_update_payload() {
  jq -nc '{action:"update",planetSettings:{coordinates:{galaxy:-2,system:33,position:-8},diameter:-15000,type:-1,temperature:-90,
resources:{"700":-100,"701":200,"702":-300},buildings:{"1":120,"33":-2},fleet:{"202":-4},defense:{"401":-5},production:{"1":0.5,"212":0.7},delete:false}}'
}

capture_update() {
  db_query "SELECT JSON_OBJECT('planet',JSON_OBJECT('type',type,'g',g,'s',s,'p',p,'diameter',diameter,'temp',temp,'fields',fields,'maxfields',maxfields,
'metal',\`700\`,'crystal',\`701\`,'deuterium',\`702\`,'b1',\`1\`,'b33',\`33\`,'f202',\`202\`,'d401',\`401\`,'prod1',prod1,'prod212',prod212),
'moon',(SELECT JSON_OBJECT('g',g,'s',s,'p',p) FROM uni1_planets WHERE name='E2EAdminMoon' AND owner_id=$target_id LIMIT 1)) FROM uni1_planets WHERE planet_id=$planet_id"
}

capture_delete() {
  db_query "SELECT JSON_OBJECT('planet_count',(SELECT COUNT(*) FROM uni1_planets WHERE planet_id=$planet_id),'build_count',(SELECT COUNT(*) FROM uni1_buildqueue WHERE planet_id=$planet_id),
'queue_count',(SELECT COUNT(*) FROM uni1_queue WHERE owner_id=$target_id AND (sub_id=$planet_id OR sub_id IN (SELECT id FROM uni1_buildqueue WHERE planet_id=$planet_id))))"
}

capture_moon() {
  db_query "SELECT JSON_OBJECT('count',COUNT(*),'valid',COALESCE(MIN(type=0 AND name='Moon' AND g=8 AND s=$system AND p=8 AND owner_id=$target_id AND diameter BETWEEN 8366 AND 8944 AND temp BETWEEN 20 AND 30 AND fields=0 AND maxfields=1 AND \`700\`=0 AND \`701\`=0 AND \`702\`=0),0)) FROM uni1_planets WHERE owner_id=$target_id AND g=8 AND s=$system AND p=8 AND type IN (0,10003)"
}

capture_debris() {
  db_query "SELECT JSON_OBJECT('count',COUNT(*),'valid',COALESCE(MIN(type=10000 AND name='Debris Field' AND owner_id=$target_id AND diameter=0 AND temp=0 AND fields=0 AND maxfields=0 AND \`700\`=0 AND \`701\`=0 AND \`702\`=0),0)) FROM uni1_planets WHERE owner_id=$target_id AND g=8 AND s=$system AND p=8 AND type=10000"
}

capture_gate() {
  if [ "$case_name" = gate-warm ]; then
    db_query "SELECT JSON_OBJECT('warm',gate_until BETWEEN UNIX_TIMESTAMP()+3596 AND UNIX_TIMESTAMP()+3602) FROM uni1_planets WHERE planet_id=$planet_id"
  else
    db_query "SELECT JSON_OBJECT('gate_until',gate_until) FROM uni1_planets WHERE planet_id=$planet_id"
  fi
}

capture_fields() {
  db_query "SELECT JSON_OBJECT('fields',fields,'maxfields',maxfields) FROM uni1_planets WHERE planet_id=$planet_id"
}

capture_diameter() {
  db_query "SELECT JSON_OBJECT('diameter',diameter) FROM uni1_planets WHERE planet_id=$planet_id"
}

run_side() {
  side="$1"; case_name="$2"
  configure_case "$case_name"
  issue=""
  if [ "$side" = legacy ]; then login_legacy; else login_go; fi
  case "$case_name" in
    update-full)
      if [ "$side" = legacy ]; then http="$(legacy_update)"; else
        http="$(curl --silent --show-error --max-time 30 --output "$TMP_DIR/go-update.body" --cookie "$cookie" --header 'Content-Type: application/json' --data "$(go_update_payload)" --write-out '%{http_code}' "$GO_BASE_URL/api/game/admin?session=$session&cp=$planet_id&mode=Planets")"
        issue="$(read_action_issue "$TMP_DIR/go-update.body")"
      fi
      state="$(capture_update)"
      ;;
    delete-home|delete-colony)
      if [ "$side" = legacy ]; then
        http="$(curl --silent --show-error --max-time 30 --output "$TMP_DIR/legacy-$case_name.body" --cookie "$TMP_DIR/legacy.cookies" --request POST --data delete_planet=Remove --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$planet_id&mode=Planets&action=update")"
      else
        payload='{"action":"update","planetSettings":{"delete":true}}'
        http="$(curl --silent --show-error --max-time 30 --output "$TMP_DIR/go-$case_name.body" --cookie "$cookie" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/game/admin?session=$session&cp=$planet_id&mode=Planets")"
        issue="$(read_action_issue "$TMP_DIR/go-$case_name.body")"
      fi
      state="$(capture_delete)"
      ;;
    create-moon|create-moon-existing|create-debris|gate-warm|gate-cool|recalc-planet|recalc-moon|random-diameter)
      case "$case_name" in
        create-moon|create-moon-existing) action=create_moon; capture=capture_moon ;;
        create-debris) action=create_debris; capture=capture_debris ;;
        gate-warm) action=warmup_gates; capture=capture_gate ;;
        gate-cool) action=cooldown_gates; capture=capture_gate ;;
        recalc-planet|recalc-moon) action=recalc_fields; capture=capture_fields ;;
        random-diameter) action=random_diam; capture=capture_diameter ;;
      esac
      if [ "$side" = legacy ]; then
        http="$(curl --silent --show-error --max-time 30 --output "$TMP_DIR/legacy-$case_name.body" --cookie "$TMP_DIR/legacy.cookies" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$planet_id&mode=Planets&action=$action")"
      else
        http="$(curl --silent --show-error --max-time 30 --output "$TMP_DIR/go-$case_name.body" --cookie "$cookie" --header 'Content-Type: application/json' --data "{\"action\":\"$action\"}" --write-out '%{http_code}' "$GO_BASE_URL/api/game/admin?session=$session&cp=$planet_id&mode=Planets")"
        issue="$(read_action_issue "$TMP_DIR/go-$case_name.body")"
      fi
      state="$($capture)"
      ;;
    search-player|search-planet)
      if [ "$case_name" = search-player ]; then search_type=playername; search_text="$(db_query "SELECT oname FROM uni1_users WHERE player_id=$target_id")"; else search_type=planetname; search_text=E2EAdmin; fi
      if [ "$side" = legacy ]; then
        http="$(curl --silent --show-error --max-time 30 --output "$TMP_DIR/legacy-$case_name.body" --cookie "$TMP_DIR/legacy.cookies" --request POST --data-urlencode "type=$search_type" --data-urlencode "searchtext=$search_text" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&mode=Planets&action=search")"
        found=false; grep -q 'E2EAdminPlanet' "$TMP_DIR/legacy-$case_name.body" && found=true
      else
        payload="$(jq -nc --arg type "$search_type" --arg text "$search_text" '{action:"search",planetSearch:{type:$type,text:$text}}')"
        http="$(curl --silent --show-error --max-time 30 --output "$TMP_DIR/go-$case_name.body" --cookie "$cookie" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/game/admin?session=$session&mode=Planets")"
        issue="$(read_action_issue "$TMP_DIR/go-$case_name.body")"
        found="$(jq -r 'any(.admin.planetSearchRows[]?; .name=="E2EAdminPlanet")' "$TMP_DIR/go-$case_name.body")"
      fi
      state="$(jq -nc --argjson found "$found" '{found:$found}')"
      ;;
  esac
  normalized="$(normalize_result)"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true
for case_name in update-full delete-home delete-colony create-moon create-moon-existing create-debris gate-warm gate-cool recalc-planet recalc-moon random-diameter search-player search-planet; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.state')" = "$(printf '%s' "$go" | jq -cS '.state')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200 or .http==302' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200 and .issue=="action_saved"' >/dev/null || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "admin-planets-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" '{pass:$pass,normalization:"moon RNG and request-second gate drift are reduced to legacy invariants; all deterministic database state is exact",cases:.}' "$results" > "$REPORT"
[ "$all_pass" = true ]
printf 'Go/PHP admin planets differential E2E: PASS (13 cases)\n'
