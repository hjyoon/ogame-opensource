package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestExpeditionPresentationCoversEventStatesAndLocales(t *testing.T) {
	expeditionLocaleCache = syncMapForTest()
	outcomes := []domaingame.ExpeditionOutcome{
		{Event: domaingame.ExpeditionNothing},
		{Event: domaingame.ExpeditionAliens, Tier: 2},
		{Event: domaingame.ExpeditionPirates, Tier: 1},
		{Event: domaingame.ExpeditionDarkMatter, Tier: 0, DarkMatter: 100},
		{Event: domaingame.ExpeditionBlackHole},
		{Event: domaingame.ExpeditionDelay},
		{Event: domaingame.ExpeditionAccel},
		{Event: domaingame.ExpeditionResources, Tier: 1, ResourceType: 1, ResourceAmount: 100, CargoLimited: true},
		{Event: domaingame.ExpeditionFleet, Tier: 2, FoundFleet: domaingame.FleetCounts{domaingame.FleetSmallCargo: 2}, FoundFleetOrder: []int{domaingame.FleetSmallCargo}, CargoLimited: true},
		{Event: domaingame.ExpeditionTrader},
	}
	for _, outcome := range outcomes {
		outcome.HasLogbook = true
		outcome.LogbookTier = int(outcome.Event) % 4
		if text := expeditionMessageText("en", outcome); text == "" {
			t.Fatalf("empty message for event %d", outcome.Event)
		}
	}
	if expeditionTierName(0, "low", "medium", "high") != "low" || expeditionTierName(1, "low", "medium", "high") != "medium" || expeditionTierName(2, "low", "medium", "high") != "high" {
		t.Fatal("unexpected expedition tier labels")
	}
	if expeditionFormat("#1/#2", "a", 2) != "a/2" {
		t.Fatal("unexpected expedition formatting")
	}
	for _, language := range []string{"de", "en", "es", "fr", "it", "jp", "ru", "invalid"} {
		if expeditionLanguage(language) == "" || expeditionResourceName(language, -1) == "" || expeditionResourceName(language, 4) == "" {
			t.Fatalf("missing locale fallback for %q", language)
		}
	}
	if expeditionLocaleValue("invalid", "FLEET_MESSAGE_FROM") == "" || expeditionLocaleValue("en", "MISSING_EXPEDITION_KEY") != "MISSING_EXPEDITION_KEY" {
		t.Fatal("unexpected locale value fallback")
	}
	_ = cachedExpeditionLocale("en")
}

func TestExpeditionBattleMessagePersistenceErrors(t *testing.T) {
	value := fleetMessageContext{OriginOwnerID: 42, TargetGalaxy: 1, TargetSystem: 2, TargetPosition: 3}
	target := expeditionTargetState{Language: "en"}
	result := domaingame.CombatResult{Outcome: domaingame.CombatDefenderWon}
	writeback := domaingame.CombatWriteback{
		AttackerLosses: []domaingame.CombatParticipantLoss{{Points: 10}},
		DefenderLosses: []domaingame.CombatParticipantLoss{{Points: 20}},
	}
	for _, test := range []struct {
		name    string
		runner  *fakeFleetRunner
		wantErr string
	}{
		{name: "insert", runner: &fakeFleetRunner{execErrs: []error{errors.New("insert failed")}}, wantErr: "insert failed"},
		{name: "id", runner: &fakeFleetRunner{execResults: []sql.Result{fakeFleetSQLErrorResult{idErr: errors.New("id failed")}}}, wantErr: "id failed"},
		{name: "zero id", runner: &fakeFleetRunner{execResults: []sql.Result{fakeFleetSQLResult(0)}}, wantErr: "id unavailable"},
		{name: "link", runner: &fakeFleetRunner{execErrs: []error{nil, errors.New("link failed")}}, wantErr: "link failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := NewFleetRepositoryWithRunner(test.runner, test.runner, "ogame_", nil)
			err := repository.insertExpeditionBattleMessages(context.Background(), "`ogame_messages`", 7, value, target, "report", result, writeback, 100)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("expected %q, got %v", test.wantErr, err)
			}
		})
	}
	runner := &fakeFleetRunner{}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.insertExpeditionBattleMessages(context.Background(), "`ogame_messages`", 7, value, target, "report", result, writeback, 100); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 2 || !strings.Contains(stringArg(runner.execCalls[0].args, 4), "<!--A:10,W:20-->") {
		t.Fatalf("unexpected battle message writes: %+v", runner.execCalls)
	}
	if err := repository.updateExpeditionBattleData(context.Background(), "`ogame_battledata`", 7, value, "en", "report", result, writeback); err != nil {
		t.Fatal(err)
	}
	if got := localizedBattleReportLink(7, value, "style", false, 20, 10, "Localized"); !strings.Contains(got, "Localized") {
		t.Fatalf("localized link=%q", got)
	}
}

