#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_ADMIN_BOTS_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-admin-bots-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/admin-bots-differential.XXXXXX")"

admin_id="$(jq -r '.player_id // 0' "$FIXTURE")"
operator_id="$(jq -r '.operator.player_id // 0' "$FIXTURE")"
target_id="$(jq -r '.message_send.sender.player_id // 0' "$FIXTURE")"
[ "$admin_id" -gt 0 ] && [ "$operator_id" -gt 0 ] && [ "$target_id" -gt 0 ]

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

suffix="$$"
tables="uni users planets iplogs queue botvars botstrat"
for table in $tables; do
  backup="uni1_e2e_adminbots_${table}_$suffix"
  db_query "DROP TABLE IF EXISTS $backup; CREATE TABLE $backup AS SELECT * FROM uni1_$table" >/dev/null
  eval "auto_$table=\$(db_query \"SELECT COALESCE(AUTO_INCREMENT,1) FROM information_schema.TABLES WHERE TABLE_SCHEMA='uni' AND TABLE_NAME='uni1_$table'\")"
done

restore_case() {
  for table in $tables; do
    backup="uni1_e2e_adminbots_${table}_$suffix"
    eval "auto=\$auto_$table"
    db_query "DELETE FROM uni1_$table; INSERT INTO uni1_$table SELECT * FROM $backup; ALTER TABLE uni1_$table AUTO_INCREMENT=$auto" >/dev/null
  done
}

cleanup() {
  restore_case >/dev/null 2>&1 || true
  for table in $tables; do
    db_query "DROP TABLE IF EXISTS uni1_e2e_adminbots_${table}_$suffix" >/dev/null 2>&1 || true
  done
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

admin_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$admin_id")"
admin_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$admin_id")"
operator_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$operator_id")"
operator_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$operator_id")"
target_name="$(db_query "SELECT oname FROM uni1_users WHERE player_id=$target_id")"
bot_name="E2EBotDiff$suffix"
bot_name_lower="$(printf '%s' "$bot_name" | tr '[:upper:]' '[:lower:]')"
space_bot_name=" E2ESpace$suffix "
space_bot_name_lower="$(printf '%s' "$space_bot_name" | tr '[:upper:]' '[:lower:]')"
start_source='{"class":"go.GraphLinksModel","nodeDataArray":[{"key":11,"category":"Start","text":"Start"},{"key":12,"category":"End","text":"End"}],"linkDataArray":[{"from":11,"to":12,"text":""}]}'
edit_source='{"class":"go.GraphLinksModel","nodeDataArray":[{"key":21,"category":"Start","text":"Original"}],"linkDataArray":[]}'
saved_source='{"class":"go.GraphLinksModel","nodeDataArray":[{"key":22,"category":"Start","text":"Saved"}],"linkDataArray":[]}'
import_source='{"class":"go.GraphLinksModel","nodeDataArray":[{"key":23,"category":"Start","text":"Imported"}],"linkDataArray":[]}'

seed_strategies() {
  db_query "DELETE FROM uni1_botstrat WHERE id IN (1,800001,800002) OR name IN ('_start','E2E Edit','E2E New','E2E Renamed');
INSERT INTO uni1_botstrat(id,name,source) VALUES
(1,'backup','E2E BACKUP ORIGINAL'),(800001,'_start','$start_source'),(800002,'E2E Edit','$edit_source');
ALTER TABLE uni1_botstrat AUTO_INCREMENT=800100" >/dev/null
}

configure_case() {
  case_name="$1"
  restore_case
  db_query "UPDATE uni1_users SET admin=2 WHERE player_id=$admin_id; UPDATE uni1_users SET admin=1 WHERE player_id=$operator_id" >/dev/null
  seed_strategies
  actor_login="$admin_login"; actor_planet="$admin_planet"; mode=Bots; action=view; name=""; strategy_id=800002
  expected_issue=""; tracked_name=""; output_kind=none
  db_query "DELETE FROM uni1_queue WHERE owner_id=$target_id AND (type='AI' OR obj_id IN (91001,91002))" >/dev/null
  case "$case_name" in
    bots-view)
      db_query "INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES($target_id,'AI',800001,91001,0,1700000000,1700000000,1000)" >/dev/null
      output_kind=bots_view
      ;;
    bots-add) action=add; name="$bot_name"; tracked_name="$bot_name_lower"; expected_issue=bot_added ;;
    bots-add-whitespace) action=add; name="$space_bot_name"; tracked_name="$space_bot_name_lower"; expected_issue=bot_added ;;
    bots-add-existing) action=add; name="$target_name"; expected_issue=bot_exists ;;
    bots-add-no-start) action=add; name="$bot_name"; tracked_name="$bot_name_lower"; expected_issue=bot_no_start; db_query "DELETE FROM uni1_botstrat WHERE name='_start'" >/dev/null ;;
    bots-stop)
      action=stop; expected_issue=bot_stopped
      db_query "INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES
