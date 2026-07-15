#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_FLEET_TEMPLATE_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-fleet-template-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/fleet-template-differential.XXXXXX")"

commander_id="$(jq -r '.fleet_templates.commander.player_id // 0' "$FIXTURE")"
commander_login="$(jq -r '.fleet_templates.commander.login // empty' "$FIXTURE")"
commander_planet="$(jq -r '.fleet_templates.commander.home_planet_id // 0' "$FIXTURE")"
regular_id="$(jq -r '.fleet_templates.non_commander.player_id // 0' "$FIXTURE")"
regular_login="$(jq -r '.fleet_templates.non_commander.login // empty' "$FIXTURE")"
regular_planet="$(jq -r '.fleet_templates.non_commander.home_planet_id // 0' "$FIXTURE")"
foreign_id="$(jq -r '.fleet_templates.foreign.player_id // 0' "$FIXTURE")"
foreign_login="$(jq -r '.fleet_templates.foreign.login // empty' "$FIXTURE")"
foreign_planet="$(jq -r '.fleet_templates.foreign.home_planet_id // 0' "$FIXTURE")"
[ "$commander_id" -gt 0 ] && [ "$regular_id" -gt 0 ] && [ "$foreign_id" -gt 0 ]
[ "$commander_planet" -gt 0 ] && [ "$regular_planet" -gt 0 ] && [ "$foreign_planet" -gt 0 ]
[ -n "$commander_login" ] && [ -n "$regular_login" ] && [ -n "$foreign_login" ]

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

users_backup="uni1_e2e_templatediff_users_$$"
templates_backup="uni1_e2e_templatediff_templates_$$"
db_query "DROP TABLE IF EXISTS $users_backup,$templates_backup;
CREATE TABLE $users_backup AS SELECT * FROM uni1_users WHERE player_id IN ($commander_id,$regular_id,$foreign_id);
CREATE TABLE $templates_backup AS SELECT * FROM uni1_template WHERE owner_id IN ($commander_id,$regular_id,$foreign_id)" >/dev/null
seed_one="$(db_query 'SELECT COALESCE(MAX(id),0)+1001 FROM uni1_template')"
seed_two=$((seed_one + 1))

