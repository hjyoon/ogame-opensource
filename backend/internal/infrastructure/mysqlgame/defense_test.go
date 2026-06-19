package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestDefenseRepositoryReadsLegacyDefense(t *testing.T) {
	queryer := &fakeQueryer{results: append(defenseReadPrefixResults(),
		fakeQueryResult{rows: fakeRowsFromValues(defenseCountRow(map[int]int{domaingame.DefenseRocketLauncher: 4}))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{2.0, 999})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{domaingame.BuildingMetalMine})},
	)}
	repository := NewDefenseRepositoryWithQueryer(queryer, "ogame_")

	defense, err := repository.GetDefense(context.Background(), appgame.DefenseQuery{PlayerID: 42})
	if err != nil {
		t.Fatal(err)
	}

	if defense.Commander != "legor" || !defense.HasShipyard || defense.Busy {
		t.Fatalf("unexpected defense summary: %+v", defense)
	}
	item := findDefenseItem(t, defense, domaingame.DefenseRocketLauncher)
	if item.Count != 4 || item.DurationSeconds != 720 || item.MaxBuild != 5 {
		t.Fatalf("unexpected rocket launcher item: %+v", item)
	}
	if !strings.Contains(queryer.calls[6].sql, "`401`, `402`, `403`") {
		t.Fatalf("expected defense numeric columns, got %+v", queryer.calls[6])
	}
}

func TestNewDefenseRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewDefenseRepository(nil, "ogame_")

	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
}

func TestDefenseRepositoryPropagatesDueQueueFinishErrors(t *testing.T) {
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("defense queue finish failed")}}}}
	repository := NewDefenseRepositoryWithRunner(runner, runner, "ogame_", nil)

	_, err := repository.GetDefense(context.Background(), appgame.DefenseQuery{PlayerID: 42})

	if err == nil || !strings.Contains(err.Error(), "defense queue finish failed") {
		t.Fatalf("expected due queue finish error, got %v", err)
	}
}

func TestDefenseRepositoryLoadDefenseCountsErrors(t *testing.T) {
	tests := []struct {
		name string
		rows *fakeRows
		err  error
		want string
	}{
		{name: "query", err: errors.New("defense query failed"), want: "defense query failed"},
		{name: "missing", rows: fakeRowsFromValues(), want: "defense counts not found"},
		{name: "empty rows", rows: fakeRowsFromValuesWithErr(errors.New("defense empty rows failed")), want: "defense empty rows failed"},
		{name: "scan", rows: fakeRowsFromValues([]any{"bad"}), want: "unexpected scan destination count"},
		{name: "post rows", rows: fakeRowsFromValuesWithErr(errors.New("defense rows failed"), defenseCountRow(nil)), want: "defense rows failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewDefenseRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: tt.rows, err: tt.err}}}, "ogame_")
			_, err := repository.loadDefenseCounts(context.Background(), "ogame_planets", 42, 99)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestDefenseRepositoryReturnsErrors(t *testing.T) {
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
		{
			name:    "defense counts",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(defenseReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues()})},
			want:    "defense counts not found",
		},
		{
			name:    "universe config",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(defenseReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues(defenseCountRow(nil))}, fakeQueryResult{err: errors.New("uni query failed")})},
			want:    "uni query failed",
		},
		{
			name:    "busy query",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(defenseReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues(defenseCountRow(nil))}, fakeQueryResult{rows: fakeRowsFromValues([]any{1.0, 999})}, fakeQueryResult{err: errors.New("busy query failed")})},
			want:    "busy query failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewDefenseRepositoryWithQueryer(tt.queryer, tt.prefix)
			_, err := repository.GetDefense(context.Background(), appgame.DefenseQuery{PlayerID: 42})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func defenseReadPrefixResults() []fakeQueryResult {
	return append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(map[int]int{domaingame.BuildingShipyard: 1}))},
		fakeQueryResult{rows: fakeRowsFromValues(allResearchLevelRow(nil))},
	)
}

func defenseCountRow(values map[int]int) []any {
	row := make([]any, 0, len(domaingame.DefenseIDs()))
	for _, id := range domaingame.DefenseIDs() {
		row = append(row, values[id])
	}
	return row
}

func findDefenseItem(t *testing.T, defense domaingame.Defense, id int) domaingame.ShipyardItem {
	t.Helper()
	for _, item := range defense.Items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("defense item %d not found in %+v", id, defense.Items)
	return domaingame.ShipyardItem{}
}
