#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
MAILHOG_BASE_URL="${OGAME_MAILHOG_BASE_URL:-http://127.0.0.1:${OGAME_MAILHOG_PORT:-8026}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_OPTIONS_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-options-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/options-differential.XXXXXX")"

actor_id="$(jq -r '.message_send.sender.player_id // 0' "$FIXTURE")"
actor_login="$(jq -r '.message_send.sender.login // empty' "$FIXTURE")"
actor_planet="$(jq -r '.message_send.sender.home_planet_id // 0' "$FIXTURE")"
other_id="$(jq -r '.message_send.recipient.player_id // 0' "$FIXTURE")"
[ "$actor_id" -gt 0 ] && [ "$other_id" -gt 0 ] && [ "$actor_planet" -gt 0 ] && [ -n "$actor_login" ]

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

clear_mailhog() {
  curl --fail --silent --show-error --request DELETE "$MAILHOG_BASE_URL/api/v1/messages" >/dev/null
}

capture_mail() {
  mail_json="$(curl --fail --silent --show-error "$MAILHOG_BASE_URL/api/v2/messages")"
  if [ "$expects_mail" = 1 ]; then
    attempt=0
    while [ "$(printf '%s' "$mail_json" | jq -r '.total // 0')" -eq 0 ] && [ "$attempt" -lt 20 ]; do
      sleep 0.1
      mail_json="$(curl --fail --silent --show-error "$MAILHOG_BASE_URL/api/v2/messages")"
      attempt=$((attempt + 1))
    done
  fi
  mail="$(printf '%s' "$mail_json" | jq -cS '
    def decoded_subject:
      if test("^=\\?UTF-8\\?B\\?.*\\?=$"; "i") then
        capture("^=\\?UTF-8\\?B\\?(?<encoded>.*)\\?=$"; "i").encoded | @base64d
      else . end | gsub("[[:space:]]+$"; "");
    def normalized_body:
      gsub("\\r"; "") |
      gsub("https?://[^[:space:]]+/game/validate\\.php\\?ack=[^[:space:]]+"; "{activation-link}") |
      gsub("[[:space:]]+$"; "");
    {count:(.total // 0), messages:([.items[] | {
      recipients:([.To[] | (.Mailbox + "@" + .Domain)] | sort),
      subject:((.Content.Headers.Subject[0] // "") | decoded_subject),
      body:((.Content.Body // "") | normalized_body)
    }] | sort_by(.recipients[0]))}' )"
}

users_backup="uni1_e2e_optdiff_users_$$"
queue_backup="uni1_e2e_optdiff_queue_$$"
planets_backup="uni1_e2e_optdiff_planets_$$"
uni_backup="uni1_e2e_optdiff_uni_$$"
db_query "DROP TABLE IF EXISTS $users_backup,$queue_backup,$planets_backup,$uni_backup;
CREATE TABLE $users_backup AS SELECT * FROM uni1_users WHERE player_id IN ($actor_id,$other_id);
CREATE TABLE $queue_backup AS SELECT * FROM uni1_queue WHERE owner_id=$actor_id;
CREATE TABLE $planets_backup AS SELECT * FROM uni1_planets WHERE owner_id=$actor_id;
CREATE TABLE $uni_backup AS SELECT * FROM uni1_uni" >/dev/null

restore_users() {
  db_query "UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id SET
u.regdate=b.regdate,u.ally_id=b.ally_id,u.joindate=b.joindate,u.allyrank=b.allyrank,u.session=b.session,u.private_session=b.private_session,
u.name=b.name,u.oname=b.oname,u.name_changed=b.name_changed,u.name_until=b.name_until,u.password=b.password,u.temp_pass=b.temp_pass,
u.pemail=b.pemail,u.email=b.email,u.validated=b.validated,u.validatemd=b.validatemd,u.admin=b.admin,u.lang=b.lang,u.skin=b.skin,
u.useskin=b.useskin,u.vacation=b.vacation,u.vacation_until=b.vacation_until,u.disable=b.disable,u.disable_until=b.disable_until,
u.deact_ip=b.deact_ip,u.sortby=b.sortby,u.sortorder=b.sortorder,u.maxspy=b.maxspy,u.maxfleetmsg=b.maxfleetmsg,u.flags=b.flags,
u.feedid=b.feedid,u.lastfeed=b.lastfeed,u.com_until=b.com_until" >/dev/null
}

restore_original() {
  db_query "DELETE FROM uni1_queue WHERE owner_id=$actor_id; INSERT INTO uni1_queue SELECT * FROM $queue_backup" >/dev/null
  restore_users
  db_query "UPDATE uni1_planets p JOIN $planets_backup b ON b.planet_id=p.planet_id SET
p.prod1=b.prod1,p.prod2=b.prod2,p.prod3=b.prod3,p.prod4=b.prod4,p.prod12=b.prod12,p.prod212=b.prod212;
UPDATE uni1_uni u JOIN $uni_backup b ON 1=1 SET u.lang=b.lang,u.force_lang=b.force_lang,u.feedage=b.feedage,u.speed=b.speed;
DROP TABLE IF EXISTS $users_backup,$queue_backup,$planets_backup,$uni_backup" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

actor_name="$(db_query "SELECT oname FROM $users_backup WHERE player_id=$actor_id")"
actor_email="$(db_query "SELECT pemail FROM $users_backup WHERE player_id=$actor_id")"
other_name="$(db_query "SELECT oname FROM $users_backup WHERE player_id=$other_id")"
other_email="$(db_query "SELECT pemail FROM $users_backup WHERE player_id=$other_id")"

reset_case() {
  restore_users
  db_query "DELETE FROM uni1_queue WHERE owner_id=$actor_id;
UPDATE uni1_users SET name=LOWER('$actor_login'),oname='$actor_name',name_changed=0,name_until=0,session='',private_session='',
email=pemail,validated=1,validatemd='',admin=0,lang='en',skin='/evolution/',useskin=1,deact_ip=1,sortby=0,sortorder=0,
maxspy=5,maxfleetmsg=3,flags=0,feedid='',lastfeed=0,com_until=0,vacation=0,vacation_until=0,disable=0,disable_until=0
WHERE player_id=$actor_id;
UPDATE uni1_planets SET prod1=10,prod2=20,prod3=30,prod4=40,prod12=50,prod212=60 WHERE owner_id=$actor_id;
UPDATE uni1_uni SET lang='en',force_lang=0,feedage=60,speed=128" >/dev/null
}

urlencode() { jq -rn --arg value "$1" '$value|@uri'; }
append_form() { form="$form&$1=$(urlencode "$2")"; }

configure_case() {
  case_name="$1"; reset_case
  name_value="$actor_name"; language=en; skin_path=/evolution/; use_skin=1; deactivate_ip=1
  sort_by=0; sort_order=0; max_spy=5; max_fleet=3; old_password=""; new_password=""; repeat_password=""
  email_value="$actor_email"; vacation=0; disable_vacation=0; delete_account=0
  espionage=0; write_message=0; buddy=0; missile=0; report=0; no_folders=0
  feed_enabled=0; feed_type=""; hide_email=0; resend_activation=0
  expects_mail=0
  case "$case_name" in
    settings) language=fr; skin_path=/download/use/lego/; use_skin=0; deactivate_ip=0; sort_by=2; sort_order=1; max_spy=9; max_fleet=11 ;;
    clamp) language=english; sort_by=9999; sort_order=-9999; max_spy=-42; max_fleet=99999 ;;
    forced-language) db_query "UPDATE uni1_uni SET lang='de',force_lang=1" >/dev/null; language=fr ;;
    password-mismatch) old_password=$PASSWORD; new_password=newpass123; repeat_password=different ;;
    password-special) old_password=$PASSWORD; new_password='newpass!!'; repeat_password='newpass!!' ;;
    password-short) old_password=$PASSWORD; new_password=short1; repeat_password=short1 ;;
    password-wrong) old_password=wrongpass; new_password=newpass123; repeat_password=newpass123 ;;
    password-success) old_password=$PASSWORD; new_password=newpass123; repeat_password=newpass123 ;;
    email-password-precedence) email_value=bad-email; old_password=wrongpass ;;
    email-invalid) email_value=bad-email; old_password=$PASSWORD ;;
    email-used) email_value="$other_email"; old_password=$PASSWORD ;;
    email-success) email_value=options-new@example.local; old_password=$PASSWORD; expects_mail=1 ;;
    unvalidated-noop) db_query "UPDATE uni1_users SET validated=0,email='pending-options@example.local',validatemd='seed-code' WHERE player_id=$actor_id" >/dev/null; email_value=pending-options@example.local; language=fr ;;
    unvalidated-change) db_query "UPDATE uni1_users SET validated=0,email='pending-options@example.local',validatemd='seed-code' WHERE player_id=$actor_id" >/dev/null; email_value=pending-new@example.local; old_password=$PASSWORD; language=fr; expects_mail=1 ;;
    activation-resend) db_query "UPDATE uni1_users SET validated=0,email='pending-options@example.local',validatemd='seed-code' WHERE player_id=$actor_id" >/dev/null; email_value=pending-options@example.local; resend_activation=1; language=fr; expects_mail=1 ;;
    deletion-queue) delete_account=1 ;;
    deletion-cancel) db_query "UPDATE uni1_users SET disable=1,disable_until=UNIX_TIMESTAMP()+600 WHERE player_id=$actor_id" >/dev/null ;;
    vacation-enable) vacation=1; language=fr; sort_by=2 ;;
    vacation-blocked) db_query "INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES($actor_id,'Build',$actor_planet,1,1,UNIX_TIMESTAMP(),UNIX_TIMESTAMP()+1000,10)" >/dev/null; vacation=1; language=fr ;;
    vacation-locked) db_query "UPDATE uni1_users SET vacation=1,vacation_until=UNIX_TIMESTAMP()+3600 WHERE player_id=$actor_id" >/dev/null; disable_vacation=1 ;;
    vacation-delete-ignored) db_query "UPDATE uni1_users SET vacation=1,vacation_until=UNIX_TIMESTAMP()+3600 WHERE player_id=$actor_id" >/dev/null; delete_account=1 ;;
    vacation-disable) db_query "UPDATE uni1_users SET vacation=1,vacation_until=UNIX_TIMESTAMP()-1 WHERE player_id=$actor_id" >/dev/null; disable_vacation=1; language=fr ;;
    name-success) name_value=OptionsPilot ;;
    name-duplicate) name_value="$other_name" ;;
    name-cooldown) db_query "INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES($actor_id,'AllowName',0,0,0,1700000000,3400604800,0)" >/dev/null; name_value=CooldownPilot ;;
    name-length) name_value=ab ;;
    name-special) name_value=Bad.Pilot ;;
    name-forbidden) name_value=AdminPilot ;;
    commander-on) db_query "UPDATE uni1_users SET com_until=UNIX_TIMESTAMP()+3600 WHERE player_id=$actor_id" >/dev/null; espionage=1; write_message=1; buddy=1; missile=1; report=1; no_folders=1 ;;
    commander-off) db_query "UPDATE uni1_users SET com_until=UNIX_TIMESTAMP()+3600,flags=63 WHERE player_id=$actor_id" >/dev/null ;;
    feed-enable) db_query "UPDATE uni1_users SET com_until=UNIX_TIMESTAMP()+3600 WHERE player_id=$actor_id" >/dev/null; feed_enabled=1 ;;
    feed-format) db_query "UPDATE uni1_users SET com_until=UNIX_TIMESTAMP()+3600,flags=32768,feedid='00112233445566778899aabbccddeeff',lastfeed=77 WHERE player_id=$actor_id" >/dev/null; feed_enabled=1; feed_type=atom ;;
    feed-disable) db_query "UPDATE uni1_users SET com_until=UNIX_TIMESTAMP()+3600,flags=98304,feedid='00112233445566778899aabbccddeeff',lastfeed=77 WHERE player_id=$actor_id" >/dev/null ;;
    feed-prohibited) db_query "UPDATE uni1_users SET com_until=UNIX_TIMESTAMP()+3600 WHERE player_id=$actor_id; UPDATE uni1_uni SET feedage=-1" >/dev/null; feed_enabled=1 ;;
    operator-on) db_query "UPDATE uni1_users SET admin=1 WHERE player_id=$actor_id" >/dev/null; hide_email=1 ;;
    operator-off) db_query "UPDATE uni1_users SET admin=1,flags=16384 WHERE player_id=$actor_id" >/dev/null ;;
  esac
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

