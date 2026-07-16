#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:8890}"
MAILHOG_BASE_URL="${OGAME_MAILHOG_BASE_URL:-http://127.0.0.1:${OGAME_MAILHOG_PORT:-8026}}"
MAILHOG_API_URL="${OGAME_MAILHOG_API_URL:-$MAILHOG_BASE_URL/api/v2/messages?limit=100}"

wait_for_url() {
  url="$1"
  attempts="${2:-30}"
  i=1
  while [ "$i" -le "$attempts" ]; do
    if curl --fail --silent "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    i=$((i + 1))
  done
  curl --fail --silent "$url" >/dev/null
}

wait_for_url "$GO_BASE_URL/api/healthz"
wait_for_url "$MAILHOG_BASE_URL/api/v2/messages"

cd "$ROOT_DIR/frontend"
OGAME_GO_BASE_URL="$GO_BASE_URL" OGAME_MAILHOG_API_URL="$MAILHOG_API_URL" bun run e2e:csr
