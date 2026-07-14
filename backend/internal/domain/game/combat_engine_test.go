package game

import (
	"strings"
	"testing"
)

func TestResolveCombatHandlesNoDefenders(t *testing.T) {
	attackers := []CombatSlot{{Units: map[int]int{FleetSmallCargo: 1}}}
	result, err := ResolveCombat(attackers, []CombatSlot{{Units: map[int]int{}}}, false, CombatMaxRounds, func(int) int { return 0 })
	if err != nil || result.Outcome != CombatAttackerWon || len(result.Rounds) != 0 {
		t.Fatalf("unexpected no-defender combat: %+v err=%v", result, err)
	}
	attackers[0].Units[FleetSmallCargo] = 9
	if result.Before.Attackers[0].Units[FleetSmallCargo] != 1 {
		t.Fatal("combat result must not alias caller slots")
	}
}

func TestResolveCombatFastDrawAfterShieldOnlyShots(t *testing.T) {
	slot := CombatSlot{Weapon: 0, Shield: 0, Armour: 0, Units: map[int]int{FleetSmallCargo: 1}}
	result, err := ResolveCombat([]CombatSlot{slot}, []CombatSlot{slot}, false, CombatMaxRounds, func(int) int { return 0 })
	if err != nil || result.Outcome != CombatDraw || len(result.Rounds) != 1 {
		t.Fatalf("unexpected fast draw: %+v err=%v", result, err)
	}
	round := result.Rounds[0]
	if round.AttackerShots != 1 || round.DefenderShots != 1 || round.AttackerPower != 5 || round.DefenderPower != 5 || round.AttackerAbsorbed != 5 || round.DefenderAbsorbed != 5 {
		t.Fatalf("unexpected fast-draw round: %+v", round)
	}
	if round.Attackers[0].Units[FleetSmallCargo] != 1 || round.Defenders[0].Units[FleetSmallCargo] != 1 {
		t.Fatalf("unexpected survivors: %+v", round)
	}
}

func TestResolveCombatDestroyedUnitStillReturnsFireSameRound(t *testing.T) {
	attacker := CombatSlot{Units: map[int]int{FleetDeathstar: 1}}
	defender := CombatSlot{Units: map[int]int{FleetSmallCargo: 1}}
	result, err := ResolveCombat([]CombatSlot{attacker}, []CombatSlot{defender}, false, CombatMaxRounds, func(int) int { return 0 })
	if err != nil || result.Outcome != CombatAttackerWon || len(result.Rounds) != 1 {
		t.Fatalf("unexpected one-round combat: %+v err=%v", result, err)
	}
	round := result.Rounds[0]
	if round.AttackerShots != 1 || round.AttackerPower != 200000 || round.DefenderShots != 1 || round.DefenderPower != 5 || round.AttackerAbsorbed != 5 {
		t.Fatalf("destroyed defender must still return fire: %+v", round)
	}
	if len(round.Defenders[0].Units) != 0 || round.Attackers[0].Units[FleetDeathstar] != 1 {
		t.Fatalf("unexpected one-round survivors: %+v", round)
	}
}

func TestResolveCombatRapidFireAndRandomBounds(t *testing.T) {
	rolls := []int{0, 99999, 0, 0, 0}
	index := 0
	random := func(max int) int {
		value := rolls[index%len(rolls)]
		index++
		return value
	}
	result, err := ResolveCombat(
		[]CombatSlot{{Units: map[int]int{FleetHeavyFighter: 1}}},
		[]CombatSlot{{Units: map[int]int{FleetSmallCargo: 2}}},
		true, 1, random,
	)
	if err != nil || len(result.Rounds) != 1 || result.Rounds[0].AttackerShots < 1 {
		t.Fatalf("unexpected rapid-fire combat: %+v err=%v", result, err)
	}
	if boundedCombatRandom(func(int) int { return -1 }, 10) != 0 || boundedCombatRandom(func(int) int { return 10 }, 10) != 9 {
		t.Fatal("combat random values must stay bounded")
	}
}

func TestResolveCombatRejectsInvalidInput(t *testing.T) {
	if _, err := ResolveCombat(nil, nil, false, 1, nil); err == nil || !strings.Contains(err.Error(), "random") {
		t.Fatalf("expected missing random error, got %v", err)
	}
	for _, units := range []map[int]int{{FleetSmallCargo: -1}, {999999: 1}} {
		_, err := ResolveCombat([]CombatSlot{{Units: units}}, nil, false, 1, func(int) int { return 0 })
		if err == nil {
			t.Fatalf("expected invalid units error for %+v", units)
		}
	}
}

func TestResolveCombatPreservesNonPositiveRoundLimits(t *testing.T) {
	slot := CombatSlot{Units: map[int]int{FleetDeathstar: 1}}
	for _, rounds := range []int{0, -1} {
		result, err := ResolveCombat([]CombatSlot{slot}, []CombatSlot{slot}, false, rounds, func(int) int { return 0 })
		if err != nil || result.Outcome != CombatDraw || len(result.Rounds) != 0 {
			t.Fatalf("rounds=%d should produce the legacy zero-round draw, result=%+v err=%v", rounds, result, err)
		}
	}
}
