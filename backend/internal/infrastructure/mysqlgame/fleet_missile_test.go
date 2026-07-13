package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestFleetRepositoryFinishesMissileArrival(t *testing.T) {
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(missilePlanetTestRow(99, 42, domaingame.PlanetTypePlanet, "Origin", "en", 0, 0, nil))},
		{rows: fakeRowsFromValues(missilePlanetTestRow(100, 43, domaingame.PlanetTypePlanet, "Target", "en", 0, 0, domaingame.DefenseCounts{
			domaingame.DefenseRocketLauncher: 10, domaingame.DefenseLightLaser: 5, domaingame.DefenseAntiBallisticMissile: 5,
		}))},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	fleet := recallFleetRow{ID: 123, OwnerID: 42, Mission: domaingame.FleetMissionMissile, StartPlanetID: 99, TargetPlanetID: 100, MissileAmount: 3, MissileTarget: domaingame.DefenseRocketLauncher}

	if err := repository.finishMissileArrival(context.Background(), "`ogame_fleet`", "`ogame_queue`", "`ogame_planets`", "`ogame_users`", "`ogame_messages`", colonizationTask(), fleet); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 13 {
		t.Fatalf("unexpected missile lifecycle writes: %+v", runner.execCalls)
	}
	defense := runner.execCalls[0]
	if defense.args[0] != 10 || defense.args[1] != 5 || defense.args[8] != 2 || defense.args[10] != int64(1_700_000_000) || defense.args[11] != 100 {
		t.Fatalf("unexpected intercepted defense update: %+v", defense)
	}
	defenderMessage := runner.execCalls[9]
	attackerMessage := runner.execCalls[10]
	if defenderMessage.args[0] != 43 || defenderMessage.args[1] != domaingame.MessageTypeBattleReportLink || !strings.Contains(defenderMessage.args[4].(string), "3 missile(s) was destroyed by your interceptor missiles") || !strings.Contains(defenderMessage.args[4].(string), "Rocket Launcher</td><td>10") {
		t.Fatalf("unexpected defender report: %+v", defenderMessage)
	}
	if attackerMessage.args[0] != 42 || !strings.Contains(attackerMessage.args[4].(string), "3 missile(s) from your planet") {
		t.Fatalf("unexpected attacker report: %+v", attackerMessage)
	}
}

func TestFleetRepositoryMissileMoonUsesPlanetInterceptors(t *testing.T) {
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(missilePlanetTestRow(99, 42, domaingame.PlanetTypePlanet, "Origin", "en", 0, 0, nil))},
		{rows: fakeRowsFromValues(missilePlanetTestRow(101, 43, domaingame.PlanetTypeMoon, "Moon", "en", 0, 0, domaingame.DefenseCounts{domaingame.DefenseRocketLauncher: 20}))},
		{rows: fakeRowsFromValues(missilePlanetTestRow(100, 43, domaingame.PlanetTypePlanet, "Target", "en", 0, 0, domaingame.DefenseCounts{domaingame.DefenseAntiBallisticMissile: 1}))},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	fleet := recallFleetRow{ID: 123, OwnerID: 42, Mission: domaingame.FleetMissionMissile, StartPlanetID: 99, TargetPlanetID: 101, MissileAmount: 2, MissileTarget: domaingame.DefenseRocketLauncher}

	if err := repository.finishMissileArrival(context.Background(), "`ogame_fleet`", "`ogame_queue`", "`ogame_planets`", "`ogame_users`", "`ogame_messages`", colonizationTask(), fleet); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 14 || runner.execCalls[0].args[0] != 0 || runner.execCalls[1].args[8] != 0 || len(runner.execCalls[1].args) != 11 {
		t.Fatalf("unexpected moon/planet defense updates: %+v", runner.execCalls[:2])
	}
}

func TestFleetRepositoryLoadsMissileMetadata(t *testing.T) {
	row := recallFleetTestRow(domaingame.FleetMissionMissile, 0, nil)
	row[12] = 3
	row[13] = domaingame.DefensePlasmaTurret
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(row)}}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	fleet, found, err := repository.loadRecallFleetAnyOwner(context.Background(), "`ogame_fleet`", 123)
	if err != nil || !found || fleet.MissileAmount != 3 || fleet.MissileTarget != domaingame.DefensePlasmaTurret {
		t.Fatalf("unexpected missile metadata: found=%v fleet=%+v err=%v", found, fleet, err)
	}
}

