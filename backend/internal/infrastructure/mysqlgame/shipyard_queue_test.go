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

func TestShipyardRepositoryMutateShipyardEnqueuesFleetOrder(t *testing.T) {
	now := time.Unix(1_700, 0)
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: append(
		shipyardMutationPrefixResults(map[int]int{domaingame.ResearchCombustionDrive: 1}, map[int]int{domaingame.BuildingShipyard: 1}),
		fakeQueryResult{rows: fakeRowsFromValues(fleetCountRow(nil))},
		fakeQueryResult{rows: fakeRowsFromValues()},
	)}}
	repository := NewShipyardRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	outcome, err := repository.MutateShipyard(context.Background(), appgame.ShipyardMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Orders:   map[int]int{domaingame.FleetLightFighter: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue != nil {
		t.Fatalf("expected successful shipyard order, got issue %+v calls=%+v", outcome.ActionIssue, runner.calls)
	}
	if len(runner.execs) != 2 {
		t.Fatalf("expected resource spend and queue insert, got %+v", runner.execs)
	}
	if runner.execs[0].args[0] != 6000.0 || runner.execs[0].args[1] != 2000.0 || runner.execs[0].args[3] != 1700 || runner.execs[0].args[4] != 99 || runner.execs[0].args[5] != 42 {
		t.Fatalf("unexpected spend args: %+v", runner.execs[0].args)
	}
	if runner.execs[1].args[0] != 42 || runner.execs[1].args[1] != queueTypeShipyard || runner.execs[1].args[2] != 99 || runner.execs[1].args[3] != domaingame.FleetLightFighter || runner.execs[1].args[4] != 2 || runner.execs[1].args[5] != 1700 {
		t.Fatalf("unexpected queue insert args: %+v", runner.execs[1].args)
	}
}

func TestDefenseRepositoryMutateDefenseEnqueuesDefenseOrder(t *testing.T) {
	now := time.Unix(1_700, 0)
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: append(
		shipyardMutationPrefixResults(nil, map[int]int{domaingame.BuildingShipyard: 1}),
		fakeQueryResult{rows: fakeRowsFromValues()},
	)}}
	repository := NewDefenseRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	outcome, err := repository.MutateDefense(context.Background(), appgame.DefenseMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Orders:   map[int]int{domaingame.DefenseRocketLauncher: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue != nil {
		t.Fatalf("expected successful defense order, got issue %+v", outcome.ActionIssue)
	}
	if len(runner.execs) != 2 {
		t.Fatalf("expected resource spend and queue insert, got %+v", runner.execs)
	}
	if runner.execs[0].args[0] != 4000.0 || runner.execs[0].args[1] != 0.0 || runner.execs[0].args[3] != 1700 {
		t.Fatalf("unexpected spend args: %+v", runner.execs[0].args)
	}
	if runner.execs[1].args[1] != queueTypeShipyard || runner.execs[1].args[3] != domaingame.DefenseRocketLauncher || runner.execs[1].args[4] != 2 {
		t.Fatalf("unexpected queue insert args: %+v", runner.execs[1].args)
	}
}

func TestShipyardRepositoryFinishDueShipyardQueuePartiallyCompletesUnits(t *testing.T) {
	task := buildingQueueTask{TaskID: 8, OwnerID: 42, Type: queueTypeShipyard, SubID: 99, ObjID: domaingame.FleetLightFighter, Level: 3, Start: 100, End: 110, Prio: 20}
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1.0, 999, 0})},
		{rows: fakeRowsFromValues(buildingQueueTaskValues(task))},
	}}}
	repository := NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)

	if err := repository.FinishDueShipyardQueues(context.Background(), 125); err != nil {
		t.Fatal(err)
	}
	if len(runner.execs) != 3 {
		t.Fatalf("expected planet update, stats update, and queue shrink, got %+v", runner.execs)
	}
	if runner.execs[0].args[0] != 2 || runner.execs[0].args[1] != 99 || runner.execs[0].args[2] != 42 {
		t.Fatalf("unexpected planet completion args: %+v", runner.execs[0].args)
	}
	if runner.execs[1].args[0] != int64(8000) || runner.execs[1].args[1] != int64(8000) || runner.execs[1].args[3] != 42 {
		t.Fatalf("unexpected stats args: %+v", runner.execs[1].args)
	}
	if runner.execs[2].args[0] != 120 || runner.execs[2].args[1] != 130 || runner.execs[2].args[2] != 2 || runner.execs[2].args[3] != 8 {
		t.Fatalf("unexpected queue shrink args: %+v", runner.execs[2].args)
	}
}