restore_users() {
  db_query "UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id SET
u.session=b.session,u.private_session=b.private_session,u.password=b.password,u.validated=b.validated,
u.vacation=b.vacation,u.vacation_until=b.vacation_until,u.disable=b.disable,u.disable_until=b.disable_until,
u.admin=b.admin,u.lang=b.lang,u.com_until=b.com_until,u.aktplanet=b.aktplanet,u.\`108\`=b.\`108\`" >/dev/null
}

restore_original() {
  restore_users
  db_query "DELETE FROM uni1_template WHERE owner_id IN ($commander_id,$regular_id,$foreign_id);
INSERT INTO uni1_template SELECT * FROM $templates_backup;
DROP TABLE IF EXISTS $users_backup,$templates_backup" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

reset_case() {
  restore_users
  db_query "UPDATE uni1_users SET session='',private_session='',validated=1,vacation=0,vacation_until=0,
disable=0,disable_until=0,admin=0,lang='en',aktplanet=hplanetid,\`108\`=1 WHERE player_id IN ($commander_id,$regular_id,$foreign_id);
UPDATE uni1_users SET com_until=UNIX_TIMESTAMP()+86400 WHERE player_id IN ($commander_id,$foreign_id);
UPDATE uni1_users SET com_until=0 WHERE player_id=$regular_id;
DELETE FROM uni1_template WHERE owner_id IN ($commander_id,$regular_id,$foreign_id)" >/dev/null
}

seed_template() {
  template_id="$1"; owner_id="$2"; name="$3"; small="$4"; large="$5"; probe="$6"; satellite="$7"
  db_query "INSERT INTO uni1_template(id,owner_id,name,date,\`202\`,\`203\`,\`210\`,\`212\`)
VALUES($template_id,$owner_id,'$name',1700000000,$small,$large,$probe,$satellite)" >/dev/null
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

urlencode() { jq -rn --arg value "$1" '$value|@uri'; }
append_form() { form="$form&$1=$(urlencode "$2")"; }

template_form() {
  form=''; append_form mode save; append_form template_id "$template_id"; append_form template_name "$template_name"
  for ship_id in 202 203 204 205 206 207 208 209 210 211 213 214 215; do
    value=0
    [ "$ship_id" = 202 ] && value="$small"
    [ "$ship_id" = 203 ] && value="$large"
    [ "$ship_id" = 210 ] && value="$probe"
    append_form "ship[$ship_id]" "$value"
  done
  form="${form#&}"
}

capture_state() {
  state="$(db_query "SELECT COALESCE(JSON_ARRAYAGG(JSON_OBJECT(
'owner',owner_name,'name',name,'smallCargo',\`202\`,'largeCargo',\`203\`,'probe',\`210\`,'satellite',\`212\`)),JSON_ARRAY())
FROM (SELECT IF(owner_id=$commander_id,'commander',IF(owner_id=$regular_id,'regular','foreign')) owner_name,
name,\`202\`,\`203\`,\`210\`,\`212\` FROM uni1_template
WHERE owner_id IN ($commander_id,$regular_id,$foreign_id) ORDER BY owner_id,name,\`202\`,\`203\`) t")"
}

configure_case() {
  case_name="$1"; actor_id="$commander_id"; actor_login="$commander_login"; actor_planet="$commander_planet"
  action=save; template_id=0; template_name='Pair Scout'; small=2; large=0; probe=4; satellite=0
  case "$case_name" in
    create) ;;
    create-bounds) template_name='abcdefghijklmnopqrstuvwxyz0123456789'; small=3; probe=1; satellite=7 ;;
    non-commander-save) actor_id="$regular_id"; actor_login="$regular_login"; actor_planet="$regular_planet" ;;
    max-limit)
      seed_template "$seed_one" "$commander_id" 'Existing A' 1 0 0 0
      seed_template "$seed_two" "$commander_id" 'Existing B' 0 1 0 0 ;;
    update-own)
      seed_template "$seed_one" "$commander_id" 'Before' 1 1 9 0
      template_id="$seed_one"; template_name='After'; small=5; large=0; probe=1 ;;
    update-foreign)
      seed_template "$seed_one" "$commander_id" 'Owner row' 1 0 0 0
      actor_id="$foreign_id"; actor_login="$foreign_login"; actor_planet="$foreign_planet"
      template_id="$seed_one"; template_name='Forbidden'; small=9 ;;
    delete-own)
      seed_template "$seed_one" "$commander_id" 'Delete own' 1 0 0 0; action=delete; template_id="$seed_one" ;;
    delete-foreign)
      seed_template "$seed_one" "$commander_id" 'Keep owner' 1 0 0 0
      actor_id="$foreign_id"; actor_login="$foreign_login"; actor_planet="$foreign_planet"
      action=delete; template_id="$seed_one" ;;
    *) printf 'Unknown fleet-template differential case: %s\n' "$case_name" >&2; exit 1 ;;
  esac
}

run_legacy_action() {
  if [ "$action" = delete ]; then
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/legacy-$case_name.body" \
      --cookie "$TMP_DIR/legacy-$case_name.cookies" --write-out '%{http_code}' \
      "$LEGACY_BASE_URL/game/index.php?page=fleet_templates&session=$session&mode=delete&id=$template_id")"
  else
    template_form
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/legacy-$case_name.body" \
      --cookie "$TMP_DIR/legacy-$case_name.cookies" --request POST --data "$form" --write-out '%{http_code}' \
      "$LEGACY_BASE_URL/game/index.php?page=fleet_templates&session=$session")"
  fi
}

run_go_action() {
  if [ "$action" = delete ]; then
    payload="$(jq -nc --argjson id "$template_id" '{action:"delete",templateId:$id}')"
  else
    payload="$(jq -nc --argjson id "$template_id" --arg name "$template_name" --argjson small "$small" \
      --argjson large "$large" --argjson probe "$probe" --argjson satellite "$satellite" \
      '{action:"save",templateId:$id,name:$name,ships:{"202":$small,"203":$large,"210":$probe,"212":$satellite}}')"
  fi
  http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/go-$case_name.body" \
    --header 'Content-Type: application/json' --cookie "$cookie" --data "$payload" --write-out '%{http_code}' \
    "$GO_BASE_URL/api/game/fleet-templates?session=$session&cp=$actor_planet")"
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
cases='create create-bounds non-commander-save max-limit update-own update-foreign delete-own delete-foreign'
for case_name in $cases; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.state')" = "$(printf '%s' "$go" | jq -cS '.state')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http == 200 or .http == 302' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http == 200' >/dev/null || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "fleet-template-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
  case_count=$((case_count + 1))
done

jq -s --argjson pass "$all_pass" --argjson cases "$case_count" \
  '{pass:$pass,normalization:"generated template IDs and update dates excluded; ownership/name/ship counts exact",metrics:{cases:$cases},cases:.}' \
  "$results" > "$REPORT"
[ "$all_pass" = true ] || { jq '.cases[] | select(.pass == false)' "$REPORT" >&2; exit 1; }
printf 'Go/PHP fleet-template differential E2E: PASS (%s cases)\n' "$case_count"
