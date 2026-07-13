package mysqlgame

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestFleetRepositoryHoldingFleetForcesGuardedCombat(t *testing.T) {
	heldRow := recallFleetTestRow(
		domaingame.FleetMissionACSHold+domaingame.FleetMissionOrbitingOffset,
		0,
		map[int]int{domaingame.FleetSmallCargo: 1},
	)
	heldRow[0], heldRow[1], heldRow[8], heldRow[9] = 200, 44, 101, 100
	holderContext := fleetMessageContextTestRow()
	holderContext[0], holderContext[1], holderContext[2], holderContext[5] = 44, "Holder", "Holder Home", 5
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(fleetMessageContextTestRow())},
		{rows: fakeRowsFromValues(attackStateTestRow(nil, 0, 0, 0, 0, 0, 0))},
		{rows: fakeRowsFromValues([]any{1})},
		{rows: fakeRowsFromValues([]any{30, 0, 1, 70, 10, 2})},
		{rows: fakeRowsFromValues([]any{200})},
		{rows: fakeRowsFromValues(heldRow)},
		{rows: fakeRowsFromValues([]any{77, int64(1_000), int64(2_000)})},
		{rows: fakeRowsFromValues(holderContext)},
		{rows: fakeRowsFromValues([]any{1, 2, 3})},
		{rows: fakeRowsFromValues()},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	repository.combatRandom = func(int) int { return 0 }
	fleet := recallFleetRow{
		ID: 123, OwnerID: 42, Mission: domaingame.FleetMissionAttack,
		StartPlanetID: 99, TargetPlanetID: 100, FlightTime: 300,
		Ships: domaingame.FleetCounts{domaingame.FleetDeathstar: 1},
	}
	task := fleetQueueTask{TaskID: 55, OwnerID: 42, FleetID: 123, End: 1_700_000_000}

	if err := repository.finishAttackFleetArrival(
		context.Background(), "`ogame_uni`", "`ogame_fleet`", "`ogame_fleetlogs`", "`ogame_queue`",
		"`ogame_planets`", "`ogame_users`", "`ogame_messages`", "`ogame_battledata`", task, fleet,
	); err != nil {
		t.Fatal(err)
	}
	if source := fmt.Sprint(runner.execCalls[2].args[0]); !strings.Contains(source, "Defenders = 2") || !strings.Contains(source, "Defender1Name = Holder") {
		t.Fatalf("expected held fleet in battle source: %s", source)
	}
	if runner.execCalls[5].args[0] != 44 || !strings.Contains(fmt.Sprint(runner.execCalls[5].args[4]), "The attacker has won the battle!") {
		t.Fatalf("expected held fleet owner report: %+v", runner.execCalls[5])
	}
	deletedHoldingFleet := false
	deletedHoldingQueue := false
	for _, call := range runner.execCalls {
		if strings.Contains(call.sql, "DELETE FROM `ogame_fleet`") && len(call.args) > 0 && call.args[0] == 200 {
			deletedHoldingFleet = true
		}
		if strings.Contains(call.sql, "DELETE FROM `ogame_queue`") && len(call.args) > 0 && call.args[0] == 77 {
			deletedHoldingQueue = true
		}
	}
	if !deletedHoldingFleet || !deletedHoldingQueue {
		t.Fatalf("destroyed held fleet and queue must be removed: %+v", runner.execCalls)
	}
}

func TestFleetRepositoryLoadsHoldingCombatParticipants(t *testing.T) {
	ctx := context.Background()
	if limit := holdingCombatFleetLimit(0); limit != 0 {
		t.Fatalf("zero ACS must allow no held fleet slots: %d", limit)
	}
	if limit := holdingCombatFleetLimit(3); limit != 8 {
		t.Fatalf("ACS three must allow eight held fleet slots: %d", limit)
	}
	repository := NewFleetRepositoryWithRunner(&fakeFleetRunner{}, nil, "ogame_", nil)
	participants, err := repository.loadHoldingCombatParticipants(ctx, "`fleet`", "`queue`", "`planets`", "`users`", 100, 0)
	if err != nil || len(participants) != 0 {
		t.Fatalf("zero-slot holding lookup must be empty: %+v err=%v", participants, err)
	}

	heldRow := recallFleetTestRow(domaingame.FleetMissionACSHold+domaingame.FleetMissionOrbitingOffset, 0, map[int]int{domaingame.FleetCruiser: 2})
	heldRow[0], heldRow[1] = 200, 44
	for _, test := range []struct {
		name    string
		results []fakeQueryResult
		want    string
	}{
		{name: "ids query", results: []fakeQueryResult{{err: errors.New("holding ids failed")}}, want: "holding ids failed"},
		{name: "ids scan", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}, want: "expected int"},
		{name: "ids trailer", results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("holding ids trailer failed"), []any{200})}}, want: "holding ids trailer failed"},
		{name: "fleet missing", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{200})}, {rows: fakeRowsFromValues()}}, want: "holding participant fleet unavailable"},
	} {
		repository = NewFleetRepositoryWithRunner(&fakeFleetRunner{fakeQueryer: fakeQueryer{results: test.results}}, nil, "ogame_", nil)
		if _, err := repository.loadHoldingCombatParticipants(ctx, "`fleet`", "`queue`", "`planets`", "`users`", 100, 2); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("%s: expected %q, got %v", test.name, test.want, err)
		}
	}

	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{200})},
		{rows: fakeRowsFromValues(heldRow)},
		{rows: fakeRowsFromValues([]any{77, int64(1_000), int64(2_000)})},
		{rows: fakeRowsFromValues(fleetMessageContextTestRow())},
		{rows: fakeRowsFromValues([]any{1, 2, 3})},
	}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	participants, err = repository.loadHoldingCombatParticipants(ctx, "`fleet`", "`queue`", "`planets`", "`users`", 100, 2)
	if err != nil || len(participants) != 1 || participants[0].Fleet.ID != 200 || participants[0].Queue.TaskID != 77 || participants[0].Armour != 3 {
		t.Fatalf("unexpected holding participants: %+v err=%v", participants, err)
	}
}

