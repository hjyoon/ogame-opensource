package game

import "math"

type CombatUnitStats struct {
	Structure int
	Shield    int
	Attack    int
}

var combatUnitStats = map[int]CombatUnitStats{
	FleetSmallCargo:              {Structure: 4000, Shield: 10, Attack: 5},
	FleetLargeCargo:              {Structure: 12000, Shield: 25, Attack: 5},
	FleetLightFighter:            {Structure: 4000, Shield: 10, Attack: 50},
	FleetHeavyFighter:            {Structure: 10000, Shield: 25, Attack: 150},
	FleetCruiser:                 {Structure: 27000, Shield: 50, Attack: 400},
	FleetBattleship:              {Structure: 60000, Shield: 200, Attack: 1000},
	FleetColonyShip:              {Structure: 30000, Shield: 100, Attack: 50},
	FleetRecycler:                {Structure: 16000, Shield: 10, Attack: 1},
	FleetEspionageProbe:          {Structure: 1000},
	FleetBomber:                  {Structure: 75000, Shield: 500, Attack: 1000},
	FleetSolarSatellite:          {Structure: 2000, Shield: 1, Attack: 1},
	FleetDestroyer:               {Structure: 110000, Shield: 500, Attack: 2000},
	FleetDeathstar:               {Structure: 9000000, Shield: 50000, Attack: 200000},
	FleetBattlecruiser:           {Structure: 70000, Shield: 400, Attack: 700},
	DefenseRocketLauncher:        {Structure: 2000, Shield: 20, Attack: 80},
	DefenseLightLaser:            {Structure: 2000, Shield: 25, Attack: 100},
	DefenseHeavyLaser:            {Structure: 8000, Shield: 100, Attack: 250},
	DefenseGaussCannon:           {Structure: 35000, Shield: 200, Attack: 1100},
	DefenseIonCannon:             {Structure: 8000, Shield: 500, Attack: 150},
	DefensePlasmaTurret:          {Structure: 100000, Shield: 300, Attack: 3000},
	DefenseSmallShieldDome:       {Structure: 20000, Shield: 2000, Attack: 1},
	DefenseLargeShieldDome:       {Structure: 100000, Shield: 10000, Attack: 1},
	DefenseAntiBallisticMissile:  {Structure: 8000, Shield: 1, Attack: 1},
	DefenseInterplanetaryMissile: {Structure: 15000, Shield: 1, Attack: 12000},
}

var combatUnitShortNames = map[int]string{
	FleetSmallCargo:        "S.Cargo",
	FleetLargeCargo:        "L.Cargo",
	FleetLightFighter:      "L.Fighter",
	FleetHeavyFighter:      "H.Fighter",
	FleetCruiser:           "Cruiser",
	FleetBattleship:        "Battleship",
	FleetColonyShip:        "Col. Ship",
	FleetRecycler:          "Recy.",
	FleetEspionageProbe:    "Esp.Probe",
	FleetBomber:            "Bomber",
	FleetSolarSatellite:    "Sol. Sat",
	FleetDestroyer:         "Dest.",
	FleetDeathstar:         "Deathstar",
	FleetBattlecruiser:     "Battlecr.",
	DefenseRocketLauncher:  "R.Launcher",
	DefenseLightLaser:      "L.Laser",
	DefenseHeavyLaser:      "H.Laser",
	DefenseGaussCannon:     "Gauss",
	DefenseIonCannon:       "Ion C.",
	DefensePlasmaTurret:    "Plasma",
	DefenseSmallShieldDome: "S.Dome",
	DefenseLargeShieldDome: "L.Dome",
}

func CombatStatsForUnit(id int) (CombatUnitStats, bool) {
	stats, ok := combatUnitStats[id]
	return stats, ok
}

func CombatShortName(id int) string {
	return combatUnitShortNames[id]
}

// BattlePlunder reproduces the legacy half-resource and cargo redistribution rules.
func BattlePlunder(cargo int, resources Resources) Resources {
	metal := maxFloat64(0, resources.Metal/2)
	crystal := maxFloat64(0, resources.Crystal/2)
	deuterium := maxFloat64(0, resources.Deuterium/2)
	remaining := maxFloat64(0, float64(cargo))

	metalCargo := minFloat64(metal, remaining/3)
	remaining -= metalCargo
	crystalCargo := minFloat64(crystal, remaining/2)
	remaining -= crystalCargo
	deuteriumCargo := minFloat64(deuterium, remaining)

	if deuterium < remaining {
		remaining -= deuteriumCargo
		metal -= metalCargo
		bonus := minFloat64(metal, remaining/2)
		metalCargo += bonus
		remaining -= bonus
		crystal -= crystalCargo
		crystalCargo += minFloat64(crystal, remaining)
	}

	return Resources{
		Metal:     math.Floor(metalCargo),
		Crystal:   math.Floor(crystalCargo),
		Deuterium: math.Floor(deuteriumCargo),
	}
}

// FleetAvailableCargo returns cargo remaining after fuel and loaded resources.
func FleetAvailableCargo(ships FleetCounts, loaded Resources, fuel int) int {
	total := 0
	for id, count := range ships {
		if count > 0 {
			total += FleetCargoCapacity(id) * count
		}
	}
	used := int(maxFloat64(0, loaded.Metal) + maxFloat64(0, loaded.Crystal) + maxFloat64(0, loaded.Deuterium))
	return max(0, total-used-max(0, fuel))
}

func minFloat64(left float64, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

func maxFloat64(left float64, right float64) float64 {
	if left > right {
		return left
	}
	return right
}
