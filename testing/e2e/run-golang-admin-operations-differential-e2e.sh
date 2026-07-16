#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_ADMIN_OPERATIONS_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-admin-operations-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/admin-operations-differential.XXXXXX")"

admin_id="$(jq -r '.player_id // 0' "$FIXTURE")"
operator_id="$(jq -r '.operator.player_id // 0' "$FIXTURE")"
target_id="$(jq -r '.admin_operations.target_player_id // 0' "$FIXTURE")"
regular_id="$(jq -r '.message_send.sender.player_id // 0' "$FIXTURE")"
[ "$admin_id" -gt 0 ] && [ "$operator_id" -gt 0 ] && [ "$target_id" -gt 0 ] && [ "$regular_id" -gt 0 ]

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

users_backup="uni1_e2e_admop_users_$$"
messages_backup="uni1_e2e_admop_messages_$$"
reports_backup="uni1_e2e_admop_reports_$$"
expedition_backup="uni1_e2e_admop_exptab_$$"
recipient_ids="$operator_id,$target_id"

db_query "DROP TABLE IF EXISTS $users_backup,$messages_backup,$reports_backup,$expedition_backup;
CREATE TABLE $users_backup AS SELECT player_id,admin,score1,place1 FROM uni1_users;
CREATE TABLE $messages_backup AS SELECT * FROM uni1_messages WHERE owner_id IN ($recipient_ids);
CREATE TABLE $reports_backup AS SELECT * FROM uni1_reports;
CREATE TABLE $expedition_backup AS SELECT * FROM uni1_exptab" >/dev/null

restore_case() {
  db_query "UPDATE uni1_users u JOIN $users_backup b ON b.player_id=u.player_id SET u.admin=b.admin,u.score1=b.score1,u.place1=b.place1;
DELETE FROM uni1_messages WHERE owner_id IN ($recipient_ids); INSERT INTO uni1_messages SELECT * FROM $messages_backup;
DELETE FROM uni1_reports; INSERT INTO uni1_reports SELECT * FROM $reports_backup;
DELETE FROM uni1_exptab; INSERT INTO uni1_exptab SELECT * FROM $expedition_backup" >/dev/null
}

cleanup() {
  restore_case >/dev/null 2>&1 || true
  db_query "DROP TABLE IF EXISTS $users_backup,$messages_backup,$reports_backup,$expedition_backup" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

admin_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$admin_id")"
admin_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$admin_id")"
operator_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$operator_id")"
operator_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$operator_id")"
regular_login="$(db_query "SELECT name FROM uni1_users WHERE player_id=$regular_id")"
regular_planet="$(db_query "SELECT hplanetid FROM uni1_users WHERE player_id=$regular_id")"

full_expedition_values='{"dm_factor":7,"chance_success":91,"depleted_min":3,"depleted_med":8,"depleted_max":13,"chance_depleted_min":17,"chance_depleted_med":23,"chance_depleted_max":29,"chance_alien":31,"chance_pirates":37,"chance_dm":41,"chance_lost":43,"chance_delay":47,"chance_accel":53,"chance_res":59,"chance_fleet":61,"score_cap1":101,"limit_cap1":201,"score_cap2":102,"limit_cap2":202,"score_cap3":103,"limit_cap3":203,"score_cap4":104,"limit_cap4":204,"score_cap5":105,"limit_cap5":205,"score_cap6":106,"limit_cap6":206,"score_cap7":107,"limit_cap7":207,"score_cap8":108,"limit_cap8":208,"limit_max":999}'
partial_expedition_values='{"dm_factor":-7,"chance_success":101}'

json_form() {
  printf '%s' "$1" | jq -r 'to_entries|map((.key|@uri)+"="+(.value|tostring|@uri))|join("&")'
}

seed_reports() {
  count="$1"
  db_query "DELETE FROM uni1_reports;
INSERT INTO uni1_reports(owner_id,msg_id,msgfrom,subj,text,date)
SELECT $target_id,n,CONCAT('From ',LPAD(n,3,'0')),CONCAT('ADM-DIFF-',LPAD(n,3,'0')),CONCAT('Body ',LPAD(n,3,'0')),100000+n
FROM (SELECT ROW_NUMBER() OVER () n FROM information_schema.columns LIMIT $count) x" >/dev/null
}

configure_case() {
  case_name="$1"
  restore_case
  db_query "UPDATE uni1_users SET admin=0,score1=5000,place1=100;
UPDATE uni1_users SET admin=2 WHERE player_id=$admin_id;
UPDATE uni1_users SET admin=1 WHERE player_id=$operator_id" >/dev/null
  actor_login=$operator_login; actor_planet=$operator_planet; mode=Broadcast; action=broadcast_send
  category=3; subject="ADM-DIFF $case_name"; body="Body $case_name"; report_ids='[]'; delete_mode=deletemarked; values='{}'
  case_epoch="$(db_query 'SELECT UNIX_TIMESTAMP()')"

  case "$case_name" in
    broadcast-operator) ;;
    broadcast-noob)
      category=1
      db_query "UPDATE uni1_users SET score1=4999 WHERE player_id=$target_id" >/dev/null
      ;;
    broadcast-top)
      category=2
      db_query "UPDATE uni1_users SET place1=99 WHERE player_id=$target_id" >/dev/null
      ;;
    broadcast-empty-subject) subject="" ;;
    broadcast-empty-text) body="" ;;
    broadcast-whitespace) subject="   "; body="Whitespace body" ;;
    broadcast-bbcode)
      body="[b]Bold[/b] [i]Italic[/i] [url=http://example.com]Link[/url] \"quote\" 'tick"
      ;;
    broadcast-bbcode-extended)
      body="[u]Under[/u] [s]Strike[/s] [sub]Low[/sub] [sup]High[/sup] [color=red]Red[/color] [font=Arial color=blue size=5]Font[/font] [size=8]Big[/size] [email=test@example.com]Mail[/email] [align=center]Center[/align] [hr] [img width=16 height=12 border=0]http://example.com/a.png[/img] [quote=Admin]Quoted[/quote]"
      ;;
    broadcast-overflow)
      category=1
      db_query "UPDATE uni1_users SET score1=4999 WHERE player_id=$target_id;
