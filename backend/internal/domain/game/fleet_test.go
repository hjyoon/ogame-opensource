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
	if fleet.Expeditions.Used != 1 || fleet.Expeditions.Max != 2 || fleet.ExpeditionLevel != 4 {
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

func TestBuildOverviewEventsNormalizesMissionRows(t *testing.T) {
	incoming := BuildFleetMission(8, FleetMissionAttack, FleetCounts{FleetLightFighter: 5}, Coordinates{Galaxy: 1, System: 2, Position: 4}, Coordinates{Galaxy: 1, System: 2, Position: 3}, PlanetTypePlanet, "legor", 100, 200)
	incoming.OwnerID = 77
	incoming.OwnerName = "raider"
	incoming.Foreign = true
	missile := BuildFleetMission(9, FleetMissionMissile, nil, Coordinates{Galaxy: 1, System: 2, Position: 3}, Coordinates{Galaxy: 1, System: 2, Position: 4}, PlanetTypePlanet, "target", 100, 200)
	missile.MissileAmount = 3
	missile.MissileTargetID = DefenseRocketLauncher
	events := BuildOverviewEvents([]FleetMission{
		BuildFleetMission(7, FleetMissionTransport+FleetMissionReturnOffset, FleetCounts{FleetSmallCargo: 3}, Coordinates{Galaxy: 1, System: 2, Position: 3}, Coordinates{Galaxy: 1, System: 2, Position: 4}, PlanetTypePlanet, "target", 100, 200),
		incoming,
		missile,
	})

	if len(events) != 3 ||
		events[0].MissionName != "Transport" ||
		events[0].StateShort != "(F)" ||
		events[0].CanRecall ||
		events[0].CanCreateUnion ||
		events[0].TotalShips != 3 {
		t.Fatalf("unexpected overview event normalization: %+v", events)
	}
	if events[1].MissionName != "Attack" ||
		events[1].OwnerName != "raider" ||
		!events[1].Foreign ||
		events[1].CanRecall ||
		events[1].CanCreateUnion ||
		events[1].TotalShips != 5 {
		t.Fatalf("unexpected incoming overview event normalization: %+v", events[1])
	}
	if events[2].MissionName != "Missile Attack" ||
		events[2].MissileAmount != 3 ||
		events[2].CanRecall ||
		events[2].CanCreateUnion {
		t.Fatalf("unexpected missile overview event normalization: %+v", events[2])
	}
}

func TestBuildFleetDispatchDraftNormalizesLegacySelection(t *testing.T) {
	fleet := BuildFleet(Overview{
		CurrentPlanet: PlanetOverview{
			Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 3},
			Resources:   Resources{Metal: 1200.9, Crystal: 500, Deuterium: 99},
		},
	}, FleetCounts{
		FleetSmallCargo:     4,
		FleetEspionageProbe: 5,
		FleetSolarSatellite: 9,
	}, ResearchLevels{ResearchCombustionDrive: 1, ResearchExpedition: 4}, nil, false, false)

	draft := BuildFleetDispatchDraft(fleet, FleetDispatchDraftInput{
		Ships: map[int]int{
			FleetSmallCargo:     99,
			FleetEspionageProbe: 2,
			FleetSolarSatellite: 9,
			FleetLargeCargo:     -1,
		},
		Target:     Coordinates{Galaxy: 2, System: 3, Position: 4},
		TargetType: GamePlanetTypeMoon,
		Mission:    FleetMissionSpy,
		Speed:      99,
	})

	if !draft.HasSelection || draft.TotalShips != 6 || draft.Speed != 10 || draft.TargetType != GamePlanetTypeMoon || draft.Mission != FleetMissionSpy {
		t.Fatalf("unexpected dispatch draft header: %+v", draft)
	}
	if len(draft.Ships) != 2 || draft.Ships[0].ID != FleetSmallCargo || draft.Ships[0].Count != 4 || draft.Ships[1].ID != FleetEspionageProbe || draft.Ships[1].Count != 2 {
		t.Fatalf("unexpected selected ships: %+v", draft.Ships)
	}
	if draft.Cargo != 4*fleetShipCargo(FleetSmallCargo) {
		t.Fatalf("probe cargo and satellites should be excluded from legacy cargo summary, got %d", draft.Cargo)
	}
	if draft.Distance != 20000 || draft.MaxSpeed != 5500 || draft.DurationSeconds != 21116 || draft.FuelConsumption != 92 || draft.SpeedFactor != 1 {
		t.Fatalf("unexpected legacy flight math: %+v", draft)
	}
	if len(draft.MissionOptions) != 3 || !draft.MissionOptions[2].Selected || draft.MissionOptions[2].ID != FleetMissionSpy {
		t.Fatalf("expected spy to be selected from legacy mission options, got %+v", draft.MissionOptions)
	}
	if len(draft.Resources) != 3 || draft.Resources[0].Available != 1200 || draft.Resources[2].Available != 99 {
		t.Fatalf("expected current planet transportable resources, got %+v", draft.Resources)
	}

	empty := BuildFleetDispatchDraft(fleet, FleetDispatchDraftInput{Ships: map[int]int{}, Speed: -1})
	if empty.HasSelection || empty.Speed != 10 || empty.Target != fleet.CurrentPlanet.Coordinates || empty.TargetType != GamePlanetTypePlanet || empty.Mission != 0 {
		t.Fatalf("unexpected empty dispatch draft defaults: %+v", empty)
	}
	if empty.Distance != 5 || empty.MaxSpeed != 0 || empty.DurationSeconds != 0 || empty.FuelConsumption != 0 {
		t.Fatalf("empty draft should keep only normalized distance, got %+v", empty)
	}
}

