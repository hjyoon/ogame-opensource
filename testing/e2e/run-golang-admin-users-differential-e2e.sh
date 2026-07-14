#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
MAILHOG_BASE_URL="${OGAME_MAILHOG_BASE_URL:-http://127.0.0.1:${OGAME_MAILHOG_PORT:-8026}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_ADMIN_USERS_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-admin-users-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/admin-users-differential.XXXXXX")"

admin_id="$(jq -r '.player_id // 0' "$FIXTURE")"
target_id="$(jq -r '.message_send.recipient.player_id // 0' "$FIXTURE")"
[ "$admin_id" -gt 0 ] && [ "$target_id" -gt 0 ] || { printf 'Invalid admin users fixture\n' >&2; exit 1; }

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

marker=$((940000000 + ($$ % 1000000)))
user_backup="uni1_e2e_admin_users_user_$$"
planet_backup="uni1_e2e_admin_users_planets_$$"
queue_backup="uni1_e2e_admin_users_queue_$$"
fleet_backup="uni1_e2e_admin_users_fleet_$$"
coltab_backup="uni1_e2e_admin_users_coltab_$$"
role_backup="uni1_e2e_admin_users_role_$$"
rank_backup="uni1_e2e_admin_users_ranks_$$"
admin_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$admin_id")"
admin_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$admin_id")"
[ "$admin_planet" -gt 0 ] && [ -n "$admin_login" ] || { printf 'Invalid admin login fixture\n' >&2; exit 1; }

db_query "DROP TABLE IF EXISTS $user_backup,$planet_backup,$queue_backup,$fleet_backup,$coltab_backup,$role_backup,$rank_backup;
CREATE TABLE $user_backup AS SELECT * FROM uni1_users WHERE player_id=$target_id;
CREATE TABLE $planet_backup AS SELECT * FROM uni1_planets WHERE owner_id=$target_id;
CREATE TABLE $queue_backup AS SELECT * FROM uni1_queue WHERE owner_id=$target_id AND type='AI';
CREATE TABLE $fleet_backup AS SELECT * FROM uni1_fleet WHERE owner_id=$target_id;
CREATE TABLE $coltab_backup AS SELECT * FROM uni1_coltab;
CREATE TABLE $role_backup AS SELECT player_id,admin FROM uni1_users WHERE player_id=$admin_id;
CREATE TABLE $rank_backup AS SELECT player_id,score1,score2,score3,place1,place2,place3 FROM uni1_users" >/dev/null

restore_case() {
  db_query "DELETE FROM uni1_queue WHERE owner_id=$target_id AND type='AI'; INSERT INTO uni1_queue SELECT * FROM $queue_backup;
DELETE FROM uni1_fleet WHERE owner_id=$target_id; INSERT INTO uni1_fleet SELECT * FROM $fleet_backup;
DELETE FROM uni1_planets WHERE owner_id=$target_id; INSERT INTO uni1_planets SELECT * FROM $planet_backup;
DELETE FROM uni1_users WHERE player_id=$target_id; INSERT INTO uni1_users SELECT * FROM $user_backup;
DELETE FROM uni1_coltab; INSERT INTO uni1_coltab SELECT * FROM $coltab_backup;
UPDATE uni1_users u JOIN $rank_backup b ON b.player_id=u.player_id SET u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3;
UPDATE uni1_users SET admin=2 WHERE player_id=$admin_id" >/dev/null
  curl --fail --silent --show-error --request DELETE "$MAILHOG_BASE_URL/api/v1/messages" >/dev/null
}

