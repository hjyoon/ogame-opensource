package game

import "testing"

func TestBuildDefenseRequiresShipyard(t *testing.T) {
	defense := BuildDefense(Overview{Commander: "legor"}, BuildingLevels{}, ResearchLevels{}, DefenseCounts{}, 1, false, 1000)

	if defense.Commander != "legor" || defense.HasShipyard || len(defense.Items) != 0 {
		t.Fatalf("expected no-shipyard defense result, got %+v", defense)
	}
}

func TestBuildDefenseUsesLegacyCostAndDuration(t *testing.T) {
	overview := Overview{CurrentPlanet: PlanetOverview{
		Type:      PlanetTypePlanet,
		Resources: Resources{Metal: 10000, Crystal: 10000, Deuterium: 10000},
	}}
	defense := BuildDefense(overview, BuildingLevels{BuildingShipyard: 1}, ResearchLevels{}, DefenseCounts{DefenseRocketLauncher: 2}, 2, false, 1000)

	launcher := defenseItemByID(t, defense, DefenseRocketLauncher)
	if launcher.Count != 2 || launcher.Cost.Metal != 2000 || launcher.DurationSeconds != 720 {
		t.Fatalf("unexpected rocket launcher item: %+v", launcher)
	}
	if !launcher.CanBuild || launcher.MaxBuild != 5 {
		t.Fatalf("expected affordable launcher, got %+v", launcher)
	}

	defaultSpeed := BuildDefense(overview, BuildingLevels{BuildingShipyard: 1}, ResearchLevels{}, DefenseCounts{}, 0, false, 1000)
	if defenseItemByID(t, defaultSpeed, DefenseRocketLauncher).DurationSeconds != 1440 {
		t.Fatalf("expected non-positive speed to default to one, got %+v", defaultSpeed)
	}
}

func TestBuildDefenseKeepsOwnedUnavailableDefenseVisible(t *testing.T) {
	overview := Overview{CurrentPlanet: PlanetOverview{Type: PlanetTypePlanet}}
	defense := BuildDefense(overview, BuildingLevels{BuildingShipyard: 1}, ResearchLevels{}, DefenseCounts{DefenseLightLaser: 1}, 1, false, 1000)

	if hasDefenseItem(defense, DefenseHeavyLaser) {
		t.Fatalf("expected unavailable unowned heavy laser to be hidden, got %+v", defense.Items)
	}
	lightLaser := defenseItemByID(t, defense, DefenseLightLaser)
	if lightLaser.MeetsRequirement || lightLaser.BlockedReason != "impossibly" || lightLaser.CanBuild {
		t.Fatalf("expected owned unavailable light laser to stay visible as blocked, got %+v", lightLaser)
	}
}

func TestBuildDefenseBlocksExistingShieldDome(t *testing.T) {
	overview := Overview{CurrentPlanet: PlanetOverview{Type: PlanetTypePlanet, Resources: Resources{Metal: 100000, Crystal: 100000}}}
	defense := BuildDefense(overview, BuildingLevels{BuildingShipyard: 1}, ResearchLevels{ResearchShield: 2}, DefenseCounts{DefenseSmallShieldDome: 1}, 1, false, 1000)

	dome := defenseItemByID(t, defense, DefenseSmallShieldDome)
	if dome.CanBuild || dome.MaxBuild != 0 || dome.BlockedReason != "A shield dome can only be built 1 time." {
		t.Fatalf("expected existing dome to block another dome, got %+v", dome)
	}

	empty := BuildDefense(overview, BuildingLevels{BuildingShipyard: 1}, ResearchLevels{ResearchShield: 2}, DefenseCounts{}, 1, false, 1000)
	if dome := defenseItemByID(t, empty, DefenseSmallShieldDome); !dome.CanBuild || dome.MaxBuild != 1 {
		t.Fatalf("expected empty dome slot to cap max build at one, got %+v", dome)
	}
}

func TestBuildDefenseCapsMissilesBySiloSpace(t *testing.T) {
	overview := Overview{CurrentPlanet: PlanetOverview{Type: PlanetTypePlanet, Resources: Resources{Metal: 1000000, Crystal: 1000000, Deuterium: 1000000}}}
	defense := BuildDefense(overview, BuildingLevels{BuildingShipyard: 1, BuildingMissileSilo: 4}, ResearchLevels{ResearchImpulseDrive: 1}, DefenseCounts{
		DefenseAntiBallisticMissile:  3,
		DefenseInterplanetaryMissile: 4,
	}, 1, false, 1000)

	abm := defenseItemByID(t, defense, DefenseAntiBallisticMissile)
	ipm := defenseItemByID(t, defense, DefenseInterplanetaryMissile)
	if abm.MaxBuild != 29 || ipm.MaxBuild != 14 {
		t.Fatalf("expected missile caps from silo free space, got abm=%+v ipm=%+v", abm, ipm)
	}
}

func TestBuildDefenseBlocksMissilesWhenSiloIsFull(t *testing.T) {
	overview := Overview{CurrentPlanet: PlanetOverview{Type: PlanetTypePlanet, Resources: Resources{Metal: 100000, Crystal: 100000, Deuterium: 100000}}}
	defense := BuildDefense(overview, BuildingLevels{BuildingShipyard: 1, BuildingMissileSilo: 2}, ResearchLevels{}, DefenseCounts{
		DefenseAntiBallisticMissile: 20,
	}, 1, false, 1000)

	abm := defenseItemByID(t, defense, DefenseAntiBallisticMissile)
	if abm.CanBuild || abm.MaxBuild != 0 {
		t.Fatalf("expected full missile silo to block more missiles, got %+v", abm)
	}
}

func TestMaxDefenseUnitsBlocksInterplanetaryMissileWhenOnlyOneSlotRemains(t *testing.T) {
	got := maxDefenseUnits(
		Resources{Metal: 100000, Crystal: 100000, Deuterium: 100000},
		BuildingCost{Metal: 12500, Crystal: 2500, Deuterium: 10000},
		BuildingLevels{BuildingMissileSilo: 1},
		DefenseCounts{DefenseAntiBallisticMissile: 9},
		DefenseInterplanetaryMissile,
		1000,
	)

	if got != 0 {
		t.Fatalf("expected one remaining silo slot to block two-slot IPM, got %d", got)
	}
}

func TestBuildDefenseBusyBlocksConstruction(t *testing.T) {
	overview := Overview{CurrentPlanet: PlanetOverview{Type: PlanetTypePlanet, Resources: Resources{Metal: 10000}}}
	defense := BuildDefense(overview, BuildingLevels{BuildingShipyard: 1}, ResearchLevels{}, DefenseCounts{}, 1, true, 1000)

	launcher := defenseItemByID(t, defense, DefenseRocketLauncher)
	if launcher.CanBuild || launcher.BlockedReason != "busy" {
		t.Fatalf("expected busy shipyard to block defense construction, got %+v", launcher)
	}
}

func defenseItemByID(t *testing.T, defense Defense, id int) ShipyardItem {
	t.Helper()
	for _, item := range defense.Items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("defense item %d not found in %+v", id, defense.Items)
	return ShipyardItem{}
}

func hasDefenseItem(defense Defense, id int) bool {
	for _, item := range defense.Items {
		if item.ID == id {
			return true
		}
	}
	return false
}
