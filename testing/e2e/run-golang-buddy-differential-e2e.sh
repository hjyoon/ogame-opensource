#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_BUDDY_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-buddy-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/buddy-differential.XXXXXX")"

requester_id="$(jq -r '.buddy_lifecycle.requester.player_id // 0' "$FIXTURE")"
requester_login="$(jq -r '.buddy_lifecycle.requester.login // empty' "$FIXTURE")"
requester_planet="$(jq -r '.buddy_lifecycle.requester.home_planet_id // 0' "$FIXTURE")"
recipient_id="$(jq -r '.buddy_lifecycle.recipient.player_id // 0' "$FIXTURE")"
recipient_login="$(jq -r '.buddy_lifecycle.recipient.login // empty' "$FIXTURE")"
recipient_planet="$(jq -r '.buddy_lifecycle.recipient.home_planet_id // 0' "$FIXTURE")"
[ "$requester_id" -gt 0 ] && [ "$recipient_id" -gt 0 ] && [ "$requester_planet" -gt 0 ] && [ "$recipient_planet" -gt 0 ]
[ -n "$requester_login" ] && [ -n "$recipient_login" ]
users="$requester_id,$recipient_id"

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

buddy_backup="uni1_e2e_buddydiff_buddy_$$"
message_backup="uni1_e2e_buddydiff_messages_$$"
user_backup="uni1_e2e_buddydiff_users_$$"
db_query "DROP TABLE IF EXISTS $buddy_backup,$message_backup,$user_backup; CREATE TABLE $buddy_backup AS SELECT * FROM uni1_buddy WHERE request_from IN ($users) OR request_to IN ($users); CREATE TABLE $message_backup AS SELECT * FROM uni1_messages WHERE owner_id IN ($users); CREATE TABLE $user_backup AS SELECT * FROM uni1_users WHERE player_id IN ($users)" >/dev/null

clear_case() {
  db_query "DELETE FROM uni1_buddy WHERE request_from IN ($users) OR request_to IN ($users); DELETE FROM uni1_messages WHERE owner_id IN ($users)" >/dev/null
}

restore_original() {
  clear_case
  db_query "INSERT INTO uni1_buddy SELECT * FROM $buddy_backup; INSERT INTO uni1_messages SELECT * FROM $message_backup; UPDATE uni1_users u JOIN $user_backup b ON b.player_id=u.player_id SET u.session=b.session,u.private_session=b.private_session,u.lastlogin=b.lastlogin,u.lastclick=b.lastclick,u.ip_addr=b.ip_addr,u.aktplanet=b.aktplanet,u.lang=b.lang,u.vacation=b.vacation,u.vacation_until=b.vacation_until,u.banned=b.banned,u.banned_until=b.banned_until,u.noattack=b.noattack,u.noattack_until=b.noattack_until,u.disable=b.disable,u.disable_until=b.disable_until,u.validated=b.validated,u.deact_ip=b.deact_ip; DROP TABLE IF EXISTS $buddy_backup,$message_backup,$user_backup" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

reset_case() {
  clear_case
  db_query "UPDATE uni1_users SET session='',private_session='',lang='en',vacation=0,vacation_until=0,banned=0,banned_until=0,noattack=0,noattack_until=0,disable=0,disable_until=0,validated=1,deact_ip=1 WHERE player_id IN ($users)" >/dev/null
}

seed_relation() {
  accepted="$1"; text="$2"
  escaped="$(printf '%s' "$text" | sed "s/'/''/g")"
  case_buddy_id="$(db_query "INSERT INTO uni1_buddy (request_from,request_to,text,accepted) VALUES ($requester_id,$recipient_id,'$escaped',$accepted); SELECT LAST_INSERT_ID()")"
}

login_legacy() {
  prefix="$1"; login="$2"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" --output "$TMP_DIR/$prefix-login.body" --cookie-jar "$TMP_DIR/$prefix.cookies" --request POST --data-urlencode "login=$login" --data-urlencode "pass=$PASSWORD" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/reg/login2.php")"
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/$prefix-login.headers")"
  session="$(printf '%s' "$location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 302 ] && [ -n "$session" ]
  cookie_arg="--cookie $TMP_DIR/$prefix.cookies"
}

