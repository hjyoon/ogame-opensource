#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_ADMIN_SIMULATORS_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-admin-simulators-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
command -v bun >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/admin-simulators-differential.XXXXXX")"

admin_id="$(jq -r '.player_id // 0' "$FIXTURE")"
operator_id="$(jq -r '.operator.player_id // 0' "$FIXTURE")"
[ "$admin_id" -gt 0 ] && [ "$operator_id" -gt 0 ]

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

suffix="$$"
users_backup="uni1_e2e_adminsim_users_$suffix"
exp_backup="uni1_e2e_adminsim_exp_$suffix"
messages_backup="uni1_e2e_adminsim_messages_$suffix"
db_query "DROP TABLE IF EXISTS $users_backup,$exp_backup,$messages_backup;
CREATE TABLE $users_backup AS SELECT player_id,admin FROM uni1_users WHERE player_id IN ($admin_id,$operator_id);
CREATE TABLE $exp_backup AS SELECT * FROM uni1_exptab;
CREATE TABLE $messages_backup AS SELECT * FROM uni1_messages WHERE owner_id IN ($admin_id,$operator_id)" >/dev/null

restore_case() {
	db_query "UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id SET u.admin=b.admin;
DELETE FROM uni1_exptab; INSERT INTO uni1_exptab SELECT * FROM $exp_backup;
DELETE FROM uni1_messages WHERE owner_id IN ($admin_id,$operator_id); INSERT INTO uni1_messages SELECT * FROM $messages_backup" >/dev/null
}

cleanup() {
  restore_case >/dev/null 2>&1 || true
  db_query "DROP TABLE IF EXISTS $users_backup,$exp_backup,$messages_backup" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

admin_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$admin_id")"
admin_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$admin_id")"
operator_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$operator_id")"
operator_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$operator_id")"
defenses="401 402 403 404 405 406 407 408 502 503"

rocket_values() {
  result='{"a_weap":0,"d_armor":0,"anz":0,"pziel":0}'
  for id in $defenses; do result="$(printf '%s' "$result" | jq -c --arg key "d_$id" '. + {($key):0}')"; done
  case "$case_name" in
    rocket-intercept|rocket-operator) result="$(printf '%s' "$result" | jq -c '.anz=3 | .d_502=5')" ;;
    rocket-primary) result="$(printf '%s' "$result" | jq -c '.anz=3 | .pziel=401 | .d_401=100 | .d_402=10 | .d_502=2')" ;;
    rocket-sweep) result="$(printf '%s' "$result" | jq -c '.a_weap=4 | .d_armor=3 | .anz=12 | .d_401=20 | .d_402=30 | .d_403=10 | .d_404=4 | .d_405=7 | .d_406=2 | .d_407=1 | .d_408=1 | .d_502=3 | .d_503=2')" ;;
  esac
  printf '%s' "$result"
}

battle_values() {
  result='{"anum":1,"dnum":1,"rapid":0,"debug":0,"fid":30,"did":0,"max_round":6,"a0_weap":0,"a0_shld":0,"a0_armor":0,"d0_weap":0,"d0_shld":0,"d0_armor":0}'
  case "$case_name" in
    battle-attacker|battle-debug|battle-operator|battle-rapid) result="$(printf '%s' "$result" | jq -c '.a0_214=1 | .d0_202=1')" ;;
    battle-defender) result="$(printf '%s' "$result" | jq -c '.a0_202=1 | .d0_214=1')" ;;
    battle-defense) result="$(printf '%s' "$result" | jq -c '.a0_214=1 | .d0_401=1 | .did=30')" ;;
    battle-draw) result="$(printf '%s' "$result" | jq -c '.a0_214=1 | .d0_214=1 | .max_round=1')" ;;
    battle-zero-round) result="$(printf '%s' "$result" | jq -c '.a0_214=1 | .d0_214=1 | .max_round=0')" ;;
    battle-source) result="$(printf '%s' "$result" | jq -c '.a0_202=1 | .d0_202=1')" ;;
  esac
  [ "$case_name" = battle-rapid ] && result="$(printf '%s' "$result" | jq -c 'del(.d0_202) | .d0_214=1 | .max_round=1 | .rapid=1')"
  [ "$case_name" = battle-debug ] && result="$(printf '%s' "$result" | jq -c '.debug=1')"
  printf '%s' "$result"
}

