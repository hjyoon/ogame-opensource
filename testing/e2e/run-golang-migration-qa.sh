#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
GO_BASE_URL="http://127.0.0.1:${OGAME_GO_PORT:-8890}"
MAILHOG_BASE_URL="http://127.0.0.1:${OGAME_MAILHOG_PORT:-8026}"
LEGACY_E2E_CONTAINER_DIR="${OGAME_E2E_CONTAINER_DIR:-/tmp/ogame-e2e}"
mkdir -p "$ROOT_DIR/.tmp"

export OGAME_MCP_RATE_LIMIT_ENABLE="${OGAME_MCP_RATE_LIMIT_ENABLE:-0}"
export OGAME_QUEUE_POLL_INTERVAL_MS="${OGAME_QUEUE_POLL_INTERVAL_MS:-0}"

if command -v bun >/dev/null 2>&1; then
  bun "$SCRIPT_DIR/audit-legacy-behavior-surface.mjs"
  bun "$SCRIPT_DIR/audit-legacy-migration-inventory.mjs"
  bun "$SCRIPT_DIR/audit-checksum-baselines.mjs"
  bun "$SCRIPT_DIR/audit-state-mutation-coverage.mjs"
  bun "$SCRIPT_DIR/audit-queue-runtime-coverage.mjs"
  bun "$SCRIPT_DIR/audit-bot-runtime-coverage.mjs"
fi

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

if [ "${OGAME_RUN_LEGACY_E2E:-1}" = "1" ]; then
  if [ "${OGAME_RUN_GO_DOCKER:-1}" = "1" ]; then
    docker compose -f "$ROOT_DIR/docker-compose.yml" up -d mailhog >/dev/null
    wait_for_url "$MAILHOG_BASE_URL/api/v2/messages"
  fi
  "$SCRIPT_DIR/run-docker-e2e.sh"
  docker compose exec -T server env OGAME_SMOKE_SEED_BOT_ACTIONS=1 php "$LEGACY_E2E_CONTAINER_DIR/prepare-golang-smoke-fixture.php" > "$ROOT_DIR/.tmp/golang-smoke-fixture.json"
else
  # Partial QA still needs a fresh due bot graph for compatibility smoke.
  docker compose cp "$SCRIPT_DIR/prepare-golang-smoke-fixture.php" "server:$LEGACY_E2E_CONTAINER_DIR/prepare-golang-smoke-fixture.php" >/dev/null
  docker compose exec -T server env OGAME_SMOKE_SEED_BOT_ACTIONS=1 php "$LEGACY_E2E_CONTAINER_DIR/prepare-golang-smoke-fixture.php" > "$ROOT_DIR/.tmp/golang-smoke-fixture.json"
fi

if command -v bun >/dev/null 2>&1; then
  (cd "$ROOT_DIR/frontend" && bun install && bun run build && bun run check && bun run test)
else
  printf 'SKIP frontend build: bun was not found\n'
fi

if command -v go >/dev/null 2>&1 && go version >/dev/null 2>&1; then
  "$ROOT_DIR/backend/scripts/test-coverage.sh"
  if [ "${OGAME_RUN_QUEUE_CLAIM_CONCURRENCY_E2E:-1}" = "1" ]; then
    "$SCRIPT_DIR/run-golang-queue-claim-concurrency-e2e.sh"
  fi
else
  printf 'SKIP backend tests: go was not available\n'
fi

