#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_MESSAGES_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-messages-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/messages-differential.XXXXXX")"

sender_id="$(jq -r '.message_send.sender.player_id // 0' "$FIXTURE")"
sender_login="$(jq -r '.message_send.sender.login // empty' "$FIXTURE")"
sender_planet="$(jq -r '.message_send.sender.home_planet_id // 0' "$FIXTURE")"
recipient_id="$(jq -r '.message_send.recipient.player_id // 0' "$FIXTURE")"
recipient_login="$(jq -r '.message_send.recipient.login // empty' "$FIXTURE")"
recipient_planet="$(jq -r '.message_send.recipient.home_planet_id // 0' "$FIXTURE")"
[ "$sender_id" -gt 0 ] && [ "$recipient_id" -gt 0 ]
[ "$sender_planet" -gt 0 ] && [ "$recipient_planet" -gt 0 ]
[ -n "$sender_login" ] && [ -n "$recipient_login" ]
users="$sender_id,$recipient_id"

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

messages_backup="uni1_e2e_msgdiff_messages_$$"
reports_backup="uni1_e2e_msgdiff_reports_$$"
users_backup="uni1_e2e_msgdiff_users_$$"
errors_backup="uni1_e2e_msgdiff_errors_$$"
db_query "DROP TABLE IF EXISTS $messages_backup,$reports_backup,$users_backup,$errors_backup;
CREATE TABLE $messages_backup AS SELECT * FROM uni1_messages WHERE owner_id IN ($users);
CREATE TABLE $reports_backup AS SELECT * FROM uni1_reports WHERE owner_id IN ($users) OR msg_id IN (SELECT msg_id FROM uni1_messages WHERE owner_id IN ($users));
CREATE TABLE $users_backup AS SELECT * FROM uni1_users WHERE player_id IN ($users);
CREATE TABLE $errors_backup AS SELECT * FROM uni1_errors WHERE owner_id IN ($users)" >/dev/null

clear_case() {
  db_query "DELETE FROM uni1_reports WHERE owner_id IN ($users) OR msg_id IN (SELECT msg_id FROM uni1_messages WHERE owner_id IN ($users));
DELETE FROM uni1_messages WHERE owner_id IN ($users);
DELETE FROM uni1_errors WHERE owner_id IN ($users)" >/dev/null
}

restore_original() {
  clear_case
  db_query "INSERT INTO uni1_messages SELECT * FROM $messages_backup;
INSERT INTO uni1_reports SELECT * FROM $reports_backup;
INSERT INTO uni1_errors SELECT * FROM $errors_backup;
UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id SET
u.session=b.session,u.private_session=b.private_session,u.lastlogin=b.lastlogin,u.lastclick=b.lastclick,u.ip_addr=b.ip_addr,
u.aktplanet=b.aktplanet,u.lang=b.lang,u.flags=b.flags,u.admin=b.admin,u.validated=b.validated,u.com_until=b.com_until,
u.useskin=b.useskin,u.skin=b.skin,u.vacation=b.vacation,u.vacation_until=b.vacation_until,u.banned=b.banned,
u.banned_until=b.banned_until,u.noattack=b.noattack,u.noattack_until=b.noattack_until,u.disable=b.disable,
u.disable_until=b.disable_until,u.deact_ip=b.deact_ip;
DROP TABLE IF EXISTS $messages_backup,$reports_backup,$users_backup,$errors_backup" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

reset_case() {
  clear_case
  db_query "UPDATE uni1_users SET session='',private_session='',lang='en',flags=0,admin=0,validated=1,com_until=0,
useskin=1,skin='/evolution/',vacation=0,vacation_until=0,banned=0,banned_until=0,noattack=0,noattack_until=0,
disable=0,disable_until=0,deact_ip=1 WHERE player_id IN ($users)" >/dev/null
}

sql_escape() {
  printf '%s' "$1" | sed "s/'/''/g"
}

seed_message() {
  seed_owner="$1"; seed_type="$2"; seed_subject="$(sql_escape "$3")"; seed_text="$(sql_escape "$4")"
  seed_when="$5"; seed_shown="${6:-0}"
  case_message_id="$(db_query "INSERT INTO uni1_messages (owner_id,pm,msgfrom,subj,text,shown,date,planet_id)
VALUES ($seed_owner,$seed_type,'E2E Sender','$seed_subject','$seed_text',$seed_shown,$seed_when,0); SELECT LAST_INSERT_ID()")"
}

seed_report() {
  message_id="$1"
  db_query "INSERT INTO uni1_reports (owner_id,msg_id,msgfrom,subj,text,date)
SELECT owner_id,msg_id,msgfrom,subj,text,date FROM uni1_messages WHERE msg_id=$message_id" >/dev/null
}

login_legacy() {
  prefix="$1"; login="$2"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" \
    --output "$TMP_DIR/$prefix-login.body" --cookie-jar "$TMP_DIR/$prefix.cookies" --request POST \
    --data-urlencode "login=$login" --data-urlencode "pass=$PASSWORD" --write-out '%{http_code}' \
    "$LEGACY_BASE_URL/game/reg/login2.php")"
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/$prefix-login.headers")"
  session="$(printf '%s' "$location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 302 ] && [ -n "$session" ]
  cookie_arg="--cookie $TMP_DIR/$prefix.cookies"
}

