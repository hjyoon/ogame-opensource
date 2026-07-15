#!/bin/sh
set -eu
[ "${OGAME_E2E_TRACE:-0}" != "1" ] && [ "${1:-}" != "--trace" ] || set -x

ROOT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
MAILHOG_BASE_URL="${OGAME_MAILHOG_BASE_URL:-http://127.0.0.1:${OGAME_MAILHOG_PORT:-8026}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_PUBLIC_ACCOUNT_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-public-account-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"
REG_LOGIN='e2epublicdiff'
REG_NAME='E2ePublicDiff'
REG_EMAIL='e2epublicdiff@example.local'
REG_PASSWORD='E2Ereg123!'

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/public-account-differential.XXXXXX")"

actor_id="$(jq -r '.planet_context.owner.player_id // 0' "$FIXTURE")"
actor_login="$(jq -r '.planet_context.owner.login // empty' "$FIXTURE")"
actor_planet="$(jq -r '.planet_context.owner.home_planet_id // 0' "$FIXTURE")"
recovery_id="$(jq -r '.password_recovery.permanent.player_id // 0' "$FIXTURE")"
recovery_email="$(jq -r '.password_recovery.permanent.email // empty' "$FIXTURE")"
[ "$actor_id" -gt 0 ] && [ "$actor_planet" -gt 0 ] && [ -n "$actor_login" ] && [ "$recovery_id" -gt 0 ] && [ -n "$recovery_email" ]

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

users_backup="uni1_e2e_publicdiff_users_$$"
ranks_backup="uni1_e2e_publicdiff_ranks_$$"
uni_backup="uni1_e2e_publicdiff_uni_$$"
iplogs_backup="uni1_e2e_publicdiff_iplogs_$$"
db_query "DROP TABLE IF EXISTS $users_backup,$ranks_backup,$uni_backup,$iplogs_backup;
CREATE TABLE $users_backup AS SELECT * FROM uni1_users WHERE player_id IN ($actor_id,$recovery_id);
CREATE TABLE $ranks_backup AS SELECT player_id,score1,score2,score3,place1,place2,place3 FROM uni1_users;
CREATE TABLE $uni_backup AS SELECT * FROM uni1_uni;
CREATE TABLE $iplogs_backup AS SELECT log_id,date FROM uni1_iplogs WHERE reg=1 AND date>UNIX_TIMESTAMP()-600;
UPDATE uni1_iplogs l JOIN $iplogs_backup b ON b.log_id=l.log_id SET l.date=UNIX_TIMESTAMP()-601" >/dev/null

restore_users() {
  db_query "UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id SET
u.session=b.session,u.private_session=b.private_session,u.password=b.password,u.validated=b.validated,u.validatemd=b.validatemd,
u.pemail=b.pemail,u.email=b.email,u.lastclick=b.lastclick,u.lastlogin=b.lastlogin,u.ip_addr=b.ip_addr,u.deact_ip=b.deact_ip,
u.admin=b.admin,u.lang=b.lang,u.aktplanet=b.aktplanet,u.vacation=b.vacation,u.vacation_until=b.vacation_until,
u.banned=b.banned,u.banned_until=b.banned_until,u.disable=b.disable,u.disable_until=b.disable_until" >/dev/null
}