func TestBuildFleetDispatchDraftMissionOptionsMatchLegacyEdges(t *testing.T) {
	base := BuildFleet(Overview{
		CurrentPlanet: PlanetOverview{Type: PlanetTypePlanet, Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 3}},
	}, FleetCounts{
		FleetSmallCargo: 1,
		FleetRecycler:   1,
		FleetColonyShip: 1,
		FleetDeathstar:  1,
	}, ResearchLevels{ResearchCombustionDrive: 1, ResearchImpulseDrive: 3, ResearchHyperspaceDrive: 1, ResearchExpedition: 9}, nil, false, false)

	expedition := BuildFleetDispatchDraft(base, FleetDispatchDraftInput{
		Ships:  map[int]int{FleetSmallCargo: 1},
		Target: Coordinates{Galaxy: 1, System: 2, Position: GalaxyFarSpace},
	})
	if len(expedition.MissionOptions) != 1 || expedition.MissionOptions[0].ID != FleetMissionExpedition || expedition.MissionOptions[0].Warning == "" || len(expedition.ExpeditionHours) != 9 {
		t.Fatalf("unexpected expedition missions: %+v hours=%+v", expedition.MissionOptions, expedition.ExpeditionHours)
	}

	debris := BuildFleetDispatchDraft(base, FleetDispatchDraftInput{
		Ships:      map[int]int{FleetRecycler: 1},
		Target:     Coordinates{Galaxy: 1, System: 2, Position: 4},
		TargetType: GamePlanetTypeDebris,
	})
	if len(debris.MissionOptions) != 1 || debris.MissionOptions[0].ID != FleetMissionRecycle {
		t.Fatalf("unexpected debris missions with recycler: %+v", debris.MissionOptions)
	}

	noRecycler := BuildFleetDispatchDraft(base, FleetDispatchDraftInput{
		Ships:      map[int]int{FleetSmallCargo: 1},
		Target:     Coordinates{Galaxy: 1, System: 2, Position: 4},
		TargetType: GamePlanetTypeDebris,
	})
	if len(noRecycler.MissionOptions) != 0 {
		t.Fatalf("debris without recycler should have no suitable missions: %+v", noRecycler.MissionOptions)
	}

	moon := BuildFleetDispatchDraft(base, FleetDispatchDraftInput{
		Ships:      map[int]int{FleetDeathstar: 1},
		Target:     Coordinates{Galaxy: 1, System: 2, Position: 4},
		TargetType: GamePlanetTypeMoon,
	})
	if len(moon.MissionOptions) != 3 || moon.MissionOptions[2].ID != FleetMissionDestroy {
		t.Fatalf("moon with deathstar should include destroy: %+v", moon.MissionOptions)
	}

	colony := BuildFleetDispatchDraft(base, FleetDispatchDraftInput{
		Ships:      map[int]int{FleetColonyShip: 1},
		Target:     Coordinates{Galaxy: 1, System: 2, Position: 5},
		TargetType: GamePlanetTypePlanet,
	})
	if len(colony.MissionOptions) != 3 || colony.MissionOptions[2].ID != FleetMissionColonize {
		t.Fatalf("planet target with colony ship should include colonize draft option: %+v", colony.MissionOptions)
	}
}

