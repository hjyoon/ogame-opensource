#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_ADMIN_AUDIT_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-admin-audit-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/admin-audit-differential.XXXXXX")"

admin_id="$(jq -r '.player_id // 0' "$FIXTURE")"
operator_id="$(jq -r '.operator.player_id // 0' "$FIXTURE")"
target_id="$(jq -r '.admin_operations.target_player_id // 0' "$FIXTURE")"
regular_id="$(jq -r '.message_send.sender.player_id // 0' "$FIXTURE")"
[ "$admin_id" -gt 0 ] && [ "$operator_id" -gt 0 ] && [ "$target_id" -gt 0 ] && [ "$regular_id" -gt 0 ]

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

suffix="$$"
for table in debug errors userlogs iplogs browse; do
  db_query "DROP TABLE IF EXISTS uni1_e2e_audit_${table}_$suffix; CREATE TABLE uni1_e2e_audit_${table}_$suffix AS SELECT * FROM uni1_$table" >/dev/null
done

restore_case() {
  for table in debug errors userlogs iplogs browse; do
    db_query "DELETE FROM uni1_$table; INSERT INTO uni1_$table SELECT * FROM uni1_e2e_audit_${table}_$suffix" >/dev/null
  done
}

cleanup() {
  restore_case >/dev/null 2>&1 || true
  for table in debug errors userlogs iplogs browse; do
    db_query "DROP TABLE IF EXISTS uni1_e2e_audit_${table}_$suffix" >/dev/null 2>&1 || true
  done
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

admin_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$admin_id")"
admin_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$admin_id")"
operator_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$operator_id")"
operator_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$operator_id")"
target_name="$(db_query "SELECT oname FROM uni1_users WHERE player_id=$target_id")"
regular_name="$(db_query "SELECT oname FROM uni1_users WHERE player_id=$regular_id")"

seed_messages() {
  table="$1"
  db_query "DELETE FROM uni1_$table;
INSERT INTO uni1_$table(owner_id,ip,agent,url,text,date)
SELECT $target_id,'198.51.100.7','audit-agent','/audit',CONCAT('AUDIT-DIFF-',UPPER('$table'),'-',LPAD(n,3,'0')),100000+n
FROM (SELECT ROW_NUMBER() OVER () n FROM information_schema.columns LIMIT 55) x" >/dev/null
}

seed_userlogs() {
  db_query "DELETE FROM uni1_userlogs;
INSERT INTO uni1_userlogs(owner_id,date,type,text) VALUES
($target_id,1704153600,'BUILD','AUDIT-DIFF-USERLOG-001'),
($target_id,1704157200,'FLEET','AUDIT-DIFF-USERLOG-002'),
($target_id,1704240000,'BUILD','AUDIT-DIFF-USERLOG-003'),
($regular_id,1704155400,'BUILD','AUDIT-DIFF-USERLOG-004'),
($target_id,1704672000,'BUILD','AUDIT-DIFF-USERLOG-OUTSIDE')" >/dev/null
}

seed_logins() {
  db_query "DELETE FROM uni1_iplogs;
INSERT INTO uni1_iplogs(ip,user_id,reg,date) VALUES
('198.51.100.11',$target_id,0,1700000011),
('198.51.100.12',$target_id,0,1700000012),
('198.51.100.13',$regular_id,0,1700000013),
('198.51.100.14',$target_id,1,1700000014)" >/dev/null
}

seed_browse() {
  db_query "DELETE FROM uni1_browse;
INSERT INTO uni1_browse(owner_id,url,method,getdata,postdata,date)
SELECT $target_id,CONCAT('/AUDIT-DIFF-BROWSE-',LPAD(n,3,'0')),'GET','a:0:{}','a:0:{}',100000+n
FROM (SELECT ROW_NUMBER() OVER () n FROM information_schema.columns LIMIT 55) x" >/dev/null
}

configure_case() {
  case_name="$1"
  restore_case
  actor_login="$admin_login"; actor_planet="$admin_planet"; mode=Debug; action=messages_delete; filter=""; delete_mode=deletemarked; target_ids=""; search_name=""; search_type=ALL
  case "$case_name" in
    debug-filter) seed_messages debug; action=messages_filter; filter=DEBUG-02 ;;
    debug-marked)
      seed_messages debug
      latest="$(db_query "SELECT error_id FROM uni1_debug WHERE text LIKE '%-055'")"
      oldest="$(db_query "SELECT error_id FROM uni1_debug WHERE text LIKE '%-001'")"
      target_ids="$latest,$oldest"
      ;;
    debug-shown) seed_messages debug; delete_mode=deleteshown ;;
    debug-all) seed_messages debug; delete_mode=deleteall ;;
    debug-filter-delete-ignored) seed_messages debug; delete_mode=deleteall; filter=DEBUG-02 ;;
    debug-operator-filter) seed_messages debug; action=messages_filter; filter=DEBUG-02; actor_login="$operator_login"; actor_planet="$operator_planet" ;;
    debug-operator-delete-denied) seed_messages debug; delete_mode=deleteall; actor_login="$operator_login"; actor_planet="$operator_planet" ;;
    errors-marked)
      mode=Errors; seed_messages errors
      latest="$(db_query "SELECT error_id FROM uni1_errors WHERE text LIKE '%-055'")"
      oldest="$(db_query "SELECT error_id FROM uni1_errors WHERE text LIKE '%-001'")"
      target_ids="$latest,$oldest"
      ;;
    errors-all) mode=Errors; seed_messages errors; delete_mode=deleteall ;;
    errors-operator-denied) mode=Errors; seed_messages errors; delete_mode=deleteall; actor_login="$operator_login"; actor_planet="$operator_planet" ;;
    userlogs-build) mode=UserLogs; action=userlogs_search; seed_userlogs; search_name="$target_name"; search_type=BUILD ;;
    userlogs-all) mode=UserLogs; action=userlogs_search; seed_userlogs; search_name="$target_name"; search_type=ALL ;;
    userlogs-casefold) mode=UserLogs; action=userlogs_search; seed_userlogs; search_name="$(printf '%s' "$target_name" | tr '[:upper:]' '[:lower:]')"; search_type=ALL ;;
    userlogs-no-match) mode=UserLogs; action=userlogs_search; seed_userlogs; search_name=zzzz-no-match; search_type=ALL ;;
    userlogs-operator) mode=UserLogs; action=userlogs_search; seed_userlogs; search_name="$target_name"; search_type=BUILD; actor_login="$operator_login"; actor_planet="$operator_planet" ;;
    logins-name) mode=Logins; seed_logins; search_name="$(printf '%.3s' "$target_name")" ;;
    logins-id) mode=Logins; seed_logins ;;
    logins-ip) mode=Logins; seed_logins ;;
    logins-combined) mode=Logins; seed_logins; search_name="$(printf '%.3s' "$target_name")" ;;
    browse-latest) mode=Browse; seed_browse ;;
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