func TestShipyardRepositoryFinishDueShipyardQueueRemovesCompletedTask(t *testing.T) {
	task := buildingQueueTask{TaskID: 8, OwnerID: 42, Type: queueTypeShipyard, SubID: 99, ObjID: domaingame.DefenseRocketLauncher, Level: 1, Start: 100, End: 110, Prio: 20}
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1.0, 999, 0})},
		{rows: fakeRowsFromValues(buildingQueueTaskValues(task))},
	}}}
	repository := NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)

	if err := repository.FinishDueShipyardQueues(context.Background(), 120); err != nil {
		t.Fatal(err)
	}
	if len(runner.execs) != 11 {
		t.Fatalf("expected completion, remove, and rank recalculation execs, got %+v", runner.execs)
	}
	if runner.execs[0].args[0] != 1 || runner.execs[1].args[0] != int64(2000) || runner.execs[1].args[1] != int64(0) {
		t.Fatalf("unexpected completion/stat args: first=%+v second=%+v", runner.execs[0].args, runner.execs[1].args)
	}
	if runner.execs[2].args[0] != 8 {
		t.Fatalf("expected completed queue removal, got %+v", runner.execs[2].args)
	}
}

func TestShipyardRepositoryFinishDueShipyardQueueHandlesUnavailableAndFrozen(t *testing.T) {
	if err := (ShipyardRepository{}).FinishDueShipyardQueues(context.Background(), 120); err == nil || !strings.Contains(err.Error(), "updater unavailable") {
		t.Fatalf("expected unavailable updater error, got %v", err)
	}

	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1.0, 999, 1})},
	}}}
	repository := NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.FinishDueShipyardQueues(context.Background(), 120); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 || len(runner.execs) != 0 {
		t.Fatalf("frozen universe should not load due tasks or exec, calls=%+v execs=%+v", runner.calls, runner.execs)
	}
}

func TestShipyardRepositoryFinishDueShipyardQueuePropagatesErrors(t *testing.T) {
	runner := &fakeBuildingsRunner{}
	repository := NewShipyardRepositoryWithRunner(runner, runner, "bad-prefix`", nil)
	if err := repository.FinishDueShipyardQueues(context.Background(), 120); err == nil {
		t.Fatal("expected table name error")
	}

	runner = &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("config failed")}}}}
	repository = NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.FinishDueShipyardQueues(context.Background(), 120); err == nil || !strings.Contains(err.Error(), "config failed") {
		t.Fatalf("expected config error, got %v", err)
	}

	runner = &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1.0, 999, 0})},
		{err: errors.New("due failed")},
	}}}
	repository = NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.FinishDueShipyardQueues(context.Background(), 120); err == nil || !strings.Contains(err.Error(), "due failed") {
		t.Fatalf("expected due task error, got %v", err)
	}

	task := buildingQueueTask{TaskID: 8, OwnerID: 42, Type: queueTypeShipyard, SubID: 99, ObjID: domaingame.FleetLightFighter, Level: 1, Start: 100, End: 110}
	finishErr := errors.New("finish failed")
	runner = &fakeBuildingsRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{1.0, 999, 0})},
			{rows: fakeRowsFromValues(buildingQueueTaskValues(task))},
		}},
		execErr: finishErr,
	}
	repository = NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.FinishDueShipyardQueues(context.Background(), 120); !errors.Is(err, finishErr) {
		t.Fatalf("expected finish error, got %v", err)
	}
}

