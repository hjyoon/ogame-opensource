package mysqlgame

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type combatReportOracle struct {
	Cases []combatReportOracleCase `json:"cases"`
}

type combatReportOracleCase struct {
	Name   string `json:"name"`
	Report string `json:"report"`
}

func TestLegacyCombatReportOracle(t *testing.T) {
	raw := os.Getenv("OGAME_COMBAT_REPORT_ORACLE_EXPECTED")
	if raw == "" {
		t.Skip("legacy combat report oracle is only run by its differential wrapper")
	}
	var expected combatReportOracle
	if err := json.Unmarshal([]byte(raw), &expected); err != nil {
		t.Fatalf("decode PHP combat report oracle: %v", err)
	}

	actual := map[string]string{}
	twoRound := pirateCombatReportResult(2, domaingame.CombatDefenderWon)
	twoRound.Rounds[1].Attackers[0].Units = map[int]int{}
	actual["pirate_two_round_defender_win"] = combatExpeditionBattleReport(twoRound, 1_700_000_000)
	actual["pirate_six_round_draw"] = combatExpeditionBattleReport(
		pirateCombatReportResult(domaingame.CombatMaxRounds, domaingame.CombatDraw),
		1_700_000_000,
	)

	if len(actual) != len(expected.Cases) {
		t.Fatalf("combat report case count=%d, want %d", len(actual), len(expected.Cases))
	}
	for _, oracleCase := range expected.Cases {
		got, ok := actual[oracleCase.Name]
		if !ok {
			t.Fatalf("missing Go combat report case %q", oracleCase.Name)
		}
		if normalizeCombatReportTimestamp(got) != normalizeCombatReportTimestamp(oracleCase.Report) {
			t.Fatalf("%s combat report differs from PHP\nwant %s\n got %s", oracleCase.Name, oracleCase.Report, got)
		}
		if !strings.Contains(got, "The attacking fleet fires") ||
			!strings.Contains(got, "</table><p> ") ||
			(!strings.Contains(got, "The defender has won the battle!") && !strings.Contains(got, "battle ended in a draw")) {
			t.Fatalf("%s combat report is incomplete: %s", oracleCase.Name, got)
		}
	}
}

func pirateCombatReportResult(roundCount int, outcome domaingame.CombatOutcome) domaingame.CombatResult {
	attacker := domaingame.CombatSlot{
		Name: "ghost", Coords: domaingame.Coordinates{Galaxy: 1, System: 1, Position: 4},
		Weapon: 9, Shield: 7, Armour: 8, Units: map[int]int{domaingame.FleetLargeCargo: 1},
	}
	defender := domaingame.CombatSlot{
		Name: "Piraten", Coords: domaingame.Coordinates{Galaxy: 1, System: 1, Position: 16},
		Weapon: 6, Shield: 4, Armour: 5, Units: map[int]int{domaingame.FleetLightFighter: 5},
	}
	result := domaingame.CombatResult{Outcome: outcome}
	result.Before.Attackers = []domaingame.CombatSlot{attacker}
	result.Before.Defenders = []domaingame.CombatSlot{defender}
	for range roundCount {
		result.Rounds = append(result.Rounds, domaingame.CombatRound{
			AttackerShots: 1, AttackerPower: 10, DefenderAbsorbed: 10,
			DefenderShots: 5, DefenderPower: 400, AttackerAbsorbed: 42,
			Attackers: []domaingame.CombatRoundSlot{{Units: map[int]int{domaingame.FleetLargeCargo: 1}}},
			Defenders: []domaingame.CombatRoundSlot{{Units: map[int]int{domaingame.FleetLightFighter: 5}}},
		})
	}
	return result
}

var combatReportTimestamp = regexp.MustCompile(`^At \d{2}-\d{2} \d{2}:\d{2}:\d{2}`)

func normalizeCombatReportTimestamp(report string) string {
	return combatReportTimestamp.ReplaceAllString(report, "At <time>")
}
