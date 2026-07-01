package game

import "testing"

func TestBuildGalaxyClampsCoordinatesAndRows(t *testing.T) {
	overview := galaxyOverview(100)
	galaxy := BuildGalaxy(overview, GalaxyInput{
		Coordinates: Coordinates{Galaxy: 99, System: -5, Position: 99},
		Bounds:      GalaxyBounds{Galaxies: 9, Systems: 499},
		Viewer:      GalaxyViewer{PlayerID: 42, Score: 10000, Admin: 0},
		FleetSlots:  FleetSlots{Used: 1, BaseMax: 4, Max: 6, Admiral: true},
		Objects: []GalaxyObject{{
			ID:          200,
			Name:        "Target",
			Type:        PlanetTypePlanet,
			Coordinates: Coordinates{Galaxy: 9, System: 1, Position: 3},
			Owner:       GalaxyObjectPlayer{ID: 7, Name: "enemy", Score: 1000, Rank: 12, LastClick: 900},
		}},
		Now: 1000,
	})

	if galaxy.Coordinates.Galaxy != 9 || galaxy.Coordinates.System != 1 || galaxy.Coordinates.Position != GalaxyFarSpace {
		t.Fatalf("unexpected clamped coordinates: %+v", galaxy.Coordinates)
	}
	if len(galaxy.Rows) != GalaxyPositions || galaxy.Rows[2].Planet == nil || galaxy.Populated != 1 {
		t.Fatalf("unexpected rows: populated=%d rows=%+v", galaxy.Populated, galaxy.Rows)
	}
	if !galaxy.RemoteSystemCostDue || !galaxy.NotEnoughDeuterium {
		t.Fatalf("expected remote system deuterium warning, got due=%v enough=%v", galaxy.RemoteSystemCostDue, galaxy.NotEnoughDeuterium)
	}
	if galaxy.Slots.Max != 6 || !galaxy.Extra.Slots.Admiral {
		t.Fatalf("unexpected slot summary: %+v extra=%+v", galaxy.Slots, galaxy.Extra)
	}
}

func TestBuildGalaxyUsesLegacyStatusPriority(t *testing.T) {
	now := int64(604800 * 5)
	tests := []struct {
		name        string
		viewerScore int64
		owner       GalaxyObjectPlayer
		wantStatus  string
		wantSuffix  string
	}{
		{
			name:        "noob",
			viewerScore: 10000,
			owner:       GalaxyObjectPlayer{ID: 7, Name: "noob", Score: 1000, LastClick: now},
			wantStatus:  "noob",
			wantSuffix:  "n",
		},
		{
			name:        "strong",
			viewerScore: 1000,
			owner:       GalaxyObjectPlayer{ID: 8, Name: "strong", Score: 10000, LastClick: now},
			wantStatus:  "strong",
			wantSuffix:  "s",
		},
		{
			name:        "vacation overrides inactive",
			viewerScore: 10000,
			owner:       GalaxyObjectPlayer{ID: 9, Name: "vac", Score: 8000, LastClick: now - 604800*5, Vacation: true},
			wantStatus:  "vacation",
			wantSuffix:  "V",
		},
		{
			name:        "banned wins over longinactive until vacation",
			viewerScore: 10000,
			owner:       GalaxyObjectPlayer{ID: 10, Name: "ban", Score: 8000, LastClick: now - 604800*5, Banned: true},
			wantStatus:  "banned",
			wantSuffix:  "b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			galaxy := BuildGalaxy(galaxyOverview(tt.viewerScore), GalaxyInput{
				Bounds: GalaxyBounds{Galaxies: 9, Systems: 499},
				Viewer: GalaxyViewer{
					PlayerID: 42,
					Score:    tt.viewerScore,
					Flags:    GalaxyActionSpy | GalaxyActionMessage | GalaxyActionBuddy,
				},
				Objects: []GalaxyObject{{
					ID:          200,
					Name:        "Target",
					Type:        PlanetTypePlanet,
					Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 3},
					Owner:       tt.owner,
				}},
				Now: now,
			})
			player := galaxy.Rows[2].Planet.Player
			if player == nil || player.Status != tt.wantStatus {
				t.Fatalf("unexpected player status: %+v", player)
			}
			found := false
			for _, suffix := range player.Suffixes {
				if suffix.Text == tt.wantSuffix {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected suffix %q in %+v", tt.wantSuffix, player.Suffixes)
			}
		})
	}
}

