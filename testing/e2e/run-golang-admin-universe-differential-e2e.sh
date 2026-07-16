#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_ADMIN_UNIVERSE_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-admin-universe-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/admin-universe-differential.XXXXXX")"

admin_id="$(jq -r '.player_id // 0' "$FIXTURE")"
operator_id="$(jq -r '.operator.player_id // 0' "$FIXTURE")"
target_id="$(jq -r '.admin_operations.target_player_id // 0' "$FIXTURE")"
regular_id="$(jq -r '.message_send.sender.player_id // 0' "$FIXTURE")"
[ "$admin_id" -gt 0 ] && [ "$operator_id" -gt 0 ]
[ "$target_id" -gt 0 ] && [ "$regular_id" -gt 0 ] && [ "$target_id" -ne "$regular_id" ]

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

uni_backup="uni1_e2e_admuni_uni_$$"
users_backup="uni1_e2e_admuni_users_$$"
db_query "DROP TABLE IF EXISTS $uni_backup,$users_backup;
CREATE TABLE $uni_backup AS SELECT * FROM uni1_uni;
CREATE TABLE $users_backup AS SELECT player_id,admin,lastclick,vacation,vacation_until FROM uni1_users" >/dev/null

restore_case() {
  db_query "DELETE FROM uni1_uni; INSERT INTO uni1_uni SELECT * FROM $uni_backup;
UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id
SET u.admin=b.admin,u.lastclick=b.lastclick,u.vacation=b.vacation,u.vacation_until=b.vacation_until" >/dev/null
}

cleanup() {
  restore_case >/dev/null 2>&1 || true
  db_query "DROP TABLE IF EXISTS $uni_backup,$users_backup" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

admin_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$admin_id")"
admin_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$admin_id")"
operator_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$operator_id")"
operator_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$operator_id")"

base_settings='{"speed":9,"fleetSpeed":8,"acs":4,"fleetDebris":50,"defenseDebris":30,"defenseRepair":75,"defenseDelta":12,"galaxies":12,"systems":650,"rapidFire":true,"moons":false,"freeze":false,"language":"de","battleEngine":"/opt/battle engine","phpBattle":true,"battleMax":765432,"forceLanguage":true,"startDarkMatter":3456,"maxShipyard":12345,"feedAge":45,"extBoard":"/board?x=1&y=2","extDiscord":"/discord room","extTutorial":"/tutorial","extRules":"/rules","extImpressum":"/imprint","maxUsers":4321,"news1":"new one","news2":"new two","newsUpdateDays":0,"newsOff":false}'

configure_case() {
  case_name="$1"
  restore_case
  case_epoch="$(db_query 'SELECT UNIX_TIMESTAMP()')"
  db_query "UPDATE uni1_uni SET speed=2,fspeed=3,galaxies=9,systems=499,maxusers=777,acs=2,fid=10,did=20,rapid=0,moons=1,
defrepair=70,defrepair_delta=10,freeze=0,news1='old one',news2='old two',news_until=123456,battle_engine='/old',lang='en',
ext_board='/old-board',ext_discord='/old-discord',ext_tutorial='/old-tutorial',ext_rules='/old-rules',ext_impressum='/old-imprint',
php_battle=0,battle_max=1000000,force_lang=0,start_dm=1000,max_werf=9999,feedage=60;
UPDATE uni1_users SET admin=2 WHERE player_id=$admin_id;
UPDATE uni1_users SET admin=1,lastclick=$case_epoch,vacation=0,vacation_until=333 WHERE player_id=$operator_id;
UPDATE uni1_users SET admin=0,lastclick=$case_epoch,vacation=0,vacation_until=111 WHERE player_id=$target_id;
UPDATE uni1_users SET admin=0,lastclick=$case_epoch-691200,vacation=0,vacation_until=222 WHERE player_id=$regular_id" >/dev/null
  actor_login=$admin_login; actor_planet=$admin_planet
  settings="$base_settings"
  case "$case_name" in
    universe-full) ;;
    universe-maxusers-zero) settings="$(printf '%s' "$settings" | jq -c '.maxUsers=0')" ;;
    universe-news-update) settings="$(printf '%s' "$settings" | jq -c '.newsUpdateDays=2')" ;;
    universe-news-disable)
      db_query "UPDATE uni1_uni SET news_until=$case_epoch+86400" >/dev/null
      settings="$(printf '%s' "$settings" | jq -c '.newsOff=true')"
      ;;
    universe-news-update-disable) settings="$(printf '%s' "$settings" | jq -c '.newsUpdateDays=2|.newsOff=true')" ;;
    universe-freeze) settings="$(printf '%s' "$settings" | jq -c '.freeze=true')" ;;
    universe-unfreeze) ;;
    universe-empty-strings)
      settings="$(printf '%s' "$settings" | jq -c '.language=""|.battleEngine=""|.extBoard=""|.extDiscord=""|.extTutorial=""|.extRules=""|.extImpressum=""')"
      ;;
    universe-operator-denied) actor_login=$operator_login; actor_planet=$operator_planet ;;
  esac
  news_days="$(printf '%s' "$settings" | jq -r '.newsUpdateDays')"
}

