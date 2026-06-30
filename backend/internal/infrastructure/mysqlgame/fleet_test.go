package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
		fakeQueryResult{rows: fakeRowsFromValues(templateRow(7, " raid wing ", 900, map[int]int{domaingame.FleetSmallCargo: 2, domaingame.FleetSolarSatellite: 3}))},
	)}
	repository := NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })

	fleet, err := repository.GetFleet(context.Background(), appgame.FleetQuery{PlayerID: 42})
	if err != nil {
		t.Fatal(err)
	}

	if fleet.Commander != "legor" || fleet.CurrentPlanet.ID != 99 || len(fleet.Missions) != 1 || len(fleet.Ships) != 2 {
		t.Fatalf("unexpected fleet summary: %+v", fleet)
	}
	if fleet.SpeedFactor != 128 {
		t.Fatalf("expected universe fleet speed factor, got %d", fleet.SpeedFactor)
	}
	if !fleet.CommanderActive || fleet.TemplateLimit != 4 || len(fleet.Templates) != 1 || fleet.Templates[0].Name != "raid wing" {
		t.Fatalf("unexpected fleet template summary: %+v", fleet)
	}
	if fleet.Slots.Used != 1 || fleet.Slots.BaseMax != 4 || fleet.Slots.Max != 6 || !fleet.Slots.Admiral {
		t.Fatalf("unexpected fleet slots: %+v", fleet.Slots)
	}
	if fleet.Missions[0].MissionName != "Transport" || fleet.Missions[0].TotalShips != 2 || fleet.Missions[0].Origin.Galaxy != 1 || fleet.Missions[0].TargetOwnerName != "target" || fleet.Missions[0].LoadedResources[domaingame.ResourceMetal] != 30 {
		t.Fatalf("unexpected mission row: %+v", fleet.Missions[0])
	}
	if fleet.Ships[0].ID != domaingame.FleetSmallCargo || fleet.Ships[0].Count != 4 || fleet.Ships[0].Speed != 20000 {
		t.Fatalf("unexpected ship selection rows: %+v", fleet.Ships)
	}
	if !fleetCallContains(queryer.calls, "`202`, `203`, `204`") || !fleetCallContains(queryer.calls, "f.`202`, f.`203`, f.`204`") || !fleetCallContains(queryer.calls, "ogame_template") {
		t.Fatalf("expected legacy fleet numeric columns, got %+v", queryer.calls)
	}
}

func TestFleetRepositorySkipsTemplatesForNonCommanderFleetScreen(t *testing.T) {
	now := time.Unix(1_000, 0)
	queryer := &fakeQueryer{results: append(fleetCountsPrefixResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0)})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0)})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{1})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{128})},
		fakeQueryResult{rows: fakeRowsFromValues()},
	)}
	repository := NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })

	fleet, err := repository.GetFleet(context.Background(), appgame.FleetQuery{PlayerID: 42})
	if err != nil {
		t.Fatal(err)
	}
	if fleet.CommanderActive || len(fleet.Templates) != 0 {
		t.Fatalf("non commander should not load templates: %+v", fleet)
	}
	if fleetCallContains(queryer.calls, "ogame_template") {
		t.Fatalf("non commander should not query template table: %+v", queryer.calls)
	}
}

func TestFleetRepositoryDrainsDueQueuesBeforeReadingFleetScreen(t *testing.T) {
	now := time.Unix(1_000, 0)
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1})},
	}, append(fleetReadPrefixResults(now),
		fakeQueryResult{rows: fakeRowsFromValues()},
		fakeQueryResult{rows: fakeRowsFromValues()},
	)...)}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	repository.finishDueQueues = true

	fleet, err := repository.GetFleet(context.Background(), appgame.FleetQuery{PlayerID: 42})
	if err != nil {
		t.Fatal(err)
	}
	if fleet.Commander != "legor" || !fleet.CommanderActive || len(fleet.Templates) != 0 {
		t.Fatalf("unexpected fleet after frozen due queue drain: %+v", fleet)
	}
	if len(runner.execCalls) != 0 {
		t.Fatalf("frozen universe should not execute due fleet queue writes: %+v", runner.execCalls)
	}
	if len(runner.calls) == 0 || !strings.Contains(runner.calls[0].sql, "SELECT freeze FROM `ogame_uni`") {
		t.Fatalf("expected frozen check before fleet screen reads, got %+v", runner.calls)
	}

	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("freeze failed")}}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	repository.finishDueQueues = true
	if _, err := repository.GetFleet(context.Background(), appgame.FleetQuery{PlayerID: 42}); err == nil || !strings.Contains(err.Error(), "freeze failed") {
		t.Fatalf("expected frozen check error before fleet read, got %v", err)
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
	if _, ok := repository.execer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL execer, got %T", repository.execer)
	}
	if !repository.finishDueQueues {
		t.Fatal("production fleet repository should drain due fleet queues")
	}
	withDefaultClock := NewFleetRepositoryWithQueryer(nil, "ogame_", nil)
	if withDefaultClock.now == nil {
		t.Fatal("expected nil clock to default")
	}
	withDefaultClock = NewFleetRepositoryWithRunner(nil, nil, "ogame_", nil)
	if withDefaultClock.now == nil {
		t.Fatal("expected runner nil clock to default")
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
			queryer: &fakeQueryer{results: append(fleetCountsPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0)})}, fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0)})}, fakeQueryResult{err: errors.New("acs failed")})},
			want:    "acs failed",
		},
		{
			name:    "fleet speed factor",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(fleetCountsPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0)})}, fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0)})}, fakeQueryResult{rows: fakeRowsFromValues([]any{1})}, fakeQueryResult{err: errors.New("fspeed failed")})},
			want:    "fspeed failed",
		},
		{
			name:    "templates",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(fleetReadPrefixResults(now), fakeQueryResult{rows: fakeRowsFromValues()}, fakeQueryResult{err: errors.New("templates failed")})},
			want:    "templates failed",
		},
		{
			name:    "missions",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(fleetCountsPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix()})}, fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0)})}, fakeQueryResult{rows: fakeRowsFromValues([]any{1})}, fakeQueryResult{rows: fakeRowsFromValues([]any{128})}, fakeQueryResult{err: errors.New("missions failed")})},
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

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}
	repository = NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadCommanderActive(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "expected int64") {
		t.Fatalf("expected commander scan error, got %v", err)
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

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}
	repository = NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if speedFactor, err := repository.loadFleetSpeedFactor(context.Background(), "ogame_uni"); err != nil || speedFactor != 1 {
		t.Fatalf("empty fleet speed factor row set should default to 1, speedFactor=%d err=%v", speedFactor, err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0})}}}
	repository = NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if speedFactor, err := repository.loadFleetSpeedFactor(context.Background(), "ogame_uni"); err != nil || speedFactor != 1 {
		t.Fatalf("non-positive fleet speed factor should default to 1, speedFactor=%d err=%v", speedFactor, err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("fspeed rows failed"), []any{128})}}}
	repository = NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadFleetSpeedFactor(context.Background(), "ogame_uni"); err == nil || !strings.Contains(err.Error(), "fspeed rows failed") {
		t.Fatalf("expected fleet speed rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}
	repository = NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadFleetSpeedFactor(context.Background(), "ogame_uni"); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected fleet speed scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("mission rows failed"))}}}
	repository = NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadActiveMissions(context.Background(), "ogame_queue", "ogame_fleet", "ogame_planets", "ogame_users", "ogame_union", 42); err == nil || !strings.Contains(err.Error(), "mission rows failed") {
		t.Fatalf("expected mission rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}
	repository = NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadActiveMissions(context.Background(), "ogame_queue", "ogame_fleet", "ogame_planets", "ogame_users", "ogame_union", 42); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
		t.Fatalf("expected mission scan error, got %v", err)
	}
}

func TestFleetRepositoryLoadsFleetMissionUnionDetails(t *testing.T) {
	now := time.Unix(1_000, 0)
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"Alpha Wing", "7, bad, 11, 7, -4, 9"})},
		{rows: fakeRowsFromValues([]any{11, "eleven"}, []any{7, "seven"})},
	}}
	repository := NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })

	details, err := repository.loadFleetMissionUnionDetails(context.Background(), "ogame_union", "ogame_users", 44)
	if err != nil {
		t.Fatal(err)
	}
	if details.Name != "Alpha Wing" || len(details.Players) != 2 {
		t.Fatalf("unexpected union details: %+v", details)
	}
	if details.Players[0].ID != 7 || details.Players[0].Name != "seven" || details.Players[1].ID != 11 || details.Players[1].Name != "eleven" {
		t.Fatalf("players should follow parsed legacy order and skip missing IDs: %+v", details.Players)
	}
	if !fleetCallContains(queryer.calls, "player_id IN (?, ?, ?)") {
		t.Fatalf("expected parsed player ids to drive placeholder count, got %+v", queryer.calls)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"Solo", "bad, 0, -1"})}}}
	repository = NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	details, err = repository.loadFleetMissionUnionDetails(context.Background(), "ogame_union", "ogame_users", 44)
	if err != nil || details.Name != "Solo" || len(details.Players) != 0 {
		t.Fatalf("invalid player list should keep union name without players, details=%+v err=%v", details, err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}
	repository = NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	details, err = repository.loadFleetMissionUnionDetails(context.Background(), "ogame_union", "ogame_users", 44)
	if err != nil || details.Name != "" || len(details.Players) != 0 {
		t.Fatalf("missing union row should return empty details, details=%+v err=%v", details, err)
	}
}

func TestFleetRepositoryLoadsActiveMissionsWithACSUnionDetails(t *testing.T) {
	now := time.Unix(1_000, 0)
	unionMission := fleetMissionRow(domaingame.FleetMissionACSAttackHead, map[int]int{domaingame.FleetCruiser: 2}, 100, 200)
	unionMission[6] = 55
	transportMission := fleetMissionRow(domaingame.FleetMissionTransport, map[int]int{domaingame.FleetSmallCargo: 3}, 110, 210)
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(unionMission, transportMission)},
		{rows: fakeRowsFromValues([]any{"Alpha Wing", "42, 77"})},
		{rows: fakeRowsFromValues([]any{77, "ally"}, []any{42, "legor"})},
	}}
	repository := NewFleetRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })

	missions, err := repository.loadActiveMissions(context.Background(), "ogame_queue", "ogame_fleet", "ogame_planets", "ogame_users", "ogame_union", 42)
	if err != nil {
		t.Fatal(err)
	}

	if len(missions) != 2 {
		t.Fatalf("expected two active missions, got %+v", missions)
	}
	if missions[0].UnionID != 55 || missions[0].UnionName != "Alpha Wing" || len(missions[0].UnionPlayers) != 2 || missions[0].UnionPlayers[0].ID != 42 {
		t.Fatalf("expected ACS union details on first mission, got %+v", missions[0])
	}
	if len(missions[0].Ships) != 1 || missions[0].Ships[0].ID != domaingame.FleetCruiser || missions[0].Ships[0].Count != 2 ||
		len(missions[1].Ships) != 1 || missions[1].Ships[0].ID != domaingame.FleetSmallCargo || missions[1].Ships[0].Count != 3 ||
		missions[1].UnionID != 0 || missions[1].LoadedResources[domaingame.ResourceCrystal] != 20 {
		t.Fatalf("unexpected mission ship or union data: %+v", missions)
	}
	if len(queryer.calls) != 3 || !fleetCallContains(queryer.calls, "FROM ogame_union") || !fleetCallContains(queryer.calls, "player_id IN (?, ?)") {
		t.Fatalf("expected mission query plus one union/player lookup, got %+v", queryer.calls)
	}
}

func TestFleetRepositoryFleetMissionUnionDetailsHandleErrors(t *testing.T) {
	now := time.Unix(1_000, 0)
	tests := []struct {
		name    string
		queryer *fakeQueryer
		want    string
	}{
		{
			name:    "union query",
			queryer: &fakeQueryer{results: []fakeQueryResult{{err: errors.New("union query failed")}}},
			want:    "union query failed",
		},
		{
			name:    "union rows",
			queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("union rows failed"))}}},
			want:    "union rows failed",
		},
		{
			name:    "union scan",
			queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"only-name"})}}},
			want:    "unexpected scan destination count",
		},
		{
			name:    "players query",
			queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"Alpha Wing", "7"})}, {err: errors.New("players query failed")}}},
			want:    "players query failed",
		},
		{
			name:    "players scan",
			queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"Alpha Wing", "7"})}, {rows: fakeRowsFromValues([]any{"bad", "seven"})}}},
			want:    "expected int",
		},
		{
			name:    "players rows",
			queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"Alpha Wing", "7"})}, {rows: fakeRowsFromValuesWithErr(errors.New("players rows failed"), []any{7, "seven"})}}},
			want:    "players rows failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewFleetRepositoryWithQueryer(tt.queryer, "ogame_", func() time.Time { return now })
			_, err := repository.loadFleetMissionUnionDetails(context.Background(), "ogame_union", "ogame_users", 44)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestFleetRepositoryMutatesFleetTemplatesWithLegacyOwnershipRules(t *testing.T) {
	now := time.Unix(1_000, 0)
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), 3})},
		{rows: fakeRowsFromValues([]any{1})},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	err := repository.MutateFleetTemplate(context.Background(), appgame.FleetTemplateMutationQuery{
		PlayerID: 42,
		Action:   "save",
		Name:     " raid wing ",
		Ships: map[int]int{
			domaingame.FleetSmallCargo:     3,
			domaingame.FleetSolarSatellite: 7,
			domaingame.FleetRecycler:       -1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 1 || !strings.Contains(runner.execCalls[0].sql, "INSERT INTO `ogame_template`") {
		t.Fatalf("expected template insert, got %+v", runner.execCalls)
	}
	if runner.execCalls[0].args[0] != 42 || runner.execCalls[0].args[1] != "raid wing" || runner.execCalls[0].args[2] != int64(1_000) {
		t.Fatalf("unexpected insert args: %+v", runner.execCalls[0].args)
	}
	if runner.execCalls[0].args[13] != 0 {
		t.Fatalf("solar satellite template value must be zero, got args: %+v", runner.execCalls[0].args)
	}

	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), 3})},
	}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if err := repository.MutateFleetTemplate(context.Background(), appgame.FleetTemplateMutationQuery{PlayerID: 42, Action: "save", TemplateID: 7, Name: "update"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 1 || !strings.Contains(runner.execCalls[0].sql, "WHERE id = ? AND owner_id = ?") {
		t.Fatalf("expected owner-scoped update, got %+v", runner.execCalls)
	}

	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), 3})},
	}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if err := repository.MutateFleetTemplate(context.Background(), appgame.FleetTemplateMutationQuery{PlayerID: 42, Action: "delete", TemplateID: 7}); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 1 || !strings.Contains(runner.execCalls[0].sql, "DELETE FROM `ogame_template` WHERE id = ? AND owner_id = ?") {
		t.Fatalf("expected owner-scoped delete, got %+v", runner.execCalls)
	}
}

