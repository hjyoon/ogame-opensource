#!/bin/sh
set -eu

ROOT="${OGAME_E2E_ROOT:-/tmp/ogame-e2e}"
OUT_DIR="${OGAME_E2E_OUT_DIR:-/tmp/ogame-e2e-results}"
mkdir -p "$OUT_DIR"

failures=0

run_json_case() {
  name="$1"
  file="$2"
  output="$OUT_DIR/${name}.json"

  printf '==> %s\n' "$name"
  if php "$file" > "$output"; then
    if php "$ROOT/assert-json-pass.php" "$output"; then
      printf 'PASS %s\n' "$name"
    else
      printf 'FAIL %s\n' "$name"
      failures=$((failures + 1))
    fi
  else
    printf 'ERROR %s\n' "$name"
    failures=$((failures + 1))
  fi
}

cleanup() {
  php "$ROOT/teardown-fixtures.php" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

run_json_case http_flow "$ROOT/http_flow_e2e.php"

eval "$(php "$ROOT/setup-fixtures.php")"
export OGAME_E2E_ATTACKER_ID OGAME_E2E_ATTACKER_PLANET
export OGAME_E2E_DEFENDER_ID OGAME_E2E_DEFENDER_PLANET
export OGAME_E2E_ATTACKER_NAME OGAME_E2E_ATTACKER_PASSWORD
export OGAME_E2E_DEFENDER_NAME OGAME_E2E_DEFENDER_PASSWORD

run_json_case route_matrix "$ROOT/http_route_matrix_e2e.php"
run_json_case render_asset_smoke "$ROOT/http_render_asset_smoke_e2e.php"
run_json_case account_actions "$ROOT/http_account_actions_e2e.php"
run_json_case message_lifecycle "$ROOT/http_message_lifecycle_e2e.php"
run_json_case galaxy_templates "$ROOT/http_galaxy_templates_e2e.php"
run_json_case target_restrictions "$ROOT/http_target_restrictions_e2e.php"
run_json_case social_access "$ROOT/http_social_access_e2e.php"
run_json_case alliance_management "$ROOT/http_alliance_management_e2e.php"
run_json_case admin_state "$ROOT/http_admin_state_e2e.php"
run_json_case coupon_payment "$ROOT/http_coupon_payment_e2e.php"
run_json_case queue_fleet "$ROOT/http_queue_fleet_e2e.php"
run_json_case tech_economy "$ROOT/http_tech_economy_e2e.php"
run_json_case stats_ranking "$ROOT/http_stats_ranking_e2e.php"
run_json_case fleet_lifecycle "$ROOT/http_fleet_lifecycle_e2e.php"
run_json_case fleet_recall_edges "$ROOT/http_fleet_recall_edges_e2e.php"
run_json_case acs_hold "$ROOT/http_acs_hold_e2e.php"
run_json_case alliance_depot "$ROOT/http_alliance_depot_e2e.php"
run_json_case trader_moon "$ROOT/http_trader_moon_e2e.php"
run_json_case jump_gate_edges "$ROOT/http_jump_gate_edges_e2e.php"
run_json_case report_simulation "$ROOT/cases/report_simulation.php"
run_json_case economy "$ROOT/cases/economy_case_tests.php"
run_json_case missile "$ROOT/cases/missile_case_tests.php"
run_json_case colony_moon "$ROOT/cases/colony_moon_case_tests.php"
run_json_case moon_edges "$ROOT/cases/moon_edge_tests.php"
run_json_case fleet_slots "$ROOT/cases/fleet_slot_sweep.php"
run_json_case expedition "$ROOT/cases/expedition_case_tests.php"

if [ "$failures" -ne 0 ]; then
  printf 'E2E failed: %s case(s). JSON results: %s\n' "$failures" "$OUT_DIR"
  exit 1
fi

printf 'E2E passed. JSON results: %s\n' "$OUT_DIR"