func TestFleetDispatchTargetHasUnionMatchesLegacyTargetTypes(t *testing.T) {
	target := Coordinates{Galaxy: 2, System: 3, Position: 4}
	fleet := Fleet{Missions: []FleetMission{
		{UnionID: 0, Target: target, TargetType: PlanetTypePlanet},
		{UnionID: 9, Target: target, TargetType: PlanetTypeMoon},
		{UnionID: 10, Target: Coordinates{Galaxy: 2, System: 3, Position: 5}, TargetType: PlanetTypeDebris},
	}}

	if !fleetDispatchTargetHasUnion(fleet, target, GamePlanetTypeMoon) {
		t.Fatalf("expected moon target to find ACS union: %+v", fleet.Missions)
	}
	if fleetDispatchTargetHasUnion(fleet, target, GamePlanetTypeDebris) {
		t.Fatalf("non-matching debris target must not find ACS union: %+v", fleet.Missions)
	}
	if got := fleetMissionTargetGameType(PlanetTypeMoon); got != GamePlanetTypeMoon {
		t.Fatalf("unexpected moon game type: %d", got)
	}
	if got := fleetMissionTargetGameType(PlanetTypeDebris); got != GamePlanetTypeDebris {
		t.Fatalf("unexpected debris game type: %d", got)
	}
	if got := fleetMissionTargetGameType(PlanetTypePlanet); got != GamePlanetTypePlanet {
		t.Fatalf("unexpected planet game type: %d", got)
	}
}

func TestFleetDispatchTargetIsOwnChecksCurrentAndSwitcherPlanets(t *testing.T) {
	fleet := Fleet{
		CurrentPlanet: PlanetOverview{
			Type:        PlanetTypePlanet,
			Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 3},
		},
		PlanetSwitcher: []PlanetSummary{{
			Type:        PlanetTypeMoon,
			Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 4},
		}},
	}

	if !fleetDispatchTargetIsOwn(fleet, Coordinates{Galaxy: 1, System: 2, Position: 3}, GamePlanetTypePlanet) {
		t.Fatal("current planet should be treated as own target")
	}
	if !fleetDispatchTargetIsOwn(fleet, Coordinates{Galaxy: 1, System: 2, Position: 4}, GamePlanetTypeMoon) {
		t.Fatal("moon in planet switcher should be treated as own target")
	}
	if fleetDispatchTargetIsOwn(fleet, Coordinates{Galaxy: 1, System: 2, Position: 4}, GamePlanetTypePlanet) {
		t.Fatal("same coordinates with different game type must not be own target")
	}
	if fleetDispatchTargetIsOwn(fleet, Coordinates{Galaxy: 1, System: 2, Position: 5}, GamePlanetTypePlanet) {
		t.Fatal("unknown coordinates must not be own target")
	}
}