func TestFleetRepositorySkipsFleetTemplateWritesWithoutCommanderOrCapacity(t *testing.T) {
	now := time.Unix(1_000, 0)
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{int64(0), 3})},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if err := repository.MutateFleetTemplate(context.Background(), appgame.FleetTemplateMutationQuery{PlayerID: 42, Action: "save"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 0 {
		t.Fatalf("non commander must not write templates: %+v", runner.execCalls)
	}

	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), 1})},
		{rows: fakeRowsFromValues([]any{2})},
	}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if err := repository.MutateFleetTemplate(context.Background(), appgame.FleetTemplateMutationQuery{PlayerID: 42, Action: "save"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 0 {
		t.Fatalf("template capacity overflow must not write: %+v", runner.execCalls)
	}
}

func TestFleetRepositoryTemplateHelpersHandleEdges(t *testing.T) {
	now := time.Unix(1_000, 0)
	repository := NewFleetRepositoryWithQueryer(&fakeQueryer{}, "ogame_", func() time.Time { return now })

	if active, err := repository.loadCommanderActive(context.Background(), "ogame_users", 42); err == nil || active {
		t.Fatalf("expected commander query error, got active=%v err=%v", active, err)
	}

	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_", func() time.Time { return now })
	if active, err := repository.loadCommanderActive(context.Background(), "ogame_users", 42); err == nil || active {
		t.Fatalf("expected missing commander state error, got active=%v err=%v", active, err)
	}

	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("commander rows failed"), []any{int64(0)})}}}, "ogame_", func() time.Time { return now })
	if _, err := repository.loadCommanderActive(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "commander rows failed") {
		t.Fatalf("expected commander rows error, got %v", err)
	}

	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_", func() time.Time { return now })
	if active, maxTemplates, err := repository.loadFleetTemplateAccess(context.Background(), "ogame_users", 42); err == nil || active || maxTemplates != 0 {
		t.Fatalf("expected missing template access error, got active=%v max=%d err=%v", active, maxTemplates, err)
	}

	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 1})}}}, "ogame_", func() time.Time { return now })
	if _, _, err := repository.loadFleetTemplateAccess(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "expected int64") {
		t.Fatalf("expected template access scan error, got %v", err)
	}

	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("access rows failed"), []any{now.Add(time.Hour).Unix(), 1})}}}, "ogame_", func() time.Time { return now })
	if _, _, err := repository.loadFleetTemplateAccess(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "access rows failed") {
		t.Fatalf("expected template access rows error, got %v", err)
	}

	if templates, err := repository.loadFleetTemplates(context.Background(), "ogame_template", 42, 0); err != nil || len(templates) != 0 {
		t.Fatalf("zero template limit should skip query, templates=%+v err=%v", templates, err)
	}

	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("template rows failed"))}}}, "ogame_", func() time.Time { return now })
	if _, err := repository.loadFleetTemplates(context.Background(), "ogame_template", 42, 1); err == nil || !strings.Contains(err.Error(), "template rows failed") {
		t.Fatalf("expected template rows error, got %v", err)
	}

	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}, "ogame_", func() time.Time { return now })
	if _, err := repository.loadFleetTemplates(context.Background(), "ogame_template", 42, 1); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
		t.Fatalf("expected template scan error, got %v", err)
	}

	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_", func() time.Time { return now })
	if count, err := repository.countFleetTemplates(context.Background(), "ogame_template", 42); err != nil || count != 0 {
		t.Fatalf("empty template count should be zero, count=%d err=%v", count, err)
	}

	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("count rows failed"), []any{1})}}}, "ogame_", func() time.Time { return now })
	if _, err := repository.countFleetTemplates(context.Background(), "ogame_template", 42); err == nil || !strings.Contains(err.Error(), "count rows failed") {
		t.Fatalf("expected count rows error, got %v", err)
	}

	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_", func() time.Time { return now })
	if _, err := repository.countFleetTemplates(context.Background(), "ogame_template", 42); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected count scan error, got %v", err)
	}
}

func TestFleetRepositoryTemplateMutationErrors(t *testing.T) {
	now := time.Unix(1_000, 0)
	repository := NewFleetRepositoryWithQueryer(&fakeQueryer{}, "ogame_", func() time.Time { return now })
	if err := repository.MutateFleetTemplate(context.Background(), appgame.FleetTemplateMutationQuery{PlayerID: 42, Action: "save"}); err == nil || !strings.Contains(err.Error(), "writer unavailable") {
		t.Fatalf("expected missing writer error, got %v", err)
	}

	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), 1})}}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "bad-prefix_", func() time.Time { return now })
	if err := repository.MutateFleetTemplate(context.Background(), appgame.FleetTemplateMutationQuery{PlayerID: 42, Action: "save"}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected unsafe prefix error, got %v", err)
	}

	runner = &fakeFleetRunner{}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if err := repository.MutateFleetTemplate(context.Background(), appgame.FleetTemplateMutationQuery{PlayerID: 42, Action: "save"}); err == nil || !strings.Contains(err.Error(), "unexpected query") {
		t.Fatalf("expected template access query error, got %v", err)
	}

	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), 1})}}}, execErr: errors.New("delete failed")}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if err := repository.MutateFleetTemplate(context.Background(), appgame.FleetTemplateMutationQuery{PlayerID: 42, Action: "delete", TemplateID: 7}); err == nil || !strings.Contains(err.Error(), "delete failed") {
		t.Fatalf("expected delete exec error, got %v", err)
	}

	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), 1})}}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if err := repository.MutateFleetTemplate(context.Background(), appgame.FleetTemplateMutationQuery{PlayerID: 42, Action: "delete"}); err != nil || len(runner.execCalls) != 0 {
		t.Fatalf("zero template delete should be a no-op, calls=%+v err=%v", runner.execCalls, err)
	}

	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), 1})}, {err: errors.New("count failed")}}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if err := repository.MutateFleetTemplate(context.Background(), appgame.FleetTemplateMutationQuery{PlayerID: 42, Action: "save"}); err == nil || !strings.Contains(err.Error(), "count failed") {
		t.Fatalf("expected count error, got %v", err)
	}

	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), 1})}, {rows: fakeRowsFromValues([]any{0})}}}, execErr: errors.New("insert failed")}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if err := repository.MutateFleetTemplate(context.Background(), appgame.FleetTemplateMutationQuery{PlayerID: 42, Action: "save"}); err == nil || !strings.Contains(err.Error(), "insert failed") {
		t.Fatalf("expected insert error, got %v", err)
	}

	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), 1})}}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if err := repository.MutateFleetTemplate(context.Background(), appgame.FleetTemplateMutationQuery{PlayerID: 42, Action: "unknown"}); err != nil || len(runner.execCalls) != 0 {
		t.Fatalf("unknown action should be a no-op, calls=%+v err=%v", runner.execCalls, err)
	}
}

func TestFleetRepositoryLaunchesLegacyDispatchWithFleetQueue(t *testing.T) {
	now := time.Unix(1_000, 0)
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{0})},
		{rows: fakeRowsFromValues([]any{100, 43, domaingame.PlanetTypePlanet})},
		{rows: fakeRowsFromValues(fleetLaunchUserStateRow(42, 10_000, 0, 0, 0, 0, now.Unix()))},
		{rows: fakeRowsFromValues(fleetLaunchUserStateRow(43, 10_000, 0, 0, 0, 0, now.Unix()))},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	issue, err := repository.LaunchFleetDispatch(context.Background(), appgame.FleetLaunchQuery{
		PlayerID: 42,
		PlanetID: 99,
		Origin: domaingame.PlanetOverview{
			Type:        domaingame.PlanetTypePlanet,
			Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
			Resources:   domaingame.Resources{Metal: 1000, Crystal: 900, Deuterium: 800},
		},
		Draft: domaingame.FleetDispatchDraft{
			Ships: []domaingame.FleetShipCount{{
				ID:    domaingame.FleetSmallCargo,
				Count: 1,
			}},
			Target:          domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4},
			TargetType:      domaingame.GamePlanetTypePlanet,
			Mission:         domaingame.FleetMissionTransport,
			DurationSeconds: 42,
			FuelConsumption: 7,
			Ready:           true,
			Resources: []domaingame.FleetResourceLoad{
				{ID: domaingame.ResourceMetal, Loaded: 123},
				{ID: domaingame.ResourceCrystal, Loaded: 45},
				{ID: domaingame.ResourceDeuterium, Loaded: 6},
			},
		},
		UnionID:     5,
		HoldSeconds: 60,
	})
	if err != nil || issue != nil {
		t.Fatalf("expected launch success, issue=%+v err=%v", issue, err)
	}
	if len(runner.execCalls) != 5 {
		t.Fatalf("expected fleet log cleanup, origin debit, fleet insert, log insert, queue insert; got %+v", runner.execCalls)
	}
	if !strings.Contains(runner.execCalls[0].sql, "DELETE FROM `ogame_fleetlogs`") {
		t.Fatalf("expected fleet log retention cleanup, got %+v", runner.execCalls[0])
	}
	debit := runner.execCalls[1]
	if !strings.Contains(debit.sql, "UPDATE `ogame_planets`") || !strings.Contains(debit.sql, "`202` = `202` - ?") || !strings.Contains(debit.sql, "`702` >= ?") {
		t.Fatalf("expected atomic origin debit SQL, got %s", debit.sql)
	}
	if debit.args[0] != 123 || debit.args[1] != 45 || debit.args[2] != 13 || debit.args[3] != 1 {
		t.Fatalf("unexpected origin debit args: %+v", debit.args)
	}
	insertFleet := runner.execCalls[2]
	if !strings.Contains(insertFleet.sql, "INSERT INTO `ogame_fleet`") || !strings.Contains(insertFleet.sql, "`202`") {
		t.Fatalf("expected legacy fleet insert, got %s", insertFleet.sql)
	}
	if insertFleet.args[0] != 42 || insertFleet.args[1] != 5 || insertFleet.args[2] != 123 || insertFleet.args[4] != 6 || insertFleet.args[5] != 7 || insertFleet.args[6] != domaingame.FleetMissionTransport || insertFleet.args[7] != 99 || insertFleet.args[8] != 100 || insertFleet.args[9] != 42 || insertFleet.args[10] != 60 {
		t.Fatalf("unexpected fleet insert args: %+v", insertFleet.args)
	}
	if insertFleet.args[11] != 1 {
		t.Fatalf("expected small cargo count in first fleet column, got %+v", insertFleet.args)
	}
	insertLog := runner.execCalls[3]
	if !strings.Contains(insertLog.sql, "INSERT INTO `ogame_fleetlogs`") || !strings.Contains(insertLog.sql, "`p700`") || !strings.Contains(insertLog.sql, "`202`") {
		t.Fatalf("expected legacy fleet log insert, got %s", insertLog.sql)
	}
	insertQueue := runner.execCalls[4]
	if !strings.Contains(insertQueue.sql, "INSERT INTO `ogame_queue`") {
		t.Fatalf("expected queue insert, got %s", insertQueue.sql)
	}
	if insertQueue.args[0] != 42 || insertQueue.args[1] != queueTypeFleet || insertQueue.args[2] != 1 || insertQueue.args[5] != int64(1_000) || insertQueue.args[6] != int64(1_042) || insertQueue.args[7] != fleetQueuePriority(domaingame.FleetMissionTransport) {
		t.Fatalf("unexpected queue insert args: %+v", insertQueue.args)
	}
}

func TestFleetRepositoryLaunchCreatesLegacySpecialTargets(t *testing.T) {
	now := time.Unix(1_000, 0)
	baseQuery := func(mission int, target domaingame.Coordinates) appgame.FleetLaunchQuery {
		return appgame.FleetLaunchQuery{
			PlayerID: 42,
			PlanetID: 99,
			Origin: domaingame.PlanetOverview{
				Type:        domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
				Resources:   domaingame.Resources{Metal: 1000, Crystal: 900, Deuterium: 800},
			},
			Draft: domaingame.FleetDispatchDraft{
				Ships: []domaingame.FleetShipCount{{
					ID:    domaingame.FleetColonyShip,
					Count: 1,
				}},
				Target:          target,
				TargetType:      domaingame.GamePlanetTypePlanet,
				Mission:         mission,
				DurationSeconds: 42,
				Ready:           true,
			},
		}
	}

	tests := []struct {
		name          string
		query         appgame.FleetLaunchQuery
		results       []fakeQueryResult
		execResults   []sql.Result
		wantExecs     int
		wantTargetID  int
		wantType      int
		wantInsert    bool
		wantTargetSQL string
	}{
		{
			name: "colonize phantom",
			query: baseQuery(
				domaingame.FleetMissionColonize,
				domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4},
			),
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues()},
			},
			execResults:   []sql.Result{fakeFleetSQLResult(77), fakeFleetSQLResult(1), fakeFleetSQLResult(1), fakeFleetSQLResult(78)},
			wantExecs:     6,
			wantTargetID:  77,
			wantType:      legacyPlanetTypeColony,
			wantInsert:    true,
			wantTargetSQL: "INSERT INTO `ogame_planets`",
		},
		{
			name: "expedition far space",
			query: baseQuery(
				domaingame.FleetMissionExpedition,
				domaingame.Coordinates{Galaxy: 2, System: 3, Position: domaingame.GalaxyFarSpace},
			),
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues()},
			},
			execResults:   []sql.Result{fakeFleetSQLResult(88), fakeFleetSQLResult(1), fakeFleetSQLResult(1), fakeFleetSQLResult(89)},
			wantExecs:     6,
			wantTargetID:  88,
			wantType:      legacyPlanetTypeFarSpace,
			wantInsert:    true,
			wantTargetSQL: "INSERT INTO `ogame_planets`",
		},
		{
			name: "existing far space",
			query: baseQuery(
				domaingame.FleetMissionExpedition,
				domaingame.Coordinates{Galaxy: 2, System: 3, Position: domaingame.GalaxyFarSpace},
			),
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues([]any{88, userSpace, legacyPlanetTypeFarSpace})},
			},
			execResults:  []sql.Result{fakeFleetSQLResult(1), fakeFleetSQLResult(1), fakeFleetSQLResult(89)},
			wantExecs:    5,
			wantTargetID: 88,
			wantType:     legacyPlanetTypeFarSpace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: tt.results}, execResults: tt.execResults}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

			issue, err := repository.LaunchFleetDispatch(context.Background(), tt.query)
			if err != nil || issue != nil {
				t.Fatalf("expected special launch success, issue=%+v err=%v", issue, err)
			}
			if len(runner.execCalls) != tt.wantExecs {
				t.Fatalf("unexpected exec calls: %+v", runner.execCalls)
			}
			fleetInsertIndex := 2
			if tt.wantInsert {
				targetInsert := runner.execCalls[0]
				if !strings.Contains(targetInsert.sql, tt.wantTargetSQL) {
					t.Fatalf("expected target insert SQL, got %s", targetInsert.sql)
				}
				if targetInsert.args[1] != tt.wantType || targetInsert.args[5] != userSpace || targetInsert.args[10] != int64(1_000) || targetInsert.args[14] != 0 {
					t.Fatalf("unexpected target insert args: %+v", targetInsert.args)
				}
				fleetInsertIndex = 3
			}
			insertFleet := runner.execCalls[fleetInsertIndex]
			if !strings.Contains(insertFleet.sql, "INSERT INTO `ogame_fleet`") || insertFleet.args[8] != tt.wantTargetID {
				t.Fatalf("expected fleet to use special target id %d, got %+v", tt.wantTargetID, insertFleet)
			}
			insertLog := runner.execCalls[fleetInsertIndex+1]
			if !strings.Contains(insertLog.sql, "INSERT INTO `ogame_fleetlogs`") || insertLog.args[16] != tt.wantType {
				t.Fatalf("expected log to use special target type %d, got %+v", tt.wantType, insertLog)
			}
		})
	}
}

func TestFleetRepositoryLaunchRejectsInvalidLegacySpecialTargets(t *testing.T) {
	now := time.Unix(1_000, 0)
	tests := []struct {
		name    string
		query   appgame.FleetLaunchQuery
		results []fakeQueryResult
	}{
		{
			name: "colonize wrong target type",
			query: appgame.FleetLaunchQuery{
				PlayerID: 42,
				PlanetID: 99,
				Draft: domaingame.FleetDispatchDraft{
					Ships: []domaingame.FleetShipCount{{
						ID:    domaingame.FleetColonyShip,
						Count: 1,
					}},
					Ready:      true,
					Mission:    domaingame.FleetMissionColonize,
					Target:     domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4},
					TargetType: domaingame.GamePlanetTypeDebris,
				},
			},
			results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0})}},
		},
		{
			name: "colonize occupied",
			query: appgame.FleetLaunchQuery{
				PlayerID: 42,
				PlanetID: 99,
				Draft: domaingame.FleetDispatchDraft{
					Ships: []domaingame.FleetShipCount{{
						ID:    domaingame.FleetColonyShip,
						Count: 1,
					}},
					Ready:      true,
					Mission:    domaingame.FleetMissionColonize,
					Target:     domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4},
					TargetType: domaingame.GamePlanetTypePlanet,
				},
			},
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues([]any{123})},
			},
		},
		{
			name: "expedition wrong position",
			query: appgame.FleetLaunchQuery{
				PlayerID: 42,
				PlanetID: 99,
				Draft: domaingame.FleetDispatchDraft{
					Ships: []domaingame.FleetShipCount{{
						ID:    domaingame.FleetSmallCargo,
						Count: 1,
					}},
					Ready:      true,
					Mission:    domaingame.FleetMissionExpedition,
					Target:     domaingame.Coordinates{Galaxy: 2, System: 3, Position: 15},
					TargetType: domaingame.GamePlanetTypePlanet,
				},
			},
			results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0})}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: tt.results}}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

			issue, err := repository.LaunchFleetDispatch(context.Background(), tt.query)
			if err != nil {
				t.Fatal(err)
			}
			if issue == nil || issue.Code != domaingame.FleetIssueInvalidTarget {
				t.Fatalf("expected invalid target issue, got %+v", issue)
			}
			if len(runner.execCalls) != 0 {
				t.Fatalf("invalid special target must not write, got %+v", runner.execCalls)
			}
		})
	}
}