marker_array() {
  pattern="$1"; file="$2"
  { grep -o "$pattern" "$file" || true; } | jq -Rsc 'split("\n")|map(select(length>0))'
}

message_state() {
  table="$1"
  db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(text)) FROM (SELECT text FROM uni1_$table WHERE text LIKE 'AUDIT-DIFF-%' ORDER BY error_id) x"
}

legacy_request() {
  case_name="$1"; body="$TMP_DIR/legacy-$case_name.body"
  url="$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$actor_planet&mode=$mode"
  case "$mode" in
    Debug|Errors)
      if [ "$action" = messages_filter ]; then
        http="$(curl --silent --show-error --max-time 20 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --get --data-urlencode "filter=$filter" --write-out '%{http_code}' "$url")"
      else
        form="deletemessages=$delete_mode"
        [ -n "$filter" ] && form="$form&filter=$filter"
        oldifs="$IFS"; IFS=,
        for id in $target_ids; do [ -n "$id" ] && form="$form&delmes$id=on"; done
        IFS="$oldifs"
        http="$(curl --silent --show-error --max-time 20 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --data "$form" --write-out '%{http_code}' "$url")"
      fi
      output="$(message_state "$(printf '%s' "$mode" | tr '[:upper:]' '[:lower:]')")"
      ;;
    UserLogs)
      http="$(curl --silent --show-error --max-time 20 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST \
        --data-urlencode "name=$search_name" --data-urlencode "type=$search_type" --data 'days=3&hours=0&since=01.01.2024' --write-out '%{http_code}' "$url")"
      output="$(marker_array 'AUDIT-DIFF-USERLOG-[0-9][0-9][0-9]' "$body")"
      ;;
    Logins)
      name=""; id=""; ip=""
      case "$case_name" in
        logins-name) name="$search_name" ;;
        logins-id) id="$target_id" ;;
        logins-ip) ip=198.51.100.12 ;;
        logins-combined) name="$search_name"; id="$target_id"; ip=198.51.100.12 ;;
      esac
      http="$(curl --silent --show-error --max-time 20 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST \
        --data-urlencode "name=$name" --data-urlencode "id=$id" --data-urlencode "ip=$ip" --write-out '%{http_code}' "$url")"
      output="$(marker_array '198\.51\.100\.[0-9][0-9]' "$body")"
      ;;
    Browse)
      http="$(curl --silent --show-error --max-time 20 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --write-out '%{http_code}' "$url")"
      output="$(marker_array 'AUDIT-DIFF-BROWSE-[0-9][0-9][0-9]' "$body")"
      ;;
  esac
  issue=""
}

