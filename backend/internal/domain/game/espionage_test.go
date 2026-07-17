package game

import (
	"errors"
	"testing"
)

func TestResolveEspionageReportLevelsAndZeroCounterChance(t *testing.T) {
	rolls := []int{0, 0}
	result, err := ResolveEspionage(EspionageInput{
		AttackerTechnology: 8,
		DefenderTechnology: 3,
		AttackerFleet:      FleetCounts{FleetEspionageProbe: 2},
		DefenderFleet:      FleetCounts{},
	}, func(maximum int) int {
		roll := rolls[0]
		rolls = rolls[1:]
		if roll >= maximum {
			t.Fatalf("roll %d outside exclusive maximum %d", roll, maximum)
		}
		return roll
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ReportLevel != 26 || result.CounterChancePercent != 0 || result.Detected {
		t.Fatalf("unexpected espionage result: %+v", result)
	}
}

func TestResolveEspionageCounterChanceAndDetection(t *testing.T) {
	rolls := []int{50, 49}
	maxima := []int{}
	result, err := ResolveEspionage(EspionageInput{
		AttackerTechnology: 2,
		DefenderTechnology: 2,
		AttackerFleet:      FleetCounts{FleetEspionageProbe: 5},
		DefenderFleet:      FleetCounts{FleetLightFighter: 100},
	}, func(maximum int) int {
		maxima = append(maxima, maximum)
		roll := rolls[0]
		rolls = rolls[1:]
		return roll
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(maxima) != 2 || maxima[0] != 63 || maxima[1] != 101 {
		t.Fatalf("unexpected random bounds: %v", maxima)
	}
	if result.ReportLevel != 4 || result.CounterChancePercent != 50 || !result.Detected {
		t.Fatalf("unexpected espionage result: %+v", result)
	}
}

func TestResolveEspionageClampsRollsAndRequiresRandom(t *testing.T) {
	if _, err := ResolveEspionage(EspionageInput{}, nil); !errors.Is(err, ErrEspionageRandomUnavailable) {
		t.Fatalf("expected random source error, got %v", err)
	}

	rolls := []int{999, 999}
	result, err := ResolveEspionage(EspionageInput{
		AttackerFleet: FleetCounts{FleetEspionageProbe: 1},
		DefenderFleet: FleetCounts{FleetLightFighter: 1},
	}, func(int) int {
		roll := rolls[0]
		rolls = rolls[1:]
		return roll
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CounterChancePercent != 1 || result.Detected {
		t.Fatalf("unexpected clamped result: %+v", result)
	}
}