login_go() {
  prefix="$1"; login="$2"
  payload="$(jq -nc --arg login "$login" --arg pass "$PASSWORD" '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" \
    --output "$TMP_DIR/$prefix-login.body" --header 'Content-Type: application/json' --data "$payload" \
    --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/$prefix-login.headers" | cut -d';' -f1 | tr -d '\r')"
  redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/$prefix-login.body")"
  session="$(printf '%s' "$redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 200 ] && [ -n "$cookie" ] && [ -n "$session" ]
  cookie_arg="--cookie $cookie"
}

capture_state() {
  messages="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT(
'ownerId',owner_id,'type',pm,'from',msgfrom,'subject',subj,'text',text,'shown',shown,'planetId',planet_id)))
FROM (SELECT * FROM uni1_messages WHERE owner_id IN ($users) ORDER BY owner_id,subj,msg_id) m")"
  reports="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT(
'ownerId',r.owner_id,'messageSubject',COALESCE(m.subj,''),'from',r.msgfrom,'subject',r.subj,'text',r.text)))
FROM uni1_reports r LEFT JOIN uni1_messages m ON m.msg_id=r.msg_id
WHERE r.owner_id IN ($users) OR m.owner_id IN ($users)")"
  user_state="$(db_query "SELECT JSON_ARRAYAGG(JSON_OBJECT('playerId',player_id,'flags',flags,'admin',admin,
'validated',validated,'commander',IF(com_until>UNIX_TIMESTAMP(),1,0))) FROM
(SELECT * FROM uni1_users WHERE player_id IN ($users) ORDER BY player_id) u")"
  jq -ncS --argjson messages "$messages" --argjson reports "$reports" --argjson users "$user_state" \
    '{messages:($messages|sort_by(.ownerId,.subject,.type,.text)),reports:($reports|sort_by(.ownerId,.messageSubject,.subject)),users:$users}'
}

