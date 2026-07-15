package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAdminCronCleanPlanetsRemovesAndReschedules(t *testing.T) {
	tables, _ := loadAdminCronTables("ogame_")
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{77})},
		{rows: fakeRowsFromValues()},
	}}}
	now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
	repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
	repository.now = func() time.Time { return now }
	err := repository.finishAdminCronCleanPlanets(context.Background(), tables, buildingQueueTask{TaskID: 9, End: 100}, "de")
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 7 {
		t.Fatalf("unexpected writes: %+v", runner.execCalls)
	}
	if !strings.Contains(runner.execCalls[3].sql, "DELETE FROM `ogame_planets`") || runner.execCalls[3].args[0] != 77 {
		t.Fatalf("planet was not deleted: %+v", runner.execCalls[3])
	}
	if got := runner.execCalls[5].args[3]; got != int(nextAdminCronDaily(now.In(legacyAdminTimeLocation), 1, 10).Unix()) {
		t.Fatalf("unexpected next cleanup time: %v", got)
	}
	if got := runner.execCalls[6].args[1]; got != "Aufräumen zerstörter Planeten (1)" {
		t.Fatalf("unexpected debug message: %v", got)
	}
}

func TestAdminCronCleanPlayersEmptyBatch(t *testing.T) {
	tables, _ := loadAdminCronTables("ogame_")
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues()},
	}}}
	now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
	repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
	repository.now = func() time.Time { return now }
	if err := repository.finishAdminCronCleanPlayers(context.Background(), tables, buildingQueueTask{TaskID: 8, End: 500}); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 2 || runner.execCalls[1].args[1] != adminQueueTypeCleanPlayers {
		t.Fatalf("unexpected writes: %+v", runner.execCalls)
	}
	if got := runner.execCalls[1].args[3]; got != int(nextAdminCronDaily(now.In(legacyAdminTimeLocation), 1, 10).Unix()) {
		t.Fatalf("unexpected next player cleanup time: %v", got)
	}
}

func TestAdminCronCleanPlayersAppliesProtectedBotAndInactiveRules(t *testing.T) {
	tables, _ := loadAdminCronTables("ogame_")
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{adminCronLegorID})},
		{rows: fakeRowsFromValues([]any{42}, []any{43})},
		{rows: fakeRowsFromValues([]any{900})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues()},
	}}}
	repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
	if err := repository.finishAdminCronCleanPlayers(context.Background(), tables, buildingQueueTask{TaskID: 8, End: 500}); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 5 || !strings.Contains(runner.calls[2].sql, "type = ?") || !strings.Contains(runner.calls[4].sql, "JOIN") {
		t.Fatalf("unexpected bot/inactive queries: %+v", runner.calls)
	}
	if len(runner.execCalls) != 29 {
		t.Fatalf("unexpected inactive cleanup writes: %d", len(runner.execCalls))
	}
}

func TestAdminCronCleanupFleetLoopsHonorRecallNoOp(t *testing.T) {
	tables, _ := loadAdminCronTables("ogame_")
	planetRunner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{77})},
		{rows: fakeRowsFromValues([]any{55})},
		{rows: fakeRowsFromValues([]any{1})},
	}}}
	if err := NewAdminRepositoryWithQueryer(planetRunner, "ogame_").finishAdminCronCleanPlanets(context.Background(), tables, buildingQueueTask{TaskID: 1, End: 100}, "en"); err != nil {
		t.Fatal(err)
	}
	userRunner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{55})},
		{rows: fakeRowsFromValues([]any{1})},
	}}}
	if err := NewAdminRepositoryWithQueryer(userRunner, "ogame_").removeAdminCronUser(context.Background(), tables, 42, 100); err != nil {
		t.Fatal(err)
	}
}

func TestAdminCronRemoveUserMatchesLegacyCleanup(t *testing.T) {
	tables, _ := loadAdminCronTables("ogame_")
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
	repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
	if err := repository.removeAdminCronUser(context.Background(), tables, 42, 100); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 27 {
		t.Fatalf("unexpected cleanup writes: %d", len(runner.execCalls))
	}
	joined := ""
	for _, call := range runner.execCalls {
		joined += call.sql + "\n"
	}
	for _, table := range []string{"reports", "messages", "notes", "browse", "template", "botvars", "userlogs", "fleetlogs", "iplogs", "union", "allyapps", "buddy"} {
		if !strings.Contains(joined, "ogame_"+table) {
			t.Fatalf("missing cleanup for %s", table)
		}
	}
	if !strings.Contains(joined, "owner_id = ? AND type <> ?") || !strings.Contains(joined, "SET owner_id = ?") {
		t.Fatal("planet and debris ownership cleanup missing")
	}
}

func TestAdminCronRemoveUserProtectsSystemAccounts(t *testing.T) {
	tables, _ := loadAdminCronTables("ogame_")
	for _, id := range []int{adminCronLegorID, adminCronSpaceID} {
		runner := &fakeGalaxyRunner{}
		if err := NewAdminRepositoryWithQueryer(runner, "ogame_").removeAdminCronUser(context.Background(), tables, id, 100); err != nil || len(runner.execCalls) != 0 || len(runner.calls) != 0 {
			t.Fatalf("protected account %d was touched", id)
		}
	}
}

