#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_ADMIN_CHECKSUM_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-admin-checksum-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/admin-checksum-differential.XXXXXX")"

admin_id="$(jq -r '.player_id // 0' "$FIXTURE")"
operator_id="$(jq -r '.operator.player_id // 0' "$FIXTURE")"
[ "$admin_id" -gt 0 ] && [ "$operator_id" -gt 0 ]

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

suffix="$$"
users_backup="uni1_e2e_adminchecksum_users_$suffix"
db_query "DROP TABLE IF EXISTS $users_backup;
CREATE TABLE $users_backup AS SELECT player_id,admin FROM uni1_users WHERE player_id IN ($admin_id,$operator_id)" >/dev/null

legacy_container="$(docker compose -f "$ROOT_DIR/docker-compose.yml" ps -q server)"
go_container="$(docker compose -f "$ROOT_DIR/docker-compose.yml" ps -q goapp)"
[ -n "$legacy_container" ] && [ -n "$go_container" ]
baseline_files="engine.md5 page_admin.md5 page.md5 reg.md5"

for file in $baseline_files; do
  docker cp "$legacy_container:/var/www/html/game/temp/$file" "$TMP_DIR/legacy-$file.orig" >/dev/null
  docker cp "$go_container:/srv/ogame/game/temp/$file" "$TMP_DIR/go-$file.orig" >/dev/null
done
printf 'a:0:{}' > "$TMP_DIR/empty.md5"

restore_users() {
  db_query "UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id SET u.admin=b.admin" >/dev/null
}

restore_baselines() {
  side="$1"
  if [ "$side" = legacy ]; then
    container="$legacy_container"; path="/var/www/html/game/temp"
  else
    container="$go_container"; path="/srv/ogame/game/temp"
  fi
  for file in $baseline_files; do
    docker cp "$TMP_DIR/$side-$file.orig" "$container:$path/$file" >/dev/null
  done
  if [ "$side" = legacy ]; then
    docker exec --user root "$container" chown www-data:www-data "$path/engine.md5" "$path/page_admin.md5" "$path/page.md5" "$path/reg.md5"
  else
    docker exec --user root "$container" chown ogame:ogame "$path/engine.md5" "$path/page_admin.md5" "$path/page.md5" "$path/reg.md5"
  fi
}

empty_baselines() {
  side="$1"
  if [ "$side" = legacy ]; then
    container="$legacy_container"; path="/var/www/html/game/temp"
  else
    container="$go_container"; path="/srv/ogame/game/temp"
  fi
  for file in $baseline_files; do
    docker cp "$TMP_DIR/empty.md5" "$container:$path/$file" >/dev/null
  done
  if [ "$side" = legacy ]; then
    docker exec --user root "$container" chown www-data:www-data "$path/engine.md5" "$path/page_admin.md5" "$path/page.md5" "$path/reg.md5"
  else
    docker exec --user root "$container" chown ogame:ogame "$path/engine.md5" "$path/page_admin.md5" "$path/page.md5" "$path/reg.md5"
  fi
}

cleanup() {
  restore_users >/dev/null 2>&1 || true
  restore_baselines legacy >/dev/null 2>&1 || true
  restore_baselines go >/dev/null 2>&1 || true
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
  actor_login="$admin_login"; actor_planet="$admin_planet"; action=view; expected_issue=""
  case "$case_name" in
    checksum-current-view) ;;
    checksum-admin-fix) action=fix; expected_issue=action_saved ;;
    checksum-operator-fix) action=fix; actor_login="$operator_login"; actor_planet="$operator_planet"; expected_issue=action_saved ;;
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
  perl -0pe 's/\R/ /g; s#</tr>#</tr>\n#g' "$1" |
    sed -n 's#.*<tr><td>\([^<]*\)</td><td>\([^<]*\)</td><td>.*<b>\([^<]*\)</b>.*#\1|\2|\3#p' |
    jq -Rsc 'split("\n") | map(select(length>0) | split("|") | {path:.[0],checksum:.[1],status:.[2]})'
}

legacy_request() {
  body="$TMP_DIR/legacy-$case_name.body"
  url="$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$actor_planet&mode=Checksum"
  if [ "$action" = view ]; then
    http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --write-out '%{http_code}' "$url")"
  else
    http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --write-out '%{http_code}' "$url")"
  fi
  issue=""; output="$(legacy_output "$body")"
}

go_request() {
  body="$TMP_DIR/go-$case_name.body"
  url="$GO_BASE_URL/api/game/admin?session=$session&cp=$actor_planet&mode=Checksum"
  if [ "$action" = view ]; then
    http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$cookie" --write-out '%{http_code}' "$url")"
  else
    http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$cookie" --header 'Content-Type: application/json' \
      --data '{"action":"fix"}' --write-out '%{http_code}' "$url")"
  fi
  issue="$(jq -r '.actionIssue.code // empty' "$body" 2>/dev/null || true)"
  output="$(jq -c '[.admin.checksumGroups[].rows[] | {path,checksum,status}]' "$body")"
}

capture_baselines() {
  side="$1"
  if [ "$side" = legacy ]; then
    container="$legacy_container"; path="/var/www/html/game/temp"
  else
    container="$go_container"; path="/srv/ogame/game/temp"
  fi
  state='{}'
  for file in $baseline_files; do
    docker cp "$container:$path/$file" "$TMP_DIR/$side-$case_name-$file" >/dev/null
    content="$(jq -Rs . < "$TMP_DIR/$side-$case_name-$file")"
    state="$(printf '%s' "$state" | jq -c --arg key "$file" --argjson content "$content" '. + {($key):$content}')"
  done
}

run_side() {
  side="$1"; configure_case; restore_baselines "$side"
  if [ "$case_name" != checksum-current-view ]; then
    empty_baselines "$side"
  fi
  if [ "$side" = legacy ]; then
    login_legacy "$side-$case_name"
    if [ "$case_name" = checksum-current-view ]; then action=fix; legacy_request; action=view; fi
    legacy_request
  else
    login_go "$side-$case_name"
    if [ "$case_name" = checksum-current-view ]; then action=fix; go_request; action=view; fi
    go_request
  fi
  capture_baselines "$side"
  normalized="$(jq -ncS --argjson http "$http" --arg issue "$issue" --argjson output "$output" --argjson state "$state" \
    '{http:$http,issue:$issue,output:$output,state:$state}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true
cases="${OGAME_ADMIN_CHECKSUM_DIFFERENTIAL_CASES:-checksum-current-view checksum-admin-fix checksum-operator-fix}"
for case_name in $cases; do
  printf 'Admin checksum differential: %s\n' "$case_name" >&2
  configure_case; expected="$expected_issue"
  run_side legacy; legacy="$normalized"
  run_side go; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.output,.state')" = "$(printf '%s' "$go" | jq -cS '.output,.state')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200' >/dev/null || pass=false
  [ "$(printf '%s' "$go" | jq -r '.issue')" = "$expected" ] || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "admin-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" \
  '{pass:$pass,normalization:"none; all source rows and PHP-serialized baseline bytes remain exact",cases:.}' \
  "$results" > "$REPORT"
[ "$all_pass" = true ]
case_count="$(printf '%s\n' $cases | wc -l | tr -d ' ')"
printf 'Go/PHP admin checksum differential E2E: PASS (%s cases)\n' "$case_count"