func TestShipyardRepositoryMutateShipyardOrderIssues(t *testing.T) {
	if _, err := (ShipyardRepository{}).MutateShipyard(context.Background(), appgame.ShipyardMutationQuery{}); err == nil {
		t.Fatal("expected unavailable updater error")
	}

	runner := &fakeBuildingsRunner{}
	repository := NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	outcome, err := repository.MutateShipyard(context.Background(), appgame.ShipyardMutationQuery{Orders: map[int]int{}})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue == nil || outcome.ActionIssue.Code != domaingame.BuildingsIssueInvalid {
		t.Fatalf("expected invalid empty order issue, got %+v", outcome)
	}
	runner = &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1.0, 999, 0})},
		{rows: fakeRowsFromValues()},
	}}}
	repository = NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	outcome, err = repository.MutateShipyard(context.Background(), appgame.ShipyardMutationQuery{Orders: map[int]int{domaingame.FleetLightFighter: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue == nil || outcome.ActionIssue.Code != domaingame.BuildingsIssueInvalid {
		t.Fatalf("expected invalid non-positive order issue, got %+v", outcome)
	}

	runner = &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: append(
		shipyardMutationPrefixResults(map[int]int{domaingame.ResearchCombustionDrive: 1}, map[int]int{domaingame.BuildingShipyard: 1}),
		fakeQueryResult{rows: fakeRowsFromValues(fleetCountRow(nil))},
	)}}
	repository = NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	outcome, err = repository.MutateShipyard(context.Background(), appgame.ShipyardMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Orders:   map[int]int{9999: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue == nil || outcome.ActionIssue.Code != domaingame.BuildingsIssueInvalid {
		t.Fatalf("expected invalid unknown unit issue, got %+v", outcome)
	}

	runner = &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1.0, 999, 0})},
		{rows: fakeRowsFromValues()},
		{err: errors.New("state failed")},
	}}}
	repository = NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	if _, err = repository.MutateShipyard(context.Background(), appgame.ShipyardMutationQuery{PlayerID: 42, PlanetID: 99, Orders: map[int]int{domaingame.FleetLightFighter: 1}}); err == nil || !strings.Contains(err.Error(), "state failed") {
		t.Fatalf("expected state load error, got %v", err)
	}

	runner = &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: append(
		shipyardMutationPrefixResults(map[int]int{domaingame.ResearchCombustionDrive: 1}, map[int]int{domaingame.BuildingShipyard: 1}),
		fakeQueryResult{rows: fakeRowsFromValues(fleetCountRow(nil))},
		fakeQueryResult{err: errors.New("latest failed")},
	)}, results: []sql.Result{buildingSQLResult{affected: 1}}}
	repository = NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	if _, err = repository.MutateShipyard(context.Background(), appgame.ShipyardMutationQuery{PlayerID: 42, PlanetID: 99, Orders: map[int]int{domaingame.FleetLightFighter: 1}}); err == nil || !strings.Contains(err.Error(), "latest failed") {
		t.Fatalf("expected enqueue latest error, got %v", err)
	}
}

func TestShipyardRepositoryEnqueueShipyardItemIssues(t *testing.T) {
	repository := NewShipyardRepositoryWithRunner(&fakeBuildingsRunner{}, &fakeBuildingsRunner{}, "ogame_", nil)
	item := goodShipyardQueueItem(domaingame.FleetLightFighter)
	tests := []struct {
		name  string
		state shipyardMutationState
		item  domaingame.ShipyardItem
		want  string
	}{
		{name: "vacation", state: shipyardMutationState{user: buildingMutationUser{Vacation: true}}, item: item, want: domaingame.BuildingsIssueVacation},
		{name: "frozen", state: shipyardMutationState{config: shipyardMutationConfig{Frozen: true}}, item: item, want: domaingame.BuildingsIssueUniversePause},
		{name: "requirements", state: shipyardMutationState{}, item: domaingame.ShipyardItem{MeetsRequirement: false}, want: domaingame.BuildingsIssueRequirements},
		{name: "busy", state: shipyardMutationState{}, item: domaingame.ShipyardItem{MeetsRequirement: true, BlockedReason: "busy", MaxBuild: 1}, want: domaingame.BuildingsIssueBusy},
		{name: "invalid reason", state: shipyardMutationState{}, item: domaingame.ShipyardItem{MeetsRequirement: true, BlockedReason: "blocked", MaxBuild: 1}, want: domaingame.BuildingsIssueInvalid},
		{name: "no resources", state: shipyardMutationState{}, item: domaingame.ShipyardItem{MeetsRequirement: true, MaxBuild: 0}, want: domaingame.BuildingsIssueNoResources},
		{name: "queue full", state: shipyardMutationState{queueRows: make([]buildingQueueTask, maxShipyardOrders)}, item: item, want: domaingame.BuildingsIssueQueueFull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue, ok, err := repository.enqueueShipyardItem(context.Background(), tt.state, tt.item, 1, 1_700)
			if err != nil || ok || issue == nil || issue.Code != tt.want {
				t.Fatalf("expected issue %q, ok=%v issue=%+v err=%v", tt.want, ok, issue, err)
			}
		})
	}
}

