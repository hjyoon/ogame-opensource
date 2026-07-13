package game

import "math"

type CombatParticipantLoss struct {
	Points     int64
	FleetUnits int64
}

type CombatWriteback struct {
	AttackerSurvivors []map[int]int
	DefenderSurvivors []map[int]int
	AttackerLosses    []CombatParticipantLoss
	DefenderLosses    []CombatParticipantLoss
	Debris            Resources
}

func RepairCombatDefense(result CombatResult, repairPercent int, repairDelta int, engineers []bool, random func(max int) int) []map[int]int {
	repaired := make([]map[int]int, len(result.Before.Defenders))
	for slot, defender := range result.Before.Defenders {
		repaired[slot] = map[int]int{}
		if !defender.Planet || len(result.Rounds) == 0 {
			continue
		}
		last := result.Rounds[len(result.Rounds)-1].Defenders[slot].Units
		for _, id := range combatUnitOrder {
			if !isCombatDefense(id) {
				continue
			}
			exploded := max(0, defender.Units[id]-last[id])
			if slot < len(engineers) && engineers[slot] {
				exploded /= 2
			}
			if exploded < 10 {
				for roll := 0; roll < exploded; roll++ {
					if boundedCombatRandom(random, 100) < repairPercent {
						repaired[slot][id]++
					}
				}
				continue
			}
			low := repairPercent - repairDelta
			high := repairPercent + repairDelta
			if high < low {
				low, high = high, low
			}
			percent := low + boundedCombatRandom(random, high-low+1)
			repaired[slot][id] = int(math.Floor(float64(percent*exploded) / 100))
		}
	}
	return repaired
}

// BuildCombatWriteback applies repaired defence and legacy fleet/defence debris factors.
func BuildCombatWriteback(result CombatResult, repaired []map[int]int, fleetDebrisPercent int, defenseDebrisPercent int) CombatWriteback {
	rawDefenderSurvivors := combatResultSurvivors(result.Before.Defenders, result.Rounds, false)
	writeback := CombatWriteback{
		AttackerSurvivors: combatResultSurvivors(result.Before.Attackers, result.Rounds, true),
		DefenderSurvivors: combatResultSurvivors(result.Before.Defenders, result.Rounds, false),
		AttackerLosses:    make([]CombatParticipantLoss, len(result.Before.Attackers)),
		DefenderLosses:    make([]CombatParticipantLoss, len(result.Before.Defenders)),
	}
	for slot := range writeback.DefenderSurvivors {
		if slot >= len(repaired) {
			continue
		}
		for id, count := range repaired[slot] {
			if isCombatDefense(id) && count > 0 {
				writeback.DefenderSurvivors[slot][id] += count
			}
		}
	}
	for slot, before := range result.Before.Attackers {
		writeback.AttackerLosses[slot] = combatSlotLoss(before.Units, writeback.AttackerSurvivors[slot])
		addCombatDebris(&writeback.Debris, before.Units, writeback.AttackerSurvivors[slot], fleetDebrisPercent, defenseDebrisPercent)
	}
	for slot, before := range result.Before.Defenders {
		// Legacy score and report losses count defenses destroyed in battle, even
		// when those defenses are subsequently repaired for planet writeback.
		writeback.DefenderLosses[slot] = combatSlotLoss(before.Units, rawDefenderSurvivors[slot])
		addCombatDebris(&writeback.Debris, before.Units, writeback.DefenderSurvivors[slot], fleetDebrisPercent, defenseDebrisPercent)
	}
	return writeback
}

func combatResultSurvivors(before []CombatSlot, rounds []CombatRound, attackers bool) []map[int]int {
	result := make([]map[int]int, len(before))
	for slot := range result {
		result[slot] = map[int]int{}
		source := before[slot].Units
		if len(rounds) > 0 {
			last := rounds[len(rounds)-1]
			if attackers {
				source = last.Attackers[slot].Units
			} else {
				source = last.Defenders[slot].Units
			}
		}
		for id, count := range source {
			if count > 0 {
				result[slot][id] = count
			}
		}
	}
	return result
}

func combatSlotLoss(before map[int]int, after map[int]int) CombatParticipantLoss {
	var loss CombatParticipantLoss
	for id, initial := range before {
		lost := max(0, initial-after[id])
		if lost == 0 {
			continue
		}
		cost, ok := CombatUnitCost(id)
		if !ok {
			continue
		}
		loss.Points += scoreCost(cost) * int64(lost)
		if isCombatFleet(id) {
			loss.FleetUnits += int64(lost)
		}
	}
	return loss
}

func addCombatDebris(total *Resources, before map[int]int, after map[int]int, fleetPercent int, defensePercent int) {
	for id, initial := range before {
		lost := max(0, initial-after[id])
		if lost == 0 {
			continue
		}
		cost, ok := CombatUnitCost(id)
		if !ok {
			continue
		}
		percent := fleetPercent
		if isCombatDefense(id) {
			percent = defensePercent
		}
		total.Metal += math.Ceil(cost.Metal * float64(lost) * float64(percent) / 100)
		total.Crystal += math.Ceil(cost.Crystal * float64(lost) * float64(percent) / 100)
	}
}

func isCombatFleet(id int) bool {
	return id >= FleetSmallCargo && id <= FleetBattlecruiser
}

func isCombatDefense(id int) bool {
	return id >= DefenseRocketLauncher && id <= DefenseLargeShieldDome
}
