package game

import (
	"errors"
	"math"
)

var ErrEspionageRandomUnavailable = errors.New("espionage random source unavailable")

type EspionageInput struct {
	AttackerTechnology int
	DefenderTechnology int
	AttackerFleet      FleetCounts
	DefenderFleet      FleetCounts
}

type EspionageResult struct {
	ReportLevel          int
	CounterChancePercent int
	Detected             bool
}

// ResolveEspionage reproduces the legacy SpyArrive report-level and counter-espionage rolls.
// random receives an exclusive upper bound, matching math/rand.IntN.
func ResolveEspionage(input EspionageInput, random func(int) int) (EspionageResult, error) {
	if random == nil {
		return EspionageResult{}, ErrEspionageRandomUnavailable
	}

	attackerShips := fleetCountTotal(input.AttackerFleet)
	defenderShips := fleetCountTotal(input.DefenderFleet)
	difference := input.AttackerTechnology - input.DefenderTechnology
	reportLevel := difference*espionageAbsInt(difference) - 1 + attackerShips

	structure := 0
	for id, count := range input.AttackerFleet {
		if count <= 0 {
			continue
		}
		stats, ok := CombatStatsForUnit(id)
		if ok {
			structure += stats.Structure * count
		}
	}

	maximum := math.Sqrt(math.Pow(2, float64(attackerShips-(reportLevel+1)))) *
		(float64(structure) / 1000 / 400 * math.Sqrt(float64(defenderShips)) * 5)
	maximum = math.Max(0, math.Min(2, maximum))
	maximumRoll := int(maximum * 100)
	counterRoll := clampEspionageRoll(random(maximumRoll+1), maximumRoll)
	counterChance := min(100, counterRoll)
	detected := clampEspionageRoll(random(101), 100) < counterChance

	return EspionageResult{
		ReportLevel:          reportLevel,
		CounterChancePercent: counterChance,
		Detected:             detected,
	}, nil
}

func fleetCountTotal(fleet FleetCounts) int {
	total := 0
	for _, count := range fleet {
		if count > 0 {
			total += count
		}
	}
	return total
}

func espionageAbsInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func clampEspionageRoll(value int, maximum int) int {
	if value < 0 {
		return 0
	}
	if value > maximum {
		return maximum
	}
	return value
}
