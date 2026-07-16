#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
FIXTURE="${OGAME_DIFFERENTIAL_FIXTURE:-$ROOT_DIR/.tmp/golang-smoke-fixture.json}"
REPORT="${OGAME_ALLIANCE_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-alliance-differential.json}"
PASSWORD="${OGAME_GO_LOGIN_SMOKE_PASS:-admin}"

[ -f "$FIXTURE" ] || { printf 'Missing fixture: %s\n' "$FIXTURE" >&2; exit 1; }
command -v jq >/dev/null
mkdir -p "$ROOT_DIR/.tmp" "$(dirname -- "$REPORT")"
TMP_DIR="$(mktemp -d "$ROOT_DIR/.tmp/alliance-differential.XXXXXX")"

founder_id="$(jq -r '.buddy_lifecycle.requester.player_id // 0' "$FIXTURE")"
founder_login="$(jq -r '.buddy_lifecycle.requester.login // empty' "$FIXTURE")"
founder_planet="$(jq -r '.buddy_lifecycle.requester.home_planet_id // 0' "$FIXTURE")"
member_id="$(jq -r '.buddy_lifecycle.recipient.player_id // 0' "$FIXTURE")"
member_login="$(jq -r '.buddy_lifecycle.recipient.login // empty' "$FIXTURE")"
member_planet="$(jq -r '.buddy_lifecycle.recipient.home_planet_id // 0' "$FIXTURE")"
[ "$founder_id" -gt 0 ] && [ "$member_id" -gt 0 ] && [ "$founder_planet" -gt 0 ] && [ "$member_planet" -gt 0 ]
[ -n "$founder_login" ] && [ -n "$member_login" ]
users="$founder_id,$member_id"
tag=E2EADIF
name='E2E Alliance Differential'

db_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -N -B -r -uroot uni -e "$1"' sh "$1"
}

user_backup="uni1_e2e_allydiff_users_$$"
message_backup="uni1_e2e_allydiff_messages_$$"
app_backup="uni1_e2e_allydiff_apps_$$"
owner_backup="uni1_e2e_allydiff_owners_$$"
db_query "DROP TABLE IF EXISTS $user_backup,$message_backup,$app_backup,$owner_backup;
CREATE TABLE $user_backup AS SELECT * FROM uni1_users WHERE player_id IN ($users);
CREATE TABLE $message_backup AS SELECT * FROM uni1_messages WHERE owner_id IN ($users);
CREATE TABLE $app_backup AS SELECT * FROM uni1_allyapps WHERE player_id IN ($users);
CREATE TABLE $owner_backup AS SELECT ally_id,owner_id FROM uni1_ally" >/dev/null

clear_test_alliances() {
  ids="$(db_query "SELECT GROUP_CONCAT(ally_id) FROM uni1_ally WHERE tag LIKE 'E2EAD%'" || true)"
  if [ -n "$ids" ] && [ "$ids" != NULL ]; then
    db_query "UPDATE uni1_users SET ally_id=0,allyrank=0,joindate=0 WHERE ally_id IN ($ids); DELETE FROM uni1_allyapps WHERE ally_id IN ($ids); DELETE FROM uni1_allyranks WHERE ally_id IN ($ids); DELETE FROM uni1_ally WHERE ally_id IN ($ids)" >/dev/null
  fi
}

clear_case() {
  clear_test_alliances
  db_query "DELETE FROM uni1_allyapps WHERE player_id IN ($users); DELETE FROM uni1_messages WHERE owner_id IN ($users); UPDATE uni1_ally a JOIN $owner_backup b ON b.ally_id=a.ally_id SET a.owner_id=b.owner_id" >/dev/null
}

restore_original() {
  clear_case
  db_query "DELETE FROM uni1_users WHERE player_id IN ($users); INSERT INTO uni1_users SELECT * FROM $user_backup; INSERT INTO uni1_messages SELECT * FROM $message_backup; INSERT INTO uni1_allyapps SELECT * FROM $app_backup; DROP TABLE IF EXISTS $user_backup,$message_backup,$app_backup,$owner_backup" >/dev/null
}

