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

	if score.Points != 35407 || score.FleetPoints != 4 || score.FleetCostPoints != 14500 || score.DefensePoints != 20000 {
		t.Fatalf("unexpected score: %+v", score)
	}
}

func TestCalculatePlanetScoreIgnoresMissingAndNegativeUnits(t *testing.T) {
	score := CalculatePlanetScore(
		BuildingLevels{BuildingMetalMine: 0},
		FleetCounts{FleetSmallCargo: -1},
		DefenseCounts{DefenseRocketLauncher: -2},
	)

	if score.Points != 0 || score.FleetPoints != 0 || score.FleetCostPoints != 0 || score.DefensePoints != 0 {
		t.Fatalf("expected zero score, got %+v", score)
	}
}

func TestBuildingScoreForLevelUsesLegacyPricePoints(t *testing.T) {
	score, ok := BuildingScoreForLevel(BuildingMetalMine, 2)
	if !ok || score != 112 {
		t.Fatalf("unexpected building score: score=%d ok=%v", score, ok)
	}
	if _, ok := BuildingScoreForLevel(9999, 1); ok {
		t.Fatal("unknown building should not have score")
	}
}

func TestUnitScoreForCountUsesLegacyUnitPricePoints(t *testing.T) {
	points, fleetPoints, ok := UnitScoreForCount(FleetLightFighter, 2)
	if !ok || points != 8000 || fleetPoints != 2 {
		t.Fatalf("unexpected fleet score: points=%d fleet=%d ok=%v", points, fleetPoints, ok)
	}

	points, fleetPoints, ok = UnitScoreForCount(DefenseRocketLauncher, 3)
	if !ok || points != 6000 || fleetPoints != 0 {
		t.Fatalf("unexpected defense score: points=%d fleet=%d ok=%v", points, fleetPoints, ok)
	}

	if _, _, ok := UnitScoreForCount(FleetLightFighter, 0); ok {
		t.Fatal("non-positive unit count should not have score")
	}
	if _, _, ok := UnitScoreForCount(9999, 1); ok {
		t.Fatal("unknown unit should not have score")
	}
}
