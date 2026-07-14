#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_ADMIN_LOCA_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-admin-loca-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
command -v bun >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/admin-loca-differential.XXXXXX")"

admin_id="$(jq -r '.player_id // 0' "$FIXTURE")"
operator_id="$(jq -r '.operator.player_id // 0' "$FIXTURE")"
[ "$admin_id" -gt 0 ] && [ "$operator_id" -gt 0 ]

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

suffix="$$"
users_backup="uni1_e2e_adminloca_users_$suffix"
db_query "DROP TABLE IF EXISTS $users_backup;
CREATE TABLE $users_backup AS SELECT player_id,admin FROM uni1_users WHERE player_id IN ($admin_id,$operator_id)" >/dev/null

restore_users() {
  db_query "UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id SET u.admin=b.admin" >/dev/null
}

cleanup() {
  restore_users >/dev/null 2>&1 || true
  db_query "DROP TABLE IF EXISTS $users_backup" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

admin_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$admin_id")"
admin_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$admin_id")"
operator_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$operator_id")"
operator_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$operator_id")"

configure_case() {
  restore_users
  db_query "UPDATE uni1_users SET admin=2 WHERE player_id=$admin_id; UPDATE uni1_users SET admin=1 WHERE player_id=$operator_id" >/dev/null
  actor_login="$admin_login"; actor_planet="$admin_planet"; source=en_en; target=de_de
  case "$case_name" in
    loca-en-de) ;;
    loca-en-en) target=en_en ;;
    loca-en-jp-missing) target=jp_jp ;;
    loca-jp-en) source=jp_jp; target=en_en ;;
    loca-operator-en-de) actor_login="$operator_login"; actor_planet="$operator_planet" ;;
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

request_side() {
  side="$1"; body="$TMP_DIR/$side-$case_name.body"
  if [ "$side" = legacy ]; then
    base="$LEGACY_BASE_URL"; cookie_arg="$TMP_DIR/legacy-$case_name.cookies"
  else
    base="$GO_BASE_URL"; cookie_arg="$cookie"
  fi
  url="$base/game/index.php?page=admin&session=$session&cp=$actor_planet&mode=Loca&action=search"
  http="$(curl --silent --show-error --max-time 60 --output "$body" --cookie "$cookie_arg" --request POST \
    --data-urlencode "loca_src=$source" --data-urlencode "loca_dst=$target" --write-out '%{http_code}' "$url")"
  output="$(bun "$ROOT_DIR/testing/e2e/admin-loca-normalize.ts" "$body")"
  normalized="$(jq -ncS --argjson http "$http" --argjson output "$output" '{http:$http,output:$output}')"
}

run_side() {
  side="$1"; configure_case
  if [ "$side" = legacy ]; then login_legacy "$side-$case_name"; else login_go "$side-$case_name"; fi
  request_side "$side"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true
cases="${OGAME_ADMIN_LOCA_DIFFERENTIAL_CASES:-loca-en-de loca-en-en loca-en-jp-missing loca-jp-en loca-operator-en-de}"
for case_name in $cases; do
  printf 'Admin localization differential: %s\n' "$case_name" >&2
  run_side legacy; legacy="$normalized"
  run_side go; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.output')" = "$(printf '%s' "$go" | jq -cS '.output')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200 and (.output|length)>0' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200 and (.output|length)>0' >/dev/null || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "admin-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" \
  '{pass:$pass,normalization:"HTML shell only; file order, keys, source/target strings, missing states, and status colors remain exact",cases:.}' \
  "$results" > "$REPORT"
[ "$all_pass" = true ]
case_count="$(printf '%s\n' $cases | wc -l | tr -d ' ')"
printf 'Go/PHP admin localization differential E2E: PASS (%s cases)\n' "$case_count"
