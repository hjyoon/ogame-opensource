package game

import (
	"errors"
	"fmt"
	"math"
)

const CombatMaxRounds = 6

type CombatOutcome string

const (
	CombatAttackerWon CombatOutcome = "awon"
	CombatDefenderWon CombatOutcome = "dwon"
	CombatDraw        CombatOutcome = "draw"
)

type CombatSlot struct {
	ObjectID int
	PlayerID int
	Name     string
	Coords   Coordinates
	Planet   bool
	Weapon   int
	Shield   int
	Armour   int
	Units    map[int]int
}

type CombatRoundSlot struct {
	Units map[int]int
}

type CombatRound struct {
	AttackerShots    int
	AttackerPower    float64
	DefenderAbsorbed float64
	DefenderShots    int
	DefenderPower    float64
	AttackerAbsorbed float64
	Attackers        []CombatRoundSlot
	Defenders        []CombatRoundSlot
}

type CombatResult struct {
	Outcome CombatOutcome
	Before  struct {
		Attackers []CombatSlot
		Defenders []CombatSlot
	}
	Rounds []CombatRound
}

type combatUnit struct {
	id       int
	slot     int
	hull     int
	shield   int
	exploded bool
}

var combatRapidFire = map[int]map[int]int{
	FleetSmallCargo:    {FleetEspionageProbe: 5, FleetSolarSatellite: 5},
	FleetLargeCargo:    {FleetEspionageProbe: 5, FleetSolarSatellite: 5},
	FleetLightFighter:  {FleetEspionageProbe: 5, FleetSolarSatellite: 5},
	FleetHeavyFighter:  {FleetSmallCargo: 3, FleetEspionageProbe: 5, FleetSolarSatellite: 5},
	FleetCruiser:       {FleetLightFighter: 6, FleetEspionageProbe: 5, FleetSolarSatellite: 5, DefenseRocketLauncher: 10},
	FleetBattleship:    {FleetEspionageProbe: 5, FleetSolarSatellite: 5},
	FleetColonyShip:    {FleetEspionageProbe: 5, FleetSolarSatellite: 5},
	FleetRecycler:      {FleetEspionageProbe: 5, FleetSolarSatellite: 5},
	FleetBomber:        {FleetEspionageProbe: 5, FleetSolarSatellite: 5, DefenseRocketLauncher: 20, DefenseLightLaser: 20, DefenseHeavyLaser: 10, DefenseIonCannon: 10},
	FleetDestroyer:     {FleetEspionageProbe: 5, FleetSolarSatellite: 5, FleetBattlecruiser: 2, DefenseLightLaser: 10},
	FleetDeathstar:     {FleetSmallCargo: 250, FleetLargeCargo: 250, FleetLightFighter: 200, FleetHeavyFighter: 100, FleetCruiser: 33, FleetBattleship: 30, FleetColonyShip: 250, FleetRecycler: 250, FleetEspionageProbe: 1250, FleetBomber: 25, FleetSolarSatellite: 1250, FleetDestroyer: 5, FleetBattlecruiser: 15, DefenseRocketLauncher: 200, DefenseLightLaser: 200, DefenseHeavyLaser: 100, DefenseGaussCannon: 50, DefenseIonCannon: 100},
	FleetBattlecruiser: {FleetSmallCargo: 3, FleetLargeCargo: 3, FleetHeavyFighter: 4, FleetCruiser: 4, FleetBattleship: 7, FleetEspionageProbe: 5, FleetSolarSatellite: 5},
}

var combatUnitOrder = []int{
	FleetSmallCargo, FleetLargeCargo, FleetLightFighter, FleetHeavyFighter,
	FleetCruiser, FleetBattleship, FleetColonyShip, FleetRecycler,
	FleetEspionageProbe, FleetBomber, FleetSolarSatellite, FleetDestroyer,
	FleetDeathstar, FleetBattlecruiser, DefenseRocketLauncher, DefenseLightLaser,
	DefenseHeavyLaser, DefenseGaussCannon, DefenseIonCannon, DefensePlasmaTurret,
	DefenseSmallShieldDome, DefenseLargeShieldDome,
}

