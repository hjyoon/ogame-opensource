package game

import "testing"

func TestBuildShipyardRequiresShipyard(t *testing.T) {
	shipyard := BuildShipyard(Overview{Commander: "legor"}, BuildingLevels{}, ResearchLevels{}, FleetCounts{}, 1, false, 1000)

	if shipyard.Commander != "legor" || shipyard.HasShipyard || len(shipyard.Items) != 0 {
		t.Fatalf("expected no-shipyard result, got %+v", shipyard)
	}
}

func TestBuildShipyardUsesLegacyFleetCostsAndDuration(t *testing.T) {
	overview := Overview{CurrentPlanet: PlanetOverview{
		Type: PlanetTypePlanet,
		Resources: Resources{
			Metal:     10000,
			Crystal:   10000,
			Deuterium: 10000,
		},
	}}
	shipyard := BuildShipyard(overview, BuildingLevels{BuildingShipyard: 2}, ResearchLevels{
		ResearchCombustionDrive: 2,
	}, FleetCounts{FleetSmallCargo: 3}, 2, false, 1000)

	smallCargo := shipyardItemByID(t, shipyard, FleetSmallCargo)
	if smallCargo.Count != 3 || smallCargo.Cost.Metal != 2000 || smallCargo.Cost.Crystal != 2000 {
		t.Fatalf("unexpected small cargo item: %+v", smallCargo)
	}
	if smallCargo.DurationSeconds != 960 || !smallCargo.CanBuild || smallCargo.MaxBuild != 5 {
		t.Fatalf("expected speed-adjusted buildable cargo, got %+v", smallCargo)
	}
}

func TestBuildShipyardKeepsOwnedUnavailableShipsVisible(t *testing.T) {
	overview := Overview{CurrentPlanet: PlanetOverview{Type: PlanetTypePlanet}}
	shipyard := BuildShipyard(overview, BuildingLevels{BuildingShipyard: 2}, ResearchLevels{}, FleetCounts{FleetHeavyFighter: 1}, 1, false, 1000)

	if hasShipyardItem(shipyard, FleetSmallCargo) {
		t.Fatalf("expected unavailable unowned small cargo to be hidden: %+v", shipyard.Items)
	}
	heavyFighter := shipyardItemByID(t, shipyard, FleetHeavyFighter)
	if heavyFighter.MeetsRequirement || heavyFighter.BlockedReason != "impossibly" || heavyFighter.CanBuild {
		t.Fatalf("expected owned unavailable heavy fighter to stay visible as blocked, got %+v", heavyFighter)
	}
}

func TestBuildShipyardUnlocksDeathstarAfterGravitonResearch(t *testing.T) {
	overview := Overview{CurrentPlanet: PlanetOverview{
		Type: PlanetTypePlanet,
		Resources: Resources{
			Metal:     5_000_000,
			Crystal:   4_000_000,
			Deuterium: 1_000_000,
		},
	}}
	levels := BuildingLevels{BuildingShipyard: 12}
	research := ResearchLevels{
		ResearchHyperspaceDrive: 7,
		ResearchHyperspace:      6,
	}

	locked := BuildShipyard(overview, levels, research, FleetCounts{}, 1, false, 1000)
	if hasShipyardItem(locked, FleetDeathstar) {
		t.Fatalf("deathstar must stay hidden before graviton research: %+v", locked.Items)
	}

	research[ResearchGraviton] = 1
	unlocked := BuildShipyard(overview, levels, research, FleetCounts{}, 1, false, 1000)
	deathstar := shipyardItemByID(t, unlocked, FleetDeathstar)
	if !deathstar.MeetsRequirement || !deathstar.CanBuild || deathstar.MaxBuild != 1 {
		t.Fatalf("expected graviton level one to unlock one affordable deathstar, got %+v", deathstar)
	}
}

func TestBuildShipyardBusyBlocksConstruction(t *testing.T) {
	overview := Overview{CurrentPlanet: PlanetOverview{Type: PlanetTypePlanet, Resources: Resources{Metal: 10000, Crystal: 10000}}}
	shipyard := BuildShipyard(overview, BuildingLevels{BuildingShipyard: 2}, ResearchLevels{ResearchCombustionDrive: 2}, FleetCounts{}, 1, true, 1000)

	smallCargo := shipyardItemByID(t, shipyard, FleetSmallCargo)
	if smallCargo.CanBuild || smallCargo.BlockedReason != "busy" {
		t.Fatalf("expected busy shipyard to block construction, got %+v", smallCargo)
	}
}

func TestBuildShipyardNormalizesSpeedAndAffordableCap(t *testing.T) {
	overview := Overview{CurrentPlanet: PlanetOverview{
		Type:      PlanetTypePlanet,
		Resources: Resources{Metal: -1, Crystal: 10000},
	}}
	shipyard := BuildShipyard(overview, BuildingLevels{BuildingShipyard: 2}, ResearchLevels{ResearchCombustionDrive: 2}, FleetCounts{}, 0, false, 0)

	smallCargo := shipyardItemByID(t, shipyard, FleetSmallCargo)
	if smallCargo.DurationSeconds != 1920 || smallCargo.MaxBuild != 0 || smallCargo.CanBuild {
		t.Fatalf("expected normalized speed and non-negative affordable cap, got %+v", smallCargo)
	}

	if max := maxAffordableUnits(Resources{Metal: -5}, BuildingCost{Metal: 1}, 1000); max != 0 {
		t.Fatalf("expected negative affordable amount to clamp to zero, got %d", max)
	}
}

func shipyardItemByID(t *testing.T, shipyard Shipyard, id int) ShipyardItem {
	t.Helper()
	for _, item := range shipyard.Items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("shipyard item %d not found in %+v", id, shipyard.Items)
	return ShipyardItem{}
}

func hasShipyardItem(shipyard Shipyard, id int) bool {
	for _, item := range shipyard.Items {
		if item.ID == id {
			return true
		}
	}
	return false
}
