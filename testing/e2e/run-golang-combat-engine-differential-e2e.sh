#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
ORACLE_SOURCE="$ROOT_DIR/testing/e2e/combat-engine-oracle.php"
ORACLE_TARGET="/tmp/ogame-combat-engine-oracle.php"
REPORT="${OGAME_COMBAT_ENGINE_DIFFERENTIAL_REPORT:-$ROOT_DIR/.tmp/golang-combat-engine-differential.json}"
EXPECTED="${OGAME_COMBAT_ENGINE_DIFFERENTIAL_EXPECTED:-$ROOT_DIR/.tmp/golang-combat-engine-oracle.json}"

mkdir -p "$(dirname -- "$REPORT")"
mkdir -p "$(dirname -- "$EXPECTED")"
docker compose -f "$ROOT_DIR/docker-compose.yml" cp "$ORACLE_SOURCE" "server:$ORACLE_TARGET" >/dev/null
docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T server php "$ORACLE_TARGET" > "$EXPECTED"
(
  cd "$ROOT_DIR/backend"
  OGAME_COMBAT_ORACLE_EXPECTED_FILE="$EXPECTED" go test ./internal/domain/game -run '^TestLegacyCombatEngineOracle$' -count=1
)

jq '{
  pass:true,
  caseCount:(.cases|length),
  pairCaseCount:([.cases[]|select(.name|startswith("pair_"))]|length),
  rapidFireCaseCount:([.cases[]|select(.rapidFire)]|length),
  attackerRefirePairCount:([.cases[]|select(.name|startswith("pair_"))|select(.rapidFire)|select(any(.rounds[];.attackerShots>1))]|length),
  defenderRefirePairCount:([.cases[]|select(.name|startswith("pair_"))|select(.rapidFire)|select(any(.rounds[];.defenderShots>1))]|length),
  cases:[.cases[]|{name,rapidFire,outcome,rounds:(.rounds|length)}]
}' "$EXPECTED" > "$REPORT"
printf 'Go/PHP combat engine differential E2E: PASS (%s cases, %s pairwise, %s rapid-fire enabled, %s attacker/%s defender refire pairs)\n' \
  "$(jq -r '.caseCount' "$REPORT")" \
  "$(jq -r '.pairCaseCount' "$REPORT")" \
  "$(jq -r '.rapidFireCaseCount' "$REPORT")" \
  "$(jq -r '.attackerRefirePairCount' "$REPORT")" \
  "$(jq -r '.defenderRefirePairCount' "$REPORT")"
