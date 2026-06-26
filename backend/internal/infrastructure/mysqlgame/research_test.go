package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestResearchRepositoryReadsLegacyResearch(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(123456), 7, 99, 1, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3, 12800, 19, 4, 163, 10000.0, 10000.0, 10000.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{2})},
		{rows: fakeRowsFromValues(buildingLevelRow(map[int]int{domaingame.BuildingResearchLab: 3}))},
		{rows: fakeRowsFromValues(allResearchLevelRow(map[int]int{domaingame.ResearchEnergy: 1, domaingame.ResearchIntergalacticNetwork: 1}))},
		{rows: fakeRowsFromValues([]any{99, 3}, []any{100, 7})},
		{rows: fakeRowsFromValues([]any{2.0})},
		{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix()})},
		{rows: fakeRowsFromValues([]any{77, 99, domaingame.ResearchEnergy, 2, int(now.Unix() - 10), int(now.Unix() + 50), 0, 0})},
	}}
	repository := NewResearchRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })

	research, err := repository.GetResearch(context.Background(), appgame.ResearchQuery{PlayerID: 42})
	if err != nil {
		t.Fatal(err)
	}

	if research.Commander != "legor" || research.CurrentPlanet.ID != 99 || !research.HasLab {
		t.Fatalf("unexpected research summary: %+v", research)
	}
	if !containsResearch(research, domaingame.ResearchComputer) || !containsResearch(research, domaingame.ResearchCombustionDrive) {
		t.Fatalf("expected unlocked research rows: %+v", research.Items)
	}
	if containsResearch(research, domaingame.ResearchShield) {
		t.Fatalf("expected locked shielding technology to be hidden: %+v", research.Items)
	}
	computer := researchByID(t, research, domaingame.ResearchComputer)
	if computer.DurationSeconds != 59 {
		t.Fatalf("expected speed, technocrat, and lab-network adjusted duration, got %+v", computer)
	}
	if research.Active == nil || research.Active.TaskID != 77 || research.Active.RemainingSeconds != 50 {
		t.Fatalf("expected active research queue mapping, got %+v", research.Active)
	}
	if !strings.Contains(queryer.calls[5].sql, "`106`, `108`") || !strings.Contains(queryer.calls[8].sql, "tec_until") {
		t.Fatalf("expected legacy research and premium columns, got %+v", queryer.calls)
	}
}

func TestResearchRepositoryReadsResearchWithRunnerFinishingDueQueues(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{2.0, 0})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 99, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3, 12800, 19, 4, 163, 10000.0, 10000.0, 10000.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{2})},
		{rows: fakeRowsFromValues(buildingLevelRow(map[int]int{domaingame.BuildingResearchLab: 3}))},
		{rows: fakeRowsFromValues(allResearchLevelRow(map[int]int{domaingame.ResearchEnergy: 1}))},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{2.0})},
		{rows: fakeRowsFromValues([]any{int64(0)})},
		{rows: fakeRowsFromValues()},
	}}}
	repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	research, err := repository.GetResearch(context.Background(), appgame.ResearchQuery{PlayerID: 42, PlanetID: 99})
	if err != nil {
		t.Fatal(err)
	}
	if research.Commander != "legor" || research.CurrentPlanet.ID != 99 {
		t.Fatalf("unexpected research summary: %+v", research)
	}
	if len(runner.execs) != 0 {
		t.Fatalf("expected empty due queue to avoid writes, got %+v", runner.execs)
	}
}

func TestResearchRepositoryReturnsActiveQueueReadError(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 99, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3, 12800, 19, 4, 163, 10000.0, 10000.0, 10000.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{2})},
		{rows: fakeRowsFromValues(buildingLevelRow(map[int]int{domaingame.BuildingResearchLab: 3}))},
		{rows: fakeRowsFromValues(allResearchLevelRow(nil))},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{2.0})},
		{rows: fakeRowsFromValues([]any{int64(0)})},
		{err: errors.New("active queue failed")},
	}}
	repository := NewResearchRepositoryWithQueryer(queryer, "ogame_", time.Now)

	if _, err := repository.GetResearch(context.Background(), appgame.ResearchQuery{PlayerID: 42, PlanetID: 99}); err == nil || !strings.Contains(err.Error(), "active queue failed") {
		t.Fatalf("expected active queue error, got %v", err)
	}
}

func TestResearchRepositoryReturnsFinishDueErrorWhileReading(t *testing.T) {
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{err: errors.New("finish due failed")},
	}}}
	repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", time.Now)

	if _, err := repository.GetResearch(context.Background(), appgame.ResearchQuery{PlayerID: 42, PlanetID: 99}); err == nil || !strings.Contains(err.Error(), "finish due failed") {
		t.Fatalf("expected finish due error, got %v", err)
	}
}

func TestNewResearchRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewResearchRepository(nil, "ogame_")

	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
	if _, ok := repository.execer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL execer, got %T", repository.execer)
	}
	if repository.now == nil {
		t.Fatal("expected default clock")
	}

	withDefaultClock := NewResearchRepositoryWithQueryer(&fakeBuildingsRunner{}, "ogame_", nil)
	if withDefaultClock.execer == nil {
		t.Fatal("expected runner execer detection")
	}
	withDefaultClock = NewResearchRepositoryWithQueryer(nil, "ogame_", nil)
	if withDefaultClock.now == nil {
		t.Fatal("expected nil clock to default")
	}
}

func TestResearchRepositoryMutationLockNoopsWithoutSQLDB(t *testing.T) {
	repository := NewResearchRepositoryWithRunner(&fakeQueryer{}, nil, "ogame_", time.Now)
	unlock, err := repository.acquireResearchMutationLock(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	unlock()

	repository = ResearchRepository{queryer: SQLQueryer{}}
	unlock, err = repository.acquireResearchMutationLock(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	unlock()

	repository = ResearchRepository{queryer: &SQLQueryer{}}
	unlock, err = repository.acquireResearchMutationLock(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}

func TestResearchRepositoryMutationLockUsesSQLDB(t *testing.T) {
	db := openBuildingLockTestDB(t, 1, nil)
	defer db.Close()
	repository := ResearchRepository{queryer: SQLQueryer{DB: db}, prefix: "ogame_"}
	unlock, err := repository.acquireResearchMutationLock(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	unlock()

	timeoutDB := openBuildingLockTestDB(t, 0, nil)
	defer timeoutDB.Close()
	repository = ResearchRepository{queryer: SQLQueryer{DB: timeoutDB}, prefix: "ogame_"}
	if _, err := repository.acquireResearchMutationLock(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "research mutation lock timeout") {
		t.Fatalf("expected lock timeout, got %v", err)
	}

	queryErrDB := openBuildingLockTestDB(t, 1, errors.New("lock query failed"))
	defer queryErrDB.Close()
	repository = ResearchRepository{queryer: SQLQueryer{DB: queryErrDB}, prefix: "ogame_"}
	if _, err := repository.acquireResearchMutationLock(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "lock query failed") {
		t.Fatalf("expected lock query error, got %v", err)
	}
}

func TestResearchRepositoryStartsResearch(t *testing.T) {
	now := time.Unix(2_000, 0)
	runner := &fakeBuildingsRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{2.0, 0})},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 99, 0, 0, 0})},
			{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3, 12800, 19, 1, 163, 10_000.0, 10_000.0, 10_000.0, 0, 0, 0})},
			{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3})},
			{rows: fakeRowsFromValues([]any{1})},
			{rows: fakeRowsFromValues(researchMutationUserRow(0, 0, nil))},
			{rows: fakeRowsFromValues([]any{2.0, 0})},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues(buildingMutationPlanetRowWithFields(map[int]int{domaingame.BuildingResearchLab: 1}, domaingame.PlanetTypePlanet, 1, 163))},
			{rows: fakeRowsFromValues()},
		}},
		results: []sql.Result{buildingSQLResult{affected: 1}, buildingSQLResult{id: 7}},
	}
	repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	outcome, err := repository.MutateResearch(context.Background(), appgame.ResearchMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Action:   "start",
		TechID:   domaingame.ResearchEnergy,
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue != nil {
		t.Fatalf("unexpected research start issue: %+v", outcome.ActionIssue)
	}
	if len(runner.execs) != 2 ||
		!strings.Contains(runner.execs[0].sql, "UPDATE `ogame_planets` SET `700` = `700` - ?") ||
		!strings.Contains(runner.execs[1].sql, "INSERT INTO `ogame_queue`") ||
		runner.execs[1].args[1] != queueTypeResearch ||
		runner.execs[1].args[2] != 99 ||
		runner.execs[1].args[3] != domaingame.ResearchEnergy ||
		runner.execs[1].args[4] != 1 ||
		runner.execs[1].args[5] != int(now.Unix()) {
		t.Fatalf("unexpected research start execs: %+v", runner.execs)
	}
}

func TestResearchRepositoryStartResearchSpendFailure(t *testing.T) {
	now := time.Unix(2_000, 0)
	runner := &fakeBuildingsRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues(researchMutationUserRow(0, 0, nil))},
			{rows: fakeRowsFromValues([]any{2.0, 0})},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues(buildingMutationPlanetRowWithFields(map[int]int{domaingame.BuildingResearchLab: 1}, domaingame.PlanetTypePlanet, 1, 163))},
			{rows: fakeRowsFromValues()},
		}},
		results: []sql.Result{buildingSQLResult{affected: 0}},
	}
	repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	issue, err := repository.startResearch(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", 42, 99, domaingame.ResearchEnergy, int(now.Unix()))
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.BuildingsIssueNoResources {
		t.Fatalf("expected spend failure issue, got %+v", issue)
	}
	if len(runner.execs) != 1 {
		t.Fatalf("expected only spend write, got %+v", runner.execs)
	}
}