login_go() {
  prefix="$1"; login="$2"
  payload="$(jq -nc --arg login "$login" --arg pass "$PASSWORD" '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" --output "$TMP_DIR/$prefix-login.body" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/$prefix-login.headers" | cut -d';' -f1 | tr -d '\r')"
  redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/$prefix-login.body")"
  session="$(printf '%s' "$redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 200 ] && [ -n "$cookie" ] && [ -n "$session" ]
  cookie_arg="--cookie $cookie"
}

capture_state() {
  buddies="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('fromId',request_from,'toId',request_to,'text',text,'accepted',accepted))) FROM (SELECT * FROM uni1_buddy WHERE request_from IN ($users) OR request_to IN ($users) ORDER BY request_from,request_to,buddy_id) b")"
  messages="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('ownerId',owner_id,'type',pm,'from',msgfrom,'subject',subj,'text',text,'shown',shown,'planetId',planet_id))) FROM (SELECT * FROM uni1_messages WHERE owner_id IN ($users) ORDER BY owner_id,msg_id) m")"
  jq -ncS --argjson buddies "$buddies" --argjson messages "$messages" '{buddies:($buddies|sort_by(.fromId,.toId,.text,.accepted)),messages:($messages|sort_by(.ownerId,.type,.from,.subject,.text))}'
}

legacy_next_action() {
  body="$1"
  if grep -q '<td class="c" colspan="6">Your requests</td>' "$body"; then printf 6
  elif grep -q '<td class="c" colspan="6">Request</td>' "$body"; then printf 5
  elif grep -q '<td class="c" colspan="2">Buddy request</td>' "$body"; then printf 7
  else printf 0
  fi
}

configure_case() {
  name="$1"; reset_case
  actor_id="$requester_id"; actor_login="$requester_login"; actor_planet="$requester_planet"
  action=0; buddy_id=0; target_id="$recipient_id"; text=""
  case "$name" in
    request-form) action=7 ;;
    add-empty) action=1 ;;
    add-long) action=1; text="$(awk 'BEGIN { for (i=0;i<5001;i++) printf "x" }')" ;;
    duplicate-pending) seed_relation 0 pending; action=1; text=duplicate ;;
    duplicate-accepted) seed_relation 1 accepted; action=1; text=duplicate ;;
    self-add) action=1; target_id="$requester_id"; text=self ;;
    wrong-accept) seed_relation 0 pending; action=2; buddy_id="$case_buddy_id" ;;
    decline) seed_relation 0 pending; actor_id="$recipient_id"; actor_login="$recipient_login"; actor_planet="$recipient_planet"; action=3; buddy_id="$case_buddy_id" ;;
    withdraw) seed_relation 0 pending; action=4; buddy_id="$case_buddy_id" ;;
    accept) seed_relation 0 pending; actor_id="$recipient_id"; actor_login="$recipient_login"; actor_planet="$recipient_planet"; action=2; buddy_id="$case_buddy_id" ;;
    delete-requester) seed_relation 1 accepted; action=8; buddy_id="$case_buddy_id" ;;
    delete-recipient) seed_relation 1 accepted; actor_id="$recipient_id"; actor_login="$recipient_login"; actor_planet="$recipient_planet"; action=8; buddy_id="$case_buddy_id" ;;
  esac
  expected_next=0
  case "$action" in 2|3) expected_next=5 ;; 4) expected_next=6 ;; 7) expected_next=7 ;; esac
}

