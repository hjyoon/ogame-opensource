package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestFleetRepositoryFinishesSuccessfulColonization(t *testing.T) {
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{legacyPlanetTypeColony, 1, 2, 4, "en"})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{1})},
		{rows: fakeRowsFromValues(colonySettingsTestRow())},
		{rows: fakeRowsFromValues(fleetMessageContextTestRow())},
	}}, execResults: []sql.Result{fakeFleetSQLResult(200)}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	rolls := []int{0, 9}
	repository.combatRandom = func(int) int { roll := rolls[0]; rolls = rolls[1:]; return roll }
	fleet := colonizationFleet(domaingame.FleetCounts{domaingame.FleetColonyShip: 1, domaingame.FleetSmallCargo: 2})

	if err := repository.finishColonizationArrival(context.Background(), "`ogame_fleet`", "`ogame_fleetlogs`", "`ogame_queue`", "`ogame_planets`", "`ogame_users`", "`ogame_messages`", colonizationTask(), fleet); err != nil {
		t.Fatal(err)
	}
	colony := firstExecContaining(runner.execCalls, "INSERT INTO `ogame_planets` (name,type,g,s,p")
	if colony == nil || colony.args[0] != "Colony" || colony.args[1] != domaingame.PlanetTypePlanet || colony.args[6] != 6_000 || colony.args[7] != 31 || colony.args[8] != 36 {
		t.Fatalf("unexpected colony insert: %+v", colony)
	}
	stats := firstExecContaining(runner.execCalls, "SET score1 = score1 -")
	if stats == nil || stats.args[0] != int64(40_000) || stats.args[1] != int64(1) || stats.args[3] != 42 {
		t.Fatalf("unexpected colony ship score adjustment: %+v", stats)
	}
	returning := firstExecContaining(runner.execCalls, "INSERT INTO `ogame_fleet`")
	if returning == nil || returning.args[6] != domaingame.FleetMissionColonize+domaingame.FleetMissionReturnOffset || returning.args[8] != 200 || fleetShipArgument(returning.args, domaingame.FleetColonyShip) != 0 || fleetShipArgument(returning.args, domaingame.FleetSmallCargo) != 2 {
		t.Fatalf("unexpected colonization return fleet: %+v", returning)
	}
	if firstExecContaining(runner.execCalls, "planet_id = ? AND type = ?") == nil || !fleetExecContains(runner.execCalls, "finds a new planet") {
		t.Fatalf("missing phantom cleanup or success report: %+v", runner.execCalls)
	}
}

func TestFleetRepositoryFinishesColonizationWithoutReturnFleet(t *testing.T) {
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{legacyPlanetTypeColony, 1, 2, 4, "de"})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{1})},
		{rows: fakeRowsFromValues(colonySettingsTestRow())},
	}}, execResults: []sql.Result{fakeFleetSQLResult(201)}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	repository.combatRandom = func(int) int { return 0 }

	err := repository.finishColonizationArrival(context.Background(), "`ogame_fleet`", "`ogame_fleetlogs`", "`ogame_queue`", "`ogame_planets`", "`ogame_users`", "`ogame_messages`", colonizationTask(), colonizationFleet(domaingame.FleetCounts{domaingame.FleetColonyShip: 1}))
	if err != nil {
		t.Fatal(err)
	}
	if firstExecContaining(runner.execCalls, "INSERT INTO `ogame_fleet`") != nil || !fleetExecContains(runner.execCalls, "Bericht der Siedler") {
		t.Fatalf("colony ship-only mission should finish without a return fleet: %+v", runner.execCalls)
	}
}

