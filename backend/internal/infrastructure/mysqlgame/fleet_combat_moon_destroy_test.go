package mysqlgame

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestFleetRepositoryMoonDestructionSkipsIneligibleCombat(t *testing.T) {
	value := fleetMessageContextTestValue()
	fleet := recallFleetRow{Mission: domaingame.FleetMissionDestroy, Ships: domaingame.FleetCounts{domaingame.FleetDeathstar: 1}}
	for _, test := range []struct {
		name        string
		fleet       recallFleetRow
		survivors   domaingame.FleetCounts
		attackerWon bool
	}{
		{name: "other mission", fleet: recallFleetRow{Mission: domaingame.FleetMissionAttack}, survivors: fleet.Ships, attackerWon: true},
		{name: "defender won", fleet: fleet, survivors: fleet.Ships},
		{name: "no surviving deathstar", fleet: fleet, survivors: domaingame.FleetCounts{}, attackerWon: true},
	} {
		runner := &fakeFleetRunner{}
		repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
		result, err := repository.finishMoonDestruction(context.Background(), "`fleet`", "`queue`", "`planets`", "`users`", "`messages`", fleetQueueTask{}, test.fleet, value, test.survivors, test.attackerWon)
		if err != nil || result != (moonDestructionResolution{}) || len(runner.calls) != 0 || len(runner.execCalls) != 0 {
			t.Fatalf("%s: expected no-op, result=%+v queries=%+v writes=%+v err=%v", test.name, result, runner.calls, runner.execCalls, err)
		}
	}
}

func TestFleetRepositoryMoonDestructionResolvesAllOutcomes(t *testing.T) {
	value := fleetMessageContextTestValue()
	value.TargetType = domaingame.PlanetTypeMoon
	fleet := recallFleetRow{
		ID: 123, OwnerID: 42, Mission: domaingame.FleetMissionDestroy,
		StartPlanetID: 99, TargetPlanetID: 100,
		Ships: domaingame.FleetCounts{domaingame.FleetDeathstar: 1},
	}
	task := fleetQueueTask{TaskID: 55, End: 1_700_000_000}

	for _, test := range []struct {
		name       string
		diameter   int
		rolls      []int
		wantCode   int
		wantReturn int
		wantText   string
	}{
		{name: "neither", diameter: 10_000, rolls: []int{998, 998}, wantText: "weakened enough"},
		{name: "fleet", diameter: 100, rolls: []int{998, 0}, wantCode: domaingame.MoonDestroyFleet, wantText: "destroys your entire fleet"},
		{name: "moon", diameter: 1, rolls: []int{0, 998}, wantCode: domaingame.MoonDestroyMoon, wantReturn: 200, wantText: "destroy the satellite"},
		{name: "both", diameter: 1, rolls: []int{0, 0}, wantCode: domaingame.MoonDestroyMoon | domaingame.MoonDestroyFleet, wantReturn: 200, wantText: "hail of debris"},
	} {
		t.Run(test.name, func(t *testing.T) {
			queryResults := []fakeQueryResult{{rows: fakeRowsFromValues([]any{domaingame.PlanetTypeMoon, test.diameter})}}
			if test.wantCode&domaingame.MoonDestroyMoon != 0 {
				scoreRow := append([]any{43}, recalcPlanetScoreRow(map[int]int{domaingame.BuildingLunarBase: 1}, nil, nil)...)
				queryResults = append(queryResults,
					fakeQueryResult{rows: fakeRowsFromValues(scoreRow)},
					fakeQueryResult{rows: fakeRowsFromValues([]any{200, 43})},
					fakeQueryResult{rows: fakeRowsFromValues()},
				)
			}
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: queryResults}}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
			rolls := append([]int(nil), test.rolls...)
			repository.combatRandom = func(max int) int {
				roll := rolls[0]
				rolls = rolls[1:]
				return roll % max
			}
			result, err := repository.finishMoonDestruction(context.Background(), "`ogame_fleet`", "`ogame_queue`", "`ogame_planets`", "`ogame_users`", "`ogame_messages`", task, fleet, value, fleet.Ships, true)
			if err != nil {
				t.Fatal(err)
			}
			if result.Code != test.wantCode || result.ReturnTargetID != test.wantReturn {
				t.Fatalf("unexpected result: %+v", result)
			}
			attackerMessage := lastExecContaining(runner.execCalls, "INSERT INTO `ogame_messages`")
			if attackerMessage == nil || !fleetExecsContainArg(runner.execCalls, test.wantText) {
				t.Fatalf("missing %q graviton message: %+v", test.wantText, runner.execCalls)
			}
			deleted := fleetExecsContainSQL(runner.execCalls, "DELETE FROM `ogame_planets`")
			if deleted != (test.wantCode&domaingame.MoonDestroyMoon != 0) {
				t.Fatalf("moon deletion mismatch: code=%d writes=%+v", test.wantCode, runner.execCalls)
			}
			fleetScoreWrites := fleetExecCountSQL(runner.execCalls, "score1 = score1 - ?")
			wantScoreWrites := 0
			if test.wantCode&domaingame.MoonDestroyMoon != 0 {
				wantScoreWrites++
			}
			if test.wantCode&domaingame.MoonDestroyFleet != 0 {
				wantScoreWrites++
			}
			if fleetScoreWrites != wantScoreWrites {
				t.Fatalf("expected %d score removals, got %d: %+v", wantScoreWrites, fleetScoreWrites, runner.execCalls)
			}
		})
	}
}

