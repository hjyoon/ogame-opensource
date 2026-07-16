#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
LEGACY_BASE_URL="${OGAME_LEGACY_BASE_URL:-http://127.0.0.1:8888}"
GO_BASE_URL="${OGAME_GO_BASE_URL:-http://127.0.0.1:8890}"
MAILHOG_BASE_URL="${OGAME_MAILHOG_BASE_URL:-http://127.0.0.1:${OGAME_MAILHOG_PORT:-8026}}"
MAILHOG_API_URL="${OGAME_MAILHOG_API_URL:-$MAILHOG_BASE_URL/api/v2/messages?limit=100}"
BROWSERS="${OGAME_PUBLIC_REGISTRATION_DYNAMIC_BROWSERS:-${OGAME_PLAYWRIGHT_BROWSER:-chromium firefox}}"
BROWSERS="$(printf '%s' "$BROWSERS" | tr ',' ' ')"

wait_for_url() {
  url="$1"
  attempts="${2:-30}"
  i=1
  while [ "$i" -le "$attempts" ]; do
    if curl --fail --silent --output /dev/null "$url"; then
      return 0
    fi
    sleep 1
    i=$((i + 1))
  done
  curl --fail --silent --output /dev/null "$url"
}

wait_for_url "$LEGACY_BASE_URL/register.php"
wait_for_url "$GO_BASE_URL/api/healthz"

for browser in $BROWSERS; do
  printf 'Public registration dynamic E2E (%s)\n' "$browser"
  (
    cd "$ROOT_DIR/frontend"
    OGAME_PLAYWRIGHT_BROWSER="$browser" \
    OGAME_LEGACY_BASE_URL="$LEGACY_BASE_URL" \
    OGAME_GO_BASE_URL="$GO_BASE_URL" \
    OGAME_MAILHOG_API_URL="$MAILHOG_API_URL" \
    bun run e2e:dynamic:public-registration
  )
done

if [ "${OGAME_PUBLIC_REGISTRATION_ACTIVATION_ONLY:-0}" != "1" ] && [ "${OGAME_RUN_PUBLIC_REGISTRATION_ACTIVATION:-1}" = "1" ]; then
  for browser in $BROWSERS; do
    printf 'Public registration activation E2E (%s)\n' "$browser"
    (
      cd "$ROOT_DIR/frontend"
      OGAME_PUBLIC_REGISTRATION_ACTIVATION_ONLY=1 \
      OGAME_PLAYWRIGHT_BROWSER="$browser" \
      OGAME_GO_BASE_URL="$GO_BASE_URL" \
      OGAME_MAILHOG_API_URL="$MAILHOG_API_URL" \
      bun run e2e:dynamic:public-registration
    )
  done
fi