func TestBuildGalaxyDebrisMoonActivityAndActions(t *testing.T) {
	now := int64(3600)
	galaxy := BuildGalaxy(galaxyOverview(10000), GalaxyInput{
		Bounds: GalaxyBounds{Galaxies: 9, Systems: 499},
		Viewer: GalaxyViewer{
			PlayerID:  42,
			Score:     10000,
			Flags:     GalaxyActionSpy | GalaxyActionMessage | GalaxyActionBuddy | GalaxyActionMissile | GalaxyActionReport,
			SpyProbes: 4,
			Recyclers: 3,
			Missiles:  2,
			Commander: true,
		},
		FleetSlots: FleetSlots{Used: 2, BaseMax: 3, Max: 3},
		Objects: []GalaxyObject{
			{
				ID:           200,
				Name:         "Target",
				Type:         PlanetTypePlanet,
				Coordinates:  Coordinates{Galaxy: 1, System: 2, Position: 3},
				LastActivity: now - 50*60,
				Owner:        GalaxyObjectPlayer{ID: 7, Name: "enemy", Score: 8000, LastClick: now},
				ReportID:     901,
			},
			{
				ID:           201,
				Name:         "Moon",
				Type:         PlanetTypeMoon,
				Coordinates:  Coordinates{Galaxy: 1, System: 2, Position: 3},
				LastActivity: now - 60,
				Owner:        GalaxyObjectPlayer{ID: 7, Name: "enemy", Score: 8000, LastClick: now},
				ReportID:     902,
			},
			{
				ID:            202,
				Type:          PlanetTypeDebris,
				Coordinates:   Coordinates{Galaxy: 1, System: 2, Position: 3},
				DebrisMetal:   200,
				DebrisCrystal: 100,
			},
		},
		Now: now,
	})

	row := galaxy.Rows[2]
	if row.Planet == nil || row.Planet.ActivityText != "(*)" || !row.Planet.Actions.Spy || !row.Planet.Actions.Missile || !row.Planet.Actions.ViewReport || row.Planet.ReportID != 901 {
		t.Fatalf("unexpected planet row: %+v", row.Planet)
	}
	if row.Moon == nil || !row.Moon.Actions.Destroy || !row.Moon.Actions.ViewReport || row.Moon.ReportID != 902 {
		t.Fatalf("expected moon destroy action, got %+v", row.Moon)
	}
	if row.Debris == nil || !row.Debris.Visible || row.Debris.Harvesters != 1 {
		t.Fatalf("unexpected debris: %+v", row.Debris)
	}
	if !galaxy.Extra.Commander || galaxy.Extra.SpyProbes != 4 || galaxy.Extra.Recyclers != 3 || galaxy.Extra.Missiles != 2 {
		t.Fatalf("unexpected extra info: %+v", galaxy.Extra)
	}
}