cleanup() {
  restore_original >/dev/null 2>&1 || true
  if [ "${OGAME_KEEP_ALLIANCE_DIFFERENTIAL_TMP:-0}" = 1 ]; then
    printf 'Alliance differential temp: %s\n' "$TMP_DIR" >&2
  else
    rm -rf "$TMP_DIR"
  fi
}
trap cleanup EXIT INT TERM

reset_case() {
  clear_case
  db_query "UPDATE uni1_users SET session='',private_session='',lang='en',admin=0,validated=1,deact_ip=1,ally_id=0,allyrank=0,joindate=0,useskin=1,skin='/evolution/',vacation=0,vacation_until=0,banned=0,banned_until=0,noattack=0,noattack_until=0,disable=0,disable_until=0 WHERE player_id IN ($users)" >/dev/null
}

seed_alliance() {
  include_member="${1:-1}"; member_rank="${2:-1}"; member_rights="${3:-0}"
  case_ally_id="$(db_query "INSERT INTO uni1_ally (tag,name,owner_id,homepage,imglogo,open,insertapp,exttext,inttext,apptext,nextrank,old_tag,old_name,tag_until,name_until,score1,score2,score3,place1,place2,place3,oldscore1,oldscore2,oldscore3,oldplace1,oldplace2,oldplace3,scoredate) VALUES ('$tag','$name',$founder_id,'','',1,0,'Welcome to the alliance page','','',3,'','',0,0,0,0,0,0,0,0,0,0,0,0,0,0,0); SELECT LAST_INSERT_ID()")"
  db_query "INSERT INTO uni1_allyranks (rank_id,ally_id,name,rights) VALUES (0,$case_ally_id,'Founder',511),(1,$case_ally_id,'Newcomer',0),(2,$case_ally_id,'Officer',$member_rights); UPDATE uni1_users SET ally_id=$case_ally_id,allyrank=0,joindate=1000 WHERE player_id=$founder_id" >/dev/null
  if [ "$include_member" = 1 ]; then
    db_query "UPDATE uni1_users SET ally_id=$case_ally_id,allyrank=$member_rank,joindate=1001 WHERE player_id=$member_id" >/dev/null
  fi
}

seed_application() {
  app_text="${1:-Application text}"
  case_app_id="$(db_query "INSERT INTO uni1_allyapps (ally_id,player_id,text,date) VALUES ($case_ally_id,$member_id,'$app_text',1002); SELECT LAST_INSERT_ID()")"
}

login_legacy() {
  prefix="$1"; login="$2"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" --output "$TMP_DIR/$prefix-login.body" --cookie-jar "$TMP_DIR/$prefix.cookies" --request POST --data-urlencode "login=$login" --data-urlencode "pass=$PASSWORD" --write-out '%{http_code}' "$LEGACY_BASE_URL/game/reg/login2.php")"
  location="$(awk 'tolower($1)=="location:" {$1=""; sub(/^ /,""); gsub(/\r/,""); print; exit}' "$TMP_DIR/$prefix-login.headers")"
  session="$(printf '%s' "$location" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 302 ] && [ -n "$session" ]
  cookie_arg="--cookie $TMP_DIR/$prefix.cookies"
}

login_go() {
  prefix="$1"; login="$2"
  login_payload="$(jq -nc --arg login "$login" --arg pass "$PASSWORD" '{login:$login,pass:$pass,universe:"http://localhost:8888"}')"
  status="$(curl --silent --show-error --max-time 15 --dump-header "$TMP_DIR/$prefix-login.headers" --output "$TMP_DIR/$prefix-login.body" --header 'Content-Type: application/json' --data "$login_payload" --write-out '%{http_code}' "$GO_BASE_URL/api/public/login")"
  cookie="$(awk 'tolower($1)=="set-cookie:" {print $2; exit}' "$TMP_DIR/$prefix-login.headers" | cut -d';' -f1 | tr -d '\r')"
  redirect="$(jq -r '.session.redirectTo // empty' "$TMP_DIR/$prefix-login.body")"
  session="$(printf '%s' "$redirect" | sed -n 's/.*[?&]session=\([^&]*\).*/\1/p')"
  [ "$status" = 200 ] && [ -n "$cookie" ] && [ -n "$session" ]
  cookie_arg="--cookie $cookie"
}