func TestFinishExpeditionBattleStopsOnPersistenceErrors(t *testing.T) {
	fleet := recallFleetRow{
		ID: 7, OwnerID: 42, StartPlanetID: 100, TargetPlanetID: 200,
		FlightTime: 120, Ships: domaingame.FleetCounts{domaingame.FleetDeathstar: 1},
	}
	value := fleetMessageContext{
		OriginOwnerID: 42, OriginOwnerName: "Explorer", OriginGalaxy: 1, OriginSystem: 2, OriginPosition: 3,
		TargetGalaxy: 1, TargetSystem: 2, TargetPosition: 16,
	}
	target := expeditionTargetState{Galaxy: 1, System: 2, Position: 16, Weapon: 10, Shield: 10, Armour: 10, Language: "en"}
	outcome := domaingame.ExpeditionOutcome{
		Event: domaingame.ExpeditionPirates, ReturnSeconds: 120,
		OpponentFleet: domaingame.FleetCounts{domaingame.FleetSmallCargo: 1},
	}
	task := fleetQueueTask{TaskID: 9, End: 1_000}
	settings := func() fakeQueryResult {
		return fakeQueryResult{rows: fakeRowsFromValues([]any{30, 0, 0, 70, 10, 5})}
	}

	for _, test := range []struct {
		name     string
		runner   *fakeFleetRunner
		wantErr  string
		wantExec int
	}{
		{name: "settings", runner: &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("settings failed")}}}}, wantErr: "settings failed"},
		{name: "battle data", runner: &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{settings()}}, execErrs: []error{errors.New("battle data failed")}}, wantErr: "battle data failed", wantExec: 1},
		{name: "battle message", runner: &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{settings()}}, execErrs: []error{nil, errors.New("message failed")}}, wantErr: "message failed", wantExec: 2},
		{name: "battle link", runner: &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{settings()}}, execErrs: []error{nil, nil, errors.New("link failed")}}, wantErr: "link failed", wantExec: 3},
		{name: "battle update", runner: &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{settings()}}, execErrs: []error{nil, nil, nil, errors.New("update failed")}}, wantErr: "update failed", wantExec: 4},
		{name: "cleanup", runner: &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{settings()}}, execErrs: []error{nil, nil, nil, nil, errors.New("cleanup failed")}}, wantErr: "cleanup failed", wantExec: 5},
		{name: "return", runner: &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{settings()}}, execErrs: []error{nil, nil, nil, nil, nil, errors.New("return failed")}}, wantErr: "return failed", wantExec: 6},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := NewFleetRepositoryWithRunner(test.runner, test.runner, "ogame_", nil)
			repository.combatRandom = func(int) int { return 0 }
			err := repository.finishExpeditionBattle(context.Background(), "`uni`", "`fleet`", "`logs`", "`queue`", "`messages`", "`users`", "`battle`", task, fleet, value, target, outcome)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("expected %q, got %v", test.wantErr, err)
			}
			if len(test.runner.execCalls) != test.wantExec {
				t.Fatalf("exec calls=%d, want %d", len(test.runner.execCalls), test.wantExec)
			}
		})
	}
}

