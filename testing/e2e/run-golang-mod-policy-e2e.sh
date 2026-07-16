#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
REPORT="${OGAME_MOD_POLICY_REPORT:-$ROOT_DIR/.tmp/golang-mod-policy.json}"

mysql_query() {
  docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T mysql \
    sh -c 'mysql -N -uroot -p"$MYSQL_ROOT_PASSWORD" uni' 2>/dev/null
}

original="$(printf '%s\n' 'SELECT modlist FROM uni1_uni LIMIT 1;' | mysql_query)"
case "$original" in *[!A-Za-z0-9_\;]*) echo "unsafe modlist fixture" >&2; exit 1;; esac
restore() {
  printf "UPDATE uni1_uni SET modlist='%s';\n" "$original" | mysql_query >/dev/null 2>&1 || true
}
trap restore EXIT INT TERM

printf '%s\n' "UPDATE uni1_uni SET modlist='BogusMod';" | mysql_query >/dev/null
status="$(curl --silent --output /tmp/ogame-mod-policy-health.json --write-out '%{http_code}' "$BASE_URL/api/healthz")"
[ "$status" = "503" ]
jq -e '.status == "unavailable" and .modRuntimeReady == false' /tmp/ogame-mod-policy-health.json >/dev/null
restore
i=0
while [ "$i" -lt 20 ]; do
  if curl --fail --silent "$BASE_URL/api/healthz" | jq -e '.modRuntimeReady == true' >/dev/null; then break; fi
  i=$((i + 1)); sleep 1
done
[ "$i" -lt 20 ]
trap - EXIT INT TERM
mkdir -p "$(dirname -- "$REPORT")"
printf '%s\n' '{"pass":true,"cases":[{"name":"active-php-mod-degrades-readiness","pass":true}]}' > "$REPORT"
printf 'Go Mod policy E2E: PASS\n'
