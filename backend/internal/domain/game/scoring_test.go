package game

import "testing"

func TestCalculatePlanetScoreMatchesLegacyPlanetPrice(t *testing.T) {
	score := CalculatePlanetScore(
		BuildingLevels{
			BuildingMetalMine:       2,
			BuildingRoboticsFactory: 1,
		},
		FleetCounts{
			FleetSmallCargo:     3,
			FleetSolarSatellite: 1,
		},
		DefenseCounts{
			DefenseAntiBallisticMissile: 2,
		},
	)

	if score.Points != 35407 || score.FleetPoints != 4 {
		t.Fatalf("unexpected score: %+v", score)
	}
}

func TestCalculatePlanetScoreIgnoresMissingAndNegativeUnits(t *testing.T) {
	score := CalculatePlanetScore(
		BuildingLevels{BuildingMetalMine: 0},
		FleetCounts{FleetSmallCargo: -1},
		DefenseCounts{DefenseRocketLauncher: -2},
	)

	if score.Points != 0 || score.FleetPoints != 0 {
		t.Fatalf("expected zero score, got %+v", score)
	}
}
