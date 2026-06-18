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

func TestFleetRepositoryReadsLegacyFleetScreen(t *testing.T) {
	now := time.Unix(1_000, 0)
	queryer := &fakeQueryer{results: append(fleetReadPrefixResults(now),
		fakeQueryResult{rows: fakeRowsFromValues(fleetMissionRow(domaingame.FleetMissionTransport, map[int]int{domaingame.FleetSmallCargo: 2}, 100, 200))},
	)}
	repository := NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })

	fleet, err := repository.GetFleet(context.Background(), appgame.FleetQuery{PlayerID: 42})
	if err != nil {
		t.Fatal(err)
	}

	if fleet.Commander != "legor" || fleet.CurrentPlanet.ID != 99 || len(fleet.Missions) != 1 || len(fleet.Ships) != 2 {
		t.Fatalf("unexpected fleet summary: %+v", fleet)
	}
	if fleet.Slots.Used != 1 || fleet.Slots.BaseMax != 4 || fleet.Slots.Max != 6 || !fleet.Slots.Admiral {
		t.Fatalf("unexpected fleet slots: %+v", fleet.Slots)
	}
	if fleet.Missions[0].MissionName != "Transport" || fleet.Missions[0].TotalShips != 2 || fleet.Missions[0].Origin.Galaxy != 1 || fleet.Missions[0].TargetOwnerName != "target" {
		t.Fatalf("unexpected mission row: %+v", fleet.Missions[0])
	}
	if fleet.Ships[0].ID != domaingame.FleetSmallCargo || fleet.Ships[0].Count != 4 || fleet.Ships[0].Speed != 20000 {
		t.Fatalf("unexpected ship selection rows: %+v", fleet.Ships)
	}
	if !strings.Contains(queryer.calls[5].sql, "`202`, `203`, `204`") || !strings.Contains(queryer.calls[8].sql, "f.`202`, f.`203`, f.`204`") {
		t.Fatalf("expected legacy fleet numeric columns, got %+v", queryer.calls)
	}
}

func TestNewFleetRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewFleetRepository(nil, "ogame_")

	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
}

func TestFleetRepositoryReturnsErrors(t *testing.T) {
	now := time.Unix(1_000, 0)
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
			name:    "research",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{err: errors.New("research query failed")})},
			want:    "research query failed",
		},
		{
			name:    "fleet counts",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues(allResearchLevelRow(nil))}, fakeQueryResult{err: errors.New("fleet counts failed")})},
			want:    "fleet counts failed",
		},
		{
			name:    "premium",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(fleetCountsPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues()})},
			want:    "fleet premium state not found",
		},
		{
			name:    "acs",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(fleetCountsPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0)})}, fakeQueryResult{err: errors.New("acs failed")})},
			want:    "acs failed",
		},
		{
			name:    "missions",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(fleetReadPrefixResults(now), fakeQueryResult{err: errors.New("missions failed")})},
			want:    "missions failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewFleetRepositoryWithQueryer(tt.queryer, tt.prefix, func() time.Time { return now })
			_, err := repository.GetFleet(context.Background(), appgame.FleetQuery{PlayerID: 42})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestFleetRepositoryLoadersHandleOptionalAndScanEdges(t *testing.T) {
	now := time.Unix(1_000, 0)
	repository := NewFleetRepositoryWithQueryer(&fakeQueryer{}, "ogame_", func() time.Time { return now })

	if admiral, err := repository.loadAdmiral(context.Background(), "ogame_users", 42); err == nil || admiral {
		t.Fatalf("expected loadAdmiral query error with no fake result, got admiral=%v err=%v", admiral, err)
	}

	queryer := &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("premium rows failed"), []any{int64(0)})}}}
	repository = NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadAdmiral(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "premium rows failed") {
		t.Fatalf("expected premium rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}
	repository = NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadAdmiral(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "expected int64") {
		t.Fatalf("expected premium scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}
	repository = NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if enabled, err := repository.loadACSEnabled(context.Background(), "ogame_uni"); err != nil || enabled {
		t.Fatalf("empty ACS row set should disable ACS without error, enabled=%v err=%v", enabled, err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("acs rows failed"), []any{0})}}}
	repository = NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadACSEnabled(context.Background(), "ogame_uni"); err == nil || !strings.Contains(err.Error(), "acs rows failed") {
		t.Fatalf("expected ACS rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}
	repository = NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadACSEnabled(context.Background(), "ogame_uni"); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected ACS scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("mission rows failed"))}}}
	repository = NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadActiveMissions(context.Background(), "ogame_queue", "ogame_fleet", "ogame_planets", "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "mission rows failed") {
		t.Fatalf("expected mission rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}
	repository = NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadActiveMissions(context.Background(), "ogame_queue", "ogame_fleet", "ogame_planets", "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
		t.Fatalf("expected mission scan error, got %v", err)
	}
}

func fleetCountsPrefixResults() []fakeQueryResult {
	return append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(allResearchLevelRow(map[int]int{
			domaingame.ResearchComputer:        3,
			domaingame.ResearchExpedition:      4,
			domaingame.ResearchCombustionDrive: 2,
			domaingame.ResearchImpulseDrive:    5,
		}))},
		fakeQueryResult{rows: fakeRowsFromValues(fleetCountRow(map[int]int{
			domaingame.FleetSmallCargo:     4,
			domaingame.FleetSolarSatellite: 2,
		}))},
	)
}

func fleetReadPrefixResults(now time.Time) []fakeQueryResult {
	return append(fleetCountsPrefixResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix()})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{4})},
	)
}

func fleetMissionRow(mission int, ships map[int]int, start int64, end int64) []any {
	row := []any{11, start, end, mission, 99, 100}
	for _, id := range domaingame.FleetIDs() {
		row = append(row, ships[id])
	}
	row = append(row, 1, 2, 3, 1, 2, 4, domaingame.PlanetTypePlanet, "target")
	return row
}
