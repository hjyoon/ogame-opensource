#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"

if [ -f "$ROOT_DIR/.env" ]; then
  set -a
  # shellcheck disable=SC1091
  . "$ROOT_DIR/.env"
  set +a
fi

mysql_port="${OGAME_MYSQL_PORT:-3306}"
mysql_password="${MYSQL_ROOT_PASSWORD:-123}"
dsn="${OGAME_TEST_MYSQL_DSN:-root:${mysql_password}@tcp(127.0.0.1:${mysql_port})/uni?parseTime=true}"

cd "$ROOT_DIR/backend"
export GOCACHE="${GOCACHE:-$ROOT_DIR/.tmp/go-cache}"
mkdir -p "$GOCACHE"
OGAME_TEST_MYSQL_DSN="$dsn" go test ./internal/infrastructure/mysqlgame \
  -run TestMySQLFinishDueQueueTaskAtomicallyRunsConcurrentClaimOnce \
  -count=1

printf 'Go queue claim concurrency E2E passed (MySQL).\n'