func TestBuildGalaxyHandlesEmptyBoundsOwnRowsAndPhantomDebris(t *testing.T) {
	overview := galaxyOverview(1000)
	overview.CurrentPlanet.Resources.Deuterium = 20
	galaxy := BuildGalaxy(overview, GalaxyInput{
		Bounds: GalaxyBounds{},
		Viewer: GalaxyViewer{PlayerID: 42, Score: 1000},
		Objects: []GalaxyObject{
			{
				ID:          99,
				Name:        "Own",
				Type:        PlanetTypePlanet,
				Coordinates: Coordinates{Galaxy: 1, System: 1, Position: 1},
				Owner:       GalaxyObjectPlayer{ID: 42, Name: "legor", Score: 1000},
			},
			{
				ID:          100,
				Name:        "Ruins",
				Type:        PlanetTypeDestroyedPlanet,
				Coordinates: Coordinates{Galaxy: 1, System: 1, Position: 2},
				Owner:       GalaxyObjectPlayer{ID: 7, Name: "enemy", Score: 1000},
			},
			{
				ID:            101,
				Type:          PlanetTypeDebris,
				Coordinates:   Coordinates{Galaxy: 1, System: 1, Position: 2},
				DebrisMetal:   100,
				DebrisCrystal: 100,
			},
		},
		Now: 0,
	})

	if galaxy.Bounds.Galaxies != 1 || galaxy.Bounds.Systems != 1 || galaxy.Coordinates.Position != 3 {
		t.Fatalf("unexpected default bounds or fallback coordinates: bounds=%+v coords=%+v", galaxy.Bounds, galaxy.Coordinates)
	}
	if own := galaxy.Rows[0].Planet; own == nil || !own.Own || !own.Actions.Deploy || own.Player == nil || own.Player.Status != "normal" {
		t.Fatalf("unexpected own planet row: %+v", own)
	}
	if destroyed := galaxy.Rows[1].Planet; destroyed == nil || destroyed.DisplayName != "Destroyed Planet" || destroyed.Player != nil || destroyed.Actions.Attack {
		t.Fatalf("unexpected destroyed planet row: %+v", destroyed)
	}
	if debris := galaxy.Rows[1].Debris; debris == nil || debris.Visible || debris.Harvesters != 1 {
		t.Fatalf("small debris should stay hidden with recycler count, got %+v", debris)
	}
}

func TestGalaxyHelpersCoverLegacyEdgeCases(t *testing.T) {
	coordinates := clampGalaxyCoordinates(
		Coordinates{Galaxy: -1, System: 999, Position: -1},
		Coordinates{Galaxy: 2, System: 2, Position: 2},
		GalaxyBounds{Galaxies: 9, Systems: 499},
	)
	if coordinates.Galaxy != 1 || coordinates.System != 499 || coordinates.Position != 1 {
		t.Fatalf("unexpected low/high clamp: %+v", coordinates)
	}

	if got := galaxyActivityText(100, 200, false, 0); got != "" {
		t.Fatalf("activity without clock should be empty, got %q", got)
	}
	if got := galaxyActivityText(4000, 0, false, 3600); got != "(*)" {
		t.Fatalf("future activity should be active marker, got %q", got)
	}
	if got := galaxyActivityText(3600-30*60, 0, false, 3600); got != "(30 min)" {
		t.Fatalf("expected minute activity marker, got %q", got)
	}
	if got := galaxyActivityText(0, 0, false, 3600); got != "" {
		t.Fatalf("missing activity should be empty, got %q", got)
	}

	if actions := galaxyActions(PlanetTypePlanet, true, GalaxyViewer{}); !actions.Deploy || !actions.Transport || actions.Attack {
		t.Fatalf("unexpected own actions: %+v", actions)
	}
	if actions := galaxyActions(PlanetTypeAbandoned, false, GalaxyViewer{Flags: GalaxyActionSpy}); actions.Spy || actions.Attack {
		t.Fatalf("abandoned planets should not expose actions: %+v", actions)
	}
	if legacyMinutes(-10) != "0" || legacyMinutes(125) != "2" {
		t.Fatalf("unexpected legacy minute formatting")
	}
}

