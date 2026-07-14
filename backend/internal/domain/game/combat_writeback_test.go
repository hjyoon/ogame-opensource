package game

import "testing"

func TestBuildCombatWritebackFleetLossAndDebris(t *testing.T) {
	result, err := ResolveCombat(
		[]CombatSlot{{Units: map[int]int{FleetDeathstar: 1}}},
		[]CombatSlot{{Units: map[int]int{FleetSmallCargo: 1}}},
		false, CombatMaxRounds, func(int) int { return 0 },
	)
	if err != nil {
		t.Fatal(err)
	}
	writeback := BuildCombatWriteback(result, nil, 30, 0)
	if writeback.AttackerSurvivors[0][FleetDeathstar] != 1 || len(writeback.DefenderSurvivors[0]) != 0 {
		t.Fatalf("unexpected survivors: %+v", writeback)
	}
	if writeback.DefenderLosses[0].Points != 4000 || writeback.DefenderLosses[0].FleetUnits != 1 {
		t.Fatalf("unexpected defender losses: %+v", writeback.DefenderLosses)
	}
	if writeback.Debris.Metal != 600 || writeback.Debris.Crystal != 600 {
		t.Fatalf("unexpected fleet debris: %+v", writeback.Debris)
	}
}

func TestBuildCombatWritebackAppliesRepairedDefense(t *testing.T) {
	result := CombatResult{Outcome: CombatAttackerWon}
	result.Before.Attackers = []CombatSlot{{Units: map[int]int{FleetLightFighter: 1}}}
	result.Before.Defenders = []CombatSlot{{Units: map[int]int{DefenseRocketLauncher: 1}}}
	result.Rounds = []CombatRound{{
		Attackers: []CombatRoundSlot{{Units: map[int]int{FleetLightFighter: 1}}},
		Defenders: []CombatRoundSlot{{Units: map[int]int{}}},
	}}
	writeback := BuildCombatWriteback(result, []map[int]int{{DefenseRocketLauncher: 1}}, 30, 30)
	if writeback.DefenderSurvivors[0][DefenseRocketLauncher] != 1 {
		t.Fatalf("expected repaired launcher survivor: %+v", writeback.DefenderSurvivors)
	}
	if writeback.DefenderLosses[0].Points != 2000 || writeback.DefenderLosses[0].FleetUnits != 0 || writeback.Debris.Metal != 0 || writeback.Debris.Crystal != 0 {
		t.Fatalf("unexpected repaired defense writeback: %+v", writeback)
	}
}

func TestCombatUnitCostAndWritebackEdges(t *testing.T) {
	if cost, ok := CombatUnitCost(FleetBattleship); !ok || cost.Metal != 45000 || cost.Crystal != 15000 {
		t.Fatalf("unexpected battleship cost: %+v ok=%v", cost, ok)
	}
	if _, ok := CombatUnitCost(999999); ok {
		t.Fatal("unknown combat cost should fail")
	}
	result := CombatResult{Outcome: CombatDraw}
	result.Before.Attackers = []CombatSlot{{Units: map[int]int{999999: 1, FleetSmallCargo: 0}}}
	result.Before.Defenders = []CombatSlot{{Units: map[int]int{}}}
	writeback := BuildCombatWriteback(result, nil, 30, 30)
	if writeback.AttackerLosses[0].Points != 0 || writeback.Debris.Metal != 0 {
		t.Fatalf("unknown and zero units must not affect writeback: %+v", writeback)
	}

	result.Rounds = []CombatRound{{
		Attackers: []CombatRoundSlot{{Units: map[int]int{}}},
		Defenders: []CombatRoundSlot{{Units: map[int]int{}}},
	}}
	writeback = BuildCombatWriteback(result, nil, 30, 30)
	if writeback.AttackerLosses[0].Points != 0 || writeback.Debris.Metal != 0 {
		t.Fatalf("destroyed unknown units must be ignored safely: %+v", writeback)
	}

	debris := Resources{}
	addCombatDebris(&debris, map[int]int{DefenseRocketLauncher: 1}, map[int]int{}, 0, 30)
	if debris.Metal != 600 || debris.Crystal != 0 {
		t.Fatalf("defense debris must use the defense factor: %+v", debris)
	}
}

func TestRepairCombatDefenseMatchesLegacyBranches(t *testing.T) {
	result := CombatResult{Outcome: CombatAttackerWon}
	result.Before.Defenders = []CombatSlot{
		{Planet: true, Units: map[int]int{DefenseRocketLauncher: 2, DefenseLightLaser: 20}},
		{Planet: false, Units: map[int]int{DefenseRocketLauncher: 5}},
	}
	result.Rounds = []CombatRound{{Defenders: []CombatRoundSlot{
		{Units: map[int]int{}},
		{Units: map[int]int{}},
	}}}
	rolls := []int{0, 99, 10}
	index := 0
	repaired := RepairCombatDefense(result, 70, 10, nil, func(int) int {
		value := rolls[index]
		index++
		return value
	})
	if repaired[0][DefenseRocketLauncher] != 1 || repaired[0][DefenseLightLaser] != 14 || len(repaired[1]) != 0 {
		t.Fatalf("unexpected repaired defenses: %+v", repaired)
	}

	result.Before.Defenders = result.Before.Defenders[:1]
	result.Rounds[0].Defenders = result.Rounds[0].Defenders[:1]
	repaired = RepairCombatDefense(result, 100, 0, []bool{true}, func(int) int { return 0 })
	if repaired[0][DefenseRocketLauncher] != 1 || repaired[0][DefenseLightLaser] != 10 {
		t.Fatalf("engineer should halve exploded defenses before repair: %+v", repaired)
	}
}

func TestRepairCombatDefenseHandlesNoRoundsAndReversedRange(t *testing.T) {
	result := CombatResult{}
	result.Before.Defenders = []CombatSlot{{Planet: true, Units: map[int]int{DefenseRocketLauncher: 10}}}
	if repaired := RepairCombatDefense(result, 70, 10, nil, func(int) int { return 0 }); len(repaired[0]) != 0 {
		t.Fatalf("no-round combat should not repair: %+v", repaired)
	}
	result.Rounds = []CombatRound{{Defenders: []CombatRoundSlot{{Units: map[int]int{}}}}}
	repaired := RepairCombatDefense(result, 10, -20, nil, func(max int) int { return max - 1 })
	if repaired[0][DefenseRocketLauncher] != 3 {
		t.Fatalf("reversed repair range should be normalized: %+v", repaired)
	}
}