func TestResearchRepositoryCancelsResearchAndRefunds(t *testing.T) {
	now := time.Unix(2_000, 0)
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{2.0, 0})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 99, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3, 12800, 19, 1, 163, 10_000.0, 10_000.0, 10_000.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{1})},
		{rows: fakeRowsFromValues([]any{2.0, 0})},
		{rows: fakeRowsFromValues([]any{8, 99, domaingame.ResearchEnergy, 2, int(now.Unix() - 50), int(now.Unix() + 150), 0, 0})},
		{rows: fakeRowsFromValues(buildingMutationPlanetRowWithFields(map[int]int{domaingame.BuildingResearchLab: 1}, domaingame.PlanetTypePlanet, 1, 163))},
	}}}
	repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	outcome, err := repository.MutateResearch(context.Background(), appgame.ResearchMutationQuery{PlayerID: 42, PlanetID: 99, Action: "cancel"})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue != nil {
		t.Fatalf("unexpected research cancel issue: %+v", outcome.ActionIssue)
	}
	if len(runner.execs) != 2 ||
		!strings.Contains(runner.execs[0].sql, "UPDATE `ogame_planets` SET `700` = `700` + ?") ||
		!strings.Contains(runner.execs[1].sql, "DELETE FROM `ogame_queue` WHERE task_id = ?") ||
		runner.execs[1].args[0] != 8 {
		t.Fatalf("unexpected research cancel execs: %+v", runner.execs)
	}
}

func TestResearchRepositoryCancelsUnknownResearchWithoutRefund(t *testing.T) {
	now := time.Unix(2_000, 0)
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{2.0, 0})},
		{rows: fakeRowsFromValues([]any{8, 99, -1, 2, int(now.Unix() - 50), int(now.Unix() + 150), 0, 0})},
		{rows: fakeRowsFromValues(buildingMutationPlanetRowWithFields(map[int]int{domaingame.BuildingResearchLab: 1}, domaingame.PlanetTypePlanet, 1, 163))},
	}}}
	repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	issue, err := repository.cancelResearch(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", 42, int(now.Unix()))
	if err != nil {
		t.Fatal(err)
	}
	if issue != nil {
		t.Fatalf("unexpected cancel issue: %+v", issue)
	}
	if len(runner.execs) != 1 || !strings.Contains(runner.execs[0].sql, "DELETE FROM `ogame_queue` WHERE task_id = ?") {
		t.Fatalf("expected unknown research cancel to skip refund, got %+v", runner.execs)
	}
}

func TestResearchRepositoryCancelResearchUpdatesResourcesBranch(t *testing.T) {
	now := time.Unix(0, 0)
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{2.0, 0})},
		{rows: fakeRowsFromValues([]any{8, 99, domaingame.ResearchEnergy, 2, int(now.Unix()), int(now.Unix() + 150), 0, 0})},
		{rows: fakeRowsFromValues(buildingMutationPlanetRowWithFields(map[int]int{domaingame.BuildingResearchLab: 1}, domaingame.PlanetTypePlanet, 1, 163))},
	}}}
	repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	repository.updateResources = true

	issue, err := repository.cancelResearch(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", 42, int(now.Unix()))
	if err != nil {
		t.Fatal(err)
	}
	if issue != nil {
		t.Fatalf("unexpected cancel issue: %+v", issue)
	}
	if len(runner.execs) != 2 {
		t.Fatalf("expected refund and queue removal writes, got %+v", runner.execs)
	}
}

func TestResearchRepositoryFinishesDueResearchQueue(t *testing.T) {
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{2.0, 0})},
		{rows: fakeRowsFromValues(buildingQueueTaskValues(buildingQueueTask{
			TaskID:  11,
			OwnerID: 42,
			Type:    queueTypeResearch,
			SubID:   99,
			ObjID:   domaingame.ResearchEnergy,
			Level:   2,
			Start:   2_000,
			End:     2_050,
			Prio:    20,
		}))},
	}}}
	repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_100, 0) })

	if err := repository.FinishDueResearchQueues(context.Background(), 2_100); err != nil {
		t.Fatal(err)
	}
	if len(runner.execs) != 11 ||
		!strings.Contains(runner.execs[0].sql, fmt.Sprintf("SET `%d` = ?", domaingame.ResearchEnergy)) ||
		runner.execs[0].args[0] != 2 ||
		!strings.Contains(runner.execs[1].sql, "DELETE FROM `ogame_queue` WHERE task_id = ?") ||
		!strings.Contains(runner.execs[2].sql, "score1 = score1 + ?") {
		t.Fatalf("unexpected research finish execs: %+v", runner.execs)
	}
}

