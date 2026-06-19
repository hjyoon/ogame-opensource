package mysqlgame

import (
	"context"
	"database/sql"
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
	if !fleet.CommanderActive || fleet.TemplateLimit != 4 || len(fleet.Templates) != 1 || fleet.Templates[0].Name != "raid wing" {
		t.Fatalf("unexpected fleet template summary: %+v", fleet)
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
			name:    "templates",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(fleetReadPrefixResults(now), fakeQueryResult{rows: fakeRowsFromValues()}, fakeQueryResult{err: errors.New("templates failed")})},
			want:    "templates failed",
		},
		{
			name:    "missions",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(fleetCountsPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix()})}, fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0)})}, fakeQueryResult{rows: fakeRowsFromValues([]any{1})}, fakeQueryResult{err: errors.New("missions failed")})},
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

	runner := &fakeFleetRunner{}
	repository = NewFleetRepositoryWithRunner(runner, runner, "bad-prefix_", func() time.Time { return now })
	if err := repository.RecallFleet(context.Background(), appgame.FleetRecallQuery{PlayerID: 42, FleetID: 123}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected unsafe prefix error, got %v", err)
	}

	if err := repository.RecallFleet(context.Background(), appgame.FleetRecallQuery{PlayerID: 42}); err != nil {
		t.Fatalf("zero fleet id should be no-op, got %v", err)
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

func templateRow(id int, name string, updatedAt int64, ships map[int]int) []any {
	row := []any{id, name, updatedAt}
	for _, shipID := range domaingame.FleetIDs() {
		row = append(row, ships[shipID])
	}
	return row
}

func recallFleetTestRow(mission int, unionID int, ships map[int]int) []any {
	row := []any{123, 42, unionID, float64(100), float64(200), float64(300), 50, mission, 99, 100, 300, 600}
	for _, shipID := range domaingame.FleetIDs() {
		row = append(row, ships[shipID])
	}
	return row
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