func TestFleetRepositoryDispatchesMissileQueueTask(t *testing.T) {
	row := recallFleetTestRow(domaingame.FleetMissionMissile, 0, nil)
	row[12] = 3
	row[13] = domaingame.DefenseRocketLauncher
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: append(
		[]fakeQueryResult{{rows: fakeRowsFromValues(row)}}, missileArrivalResults(false)...,
	)}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	err := repository.finishFleetQueueTask(context.Background(), "`ogame_uni`", "`ogame_fleet`", "`ogame_fleetlogs`", "`ogame_queue`", "`ogame_planets`", "`ogame_messages`", "`ogame_users`", "`ogame_exptab`", "`ogame_battledata`", "`ogame_union`", colonizationTask())
	if err != nil || len(runner.execCalls) != 13 {
		t.Fatalf("unexpected missile queue dispatch: calls=%d err=%v", len(runner.execCalls), err)
	}
}

func TestFleetRepositoryMissilePlanetLoadBoundaries(t *testing.T) {
	valid := missilePlanetTestRow(99, 42, domaingame.PlanetTypePlanet, "Origin", "en", 0, 0, nil)
	for _, loader := range []struct {
		name string
		run  func(FleetRepository) (bool, error)
	}{
		{name: "direct", run: func(repository FleetRepository) (bool, error) {
			_, found, err := repository.loadMissilePlanet(context.Background(), "`planets`", "`users`", 99)
			return found, err
		}},
		{name: "moon defender", run: func(repository FleetRepository) (bool, error) {
			_, found, err := repository.loadMissileDefendingPlanet(context.Background(), "`planets`", "`users`", missilePlanetState{Galaxy: 1, System: 2, Position: 3})
			return found, err
		}},
	} {
		for _, test := range []struct {
			name      string
			result    fakeQueryResult
			wantFound bool
			wantErr   bool
		}{
			{name: "query", result: fakeQueryResult{err: errors.New("missile query failed")}, wantErr: true},
			{name: "missing", result: fakeQueryResult{rows: fakeRowsFromValues()}},
			{name: "empty rows error", result: fakeQueryResult{rows: fakeRowsError(errors.New("missile rows failed"))}, wantErr: true},
			{name: "scan", result: fakeQueryResult{rows: fakeRowsFromValues([]any{1})}, wantErr: true},
			{name: "trailer", result: fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("missile trailer failed"), valid)}, wantFound: true, wantErr: true},
			{name: "found", result: fakeQueryResult{rows: fakeRowsFromValues(valid)}, wantFound: true},
		} {
			t.Run(loader.name+"/"+test.name, func(t *testing.T) {
				runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{test.result}}}
				found, err := loader.run(NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil))
				if (err != nil) != test.wantErr || found != test.wantFound {
					t.Fatalf("found=%v err=%v", found, err)
				}
			})
		}
	}
}

func TestFleetRepositoryMissileArrivalErrors(t *testing.T) {
	for _, test := range []struct {
		name     string
		results  []fakeQueryResult
		execErrs []error
		want     string
	}{
		{name: "origin query", results: []fakeQueryResult{{err: errors.New("origin query failed")}}, want: "origin query failed"},
		{name: "origin missing", results: []fakeQueryResult{{rows: fakeRowsFromValues()}}, want: "missile origin unavailable"},
		{name: "target query", results: []fakeQueryResult{missileOriginResult(), {err: errors.New("target query failed")}}, want: "target query failed"},
		{name: "target missing", results: []fakeQueryResult{missileOriginResult(), {rows: fakeRowsFromValues()}}, want: "missile target unavailable"},
		{name: "defender query", results: append(missileArrivalResults(true)[:2], fakeQueryResult{err: errors.New("defender query failed")}), want: "defender query failed"},
		{name: "defender missing", results: append(missileArrivalResults(true)[:2], fakeQueryResult{rows: fakeRowsFromValues()}), want: "missile defending planet unavailable"},
		{name: "target update", results: missileArrivalResults(false), execErrs: []error{errors.New("target update failed")}, want: "target update failed"},
		{name: "defender update", results: missileArrivalResults(true), execErrs: []error{nil, errors.New("defender update failed")}, want: "defender update failed"},
		{name: "rank", results: missileArrivalResults(false), execErrs: []error{nil, errors.New("rank failed")}, want: "rank failed"},
		{name: "defender report", results: missileArrivalResults(false), execErrs: missileExecFailure(9, "defender report failed"), want: "defender report failed"},
		{name: "attacker report", results: missileArrivalResults(false), execErrs: missileExecFailure(10, "attacker report failed"), want: "attacker report failed"},
		{name: "fleet cleanup", results: missileArrivalResults(false), execErrs: missileExecFailure(11, "fleet cleanup failed"), want: "fleet cleanup failed"},
		{name: "queue cleanup", results: missileArrivalResults(false), execErrs: missileExecFailure(12, "queue cleanup failed"), want: "queue cleanup failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: test.results}, execErrs: test.execErrs}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
			fleet := recallFleetRow{ID: 123, OwnerID: 42, Mission: domaingame.FleetMissionMissile, StartPlanetID: 99, TargetPlanetID: 100, MissileAmount: 3, MissileTarget: domaingame.DefenseRocketLauncher}
			if test.name == "defender query" || test.name == "defender missing" || test.name == "defender update" {
				fleet.TargetPlanetID = 101
			}
			err := repository.finishMissileArrival(context.Background(), "`fleet`", "`queue`", "`planets`", "`users`", "`messages`", colonizationTask(), fleet)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q, got %v", test.want, err)
			}
		})
	}
}