func TestFleetRepositoryDestroyBattleMoonRecallsAndRedirectsForeignFleet(t *testing.T) {
	scoreRow := append([]any{43}, recalcPlanetScoreRow(nil, nil, nil)...)
	foreignFleet := recallFleetTestRow(domaingame.FleetMissionTransport, 0, map[int]int{domaingame.FleetSmallCargo: 1})
	foreignFleet[0] = 777
	foreignFleet[1] = 44
	foreignFleet[8] = 300
	foreignFleet[9] = 100
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(scoreRow)},
		{rows: fakeRowsFromValues([]any{200, 43})},
		{rows: fakeRowsFromValues([]any{777})},
		{rows: fakeRowsFromValues([]any{0})},
		{rows: fakeRowsFromValues(foreignFleet)},
		{rows: fakeRowsFromValues([]any{888, int64(900), int64(1_500)})},
		{rows: fakeRowsFromValues([]any{44})},
		{rows: fakeRowsFromValues([]any{100})},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
	planetID, err := repository.destroyBattleMoon(context.Background(), "`ogame_fleet`", "`ogame_queue`", "`ogame_planets`", "`ogame_users`", 100, 123, fleetMessageContextTestValue(), 1_000)
	if err != nil {
		t.Fatal(err)
	}
	if planetID != 200 {
		t.Fatalf("unexpected underlying planet: %d", planetID)
	}
	inserted := firstExecContaining(runner.execCalls, "INSERT INTO `ogame_fleet`")
	if inserted == nil || inserted.args[6] != domaingame.FleetMissionTransport+domaingame.FleetMissionReturnOffset || inserted.args[7] != 300 || inserted.args[8] != 100 || inserted.args[9] != int64(100) {
		t.Fatalf("unexpected recalled fleet: %+v", inserted)
	}
	queue := firstExecContaining(runner.execCalls, "INSERT INTO `ogame_queue`")
	if queue == nil || queue.args[5] != int64(1_000) || queue.args[6] != int64(1_100) {
		t.Fatalf("recall must use moon destruction time: %+v", queue)
	}
	if !fleetExecsContainArgs(runner.execCalls, "UPDATE `ogame_fleet` SET target_planet", 200, 100) ||
		!fleetExecsContainArgs(runner.execCalls, "UPDATE `ogame_users` SET aktplanet", 200, 43) {
		t.Fatalf("moon references and active planet were not redirected: %+v", runner.execCalls)
	}
}

