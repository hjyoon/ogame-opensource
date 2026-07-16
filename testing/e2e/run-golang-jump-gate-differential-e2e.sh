#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_JUMP_GATE_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-jump-gate-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/jump-gate-differential.XXXXXX")"

source_id="$(jq -r '.phalanx.source_moon_id // 0' "$FIXTURE")"
target_id="$(jq -r '.phalanx_edges.own_target_planet_id // 0' "$FIXTURE")"
no_gate_id="$(jq -r '.phalanx_edges.low_deut_moon_id // 0' "$FIXTURE")"
cooling_id="$(jq -r '.phalanx.target_planet_id // 0' "$FIXTURE")"
foreign_id="$(jq -r '.fleet_lifecycle.target.home_planet_id // 0' "$FIXTURE")"
for id in "$source_id" "$target_id" "$no_gate_id" "$cooling_id" "$foreign_id"; do
  [ "$id" -gt 0 ]
done

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

source_owner="$(db_query "SELECT owner_id FROM uni1_planets WHERE planet_id=$source_id")"
foreign_owner="$(db_query "SELECT owner_id FROM uni1_planets WHERE planet_id=$foreign_id")"
source_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$source_owner")"
[ "$source_owner" -gt 0 ] && [ "$foreign_owner" -gt 0 ] && [ "$source_owner" != "$foreign_owner" ] && [ -n "$source_login" ]

planets="$source_id,$target_id,$no_gate_id,$cooling_id,$foreign_id"
users="$(db_query "SELECT GROUP_CONCAT(DISTINCT owner_id ORDER BY owner_id) FROM uni1_planets WHERE planet_id IN ($planets)")"
planet_backup="uni1_e2e_jumpdiff_planets_$$"
user_backup="uni1_e2e_jumpdiff_users_$$"
fleet_columns='`202`=0,`203`=0,`204`=0,`205`=0,`206`=0,`207`=0,`208`=0,`209`=0,`210`=0,`211`=0,`212`=0,`213`=0,`214`=0,`215`=0'

db_query "DROP TABLE IF EXISTS $planet_backup,$user_backup; CREATE TABLE $planet_backup AS SELECT * FROM uni1_planets WHERE planet_id IN ($planets); CREATE TABLE $user_backup AS SELECT * FROM uni1_users WHERE player_id IN ($users) OR player_id=$source_owner OR player_id=$foreign_owner" >/dev/null

restore_original() {
  db_query "DELETE FROM uni1_planets WHERE planet_id IN ($planets); INSERT INTO uni1_planets SELECT * FROM $planet_backup; UPDATE uni1_users u JOIN $user_backup b ON b.player_id=u.player_id SET u.session=b.session,u.private_session=b.private_session,u.lastlogin=b.lastlogin,u.lastclick=b.lastclick,u.ip_addr=b.ip_addr,u.aktplanet=b.aktplanet,u.lang=b.lang,u.vacation=b.vacation,u.vacation_until=b.vacation_until,u.banned=b.banned,u.banned_until=b.banned_until,u.noattack=b.noattack,u.noattack_until=b.noattack_until,u.disable=b.disable,u.disable_until=b.disable_until,u.validated=b.validated,u.deact_ip=b.deact_ip; DROP TABLE IF EXISTS $planet_backup,$user_backup" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

reset_case() {
  db_query "UPDATE uni1_users SET session='',private_session='',lang='en',vacation=0,vacation_until=0,banned=0,banned_until=0,noattack=0,noattack_until=0,disable=0,disable_until=0,validated=1,deact_ip=1 WHERE player_id IN ($users) OR player_id=$source_owner OR player_id=$foreign_owner; UPDATE uni1_planets SET $fleet_columns,\`43\`=0,gate_until=0,prod1=0,prod2=0,prod3=0,prod4=0,prod12=0,prod212=0,\`700\`=1000000,\`701\`=1000000,\`702\`=1000000,lastpeek=1 WHERE planet_id IN ($planets); UPDATE uni1_planets SET owner_id=$source_owner,type=0,\`43\`=1,\`202\`=5,\`204\`=3,\`212\`=4,name='JumpSource' WHERE planet_id=$source_id; UPDATE uni1_planets SET owner_id=$source_owner,type=0,\`43\`=1,\`202\`=1,\`204\`=1,name='JumpTarget' WHERE planet_id=$target_id; UPDATE uni1_planets SET owner_id=$source_owner,type=0,\`43\`=0,\`202\`=5,name='NoGateMoon' WHERE planet_id=$no_gate_id; UPDATE uni1_planets SET owner_id=$source_owner,type=0,\`43\`=1,\`202\`=5,gate_until=UNIX_TIMESTAMP()+3600,name='CoolingMoon' WHERE planet_id=$cooling_id; UPDATE uni1_planets SET owner_id=$foreign_owner,type=0,\`43\`=1,name='ForeignMoon' WHERE planet_id=$foreign_id" >/dev/null
}

login_legacy() {
  prefix="$1"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" --output "$TMP_DIR/$prefix-login.body" --cookie-jar "$TMP_DIR/$prefix.cookies" --request POST --data-urlencode "login=$source_login" --data-urlencode "pass=$PASSWORD" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/reg/login2.php")"
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/$prefix-login.headers")"
  session="$(printf '%s' "$location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 302 ] && [ -n "$session" ]
  cookie_arg="--cookie $TMP_DIR/$prefix.cookies"
}

