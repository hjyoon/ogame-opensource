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

func TestAdminCronRunsGlobalBatchAndHonorsUniverseFreeze(t *testing.T) {
	row := adminUniverseSettingsRow()
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(row)},
		{rows: fakeRowsFromValues(buildingQueueTaskValues(buildingQueueTask{TaskID: 7, OwnerID: 42, Type: adminQueueTypeDebug, End: 100}))},
		{rows: fakeRowsFromValues()},
	}}}
	repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
	if _, err := repository.runAdminCron(context.Background(), 100); err != nil {
		t.Fatalf("runAdminCron returned error: %v", err)
	}
	if len(runner.execCalls) != 1 || !strings.Contains(runner.execCalls[0].sql, "DELETE FROM") {
		t.Fatalf("expected due debug removal, calls=%+v", runner.execCalls)
	}

	frozen := adminUniverseSettingsRow()
	frozen[14] = 1
	runner = &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(frozen)}}}}
	if _, err := NewAdminRepositoryWithQueryer(runner, "ogame_").runAdminCron(context.Background(), 100); err != nil || len(runner.calls) != 1 {
		t.Fatalf("frozen universe should skip queue, calls=%+v err=%v", runner.calls, err)
	}
}

func TestAdminCronRunErrors(t *testing.T) {
	tests := []struct {
		name    string
		results []fakeQueryResult
		execErr error
	}{
		{name: "universe", results: []fakeQueryResult{{err: errors.New("universe failed")}}},
		{name: "queue", results: []fakeQueryResult{{rows: fakeRowsFromValues(adminUniverseSettingsRow())}, {err: errors.New("queue failed")}}},
		{name: "dispatch", results: []fakeQueryResult{
			{rows: fakeRowsFromValues(adminUniverseSettingsRow())},
			{rows: fakeRowsFromValues(buildingQueueTaskValues(buildingQueueTask{TaskID: 7, Type: adminQueueTypeDebug}))},
		}, execErr: errors.New("dispatch failed")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: test.results}}
			if test.execErr != nil {
				runner.execErrs = []error{test.execErr}
			}
			_, err := NewAdminRepositoryWithQueryer(runner, "ogame_").runAdminCron(context.Background(), 100)
			if err == nil || !strings.Contains(err.Error(), test.name+" failed") {
				t.Fatalf("expected %s error, got %v", test.name, err)
			}
		})
	}
	if _, err := loadAdminCronTables("bad-prefix_"); err == nil {
		t.Fatal("expected invalid prefix error")
	}
	if _, err := NewAdminRepositoryWithQueryer(&fakeQueryer{}, "bad-prefix_").runAdminCron(context.Background(), 100); err == nil {
		t.Fatal("expected run invalid prefix error")
	}
}

func TestAdminCronTaskLoaderEdges(t *testing.T) {
	task := buildingQueueTask{TaskID: 1, OwnerID: 42, Type: adminQueueTypeDebug, SubID: 2, ObjID: 3, Level: 4, Start: 5, End: 6, Prio: 7}
	repository := NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(buildingQueueTaskValues(task))}}}, "ogame_")
	tasks, err := repository.loadAdminCronTasks(context.Background(), "queue", 10)
	if err != nil || len(tasks) != 1 || tasks[0] != task {
		t.Fatalf("unexpected tasks=%+v err=%v", tasks, err)
	}
	for name, rows := range map[string]*fakeRows{
		"scan": fakeRowsFromValues([]any{1}),
		"rows": fakeRowsFromValuesWithErr(errors.New("rows failed")),
	} {
		repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: rows}}}, "ogame_")
		if _, err := repository.loadAdminCronTasks(context.Background(), "queue", 10); err == nil {
			t.Fatalf("expected %s error", name)
		}
	}
}