if [ "${OGAME_RUN_GO_DOCKER:-1}" = "1" ]; then
  if [ "${OGAME_KEEP_GO_DOCKER:-0}" != "1" ]; then
    trap 'docker compose -f "$ROOT_DIR/docker-compose.yml" stop goapp >/dev/null 2>&1 || true' EXIT INT TERM
  fi
  docker compose -f "$ROOT_DIR/docker-compose.yml" up -d --build --force-recreate goapp
  wait_for_url "$GO_BASE_URL/api/healthz"
  wait_for_url "$GO_BASE_URL/"
  if [ "${OGAME_RUN_DB_RECOVERY_E2E:-1}" = "1" ]; then
    OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-db-recovery-e2e.sh"
  fi
  if [ "${OGAME_RUN_DB_POOL_E2E:-1}" = "1" ]; then
    OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-db-pool-e2e.sh"
  fi
  if [ "${OGAME_RUN_MOD_POLICY_E2E:-1}" = "1" ]; then
    OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-mod-policy-e2e.sh"
  fi
  if command -v bun >/dev/null 2>&1; then
    bun "$SCRIPT_DIR/golang-mcp-smoke.mjs" --go-base-url "$GO_BASE_URL" > "$ROOT_DIR/.tmp/golang-mcp-smoke.json"
    printf 'Go MCP smoke: %s\n' "$ROOT_DIR/.tmp/golang-mcp-smoke.json"
    bun "$SCRIPT_DIR/golang-compat-smoke.mjs" --go-base-url "$GO_BASE_URL" --mailhog-base-url "$MAILHOG_BASE_URL" --fixture "$ROOT_DIR/.tmp/golang-smoke-fixture.json" > "$ROOT_DIR/.tmp/golang-compat-smoke.json"
    printf 'Go compatibility smoke: %s\n' "$ROOT_DIR/.tmp/golang-compat-smoke.json"
    # Universe freeze parity intentionally moves every active player into vacation mode.
    # Re-seed shared actors before pairwise suites so one valid smoke state cannot leak into the next suite.
    docker compose exec -T server env OGAME_SMOKE_SEED_BOT_ACTIONS=0 php "$LEGACY_E2E_CONTAINER_DIR/prepare-golang-smoke-fixture.php" > "$ROOT_DIR/.tmp/golang-smoke-fixture.json"
    if [ "${OGAME_RUN_RESOURCE_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-resource-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_OVERVIEW_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-overview-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_OFFICERS_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-officers-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_MERCHANT_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-merchant-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_PAYMENT_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-payment-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_EMPIRE_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-empire-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_GALAXY_ACTIONS_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-galaxy-actions-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_PUBLIC_ACCOUNT_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" OGAME_MAILHOG_BASE_URL="$MAILHOG_BASE_URL" "$SCRIPT_DIR/run-golang-public-account-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_BUILDING_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-building-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_RESEARCH_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-research-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_GRAVITON_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-graviton-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_SHIPYARD_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-shipyard-defense-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_BUILDING_ADVANCED_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-building-advanced-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_TERRAFORMER_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-terraformer-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_DEFENSE_LIMITS_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-defense-limits-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_FLEET_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-fleet-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_FLEET_SPEED_RECALL_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-fleet-speed-recall-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_FLEET_TEMPLATE_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-fleet-template-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_COLONIZATION_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-colonization-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_MISSILE_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-missile-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_EXPEDITION_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-expedition-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_PHALANX_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-phalanx-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_JUMP_GATE_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-jump-gate-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_BUDDY_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-buddy-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_MESSAGES_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-messages-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_NOTES_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-notes-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_ALLIANCE_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-alliance-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_OPTIONS_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" OGAME_MAILHOG_BASE_URL="$MAILHOG_BASE_URL" "$SCRIPT_DIR/run-golang-options-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_ADMIN_BANS_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-admin-bans-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_ADMIN_QUEUE_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-admin-queue-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_ADMIN_CRON_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-admin-cron-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_RUNTIME_QUEUE_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-runtime-queue-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_ADMIN_CLEANUP_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-admin-cleanup-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_ADMIN_COUPON_CRON_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" OGAME_MAILHOG_BASE_URL="$MAILHOG_BASE_URL" "$SCRIPT_DIR/run-golang-admin-coupon-cron-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_ADMIN_USERS_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" OGAME_MAILHOG_BASE_URL="$MAILHOG_BASE_URL" "$SCRIPT_DIR/run-golang-admin-users-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_ADMIN_PLANETS_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-admin-planets-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_ADMIN_OPERATIONS_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-admin-operations-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_ADMIN_UNIVERSE_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-admin-universe-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_ADMIN_AUDIT_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-admin-audit-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_ADMIN_COUPONS_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-admin-coupons-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_ADMIN_DATABASE_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-admin-database-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_ADMIN_BOTS_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-admin-bots-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_ADMIN_COLONY_SETTINGS_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-admin-colony-settings-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_ADMIN_CHECKSUM_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-admin-checksum-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_ADMIN_LOCA_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-admin-loca-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_ADMIN_SIMULATORS_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-admin-simulators-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_ACS_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-acs-attack-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_HOLDING_DEFENSE_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-holding-defense-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_BATTLE_MOON_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-battle-moon-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_MOON_DESTRUCTION_DIFFERENTIAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-moon-destruction-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_COMBAT_ENGINE_DIFFERENTIAL:-1}" = "1" ]; then
      "$SCRIPT_DIR/run-golang-combat-engine-differential-e2e.sh"
    fi
    if [ "${OGAME_RUN_COMBAT_REPORT_DIFFERENTIAL:-1}" = "1" ]; then
      "$SCRIPT_DIR/run-golang-combat-report-differential-e2e.sh"
    fi
    docker compose exec -T server php "$LEGACY_E2E_CONTAINER_DIR/cleanup-golang-migration-fixtures.php" >/dev/null
    docker compose cp "$SCRIPT_DIR/prepare-golang-user-type-fixture.php" "server:$LEGACY_E2E_CONTAINER_DIR/prepare-golang-user-type-fixture.php" >/dev/null
    docker compose exec -T server php "$LEGACY_E2E_CONTAINER_DIR/prepare-golang-user-type-fixture.php" > "$ROOT_DIR/.tmp/golang-user-type-fixture.json"
    OGAME_GO_BASE_URL="$GO_BASE_URL" OGAME_USER_TYPE_FIXTURE_FILE="$ROOT_DIR/.tmp/golang-user-type-fixture.json" bun "$SCRIPT_DIR/golang-user-type-qa.mjs" > "$ROOT_DIR/.tmp/golang-user-type-qa.json"
    printf 'Go user type QA: %s\n' "$ROOT_DIR/.tmp/golang-user-type-qa.json"
    for browser in ${OGAME_USER_TYPE_BROWSERS:-chromium firefox}; do
      printf 'Go user type Playwright QA (%s)\n' "$browser"
      (cd "$ROOT_DIR/frontend" && OGAME_PLAYWRIGHT_BROWSER="$browser" OGAME_GO_BASE_URL="$GO_BASE_URL" OGAME_USER_TYPE_FIXTURE_FILE="$ROOT_DIR/.tmp/golang-user-type-fixture.json" bun run e2e:user-types)
    done
    if [ "${OGAME_RUN_PUBLIC_VISUAL:-1}" = "1" ]; then
      for browser in ${OGAME_PUBLIC_VISUAL_BROWSERS:-chromium firefox}; do
        printf 'Public visual E2E (%s)\n' "$browser"
        OGAME_PLAYWRIGHT_BROWSER="$browser" \
        OGAME_GO_BASE_URL="$GO_BASE_URL" \
        "$SCRIPT_DIR/run-playwright-visual-e2e.sh"
      done
    fi
    if [ "${OGAME_RUN_AUTH_VISUAL:-1}" = "1" ]; then
      for browser in ${OGAME_AUTH_VISUAL_BROWSERS:-chromium firefox}; do
        printf 'Authenticated visual E2E (%s)\n' "$browser"
        OGAME_PLAYWRIGHT_BROWSER="$browser" \
        OGAME_GO_BASE_URL="$GO_BASE_URL" \
        OGAME_AUTH_VISUAL_OUTPUT_DIR="$ROOT_DIR/.tmp/playwright-auth-visual/auth/$browser" \
        OGAME_AUTH_VISUAL_ENFORCE_DIFF="${OGAME_AUTH_VISUAL_ENFORCE_DIFF:-1}" \
        OGAME_AUTH_VISUAL_ENFORCE_LAYOUT="${OGAME_AUTH_VISUAL_ENFORCE_LAYOUT:-1}" \
        OGAME_AUTH_VISUAL_MAX_DIFF_RATIO="${OGAME_AUTH_VISUAL_MAX_DIFF_RATIO:-0}" \
        OGAME_AUTH_VISUAL_MAX_BOX_DELTA="${OGAME_AUTH_VISUAL_MAX_BOX_DELTA:-0}" \
        "$SCRIPT_DIR/run-playwright-auth-visual-e2e.sh"
      done
    fi
    if [ "${OGAME_RUN_PUBLIC_LOGIN_DYNAMIC:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-playwright-public-login-dynamic-e2e.sh"
    fi
    if [ "${OGAME_RUN_PUBLIC_REGISTRATION_ACTIVATION:-1}" = "1" ]; then
      OGAME_PUBLIC_REGISTRATION_ACTIVATION_ONLY=1 \
      OGAME_PUBLIC_REGISTRATION_DYNAMIC_BROWSERS="${OGAME_PUBLIC_REGISTRATION_ACTIVATION_BROWSERS:-chromium firefox}" \
      OGAME_GO_BASE_URL="$GO_BASE_URL" \
      OGAME_MAILHOG_BASE_URL="$MAILHOG_BASE_URL" \
      "$SCRIPT_DIR/run-playwright-public-registration-dynamic-e2e.sh"
    fi
    if [ "${OGAME_RUN_SESSION_EXPIRY_VISUAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-playwright-session-expiry-visual-e2e.sh"
    fi
    if [ "${OGAME_RUN_AUTH_GAME_VISUAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-playwright-authenticated-game-visual-e2e.sh"
    fi
    if [ "${OGAME_RUN_AUTH_GAME_COMMANDER_VISUAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" \
      OGAME_GAME_VISUAL_COMMANDER_FIXTURE=1 \
      OGAME_GAME_VISUAL_OUTPUT_ROOT="$ROOT_DIR/.tmp/playwright-authenticated-game-commander-visual" \
      OGAME_GAME_VISUAL_FIXTURE_FILE="$ROOT_DIR/.tmp/authenticated-game-commander-visual-fixture.json" \
      "$SCRIPT_DIR/run-playwright-authenticated-game-visual-e2e.sh"
    fi
    if [ "${OGAME_RUN_AUTH_GAME_DYNAMIC:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-playwright-authenticated-game-dynamic-e2e.sh"
    fi
    if [ "${OGAME_RUN_EMPIRE_VISUAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-playwright-empire-visual-e2e.sh"
    fi
    if [ "${OGAME_RUN_ALLIANCE_VISUAL:-1}" = "1" ]; then
      for browser in ${OGAME_ALLIANCE_VISUAL_BROWSERS:-chromium firefox}; do
        printf 'Alliance visual E2E (%s)\n' "$browser"
        OGAME_PLAYWRIGHT_BROWSER="$browser" \
        OGAME_GO_BASE_URL="$GO_BASE_URL" \
        OGAME_AUTH_VISUAL_OUTPUT_DIR="$ROOT_DIR/.tmp/playwright-auth-visual/alliance/$browser" \
        OGAME_AUTH_VISUAL_ENFORCE_DIFF="${OGAME_AUTH_VISUAL_ENFORCE_DIFF:-1}" \
        OGAME_AUTH_VISUAL_ENFORCE_LAYOUT="${OGAME_AUTH_VISUAL_ENFORCE_LAYOUT:-1}" \
        OGAME_AUTH_VISUAL_MAX_DIFF_RATIO="${OGAME_AUTH_VISUAL_MAX_DIFF_RATIO:-0}" \
        OGAME_AUTH_VISUAL_MAX_BOX_DELTA="${OGAME_AUTH_VISUAL_MAX_BOX_DELTA:-0}" \
        "$SCRIPT_DIR/run-playwright-alliance-visual-e2e.sh"
      done
    fi
    if [ "${OGAME_RUN_OVERVIEW_FLEET_VISUAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-playwright-overview-fleet-visual-e2e.sh"
    fi
    if [ "${OGAME_RUN_OVERVIEW_FLEET_COUNTDOWN:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-playwright-overview-fleet-countdown-e2e.sh"
    fi
    if [ "${OGAME_RUN_OVERVIEW_FLEET_REFRESH:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-overview-fleet-refresh-e2e.sh"
    fi
    if [ "${OGAME_RUN_BATTLE_REPORT_IMMEDIATE:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-golang-battle-report-immediate-e2e.sh"
    fi
    if [ "${OGAME_RUN_OVERVIEW_ALL_CASES:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-playwright-overview-all-cases-e2e.sh"
    fi
    if [ "${OGAME_RUN_FLEET_CONTINUE_VISUAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-playwright-fleet-continue-visual-e2e.sh"
    fi
    if [ "${OGAME_RUN_FLEET_ALL_CASES:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-playwright-fleet-all-cases-e2e.sh"
    fi
    if [ "${OGAME_RUN_FLEET_RECALL_TIMING:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-playwright-fleet-recall-timing-e2e.sh"
    fi
    if [ "${OGAME_RUN_NAVIGATION_VISUAL:-1}" = "1" ]; then
      OGAME_GO_BASE_URL="$GO_BASE_URL" "$SCRIPT_DIR/run-playwright-navigation-visual-e2e.sh"
    fi
  fi
fi

if command -v bun >/dev/null 2>&1; then
  OGAME_GO_BASE_URL="$GO_BASE_URL" bun "$SCRIPT_DIR/golang-migration-qa-summary.mjs"
fi