func TestMoonDestructionLoadAndValidationErrors(t *testing.T) {
	ctx := context.Background()
	value := fleetMessageContextTestValue()
	fleet := recallFleetRow{Mission: domaingame.FleetMissionDestroy, TargetPlanetID: 100, Ships: domaingame.FleetCounts{domaingame.FleetDeathstar: 1}}
	task := fleetQueueTask{}
	for _, test := range []struct {
		name    string
		result  fakeQueryResult
		wantErr string
	}{
		{name: "query", result: fakeQueryResult{err: fmt.Errorf("moon destroy query failed")}, wantErr: "query failed"},
		{name: "empty trailer", result: fakeQueryResult{rows: fakeRowsFromValuesWithErr(fmt.Errorf("moon destroy empty trailer failed"))}, wantErr: "empty trailer failed"},
		{name: "scan", result: fakeQueryResult{rows: fakeRowsFromValues([]any{"bad"})}, wantErr: "unexpected scan"},
		{name: "trailer", result: fakeQueryResult{rows: fakeRowsFromValuesWithErr(fmt.Errorf("moon destroy trailer failed"), []any{domaingame.PlanetTypeMoon, 100})}, wantErr: "trailer failed"},
		{name: "planet", result: fakeQueryResult{rows: fakeRowsFromValues([]any{domaingame.PlanetTypePlanet, 100})}, wantErr: "only moons"},
	} {
		runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{test.result}}}
		repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
		repository.combatRandom = func(int) int { return 0 }
		_, err := repository.finishMoonDestruction(ctx, "`fleet`", "`queue`", "`planets`", "`users`", "`messages`", task, fleet, value, fleet.Ships, true)
		if err == nil || !strings.Contains(err.Error(), test.wantErr) {
			t.Fatalf("%s: expected %q, got %v", test.name, test.wantErr, err)
		}
	}

	repository := NewFleetRepositoryWithRunner(&fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}, nil, "ogame_", nil)
	repository.combatRandom = func(int) int { return 0 }
	if result, err := repository.finishMoonDestruction(ctx, "`fleet`", "`queue`", "`planets`", "`users`", "`messages`", task, fleet, value, fleet.Ships, true); err != nil || result != (moonDestructionResolution{}) {
		t.Fatalf("missing target should match legacy no-op: result=%+v err=%v", result, err)
	}

	repository = NewFleetRepositoryWithRunner(&fakeFleetRunner{}, nil, "ogame_", nil)
	repository.combatRandom = nil
	if _, err := repository.finishMoonDestruction(ctx, "`fleet`", "`queue`", "`planets`", "`users`", "`messages`", task, fleet, value, fleet.Ships, true); err == nil || !strings.Contains(err.Error(), "random source") {
		t.Fatalf("expected random source error, got %v", err)
	}
}

func TestMoonDestructionLookupErrorBoundaries(t *testing.T) {
	ctx := context.Background()
	value := fleetMessageContextTestValue()
	for _, test := range []struct {
		name    string
		result  fakeQueryResult
		wantErr string
	}{
		{name: "query", result: fakeQueryResult{err: errors.New("underlying query failed")}, wantErr: "query failed"},
		{name: "empty trailer", result: fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("underlying empty trailer failed"))}, wantErr: "empty trailer failed"},
		{name: "scan", result: fakeQueryResult{rows: fakeRowsFromValues([]any{"bad"})}, wantErr: "unexpected scan"},
		{name: "trailer", result: fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("underlying trailer failed"), []any{200, 43})}, wantErr: "trailer failed"},
	} {
		repository := NewFleetRepositoryWithRunner(&fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{test.result}}}, nil, "ogame_", nil)
		if _, _, _, err := repository.loadMoonUnderlyingPlanet(ctx, "`planets`", value); err == nil || !strings.Contains(err.Error(), test.wantErr) {
			t.Fatalf("%s: expected %q, got %v", test.name, test.wantErr, err)
		}
	}
	repository := NewFleetRepositoryWithRunner(&fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}, nil, "ogame_", nil)
	if id, owner, found, err := repository.loadMoonUnderlyingPlanet(ctx, "`planets`", value); err != nil || found || id != 0 || owner != 0 {
		t.Fatalf("missing underlying planet should be a no-op: id=%d owner=%d found=%v err=%v", id, owner, found, err)
	}

	for _, test := range []struct {
		name    string
		result  fakeQueryResult
		wantErr string
	}{
		{name: "query", result: fakeQueryResult{err: errors.New("foreign query failed")}, wantErr: "query failed"},
		{name: "scan", result: fakeQueryResult{rows: fakeRowsFromValues([]any{"bad"})}, wantErr: "expected int"},
		{name: "trailer", result: fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("foreign trailer failed"), []any{777})}, wantErr: "trailer failed"},
	} {
		repository = NewFleetRepositoryWithRunner(&fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{test.result}}}, nil, "ogame_", nil)
		if _, err := repository.loadMoonForeignFleetIDs(ctx, "`fleet`", 100, 123, 43); err == nil || !strings.Contains(err.Error(), test.wantErr) {
			t.Fatalf("%s: expected %q, got %v", test.name, test.wantErr, err)
		}
	}
}

