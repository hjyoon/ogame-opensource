#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
ORACLE_SOURCE="$ROOT_DIR/testing/e2e/combat-engine-oracle.php"
ORACLE_TARGET="/tmp/ogame-combat-engine-oracle.php"
REPORT="${OGAME_COMBAT_ENGINE_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-combat-engine-differential.json}"

mkdir -p "$(dirname -- "$REPORT")"
docker compose -f "$ROOT_DIR/compose.yaml" cp "$ORACLE_SOURCE" "server:$ORACLE_TARGET" >/dev/null
expected="$(docker compose -f "$ROOT_DIR/compose.yaml" exec -T server php "$ORACLE_TARGET")"
(
  cd "$ROOT_DIR/backend"
  OGAME_COMBAT_ORACLE_EXPECTED="$expected" go test ./internal/domain/game -run '^TestLegacyCombatEngineOracle$' -count=1 >/dev/null
)

printf '%s' "$expected" | jq '{pass:true,caseCount:(.cases|length),cases:[.cases[]|{name,outcome,rounds:(.rounds|length)}]}' > "$REPORT"
printf 'Go/PHP combat engine differential E2E: PASS (%s cases)\n' "$(jq -r '.caseCount' "$REPORT")"
