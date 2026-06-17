#!/bin/sh
set -eu

ROOT="${OGAME_E2E_ROOT:-/tmp/ogame-e2e}"
OUT_DIR="${OGAME_E2E_OUT_DIR:-/tmp/ogame-e2e-results}"
mkdir -p "$OUT_DIR"
export OGAME_E2E_OUT_DIR="$OUT_DIR"
find "$OUT_DIR" -maxdepth 1 -type f \( -name '*.stderr' -o -name '*.json' -o -name 'summary.md' \) ! -name 'performance-baseline-metrics.json' -exec rm -f {} \;

failures=0

run_json_case() {
  name="$1"
  file="$2"
  output="$OUT_DIR/${name}.json"
  stderr="$OUT_DIR/${name}.stderr"

  printf '==> %s\n' "$name"
  if php "$file" > "$output" 2>"$stderr"; then
    if [ -s "$stderr" ]; then
      printf 'FAIL %s\n' "$name"
      printf 'PHP stderr was not empty for %s:\n' "$name"
      sed -n '1,20p' "$stderr"
      failures=$((failures + 1))
    elif php "$ROOT/assert-json-pass.php" "$output"; then
      printf 'PASS %s\n' "$name"
    else
      printf 'FAIL %s\n' "$name"
      failures=$((failures + 1))
    fi
  else
    printf 'ERROR %s\n' "$name"
    if [ -s "$stderr" ]; then
      printf 'PHP stderr for %s:\n' "$name"
      sed -n '1,20p' "$stderr"
    fi
    failures=$((failures + 1))
  fi
}