func TestFleetRepositoryHandlesColonizationCapacityAndRace(t *testing.T) {
	for _, test := range []struct {
		name          string
		queries       []fakeQueryResult
		wantTarget    int
		wantType      int
		wantMessage   string
		wantDeleteNow bool
	}{
		{
			name: "maximum planets",
			queries: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{legacyPlanetTypeColony, 1, 2, 4, "en"})},
				{rows: fakeRowsFromValues()},
				{rows: fakeRowsFromValues([]any{domaingame.MaxColonizedPlanets})},
				{rows: fakeRowsFromValues(fleetMessageContextTestRow())},
			},
			wantTarget: 300, wantType: legacyPlanetTypeAbandoned, wantMessage: "empire becomes too large", wantDeleteNow: true,
		},
		{
			name: "arrival race",
			queries: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{legacyPlanetTypeColony, 1, 2, 4, "en"})},
				{rows: fakeRowsFromValues([]any{777})},
				{rows: fakeRowsFromValues(fleetMessageContextTestRow())},
			},
			wantTarget: 100, wantMessage: "finds no planet suitable", wantDeleteNow: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: test.queries}, execResults: []sql.Result{fakeFleetSQLResult(test.wantTarget)}}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
			if err := repository.finishColonizationArrival(context.Background(), "`ogame_fleet`", "`ogame_fleetlogs`", "`ogame_queue`", "`ogame_planets`", "`ogame_users`", "`ogame_messages`", colonizationTask(), colonizationFleet(domaingame.FleetCounts{domaingame.FleetColonyShip: 1})); err != nil {
				t.Fatal(err)
			}
			returning := firstExecContaining(runner.execCalls, "INSERT INTO `ogame_fleet`")
			if returning == nil || returning.args[8] != test.wantTarget || fleetShipArgument(returning.args, domaingame.FleetColonyShip) != 1 || !fleetExecContains(runner.execCalls, test.wantMessage) {
				t.Fatalf("unexpected boundary return: %+v", runner.execCalls)
			}
			created := firstExecContaining(runner.execCalls, "INSERT INTO `ogame_planets` (name,type,g,s,p")
			if test.wantType != 0 && (created == nil || created.args[1] != test.wantType) {
				t.Fatalf("unexpected abandoned colony: %+v", created)
			}
			deleted := firstExecContaining(runner.execCalls, "planet_id = ? AND type = ?") != nil
			if deleted != test.wantDeleteNow {
				t.Fatalf("phantom cleanup=%v, want %v", deleted, test.wantDeleteNow)
			}
		})
	}
}

func TestFleetRepositoryCleansColonizationPhantomOnReturn(t *testing.T) {
	runner := &fakeFleetRunner{}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	fleet := colonizationFleet(domaingame.FleetCounts{domaingame.FleetColonyShip: 1})
	fleet.Mission = domaingame.FleetMissionColonize + domaingame.FleetMissionReturnOffset

	if err := repository.finishReturningFleetArrival(context.Background(), "`ogame_fleet`", "`ogame_queue`", "`ogame_planets`", "`ogame_users`", "`ogame_messages`", colonizationTask(), fleet); err != nil {
		t.Fatal(err)
	}
	cleanup := firstExecContaining(runner.execCalls, "planet_id = ? AND type = ?")
	if cleanup == nil || cleanup.args[0] != 100 || cleanup.args[1] != legacyPlanetTypeColony {
		t.Fatalf("missing colonization phantom cleanup: %+v", runner.execCalls)
	}
}

func TestFleetRepositoryUpdatesQueuePlanetProduction(t *testing.T) {
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{42})},
		{rows: fakeRowsFromValues(resourceUpdatePlanetRow(resourceUpdatePlanetFixture{lastPeek: 1000, metal: 500, crystal: 500}))},
		{rows: fakeRowsFromValues(resourceUpdateUserRow(42, 0, 0, 0))},
		{rows: fakeRowsFromValues([]any{128.0})},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(1300, 0) })
	found, err := repository.updateFleetQueuePlanetResources(context.Background(), "`ogame_users`", "`ogame_planets`", 99, 1300)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if len(runner.execCalls) != 1 || math.Abs(runner.execCalls[0].args[0].(float64)-713.3333333333334) > 0.000001 || runner.execCalls[0].args[3] != 1300 {
		t.Fatalf("unexpected queue production write: %+v", runner.execCalls)
	}

	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	if found, err := repository.updateFleetQueuePlanetResources(context.Background(), "`ogame_users`", "`ogame_planets`", 404, 1300); err != nil || found {
		t.Fatalf("missing queue planet should be preserved, found=%v err=%v", found, err)
	}
}