DELETE FROM uni1_messages WHERE owner_id=$target_id;
INSERT INTO uni1_messages(owner_id,pm,msgfrom,subj,text,shown,date,planet_id)
SELECT $target_id,0,'seed',CONCAT('seed-',LPAD(n,3,'0')),'seed',0,n,0
FROM (SELECT ROW_NUMBER() OVER () n FROM information_schema.columns LIMIT 127) x" >/dev/null
      ;;
    reports-marked)
      mode=Reports; action=reports_delete; seed_reports 3
      ids="$(db_query "SELECT GROUP_CONCAT(id ORDER BY id SEPARATOR ',') FROM uni1_reports")"
      first="$(printf '%s' "$ids" | cut -d, -f1)"; third="$(printf '%s' "$ids" | cut -d, -f3)"
      report_ids="[$first,$third]"
      ;;
    reports-delete-all)
      mode=Reports; action=reports_delete; delete_mode=deleteall; seed_reports 55
      ;;
    reports-empty)
      mode=Reports; action=reports_delete; seed_reports 3
      ;;
    reports-regular-denied)
      mode=Reports; action=reports_delete; seed_reports 3
      actor_login=$regular_login; actor_planet=$regular_planet
      first="$(db_query 'SELECT id FROM uni1_reports ORDER BY id LIMIT 1')"; report_ids="[$first]"
      ;;
    expedition-full)
      actor_login=$admin_login; actor_planet=$admin_planet; mode=Expedition; action=settings; values=$full_expedition_values
      ;;
    expedition-partial)
      actor_login=$admin_login; actor_planet=$admin_planet; mode=Expedition; action=settings; values=$partial_expedition_values
      ;;
    expedition-operator-denied)
      mode=Expedition; action=settings; values=$partial_expedition_values
      ;;
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

capture_state() {
  messages="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('owner',owner_id,'pm',pm,'from',msgfrom,'subject',subj,'text',text,
'shown',shown,'dateAge',CAST(UNIX_TIMESTAMP() AS SIGNED)-CAST(date AS SIGNED),'planet',planet_id)))
FROM (SELECT * FROM uni1_messages WHERE owner_id IN ($recipient_ids) AND date >= $case_epoch-5 ORDER BY owner_id,msg_id) x")"
  message_counts="$(db_query "SELECT JSON_OBJECT('operator',SUM(owner_id=$operator_id),'target',SUM(owner_id=$target_id)) FROM uni1_messages WHERE owner_id IN ($recipient_ids)")"
  reports="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('owner',owner_id,'msgId',msg_id,'from',msgfrom,'subject',subj,'text',text,'date',date)))