func TestBuildFleetDispatchValidationPlansLegacyResourceLoading(t *testing.T) {
	fleet := BuildFleet(Overview{
		CurrentPlanet: PlanetOverview{
			Type:        PlanetTypePlanet,
			Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 3},
			Resources:   Resources{Metal: 20000, Crystal: 20000, Deuterium: 5000},
		},
	}, FleetCounts{
		FleetSmallCargo:     4,
		FleetEspionageProbe: 2,
	}, ResearchLevels{ResearchCombustionDrive: 1}, nil, false, false)

	draft, issue := BuildFleetDispatchValidation(fleet, FleetDispatchValidationInput{
		Ships: map[int]int{
			FleetSmallCargo:     4,
			FleetEspionageProbe: 2,
		},
		Resources: map[int]int{
			ResourceMetal:     15000,
			ResourceCrystal:   10000,
			ResourceDeuterium: 100,
		},
		Target:     Coordinates{Galaxy: 2, System: 3, Position: 4},
		TargetType: GamePlanetTypePlanet,
		Mission:    FleetMissionTransport,
		Speed:      10,
	})
	if issue != nil || !draft.Ready {
		t.Fatalf("expected dispatch validation to pass, issue=%+v draft=%+v", issue, draft)
	}
	if draft.FuelConsumption != 92 || draft.RemainingCargo != 0 {
		t.Fatalf("unexpected fuel or remaining cargo: %+v", draft)
	}
	if len(draft.Resources) != 3 || draft.Resources[0].Loaded != 15000 || draft.Resources[1].Loaded != 4909 || draft.Resources[2].Loaded != 0 {
		t.Fatalf("unexpected capped resource loading plan: %+v", draft.Resources)
	}
}

func TestFleetDispatchResourcePlanClampsRequestedResources(t *testing.T) {
	rows, remaining := fleetDispatchResourcePlan(
		Resources{Metal: 100, Crystal: 50, Deuterium: 25},
		map[int]int{ResourceMetal: -10, ResourceCrystal: 500, ResourceDeuterium: 20},
		60,
	)

	if remaining != 0 ||
		rows[0].Requested != 0 ||
		rows[0].Loaded != 0 ||
		rows[1].Requested != 50 ||
		rows[1].Loaded != 50 ||
		rows[2].Requested != 20 ||
		rows[2].Loaded != 10 {
		t.Fatalf("unexpected clamped resource plan: rows=%+v remaining=%d", rows, remaining)
	}
}

func TestBuildFleetDispatchValidationAppliesLegacyHoldFuel(t *testing.T) {
	fleet := BuildFleet(Overview{
		CurrentPlanet: PlanetOverview{
			Type:        PlanetTypePlanet,
			Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 3},
			Resources:   Resources{Metal: 100000, Deuterium: 100000},
		},
	}, FleetCounts{FleetSmallCargo: 1}, ResearchLevels{ResearchCombustionDrive: 1, ResearchExpedition: 9}, nil, false, false)

	draft, issue := BuildFleetDispatchValidation(fleet, FleetDispatchValidationInput{
		Ships:           map[int]int{FleetSmallCargo: 1},
		Resources:       map[int]int{ResourceMetal: 100000},
		Target:          Coordinates{Galaxy: 1, System: 2, Position: GalaxyFarSpace},
		TargetType:      GamePlanetTypePlanet,
		Mission:         FleetMissionExpedition,
		Speed:           10,
		ExpeditionHours: 3,
	})
	if issue != nil || !draft.Ready {
		t.Fatalf("expected expedition validation to pass, issue=%+v draft=%+v", issue, draft)
	}
	counts := FleetCounts{FleetSmallCargo: 1}
	noHoldFuel := fleetFlightConsumption(fleet.Ships, counts, draft.Distance, draft.DurationSeconds, draft.SpeedFactor, 0)
	holdFuel := fleetFlightConsumption(fleet.Ships, counts, draft.Distance, draft.DurationSeconds, draft.SpeedFactor, 3)
	if draft.FuelConsumption != holdFuel || draft.FuelConsumption <= noHoldFuel {
		t.Fatalf("expected hold fuel %d above no-hold %d, got draft %+v", holdFuel, noHoldFuel, draft)
	}
	if draft.Resources[0].Loaded != draft.Cargo-holdFuel {
		t.Fatalf("expected hold fuel to reduce transport cargo, resources=%+v draft=%+v", draft.Resources, draft)
	}
}