func TestColonizationMessagesMatchLegacyLocales(t *testing.T) {
	for language, want := range map[string]string{"en": "Settlers' report", "de": "Bericht der Siedler", "fr": "Доклад поселенцев", "ru": "Доклад поселенцев", "it": "Rapporto colonizzazione", "es": "Settlers' report"} {
		message := colonizationMessageFor(language)
		if message.Subject != want || message.Prefix == "" || message.Success == "" || message.Maximum == "" || message.Failure == "" {
			t.Fatalf("language %s: %+v", language, message)
		}
	}
	for language, want := range map[string]string{"de": "Kolonie", "fr": "Colonie", "es": "Colonia", "it": "Colonia", "ru": "Колония", "jp": "コロニー", "unknown": "Colony"} {
		if got := colonyName(language); got != want {
			t.Fatalf("language %s colony name=%q, want %q", language, got, want)
		}
	}
}

func TestColonizationLoadHelpersReturnErrors(t *testing.T) {
	fleet := colonizationFleet(nil)
	for _, test := range []struct {
		name   string
		result fakeQueryResult
		run    func(FleetRepository) error
	}{
		{name: "target query", result: fakeQueryResult{err: errors.New("target query failed")}, run: func(repository FleetRepository) error {
			_, _, err := repository.loadColonizationTarget(context.Background(), "`ogame_planets`", "`ogame_users`", fleet)
			return err
		}},
		{name: "target rows", result: fakeQueryResult{rows: fakeRowsError(errors.New("target rows failed"))}, run: func(repository FleetRepository) error {
			_, _, err := repository.loadColonizationTarget(context.Background(), "`ogame_planets`", "`ogame_users`", fleet)
			return err
		}},
		{name: "target scan", result: fakeQueryResult{rows: fakeRowsFromValues([]any{1})}, run: func(repository FleetRepository) error {
			_, _, err := repository.loadColonizationTarget(context.Background(), "`ogame_planets`", "`ogame_users`", fleet)
			return err
		}},
		{name: "target trailer", result: fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("target trailer failed"), []any{legacyPlanetTypeColony, 1, 2, 3, "en"})}, run: func(repository FleetRepository) error {
			_, _, err := repository.loadColonizationTarget(context.Background(), "`ogame_planets`", "`ogame_users`", fleet)
			return err
		}},
		{name: "count query", result: fakeQueryResult{err: errors.New("count query failed")}, run: func(repository FleetRepository) error {
			_, err := repository.loadColonizedPlanetCount(context.Background(), "`ogame_planets`", 42)
			return err
		}},
		{name: "count rows", result: fakeQueryResult{rows: fakeRowsError(errors.New("count rows failed"))}, run: func(repository FleetRepository) error {
			_, err := repository.loadColonizedPlanetCount(context.Background(), "`ogame_planets`", 42)
			return err
		}},
		{name: "count missing", result: fakeQueryResult{rows: fakeRowsFromValues()}, run: func(repository FleetRepository) error {
			_, err := repository.loadColonizedPlanetCount(context.Background(), "`ogame_planets`", 42)
			return err
		}},
		{name: "count scan", result: fakeQueryResult{rows: fakeRowsFromValues([]any{"bad"})}, run: func(repository FleetRepository) error {
			_, err := repository.loadColonizedPlanetCount(context.Background(), "`ogame_planets`", 42)
			return err
		}},
		{name: "count trailer", result: fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("count trailer failed"), []any{1})}, run: func(repository FleetRepository) error {
			_, err := repository.loadColonizedPlanetCount(context.Background(), "`ogame_planets`", 42)
			return err
		}},
		{name: "settings query", result: fakeQueryResult{err: errors.New("settings query failed")}, run: func(repository FleetRepository) error {
			_, err := repository.loadColonySettings(context.Background(), "`ogame_coltab`")
			return err
		}},
		{name: "settings rows", result: fakeQueryResult{rows: fakeRowsError(errors.New("settings rows failed"))}, run: func(repository FleetRepository) error {
			_, err := repository.loadColonySettings(context.Background(), "`ogame_coltab`")
			return err
		}},
		{name: "settings missing", result: fakeQueryResult{rows: fakeRowsFromValues()}, run: func(repository FleetRepository) error {
			_, err := repository.loadColonySettings(context.Background(), "`ogame_coltab`")
			return err
		}},
		{name: "settings scan", result: fakeQueryResult{rows: fakeRowsFromValues([]any{1})}, run: func(repository FleetRepository) error {
			_, err := repository.loadColonySettings(context.Background(), "`ogame_coltab`")
			return err
		}},
		{name: "settings trailer", result: fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("settings trailer failed"), colonySettingsTestRow())}, run: func(repository FleetRepository) error {
			_, err := repository.loadColonySettings(context.Background(), "`ogame_coltab`")
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{test.result}}}
			if err := test.run(NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)); err == nil {
				t.Fatal("expected helper error")
			}
		})
	}

	repository := NewFleetRepositoryWithRunner(&fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}, nil, "ogame_", nil)
	if _, found, err := repository.loadColonizationTarget(context.Background(), "`ogame_planets`", "`ogame_users`", fleet); err != nil || found {
		t.Fatalf("missing target found=%v err=%v", found, err)
	}
}

