package mysqlgame

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestFleetRepositoryFinishesACSAttack(t *testing.T) {
	results, head, task := acsCombatTestFixture()
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	repository.combatRandom = func(int) int { return 0 }

	err := repository.finishACSAttackFleetArrival(
		context.Background(), "`ogame_uni`", "`ogame_fleet`", "`ogame_fleetlogs`", "`ogame_queue`",
		"`ogame_planets`", "`ogame_users`", "`ogame_messages`", "`ogame_battledata`", "`ogame_union`", task, head,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 35 {
		t.Fatalf("expected complete two-participant ACS writeback, got %d calls: %+v", len(runner.execCalls), runner.execCalls)
	}
	if source := fmt.Sprint(runner.execCalls[2].args[0]); !strings.Contains(source, "Rapidfire = 1") || !strings.Contains(source, "Attackers = 2") || !strings.Contains(source, "Attacker1Name = Support") {
		t.Fatalf("unexpected ACS battle source: %s", source)
	}
	if !strings.Contains(runner.execCalls[34].sql, "DELETE FROM `ogame_union`") || runner.execCalls[34].args[0] != 7 {
		t.Fatalf("expected final union cleanup, got %+v", runner.execCalls[34])
	}
}

func TestFleetRepositoryFinishesDueACSAttackQueue(t *testing.T) {
	results, _, task := acsCombatTestFixture()
	headRow := recallFleetTestRow(domaingame.FleetMissionACSAttackHead, 7, map[int]int{domaingame.FleetSmallCargo: 1})
	results = append([]fakeQueryResult{
		{rows: fakeRowsFromValues([]any{0})},
		{rows: fakeRowsFromValues([]any{task.TaskID, task.OwnerID, task.FleetID, task.End})},
		{rows: fakeRowsFromValues(headRow)},
	}, results...)
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	repository.combatRandom = func(int) int { return 0 }
	if err := repository.FinishDueFleetQueues(context.Background(), int(task.End)); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 35 || !strings.Contains(runner.execCalls[34].sql, "ogame_union") {
		t.Fatalf("expected ACS queue entrypoint to complete union cleanup: %+v", runner.execCalls)
	}
}

func TestFleetRepositoryACSAttackErrors(t *testing.T) {
	ctx := context.Background()
	call := func(repository FleetRepository, head recallFleetRow, task fleetQueueTask) error {
		return repository.finishACSAttackFleetArrival(
			ctx, "`ogame_uni`", "`ogame_fleet`", "`ogame_fleetlogs`", "`ogame_queue`",
			"`ogame_planets`", "`ogame_users`", "`ogame_messages`", "`ogame_battledata`", "`ogame_union`", task, head,
		)
	}
	_, head, task := acsCombatTestFixture()
	invalidHead := head
	invalidHead.UnionID = 0
	if err := call(NewFleetRepositoryWithRunner(&fakeFleetRunner{}, nil, "ogame_", nil), invalidHead, task); err == nil || !strings.Contains(err.Error(), "union unavailable") {
		t.Fatalf("expected missing union error, got %v", err)
	}
	for _, test := range []struct {
		name    string
		results []fakeQueryResult
		want    string
	}{
		{name: "participants", results: []fakeQueryResult{{err: errors.New("participants failed")}}, want: "participants failed"},
		{name: "empty participants", results: []fakeQueryResult{{rows: fakeRowsFromValues()}}, want: "fleets unavailable"},
	} {
		runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: test.results}}
		repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
		if err := call(repository, head, task); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("%s: expected %q, got %v", test.name, test.want, err)
		}
	}

	for _, test := range []struct {
		name  string
		index int
		row   fakeQueryResult
		want  string
	}{
		{name: "target query", index: 9, row: fakeQueryResult{err: errors.New("target failed")}, want: "target failed"},
		{name: "target missing", index: 9, row: fakeQueryResult{rows: fakeRowsFromValues()}, want: "target unavailable"},
		{name: "settings", index: 10, row: fakeQueryResult{err: errors.New("settings failed")}, want: "settings failed"},
	} {
		results, testHead, testTask := acsCombatTestFixture()
		results[test.index] = test.row
		runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: results}}
		repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
		if err := call(repository, testHead, testTask); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("%s: expected %q, got %v", test.name, test.want, err)
		}
	}
	results, testHead, testTask := acsCombatTestFixture()
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	repository.combatRandom = nil
	if err := call(repository, testHead, testTask); err == nil || !strings.Contains(err.Error(), "random source unavailable") {
		t.Fatalf("expected missing combat random source, got %v", err)
	}

	for failAt := 0; failAt < 35; failAt++ {
		results, testHead, testTask := acsCombatTestFixture()
		execErrs := make([]error, 35)
		execErrs[failAt] = fmt.Errorf("ACS write %d failed", failAt)
		runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: results}, execErrs: execErrs}
		repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
		repository.combatRandom = func(int) int { return 0 }
		if err := call(repository, testHead, testTask); err == nil || !strings.Contains(err.Error(), fmt.Sprintf("ACS write %d failed", failAt)) {
			t.Fatalf("write %d: unexpected error %v", failAt, err)
		}
	}
}