capture_state() {
  user_state="$(db_query "SELECT JSON_OBJECT(
'name',oname,'nameKey',name,'nameChanged',name_changed,'nameDelay',IF(name_until=0,0,IF(ABS(CAST(name_until AS SIGNED)-CAST(UNIX_TIMESTAMP() AS SIGNED)-604800)<=10,604800,-1)),
'password',password,'email',email,'pemail',pemail,'validated',validated,'validationSet',IF(validatemd='',0,1),'sessionCleared',IF(session='',1,0),
'lang',lang,'skin',skin,'useSkin',useskin,'deactivateIp',deact_ip,'sortBy',sortby,'sortOrder',sortorder,'maxSpy',maxspy,'maxFleet',maxfleetmsg,
'flags',flags,'feedLength',CHAR_LENGTH(feedid),'lastFeed',lastfeed,'admin',admin,'vacation',vacation,
'vacationDelay',IF(vacation_until=0,0,IF(ABS(CAST(vacation_until AS SIGNED)-CAST(UNIX_TIMESTAMP() AS SIGNED)-43200)<=10,43200,IF(vacation_until<UNIX_TIMESTAMP(),-1,-2))),
'disable',disable,'disableDelay',IF(disable_until=0,0,IF(ABS(CAST(disable_until AS SIGNED)-CAST(UNIX_TIMESTAMP() AS SIGNED)-604800)<=10,604800,IF(disable_until>UNIX_TIMESTAMP(),1,-1))))
FROM uni1_users WHERE player_id=$actor_id")"
  queues="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('type',type,'subId',sub_id,'objectId',obj_id,'level',level,'priority',prio,
'timing',CASE WHEN type IN ('AllowName','ChangeEmail') THEN end-2*start ELSE end-start END)))
FROM (SELECT * FROM uni1_queue WHERE owner_id=$actor_id AND type <> 'RecalcPoints' ORDER BY type,task_id) q")"
  planets="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('id',planet_id,'p1',prod1,'p2',prod2,'p3',prod3,'p4',prod4,'p12',prod12,'p212',prod212)))