configure_case() {
  restore_case
  db_query "UPDATE uni1_users SET admin=2 WHERE player_id=$admin_id; UPDATE uni1_users SET admin=1 WHERE player_id=$operator_id" >/dev/null
  actor_login="$admin_login"; actor_planet="$admin_planet"; mode=rocket; expcount=0; battle_source=""
  case "$case_name" in
    rocket-*) values="$(rocket_values)" ;;
    battle-*) mode=battle; values="$(battle_values)" ;;
    expedition-zero) mode=expedition; expcount=0 ;;
    expedition-negative) mode=expedition; expcount=-5 ;;
    expedition-nothing) mode=expedition; expcount=25; db_query "UPDATE uni1_exptab SET chance_success=-1" >/dev/null ;;
    expedition-alien|expedition-operator) mode=expedition; expcount=25; db_query "UPDATE uni1_exptab SET chance_success=100,chance_alien=0" >/dev/null ;;
    expedition-trader) mode=expedition; expcount=25; db_query "UPDATE uni1_exptab SET chance_success=100,chance_alien=100,chance_pirates=100,chance_dm=100,chance_lost=100,chance_delay=100,chance_accel=100,chance_res=100,chance_fleet=100" >/dev/null ;;
  esac
  [ "$case_name" = battle-source ] && battle_source='MaxRound = 6
Rapidfire = 0
Attackers = 1
Defenders = 1
Attacker0 = 0 0 0 214 1
Defender0 = 0 0 0 202 1'
  case "$case_name" in *-operator) actor_login="$operator_login"; actor_planet="$operator_planet" ;; esac
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
  prefix="$1"; payload="$(jq -nc --arg login "$actor_login" --arg pass "$PASSWORD" '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" --output "$TMP_DIR/$prefix-login.body" \
    --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/$prefix-login.headers" | cut -d';' -f1 | tr -d '\r')"
  redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/$prefix-login.body")"
  session="$(printf '%s' "$redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 200 ] && [ -n "$cookie" ] && [ -n "$session" ]
}

legacy_request() {
  body="$TMP_DIR/legacy-$case_name.body"
  if [ "$mode" = rocket ]; then
    url="$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$actor_planet&mode=RakSim"
    form="$(printf '%s' "$values" | jq -r 'to_entries | map(.key + "=" + (.value|tostring)) | join("&")')"
    http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --data "$form" --write-out '%{http_code}' "$url")"
    output="$(bun "$ROOT_DIR/testing/e2e/admin-simulators-normalize.ts" "$mode" "$body")"
  elif [ "$mode" = battle ]; then
    url="$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$actor_planet&mode=BattleSim"
    form="$(printf '%s' "$values" | jq -r 'to_entries | map(.key + "=" + (if (.key == "rapid" or .key == "debug") and .value == 1 then "on" else (.value|tostring) end)) | join("&")')"
    http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --data "$form" --data-urlencode "battle_source=$battle_source" --write-out '%{http_code}' "$url")"
    output="$(bun "$ROOT_DIR/testing/e2e/admin-simulators-normalize.ts" battle-legacy "$body")"
  else
    url="$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$actor_planet&mode=Expedition&action=sim"
    http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --data-urlencode "expcount=$expcount" --write-out '%{http_code}' "$url")"
    output="$(bun "$ROOT_DIR/testing/e2e/admin-simulators-normalize.ts" "$mode" "$body")"
  fi
  issue=""
}

