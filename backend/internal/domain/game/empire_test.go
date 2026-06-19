package game

import "testing"

func TestBuildEmpireAggregatesLegacyRows(t *testing.T) {
	overview := Overview{
		Commander: "legor",
		CurrentPlanet: PlanetOverview{
			ID:          1,
			Name:        "Homeworld",
			Type:        PlanetTypePlanet,
			Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 3},
		},
		PlanetSwitcher: []PlanetSummary{{ID: 1, Name: "Homeworld", Type: PlanetTypePlanet}},
	}
	planets := []EmpirePlanet{
		{
			ID:          1,
			Name:        "Homeworld",
			Type:        PlanetTypePlanet,
			Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 3},
			Fields:      10,
			MaxFields:   160,
			Resources:   Resources{Metal: 1234.9, Crystal: 100, Deuterium: 50},
			Production:  EmpireProduction{MetalHourly: 30, CrystalHourly: 20, DeuteriumHourly: 10, EnergyBalance: -4, EnergyCapacity: 100},
			Levels:      BuildingLevels{BuildingMetalMine: 12, BuildingRoboticsFactory: 2},
			BuildQueue: map[int][]EmpireBuildQueueEntry{
				BuildingMetalMine: []EmpireBuildQueueEntry{
					{ListID: 1, Level: 13, Active: true},
					{ListID: 2, Level: 14},
				},
			},
			Fleet:   FleetCounts{FleetSmallCargo: 5},
			Defense: DefenseCounts{DefenseRocketLauncher: 7},
		},
		{
			ID:          2,
			Name:        "Colony",
			Type:        PlanetTypePlanet,
			Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 4},
			Fields:      6,
			MaxFields:   140,
			Resources:   Resources{Metal: 10, Crystal: 20, Deuterium: 30},
			Production:  EmpireProduction{MetalHourly: 11, CrystalHourly: 9, DeuteriumHourly: 5, EnergyBalance: 6, EnergyCapacity: 80},
			Levels:      BuildingLevels{BuildingMetalMine: 4},
			Fleet:       FleetCounts{FleetSmallCargo: 1},
			Defense:     DefenseCounts{},
		},
	}
	empire := BuildEmpire(overview, true, EmpirePlanetTypeMoons, false, true, planets, ResearchLevels{ResearchComputer: 3})

	if empire.PlanetType != EmpirePlanetTypePlanets || !empire.CommanderActive || !empire.HasMoons {
		t.Fatalf("unexpected empire metadata: %+v", empire)
	}
	metal := findEmpireResourceRow(t, empire.Resources, ResourceMetal)
	if metal.Total != 1244 || metal.Production != 21 || len(metal.Values) != 2 {
		t.Fatalf("unexpected metal row: %+v", metal)
	}
	energy := findEmpireResourceRow(t, empire.Resources, ResourceEnergy)
	if energy.Total != 2 || energy.Production != 180 {
		t.Fatalf("unexpected energy row: %+v", energy)
	}
	metalMine := findEmpireLevelRow(t, empire.Buildings, BuildingMetalMine)
	if metalMine.Total != 16 || metalMine.Average != 8 {
		t.Fatalf("unexpected building row: %+v", metalMine)
	}
	if queue := metalMine.Values[0].Queue; len(queue) != 2 || !queue[0].Active || queue[0].Level != 13 || queue[1].ListID != 2 {
		t.Fatalf("unexpected building queue values: %+v", queue)
	}
	research := findEmpireLevelRow(t, empire.Research, ResearchComputer)
	if research.Total != 3 || research.Average != 3 || research.Values[1].Level != 3 {
		t.Fatalf("unexpected research row: %+v", research)
	}
	if len(research.Values[0].Queue) != 0 {
		t.Fatalf("research rows must not inherit building queue values: %+v", research.Values[0].Queue)
	}
	fleet := findEmpireCountRow(t, empire.Fleet, FleetSmallCargo)
	if fleet.Total != 6 {
		t.Fatalf("unexpected fleet row: %+v", fleet)
	}
	defense := findEmpireCountRow(t, empire.Defense, DefenseRocketLauncher)
	if defense.Total != 7 {
		t.Fatalf("unexpected defense row: %+v", defense)
	}
	if issue := EmpireActionIssueFor(EmpireIssueCommanderRequired); issue.Code != EmpireIssueCommanderRequired || issue.Message == "" {
		t.Fatalf("unexpected issue: %+v", issue)
	}
	if issue := EmpireActionIssueFor("unknown"); issue.Code != "unknown" || issue.Message == "" {
		t.Fatalf("unexpected fallback issue: %+v", issue)
	}
	if TechnologyName(BuildingMetalMine) == "" || TechnologyName(999999) != "" {
		t.Fatalf("unexpected technology name lookup")
	}
	if floorResource(-1) != 0 {
		t.Fatalf("negative resources should floor to zero")
	}
}

func findEmpireResourceRow(t *testing.T, rows []EmpireResourceRow, id int) EmpireResourceRow {
	t.Helper()
	for _, row := range rows {
		if row.ID == id {
			return row
		}
	}
	t.Fatalf("missing resource row %d in %+v", id, rows)
	return EmpireResourceRow{}
}

func findEmpireLevelRow(t *testing.T, rows []EmpireLevelRow, id int) EmpireLevelRow {
	t.Helper()
	for _, row := range rows {
		if row.ID == id {
			return row
		}
	}
	t.Fatalf("missing level row %d in %+v", id, rows)
	return EmpireLevelRow{}
}

func findEmpireCountRow(t *testing.T, rows []EmpireCountRow, id int) EmpireCountRow {
	t.Helper()
	for _, row := range rows {
		if row.ID == id {
			return row
		}
	}
	t.Fatalf("missing count row %d in %+v", id, rows)
	return EmpireCountRow{}
}
