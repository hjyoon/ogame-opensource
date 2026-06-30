package game

import "testing"

func TestBuildTechnologyUsesLegacyGroupsAndRequirements(t *testing.T) {
	overview := Overview{
		Commander: "legor",
		CurrentPlanet: PlanetOverview{
			ID:   99,
			Name: "Arakis",
		},
	}
	technology := BuildTechnology(overview, BuildingLevels{
		BuildingDeuteriumSynth: 5,
		BuildingShipyard:       1,
	}, ResearchLevels{
		ResearchEnergy: 2,
	})

	if technology.Commander != "legor" || technology.CurrentPlanet.ID != 99 || len(technology.Groups) != 5 {
		t.Fatalf("unexpected technology summary: %+v", technology)
	}
	if technology.Groups[0].Name != "Buildings" || technology.Groups[1].Name != "Research" || technology.Groups[2].Name != "Ships" || technology.Groups[4].Name != "Lunar Buildings" {
		t.Fatalf("unexpected technology groups: %+v", technology.Groups)
	}

	fusion := technologyItemByID(t, technology, BuildingFusionReactor)
	if fusion.Name != "Fusion Reactor" || !fusion.DetailsAvailable || len(fusion.Requirements) != 2 {
		t.Fatalf("unexpected fusion reactor row: %+v", fusion)
	}
	if fusion.Requirements[0].ID != BuildingDeuteriumSynth || !fusion.Requirements[0].Met || fusion.Requirements[0].CurrentLevel != 5 {
		t.Fatalf("expected met deuterium requirement, got %+v", fusion.Requirements)
	}
	if fusion.Requirements[1].ID != ResearchEnergy || fusion.Requirements[1].Met || fusion.Requirements[1].CurrentLevel != 2 {
		t.Fatalf("expected unmet energy requirement, got %+v", fusion.Requirements)
	}

	metalMine := technologyItemByID(t, technology, BuildingMetalMine)
	if metalMine.DetailsAvailable || len(metalMine.Requirements) != 0 {
		t.Fatalf("expected no-requirement building row, got %+v", metalMine)
	}

	hyperspace := technologyItemByID(t, technology, ResearchHyperspace)
	assertRequirementIDs(t, hyperspace.Requirements, []int{ResearchEnergy, ResearchShield, BuildingResearchLab})
	ion := technologyItemByID(t, technology, ResearchIon)
	assertRequirementIDs(t, ion.Requirements, []int{BuildingResearchLab, ResearchLaser, ResearchEnergy})
}

func TestBuildTechnologyIncludesDefenseMissilesAndMoonBuildings(t *testing.T) {
	technology := BuildTechnology(Overview{}, BuildingLevels{BuildingMissileSilo: 4, BuildingLunarBase: 1}, ResearchLevels{})

	ipm := technologyItemByID(t, technology, DefenseInterplanetaryMissile)
	if ipm.Name != "Interplanetary Missiles" || len(ipm.Requirements) != 3 {
		t.Fatalf("expected IPM requirements, got %+v", ipm)
	}
	jumpGate := technologyItemByID(t, technology, BuildingJumpGate)
	if jumpGate.Name != "Jump Gate" || len(jumpGate.Requirements) != 2 || !jumpGate.Requirements[0].Met {
		t.Fatalf("expected lunar jump gate requirements, got %+v", jumpGate)
	}
}

