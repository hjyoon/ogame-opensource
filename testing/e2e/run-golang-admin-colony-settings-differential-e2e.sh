#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_ADMIN_COLONY_SETTINGS_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-admin-colony-settings-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/admin-colony-settings-differential.XXXXXX")"

admin_id="$(jq -r '.player_id // 0' "$FIXTURE")"
operator_id="$(jq -r '.operator.player_id // 0' "$FIXTURE")"
[ "$admin_id" -gt 0 ] && [ "$operator_id" -gt 0 ]

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

suffix="$$"
colony_backup="uni1_e2e_admincolony_coltab_$suffix"
users_backup="uni1_e2e_admincolony_users_$suffix"
db_query "DROP TABLE IF EXISTS $colony_backup,$users_backup;
CREATE TABLE $colony_backup AS SELECT * FROM uni1_coltab;
CREATE TABLE $users_backup AS SELECT player_id,admin FROM uni1_users WHERE player_id IN ($admin_id,$operator_id)" >/dev/null

restore_case() {
  db_query "DELETE FROM uni1_coltab; INSERT INTO uni1_coltab SELECT * FROM $colony_backup;
UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id SET u.admin=b.admin" >/dev/null
}

cleanup() {
  restore_case >/dev/null 2>&1 || true
  db_query "DROP TABLE IF EXISTS $colony_backup,$users_backup" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

admin_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$admin_id")"
admin_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$admin_id")"
operator_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$operator_id")"
operator_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$operator_id")"
columns="t1_a t1_b t1_c t2_a t2_b t2_c t3_a t3_b t3_c t4_a t4_b t4_c t5_a t5_b t5_c"

settings_json() {
  start="$1"; result='{}'; index=0
  for column in $columns; do
    value=$((start + index))
    result="$(printf '%s' "$result" | jq -c --arg key "$column" --argjson value "$value" '. + {($key):$value}')"
    index=$((index + 1))
  done
  printf '%s' "$result"
}

settings_sql() {
  printf '%s' "$1" | jq -r 'to_entries | map("`" + .key + "`=" + (.value|tostring)) | join(",")'
}

configure_case() {
  case_name="$1"; restore_case
  db_query "UPDATE uni1_users SET admin=2 WHERE player_id=$admin_id; UPDATE uni1_users SET admin=1 WHERE player_id=$operator_id" >/dev/null
  actor_login="$admin_login"; actor_planet="$admin_planet"; action=view; expected_issue=""; values="$(settings_json 11)"
  db_query "UPDATE uni1_coltab SET $(settings_sql "$values")" >/dev/null
  case "$case_name" in
    colony-view) ;;
    colony-save) action=settings; values="$(settings_json 100)"; expected_issue=action_saved ;;
    colony-save-zero) action=settings; values="$(settings_json 0 | jq -c 'with_entries(.value=0)')"; expected_issue=action_saved ;;
    colony-save-uint-max) action=settings; values="$(settings_json 0 | jq -c 'with_entries(.value=4294967295)')"; expected_issue=action_saved ;;
    colony-operator-denied) action=settings; values="$(settings_json 200)"; actor_login="$operator_login"; actor_planet="$operator_planet"; expected_issue=access_denied ;;
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

legacy_output() {
  body="$1"; result='{}'
  for column in $columns; do
    value="$(grep "name=\"$column\"" "$body" | sed -n 's/.*value="\([^"]*\)".*/\1/p' | head -1)"
    result="$(printf '%s' "$result" | jq -c --arg key "$column" --argjson value "${value:-0}" '. + {($key):$value}')"
  done
  printf '%s' "$result"
}

legacy_request() {
  body="$TMP_DIR/legacy-$case_name.body"
  url="$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$actor_planet&mode=ColonySettings"
  if [ "$action" = view ]; then
    http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --write-out '%{http_code}' "$url")"
  else
    form="$(printf '%s' "$values" | jq -r 'to_entries | map(.key + "=" + (.value|tostring)) | join("&")')"
    http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --data "$form" --write-out '%{http_code}' "$url")"
  fi
  issue=""; output="$(legacy_output "$body")"
}

go_request() {
  body="$TMP_DIR/go-$case_name.body"
  url="$GO_BASE_URL/api/game/admin?session=$session&cp=$actor_planet&mode=ColonySettings"
  if [ "$action" = view ]; then
    http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$cookie" --write-out '%{http_code}' "$url")"
  else
    payload="$(jq -nc --arg action "$action" --argjson values "$values" '{action:$action,values:$values}')"
    http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$cookie" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$url")"
  fi
  issue="$(jq -r '.actionIssue.code // empty' "$body" 2>/dev/null || true)"
  output="$(jq -cS '.admin.colonySettings // {}' "$body")"
}

capture_state() {
  state="$(db_query "SELECT JSON_OBJECT(
't1_a',t1_a,'t1_b',t1_b,'t1_c',t1_c,'t2_a',t2_a,'t2_b',t2_b,'t2_c',t2_c,
't3_a',t3_a,'t3_b',t3_b,'t3_c',t3_c,'t4_a',t4_a,'t4_b',t4_b,'t4_c',t4_c,
't5_a',t5_a,'t5_b',t5_b,'t5_c',t5_c) FROM uni1_coltab LIMIT 1" | jq -cS .)"
}

run_side() {
  side="$1"; configure_case "$case_name"
  if [ "$side" = legacy ]; then login_legacy "$side-$case_name"; legacy_request; else login_go "$side-$case_name"; go_request; fi
  capture_state
  normalized="$(jq -ncS --argjson http "$http" --arg issue "$issue" --argjson output "$output" --argjson state "$state" '{http:$http,issue:$issue,output:$output,state:$state}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true
cases="${OGAME_ADMIN_COLONY_SETTINGS_DIFFERENTIAL_CASES:-colony-view colony-save colony-save-zero colony-save-uint-max colony-operator-denied}"
for case_name in $cases; do
  printf 'Admin colony settings differential: %s\n' "$case_name" >&2
  configure_case "$case_name"; expected="$expected_issue"
  run_side legacy; legacy="$normalized"
  run_side go; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.output,.state')" = "$(printf '%s' "$go" | jq -cS '.output,.state')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200' >/dev/null || pass=false
  [ "$(printf '%s' "$go" | jq -r '.issue')" = "$expected" ] || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "admin-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" '{pass:$pass,normalization:"none; all 15 rendered values and persisted unsigned integer settings remain exact",cases:.}' "$results" > "$REPORT"
[ "$all_pass" = true ]
case_count="$(printf '%s\n' $cases | wc -l | tr -d ' ')"
printf 'Go/PHP admin colony settings differential E2E: PASS (%s cases)\n' "$case_count"