run_side() {
  side="$1"; name="$2"; configure_case "$name"
  if [ "$side" = legacy ]; then
    login_legacy "$side-$name" "$actor_login"
    url="$LEGACY_BASE_URL/game/index.php?page=buddy&session=$session&cp=$actor_planet&action=$action&buddy_id=$target_id"
    if [ "$action" = 2 ] || [ "$action" = 3 ] || [ "$action" = 4 ] || [ "$action" = 8 ]; then
      url="$LEGACY_BASE_URL/game/index.php?page=buddy&session=$session&cp=$actor_planet&action=$action&buddy_id=$buddy_id"
    fi
    if [ "$action" = 1 ]; then
      http="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$name.body" --data-urlencode "text=$text" $cookie_arg --write-out '%{http_code}' "$url")"
    else
      http="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$name.body" $cookie_arg --write-out '%{http_code}' "$url")"
    fi
    next="$(legacy_next_action "$TMP_DIR/$side-$name.body")"
    issue=""; grep -q 'There is already a request or membership' "$TMP_DIR/$side-$name.body" && issue=already_sent || true
    if [ "$name" = request-form ]; then
      target="$(grep -io '<th>gobuddyb</th>' "$TMP_DIR/$side-$name.body" | wc -l | tr -d ' ')"
    else target=0; fi
  else
    login_go "$side-$name" "$actor_login"
    if [ "$name" = request-form ]; then
      http="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$name.body" $cookie_arg --write-out '%{http_code}' "$GO_BASE_URL/api/game/buddy?session=$session&cp=$actor_planet&action=7&buddy_id=$target_id")"
    else
      payload="$(jq -nc --arg action "$action" --arg buddy "$buddy_id" --arg target "$target_id" --arg text "$text" '{action:($action|tonumber),buddyId:(if ($action|tonumber)==1 then ($target|tonumber) else ($buddy|tonumber) end),text:$text}')"
      http="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$name.body" --header 'Content-Type: application/json' --data "$payload" $cookie_arg --write-out '%{http_code}' "$GO_BASE_URL/api/game/buddy?session=$session&cp=$actor_planet")"
    fi
    next="$(jq -r '.buddy.action // -1' "$TMP_DIR/$side-$name.body")"
    issue="$(jq -r '.actionIssue.code // ""' "$TMP_DIR/$side-$name.body")"
    if [ "$name" = request-form ]; then
      target="$(jq -r --arg target "$recipient_id" 'if (.buddy.target.playerId // 0)==($target|tonumber) then 1 else 0 end' "$TMP_DIR/$side-$name.body")"
    else target=0; fi
  fi
  state="$(capture_state)"
  normalized="$(jq -ncS --arg http "$http" --arg issue "$issue" --arg next "$next" --arg target "$target" --argjson state "$state" '{http:($http|tonumber),issue:$issue,next:($next|tonumber),target:($target|tonumber),state:$state}')"
}

results="$TMP_DIR/cases.jsonl"; : > "$results"; all_pass=true
for name in request-form add-empty add-long duplicate-pending duplicate-accepted self-add wrong-accept decline withdraw accept delete-requester delete-recipient; do
  run_side legacy "$name"; legacy="$normalized"; expected="$expected_next"
  run_side go "$name"; go="$normalized"
  pass=true; [ "$legacy" = "$go" ] || pass=false
  printf '%s' "$legacy" | jq -e --arg next "$expected" '.http==200 and .next==($next|tonumber)' >/dev/null || pass=false
  case "$name" in
    request-form) printf '%s' "$legacy" | jq -e '.target==1 and (.state.buddies|length)==0 and (.state.messages|length)==0' >/dev/null || pass=false ;;
    add-empty) printf '%s' "$legacy" | jq -e --arg owner "$recipient_id" '(.state.buddies|length)==1 and (.state.buddies[0].text|length)==5 and (.state.messages|length)==1 and .state.messages[0].ownerId==($owner|tonumber) and .state.messages[0].subject=="Buddy request" and .state.messages[0].text==""' >/dev/null || pass=false ;;
    add-long) printf '%s' "$legacy" | jq -e '(.state.buddies[0].text|length)==5000 and (.state.messages[0].text|length)==2000' >/dev/null || pass=false ;;
    duplicate-*) printf '%s' "$legacy" | jq -e '.issue=="already_sent" and (.state.buddies|length)==1 and (.state.messages|length)==0' >/dev/null || pass=false ;;
    self-add) printf '%s' "$legacy" | jq -e '(.state.buddies|length)==0 and (.state.messages|length)==0' >/dev/null || pass=false ;;
    wrong-accept) printf '%s' "$legacy" | jq -e '.state.buddies[0].accepted==0 and (.state.messages|length)==0' >/dev/null || pass=false ;;
    decline|withdraw|delete-requester|delete-recipient) printf '%s' "$legacy" | jq -e '(.state.buddies|length)==0 and (.state.messages|length)==1' >/dev/null || pass=false ;;
    accept) printf '%s' "$legacy" | jq -e '.state.buddies[0].accepted==1 and (.state.messages|length)==1 and .state.messages[0].subject=="confirm"' >/dev/null || pass=false ;;
  esac
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "buddy-$name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" '{pass:$pass,cases:.}' "$results" > "$REPORT"
[ "$all_pass" = true ]
printf 'Go/PHP buddy differential E2E: PASS (12 cases)\n'