func TestFleetRepositoryLaunchRejectsMissionShipRequirementsBeforeWrites(t *testing.T) {
	now := time.Unix(1_000, 0)
	baseQuery := func(mission int, ships []domaingame.FleetShipCount) appgame.FleetLaunchQuery {
		return appgame.FleetLaunchQuery{
			PlayerID: 42,
			PlanetID: 99,
			Draft: domaingame.FleetDispatchDraft{
				Ships:      ships,
				Ready:      true,
				Mission:    mission,
				Target:     domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4},
				TargetType: domaingame.GamePlanetTypePlanet,
			},
		}
	}

	tests := []struct {
		name  string
		query appgame.FleetLaunchQuery
		code  string
	}{
		{
			name:  "spy without probe",
			query: baseQuery(domaingame.FleetMissionSpy, []domaingame.FleetShipCount{{ID: domaingame.FleetSmallCargo, Count: 1}}),
			code:  domaingame.FleetIssueInvalidTarget,
		},
		{
			name:  "colonize without colony ship",
			query: baseQuery(domaingame.FleetMissionColonize, []domaingame.FleetShipCount{{ID: domaingame.FleetSmallCargo, Count: 1}}),
			code:  domaingame.FleetIssueInvalidTarget,
		},
		{
			name:  "recycle without recycler",
			query: baseQuery(domaingame.FleetMissionRecycle, []domaingame.FleetShipCount{{ID: domaingame.FleetSmallCargo, Count: 1}}),
			code:  domaingame.FleetIssueInvalidTarget,
		},
		{
			name:  "destroy without deathstar",
			query: baseQuery(domaingame.FleetMissionDestroy, []domaingame.FleetShipCount{{ID: domaingame.FleetSmallCargo, Count: 1}}),
			code:  domaingame.FleetIssueInvalidTarget,
		},
		{
			name:  "expedition without crewed ship",
			query: baseQuery(domaingame.FleetMissionExpedition, []domaingame.FleetShipCount{{ID: domaingame.FleetEspionageProbe, Count: 1}}),
			code:  domaingame.FleetIssueExpRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0})}}}}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

			issue, err := repository.LaunchFleetDispatch(context.Background(), tt.query)
			if err != nil {
				t.Fatal(err)
			}
			if issue == nil || issue.Code != tt.code {
				t.Fatalf("expected issue %s, got %+v", tt.code, issue)
			}
			if len(runner.execCalls) != 0 || len(runner.calls) != 1 {
				t.Fatalf("ship requirement failure should stop after freeze check, calls=%+v execs=%+v", runner.calls, runner.execCalls)
			}
		})
	}
}

func TestFleetRepositoryLaunchRejectsLegacyMissionTargetGuards(t *testing.T) {
	now := time.Unix(1_000, 0)
	baseQuery := func(mission int, targetType int, ships []domaingame.FleetShipCount) appgame.FleetLaunchQuery {
		return appgame.FleetLaunchQuery{
			PlayerID: 42,
			PlanetID: 99,
			Draft: domaingame.FleetDispatchDraft{
				Ships:      ships,
				Ready:      true,
				Mission:    mission,
				Target:     domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4},
				TargetType: targetType,
			},
		}
	}
	smallCargo := []domaingame.FleetShipCount{{ID: domaingame.FleetSmallCargo, Count: 1}}

	tests := []struct {
		name   string
		query  appgame.FleetLaunchQuery
		target []any
	}{
		{
			name:   "attack own planet",
			query:  baseQuery(domaingame.FleetMissionAttack, domaingame.GamePlanetTypePlanet, smallCargo),
			target: []any{100, 42, domaingame.PlanetTypePlanet},
		},
		{
			name:   "spy own planet",
			query:  baseQuery(domaingame.FleetMissionSpy, domaingame.GamePlanetTypePlanet, []domaingame.FleetShipCount{{ID: domaingame.FleetEspionageProbe, Count: 1}}),
			target: []any{100, 42, domaingame.PlanetTypePlanet},
		},
		{
			name:   "deploy foreign planet",
			query:  baseQuery(domaingame.FleetMissionDeploy, domaingame.GamePlanetTypePlanet, smallCargo),
			target: []any{100, 43, domaingame.PlanetTypePlanet},
		},
		{
			name:   "transport debris",
			query:  baseQuery(domaingame.FleetMissionTransport, domaingame.GamePlanetTypeDebris, smallCargo),
			target: []any{100, userSpace, domaingame.PlanetTypeDebris},
		},
		{
			name:   "recycle planet",
			query:  baseQuery(domaingame.FleetMissionRecycle, domaingame.GamePlanetTypePlanet, []domaingame.FleetShipCount{{ID: domaingame.FleetRecycler, Count: 1}}),
			target: []any{100, 43, domaingame.PlanetTypePlanet},
		},
		{
			name:   "destroy planet",
			query:  baseQuery(domaingame.FleetMissionDestroy, domaingame.GamePlanetTypePlanet, []domaingame.FleetShipCount{{ID: domaingame.FleetDeathstar, Count: 1}}),
			target: []any{100, 43, domaingame.PlanetTypePlanet},
		},
		{
			name:   "destroy own moon",
			query:  baseQuery(domaingame.FleetMissionDestroy, domaingame.GamePlanetTypeMoon, []domaingame.FleetShipCount{{ID: domaingame.FleetDeathstar, Count: 1}}),
			target: []any{100, 42, domaingame.PlanetTypeMoon},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues(tt.target)},
			}}}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

			issue, err := repository.LaunchFleetDispatch(context.Background(), tt.query)
			if err != nil {
				t.Fatal(err)
			}
			if issue == nil || issue.Code != domaingame.FleetIssueInvalidTarget {
				t.Fatalf("expected invalid target issue, got %+v", issue)
			}
			if len(runner.execCalls) != 0 {
				t.Fatalf("target guard failure must not write, got %+v", runner.execCalls)
			}
		})
	}
}

func TestFleetRepositoryLaunchAllowsLegacyMissionTargetGuards(t *testing.T) {
	now := time.Unix(1_000, 0)
	baseQuery := func(mission int, targetType int, ships []domaingame.FleetShipCount) appgame.FleetLaunchQuery {
		return appgame.FleetLaunchQuery{
			PlayerID: 42,
			PlanetID: 99,
			Origin: domaingame.PlanetOverview{
				Type:        domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
				Resources:   domaingame.Resources{Metal: 1000, Crystal: 1000, Deuterium: 1000},
			},
			Draft: domaingame.FleetDispatchDraft{
				Ships:           ships,
				Ready:           true,
				Mission:         mission,
				Target:          domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4},
				TargetType:      targetType,
				DurationSeconds: 42,
			},
		}
	}

	tests := []struct {
		name   string
		query  appgame.FleetLaunchQuery
		target []any
		users  []fakeQueryResult
	}{
		{
			name:   "deploy own planet",
			query:  baseQuery(domaingame.FleetMissionDeploy, domaingame.GamePlanetTypePlanet, []domaingame.FleetShipCount{{ID: domaingame.FleetSmallCargo, Count: 1}}),
			target: []any{100, 42, domaingame.PlanetTypePlanet},
			users: []fakeQueryResult{
				{rows: fakeRowsFromValues(fleetLaunchUserStateRow(42, 10_000, 0, 0, 0, 0, now.Unix()))},
				{rows: fakeRowsFromValues(fleetLaunchUserStateRow(42, 10_000, 0, 0, 0, 0, now.Unix()))},
			},
		},
		{
			name:   "spy foreign planet",
			query:  baseQuery(domaingame.FleetMissionSpy, domaingame.GamePlanetTypePlanet, []domaingame.FleetShipCount{{ID: domaingame.FleetEspionageProbe, Count: 1}}),
			target: []any{100, 43, domaingame.PlanetTypePlanet},
			users: []fakeQueryResult{
				{rows: fakeRowsFromValues(fleetLaunchUserStateRow(42, 10_000, 0, 0, 0, 0, now.Unix()))},
				{rows: fakeRowsFromValues(fleetLaunchUserStateRow(43, 10_000, 0, 0, 0, 0, now.Unix()))},
			},
		},
		{
			name:   "recycle debris",
			query:  baseQuery(domaingame.FleetMissionRecycle, domaingame.GamePlanetTypeDebris, []domaingame.FleetShipCount{{ID: domaingame.FleetRecycler, Count: 1}}),
			target: []any{100, userSpace, domaingame.PlanetTypeDebris},
		},
		{
			name:   "destroy foreign moon",
			query:  baseQuery(domaingame.FleetMissionDestroy, domaingame.GamePlanetTypeMoon, []domaingame.FleetShipCount{{ID: domaingame.FleetDeathstar, Count: 1}}),
			target: []any{100, 43, domaingame.PlanetTypeMoon},
			users: []fakeQueryResult{
				{rows: fakeRowsFromValues(fleetLaunchUserStateRow(42, 10_000, 0, 0, 0, 0, now.Unix()))},
				{rows: fakeRowsFromValues(fleetLaunchUserStateRow(43, 10_000, 0, 0, 0, 0, now.Unix()))},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues(tt.target)},
			}
			results = append(results, tt.users...)
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: results}}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

			issue, err := repository.LaunchFleetDispatch(context.Background(), tt.query)
			if err != nil || issue != nil {
				t.Fatalf("expected launch success, issue=%+v err=%v", issue, err)
			}
			if len(runner.execCalls) != 5 {
				t.Fatalf("expected normal launch writes, got %+v", runner.execCalls)
			}
		})
	}
}

func TestFleetRepositoryLaunchRejectsLegacyTargetProtectionBeforeWrites(t *testing.T) {
	now := time.Unix(1_000, 0)
	baseQuery := func(mission int, ships []domaingame.FleetShipCount) appgame.FleetLaunchQuery {
		return appgame.FleetLaunchQuery{
			PlayerID: 42,
			PlanetID: 99,
			Draft: domaingame.FleetDispatchDraft{
				Ships:      ships,
				Ready:      true,
				Mission:    mission,
				Target:     domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4},
				TargetType: domaingame.GamePlanetTypePlanet,
			},
		}
	}

	tests := []struct {
		name   string
		query  appgame.FleetLaunchQuery
		origin []any
		target []any
		code   string
	}{
		{
			name:   "origin vacation blocks fleet dispatch",
			query:  baseQuery(domaingame.FleetMissionAttack, []domaingame.FleetShipCount{{ID: domaingame.FleetSmallCargo, Count: 1}}),
			origin: fleetLaunchUserStateRow(42, 10_000, 0, 1, 0, 0, now.Unix()),
			target: fleetLaunchUserStateRow(43, 10_000, 0, 0, 0, 0, now.Unix()),
			code:   domaingame.FleetIssueVacationSelf,
		},
		{
			name:   "target vacation blocks hostile dispatch",
			query:  baseQuery(domaingame.FleetMissionAttack, []domaingame.FleetShipCount{{ID: domaingame.FleetSmallCargo, Count: 1}}),
			origin: fleetLaunchUserStateRow(42, 10_000, 0, 0, 0, 0, now.Unix()),
			target: fleetLaunchUserStateRow(43, 10_000, 0, 1, 0, 0, now.Unix()),
			code:   domaingame.FleetIssueVacationOther,
		},
		{
			name:   "operator target blocks spy dispatch",
			query:  baseQuery(domaingame.FleetMissionSpy, []domaingame.FleetShipCount{{ID: domaingame.FleetEspionageProbe, Count: 1}}),
			origin: fleetLaunchUserStateRow(42, 10_000, 0, 0, 0, 0, now.Unix()),
			target: fleetLaunchUserStateRow(43, 10_000, domaingame.AdminLevelOperator, 0, 0, 0, now.Unix()),
			code:   domaingame.FleetIssueTargetAdmin,
		},
		{
			name:   "newbie target blocks attack dispatch",
			query:  baseQuery(domaingame.FleetMissionAttack, []domaingame.FleetShipCount{{ID: domaingame.FleetSmallCargo, Count: 1}}),
			origin: fleetLaunchUserStateRow(42, 100_000, 0, 0, 0, 0, now.Unix()),
			target: fleetLaunchUserStateRow(43, 1_000, 0, 0, 0, 0, now.Unix()),
			code:   domaingame.FleetIssueTargetNoob,
		},
		{
			name:   "strong target blocks attack dispatch",
			query:  baseQuery(domaingame.FleetMissionAttack, []domaingame.FleetShipCount{{ID: domaingame.FleetSmallCargo, Count: 1}}),
			origin: fleetLaunchUserStateRow(42, 1_000, 0, 0, 0, 0, now.Unix()),
			target: fleetLaunchUserStateRow(43, 100_000, 0, 0, 0, 0, now.Unix()),
			code:   domaingame.FleetIssueTargetNoob,
		},
		{
			name:   "origin attack ban blocks hostile dispatch",
			query:  baseQuery(domaingame.FleetMissionAttack, []domaingame.FleetShipCount{{ID: domaingame.FleetSmallCargo, Count: 1}}),
			origin: fleetLaunchUserStateRow(42, 10_000, 0, 0, 0, 1, now.Unix()),
			target: fleetLaunchUserStateRow(43, 10_000, 0, 0, 0, 0, now.Unix()),
			code:   domaingame.FleetIssueAttackBan,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues([]any{100, 43, domaingame.PlanetTypePlanet})},
				{rows: fakeRowsFromValues(tt.origin)},
				{rows: fakeRowsFromValues(tt.target)},
			}}}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

			issue, err := repository.LaunchFleetDispatch(context.Background(), tt.query)
			if err != nil {
				t.Fatal(err)
			}
			if issue == nil || issue.Code != tt.code {
				t.Fatalf("expected issue %s, got %+v", tt.code, issue)
			}
			if len(runner.execCalls) != 0 {
				t.Fatalf("target protection failure must not write, got %+v", runner.execCalls)
			}
		})
	}
}

func TestFleetRepositoryLaunchRejectsACSHoldWithoutBuddyOrAlliance(t *testing.T) {
	now := time.Unix(1_000, 0)
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{0})},
		{rows: fakeRowsFromValues([]any{100, 43, domaingame.PlanetTypePlanet})},
		{rows: fakeRowsFromValues(fleetLaunchUserStateRow(42, 10_000, 0, 0, 0, 0, now.Unix()))},
		{rows: fakeRowsFromValues(fleetLaunchUserStateRow(43, 10_000, 0, 0, 0, 0, now.Unix()))},
		{rows: fakeRowsFromValues([]any{42, 0}, []any{43, 0})},
		{rows: fakeRowsFromValues([]any{0})},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	issue, err := repository.LaunchFleetDispatch(context.Background(), appgame.FleetLaunchQuery{
		PlayerID: 42,
		PlanetID: 99,
		Draft: domaingame.FleetDispatchDraft{
			Ships:           []domaingame.FleetShipCount{{ID: domaingame.FleetLightFighter, Count: 1}},
			Ready:           true,
			Mission:         domaingame.FleetMissionACSHold,
			Target:          domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4},
			TargetType:      domaingame.GamePlanetTypePlanet,
			DurationSeconds: 42,
		},
		HoldSeconds: 3600,
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.FleetIssueHoldAlliance {
		t.Fatalf("expected hold alliance issue, got %+v", issue)
	}
	if len(runner.execCalls) != 0 {
		t.Fatalf("ACS hold relation guard failure must not write, got %+v", runner.execCalls)
	}
}

func TestFleetRepositoryACSHoldRelationAllowsAllianceOrBuddy(t *testing.T) {
	tests := []struct {
		name    string
		results []fakeQueryResult
	}{
		{
			name: "same alliance",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{42, 7}, []any{43, 7})},
			},
		},
		{
			name: "accepted buddy",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{42, 0}, []any{43, 0})},
				{rows: fakeRowsFromValues([]any{1})},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: tt.results}}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", time.Now)

			issue, err := repository.validateFleetLaunchACSHoldRelation(
				context.Background(),
				"`ogame_users`",
				"`ogame_buddy`",
				appgame.FleetLaunchQuery{
					PlayerID: 42,
					Draft:    domaingame.FleetDispatchDraft{Mission: domaingame.FleetMissionACSHold},
				},
				fleetLaunchTarget{OwnerID: 43},
			)
			if err != nil || issue != nil {
				t.Fatalf("expected relation to allow ACS hold, issue=%+v err=%v", issue, err)
			}
		})
	}
}

