#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_PHALANX_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-phalanx-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/phalanx-differential.XXXXXX")"

source_id="$(jq -r '.phalanx.source_moon_id // 0' "$FIXTURE")"
target_id="$(jq -r '.phalanx.target_planet_id // 0' "$FIXTURE")"
home_id="$(jq -r '.phalanx_edges.own_target_planet_id // 0' "$FIXTURE")"
low_source_id="$(jq -r '.phalanx_edges.low_deut_moon_id // 0' "$FIXTURE")"
low_login="$(jq -r '.phalanx_edges.low_login // empty' "$FIXTURE")"
[ "$source_id" -gt 0 ] && [ "$target_id" -gt 0 ] && [ "$home_id" -gt 0 ] && [ "$low_source_id" -gt 0 ] && [ -n "$low_login" ]

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

source_owner="$(db_query "SELECT owner_id FROM uni1_planets WHERE planet_id=$source_id")"
target_owner="$(db_query "SELECT owner_id FROM uni1_planets WHERE planet_id=$target_id")"
low_owner="$(db_query "SELECT owner_id FROM uni1_planets WHERE planet_id=$low_source_id")"
source_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$source_owner")"
source_g="$(db_query "SELECT g FROM uni1_planets WHERE planet_id=$source_id")"
source_s="$(db_query "SELECT s FROM uni1_planets WHERE planet_id=$source_id")"
far_target_id="$(db_query "SELECT planet_id FROM uni1_planets WHERE owner_id<>$source_owner AND (g<>$source_g OR ABS(s-$source_s)>8) AND (type=1 OR type=3 OR type>=20001) ORDER BY ABS(g-$source_g),ABS(s-$source_s),planet_id LIMIT 1")"
[ "$source_owner" -gt 0 ] && [ "$target_owner" -gt 0 ] && [ "$low_owner" -gt 0 ] && [ -n "$source_login" ] && [ "$far_target_id" -gt 0 ]

planets="$source_id,$home_id,$low_source_id"
users="$source_owner,$target_owner,$low_owner"
planet_backup="uni1_e2e_phalanxdiff_planets_$$"
user_backup="uni1_e2e_phalanxdiff_users_$$"
fleet_backup="uni1_e2e_phalanxdiff_fleet_$$"
queue_backup="uni1_e2e_phalanxdiff_queue_$$"

db_query "DROP TABLE IF EXISTS $planet_backup,$user_backup,$fleet_backup,$queue_backup; CREATE TABLE $planet_backup AS SELECT * FROM uni1_planets WHERE planet_id IN ($planets); CREATE TABLE $user_backup AS SELECT * FROM uni1_users WHERE player_id IN ($users); CREATE TABLE $fleet_backup AS SELECT * FROM uni1_fleet WHERE start_planet=$target_id OR target_planet=$target_id; CREATE TABLE $queue_backup AS SELECT * FROM uni1_queue WHERE type='Fleet' AND sub_id IN (SELECT fleet_id FROM $fleet_backup)" >/dev/null

clear_target_fleets() {
  db_query "DELETE q FROM uni1_queue q JOIN uni1_fleet f ON f.fleet_id=q.sub_id WHERE q.type='Fleet' AND (f.start_planet=$target_id OR f.target_planet=$target_id); DELETE FROM uni1_fleet WHERE start_planet=$target_id OR target_planet=$target_id" >/dev/null
}