func TestBuildGalaxyKeepsStandaloneMoonDebrisAndSkipsInvalidObjects(t *testing.T) {
	galaxy := BuildGalaxy(galaxyOverview(1000), GalaxyInput{
		Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 3},
		Bounds:      GalaxyBounds{Galaxies: 9, Systems: 499},
		Viewer:      GalaxyViewer{PlayerID: 42, Score: 1000},
		Objects: []GalaxyObject{
			{
				ID:          1,
				Name:        "Orphan Moon",
				Type:        PlanetTypeMoon,
				Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 5},
				Owner:       GalaxyObjectPlayer{ID: 7, Name: "enemy", Score: 1000},
			},
			{
				ID:            2,
				Type:          PlanetTypeDebris,
				Coordinates:   Coordinates{Galaxy: 1, System: 2, Position: 6},
				DebrisMetal:   400,
				DebrisCrystal: 0,
			},
			{
				ID:          3,
				Name:        "Skipped",
				Type:        PlanetTypePlanet,
				Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 99},
			},
			{
				ID:          4,
				Name:        "Unknown",
				Type:        999999,
				Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 7},
			},
		},
		Now: 1000,
	})

	if galaxy.Populated != 0 {
		t.Fatalf("standalone moon/debris should not count as populated planets: %+v", galaxy)
	}
	if galaxy.Rows[4].Moon == nil || galaxy.Rows[4].Moon.Name != "Orphan Moon" {
		t.Fatalf("expected standalone moon row, got %+v", galaxy.Rows[4])
	}
	if galaxy.Rows[5].Debris == nil || !galaxy.Rows[5].Debris.Visible {
		t.Fatalf("expected standalone visible debris row, got %+v", galaxy.Rows[5])
	}
	if galaxy.Rows[6].Planet != nil {
		t.Fatalf("unknown object type should be ignored, got %+v", galaxy.Rows[6])
	}
}

func TestGalaxyMissileHelpersCoverLegacyEdgeCases(t *testing.T) {
	if !GalaxyMissileTargetAllowed(DefenseRocketLauncher) || !GalaxyMissileTargetAllowed(DefenseLargeShieldDome) || GalaxyMissileTargetAllowed(999999) {
		t.Fatalf("unexpected missile target allow-list result")
	}
	if issue := GalaxyActionIssueFor("unknown"); issue.Code != "unknown" || issue.Message != "There was an error" {
		t.Fatalf("unexpected fallback issue: %+v", issue)
	}
	if issue := GalaxyActionIssueFromFleet(nil); issue != nil {
		t.Fatalf("nil fleet issue should stay nil, got %+v", issue)
	}
	if issue := GalaxyActionIssueFromFleet(FleetActionIssueFor(FleetIssueNoShips)); issue == nil || issue.Code != "fleet_no_ships" {
		t.Fatalf("unexpected fleet issue conversion: %+v", issue)
	}
	if issue := GalaxyMissileLaunchedIssue(-3); issue.Code != GalaxyIssueRocketLaunched || issue.Message != "Start of rocket 3!" {
		t.Fatalf("unexpected negative launch issue: %+v", issue)
	}
	if issue := GalaxyMissileLaunchedIssue(0); issue.Code != GalaxyIssueRocketNoRockets {
		t.Fatalf("unexpected zero launch issue: %+v", issue)
	}
	if !GalaxyPlayerProtectedFromMissiles(
		GalaxyObjectPlayer{Score: 1000, LastClick: 1000},
		GalaxyViewer{Score: 100000},
		1000,
	) {
		t.Fatal("expected active low-score target to be protected")
	}
	if GalaxyPlayerProtectedFromMissiles(
		GalaxyObjectPlayer{Score: 1000, LastClick: 1000, Vacation: true},
		GalaxyViewer{Score: 100000},
		1000,
	) {
		t.Fatal("vacation target should not use noob protection calculation")
	}
}

func galaxyOverview(score int64) Overview {
	return Overview{
		Commander: "legor",
		Score:     ScoreSummary{RawScore: score},
		CurrentPlanet: PlanetOverview{
			ID:          99,
			Name:        "Homeworld",
			Type:        PlanetTypePlanet,
			Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 3},
			Resources:   Resources{Deuterium: 5},
		},
		PlanetSwitcher: []PlanetSummary{{
			ID:          99,
			Name:        "Homeworld",
			Type:        PlanetTypePlanet,
			Coordinates: Coordinates{Galaxy: 1, System: 2, Position: 3},
			Current:     true,
		}},
	}
}
