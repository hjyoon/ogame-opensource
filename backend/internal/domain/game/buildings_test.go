package game

import "testing"

func TestBuildBuildingsUsesLegacyCostAndDurationFormula(t *testing.T) {
	overview := Overview{
		Commander: "legor",
		CurrentPlanet: PlanetOverview{
			ID:        99,
			Name:      "Arakis",
			Type:      PlanetTypePlanet,
			Fields:    1,
			MaxFields: 163,
			Resources: Resources{
				Metal:     10000,
				Crystal:   10000,
				Deuterium: 10000,
			},
		},
	}
	buildings := BuildBuildings(overview, BuildingLevels{
		BuildingMetalMine:       2,
		BuildingRoboticsFactory: 1,
	}, ResearchLevels{}, 1)

	metalMine := findBuilding(t, buildings, BuildingMetalMine)
	if metalMine.Level != 2 || metalMine.NextLevel != 3 {
		t.Fatalf("unexpected metal mine level: %+v", metalMine)
	}
	if metalMine.Cost.Metal != 135 || metalMine.Cost.Crystal != 33.75 {
		t.Fatalf("unexpected metal mine cost: %+v", metalMine.Cost)
	}
	if metalMine.DurationSeconds != 121 {
		t.Fatalf("expected legacy duration floor to 121 seconds, got %d", metalMine.DurationSeconds)
	}
	if !metalMine.CanBuild {
		t.Fatalf("expected affordable metal mine: %+v", metalMine)
	}
}

func TestBuildBuildingsFiltersByRequirementsAndPlanetType(t *testing.T) {
	planet := Overview{CurrentPlanet: PlanetOverview{Type: PlanetTypePlanet, MaxFields: 10, Resources: Resources{Metal: 1e9, Crystal: 1e9, Deuterium: 1e9}}}
	withoutRequirements := BuildBuildings(planet, BuildingLevels{}, ResearchLevels{}, 1)
	if hasBuilding(withoutRequirements, BuildingFusionReactor) || hasBuilding(withoutRequirements, BuildingNaniteFactory) {
		t.Fatalf("expected unavailable requirements to hide gated buildings: %+v", withoutRequirements.Items)
	}

	withRequirements := BuildBuildings(planet, BuildingLevels{
		BuildingDeuteriumSynth:  5,
		BuildingRoboticsFactory: 10,
	}, ResearchLevels{
		ResearchComputer: 10,
		ResearchEnergy:   3,
	}, 1)
	if !hasBuilding(withRequirements, BuildingFusionReactor) || !hasBuilding(withRequirements, BuildingNaniteFactory) {
		t.Fatalf("expected requirements to reveal gated buildings: %+v", withRequirements.Items)
	}

	moon := BuildBuildings(Overview{CurrentPlanet: PlanetOverview{Type: PlanetTypeMoon, MaxFields: 10}}, BuildingLevels{BuildingLunarBase: 1}, ResearchLevels{}, 1)
	if hasBuilding(moon, BuildingMetalMine) || !hasBuilding(moon, BuildingLunarBase) || !hasBuilding(moon, BuildingSensorPhalanx) {
		t.Fatalf("unexpected moon building set: %+v", moon.Items)
	}
}

func TestBuildBuildingsMarksUnavailableWhenResourcesOrFieldsAreMissing(t *testing.T) {
	overview := Overview{CurrentPlanet: PlanetOverview{Type: PlanetTypePlanet, Fields: 10, MaxFields: 10}}
	buildings := BuildBuildings(overview, BuildingLevels{}, ResearchLevels{}, 1)
	metalMine := findBuilding(t, buildings, BuildingMetalMine)

	if metalMine.CanBuild || metalMine.Action != "There's no space!" {
		t.Fatalf("expected full planet to block construction, got %+v", metalMine)
	}
}

func TestBuildingMutationHelpersFollowCatalog(t *testing.T) {
	if NormalizeBuildingsMutationAction(BuildingsMutationAdd) != BuildingsMutationAdd ||
		NormalizeBuildingsMutationAction(BuildingsMutationRemove) != BuildingsMutationRemove ||
		NormalizeBuildingsMutationAction("destroy") != "" {
		t.Fatalf("unexpected building mutation action normalization")
	}
	cost, ok := BuildingCostForLevel(BuildingMetalMine, 3)
	if !ok || cost.Metal != 135 || cost.Crystal != 33.75 {
		t.Fatalf("unexpected exported building cost: cost=%+v ok=%v", cost, ok)
	}
	if _, ok := BuildingCostForLevel(9999, 1); ok {
		t.Fatalf("unknown building should not have a cost")
	}
	if !BuildingAllowedOnPlanet(BuildingLunarBase, PlanetTypeMoon) || BuildingAllowedOnPlanet(BuildingMetalMine, PlanetTypeMoon) {
		t.Fatalf("building planet type helper disagrees with catalog")
	}
	if !BuildingRequirementsMet(BuildingFusionReactor, BuildingLevels{BuildingDeuteriumSynth: 5}, ResearchLevels{ResearchEnergy: 3}) {
		t.Fatalf("expected fusion reactor requirements to be met")
	}
	if BuildingRequirementsMet(BuildingFusionReactor, BuildingLevels{}, ResearchLevels{}) {
		t.Fatalf("expected missing fusion reactor requirements")
	}
	if !BuildingRequirementsMet(BuildingJumpGate, BuildingLevels{BuildingLunarBase: 1}, ResearchLevels{ResearchHyperspace: 7}) {
		t.Fatalf("expected jump gate research requirements to be met")
	}
	if BuildingDurationForCost(cost, 1, 0, 1) != 121 {
		t.Fatalf("duration helper should reuse legacy formula")
	}
	if BuildingActionIssue(BuildingsIssueNoResources).Message == "" || BuildingActionIssue("unknown").Code != "unknown" {
		t.Fatalf("expected building action issues to be populated")
	}
}

func findBuilding(t *testing.T, buildings Buildings, id int) BuildingItem {
	t.Helper()
	for _, item := range buildings.Items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("building %d not found in %+v", id, buildings.Items)
	return BuildingItem{}
}

func hasBuilding(buildings Buildings, id int) bool {
	for _, item := range buildings.Items {
		if item.ID == id {
			return true
		}
	}
	return false
}