func TestResearchRepositoryFinishesTaskBranches(t *testing.T) {
	runner := &fakeBuildingsRunner{}
	repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_100, 0) })
	repository.updateResources = true

	err := repository.finishResearchQueueTask(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", buildingQueueTask{
		TaskID:  12,
		OwnerID: 42,
		SubID:   99,
		ObjID:   domaingame.ResearchEnergy,
		Level:   3,
		End:     0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.execs) != 11 || !strings.Contains(runner.execs[0].sql, fmt.Sprintf("SET `%d` = ?", domaingame.ResearchEnergy)) {
		t.Fatalf("unexpected update-resources finish execs: %+v", runner.execs)
	}

	runner = &fakeBuildingsRunner{}
	repository = NewResearchRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	err = repository.finishResearchQueueTask(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", buildingQueueTask{
		TaskID:  13,
		OwnerID: 42,
		SubID:   99,
		ObjID:   -1,
		Level:   1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.execs) != 10 || strings.Contains(runner.execs[2].sql, "score1 = score1 + ?") {
		t.Fatalf("expected unknown research score to skip ranking writes, got %+v", runner.execs)
	}
}

func TestResearchRepositoryFinishTaskPropagatesResourceUpdateError(t *testing.T) {
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{err: errors.New("resource update failed")},
	}}}
	repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	repository.updateResources = true

	err := repository.finishResearchQueueTask(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", buildingQueueTask{
		TaskID:  12,
		OwnerID: 42,
		SubID:   99,
		ObjID:   domaingame.ResearchEnergy,
		Level:   3,
		End:     2_000,
	})
	if err == nil || !strings.Contains(err.Error(), "resource update failed") {
		t.Fatalf("expected resource update error, got %v", err)
	}
}

func TestResearchRepositoryFinishTaskPropagatesWriteErrors(t *testing.T) {
	tests := []struct {
		name     string
		execErrs []error
		want     string
	}{
		{name: "remove", execErrs: []error{nil, errors.New("remove failed")}, want: "remove failed"},
		{name: "stats", execErrs: []error{nil, nil, errors.New("stats failed")}, want: "stats failed"},
		{name: "rank", execErrs: []error{nil, nil, nil, errors.New("rank failed")}, want: "rank failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeBuildingsRunner{execErrs: tt.execErrs}
			repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", time.Now)
			err := repository.finishResearchQueueTask(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", buildingQueueTask{
				TaskID:  12,
				OwnerID: 42,
				SubID:   99,
				ObjID:   domaingame.ResearchEnergy,
				Level:   3,
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestResearchRepositoryMutationIssues(t *testing.T) {
	if _, err := NewResearchRepositoryWithQueryer(&fakeQueryer{}, "ogame_", nil).MutateResearch(context.Background(), appgame.ResearchMutationQuery{}); err == nil {
		t.Fatal("expected missing execer error")
	}
	if _, err := NewResearchRepositoryWithRunner(&fakeBuildingsRunner{}, &fakeBuildingsRunner{}, "bad-prefix_", nil).MutateResearch(context.Background(), appgame.ResearchMutationQuery{}); err == nil {
		t.Fatal("expected unsafe prefix mutation error")
	}

	now := time.Unix(2_000, 0)
	t.Run("invalid action", func(t *testing.T) {
		runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: append(researchMutationPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues()})}}
		repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
		outcome, err := repository.MutateResearch(context.Background(), appgame.ResearchMutationQuery{PlayerID: 42, PlanetID: 99, Action: "bad"})
		if err != nil {
			t.Fatal(err)
		}
		if outcome.ActionIssue == nil || outcome.ActionIssue.Code != domaingame.BuildingsIssueInvalid {
			t.Fatalf("expected invalid action issue, got %+v", outcome.ActionIssue)
		}
	})

	t.Run("vacation", func(t *testing.T) {
		runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: append(researchMutationPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues(researchMutationUserRow(1, 0, nil))})}}
		repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
		outcome, err := repository.MutateResearch(context.Background(), appgame.ResearchMutationQuery{PlayerID: 42, PlanetID: 99, Action: "start", TechID: domaingame.ResearchEnergy})
		if err != nil {
			t.Fatal(err)
		}
		if outcome.ActionIssue == nil || outcome.ActionIssue.Code != domaingame.BuildingsIssueVacation {
			t.Fatalf("expected vacation issue, got %+v", outcome.ActionIssue)
		}
	})
}

func TestResearchRepositoryMutateResearchPropagatesErrors(t *testing.T) {
	t.Run("finish due", func(t *testing.T) {
		runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{err: errors.New("finish due failed")},
		}}}
		repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", time.Now)
		if _, err := repository.MutateResearch(context.Background(), appgame.ResearchMutationQuery{PlayerID: 42, PlanetID: 99, Action: "start"}); err == nil || !strings.Contains(err.Error(), "finish due failed") {
			t.Fatalf("expected finish due error, got %v", err)
		}
	})

	t.Run("overview", func(t *testing.T) {
		runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{2.0, 0})},
			{rows: fakeRowsFromValues()},
			{err: errors.New("overview failed")},
		}}}
		repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", time.Now)
		if _, err := repository.MutateResearch(context.Background(), appgame.ResearchMutationQuery{PlayerID: 42, PlanetID: 99, Action: "start"}); err == nil || !strings.Contains(err.Error(), "overview failed") {
			t.Fatalf("expected overview error, got %v", err)
		}
	})
}

