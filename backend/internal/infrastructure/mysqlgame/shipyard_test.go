package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestShipyardRepositoryReadsLegacyFleet(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardReadPrefixResults(),
		fakeQueryResult{rows: fakeRowsFromValues(fleetCountRow(map[int]int{domaingame.FleetSmallCargo: 4}))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{2.0, 999})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{domaingame.BuildingMetalMine})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0)})},
		fakeQueryResult{rows: fakeRowsFromValues()},
	)}
	repository := NewShipyardRepositoryWithQueryer(queryer, "ogame_")

	shipyard, err := repository.GetShipyard(context.Background(), appgame.ShipyardQuery{PlayerID: 42})
	if err != nil {
		t.Fatal(err)
	}

	if shipyard.Commander != "legor" || !shipyard.HasShipyard || shipyard.Busy {
		t.Fatalf("unexpected shipyard summary: %+v", shipyard)
	}
	if shipyard.CommanderActive || len(shipyard.Queue) != 0 {
		t.Fatalf("unexpected shipyard commander/queue state: %+v", shipyard)
	}
	item := findShipyardItem(t, shipyard, domaingame.FleetSmallCargo)
	if item.Count != 4 || item.DurationSeconds != 960 || item.MaxBuild != 5 {
		t.Fatalf("unexpected small cargo item: %+v", item)
	}
	if !strings.Contains(queryer.calls[6].sql, "`202`, `203`, `204`") {
		t.Fatalf("expected fleet numeric columns, got %+v", queryer.calls[6])
	}
}

func TestNewShipyardRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewShipyardRepository(nil, "ogame_")

	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
}

func TestShipyardRepositoryPropagatesDueQueueFinishErrors(t *testing.T) {
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("shipyard queue finish failed")}}}}
	repository := NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)

	_, err := repository.GetShipyard(context.Background(), appgame.ShipyardQuery{PlayerID: 42})

	if err == nil || !strings.Contains(err.Error(), "shipyard queue finish failed") {
		t.Fatalf("expected due queue finish error, got %v", err)
	}
}

func TestShipyardRepositoryLoadHelpersHandleFallbacks(t *testing.T) {
	repository := NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_")

	speed, orderCap, err := repository.loadShipyardUniverseConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if speed != 1 || orderCap != 1000 {
		t.Fatalf("expected default universe config, got speed=%v orderCap=%d", speed, orderCap)
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0.0, 0})}}}, "ogame_")
	speed, orderCap, err = repository.loadShipyardUniverseConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if speed != 1 || orderCap != 1000 {
		t.Fatalf("expected non-positive universe config to default, got speed=%v orderCap=%d", speed, orderCap)
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{domaingame.BuildingShipyard})}}}, "ogame_")
	busy, err := repository.loadShipyardBusy(context.Background(), "ogame_buildqueue", 99)
	if err != nil {
		t.Fatal(err)
	}
	if !busy {
		t.Fatal("expected active shipyard buildqueue row to mark shipyard busy")
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_")
	busy, err = repository.loadShipyardBusy(context.Background(), "ogame_buildqueue", 99)
	if err != nil {
		t.Fatal(err)
	}
	if busy {
		t.Fatal("expected empty buildqueue to leave shipyard available")
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{domaingame.BuildingMetalMine})}}}, "ogame_")
	busy, err = repository.loadShipyardBusy(context.Background(), "ogame_buildqueue", 99)
	if err != nil {
		t.Fatal(err)
	}
	if busy {
		t.Fatal("expected unrelated buildqueue row not to mark shipyard busy")
	}
}

func TestShipyardRepositoryLoadQueueEntriesMapsKnownUnits(t *testing.T) {
	repository := NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(
		buildingQueueTaskValues(buildingQueueTask{
			TaskID:  11,
			OwnerID: 42,
			Type:    queueTypeShipyard,
			SubID:   99,
			ObjID:   domaingame.FleetSmallCargo,
			Level:   3,
			Start:   100,
			End:     220,
		}),
		buildingQueueTaskValues(buildingQueueTask{
			TaskID:  12,
			OwnerID: 42,
			Type:    queueTypeShipyard,
			SubID:   99,
			ObjID:   999999,
			Level:   1,
			Start:   100,
			End:     200,
		}),
		buildingQueueTaskValues(buildingQueueTask{
			TaskID:  13,
			OwnerID: 42,
			Type:    queueTypeShipyard,
			SubID:   99,
			ObjID:   domaingame.FleetLightFighter,
			Level:   1,
			Start:   10,
			End:     90,
		}),
	)}}}, "ogame_")

	entries, err := repository.loadShipyardQueueEntries(context.Background(), "`ogame_queue`", 99, 120)
	if err != nil {
		t.Fatalf("loadShipyardQueueEntries returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected unknown unit to be skipped, got %+v", entries)
	}
	if entries[0].TaskID != 11 || entries[0].Name != "Small Cargo" || entries[0].RemainingSeconds != 100 {
		t.Fatalf("unexpected first queue entry: %+v", entries[0])
	}
	if entries[1].TaskID != 13 || entries[1].RemainingSeconds != 0 {
		t.Fatalf("expected overdue queue entry to clamp remaining time, got %+v", entries[1])
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("queue failed")}}}, "ogame_")
	if _, err := repository.loadShipyardQueueEntries(context.Background(), "`ogame_queue`", 99, 120); err == nil || !strings.Contains(err.Error(), "queue failed") {
		t.Fatalf("expected queue error, got %v", err)
	}
}

