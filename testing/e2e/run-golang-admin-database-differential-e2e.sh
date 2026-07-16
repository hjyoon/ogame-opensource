#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_ADMIN_DATABASE_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-admin-database-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/admin-database-differential.XXXXXX")"

admin_id="$(jq -r '.player_id // 0' "$FIXTURE")"
operator_id="$(jq -r '.operator.player_id // 0' "$FIXTURE")"
[ "$admin_id" -gt 0 ] && [ "$operator_id" -gt 0 ]

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

legacy_container="$(docker compose -f "$ROOT_DIR/docker-compose.yml" ps -q server)"
go_container="$(docker compose -f "$ROOT_DIR/docker-compose.yml" ps -q goapp)"
[ -n "$legacy_container" ] && [ -n "$go_container" ]

suffix="$$"
users_backup="uni1_e2e_admdb_users_$suffix"
db_query "DROP TABLE IF EXISTS $users_backup; CREATE TABLE $users_backup AS
SELECT player_id,admin FROM uni1_users WHERE player_id IN ($admin_id,$operator_id)" >/dev/null

container_path() {
  if [ "$1" = legacy ]; then printf '/var/www/html/game/temp/%s' "$2"; else printf '/srv/ogame/game/temp/%s' "$2"; fi
}

container_write() {
  side="$1"; name="$2"; content="$3"; host="$TMP_DIR/write-$side-$name"
  printf '%s' "$content" > "$host"
  if [ "$side" = legacy ]; then docker cp "$host" "$legacy_container:$(container_path legacy "$name")" >/dev/null
  else docker cp "$host" "$go_container:$(container_path go "$name")" >/dev/null; fi
}

container_remove() {
  side="$1"; name="$2"; path="$(container_path "$side" "$name")"
  if [ "$side" = legacy ]; then docker exec "$legacy_container" rm -f "$path" >/dev/null
  else docker exec "$go_container" rm -f "$path" >/dev/null; fi
}

container_exists() {
  side="$1"; name="$2"; path="$(container_path "$side" "$name")"
  if [ "$side" = legacy ]; then docker exec "$legacy_container" sh -c '[ -f "$1" ] && printf 1 || printf 0' sh "$path"
  else docker exec "$go_container" sh -c '[ -f "$1" ] && printf 1 || printf 0' sh "$path"; fi
}

container_size() {
  side="$1"; name="$2"; path="$(container_path "$side" "$name")"
  if [ "$side" = legacy ]; then docker exec "$legacy_container" stat -c '%s' "$path"
  else docker exec "$go_container" stat -c '%s' "$path"; fi
}

cleanup() {
  db_query "UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id SET u.admin=b.admin;
DELETE FROM uni1_userlogs WHERE type='E2E_DB_DIFF'; DROP TABLE IF EXISTS $users_backup" >/dev/null 2>&1 || true
  for side in legacy go; do
    for name in backup_diff_delete.json backup_diff_guard.json backup_diff_invalid.json backup_diff_partial.json; do
      container_remove "$side" "$name" >/dev/null 2>&1 || true
    done
  done
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

admin_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$admin_id")"
admin_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$admin_id")"
operator_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$operator_id")"
operator_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$operator_id")"
userlog_columns="$(db_query "SELECT JSON_ARRAYAGG(COLUMN_NAME) FROM (SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='uni' AND TABLE_NAME='uni1_userlogs' ORDER BY ORDINAL_POSITION) x")"
partial_backup="$(jq -nc --argjson cols "$userlog_columns" '{userlogs:{auto_increment:null,cols:$cols,values:[]}}')"

configure_case() {
  case_name="$1"
  db_query "DELETE FROM uni1_userlogs WHERE type='E2E_DB_DIFF'; UPDATE uni1_users SET admin=2 WHERE player_id=$admin_id; UPDATE uni1_users SET admin=1 WHERE player_id=$operator_id" >/dev/null
  actor_login="$admin_login"; actor_planet="$admin_planet"; action=create; file_name=""; expected_issue=action_saved
  case "$case_name" in
    database-create) ;;
    database-delete-existing) action=delete; file_name=backup_diff_delete.json ;;
    database-delete-missing) action=delete; file_name=backup_diff_delete.json; expected_issue=action_failed ;;
    database-delete-traversal) action=delete; file_name=../backup_diff_guard.json; expected_issue=action_failed ;;
    database-operator-create) actor_login="$operator_login"; actor_planet="$operator_planet"; expected_issue=access_denied ;;
    database-operator-delete) action=delete; file_name=backup_diff_guard.json; actor_login="$operator_login"; actor_planet="$operator_planet"; expected_issue=access_denied ;;
    database-restore-invalid) action=restore; file_name=backup_diff_invalid.json; expected_issue=action_failed ;;
    database-restore-partial) action=restore; file_name=backup_diff_partial.json; expected_issue=action_failed ;;
    database-restore-missing) action=restore; file_name=backup_diff_guard.json; expected_issue=action_failed ;;
    database-restore-traversal) action=restore; file_name=../backup_diff_guard.json; expected_issue=action_failed ;;
    database-operator-restore) action=restore; file_name=backup_diff_guard.json; actor_login="$operator_login"; actor_planet="$operator_planet"; expected_issue=access_denied ;;
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