// ResolveCombat reproduces the legacy PHP round order. random returns [0,max).
func ResolveCombat(attackers []CombatSlot, defenders []CombatSlot, rapidFire bool, maxRounds int, random func(max int) int) (CombatResult, error) {
	result := CombatResult{}
	result.Before.Attackers = cloneCombatSlots(attackers)
	result.Before.Defenders = cloneCombatSlots(defenders)
	if random == nil {
		return result, errors.New("combat random source unavailable")
	}
	attackerUnits, err := initializeCombatUnits(attackers)
	if err != nil {
		return result, err
	}
	defenderUnits, err := initializeCombatUnits(defenders)
	if err != nil {
		return result, err
	}

	for roundIndex := 0; roundIndex < maxRounds; roundIndex++ {
		if len(attackerUnits) == 0 || len(defenderUnits) == 0 {
			break
		}
		chargeCombatShields(attackerUnits, attackers)
		chargeCombatShields(defenderUnits, defenders)
		round := CombatRound{}

		for slot := range attackers {
			for index := range attackerUnits {
				if attackerUnits[index].slot != slot {
					continue
				}
				refire := true
				for refire {
					target := boundedCombatRandom(random, len(defenderUnits))
					power := combatUnitShoot(&attackerUnits[index], attackers, &defenderUnits[target], defenders, random, &round.DefenderAbsorbed)
					round.AttackerShots++
					round.AttackerPower += power
					refire = rapidFire && combatRefires(attackerUnits[index].id, defenderUnits[target].id, random)
				}
			}
		}
		for slot := range defenders {
			for index := range defenderUnits {
				if defenderUnits[index].slot != slot {
					continue
				}
				refire := true
				for refire {
					target := boundedCombatRandom(random, len(attackerUnits))
					power := combatUnitShoot(&defenderUnits[index], defenders, &attackerUnits[target], attackers, random, &round.AttackerAbsorbed)
					round.DefenderShots++
					round.DefenderPower += power
					refire = rapidFire && combatRefires(defenderUnits[index].id, attackerUnits[target].id, random)
				}
			}
		}

		fastDraw := combatFastDraw(attackerUnits, attackers) && combatFastDraw(defenderUnits, defenders)
		attackerUnits = wipeCombatExploded(attackerUnits)
		defenderUnits = wipeCombatExploded(defenderUnits)
		round.Attackers = combatRoundSlots(attackerUnits, len(attackers))
		round.Defenders = combatRoundSlots(defenderUnits, len(defenders))
		result.Rounds = append(result.Rounds, round)
		if fastDraw {
			break
		}
	}

	switch {
	case len(attackerUnits) > 0 && len(defenderUnits) == 0:
		result.Outcome = CombatAttackerWon
	case len(defenderUnits) > 0 && len(attackerUnits) == 0:
		result.Outcome = CombatDefenderWon
	default:
		result.Outcome = CombatDraw
	}
	return result, nil
}

func initializeCombatUnits(slots []CombatSlot) ([]combatUnit, error) {
	units := []combatUnit{}
	for slotIndex, slot := range slots {
		for id, count := range slot.Units {
			if count < 0 {
				return nil, fmt.Errorf("negative combat unit count for %d", id)
			}
			if count > 0 {
				if _, ok := CombatStatsForUnit(id); !ok {
					return nil, fmt.Errorf("unknown combat unit %d", id)
				}
			}
		}
		for _, id := range combatUnitOrder {
			stats, _ := CombatStatsForUnit(id)
			hull := int(float64(stats.Structure) * 0.1 * float64(10+slot.Armour) / 10)
			for count := 0; count < slot.Units[id]; count++ {
				units = append(units, combatUnit{id: id, slot: slotIndex, hull: hull})
			}
		}
	}
	return units, nil
}