configure_case() {
  name="$1"; reset_case
  actor_id="$sender_id"; actor_login="$sender_login"; actor_planet="$sender_planet"
  now="$(date +%s)"; mode="get"; delete_mode=""; selected_ids='[]'; report_ids='[]'
  subject=""; text_value=""; partial=null; folders=null; expected_issue=""
  case "$name" in
    inbox-read)
      seed_message "$sender_id" 0 Read-PM private "$((now + 1))"
      seed_message "$sender_id" 1 Read-Spy spy "$((now + 2))"
      seed_message "$sender_id" 3 Read-Exp expedition "$((now + 3))"
      seed_message "$sender_id" 5 Read-Misc misc "$((now + 4))"
      ;;
    retention-regular)
      seed_message "$sender_id" 5 Retention-Old old "$((now - 172800))"
      seed_message "$sender_id" 5 Retention-Fresh fresh "$((now + 1))"
      ;;
    retention-admin)
      db_query "UPDATE uni1_users SET admin=1 WHERE player_id=$sender_id" >/dev/null
      seed_message "$sender_id" 5 Retention-Admin old "$((now - 172800))"
      ;;
    delete-marked)
      mode=delete; delete_mode=deletemarked
      seed_message "$sender_id" 5 Delete-A a "$((now + 3))"; selected="$case_message_id"
      seed_message "$sender_id" 5 Delete-B b "$((now + 2))"
      seed_message "$sender_id" 6 Delete-Battle battle "$((now + 1))"
      selected_ids="[$selected]"
      ;;
    delete-nonmarked)
      mode=delete; delete_mode=deletenonmarked
      seed_message "$sender_id" 5 Keep-Selected selected "$((now + 3))"; selected="$case_message_id"
      seed_message "$sender_id" 5 Delete-Unselected unselected "$((now + 2))"
      seed_message "$sender_id" 6 Keep-Battle battle "$((now + 1))"
      selected_ids="[$selected]"
      ;;
    delete-allshown)
      mode=delete; delete_mode=deleteallshown; i=0
      while [ "$i" -lt 30 ]; do
        suffix="$(printf '%02d' "$i")"
        seed_message "$sender_id" 5 "Bulk-$suffix" bulk "$((now + i))"
        i=$((i + 1))
      done
      ;;
    delete-all)
      mode=delete; delete_mode=deleteall
      seed_message "$sender_id" 5 Delete-All misc "$((now + 2))"
      seed_message "$sender_id" 6 Delete-All-Battle battle "$((now + 1))"
      ;;
    report-pm)
      mode=delete; delete_mode=deletemarked; expected_issue=reported
      seed_message "$sender_id" 0 Report-PM reportable "$((now + 1))"; report_ids="[$case_message_id]"
      ;;
    report-duplicate)
      mode=delete; delete_mode=deletemarked; expected_issue=report_exists
      seed_message "$sender_id" 0 Report-Duplicate reportable "$((now + 1))"; report_ids="[$case_message_id]"; seed_report "$case_message_id"
      ;;
    report-foreign)
      mode=delete; delete_mode=deletemarked
      seed_message "$recipient_id" 0 Report-Foreign foreign "$((now + 1))"; report_ids="[$case_message_id]"
      ;;
    flags-regular)
      mode=delete; delete_mode=deletemarked; partial=true
      folders='{"spy":true,"battle":false,"expedition":true,"alliance":false,"personal":false,"other":false}'
      ;;
    flags-commander)
      mode=delete; delete_mode=deletemarked; partial=true
      db_query "UPDATE uni1_users SET com_until=$((now + 86400)) WHERE player_id=$sender_id" >/dev/null
      folders='{"spy":true,"battle":false,"expedition":true,"alliance":false,"personal":false,"other":false}'
      ;;
    send-basic)
      mode=send; subject='Simple subject'; text_value='Simple body'; expected_issue=sent
      ;;
    send-long)
      mode=send; subject='Long message'; text_value="$(awk 'BEGIN { for (i=0;i<2001;i++) printf "x" }')"; expected_issue=sent
      ;;
    send-missing-subject)
      mode=send; subject=''; text_value='Body'; expected_issue=missing_subject
      ;;
    send-missing-text)
      mode=send; subject='Subject'; text_value=''; expected_issue=missing_text
      ;;
    send-unvalidated)
      mode=send; subject='Blocked'; text_value='Body'; expected_issue=not_activated
      ;;
    mailbox-cap)
      mode=send; subject='Cap-New'; text_value='new'; expected_issue=sent; i=0
      while [ "$i" -lt 127 ]; do
        suffix="$(printf '%03d' "$i")"
        seed_message "$recipient_id" 5 "Cap-$suffix" old "$((now - 500 + i))"
        i=$((i + 1))
      done
      ;;
  esac
}

