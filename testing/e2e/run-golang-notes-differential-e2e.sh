#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_NOTES_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-notes-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/notes-differential.XXXXXX")"

actor_id="$(jq -r '.message_send.sender.player_id // 0' "$FIXTURE")"
actor_login="$(jq -r '.message_send.sender.login // empty' "$FIXTURE")"
actor_planet="$(jq -r '.message_send.sender.home_planet_id // 0' "$FIXTURE")"
foreign_id="$(jq -r '.message_send.recipient.player_id // 0' "$FIXTURE")"
[ "$actor_id" -gt 0 ] && [ "$actor_planet" -gt 0 ] && [ "$foreign_id" -gt 0 ] && [ -n "$actor_login" ]

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

users_backup="uni1_e2e_notesdiff_users_$$"
notes_backup="uni1_e2e_notesdiff_notes_$$"
db_query "DROP TABLE IF EXISTS $users_backup,$notes_backup;
CREATE TABLE $users_backup AS SELECT * FROM uni1_users WHERE player_id IN ($actor_id,$foreign_id);
CREATE TABLE $notes_backup AS SELECT * FROM uni1_notes WHERE owner_id IN ($actor_id,$foreign_id)" >/dev/null

seed_actor="$(db_query 'SELECT COALESCE(MAX(note_id),0)+1001 FROM uni1_notes')"
seed_foreign=$((seed_actor + 1))

restore_users() {
  db_query "UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id SET
u.session=b.session,u.private_session=b.private_session,u.password=b.password,u.validated=b.validated,
u.vacation=b.vacation,u.vacation_until=b.vacation_until,u.disable=b.disable,u.disable_until=b.disable_until,
u.admin=b.admin,u.lang=b.lang,u.com_until=b.com_until,u.aktplanet=b.aktplanet" >/dev/null
}

restore_original() {
  restore_users
  db_query "DELETE FROM uni1_notes WHERE owner_id IN ($actor_id,$foreign_id);
INSERT INTO uni1_notes SELECT * FROM $notes_backup;
DROP TABLE IF EXISTS $users_backup,$notes_backup" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

reset_case() {
  restore_users
  db_query "UPDATE uni1_users SET session='',private_session='',validated=1,vacation=0,vacation_until=0,
disable=0,disable_until=0,admin=0,lang='en',com_until=0,aktplanet=hplanetid
WHERE player_id IN ($actor_id,$foreign_id);
DELETE FROM uni1_notes WHERE owner_id IN ($actor_id,$foreign_id);
INSERT INTO uni1_notes(note_id,owner_id,subj,text,textsize,prio,date) VALUES
($seed_actor,$actor_id,'Actor seed','Actor body',10,1,1700000000),
($seed_foreign,$foreign_id,'Foreign seed','Foreign body',12,2,1700000001)" >/dev/null
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
  state="$(db_query "SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT(
'owner',owner_name,'subject',subj,'text',text,'textSize',textsize,'priority',prio)),JSON_ARRAY())
FROM (SELECT IF(owner_id=$actor_id,'actor','foreign') owner_name,subj,text,textsize,prio
FROM uni1_notes WHERE owner_id IN ($actor_id,$foreign_id) ORDER BY owner_id,subj,text) n")"
}

configure_case() {
  case_name="$1"
  subject='Created note'; body='Created body'; priority=2
  case "$case_name" in
    create) action=create ;;
    create-empty) action=create; subject=''; body=''; priority=9 ;;
    create-bounds) action=create; subject='abcdefghijklmnopqrstuvwxyz0123456789'; body="$(jq -rn '"x" * 5001')"; priority=-4 ;;
    update) action=update; subject='Updated note'; body='Updated body'; priority=0 ;;
    update-empty) action=update; subject=''; body=''; priority=9 ;;
    foreign-update) action=foreign-update; subject='Forbidden update'; body='Forbidden body'; priority=0 ;;
    delete-own) action=delete-own ;;
    delete-mixed) action=delete-mixed ;;
    delete-duplicates) action=delete-duplicates ;;
    *) printf 'Unknown notes differential case: %s\n' "$case_name" >&2; exit 1 ;;
  esac
}