func TestBuildFleetDispatchValidationReturnsLegacyCommonIssues(t *testing.T) {
	base := BuildFleet(Overview{
		CurrentPlanet: PlanetOverview{
			Type:        PlanetTypePlanet,
			Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 3},
			Resources:   Resources{Deuterium: 5000},
		},
	}, FleetCounts{FleetSmallCargo: 1}, ResearchLevels{ResearchCombustionDrive: 1}, nil, false, false)
	base.Slots = FleetSlots{Used: 0, Max: 1}

	tests := []struct {
		name  string
		fleet Fleet
		input FleetDispatchValidationInput
		want  string
	}{
		{
			name:  "no ships",
			fleet: base,
			input: FleetDispatchValidationInput{Target: Coordinates{Galaxy: 1, System: 2, Position: 4}, TargetType: GamePlanetTypePlanet, Mission: FleetMissionTransport, Speed: 10},
			want:  FleetIssueNoShips,
		},
		{
			name:  "same planet",
			fleet: base,
			input: FleetDispatchValidationInput{Ships: map[int]int{FleetSmallCargo: 1}, Target: base.CurrentPlanet.Coordinates, TargetType: GamePlanetTypePlanet, Mission: FleetMissionTransport, Speed: 10},
			want:  FleetIssueSamePlanet,
		},
		{
			name: "max fleet",
			fleet: func() Fleet {
				fleet := base
				fleet.Slots = FleetSlots{Used: 1, Max: 1}
				return fleet
			}(),
			input: FleetDispatchValidationInput{Ships: map[int]int{FleetSmallCargo: 1}, Target: Coordinates{Galaxy: 1, System: 2, Position: 4}, TargetType: GamePlanetTypePlanet, Mission: FleetMissionTransport, Speed: 10},
			want:  FleetIssueMaxFleet,
		},
		{
			name:  "invalid order",
			fleet: base,
			input: FleetDispatchValidationInput{Ships: map[int]int{FleetSmallCargo: 1}, Target: Coordinates{Galaxy: 1, System: 2, Position: 4}, TargetType: GamePlanetTypePlanet, Mission: FleetMissionSpy, Speed: 10},
			want:  FleetIssueInvalidOrder,
		},
		{
			name: "no fuel",
			fleet: func() Fleet {
				fleet := base
				fleet.CurrentPlanet.Resources.Deuterium = 0
				return fleet
			}(),
			input: FleetDispatchValidationInput{Ships: map[int]int{FleetSmallCargo: 1}, Target: Coordinates{Galaxy: 2, System: 2, Position: 4}, TargetType: GamePlanetTypePlanet, Mission: FleetMissionTransport, Speed: 10},
			want:  FleetIssueNoFuel,
		},
		{
			name: "no cargo",
			fleet: BuildFleet(Overview{
				CurrentPlanet: PlanetOverview{Type: PlanetTypePlanet, Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 3}, Resources: Resources{Deuterium: 100000}},
			}, FleetCounts{FleetBomber: 1}, ResearchLevels{}, nil, false, false),
			input: FleetDispatchValidationInput{Ships: map[int]int{FleetBomber: 1}, Target: Coordinates{Galaxy: 2, System: 2, Position: 4}, TargetType: GamePlanetTypePlanet, Mission: FleetMissionTransport, Speed: 10},
			want:  FleetIssueNoCargo,
		},
		{
			name: "probe no cargo",
			fleet: BuildFleet(Overview{
				CurrentPlanet: PlanetOverview{Type: PlanetTypePlanet, Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 3}, Resources: Resources{Deuterium: 100000}},
			}, FleetCounts{FleetEspionageProbe: 1}, ResearchLevels{}, nil, false, false),
			input: FleetDispatchValidationInput{Ships: map[int]int{FleetEspionageProbe: 1}, Target: Coordinates{Galaxy: 9, System: 2, Position: 4}, TargetType: GamePlanetTypePlanet, Mission: FleetMissionSpy, Speed: 10},
			want:  FleetIssueNoCargo,
		},
		{
			name: "expedition limit",
			fleet: BuildFleet(Overview{
				CurrentPlanet: PlanetOverview{Type: PlanetTypePlanet, Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 3}, Resources: Resources{Deuterium: 100000}},
			}, FleetCounts{FleetSmallCargo: 1}, ResearchLevels{ResearchComputer: 3, ResearchCombustionDrive: 1, ResearchExpedition: 1}, []FleetMission{
				BuildFleetMission(7, FleetMissionExpedition, FleetCounts{FleetSmallCargo: 1}, Coordinates{}, Coordinates{}, PlanetTypePlanet, "", 0, 0),
			}, false, false),
			input: FleetDispatchValidationInput{Ships: map[int]int{FleetSmallCargo: 1}, Target: Coordinates{Galaxy: 1, System: 2, Position: GalaxyFarSpace}, TargetType: GamePlanetTypePlanet, Mission: FleetMissionExpedition, Speed: 10},
			want:  FleetIssueExpLimit,
		},
		{
			name: "expedition probe only",
			fleet: BuildFleet(Overview{
				CurrentPlanet: PlanetOverview{Type: PlanetTypePlanet, Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 3}, Resources: Resources{Deuterium: 100000}},
			}, FleetCounts{FleetEspionageProbe: 1}, ResearchLevels{ResearchCombustionDrive: 1, ResearchExpedition: 4}, nil, false, false),
			input: FleetDispatchValidationInput{Ships: map[int]int{FleetEspionageProbe: 1}, Target: Coordinates{Galaxy: 1, System: 2, Position: GalaxyFarSpace}, TargetType: GamePlanetTypePlanet, Mission: FleetMissionExpedition, Speed: 10},
			want:  FleetIssueExpRequired,
		},
		{
			name: "expedition invalid far space target",
			fleet: BuildFleet(Overview{
				CurrentPlanet: PlanetOverview{Type: PlanetTypePlanet, Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 3}, Resources: Resources{Deuterium: 100000}},
			}, FleetCounts{FleetSmallCargo: 1}, ResearchLevels{ResearchCombustionDrive: 1, ResearchExpedition: 4}, nil, false, false),
			input: FleetDispatchValidationInput{Ships: map[int]int{FleetSmallCargo: 1}, Target: Coordinates{Galaxy: 1, System: 2, Position: 17}, TargetType: GamePlanetTypePlanet, Mission: FleetMissionExpedition, Speed: 10},
			want:  FleetIssueInvalidTarget,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, issue := BuildFleetDispatchValidation(tt.fleet, tt.input)
			if issue == nil || issue.Code != tt.want || issue.Message == "" {
				t.Fatalf("expected issue %q, got %+v", tt.want, issue)
			}
		})
	}
	holdDraft, holdIssue := BuildFleetDispatchValidation(base, FleetDispatchValidationInput{
		Ships:      map[int]int{FleetSmallCargo: 1},
		Target:     Coordinates{Galaxy: 1, System: 2, Position: 4},
		TargetType: GamePlanetTypePlanet,
		Mission:    FleetMissionACSHold,
		Speed:      10,
		HoldHours:  1,
	})
	if holdIssue != nil || holdDraft.Mission != FleetMissionACSHold || !holdDraft.Ready {
		t.Fatalf("direct legacy ACS hold POST should pass domain validation for repository guards: draft=%+v issue=%+v", holdDraft, holdIssue)
	}
	if issue := FleetActionIssueFor("unknown"); issue.Code != "unknown" || issue.Message == "" {
		t.Fatalf("unknown fleet issue should keep code with fallback message: %+v", issue)
	}
}