func TestFleetRepositoryHoldingLookupEdges(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name    string
		result  fakeQueryResult
		wantErr string
		found   bool
	}{
		{name: "query", result: fakeQueryResult{err: errors.New("holding query failed")}, wantErr: "holding query failed"},
		{name: "empty trailer", result: fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("holding empty trailer failed"))}, wantErr: "holding empty trailer failed"},
		{name: "missing", result: fakeQueryResult{rows: fakeRowsFromValues()}, wantErr: "lookup unavailable"},
		{name: "scan", result: fakeQueryResult{rows: fakeRowsFromValues([]any{"bad"})}, wantErr: "expected int"},
		{name: "trailer", result: fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("holding trailer failed"), []any{0})}, wantErr: "holding trailer failed"},
		{name: "absent", result: fakeQueryResult{rows: fakeRowsFromValues([]any{0})}},
		{name: "present", result: fakeQueryResult{rows: fakeRowsFromValues([]any{1})}, found: true},
	} {
		repository := NewFleetRepositoryWithRunner(&fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{test.result}}}, nil, "ogame_", nil)
		found, err := repository.hasHoldingCombatFleet(ctx, "`fleet`", 100)
		if test.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("%s: expected %q, got %v", test.name, test.wantErr, err)
			}
			continue
		}
		if err != nil || found != test.found {
			t.Fatalf("%s: found=%v err=%v", test.name, found, err)
		}
	}
}

func TestFleetRepositoryWritesBackHoldingCombatFleets(t *testing.T) {
	ctx := context.Background()
	holding := []combatFleetParticipant{{Fleet: recallFleetRow{ID: 200, OwnerID: 44}, Queue: recallQueueRow{TaskID: 77}}}

	runner := &fakeFleetRunner{}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.writebackHoldingCombatFleets(ctx, "`fleet`", "`queue`", holding, []map[int]int{{}, {domaingame.FleetCruiser: 2}}); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 1 || !strings.Contains(runner.execCalls[0].sql, "UPDATE `fleet` SET") || runner.execCalls[0].args[len(runner.execCalls[0].args)-1] != 200 {
		t.Fatalf("surviving held fleet must be updated in place: %+v", runner.execCalls)
	}

	runner = &fakeFleetRunner{}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.writebackHoldingCombatFleets(ctx, "`fleet`", "`queue`", holding, []map[int]int{{}, {}}); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 2 || !strings.Contains(runner.execCalls[0].sql, "DELETE FROM `fleet`") || !strings.Contains(runner.execCalls[1].sql, "DELETE FROM `queue`") {
		t.Fatalf("destroyed held fleet and task must be removed: %+v", runner.execCalls)
	}
	runner = &fakeFleetRunner{}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.writebackHoldingCombatFleets(ctx, "`fleet`", "`queue`", holding, []map[int]int{{}}); err != nil || len(runner.execCalls) != 2 {
		t.Fatalf("missing held survivor slot must be treated as destroyed: %+v err=%v", runner.execCalls, err)
	}

	for _, test := range []struct {
		name     string
		survived bool
		failAt   int
	}{
		{name: "survivor update", survived: true, failAt: 0},
		{name: "fleet delete", failAt: 0},
		{name: "queue delete", failAt: 1},
	} {
		execErrs := make([]error, test.failAt+1)
		execErrs[test.failAt] = errors.New(test.name + " failed")
		runner = &fakeFleetRunner{execErrs: execErrs}
		repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
		survivors := []map[int]int{{}, {}}
		if test.survived {
			survivors[1] = map[int]int{domaingame.FleetCruiser: 1}
		}
		if err := repository.writebackHoldingCombatFleets(ctx, "`fleet`", "`queue`", holding, survivors); err == nil || !strings.Contains(err.Error(), test.name) {
			t.Fatalf("%s: got %v", test.name, err)
		}
	}
}