capture_state() {
  alliances="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('tag',tag,'name',name,'owner',IF(owner_id=$founder_id,'founder',IF(owner_id=$member_id,'member','other')),'open',open,'insertApp',insertapp,'external',exttext,'internal',inttext,'application',apptext,'nextRank',nextrank,'oldTag',old_tag,'oldName',old_name,'tagLocked',IF(tag_until>UNIX_TIMESTAMP(),1,0),'nameLocked',IF(name_until>UNIX_TIMESTAMP(),1,0)))) FROM (SELECT * FROM uni1_ally WHERE tag LIKE 'E2EAD%' ORDER BY tag) a")"
  ranks="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('rank',rank_id,'name',name,'rights',rights))) FROM (SELECT r.* FROM uni1_allyranks r JOIN uni1_ally a ON a.ally_id=r.ally_id WHERE a.tag LIKE 'E2EAD%' ORDER BY r.rank_id) x")"
  user_state="$(db_query "SELECT JSON_ARRAYAGG(JSON_OBJECT('player',IF(player_id=$founder_id,'founder','member'),'allied',IF(ally_id>0,1,0),'rank',allyrank,'joined',IF(joindate>0,1,0))) FROM (SELECT * FROM uni1_users WHERE player_id IN ($users) ORDER BY player_id) u")"
  apps="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('player',IF(player_id=$founder_id,'founder','member'),'text',text))) FROM (SELECT * FROM uni1_allyapps WHERE player_id IN ($users) ORDER BY player_id,text) aa")"
  messages="$(db_query "SELECT IF(COUNT(*)=0,'[]',JSON_ARRAYAGG(JSON_OBJECT('owner',IF(owner_id=$founder_id,'founder','member'),'type',pm,'from',msgfrom,'subject',subj,'text',text,'shown',shown))) FROM (SELECT * FROM uni1_messages WHERE owner_id IN ($users) ORDER BY owner_id,subj,text,msg_id) m")"
  jq -ncS --argjson alliances "$alliances" --argjson ranks "$ranks" --argjson users "$user_state" --argjson apps "$apps" --argjson messages "$messages" '{alliances:$alliances,ranks:$ranks,users:$users,apps:$apps,messages:$messages}'
}