func TestFleetDispatchFlightMathMatchesLegacyFormulas(t *testing.T) {
	origin := Coordinates{Galaxy: 1, System: 2, Position: 3}
	if got := fleetFlightDistance(origin, Coordinates{Galaxy: 1, System: 2, Position: 3}); got != 5 {
		t.Fatalf("unexpected same planet distance: %d", got)
	}
	if got := fleetFlightDistance(origin, Coordinates{Galaxy: 1, System: 2, Position: 5}); got != 1010 {
		t.Fatalf("unexpected same system distance: %d", got)
	}
	if got := fleetFlightDistance(origin, Coordinates{Galaxy: 1, System: 4, Position: 9}); got != 2890 {
		t.Fatalf("unexpected same galaxy distance: %d", got)
	}
	if got := fleetFlightDistance(origin, Coordinates{Galaxy: 2, System: 4, Position: 9}); got != 20000 {
		t.Fatalf("unexpected cross galaxy distance: %d", got)
	}
	if got := fleetFlightTime(1010, 5500, 10, 1); got != 4753 {
		t.Fatalf("unexpected legacy flight time: %d", got)
	}
	if got := fleetFlightTime(20000, 5500, 10, 128); got != 165 {
		t.Fatalf("unexpected 128x legacy flight time: %d", got)
	}
	ships := []FleetShipSelection{
		{ID: FleetSmallCargo, Speed: 5500, Consumption: 10},
		{ID: FleetEspionageProbe, Speed: 110000000, Consumption: 1},
	}
	counts := FleetCounts{FleetSmallCargo: 4, FleetEspionageProbe: 2}
	if got := fleetDispatchMaxSpeed(ships, counts); got != 5500 {
		t.Fatalf("unexpected slowest fleet speed: %d", got)
	}
	if got := fleetFlightConsumption(ships, counts, 20000, 21116, 1, 0); got != 92 {
		t.Fatalf("unexpected legacy fuel consumption: %d", got)
	}
	if got := fleetFlightConsumption(ships, counts, 20000, 165, 128, 0); got != 92 {
		t.Fatalf("unexpected 128x legacy fuel consumption: %d", got)
	}
}