func TestShipyardRepositoryEnqueueShipyardItemDatabaseEdges(t *testing.T) {
	item := goodShipyardQueueItem(domaingame.FleetLightFighter)
	state := shipyardMutationState{playerID: 42, planetID: 99, config: shipyardMutationConfig{OrderCap: 1_000}, items: []domaingame.ShipyardItem{item}}

	runner := &fakeBuildingsRunner{results: []sql.Result{buildingSQLResult{affected: 0}}}
	repository := NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	issue, ok, err := repository.enqueueShipyardItem(context.Background(), state, item, 1, 1_700)
	if err != nil || ok || issue == nil || issue.Code != domaingame.BuildingsIssueNoResources {
		t.Fatalf("expected no-resource issue from atomic spend, ok=%v issue=%+v err=%v", ok, issue, err)
	}

	spendErr := errors.New("spend failed")
	runner = &fakeBuildingsRunner{execErr: spendErr}
	repository = NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	if _, _, err := repository.enqueueShipyardItem(context.Background(), state, item, 1, 1_700); !errors.Is(err, spendErr) {
		t.Fatalf("expected spend error, got %v", err)
	}

	runner = &fakeBuildingsRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("latest failed")}}},
		results:     []sql.Result{buildingSQLResult{affected: 1}},
	}
	repository = NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	if _, _, err := repository.enqueueShipyardItem(context.Background(), state, item, 1, 1_700); err == nil || !strings.Contains(err.Error(), "latest failed") {
		t.Fatalf("expected latest query error, got %v", err)
	}

	runner = &fakeBuildingsRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}},
		results:     []sql.Result{buildingSQLResult{affected: 1}},
		execErrs:    []error{nil, errors.New("insert failed")},
	}
	repository = NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	if _, _, err := repository.enqueueShipyardItem(context.Background(), state, item, 1, 1_700); err == nil || !strings.Contains(err.Error(), "insert failed") {
		t.Fatalf("expected insert error, got %v", err)
	}
}

func TestShipyardRepositoryEnqueueShipyardItemClampsRequestedAmount(t *testing.T) {
	item := goodShipyardQueueItem(domaingame.FleetLightFighter)
	state := shipyardMutationState{playerID: 42, planetID: 99, config: shipyardMutationConfig{OrderCap: 2}}
	runner := &fakeBuildingsRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}},
		results:     []sql.Result{buildingSQLResult{affected: 1}},
	}
	repository := NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	if issue, ok, err := repository.enqueueShipyardItem(context.Background(), state, item, 5, 1_700); err != nil || !ok || issue != nil {
		t.Fatalf("expected capped enqueue success, ok=%v issue=%+v err=%v", ok, issue, err)
	}
	if runner.execs[0].args[0] != 2.0 {
		t.Fatalf("expected order cap to limit resource spend to two units, got args=%+v", runner.execs[0].args)
	}

	item.MaxBuild = 1
	runner = &fakeBuildingsRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}},
		results:     []sql.Result{buildingSQLResult{affected: 1}},
	}
	repository = NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	state.config.OrderCap = 99
	if issue, ok, err := repository.enqueueShipyardItem(context.Background(), state, item, 5, 1_700); err != nil || !ok || issue != nil {
		t.Fatalf("expected max-build capped enqueue success, ok=%v issue=%+v err=%v", ok, issue, err)
	}
	if runner.execs[0].args[0] != 1.0 {
		t.Fatalf("expected max build to limit resource spend to one unit, got args=%+v", runner.execs[0].args)
	}
}

func TestShipyardRepositoryFinishShipyardQueueTaskEdges(t *testing.T) {
	repository := NewShipyardRepositoryWithRunner(&fakeBuildingsRunner{}, &fakeBuildingsRunner{}, "ogame_", nil)
	if err := repository.finishShipyardQueueTask(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", buildingQueueTask{TaskID: 8, Level: 0}, 120); err != nil {
		t.Fatal(err)
	}

	task := buildingQueueTask{TaskID: 8, OwnerID: 42, Type: queueTypeShipyard, SubID: 99, ObjID: domaingame.FleetLightFighter, Level: 2, Start: 100, End: 200, Prio: 20}
	updateErr := errors.New("planet update failed")
	runner := &fakeBuildingsRunner{execErr: updateErr}
	repository = NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.finishShipyardQueueTask(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", task, 200); !errors.Is(err, updateErr) {
		t.Fatalf("expected planet update error, got %v", err)
	}

	statsErr := errors.New("stats failed")
	runner = &fakeBuildingsRunner{execErrs: []error{nil, statsErr}}
	repository = NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.finishShipyardQueueTask(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", task, 200); !errors.Is(err, statsErr) {
		t.Fatalf("expected stats error, got %v", err)
	}

	shrinkErr := errors.New("shrink failed")
	runner = &fakeBuildingsRunner{execErrs: []error{nil, nil, shrinkErr}}
	repository = NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.finishShipyardQueueTask(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", task, 200); !errors.Is(err, shrinkErr) {
		t.Fatalf("expected queue shrink error, got %v", err)
	}

	runner = &fakeBuildingsRunner{}
	repository = NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.finishShipyardQueueTask(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", task, 200); err != nil {
		t.Fatal(err)
	}
	if len(runner.execs) != 11 {
		t.Fatalf("expected partial long task to recalc ranks, got execs=%+v", runner.execs)
	}

	removeErr := errors.New("remove failed")
	fullTask := buildingQueueTask{TaskID: 9, OwnerID: 42, Type: queueTypeShipyard, SubID: 99, ObjID: domaingame.FleetLightFighter, Level: 1, Start: 100, End: 110}
	runner = &fakeBuildingsRunner{execErrs: []error{nil, nil, removeErr}}
	repository = NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.finishShipyardQueueTask(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", fullTask, 120); !errors.Is(err, removeErr) {
		t.Fatalf("expected remove error, got %v", err)
	}

	recalcErr := errors.New("recalc failed")
	runner = &fakeBuildingsRunner{execErrs: []error{nil, nil, nil, recalcErr}}
	repository = NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.finishShipyardQueueTask(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", fullTask, 120); !errors.Is(err, recalcErr) {
		t.Fatalf("expected recalc error, got %v", err)
	}

	unknownUnitTask := buildingQueueTask{TaskID: 10, OwnerID: 42, Type: queueTypeShipyard, SubID: 99, ObjID: 9999, Level: 1, Start: 100, End: 110}
	runner = &fakeBuildingsRunner{}
	repository = NewShipyardRepositoryWithRunner(runner, runner, "ogame_", nil)
	if err := repository.finishShipyardQueueTask(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", unknownUnitTask, 120); err != nil {
		t.Fatal(err)
	}
	if len(runner.execs) < 2 || strings.Contains(runner.execs[1].sql, "score1") {
		t.Fatalf("unknown units should not adjust score stats, execs=%+v", runner.execs)
	}
}