legacy_call() {
  request_action="$1"; request_file="$2"; body="$3"
  url="$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$actor_planet&mode=DB&action=$request_action"
  if [ "$request_action" = create ]; then
    curl --silent --show-error --max-time 90 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --write-out '%{http_code}' "$url"
  else
    curl --silent --show-error --max-time 90 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --get --data-urlencode "fname=$request_file" --write-out '%{http_code}' "$url"
  fi
}

go_call() {
  request_action="$1"; request_file="$2"; body="$3"
  payload="$(jq -nc --arg action "$request_action" --arg file "$request_file" '{action:$action,fileName:$file}')"
  curl --silent --show-error --max-time 90 --output "$body" --cookie "$cookie" --header 'Content-Type: application/json' --data "$payload" \
    --write-out '%{http_code}' "$GO_BASE_URL/api/game/admin?session=$session&cp=$actor_planet&mode=DB"
}

created_name_from_body() {
  side="$1"; body="$2"
  if [ "$side" = legacy ]; then grep -o 'backup_[0-9][0-9_]*\.json' "$body" | head -1
  else jq -r '.actionIssue.message // ""' "$body" | grep -o 'backup_[0-9][0-9_]*\.json' | head -1; fi
}

prepare_case_file() {
  side="$1"
  case "$case_name" in
    database-delete-existing) container_write "$side" backup_diff_delete.json '{"delete":"me"}' ;;
    database-delete-traversal|database-operator-delete|database-operator-restore) container_write "$side" backup_diff_guard.json '{"guard":true}' ;;
    database-restore-invalid) container_write "$side" backup_diff_invalid.json '{' ;;
    database-restore-partial) container_write "$side" backup_diff_partial.json "$partial_backup" ;;
  esac
}

request_side() {
  side="$1"; body="$TMP_DIR/$side-$case_name.body"; prepare_case_file "$side"
  created_name=""; non_empty=0
  if [ "$side" = legacy ]; then http="$(legacy_call "$action" "$file_name" "$body")"; issue=""
  else http="$(go_call "$action" "$file_name" "$body")"; issue="$(jq -r '.actionIssue.code // empty' "$body" 2>/dev/null || true)"; fi
  if [ "$case_name" = database-create ]; then
    created_name="$(created_name_from_body "$side" "$body")"
    if [ -n "$created_name" ] && [ "$(container_exists "$side" "$created_name")" = 1 ]; then
      [ "$(container_size "$side" "$created_name")" -gt 0 ] && non_empty=1
      db_query "INSERT INTO uni1_userlogs(owner_id,date,type,text) VALUES($admin_id,UNIX_TIMESTAMP(),'E2E_DB_DIFF','DB-DIFF-POST-BACKUP')" >/dev/null
      if [ "$side" = legacy ]; then http="$(legacy_call restore "$created_name" "$body")"; issue=""
      else http="$(go_call restore "$created_name" "$body")"; issue="$(jq -r '.actionIssue.code // empty' "$body" 2>/dev/null || true)"; fi
    fi
  fi
  marker_count="$(db_query "SELECT COUNT(*) FROM uni1_userlogs WHERE type='E2E_DB_DIFF' AND text='DB-DIFF-POST-BACKUP'")"
  tracked_name="${created_name:-${file_name#../}}"
  if [ -n "$tracked_name" ]; then file_exists="$(container_exists "$side" "$tracked_name")"; else file_exists=0; fi
  state="$(jq -ncS --argjson marker "$marker_count" --argjson exists "$file_exists" --argjson nonEmpty "$non_empty" '{marker:$marker,fileExists:$exists,nonEmpty:$nonEmpty}')"
  if [ -n "$created_name" ]; then
    container_remove "$side" "$created_name"
  fi
  for name in backup_diff_delete.json backup_diff_guard.json backup_diff_invalid.json backup_diff_partial.json; do container_remove "$side" "$name" >/dev/null 2>&1 || true; done
}

run_side() {
  side="$1"; configure_case "$case_name"
  if [ "$side" = legacy ]; then login_legacy "$side-$case_name"; else login_go "$side-$case_name"; fi
  request_side "$side"
  normalized="$(jq -ncS --argjson http "$http" --arg issue "$issue" --argjson state "$state" '{http:$http,issue:$issue,state:$state}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true
cases="${OGAME_ADMIN_DATABASE_DIFFERENTIAL_CASES:-database-create database-delete-existing database-delete-missing database-delete-traversal database-operator-create database-operator-delete database-restore-invalid database-restore-partial database-restore-missing database-restore-traversal database-operator-restore}"
for case_name in $cases; do
  configure_case "$case_name"; expected="$expected_issue"
  run_side legacy; legacy="$normalized"
  run_side go; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.state')" = "$(printf '%s' "$go" | jq -cS '.state')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200 or .http==302' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200' >/dev/null || pass=false
  [ "$(printf '%s' "$go" | jq -r '.issue')" = "$expected" ] || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "admin-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" \
  '{pass:$pass,normalization:"timestamped backup filenames and backup row values are excluded; create/restore lifecycle, non-empty file effects and restore marker state remain exact",cases:.}' \
  "$results" > "$REPORT"
[ "$all_pass" = true ]
case_count="$(printf '%s\n' $cases | wc -l | tr -d ' ')"
printf 'Go/PHP admin database differential E2E: PASS (%s cases)\n' "$case_count"