func TestAdminCronDispatchesSimpleAndDeferredTypes(t *testing.T) {
	tables, err := loadAdminCronTables("ogame_")
	if err != nil {
		t.Fatal(err)
	}
	simple := []string{adminQueueTypeAllowName, adminQueueTypeChangeEmail, adminQueueTypeUnban, adminQueueTypeAllowAttacks, adminQueueTypeDebug}
	for _, queueType := range simple {
		runner := &fakeGalaxyRunner{}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if err := repository.finishAdminCronTask(context.Background(), tables, buildingQueueTask{TaskID: 1, OwnerID: 42, Type: queueType}, "en"); err != nil {
			t.Fatalf("%s returned error: %v", queueType, err)
		}
		if len(runner.execCalls) == 0 || !strings.Contains(runner.execCalls[len(runner.execCalls)-1].sql, "DELETE FROM") {
			t.Fatalf("%s did not remove task: %+v", queueType, runner.execCalls)
		}
	}
	runner := &fakeGalaxyRunner{}
	err = NewAdminRepositoryWithQueryer(runner, "ogame_").finishAdminCronTask(context.Background(), tables, buildingQueueTask{Type: adminCouponQueueType}, "en")
	if err != nil || len(runner.execCalls) != 0 {
		t.Fatalf("deferred coupon should no-op, calls=%+v err=%v", runner.execCalls, err)
	}
	runner = &fakeGalaxyRunner{}
	if err := NewAdminRepositoryWithQueryer(runner, "ogame_").finishAdminCronTask(context.Background(), tables, buildingQueueTask{TaskID: 9, Type: "Unknown"}, "de"); err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 2 || !strings.Contains(runner.execCalls[1].args[1].(string), "Unbekannter Auftragstyp") {
		t.Fatalf("unexpected unknown handling: %+v", runner.execCalls)
	}
}

func TestAdminCronDispatchesCoreQueueTypes(t *testing.T) {
	tables, _ := loadAdminCronTables("ogame_")
	tests := []struct {
		queueType string
		task      buildingQueueTask
		results   []fakeQueryResult
		wantErr   bool
	}{
		{queueType: queueTypeBuild, task: buildingQueueTask{TaskID: 1, Type: queueTypeBuild}, results: []fakeQueryResult{{err: errors.New("build failed")}}, wantErr: true},
		{queueType: queueTypeResearch, task: buildingQueueTask{TaskID: 2, OwnerID: 42, Type: queueTypeResearch, SubID: 99, End: 100}, results: []fakeQueryResult{{err: errors.New("research failed")}}, wantErr: true},
		{queueType: queueTypeShipyard, task: buildingQueueTask{TaskID: 3, Type: queueTypeShipyard}},
		{queueType: queueTypeFleet, task: buildingQueueTask{TaskID: 4, Type: queueTypeFleet, SubID: 44}, results: []fakeQueryResult{{rows: fakeRowsFromValues()}}},
		{queueType: queueTypeRecalcPoints, task: buildingQueueTask{TaskID: 5, Type: queueTypeRecalcPoints, OwnerID: 42}, results: []fakeQueryResult{{err: errors.New("recalc failed")}}, wantErr: true},
		{queueType: queueTypeAI, task: buildingQueueTask{TaskID: 6, Type: queueTypeAI, SubID: 7}, results: []fakeQueryResult{{rows: fakeRowsFromValues()}}},
	}
	for _, test := range tests {
		t.Run(test.queueType, func(t *testing.T) {
			runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: test.results}}
			err := NewAdminRepositoryWithQueryer(runner, "ogame_").finishAdminCronTask(context.Background(), tables, test.task, "en")
			if test.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestAdminCronRecalculatesPointsAndRemovesTask(t *testing.T) {
	tables, _ := loadAdminCronTables("ogame_")
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues(allResearchLevelRow(nil))},
		{rows: fakeRowsFromValues()},
	}}}
	task := buildingQueueTask{TaskID: 5, OwnerID: 42, Type: queueTypeRecalcPoints}
	if err := NewAdminRepositoryWithQueryer(runner, "ogame_").finishAdminCronTask(context.Background(), tables, task, "en"); err != nil {
		t.Fatalf("recalc returned error: %v", err)
	}
	if len(runner.execCalls) != 10 || !strings.Contains(runner.execCalls[9].sql, "DELETE FROM") {
		t.Fatalf("unexpected recalc calls: %+v", runner.execCalls)
	}
}