func TestBuildTechnologyDetailsUsesLegacyDepthOrder(t *testing.T) {
	details, ok := BuildTechnologyDetails(FleetCruiser, BuildingLevels{
		BuildingShipyard: 5,
	}, ResearchLevels{
		ResearchEnergy:       2,
		ResearchImpulseDrive: 4,
		ResearchLaser:        3,
	})
	if !ok {
		t.Fatal("expected cruiser technology details")
	}
	if details.Target.ID != FleetCruiser || details.Target.Name != "Cruiser" || len(details.Levels) != 4 {
		t.Fatalf("unexpected cruiser details: %+v", details)
	}
	if details.Levels[0].Step != 1 || len(details.Levels[0].Requirements) != 1 {
		t.Fatalf("expected deepest requirements first, got %+v", details.Levels)
	}
	if details.Levels[0].Requirements[0].ID != BuildingResearchLab {
		t.Fatalf("unexpected deepest requirement: %+v", details.Levels[0].Requirements)
	}
	lastLevel := details.Levels[len(details.Levels)-1]
	if lastLevel.Step != 4 || len(lastLevel.Requirements) != 3 {
		t.Fatalf("expected direct requirements last, got %+v", lastLevel)
	}
	if lastLevel.Requirements[1].ID != ResearchImpulseDrive || !lastLevel.Requirements[1].Met {
		t.Fatalf("expected met impulse drive requirement, got %+v", lastLevel.Requirements)
	}

	hyperspace, ok := BuildTechnologyDetails(ResearchHyperspace, BuildingLevels{}, ResearchLevels{})
	if !ok {
		t.Fatal("expected hyperspace technology details")
	}
	assertRequirementIDs(t, hyperspace.Target.Requirements, []int{ResearchEnergy, ResearchShield, BuildingResearchLab})
	assertRequirementIDs(t, hyperspace.Levels[len(hyperspace.Levels)-1].Requirements, []int{ResearchEnergy, ResearchShield, BuildingResearchLab})
}

func TestBuildTechnologyDetailsHandlesNoConditionsAndUnknownIDs(t *testing.T) {
	details, ok := BuildTechnologyDetails(BuildingMetalMine, BuildingLevels{}, ResearchLevels{})
	if !ok {
		t.Fatal("expected metal mine details")
	}
	if details.Target.ID != BuildingMetalMine || details.Target.DetailsAvailable || len(details.Levels) != 0 {
		t.Fatalf("expected no-condition metal mine details, got %+v", details)
	}

	if _, ok := BuildTechnologyDetails(9999, BuildingLevels{}, ResearchLevels{}); ok {
		t.Fatal("expected unknown technology id to be rejected")
	}
}

func TestBuildTechnologyDetailsIncludesLegacyDemolishInfo(t *testing.T) {
	details, ok := BuildTechnologyDetailsWithSpeed(BuildingMetalStorage, BuildingLevels{
		BuildingMetalStorage:    2,
		BuildingRoboticsFactory: 1,
	}, ResearchLevels{}, 128)
	if !ok {
		t.Fatal("expected metal storage details")
	}
	if details.Demolish == nil {
		t.Fatal("expected demolish details")
	}
	if details.Demolish.Level != 2 || details.Demolish.Cost.Metal != 2000 || details.Demolish.DurationSeconds <= 0 {
		t.Fatalf("unexpected demolish details: %+v", details.Demolish)
	}

	details, ok = BuildTechnologyDetails(BuildingTerraformer, BuildingLevels{BuildingTerraformer: 1}, ResearchLevels{})
	if !ok {
		t.Fatal("expected terraformer details")
	}
	if details.Demolish != nil {
		t.Fatalf("terraformer must not expose demolition: %+v", details.Demolish)
	}

	details, ok = BuildTechnologyDetails(ResearchEnergy, BuildingLevels{ResearchEnergy: 1}, ResearchLevels{})
	if !ok {
		t.Fatal("expected energy technology details")
	}
	if details.Demolish != nil {
		t.Fatalf("research entries must not expose demolition: %+v", details.Demolish)
	}
}