func TestDestroyBattleMoonPropagatesMutationErrors(t *testing.T) {
	ctx := context.Background()
	value := fleetMessageContextTestValue()
	scoreRow := append([]any{43}, recalcPlanetScoreRow(nil, nil, nil)...)
	validQueries := func() []fakeQueryResult {
		return []fakeQueryResult{
			{rows: fakeRowsFromValues(scoreRow)},
			{rows: fakeRowsFromValues([]any{200, 43})},
			{rows: fakeRowsFromValues()},
		}
	}

	repository := NewFleetRepositoryWithRunner(&fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("moon score failed")}}}}, nil, "ogame_", nil)
	if _, err := repository.destroyBattleMoon(ctx, "`fleet`", "`queue`", "`planets`", "`users`", 100, 123, value, 1_000); err == nil || !strings.Contains(err.Error(), "score failed") {
		t.Fatalf("expected score error, got %v", err)
	}
	for _, result := range []fakeQueryResult{
		{err: errors.New("underlying failed")},
		{rows: fakeRowsFromValues()},
	} {
		runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(scoreRow)}, result}}}
		repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
		if id, err := repository.destroyBattleMoon(ctx, "`fleet`", "`queue`", "`planets`", "`users`", 100, 123, value, 1_000); err == nil && id != 0 {
			t.Fatalf("underlying failure/missing must not destroy moon: id=%d err=%v", id, err)
		}
	}
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(scoreRow)}, {rows: fakeRowsFromValues([]any{200, 43})}, {err: errors.New("foreign load failed")},
	}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	if _, err := repository.destroyBattleMoon(ctx, "`fleet`", "`queue`", "`planets`", "`users`", 100, 123, value, 1_000); err == nil || !strings.Contains(err.Error(), "foreign load failed") {
		t.Fatalf("expected foreign load error, got %v", err)
	}
	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(scoreRow)}, {rows: fakeRowsFromValues([]any{200, 43})}, {rows: fakeRowsFromValues([]any{777})}, {err: errors.New("recall failed")},
	}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	if _, err := repository.destroyBattleMoon(ctx, "`fleet`", "`queue`", "`planets`", "`users`", 100, 123, value, 1_000); err == nil || !strings.Contains(err.Error(), "recall failed") {
		t.Fatalf("expected recall error, got %v", err)
	}

	for failAt := 0; failAt < 16; failAt++ {
		execErrs := make([]error, failAt+1)
		execErrs[failAt] = fmt.Errorf("moon mutation %d failed", failAt)
		runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: validQueries()}, execErrs: execErrs}
		repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
		if _, err := repository.destroyBattleMoon(ctx, "`fleet`", "`queue`", "`planets`", "`users`", 100, 123, value, 1_000); err == nil || !strings.Contains(err.Error(), fmt.Sprintf("moon mutation %d failed", failAt)) {
			t.Fatalf("write %d: unexpected error %v", failAt, err)
		}
	}
	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: validQueries()}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "invalid-", nil)
	if _, err := repository.destroyBattleMoon(ctx, "`fleet`", "`queue`", "`planets`", "`users`", 100, 123, value, 1_000); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected build queue table error, got %v", err)
	}
}

func TestFinishMoonDestructionPropagatesWriteErrors(t *testing.T) {
	ctx := context.Background()
	value := fleetMessageContextTestValue()
	fleet := recallFleetRow{ID: 123, OwnerID: 42, Mission: domaingame.FleetMissionDestroy, TargetPlanetID: 100, Ships: domaingame.FleetCounts{domaingame.FleetDeathstar: 1}}
	call := func(repository FleetRepository) error {
		_, err := repository.finishMoonDestruction(ctx, "`fleet`", "`queue`", "`planets`", "`users`", "`messages`", fleetQueueTask{}, fleet, value, fleet.Ships, true)
		return err
	}

	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{domaingame.PlanetTypeMoon, 1})},
		{err: errors.New("destroy moon failed")},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	repository.combatRandom = func(int) int { return 0 }
	if err := call(repository); err == nil || !strings.Contains(err.Error(), "destroy moon failed") {
		t.Fatalf("expected destroy error, got %v", err)
	}

	for _, test := range []struct {
		name     string
		execErrs []error
		want     string
	}{
		{name: "fleet score", execErrs: []error{errors.New("fleet score failed")}, want: "score failed"},
		{name: "fleet rank", execErrs: []error{nil, errors.New("fleet rank failed")}, want: "rank failed"},
	} {
		runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{domaingame.PlanetTypeMoon, 100})}}}, execErrs: test.execErrs}
		repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
		rolls := []int{998, 0}
		repository.combatRandom = func(int) int { roll := rolls[0]; rolls = rolls[1:]; return roll }
		if err := call(repository); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("%s: expected %q, got %v", test.name, test.want, err)
		}
	}
	for failAt := 0; failAt < 2; failAt++ {
		execErrs := make([]error, failAt+1)
		execErrs[failAt] = fmt.Errorf("graviton message %d failed", failAt)
		runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{domaingame.PlanetTypeMoon, 10_000})}}}, execErrs: execErrs}
		repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
		repository.combatRandom = func(int) int { return 998 }
		if err := call(repository); err == nil || !strings.Contains(err.Error(), fmt.Sprintf("message %d failed", failAt)) {
			t.Fatalf("message %d: unexpected error %v", failAt, err)
		}
	}
}