func TestAdminCronMaintenanceHandlers(t *testing.T) {
	tables, _ := loadAdminCronTables("ogame_")
	now := time.Date(2026, time.July, 13, 8, 5, 0, 0, time.UTC)
	tests := []struct {
		name string
		run  func(AdminRepository) error
		min  int
	}{
		{name: "unload", min: 4, run: func(r AdminRepository) error {
			return r.finishAdminCronUnloadAll(context.Background(), tables, buildingQueueTask{TaskID: 1})
		}},
		{name: "debris", min: 3, run: func(r AdminRepository) error {
			return r.finishAdminCronCleanDebris(context.Background(), tables, buildingQueueTask{TaskID: 2, End: int(now.Unix())})
		}},
		{name: "stats", min: 5, run: func(r AdminRepository) error {
			return r.finishAdminCronUpdateStats(context.Background(), tables, buildingQueueTask{TaskID: 3, End: int(now.Unix())}, "ru")
		}},
		{name: "ally", min: 8, run: func(r AdminRepository) error {
			return r.finishAdminCronRecalcAllyPoints(context.Background(), tables, buildingQueueTask{TaskID: 4})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeGalaxyRunner{}
			repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
			repository.now = func() time.Time { return now }
			err := test.run(repository)
			if err != nil || len(runner.execCalls) < test.min {
				t.Fatalf("calls=%d err=%v", len(runner.execCalls), err)
			}
		})
	}
}

func TestAdminCronDispatchesMaintenanceTypes(t *testing.T) {
	tables, _ := loadAdminCronTables("ogame_")
	now := time.Date(2026, time.July, 13, 8, 5, 0, 0, time.UTC)
	for _, queueType := range []string{adminQueueTypeUnloadAll, adminQueueTypeCleanDebris, adminQueueTypeUpdateStats, adminQueueTypeRecalcAllyPoints} {
		runner := &fakeGalaxyRunner{}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		repository.now = func() time.Time { return now }
		repository.randomIntN = nil
		task := buildingQueueTask{TaskID: 1, Type: queueType, End: int(now.Unix())}
		if err := repository.finishAdminCronTask(context.Background(), tables, task, "en"); err != nil {
			t.Fatalf("%s returned error: %v", queueType, err)
		}
	}
}

func TestAdminCronMaintenanceErrors(t *testing.T) {
	tables, _ := loadAdminCronTables("ogame_")
	tests := []func(AdminRepository) error{
		func(r AdminRepository) error {
			return r.finishAdminCronUserFlag(context.Background(), tables, buildingQueueTask{TaskID: 1}, "UPDATE users")
		},
		func(r AdminRepository) error {
			return r.finishAdminCronUnloadAll(context.Background(), tables, buildingQueueTask{TaskID: 1})
		},
		func(r AdminRepository) error {
			return r.finishAdminCronCleanDebris(context.Background(), tables, buildingQueueTask{TaskID: 1})
		},
		func(r AdminRepository) error {
			return r.finishAdminCronUpdateStats(context.Background(), tables, buildingQueueTask{TaskID: 1}, "en")
		},
		func(r AdminRepository) error {
			return r.finishAdminCronRecalcAllyPoints(context.Background(), tables, buildingQueueTask{TaskID: 1})
		},
	}
	for index, run := range tests {
		runner := &fakeGalaxyRunner{execErrs: []error{errors.New("exec failed")}}
		if err := run(NewAdminRepositoryWithQueryer(runner, "ogame_")); err == nil {
			t.Fatalf("case %d expected error", index)
		}
	}
}