func TestAdjustExpeditionStatsSigns(t *testing.T) {
	runner := &fakeFleetRunner{}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.adjustExpeditionStats(context.Background(), "`ogame_users`", 42, 10, 2, "+"); err != nil {
		t.Fatal(err)
	}
	if err := repository.adjustExpeditionStats(context.Background(), "`ogame_users`", 42, 10, 2, "unexpected"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(runner.execCalls[0].sql, "score1 = score1 + ?") || !strings.Contains(runner.execCalls[1].sql, "score1 = score1 - ?") {
		t.Fatalf("unexpected score SQL: %+v", runner.execCalls)
	}
}

func TestApplyExpeditionOutcomeRemainingReturnBranches(t *testing.T) {
	fleet := recallFleetRow{
		ID: 7, OwnerID: 42, StartPlanetID: 100, TargetPlanetID: 200,
		Metal: 1, Crystal: 2, Deuterium: 3, Fuel: 100, FlightTime: 120,
		Ships: domaingame.FleetCounts{domaingame.FleetSmallCargo: 1},
	}
	value := fleetMessageContext{OriginOwnerID: 42, TargetOwnerID: 99999}
	task := fleetQueueTask{TaskID: 9, End: 1_000}
	for _, test := range []struct {
		name    string
		outcome domaingame.ExpeditionOutcome
		arg     int
		want    float64
	}{
		{name: "crystal", outcome: domaingame.ExpeditionOutcome{Event: domaingame.ExpeditionResources, ResourceType: 1, ResourceAmount: 10, ReturnSeconds: 120}, arg: 3, want: 12},
		{name: "deuterium", outcome: domaingame.ExpeditionOutcome{Event: domaingame.ExpeditionResources, ResourceType: 2, ResourceAmount: 10, ReturnSeconds: 120}, arg: 4, want: 13},
		{name: "fleet without reward", outcome: domaingame.ExpeditionOutcome{Event: domaingame.ExpeditionFleet, FoundFleet: domaingame.FleetCounts{}, ReturnSeconds: 120}, arg: 2, want: 1},
		{name: "existing trader retained", outcome: domaingame.ExpeditionOutcome{Event: domaingame.ExpeditionTrader, UpdateTrader: false, ReturnSeconds: 120}, arg: 2, want: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeFleetRunner{}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
			if err := repository.applyExpeditionOutcome(context.Background(), "`uni`", "`fleet`", "`logs`", "`queue`", "`planets`", "`messages`", "`users`", "`battle`", task, fleet, value, expeditionTargetState{}, test.outcome); err != nil {
				t.Fatal(err)
			}
			if len(runner.execCalls) != 3 || runner.execCalls[0].args[test.arg] != test.want || runner.execCalls[0].args[5] != 0 {
				t.Fatalf("unexpected return writes: %+v", runner.execCalls)
			}
		})
	}
}

func TestApplyExpeditionFleetPropagatesRankFailure(t *testing.T) {
	runner := &fakeFleetRunner{execErrs: []error{nil, errors.New("rank failed")}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	fleet := recallFleetRow{ID: 7, OwnerID: 42, Ships: domaingame.FleetCounts{domaingame.FleetSmallCargo: 1}}
	outcome := domaingame.ExpeditionOutcome{
		Event: domaingame.ExpeditionFleet, FoundFleet: domaingame.FleetCounts{domaingame.FleetLightFighter: 1},
		FleetPoints: 4, FleetUnits: 1,
	}
	err := repository.applyExpeditionOutcome(context.Background(), "`uni`", "`fleet`", "`logs`", "`queue`", "`planets`", "`messages`", "`users`", "`battle`", fleetQueueTask{}, fleet, fleetMessageContext{}, expeditionTargetState{}, outcome)
	if err == nil || !strings.Contains(err.Error(), "rank failed") {
		t.Fatalf("expected rank failure, got %v", err)
	}
}

func TestExpeditionTransitionContextAndLogErrors(t *testing.T) {
	fleet := recallFleetRow{ID: 7, OwnerID: 42, StartPlanetID: 100, TargetPlanetID: 200, FlightTime: 120, DeployTime: 60, Ships: domaingame.FleetCounts{domaingame.FleetSmallCargo: 1}}
	task := fleetQueueTask{TaskID: 9, End: 1_000}
	for _, test := range []struct {
		name    string
		runner  *fakeFleetRunner
		wantErr string
	}{
		{name: "context query", runner: &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("context failed")}}}}, wantErr: "context failed"},
		{name: "context missing", runner: &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}, wantErr: "context unavailable"},
		{name: "transition log", runner: &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(fleetMessageContextTestRow())}}}, execErrs: []error{nil, nil, errors.New("log failed")}}, wantErr: "log failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := NewFleetRepositoryWithRunner(test.runner, test.runner, "ogame_", nil)
			err := repository.finishExpeditionArrival(context.Background(), "`fleet`", "`logs`", "`queue`", "`planets`", "`users`", task, fleet)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("expected %q, got %v", test.wantErr, err)
			}
		})
	}
}

func syncMapForTest() sync.Map {
	return sync.Map{}
}

func stringArg(args []any, index int) string {
	if index < 0 || index >= len(args) {
		return ""
	}
	return args[index].(string)
}