func TestShipyardRepositoryLoadShipyardQueueHelpers(t *testing.T) {
	repository := NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_")
	config, err := repository.loadShipyardMutationConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if config.Speed != 1 || config.OrderCap != 1000 || config.Frozen {
		t.Fatalf("unexpected default config: %+v", config)
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0.0, 0, 1})}}}, "ogame_")
	config, err = repository.loadShipyardMutationConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if config.Speed != 1 || config.OrderCap != 1000 || !config.Frozen {
		t.Fatalf("unexpected normalized config: %+v", config)
	}

	task := buildingQueueTask{TaskID: 7, OwnerID: 42, Type: queueTypeShipyard, SubID: 99, ObjID: domaingame.FleetLightFighter, Level: 2, Start: 10, End: 20, Prio: 20}
	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(buildingQueueTaskValues(task))}}}, "ogame_")
	tasks, err := repository.loadShipyardQueueTasks(context.Background(), "`ogame_queue`", 99)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].TaskID != 7 || tasks[0].ObjID != domaingame.FleetLightFighter {
		t.Fatalf("unexpected queue tasks: %+v", tasks)
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{10, 20, 3})}}}, "ogame_")
	latest, err := repository.loadShipyardLatestTime(context.Background(), "`ogame_queue`", 99, 5)
	if err != nil {
		t.Fatal(err)
	}
	if latest != 40 {
		t.Fatalf("expected latest serialized shipyard time, got %d", latest)
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("config failed")}}}, "ogame_")
	if _, err := repository.loadShipyardMutationConfig(context.Background()); err == nil || !strings.Contains(err.Error(), "config failed") {
		t.Fatalf("expected config query error, got %v", err)
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 0, 0})}}}, "ogame_")
	if _, err := repository.loadShipyardMutationConfig(context.Background()); err == nil {
		t.Fatal("expected config scan error")
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("config rows failed"), []any{1.0, 999, 0})}}}, "ogame_")
	if _, err := repository.loadShipyardMutationConfig(context.Background()); err == nil || !strings.Contains(err.Error(), "config rows failed") {
		t.Fatalf("expected config rows error, got %v", err)
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("queue failed")}}}, "ogame_")
	if _, err := repository.loadShipyardQueueTasks(context.Background(), "`ogame_queue`", 99); err == nil || !strings.Contains(err.Error(), "queue failed") {
		t.Fatalf("expected queue query error, got %v", err)
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_")
	if _, err := repository.loadShipyardQueueTasks(context.Background(), "`ogame_queue`", 99); err == nil {
		t.Fatal("expected queue scan error")
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("queue rows failed"), buildingQueueTaskValues(task))}}}, "ogame_")
	if _, err := repository.loadShipyardQueueTasks(context.Background(), "`ogame_queue`", 99); err == nil || !strings.Contains(err.Error(), "queue rows failed") {
		t.Fatalf("expected queue rows error, got %v", err)
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("latest failed")}}}, "ogame_")
	if _, err := repository.loadShipyardLatestTime(context.Background(), "`ogame_queue`", 99, 5); err == nil || !strings.Contains(err.Error(), "latest failed") {
		t.Fatalf("expected latest query error, got %v", err)
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 20, 1})}}}, "ogame_")
	if _, err := repository.loadShipyardLatestTime(context.Background(), "`ogame_queue`", 99, 5); err == nil {
		t.Fatal("expected latest scan error")
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("latest rows failed"), []any{10, 20, 1})}}}, "ogame_")
	if _, err := repository.loadShipyardLatestTime(context.Background(), "`ogame_queue`", 99, 5); err == nil || !strings.Contains(err.Error(), "latest rows failed") {
		t.Fatalf("expected latest rows error, got %v", err)
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{30, 20, 0})}}}, "ogame_")
	latest, err = repository.loadShipyardLatestTime(context.Background(), "`ogame_queue`", 99, 5)
	if err != nil {
		t.Fatal(err)
	}
	if latest != 20 {
		t.Fatalf("expected non-negative serialized latest time, got %d", latest)
	}
}

