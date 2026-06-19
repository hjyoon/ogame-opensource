package game

import "testing"

func TestDomainMiscHelpers(t *testing.T) {
	researchIDs := BuildingResearchIDs()
	if len(researchIDs) != 3 || researchIDs[0] != ResearchComputer {
		t.Fatalf("unexpected building research ids: %+v", researchIDs)
	}
	if cost, ok := ResearchCostForLevel(ResearchComputer, 2); !ok || cost.Crystal <= 0 || cost.Deuterium <= 0 {
		t.Fatalf("expected research cost, cost=%+v ok=%v", cost, ok)
	}
	if cost, ok := ResearchCostForLevel(-1, 1); ok || cost != (BuildingCost{}) {
		t.Fatalf("unknown research cost should be empty, cost=%+v ok=%v", cost, ok)
	}
	if points, ok := ResearchScoreForLevel(ResearchComputer, 1); !ok || points <= 0 {
		t.Fatalf("expected research score, points=%d ok=%v", points, ok)
	}
	if points, ok := ResearchScoreForLevel(-1, 1); ok || points != 0 {
		t.Fatalf("unknown research score should be empty, points=%d ok=%v", points, ok)
	}
	if seconds, ok := ResearchDurationForLevel(ResearchComputer, 1, 1, 0); !ok || seconds <= 0 {
		t.Fatalf("expected research duration, seconds=%d ok=%v", seconds, ok)
	}
	if seconds, ok := ResearchDurationForLevel(-1, 1, 1, 1); ok || seconds != 0 {
		t.Fatalf("unknown research duration should be empty, seconds=%d ok=%v", seconds, ok)
	}
	if !ResearchRequirementsMet(ResearchComputer, BuildingLevels{BuildingResearchLab: 1}, nil) ||
		ResearchRequirementsMet(ResearchComputer, nil, nil) ||
		ResearchRequirementsMet(-1, nil, nil) {
		t.Fatal("unexpected research requirements result")
	}
	if NormalizeEmpirePlanetType(EmpirePlanetTypeMoons, true) != EmpirePlanetTypeMoons ||
		NormalizeEmpirePlanetType(EmpirePlanetTypeMoons, false) != EmpirePlanetTypePlanets {
		t.Fatal("unexpected empire planet type normalization")
	}
	issue := BuddyAlreadySentIssue()
	if issue == nil || issue.Code != BuddyIssueAlreadySent || issue.Message == "" {
		t.Fatalf("unexpected buddy issue: %+v", issue)
	}
}