go_request() {
  body="$TMP_DIR/go-$case_name.body"; url="$GO_BASE_URL/api/game/admin?session=$session&cp=$actor_planet"
  if [ "$mode" = rocket ]; then
    url="$url&mode=RakSim"; payload="$(jq -nc --argjson values "$values" '{action:"rak_sim",values:$values}')"
  elif [ "$mode" = battle ]; then
    url="$url&mode=BattleSim"; payload="$(jq -nc --argjson values "$values" --arg text "$battle_source" '{action:"battle_sim",values:$values,text:$text}')"
  else
    url="$url&mode=Expedition"; payload="$(jq -nc --argjson count "$expcount" '{action:"sim",values:{expcount:$count}}')"
  fi
  http="$(curl --silent --show-error --max-time 30 --output "$body" --cookie "$cookie" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$url")"
  issue="$(jq -r '.actionIssue.code // empty' "$body")"
  if [ "$mode" = rocket ]; then
    output="$(jq -cS '.actionIssue.result.values' "$body")"
  elif [ "$mode" = battle ]; then
    output="$(bun "$ROOT_DIR/testing/e2e/admin-simulators-normalize.ts" battle-go "$body")"
  else
    output="$(jq -c '.actionIssue.result.series' "$body")"
  fi
}

capture_state() {
  if [ "$mode" = battle ]; then
    state_file="$TMP_DIR/$side-$case_name-state.tsv"
    db_query "SELECT REPLACE(TO_BASE64(text),CHAR(10),''),pm,msgfrom,subj,shown,planet_id,(SELECT COUNT(*) FROM uni1_messages WHERE owner_id=$actor_id),(SELECT COUNT(*) FROM uni1_battledata) FROM uni1_messages WHERE owner_id=$actor_id ORDER BY msg_id DESC LIMIT 1" > "$state_file"
    state="$(bun "$ROOT_DIR/testing/e2e/admin-simulators-normalize.ts" battle-report "$state_file")"
  else
    state="$(jq -nc --arg hash "$(db_query "SELECT MD5(GROUP_CONCAT(CONCAT_WS(':',dm_factor,chance_success,depleted_min,depleted_med,depleted_max,chance_depleted_min,chance_depleted_med,chance_depleted_max,chance_alien,chance_pirates,chance_dm,chance_lost,chance_delay,chance_accel,chance_res,chance_fleet) SEPARATOR '|')) FROM uni1_exptab")" '{hash:$hash}')"
  fi
}

run_side() {
  side="$1"; configure_case
  actor_id="$admin_id"; [ "$actor_login" = "$operator_login" ] && actor_id="$operator_id"
  if [ "$side" = legacy ]; then login_legacy "$side-$case_name"; legacy_request; else login_go "$side-$case_name"; go_request; fi
  capture_state
  normalized="$(jq -ncS --argjson http "$http" --arg issue "$issue" --argjson output "$output" --argjson state "$state" '{http:$http,issue:$issue,output:$output,state:$state}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true
cases="${OGAME_ADMIN_SIMULATORS_DIFFERENTIAL_CASES:-rocket-empty rocket-intercept rocket-primary rocket-sweep rocket-operator expedition-zero expedition-negative expedition-nothing expedition-alien expedition-trader expedition-operator battle-attacker battle-defender battle-defense battle-draw battle-zero-round battle-source battle-rapid battle-debug battle-operator}"
for case_name in $cases; do
  printf 'Admin simulator differential: %s\n' "$case_name" >&2
  run_side legacy; legacy="$normalized"
  run_side go; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.output,.state')" = "$(printf '%s' "$go" | jq -cS '.output,.state')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200 and .issue=="action_saved"' >/dev/null || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "admin-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" '{pass:$pass,normalization:"HTML shell only; Rocket values, ten Expedition buckets, and Battle form/result/report/message/debug-diagnostic semantics are exact after masking only timestamps, random coordinates, and generated IDs",cases:.}' "$results" > "$REPORT"
[ "$all_pass" = true ]
case_count="$(printf '%s\n' $cases | wc -l | tr -d ' ')"
printf 'Go/PHP admin simulator differential E2E: PASS (%s cases)\n' "$case_count"
