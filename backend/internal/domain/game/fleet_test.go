package game

import "testing"

func TestBuildFleetUsesLegacySlotsAndShipSelection(t *testing.T) {
	overview := Overview{
		Commander: "legor",
		CurrentPlanet: PlanetOverview{
			ID:   99,
			Name: "Arakis",
		},
	}
	missions := []FleetMission{
		BuildFleetMission(11, FleetMissionTransport, FleetCounts{FleetSmallCargo: 2}, Coordinates{Galaxy: 1, System: 2, Position: 3}, Coordinates{Galaxy: 1, System: 2, Position: 4}, PlanetTypePlanet, "target", 100, 200),
		BuildFleetMission(12, FleetMissionExpedition+FleetMissionReturnOffset, FleetCounts{FleetEspionageProbe: 1}, Coordinates{Galaxy: 1, System: 2, Position: 3}, Coordinates{Galaxy: 9, System: 499, Position: 16}, PlanetTypePlanet, "space", 120, 220),
	}

	fleet := BuildFleet(overview, FleetCounts{
		FleetSmallCargo:     4,
		FleetSolarSatellite: 2,
	}, ResearchLevels{
		ResearchComputer:        3,
		ResearchExpedition:      4,
		ResearchCombustionDrive: 2,
		ResearchImpulseDrive:    5,
	}, missions, true, true)

	if fleet.Commander != "legor" || fleet.CurrentPlanet.ID != 99 {
		t.Fatalf("unexpected fleet summary: %+v", fleet)
	}
	if fleet.Slots.Used != 2 || fleet.Slots.BaseMax != 4 || fleet.Slots.Max != 6 || !fleet.Slots.Admiral {
		t.Fatalf("unexpected fleet slots: %+v", fleet.Slots)
	}
	if fleet.Expeditions.Used != 1 || fleet.Expeditions.Max != 2 {
		t.Fatalf("unexpected expedition slots: %+v", fleet.Expeditions)
	}
	if len(fleet.Ships) != 2 || fleet.Ships[0].ID != FleetSmallCargo || fleet.Ships[0].Speed != 20000 || fleet.Ships[0].Consumption != 20 || !fleet.Ships[0].Selectable {
		t.Fatalf("unexpected small cargo row: %+v", fleet.Ships)
	}
	if fleet.Ships[1].ID != FleetSolarSatellite || fleet.Ships[1].Speed != 0 || fleet.Ships[1].Selectable {
		t.Fatalf("expected non-selectable solar satellite row: %+v", fleet.Ships[1])
	}
	if fleet.Missions[0].MissionName != "Transport" || fleet.Missions[0].StateShort != "(G)" || !fleet.Missions[0].CanRecall || fleet.Missions[0].TotalShips != 2 {
		t.Fatalf("unexpected transport mission row: %+v", fleet.Missions[0])
	}
	if fleet.Missions[1].MissionName != "Expedition" || fleet.Missions[1].StateShort != "(F)" || fleet.Missions[1].CanRecall {
		t.Fatalf("unexpected returning expedition row: %+v", fleet.Missions[1])
	}
}

func TestBuildFleetMarksAttackUnionAvailability(t *testing.T) {
	fleet := BuildFleet(Overview{}, FleetCounts{}, ResearchLevels{}, []FleetMission{
		BuildFleetMission(1, FleetMissionAttack, FleetCounts{FleetLightFighter: 1}, Coordinates{}, Coordinates{}, PlanetTypePlanet, "", 0, 0),
		BuildFleetMission(2, FleetMissionACSAttackHead, FleetCounts{FleetLightFighter: 1}, Coordinates{}, Coordinates{}, PlanetTypePlanet, "", 0, 0),
		BuildFleetMission(3, FleetMissionTransport, FleetCounts{FleetSmallCargo: 1}, Coordinates{}, Coordinates{}, PlanetTypePlanet, "", 0, 0),
	}, false, true)

	if !fleet.Missions[0].CanCreateUnion || !fleet.Missions[1].CanCreateUnion || fleet.Missions[2].CanCreateUnion {
		t.Fatalf("unexpected ACS union flags: %+v", fleet.Missions)
	}
}