configure_case() {
  name_case="$1"; reset_case
  actor_login="$founder_login"; actor_planet="$founder_planet"; method=POST; page=allianzen; query=''; form=''; action=''; payload='{}'
  case "$name_case" in
    create)
      query='a=1&weiter=1'; form="tag=$tag&name=E2E+Alliance+Differential"; action=create; payload="$(jq -nc --arg tag "$tag" --arg name "$name" '{action:"create",tag:$tag,name:$name}')" ;;
    create-invalid)
      query='a=1&weiter=1'; form='tag=AB&name=No'; action=create; payload='{"action":"create","tag":"AB","name":"No"}' ;;
    apply|apply-closed|apply-unvalidated)
      seed_alliance 0; actor_login="$member_login"; actor_planet="$member_planet"; page=bewerben; query="allyid=$case_ally_id"; form='weiter=Submit&text=Application+text'; action=apply; payload="$(jq -nc --argjson id "$case_ally_id" '{action:"apply",allianceId:$id,text:"Application text"}')"
      [ "$name_case" != apply-closed ] || db_query "UPDATE uni1_ally SET open=0 WHERE ally_id=$case_ally_id" >/dev/null
      [ "$name_case" != apply-unvalidated ] || db_query "UPDATE uni1_users SET validated=0 WHERE player_id=$member_id" >/dev/null ;;
    withdraw)
      seed_alliance 0; seed_application; actor_login="$member_login"; actor_planet="$member_planet"; form='bcancel=Withdraw+application'; action=withdraw; payload="$(jq -nc --argjson id "$case_app_id" '{action:"withdraw",applicationId:$id}')" ;;
    accept|reject-default|reject-reason)
      seed_alliance 0; seed_application; page=bewerbungen; query="show=$case_app_id&sort=1"
      if [ "$name_case" = accept ]; then form='aktion=Accept&text='; action=accept; payload="$(jq -nc --argjson id "$case_app_id" '{action:"accept",applicationId:$id}')"
      else action=reject; reason=''; [ "$name_case" != reject-reason ] || reason='Not suitable'; form="aktion=Reject&text=$(printf '%s' "$reason" | sed 's/ /+/g')"; payload="$(jq -nc --argjson id "$case_app_id" --arg text "$reason" '{action:"reject",applicationId:$id,text:$text}')"; fi ;;
    leave)
      seed_alliance 1; actor_login="$member_login"; actor_planet="$member_planet"; query='a=3&weiter=1'; form='confirm=1'; action=leave; payload='{"action":"leave"}' ;;
    add-rank)
      seed_alliance 1; db_query "DELETE FROM uni1_allyranks WHERE ally_id=$case_ally_id AND rank_id=2; UPDATE uni1_ally SET nextrank=2 WHERE ally_id=$case_ally_id" >/dev/null; query='a=15'; form='newrangname=Right+Hand'; action=add_rank; payload='{"action":"add_rank","rankName":"Right Hand"}' ;;
    save-ranks)
      seed_alliance 1; query='a=15'; form='u2r1=on&u2r3=on&u2r8=on'; action=save_ranks; payload='{"action":"save_ranks","rankRights":[{"id":2,"rights":266}]}' ;;
    assign-rank)
      seed_alliance 1; query="a=16&u=$member_id"; form='newrang=2'; action=assign_rank; payload="$(jq -nc --argjson id "$member_id" '{action:"assign_rank",targetPlayerId:$id,targetRankId:2}')" ;;
    kick-member)
      seed_alliance 1; method=GET; query="a=13&u=$member_id"; action=kick_member; payload="$(jq -nc --argjson id "$member_id" '{action:"kick_member",targetPlayerId:$id}')" ;;
    circular)
      seed_alliance 1 2 128; actor_login="$member_login"; actor_planet="$member_planet"; query='a=17&sendmail=1'; form='r=2&text=Alliance+circular'; action=send_circular; payload='{"action":"send_circular","circularRankId":2,"text":"Alliance circular"}' ;;
    text-external|text-internal|text-application)
      seed_alliance 1; kind=1; value='External text'; [ "$name_case" != text-internal ] || { kind=2; value='Internal text'; }; [ "$name_case" != text-application ] || { kind=3; value='Application text'; }; query="a=11&d=1&t=$kind"; form="text=$(printf '%s' "$value" | sed 's/ /+/g')&bewforce=1"; action=save_text; payload="$(jq -nc --argjson kind "$kind" --arg text "$value" '{action:"save_text",textKind:$kind,text:$text,insertApp:true}')" ;;
    settings)
      seed_alliance 1; query='a=11&d=2'; form='hp=https%3A%2F%2Fexample.test&logo=https%3A%2F%2Fexample.test%2Flogo.png&bew=1&fname=Leader'; action=save_settings; payload='{"action":"save_settings","homepage":"https://example.test","imageLogo":"https://example.test/logo.png","open":false,"founderRankName":"Leader"}' ;;
    delete-rank)
      seed_alliance 1; method=GET; query='a=15&d=2'; action=delete_rank; payload='{"action":"delete_rank","rankId":2}' ;;
    change-tag|change-tag-cooldown)
      seed_alliance 1; query='a=9&weiter=1'; form='newtag=E2EADNEW'; action=change_tag; payload='{"action":"change_tag","tag":"E2EADNEW"}'; [ "$name_case" != change-tag-cooldown ] || db_query "UPDATE uni1_ally SET tag_until=UNIX_TIMESTAMP()+3600 WHERE ally_id=$case_ally_id" >/dev/null ;;
    change-name|change-name-cooldown)
      seed_alliance 1; query='a=10&weiter=1'; form='newname=Renamed+Alliance'; action=change_name; payload='{"action":"change_name","name":"Renamed Alliance"}'; [ "$name_case" != change-name-cooldown ] || db_query "UPDATE uni1_ally SET name_until=UNIX_TIMESTAMP()+3600 WHERE ally_id=$case_ally_id" >/dev/null ;;
    dismiss)
      seed_alliance 1; query='a=12&weiter=1'; form='confirm=1'; action=dismiss; payload='{"action":"dismiss"}' ;;
    transfer-founder)
      seed_alliance 1 2 256; query='a=18'; form="s=1&uid=$member_id"; action=transfer_founder; payload="$(jq -nc --argjson id "$member_id" '{action:"transfer_founder",targetPlayerId:$id}')" ;;
    denied-management)
      seed_alliance 1; actor_login="$member_login"; actor_planet="$member_planet"; query='a=11&d=1&t=2'; form='text=Forbidden'; action=save_text; payload='{"action":"save_text","textKind":2,"text":"Forbidden","insertApp":false}' ;;
  esac
}