func TestColonizationCreationReturnsErrors(t *testing.T) {
	coordinates := domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3}
	tests := []struct {
		name   string
		prefix string
		runner *fakeFleetRunner
		random func(int) int
		run    func(FleetRepository, *fakeFleetRunner) error
	}{
		{name: "random missing", prefix: "ogame_", runner: &fakeFleetRunner{}, random: nil, run: func(repository FleetRepository, _ *fakeFleetRunner) error {
			_, err := repository.createFleetColony(context.Background(), "`ogame_planets`", 42, coordinates, "en", 1000)
			return err
		}},
		{name: "invalid prefix", prefix: "bad-prefix_", runner: &fakeFleetRunner{}, random: func(int) int { return 0 }, run: func(repository FleetRepository, _ *fakeFleetRunner) error {
			_, err := repository.createFleetColony(context.Background(), "`ogame_planets`", 42, coordinates, "en", 1000)
			return err
		}},
		{name: "settings", prefix: "ogame_", runner: &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("settings failed")}}}}, random: func(int) int { return 0 }, run: func(repository FleetRepository, _ *fakeFleetRunner) error {
			_, err := repository.createFleetColony(context.Background(), "`ogame_planets`", 42, coordinates, "en", 1000)
			return err
		}},
		{name: "colony insert", prefix: "ogame_", runner: &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(colonySettingsTestRow())}}}, execErr: errors.New("insert failed")}, random: func(int) int { return 0 }, run: func(repository FleetRepository, _ *fakeFleetRunner) error {
			_, err := repository.createFleetColony(context.Background(), "`ogame_planets`", 42, coordinates, "en", 1000)
			return err
		}},
		{name: "colony id", prefix: "ogame_", runner: &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(colonySettingsTestRow())}}}, execResults: []sql.Result{fakeFleetSQLErrorResult{idErr: errors.New("id failed")}}}, random: func(int) int { return 0 }, run: func(repository FleetRepository, _ *fakeFleetRunner) error {
			_, err := repository.createFleetColony(context.Background(), "`ogame_planets`", 42, coordinates, "en", 1000)
			return err
		}},
		{name: "colony zero id", prefix: "ogame_", runner: &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(colonySettingsTestRow())}}}, execResults: []sql.Result{fakeFleetSQLResult(0)}}, random: func(int) int { return 0 }, run: func(repository FleetRepository, _ *fakeFleetRunner) error {
			_, err := repository.createFleetColony(context.Background(), "`ogame_planets`", 42, coordinates, "en", 1000)
			return err
		}},
		{name: "abandoned insert", prefix: "ogame_", runner: &fakeFleetRunner{execErr: errors.New("abandoned failed")}, random: func(int) int { return 0 }, run: func(repository FleetRepository, _ *fakeFleetRunner) error {
			_, err := repository.createAbandonedColony(context.Background(), "`ogame_planets`", coordinates, 1000)
			return err
		}},
		{name: "abandoned id", prefix: "ogame_", runner: &fakeFleetRunner{execResults: []sql.Result{fakeFleetSQLErrorResult{idErr: errors.New("id failed")}}}, random: func(int) int { return 0 }, run: func(repository FleetRepository, _ *fakeFleetRunner) error {
			_, err := repository.createAbandonedColony(context.Background(), "`ogame_planets`", coordinates, 1000)
			return err
		}},
		{name: "abandoned zero id", prefix: "ogame_", runner: &fakeFleetRunner{execResults: []sql.Result{fakeFleetSQLResult(0)}}, random: func(int) int { return 0 }, run: func(repository FleetRepository, _ *fakeFleetRunner) error {
			_, err := repository.createAbandonedColony(context.Background(), "`ogame_planets`", coordinates, 1000)
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := NewFleetRepositoryWithRunner(test.runner, test.runner, test.prefix, nil)
			repository.combatRandom = test.random
			if err := test.run(repository, test.runner); err == nil {
				t.Fatal("expected creation error")
			}
		})
	}
}