func TestAdminCronLaterWriteErrors(t *testing.T) {
	tables, _ := loadAdminCronTables("ogame_")
	tests := []struct {
		name string
		errs []error
		run  func(AdminRepository) error
	}{
		{name: "unknown remove", errs: []error{errors.New("failed")}, run: func(r AdminRepository) error {
			return r.finishAdminCronTask(context.Background(), tables, buildingQueueTask{TaskID: 1, Type: "Unknown"}, "en")
		}},
		{name: "unload sessions", errs: []error{nil, errors.New("failed")}, run: func(r AdminRepository) error {
			return r.finishAdminCronUnloadAll(context.Background(), tables, buildingQueueTask{TaskID: 1})
		}},
		{name: "unload remove", errs: []error{nil, nil, errors.New("failed")}, run: func(r AdminRepository) error {
			return r.finishAdminCronUnloadAll(context.Background(), tables, buildingQueueTask{TaskID: 1})
		}},
		{name: "debris remove", errs: []error{nil, errors.New("failed")}, run: func(r AdminRepository) error {
			return r.finishAdminCronCleanDebris(context.Background(), tables, buildingQueueTask{TaskID: 1})
		}},
		{name: "stats remove", errs: []error{nil, nil, errors.New("failed")}, run: func(r AdminRepository) error {
			return r.finishAdminCronUpdateStats(context.Background(), tables, buildingQueueTask{TaskID: 1}, "en")
		}},
		{name: "stats schedule", errs: []error{nil, nil, nil, errors.New("failed")}, run: func(r AdminRepository) error {
			return r.finishAdminCronUpdateStats(context.Background(), tables, buildingQueueTask{TaskID: 1}, "en")
		}},
		{name: "ally rank", errs: []error{nil, errors.New("failed")}, run: func(r AdminRepository) error {
			return r.finishAdminCronRecalcAllyPoints(context.Background(), tables, buildingQueueTask{TaskID: 1})
		}},
	}
	for _, test := range tests {
		runner := &fakeGalaxyRunner{execErrs: test.errs}
		if err := test.run(NewAdminRepositoryWithQueryer(runner, "ogame_")); err == nil {
			t.Fatalf("%s expected error", test.name)
		}
	}
}

func TestAdminQueueCronMutation(t *testing.T) {
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(adminUniverseSettingsRow())},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues()},
	}}}
	repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
	repository.now = func() time.Time { return time.Unix(100, 0) }
	issue, err := repository.mutateAdminQueue(context.Background(), "queue", appgame.AdminMutationQuery{Action: domaingame.AdminActionQueueCron})
	if err != nil || issue == nil || issue.Code != domaingame.AdminIssueActionSaved {
		t.Fatalf("unexpected issue=%+v err=%v", issue, err)
	}
	runner = &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("cron failed")}}}}
	repository = NewAdminRepositoryWithQueryer(runner, "ogame_")
	repository.now = func() time.Time { return time.Unix(100, 0) }
	if _, err := repository.mutateAdminQueue(context.Background(), "queue", appgame.AdminMutationQuery{Action: domaingame.AdminActionQueueCron}); err == nil {
		t.Fatal("expected cron mutation error")
	}
}

func TestAdminCronTimeAndLocalizationHelpers(t *testing.T) {
	location := time.FixedZone("test", 3*60*60)
	for hour, want := range map[int]int{7: 8, 8: 16, 16: 20, 20: 8} {
		at := time.Date(2026, time.July, 13, hour, 5, 0, 0, location)
		next := nextAdminCronStatsTime(at)
		if next.Hour() != want || !next.After(at) {
			t.Fatalf("hour %d returned %v", hour, next)
		}
	}
	monday := time.Date(2026, time.July, 13, 1, 10, 0, 0, location)
	if next := nextAdminCronWeekday(monday, time.Monday, 1, 10); next.Sub(monday) != 7*24*time.Hour {
		t.Fatalf("unexpected next weekday: %v", next)
	}
	if !strings.Contains(adminCronUnknownMessage("ru"), "Неизвестный") || !strings.Contains(adminCronUnknownMessage("en"), "Unknown") {
		t.Fatal("unknown localization mismatch")
	}
	if !strings.Contains(adminCronOldStatsMessage("de", monday), "01:10") || !strings.Contains(adminCronOldStatsMessage("en", monday), "timestamp") {
		t.Fatal("stats localization mismatch")
	}
	cronLocation := (AdminRepository{}).adminCronLocation()
	if cronLocation != legacyAdminTimeLocation {
		t.Fatalf("unexpected legacy cron location: %v", cronLocation)
	}
	stored := nextAdminCronWeekday(time.Date(2026, time.July, 13, 0, 0, 0, 0, time.UTC).In(cronLocation), time.Monday, 1, 10).UTC()
	if stored.Weekday() != time.Sunday || stored.Hour() != 22 || stored.Minute() != 10 {
		t.Fatalf("unexpected UTC cron slot: %v", stored)
	}
}