cleanup_registered() {
  registered_id="$(db_query "SELECT COALESCE(MAX(player_id),0) FROM uni1_users WHERE name='$REG_LOGIN'")"
  if [ "$registered_id" -gt 0 ]; then
    db_query "DELETE FROM uni1_fleet WHERE owner_id=$registered_id;
DELETE FROM uni1_queue WHERE owner_id=$registered_id; DELETE FROM uni1_buildqueue WHERE owner_id=$registered_id;
DELETE FROM uni1_reports WHERE owner_id=$registered_id OR msg_id IN (SELECT msg_id FROM uni1_messages WHERE owner_id=$registered_id);
DELETE FROM uni1_messages WHERE owner_id=$registered_id; DELETE FROM uni1_notes WHERE owner_id=$registered_id;
DELETE FROM uni1_browse WHERE owner_id=$registered_id; DELETE FROM uni1_template WHERE owner_id=$registered_id;
DELETE FROM uni1_botvars WHERE owner_id=$registered_id; DELETE FROM uni1_userlogs WHERE owner_id=$registered_id;
DELETE FROM uni1_fleetlogs WHERE owner_id=$registered_id OR target_id=$registered_id;
DELETE FROM uni1_iplogs WHERE user_id=$registered_id;
DELETE FROM uni1_union WHERE target_player=$registered_id OR players REGEXP '(^|,)$registered_id(,|$)';
DELETE FROM uni1_planets WHERE owner_id=$registered_id; DELETE FROM uni1_allyapps WHERE player_id=$registered_id;
DELETE FROM uni1_buddy WHERE request_from=$registered_id OR request_to=$registered_id;
DELETE FROM uni1_users WHERE player_id=$registered_id" >/dev/null
    residue="$(db_query "SELECT (SELECT COUNT(*) FROM uni1_users WHERE player_id=$registered_id)+
(SELECT COUNT(*) FROM uni1_planets WHERE owner_id=$registered_id)+(SELECT COUNT(*) FROM uni1_queue WHERE owner_id=$registered_id)+
(SELECT COUNT(*) FROM uni1_buildqueue WHERE owner_id=$registered_id)+(SELECT COUNT(*) FROM uni1_messages WHERE owner_id=$registered_id)")"
    [ "$residue" = 0 ]
  fi
  db_query "UPDATE uni1_uni u JOIN $uni_backup b ON b.num=u.num SET u.usercount=b.usercount;
UPDATE uni1_users u JOIN $ranks_backup b ON b.player_id=u.player_id SET
u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3" >/dev/null
}

cleanup() {
  restore_users >/dev/null 2>&1 || true
  cleanup_registered >/dev/null 2>&1 || true
  db_query "UPDATE uni1_iplogs l JOIN $iplogs_backup b ON b.log_id=l.log_id SET l.date=b.date;
DROP TABLE IF EXISTS $users_backup,$ranks_backup,$uni_backup,$iplogs_backup" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

reset_actor() {
  restore_users
  db_query "UPDATE uni1_users SET session='',private_session='',validated=1,validatemd='',deact_ip=1,admin=0,
vacation=0,vacation_until=0,banned=0,banned_until=0,disable=0,disable_until=0,aktplanet=$actor_planet,
lastclick=UNIX_TIMESTAMP()-100,lastlogin=UNIX_TIMESTAMP()-100 WHERE player_id=$actor_id" >/dev/null
}

reset_recovery() {
  restore_users
  db_query "UPDATE uni1_users SET session='',private_session='' WHERE player_id=$recovery_id" >/dev/null
  original_recovery_password="$(db_query "SELECT password FROM $users_backup WHERE player_id=$recovery_id")"
}

clear_mail() {
  curl --silent --show-error --request DELETE "$MAILHOG_BASE_URL/api/v1/messages" >/dev/null
}

mail_delivered() {
  sleep 1
  total="$(curl --silent --show-error "$MAILHOG_BASE_URL/api/v2/messages" | jq -r '.total // 0')"
  [ "$total" -gt 0 ] && printf true || printf false
}

login_legacy() {
  prefix="$1"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" \
    --output "$TMP_DIR/$prefix-login.body" --cookie-jar "$TMP_DIR/$prefix.cookies" --request POST \
    --data-urlencode "login=$actor_login" --data-urlencode "pass=$PASSWORD" --write-out '%{http_code}' \
    "$LEGACY_BASE_URL/game/reg/login2.php")"
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/$prefix-login.headers")"
  session="$(printf '%s' "$location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  cookie_file="$TMP_DIR/$prefix.cookies"
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

capture_session_state() {
  state="$(db_query "SELECT JSON_OBJECT('public',IF(session='',FALSE,TRUE),'private',IF(private_session='',FALSE,TRUE),
'lastClickRecent',IF(lastclick>=UNIX_TIMESTAMP()-10,TRUE,FALSE),'lastLoginRecent',IF(lastlogin>=UNIX_TIMESTAMP()-10,TRUE,FALSE))
FROM uni1_users WHERE player_id=$actor_id" | jq -cS '.')"
}

run_login_case() {
  side="$1"; reset_actor
  if [ "$side" = legacy ]; then login_legacy "$side-login"; else login_go "$side-login"; fi
  capture_session_state
}

run_session_case() {
  side="$1"; reset_actor
  if [ "$side" = legacy ]; then
    login_legacy "$side-session"; db_query "UPDATE uni1_users SET lastclick=UNIX_TIMESTAMP()-100 WHERE player_id=$actor_id" >/dev/null
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/$side-session.body" --cookie "$cookie_file" \
      --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=overview&session=$session&cp=$actor_planet")"
  else
    login_go "$side-session"; db_query "UPDATE uni1_users SET lastclick=UNIX_TIMESTAMP()-100 WHERE player_id=$actor_id" >/dev/null
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/$side-session.body" --cookie "$cookie" \
      --write-out '%{http_code}' "$GO_BASE_URL/api/game/overview?session=$session&cp=$actor_planet")"
  fi
  [ "$http" = 200 ]; capture_session_state
}

run_logout_case() {
  side="$1"; reset_actor
  if [ "$side" = legacy ]; then
    login_legacy "$side-logout"
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/$side-logout.body" --cookie "$cookie_file" \
      --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=logout&session=$session")"
    [ "$http" = 200 ] || [ "$http" = 302 ]
  else
    login_go "$side-logout"
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/$side-logout.body" --cookie "$cookie" --request POST \
      --write-out '%{http_code}' "$GO_BASE_URL/api/game/logout?session=$session")"
    [ "$http" = 200 ]
  fi
  capture_session_state
  state="$(printf '%s' "$state" | jq -cS '{public:.public,private:.private}')"
}

