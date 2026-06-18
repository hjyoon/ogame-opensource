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
