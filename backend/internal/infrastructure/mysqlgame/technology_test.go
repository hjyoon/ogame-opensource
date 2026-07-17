package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

func TestTechnologyRepositoryReadsLegacyTechnology(t *testing.T) {
	queryer := &fakeQueryer{results: technologyReadResults(map[int]int{
		domaingame.BuildingDeuteriumSynth: 5,
	}, map[int]int{
		domaingame.ResearchEnergy: 3,
	})}
	repository := NewTechnologyRepositoryWithQueryer(queryer, "ogame_")

	technology, err := repository.GetTechnology(context.Background(), appgame.TechnologyQuery{
		PlayerID:            42,
		TechnologyDetailsID: domaingame.FleetCruiser,
		TechnologyInfoID:    domaingame.BuildingMetalMine,
	})
	if err != nil {
		t.Fatal(err)
	}

	if technology.Commander != "legor" || technology.CurrentPlanet.ID != 99 || len(technology.Groups) != 5 {
		t.Fatalf("unexpected technology summary: %+v", technology)
	}
	fusion := findTechnologyItem(t, technology, domaingame.BuildingFusionReactor)
	if fusion.Name != "Fusion Reactor" || len(fusion.Requirements) != 2 || !fusion.Requirements[0].Met || !fusion.Requirements[1].Met {
		t.Fatalf("expected met fusion requirements, got %+v", fusion)
	}
	if !strings.Contains(queryer.calls[4].sql, "`1`, `2`, `3`") || !strings.Contains(queryer.calls[5].sql, "`106`, `108`, `109`") {
		t.Fatalf("expected legacy numeric columns, got %+v", queryer.calls)
	}
	if technology.Details == nil || technology.Details.Target.ID != domaingame.FleetCruiser || len(technology.Details.Levels) == 0 {
		t.Fatalf("expected cruiser detail tree, got %+v", technology.Details)
	}
	if technology.Info == nil || technology.Info.ID != domaingame.BuildingMetalMine || len(technology.Info.Rows) != 15 {
		t.Fatalf("expected metal mine info rows, got %+v", technology.Info)
	}
	if !strings.Contains(queryer.calls[6].sql, "speed") || !strings.Contains(queryer.calls[6].sql, "defrepair") {
		t.Fatalf("expected technology universe settings query, got %+v", queryer.calls[6])
	}
}

func TestTechnologyRepositoryMapsMCPTechnology(t *testing.T) {
	queryer := &fakeQueryer{results: technologyReadResults(map[int]int{
		domaingame.BuildingMetalMine: 12,
	}, map[int]int{
		domaingame.ResearchEnergy: 3,
	})}
	repository := NewTechnologyRepositoryWithQueryer(queryer, "ogame_")

	technology, err := repository.GetMCPTechnology(context.Background(), 42, domainmcp.TechnologyCommand{
		DetailsID: domaingame.FleetCruiser,
		InfoID:    domaingame.BuildingMetalMine,
	})
	if err != nil {
		t.Fatal(err)
	}
	if technology.PlayerID != 42 || technology.PlanetID != 99 || len(technology.Groups) != 5 {
		t.Fatalf("unexpected mcp technology summary: %+v", technology)
	}
	if technology.Details == nil || technology.Details.Target.ID != domaingame.FleetCruiser || len(technology.Details.Levels) == 0 {
		t.Fatalf("expected mcp cruiser detail tree, got %+v", technology.Details)
	}
	if technology.Info == nil || technology.Info.ID != domaingame.BuildingMetalMine || technology.Info.Level != 12 || len(technology.Info.Rows) != 15 {
		t.Fatalf("expected mcp metal mine info rows, got %+v", technology.Info)
	}
	if len(technology.Groups[0].Items) == 0 || technology.Groups[0].Items[0].Name == "" {
		t.Fatalf("expected mcp technology group items, got %+v", technology.Groups[0])
	}

	if mcpTechnologyUnitInfo(nil) != nil || mcpTechnologyAllianceDepotInfo(nil) != nil {
		t.Fatal("nil technology special info must remain nil")
	}
	unit := mcpTechnologyUnitInfo(&domaingame.TechnologyUnitInfo{
		Structure:    4000,
		Shield:       10,
		Attack:       50,
		Cargo:        50,
		BaseSpeed:    12500,
		RapidFireOut: []domaingame.TechnologyRapidFire{{ID: domaingame.FleetEspionageProbe, Name: "Espionage Probe", Count: 5}},
		RapidFireIn:  []domaingame.TechnologyRapidFire{{ID: domaingame.FleetCruiser, Name: "Cruiser", Count: 6}},
	})
	if unit == nil || unit.Structure != 4000 || unit.RapidFireOut[0].Count != 5 || unit.RapidFireIn[0].ID != domaingame.FleetCruiser {
		t.Fatalf("unexpected MCP technology unit mapping: %+v", unit)
	}
	depot := mcpTechnologyAllianceDepotInfo(&domaingame.TechnologyAllianceDepotInfo{AvailableDeuterium: 1000, Capacity: 10000})
	if depot == nil || depot.AvailableDeuterium != 1000 || depot.Capacity != 10000 {
		t.Fatalf("unexpected MCP alliance depot mapping: %+v", depot)
	}
	if mcpTechnologyDetails(nil) != nil || mcpTechnologyInfo(nil) != nil {
		t.Fatal("nil technology details and info must remain nil")
	}
}

func TestNewTechnologyRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewTechnologyRepository(nil, "ogame_")

	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
	if _, err := (TechnologyRepository{}).GetMCPTechnology(context.Background(), 42, domainmcp.TechnologyCommand{}); err == nil {
		t.Fatal("expected nil technology reader error")
	}
}

func TestTechnologyRepositoryReturnsErrors(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		queryer *fakeQueryer
		want    string
		details int
	}{
		{
			name:    "unsafe prefix",
			prefix:  "bad-prefix_",
			queryer: &fakeQueryer{},
			want:    "invalid database table prefix",
		},
		{
			name:    "overview",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{{err: errors.New("overview user failed")}}},
			want:    "overview user failed",
		},
		{
			name:    "building levels",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{err: errors.New("building query failed")})},
			want:    "building query failed",
		},
		{
			name:    "research levels",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(nil))}, fakeQueryResult{err: errors.New("research query failed")})},
			want:    "research query failed",
		},
		{
			name:   "universe settings",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: append(
				shipyardOverviewResults(),
				fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(nil))},
				fakeQueryResult{rows: fakeRowsFromValues(allResearchLevelRow(nil))},
				fakeQueryResult{err: errors.New("universe settings failed")},
			)},
			want:    "universe settings failed",
			details: domaingame.FleetCruiser,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewTechnologyRepositoryWithQueryer(tt.queryer, tt.prefix)
			_, err := repository.GetTechnology(context.Background(), appgame.TechnologyQuery{
				PlayerID:            42,
				TechnologyDetailsID: tt.details,
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestTechnologyRepositoryLoadsUniverseSettingsBranches(t *testing.T) {
	tests := []struct {
		name       string
		prefix     string
		result     fakeQueryResult
		wantErr    string
		wantSpeed  float64
		wantRepair int
	}{
		{
			name:    "unsafe prefix",
			prefix:  "bad-prefix_",
			wantErr: "invalid database table prefix",
		},
		{
			name:    "query error",
			prefix:  "ogame_",
			result:  fakeQueryResult{err: errors.New("settings query failed")},
			wantErr: "settings query failed",
		},
		{
			name:    "empty universe",
			prefix:  "ogame_",
			result:  fakeQueryResult{rows: fakeRowsFromValues()},
			wantErr: "no rows",
		},
		{
			name:    "row iteration error",
			prefix:  "ogame_",
			result:  fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("settings rows failed"))},
			wantErr: "settings rows failed",
		},
		{
			name:    "scan error",
			prefix:  "ogame_",
			result:  fakeQueryResult{rows: fakeRowsFromValues([]any{float64(128)})},
			wantErr: "unexpected scan destination count",
		},
		{
			name:       "nonpositive speed defaults to one",
			prefix:     "ogame_",
			result:     fakeQueryResult{rows: fakeRowsFromValues([]any{float64(0), 70})},
			wantSpeed:  1,
			wantRepair: 70,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryer := &fakeQueryer{}
			if tt.prefix == "ogame_" {
				queryer.results = []fakeQueryResult{tt.result}
			}
			settings, err := NewTechnologyRepositoryWithQueryer(queryer, tt.prefix).loadTechnologyUniverseSettings(context.Background())
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected %q error, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if settings.speed != tt.wantSpeed || settings.defenseRepair != tt.wantRepair {
				t.Fatalf("unexpected universe settings: %+v", settings)
			}
		})
	}
}

func TestTechnologyRepositoryMCPPropagatesReadError(t *testing.T) {
	repository := NewTechnologyRepositoryWithQueryer(&fakeQueryer{}, "bad-prefix_")
	if _, err := repository.GetMCPTechnology(context.Background(), 42, domainmcp.TechnologyCommand{}); err == nil ||
		!strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected MCP technology read error, got %v", err)
	}
}

func technologyReadResults(buildings map[int]int, research map[int]int) []fakeQueryResult {
	return append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(buildings))},
		fakeQueryResult{rows: fakeRowsFromValues(allResearchLevelRow(research))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{float64(128), 70})},
	)
}

func findTechnologyItem(t *testing.T, technology domaingame.Technology, id int) domaingame.TechnologyItem {
	t.Helper()
	for _, group := range technology.Groups {
		for _, item := range group.Items {
			if item.ID == id {
				return item
			}
		}
	}
	t.Fatalf("technology item %d not found in %+v", id, technology.Groups)
	return domaingame.TechnologyItem{}
}
