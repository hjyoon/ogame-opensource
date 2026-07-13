package game

import "math"

const (
	MoonDestroyMoon  = 1
	MoonDestroyFleet = 2
)

type MoonDestructionOdds struct {
	MoonPercent  float64
	FleetPercent float64
}

func BattleMoonChance(debris Resources) int {
	chance := int(math.Floor((maxFloat64(0, debris.Metal) + maxFloat64(0, debris.Crystal)) / 100000))
	return min(20, chance)
}

func BattleMoonDiameter(chance int, sizeRoll int) int {
	chance = max(0, chance)
	sizeRoll = min(20, max(10, sizeRoll))
	return int(math.Floor(1000 * math.Sqrt(float64(sizeRoll+3*chance))))
}

func MoonDestructionChances(diameter int, deathstars int) MoonDestructionOdds {
	diameter = max(0, diameter)
	deathstars = max(0, deathstars)
	rootDiameter := math.Sqrt(float64(diameter))
	moon := (100 - rootDiameter) * math.Sqrt(float64(deathstars))
	if moon >= 100 {
		moon = 99.9
	}
	return MoonDestructionOdds{MoonPercent: moon, FleetPercent: rootDiameter / 2}
}

func ResolveMoonDestruction(odds MoonDestructionOdds, moonRoll int, fleetRoll int) int {
	result := 0
	if moonRoll >= 1 && moonRoll <= 999 && float64(moonRoll) < odds.MoonPercent*10 {
		result |= MoonDestroyMoon
	}
	if fleetRoll >= 1 && fleetRoll <= 999 && float64(fleetRoll) < odds.FleetPercent*10 {
		result |= MoonDestroyFleet
	}
	return result
}
