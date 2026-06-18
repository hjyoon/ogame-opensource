package game

import "testing"

func TestBuildResearchUsesLegacyCostDurationAndRequirements(t *testing.T) {
	overview := Overview{
		Commander: "legor",
		CurrentPlanet: PlanetOverview{
			ID:   99,
			Name: "Arakis",
			Type: PlanetTypePlanet,
			Resources: Resources{
				Metal:     10000,
				Crystal:   10000,
				Deuterium: 10000,
			},
		},
	}
	levels := BuildingLevels{BuildingResearchLab: 3}
	research := ResearchLevels{ResearchEnergy: 1}
	labs := BuildResearchLabLevels(levels[BuildingResearchLab], nil, research)

	result := BuildResearch(overview, levels, research, labs, 1, false)

	if !result.HasLab || result.Commander != "legor" {
		t.Fatalf("unexpected research summary: %+v", result)
	}
	computer := findResearch(t, result, ResearchComputer)
	if computer.Level != 0 || computer.NextLevel != 1 {
		t.Fatalf("unexpected computer level: %+v", computer)
	}
	if computer.Cost.Crystal != 400 || computer.Cost.Deuterium != 600 {
		t.Fatalf("unexpected computer cost: %+v", computer.Cost)
	}
	if computer.DurationSeconds != 360 {
		t.Fatalf("expected legacy research duration, got %d", computer.DurationSeconds)
	}
	if !computer.CanBuild || computer.Action != "research" {
		t.Fatalf("expected affordable first research action, got %+v", computer)
	}
	if hasResearch(result, ResearchShield) {
		t.Fatalf("shielding should be locked without energy 3 and lab 6: %+v", result.Items)
	}
}

func TestBuildResearchHandlesLabNetworkTechnocratAndNoLab(t *testing.T) {
	overview := Overview{CurrentPlanet: PlanetOverview{Type: PlanetTypePlanet, Resources: Resources{Crystal: 1e9, Deuterium: 1e9}}}
	levels := BuildingLevels{BuildingResearchLab: 3}
	research := ResearchLevels{ResearchComputer: 1, ResearchIntergalacticNetwork: 1}
	labs := BuildResearchLabLevels(levels[BuildingResearchLab], []int{7, 1}, research)
	result := BuildResearch(overview, levels, research, labs, 2, true)

	computer := findResearch(t, result, ResearchComputer)
	if computer.DurationSeconds != 119 {
		t.Fatalf("expected speed, technocrat, and lab network duration, got %+v", computer)
	}
	if computer.Action != "Research level" {
		t.Fatalf("expected next-level action, got %+v", computer)
	}

	noLab := BuildResearch(overview, BuildingLevels{}, research, nil, 1, false)
	if noLab.HasLab || len(noLab.Items) != 0 {
		t.Fatalf("expected research lab requirement screen, got %+v", noLab)
	}
}

func TestBuildResearchMarksMaximumLevel(t *testing.T) {
	result := BuildResearch(
		Overview{CurrentPlanet: PlanetOverview{Type: PlanetTypePlanet, Resources: Resources{Crystal: 1e9, Deuterium: 1e9}}},
		BuildingLevels{BuildingResearchLab: 1},
		ResearchLevels{ResearchComputer: maxResearchLevel},
		BuildResearchLabLevels(1, nil, ResearchLevels{ResearchComputer: maxResearchLevel}),
		1,
		false,
	)
	computer := findResearch(t, result, ResearchComputer)
	if computer.CanBuild || computer.Action != "Maximum level reached." {
		t.Fatalf("expected max-level research block, got %+v", computer)
	}
}

func TestResearchDurationFloorsToOneSecond(t *testing.T) {
	if got := researchDuration(BuildingCost{Energy: 300000}, 12, 128); got != 1 {
		t.Fatalf("expected energy-only graviton research to floor to one second, got %d", got)
	}
	if got := mathFloorPositive(0.5); got != 0 {
		t.Fatalf("expected sub-second floor helper to return zero before duration minimum, got %d", got)
	}
}

func findResearch(t *testing.T, research Research, id int) BuildingItem {
	t.Helper()
	for _, item := range research.Items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("research %d not found in %+v", id, research.Items)
	return BuildingItem{}
}

func hasResearch(research Research, id int) bool {
	for _, item := range research.Items {
		if item.ID == id {
			return true
		}
	}
	return false
}