func TestBuildFleetTemplateUsesLegacyShipIDsWithoutSolarSatellites(t *testing.T) {
	template := BuildFleetTemplate(7, "  raid wing  ", 1234, FleetCounts{
		FleetSmallCargo:     5,
		FleetSolarSatellite: 9,
		FleetRecycler:       -1,
		FleetBattlecruiser:  2,
	})

	if template.ID != 7 || template.Name != "raid wing" || template.UpdatedAt != 1234 {
		t.Fatalf("unexpected template header: %+v", template)
	}
	if len(template.Ships) != 2 || template.Ships[0].ID != FleetSmallCargo || template.Ships[1].ID != FleetBattlecruiser {
		t.Fatalf("unexpected template ships: %+v", template.Ships)
	}
	for _, id := range FleetTemplateShipIDs() {
		if id == FleetSolarSatellite {
			t.Fatal("solar satellites must not be selectable for standard fleets")
		}
	}
}

func TestFleetMissionDisplayAndNames(t *testing.T) {
	tests := []struct {
		mission int
		name    string
		title   string
		short   string
	}{
		{FleetMissionAttack, "Attack", "Going on a mission", "(G)"},
		{FleetMissionACSAttack, "Joint attack", "Going on a mission", "(G)"},
		{FleetMissionTransport + FleetMissionReturnOffset, "Transport", "Fleet Returns home", "(F)"},
		{FleetMissionDeploy + FleetMissionOrbitingOffset, "Station", "On the planet", "(H)"},
		{FleetMissionACSHold, "Defend", "Going on a mission", "(G)"},
		{FleetMissionSpy, "Espionage", "Going on a mission", "(G)"},
		{FleetMissionColonize, "Colonise", "Going on a mission", "(G)"},
		{FleetMissionRecycle, "Recycle", "Going on a mission", "(G)"},
		{FleetMissionDestroy, "Destroy", "Going on a mission", "(G)"},
		{FleetMissionExpedition, "Expedition", "Going on a mission", "(G)"},
		{FleetMissionMissile, "Missile Attack", "Going on a mission", "(G)"},
		{FleetMissionACSAttackHead, "Attack", "Going on a mission", "(G)"},
		{FleetMissionCustomOffset + 7, "Custom task", "Custom task", "(C)"},
		{99, "Custom task", "Going on a mission", "(G)"},
	}
	for _, tt := range tests {
		base, title, short := fleetMissionDisplay(tt.mission)
		if title != tt.title || short != tt.short {
			t.Fatalf("unexpected mission display for %d: base=%d title=%q short=%q", tt.mission, base, title, short)
		}
		if fleetMissionName(base) != tt.name {
			t.Fatalf("unexpected mission name for %d base %d: %q", tt.mission, base, fleetMissionName(base))
		}
	}
	if fleetName(123456) != "" {
		t.Fatal("unknown fleet id should not have a display name")
	}
}

func TestFleetShipSpeedsMatchLegacyDriveFamilies(t *testing.T) {
	research := ResearchLevels{
		ResearchCombustionDrive: 1,
		ResearchImpulseDrive:    2,
		ResearchHyperspaceDrive: 3,
	}
	tests := map[int]int{
		FleetSmallCargo:     5500,
		FleetLargeCargo:     8250,
		FleetLightFighter:   13750,
		FleetRecycler:       2200,
		FleetEspionageProbe: 110000000,
		FleetSolarSatellite: 0,
		FleetHeavyFighter:   14000,
		FleetCruiser:        21000,
		FleetColonyShip:     3500,
		FleetBattleship:     19000,
		FleetDestroyer:      9500,
		FleetDeathstar:      190,
		FleetBattlecruiser:  19000,
		FleetBomber:         5600,
		999999:              0,
	}
	for id, want := range tests {
		if got := fleetShipSpeed(id, research); got != want {
			t.Fatalf("unexpected speed for %d: got %d want %d", id, got, want)
		}
	}

	advanced := ResearchLevels{ResearchHyperspaceDrive: 8}
	if got := fleetShipSpeed(FleetBomber, advanced); got != 17000 {
		t.Fatalf("unexpected advanced bomber speed: %d", got)
	}
}