run_recovery_case() {
  side="$1"; reset_recovery; clear_mail
  if [ "$side" = legacy ]; then
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/$side-recovery.body" --request POST \
      --data-urlencode "email=$recovery_email" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/reg/fa_pass.php")"
  else
    payload="$(jq -nc --arg email "$recovery_email" '{email:$email}')"
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/$side-recovery.body" --header 'Content-Type: application/json' \
      --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/public/password-recovery")"
  fi
  [ "$http" = 200 ]
  current="$(db_query "SELECT password FROM uni1_users WHERE player_id=$recovery_id")"
  changed=false; [ "$current" = "$original_recovery_password" ] || changed=true
  delivered="$(mail_delivered)"
  state="$(jq -ncS --argjson changed "$changed" --argjson mail "$delivered" '{passwordChanged:$changed,mailDelivered:$mail}')"
}

register_account() {
  side="$1"; clear_mail; cleanup_registered
  if [ "$side" = legacy ]; then
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/$side-registration.body" --request POST \
      --data-urlencode "character=$REG_NAME" --data-urlencode "password=$REG_PASSWORD" --data-urlencode "email=$REG_EMAIL" \
      --data-urlencode 'agb=on' --data-urlencode 'universe=http://localhost:8888' \
      --write-out '%{http_code}' "$LEGACY_BASE_URL/game/reg/newredirect.php")"
  else
    payload="$(jq -nc --arg name "$REG_NAME" --arg password "$REG_PASSWORD" --arg email "$REG_EMAIL" \
      '{character:$name,password:$password,email:$email,universe:"http://localhost:8888",termsAccepted:true}')"
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/$side-registration.body" --header 'Content-Type: application/json' \
      --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/public/registration")"
  fi
  [ "$http" = 200 ] || [ "$http" = 302 ]
  registered_id="$(db_query "SELECT COALESCE(MAX(player_id),0) FROM uni1_users WHERE name='$REG_LOGIN'")"
  [ "$registered_id" -gt 0 ]
}