cleanup() {
  restore_case >/dev/null 2>&1 || true
  db_query "UPDATE uni1_users u JOIN $role_backup b ON b.player_id=u.player_id SET u.admin=b.admin;
DROP TABLE IF EXISTS $user_backup,$planet_backup,$queue_backup,$fleet_backup,$coltab_backup,$role_backup,$rank_backup" >/dev/null 2>&1 || true
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

configure_case() {
  case_name="$1"
  restore_case
  anchor="$(db_query 'SELECT UNIX_TIMESTAMP()')"
  email="u${marker}@e.test"
  db_query "UPDATE uni1_users SET pemail='old-$email',email='temp-$email',disable=0,disable_until=0,vacation=0,vacation_until=0,banned=1,noattack=1,
validated=0,sniff=0,debug=0,admin=0,dm=3,dmfree=4,sortby=0,sortorder=0,skin='old/',useskin=0,deact_ip=0,maxspy=1,maxfleetmsg=2,
\`106\`=1,\`108\`=2,com_until=$((anchor+100)),adm_until=$((anchor+200)),eng_until=$((anchor+300)),geo_until=$((anchor+400)),tec_until=$((anchor+500)) WHERE player_id=$target_id" >/dev/null
  case "$case_name" in
    update-full) : ;;
    update-clear) db_query "UPDATE uni1_users SET disable=1,disable_until=$((anchor+10)),vacation=1,vacation_until=$((anchor+10)) WHERE player_id=$target_id" >/dev/null ;;
    create-planet|occupied-planet)
      galaxy=8; system=$((700 + ($$ % 100))); position=8
      db_query "UPDATE uni1_coltab SET t3_a=12,t3_b=12,t3_c=1000" >/dev/null
      if [ "$case_name" = occupied-planet ]; then
        db_query "UPDATE uni1_planets SET g=$galaxy,s=$system,p=$position WHERE planet_id=(SELECT planet_id FROM $planet_backup ORDER BY planet_id LIMIT 1)" >/dev/null
      fi
      ;;
    bot-start|bot-stop)
      db_query "DELETE FROM uni1_queue WHERE owner_id=$target_id AND type='AI'" >/dev/null
      if [ "$case_name" = bot-stop ]; then
        strategy="$(db_query "SELECT id FROM uni1_botstrat WHERE name='_start' LIMIT 1")"
        block="$(db_query "SELECT CAST(JSON_UNQUOTE(JSON_EXTRACT(source,'$.nodeDataArray[0].key')) AS UNSIGNED) FROM uni1_botstrat WHERE id=$strategy")"
        db_query "INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES($target_id,'AI',$strategy,$block,0,$anchor,$anchor,1000)" >/dev/null
      fi
      ;;
    reactivate) db_query "UPDATE uni1_users SET oname='Admin Target',pemail='$email',validated=1,validatemd='old-code',password='old-password' WHERE player_id=$target_id" >/dev/null ;;
    recalc-stats) db_query "UPDATE uni1_users SET banned=0,admin=0,score1=123,score2=456,score3=789 WHERE player_id=$target_id" >/dev/null ;;
  esac
}

legacy_update() {
  checked="$1"
  checkbox_args=""
  [ "$checked" = full ] && checkbox_args="--data validated=on --data sniff=on --data debug=on --data design=on --data deact_ip=on --data deaktjava=on --data vacation=on"
  [ "$checked" = preserve ] && checkbox_args="--data banned=on --data noattack=on --data validated=on"
  # shellcheck disable=SC2086
  curl --silent --show-error --max-time 30 --output "$TMP_DIR/legacy-update.body" --cookie "$TMP_DIR/legacy.cookies" --request POST \
    --data-urlencode "pemail=$email" --data-urlencode "email=new-$email" --data-urlencode 'dpath=skin/new/' \
    --data admin=1 --data dm=50 --data dmfree=60 --data settings_sort=2 --data settings_order=1 --data spio_anz=7 --data settings_fleetactions=8 \
    --data r106=-7 --data r108=150 --data r109=3 --data r110=4 --data r111=5 --data r113=6 --data r114=7 --data r115=8 \
    --data r117=9 --data r118=10 --data r120=11 --data r121=12 --data r122=13 --data r123=14 --data r124=15 --data r199=16 \
    --data pr_1=2 --data pr_2=-1 $checkbox_args --write-out '%{http_code}' \
    "$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$admin_planet&mode=Users&action=update&player_id=$target_id"
}

go_update_payload() {
  full=false; preserve=false
  [ "$1" = full ] && full=true
  [ "$1" = preserve ] && preserve=true
  jq -nc --arg email "$email" --argjson full "$full" --argjson preserve "$preserve" --argjson target "$target_id" '{
    action:"update",targetIds:[$target],userSettings:{permanentEmail:$email,email:("new-"+$email),skin:"skin/new/",disable:$full,vacation:$full,
    banned:$preserve,noAttack:$preserve,validated:true,sniff:$full,debug:$full,useSkin:$full,deactivateIP:$full,adminLevel:1,darkMatter:50,
    darkMatterFree:60,sortBy:2,sortOrder:1,maxSpy:7,maxFleetMsg:8,
    research:{"106":-7,"108":150,"109":3,"110":4,"111":5,"113":6,"114":7,"115":8,"117":9,"118":10,"120":11,"121":12,"122":13,"123":14,"124":15,"199":16},
    officerDays:{"1":2,"2":-1}}}'
}

capture_user() {
  db_query "SELECT JSON_OBJECT('pemail',pemail,'email',email,'disable',disable,'disable_window',disable=0 OR disable_until BETWEEN UNIX_TIMESTAMP()+604780 AND UNIX_TIMESTAMP()+604820,
'vacation',vacation,'vacation_window',vacation=0 OR vacation_until BETWEEN UNIX_TIMESTAMP()+172780 AND UNIX_TIMESTAMP()+172820,'banned',banned,'noattack',noattack,
'validated',validated,'sniff',sniff,'debug',debug,'admin',admin,'dm',dm,'dmfree',dmfree,'sortby',sortby,'sortorder',sortorder,'skin',skin,'useskin',useskin,
'deact_ip',deact_ip,'maxspy',maxspy,'maxfleetmsg',maxfleetmsg,'r106',\`106\`,'r108',\`108\`,'r199',\`199\`,'commander',com_until-$anchor,'admiral',adm_until) FROM uni1_users WHERE player_id=$target_id"
}

capture_planet() {
  db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('name',name,'type',type,'g',g,'s',s,'p',p,'owner',owner_id,'diameter',diameter,
'temperature_range',temp BETWEEN -6 AND 3,'fields',fields,'maxfields',maxfields,'metal',\`700\`,'crystal',\`701\`,'deuterium',\`702\`,
'prod1',prod1,'prod2',prod2,'prod3',prod3))) FROM uni1_planets WHERE owner_id=$target_id AND g=$galaxy AND s=$system AND p=$position"
}

capture_bot() {
  db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('type',type,'sub_id',sub_id,'obj_id',obj_id,'level',level,'prio',prio,
'start_window',start BETWEEN $((anchor-2)) AND UNIX_TIMESTAMP()+2,'legacy_end',end=start*2))) FROM uni1_queue WHERE owner_id=$target_id AND type='AI'"
}

capture_reactivation() {
  db_query "SELECT JSON_OBJECT('validated',validated,'activation_format',validatemd REGEXP '^[0-9a-f]{32}$','password_format',password REGEXP '^[0-9a-f]{32}$','activation_changed',validatemd<>'old-code','password_changed',password<>'old-password') FROM uni1_users WHERE player_id=$target_id"
}

capture_reactivation_mail() {
  attempt=0
  mail_json="$(curl --fail --silent --show-error "$MAILHOG_BASE_URL/api/v2/messages")"
  while [ "$(printf '%s' "$mail_json" | jq -r '.total // 0')" -eq 0 ] && [ "$attempt" -lt 20 ]; do
    sleep 0.1
    mail_json="$(curl --fail --silent --show-error "$MAILHOG_BASE_URL/api/v2/messages")"
    attempt=$((attempt+1))
  done
  printf '%s' "$mail_json" | jq -cS --arg recipient "$email" '
    def decoded_subject:
      if test("^=\\?UTF-8\\?B\\?.*\\?=$"; "i") then capture("^=\\?UTF-8\\?B\\?(?<encoded>.*)\\?=$"; "i").encoded | @base64d else . end
      | gsub("[[:space:]]+$"; "");
    {messages:[.items[] | select(any(.To[]; (.Mailbox+"@"+.Domain)==$recipient)) | {
      recipient:$recipient,
      subject:((.Content.Headers.Subject[0]//"")|decoded_subject),
      body:((.Content.Body//"")
        | gsub("\\r";"")
        | gsub("https?://[^/]+/";"<ORIGIN>/")
        | gsub("ack=[0-9a-f]{32}";"ack=<ACK>")
        | gsub("Password: [a-z]{8}";"Password: <PASSWORD>")
        | split("\\n") | map(select(test("^[[:space:]]*$")|not)) | join("\\n"))
    }]} | .count=(.messages|length) | {count,messages}'
}

capture_recalc() {
  db_query "SELECT JSON_OBJECT('score1',score1,'score2',score2,'score3',score3,'place1',place1,'place2',place2,'place3',place3) FROM uni1_users WHERE player_id=$target_id"
}

run_side() {
  side="$1"; case_name="$2"
  configure_case "$case_name"
  issue=""
  if [ "$side" = legacy ]; then login_legacy; else login_go; fi
  case "$case_name" in
    update-full|update-clear)
      mode=full; [ "$case_name" = update-clear ] && mode=preserve
      if [ "$side" = legacy ]; then
        http="$(legacy_update "$mode")"
      else
        payload="$(go_update_payload "$mode")"
        http="$(curl --silent --show-error --max-time 30 --output "$TMP_DIR/go-$case_name.body" --cookie "$cookie" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/game/admin?session=$session&cp=$admin_planet&mode=Users&player_id=$target_id")"
        issue="$(jq -r '.actionIssue.code // empty' "$TMP_DIR/go-$case_name.body")"
      fi
      state="$(capture_user)"
      ;;
    create-planet|occupied-planet)
      if [ "$side" = legacy ]; then
        http="$(curl --silent --show-error --max-time 30 --output "$TMP_DIR/legacy-$case_name.body" --cookie "$TMP_DIR/legacy.cookies" --request POST --data "g=$galaxy&s=$system&p=$position" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$admin_planet&mode=Users&action=create_planet&player_id=$target_id")"
      else
        payload="$(jq -nc --argjson target "$target_id" --argjson g "$galaxy" --argjson s "$system" --argjson p "$position" '{action:"create_planet",targetIds:[$target],values:{g:$g,s:$s,p:$p}}')"
        http="$(curl --silent --show-error --max-time 30 --output "$TMP_DIR/go-$case_name.body" --cookie "$cookie" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/game/admin?session=$session&cp=$admin_planet&mode=Users&player_id=$target_id")"
        issue="$(jq -r '.actionIssue.code // empty' "$TMP_DIR/go-$case_name.body")"
      fi
      state="$(capture_planet)"
      ;;
    bot-start|bot-stop|reactivate|recalc-stats)
      action="$(printf '%s' "$case_name" | tr '-' '_')"
      if [ "$side" = legacy ]; then
        http="$(curl --silent --show-error --max-time 30 --output "$TMP_DIR/legacy-$case_name.body" --cookie "$TMP_DIR/legacy.cookies" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$admin_planet&mode=Users&action=$action&player_id=$target_id")"
      else
        payload="$(jq -nc --arg action "$action" --argjson target "$target_id" '{action:$action,targetIds:[$target]}')"
        http="$(curl --silent --show-error --max-time 30 --output "$TMP_DIR/go-$case_name.body" --cookie "$cookie" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/game/admin?session=$session&cp=$admin_planet&mode=Users&player_id=$target_id")"
        issue="$(jq -r '.actionIssue.code // empty' "$TMP_DIR/go-$case_name.body")"
      fi
      if [ "$case_name" = reactivate ]; then
        user_state="$(capture_reactivation)"
        mail_state="$(capture_reactivation_mail)"
        state="$(jq -ncS --argjson user "$user_state" --argjson mail "$mail_state" '{user:$user,mail:$mail}')"
      elif [ "$case_name" = recalc-stats ]; then
        state="$(capture_recalc)"
      else
        state="$(capture_bot)"
      fi
      ;;
  esac
  normalized="$(jq -ncS --argjson http "$http" --arg issue "$issue" --argjson state "$state" '{http:$http,issue:$issue,state:$state}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true
for case_name in update-full update-clear create-planet occupied-planet bot-start bot-stop reactivate recalc-stats; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.state')" = "$(printf '%s' "$go" | jq -cS '.state')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200 or .http==302' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200 and .issue=="action_saved"' >/dev/null || pass=false
  if [ "$case_name" = update-full ]; then
    printf '%s' "$legacy" | jq -e '.state.disable_window and .state.vacation_window' >/dev/null || pass=false
    printf '%s' "$go" | jq -e '.state.disable_window and .state.vacation_window' >/dev/null || pass=false
  fi
  if [ "$case_name" = bot-start ]; then
    printf '%s' "$legacy" | jq -e '.state[0].start_window and .state[0].legacy_end' >/dev/null || pass=false
    printf '%s' "$go" | jq -e '.state[0].start_window and .state[0].legacy_end' >/dev/null || pass=false
  fi
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "admin-users-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" '{pass:$pass,normalization:"generated planet IDs, temperature RNG, password/activation bytes, public origins and request-second timer drift are reduced to legacy invariants",cases:.}' "$results" > "$REPORT"
[ "$all_pass" = true ]
printf 'Go/PHP admin users differential E2E: PASS (8 cases)\n'