func TestACSCombatHelpersAndMessageDeduplication(t *testing.T) {
	captured := domaingame.Resources{Metal: 90, Crystal: 60, Deuterium: 30}
	if empty := combatPlunderShare(captured, 0, 100); empty != (domaingame.Resources{}) {
		t.Fatalf("zero capacity must receive no plunder: %+v", empty)
	}
	share := combatPlunderShare(captured, 25, 100)
	if share.Metal != 22.5 || share.Crystal != 15 || share.Deuterium != 7.5 {
		t.Fatalf("unexpected proportional ACS plunder: %+v", share)
	}
	writeback := domaingame.CombatWriteback{
		AttackerLosses: []domaingame.CombatParticipantLoss{{Points: 100}, {Points: 200}},
		DefenderLosses: []domaingame.CombatParticipantLoss{{Points: 400}},
	}
	if attacker, defender := combatLossTotals(writeback); attacker != 300 || defender != 400 {
		t.Fatalf("unexpected ACS loss totals: %d/%d", attacker, defender)
	}

	value := fleetMessageContextTestValue()
	participants := []combatFleetParticipant{
		{Fleet: recallFleetRow{OwnerID: 42}},
		{Fleet: recallFleetRow{OwnerID: 42}},
	}
	result := domaingame.CombatResult{Outcome: domaingame.CombatDefenderWon}
	runner := &fakeFleetRunner{}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.insertACSBattleMessages(context.Background(), "`ogame_messages`", participants, nil, value, "report", result, writeback, 1_000); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 4 || !strings.Contains(fmt.Sprint(runner.execCalls[2].args[4]), "Contact with the attacking fleet has been lost") {
		t.Fatalf("expected one defender and one deduplicated attacker report pair: %+v", runner.execCalls)
	}

	sourceResult := domaingame.CombatResult{}
	sourceResult.Before.Attackers = []domaingame.CombatSlot{{Name: "A", Weapon: 1, Units: map[int]int{domaingame.FleetSmallCargo: 2}}}
	sourceResult.Before.Defenders = []domaingame.CombatSlot{{Name: "D", Shield: 2, Units: map[int]int{domaingame.DefenseRocketLauncher: 3}}}
	source := acsBattleSource(sourceResult, false)
	if !strings.Contains(source, "Rapidfire = 0") || !strings.Contains(source, "Defender0Name = D") || combatBoolInt(true) != 1 || combatBoolInt(false) != 0 {
		t.Fatalf("unexpected ACS source helpers: %s", source)
	}
}

func TestFleetRepositoryLoadsACSCombatParticipants(t *testing.T) {
	ctx := context.Background()
	headRow := recallFleetTestRow(domaingame.FleetMissionACSAttackHead, 7, map[int]int{domaingame.FleetSmallCargo: 1})
	queueRow := []any{55, int64(1_000), int64(1_300)}

	tests := []struct {
		name    string
		results []fakeQueryResult
		want    string
	}{
		{name: "ids query", results: []fakeQueryResult{{err: errors.New("ids failed")}}, want: "ids failed"},
		{name: "ids scan", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}, want: "expected int"},
		{name: "ids trailer", results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("ids trailer failed"), []any{123})}}, want: "ids trailer failed"},
		{name: "fleet query", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{123})}, {err: errors.New("fleet query failed")}}, want: "fleet query failed"},
		{name: "fleet missing", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{123})}, {rows: fakeRowsFromValues()}}, want: "fleet unavailable"},
		{name: "queue query", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{123})}, {rows: fakeRowsFromValues(headRow)}, {err: errors.New("queue query failed")}}, want: "queue query failed"},
		{name: "queue missing", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{123})}, {rows: fakeRowsFromValues(headRow)}, {rows: fakeRowsFromValues()}}, want: "queue unavailable"},
		{name: "context query", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{123})}, {rows: fakeRowsFromValues(headRow)}, {rows: fakeRowsFromValues(queueRow)}, {err: errors.New("context query failed")}}, want: "context query failed"},
		{name: "context missing", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{123})}, {rows: fakeRowsFromValues(headRow)}, {rows: fakeRowsFromValues(queueRow)}, {rows: fakeRowsFromValues()}}, want: "context unavailable"},
		{name: "technology query", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{123})}, {rows: fakeRowsFromValues(headRow)}, {rows: fakeRowsFromValues(queueRow)}, {rows: fakeRowsFromValues(fleetMessageContextTestRow())}, {err: errors.New("technology query failed")}}, want: "technology query failed"},
		{name: "technology missing", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{123})}, {rows: fakeRowsFromValues(headRow)}, {rows: fakeRowsFromValues(queueRow)}, {rows: fakeRowsFromValues(fleetMessageContextTestRow())}, {rows: fakeRowsFromValues()}}, want: "technology unavailable"},
	}
	for _, test := range tests {
		repository := NewFleetRepositoryWithRunner(&fakeFleetRunner{fakeQueryer: fakeQueryer{results: test.results}}, nil, "ogame_", nil)
		if _, err := repository.loadACSCombatParticipants(ctx, "`fleet`", "`queue`", "`planets`", "`users`", 7); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("%s: expected %q, got %v", test.name, test.want, err)
		}
	}
}