FROM (SELECT * FROM uni1_planets WHERE owner_id=$actor_id ORDER BY planet_id) p")"
  jq -ncS --argjson user "$user_state" --argjson queues "$queues" --argjson planets "$planets" \
    '{user:$user,queues:($queues|sort_by(.type,.subId,.objectId)),planets:$planets}'
}

run_side() {
  side="$1"; case_name="$2"; configure_case "$case_name"
  clear_mailhog
  form=""; append_form db_character "$name_value"; append_form db_password "$old_password"; append_form newpass1 "$new_password"
  append_form newpass2 "$repeat_password"; append_form db_email "$email_value"; append_form dpath "$skin_path"; append_form lang "$language"
  append_form settings_sort "$sort_by"; append_form settings_order "$sort_order"; append_form spio_anz "$max_spy"; append_form settings_fleetactions "$max_fleet"
  [ "$use_skin" = 1 ] && append_form design on; [ "$deactivate_ip" = 1 ] && append_form noipcheck on
  [ "$vacation" = 1 ] && append_form urlaubs_modus on; [ "$disable_vacation" = 1 ] && append_form urlaub_aus on
  [ "$delete_account" = 1 ] && append_form db_deaktjava on; [ "$espionage" = 1 ] && append_form settings_esp on
  [ "$write_message" = 1 ] && append_form settings_wri on; [ "$buddy" = 1 ] && append_form settings_bud on
  [ "$missile" = 1 ] && append_form settings_mis on; [ "$report" = 1 ] && append_form settings_rep on
  [ "$no_folders" = 1 ] && append_form settings_folders on; [ "$feed_enabled" = 1 ] && append_form feed_activated on
  [ -n "$feed_type" ] && append_form feed_type "$feed_type"; [ "$hide_email" = 1 ] && append_form hide_go_email on
  [ "$resend_activation" = 1 ] && append_form validate 1
  form="${form#&}"

  if [ "$side" = legacy ]; then
    login_legacy "$side-$case_name"
    http="$(curl --silent --show-error --max-time 20 --dump-header "$TMP_DIR/$side-$case_name.headers" --output "$TMP_DIR/$side-$case_name.body" \
      --cookie "$TMP_DIR/$side-$case_name.cookies" --request POST --data "$form" --write-out '%{http_code}' \
      "$LEGACY_BASE_URL/game/index.php?page=options&session=$session&mode=change")"
  else
    login_go "$side-$case_name"
    payload="$(jq -nc --arg name "$name_value" --arg language "$language" --arg skin "$skin_path" --arg old "$old_password" \
      --arg new "$new_password" --arg repeat "$repeat_password" --arg email "$email_value" --arg feedType "$feed_type" \
      --argjson useSkin "$use_skin" --argjson deactivate "$deactivate_ip" --argjson sortBy "$sort_by" --argjson sortOrder "$sort_order" \
      --argjson maxSpy "$max_spy" --argjson maxFleet "$max_fleet" --argjson vacation "$vacation" --argjson disableVacation "$disable_vacation" \
      --argjson deleteAccount "$delete_account" --argjson espionage "$espionage" --argjson writeMessage "$write_message" --argjson buddy "$buddy" \
      --argjson missile "$missile" --argjson report "$report" --argjson folders "$no_folders" --argjson feed "$feed_enabled" \
      --argjson hide "$hide_email" --argjson resend "$resend_activation" \
      '{name:$name,language:$language,skinPath:$skin,useSkin:($useSkin==1),deactivateIp:($deactivate==1),sortBy:$sortBy,sortOrder:$sortOrder,
maxSpy:$maxSpy,maxFleetMessages:$maxFleet,oldPassword:$old,newPassword:$new,newPasswordRepeat:$repeat,email:$email,vacationMode:($vacation==1),
disableVacation:($disableVacation==1),deleteAccount:($deleteAccount==1),showEspionageButton:($espionage==1),showWriteMessage:($writeMessage==1),
showBuddy:($buddy==1),showRocketAttack:($missile==1),showViewReport:($report==1),doNotUseFolders:($folders==1),feedEnabled:($feed==1),
feedType:$feedType,hideGoEmail:($hide==1),resendActivation:($resend==1)}')"
    http="$(curl --silent --show-error --max-time 20 --dump-header "$TMP_DIR/$side-$case_name.headers" --output "$TMP_DIR/$side-$case_name.body" \
      --header 'Content-Type: application/json' --cookie "$cookie" --data "$payload" --write-out '%{http_code}' \
      "$GO_BASE_URL/api/game/options?session=$session&cp=$actor_planet")"
  fi
  cookie_cleared=false
  grep -qi '^Set-Cookie: prsess_.*\(expires=Thu, 01 Jan 1970\|Max-Age=0\)' "$TMP_DIR/$side-$case_name.headers" && cookie_cleared=true || true
  state="$(capture_state)"
  capture_mail
  action_issue=""
  [ "$side" = go ] && action_issue="$(jq -r '.actionIssue.code // empty' "$TMP_DIR/$side-$case_name.body" 2>/dev/null || true)"
  normalized="$(jq -ncS --argjson http "$http" --argjson cookieCleared "$cookie_cleared" --arg issue "$action_issue" --argjson state "$state" --argjson mail "$mail" \
    '{http:$http,cookieCleared:$cookieCleared,issue:$issue,state:$state,mail:$mail}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true
