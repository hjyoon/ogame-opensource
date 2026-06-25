package game

import "testing"

func TestPhalanxScanIssueMatchesLegacyGuards(t *testing.T) {
	playerID := 42
	source := PhalanxPlanet{
		ID:           10,
		OwnerID:      playerID,
		Type:         PlanetTypeMoon,
		Coordinates:  Coordinates{Galaxy: 1, System: 100, Position: 4},
		PhalanxLevel: 3,
		Deuterium:    20000,
	}
	target := PhalanxPlanet{
		ID:          20,
		OwnerID:     77,
		Type:        PlanetTypePlanet,
		Coordinates: Coordinates{Galaxy: 1, System: 105, Position: 5},
	}

	if issue := PhalanxScanIssue(playerID, source, target); issue != nil {
		t.Fatalf("expected valid scan, got %+v", issue)
	}

	tests := []struct {
		name   string
		source PhalanxPlanet
		target PhalanxPlanet
		code   string
	}{
		{"missing sensor", withPhalanxLevel(source, 0), target, PhalanxIssueMissingSensor},
		{"insufficient deut", withPhalanxDeuterium(source, 4999), target, PhalanxIssueInsufficientDeut},
		{"own target", source, withPhalanxOwner(target, playerID), PhalanxIssueForbidden},
		{"foreign source", withPhalanxOwner(source, 99), target, PhalanxIssueForbidden},
		{"moon target", source, withPhalanxType(target, PlanetTypeMoon), PhalanxIssueForbidden},
		{"other galaxy", source, withPhalanxCoordinates(target, Coordinates{Galaxy: 2, System: 105, Position: 5}), PhalanxIssueForbidden},
		{"out of range", source, withPhalanxCoordinates(target, Coordinates{Galaxy: 1, System: 109, Position: 5}), PhalanxIssueForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := PhalanxScanIssue(playerID, tt.source, tt.target)
			if issue == nil || issue.Code != tt.code {
				t.Fatalf("expected issue %q, got %+v", tt.code, issue)
			}
		})
	}
}

func TestNewPhalanxSpendsOnlyOnSuccess(t *testing.T) {
	overview := Overview{Commander: "legor", CurrentPlanet: PlanetOverview{ID: 10}}
	source := PhalanxPlanet{ID: 10, Deuterium: 20000}
	target := PhalanxPlanet{ID: 20}

	success := NewPhalanx(overview, source, target, []FleetMission{
		BuildFleetMission(7, FleetMissionTransport, FleetCounts{FleetSmallCargo: 1}, Coordinates{}, Coordinates{}, PlanetTypePlanet, "", 10, 20),
	}, nil)
	if success.RemainingDeuterium != 15000 || success.Cost != PhalanxCost || len(success.Events) != 1 || success.Events[0].MissionName != "Transport" {
		t.Fatalf("unexpected successful phalanx: %+v", success)
	}

	rejected := NewPhalanx(overview, source, target, nil, &PhalanxActionIssue{Code: PhalanxIssueForbidden})
	if rejected.RemainingDeuterium != 20000 || rejected.ActionIssue == nil {
		t.Fatalf("unexpected rejected phalanx: %+v", rejected)
	}

	floored := NewPhalanx(overview, PhalanxPlanet{ID: 10, Deuterium: 1000}, target, nil, nil)
	if floored.RemainingDeuterium != 0 {
		t.Fatalf("expected remaining deuterium to be floored, got %+v", floored)
	}
}

func TestPhalanxRadiusUsesLegacyFormula(t *testing.T) {
	if PhalanxRadius(0) != 0 || PhalanxRadius(1) != 0 || PhalanxRadius(3) != 8 {
		t.Fatalf("unexpected phalanx radius values")
	}
}

func withPhalanxLevel(planet PhalanxPlanet, level int) PhalanxPlanet {
	planet.PhalanxLevel = level
	return planet
}

func withPhalanxDeuterium(planet PhalanxPlanet, deuterium float64) PhalanxPlanet {
	planet.Deuterium = deuterium
	return planet
}

func withPhalanxOwner(planet PhalanxPlanet, ownerID int) PhalanxPlanet {
	planet.OwnerID = ownerID
	return planet
}

func withPhalanxType(planet PhalanxPlanet, planetType int) PhalanxPlanet {
	planet.Type = planetType
	return planet
}

func withPhalanxCoordinates(planet PhalanxPlanet, coordinates Coordinates) PhalanxPlanet {
	planet.Coordinates = coordinates
	return planet
}
