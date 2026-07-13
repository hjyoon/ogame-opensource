#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_ADMIN_BANS_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-admin-bans-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/admin-bans-differential.XXXXXX")"

admin_id="$(jq -r '.player_id // 0' "$FIXTURE")"
admin_planet="$(jq -r '.home_planet_id // 0' "$FIXTURE")"
operator_id="$(jq -r '.operator.player_id // 0' "$FIXTURE")"
target_id="$(jq -r '.admin_operations.target_player_id // 0' "$FIXTURE")"
regular_id="$(jq -r '.message_send.sender.player_id // 0' "$FIXTURE")"
[ "$admin_id" -gt 0 ] && [ "$operator_id" -gt 0 ] && [ "$target_id" -gt 0 ] && [ "$regular_id" -gt 0 ]

db_query() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

users_backup="uni1_e2e_admban_users_$$"
ranks_backup="uni1_e2e_admban_ranks_$$"
queue_backup="uni1_e2e_admban_queue_$$"
pranger_backup="uni1_e2e_admban_pranger_$$"
ids="$admin_id,$operator_id,$target_id,$regular_id"
db_query "DROP TABLE IF EXISTS $users_backup,$ranks_backup,$queue_backup,$pranger_backup;
CREATE TABLE $users_backup AS SELECT * FROM uni1_users WHERE player_id IN ($ids);
CREATE TABLE $ranks_backup AS SELECT player_id,score1,score2,score3,place1,place2,place3 FROM uni1_users;
CREATE TABLE $queue_backup AS SELECT * FROM uni1_queue WHERE owner_id IN ($ids);
CREATE TABLE $pranger_backup AS SELECT * FROM uni1_pranger WHERE user_id=$target_id OR admin_id IN ($admin_id,$operator_id,$regular_id)" >/dev/null

restore_case() {
  db_query "UPDATE uni1_users u JOIN $ranks_backup b ON b.player_id=u.player_id SET
u.score1=b.score1,u.score2=b.score2,u.score3=b.score3,u.place1=b.place1,u.place2=b.place2,u.place3=b.place3;
DELETE FROM uni1_users WHERE player_id IN ($ids); INSERT INTO uni1_users SELECT * FROM $users_backup;
DELETE FROM uni1_queue WHERE owner_id IN ($ids); INSERT INTO uni1_queue SELECT * FROM $queue_backup;
DELETE FROM uni1_pranger WHERE user_id=$target_id OR admin_id IN ($admin_id,$operator_id,$regular_id);
INSERT INTO uni1_pranger SELECT * FROM $pranger_backup" >/dev/null
}

restore_original() {
  restore_case
  db_query "DROP TABLE IF EXISTS $users_backup,$ranks_backup,$queue_backup,$pranger_backup" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

admin_login="$(db_query "SELECT name FROM $users_backup WHERE player_id=$admin_id")"
operator_login="$(db_query "SELECT name FROM $users_backup WHERE player_id=$operator_id")"
regular_login="$(db_query "SELECT name FROM $users_backup WHERE player_id=$regular_id")"

configure_case() {
  case_name="$1"; restore_case
  actor_id=$operator_id; actor_login=$operator_login; actor_planet=$admin_planet
  ban_mode=0; days=2; hours=3; reason="Differential admin ban"; include_target=1
  db_query "UPDATE uni1_users SET admin=0,banned=0,banned_until=0,noattack=0,noattack_until=0,vacation=0,vacation_until=0 WHERE player_id=$target_id;
DELETE FROM uni1_queue WHERE owner_id=$target_id AND type IN ('UnbanPlayer','AllowAttacks')" >/dev/null
  case "$case_name" in
    regular-denied) actor_id=$regular_id; actor_login=$regular_login; ban_mode=1 ;;
    admin-no-vacation) actor_id=$admin_id; actor_login=$admin_login ;;
    operator-vacation) ban_mode=1 ;;
    operator-attack) ban_mode=2 ;;
    operator-unban)
      ban_mode=3
      db_query "UPDATE uni1_users SET banned=1,banned_until=UNIX_TIMESTAMP()+7200,score1=0,score2=0,score3=0 WHERE player_id=$target_id;
INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES($target_id,'UnbanPlayer',0,0,0,UNIX_TIMESTAMP(),2*UNIX_TIMESTAMP()+7200,0)" >/dev/null
      ;;
    operator-unban-attack)
      ban_mode=4
      db_query "UPDATE uni1_users SET noattack=1,noattack_until=UNIX_TIMESTAMP()+7200 WHERE player_id=$target_id;
INSERT INTO uni1_queue(owner_id,type,sub_id,obj_id,level,start,end,prio) VALUES($target_id,'AllowAttacks',0,0,0,UNIX_TIMESTAMP(),2*UNIX_TIMESTAMP()+7200,0)" >/dev/null
      ;;
    negative-duration) days=-1; hours=-2 ;;
    no-target) include_target=0 ;;
  esac
  actor_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$actor_id")"
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