func TestFleetDispatchFlightMathDefensiveEdges(t *testing.T) {
	if hours := fleetDispatchHoldHours([]FleetMissionOption{{ID: FleetMissionACSHold}}); len(hours) != 7 || hours[0] != 0 || hours[6] != 32 {
		t.Fatalf("unexpected ACS hold hours: %+v", hours)
	}
	if got := fleetFlightTime(0, 5500, 10, 1); got != 0 {
		t.Fatalf("zero distance should not produce a duration: %d", got)
	}
	if got := fleetFlightTime(1010, 5500, -1, 0); got != 4753 {
		t.Fatalf("invalid speed and speed factor should default to legacy values, got %d", got)
	}
	if got := fleetFlightTime(1010, 5500, 99, 1); got != 4753 {
		t.Fatalf("oversized speed should clamp to 100%%, got %d", got)
	}
	if got := fleetFlightConsumption(nil, nil, 0, 10, 1, 0); got != 0 {
		t.Fatalf("zero distance should not consume fuel: %d", got)
	}
	if got := fleetFlightConsumption([]FleetShipSelection{{ID: FleetSmallCargo, Speed: 5500, Consumption: 10}}, FleetCounts{FleetSmallCargo: 1}, 5, 10, 1, 0); got != 0 {
		t.Fatalf("non-positive denominator should not consume fuel: %d", got)
	}
	if got := fleetFlightConsumption([]FleetShipSelection{{ID: FleetSmallCargo}}, FleetCounts{FleetSmallCargo: 1}, 1010, 484, 0, 1); got != 0 {
		t.Fatalf("ships without speed and consumption should be ignored: %d", got)
	}
	if got := gamePlanetTypeFromPlanet(PlanetTypeMoon); got != GamePlanetTypeMoon {
		t.Fatalf("moon planet type should map to game moon type, got %d", got)
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