func TestFleetRepositoryACSHoldRelationHandlesEdges(t *testing.T) {
	repository := NewFleetRepositoryWithRunner(&fakeFleetRunner{}, nil, "ogame_", time.Now)
	issue, err := repository.validateFleetLaunchACSHoldRelation(
		context.Background(),
		"`ogame_users`",
		"`ogame_buddy`",
		appgame.FleetLaunchQuery{PlayerID: 42, Draft: domaingame.FleetDispatchDraft{Mission: domaingame.FleetMissionTransport}},
		fleetLaunchTarget{OwnerID: 43},
	)
	if err != nil || issue != nil {
		t.Fatalf("non ACS hold mission should bypass relation guard, issue=%+v err=%v", issue, err)
	}

	issue, err = repository.validateFleetLaunchACSHoldRelation(
		context.Background(),
		"`ogame_users`",
		"`ogame_buddy`",
		appgame.FleetLaunchQuery{PlayerID: 42, Draft: domaingame.FleetDispatchDraft{Mission: domaingame.FleetMissionACSHold}},
		fleetLaunchTarget{OwnerID: 0},
	)
	if err != nil || issue == nil || issue.Code != domaingame.FleetIssueHoldAlliance {
		t.Fatalf("space target should be rejected as hold alliance, issue=%+v err=%v", issue, err)
	}
	issue, err = repository.validateFleetLaunchACSHoldRelation(
		context.Background(),
		"`ogame_users`",
		"`ogame_buddy`",
		appgame.FleetLaunchQuery{PlayerID: 42, Draft: domaingame.FleetDispatchDraft{Mission: domaingame.FleetMissionACSHold}},
		fleetLaunchTarget{OwnerID: userSpace},
	)
	if err != nil || issue == nil || issue.Code != domaingame.FleetIssueHoldAlliance {
		t.Fatalf("user-space target should be rejected as hold alliance, issue=%+v err=%v", issue, err)
	}

	if related, err := repository.fleetLaunchUsersCanHold(context.Background(), "`ogame_users`", "`ogame_buddy`", 0, 43); err != nil || related {
		t.Fatalf("zero origin should not be related, related=%v err=%v", related, err)
	}
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("relation lookup failed")}}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	issue, err = repository.validateFleetLaunchACSHoldRelation(
		context.Background(),
		"`ogame_users`",
		"`ogame_buddy`",
		appgame.FleetLaunchQuery{PlayerID: 42, Draft: domaingame.FleetDispatchDraft{Mission: domaingame.FleetMissionACSHold}},
		fleetLaunchTarget{OwnerID: 43},
	)
	if err == nil || !strings.Contains(err.Error(), "relation lookup failed") || issue != nil {
		t.Fatalf("expected relation lookup error without issue, issue=%+v err=%v", issue, err)
	}

	tests := []struct {
		name    string
		results []fakeQueryResult
		wantErr string
	}{
		{
			name:    "user query",
			results: []fakeQueryResult{{err: errors.New("user relation failed")}},
			wantErr: "user relation failed",
		},
		{
			name:    "user scan",
			results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}},
			wantErr: "unexpected scan destination count",
		},
		{
			name:    "user rows",
			results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("user rows failed"), []any{42, 0})}},
			wantErr: "user rows failed",
		},
		{
			name: "buddy query",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{42, 0}, []any{43, 0})},
				{err: errors.New("buddy query failed")},
			},
			wantErr: "buddy query failed",
		},
		{
			name: "buddy scan",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{42, 0}, []any{43, 0})},
				{rows: fakeRowsFromValues([]any{"bad"})},
			},
			wantErr: "expected int",
		},
		{
			name: "buddy rows",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{42, 0}, []any{43, 0})},
				{rows: fakeRowsFromValuesWithErr(errors.New("buddy rows failed"), []any{0})},
			},
			wantErr: "buddy rows failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: tt.results}}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", time.Now)

			_, err := repository.fleetLaunchUsersCanHold(context.Background(), "`ogame_users`", "`ogame_buddy`", 42, 43)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q error, got %v", tt.wantErr, err)
			}
		})
	}

	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{42, 0}, []any{43, 0})},
		{rows: fakeRowsFromValues()},
	}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if related, err := repository.fleetLaunchUsersCanHold(context.Background(), "`ogame_users`", "`ogame_buddy`", 42, 43); err != nil || related {
		t.Fatalf("empty buddy rows should not be related, related=%v err=%v", related, err)
	}
	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{42, 0}, []any{43, 0})},
		{rows: fakeRowsError(errors.New("empty buddy rows failed"))},
	}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.fleetLaunchUsersCanHold(context.Background(), "`ogame_users`", "`ogame_buddy`", 42, 43); err == nil || !strings.Contains(err.Error(), "empty buddy rows failed") {
		t.Fatalf("expected empty buddy rows error, got %v", err)
	}
}

func TestFleetLaunchProtectionHelpersCoverLegacyBranches(t *testing.T) {
	now := time.Unix(1_000, 0)
	origin := fleetLaunchUserState{ID: 42, Score: 10_000, LastClick: now.Unix()}
	target := fleetLaunchUserState{ID: 43, Score: 10_000, LastClick: now.Unix()}

	if !fleetLaunchNeedsUserState(domaingame.FleetMissionTransport, 43) {
		t.Fatal("transport to a player target should load user state for vacation guards")
	}
	if fleetLaunchNeedsUserState(domaingame.FleetMissionRecycle, 43) ||
		fleetLaunchNeedsUserState(domaingame.FleetMissionExpedition, userSpace) ||
		fleetLaunchNeedsUserState(999, 43) {
		t.Fatal("recycle, space, and unknown missions should not load player target state")
	}

	tests := []struct {
		name    string
		mission int
		origin  fleetLaunchUserState
		target  fleetLaunchUserState
		code    string
	}{
		{
			name:    "origin vacation",
			mission: domaingame.FleetMissionTransport,
			origin:  fleetLaunchUserState{ID: 42, Vacation: true},
			target:  target,
			code:    domaingame.FleetIssueVacationSelf,
		},
		{
			name:    "target vacation",
			mission: domaingame.FleetMissionTransport,
			origin:  origin,
			target:  fleetLaunchUserState{ID: 43, Vacation: true},
			code:    domaingame.FleetIssueVacationOther,
		},
		{
			name:    "acs hold noob",
			mission: domaingame.FleetMissionACSHold,
			origin:  fleetLaunchUserState{ID: 42, Score: 100_000, LastClick: now.Unix()},
			target:  fleetLaunchUserState{ID: 43, Score: 1_000, LastClick: now.Unix()},
			code:    domaingame.FleetIssueTargetNoob,
		},
		{
			name:    "destroy admin",
			mission: domaingame.FleetMissionDestroy,
			origin:  origin,
			target:  fleetLaunchUserState{ID: 43, Admin: domaingame.AdminLevelAdmin, LastClick: now.Unix()},
			code:    domaingame.FleetIssueTargetAdmin,
		},
		{
			name:    "destroy attack ban",
			mission: domaingame.FleetMissionDestroy,
			origin:  fleetLaunchUserState{ID: 42, Score: 10_000, NoAttack: true, LastClick: now.Unix()},
			target:  target,
			code:    domaingame.FleetIssueAttackBan,
		},
		{
			name:    "normal transport",
			mission: domaingame.FleetMissionTransport,
			origin:  origin,
			target:  target,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := validateFleetLaunchProtection(tt.mission, tt.origin, tt.target, now.Unix())
			if tt.code == "" {
				if issue != nil {
					t.Fatalf("expected no issue, got %+v", issue)
				}
				return
			}
			if issue == nil || issue.Code != tt.code {
				t.Fatalf("expected issue %s, got %+v", tt.code, issue)
			}
		})
	}
}

func TestFleetRepositoryLaunchUserStateLoaderEdges(t *testing.T) {
	now := time.Unix(1_000, 0)
	row := fleetLaunchUserStateRow(42, 12_345, domaingame.AdminLevelOperator, 1, 1, 1, now.Unix())
	repository := NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(row)}}}, "ogame_", func() time.Time { return now })
	state, found, err := repository.loadFleetLaunchUserState(context.Background(), "`ogame_users`", 42)
	if err != nil || !found {
		t.Fatalf("expected user state, found=%v err=%v", found, err)
	}
	if state.ID != 42 || state.Score != 12_345 || state.Admin != domaingame.AdminLevelOperator || !state.Vacation || !state.Banned || !state.NoAttack || state.LastClick != now.Unix() {
		t.Fatalf("unexpected user state: %+v", state)
	}

	tests := []struct {
		name    string
		results []fakeQueryResult
		wantErr string
		found   bool
	}{
		{
			name:    "missing",
			results: []fakeQueryResult{{rows: fakeRowsFromValues()}},
		},
		{
			name:    "query error",
			results: []fakeQueryResult{{err: errors.New("user state query failed")}},
			wantErr: "user state query failed",
		},
		{
			name:    "rows error",
			results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("user state rows failed"))}},
			wantErr: "user state rows failed",
		},
		{
			name:    "scan error",
			results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", int64(0), 0, 0, 0, 0, int64(0), int64(0)})}},
			wantErr: "expected int",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewFleetRepositoryWithQueryer(&fakeQueryer{results: tt.results}, "ogame_", func() time.Time { return now })
			_, found, err := repository.loadFleetLaunchUserState(context.Background(), "`ogame_users`", 42)
			if tt.wantErr == "" {
				if err != nil || found != tt.found {
					t.Fatalf("unexpected found=%v err=%v", found, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestFleetRepositoryValidateFleetLaunchUserStateEdges(t *testing.T) {
	now := time.Unix(1_000, 0)
	query := appgame.FleetLaunchQuery{
		PlayerID: 42,
		Draft: domaingame.FleetDispatchDraft{
			Mission: domaingame.FleetMissionAttack,
		},
	}
	target := fleetLaunchTarget{ID: 100, OwnerID: 43, Type: domaingame.PlanetTypePlanet}
	originRow := fleetLaunchUserStateRow(42, 10_000, 0, 0, 0, 0, now.Unix())

	repository := NewFleetRepositoryWithQueryer(&fakeQueryer{}, "ogame_", func() time.Time { return now })
	issue, err := repository.validateFleetLaunchUserState(context.Background(), "`ogame_users`", appgame.FleetLaunchQuery{
		PlayerID: 42,
		Draft: domaingame.FleetDispatchDraft{
			Mission: domaingame.FleetMissionRecycle,
		},
	}, target, now.Unix())
	if err != nil || issue != nil {
		t.Fatalf("recycle should skip user state, issue=%+v err=%v", issue, err)
	}

	tests := []struct {
		name    string
		results []fakeQueryResult
		wantErr string
	}{
		{
			name:    "missing origin",
			results: []fakeQueryResult{{rows: fakeRowsFromValues()}},
		},
		{
			name: "target query error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(originRow)},
				{err: errors.New("target state failed")},
			},
			wantErr: "target state failed",
		},
		{
			name: "missing target",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(originRow)},
				{rows: fakeRowsFromValues()},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewFleetRepositoryWithQueryer(&fakeQueryer{results: tt.results}, "ogame_", func() time.Time { return now })
			issue, err := repository.validateFleetLaunchUserState(context.Background(), "`ogame_users`", query, target, now.Unix())
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) || issue == nil || issue.Code != domaingame.FleetIssueInvalidTarget {
					t.Fatalf("expected invalid target with %q, issue=%+v err=%v", tt.wantErr, issue, err)
				}
				return
			}
			if err != nil || issue == nil || issue.Code != domaingame.FleetIssueInvalidTarget {
				t.Fatalf("expected invalid target issue without error, issue=%+v err=%v", issue, err)
			}
		})
	}
}

func TestFleetRepositoryLaunchRejectsACSAttackUnionGuards(t *testing.T) {
	now := time.Unix(1_000, 0)
	baseQuery := func(unionID int, duration int) appgame.FleetLaunchQuery {
		return appgame.FleetLaunchQuery{
			PlayerID: 42,
			PlanetID: 99,
			Draft: domaingame.FleetDispatchDraft{
				Ships: []domaingame.FleetShipCount{{
					ID:    domaingame.FleetLightFighter,
					Count: 1,
				}},
				Ready:           true,
				Mission:         domaingame.FleetMissionACSAttack,
				Target:          domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4},
				TargetType:      domaingame.GamePlanetTypePlanet,
				DurationSeconds: duration,
			},
			UnionID: unionID,
		}
	}
	targetResult := func() fakeQueryResult {
		return fakeQueryResult{rows: fakeRowsFromValues([]any{100, 43, domaingame.PlanetTypePlanet})}
	}

	tests := []struct {
		name    string
		query   appgame.FleetLaunchQuery
		results []fakeQueryResult
		code    string
	}{
		{
			name: "missing union id",
			query: func() appgame.FleetLaunchQuery {
				return baseQuery(0, 200)
			}(),
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				targetResult(),
			},
			code: domaingame.FleetIssueInvalidTarget,
		},
		{
			name:  "acs disabled",
			query: baseQuery(55, 200),
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				targetResult(),
				{rows: fakeRowsFromValues([]any{0, "42,99", int64(1_300), 1})},
			},
			code: domaingame.FleetIssueInvalidTarget,
		},
		{
			name:  "missing union",
			query: baseQuery(55, 200),
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				targetResult(),
				{rows: fakeRowsFromValues()},
			},
			code: domaingame.FleetIssueInvalidTarget,
		},
		{
			name:  "not invited",
			query: baseQuery(55, 200),
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				targetResult(),
				{rows: fakeRowsFromValues([]any{2, "99", int64(1_300), 1})},
			},
			code: domaingame.FleetIssueInvalidTarget,
		},
		{
			name:  "too slow",
			query: baseQuery(55, 200),
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				targetResult(),
				{rows: fakeRowsFromValues([]any{2, "42,99", int64(1_100), 1})},
			},
			code: domaingame.FleetIssueInvalidTarget,
		},
		{
			name:  "fleet limit",
			query: baseQuery(55, 200),
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				targetResult(),
				{rows: fakeRowsFromValues([]any{1, "42,99", int64(1_300), 1})},
			},
			code: domaingame.FleetIssueMaxFleet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: tt.results}}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

			issue, err := repository.LaunchFleetDispatch(context.Background(), tt.query)
			if err != nil {
				t.Fatal(err)
			}
			if issue == nil || issue.Code != tt.code {
				t.Fatalf("expected issue %s, got %+v", tt.code, issue)
			}
			if len(runner.execCalls) != 0 {
				t.Fatalf("ACS guard failure must not write, got %+v", runner.execCalls)
			}
		})
	}
}

func TestFleetRepositoryLaunchAllowsACSAttackAndSyncsUnionQueue(t *testing.T) {
	now := time.Unix(1_000, 0)
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{0})},
		{rows: fakeRowsFromValues([]any{100, 43, domaingame.PlanetTypePlanet})},
		{rows: fakeRowsFromValues([]any{2, "42,99", int64(1_300), 1})},
		{rows: fakeRowsFromValues(fleetLaunchUserStateRow(42, 10_000, 0, 0, 0, 0, now.Unix()))},
		{rows: fakeRowsFromValues(fleetLaunchUserStateRow(43, 10_000, 0, 0, 0, 0, now.Unix()))},
		{rows: fakeRowsFromValues([]any{int64(1_300)})},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	issue, err := repository.LaunchFleetDispatch(context.Background(), appgame.FleetLaunchQuery{
		PlayerID: 42,
		PlanetID: 99,
		Origin: domaingame.PlanetOverview{
			Type:        domaingame.PlanetTypePlanet,
			Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
			Resources:   domaingame.Resources{Metal: 1000, Crystal: 1000, Deuterium: 1000},
		},
		Draft: domaingame.FleetDispatchDraft{
			Ships: []domaingame.FleetShipCount{{
				ID:    domaingame.FleetLightFighter,
				Count: 1,
			}},
			Ready:           true,
			Mission:         domaingame.FleetMissionACSAttack,
			Target:          domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4},
			TargetType:      domaingame.GamePlanetTypePlanet,
			DurationSeconds: 200,
		},
		UnionID: 55,
	})
	if err != nil || issue != nil {
		t.Fatalf("expected ACS launch success, issue=%+v err=%v", issue, err)
	}
	if len(runner.execCalls) != 6 {
		t.Fatalf("expected normal launch writes plus ACS queue sync, got %+v", runner.execCalls)
	}
	insertQueue := runner.execCalls[4]
	if !strings.Contains(insertQueue.sql, "INSERT INTO `ogame_queue`") || insertQueue.args[6] != int64(1_200) {
		t.Fatalf("expected initial ACS queue at requested flight end, got %+v", insertQueue)
	}
	syncQueue := runner.execCalls[5]
	if !strings.Contains(syncQueue.sql, "UPDATE `ogame_queue` q JOIN `ogame_fleet` f") || syncQueue.args[0] != int64(1_300) || syncQueue.args[2] != 55 {
		t.Fatalf("expected ACS queue sync to union arrival, got %+v", syncQueue)
	}
}

