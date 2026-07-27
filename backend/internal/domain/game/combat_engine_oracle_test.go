package game

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type combatOracle struct {
	Cases []combatOracleCase `json:"cases"`
}

type combatOracleCase struct {
	Name        string                   `json:"name"`
	RapidFire   bool                     `json:"rapidFire"`
	RandomCalls []combatOracleRandomCall `json:"randomCalls"`
	Outcome     CombatOutcome            `json:"outcome"`
	Rounds      []combatOracleRound      `json:"rounds"`
}

type combatOracleRandomCall struct {
	Max   int `json:"max"`
	Value int `json:"value"`
}

type combatOracleInput struct {
	name      string
	attackers []CombatSlot
	defenders []CombatSlot
	rapidFire bool
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
	raw := loadCombatOracle(t)
	if raw == "" {
		t.Skip("legacy combat oracle is only run by its differential wrapper")
	}
	var expected combatOracle
	if err := json.Unmarshal([]byte(raw), &expected); err != nil {
		t.Fatalf("decode PHP combat oracle: %v", err)
	}

	cases := []combatOracleInput{
		{name: "unguarded", attackers: []CombatSlot{{Units: map[int]int{FleetSmallCargo: 1}}}, defenders: []CombatSlot{{Units: map[int]int{}}}},
		{name: "shield_fast_draw", attackers: []CombatSlot{{Units: map[int]int{FleetSmallCargo: 1}}}, defenders: []CombatSlot{{Units: map[int]int{FleetSmallCargo: 1}}}},
		{name: "deathstar_win", attackers: []CombatSlot{{Units: map[int]int{FleetDeathstar: 1}}}, defenders: []CombatSlot{{Units: map[int]int{FleetSmallCargo: 1}}}},
		{name: "plasma_defence_win", attackers: []CombatSlot{{Units: map[int]int{FleetLightFighter: 1}}}, defenders: []CombatSlot{{Units: map[int]int{DefensePlasmaTurret: 10}}}},
	}
	for _, rapidFire := range []bool{false, true} {
		for _, attackerID := range combatUnitOrder {
			for _, defenderID := range combatUnitOrder {
				cases = append(cases, combatOracleInput{
					name:      fmt.Sprintf("pair_%d_vs_%d_rapid_%t", attackerID, defenderID, rapidFire),
					attackers: []CombatSlot{{Units: map[int]int{attackerID: 1}}},
					defenders: []CombatSlot{{Units: map[int]int{defenderID: 1}}},
					rapidFire: rapidFire,
				})
			}
		}
	}
	if len(expected.Cases) != len(cases) {
		t.Fatalf("PHP returned %d combat cases, Go defines %d", len(expected.Cases), len(cases))
	}
	for index, test := range cases {
		expectedCase := expected.Cases[index]
		if expectedCase.Name != test.name || expectedCase.RapidFire != test.rapidFire {
			t.Fatalf("case %d metadata differs: PHP=%q/%t Go=%q/%t", index, expectedCase.Name, expectedCase.RapidFire, test.name, test.rapidFire)
		}
		randomIndex := 0
		replayed := make([]combatOracleRandomCall, 0, len(expectedCase.RandomCalls))
		random := func(max int) int {
			if randomIndex >= len(expectedCase.RandomCalls) {
				t.Fatalf("%s: Go requested an extra random value with max %d", test.name, max)
			}
			call := expectedCase.RandomCalls[randomIndex]
			randomIndex++
			if call.Max != max || call.Value < 0 || call.Value >= max {
				t.Fatalf("%s: random call %d differs: PHP=%+v Go max=%d", test.name, randomIndex, call, max)
			}
			replayed = append(replayed, call)
			return call.Value
		}
		result, err := ResolveCombat(test.attackers, test.defenders, test.rapidFire, CombatMaxRounds, random)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		if randomIndex != len(expectedCase.RandomCalls) {
			t.Fatalf("%s: Go consumed %d of %d PHP random calls", test.name, randomIndex, len(expectedCase.RandomCalls))
		}
		entry := combatOracleCase{
			Name: test.name, RapidFire: test.rapidFire, RandomCalls: replayed,
			Outcome: result.Outcome, Rounds: make([]combatOracleRound, 0, len(result.Rounds)),
		}
		for _, round := range result.Rounds {
			entry.Rounds = append(entry.Rounds, combatOracleRound{
				AttackerShots: round.AttackerShots, AttackerPower: round.AttackerPower, DefenderAbsorbed: round.DefenderAbsorbed,
				DefenderShots: round.DefenderShots, DefenderPower: round.DefenderPower, AttackerAbsorbed: round.AttackerAbsorbed,
				Attackers: oracleRoundUnits(round.Attackers), Defenders: oracleRoundUnits(round.Defenders),
			})
		}
		if !reflect.DeepEqual(entry, expectedCase) {
			want, _ := json.Marshal(expectedCase)
			got, _ := json.Marshal(entry)
			t.Fatalf("%s: combat engine differs from PHP\nwant %s\n got %s", test.name, want, got)
		}
	}
}

func loadCombatOracle(t *testing.T) string {
	t.Helper()
	if path := os.Getenv("OGAME_COMBAT_ORACLE_EXPECTED_FILE"); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read PHP combat oracle: %v", err)
		}
		return string(raw)
	}
	return os.Getenv("OGAME_COMBAT_ORACLE_EXPECTED")
}

func oracleRoundUnits(slots []CombatRoundSlot) []map[int]int {
	result := make([]map[int]int, len(slots))
	for index := range slots {
		result[index] = slots[index].Units
	}
	return result
}