func chargeCombatShields(units []combatUnit, slots []CombatSlot) {
	for index := range units {
		if units[index].exploded {
			units[index].shield = 0
			continue
		}
		stats, _ := CombatStatsForUnit(units[index].id)
		units[index].shield = int(float64(stats.Shield) * float64(10+slots[units[index].slot].Shield) / 10)
	}
}

func combatUnitShoot(attacker *combatUnit, attackerSlots []CombatSlot, defender *combatUnit, defenderSlots []CombatSlot, random func(int) int, absorbed *float64) float64 {
	attackerStats, _ := CombatStatsForUnit(attacker.id)
	power := float64(attackerStats.Attack) * float64(10+attackerSlots[attacker.slot].Weapon) / 10
	if defender.exploded {
		return power
	}
	defenderStats, _ := CombatStatsForUnit(defender.id)
	if defender.shield == 0 {
		defender.hull = subtractPackedCombatValue(defender.hull, power)
	} else {
		shieldMax := float64(defenderStats.Shield) * float64(10+defenderSlots[defender.slot].Shield) / 10
		percent := shieldMax * 0.01
		depleted := math.Floor(power / percent)
		shieldDamage := depleted * percent
		if float64(defender.shield) < shieldDamage {
			*absorbed += float64(defender.shield)
			remaining := power - float64(defender.shield)
			defender.hull = subtractPackedCombatValue(defender.hull, remaining)
			defender.shield = 0
		} else {
			defender.shield = int(float64(defender.shield) - shieldDamage)
			*absorbed += power
		}
	}

	hullMax := float64(defenderStats.Structure) * 0.1 * float64(10+defenderSlots[defender.slot].Armour) / 10
	if float64(defender.hull) <= hullMax*0.7 && defender.shield == 0 {
		if defender.hull == 0 || boundedCombatRandom(random, 100) >= int(float64(defender.hull)*100/hullMax) {
			defender.exploded = true
		}
	}
	return power
}

func subtractPackedCombatValue(current int, damage float64) int {
	if damage >= float64(current) {
		return 0
	}
	return int(float64(current) - damage)
}

func combatFastDraw(units []combatUnit, slots []CombatSlot) bool {
	for _, unit := range units {
		stats, _ := CombatStatsForUnit(unit.id)
		hullMax := int(float64(stats.Structure) * 0.1 * float64(10+slots[unit.slot].Armour) / 10)
		if unit.hull != hullMax {
			return false
		}
	}
	return true
}

func wipeCombatExploded(units []combatUnit) []combatUnit {
	remaining := make([]combatUnit, 0, len(units))
	for _, unit := range units {
		if !unit.exploded {
			remaining = append(remaining, unit)
		}
	}
	return remaining
}

func combatRoundSlots(units []combatUnit, count int) []CombatRoundSlot {
	slots := make([]CombatRoundSlot, count)
	for index := range slots {
		slots[index].Units = map[int]int{}
	}
	for _, unit := range units {
		slots[unit.slot].Units[unit.id]++
	}
	return slots
}

func combatRefires(attackerID int, defenderID int, random func(int) int) bool {
	count := combatRapidFire[attackerID][defenderID]
	if count == 0 {
		return false
	}
	roll := boundedCombatRandom(random, 100000) + 1
	return float64(roll) > 100000/float64(count)
}

func boundedCombatRandom(random func(int) int, max int) int {
	value := random(max)
	if value < 0 {
		return 0
	}
	if value >= max {
		return max - 1
	}
	return value
}

func cloneCombatSlots(slots []CombatSlot) []CombatSlot {
	cloned := make([]CombatSlot, len(slots))
	copy(cloned, slots)
	for index := range cloned {
		cloned[index].Units = make(map[int]int, len(slots[index].Units))
		for id, count := range slots[index].Units {
			cloned[index].Units[id] = count
		}
	}
	return cloned
}