func TestFleetRepositoryLaunchTargetHelpersHandleEdges(t *testing.T) {
	now := time.Unix(1_000, 0)
	coordinates := domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4}

	tests := []struct {
		name string
		run  func() error
		want string
	}{
		{
			name: "normal target row error",
			run: func() error {
				repository := NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{
					rows: fakeRowsFromValuesWithErr(errors.New("target rows failed")),
				}}}, "ogame_", func() time.Time { return now })
				_, _, err := repository.loadFleetLaunchTarget(context.Background(), "`ogame_planets`", coordinates, domaingame.GamePlanetTypePlanet)
				return err
			},
			want: "target rows failed",
		},
		{
			name: "special target scan",
			run: func() error {
				repository := NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{
					rows: fakeRowsFromValues([]any{"bad", userSpace, legacyPlanetTypeFarSpace}),
				}}}, "ogame_", func() time.Time { return now })
				_, _, err := repository.loadFleetLaunchSpecialTarget(context.Background(), "`ogame_planets`", coordinates, legacyPlanetTypeFarSpace)
				return err
			},
			want: "expected int",
		},
		{
			name: "special target row error",
			run: func() error {
				repository := NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{
					rows: fakeRowsFromValuesWithErr(errors.New("special rows failed"), []any{44, userSpace, legacyPlanetTypeFarSpace}),
				}}}, "ogame_", func() time.Time { return now })
				_, _, err := repository.loadFleetLaunchSpecialTarget(context.Background(), "`ogame_planets`", coordinates, legacyPlanetTypeFarSpace)
				return err
			},
			want: "special rows failed",
		},
		{
			name: "occupied row error",
			run: func() error {
				repository := NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{
					rows: fakeRowsFromValuesWithErr(errors.New("occupied rows failed")),
				}}}, "ogame_", func() time.Time { return now })
				_, err := repository.fleetLaunchColonizeOccupied(context.Background(), "`ogame_planets`", coordinates)
				return err
			},
			want: "occupied rows failed",
		},
		{
			name: "occupied scan",
			run: func() error {
				repository := NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{
					rows: fakeRowsFromValues([]any{"bad"}),
				}}}, "ogame_", func() time.Time { return now })
				_, err := repository.fleetLaunchColonizeOccupied(context.Background(), "`ogame_planets`", coordinates)
				return err
			},
			want: "expected int",
		},
		{
			name: "occupied row error after scan",
			run: func() error {
				repository := NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{
					rows: fakeRowsFromValuesWithErr(errors.New("occupied rows after scan failed"), []any{44}),
				}}}, "ogame_", func() time.Time { return now })
				_, err := repository.fleetLaunchColonizeOccupied(context.Background(), "`ogame_planets`", coordinates)
				return err
			},
			want: "occupied rows after scan failed",
		},
		{
			name: "insert target missing id",
			run: func() error {
				runner := &fakeFleetRunner{execResults: []sql.Result{fakeFleetSQLResult(0)}}
				repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
				_, err := repository.insertFleetLaunchPlanetTarget(context.Background(), "`ogame_planets`", "Planet", legacyPlanetTypeColony, coordinates, now.Unix())
				return err
			},
			want: "fleet launch target id unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, err)
			}
		})
	}
}

func TestFleetRepositoryLaunchACSHelpersHandleEdges(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
		want string
	}{
		{
			name: "acs union query",
			run: func() error {
				repository := NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("acs union failed")}}}, "ogame_", nil)
				_, _, err := repository.loadFleetLaunchACSUnion(context.Background(), "`ogame_uni`", "`ogame_union`", "`ogame_fleet`", "`ogame_queue`", 55)
				return err
			},
			want: "acs union failed",
		},
		{
			name: "acs union rows",
			run: func() error {
				repository := NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("acs union rows failed"))}}}, "ogame_", nil)
				_, _, err := repository.loadFleetLaunchACSUnion(context.Background(), "`ogame_uni`", "`ogame_union`", "`ogame_fleet`", "`ogame_queue`", 55)
				return err
			},
			want: "acs union rows failed",
		},
		{
			name: "acs union scan",
			run: func() error {
				repository := NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", "42", int64(1_300), 1})}}}, "ogame_", nil)
				_, _, err := repository.loadFleetLaunchACSUnion(context.Background(), "`ogame_uni`", "`ogame_union`", "`ogame_fleet`", "`ogame_queue`", 55)
				return err
			},
			want: "expected int",
		},
		{
			name: "acs union trailing rows",
			run: func() error {
				repository := NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("acs union trailing failed"), []any{2, "42", int64(1_300), 1})}}}, "ogame_", nil)
				_, _, err := repository.loadFleetLaunchACSUnion(context.Background(), "`ogame_uni`", "`ogame_union`", "`ogame_fleet`", "`ogame_queue`", 55)
				return err
			},
			want: "acs union trailing failed",
		},
		{
			name: "acs sync query",
			run: func() error {
				runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("acs sync failed")}}}}
				repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
				return repository.syncFleetLaunchACSQueue(context.Background(), "`ogame_queue`", "`ogame_fleet`", 55, 1_200)
			},
			want: "acs sync failed",
		},
		{
			name: "acs sync scan",
			run: func() error {
				runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}}
				repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
				return repository.syncFleetLaunchACSQueue(context.Background(), "`ogame_queue`", "`ogame_fleet`", 55, 1_200)
			},
			want: "expected int64",
		},
		{
			name: "acs sync trailing rows",
			run: func() error {
				runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("acs sync trailing failed"), []any{int64(1_300)})}}}}
				repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
				return repository.syncFleetLaunchACSQueue(context.Background(), "`ogame_queue`", "`ogame_fleet`", 55, 1_200)
			},
			want: "acs sync trailing failed",
		},
		{
			name: "acs sync update",
			run: func() error {
				runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{int64(1_300)})}}}, execErr: errors.New("acs sync update failed")}
				repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
				return repository.syncFleetLaunchACSQueue(context.Background(), "`ogame_queue`", "`ogame_fleet`", 55, 1_200)
			},
			want: "acs sync update failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, err)
			}
		})
	}

	if !fleetLaunchUnionContainsPlayer("7, 42, bad", 42) {
		t.Fatal("expected ACS players parser to accept trimmed numeric ids")
	}
	if fleetLaunchUnionContainsPlayer("7,bad", 42) {
		t.Fatal("malformed or absent ACS player id should not match")
	}
}

func TestFleetRepositoryLaunchReturnsLegacyIssuesWithoutWrites(t *testing.T) {
	now := time.Unix(1_000, 0)
	tests := []struct {
		name    string
		query   appgame.FleetLaunchQuery
		results []fakeQueryResult
		code    string
	}{
		{
			name:  "unready",
			query: appgame.FleetLaunchQuery{PlayerID: 42, PlanetID: 99},
			code:  domaingame.FleetIssueInvalidOrder,
		},
		{
			name: "frozen",
			query: appgame.FleetLaunchQuery{
				PlayerID: 42,
				PlanetID: 99,
				Draft:    domaingame.FleetDispatchDraft{Ready: true},
			},
			results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}},
			code:    domaingame.FleetIssueFrozen,
		},
		{
			name: "missing target",
			query: appgame.FleetLaunchQuery{
				PlayerID: 42,
				PlanetID: 99,
				Draft: domaingame.FleetDispatchDraft{
					Ready:      true,
					Target:     domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4},
					TargetType: domaingame.GamePlanetTypePlanet,
				},
			},
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues()},
			},
			code: domaingame.FleetIssueInvalidTarget,
		},
		{
			name: "lost resources",
			query: appgame.FleetLaunchQuery{
				PlayerID: 42,
				PlanetID: 99,
				Draft: domaingame.FleetDispatchDraft{
					Ready:      true,
					Target:     domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4},
					TargetType: domaingame.GamePlanetTypePlanet,
				},
			},
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues([]any{100, 43, domaingame.PlanetTypePlanet})},
			},
			code: domaingame.FleetIssueLaunchRace,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: tt.results}}
			if tt.name == "lost resources" {
				runner.execResults = []sql.Result{fakeFleetSQLResult(1), fakeFleetSQLResult(0)}
			}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
			issue, err := repository.LaunchFleetDispatch(context.Background(), tt.query)
			if err != nil {
				t.Fatal(err)
			}
			if issue == nil || issue.Code != tt.code {
				t.Fatalf("expected issue %s, got %+v", tt.code, issue)
			}
			if tt.name != "lost resources" && len(runner.execCalls) != 0 {
				t.Fatalf("expected no writes, got %+v", runner.execCalls)
			}
		})
	}
}

func TestFleetRepositoryLaunchPropagatesPipelineErrors(t *testing.T) {
	now := time.Unix(1_000, 0)
	readyQuery := func() appgame.FleetLaunchQuery {
		return appgame.FleetLaunchQuery{
			PlayerID: 42,
			PlanetID: 99,
			Origin: domaingame.PlanetOverview{
				Type:        domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
			},
			Draft: domaingame.FleetDispatchDraft{
				Ships: []domaingame.FleetShipCount{{
					ID:    domaingame.FleetSmallCargo,
					Count: 1,
				}},
				Target:          domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4},
				TargetType:      domaingame.GamePlanetTypeMoon,
				Mission:         domaingame.FleetMissionTransport,
				DurationSeconds: 42,
				Ready:           true,
			},
		}
	}
	baseResults := func() []fakeQueryResult {
		return []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{0})},
			{rows: fakeRowsFromValues([]any{100, 43, domaingame.PlanetTypeMoon})},
			{rows: fakeRowsFromValues(fleetLaunchUserStateRow(42, 10_000, 0, 0, 0, 0, now.Unix()))},
			{rows: fakeRowsFromValues(fleetLaunchUserStateRow(43, 10_000, 0, 0, 0, 0, now.Unix()))},
		}
	}
	tests := []struct {
		name        string
		prefix      string
		query       appgame.FleetLaunchQuery
		results     []fakeQueryResult
		execErrs    []error
		execResults []sql.Result
		want        string
	}{
		{
			name:   "unsafe prefix",
			prefix: "bad-prefix_",
			query:  readyQuery(),
			want:   "invalid database table prefix",
		},
		{
			name:  "freeze query",
			query: readyQuery(),
			results: []fakeQueryResult{
				{err: errors.New("freeze failed")},
			},
			want: "freeze failed",
		},
		{
			name:  "target query",
			query: readyQuery(),
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{err: errors.New("target failed")},
			},
			want: "target failed",
		},
		{
			name:  "target scan",
			query: readyQuery(),
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues([]any{"bad", 43, domaingame.PlanetTypeMoon})},
			},
			want: "expected int",
		},
		{
			name: "colonize occupied query",
			query: func() appgame.FleetLaunchQuery {
				query := readyQuery()
				query.Draft.Mission = domaingame.FleetMissionColonize
				query.Draft.TargetType = domaingame.GamePlanetTypePlanet
				query.Draft.Ships = []domaingame.FleetShipCount{{ID: domaingame.FleetColonyShip, Count: 1}}
				return query
			}(),
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{err: errors.New("occupied failed")},
			},
			want: "occupied failed",
		},
		{
			name: "expedition far space rows",
			query: func() appgame.FleetLaunchQuery {
				query := readyQuery()
				query.Draft.Mission = domaingame.FleetMissionExpedition
				query.Draft.TargetType = domaingame.GamePlanetTypePlanet
				query.Draft.Target.Position = domaingame.GalaxyFarSpace
				return query
			}(),
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValuesWithErr(errors.New("far space rows failed"))},
			},
			want: "far space rows failed",
		},
		{
			name: "special target insert",
			query: func() appgame.FleetLaunchQuery {
				query := readyQuery()
				query.Draft.Mission = domaingame.FleetMissionColonize
				query.Draft.TargetType = domaingame.GamePlanetTypePlanet
				query.Draft.Ships = []domaingame.FleetShipCount{{ID: domaingame.FleetColonyShip, Count: 1}}
				return query
			}(),
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues()},
			},
			execErrs: []error{errors.New("target insert failed")},
			want:     "target insert failed",
		},
		{
			name: "special target id",
			query: func() appgame.FleetLaunchQuery {
				query := readyQuery()
				query.Draft.Mission = domaingame.FleetMissionExpedition
				query.Draft.TargetType = domaingame.GamePlanetTypePlanet
				query.Draft.Target.Position = domaingame.GalaxyFarSpace
				return query
			}(),
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues()},
			},
			execResults: []sql.Result{fakeFleetSQLErrorResult{idErr: errors.New("target id failed")}},
			want:        "target id failed",
		},
		{
			name:     "fleet log cleanup",
			query:    readyQuery(),
			results:  baseResults(),
			execErrs: []error{errors.New("cleanup failed")},
			want:     "cleanup failed",
		},
		{
			name:     "origin debit",
			query:    readyQuery(),
			results:  baseResults(),
			execErrs: []error{nil, errors.New("debit failed")},
			want:     "debit failed",
		},
		{
			name:     "fleet insert",
			query:    readyQuery(),
			results:  baseResults(),
			execErrs: []error{nil, nil, errors.New("fleet insert failed")},
			want:     "fleet insert failed",
		},
		{
			name:        "fleet insert id",
			query:       readyQuery(),
			results:     baseResults(),
			execResults: []sql.Result{fakeFleetSQLResult(1), fakeFleetSQLResult(1), fakeFleetSQLErrorResult{idErr: errors.New("id failed")}},
			want:        "id failed",
		},
		{
			name:        "fleet insert missing id",
			query:       readyQuery(),
			results:     baseResults(),
			execResults: []sql.Result{fakeFleetSQLResult(1), fakeFleetSQLResult(1), fakeFleetSQLResult(0)},
			want:        "fleet launch id unavailable",
		},
		{
			name:     "fleet log insert",
			query:    readyQuery(),
			results:  baseResults(),
			execErrs: []error{nil, nil, nil, errors.New("log failed")},
			want:     "log failed",
		},
		{
			name:     "queue insert",
			query:    readyQuery(),
			results:  baseResults(),
			execErrs: []error{nil, nil, nil, nil, errors.New("queue failed")},
			want:     "queue failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := tt.prefix
			if prefix == "" {
				prefix = "ogame_"
			}
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: tt.results}, execErrs: tt.execErrs, execResults: tt.execResults}
			repository := NewFleetRepositoryWithRunner(runner, runner, prefix, func() time.Time { return now })
			_, err := repository.LaunchFleetDispatch(context.Background(), tt.query)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, err)
			}
		})
	}

	repository := NewFleetRepositoryWithQueryer(&fakeQueryer{}, "ogame_", func() time.Time { return now })
	if _, err := repository.LaunchFleetDispatch(context.Background(), readyQuery()); err == nil || !strings.Contains(err.Error(), "writer unavailable") {
		t.Fatalf("expected missing writer error, got %v", err)
	}
	if fleetLaunchPlanetType(domaingame.GamePlanetTypeMoon) != domaingame.PlanetTypeMoon ||
		fleetLaunchPlanetType(domaingame.GamePlanetTypeDebris) != domaingame.PlanetTypeDebris ||
		fleetLaunchPlanetType(domaingame.GamePlanetTypePlanet) != domaingame.PlanetTypePlanet {
		t.Fatal("unexpected fleet launch planet type mapping")
	}
}

