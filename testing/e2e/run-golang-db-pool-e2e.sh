#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
REPORT="${OGAME_DB_POOL_REPORT:-$ROOT_DIR/.tmp/golang-db-pool.json}"
MAX_GROWTH="${OGAME_DB_POOL_MAX_GROWTH:-12}"

connections() {
  docker compose -f "$ROOT_DIR/compose.golang.yaml" exec -T mysql \
    sh -c 'mysql -N -uroot -p"$MYSQL_ROOT_PASSWORD" -e "SHOW STATUS LIKE '\''Threads_connected'\''"' 2>/dev/null | awk '{print $2}'
}

baseline="$(connections)"
i=0
while [ "$i" -lt 60 ]; do
  curl --fail --silent "$BASE_URL/api/public/universes" >/dev/null &
  curl --fail --silent -H 'Content-Type: application/json' \
    --data '{"login":"pool-audit","pass":"invalid","universe":"http://localhost:8888"}' \
    "$BASE_URL/api/public/login/validate" >/dev/null &
  i=$((i + 1))
done
wait
sleep 2
after="$(connections)"
growth=$((after - baseline))
[ "$growth" -le "$MAX_GROWTH" ]
mkdir -p "$(dirname -- "$REPORT")"
printf '{"pass":true,"cases":[{"name":"shared-db-pool-bounded","pass":true}],"baseline":%s,"after":%s,"growth":%s}\n' \
  "$baseline" "$after" "$growth" > "$REPORT"
printf 'Go DB pool E2E: PASS (baseline=%s after=%s growth=%s)\n' "$baseline" "$after" "$growth"