func TestShipyardRepositoryLoadDueShipyardQueueTasksEdges(t *testing.T) {
	task := buildingQueueTask{TaskID: 7, OwnerID: 42, Type: queueTypeShipyard, SubID: 99, ObjID: domaingame.FleetLightFighter, Level: 1, Start: 10, End: 20, Prio: 20}
	repository := NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(buildingQueueTaskValues(task))}}}, "ogame_")
	tasks, err := repository.loadDueShipyardQueueTasks(context.Background(), "`ogame_queue`", 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].TaskID != 7 {
		t.Fatalf("unexpected due tasks: %+v", tasks)
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("due failed")}}}, "ogame_")
	if _, err := repository.loadDueShipyardQueueTasks(context.Background(), "`ogame_queue`", 20, 1); err == nil || !strings.Contains(err.Error(), "due failed") {
		t.Fatalf("expected due query error, got %v", err)
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_")
	if _, err := repository.loadDueShipyardQueueTasks(context.Background(), "`ogame_queue`", 20, 1); err == nil {
		t.Fatal("expected due scan error")
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("due rows failed"), buildingQueueTaskValues(task))}}}, "ogame_")
	if _, err := repository.loadDueShipyardQueueTasks(context.Background(), "`ogame_queue`", 20, 1); err == nil || !strings.Contains(err.Error(), "due rows failed") {
		t.Fatalf("expected due rows error, got %v", err)
	}
}

func TestShipyardRepositoryLoadMutationUserEdges(t *testing.T) {
	repository := NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("user failed")}}}, "ogame_")
	if _, err := repository.loadShipyardMutationUser(context.Background(), "`ogame_users`", 42); err == nil || !strings.Contains(err.Error(), "user failed") {
		t.Fatalf("expected user query error, got %v", err)
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_")
	if _, err := repository.loadShipyardMutationUser(context.Background(), "`ogame_users`", 42); err == nil || !strings.Contains(err.Error(), "shipyard user not found") {
		t.Fatalf("expected missing user error, got %v", err)
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_")
	if _, err := repository.loadShipyardMutationUser(context.Background(), "`ogame_users`", 42); err == nil {
		t.Fatal("expected user scan error")
	}

	repository = NewShipyardRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("user rows failed"), shipyardMutationUserRow(0, 0, nil))}}}, "ogame_")
	if _, err := repository.loadShipyardMutationUser(context.Background(), "`ogame_users`", 42); err == nil || !strings.Contains(err.Error(), "user rows failed") {
		t.Fatalf("expected user rows error, got %v", err)
	}
}