func TestColonizationArrivalRejectsInvalidState(t *testing.T) {
	settings := fakeQueryResult{rows: fakeRowsFromValues(colonySettingsTestRow())}
	for _, test := range []struct {
		name     string
		results  []fakeQueryResult
		execErrs []error
		want     string
	}{
		{name: "target query", results: []fakeQueryResult{{err: errors.New("target failed")}}, want: "target failed"},
		{name: "target missing", results: []fakeQueryResult{{rows: fakeRowsFromValues()}}, want: "target unavailable"},
		{name: "target wrong type", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{domaingame.PlanetTypePlanet, 1, 2, 4, "en"})}}, want: "target unavailable"},
		{name: "occupied query", results: []fakeQueryResult{colonizationTargetQueryResult(), {err: errors.New("occupied failed")}}, want: "occupied failed"},
		{name: "planet count", results: []fakeQueryResult{colonizationTargetQueryResult(), {rows: fakeRowsFromValues()}, {err: errors.New("count failed")}}, want: "count failed"},
		{name: "colony insert", results: []fakeQueryResult{colonizationTargetQueryResult(), {rows: fakeRowsFromValues()}, {rows: fakeRowsFromValues([]any{1})}, settings}, execErrs: []error{errors.New("colony insert failed")}, want: "colony insert failed"},
		{name: "abandoned insert", results: []fakeQueryResult{colonizationTargetQueryResult(), {rows: fakeRowsFromValues()}, {rows: fakeRowsFromValues([]any{domaingame.MaxColonizedPlanets})}}, execErrs: []error{errors.New("abandoned insert failed")}, want: "abandoned insert failed"},
		{name: "return context missing", results: []fakeQueryResult{colonizationTargetQueryResult(), {rows: fakeRowsFromValues()}, {rows: fakeRowsFromValues([]any{domaingame.MaxColonizedPlanets})}, {rows: fakeRowsFromValues()}}, want: "return context unavailable"},
		{name: "return context query", results: []fakeQueryResult{colonizationTargetQueryResult(), {rows: fakeRowsFromValues()}, {rows: fakeRowsFromValues([]any{domaingame.MaxColonizedPlanets})}, {err: errors.New("context failed")}}, want: "context failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: test.results}, execErrs: test.execErrs, execResults: []sql.Result{fakeFleetSQLResult(300)}}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
			repository.combatRandom = func(int) int { return 0 }
			err := repository.finishColonizationArrival(context.Background(), "`ogame_fleet`", "`ogame_fleetlogs`", "`ogame_queue`", "`ogame_planets`", "`ogame_users`", "`ogame_messages`", colonizationTask(), colonizationFleet(domaingame.FleetCounts{domaingame.FleetColonyShip: 1}))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q, got %v", test.want, err)
			}
		})
	}
}

func TestColonizationArrivalReturnsWriteErrors(t *testing.T) {
	for _, test := range []struct {
		name     string
		results  []fakeQueryResult
		execErrs []error
		want     string
	}{
		{name: "score", results: colonizationSuccessResults(false), execErrs: []error{nil, errors.New("score failed")}, want: "score failed"},
		{name: "rank", results: colonizationSuccessResults(false), execErrs: []error{nil, nil, errors.New("rank failed")}, want: "rank failed"},
		{name: "phantom delete", results: colonizationMaximumResults(false), execErrs: []error{nil, errors.New("delete failed")}, want: "delete failed"},
		{name: "return fleet", results: colonizationMaximumResults(false), execErrs: []error{nil, nil, errors.New("return fleet failed")}, want: "return fleet failed"},
		{name: "return queue", results: colonizationMaximumResults(false), execErrs: []error{nil, nil, nil, errors.New("return queue failed")}, want: "return queue failed"},
		{name: "transition log", results: colonizationMaximumResults(true), execErrs: []error{nil, nil, nil, nil, errors.New("log failed")}, want: "log failed"},
		{name: "message", results: colonizationMaximumResults(true), execErrs: []error{nil, nil, nil, nil, nil, errors.New("message failed")}, want: "message failed"},
		{name: "cleanup", results: colonizationMaximumResults(true), execErrs: []error{nil, nil, nil, nil, nil, nil, errors.New("cleanup failed")}, want: "cleanup failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: test.results}, execErrs: test.execErrs, execResults: []sql.Result{fakeFleetSQLResult(300)}}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
			repository.combatRandom = func(int) int { return 0 }
			err := repository.finishColonizationArrival(context.Background(), "`ogame_fleet`", "`ogame_fleetlogs`", "`ogame_queue`", "`ogame_planets`", "`ogame_users`", "`ogame_messages`", colonizationTask(), colonizationFleet(domaingame.FleetCounts{domaingame.FleetColonyShip: 1}))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q, got %v", test.want, err)
			}
		})
	}
}

