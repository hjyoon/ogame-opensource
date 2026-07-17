#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
ORACLE_SOURCE="$ROOT_DIR/testing/e2e/combat-report-oracle.php"
ORACLE_TARGET="/tmp/ogame-combat-report-oracle.php"
REPORT="${OGAME_COMBAT_REPORT_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-combat-report-differential.json}"

mkdir -p "$(dirname -- "$REPORT")"
docker compose -f "$ROOT_DIR/docker-compose.yml" cp "$ORACLE_SOURCE" "server:$ORACLE_TARGET" >/dev/null
expected="$(docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T server php "$ORACLE_TARGET")"
(
  cd "$ROOT_DIR/backend"
  OGAME_COMBAT_REPORT_ORACLE_EXPECTED="$expected" go test ./internal/infrastructure/mysqlgame -run '^TestLegacyCombatReportOracle$' -count=1 >/dev/null
)

printf '%s' "$expected" | jq '{
  pass:true,
  caseCount:(.cases|length),
  cases:[.cases[]|{
    name,
    reportBytes:(.report|length),
    rounds:([.report|scan("The attacking fleet fires")]|length),
    complete:(
      (.report|contains("</table><p> ")) and
      ((.report|contains("The defender has won the battle!")) or (.report|contains("battle ended in a draw")))
    )
  }]
}' > "$REPORT"
jq -e '.pass and all(.cases[]; .complete)' "$REPORT" >/dev/null
printf 'Go/PHP combat report differential E2E: PASS (%s cases)\n' "$(jq -r '.caseCount' "$REPORT")"