cleanup() {
  php "$ROOT/teardown-fixtures.php" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

eval "$(php "$ROOT/audit-baseline.php")"
export OGAME_E2E_AUDIT_BASE_MESSAGE_ID
export OGAME_E2E_AUDIT_BASE_NOTE_ID
export OGAME_E2E_AUDIT_BASE_REPORT_ID
export OGAME_E2E_AUDIT_BASE_TEMPLATE_ID
export OGAME_E2E_AUDIT_BASE_BOTVAR_ID
export OGAME_E2E_AUDIT_BASE_ALLYAPP_ID
export OGAME_E2E_AUDIT_BASE_UNION_ID
export OGAME_E2E_AUDIT_BASE_BATTLE_ID

run_json_case http_flow "$ROOT/http_flow_e2e.php"

eval "$(php "$ROOT/setup-fixtures.php")"
export OGAME_E2E_ATTACKER_ID OGAME_E2E_ATTACKER_PLANET
export OGAME_E2E_DEFENDER_ID OGAME_E2E_DEFENDER_PLANET
export OGAME_E2E_ATTACKER_NAME OGAME_E2E_ATTACKER_PASSWORD
export OGAME_E2E_DEFENDER_NAME OGAME_E2E_DEFENDER_PASSWORD

run_json_case route_matrix "$ROOT/http_route_matrix_e2e.php"
run_json_case render_asset_smoke "$ROOT/http_render_asset_smoke_e2e.php"
run_json_case multi_universe_isolation "$ROOT/http_multi_universe_isolation_e2e.php"
run_json_case account_actions "$ROOT/http_account_actions_e2e.php"
run_json_case account_security "$ROOT/http_account_security_e2e.php"
run_json_case localization_edges "$ROOT/http_localization_edges_e2e.php"
run_json_case session_security_edges "$ROOT/http_session_security_edges_e2e.php"
run_json_case security_hardening "$ROOT/http_security_hardening_e2e.php"
run_json_case direct_entry_security "$ROOT/http_direct_entry_security_e2e.php"
run_json_case password_recovery "$ROOT/http_password_recovery_e2e.php"
run_json_case registration_validation "$ROOT/http_registration_validation_e2e.php"
run_json_case account_deletion_cleanup "$ROOT/http_account_deletion_cleanup_e2e.php"
run_json_case message_lifecycle "$ROOT/http_message_lifecycle_e2e.php"
run_json_case report_retention_edges "$ROOT/http_report_retention_edges_e2e.php"
run_json_case galaxy_templates "$ROOT/http_galaxy_templates_e2e.php"
run_json_case target_restrictions "$ROOT/http_target_restrictions_e2e.php"
run_json_case planet_context "$ROOT/http_planet_context_e2e.php"
run_json_case planet_cleanup_edges "$ROOT/http_planet_cleanup_edges_e2e.php"
run_json_case social_access "$ROOT/http_social_access_e2e.php"
run_json_case idor_sweep "$ROOT/http_idor_sweep_e2e.php"
run_json_case input_hardening "$ROOT/http_input_hardening_e2e.php"
run_json_case alliance_management "$ROOT/http_alliance_management_e2e.php"
run_json_case admin_state "$ROOT/http_admin_state_e2e.php"
run_json_case admin_permission_matrix "$ROOT/http_admin_permission_matrix_e2e.php"
run_json_case admin_audit_logs "$ROOT/http_admin_audit_logs_e2e.php"
run_json_case admin_tools_smoke "$ROOT/http_admin_tools_smoke_e2e.php"
run_json_case admin_operations "$ROOT/http_admin_operations_e2e.php"
run_json_case admin_db_backup "$ROOT/http_admin_db_backup_e2e.php"
run_json_case admin_destructive "$ROOT/http_admin_destructive_e2e.php"
run_json_case coupon_payment "$ROOT/http_coupon_payment_e2e.php"
run_json_case queue_fleet "$ROOT/http_queue_fleet_e2e.php"
run_json_case queue_idempotency "$ROOT/http_queue_idempotency_e2e.php"
run_json_case long_scheduler "$ROOT/http_long_scheduler_e2e.php"
run_json_case soak_state_invariants "$ROOT/http_soak_state_invariants_e2e.php"
run_json_case recovery_bulk_journey "$ROOT/http_recovery_bulk_journey_e2e.php"
run_json_case performance_baseline "$ROOT/http_performance_baseline_e2e.php"
run_json_case vacation_freeze_edges "$ROOT/http_vacation_freeze_edges_e2e.php"
run_json_case global_maintenance_queue "$ROOT/http_global_maintenance_queue_e2e.php"
run_json_case cron_resilience "$ROOT/http_cron_resilience_e2e.php"
run_json_case concurrency_race "$ROOT/http_concurrency_race_e2e.php"
run_json_case tech_economy "$ROOT/http_tech_economy_e2e.php"
run_json_case stats_ranking "$ROOT/http_stats_ranking_e2e.php"
run_json_case fleet_lifecycle "$ROOT/http_fleet_lifecycle_e2e.php"
run_json_case fleet_recall_edges "$ROOT/http_fleet_recall_edges_e2e.php"
run_json_case acs_hold "$ROOT/http_acs_hold_e2e.php"
run_json_case alliance_depot "$ROOT/http_alliance_depot_e2e.php"
run_json_case trader_moon "$ROOT/http_trader_moon_e2e.php"
run_json_case premium_dm_edges "$ROOT/http_premium_dm_edges_e2e.php"
run_json_case jump_gate_edges "$ROOT/http_jump_gate_edges_e2e.php"
run_json_case report_simulation "$ROOT/cases/report_simulation.php"
run_json_case economy "$ROOT/cases/economy_case_tests.php"
run_json_case missile "$ROOT/cases/missile_case_tests.php"
run_json_case colony_moon "$ROOT/cases/colony_moon_case_tests.php"
run_json_case moon_edges "$ROOT/cases/moon_edge_tests.php"
run_json_case fleet_slots "$ROOT/cases/fleet_slot_sweep.php"
run_json_case expedition "$ROOT/cases/expedition_case_tests.php"
run_json_case db_invariant_audit "$ROOT/http_db_invariant_audit_e2e.php"

if php "$ROOT/summarize-results.php" "$OUT_DIR"; then
  printf 'E2E summary: %s/summary.md\n' "$OUT_DIR"
else
  printf 'E2E summary generation failed.\n'
  failures=$((failures + 1))
fi

if [ "$failures" -ne 0 ]; then
  printf 'E2E failed: %s case(s). JSON results: %s\n' "$failures" "$OUT_DIR"
  exit 1
fi

printf 'E2E passed. JSON results: %s\n' "$OUT_DIR"