func TestBuildTechnologyInfoUsesLegacyInfosPreview(t *testing.T) {
	info, ok := BuildTechnologyInfoWithSpeed(BuildingMetalMine, PlanetOverview{}, BuildingLevels{}, ResearchLevels{}, 128)
	if !ok {
		t.Fatal("expected metal mine info")
	}
	if info.Name != "Metal Mine" || info.Kind != "mine" || len(info.Rows) != 15 {
		t.Fatalf("unexpected info summary: %+v", info)
	}
	first := info.Rows[0]
	if first.Level != 1 || first.Production != 4224 || first.ProductionDifference != 4224 || first.Energy != -11 || first.EnergyDifference != -11 {
		t.Fatalf("unexpected metal mine first row: %+v", first)
	}
	if info.Rows[0].Current {
		t.Fatalf("level zero mine should not mark preview row current: %+v", info.Rows[0])
	}

	storage, ok := BuildTechnologyInfoWithSpeed(BuildingMetalStorage, PlanetOverview{}, BuildingLevels{BuildingMetalStorage: 1}, ResearchLevels{}, 128)
	if !ok {
		t.Fatal("expected metal storage info")
	}
	if storage.Kind != "storage" || storage.Rows[0].Level != 1 || !storage.Rows[0].Current || storage.Rows[0].Storage <= 0 {
		t.Fatalf("unexpected storage info: %+v", storage)
	}

	crystal, ok := BuildTechnologyInfoWithSpeed(BuildingCrystalMine, PlanetOverview{}, BuildingLevels{BuildingCrystalMine: 3}, ResearchLevels{}, 0)
	if !ok {
		t.Fatal("expected crystal mine info")
	}
	if crystal.Kind != "mine" || len(crystal.Rows) != 15 || !crystal.Rows[2].Current || crystal.Rows[2].Production <= 0 || crystal.Rows[2].Energy >= 0 {
		t.Fatalf("unexpected crystal mine info: %+v", crystal)
	}

	deuterium, ok := BuildTechnologyInfoWithSpeed(BuildingDeuteriumSynth, PlanetOverview{Temperature: 15}, BuildingLevels{BuildingDeuteriumSynth: 4}, ResearchLevels{}, 64)
	if !ok {
		t.Fatal("expected deuterium synthesizer info")
	}
	if deuterium.Kind != "mine" || !deuterium.Rows[2].Current || deuterium.Rows[2].Production <= 0 || deuterium.Rows[2].Energy >= 0 {
		t.Fatalf("unexpected deuterium synthesizer info: %+v", deuterium)
	}

	solar, ok := BuildTechnologyInfoWithSpeed(BuildingSolarPlant, PlanetOverview{}, BuildingLevels{BuildingSolarPlant: 2}, ResearchLevels{}, 128)
	if !ok {
		t.Fatal("expected solar plant info")
	}
	if solar.Kind != "solar" || !solar.Rows[1].Current || solar.Rows[1].Energy <= 0 || solar.Rows[1].Production != 0 {
		t.Fatalf("unexpected solar plant info: %+v", solar)
	}

	fusion, ok := BuildTechnologyInfoWithSpeed(BuildingFusionReactor, PlanetOverview{}, BuildingLevels{BuildingFusionReactor: 3}, ResearchLevels{ResearchEnergy: 5}, 128)
	if !ok {
		t.Fatal("expected fusion reactor info")
	}
	if fusion.Kind != "fusion" || !fusion.Rows[2].Current || fusion.Rows[2].Energy <= 0 || fusion.Rows[2].DeuteriumConsumption <= 0 {
		t.Fatalf("unexpected fusion reactor info: %+v", fusion)
	}

	description, ok := BuildTechnologyInfoWithSpeed(FleetSmallCargo, PlanetOverview{}, BuildingLevels{}, ResearchLevels{}, 128)
	if !ok {
		t.Fatal("expected ship info")
	}
	if description.Kind != "description" || len(description.Rows) != 0 || description.Description == "" {
		t.Fatalf("unexpected description-only info: %+v", description)
	}

	if _, ok := BuildTechnologyInfoWithSpeed(9999, PlanetOverview{}, BuildingLevels{}, ResearchLevels{}, 128); ok {
		t.Fatal("expected unknown technology info id to be rejected")
	}
	if production, energy, deuterium := technologyInfoProduction(9999, 0, PlanetOverview{}, ResearchLevels{}, 128); production != 0 || energy != 0 || deuterium != 0 {
		t.Fatalf("expected empty production for invalid level, got production=%d energy=%d deuterium=%d", production, energy, deuterium)
	}
}