restore_original() {
  clear_target_fleets
  db_query "INSERT INTO uni1_fleet SELECT * FROM $fleet_backup; INSERT INTO uni1_queue SELECT * FROM $queue_backup; UPDATE uni1_planets p JOIN $planet_backup b ON b.planet_id=p.planet_id SET p.owner_id=b.owner_id,p.type=b.type,p.\`42\`=b.\`42\`,p.\`702\`=b.\`702\`,p.lastpeek=b.lastpeek; UPDATE uni1_users u JOIN $user_backup b ON b.player_id=u.player_id SET u.session=b.session,u.private_session=b.private_session,u.lastlogin=b.lastlogin,u.lastclick=b.lastclick,u.ip_addr=b.ip_addr,u.aktplanet=b.aktplanet,u.vacation=b.vacation,u.vacation_until=b.vacation_until,u.banned=b.banned,u.banned_until=b.banned_until,u.noattack=b.noattack,u.noattack_until=b.noattack_until,u.disable=b.disable,u.disable_until=b.disable_until,u.validated=b.validated,u.deact_ip=b.deact_ip; DROP TABLE IF EXISTS $planet_backup,$user_backup,$fleet_backup,$queue_backup" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

reset_case() {
  clear_target_fleets
  db_query "UPDATE uni1_planets SET \`42\`=3,\`702\`=20000,lastpeek=1 WHERE planet_id=$source_id; UPDATE uni1_planets SET \`42\`=0,\`702\`=20000,lastpeek=1 WHERE planet_id=$home_id; UPDATE uni1_planets SET \`42\`=3,\`702\`=4000,lastpeek=1 WHERE planet_id=$low_source_id; UPDATE uni1_users SET session='',private_session='',vacation=0,vacation_until=0,banned=0,banned_until=0,noattack=0,noattack_until=0,disable=0,disable_until=0,validated=1,deact_ip=1 WHERE player_id IN ($users)" >/dev/null
}

seed_fleet() {
  owner="$1" mission="$2" start_planet="$3" target_planet="$4" deploy_time="$5" union_id="$6"
  db_query "INSERT INTO uni1_fleet (owner_id,union_id,mission,start_planet,target_planet,flight_time,deploy_time,\`202\`) VALUES ($owner,$union_id,$mission,$start_planet,$target_planet,3600,$deploy_time,1); SET @fleet_id=LAST_INSERT_ID(); INSERT INTO uni1_queue (owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES ($owner,'Fleet',@fleet_id,0,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP()+3600,1200+$mission); SELECT @fleet_id" >/dev/null
}

configure_case() {
  mode="$1"
  reset_case
  case "$mode" in
    missing) case_login="$source_login"; case_source="$home_id"; case_target="$target_id" ;;
    insufficient) case_login="$low_login"; case_source="$low_source_id"; case_target="$target_id" ;;
    own) case_login="$source_login"; case_source="$source_id"; case_target="$home_id" ;;
    range) case_login="$source_login"; case_source="$source_id"; case_target="$far_target_id" ;;
    empty) case_login="$source_login"; case_source="$source_id"; case_target="$target_id" ;;
    outbound) case_login="$source_login"; case_source="$source_id"; case_target="$target_id"; seed_fleet "$target_owner" 3 "$target_id" "$home_id" 0 0 ;;
    inbound) case_login="$source_login"; case_source="$source_id"; case_target="$target_id"; seed_fleet "$source_owner" 3 "$home_id" "$target_id" 0 0 ;;
    return-hidden) case_login="$source_login"; case_source="$source_id"; case_target="$target_id"; seed_fleet "$source_owner" 103 "$home_id" "$target_id" 0 0 ;;
    deploy-hidden) case_login="$source_login"; case_source="$source_id"; case_target="$target_id"; seed_fleet "$target_owner" 4 "$target_id" "$home_id" 0 0 ;;
    acs-hold) case_login="$source_login"; case_source="$source_id"; case_target="$target_id"; seed_fleet "$source_owner" 5 "$home_id" "$target_id" 600 0 ;;
    acs-union)
      case_login="$source_login"; case_source="$source_id"; case_target="$target_id"
      seed_fleet "$source_owner" 21 "$home_id" "$target_id" 0 987654
      seed_fleet "$low_owner" 2 "$low_source_id" "$target_id" 0 987654
      ;;
  esac
}

login_legacy() {
  prefix="$1"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" --output "$TMP_DIR/$prefix-login.body" --cookie-jar "$TMP_DIR/$prefix.cookies" --request POST --data-urlencode "login=$case_login" --data-urlencode "pass=$PASSWORD" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/reg/login2.php")"
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/$prefix-login.headers")"
  legacy_session="$(printf '%s' "$location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 302 ] && [ -n "$legacy_session" ]
}

login_go() {
  prefix="$1"
  payload="$(jq -nc --arg login "$case_login" --arg pass "$PASSWORD" '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" --output "$TMP_DIR/$prefix-login.body" --header 'Content-Type: application/json' --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  go_cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/$prefix-login.headers" | cut -d';' -f1 | tr -d '\r')"
  redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/$prefix-login.body")"
  go_session="$(printf '%s' "$redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 200 ] && [ -n "$go_cookie" ] && [ -n "$go_session" ]
}

source_state() {
  db_query "SELECT ROUND(\`702\`),lastpeek FROM uni1_planets WHERE planet_id=$case_source"
}

normalize_legacy() {
  body="$1" status="$2" before_deut="$3"
  issue=""
  grep -q 'No cheating!' "$body" && issue="missing_sensor"
  grep -q 'Not enough deuterium!' "$body" && issue="insufficient_deuterium"
  grep -q 'attempting to manipulate a phalanx' "$body" && issue="forbidden"
  classes="$(sed -n "s/.*<tr class='\([^']*\)'>.*/\1/p" "$body" | awk '{if ($0=="") print "group"; else print $0}' | paste -sd, -)"
  group_sizes=""
  if printf '%s' "$classes" | grep -q 'group'; then
    group_sizes="$(grep -o 'Mission:' "$body" | wc -l | tr -d ' ')"
  fi
  after="$(source_state)"; after_deut="$(printf '%s' "$after" | cut -f1)"; lastpeek="$(printf '%s' "$after" | cut -f2)"
  fleet_count="$(db_query "SELECT COUNT(*) FROM uni1_fleet WHERE start_planet=$target_id OR target_planet=$target_id")"
  queue_count="$(db_query "SELECT COUNT(*) FROM uni1_queue WHERE type='Fleet' AND sub_id IN (SELECT fleet_id FROM uni1_fleet WHERE start_planet=$target_id OR target_planet=$target_id)")"
  report=false; events_heading=false
  grep -q 'Sensor report from the moon on the coordinates' "$body" && report=true || true
  grep -q 'Fleet movements' "$body" && events_heading=true || true
  changed=false; [ "$lastpeek" -gt 1 ] && changed=true
  jq -ncS --arg status "$status" --arg issue "$issue" --arg delta "$((after_deut-before_deut))" --arg classes "$classes" --arg groups "$group_sizes" --argjson changed "$changed" --argjson report "$report" --argjson eventsHeading "$events_heading" --arg fleets "$fleet_count" --arg queues "$queue_count" '{status:($status|tonumber),issue:$issue,deuteriumDelta:($delta|tonumber),lastpeekChanged:$changed,eventClasses:$classes,groupSizes:$groups,reportHeading:$report,eventsHeading:$eventsHeading,fleetCount:($fleets|tonumber),queueCount:($queues|tonumber)}'
}