legacy_issue() {
  body="$1"
  if grep -q 'The report has already been sent earlier!' "$body"; then printf report_exists
  elif grep -q 'Report sent!' "$body"; then printf reported
  elif grep -q 'Missing topic' "$body"; then printf missing_subject
  elif grep -q "Where's the message?" "$body"; then printf missing_text
  elif grep -q 'This feature is only available after account activation.' "$body"; then printf not_activated
  elif grep -q 'Message sent' "$body"; then printf sent
  else printf ''
  fi
}

run_side() {
  side="$1"; name="$2"; configure_case "$name"
  if [ "$side" = legacy ]; then login_legacy "$side-$name" "$actor_login"; else login_go "$side-$name" "$actor_login"; fi
  if [ "$name" = send-unvalidated ]; then db_query "UPDATE uni1_users SET validated=0 WHERE player_id=$sender_id" >/dev/null; fi
  if [ "$side" = legacy ]; then
    url="$LEGACY_BASE_URL/game/index.php?page=messages&dsp=1&session=$session&cp=$actor_planet"
    if [ "$mode" = get ]; then
      http="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$name.body" $cookie_arg --write-out '%{http_code}' "$url")"
    elif [ "$mode" = send ]; then
      url="$LEGACY_BASE_URL/game/index.php?page=writemessages&session=$session&cp=$actor_planet&gesendet=1&messageziel=$recipient_id"
      http="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$name.body" $cookie_arg \
        --data-urlencode "betreff=$subject" --data-urlencode "text=$text_value" --write-out '%{http_code}' "$url")"
    else
      set -- --data-urlencode "deletemessages=$delete_mode" --data-urlencode messages=1
      for id in $(printf '%s' "$selected_ids" | jq -r '.[]'); do set -- "$@" --data-urlencode "delmes$id=on"; done
      for id in $(printf '%s' "$report_ids" | jq -r '.[]'); do set -- "$@" --data-urlencode "sneak$id=on"; done
      if [ "$partial" = true ]; then set -- "$@" --data-urlencode fullreports=on; fi
      if [ "$folders" != null ]; then
        [ "$(printf '%s' "$folders" | jq -r .spy)" = true ] && set -- "$@" --data-urlencode espioopen=on
        [ "$(printf '%s' "$folders" | jq -r .expedition)" = true ] && set -- "$@" --data-urlencode expopen=on
      fi
      http="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$name.body" $cookie_arg "$@" --write-out '%{http_code}' "$url")"
    fi
    issue="$(legacy_issue "$TMP_DIR/$side-$name.body")"
  else
    url="$GO_BASE_URL/api/game/messages?session=$session&cp=$actor_planet&dsp=1"
    if [ "$mode" = get ]; then
      http="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$name.body" $cookie_arg --write-out '%{http_code}' "$url")"
    elif [ "$mode" = send ]; then
      payload="$(jq -nc --arg subject "$subject" --arg text "$text_value" --arg target "$recipient_id" \
        '{action:"send",targetPlayerId:($target|tonumber),subject:$subject,text:$text}')"
      http="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$name.body" $cookie_arg \
        --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$url")"
    else
      payload="$(jq -nc --arg mode "$delete_mode" --argjson selected "$selected_ids" --argjson reports "$report_ids" \
        --argjson partial "$partial" --argjson folders "$folders" \
        '{action:"delete",deleteMode:$mode,messageIds:$selected,reportIds:$reports}
        + (if $partial==null then {} else {partialReports:$partial} end)
        + (if $folders==null then {} else {folderSelection:$folders} end)')"
      http="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$name.body" $cookie_arg \
        --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$url")"
    fi
    issue="$(jq -r '.actionIssue.code // ""' "$TMP_DIR/$side-$name.body")"
  fi
  state="$(capture_state)"
  normalized="$(jq -ncS --arg http "$http" --arg issue "$issue" --argjson state "$state" \
    '{http:($http|tonumber),issue:$issue,state:$state}')"
}