func TestResearchRepositoryStartResearchIssues(t *testing.T) {
	now := time.Unix(2_000, 0)
	tests := []struct {
		name    string
		results []fakeQueryResult
		want    string
	}{
		{
			name: "universe frozen",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(researchMutationUserRow(0, 0, nil))},
				{rows: fakeRowsFromValues([]any{2.0, 1})},
			},
			want: domaingame.BuildingsIssueUniversePause,
		},
		{
			name: "already running",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(researchMutationUserRow(0, 0, nil))},
				{rows: fakeRowsFromValues([]any{2.0, 0})},
				{rows: fakeRowsFromValues([]any{8, 99, domaingame.ResearchEnergy, 1, int(now.Unix()), int(now.Unix() + 100), 0, 0})},
			},
			want: "research_already",
		},
		{
			name: "lab busy",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(researchMutationUserRow(0, 0, nil))},
				{rows: fakeRowsFromValues([]any{2.0, 0})},
				{rows: fakeRowsFromValues()},
				{rows: fakeRowsFromValues([]any{10})},
			},
			want: "research_lab_building",
		},
		{
			name: "missing planet",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(researchMutationUserRow(0, 0, nil))},
				{rows: fakeRowsFromValues([]any{2.0, 0})},
				{rows: fakeRowsFromValues()},
				{rows: fakeRowsFromValues()},
				{rows: fakeRowsFromValues()},
			},
			want: domaingame.BuildingsIssueInvalid,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: tt.results}}
			repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
			issue, err := repository.startResearch(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", 42, 99, domaingame.ResearchEnergy, int(now.Unix()))
			if err != nil {
				t.Fatal(err)
			}
			if issue == nil || issue.Code != tt.want {
				t.Fatalf("expected %q issue, got %+v", tt.want, issue)
			}
			if len(runner.execs) != 0 {
				t.Fatalf("expected no writes on issue, got %+v", runner.execs)
			}
		})
	}
}

func TestResearchRepositoryStartResearchPropagatesErrors(t *testing.T) {
	now := time.Unix(2_000, 0)
	baseRows := func() []fakeQueryResult {
		return []fakeQueryResult{
			{rows: fakeRowsFromValues(researchMutationUserRow(0, 0, nil))},
			{rows: fakeRowsFromValues([]any{2.0, 0})},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues(buildingMutationPlanetRowWithFields(map[int]int{domaingame.BuildingResearchLab: 1}, domaingame.PlanetTypePlanet, 1, 163))},
			{rows: fakeRowsFromValues()},
		}
	}
	tests := []struct {
		name     string
		results  []fakeQueryResult
		execErrs []error
		want     string
	}{
		{name: "user", results: []fakeQueryResult{{err: errors.New("research user failed")}}, want: "research user failed"},
		{name: "config", results: []fakeQueryResult{{rows: fakeRowsFromValues(researchMutationUserRow(0, 0, nil))}, {err: errors.New("config failed")}}, want: "config failed"},
		{name: "active", results: []fakeQueryResult{{rows: fakeRowsFromValues(researchMutationUserRow(0, 0, nil))}, {rows: fakeRowsFromValues([]any{2.0, 0})}, {err: errors.New("active failed")}}, want: "active failed"},
		{name: "busy", results: []fakeQueryResult{{rows: fakeRowsFromValues(researchMutationUserRow(0, 0, nil))}, {rows: fakeRowsFromValues([]any{2.0, 0})}, {rows: fakeRowsFromValues()}, {err: errors.New("busy failed")}}, want: "busy failed"},
		{name: "planet", results: []fakeQueryResult{{rows: fakeRowsFromValues(researchMutationUserRow(0, 0, nil))}, {rows: fakeRowsFromValues([]any{2.0, 0})}, {rows: fakeRowsFromValues()}, {rows: fakeRowsFromValues()}, {err: errors.New("planet failed")}}, want: "planet failed"},
		{name: "labs", results: append(baseRows()[:5], fakeQueryResult{err: errors.New("labs failed")}), want: "labs failed"},
		{name: "spend", results: baseRows(), execErrs: []error{errors.New("spend failed")}, want: "spend failed"},
		{name: "queue insert", results: baseRows(), execErrs: []error{nil, errors.New("queue insert failed")}, want: "queue insert failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: tt.results}, execErrs: tt.execErrs}
			repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
			_, err := repository.startResearch(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", 42, 99, domaingame.ResearchEnergy, int(now.Unix()))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestResearchRepositoryCancelResearchIssues(t *testing.T) {
	now := time.Unix(2_000, 0)
	t.Run("universe frozen", func(t *testing.T) {
		runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{2.0, 1})},
		}}}
		repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
		issue, err := repository.cancelResearch(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", 42, int(now.Unix()))
		if err != nil || issue != nil || len(runner.execs) != 0 {
			t.Fatalf("expected frozen cancel noop, issue=%+v err=%v execs=%+v", issue, err, runner.execs)
		}
	})

	t.Run("empty queue", func(t *testing.T) {
		runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{2.0, 0})},
			{rows: fakeRowsFromValues()},
		}}}
		repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
		issue, err := repository.cancelResearch(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", 42, int(now.Unix()))
		if err != nil || issue != nil || len(runner.execs) != 0 {
			t.Fatalf("expected empty cancel noop, issue=%+v err=%v execs=%+v", issue, err, runner.execs)
		}
	})

	t.Run("missing planet", func(t *testing.T) {
		runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{2.0, 0})},
			{rows: fakeRowsFromValues([]any{8, 99, domaingame.ResearchEnergy, 2, int(now.Unix()), int(now.Unix() + 100), 0, 0})},
			{rows: fakeRowsFromValues()},
		}}}
		repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
		issue, err := repository.cancelResearch(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", 42, int(now.Unix()))
		if err != nil {
			t.Fatal(err)
		}
		if issue == nil || issue.Code != domaingame.BuildingsIssueInvalid {
			t.Fatalf("expected invalid planet issue, got %+v", issue)
		}
	})
}