go_request() {
  case_name="$1"; body="$TMP_DIR/go-$case_name.body"
  url="$GO_BASE_URL/api/game/admin?session=$session&cp=$actor_planet&mode=$mode"
  case "$mode" in
    Debug|Errors)
      targets="[$target_ids]"; [ -n "$target_ids" ] || targets='[]'
      payload="$(jq -nc --arg action "$action" --arg mode "$delete_mode" --arg filter "$filter" --argjson ids "$targets" \
        '{action:$action,deleteMode:$mode,filter:$filter,targetIds:$ids}')"
      http="$(curl --silent --show-error --max-time 20 --output "$body" --cookie "$cookie" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$url")"
      output="$(message_state "$(printf '%s' "$mode" | tr '[:upper:]' '[:lower:]')")"
      ;;
    UserLogs)
      payload="$(jq -nc --arg name "$search_name" --arg type "$search_type" '{action:"userlogs_search",userLogSearch:{name:$name,type:$type,days:3,hours:0,since:"01.01.2024"}}')"
      http="$(curl --silent --show-error --max-time 20 --output "$body" --cookie "$cookie" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$url")"
      output="$(jq -c '[.admin.userLogGroups[]?.rows[]?.text | select(test("^AUDIT-DIFF-USERLOG-[0-9]{3}$"))]' "$body")"
      ;;
    Logins)
      case "$case_name" in
        logins-name) url="$url&name=$(printf '%s' "$search_name" | jq -sRr @uri)" ;;
        logins-id) url="$url&id=$target_id" ;;
        logins-ip) url="$url&ip=198.51.100.12" ;;
        logins-combined) url="$url&name=$(printf '%s' "$search_name" | jq -sRr @uri)&id=$target_id&ip=198.51.100.12" ;;
      esac
      http="$(curl --silent --show-error --max-time 20 --output "$body" --cookie "$cookie" --write-out '%{http_code}' "$url")"
      output="$(jq -c '[.admin.loginRows[]?.ip | select(test("^198\\.51\\.100\\.[0-9]{2}$"))]' "$body")"
      ;;
    Browse)
      http="$(curl --silent --show-error --max-time 20 --output "$body" --cookie "$cookie" --write-out '%{http_code}' "$url")"
      output="$(jq -c '[.admin.browseRows[]?.url | capture("/(?<m>AUDIT-DIFF-BROWSE-[0-9]{3})").m]' "$body")"
      ;;
  esac
  issue="$(jq -r '.actionIssue.code // empty' "$body" 2>/dev/null || true)"
}

run_side() {
  side="$1"; case_name="$2"; configure_case "$case_name"
  if [ "$side" = legacy ]; then login_legacy "$side-$case_name"; legacy_request "$case_name"; else login_go "$side-$case_name"; go_request "$case_name"; fi
  normalized="$(jq -ncS --argjson http "$http" --arg issue "$issue" --argjson output "$output" '{http:$http,issue:$issue,output:$output}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true
cases="debug-filter debug-marked debug-shown debug-all debug-filter-delete-ignored debug-operator-filter debug-operator-delete-denied errors-marked errors-all errors-operator-denied userlogs-build userlogs-all userlogs-casefold userlogs-no-match userlogs-operator logins-name logins-id logins-ip logins-combined browse-latest"
for case_name in $cases; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.output')" = "$(printf '%s' "$go" | jq -cS '.output')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200 or .http==302' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200' >/dev/null || pass=false
  case "$case_name" in
    *operator-delete-denied) [ "$(printf '%s' "$go" | jq -r '.issue')" = access_denied ] || pass=false ;;
  esac
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "admin-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" '{pass:$pass,normalization:"none; exact marker order and post-action table state",cases:.}' "$results" > "$REPORT"
[ "$all_pass" = true ]
case_count="$(printf '%s\n' $cases | wc -l | tr -d ' ')"
printf 'Go/PHP admin audit differential E2E: PASS (%s cases)\n' "$case_count"
