package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestTechnologyRepositoryReadsLegacyTechnology(t *testing.T) {
	queryer := &fakeQueryer{results: technologyReadResults(map[int]int{
		domaingame.BuildingDeuteriumSynth: 5,
	}, map[int]int{
		domaingame.ResearchEnergy: 3,
	})}
	repository := NewTechnologyRepositoryWithQueryer(queryer, "ogame_")

	technology, err := repository.GetTechnology(context.Background(), appgame.TechnologyQuery{
		PlayerID:     42,
		TechnologyID: domaingame.FleetCruiser,
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
}

func TestNewTechnologyRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewTechnologyRepository(nil, "ogame_")

	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
}

func TestTechnologyRepositoryReturnsErrors(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewTechnologyRepositoryWithQueryer(tt.queryer, tt.prefix)
			_, err := repository.GetTechnology(context.Background(), appgame.TechnologyQuery{PlayerID: 42})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func technologyReadResults(buildings map[int]int, research map[int]int) []fakeQueryResult {
	return append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(buildings))},
		fakeQueryResult{rows: fakeRowsFromValues(allResearchLevelRow(research))},
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
