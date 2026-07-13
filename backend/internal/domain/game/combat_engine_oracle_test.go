package game

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type combatOracle struct {
	Cases []combatOracleCase `json:"cases"`
}

type combatOracleCase struct {
	Name    string              `json:"name"`
	Outcome CombatOutcome       `json:"outcome"`
	Rounds  []combatOracleRound `json:"rounds"`
}

type combatOracleRound struct {
	AttackerShots    int           `json:"attackerShots"`
	AttackerPower    float64       `json:"attackerPower"`
	DefenderAbsorbed float64       `json:"defenderAbsorbed"`
	DefenderShots    int           `json:"defenderShots"`
	DefenderPower    float64       `json:"defenderPower"`
	AttackerAbsorbed float64       `json:"attackerAbsorbed"`
	Attackers        []map[int]int `json:"attackers"`
	Defenders        []map[int]int `json:"defenders"`
}

func TestLegacyCombatEngineOracle(t *testing.T) {
	raw := os.Getenv("OGAME_COMBAT_ORACLE_EXPECTED")
	if raw == "" {
		t.Skip("legacy combat oracle is only run by its differential wrapper")
	}
	var expected combatOracle
	if err := json.Unmarshal([]byte(raw), &expected); err != nil {
		t.Fatalf("decode PHP combat oracle: %v", err)
	}

	cases := []struct {
		name      string
		attackers []CombatSlot
		defenders []CombatSlot
	}{
		{name: "unguarded", attackers: []CombatSlot{{Units: map[int]int{FleetSmallCargo: 1}}}, defenders: []CombatSlot{{Units: map[int]int{}}}},
		{name: "shield_fast_draw", attackers: []CombatSlot{{Units: map[int]int{FleetSmallCargo: 1}}}, defenders: []CombatSlot{{Units: map[int]int{FleetSmallCargo: 1}}}},
		{name: "deathstar_win", attackers: []CombatSlot{{Units: map[int]int{FleetDeathstar: 1}}}, defenders: []CombatSlot{{Units: map[int]int{FleetSmallCargo: 1}}}},
		{name: "plasma_defence_win", attackers: []CombatSlot{{Units: map[int]int{FleetLightFighter: 1}}}, defenders: []CombatSlot{{Units: map[int]int{DefensePlasmaTurret: 10}}}},
	}
	actual := combatOracle{Cases: make([]combatOracleCase, 0, len(cases))}
	for _, test := range cases {
		result, err := ResolveCombat(test.attackers, test.defenders, false, CombatMaxRounds, func(int) int { return 0 })
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		entry := combatOracleCase{Name: test.name, Outcome: result.Outcome, Rounds: make([]combatOracleRound, 0, len(result.Rounds))}
		for _, round := range result.Rounds {
			entry.Rounds = append(entry.Rounds, combatOracleRound{
				AttackerShots: round.AttackerShots, AttackerPower: round.AttackerPower, DefenderAbsorbed: round.DefenderAbsorbed,
				DefenderShots: round.DefenderShots, DefenderPower: round.DefenderPower, AttackerAbsorbed: round.AttackerAbsorbed,
				Attackers: oracleRoundUnits(round.Attackers), Defenders: oracleRoundUnits(round.Defenders),
			})
		}
		actual.Cases = append(actual.Cases, entry)
	}
	if !reflect.DeepEqual(actual, expected) {
		want, _ := json.Marshal(expected)
		got, _ := json.Marshal(actual)
		t.Fatalf("combat engine differs from PHP\nwant %s\n got %s", want, got)
	}
}

func oracleRoundUnits(slots []CombatRoundSlot) []map[int]int {
	result := make([]map[int]int, len(slots))
	for index := range slots {
		result[index] = slots[index].Units
	}
	return result
}