login_legacy() {
  prefix="$1"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" --output "$TMP_DIR/$prefix-login.body" \
    --cookie-jar "$TMP_DIR/$prefix.cookies" --request POST --data-urlencode "login=$actor_login" --data-urlencode "pass=$PASSWORD" \
    --write-out '%{http_code}' "$LEGACY_BASE_URL/game/reg/login2.php")"
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/$prefix-login.headers")"
  session="$(printf '%s' "$location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 302 ] && [ -n "$session" ]
}

login_go() {
  prefix="$1"
  payload="$(jq -nc --arg login "$actor_login" --arg pass "$PASSWORD" '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" --output "$TMP_DIR/$prefix-login.body" \
    --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/$prefix-login.headers" | cut -d';' -f1 | tr -d '\r')"
  redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/$prefix-login.body")"
  session="$(printf '%s' "$redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 200 ] && [ -n "$cookie" ] && [ -n "$session" ]
}

legacy_form() {
  printf '%s' "$settings" | jq -r '
    {speed:.speed,fspeed:.fleetSpeed,acs:.acs,fid:.fleetDebris,did:.defenseDebris,defrepair:.defenseRepair,
     defrepair_delta:.defenseDelta,galaxies:.galaxies,systems:.systems,lang:.language,battle_engine:.battleEngine,
     battle_max:.battleMax,start_dm:.startDarkMatter,max_werf:.maxShipyard,feedage:.feedAge,ext_board:.extBoard,
     ext_discord:.extDiscord,ext_tutorial:.extTutorial,ext_rules:.extRules,ext_impressum:.extImpressum,maxusers:.maxUsers,
     news1:.news1,news2:.news2,news_upd:.newsUpdateDays}
    | to_entries | map((.key|@uri)+"="+(.value|tostring|@uri)) | join("&")'
}