run_side() {
  side="$1"; name_case="$2"; configure_case "$name_case"
  if [ "$side" = legacy ]; then
    login_legacy "$side-$name_case" "$actor_login"
    url="$LEGACY_BASE_URL/game/index.php?page=$page&session=$session&cp=$actor_planet"
    [ -z "$query" ] || url="$url&$query"
    if [ "$method" = POST ]; then
      http="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$name_case.body" --header 'Content-Type: application/x-www-form-urlencoded' --data "$form" $cookie_arg --write-out '%{http_code}' "$url")"
    else
      http="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$name_case.body" $cookie_arg --write-out '%{http_code}' "$url")"
    fi
    issue=''
  else
    login_go "$side-$name_case" "$actor_login"
    url="$GO_BASE_URL/api/game/alliance?session=$session&cp=$actor_planet"
    [ -z "$query" ] || url="$url&$query"
    http="$(curl --silent --show-error --max-time 15 --output "$TMP_DIR/$side-$name_case.body" --header 'Content-Type: application/json' --data "$payload" $cookie_arg --write-out '%{http_code}' "$url")"
    issue="$(jq -r '.actionIssue.code // ""' "$TMP_DIR/$side-$name_case.body")"
  fi
  state="$(capture_state)"
  normalized="$(jq -ncS --arg http "$http" --arg issue "$issue" --argjson state "$state" '{http:($http|tonumber),issue:$issue,state:$state}')"
}

results="$TMP_DIR/cases.jsonl"; : > "$results"; all_pass=true
cases="${OGAME_ALLIANCE_DIFFERENTIAL_CASES:-create create-invalid apply apply-closed apply-unvalidated withdraw accept reject-default reject-reason leave add-rank save-ranks assign-rank kick-member circular text-external text-internal text-application settings delete-rank change-tag change-tag-cooldown change-name change-name-cooldown dismiss transfer-founder denied-management}"
for name_case in $cases; do
  run_side legacy "$name_case"; legacy="$normalized"
  run_side go "$name_case"; go="$normalized"
  pass=true
  [ "$(printf '%s' "$legacy" | jq -cS '.state')" = "$(printf '%s' "$go" | jq -cS '.state')" ] || pass=false
  printf '%s' "$legacy" | jq -e '.http==200 or .http==302' >/dev/null || pass=false
  printf '%s' "$go" | jq -e '.http==200' >/dev/null || pass=false
  [ "$pass" = true ] || all_pass=false
  jq -nc --arg name "alliance-$name_case" --argjson pass "$pass" --argjson legacy "$legacy" --argjson go "$go" '{name:$name,pass:$pass,legacy:$legacy,go:$go}' >> "$results"
done

jq -s --argjson pass "$all_pass" '{pass:$pass,normalization:"auto IDs and exact timestamps are reduced to stable roles and boolean time contracts; all persisted values and messages remain exact",cases:.}' "$results" > "$REPORT"
[ "$all_pass" = true ]
case_count="$(printf '%s\n' $cases | wc -l | tr -d ' ')"
printf 'Go/PHP alliance differential E2E: PASS (%s cases)\n' "$case_count"