run_legacy_action() {
  url="$LEGACY_BASE_URL/game/index.php?page=notizen&session=$session"
  case "$action" in
    create)
      http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/legacy-$case_name.body" \
        --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --data-urlencode 's=1' \
        --data-urlencode "betreff=$subject" --data-urlencode "text=$body" --data-urlencode "u=$priority" \
        --write-out '%{http_code}' "$url")" ;;
    update|foreign-update)
      note_id="$seed_actor"; [ "$action" = foreign-update ] && note_id="$seed_foreign"
      http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/legacy-$case_name.body" \
        --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --data-urlencode 's=2' \
        --data-urlencode "n=$note_id" --data-urlencode "betreff=$subject" --data-urlencode "text=$body" \
        --data-urlencode "u=$priority" --write-out '%{http_code}' "$url")" ;;
    delete-own)
      http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/legacy-$case_name.body" \
        --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --data-urlencode "delmes[$seed_actor]=y" \
        --write-out '%{http_code}' "$url")" ;;
    delete-mixed)
      http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/legacy-$case_name.body" \
        --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --data-urlencode "delmes[$seed_actor]=y" \
        --data-urlencode "delmes[$seed_foreign]=y" --write-out '%{http_code}' "$url")" ;;
    delete-duplicates)
      http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/legacy-$case_name.body" \
        --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --data-urlencode "delmes[$seed_actor]=y" \
        --write-out '%{http_code}' "$url")" ;;
  esac
}

run_go_action() {
  case "$action" in
    create) payload="$(jq -nc --arg subject "$subject" --arg text "$body" --argjson priority "$priority" '{action:"create",subject:$subject,text:$text,priority:$priority}')" ;;
    update|foreign-update)
      note_id="$seed_actor"; [ "$action" = foreign-update ] && note_id="$seed_foreign"
      payload="$(jq -nc --arg subject "$subject" --arg text "$body" --argjson priority "$priority" --argjson id "$note_id" '{action:"update",noteId:$id,subject:$subject,text:$text,priority:$priority}')" ;;
    delete-own) payload="$(jq -nc --argjson id "$seed_actor" '{action:"delete",noteIds:[$id]}')" ;;
    delete-mixed) payload="$(jq -nc --argjson own "$seed_actor" --argjson foreign "$seed_foreign" '{action:"delete",noteIds:[$own,$foreign]}')" ;;
    delete-duplicates) payload="$(jq -nc --argjson id "$seed_actor" '{action:"delete",noteIds:[$id,$id,0,-1]}')" ;;
  esac
  http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/go-$case_name.body" \
    --header 'Content-Type: application/json' --cookie "$cookie" --data "$payload" --write-out '%{http_code}' \
    "$GO_BASE_URL/api/game/notes?session=$session&cp=$actor_planet")"
}

run_side() {
  side="$1"; case_name="$2"
  reset_case; configure_case "$case_name"
  if [ "$side" = legacy ]; then
    login_legacy "$side-$case_name"; run_legacy_action
  else
    login_go "$side-$case_name"; run_go_action
  fi
  capture_state
  normalized="$(jq -ncS --argjson http "$http" --argjson state "$state" '{http:$http,state:$state}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true; case_count=0
cases='create create-empty create-bounds update update-empty foreign-update delete-own delete-mixed delete-duplicates'
for case_name in $cases; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.state')" = "$(printf '%s' "$go" | jq -cS '.state')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http == 200' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http == 200' >/dev/null || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "notes-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
  case_count=$((case_count + 1))
done

jq -s --argjson pass "$all_pass" --argjson cases "$case_count" \
  '{pass:$pass,normalization:"generated note IDs and wall-clock dates excluded; persisted owner/content/size/priority exact",metrics:{cases:$cases},cases:.}' \
  "$results" > "$REPORT"
[ "$all_pass" = true ] || { jq '.cases[] | select(.pass == false)' "$REPORT" >&2; exit 1; }
printf 'Go/PHP notes differential E2E: PASS (%s cases)\n' "$case_count"