func TestLegacyTechnologyRequirementOrderingFallbacks(t *testing.T) {
	if ids := legacyTechnologyRequirementIDs(ResearchEnergy, nil); ids != nil {
		t.Fatalf("expected empty requirements to keep nil order, got %v", ids)
	}

	requirements := map[int]int{ResearchLaser: 5, BuildingResearchLab: 4, ResearchEnergy: 4, 9999: 1}
	assertIDs(t, legacyTechnologyRequirementIDs(ResearchIon, requirements), []int{BuildingResearchLab, ResearchLaser, ResearchEnergy, 9999})
	assertIDs(t, legacyTechnologyRequirementIDs(9998, requirements), []int{BuildingResearchLab, ResearchEnergy, ResearchLaser, 9999})

	ordered := buildTechnologyRequirementsOrdered(requirements, nil, BuildingLevels{BuildingResearchLab: 5}, ResearchLevels{ResearchEnergy: 3, ResearchLaser: 5})
	assertRequirementIDs(t, ordered, []int{BuildingResearchLab, ResearchEnergy, ResearchLaser, 9999})
	if !ordered[0].Met || ordered[1].Met || !ordered[2].Met || ordered[3].Name != "" {
		t.Fatalf("unexpected fallback requirement values: %+v", ordered)
	}
	if name := technologyName(9999); name != "" {
		t.Fatalf("expected unknown technology name to be empty, got %q", name)
	}

	group := buildTechnologyGroup("mixed", "Mixed", []int{9999, BuildingMetalMine}, BuildingLevels{}, ResearchLevels{})
	if len(group.Items) != 1 || group.Items[0].ID != BuildingMetalMine {
		t.Fatalf("expected unknown ids to be skipped in technology group, got %+v", group)
	}
	if isBuildingID(9999) {
		t.Fatal("unknown technology id must not be treated as a building")
	}
}

func TestTechnologyRequirementHelpersCoverSparseAndUnknownInputs(t *testing.T) {
	if requirements := buildTechnologyRequirementsOrdered(nil, []int{ResearchEnergy}, BuildingLevels{}, ResearchLevels{}); requirements != nil {
		t.Fatalf("expected nil requirements for empty map, got %+v", requirements)
	}

	requirements := buildTechnologyRequirementsOrdered(
		map[int]int{ResearchLaser: 2, BuildingResearchLab: 1},
		[]int{ResearchEnergy, ResearchLaser},
		BuildingLevels{BuildingResearchLab: 1},
		ResearchLevels{ResearchLaser: 1},
	)
	assertRequirementIDs(t, requirements, []int{ResearchLaser, BuildingResearchLab})
	if requirements[0].Met || !requirements[1].Met {
		t.Fatalf("unexpected sparse requirement status: %+v", requirements)
	}

	tree := newTechnologyRequirementsByDepth()
	if depth := walkTechnologyRequirements(9999, map[int]int{9998: 3}, 0, tree); depth != 1 {
		t.Fatalf("expected one-level unknown child depth, got %d", depth)
	}
	requirements = buildTechnologyRequirementsOrdered(tree.requirements[0], tree.order[0], BuildingLevels{}, ResearchLevels{})
	assertRequirementIDs(t, requirements, []int{9998})
	if requirements[0].Name != "" {
		t.Fatalf("unexpected unknown child requirements: %+v", requirements)
	}
}

func technologyItemByID(t *testing.T, technology Technology, id int) TechnologyItem {
	t.Helper()
	for _, group := range technology.Groups {
		for _, item := range group.Items {
			if item.ID == id {
				return item
			}
		}
	}
	t.Fatalf("technology item %d not found in %+v", id, technology.Groups)
	return TechnologyItem{}
}

func assertRequirementIDs(t *testing.T, requirements []TechnologyRequirement, expected []int) {
	t.Helper()
	if len(requirements) != len(expected) {
		t.Fatalf("expected requirement ids %v, got %+v", expected, requirements)
	}
	for i, id := range expected {
		if requirements[i].ID != id {
			t.Fatalf("expected requirement ids %v, got %+v", expected, requirements)
		}
	}
}

func assertIDs(t *testing.T, ids []int, expected []int) {
	t.Helper()
	if len(ids) != len(expected) {
		t.Fatalf("expected ids %v, got %v", expected, ids)
	}
	for i, id := range expected {
		if ids[i] != id {
			t.Fatalf("expected ids %v, got %v", expected, ids)
		}
	}
}