func TestAdminCronCleanupHelpers(t *testing.T) {
	repository := NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{4}, []any{5})}}}, "ogame_")
	ids, err := repository.loadAdminCronIDs(context.Background(), "SELECT id")
	if err != nil || len(ids) != 2 || ids[1] != 5 {
		t.Fatalf("ids=%v err=%v", ids, err)
	}
	for _, result := range []fakeQueryResult{
		{err: errors.New("query failed")},
		{rows: fakeRowsFromValues([]any{"bad"})},
		{rows: fakeRowsFromValuesWithErr(errors.New("rows failed"))},
	} {
		if _, err := NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{result}}, "ogame_").loadAdminCronIDs(context.Background(), "SELECT id"); err == nil {
			t.Fatal("expected ID loader error")
		}
	}
	for language, want := range map[string]string{"en": "Cleanup of destroyed planets (2)", "es": "Cleanup of destroyed planets (2)", "ru": "Чистка уничтоженных планет (2)"} {
		if got := adminCronCleanPlanetsMessage(language, 2); got != want {
			t.Fatalf("%s: %q", language, got)
		}
	}
	if got := nextAdminCronDaily(time.Date(2026, 7, 13, 23, 0, 0, 0, time.UTC), 1, 10); got.Day() != 14 || got.Hour() != 1 || got.Minute() != 10 {
		t.Fatalf("unexpected daily time: %v", got)
	}
}

func TestAdminCronCleanupPropagatesEarlyErrors(t *testing.T) {
	tables, _ := loadAdminCronTables("ogame_")
	if err := NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("planets failed")}}}, "ogame_").finishAdminCronCleanPlanets(context.Background(), tables, buildingQueueTask{}, "en"); err == nil {
		t.Fatal("expected planet query error")
	}
	if err := NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("players failed")}}}, "ogame_").finishAdminCronCleanPlayers(context.Background(), tables, buildingQueueTask{}); err == nil {
		t.Fatal("expected player query error")
	}
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, execErrs: []error{errors.New("delete failed")}}
	if err := NewAdminRepositoryWithQueryer(runner, "ogame_").removeAdminCronUser(context.Background(), tables, 42, 100); err == nil {
		t.Fatal("expected user cleanup error")
	}
}

func TestAdminCronCleanupPropagatesHandlerErrors(t *testing.T) {
	tables, _ := loadAdminCronTables("ogame_")
	planetCases := []struct {
		results []fakeQueryResult
		errs    []error
	}{
		{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{77})}, {err: errors.New("fleet query failed")}}},
		{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{77})}, {rows: fakeRowsFromValues()}}, errs: []error{errors.New("flush failed")}},
		{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{77})}, {rows: fakeRowsFromValues()}}, errs: []error{nil, nil, nil, errors.New("planet delete failed")}},
		{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{77})}, {rows: fakeRowsFromValues([]any{55})}, {err: errors.New("recall failed")}}},
		{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}, errs: []error{errors.New("remove failed")}},
		{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}, errs: []error{nil, errors.New("schedule failed")}},
		{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}, errs: []error{nil, nil, errors.New("debug failed")}},
	}
	for index, test := range planetCases {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: test.results}, execErrs: test.errs}
		if err := NewAdminRepositoryWithQueryer(runner, "ogame_").finishAdminCronCleanPlanets(context.Background(), tables, buildingQueueTask{TaskID: 1}, "en"); err == nil {
			t.Fatalf("planet case %d expected error", index)
		}
	}
	playerCases := []struct {
		results []fakeQueryResult
		errs    []error
	}{
		{results: []fakeQueryResult{{rows: fakeRowsFromValues()}, {err: errors.New("inactive failed")}}},
		{results: []fakeQueryResult{{rows: fakeRowsFromValues()}, {rows: fakeRowsFromValues([]any{42})}, {err: errors.New("bot failed")}}},
		{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{42})}, {err: errors.New("user failed")}}},
		{results: []fakeQueryResult{{rows: fakeRowsFromValues()}, {rows: fakeRowsFromValues([]any{42})}, {rows: fakeRowsFromValues()}, {err: errors.New("inactive user failed")}}},
		{results: []fakeQueryResult{{rows: fakeRowsFromValues()}, {rows: fakeRowsFromValues()}}, errs: []error{errors.New("remove failed")}},
		{results: []fakeQueryResult{{rows: fakeRowsFromValues()}, {rows: fakeRowsFromValues()}}, errs: []error{nil, errors.New("schedule failed")}},
	}
	for index, test := range playerCases {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: test.results}, execErrs: test.errs}
		if err := NewAdminRepositoryWithQueryer(runner, "ogame_").finishAdminCronCleanPlayers(context.Background(), tables, buildingQueueTask{TaskID: 1}); err == nil {
			t.Fatalf("player case %d expected error", index)
		}
	}
	if now := (AdminRepository{}).adminCronNow(); now.IsZero() {
		t.Fatal("default clock returned zero")
	}
	if got := adminCronClock(100, time.UTC)().Unix(); got != 100 {
		t.Fatalf("fixed clock=%d", got)
	}
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{55})},
		{err: errors.New("recall failed")},
	}}}
	if err := NewAdminRepositoryWithQueryer(runner, "ogame_").removeAdminCronUser(context.Background(), tables, 42, 100); err == nil {
		t.Fatal("expected incoming recall error")
	}
}
