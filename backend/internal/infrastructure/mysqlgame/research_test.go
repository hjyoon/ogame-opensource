package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestResearchRepositoryReadsLegacyResearch(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(123456), 7, 99, 1, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3, 12800, 19, 4, 163, 10000.0, 10000.0, 10000.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{2})},
		{rows: fakeRowsFromValues(buildingLevelRow(map[int]int{domaingame.BuildingResearchLab: 3}))},
		{rows: fakeRowsFromValues(allResearchLevelRow(map[int]int{domaingame.ResearchEnergy: 1, domaingame.ResearchIntergalacticNetwork: 1}))},
		{rows: fakeRowsFromValues([]any{99, 3}, []any{100, 7})},
		{rows: fakeRowsFromValues([]any{2.0})},
		{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix()})},
	}}
	repository := NewResearchRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })

	research, err := repository.GetResearch(context.Background(), appgame.ResearchQuery{PlayerID: 42})
	if err != nil {
		t.Fatal(err)
	}

	if research.Commander != "legor" || research.CurrentPlanet.ID != 99 || !research.HasLab {
		t.Fatalf("unexpected research summary: %+v", research)
	}
	if !containsResearch(research, domaingame.ResearchComputer) || !containsResearch(research, domaingame.ResearchCombustionDrive) {
		t.Fatalf("expected unlocked research rows: %+v", research.Items)
	}
	if containsResearch(research, domaingame.ResearchShield) {
		t.Fatalf("expected locked shielding technology to be hidden: %+v", research.Items)
	}
	computer := researchByID(t, research, domaingame.ResearchComputer)
	if computer.DurationSeconds != 59 {
		t.Fatalf("expected speed, technocrat, and lab-network adjusted duration, got %+v", computer)
	}
	if !strings.Contains(queryer.calls[5].sql, "`106`, `108`") || !strings.Contains(queryer.calls[8].sql, "tec_until") {
		t.Fatalf("expected legacy research and premium columns, got %+v", queryer.calls)
	}
}

func TestNewResearchRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewResearchRepository(nil, "ogame_")

	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
	if repository.now == nil {
		t.Fatal("expected default clock")
	}

	withDefaultClock := NewResearchRepositoryWithQueryer(nil, "ogame_", nil)
	if withDefaultClock.now == nil {
		t.Fatal("expected nil clock to default")
	}
}

func TestResearchRepositoryReturnsErrors(t *testing.T) {
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
			name:    "building levels",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(researchOverviewResults(), fakeQueryResult{err: errors.New("building query failed")})},
			want:    "building query failed",
		},
		{
			name:    "research query",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(append(researchOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(nil))}), fakeQueryResult{err: errors.New("research query failed")})},
			want:    "research query failed",
		},
		{
			name:    "missing research",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(append(researchOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(nil))}), fakeQueryResult{rows: fakeRowsFromValues()})},
			want:    "research levels not found",
		},
		{
			name:    "research scan",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(append(researchOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(nil))}), fakeQueryResult{rows: fakeRowsFromValues([]any{"bad"})})},
			want:    "unexpected scan destination count",
		},
		{
			name:    "research rows",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(append(researchOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(nil))}), fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("research rows failed"), allResearchLevelRow(nil))})},
			want:    "research rows failed",
		},
		{
			name:    "labs query",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(researchReadPrefixResults(), fakeQueryResult{err: errors.New("labs query failed")})},
			want:    "labs query failed",
		},
		{
			name:    "labs scan",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(researchReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{"bad", 1})})},
			want:    "expected int",
		},
		{
			name:    "labs rows",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(researchReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("labs rows failed"), []any{100, 7})})},
			want:    "labs rows failed",
		},
		{
			name:    "speed",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(researchReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues()}, fakeQueryResult{err: errors.New("speed query failed")})},
			want:    "speed query failed",
		},
		{
			name:    "technocrat query",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(researchReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues()}, fakeQueryResult{rows: fakeRowsFromValues([]any{1.0})}, fakeQueryResult{err: errors.New("premium query failed")})},
			want:    "premium query failed",
		},
		{
			name:    "missing technocrat",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(researchReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues()}, fakeQueryResult{rows: fakeRowsFromValues([]any{1.0})}, fakeQueryResult{rows: fakeRowsFromValues()})},
			want:    "research premium state not found",
		},
		{
			name:    "technocrat scan",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(researchReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues()}, fakeQueryResult{rows: fakeRowsFromValues([]any{1.0})}, fakeQueryResult{rows: fakeRowsFromValues([]any{"bad"})})},
			want:    "expected int64",
		},
		{
			name:    "technocrat rows",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(researchReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues()}, fakeQueryResult{rows: fakeRowsFromValues([]any{1.0})}, fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("premium rows failed"), []any{int64(0)})})},
			want:    "premium rows failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewResearchRepositoryWithQueryer(tt.queryer, tt.prefix, func() time.Time { return time.Unix(1, 0) })
			_, err := repository.GetResearch(context.Background(), appgame.ResearchQuery{PlayerID: 42})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func researchOverviewResults() []fakeQueryResult {
	return []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{1})},
	}
}

func researchReadPrefixResults() []fakeQueryResult {
	return append(researchOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(map[int]int{domaingame.BuildingResearchLab: 1}))},
		fakeQueryResult{rows: fakeRowsFromValues(allResearchLevelRow(nil))},
	)
}

func allResearchLevelRow(values map[int]int) []any {
	row := make([]any, 0, len(domaingame.ResearchIDs()))
	for _, id := range domaingame.ResearchIDs() {
		row = append(row, values[id])
	}
	return row
}

func researchByID(t *testing.T, research domaingame.Research, id int) domaingame.BuildingItem {
	t.Helper()
	for _, item := range research.Items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("research %d not found in %+v", id, research.Items)
	return domaingame.BuildingItem{}
}

func containsResearch(research domaingame.Research, id int) bool {
	for _, item := range research.Items {
		if item.ID == id {
			return true
		}
	}
	return false
}