func TestFleetRepositoryRecallsOutboundFleetWithLegacyReturnQueue(t *testing.T) {
	now := time.Unix(1_000, 0)
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{0})},
		{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionTransport, 0, map[int]int{domaingame.FleetSmallCargo: 2, domaingame.FleetSolarSatellite: 1}))},
		{rows: fakeRowsFromValues([]any{55, int64(940), int64(1_240)})},
		{rows: fakeRowsFromValues([]any{44})},
		{rows: fakeRowsFromValues([]any{100})},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	if err := repository.RecallFleet(context.Background(), appgame.FleetRecallQuery{PlayerID: 42, FleetID: 123}); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 4 {
		t.Fatalf("expected insert return fleet, insert queue, delete fleet, delete queue calls, got %+v", runner.execCalls)
	}
	insertFleet := runner.execCalls[0]
	if !strings.Contains(insertFleet.sql, "INSERT INTO `ogame_fleet`") {
		t.Fatalf("expected fleet insert, got %s", insertFleet.sql)
	}
	if insertFleet.args[0] != 44 || insertFleet.args[5] != 25 || insertFleet.args[6] != domaingame.FleetMissionTransport+domaingame.FleetMissionReturnOffset || insertFleet.args[9] != int64(60) {
		t.Fatalf("unexpected return fleet args: %+v", insertFleet.args)
	}
	if insertFleet.args[11] != 2 || insertFleet.args[21] != 1 {
		t.Fatalf("recall must preserve ship counts, got args: %+v", insertFleet.args)
	}
	insertQueue := runner.execCalls[1]
	if !strings.Contains(insertQueue.sql, "INSERT INTO `ogame_queue`") {
		t.Fatalf("expected queue insert, got %s", insertQueue.sql)
	}
	if insertQueue.args[0] != 44 || insertQueue.args[1] != queueTypeFleet || insertQueue.args[2] != 1 || insertQueue.args[5] != int64(1_000) || insertQueue.args[6] != int64(1_060) {
		t.Fatalf("unexpected return queue args: %+v", insertQueue.args)
	}
	if !strings.Contains(runner.execCalls[2].sql, "DELETE FROM `ogame_fleet`") || !strings.Contains(runner.execCalls[3].sql, "DELETE FROM `ogame_queue`") {
		t.Fatalf("expected original fleet and queue deletes, got %+v", runner.execCalls)
	}
}

func TestFleetRepositoryRecallSkipsFrozenReturningAndMissingFleet(t *testing.T) {
	now := time.Unix(1_000, 0)
	tests := []struct {
		name    string
		results []fakeQueryResult
	}{
		{
			name: "frozen universe",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{1})},
			},
		},
		{
			name: "missing fleet",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues()},
			},
		},
		{
			name: "already returning",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionTransport+domaingame.FleetMissionReturnOffset, 0, nil))},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: tt.results}}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
			if err := repository.RecallFleet(context.Background(), appgame.FleetRecallQuery{PlayerID: 42, FleetID: 123}); err != nil {
				t.Fatal(err)
			}
			if len(runner.execCalls) != 0 {
				t.Fatalf("recall no-op should not write, got %+v", runner.execCalls)
			}
		})
	}
}

func TestFleetRepositoryRecallCleansUpEmptyACSUnion(t *testing.T) {
	now := time.Unix(1_000, 0)
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{0})},
		{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionACSAttack, 77, map[int]int{domaingame.FleetLightFighter: 3}))},
		{rows: fakeRowsFromValues([]any{55, int64(900), int64(1_200)})},
		{rows: fakeRowsFromValues([]any{44})},
		{rows: fakeRowsFromValues([]any{100})},
		{rows: fakeRowsFromValues([]any{0})},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	if err := repository.RecallFleet(context.Background(), appgame.FleetRecallQuery{PlayerID: 42, FleetID: 123}); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 5 || !strings.Contains(runner.execCalls[4].sql, "DELETE FROM `ogame_union`") {
		t.Fatalf("expected empty ACS union delete, got %+v", runner.execCalls)
	}
}

func TestFleetRepositoryRecallErrorsAndHelpers(t *testing.T) {
	now := time.Unix(1_000, 0)
	repository := NewFleetRepositoryWithQueryer(&fakeQueryer{}, "ogame_", func() time.Time { return now })
	if err := repository.RecallFleet(context.Background(), appgame.FleetRecallQuery{PlayerID: 42, FleetID: 123}); err == nil || !strings.Contains(err.Error(), "writer unavailable") {
		t.Fatalf("expected missing writer error, got %v", err)
	}
	if err := repository.RecallFleetAnyOwner(context.Background(), 123); err == nil || !strings.Contains(err.Error(), "fleet writer unavailable") {
		t.Fatalf("expected missing writer error for admin recall, got %v", err)
	}

	runner := &fakeFleetRunner{}
	repository = NewFleetRepositoryWithRunner(runner, runner, "bad-prefix_", func() time.Time { return now })
	if err := repository.RecallFleet(context.Background(), appgame.FleetRecallQuery{PlayerID: 42, FleetID: 123}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected unsafe prefix error, got %v", err)
	}
	if err := repository.RecallFleetAnyOwner(context.Background(), 123); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected unsafe prefix error for admin recall, got %v", err)
	}

	if err := repository.RecallFleet(context.Background(), appgame.FleetRecallQuery{PlayerID: 42}); err != nil {
		t.Fatalf("zero fleet id should be no-op, got %v", err)
	}
	if err := repository.RecallFleetAnyOwner(context.Background(), 0); err != nil {
		t.Fatalf("zero admin fleet id should be no-op, got %v", err)
	}
	if !fleetRecallable(domaingame.FleetMissionTransport) || fleetRecallable(domaingame.FleetMissionTransport+domaingame.FleetMissionReturnOffset) {
		t.Fatal("unexpected recallability")
	}
	mission, seconds := recallMissionAndDuration(recallFleetRow{Mission: domaingame.FleetMissionTransport}, recallQueueRow{Start: 1_010}, 1_000)
	if mission != domaingame.FleetMissionTransport+domaingame.FleetMissionReturnOffset || seconds != 0 {
		t.Fatalf("unexpected negative outbound duration clamp: mission=%d seconds=%d", mission, seconds)
	}
	mission, seconds = recallMissionAndDuration(recallFleetRow{Mission: domaingame.FleetMissionOrbitingOffset + domaingame.FleetMissionDeploy, DeployTime: -1}, recallQueueRow{}, 1_000)
	if mission != domaingame.FleetMissionDeploy+domaingame.FleetMissionReturnOffset || seconds != 0 {
		t.Fatalf("unexpected orbiting duration clamp: mission=%d seconds=%d", mission, seconds)
	}
	if fleetQueuePriority(domaingame.FleetMissionMissile) != queuePriorityFleet+1300 || fleetQueuePriority(domaingame.FleetMissionRecycle) != queuePriorityFleet+900 {
		t.Fatal("unexpected fleet queue priority")
	}
	if fleetQueuePriority(domaingame.FleetMissionAttack) != queuePriorityFleet+1000+domaingame.FleetMissionAttack {
		t.Fatal("unexpected attack queue priority")
	}
	values := fleetCountValues(domaingame.FleetIDs(), map[int]int{domaingame.FleetSmallCargo: -3, domaingame.FleetSolarSatellite: 2})
	if values[0] != 0 || values[10] != 2 {
		t.Fatalf("fleet count values should clamp negatives while preserving satellites, got %+v", values)
	}
}

func TestFleetRepositoryFinishDueTransportCreatesReturnFleet(t *testing.T) {
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{0})},
		{rows: fakeRowsFromValues([]any{55, 42, 123, int64(2_000)})},
		{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionTransport, 0, map[int]int{domaingame.FleetSmallCargo: 1}))},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })

	if err := repository.FinishDueFleetQueues(context.Background(), 2_000); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 5 {
		t.Fatalf("expected transport resource, return fleet, return queue, and cleanup writes, got %+v", runner.execCalls)
	}
	if !strings.Contains(runner.execCalls[0].sql, "UPDATE `ogame_planets`") || runner.execCalls[0].args[0] != float64(100) || runner.execCalls[0].args[4] != 100 {
		t.Fatalf("expected resources delivered to target planet, got %+v", runner.execCalls[0])
	}
	insertFleet := runner.execCalls[1]
	if !strings.Contains(insertFleet.sql, "INSERT INTO `ogame_fleet`") || insertFleet.args[2] != float64(0) || insertFleet.args[3] != float64(0) || insertFleet.args[4] != float64(0) || insertFleet.args[6] != domaingame.FleetMissionTransport+domaingame.FleetMissionReturnOffset {
		t.Fatalf("expected zero-cargo transport return fleet, got %+v", insertFleet)
	}
	insertQueue := runner.execCalls[2]
	if !strings.Contains(insertQueue.sql, "INSERT INTO `ogame_queue`") || insertQueue.args[5] != int64(2_000) || insertQueue.args[6] != int64(2_300) {
		t.Fatalf("expected return queue to start at arrival and reuse flight time, got %+v", insertQueue)
	}
}

func TestFleetRepositoryFinishDueDeployKeepsShipsOnTarget(t *testing.T) {
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{0})},
		{rows: fakeRowsFromValues([]any{56, 42, 124, int64(2_100)})},
		{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionDeploy, 0, map[int]int{domaingame.FleetSmallCargo: 2}))},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_100, 0) })

	if err := repository.FinishDueFleetQueues(context.Background(), 2_100); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 4 {
		t.Fatalf("expected deploy resource, ship, and cleanup writes, got %+v", runner.execCalls)
	}
	if runner.execCalls[0].args[2] != float64(325) || runner.execCalls[0].args[4] != 100 {
		t.Fatalf("expected deploy to unload resources plus half fuel on target, got %+v", runner.execCalls[0])
	}
	if !strings.Contains(runner.execCalls[1].sql, "`202` = `202` + ?") || runner.execCalls[1].args[0] != 2 || runner.execCalls[1].args[2] != 100 {
		t.Fatalf("expected deploy ships to remain on target, got %+v", runner.execCalls[1])
	}
}

func TestFleetRepositoryFinishDueReturnRestoresOrigin(t *testing.T) {
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{0})},
		{rows: fakeRowsFromValues([]any{57, 42, 125, int64(2_200)})},
		{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionTransport+domaingame.FleetMissionReturnOffset, 0, map[int]int{domaingame.FleetSmallCargo: 1}))},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_200, 0) })

	if err := repository.FinishDueFleetQueues(context.Background(), 2_200); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 4 {
		t.Fatalf("expected return resource, ship, and cleanup writes, got %+v", runner.execCalls)
	}
	if runner.execCalls[0].args[4] != 99 {
		t.Fatalf("expected return resources to go to origin planet, got %+v", runner.execCalls[0])
	}
	if !strings.Contains(runner.execCalls[1].sql, "`202` = `202` + ?") || runner.execCalls[1].args[2] != 99 {
		t.Fatalf("expected return ships to go to origin planet, got %+v", runner.execCalls[1])
	}
}

func TestFleetRepositoryFinishDueExpeditionCreatesHoldAndReturn(t *testing.T) {
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{0})},
		{rows: fakeRowsFromValues([]any{58, 42, 126, int64(2_300)})},
		{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionExpedition, 0, map[int]int{domaingame.FleetSmallCargo: 1, domaingame.FleetEspionageProbe: 1}))},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_300, 0) })

	if err := repository.FinishDueFleetQueues(context.Background(), 2_300); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 4 {
		t.Fatalf("expected expedition hold fleet, hold queue, and cleanup writes, got %+v", runner.execCalls)
	}
	holdFleet := runner.execCalls[0]
	if !strings.Contains(holdFleet.sql, "INSERT INTO `ogame_fleet`") ||
		holdFleet.args[6] != domaingame.FleetMissionExpedition+domaingame.FleetMissionOrbitingOffset ||
		holdFleet.args[9] != int64(600) ||
		holdFleet.args[10] != int64(300) {
		t.Fatalf("expected expedition arrival to preserve hold and return timings, got %+v", holdFleet)
	}
	holdQueue := runner.execCalls[1]
	if !strings.Contains(holdQueue.sql, "INSERT INTO `ogame_queue`") || holdQueue.args[5] != int64(2_300) || holdQueue.args[6] != int64(2_900) {
		t.Fatalf("expected expedition hold queue to use deploy time, got %+v", holdQueue)
	}

	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{0})},
		{rows: fakeRowsFromValues([]any{59, 42, 127, int64(2_900)})},
		{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionExpedition+domaingame.FleetMissionOrbitingOffset, 0, map[int]int{domaingame.FleetSmallCargo: 1, domaingame.FleetEspionageProbe: 1}))},
		{rows: fakeRowsFromValues(expeditionSettingsTestRow("nothing"))},
		{rows: fakeRowsFromValues(expeditionTargetTestRow())},
	}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_900, 0) })

	if err := repository.FinishDueFleetQueues(context.Background(), 2_900); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 6 {
		t.Fatalf("expected expedition return, visit counter, message, and cleanup writes, got %+v", runner.execCalls)
	}
	returnFleet := runner.execCalls[0]
	if !strings.Contains(returnFleet.sql, "INSERT INTO `ogame_fleet`") ||
		returnFleet.args[6] != domaingame.FleetMissionExpedition+domaingame.FleetMissionReturnOffset ||
		returnFleet.args[9] != int64(600) ||
		returnFleet.args[10] != int64(0) {
		t.Fatalf("expected expedition hold to create return fleet with original flight time, got %+v", returnFleet)
	}
	visitCounter := runner.execCalls[2]
	if !strings.Contains(visitCounter.sql, "UPDATE `ogame_planets`") || visitCounter.args[0] != float64(1) || visitCounter.args[4] != 100 {
		t.Fatalf("expected expedition hold to increment farspace visit counter, got %+v", visitCounter)
	}
	message := runner.execCalls[3]
	if !strings.Contains(message.sql, "INSERT INTO `ogame_messages`") ||
		message.args[0] != 42 ||
		message.args[1] != domaingame.MessageTypeExpedition ||
		message.args[5] != int64(2_900) {
		t.Fatalf("expected expedition result message, got %+v", message)
	}
}

func TestFleetRepositoryFinishDueExpeditionForcedOutcomes(t *testing.T) {
	tests := []struct {
		name          string
		event         string
		wantReturn    bool
		wantUserWrite string
		wantMessage   string
		checkReturn   func(t *testing.T, call fakeFleetExecCall)
	}{
		{
			name:        "nothing",
			event:       "nothing",
			wantReturn:  true,
			wantMessage: "Nothing happened",
		},
		{
			name:          "dark matter",
			event:         "dark_matter",
			wantReturn:    true,
			wantUserWrite: "dmfree = dmfree + ?",
			wantMessage:   "Dark Matter",
		},
		{
			name:        "resources",
			event:       "resources",
			wantReturn:  true,
			wantMessage: "You got",
			checkReturn: func(t *testing.T, call fakeFleetExecCall) {
				t.Helper()
				if call.args[2] != float64(1100) {
					t.Fatalf("expected found metal to be carried home, got %+v", call)
				}
			},
		},
		{
			name:        "fleet",
			event:       "fleet",
			wantReturn:  true,
			wantMessage: "following ships",
			checkReturn: func(t *testing.T, call fakeFleetExecCall) {
				t.Helper()
				firstShipArg := 11
				if call.args[firstShipArg] != 2 {
					t.Fatalf("expected found small cargo to be carried home, got %+v", call)
				}
			},
		},
		{
			name:          "trader",
			event:         "trader",
			wantReturn:    true,
			wantUserWrite: "trader = 1",
			wantMessage:   "representative with goods to trade",
		},
		{
			name:        "delay",
			event:       "delay",
			wantReturn:  true,
			wantMessage: "return later",
			checkReturn: func(t *testing.T, call fakeFleetExecCall) {
				t.Helper()
				if got := call.args[9]; got != int64(1200) {
					t.Fatalf("expected delayed return flight time, got %+v", call)
				}
			},
		},
		{
			name:        "accel",
			event:       "accel",
			wantReturn:  true,
			wantMessage: "return earlier",
			checkReturn: func(t *testing.T, call fakeFleetExecCall) {
				t.Helper()
				if got := call.args[9]; got != int64(300) {
					t.Fatalf("expected accelerated return flight time, got %+v", call)
				}
			},
		},
		{
			name:        "aliens",
			event:       "aliens",
			wantReturn:  true,
			wantMessage: "alien",
		},
		{
			name:        "pirates",
			event:       "pirates",
			wantReturn:  true,
			wantMessage: "Pirate",
		},
		{
			name:        "black hole",
			event:       "black_hole",
			wantReturn:  false,
			wantMessage: "lost forever",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues([]any{59, 42, 127, int64(2_900)})},
				{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionExpedition+domaingame.FleetMissionOrbitingOffset, 0, map[int]int{domaingame.FleetSmallCargo: 1, domaingame.FleetEspionageProbe: 1}))},
				{rows: fakeRowsFromValues(expeditionSettingsTestRow(tt.event))},
				{rows: fakeRowsFromValues(expeditionTargetTestRow())},
			}}}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_900, 0) })

			if err := repository.FinishDueFleetQueues(context.Background(), 2_900); err != nil {
				t.Fatal(err)
			}
			returnInsert := firstExecContaining(runner.execCalls, "INSERT INTO `ogame_fleet`")
			if tt.wantReturn && returnInsert == nil {
				t.Fatalf("expected return fleet insert, got %+v", runner.execCalls)
			}
			if !tt.wantReturn && returnInsert != nil {
				t.Fatalf("expected no return fleet insert, got %+v", runner.execCalls)
			}
			if tt.wantUserWrite != "" && firstExecContaining(runner.execCalls, tt.wantUserWrite) == nil {
				t.Fatalf("expected user write %q, got %+v", tt.wantUserWrite, runner.execCalls)
			}
			message := lastExecContaining(runner.execCalls, "INSERT INTO `ogame_messages`")
			if message == nil || !strings.Contains(fmt.Sprint(message.args[4]), tt.wantMessage) {
				t.Fatalf("expected expedition message containing %q, got %+v", tt.wantMessage, message)
			}
			if tt.checkReturn != nil {
				tt.checkReturn(t, *returnInsert)
			}
		})
	}
}