func TestShipyardRepositoryLoadMutationStateErrors(t *testing.T) {
	tests := []struct {
		name    string
		results []fakeQueryResult
		want    string
	}{
		{name: "user", results: []fakeQueryResult{{err: errors.New("state user failed")}}, want: "state user failed"},
		{name: "config", results: []fakeQueryResult{
			{rows: fakeRowsFromValues(shipyardMutationUserRow(0, 0, nil))},
			{err: errors.New("state config failed")},
		}, want: "state config failed"},
		{name: "overview", results: []fakeQueryResult{
			{rows: fakeRowsFromValues(shipyardMutationUserRow(0, 0, nil))},
			{rows: fakeRowsFromValues([]any{1.0, 999, 0})},
			{err: errors.New("state overview failed")},
		}, want: "state overview failed"},
		{name: "levels", results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(shipyardMutationUserRow(0, 0, nil))},
			{rows: fakeRowsFromValues([]any{1.0, 999, 0})},
		}, append(shipyardOverviewResults(), fakeQueryResult{err: errors.New("state levels failed")})...), want: "state levels failed"},
		{name: "busy", results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(shipyardMutationUserRow(0, 0, nil))},
			{rows: fakeRowsFromValues([]any{1.0, 999, 0})},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(nil))},
			fakeQueryResult{err: errors.New("state busy failed")},
		)...), want: "state busy failed"},
		{name: "defense", results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(shipyardMutationUserRow(0, 0, nil))},
			{rows: fakeRowsFromValues([]any{1.0, 999, 0})},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(nil))},
			fakeQueryResult{rows: fakeRowsFromValues()},
			fakeQueryResult{err: errors.New("state defense failed")},
		)...), want: "state defense failed"},
		{name: "queue", results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(shipyardMutationUserRow(0, 0, nil))},
			{rows: fakeRowsFromValues([]any{1.0, 999, 0})},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(nil))},
			fakeQueryResult{rows: fakeRowsFromValues()},
			fakeQueryResult{rows: fakeRowsFromValues(defenseCountRow(nil))},
			fakeQueryResult{err: errors.New("state queue failed")},
		)...), want: "state queue failed"},
		{name: "fleet", results: append([]fakeQueryResult{
			{rows: fakeRowsFromValues(shipyardMutationUserRow(0, 0, nil))},
			{rows: fakeRowsFromValues([]any{1.0, 999, 0})},
		}, append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(nil))},
			fakeQueryResult{rows: fakeRowsFromValues()},
			fakeQueryResult{rows: fakeRowsFromValues(defenseCountRow(nil))},
			fakeQueryResult{rows: fakeRowsFromValues()},
			fakeQueryResult{err: errors.New("state fleet failed")},
		)...), want: "state fleet failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewShipyardRepositoryWithQueryer(&fakeQueryer{results: tt.results}, "ogame_")
			_, err := repository.loadShipyardMutationState(context.Background(), 42, 99, shipyardOrderFleet)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestShipyardRepositoryConstructorsAndClock(t *testing.T) {
	runner := &fakeBuildingsRunner{}
	shipyard := NewShipyardRepositoryWithQueryer(runner, "ogame_")
	if shipyard.queryer != runner || shipyard.execer != runner || shipyard.prefix != "ogame_" || shipyard.now == nil {
		t.Fatalf("unexpected shipyard queryer constructor state: %+v", shipyard)
	}

	fixed := time.Unix(123, 0)
	shipyard = NewShipyardRepositoryWithRunner(runner, nil, "ogame_", func() time.Time { return fixed })
	if !shipyard.currentTime().Equal(fixed) {
		t.Fatalf("expected custom shipyard clock, got %v", shipyard.currentTime())
	}
	if (ShipyardRepository{}).currentTime().IsZero() {
		t.Fatal("default shipyard clock should return current time")
	}

	defense := NewDefenseRepositoryWithQueryer(runner, "ogame_")
	if defense.queryer != runner || defense.execer != runner || defense.prefix != "ogame_" || defense.now == nil {
		t.Fatalf("unexpected defense queryer constructor state: %+v", defense)
	}

	defense = NewDefenseRepositoryWithRunner(runner, nil, "ogame_", nil)
	if defense.now == nil || defense.execer != nil {
		t.Fatalf("nil defense clock should be normalized without forcing execer, got %+v", defense)
	}
}

func TestShipyardQueueDefenseAmountClampsQueuedMissilesAndDomes(t *testing.T) {
	queueRows := []buildingQueueTask{
		{ObjID: domaingame.DefenseAntiBallisticMissile, Level: 3},
		{ObjID: domaingame.DefenseSmallShieldDome, Level: 1},
	}
	levels := domaingame.BuildingLevels{domaingame.BuildingMissileSilo: 2}
	defense := domaingame.DefenseCounts{domaingame.DefenseAntiBallisticMissile: 4, domaingame.DefenseInterplanetaryMissile: 1}

	if got := clampDefenseShipyardAmount(domaingame.DefenseAntiBallisticMissile, 99, levels, defense, queueRows); got != 11 {
		t.Fatalf("expected ABM amount to account for queued missiles, got %d", got)
	}
	if got := clampDefenseShipyardAmount(domaingame.DefenseAntiBallisticMissile, 2, levels, defense, nil); got != 2 {
		t.Fatalf("expected ABM amount under capacity to pass through, got %d", got)
	}
	if got := clampDefenseShipyardAmount(domaingame.DefenseInterplanetaryMissile, 99, levels, defense, queueRows); got != 5 {
		t.Fatalf("expected IPM amount to account for queued missiles, got %d", got)
	}
	if got := clampDefenseShipyardAmount(domaingame.DefenseSmallShieldDome, 1, levels, defense, queueRows); got != 0 {
		t.Fatalf("expected queued dome to block duplicate dome, got %d", got)
	}
	if got := clampDefenseShipyardAmount(domaingame.DefenseRocketLauncher, 7, levels, defense, queueRows); got != 7 {
		t.Fatalf("expected non-missile defense amount to pass through, got %d", got)
	}
	if got := clampDefenseShipyardAmount(domaingame.DefenseLargeShieldDome, 3, levels, domaingame.DefenseCounts{}, nil); got != 1 {
		t.Fatalf("expected dome amount to clamp to one, got %d", got)
	}
	if got := clampDefenseShipyardAmount(domaingame.DefenseLargeShieldDome, 1, levels, domaingame.DefenseCounts{domaingame.DefenseLargeShieldDome: 1}, nil); got != 0 {
		t.Fatalf("expected existing dome to block duplicate dome, got %d", got)
	}
	if got := clampDefenseShipyardAmount(domaingame.DefenseAntiBallisticMissile, 1, domaingame.BuildingLevels{}, domaingame.DefenseCounts{}, nil); got != 0 {
		t.Fatalf("expected zero silo capacity to block missiles, got %d", got)
	}
	if got := clampDefenseShipyardAmount(domaingame.DefenseAntiBallisticMissile, 0, levels, defense, queueRows); got != 0 {
		t.Fatalf("expected non-positive amount to remain zero, got %d", got)
	}
}