func TestResearchRepositoryCancelResearchPropagatesErrors(t *testing.T) {
	now := time.Unix(2_000, 0)
	active := func() fakeQueryResult {
		return fakeQueryResult{rows: fakeRowsFromValues([]any{8, 99, domaingame.ResearchEnergy, 2, int(now.Unix()), int(now.Unix() + 100), 0, 0})}
	}
	planet := func() fakeQueryResult {
		return fakeQueryResult{rows: fakeRowsFromValues(buildingMutationPlanetRowWithFields(map[int]int{domaingame.BuildingResearchLab: 1}, domaingame.PlanetTypePlanet, 1, 163))}
	}
	tests := []struct {
		name     string
		results  []fakeQueryResult
		execErrs []error
		want     string
	}{
		{name: "config", results: []fakeQueryResult{{err: errors.New("config failed")}}, want: "config failed"},
		{name: "active", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{2.0, 0})}, {err: errors.New("active failed")}}, want: "active failed"},
		{name: "planet", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{2.0, 0})}, active(), {err: errors.New("planet failed")}}, want: "planet failed"},
		{name: "refund", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{2.0, 0})}, active(), planet()}, execErrs: []error{errors.New("refund failed")}, want: "refund failed"},
		{name: "remove", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{2.0, 0})}, active(), planet()}, execErrs: []error{nil, errors.New("remove failed")}, want: "remove failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: tt.results}, execErrs: tt.execErrs}
			repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
			_, err := repository.cancelResearch(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_queue`", 42, int(now.Unix()))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestResearchRepositoryLoadsFrozenActiveResearchQueue(t *testing.T) {
	now := time.Unix(2_000, 0)
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{8, 100, domaingame.ResearchEnergy, 2, int(now.Unix() - 50), int(now.Unix() + 150), 1, int(now.Unix() - 20)})},
	}}
	repository := NewResearchRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })

	queue, err := repository.loadActiveResearchQueue(context.Background(), "`ogame_queue`", 42, 99, int(now.Unix()))
	if err != nil {
		t.Fatal(err)
	}
	if queue == nil || queue.RemainingSeconds != 170 || queue.Cancelable {
		t.Fatalf("unexpected frozen active queue: %+v", queue)
	}
}

func TestResearchRepositoryLoadsActiveResearchQueueWithElapsedEnd(t *testing.T) {
	now := time.Unix(2_000, 0)
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{8, 99, domaingame.ResearchEnergy, 2, int(now.Unix() - 100), int(now.Unix() - 10), 0, 0})},
	}}
	repository := NewResearchRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })

	queue, err := repository.loadActiveResearchQueue(context.Background(), "`ogame_queue`", 42, 99, int(now.Unix()))
	if err != nil {
		t.Fatal(err)
	}
	if queue == nil || queue.RemainingSeconds != 0 || !queue.Cancelable {
		t.Fatalf("unexpected elapsed active queue: %+v", queue)
	}
}

func TestResearchRepositoryLoadResearchQueueErrors(t *testing.T) {
	repository := NewResearchRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"bad"})},
	}}, "ogame_", nil)
	if _, err := repository.loadActiveResearchQueue(context.Background(), "`ogame_queue`", 42, 99, 2_000); err == nil {
		t.Fatal("expected active queue scan error")
	}

	repository = NewResearchRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues()},
	}}, "ogame_", nil)
	tasks, err := repository.loadDueResearchQueueTasks(context.Background(), "`ogame_queue`", 2_000, 0)
	if err != nil || len(tasks) != 0 {
		t.Fatalf("expected empty default-limit due tasks, tasks=%+v err=%v", tasks, err)
	}

	repository = NewResearchRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"bad"})},
	}}, "ogame_", nil)
	if _, err := repository.loadDueResearchQueueTasks(context.Background(), "`ogame_queue`", 2_000, 1); err == nil {
		t.Fatal("expected due queue scan error")
	}
}

func TestResearchRepositoryValidatesResearchOrders(t *testing.T) {
	now := time.Unix(2_000, 0)
	planet := buildingMutationPlanet{
		ID:        99,
		OwnerID:   42,
		Resources: domaingame.Resources{Metal: 10_000, Crystal: 10_000, Deuterium: 10_000},
		Levels:    domaingame.BuildingLevels{domaingame.BuildingResearchLab: 1},
	}
	user := researchMutationUser{Research: domaingame.ResearchLevels{}}
	repository := NewResearchRepositoryWithQueryer(&fakeQueryer{}, "ogame_", func() time.Time { return now })

	issue, _, _, err := repository.validateResearchOrder(context.Background(), "`ogame_planets`", user, planet, -1, 1, 1)
	if err != nil || issue == nil || issue.Code != domaingame.BuildingsIssueRequirements {
		t.Fatalf("expected requirement issue, got issue=%+v err=%v", issue, err)
	}
	issue, _, _, err = repository.validateResearchOrder(context.Background(), "`ogame_planets`", user, planet, domaingame.ResearchEnergy, 100, 1)
	if err != nil || issue == nil || issue.Code != "max_level" {
		t.Fatalf("expected max level issue, got issue=%+v err=%v", issue, err)
	}
	planet.Resources = domaingame.Resources{}
	issue, _, _, err = repository.validateResearchOrder(context.Background(), "`ogame_planets`", user, planet, domaingame.ResearchEnergy, 1, 1)
	if err != nil || issue == nil || issue.Code != domaingame.BuildingsIssueNoResources {
		t.Fatalf("expected no resources issue, got issue=%+v err=%v", issue, err)
	}

	planet.Resources = domaingame.Resources{Metal: 10_000, Crystal: 10_000, Deuterium: 10_000}
	repository = NewResearchRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues()},
	}}, "ogame_", func() time.Time { return now })
	user.TechnocratUntil = now.Add(time.Hour).Unix()
	issue, _, duration, err := repository.validateResearchOrder(context.Background(), "`ogame_planets`", user, planet, domaingame.ResearchEnergy, 1, 1)
	if err != nil || issue != nil || duration <= 0 {
		t.Fatalf("expected technocrat-adjusted valid order, issue=%+v duration=%d err=%v", issue, duration, err)
	}
}

func TestResearchRepositoryLoadResearchMutationUserErrors(t *testing.T) {
	repository := NewResearchRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues()},
	}}, "ogame_", nil)
	if _, err := repository.loadResearchMutationUser(context.Background(), "`ogame_users`", 42); err == nil || !strings.Contains(err.Error(), "research user not found") {
		t.Fatalf("expected missing research user error, got %v", err)
	}

	repository = NewResearchRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"bad"})},
	}}, "ogame_", nil)
	if _, err := repository.loadResearchMutationUser(context.Background(), "`ogame_users`", 42); err == nil {
		t.Fatal("expected research mutation user scan error")
	}
}

func TestResearchRepositoryFinishingBranches(t *testing.T) {
	if err := NewResearchRepositoryWithQueryer(&fakeQueryer{}, "ogame_", nil).FinishDueResearchQueues(context.Background(), 1); err == nil {
		t.Fatal("expected missing execer error")
	}
	if err := NewResearchRepositoryWithRunner(&fakeBuildingsRunner{}, &fakeBuildingsRunner{}, "bad-prefix_", nil).FinishDueResearchQueues(context.Background(), 1); err == nil {
		t.Fatal("expected unsafe prefix finish error")
	}

	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{2.0, 1})},
	}}}
	repository := NewResearchRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if err := repository.FinishDueResearchQueues(context.Background(), 2_100); err != nil {
		t.Fatal(err)
	}
	if len(runner.execs) != 0 {
		t.Fatalf("expected frozen universe to skip due queues, got %+v", runner.execs)
	}

	runner = &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{err: errors.New("config failed")},
	}}}
	repository = NewResearchRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if err := repository.FinishDueResearchQueues(context.Background(), 2_100); err == nil || !strings.Contains(err.Error(), "config failed") {
		t.Fatalf("expected config error, got %v", err)
	}

	runner = &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{2.0, 0})},
		{err: errors.New("due failed")},
	}}}
	repository = NewResearchRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if err := repository.FinishDueResearchQueues(context.Background(), 2_100); err == nil || !strings.Contains(err.Error(), "due failed") {
		t.Fatalf("expected due queue error, got %v", err)
	}

	runner = &fakeBuildingsRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{2.0, 0})},
			{rows: fakeRowsFromValues(buildingQueueTaskValues(buildingQueueTask{TaskID: 8, OwnerID: 42, ObjID: domaingame.ResearchEnergy, Level: 1}))},
		}},
		execErrs: []error{errors.New("finish failed")},
	}
	repository = NewResearchRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if err := repository.FinishDueResearchQueues(context.Background(), 2_100); err == nil || !strings.Contains(err.Error(), "finish failed") {
		t.Fatalf("expected finish error, got %v", err)
	}
}

func TestResearchRepositoryReturnsErrors(t *testing.T) {
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
			name:   "overview",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{err: errors.New("overview user failed")},
			}},
			want: "overview user failed",
		},
		{
			name:    "building levels",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(researchOverviewResults(), fakeQueryResult{err: errors.New("building query failed")})},
			want:    "building query failed",
		},
		{
			name:    "research query",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(append(researchOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(nil))}), fakeQueryResult{err: errors.New("research query failed")})},
			want:    "research query failed",
		},
		{
			name:    "missing research",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(append(researchOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(nil))}), fakeQueryResult{rows: fakeRowsFromValues()})},
			want:    "research levels not found",
		},
		{
			name:    "research scan",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(append(researchOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(nil))}), fakeQueryResult{rows: fakeRowsFromValues([]any{"bad"})})},
			want:    "unexpected scan destination count",
		},
		{
			name:    "research rows",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(append(researchOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(nil))}), fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("research rows failed"), allResearchLevelRow(nil))})},
			want:    "research rows failed",
		},
		{
			name:    "labs query",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(researchReadPrefixResults(), fakeQueryResult{err: errors.New("labs query failed")})},
			want:    "labs query failed",
		},
		{
			name:    "labs scan",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(researchReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{"bad", 1})})},
			want:    "expected int",
		},
		{
			name:    "labs rows",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(researchReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("labs rows failed"), []any{100, 7})})},
			want:    "labs rows failed",
		},
		{
			name:    "speed",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(researchReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues()}, fakeQueryResult{err: errors.New("speed query failed")})},
			want:    "speed query failed",
		},
		{
			name:    "technocrat query",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(researchReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues()}, fakeQueryResult{rows: fakeRowsFromValues([]any{1.0})}, fakeQueryResult{err: errors.New("premium query failed")})},
			want:    "premium query failed",
		},
		{
			name:    "missing technocrat",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(researchReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues()}, fakeQueryResult{rows: fakeRowsFromValues([]any{1.0})}, fakeQueryResult{rows: fakeRowsFromValues()})},
			want:    "research premium state not found",
		},
		{
			name:    "technocrat scan",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(researchReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues()}, fakeQueryResult{rows: fakeRowsFromValues([]any{1.0})}, fakeQueryResult{rows: fakeRowsFromValues([]any{"bad"})})},
			want:    "expected int64",
		},
		{
			name:    "technocrat rows",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(researchReadPrefixResults(), fakeQueryResult{rows: fakeRowsFromValues()}, fakeQueryResult{rows: fakeRowsFromValues([]any{1.0})}, fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("premium rows failed"), []any{int64(0)})})},
			want:    "premium rows failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewResearchRepositoryWithQueryer(tt.queryer, tt.prefix, func() time.Time { return time.Unix(1, 0) })
			_, err := repository.GetResearch(context.Background(), appgame.ResearchQuery{PlayerID: 42})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func researchOverviewResults() []fakeQueryResult {
	return []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{1})},
	}
}

func researchReadPrefixResults() []fakeQueryResult {
	return append(researchOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(map[int]int{domaingame.BuildingResearchLab: 1}))},
		fakeQueryResult{rows: fakeRowsFromValues(allResearchLevelRow(nil))},
	)
}

func researchMutationPrefixResults() []fakeQueryResult {
	return []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{2.0, 0})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 99, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3, 12800, 19, 1, 163, 10_000.0, 10_000.0, 10_000.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{1})},
	}
}

func researchMutationUserRow(vacation int, technocratUntil int64, values map[int]int) []any {
	row := []any{vacation, technocratUntil}
	return append(row, allResearchLevelRow(values)...)
}

func allResearchLevelRow(values map[int]int) []any {
	row := make([]any, 0, len(domaingame.ResearchIDs()))
	for _, id := range domaingame.ResearchIDs() {
		row = append(row, values[id])
	}
	return row
}

func researchByID(t *testing.T, research domaingame.Research, id int) domaingame.BuildingItem {
	t.Helper()
	for _, item := range research.Items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("research %d not found in %+v", id, research.Items)
	return domaingame.BuildingItem{}
}

func containsResearch(research domaingame.Research, id int) bool {
	for _, item := range research.Items {
		if item.ID == id {
			return true
		}
	}
	return false
}