func TestFleetRepositoryFinishDueExpeditionForcedOutcomeWriteErrors(t *testing.T) {
	tests := []struct {
		name     string
		event    string
		results  []fakeQueryResult
		execErrs []error
		want     string
	}{
		{
			name: "settings query",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues([]any{59, 42, 127, int64(2_900)})},
				{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionExpedition+domaingame.FleetMissionOrbitingOffset, 0, map[int]int{domaingame.FleetSmallCargo: 1, domaingame.FleetEspionageProbe: 1}))},
				{err: errors.New("settings failed")},
			},
			want: "settings failed",
		},
		{
			name: "target query",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues([]any{59, 42, 127, int64(2_900)})},
				{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionExpedition+domaingame.FleetMissionOrbitingOffset, 0, map[int]int{domaingame.FleetSmallCargo: 1, domaingame.FleetEspionageProbe: 1}))},
				{rows: fakeRowsFromValues(expeditionSettingsTestRow("nothing"))},
				{err: errors.New("target failed")},
			},
			want: "target failed",
		},
		{
			name:     "dark matter update",
			event:    "dark_matter",
			execErrs: []error{errors.New("dark matter update failed")},
			want:     "dark matter update failed",
		},
		{
			name:     "dark matter return",
			event:    "dark_matter",
			execErrs: []error{nil, errors.New("dark matter return failed")},
			want:     "dark matter return failed",
		},
		{
			name:     "resources return",
			event:    "resources",
			execErrs: []error{errors.New("resources return failed")},
			want:     "resources return failed",
		},
		{
			name:     "fleet return",
			event:    "fleet",
			execErrs: []error{errors.New("fleet return failed")},
			want:     "fleet return failed",
		},
		{
			name:     "trader update",
			event:    "trader",
			execErrs: []error{errors.New("trader update failed")},
			want:     "trader update failed",
		},
		{
			name:     "trader return",
			event:    "trader",
			execErrs: []error{nil, errors.New("trader return failed")},
			want:     "trader return failed",
		},
		{
			name:     "alien battle message",
			event:    "aliens",
			execErrs: []error{errors.New("alien battle failed")},
			want:     "alien battle failed",
		},
		{
			name:     "alien return",
			event:    "aliens",
			execErrs: []error{nil, errors.New("alien return failed")},
			want:     "alien return failed",
		},
		{
			name:     "pirate battle message",
			event:    "pirates",
			execErrs: []error{errors.New("pirate battle failed")},
			want:     "pirate battle failed",
		},
		{
			name:     "pirate return",
			event:    "pirates",
			execErrs: []error{nil, errors.New("pirate return failed")},
			want:     "pirate return failed",
		},
		{
			name:     "black hole visit counter",
			event:    "black_hole",
			execErrs: []error{errors.New("black hole visit failed")},
			want:     "black hole visit failed",
		},
		{
			name:     "black hole message",
			event:    "black_hole",
			execErrs: []error{nil, errors.New("black hole message failed")},
			want:     "black hole message failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := tt.results
			if results == nil {
				results = []fakeQueryResult{
					{rows: fakeRowsFromValues([]any{0})},
					{rows: fakeRowsFromValues([]any{59, 42, 127, int64(2_900)})},
					{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionExpedition+domaingame.FleetMissionOrbitingOffset, 0, map[int]int{domaingame.FleetSmallCargo: 1, domaingame.FleetEspionageProbe: 1}))},
					{rows: fakeRowsFromValues(expeditionSettingsTestRow(tt.event))},
					{rows: fakeRowsFromValues(expeditionTargetTestRow())},
				}
			}
			runner := &fakeFleetRunner{
				fakeQueryer: fakeQueryer{results: results},
				execErrs:    append([]error(nil), tt.execErrs...),
			}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_900, 0) })
			err := repository.FinishDueFleetQueues(context.Background(), 2_900)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestFleetRepositoryFinishDueFleetQueuesSkipsFrozenAndRejectsInvalidSetup(t *testing.T) {
	repository := NewFleetRepositoryWithQueryer(&fakeQueryer{}, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
	if err := repository.FinishDueFleetQueues(context.Background(), 2_000); err == nil || !strings.Contains(err.Error(), "fleet queue updater unavailable") {
		t.Fatalf("expected missing writer error, got %v", err)
	}

	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1})},
	}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
	if err := repository.FinishDueFleetQueues(context.Background(), 2_000); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 0 || len(runner.calls) != 1 {
		t.Fatalf("frozen universe should only read freeze state, calls=%+v execs=%+v", runner.calls, runner.execCalls)
	}

	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{err: errors.New("freeze failed")},
	}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
	if err := repository.FinishDueFleetQueues(context.Background(), 2_000); err == nil || !strings.Contains(err.Error(), "freeze failed") {
		t.Fatalf("expected freeze query error, got %v", err)
	}

	repository = NewFleetRepositoryWithRunner(runner, runner, "bad-prefix_", func() time.Time { return time.Unix(2_000, 0) })
	if err := repository.FinishDueFleetQueues(context.Background(), 2_000); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected unsafe prefix error, got %v", err)
	}
}

func TestFleetRepositoryQueueHelperEdges(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}
	repository := NewFleetRepositoryWithQueryer(queryer, "ogame_", time.Now)
	tasks, err := repository.loadDueFleetQueueTasks(context.Background(), "`ogame_queue`", 2_000, 0)
	if err != nil || len(tasks) != 0 {
		t.Fatalf("expected empty due queue with default limit, tasks=%+v err=%v", tasks, err)
	}
	if len(queryer.calls) != 1 || queryer.calls[0].args[2] != buildQueueBatch {
		t.Fatalf("expected default batch limit, calls=%+v", queryer.calls)
	}

	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("settings empty rows failed"))}}}, "ogame_", time.Now)
	if _, err := repository.loadExpeditionSettings(context.Background(), "`ogame_exptab`"); err == nil || !strings.Contains(err.Error(), "settings empty rows failed") {
		t.Fatalf("expected settings rows error, got %v", err)
	}

	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("target empty rows failed"))}}}, "ogame_", time.Now)
	if _, err := repository.loadExpeditionTargetState(context.Background(), "`ogame_planets`", 99); err == nil || !strings.Contains(err.Error(), "target empty rows failed") {
		t.Fatalf("expected target rows error, got %v", err)
	}
}

func TestFleetRepositoryFinishDueFleetQueueEdges(t *testing.T) {
	tests := []struct {
		name       string
		results    []fakeQueryResult
		want       string
		wantExecs  int
		wantNoErr  bool
		wantDelete bool
	}{
		{
			name: "due queue query",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{err: errors.New("due failed")},
			},
			want: "due failed",
		},
		{
			name: "due queue scan",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues([]any{55})},
			},
			want: "unexpected scan destination count",
		},
		{
			name: "due queue rows",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValuesWithErr(errors.New("due rows failed"), []any{55, 42, 123, int64(2_000)})},
			},
			want: "due rows failed",
		},
		{
			name: "fleet query",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues([]any{55, 42, 123, int64(2_000)})},
				{err: errors.New("fleet any failed")},
			},
			want: "fleet any failed",
		},
		{
			name: "missing fleet",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues([]any{55, 42, 123, int64(2_000)})},
				{rows: fakeRowsFromValues()},
			},
			wantNoErr:  true,
			wantExecs:  1,
			wantDelete: true,
		},
		{
			name: "unsupported mission",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues([]any{55, 42, 123, int64(2_000)})},
				{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionSpy, 0, nil))},
			},
			wantNoErr: true,
		},
		{
			name: "empty queue",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues()},
			},
			wantNoErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: tt.results}}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
			err := repository.FinishDueFleetQueues(context.Background(), 2_000)
			if tt.wantNoErr {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
			if len(runner.execCalls) != tt.wantExecs {
				t.Fatalf("unexpected exec count: got %+v", runner.execCalls)
			}
			if tt.wantDelete && !strings.Contains(runner.execCalls[0].sql, "DELETE FROM `ogame_queue`") {
				t.Fatalf("expected orphan queue delete, got %+v", runner.execCalls[0])
			}
		})
	}
}

