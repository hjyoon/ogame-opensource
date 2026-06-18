package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestBuildingsRepositoryReadsLegacyBuildings(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(123456), 7, 99, 1, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3, 12800, 19, 4, 163, 10000.0, 10000.0, 10000.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{2})},
		{rows: fakeRowsFromValues(buildingLevelRow(map[int]int{
			domaingame.BuildingMetalMine:       2,
			domaingame.BuildingDeuteriumSynth:  5,
			domaingame.BuildingRoboticsFactory: 10,
		}))},
		{rows: fakeRowsFromValues(researchLevelRow(map[int]int{
			domaingame.ResearchComputer: 10,
			domaingame.ResearchEnergy:   3,
		}))},
		{rows: fakeRowsFromValues([]any{2.0})},
	}}
	repository := NewBuildingsRepositoryWithQueryer(queryer, "ogame_")

	buildings, err := repository.GetBuildings(context.Background(), appgame.BuildingsQuery{PlayerID: 42})
	if err != nil {
		t.Fatal(err)
	}

	if buildings.Commander != "legor" || buildings.CurrentPlanet.ID != 99 {
		t.Fatalf("unexpected buildings summary: %+v", buildings)
	}
	if !containsBuilding(buildings, domaingame.BuildingFusionReactor) || !containsBuilding(buildings, domaingame.BuildingNaniteFactory) {
		t.Fatalf("expected gated buildings after matching levels: %+v", buildings.Items)
	}
	metalMine := buildingByID(t, buildings, domaingame.BuildingMetalMine)
	if metalMine.Level != 2 || metalMine.DurationSeconds != 11 {
		t.Fatalf("expected speed-adjusted metal mine, got %+v", metalMine)
	}
	if !strings.Contains(queryer.calls[4].sql, "`1`, `2`, `3`") || !strings.Contains(queryer.calls[5].sql, "`108`, `113`") {
		t.Fatalf("expected numeric legacy columns, got %+v", queryer.calls)
	}
}

func TestNewBuildingsRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewBuildingsRepository(nil, "ogame_")

	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
}

func TestBuildingsRepositoryLoadHelpersHandleFallbacks(t *testing.T) {
	repository := NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues()},
	}}, "ogame_")

	speed, err := repository.loadUniverseSpeed(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if speed != 1 {
		t.Fatalf("expected missing speed row to default to one, got %v", speed)
	}

	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{0.0})},
	}}, "ogame_")
	speed, err = repository.loadUniverseSpeed(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if speed != 1 {
		t.Fatalf("expected non-positive speed to default to one, got %v", speed)
	}
}

func TestBuildingsRepositoryReturnsErrors(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		queryer *fakeQueryer
		want    string
	}{
		{
			name:    "unsafe prefix",
			prefix:  "bad-prefix_",
			queryer: &fakeQueryer{},
			want:    "invalid database table prefix",
		},
		{
			name:   "overview",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{err: errors.New("overview user failed")},
			}},
			want: "overview user failed",
		},
		{
			name:   "building levels",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{err: errors.New("building query failed")},
			}},
			want: "building query failed",
		},
		{
			name:   "missing building levels",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValues()},
			}},
			want: "building levels not found",
		},
		{
			name:   "building rows",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsError(errors.New("building rows failed"))},
			}},
			want: "building rows failed",
		},
		{
			name:   "building scan",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValues([]any{"bad"})},
			}},
			want: "unexpected scan destination count",
		},
		{
			name:   "research query",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValues(buildingLevelRow(nil))},
				{err: errors.New("research query failed")},
			}},
			want: "research query failed",
		},
		{
			name:   "research levels",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValues(buildingLevelRow(nil))},
				{rows: fakeRowsFromValues()},
			}},
			want: "research levels not found",
		},
		{
			name:   "research rows",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValues(buildingLevelRow(nil))},
				{rows: fakeRowsError(errors.New("research rows failed"))},
			}},
			want: "research rows failed",
		},
		{
			name:   "research scan",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValues(buildingLevelRow(nil))},
				{rows: fakeRowsFromValues([]any{"bad"})},
			}},
			want: "unexpected scan destination count",
		},
		{
			name:   "speed",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValues(buildingLevelRow(nil))},
				{rows: fakeRowsFromValues(researchLevelRow(nil))},
				{err: errors.New("speed query failed")},
			}},
			want: "speed query failed",
		},
		{
			name:   "speed rows",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValues(buildingLevelRow(nil))},
				{rows: fakeRowsFromValues(researchLevelRow(nil))},
				{rows: fakeRowsError(errors.New("speed rows failed"))},
			}},
			want: "speed rows failed",
		},
		{
			name:   "speed scan",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValues(buildingLevelRow(nil))},
				{rows: fakeRowsFromValues(researchLevelRow(nil))},
				{rows: fakeRowsFromValues([]any{"bad"})},
			}},
			want: "expected float64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewBuildingsRepositoryWithQueryer(tt.queryer, tt.prefix)
			_, err := repository.GetBuildings(context.Background(), appgame.BuildingsQuery{PlayerID: 42})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func buildingLevelRow(values map[int]int) []any {
	row := make([]any, 0, len(domaingame.BuildingIDs()))
	for _, id := range domaingame.BuildingIDs() {
		row = append(row, values[id])
	}
	return row
}

func researchLevelRow(values map[int]int) []any {
	row := make([]any, 0, len(domaingame.BuildingResearchIDs()))
	for _, id := range domaingame.BuildingResearchIDs() {
		row = append(row, values[id])
	}
	return row
}

func buildingByID(t *testing.T, buildings domaingame.Buildings, id int) domaingame.BuildingItem {
	t.Helper()
	for _, item := range buildings.Items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("building %d not found in %+v", id, buildings.Items)
	return domaingame.BuildingItem{}
}

func containsBuilding(buildings domaingame.Buildings, id int) bool {
	for _, item := range buildings.Items {
		if item.ID == id {
			return true
		}
	}
	return false
}