normalize_go() {
  body="$1" status="$2" before_deut="$3"
  issue="$(jq -r '.phalanx.actionIssue.code // ""' "$body")"
  classes="$(jq -r '[.phalanx.events[]? | if ((.groupMissions // []) | length) > 0 then "group" elif .mission >= 200 then "holding" elif .mission >= 100 then "return" else "flight" end] | join(",")' "$body")"
  group_sizes="$(jq -r '[.phalanx.events[]? | select(((.groupMissions // []) | length) > 0) | ((.groupMissions | length) | tostring)] | join(",")' "$body")"
  after="$(source_state)"; after_deut="$(printf '%s' "$after" | cut -f1)"; lastpeek="$(printf '%s' "$after" | cut -f2)"
  fleet_count="$(db_query "SELECT COUNT(*) FROM uni1_fleet WHERE start_planet=$target_id OR target_planet=$target_id")"
  queue_count="$(db_query "SELECT COUNT(*) FROM uni1_queue WHERE type='Fleet' AND sub_id IN (SELECT fleet_id FROM uni1_fleet WHERE start_planet=$target_id OR target_planet=$target_id)")"
  report="$(jq -e '.phalanx.reportHeading == "Sensor report from the moon on the coordinates"' "$body" >/dev/null && printf true || printf false)"
  events_heading="$(jq -e '.phalanx.eventsHeading == "Fleet movements"' "$body" >/dev/null && printf true || printf false)"
  changed=false; [ "$lastpeek" -gt 1 ] && changed=true
  jq -ncS --arg status "$status" --arg issue "$issue" --arg delta "$((after_deut-before_deut))" --arg classes "$classes" --arg groups "$group_sizes" --argjson changed "$changed" --argjson report "$report" --argjson eventsHeading "$events_heading" --arg fleets "$fleet_count" --arg queues "$queue_count" '{status:($status|tonumber),issue:$issue,deuteriumDelta:($delta|tonumber),lastpeekChanged:$changed,eventClasses:$classes,groupSizes:$groups,reportHeading:$report,eventsHeading:$eventsHeading,fleetCount:($fleets|tonumber),queueCount:($queues|tonumber)}'
}

run_side() {
  side="$1" mode="$2"
  configure_case "$mode"
  before="$(source_state)"; before_deut="$(printf '%s' "$before" | cut -f1)"
  if [ "$side" = legacy ]; then
    login_legacy "$side-$mode"
    status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$mode.body" --cookie "$TMP_DIR/$side-$mode.cookies" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=phalanx&session=$legacy_session&cp=$case_source&spid=$case_target")"
    normalized="$(normalize_legacy "$TMP_DIR/$side-$mode.body" "$status" "$before_deut")"
  else
    login_go "$side-$mode"
    status="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$mode.body" --cookie "$go_cookie" --write-out '%{http_code}' "$GO_BASE_URL/api/game/phalanx?session=$go_session&cp=$case_source&spid=$case_target")"
    normalized="$(normalize_go "$TMP_DIR/$side-$mode.body" "$status" "$before_deut")"
  fi
}

case_results="$TMP_DIR/cases.jsonl"
: > "$case_results"
all_pass=true
for mode in missing insufficient own range empty outbound inbound return-hidden deploy-hidden acs-hold acs-union; do
  run_side legacy "$mode"; legacy="$normalized"
  run_side go "$mode"; go="$normalized"
  pass=true; [ "$legacy" = "$go" ] || pass=false
  [ "$(printf '%s' "$legacy" | jq -r '.status')" = 200 ] || pass=false
  [ "$(printf '%s' "$legacy" | jq -r '.reportHeading and .eventsHeading')" = true ] || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "phalanx-$mode" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$case_results"
done

jq -s --argjson pass "$all_pass" '{pass:$pass,cases:.}' "$case_results" > "$REPORT"
[ "$all_pass" = true ]
printf 'Go/PHP phalanx differential E2E: PASS (11 cases)\n'