($target_id,'AI',800001,91001,0,1700000000,1700000000,1000),($target_id,'Build',1,91002,0,1700000000,1700000100,0)" >/dev/null
      ;;
    bots-stop-missing) action=stop; expected_issue=bot_stopped ;;
    bots-operator-add) action=add; name="$bot_name"; tracked_name="$bot_name_lower"; actor_login="$operator_login"; actor_planet="$operator_planet"; expected_issue=access_denied ;;
    bots-operator-stop)
      action=stop; actor_login="$operator_login"; actor_planet="$operator_planet"; expected_issue=access_denied
      db_query "INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES($target_id,'AI',800001,91001,0,1700000000,1700000000,1000)" >/dev/null
      ;;
    botedit-view) mode=BotEdit; output_kind=botedit_view ;;
    botedit-load) mode=BotEdit; action=load; output_kind=source ;;
    botedit-save) mode=BotEdit; action=save ;;
    botedit-new) mode=BotEdit; action=new; name='E2E New' ;;
    botedit-rename) mode=BotEdit; action=rename; name='E2E Renamed' ;;
    botedit-import) mode=BotEdit; action=import ;;
    botedit-import-zero) mode=BotEdit; action=import; strategy_id=0 ;;
    botedit-export) mode=BotEdit; action=export; output_kind=source ;;
    botedit-operator-save) mode=BotEdit; action=save; actor_login="$operator_login"; actor_planet="$operator_planet"; expected_issue=access_denied ;;
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

bots_request() {
  side="$1"; body="$TMP_DIR/$side-$case_name.body"
  if [ "$side" = legacy ]; then
    base="$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$actor_planet&mode=Bots"
    case "$action" in
      view) http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --write-out '%{http_code}' "$base")" ;;
      add) http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --data-urlencode "name=$name" --write-out '%{http_code}' "$base")" ;;
      stop) http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --write-out '%{http_code}' "$base&action=stop&id=$target_id")" ;;
    esac
    issue=""
  else
    base="$GO_BASE_URL/api/game/admin?session=$session&cp=$actor_planet&mode=Bots"
    if [ "$action" = view ]; then
      http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$cookie" --write-out '%{http_code}' "$base")"
    else
      payload="$(jq -nc --arg action "$action" --arg name "$name" --argjson target "$target_id" '{action:$action,name:$name,targetIds:[$target]}')"
      http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$cookie" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$base")"
    fi
    issue="$(jq -r '.actionIssue.code // empty' "$body" 2>/dev/null || true)"
  fi
  output="$(request_output "$side" "$body")"
}

botedit_request() {
  side="$1"; body="$TMP_DIR/$side-$case_name.body"
  if [ "$side" = legacy ]; then base="$LEGACY_BASE_URL"; request_cookie="$TMP_DIR/legacy-$case_name.cookies"; else base="$GO_BASE_URL"; request_cookie="$cookie"; fi
  url="$base/game/index.php?page=admin&session=$session&cp=$actor_planet&mode=BotEdit"
  case "$action" in
    view)
      if [ "$side" = go ]; then
        http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$request_cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/admin?session=$session&cp=$actor_planet&mode=BotEdit")"
      else
        http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$request_cookie" --write-out '%{http_code}' "$url")"
      fi
      ;;
    load) http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$request_cookie" --request POST --data "action=load&strat=$strategy_id" --write-out '%{http_code}' "$url")" ;;
    save) http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$request_cookie" --request POST --data-urlencode 'action=save' --data-urlencode "strat=$strategy_id" --data-urlencode "source=$saved_source" --write-out '%{http_code}' "$url")" ;;
    new) http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$request_cookie" --request POST --data-urlencode 'action=new' --data-urlencode "name=$name" --write-out '%{http_code}' "$url")" ;;
    rename) http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$request_cookie" --request POST --data-urlencode 'action=rename' --data-urlencode "strat=$strategy_id" --data-urlencode "name=$name" --write-out '%{http_code}' "$url")" ;;
    import) http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$request_cookie" --request POST --form "strategyId_ForImport=$strategy_id" --form "fileToUpload=@$TMP_DIR/import.json;type=application/json" --write-out '%{http_code}' "$url&action=import")" ;;
    export) http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$request_cookie" --write-out '%{http_code}' "$url&action=export&strat=$strategy_id")" ;;
  esac
  if [ "$side" = go ] && [ "$http" = 403 ]; then issue=access_denied; else issue=""; fi
  output="$(request_output "$side" "$body")"
}

request_output() {
  side="$1"; body="$2"
  case "$output_kind" in
    source) jq -Rsc 'rtrimstr("\n")' "$body" ;;
    bots_view)
      if [ "$side" = legacy ]; then
        if grep -q "$target_name" "$body"; then printf '{"found":true}'; else printf '{"found":false}'; fi
      else jq -c --argjson id "$target_id" '{found:any(.admin.botRows[]?; .playerId==$id)}' "$body"; fi
      ;;
    botedit_view)
      if [ "$side" = legacy ]; then
        if grep -q 'E2E Edit' "$body"; then printf '{"found":true}'; else printf '{"found":false}'; fi
      else jq -c '{found:any(.admin.botStrategies[]?; .name=="E2E Edit")}' "$body"; fi
      ;;
    *) printf '{}' ;;
  esac
}

