#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:${OGAME_GO_PORT:-8890}}"
URL_CHECK_FILE="$ROOT_DIR/.tmp/navigation-url-check"
mkdir -p "$ROOT_DIR/.tmp"

wait_for_url() {
  url="$1"
  attempts="${2:-30}"
  i=1
  while [ "$i" -le "$attempts" ]; do
    if curl --fail --silent --output "$URL_CHECK_FILE" "$url"; then
      return 0
    fi
    sleep 1
    i=$((i + 1))
  done
  curl --fail --silent --output "$URL_CHECK_FILE" "$url"
}

wait_for_url "$LEGACY_BASE_URL/home.php"
wait_for_url "$GO_BASE_URL/api/healthz"

cd "$ROOT_DIR/frontend"
status=0
for browser in ${OGAME_NAV_VISUAL_BROWSERS:-chromium firefox}; do
  printf 'Navigation visual E2E (%s)\n' "$browser"
  if ! OGAME_PLAYWRIGHT_BROWSER="$browser" \
    OGAME_LEGACY_BASE_URL="$LEGACY_BASE_URL" \
    OGAME_GO_BASE_URL="$GO_BASE_URL" \
    bun run e2e:visual:navigation; then
    status=1
  fi
done
if ! bun run scripts/render-navigation-visual-coverage.ts; then
  status=1
fi
exit "$status"