FROM (SELECT * FROM uni1_reports WHERE subj LIKE 'ADM-DIFF-%' ORDER BY subj) x")"
  expedition="$(db_query "SELECT JSON_OBJECT('dm_factor',dm_factor,'chance_success',chance_success,'depleted_min',depleted_min,'depleted_med',depleted_med,
'depleted_max',depleted_max,'chance_depleted_min',chance_depleted_min,'chance_depleted_med',chance_depleted_med,'chance_depleted_max',chance_depleted_max,
'chance_alien',chance_alien,'chance_pirates',chance_pirates,'chance_dm',chance_dm,'chance_lost',chance_lost,'chance_delay',chance_delay,
'chance_accel',chance_accel,'chance_res',chance_res,'chance_fleet',chance_fleet,'score_cap1',score_cap1,'limit_cap1',limit_cap1,
'score_cap2',score_cap2,'limit_cap2',limit_cap2,'score_cap3',score_cap3,'limit_cap3',limit_cap3,'score_cap4',score_cap4,'limit_cap4',limit_cap4,
'score_cap5',score_cap5,'limit_cap5',limit_cap5,'score_cap6',score_cap6,'limit_cap6',limit_cap6,'score_cap7',score_cap7,'limit_cap7',limit_cap7,
'score_cap8',score_cap8,'limit_cap8',limit_cap8,'limit_max',limit_max) FROM uni1_exptab LIMIT 1")"
  jq -ncS --argjson messages "$messages" --argjson messageCounts "$message_counts" --argjson reports "$reports" --argjson expedition "$expedition" '
    def stable_age: if . >= -5 and . <= 5 then 0 else . end;
    {messages:($messages|map(.dateAge|=stable_age)),messageCounts:$messageCounts,reports:$reports,expedition:$expedition}'
}

run_side() {
  side="$1"; case_name="$2"; configure_case "$case_name"
  if [ "$side" = legacy ]; then
    login_legacy "$side-$case_name"
    case "$mode" in
      Broadcast)
        form="cat=$category&subj=$(jq -rn --arg value "$subject" '$value|@uri')&text=$(jq -rn --arg value "$body" '$value|@uri')"
        url="$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$actor_planet&mode=Broadcast"
        ;;
      Reports)
        form="deletemessages=$delete_mode"
        for id in $(printf '%s' "$report_ids" | jq -r '.[]'); do form="$form&delmes$id=on"; done
        url="$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$actor_planet&mode=Reports"
        ;;
      Expedition)
        form="$(json_form "$values")"
        url="$LEGACY_BASE_URL/game/index.php?page=admin&session=$session&cp=$actor_planet&mode=Expedition&action=settings"
        ;;
    esac
    http="$(curl --silent --show-error --max-time 30 --output "$TMP_DIR/$side-$case_name.body" --cookie "$TMP_DIR/$side-$case_name.cookies" \
      --request POST --data "$form" --write-out '%{http_code}' "$url")"
    issue=""
  else
    login_go "$side-$case_name"
    case "$mode" in
      Broadcast) payload="$(jq -nc --arg action "$action" --argjson category "$category" --arg subject "$subject" --arg text "$body" '{action:$action,category:$category,subject:$subject,text:$text}')" ;;
      Reports) payload="$(jq -nc --arg action "$action" --argjson reportIds "$report_ids" --arg deleteMode "$delete_mode" '{action:$action,reportIds:$reportIds,deleteMode:$deleteMode}')" ;;
      Expedition) payload="$(jq -nc --arg action "$action" --argjson values "$values" '{action:$action,values:$values}')" ;;
    esac
    http="$(curl --silent --show-error --max-time 30 --output "$TMP_DIR/$side-$case_name.body" --header 'Content-Type: application/json' \
      --cookie "$cookie" --data "$payload" --write-out '%{http_code}' "$GO_BASE_URL/api/game/admin?session=$session&cp=$actor_planet&mode=$mode")"
    issue="$(jq -r '.actionIssue.code // empty' "$TMP_DIR/$side-$case_name.body" 2>/dev/null || true)"
  fi
  state="$(capture_state)"
  normalized="$(jq -ncS --argjson http "$http" --arg issue "$issue" --argjson state "$state" '{http:$http,issue:$issue,state:$state}')"
}

results="$TMP_DIR/results.jsonl"; : > "$results"; all_pass=true
cases="broadcast-operator broadcast-noob broadcast-top broadcast-empty-subject broadcast-empty-text broadcast-whitespace broadcast-bbcode broadcast-bbcode-extended broadcast-overflow reports-marked reports-delete-all reports-empty reports-regular-denied expedition-full expedition-partial expedition-operator-denied"
for case_name in $cases; do
  run_side legacy "$case_name"; legacy="$normalized"
  run_side go "$case_name"; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.state')" = "$(printf '%s' "$go" | jq -cS '.state')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200 or .http==302' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200' >/dev/null || pass=false
  case "$case_name" in
    *-denied) [ "$(printf '%s' "$go" | jq -r '.issue')" = access_denied ] || pass=false ;;
    *) [ "$(printf '%s' "$go" | jq -r '.issue')" = action_saved ] || pass=false ;;
  esac
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "admin-$case_name" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" \
    '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" \
  '{pass:$pass,normalization:"generated IDs and bounded request timestamps only; recipients, message/report content, retention and all expedition settings remain exact",cases:.}' \
  "$results" > "$REPORT"
[ "$all_pass" = true ]
case_count="$(printf '%s\n' $cases | wc -l | tr -d ' ')"
printf 'Go/PHP admin operations differential E2E: PASS (%s cases)\n' "$case_count"