capture_state() {
  user_id=0
  if [ -n "$tracked_name" ]; then user_id="$(db_query "SELECT COALESCE(MAX(player_id),0) FROM uni1_users WHERE name='$(printf '%s' "$tracked_name" | sed "s/'/''/g")'")"; fi
  if [ "$user_id" -gt 0 ]; then
    user="$(db_query "SELECT JSON_OBJECT('id',0,'name',name,'oname',oname,'validated',validated,'validatemd',validatemd,'home',0,'active',0,'admin',admin,'lang',lang,'dm',dm,'dmfree',dmfree,'flags',flags) FROM uni1_users WHERE player_id=$user_id" | jq -cS .)"
    planet="$(db_query "SELECT JSON_OBJECT('id',0,'name',name,'type',type,'g',g,'s',s,'p',p,'owner',0,'diameter',diameter,'fields',fields,'maxfields',maxfields,'metal',\`700\`,'crystal',\`701\`,'deuterium',\`702\`,'gate',gate_until,'remove',remove) FROM uni1_planets WHERE owner_id=$user_id ORDER BY planet_id LIMIT 1" | jq -cS .)"
    variables="$(db_query "SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT('var',var,'value',IF(var='password','PASSWORD',value))),JSON_ARRAY()) FROM (SELECT var,value FROM uni1_botvars WHERE owner_id=$user_id ORDER BY var) x" | jq -cS .)"
    bot_queue="$(db_query "SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT('type',type,'sub',sub_id,'obj',obj_id,'level',level,'prio',prio,'freeze',freeze,'frozen',frozen)),JSON_ARRAY()) FROM (SELECT * FROM uni1_queue WHERE owner_id=$user_id ORDER BY task_id) x" | jq -cS .)"
    iplogs="$(db_query "SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT('reg',reg)),JSON_ARRAY()) FROM uni1_iplogs WHERE user_id=$user_id" | jq -cS .)"
  else
    user=null; planet=null; variables='[]'; bot_queue='[]'; iplogs='[]'
  fi
  marker_queue="$(db_query "SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT('type',type,'sub',sub_id,'obj',obj_id,'level',level,'prio',prio)),JSON_ARRAY()) FROM (SELECT * FROM uni1_queue WHERE owner_id=$target_id AND obj_id IN (91001,91002) ORDER BY task_id) x" | jq -cS .)"
  strategies="$(db_query "SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT('id',id,'name',name,'source',source)),JSON_ARRAY()) FROM (SELECT * FROM uni1_botstrat WHERE id=1 OR id>=800000 ORDER BY id) x" | jq -cS .)"
  usercount="$(db_query 'SELECT usercount FROM uni1_uni LIMIT 1')"
  state="$(jq -ncS --argjson user "$user" --argjson planet "$planet" --argjson variables "$variables" --argjson queue "$bot_queue" --argjson iplogs "$iplogs" --argjson marker "$marker_queue" --argjson strategies "$strategies" --argjson usercount "$usercount" '{user:$user,planet:$planet,variables:$variables,botQueue:$queue,iplogs:$iplogs,markerQueue:$marker,strategies:$strategies,usercount:$usercount}')"
}

run_side() {
  side="$1"; configure_case "$case_name"
  if [ "$side" = legacy ]; then login_legacy "$side-$case_name"; else login_go "$side-$case_name"; fi
  if [ "$mode" = Bots ]; then bots_request "$side"; else botedit_request "$side"; fi
  capture_state
  normalized="$(jq -ncS --argjson http "$http" --arg issue "$issue" --argjson output "$output" --argjson state "$state" '{http:$http,issue:$issue,output:$output,state:$state}')"
}

printf '%s' "$import_source" > "$TMP_DIR/import.json"
results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true
cases="${OGAME_ADMIN_BOTS_DIFFERENTIAL_CASES:-bots-view bots-add bots-add-whitespace bots-add-existing bots-add-no-start bots-stop bots-stop-missing bots-operator-add bots-operator-stop botedit-view botedit-load botedit-save botedit-new botedit-rename botedit-import botedit-import-zero botedit-export botedit-operator-save}"
for case_name in $cases; do
  printf 'Admin bots differential: %s\n' "$case_name" >&2
  configure_case "$case_name"; expected="$expected_issue"
  run_side legacy; legacy="$normalized"
  run_side go; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.output,.state')" = "$(printf '%s' "$go" | jq -cS '.output,.state')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200 or .http==302' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200 or .http==302 or .http==403' >/dev/null || pass=false
  [ "$(printf '%s' "$go" | jq -r '.issue')" = "$expected" ] || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "admin-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" '{pass:$pass,normalization:"generated player/planet IDs, passwords, IPs, timestamps and random home-planet temperature are excluded; account, planet, variable, queue, strategy and user-count effects remain exact",cases:.}' "$results" > "$REPORT"
[ "$all_pass" = true ]
case_count="$(printf '%s\n' $cases | wc -l | tr -d ' ')"
printf 'Go/PHP admin bots differential E2E: PASS (%s cases)\n' "$case_count"