func TestFleetRepositoryFinishDueMoonDestructionRedirectsReturn(t *testing.T) {
	messageRow := fleetMessageContextTestRow()
	messageRow[16] = domaingame.PlanetTypeMoon
	fleetRow := recallFleetTestRow(domaingame.FleetMissionDestroy, 0, map[int]int{domaingame.FleetDeathstar: 1})
	scoreRow := append([]any{43}, recalcPlanetScoreRow(nil, nil, nil)...)
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(fleetRow)},
		{rows: fakeRowsFromValues(messageRow)},
		{rows: fakeRowsFromValues(attackStateTestRow(nil, 0, 0, 0, 0, 0, 0))},
		{rows: fakeRowsFromValues([]any{0})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{domaingame.PlanetTypeMoon, 1})},
		{rows: fakeRowsFromValues(scoreRow)},
		{rows: fakeRowsFromValues([]any{200, 43})},
		{rows: fakeRowsFromValues()},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	rolls := []int{0, 998}
	repository.combatRandom = func(int) int {
		roll := rolls[0]
		rolls = rolls[1:]
		return roll
	}
	task := fleetQueueTask{TaskID: 55, OwnerID: 42, FleetID: 123, End: 1_700_000_000}

	err := repository.finishFleetQueueTask(
		context.Background(), "`ogame_uni`", "`ogame_fleet`", "`ogame_fleetlogs`", "`ogame_queue`",
		"`ogame_planets`", "`ogame_messages`", "`ogame_users`", "`ogame_exptab`", "`ogame_battledata`", "`ogame_union`", task,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(rolls) != 0 {
		t.Fatalf("expected both moon-destruction rolls, remaining=%v", rolls)
	}
	if !fleetExecsContainSQL(runner.execCalls, "DELETE FROM `ogame_planets`") {
		t.Fatalf("expected moon deletion: %+v", runner.execCalls)
	}
	returning := lastExecContaining(runner.execCalls, "INSERT INTO `ogame_fleet`")
	if returning == nil || returning.args[6] != domaingame.FleetMissionDestroy+domaingame.FleetMissionReturnOffset || returning.args[8] != 200 {
		t.Fatalf("expected destroy fleet to return to the underlying planet: %+v", returning)
	}
}

func TestFleetRepositoryMoonDestructionValidationPropagatesFromAttack(t *testing.T) {
	messageRow := fleetMessageContextTestRow()
	messageRow[16] = domaingame.PlanetTypeMoon
	fleet := recallFleetRow{
		ID: 123, OwnerID: 42, Mission: domaingame.FleetMissionDestroy,
		StartPlanetID: 99, TargetPlanetID: 100, FlightTime: 300,
		Ships: domaingame.FleetCounts{domaingame.FleetDeathstar: 1},
	}
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(messageRow)},
		{rows: fakeRowsFromValues(attackStateTestRow(nil, 0, 0, 0, 0, 0, 0))},
		{rows: fakeRowsFromValues([]any{0})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{domaingame.PlanetTypePlanet, 1})},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	repository.combatRandom = func(int) int { return 0 }
	task := fleetQueueTask{TaskID: 55, OwnerID: 42, FleetID: 123, End: 1_700_000_000}

	err := repository.finishAttackFleetArrival(
		context.Background(), "`ogame_uni`", "`ogame_fleet`", "`ogame_fleetlogs`", "`ogame_queue`",
		"`ogame_planets`", "`ogame_users`", "`ogame_messages`", "`ogame_battledata`", task, fleet,
	)
	if err == nil || !strings.Contains(err.Error(), "only moons can be destroyed") {
		t.Fatalf("expected moon target validation error, got %v", err)
	}
}

func fleetExecsContainSQL(calls []fakeFleetExecCall, needle string) bool {
	return fleetExecCountSQL(calls, needle) > 0
}

func fleetExecCountSQL(calls []fakeFleetExecCall, needle string) int {
	count := 0
	for _, call := range calls {
		if strings.Contains(call.sql, needle) {
			count++
		}
	}
	return count
}

func fleetExecsContainArg(calls []fakeFleetExecCall, needle string) bool {
	for _, call := range calls {
		for _, arg := range call.args {
			if strings.Contains(fmt.Sprint(arg), needle) {
				return true
			}
		}
	}
	return false
}

func fleetExecsContainArgs(calls []fakeFleetExecCall, sqlNeedle string, first any, second any) bool {
	for _, call := range calls {
		if strings.Contains(call.sql, sqlNeedle) && len(call.args) >= 2 && call.args[0] == first && call.args[1] == second {
			return true
		}
	}
	return false
}