func TestShipyardRepositoryMutationLockNoopsWithoutSQLDB(t *testing.T) {
	repository := NewShipyardRepositoryWithRunner(&fakeQueryer{}, nil, "ogame_", time.Now)
	unlock, err := repository.acquireShipyardMutationLock(context.Background(), 42, 99)
	if err != nil {
		t.Fatal(err)
	}
	unlock()

	repository = ShipyardRepository{queryer: SQLQueryer{}}
	unlock, err = repository.acquireShipyardMutationLock(context.Background(), 42, 99)
	if err != nil {
		t.Fatal(err)
	}
	unlock()

	repository = ShipyardRepository{queryer: &SQLQueryer{}}
	unlock, err = repository.acquireShipyardMutationLock(context.Background(), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}

func TestShipyardRepositoryMutationLockUsesSQLDB(t *testing.T) {
	db := openBuildingLockTestDB(t, 1, nil)
	defer db.Close()
	repository := ShipyardRepository{queryer: SQLQueryer{DB: db}, prefix: "ogame_"}
	unlock, err := repository.acquireShipyardMutationLock(context.Background(), 42, 99)
	if err != nil {
		t.Fatal(err)
	}
	unlock()

	planetlessDB := openBuildingLockTestDB(t, 1, nil)
	defer planetlessDB.Close()
	repository = ShipyardRepository{queryer: SQLQueryer{DB: planetlessDB}, prefix: "ogame_"}
	unlock, err = repository.acquireShipyardMutationLock(context.Background(), 42, 0)
	if err != nil {
		t.Fatal(err)
	}
	unlock()

	timeoutDB := openBuildingLockTestDB(t, 0, nil)
	defer timeoutDB.Close()
	repository = ShipyardRepository{queryer: SQLQueryer{DB: timeoutDB}, prefix: "ogame_"}
	if _, err := repository.acquireShipyardMutationLock(context.Background(), 42, 99); err == nil || !strings.Contains(err.Error(), "shipyard mutation lock timeout") {
		t.Fatalf("expected lock timeout, got %v", err)
	}

	queryErrDB := openBuildingLockTestDB(t, 1, errors.New("lock query failed"))
	defer queryErrDB.Close()
	repository = ShipyardRepository{queryer: SQLQueryer{DB: queryErrDB}, prefix: "ogame_"}
	if _, err := repository.acquireShipyardMutationLock(context.Background(), 42, 99); err == nil || !strings.Contains(err.Error(), "lock query failed") {
		t.Fatalf("expected lock query error, got %v", err)
	}
}

func goodShipyardQueueItem(id int) domaingame.ShipyardItem {
	return domaingame.ShipyardItem{
		ID:               id,
		MeetsRequirement: true,
		MaxBuild:         10,
		Cost:             domaingame.BuildingCost{Metal: 1},
		DurationSeconds:  1,
		CanBuild:         true,
	}
}

func shipyardMutationPrefixResults(research map[int]int, levels map[int]int) []fakeQueryResult {
	return append([]fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1.0, 999, 0})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues(shipyardMutationUserRow(0, 0, research))},
		{rows: fakeRowsFromValues([]any{1.0, 999, 0})},
	}, append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(levels))},
		fakeQueryResult{rows: fakeRowsFromValues()},
		fakeQueryResult{rows: fakeRowsFromValues(defenseCountRow(nil))},
		fakeQueryResult{rows: fakeRowsFromValues()},
	)...)
}

func shipyardMutationUserRow(vacation int, commanderUntil int64, research map[int]int) []any {
	row := []any{vacation, commanderUntil}
	for _, id := range domaingame.ResearchIDs() {
		row = append(row, research[id])
	}
	return row
}