func TestMissileReportLocales(t *testing.T) {
	for language, subject := range map[string]string{"en": "Missile attack", "de": "Raketenangriff", "fr": "Ракетная атака", "ru": "Ракетная атака", "it": "Attacco missilistico", "es": "Missile attack"} {
		locale := missileLocaleFor(language)
		if locale.Subject != subject || locale.From == "" || locale.Title == "" || missileDefenseName(language, domaingame.DefenseRocketLauncher) == "" {
			t.Fatalf("language %s: %+v", language, locale)
		}
	}
	report := missileDefenseReport("en", domaingame.DefenseCounts{domaingame.DefenseRocketLauncher: 1, domaingame.DefenseInterplanetaryMissile: 2}, nil, false)
	if !strings.Contains(report, "Interplanetary Missiles</td><td>2") || !strings.Contains(report, "Rocket Launcher</td><td>1") {
		t.Fatalf("unexpected reverse-order report: %s", report)
	}
	moonReport := missileDefenseReport("en", domaingame.DefenseCounts{domaingame.DefenseAntiBallisticMissile: 2}, domaingame.DefenseCounts{domaingame.DefenseAntiBallisticMissile: 7}, true)
	if !strings.Contains(moonReport, "Anti-Ballistic Missiles</td><td>7") {
		t.Fatalf("moon report must use planet interceptor count: %s", moonReport)
	}
}

func missileOriginResult() fakeQueryResult {
	return fakeQueryResult{rows: fakeRowsFromValues(missilePlanetTestRow(99, 42, domaingame.PlanetTypePlanet, "Origin", "en", 0, 0, nil))}
}

func missileArrivalResults(moon bool) []fakeQueryResult {
	results := []fakeQueryResult{
		missileOriginResult(),
		{rows: fakeRowsFromValues(missilePlanetTestRow(100, 43, domaingame.PlanetTypePlanet, "Target", "en", 0, 0, domaingame.DefenseCounts{domaingame.DefenseRocketLauncher: 10}))},
	}
	if moon {
		results[1] = fakeQueryResult{rows: fakeRowsFromValues(missilePlanetTestRow(101, 43, domaingame.PlanetTypeMoon, "Moon", "en", 0, 0, domaingame.DefenseCounts{domaingame.DefenseRocketLauncher: 10}))}
		results = append(results, fakeQueryResult{rows: fakeRowsFromValues(missilePlanetTestRow(100, 43, domaingame.PlanetTypePlanet, "Target", "en", 0, 0, domaingame.DefenseCounts{domaingame.DefenseAntiBallisticMissile: 1}))})
	}
	return results
}

func missileExecFailure(index int, message string) []error {
	errs := make([]error, index+1)
	errs[index] = errors.New(message)
	return errs
}

func missilePlanetTestRow(id int, ownerID int, planetType int, name string, language string, weapon int, armour int, defense domaingame.DefenseCounts) []any {
	row := []any{id, ownerID, planetType, name, 1, 2, id - 96, language, weapon, armour}
	for _, defenseID := range domaingame.DefenseIDs() {
		row = append(row, defense[defenseID])
	}
	return row
}