cases="settings clamp forced-language password-mismatch password-special password-short password-wrong password-success
email-password-precedence email-invalid email-used email-success unvalidated-noop unvalidated-change activation-resend
deletion-queue deletion-cancel vacation-enable vacation-blocked vacation-locked vacation-delete-ignored vacation-disable
name-success name-duplicate name-cooldown name-length name-special name-forbidden commander-on commander-off
feed-enable feed-format feed-disable feed-prohibited operator-on operator-off"
for case_name in $cases; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.state')" = "$(printf '%s' "$go" | jq -cS '.state')" ] || pass=false
  [ "$(printf '%s' "$legacy" | jq -cS '.mail')" = "$(printf '%s' "$go" | jq -cS '.mail')" ] || pass=false
  [ "$(printf '%s' "$legacy" | jq -r '.cookieCleared')" = "$(printf '%s' "$go" | jq -r '.cookieCleared')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200 or .http==302' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200' >/dev/null || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "options-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" '{pass:$pass,normalization:"generated session, validation, feed IDs, wall-clock deadlines and activation hosts/codes reduce to stable contracts; persisted account state and normalized mail content remain exact",cases:.}' "$results" > "$REPORT"
[ "$all_pass" = true ]
case_count="$(printf '%s\n' $cases | wc -l | tr -d ' ')"
printf 'Go/PHP options differential E2E: PASS (%s cases)\n' "$case_count"