func TestFleetQueueProductionBoundaries(t *testing.T) {
	for _, result := range []fakeQueryResult{
		{err: errors.New("owner failed")},
		{rows: fakeRowsError(errors.New("owner rows failed"))},
		{rows: fakeRowsFromValues([]any{"bad"})},
		{rows: fakeRowsFromValuesWithErr(errors.New("owner trailer failed"), []any{42})},
	} {
		runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{result}}}
		repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
		if _, err := repository.updateFleetQueuePlanetResources(context.Background(), "`ogame_users`", "`ogame_planets`", 99, 1300); err == nil {
			t.Fatal("expected owner lookup error")
		}
	}

	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(recallFleetTestRow(domaingame.FleetMissionTransport, 0, nil))},
		{rows: fakeRowsFromValues()},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	repository.queueProduction = true
	err := repository.finishFleetQueueTask(context.Background(), "`ogame_uni`", "`ogame_fleet`", "`ogame_fleetlogs`", "`ogame_queue`", "`ogame_planets`", "`ogame_messages`", "`ogame_users`", "`ogame_exptab`", "`ogame_battledata`", "`ogame_union`", colonizationTask())
	if err != nil || len(runner.execCalls) != 0 {
		t.Fatalf("missing origin must preserve queue, calls=%+v err=%v", runner.execCalls, err)
	}
}

func colonizationFleet(ships domaingame.FleetCounts) recallFleetRow {
	return recallFleetRow{ID: 123, OwnerID: 42, Mission: domaingame.FleetMissionColonize, StartPlanetID: 99, TargetPlanetID: 100, FlightTime: 300, Fuel: 100, Ships: ships}
}

func colonizationTask() fleetQueueTask {
	return fleetQueueTask{TaskID: 55, OwnerID: 42, FleetID: 123, End: 1_700_000_000}
}

func colonySettingsTestRow() []any {
	return []any{50, 120, 72, 50, 150, 120, 50, 120, 120, 50, 120, 96, 50, 150, 96}
}

func colonizationTargetQueryResult() fakeQueryResult {
	return fakeQueryResult{rows: fakeRowsFromValues([]any{legacyPlanetTypeColony, 1, 2, 4, "en"})}
}

func colonizationSuccessResults(withContext bool) []fakeQueryResult {
	results := []fakeQueryResult{
		colonizationTargetQueryResult(),
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{1})},
		{rows: fakeRowsFromValues(colonySettingsTestRow())},
	}
	if withContext {
		results = append(results, fakeQueryResult{rows: fakeRowsFromValues(fleetMessageContextTestRow())})
	}
	return results
}

func colonizationMaximumResults(withContext bool) []fakeQueryResult {
	results := []fakeQueryResult{
		colonizationTargetQueryResult(),
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{domaingame.MaxColonizedPlanets})},
	}
	if withContext {
		results = append(results, fakeQueryResult{rows: fakeRowsFromValues(fleetMessageContextTestRow())})
	}
	return results
}

func fleetShipArgument(args []any, shipID int) int {
	for index, id := range domaingame.FleetIDs() {
		if id == shipID {
			return args[11+index].(int)
		}
	}
	return -1
}

func fleetExecContains(calls []fakeFleetExecCall, needle string) bool {
	for _, call := range calls {
		for _, argument := range call.args {
			if value, ok := argument.(string); ok && strings.Contains(value, needle) {
				return true
			}
		}
	}
	return false
}