capture_state() {
  user="$(db_query "SELECT JSON_OBJECT('banned',banned,'banDelay',IF(banned_until=0,0,CAST(banned_until AS SIGNED)-CAST(UNIX_TIMESTAMP() AS SIGNED)),
'noAttack',noattack,'attackDelay',IF(noattack_until=0,0,CAST(noattack_until AS SIGNED)-CAST(UNIX_TIMESTAMP() AS SIGNED)),'vacation',vacation,
'vacationDelay',IF(vacation_until=0,0,CAST(vacation_until AS SIGNED)-CAST(UNIX_TIMESTAMP() AS SIGNED)),'score1',score1,'score2',score2,'score3',score3,
'place1',place1,'place2',place2,'place3',place3) FROM uni1_users WHERE player_id=$target_id")"
  queues="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('type',type,'duration',CAST(end AS SIGNED)-2*CAST(start AS SIGNED))))
FROM (SELECT * FROM uni1_queue WHERE owner_id=$target_id AND type IN ('UnbanPlayer','AllowAttacks') ORDER BY type,task_id) q")"
  pranger="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('adminId',admin_id,'userId',user_id,
'whenDelay',CAST(ban_when AS SIGNED)-CAST(UNIX_TIMESTAMP() AS SIGNED),'untilDelay',CAST(ban_until AS SIGNED)-CAST(UNIX_TIMESTAMP() AS SIGNED),'reason',reason)))
FROM (SELECT * FROM uni1_pranger WHERE user_id=$target_id ORDER BY ban_id) p")"
  ranks="$(db_query "SET SESSION group_concat_max_len=16777216;
SELECT JSON_OBJECT('count',COUNT(*),'sha256',SHA2(GROUP_CONCAT(CONCAT_WS(':',player_id,score1,score2,score3,place1,place2,place3) ORDER BY player_id SEPARATOR '|'),256)) FROM uni1_users")"
  jq -ncS --argjson user "$user" --argjson queues "$queues" --argjson pranger "$pranger" --argjson ranks "$ranks" '
    def normalize_delay: if . == 0 then 0 elif . >= -3 and . <= 3 then 0 elif . > 0 then ((./3600)|round)*3600 else ((./3600)|round)*3600 end;
    {user:($user | .banDelay |= normalize_delay | .attackDelay |= normalize_delay | .vacationDelay |= normalize_delay),
     queues:($queues | map(.duration |= normalize_delay)),
     pranger:($pranger | map(.whenDelay |= normalize_delay | .untilDelay |= normalize_delay)),ranks:$ranks}'
}

run_side() {
  side="$1"; case_name="$2"; configure_case "$case_name"
  if [ "$side" = legacy ]; then
    login_legacy "$side-$case_name"
    form="banmode=$ban_mode&days=$days&hours=$hours&reason=$(jq -rn --arg value "$reason" '$value|@uri')"
    [ "$include_target" = 1 ] && form="$form&id%5B$target_id%5D=on"
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/$side-$case_name.body" --cookie "$TMP_DIR/$side-$case_name.cookies" \
      --request POST --data "$form" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$actor_planet&mode=Bans&action=ban")"
  else
    login_go "$side-$case_name"
    targets='[]'; [ "$include_target" = 1 ] && targets="[$target_id]"
    payload="$(jq -nc --argjson targets "$targets" --argjson mode "$ban_mode" --argjson days "$days" --argjson hours "$hours" --arg reason "$reason" \
      '{action:"ban",targetIds:$targets,banMode:$mode,days:$days,hours:$hours,reason:$reason}')"
    http="$(curl --silent --show-error --max-time 20 --output "$TMP_DIR/$side-$case_name.body" --header 'Content-Type: application/json' \
      --cookie "$cookie" --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/game/admin?session=$session&cp=$actor_planet&mode=Bans")"
  fi
  state="$(capture_state)"
  issue=""; [ "$side" = go ] && issue="$(jq -r '.actionIssue.code // empty' "$TMP_DIR/$side-$case_name.body" 2>/dev/null || true)"
  normalized="$(jq -ncS --argjson http "$http" --arg issue "$issue" --argjson state "$state" '{http:$http,issue:$issue,state:$state}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true
cases="regular-denied admin-no-vacation operator-vacation operator-attack operator-unban operator-unban-attack negative-duration no-target"
for case_name in $cases; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.state')" = "$(printf '%s' "$go" | jq -cS '.state')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200 or .http==302' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200' >/dev/null || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "admin-bans-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" '{pass:$pass,normalization:"sessions, generated IDs and wall-clock ban deadlines reduce to stable contracts; user, queue, pranger, score and rank effects remain exact",cases:.}' "$results" > "$REPORT"
[ "$all_pass" = true ]
case_count="$(printf '%s\n' $cases | wc -l | tr -d ' ')"
printf 'Go/PHP admin bans differential E2E: PASS (%s cases)\n' "$case_count"
