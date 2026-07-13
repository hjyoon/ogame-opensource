package game

import "math"

type MissileAttackInput struct {
	Amount           int
	PrimaryDefenseID int
	MoonAttack       bool
	Target           DefenseCounts
	DefendingPlanet  DefenseCounts
	AttackerWeapon   int
	DefenderArmour   int
}

type MissileAttackResult struct {
	Target          DefenseCounts
	DefendingPlanet DefenseCounts
	Intercepted     int
}

func ResolveMissileAttack(input MissileAttackInput) MissileAttackResult {
	target := copyDefenseCounts(input.Target)
	defendingPlanet := copyDefenseCounts(input.DefendingPlanet)
	amount := max(0, input.Amount)
	interceptors := target[DefenseAntiBallisticMissile]
	if input.MoonAttack {
		interceptors = defendingPlanet[DefenseAntiBallisticMissile]
	}
	intercepted := min(amount, max(0, interceptors))
	if input.MoonAttack {
		defendingPlanet[DefenseAntiBallisticMissile] -= intercepted
	} else {
		target[DefenseAntiBallisticMissile] -= intercepted
	}

	remaining := amount - intercepted
	damage := float64(remaining*12000) * (1 + float64(input.AttackerWeapon)/10)
	if input.PrimaryDefenseID > 0 && remaining > 0 {
		damage = applyMissileDamage(target, input.PrimaryDefenseID, input.DefenderArmour, damage)
	}
	if damage > 0 {
		for _, id := range DefenseIDs() {
			if id == input.PrimaryDefenseID {
				continue
			}
			damage = applyMissileDamage(target, id, input.DefenderArmour, damage)
			if damage <= 0 {
				break
			}
		}
	}
	return MissileAttackResult{Target: target, DefendingPlanet: defendingPlanet, Intercepted: intercepted}
}

func applyMissileDamage(defense DefenseCounts, id int, armourLevel int, damage float64) float64 {
	count := max(0, defense[id])
	stats, found := CombatStatsForUnit(id)
	if count == 0 || !found || stats.Structure <= 0 {
		return damage
	}
	armour := float64(stats.Structure) * (1 + 0.1*float64(armourLevel)) / 10
	destroyed := min(int(math.Floor(damage/armour)), count)
	defense[id] -= destroyed
	return damage - float64(destroyed)*armour - float64(destroyed)
}

func copyDefenseCounts(source DefenseCounts) DefenseCounts {
	result := make(DefenseCounts, len(source))
	for id, count := range source {
		result[id] = max(0, count)
	}
	return result
}