func TestFleetRepositoryFinishDueFleetQueueWriteErrors(t *testing.T) {
	tests := []struct {
		name     string
		mission  int
		execErrs []error
		want     string
	}{
		{
			name:     "transport resource update",
			mission:  domaingame.FleetMissionTransport,
			execErrs: []error{errors.New("resource update failed")},
			want:     "resource update failed",
		},
		{
			name:     "transport return fleet insert",
			mission:  domaingame.FleetMissionTransport,
			execErrs: []error{nil, errors.New("return fleet failed")},
			want:     "return fleet failed",
		},
		{
			name:     "transport return queue insert",
			mission:  domaingame.FleetMissionTransport,
			execErrs: []error{nil, nil, errors.New("return queue failed")},
			want:     "return queue failed",
		},
		{
			name:     "deploy ship update",
			mission:  domaingame.FleetMissionDeploy,
			execErrs: []error{nil, errors.New("ship update failed")},
			want:     "ship update failed",
		},
		{
			name:     "deploy resource update",
			mission:  domaingame.FleetMissionDeploy,
			execErrs: []error{errors.New("deploy resource failed")},
			want:     "deploy resource failed",
		},
		{
			name:     "deploy cleanup fleet delete",
			mission:  domaingame.FleetMissionDeploy,
			execErrs: []error{nil, nil, errors.New("fleet cleanup failed")},
			want:     "fleet cleanup failed",
		},
		{
			name:     "return resource update",
			mission:  domaingame.FleetMissionTransport + domaingame.FleetMissionReturnOffset,
			execErrs: []error{errors.New("return resource failed")},
			want:     "return resource failed",
		},
		{
			name:     "return ship update",
			mission:  domaingame.FleetMissionTransport + domaingame.FleetMissionReturnOffset,
			execErrs: []error{nil, errors.New("return ship failed")},
			want:     "return ship failed",
		},
		{
			name:     "return cleanup queue delete",
			mission:  domaingame.FleetMissionTransport + domaingame.FleetMissionReturnOffset,
			execErrs: []error{nil, nil, nil, errors.New("queue cleanup failed")},
			want:     "queue cleanup failed",
		},
		{
			name:     "expedition hold fleet insert",
			mission:  domaingame.FleetMissionExpedition,
			execErrs: []error{errors.New("expedition hold fleet failed")},
			want:     "expedition hold fleet failed",
		},
		{
			name:     "expedition hold queue insert",
			mission:  domaingame.FleetMissionExpedition,
			execErrs: []error{nil, errors.New("expedition hold queue failed")},
			want:     "expedition hold queue failed",
		},
		{
			name:     "expedition return fleet insert",
			mission:  domaingame.FleetMissionExpedition + domaingame.FleetMissionOrbitingOffset,
			execErrs: []error{errors.New("expedition return fleet failed")},
			want:     "expedition return fleet failed",
		},
		{
			name:     "expedition return queue insert",
			mission:  domaingame.FleetMissionExpedition + domaingame.FleetMissionOrbitingOffset,
			execErrs: []error{nil, errors.New("expedition return queue failed")},
			want:     "expedition return queue failed",
		},
		{
			name:     "expedition visit counter update",
			mission:  domaingame.FleetMissionExpedition + domaingame.FleetMissionOrbitingOffset,
			execErrs: []error{nil, nil, errors.New("expedition visit failed")},
			want:     "expedition visit failed",
		},
		{
			name:     "expedition message insert",
			mission:  domaingame.FleetMissionExpedition + domaingame.FleetMissionOrbitingOffset,
			execErrs: []error{nil, nil, nil, errors.New("expedition message failed")},
			want:     "expedition message failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues([]any{55, 42, 123, int64(2_000)})},
				{rows: fakeRowsFromValues(recallFleetTestRow(tt.mission, 0, map[int]int{domaingame.FleetSmallCargo: 1}))},
			}
			if tt.mission == domaingame.FleetMissionExpedition+domaingame.FleetMissionOrbitingOffset {
				results = append(results,
					fakeQueryResult{rows: fakeRowsFromValues(expeditionSettingsTestRow("nothing"))},
					fakeQueryResult{rows: fakeRowsFromValues(expeditionTargetTestRow())},
				)
			}
			runner := &fakeFleetRunner{
				fakeQueryer: fakeQueryer{results: results},
				execErrs:    append([]error(nil), tt.execErrs...),
			}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
			err := repository.FinishDueFleetQueues(context.Background(), 2_000)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestFleetRepositoryFinishDueFleetQueueHelpers(t *testing.T) {
	if maxFloat(3, 1) != 3 || maxFloat(1, 3) != 3 {
		t.Fatal("maxFloat should return the larger value")
	}
	if maxInt(3, 1) != 3 || maxInt(1, 3) != 3 {
		t.Fatal("maxInt should return the larger value")
	}
	if maxInt64(3, 1) != 3 || maxInt64(1, 3) != 3 {
		t.Fatal("maxInt64 should return the larger value")
	}
	defaultSettings := expeditionSettings{
		ChanceSuccess: 70,
		ChanceAlien:   95,
		ChancePirates: 85,
		ChanceDM:      70,
		ChanceLost:    69,
		ChanceDelay:   63,
		ChanceAccel:   60,
		ChanceRes:     25,
		ChanceFleet:   1,
		DMFactor:      3,
	}
	if expeditionForcedResult(defaultSettings) != expeditionResultNothing {
		t.Fatal("default expedition settings should not force a legacy E2E outcome")
	}
}

func TestFleetRepositoryExpeditionQueueLoadersEdges(t *testing.T) {
	repository := NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("settings query failed")}}}, "ogame_", nil)
	if _, err := repository.loadExpeditionSettings(context.Background(), "ogame_exptab"); err == nil || !strings.Contains(err.Error(), "settings query failed") {
		t.Fatalf("expected settings query error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_", nil)
	if _, err := repository.loadExpeditionSettings(context.Background(), "ogame_exptab"); err == nil || !strings.Contains(err.Error(), "expedition settings not found") {
		t.Fatalf("expected missing settings error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_", nil)
	if _, err := repository.loadExpeditionSettings(context.Background(), "ogame_exptab"); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
		t.Fatalf("expected settings scan error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("settings rows failed"), expeditionSettingsTestRow("nothing"))}}}, "ogame_", nil)
	if _, err := repository.loadExpeditionSettings(context.Background(), "ogame_exptab"); err == nil || !strings.Contains(err.Error(), "settings rows failed") {
		t.Fatalf("expected settings rows error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(expeditionSettingsTestRow("dark_matter"))}}}, "ogame_", nil)
	settings, err := repository.loadExpeditionSettings(context.Background(), "ogame_exptab")
	if err != nil || settings.ChanceDM != 0 || settings.DMFactor != 3 {
		t.Fatalf("unexpected settings=%+v err=%v", settings, err)
	}

	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("target query failed")}}}, "ogame_", nil)
	if _, err := repository.loadExpeditionTargetState(context.Background(), "ogame_planets", 100); err == nil || !strings.Contains(err.Error(), "target query failed") {
		t.Fatalf("expected target query error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_", nil)
	if _, err := repository.loadExpeditionTargetState(context.Background(), "ogame_planets", 100); err == nil || !strings.Contains(err.Error(), "expedition target not found") {
		t.Fatalf("expected missing target error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_", nil)
	if _, err := repository.loadExpeditionTargetState(context.Background(), "ogame_planets", 100); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
		t.Fatalf("expected target scan error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("target rows failed"), expeditionTargetTestRow())}}}, "ogame_", nil)
	if _, err := repository.loadExpeditionTargetState(context.Background(), "ogame_planets", 100); err == nil || !strings.Contains(err.Error(), "target rows failed") {
		t.Fatalf("expected target rows error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(expeditionTargetTestRow())}}}, "ogame_", nil)
	target, err := repository.loadExpeditionTargetState(context.Background(), "ogame_planets", 100)
	if err != nil || target.Galaxy != 1 || target.System != 470 || target.Position != 16 {
		t.Fatalf("unexpected target=%+v err=%v", target, err)
	}
}

func TestFleetRepositoryRecallPropagatesPipelineErrors(t *testing.T) {
	now := time.Unix(1_000, 0)
	baseResults := func(mission int, unionID int) []fakeQueryResult {
		return []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{0})},
			{rows: fakeRowsFromValues(recallFleetTestRow(mission, unionID, map[int]int{domaingame.FleetSmallCargo: 2}))},
			{rows: fakeRowsFromValues([]any{55, int64(940), int64(1_240)})},
			{rows: fakeRowsFromValues([]any{44})},
			{rows: fakeRowsFromValues([]any{100})},
		}
	}
	tests := []struct {
		name        string
		results     []fakeQueryResult
		execErrs    []error
		execResults []sql.Result
		want        string
	}{
		{
			name:    "universe query",
			results: []fakeQueryResult{{err: errors.New("freeze failed")}},
			want:    "freeze failed",
		},
		{
			name: "fleet query",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{err: errors.New("fleet failed")},
			},
			want: "fleet failed",
		},
		{
			name: "queue query",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionTransport, 0, nil))},
				{err: errors.New("queue failed")},
			},
			want: "queue failed",
		},
		{
			name: "origin query",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionTransport, 0, nil))},
				{rows: fakeRowsFromValues([]any{55, int64(940), int64(1_240)})},
				{err: errors.New("origin failed")},
			},
			want: "origin failed",
		},
		{
			name: "target query",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionTransport, 0, nil))},
				{rows: fakeRowsFromValues([]any{55, int64(940), int64(1_240)})},
				{rows: fakeRowsFromValues([]any{44})},
				{err: errors.New("target failed")},
			},
			want: "target failed",
		},
		{
			name:     "return fleet insert",
			results:  baseResults(domaingame.FleetMissionTransport, 0),
			execErrs: []error{errors.New("insert fleet failed")},
			want:     "insert fleet failed",
		},
		{
			name:        "return fleet id",
			results:     baseResults(domaingame.FleetMissionTransport, 0),
			execResults: []sql.Result{fakeFleetSQLErrorResult{idErr: errors.New("last id failed")}},
			want:        "last id failed",
		},
		{
			name:        "return fleet zero id",
			results:     baseResults(domaingame.FleetMissionTransport, 0),
			execResults: []sql.Result{fakeFleetSQLResult(0)},
			want:        "recall return fleet id unavailable",
		},
		{
			name:     "return queue insert",
			results:  baseResults(domaingame.FleetMissionTransport, 0),
			execErrs: []error{nil, errors.New("insert queue failed")},
			want:     "insert queue failed",
		},
		{
			name:     "old fleet delete",
			results:  baseResults(domaingame.FleetMissionTransport, 0),
			execErrs: []error{nil, nil, errors.New("delete fleet failed")},
			want:     "delete fleet failed",
		},
		{
			name:     "old queue delete",
			results:  baseResults(domaingame.FleetMissionTransport, 0),
			execErrs: []error{nil, nil, nil, errors.New("delete queue failed")},
			want:     "delete queue failed",
		},
		{
			name:    "acs union query",
			results: append(baseResults(domaingame.FleetMissionACSAttack, 77), fakeQueryResult{err: errors.New("union count failed")}),
			want:    "union count failed",
		},
		{
			name:     "acs union delete",
			results:  append(baseResults(domaingame.FleetMissionACSAttack, 77), fakeQueryResult{rows: fakeRowsFromValues([]any{0})}),
			execErrs: []error{nil, nil, nil, nil, errors.New("union delete failed")},
			want:     "union delete failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeFleetRunner{
				fakeQueryer: fakeQueryer{results: tt.results},
				execErrs:    append([]error(nil), tt.execErrs...),
				execResults: append([]sql.Result(nil), tt.execResults...),
			}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
			err := repository.RecallFleet(context.Background(), appgame.FleetRecallQuery{PlayerID: 42, FleetID: 123})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestFleetRepositoryRecallNoOpsWhenIntermediateRowsAreMissing(t *testing.T) {
	now := time.Unix(1_000, 0)
	tests := []struct {
		name    string
		results []fakeQueryResult
	}{
		{
			name: "missing queue",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionTransport, 0, nil))},
				{rows: fakeRowsFromValues()},
			},
		},
		{
			name: "missing origin",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionTransport, 0, nil))},
				{rows: fakeRowsFromValues([]any{55, int64(940), int64(1_240)})},
				{rows: fakeRowsFromValues()},
			},
		},
		{
			name: "missing target",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionTransport, 0, nil))},
				{rows: fakeRowsFromValues([]any{55, int64(940), int64(1_240)})},
				{rows: fakeRowsFromValues([]any{44})},
				{rows: fakeRowsFromValues()},
			},
		},
		{
			name: "acs union still has fleets",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionACSAttack, 77, nil))},
				{rows: fakeRowsFromValues([]any{55, int64(940), int64(1_240)})},
				{rows: fakeRowsFromValues([]any{44})},
				{rows: fakeRowsFromValues([]any{100})},
				{rows: fakeRowsFromValues([]any{1})},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: tt.results}}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
			if err := repository.RecallFleet(context.Background(), appgame.FleetRecallQuery{PlayerID: 42, FleetID: 123}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFleetRepositoryRecallLoaderEdges(t *testing.T) {
	now := time.Unix(1_000, 0)
	repository := NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("freeze rows failed"))}}}, "ogame_", func() time.Time { return now })
	if _, err := repository.loadUniverseFrozen(context.Background(), "ogame_uni"); err == nil || !strings.Contains(err.Error(), "freeze rows failed") {
		t.Fatalf("expected freeze rows error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_", func() time.Time { return now })
	if _, err := repository.loadUniverseFrozen(context.Background(), "ogame_uni"); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected freeze scan error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("freeze trailer failed"), []any{0})}}}, "ogame_", func() time.Time { return now })
	if _, err := repository.loadUniverseFrozen(context.Background(), "ogame_uni"); err == nil || !strings.Contains(err.Error(), "freeze trailer failed") {
		t.Fatalf("expected freeze trailing rows error, got %v", err)
	}

	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("fleet rows failed"))}}}, "ogame_", func() time.Time { return now })
	if _, _, err := repository.loadRecallFleet(context.Background(), "ogame_fleet", 42, 123); err == nil || !strings.Contains(err.Error(), "fleet rows failed") {
		t.Fatalf("expected fleet rows error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}, "ogame_", func() time.Time { return now })
	if _, _, err := repository.loadRecallFleet(context.Background(), "ogame_fleet", 42, 123); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
		t.Fatalf("expected fleet scan error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("fleet row trailer failed"), recallFleetTestRow(domaingame.FleetMissionTransport, 0, nil))}}}, "ogame_", func() time.Time { return now })
	if _, _, err := repository.loadRecallFleet(context.Background(), "ogame_fleet", 42, 123); err == nil || !strings.Contains(err.Error(), "fleet row trailer failed") {
		t.Fatalf("expected fleet trailing rows error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_", func() time.Time { return now })
	if _, found, err := repository.loadRecallFleetAnyOwner(context.Background(), "ogame_fleet", 123); err != nil || found {
		t.Fatalf("expected missing admin recall fleet to no-op, found=%v err=%v", found, err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("fleet any rows failed"))}}}, "ogame_", func() time.Time { return now })
	if _, _, err := repository.loadRecallFleetAnyOwner(context.Background(), "ogame_fleet", 123); err == nil || !strings.Contains(err.Error(), "fleet any rows failed") {
		t.Fatalf("expected admin recall fleet rows error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}, "ogame_", func() time.Time { return now })
	if _, _, err := repository.loadRecallFleetAnyOwner(context.Background(), "ogame_fleet", 123); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
		t.Fatalf("expected admin recall fleet scan error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("fleet any trailer failed"), recallFleetTestRow(domaingame.FleetMissionTransport, 0, nil))}}}, "ogame_", func() time.Time { return now })
	if _, _, err := repository.loadRecallFleetAnyOwner(context.Background(), "ogame_fleet", 123); err == nil || !strings.Contains(err.Error(), "fleet any trailer failed") {
		t.Fatalf("expected admin recall fleet trailing rows error, got %v", err)
	}

	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("queue rows failed"))}}}, "ogame_", func() time.Time { return now })
	if _, _, err := repository.loadRecallQueue(context.Background(), "ogame_queue", 123); err == nil || !strings.Contains(err.Error(), "queue rows failed") {
		t.Fatalf("expected queue rows error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}, "ogame_", func() time.Time { return now })
	if _, _, err := repository.loadRecallQueue(context.Background(), "ogame_queue", 123); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
		t.Fatalf("expected queue scan error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("queue trailer failed"), []any{55, int64(940), int64(1_240)})}}}, "ogame_", func() time.Time { return now })
	if _, _, err := repository.loadRecallQueue(context.Background(), "ogame_queue", 123); err == nil || !strings.Contains(err.Error(), "queue trailer failed") {
		t.Fatalf("expected queue trailing rows error, got %v", err)
	}

	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("owner rows failed"))}}}, "ogame_", func() time.Time { return now })
	if _, _, err := repository.loadRecallOriginOwner(context.Background(), "ogame_planets", 99); err == nil || !strings.Contains(err.Error(), "owner rows failed") {
		t.Fatalf("expected owner rows error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_", func() time.Time { return now })
	if _, _, err := repository.loadRecallOriginOwner(context.Background(), "ogame_planets", 99); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected owner scan error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("owner trailer failed"), []any{44})}}}, "ogame_", func() time.Time { return now })
	if _, _, err := repository.loadRecallOriginOwner(context.Background(), "ogame_planets", 99); err == nil || !strings.Contains(err.Error(), "owner trailer failed") {
		t.Fatalf("expected owner trailing rows error, got %v", err)
	}

	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("target rows failed"))}}}, "ogame_", func() time.Time { return now })
	if _, err := repository.recallPlanetExists(context.Background(), "ogame_planets", 100); err == nil || !strings.Contains(err.Error(), "target rows failed") {
		t.Fatalf("expected target rows error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_", func() time.Time { return now })
	if _, err := repository.recallPlanetExists(context.Background(), "ogame_planets", 100); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected target scan error, got %v", err)
	}
	repository = NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("target trailer failed"), []any{100})}}}, "ogame_", func() time.Time { return now })
	if _, err := repository.recallPlanetExists(context.Background(), "ogame_planets", 100); err == nil || !strings.Contains(err.Error(), "target trailer failed") {
		t.Fatalf("expected target trailing rows error, got %v", err)
	}

	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("union rows failed"))}}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if err := repository.removeEmptyRecallUnion(context.Background(), "ogame_fleet", "ogame_union", 77); err == nil || !strings.Contains(err.Error(), "union rows failed") {
		t.Fatalf("expected union rows error, got %v", err)
	}
	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if err := repository.removeEmptyRecallUnion(context.Background(), "ogame_fleet", "ogame_union", 77); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected union scan error, got %v", err)
	}
	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if err := repository.removeEmptyRecallUnion(context.Background(), "ogame_fleet", "ogame_union", 77); err != nil || len(runner.execCalls) != 0 {
		t.Fatalf("empty union count should be no-op, calls=%+v err=%v", runner.execCalls, err)
	}
	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("union trailer failed"), []any{0})}}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if err := repository.removeEmptyRecallUnion(context.Background(), "ogame_fleet", "ogame_union", 77); err == nil || !strings.Contains(err.Error(), "union trailer failed") {
		t.Fatalf("expected union trailing rows error, got %v", err)
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
		fakeQueryResult{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix()})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{4})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{128})},
	)
}

func fleetMissionRow(mission int, ships map[int]int, start int64, end int64) []any {
	row := []any{11, start, end, mission, 42, "legor", 0, 99, 100}
	for _, id := range domaingame.FleetIDs() {
		row = append(row, ships[id])
	}
	row = append(row, 30, 20, 10)
	row = append(row, "Arakis", 1, 2, 3, "Target", 1, 2, 4, domaingame.PlanetTypePlanet, "target")
	return row
}

func templateRow(id int, name string, updatedAt int64, ships map[int]int) []any {
	row := []any{id, name, updatedAt}
	for _, shipID := range domaingame.FleetIDs() {
		row = append(row, ships[shipID])
	}
	return row
}

func fleetLaunchUserStateRow(id int, score int64, admin int, vacation int, banned int, noAttack int, lastClick int64) []any {
	return []any{id, score, admin, vacation, banned, noAttack, int64(0), lastClick}
}

func recallFleetTestRow(mission int, unionID int, ships map[int]int) []any {
	row := []any{123, 42, unionID, float64(100), float64(200), float64(300), 50, mission, 99, 100, 300, 600}
	for _, shipID := range domaingame.FleetIDs() {
		row = append(row, ships[shipID])
	}
	return row
}

func expeditionSettingsTestRow(event string) []any {
	settings := map[string]int{
		"chance_success": 100,
		"chance_alien":   100,
		"chance_pirates": 100,
		"chance_dm":      100,
		"chance_lost":    100,
		"chance_delay":   100,
		"chance_accel":   100,
		"chance_res":     100,
		"chance_fleet":   100,
		"dm_factor":      3,
	}
	switch event {
	case "nothing":
		settings["chance_success"] = 0
	case "aliens":
		settings["chance_alien"] = 0
	case "pirates":
		settings["chance_pirates"] = 0
	case "dark_matter":
		settings["chance_dm"] = 0
	case "black_hole":
		settings["chance_lost"] = 0
	case "delay":
		settings["chance_delay"] = 0
	case "accel":
		settings["chance_accel"] = 0
	case "resources":
		settings["chance_res"] = 0
	case "fleet":
		settings["chance_fleet"] = 0
	case "trader":
	default:
		panic("unknown expedition test event: " + event)
	}
	return []any{
		settings["chance_success"],
		settings["chance_alien"],
		settings["chance_pirates"],
		settings["chance_dm"],
		settings["chance_lost"],
		settings["chance_delay"],
		settings["chance_accel"],
		settings["chance_res"],
		settings["chance_fleet"],
		settings["dm_factor"],
	}
}

func expeditionTargetTestRow() []any {
	return []any{1, 470, 16, 0}
}

func firstExecContaining(calls []fakeFleetExecCall, needle string) *fakeFleetExecCall {
	for index := range calls {
		if strings.Contains(calls[index].sql, needle) {
			return &calls[index]
		}
	}
	return nil
}

func lastExecContaining(calls []fakeFleetExecCall, needle string) *fakeFleetExecCall {
	for index := len(calls) - 1; index >= 0; index-- {
		if strings.Contains(calls[index].sql, needle) {
			return &calls[index]
		}
	}
	return nil
}

func fleetCallContains(calls []fakeQueryCall, needle string) bool {
	for _, call := range calls {
		if strings.Contains(call.sql, needle) {
			return true
		}
	}
	return false
}

type fakeFleetRunner struct {
	fakeQueryer
	execCalls   []fakeFleetExecCall
	execErr     error
	execErrs    []error
	execResults []sql.Result
}

type fakeFleetExecCall struct {
	sql  string
	args []any
}

func (f *fakeFleetRunner) ExecContext(_ context.Context, sqlText string, args ...any) (sql.Result, error) {
	f.execCalls = append(f.execCalls, fakeFleetExecCall{sql: sqlText, args: args})
	result := sql.Result(fakeFleetSQLResult(1))
	if len(f.execResults) > 0 {
		result = f.execResults[0]
		f.execResults = f.execResults[1:]
	}
	err := f.execErr
	if len(f.execErrs) > 0 {
		err = f.execErrs[0]
		f.execErrs = f.execErrs[1:]
	}
	return result, err
}

type fakeFleetSQLResult int64

func (r fakeFleetSQLResult) LastInsertId() (int64, error) {
	return int64(r), nil
}

func (r fakeFleetSQLResult) RowsAffected() (int64, error) {
	return int64(r), nil
}

type fakeFleetSQLErrorResult struct {
	id      int64
	idErr   error
	rows    int64
	rowsErr error
}

func (r fakeFleetSQLErrorResult) LastInsertId() (int64, error) {
	return r.id, r.idErr
}

func (r fakeFleetSQLErrorResult) RowsAffected() (int64, error) {
	return r.rows, r.rowsErr
}