func TestCombatDefenderHelpers(t *testing.T) {
	value := fleetMessageContextTestValue()
	state := unguardedAttackState{DefenderUnits: map[int]int{domaingame.DefenseRocketLauncher: 2}, DefenderWeapon: 1}
	holding := []combatFleetParticipant{{
		Fleet:   recallFleetRow{ID: 200, OwnerID: 44, Ships: domaingame.FleetCounts{domaingame.FleetCruiser: 3}},
		Context: fleetMessageContext{OriginOwnerName: "Holder", OriginGalaxy: 2, OriginSystem: 3, OriginPosition: 4},
		Weapon:  5, Shield: 6, Armour: 7,
	}}
	defenders := combatDefenderSlots(100, value, state, holding)
	if len(defenders) != 2 || defenders[0].ObjectID != 100 || !defenders[0].Planet || defenders[1].Name != "Holder" || defenders[1].Units[domaingame.FleetCruiser] != 3 {
		t.Fatalf("unexpected defender slots: %+v", defenders)
	}
	owners := combatDefenderOwnerIDs(value, holding)
	if len(owners) != 2 || owners[0] != value.TargetOwnerID || owners[1] != 44 {
		t.Fatalf("unexpected defender owners: %+v", owners)
	}

	runner := &fakeFleetRunner{}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	losses := []domaingame.CombatParticipantLoss{{Points: 10}, {Points: 20}}
	if err := repository.adjustCombatDefenderStats(context.Background(), "`users`", value, holding, losses); err != nil || len(runner.execCalls) != 2 {
		t.Fatalf("unexpected defender stats writes: %+v err=%v", runner.execCalls, err)
	}
	runner = &fakeFleetRunner{}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.adjustCombatDefenderStats(context.Background(), "`users`", value, holding, losses[:1]); err != nil || len(runner.execCalls) != 1 {
		t.Fatalf("short defender loss list must stop cleanly: %+v err=%v", runner.execCalls, err)
	}
	runner = &fakeFleetRunner{execErrs: []error{errors.New("defender stats failed")}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.adjustCombatDefenderStats(context.Background(), "`users`", value, holding, losses); err == nil || !strings.Contains(err.Error(), "defender stats failed") {
		t.Fatalf("expected defender stats error, got %v", err)
	}
}

func TestHoldingCombatMessageRecipientsAndLoadFailures(t *testing.T) {
	ctx := context.Background()
	value := fleetMessageContextTestValue()
	holding := []combatFleetParticipant{{Fleet: recallFleetRow{OwnerID: 44}}}
	result := domaingame.CombatResult{Outcome: domaingame.CombatAttackerWon}
	writeback := domaingame.CombatWriteback{
		AttackerLosses: []domaingame.CombatParticipantLoss{{Points: 10}},
		DefenderLosses: []domaingame.CombatParticipantLoss{{Points: 20}, {Points: 30}},
	}
	runner := &fakeFleetRunner{}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.insertGuardedBattleMessages(ctx, "`messages`", holding, value, "report", result, writeback, 1_000); err != nil || len(runner.execCalls) != 6 {
		t.Fatalf("planet, holder, and attacker must each receive a report pair: %+v err=%v", runner.execCalls, err)
	}

	runner = &fakeFleetRunner{execErrs: []error{nil, nil, errors.New("holder report failed")}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.insertACSBattleMessages(ctx, "`messages`", []combatFleetParticipant{{Fleet: recallFleetRow{OwnerID: 42}}}, holding, value, "report", result, writeback, 1_000); err == nil || !strings.Contains(err.Error(), "holder report failed") {
		t.Fatalf("expected held-defender message error, got %v", err)
	}

	fleet := recallFleetRow{ID: 123, OwnerID: 42, TargetPlanetID: 100, Ships: domaingame.FleetCounts{domaingame.FleetSmallCargo: 1}}
	task := fleetQueueTask{TaskID: 55, End: 1_000}
	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{30, 0, 0, 70, 10, 2})},
		{err: errors.New("guarded holding load failed")},
	}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.finishGuardedAttackFleetArrival(ctx, "`uni`", "`fleet`", "`logs`", "`queue`", "`planets`", "`users`", "`messages`", "`battle`", task, fleet, value, unguardedAttackState{}); err == nil || !strings.Contains(err.Error(), "guarded holding load failed") {
		t.Fatalf("expected guarded holding loader error, got %v", err)
	}
}