login_go() {
  prefix="$1"
  payload="$(jq -nc --arg login "$source_login" --arg pass "$PASSWORD" '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" --output "$TMP_DIR/$prefix-login.body" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/$prefix-login.headers" | cut -d';' -f1 | tr -d '\r')"
  redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/$prefix-login.body")"
  session="$(printf '%s' "$redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 200 ] && [ -n "$cookie" ] && [ -n "$session" ]
  cookie_arg="--cookie $cookie"
}

issue_from_legacy() {
  body="$1"
  if grep -q 'no source moon selected' "$body"; then printf source_moon_missing
  elif grep -q 'no target moon selected' "$body"; then printf target_moon_missing
  elif grep -q 'no jump gate found at source moon' "$body"; then printf source_gate_missing
  elif grep -q 'no jump gate found at the target moon' "$body"; then printf target_gate_missing
  elif grep -q "doesn't belong to you" "$body"; then printf foreign_moon
  elif grep -q 'Jump Gate is in recharge mode' "$body"; then printf cooldown
  elif grep -q 'no ships selected' "$body"; then printf no_ships
  elif grep -q 'not enough ships available' "$body"; then printf not_enough_ships
  else printf moved
  fi
}

capture_state() {
  before_epoch="$1"; after_epoch="$2"; current="$3"; moved_source="$4"; moved_target="$5"
  raw="$(db_query "SELECT JSON_ARRAYAGG(JSON_OBJECT('id',planet_id,'small',\`202\`,'light',\`204\`,'sat',\`212\`,'gate',gate_until,'lastpeek',lastpeek)) FROM (SELECT * FROM uni1_planets WHERE planet_id IN ($planets) ORDER BY planet_id) p")"
  speed="$(db_query 'SELECT fspeed FROM uni1_uni LIMIT 1')"
  expected="$(awk -v speed="$speed" 'BEGIN { value=int(3600/speed)-1; if (value<0) value=0; print value }')"
  printf '%s' "$raw" | jq -cS --arg current "$current" --arg source "$moved_source" --arg target "$moved_target" --arg before "$before_epoch" --arg after "$after_epoch" --arg expected "$expected" '
    map({id,small,light,sat,gate,lastpeek}) as $rows |
    ($rows | map({id,gate,cooling:(.gate > ($after|tonumber))})) as $gates |
    ($gates | map(select(.id==($source|tonumber) or .id==($target|tonumber)))) as $moved |
    {ships:($rows|map({id,small,light,sat})),
     gates:{cooling:($gates|map(.cooling)),
       movedCooldownExact:(if ($moved|length)==2 and all($moved[];.cooling) then
         ($moved[0].gate==$moved[1].gate and all($moved[];.gate >= (($before|tonumber)+($expected|tonumber)-1) and .gate <= (($after|tonumber)+($expected|tonumber)+1)))
       else true end)},
     currentLastpeekChanged:($rows|map(select(.id==($current|tonumber)))[0].lastpeek > 1)}'
}

normalize_view_legacy() {
  body="$1" current="$2"
  targets="$(sed -n 's/.*<option value="\([0-9][0-9]*\)">.*/\1/p' "$body" | sort -n | paste -sd, -)"
  ships="$(sed -n 's/.*name="c\([0-9][0-9]*\)".*/\1/p' "$body" | sort -n | paste -sd, -)"
  issue="$(issue_from_legacy "$body")"; [ "$issue" = moved ] && issue=""
  changed="$( [ "$(db_query "SELECT lastpeek FROM uni1_planets WHERE planet_id=$current")" -gt 1 ] && printf true || printf false )"
  jq -ncS --arg issue "$issue" --arg targets "$targets" --arg ships "$ships" --argjson changed "$changed" '{issue:$issue,targets:$targets,ships:$ships,lastpeekChanged:$changed}'
}

normalize_view_go() {
  body="$1" current="$2"
  changed="$( [ "$(db_query "SELECT lastpeek FROM uni1_planets WHERE planet_id=$current")" -gt 1 ] && printf true || printf false )"
  jq -cS --argjson changed "$changed" '{issue:(.jumpGate.actionIssue.code // ""),targets:([.jumpGate.targets[].id]|sort|map(tostring)|join(",")),ships:([.jumpGate.ships[].id]|sort|map(tostring)|join(",")),lastpeekChanged:$changed}' "$body"
}

run_view() {
  side="$1"; reset_case
  if [ "$side" = legacy ]; then
    login_legacy "$side-view"
    curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-view.body" $cookie_arg "$LEGACY_BASE_URL/game/index.php?page=infos&session=$session&cp=$source_id&gid=43"
    normalized="$(normalize_view_legacy "$TMP_DIR/$side-view.body" "$source_id")"
  else
    login_go "$side-view"
    curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-view.body" $cookie_arg "$GO_BASE_URL/api/game/jump-gate?session=$session&cp=$source_id"
    normalized="$(normalize_view_go "$TMP_DIR/$side-view.body" "$source_id")"
  fi
}

configure_mutation() {
  mode="$1"; reset_case
  case_source="$source_id"; case_target="$target_id"; ships='{}'
  case "$mode" in
    source-planet) case_source="$target_id"; case_target="$source_id"; db_query "UPDATE uni1_planets SET type=1 WHERE planet_id=$case_source" >/dev/null; ships='{"202":1}' ;;
    target-planet) db_query "UPDATE uni1_planets SET type=1 WHERE planet_id=$case_target" >/dev/null; ships='{"202":1}' ;;
    source-no-gate) case_source="$no_gate_id"; ships='{"202":1}' ;;
    target-no-gate) case_target="$no_gate_id"; ships='{"202":1}' ;;
    foreign) case_target="$foreign_id"; ships='{"202":1}' ;;
    empty) ships='{}' ;;
    satellite) ships='{"212":3}' ;;
    insufficient) ships='{"202":999}' ;;
    same) case_target="$source_id"; ships='{"202":1}' ;;
    cooldown) case_target="$cooling_id"; ships='{"202":1}' ;;
    success) ships='{"202":2,"204":1,"212":4}' ;;
    negative) ships='{"202":-2,"204":-1}' ;;
  esac
}

legacy_form() {
  printf 'qm=%s&zm=%s' "$case_source" "$case_target"
  printf '%s' "$ships" | jq -r 'to_entries[] | "&c\(.key)=\(.value|@uri)"' | tr -d '\n'
}

run_mutation() {
  side="$1"; mode="$2"; configure_mutation "$mode"
  transport_valid=false; post_issue=""
  before_epoch="$(date +%s)"
  if [ "$side" = legacy ]; then
    login_legacy "$side-$mode"
    form="$(legacy_form)"
    http="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$side-$mode.headers" --output "$TMP_DIR/$side-$mode.body" --data "$form" $cookie_arg --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=sprungtor&session=$session&cp=$case_source")"
    issue="$(issue_from_legacy "$TMP_DIR/$side-$mode.body")"
    [ "$http" = 302 ] && issue=moved
    if [ "$issue" = moved ]; then
      location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/$side-$mode.headers")"
      printf '%s' "$location" | grep -q "page=infos.*cp=$case_target.*gid=43" && transport_valid=true
      curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$mode-post.body" $cookie_arg "$LEGACY_BASE_URL/game/index.php?page=infos&session=$session&cp=$case_source&gid=43"
      post_issue="$(issue_from_legacy "$TMP_DIR/$side-$mode-post.body")"
    elif [ "$http" = 200 ]; then
      transport_valid=true
    fi
  else
    login_go "$side-$mode"
    payload="$(jq -nc --arg source "$case_source" --arg target "$case_target" --argjson ships "$ships" '{sourceMoonId:($source|tonumber),targetMoonId:($target|tonumber),ships:$ships}')"
    http="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$mode.body" --header 'Content-Type: application/json' --data "$payload" $cookie_arg --write-out '%{http_code}' "$GO_BASE_URL/api/game/jump-gate?session=$session&cp=$case_source")"
    issue="$(jq -r '.jumpGate.actionIssue.code // ""' "$TMP_DIR/$side-$mode.body")"
    [ "$http" = 200 ] && transport_valid=true
    if [ "$issue" = moved ]; then
      curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$mode-post.body" $cookie_arg "$GO_BASE_URL/api/game/jump-gate?session=$session&cp=$case_source"
      post_issue="$(jq -r '.jumpGate.actionIssue.code // ""' "$TMP_DIR/$side-$mode-post.body")"
    fi
  fi
  after_epoch="$(date +%s)"
  state="$(capture_state "$before_epoch" "$after_epoch" "$case_source" "$case_source" "$case_target")"
  normalized="$(jq -ncS --arg issue "$issue" --arg postIssue "$post_issue" --argjson state "$state" '{issue:$issue,postIssue:$postIssue,state:$state}')"
}

results="$TMP_DIR/cases.jsonl"; : > "$results"; all_pass=true
run_view legacy; legacy="$normalized"
run_view go; go="$normalized"
pass=true; [ "$legacy" = "$go" ] || pass=false
printf '%s' "$legacy" | jq -e --arg target "$target_id" '.issue=="" and .targets==($target) and .ships=="202,204"' >/dev/null || pass=false
[ "$pass" = true ] || all_pass=false
jq -nc --arg name view-filtering --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"

for mode in source-planet target-planet source-no-gate target-no-gate foreign empty satellite insufficient same cooldown success negative; do
  run_mutation legacy "$mode"; legacy="$normalized"; legacy_transport_valid="$transport_valid"
  run_mutation go "$mode"; go="$normalized"; go_transport_valid="$transport_valid"
  pass=true; [ "$legacy" = "$go" ] || pass=false
  [ "$legacy_transport_valid" = true ] && [ "$go_transport_valid" = true ] || pass=false
  printf '%s' "$legacy" | jq -e '.state.gates.movedCooldownExact' >/dev/null || pass=false
  if [ "$mode" = success ] || [ "$mode" = negative ]; then
    printf '%s' "$legacy" | jq -e --arg source "$source_id" --arg target "$target_id" '.issue=="moved" and .postIssue=="cooldown" and (.state.ships|map(select(.id==($source|tonumber)))[0])=={id:($source|tonumber),small:3,light:2,sat:4} and (.state.ships|map(select(.id==($target|tonumber)))[0])=={id:($target|tonumber),small:3,light:2,sat:0}' >/dev/null || pass=false
  fi
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "jump-gate-$mode" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" '{pass:$pass,cases:.}' "$results" > "$REPORT"
[ "$all_pass" = true ]
printf 'Go/PHP jump-gate differential E2E: PASS (13 cases)\n'