capture_state() {
  uni="$(db_query "SELECT JSON_OBJECT('speed',speed,'fleetSpeed',fspeed,'galaxies',galaxies,'systems',systems,'maxUsers',maxusers,'acs',acs,
'fleetDebris',fid,'defenseDebris',did,'rapidFire',rapid,'moons',moons,'defenseRepair',defrepair,'defenseDelta',defrepair_delta,
'freeze',freeze,'news1',news1,'news2',news2,'newsUntil',news_until,'battleEngine',battle_engine,'language',lang,
'extBoard',ext_board,'extDiscord',ext_discord,'extTutorial',ext_tutorial,'extRules',ext_rules,'extImpressum',ext_impressum,
'phpBattle',php_battle,'battleMax',battle_max,'forceLanguage',force_lang,'startDarkMatter',start_dm,'maxShipyard',max_werf,'feedAge',feedage)
FROM uni1_uni LIMIT 1")"
  users="$(db_query "SELECT JSON_ARRAYAGG(JSON_OBJECT('id',player_id,'admin',admin,'vacation',vacation,'vacationUntil',vacation_until))
FROM (SELECT player_id,admin,vacation,vacation_until FROM uni1_users WHERE player_id IN ($admin_id,$operator_id,$target_id,$regular_id) ORDER BY player_id) x")"
  jq -ncS --argjson uni "$uni" --argjson users "$users" --argjson epoch "$case_epoch" --argjson newsDays "$news_days" '
    def normalized_time: if . >= $epoch-5 and . <= $epoch+5 then 0 else . end;
    {uni:$uni,users:($users|map(.vacationUntil|=normalized_time))}
    | if $newsDays > 0 and .uni.newsUntil >= $epoch+($newsDays*86400)-5 and .uni.newsUntil <= $epoch+($newsDays*86400)+5
      then .uni.newsUntil=($newsDays*86400) else . end'
}

run_side() {
  side="$1"; case_name="$2"; configure_case "$case_name"
  if [ "$side" = legacy ]; then
    login_legacy "$side-$case_name"
    form="$(legacy_form)"
    [ "$(printf '%s' "$settings" | jq -r '.rapidFire')" = true ] && form="$form&rapid=on"
    [ "$(printf '%s' "$settings" | jq -r '.moons')" = true ] && form="$form&moons=on"
    [ "$(printf '%s' "$settings" | jq -r '.freeze')" = true ] && form="$form&freeze=on"
    [ "$(printf '%s' "$settings" | jq -r '.phpBattle')" = true ] && form="$form&php_battle=on"
    [ "$(printf '%s' "$settings" | jq -r '.forceLanguage')" = true ] && form="$form&force_lang=on"
    [ "$(printf '%s' "$settings" | jq -r '.newsOff')" = true ] && form="$form&news_off=on"
    http="$(curl --silent --show-error --max-time 30 --output "$TMP_DIR/$side-$case_name.body" --cookie "$TMP_DIR/$side-$case_name.cookies" \
      --request POST --data "$form" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$actor_planet&mode=Uni")"
    issue=""
  else
    login_go "$side-$case_name"
    payload="$(jq -nc --argjson settings "$settings" '{action:"settings",universeSettings:$settings}')"
    http="$(curl --silent --show-error --max-time 30 --output "$TMP_DIR/$side-$case_name.body" --header 'Content-Type: application/json' \
      --cookie "$cookie" --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/game/admin?session=$session&cp=$actor_planet&mode=Uni")"
    issue="$(jq -r '.actionIssue.code // empty' "$TMP_DIR/$side-$case_name.body" 2>/dev/null || true)"
  fi
  state="$(capture_state)"
  normalized="$(jq -ncS --argjson http "$http" --arg issue "$issue" --argjson state "$state" '{http:$http,issue:$issue,state:$state}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true
cases="universe-full universe-maxusers-zero universe-news-update universe-news-disable universe-news-update-disable universe-freeze universe-unfreeze universe-empty-strings universe-operator-denied"
for case_name in $cases; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.state')" = "$(printf '%s' "$go" | jq -cS '.state')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200 or .http==302' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200' >/dev/null || pass=false
  if [ "$case_name" = universe-operator-denied ]; then
    [ "$(printf '%s' "$go" | jq -r '.issue')" = access_denied ] || pass=false
  else
    [ "$(printf '%s' "$go" | jq -r '.issue')" = action_saved ] || pass=false
  fi
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "admin-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" \
  '{pass:$pass,normalization:"request-relative news/vacation timestamps only; all universe values, strings and affected users remain exact",cases:.}' \
  "$results" > "$REPORT"
[ "$all_pass" = true ]
case_count="$(printf '%s\n' $cases | wc -l | tr -d ' ')"
printf 'Go/PHP admin universe differential E2E: PASS (%s cases)\n' "$case_count"