func TestShipyardRepositoryLoadHelperErrors(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
		want string
	}{
		{
			name: "fleet query",
			run: func() error {
				repository := NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("fleet query failed")}}}, "ogame_")
				_, err := repository.loadFleetCounts(context.Background(), "ogame_planets", 42, 99)
				return err
			},
			want: "fleet query failed",
		},
		{
			name: "fleet empty rows error",
			run: func() error {
				repository := NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("fleet empty rows failed"))}}}, "ogame_")
				_, err := repository.loadFleetCounts(context.Background(), "ogame_planets", 42, 99)
				return err
			},
			want: "fleet empty rows failed",
		},
		{
			name: "fleet scan",
			run: func() error {
				repository := NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_")
				_, err := repository.loadFleetCounts(context.Background(), "ogame_planets", 42, 99)
				return err
			},
			want: "unexpected scan destination count",
		},
		{
			name: "universe table",
			run: func() error {
				repository := NewShipyardRepositoryWithQueryer(&fakeQueryer{}, "bad-prefix_")
				_, _, err := repository.loadShipyardUniverseConfig(context.Background())
				return err
			},
			want: "invalid database table prefix",
		},
		{
			name: "universe empty rows error",
			run: func() error {
				repository := NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("uni empty rows failed"))}}}, "ogame_")
				_, _, err := repository.loadShipyardUniverseConfig(context.Background())
				return err
			},
			want: "uni empty rows failed",
		},
		{
			name: "universe scan",
			run: func() error {
				repository := NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 999})}}}, "ogame_")
				_, _, err := repository.loadShipyardUniverseConfig(context.Background())
				return err
			},
			want: "expected float64",
		},
		{
			name: "universe post rows",
			run: func() error {
				repository := NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("uni post rows failed"), []any{1.0, 999})}}}, "ogame_")
				_, _, err := repository.loadShipyardUniverseConfig(context.Background())
				return err
			},
			want: "uni post rows failed",
		},
		{
			name: "busy query",
			run: func() error {
				repository := NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("busy query failed")}}}, "ogame_")
				_, err := repository.loadShipyardBusy(context.Background(), "ogame_buildqueue", 99)
				return err
			},
			want: "busy query failed",
		},
		{
			name: "busy empty rows error",
			run: func() error {
				repository := NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("busy empty rows failed"))}}}, "ogame_")
				_, err := repository.loadShipyardBusy(context.Background(), "ogame_buildqueue", 99)
				return err
			},
			want: "busy empty rows failed",
		},
		{
			name: "busy post rows",
			run: func() error {
				repository := NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("busy post rows failed"), []any{domaingame.BuildingNaniteFactory})}}}, "ogame_")
				_, err := repository.loadShipyardBusy(context.Background(), "ogame_buildqueue", 99)
				return err
			},
			want: "busy post rows failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestShipyardRepositoryReturnsErrors(t *testing.T) {
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
			name:    "fleet counts",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(shipyardReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues()})},
			want:    "fleet counts not found",
		},
		{
			name:    "fleet rows",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(shipyardReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("fleet rows failed"), fleetCountRow(nil))})},
			want:    "fleet rows failed",
		},
		{
			name:    "universe config",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(shipyardReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues(fleetCountRow(nil))}, fakeQueryResult{err: errors.New("uni query failed")})},
			want:    "uni query failed",
		},
		{
			name:    "busy query",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(shipyardReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues(fleetCountRow(nil))}, fakeQueryResult{rows: fakeRowsFromValues([]any{1.0, 999})}, fakeQueryResult{err: errors.New("busy query failed")})},
			want:    "busy query failed",
		},
		{
			name:    "busy scan",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(shipyardReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues(fleetCountRow(nil))}, fakeQueryResult{rows: fakeRowsFromValues([]any{1.0, 999})}, fakeQueryResult{rows: fakeRowsFromValues([]any{"bad"})})},
			want:    "expected int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewShipyardRepositoryWithQueryer(tt.queryer, tt.prefix)
			_, err := repository.GetShipyard(context.Background(), appgame.ShipyardQuery{PlayerID: 42})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func shipyardOverviewResults() []fakeQueryResult {
	return []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3, 12800, 19, 1, 163, 10000.0, 10000.0, 10000.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{1})},
	}
}

func shipyardReadPrefixResults() []fakeQueryResult {
	return append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(map[int]int{domaingame.BuildingShipyard: 2}))},
		fakeQueryResult{rows: fakeRowsFromValues(allResearchLevelRow(map[int]int{domaingame.ResearchCombustionDrive: 2}))},
	)
}

func fleetCountRow(values map[int]int) []any {
	row := make([]any, 0, len(domaingame.FleetIDs()))
	for _, id := range domaingame.FleetIDs() {
		row = append(row, values[id])
	}
	return row
}

func findShipyardItem(t *testing.T, shipyard domaingame.Shipyard, id int) domaingame.ShipyardItem {
	t.Helper()
	for _, item := range shipyard.Items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("shipyard item %d not found in %+v", id, shipyard.Items)
	return domaingame.ShipyardItem{}
}