func acsCombatTestFixture() ([]fakeQueryResult, recallFleetRow, fleetQueueTask) {
	headRow := recallFleetTestRow(domaingame.FleetMissionACSAttackHead, 7, map[int]int{domaingame.FleetSmallCargo: 1})
	supportRow := recallFleetTestRow(domaingame.FleetMissionACSAttack, 7, map[int]int{domaingame.FleetSmallCargo: 1})
	supportRow[0], supportRow[1], supportRow[8] = 124, 44, 101
	supportContext := fleetMessageContextTestRow()
	supportContext[0], supportContext[1], supportContext[2], supportContext[5] = 44, "Support", "Support Home", 5
	results := []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{123}, []any{124})},
		{rows: fakeRowsFromValues(headRow)},
		{rows: fakeRowsFromValues([]any{55, int64(1_000), int64(1_300)})},
		{rows: fakeRowsFromValues(fleetMessageContextTestRow())},
		{rows: fakeRowsFromValues([]any{1, 2, 3})},
		{rows: fakeRowsFromValues(supportRow)},
		{rows: fakeRowsFromValues([]any{56, int64(1_000), int64(1_300)})},
		{rows: fakeRowsFromValues(supportContext)},
		{rows: fakeRowsFromValues([]any{4, 5, 6})},
		{rows: fakeRowsFromValues(attackStateTestRow(nil, 0, 0, 0, 0, 0, 0))},
		{rows: fakeRowsFromValues([]any{30, 0, 1, 70, 10, 2})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues()},
	}
	head := recallFleetRow{
		ID: 123, OwnerID: 42, UnionID: 7, Mission: domaingame.FleetMissionACSAttackHead,
		StartPlanetID: 99, TargetPlanetID: 100, FlightTime: 300,
		Ships: domaingame.FleetCounts{domaingame.FleetSmallCargo: 1},
	}
	task := fleetQueueTask{TaskID: 55, OwnerID: 42, FleetID: 123, End: 1_700_000_000}
	return results, head, task
}

func TestFleetRepositoryLoadsCombatPlayerTechnologyEdges(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		result  fakeQueryResult
		wantErr string
		found   bool
	}{
		{name: "query", result: fakeQueryResult{err: errors.New("technology query failed")}, wantErr: "technology query failed"},
		{name: "missing", result: fakeQueryResult{rows: fakeRowsFromValues()}},
		{name: "scan", result: fakeQueryResult{rows: fakeRowsFromValues([]any{"bad"})}, wantErr: "unexpected scan"},
		{name: "trailer", result: fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("technology trailer failed"), []any{1, 2, 3})}, wantErr: "technology trailer failed"},
		{name: "success", result: fakeQueryResult{rows: fakeRowsFromValues([]any{1, 2, 3})}, found: true},
	}
	for _, test := range tests {
		repository := NewFleetRepositoryWithRunner(&fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{test.result}}}, nil, "ogame_", nil)
		weapon, shield, armour, found, err := repository.loadCombatPlayerTechnology(ctx, "`users`", 42)
		if test.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("%s: expected %q, got %v", test.name, test.wantErr, err)
			}
			continue
		}
		if err != nil || found != test.found || (found && (weapon != 1 || shield != 2 || armour != 3)) {
			t.Fatalf("%s: unexpected result %d/%d/%d found=%v err=%v", test.name, weapon, shield, armour, found, err)
		}
	}
}