results="$TMP_DIR/cases.jsonl"; : > "$results"; all_pass=true
cases='inbox-read retention-regular retention-admin delete-marked delete-nonmarked delete-allshown delete-all report-pm report-duplicate report-foreign flags-regular flags-commander send-basic send-long send-missing-subject send-missing-text send-unvalidated mailbox-cap'
for name in $cases; do
  run_side legacy "$name"; legacy="$normalized"; expected="$expected_issue"
  run_side go "$name"; go="$normalized"
  pass=true; [ "$legacy" = "$go" ] || pass=false
  printf '%s' "$legacy" | jq -e --arg issue "$expected" '.http==200 and .issue==$issue' >/dev/null || pass=false
  case "$name" in
    inbox-read) printf '%s' "$legacy" | jq -e 'all(.state.messages[];.shown==1)' >/dev/null || pass=false ;;
    retention-regular) printf '%s' "$legacy" | jq -e '[.state.messages[].subject]==["Retention-Fresh"] and .state.messages[0].shown==1' >/dev/null || pass=false ;;
    retention-admin) printf '%s' "$legacy" | jq -e '[.state.messages[].subject]==["Retention-Admin"]' >/dev/null || pass=false ;;
    delete-marked) printf '%s' "$legacy" | jq -e '[.state.messages[].subject]==["Delete-B","Delete-Battle"]' >/dev/null || pass=false ;;
    delete-nonmarked) printf '%s' "$legacy" | jq -e '[.state.messages[].subject]==["Keep-Battle","Keep-Selected"]' >/dev/null || pass=false ;;
    delete-allshown) printf '%s' "$legacy" | jq -e '([.state.messages[].subject]==["Bulk-00","Bulk-01","Bulk-02","Bulk-03","Bulk-04"])' >/dev/null || pass=false ;;
    delete-all) printf '%s' "$legacy" | jq -e '(.state.messages|length)==0' >/dev/null || pass=false ;;
    report-pm|report-duplicate) printf '%s' "$legacy" | jq -e '(.state.reports|length)==1' >/dev/null || pass=false ;;
    report-foreign) printf '%s' "$legacy" | jq -e '(.state.reports|length)==0 and (.state.messages|length)==1' >/dev/null || pass=false ;;
    flags-regular) printf '%s' "$legacy" | jq -e '.state.users[0].flags==64' >/dev/null || pass=false ;;
    flags-commander) printf '%s' "$legacy" | jq -e '.state.users[0].flags==1344 and .state.users[0].commander==1' >/dev/null || pass=false ;;
    send-basic) printf '%s' "$legacy" | jq -e '(.state.messages|length)==1 and (.state.messages[0].subject|contains("Simple subject"))' >/dev/null || pass=false ;;
    send-long) printf '%s' "$legacy" | jq -e '.state.messages[0].text|length==2000' >/dev/null || pass=false ;;
    send-missing-*|send-unvalidated) printf '%s' "$legacy" | jq -e '(.state.messages|length)==0' >/dev/null || pass=false ;;
    mailbox-cap) printf '%s' "$legacy" | jq -e '(.state.messages|length)==127 and any(.state.messages[];.subject|contains("Cap-New")) and all(.state.messages[];.subject!="Cap-000")' >/dev/null || pass=false ;;
  esac
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "messages-$name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" '{pass:$pass,cases:.}' "$results" > "$REPORT"
[ "$all_pass" = true ]
printf 'Go/PHP messages differential E2E: PASS (18 cases)\n'