capture_registration_state() {
  user="$(db_query "SELECT JSON_OBJECT('exists',TRUE,'validated',IF(validated<>0,TRUE,FALSE),'activationCode',IF(validatemd='',FALSE,TRUE),
'email',email,'permanentEmail',pemail,'home',IF(hplanetid>0,TRUE,FALSE),'session',IF(session='',FALSE,TRUE),
'private',IF(private_session='',FALSE,TRUE)) FROM uni1_users WHERE player_id=$registered_id")"
  home="$(db_query "SELECT IF(COUNT(*)=1,TRUE,FALSE) FROM uni1_planets WHERE owner_id=$registered_id AND planet_id=(SELECT hplanetid FROM uni1_users WHERE player_id=$registered_id)")"
  greeting="$(db_query "SELECT IF(COUNT(*)=1,TRUE,FALSE) FROM uni1_messages WHERE owner_id=$registered_id AND pm=0")"
  time_limit="$(db_query "SELECT IF(COUNT(*)=1,TRUE,FALSE) FROM uni1_botvars WHERE owner_id=$registered_id AND var='TimeLimit'")"
  ip_log="$(db_query "SELECT IF(COUNT(*)=1,TRUE,FALSE) FROM uni1_iplogs WHERE user_id=$registered_id AND reg=1")"
  user_count="$(db_query "SELECT IF(usercount=(SELECT usercount+1 FROM $uni_backup LIMIT 1),TRUE,FALSE) FROM uni1_uni LIMIT 1")"
  delivered="$(mail_delivered)"
  state="$(jq -ncS --argjson user "$user" --argjson home "$home" --argjson greeting "$greeting" \
    --argjson timeLimit "$time_limit" --argjson ipLog "$ip_log" --argjson userCount "$user_count" --argjson mail "$delivered" \
    '{exists:$user.exists,validated:$user.validated,activationCode:$user.activationCode,email:$user.email,
      permanentEmail:$user.permanentEmail,home:$home,session:$user.session,private:$user.private,greeting:$greeting,
      timeLimit:$timeLimit,ipLog:$ipLog,userCountIncremented:$userCount,mailDelivered:$mail}')"
}

run_registration_case() {
  side="$1"; register_account "$side"; capture_registration_state
}

run_activation_case() {
  side="$1"; register_account "$side"
  code="$(db_query "SELECT validatemd FROM uni1_users WHERE player_id=$registered_id")"; [ -n "$code" ]
  if [ "$side" = legacy ]; then base="$LEGACY_BASE_URL/game/validate.php"; else base="$GO_BASE_URL/activation"; fi
  http="$(curl --silent --show-error --max-time 20 --dump-header "$TMP_DIR/$side-activation.headers" \
    --output "$TMP_DIR/$side-activation.body" --write-out '%{http_code}' "$base?ack=$code")"
  [ "$http" = 302 ]
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/$side-activation.headers")"
  user="$(db_query "SELECT JSON_OBJECT('validated',IF(validated<>0,TRUE,FALSE),'activationCode',IF(validatemd='',FALSE,TRUE),
'emailsMatch',IF(email=pemail,TRUE,FALSE),'session',IF(session='',FALSE,TRUE),'private',IF(private_session='',FALSE,TRUE),
'home',IF(hplanetid>0,TRUE,FALSE)) FROM uni1_users WHERE player_id=$registered_id")"
  redirect=false; printf '%s' "$location" | grep -q 'overview' && redirect=true
  state="$(jq -ncS --argjson user "$user" --argjson redirect "$redirect" \
    '{validated:$user.validated,activationCode:$user.activationCode,emailsMatch:$user.emailsMatch,session:$user.session,private:$user.private,home:$user.home,redirectOverview:$redirect}')"
}

run_side() {
  side="$1"; case_name="$2"
  case "$case_name" in
    login) run_login_case "$side" ;;
    session-touch) run_session_case "$side" ;;
    logout) run_logout_case "$side" ;;
    password-recovery) run_recovery_case "$side" ;;
    registration) run_registration_case "$side" ;;
    activation) run_activation_case "$side" ;;
    *) printf 'Unknown public account differential case: %s\n' "$case_name" >&2; exit 1 ;;
  esac
  normalized="$state"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true; case_count=0
cases='login session-touch logout password-recovery registration activation'
for case_name in $cases; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true; [ "$legacy" = "$go" ] || pass=false; [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "public-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
  case_count=$((case_count + 1))
done

cleanup_registered
jq -s --argjson pass "$all_pass" --argjson cases "$case_count" \
  '{pass:$pass,normalization:"random IDs, passwords and session tokens reduced to lifecycle semantics; account, planet, mail, validation and session effects compared",metrics:{cases:$cases},cases:.}' \
  "$results" > "$REPORT"
[ "$all_pass" = true ] || { jq '.cases[] | select(.pass == false)' "$REPORT" >&2; exit 1; }
printf 'Go/PHP public account differential E2E: PASS (%s cases)\n' "$case_count"
