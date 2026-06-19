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

func TestBuildingsRepositoryReadsLegacyBuildings(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(123456), 7, 99, 1, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3, 12800, 19, 4, 163, 10000.0, 10000.0, 10000.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{2})},
		{rows: fakeRowsFromValues(buildingLevelRow(map[int]int{
			domaingame.BuildingMetalMine:       2,
			domaingame.BuildingDeuteriumSynth:  5,
			domaingame.BuildingRoboticsFactory: 10,
		}))},
		{rows: fakeRowsFromValues(researchLevelRow(map[int]int{
			domaingame.ResearchComputer: 10,
			domaingame.ResearchEnergy:   3,
		}))},
		{rows: fakeRowsFromValues([]any{2.0})},
	}}
	repository := NewBuildingsRepositoryWithQueryer(queryer, "ogame_")

	buildings, err := repository.GetBuildings(context.Background(), appgame.BuildingsQuery{PlayerID: 42})
	if err != nil {
		t.Fatal(err)
	}

	if buildings.Commander != "legor" || buildings.CurrentPlanet.ID != 99 {
		t.Fatalf("unexpected buildings summary: %+v", buildings)
	}
	if !containsBuilding(buildings, domaingame.BuildingFusionReactor) || !containsBuilding(buildings, domaingame.BuildingNaniteFactory) {
		t.Fatalf("expected gated buildings after matching levels: %+v", buildings.Items)
	}
	metalMine := buildingByID(t, buildings, domaingame.BuildingMetalMine)
	if metalMine.Level != 2 || metalMine.DurationSeconds != 11 {
		t.Fatalf("expected speed-adjusted metal mine, got %+v", metalMine)
	}
	if !strings.Contains(queryer.calls[4].sql, "`1`, `2`, `3`") || !strings.Contains(queryer.calls[5].sql, "`108`, `113`, `114`") {
		t.Fatalf("expected numeric legacy columns, got %+v", queryer.calls)
	}
}

func TestNewBuildingsRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewBuildingsRepository(nil, "ogame_")

	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
	if _, ok := repository.execer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL execer, got %T", repository.execer)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeBuildingsRunner{}, "ogame_")
	if repository.execer == nil {
		t.Fatalf("expected runner execer detection")
	}
	repository = NewBuildingsRepositoryWithRunner(&fakeQueryer{}, nil, "ogame_", nil)
	if repository.now == nil {
		t.Fatalf("expected nil clock to default")
	}
}

func TestBuildingsRepositoryEnqueuesBuilding(t *testing.T) {
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(buildingMutationUserRow(0, 9_999, nil))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{2.0, 0})},
		fakeQueryResult{rows: fakeRowsFromValues(buildingMutationPlanetRow(map[int]int{}))},
		fakeQueryResult{rows: fakeRowsFromValues()},
	)}, results: []sql.Result{buildingSQLResult{affected: 1}, buildingSQLResult{id: 7}, buildingSQLResult{id: 8}}}
	repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })

	outcome, err := repository.MutateBuildings(context.Background(), appgame.BuildingsMutationQuery{PlayerID: 42, PlanetID: 99, Action: domaingame.BuildingsMutationAdd, TechID: domaingame.BuildingMetalMine})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue != nil {
		t.Fatalf("unexpected building mutation issue: %+v", outcome.ActionIssue)
	}
	if len(runner.execs) != 3 ||
		!strings.Contains(runner.execs[0].sql, "UPDATE `ogame_planets` SET `700` = `700` - ?") ||
		!strings.Contains(runner.execs[1].sql, "INSERT INTO `ogame_buildqueue`") ||
		!strings.Contains(runner.execs[2].sql, "INSERT INTO `ogame_queue`") ||
		runner.execs[1].args[0] != 42 || runner.execs[1].args[1] != 99 || runner.execs[1].args[2] != 1 ||
		runner.execs[1].args[3] != domaingame.BuildingMetalMine || runner.execs[1].args[4] != 1 ||
		runner.execs[2].args[1] != queueTypeBuild || runner.execs[2].args[2] != 7 {
		t.Fatalf("unexpected building enqueue execs: %+v", runner.execs)
	}
}

func TestBuildingsRepositoryEnqueuesDemolition(t *testing.T) {
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(buildingMutationUserRow(0, 9_999, nil))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{2.0, 0})},
		fakeQueryResult{rows: fakeRowsFromValues(buildingMutationPlanetRow(map[int]int{domaingame.BuildingMetalMine: 2}))},
		fakeQueryResult{rows: fakeRowsFromValues()},
	)}, results: []sql.Result{buildingSQLResult{affected: 1}, buildingSQLResult{id: 7}, buildingSQLResult{id: 8}}}
	repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })

	outcome, err := repository.MutateBuildings(context.Background(), appgame.BuildingsMutationQuery{PlayerID: 42, PlanetID: 99, Action: domaingame.BuildingsMutationDestroy, TechID: domaingame.BuildingMetalMine})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue != nil {
		t.Fatalf("unexpected demolition mutation issue: %+v", outcome.ActionIssue)
	}
	if len(runner.execs) != 3 ||
		!strings.Contains(runner.execs[0].sql, "UPDATE `ogame_planets` SET `700` = `700` - ?") ||
		!strings.Contains(runner.execs[1].sql, "INSERT INTO `ogame_buildqueue`") ||
		!strings.Contains(runner.execs[2].sql, "INSERT INTO `ogame_queue`") ||
		runner.execs[0].args[0] != 60.0 || runner.execs[0].args[1] != 15.0 ||
		runner.execs[1].args[2] != 1 || runner.execs[1].args[3] != domaingame.BuildingMetalMine ||
		runner.execs[1].args[4] != 1 || runner.execs[1].args[5] != 1 ||
		runner.execs[2].args[1] != queueTypeDemolish || runner.execs[2].args[2] != 7 || runner.execs[2].args[4] != 1 {
		t.Fatalf("unexpected demolition enqueue execs: %+v", runner.execs)
	}
}

func TestBuildingsRepositoryDequeuesCurrentBuildingAndRefunds(t *testing.T) {
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 1, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 1, Start: 2_000, End: 2_011}))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{8})},
		fakeQueryResult{rows: fakeRowsFromValues(buildingMutationUserRow(0, 9_999, nil))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{2.0, 0})},
		fakeQueryResult{rows: fakeRowsFromValues()},
	)}, results: []sql.Result{buildingSQLResult{affected: 1}, buildingSQLResult{affected: 1}, buildingSQLResult{affected: 1}, buildingSQLResult{affected: 1}}}
	repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_005, 0) })

	outcome, err := repository.MutateBuildings(context.Background(), appgame.BuildingsMutationQuery{PlayerID: 42, PlanetID: 99, Action: domaingame.BuildingsMutationRemove, ListID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue != nil {
		t.Fatalf("unexpected building dequeue issue: %+v", outcome.ActionIssue)
	}
	if len(runner.execs) != 4 ||
		!strings.Contains(runner.execs[0].sql, "UPDATE `ogame_planets` SET `700` = `700` + ?") ||
		!strings.Contains(runner.execs[1].sql, "DELETE FROM `ogame_queue` WHERE task_id = ?") ||
		!strings.Contains(runner.execs[2].sql, "UPDATE `ogame_buildqueue` SET level = level - 1") ||
		!strings.Contains(runner.execs[3].sql, "DELETE FROM `ogame_buildqueue` WHERE id = ?") {
		t.Fatalf("unexpected building dequeue execs: %+v", runner.execs)
	}
}

func TestBuildingsRepositoryStartsNextBuildAfterCurrentCancel(t *testing.T) {
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 1, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 1, Start: 2_000, End: 2_011}))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{8})},
		fakeQueryResult{rows: fakeRowsFromValues(buildingMutationUserRow(0, 9_999, nil))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{2.0, 0})},
		fakeQueryResult{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 2, OwnerID: 42, PlanetID: 99, ListID: 2, TechID: domaingame.BuildingCrystalMine, Level: 1, Start: 0, End: 0}))},
		fakeQueryResult{rows: fakeRowsFromValues(buildingMutationPlanetRow(map[int]int{}))},
	)}, results: []sql.Result{
		buildingSQLResult{affected: 1},
		buildingSQLResult{affected: 1},
		buildingSQLResult{affected: 1},
		buildingSQLResult{affected: 1},
		buildingSQLResult{affected: 1},
		buildingSQLResult{id: 9},
		buildingSQLResult{affected: 1},
	}}
	repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_005, 0) })

	if _, err := repository.MutateBuildings(context.Background(), appgame.BuildingsMutationQuery{PlayerID: 42, PlanetID: 99, Action: domaingame.BuildingsMutationRemove, ListID: 1}); err != nil {
		t.Fatal(err)
	}
	if len(runner.execs) != 7 ||
		!strings.Contains(runner.execs[4].sql, "UPDATE `ogame_planets` SET `700` = `700` - ?") ||
		!strings.Contains(runner.execs[5].sql, "INSERT INTO `ogame_queue`") ||
		!strings.Contains(runner.execs[6].sql, "UPDATE `ogame_buildqueue` SET start = ?, end = ? WHERE id = ?") ||
		runner.execs[5].args[2] != 2 || runner.execs[6].args[2] != 2 {
		t.Fatalf("expected cancel to start next build queue, got %+v", runner.execs)
	}
}

func TestBuildingsRepositoryStartsNextDemolitionAfterCurrentCancel(t *testing.T) {
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 1, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingCrystalMine, Level: 1, Start: 2_000, End: 2_011}))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{8})},
		fakeQueryResult{rows: fakeRowsFromValues(buildingMutationUserRow(0, 9_999, nil))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{2.0, 0})},
		fakeQueryResult{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 2, OwnerID: 42, PlanetID: 99, ListID: 2, TechID: domaingame.BuildingMetalMine, Level: 1, Destroy: 1, Start: 0, End: 0}))},
		fakeQueryResult{rows: fakeRowsFromValues(buildingMutationPlanetRow(map[int]int{domaingame.BuildingMetalMine: 2}))},
	)}, results: []sql.Result{
		buildingSQLResult{affected: 1},
		buildingSQLResult{affected: 1},
		buildingSQLResult{affected: 1},
		buildingSQLResult{affected: 1},
		buildingSQLResult{affected: 1},
		buildingSQLResult{id: 9},
		buildingSQLResult{affected: 1},
	}}
	repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_005, 0) })

	if _, err := repository.MutateBuildings(context.Background(), appgame.BuildingsMutationQuery{PlayerID: 42, PlanetID: 99, Action: domaingame.BuildingsMutationRemove, ListID: 1}); err != nil {
		t.Fatal(err)
	}
	if len(runner.execs) != 7 ||
		!strings.Contains(runner.execs[4].sql, "UPDATE `ogame_planets` SET `700` = `700` - ?") ||
		!strings.Contains(runner.execs[5].sql, "INSERT INTO `ogame_queue`") ||
		runner.execs[5].args[1] != queueTypeDemolish || runner.execs[5].args[2] != 2 || runner.execs[5].args[4] != 1 {
		t.Fatalf("expected cancel to start next demolition queue, got %+v", runner.execs)
	}
}

func TestBuildingsRepositoryFinishesDueBuildQueue(t *testing.T) {
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1.0, 0})},
		{rows: fakeRowsFromValues(buildingQueueTaskValues(buildingQueueTask{TaskID: 8, OwnerID: 42, Type: queueTypeBuild, SubID: 7, ObjID: domaingame.BuildingMetalMine, Level: 1, Start: 2_000, End: 2_005, Prio: 20}))},
		{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 7, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 1, Start: 2_000, End: 2_005}))},
		{rows: fakeRowsFromValues(buildingMutationPlanetRow(map[int]int{}))},
		{rows: fakeRowsFromValues(buildingMutationUserRow(0, 0, nil))},
		{rows: fakeRowsFromValues([]any{1.0, 0})},
		{rows: fakeRowsFromValues()},
	}}}
	repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_006, 0) })

	if err := repository.FinishDueBuildingQueues(context.Background(), 2_006); err != nil {
		t.Fatal(err)
	}
	if len(runner.execs) != 4 ||
		!strings.Contains(runner.execs[0].sql, "UPDATE `ogame_planets` SET `1` = ?, fields = fields + ?") ||
		runner.execs[0].args[0] != 1 || runner.execs[0].args[1] != 1 ||
		!strings.Contains(runner.execs[1].sql, "DELETE FROM `ogame_queue` WHERE task_id = ?") ||
		!strings.Contains(runner.execs[2].sql, "DELETE FROM `ogame_buildqueue` WHERE id = ?") ||
		!strings.Contains(runner.execs[3].sql, "score1 = score1 + ?") ||
		runner.execs[3].args[0] != int64(75) {
		t.Fatalf("unexpected build finish execs: %+v", runner.execs)
	}
}

func TestBuildingsRepositoryFinishesDueDemolitionQueue(t *testing.T) {
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1.0, 0})},
		{rows: fakeRowsFromValues(buildingQueueTaskValues(buildingQueueTask{TaskID: 8, OwnerID: 42, Type: queueTypeDemolish, SubID: 7, ObjID: domaingame.BuildingMetalMine, Level: 1, Start: 2_000, End: 2_005, Prio: 20}))},
		{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 7, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 1, Destroy: 1, Start: 2_000, End: 2_005}))},
		{rows: fakeRowsFromValues(buildingMutationPlanetRow(map[int]int{domaingame.BuildingMetalMine: 2}))},
		{rows: fakeRowsFromValues(buildingMutationUserRow(0, 0, nil))},
		{rows: fakeRowsFromValues([]any{1.0, 0})},
		{rows: fakeRowsFromValues()},
	}}}
	repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_006, 0) })

	if err := repository.FinishDueBuildingQueues(context.Background(), 2_006); err != nil {
		t.Fatal(err)
	}
	if len(runner.execs) != 4 ||
		!strings.Contains(runner.execs[0].sql, "UPDATE `ogame_planets` SET `1` = ?, fields = fields + ?") ||
		runner.execs[0].args[0] != 1 || runner.execs[0].args[1] != -1 ||
		!strings.Contains(runner.execs[3].sql, "score1 = score1 - ?") ||
		runner.execs[3].args[0] != int64(112) {
		t.Fatalf("unexpected demolition finish execs: %+v", runner.execs)
	}
}

func TestBuildingsRepositoryFinishesSpecialBuildingFields(t *testing.T) {
	tests := []struct {
		name        string
		techID      int
		maxIncrease int
	}{
		{name: "terraformer", techID: domaingame.BuildingTerraformer, maxIncrease: 5},
		{name: "lunar base", techID: domaingame.BuildingLunarBase, maxIncrease: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planetType := domaingame.PlanetTypePlanet
			if tt.techID == domaingame.BuildingLunarBase {
				planetType = domaingame.PlanetTypeMoon
			}
			runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{1.0, 0})},
				{rows: fakeRowsFromValues(buildingQueueTaskValues(buildingQueueTask{TaskID: 8, OwnerID: 42, Type: queueTypeBuild, SubID: 7, ObjID: tt.techID, Level: 1, End: 2_005, Prio: 20}))},
				{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 7, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: tt.techID, Level: 1, End: 2_005}))},
				{rows: fakeRowsFromValues(buildingMutationPlanetRowWithFields(map[int]int{}, planetType, 0, 163))},
				{rows: fakeRowsFromValues(buildingMutationUserRow(0, 0, map[int]int{domaingame.ResearchEnergy: 12}))},
				{rows: fakeRowsFromValues([]any{1.0, 0})},
				{rows: fakeRowsFromValues()},
			}}}
			repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_006, 0) })

			if err := repository.FinishDueBuildingQueues(context.Background(), 2_006); err != nil {
				t.Fatal(err)
			}
			if len(runner.execs) != 4 ||
				!strings.Contains(runner.execs[0].sql, "maxfields = maxfields + ?") ||
				runner.execs[0].args[2] != tt.maxIncrease {
				t.Fatalf("expected maxfields increase for %s, got %+v", tt.name, runner.execs)
			}
		})
	}
}

func TestBuildingsRepositoryFinishesQueueCleanupBranches(t *testing.T) {
	tests := []struct {
		name    string
		results []fakeQueryResult
		wantSQL string
	}{
		{
			name: "missing build queue row removes global queue",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{1.0, 0})},
				{rows: fakeRowsFromValues(buildingQueueTaskValues(buildingQueueTask{TaskID: 8, OwnerID: 42, Type: queueTypeBuild, SubID: 7, ObjID: domaingame.BuildingMetalMine, Level: 1, End: 2_005, Prio: 20}))},
				{rows: fakeRowsFromValues()},
			},
			wantSQL: "DELETE FROM `ogame_queue` WHERE task_id = ?",
		},
		{
			name: "already applied build removes both queue rows",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{1.0, 0})},
				{rows: fakeRowsFromValues(buildingQueueTaskValues(buildingQueueTask{TaskID: 8, OwnerID: 42, Type: queueTypeBuild, SubID: 7, ObjID: domaingame.BuildingMetalMine, Level: 1, End: 2_005, Prio: 20}))},
				{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 7, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 1, End: 2_005}))},
				{rows: fakeRowsFromValues(buildingMutationPlanetRow(map[int]int{domaingame.BuildingMetalMine: 1}))},
			},
			wantSQL: "DELETE FROM `ogame_buildqueue` WHERE id = ?",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: tt.results}}
			repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_006, 0) })

			if err := repository.FinishDueBuildingQueues(context.Background(), 2_006); err != nil {
				t.Fatal(err)
			}
			if len(runner.execs) == 0 || !strings.Contains(runner.execs[len(runner.execs)-1].sql, tt.wantSQL) {
				t.Fatalf("expected final cleanup SQL %q, got %+v", tt.wantSQL, runner.execs)
			}
		})
	}
}

func TestBuildingsRepositoryGetBuildingsFinishesDueQueuesFirst(t *testing.T) {
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1.0, 0})},
		{rows: fakeRowsFromValues()},
	},
		append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues(buildingLevelRow(map[int]int{}))},
			fakeQueryResult{rows: fakeRowsFromValues(researchLevelRow(map[int]int{}))},
			fakeQueryResult{rows: fakeRowsFromValues([]any{1.0})},
		)...,
	)}}
	repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_006, 0) })

	if _, err := repository.GetBuildings(context.Background(), appgame.BuildingsQuery{PlayerID: 42, PlanetID: 99}); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) < 2 ||
		!strings.Contains(runner.calls[0].sql, "SELECT speed, freeze") ||
		!strings.Contains(runner.calls[1].sql, "FROM `ogame_queue` WHERE end <= ?") {
		t.Fatalf("expected due queues to be checked before buildings read, got %+v", runner.calls)
	}
}

func TestBuildingsRepositoryGetBuildingsReturnsFinishError(t *testing.T) {
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{err: errors.New("finish before read failed")},
	}}}
	repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", time.Now)

	if _, err := repository.GetBuildings(context.Background(), appgame.BuildingsQuery{PlayerID: 42, PlanetID: 99}); err == nil || !strings.Contains(err.Error(), "finish before read failed") {
		t.Fatalf("expected finish error before read, got %v", err)
	}
}

func TestBuildingsRepositoryFinishDueQueueBranches(t *testing.T) {
	if err := NewBuildingsRepositoryWithQueryer(&fakeQueryer{}, "ogame_").FinishDueBuildingQueues(context.Background(), 2_006); err == nil || !strings.Contains(err.Error(), "updater") {
		t.Fatalf("expected missing updater error, got %v", err)
	}
	repository := NewBuildingsRepositoryWithRunner(&fakeBuildingsRunner{}, &fakeBuildingsRunner{}, "bad-prefix_", time.Now)
	if err := repository.FinishDueBuildingQueues(context.Background(), 2_006); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected prefix error, got %v", err)
	}

	tests := []struct {
		name    string
		results []fakeQueryResult
		want    string
	}{
		{
			name:    "frozen universe skips queue query",
			results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1.0, 1})}},
		},
		{
			name: "due queue query error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{1.0, 0})},
				{err: errors.New("due query failed")},
			},
			want: "due query failed",
		},
		{
			name: "due queue scan error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{1.0, 0})},
				{rows: fakeRowsFromValues([]any{"bad"})},
			},
			want: "unexpected scan destination count",
		},
		{
			name: "due queue rows error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{1.0, 0})},
				{rows: fakeRowsFromValuesWithErr(errors.New("due rows failed"), buildingQueueTaskValues(buildingQueueTask{TaskID: 8}))},
			},
			want: "due rows failed",
		},
		{
			name: "task finish error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{1.0, 0})},
				{rows: fakeRowsFromValues(buildingQueueTaskValues(buildingQueueTask{TaskID: 8, OwnerID: 42, Type: queueTypeBuild, SubID: 7, ObjID: domaingame.BuildingMetalMine, Level: 1, End: 2_005, Prio: 20}))},
				{err: errors.New("task finish failed")},
			},
			want: "task finish failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: tt.results}}
			repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", time.Now)
			err := repository.FinishDueBuildingQueues(context.Background(), 2_006)
			if tt.want != "" {
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("expected %q error, got %v", tt.want, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if tt.want == "" && len(runner.calls) != 1 {
				t.Fatalf("expected frozen universe to skip due queue query, got %+v", runner.calls)
			}
		})
	}
}

func TestBuildingsRepositoryFinishBuildingTaskBranches(t *testing.T) {
	task := buildingQueueTask{TaskID: 8, OwnerID: 42, Type: queueTypeBuild, SubID: 7, ObjID: domaingame.BuildingMetalMine, Level: 1, End: 2_005}
	tests := []struct {
		name     string
		results  []fakeQueryResult
		execErrs []error
		want     string
		wantExec int
	}{
		{
			name:     "buildqueue query error",
			results:  []fakeQueryResult{{err: errors.New("buildqueue id failed")}},
			want:     "buildqueue id failed",
			wantExec: 0,
		},
		{
			name: "buildqueue scan error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"bad"})},
			},
			want:     "unexpected scan destination count",
			wantExec: 0,
		},
		{
			name: "buildqueue rows error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValuesWithErr(errors.New("buildqueue id rows failed"), buildQueueRowValues(buildQueueRow{ID: 7}))},
			},
			want:     "buildqueue id rows failed",
			wantExec: 0,
		},
		{
			name: "planet missing cleans stale rows",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 7, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 1}))},
				{rows: fakeRowsFromValues()},
			},
			wantExec: 2,
		},
		{
			name: "planet query error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 7, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 1}))},
				{err: errors.New("finish planet failed")},
			},
			want:     "finish planet failed",
			wantExec: 0,
		},
		{
			name: "completion update error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 7, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 1}))},
				{rows: fakeRowsFromValues(buildingMutationPlanetRow(map[int]int{}))},
			},
			execErrs: []error{errors.New("completion update failed")},
			want:     "completion update failed",
			wantExec: 1,
		},
		{
			name: "global queue delete error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 7, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 1}))},
				{rows: fakeRowsFromValues(buildingMutationPlanetRow(map[int]int{}))},
			},
			execErrs: []error{nil, errors.New("global delete failed")},
			want:     "global delete failed",
			wantExec: 2,
		},
		{
			name: "buildqueue delete error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 7, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 1}))},
				{rows: fakeRowsFromValues(buildingMutationPlanetRow(map[int]int{}))},
			},
			execErrs: []error{nil, nil, errors.New("buildqueue delete failed")},
			want:     "buildqueue delete failed",
			wantExec: 3,
		},
		{
			name: "stats update error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 7, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 1}))},
				{rows: fakeRowsFromValues(buildingMutationPlanetRow(map[int]int{}))},
			},
			execErrs: []error{nil, nil, nil, errors.New("stats failed")},
			want:     "stats failed",
			wantExec: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: tt.results}, execErrs: tt.execErrs}
			repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", time.Now)
			err := repository.finishBuildingQueueTask(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_buildqueue`", "`ogame_queue`", task)
			if tt.want != "" {
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("expected %q error, got %v", tt.want, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if len(runner.execs) != tt.wantExec {
				t.Fatalf("expected %d execs, got %+v", tt.wantExec, runner.execs)
			}
		})
	}
}

func TestBuildingsRepositoryHighLevelFinishRecalculatesRanks(t *testing.T) {
	task := buildingQueueTask{TaskID: 8, OwnerID: 42, Type: queueTypeBuild, SubID: 7, ObjID: domaingame.BuildingMetalMine, Level: 11, End: 2_005}
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 7, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 11}))},
		{rows: fakeRowsFromValues(buildingMutationPlanetRow(map[int]int{domaingame.BuildingMetalMine: 10}))},
		{rows: fakeRowsFromValues(buildingMutationUserRow(0, 0, nil))},
		{rows: fakeRowsFromValues([]any{1.0, 0})},
		{rows: fakeRowsFromValues()},
	}}}
	repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", time.Now)

	if err := repository.finishBuildingQueueTask(context.Background(), "`ogame_users`", "`ogame_planets`", "`ogame_buildqueue`", "`ogame_queue`", task); err != nil {
		t.Fatal(err)
	}
	if len(runner.execs) != 12 ||
		!strings.Contains(runner.execs[4].sql, "UPDATE `ogame_users` SET score1 = -1") ||
		!strings.Contains(runner.execs[11].sql, "UPDATE `ogame_users` SET place1 = 0") {
		t.Fatalf("expected high-level completion to recalculate ranks, got %+v", runner.execs)
	}
}

func TestBuildingsRepositoryMutationIssuesAndErrors(t *testing.T) {
	if _, err := NewBuildingsRepositoryWithQueryer(&fakeQueryer{}, "ogame_").MutateBuildings(context.Background(), appgame.BuildingsMutationQuery{}); err == nil || !strings.Contains(err.Error(), "updater") {
		t.Fatalf("expected missing updater error, got %v", err)
	}

	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(buildingMutationUserRow(1, 0, nil))},
	)}}
	repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	outcome, err := repository.MutateBuildings(context.Background(), appgame.BuildingsMutationQuery{PlayerID: 42, PlanetID: 99, Action: domaingame.BuildingsMutationAdd, TechID: domaingame.BuildingMetalMine})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue == nil || outcome.ActionIssue.Code != domaingame.BuildingsIssueVacation || len(runner.execs) != 0 {
		t.Fatalf("expected vacation issue without writes, got outcome=%+v execs=%+v", outcome, runner.execs)
	}

	runner = &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(buildingMutationUserRow(0, 0, nil))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{1.0, 0})},
		fakeQueryResult{rows: fakeRowsFromValues(buildingMutationPlanetRow(map[int]int{}))},
		fakeQueryResult{rows: fakeRowsFromValues()},
	)}}
	repository = NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	outcome, err = repository.MutateBuildings(context.Background(), appgame.BuildingsMutationQuery{PlayerID: 42, PlanetID: 99, Action: domaingame.BuildingsMutationAdd, TechID: 9999})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue == nil || outcome.ActionIssue.Code != domaingame.BuildingsIssueInvalid || len(runner.execs) != 0 {
		t.Fatalf("expected invalid issue without writes, got outcome=%+v execs=%+v", outcome, runner.execs)
	}
}

func TestBuildingsRepositoryEnqueueValidationIssues(t *testing.T) {
	tests := []struct {
		name    string
		userRow []any
		uniRow  []any
		planet  []any
		queue   *fakeRows
		now     int64
		techID  int
		want    string
	}{
		{
			name:    "universe paused",
			userRow: buildingMutationUserRow(0, 0, nil),
			uniRow:  []any{1.0, 1},
			want:    domaingame.BuildingsIssueUniversePause,
		},
		{
			name:    "queue full",
			userRow: buildingMutationUserRow(0, 0, nil),
			uniRow:  []any{1.0, 0},
			planet:  buildingMutationPlanetRow(map[int]int{}),
			queue:   fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 1, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 1})),
			want:    domaingame.BuildingsIssueQueueFull,
		},
		{
			name:    "same second",
			userRow: buildingMutationUserRow(0, 9_999, nil),
			uniRow:  []any{1.0, 0},
			planet:  buildingMutationPlanetRow(map[int]int{}),
			queue:   fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 1, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 1, Start: 2_000})),
			now:     2_000,
			want:    domaingame.BuildingsIssueSameSecond,
		},
		{
			name:    "missing resources",
			userRow: buildingMutationUserRow(0, 0, nil),
			uniRow:  []any{1.0, 0},
			planet:  buildingMutationPlanetRowWithResources(map[int]int{}, 0, 0, 0),
			queue:   fakeRowsFromValues(),
			want:    domaingame.BuildingsIssueNoResources,
		},
		{
			name:    "missing requirements",
			userRow: buildingMutationUserRow(0, 0, nil),
			uniRow:  []any{1.0, 0},
			planet:  buildingMutationPlanetRow(map[int]int{}),
			queue:   fakeRowsFromValues(),
			techID:  domaingame.BuildingFusionReactor,
			want:    domaingame.BuildingsIssueRequirements,
		},
		{
			name:    "no fields",
			userRow: buildingMutationUserRow(0, 0, nil),
			uniRow:  []any{1.0, 0},
			planet:  buildingMutationPlanetRowWithFields(map[int]int{}, domaingame.PlanetTypePlanet, 163, 163),
			queue:   fakeRowsFromValues(),
			want:    domaingame.BuildingsIssueNoSpace,
		},
		{
			name:    "not allowed on planet type",
			userRow: buildingMutationUserRow(0, 0, nil),
			uniRow:  []any{1.0, 0},
			planet:  buildingMutationPlanetRowWithFields(map[int]int{}, domaingame.PlanetTypePlanet, 0, 163),
			queue:   fakeRowsFromValues(),
			techID:  domaingame.BuildingLunarBase,
			want:    domaingame.BuildingsIssueInvalid,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := tt.now
			if now == 0 {
				now = 2_000
			}
			techID := tt.techID
			if techID == 0 {
				techID = domaingame.BuildingMetalMine
			}
			results := append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues(tt.userRow)}, fakeQueryResult{rows: fakeRowsFromValues(tt.uniRow)})
			if tt.planet != nil {
				results = append(results, fakeQueryResult{rows: fakeRowsFromValues(tt.planet)})
			}
			if tt.queue != nil {
				results = append(results, fakeQueryResult{rows: tt.queue})
			}
			runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: results}}
			repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(now, 0) })
			outcome, err := repository.MutateBuildings(context.Background(), appgame.BuildingsMutationQuery{PlayerID: 42, PlanetID: 99, Action: domaingame.BuildingsMutationAdd, TechID: techID})
			if err != nil {
				t.Fatal(err)
			}
			if outcome.ActionIssue == nil || outcome.ActionIssue.Code != tt.want {
				t.Fatalf("expected issue %q, got %+v", tt.want, outcome.ActionIssue)
			}
			if len(runner.execs) != 0 {
				t.Fatalf("validation issue should not write, got %+v", runner.execs)
			}
		})
	}
}

func TestBuildingsRepositoryDemolitionValidationIssues(t *testing.T) {
	tests := []struct {
		name   string
		levels map[int]int
		techID int
		want   string
	}{
		{
			name:   "no such building",
			levels: map[int]int{},
			techID: domaingame.BuildingMetalMine,
			want:   domaingame.BuildingsIssueNoSuchBuilding,
		},
		{
			name:   "terraformer cannot be demolished",
			levels: map[int]int{domaingame.BuildingTerraformer: 1, domaingame.BuildingNaniteFactory: 1},
			techID: domaingame.BuildingTerraformer,
			want:   domaingame.BuildingsIssueCannotDemolish,
		},
		{
			name:   "lunar base cannot be demolished",
			levels: map[int]int{domaingame.BuildingLunarBase: 1},
			techID: domaingame.BuildingLunarBase,
			want:   domaingame.BuildingsIssueCannotDemolish,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planetType := domaingame.PlanetTypePlanet
			if tt.techID == domaingame.BuildingLunarBase {
				planetType = domaingame.PlanetTypeMoon
			}
			runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: append(shipyardOverviewResults(),
				fakeQueryResult{rows: fakeRowsFromValues(buildingMutationUserRow(0, 0, map[int]int{domaingame.ResearchEnergy: 12}))},
				fakeQueryResult{rows: fakeRowsFromValues([]any{1.0, 0})},
				fakeQueryResult{rows: fakeRowsFromValues(buildingMutationPlanetRowWithFields(tt.levels, planetType, 1, 163))},
				fakeQueryResult{rows: fakeRowsFromValues()},
			)}}
			repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
			outcome, err := repository.MutateBuildings(context.Background(), appgame.BuildingsMutationQuery{PlayerID: 42, PlanetID: 99, Action: domaingame.BuildingsMutationDestroy, TechID: tt.techID})
			if err != nil {
				t.Fatal(err)
			}
			if outcome.ActionIssue == nil || outcome.ActionIssue.Code != tt.want {
				t.Fatalf("expected issue %q, got %+v", tt.want, outcome.ActionIssue)
			}
			if len(runner.execs) != 0 {
				t.Fatalf("demolition validation issue should not write, got %+v", runner.execs)
			}
		})
	}
}

func TestBuildingsRepositoryEnqueueReadBranches(t *testing.T) {
	tests := []struct {
		name     string
		results  []fakeQueryResult
		wantErr  string
		wantCode string
	}{
		{
			name:    "user query",
			results: []fakeQueryResult{{err: errors.New("enqueue user failed")}},
			wantErr: "enqueue user failed",
		},
		{
			name: "config query",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildingMutationUserRow(0, 0, nil))},
				{err: errors.New("enqueue config failed")},
			},
			wantErr: "enqueue config failed",
		},
		{
			name: "planet query",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildingMutationUserRow(0, 0, nil))},
				{rows: fakeRowsFromValues([]any{1.0, 0})},
				{err: errors.New("enqueue planet failed")},
			},
			wantErr: "enqueue planet failed",
		},
		{
			name: "missing planet",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildingMutationUserRow(0, 0, nil))},
				{rows: fakeRowsFromValues([]any{1.0, 0})},
				{rows: fakeRowsFromValues()},
			},
			wantCode: domaingame.BuildingsIssueInvalid,
		},
		{
			name: "queue rows",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildingMutationUserRow(0, 0, nil))},
				{rows: fakeRowsFromValues([]any{1.0, 0})},
				{rows: fakeRowsFromValues(buildingMutationPlanetRow(map[int]int{}))},
				{err: errors.New("enqueue queue failed")},
			},
			wantErr: "enqueue queue failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: append(shipyardOverviewResults(), tt.results...)}}
			repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
			outcome, err := repository.MutateBuildings(context.Background(), appgame.BuildingsMutationQuery{PlayerID: 42, PlanetID: 99, Action: domaingame.BuildingsMutationAdd, TechID: domaingame.BuildingMetalMine})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected %q error, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if tt.wantCode != "" {
				if outcome.ActionIssue == nil || outcome.ActionIssue.Code != tt.wantCode {
					t.Fatalf("expected issue %q, got %+v", tt.wantCode, outcome.ActionIssue)
				}
			}
			if len(runner.execs) != 0 {
				t.Fatalf("read branch should not write, got %+v", runner.execs)
			}
		})
	}
}

func TestBuildingsRepositoryBusyQueueValidation(t *testing.T) {
	repository := NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}, "ogame_")
	issue, _, _, err := repository.validateBuildingOrder(
		context.Background(),
		"ogame_queue",
		buildingMutationUser{},
		buildingMutationPlanet{
			ID:        99,
			OwnerID:   42,
			Type:      domaingame.PlanetTypePlanet,
			MaxFields: 163,
			Resources: domaingame.Resources{Metal: 1_000, Crystal: 1_000, Deuterium: 1_000},
			Levels:    domaingame.BuildingLevels{},
		},
		domaingame.BuildingResearchLab,
		1,
		false,
		true,
		1,
	)
	if err != nil || issue == nil || issue.Code != domaingame.BuildingsIssueBusy {
		t.Fatalf("expected busy research lab issue, issue=%+v err=%v", issue, err)
	}
}

func TestBuildingsRepositoryMutationHelperEdges(t *testing.T) {
	repository := NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("user query failed")}}}, "ogame_")
	if _, err := repository.loadBuildingMutationUser(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "user query failed") {
		t.Fatalf("expected user query error, got %v", err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("user empty rows failed"))}}}, "ogame_")
	if _, err := repository.loadBuildingMutationUser(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "user empty rows failed") {
		t.Fatalf("expected user empty rows error, got %v", err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_")
	if _, err := repository.loadBuildingMutationUser(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
		t.Fatalf("expected user scan error, got %v", err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_")
	if _, err := repository.loadBuildingMutationUser(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "building user not found") {
		t.Fatalf("expected user not found error, got %v", err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("user rows failed"), buildingMutationUserRow(0, 0, nil))}}}, "ogame_")
	if _, err := repository.loadBuildingMutationUser(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "user rows failed") {
		t.Fatalf("expected user rows error, got %v", err)
	}

	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("planet query failed")}}}, "ogame_")
	if _, err := repository.loadBuildingMutationPlanet(context.Background(), "ogame_planets", 42, 99); err == nil || !strings.Contains(err.Error(), "planet query failed") {
		t.Fatalf("expected planet query error, got %v", err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_")
	if planet, err := repository.loadBuildingMutationPlanet(context.Background(), "ogame_planets", 42, 99); err != nil || planet.ID != 0 {
		t.Fatalf("expected missing planet, got planet=%+v err=%v", planet, err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("planet empty rows failed"))}}}, "ogame_")
	if _, err := repository.loadBuildingMutationPlanet(context.Background(), "ogame_planets", 42, 99); err == nil || !strings.Contains(err.Error(), "planet empty rows failed") {
		t.Fatalf("expected planet empty rows error, got %v", err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_")
	if _, err := repository.loadBuildingMutationPlanet(context.Background(), "ogame_planets", 42, 99); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
		t.Fatalf("expected planet scan error, got %v", err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("planet rows failed"), buildingMutationPlanetRow(map[int]int{}))}}}, "ogame_")
	if _, err := repository.loadBuildingMutationPlanet(context.Background(), "ogame_planets", 42, 99); err == nil || !strings.Contains(err.Error(), "planet rows failed") {
		t.Fatalf("expected planet rows error, got %v", err)
	}

	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_")
	config, err := repository.loadBuildingUniverseConfig(context.Background())
	if err != nil || config.Speed != 1 || config.Frozen {
		t.Fatalf("expected default universe config, got %+v err=%v", config, err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0.0, 1})}}}, "ogame_")
	config, err = repository.loadBuildingUniverseConfig(context.Background())
	if err != nil || config.Speed != 1 || !config.Frozen {
		t.Fatalf("expected normalized frozen universe config, got %+v err=%v", config, err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("uni query failed")}}}, "ogame_")
	if _, err := repository.loadBuildingUniverseConfig(context.Background()); err == nil || !strings.Contains(err.Error(), "uni query failed") {
		t.Fatalf("expected universe query error, got %v", err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("uni rows failed"))}}}, "ogame_")
	if _, err := repository.loadBuildingUniverseConfig(context.Background()); err == nil || !strings.Contains(err.Error(), "uni rows failed") {
		t.Fatalf("expected universe rows error, got %v", err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 0})}}}, "ogame_")
	if _, err := repository.loadBuildingUniverseConfig(context.Background()); err == nil || !strings.Contains(err.Error(), "expected float64") {
		t.Fatalf("expected universe scan error, got %v", err)
	}
}

func TestBuildingsRepositoryQueueHelperEdges(t *testing.T) {
	repository := NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("queue rows query failed")}}}, "ogame_")
	if _, err := repository.loadBuildQueueRows(context.Background(), "ogame_buildqueue", 99); err == nil || !strings.Contains(err.Error(), "queue rows query failed") {
		t.Fatalf("expected queue rows query error, got %v", err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_")
	if _, err := repository.loadBuildQueueRows(context.Background(), "ogame_buildqueue", 99); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
		t.Fatalf("expected queue rows scan error, got %v", err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("queue rows failed"), buildQueueRowValues(buildQueueRow{ID: 1}))}}}, "ogame_")
	if _, err := repository.loadBuildQueueRows(context.Background(), "ogame_buildqueue", 99); err == nil || !strings.Contains(err.Error(), "queue rows failed") {
		t.Fatalf("expected queue rows post error, got %v", err)
	}

	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("queue row query failed")}}}, "ogame_")
	if _, err := repository.loadBuildQueueRow(context.Background(), "ogame_buildqueue", 42, 99, 1); err == nil || !strings.Contains(err.Error(), "queue row query failed") {
		t.Fatalf("expected queue row query error, got %v", err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_")
	if row, err := repository.loadBuildQueueRow(context.Background(), "ogame_buildqueue", 42, 99, 1); err != nil || row != nil {
		t.Fatalf("expected missing queue row, got row=%+v err=%v", row, err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("queue row empty failed"))}}}, "ogame_")
	if _, err := repository.loadBuildQueueRow(context.Background(), "ogame_buildqueue", 42, 99, 1); err == nil || !strings.Contains(err.Error(), "queue row empty failed") {
		t.Fatalf("expected queue row empty error, got %v", err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_")
	if _, err := repository.loadBuildQueueRow(context.Background(), "ogame_buildqueue", 42, 99, 1); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
		t.Fatalf("expected queue row scan error, got %v", err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("queue row post failed"), buildQueueRowValues(buildQueueRow{ID: 1}))}}}, "ogame_")
	if _, err := repository.loadBuildQueueRow(context.Background(), "ogame_buildqueue", 42, 99, 1); err == nil || !strings.Contains(err.Error(), "queue row post failed") {
		t.Fatalf("expected queue row post error, got %v", err)
	}

	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("active task query failed")}}}, "ogame_")
	if _, err := repository.loadActiveBuildQueueTask(context.Background(), "ogame_queue", 1); err == nil || !strings.Contains(err.Error(), "active task query failed") {
		t.Fatalf("expected active task query error, got %v", err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_")
	if id, err := repository.loadActiveBuildQueueTask(context.Background(), "ogame_queue", 1); err != nil || id != 0 {
		t.Fatalf("expected missing active task, got id=%d err=%v", id, err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("active task empty failed"))}}}, "ogame_")
	if _, err := repository.loadActiveBuildQueueTask(context.Background(), "ogame_queue", 1); err == nil || !strings.Contains(err.Error(), "active task empty failed") {
		t.Fatalf("expected active task empty error, got %v", err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_")
	if _, err := repository.loadActiveBuildQueueTask(context.Background(), "ogame_queue", 1); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected active task scan error, got %v", err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("active task post failed"), []any{8})}}}, "ogame_")
	if _, err := repository.loadActiveBuildQueueTask(context.Background(), "ogame_queue", 1); err == nil || !strings.Contains(err.Error(), "active task post failed") {
		t.Fatalf("expected active task post error, got %v", err)
	}

	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("busy query failed")}}}, "ogame_")
	if _, err := repository.queueExists(context.Background(), "ogame_queue", "owner_id = ?", 42); err == nil || !strings.Contains(err.Error(), "busy query failed") {
		t.Fatalf("expected busy query error, got %v", err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("busy rows failed"), []any{1})}}}, "ogame_")
	if _, err := repository.queueExists(context.Background(), "ogame_queue", "owner_id = ?", 42); err == nil || !strings.Contains(err.Error(), "busy rows failed") {
		t.Fatalf("expected busy rows error, got %v", err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}, "ogame_")
	busy, err := repository.buildingBlocksOnBusyQueue(context.Background(), "ogame_queue", domaingame.BuildingShipyard, 99, 42)
	if err != nil || !busy {
		t.Fatalf("expected shipyard busy branch, busy=%v err=%v", busy, err)
	}
	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{}, "ogame_")
	busy, err = repository.buildingBlocksOnBusyQueue(context.Background(), "ogame_queue", domaingame.BuildingMetalMine, 99, 42)
	if err != nil || busy {
		t.Fatalf("expected default building to ignore queues, busy=%v err=%v", busy, err)
	}

	runner := &fakeBuildingsRunner{}
	repository = NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if err := repository.removeGlobalQueue(context.Background(), "ogame_queue", 0); err != nil || len(runner.execs) != 0 {
		t.Fatalf("expected empty global queue remove no-op, err=%v execs=%+v", err, runner.execs)
	}
}

func TestBuildingsRepositoryMutationBranches(t *testing.T) {
	runner := &fakeBuildingsRunner{}
	repository := NewBuildingsRepositoryWithRunner(runner, runner, "bad-prefix_", time.Now)
	if _, err := repository.MutateBuildings(context.Background(), appgame.BuildingsMutationQuery{PlayerID: 42}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected unsafe prefix error, got %v", err)
	}

	runner = &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: shipyardOverviewResults()}}
	repository = NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	outcome, err := repository.MutateBuildings(context.Background(), appgame.BuildingsMutationQuery{PlayerID: 42, Action: "noop"})
	if err != nil || outcome.ActionIssue != nil || len(runner.execs) != 0 {
		t.Fatalf("expected unknown action no-op, outcome=%+v err=%v execs=%+v", outcome, err, runner.execs)
	}

	runner = &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: shipyardOverviewResults()}}
	repository = NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	outcome, err = repository.MutateBuildings(context.Background(), appgame.BuildingsMutationQuery{PlayerID: 42, Action: domaingame.BuildingsMutationRemove})
	if err != nil || outcome.ActionIssue != nil || len(runner.execs) != 0 {
		t.Fatalf("expected empty remove no-op, outcome=%+v err=%v execs=%+v", outcome, err, runner.execs)
	}

	runner = &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("overview failed")}}}}
	repository = NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.MutateBuildings(context.Background(), appgame.BuildingsMutationQuery{PlayerID: 42, Action: domaingame.BuildingsMutationAdd}); err == nil || !strings.Contains(err.Error(), "overview failed") {
		t.Fatalf("expected overview error, got %v", err)
	}
}

func TestBuildingsRepositoryEnqueueWriteBranches(t *testing.T) {
	tests := []struct {
		name     string
		results  []sql.Result
		execErrs []error
		wantErr  string
		wantCode string
		wantExec int
	}{
		{
			name:     "spend affected none",
			results:  []sql.Result{buildingSQLResult{affected: 0}},
			wantCode: domaingame.BuildingsIssueNoResources,
			wantExec: 1,
		},
		{
			name:     "spend exec",
			execErrs: []error{errors.New("spend failed")},
			wantErr:  "spend failed",
			wantExec: 1,
		},
		{
			name:     "spend rows affected ignored",
			results:  []sql.Result{buildingSQLResult{rowsErr: errors.New("affected unavailable")}, buildingSQLResult{id: 7}, buildingSQLResult{id: 8}},
			wantExec: 3,
		},
		{
			name:     "insert build exec",
			results:  []sql.Result{buildingSQLResult{affected: 1}},
			execErrs: []error{nil, errors.New("insert build failed")},
			wantErr:  "insert build failed",
			wantExec: 2,
		},
		{
			name:     "insert build last id",
			results:  []sql.Result{buildingSQLResult{affected: 1}, buildingSQLResult{lastErr: errors.New("last id failed")}},
			wantErr:  "last id failed",
			wantExec: 2,
		},
		{
			name:     "insert build empty id",
			results:  []sql.Result{buildingSQLResult{affected: 1}, buildingSQLResult{id: 0}},
			wantErr:  "empty build queue id",
			wantExec: 2,
		},
		{
			name:     "insert global exec",
			results:  []sql.Result{buildingSQLResult{affected: 1}, buildingSQLResult{id: 7}},
			execErrs: []error{nil, nil, errors.New("insert global failed")},
			wantErr:  "insert global failed",
			wantExec: 3,
		},
		{
			name:     "insert global last id",
			results:  []sql.Result{buildingSQLResult{affected: 1}, buildingSQLResult{id: 7}, buildingSQLResult{lastErr: errors.New("global id failed")}},
			wantErr:  "global id failed",
			wantExec: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeBuildingsRunner{
				fakeQueryer: fakeQueryer{results: append(shipyardOverviewResults(),
					fakeQueryResult{rows: fakeRowsFromValues(buildingMutationUserRow(0, 9_999, nil))},
					fakeQueryResult{rows: fakeRowsFromValues([]any{2.0, 0})},
					fakeQueryResult{rows: fakeRowsFromValues(buildingMutationPlanetRow(map[int]int{}))},
					fakeQueryResult{rows: fakeRowsFromValues()},
				)},
				results:  append([]sql.Result(nil), tt.results...),
				execErrs: append([]error(nil), tt.execErrs...),
			}
			repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
			outcome, err := repository.MutateBuildings(context.Background(), appgame.BuildingsMutationQuery{PlayerID: 42, PlanetID: 99, Action: domaingame.BuildingsMutationAdd, TechID: domaingame.BuildingMetalMine})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected %q error, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if tt.wantCode != "" {
				if outcome.ActionIssue == nil || outcome.ActionIssue.Code != tt.wantCode {
					t.Fatalf("expected issue %q, got %+v", tt.wantCode, outcome.ActionIssue)
				}
			} else if outcome.ActionIssue != nil {
				t.Fatalf("unexpected action issue: %+v", outcome.ActionIssue)
			}
			if len(runner.execs) != tt.wantExec {
				t.Fatalf("expected %d execs, got %+v", tt.wantExec, runner.execs)
			}
		})
	}
}

func TestBuildingsRepositoryDequeueBranches(t *testing.T) {
	tests := []struct {
		name      string
		results   []fakeQueryResult
		sqlResult []sql.Result
		execErrs  []error
		wantErr   string
		wantExec  int
	}{
		{
			name:    "missing row",
			results: []fakeQueryResult{{rows: fakeRowsFromValues()}},
		},
		{
			name: "active query error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 1, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 1}))},
				{err: errors.New("active failed")},
			},
			wantErr: "active failed",
		},
		{
			name: "queued row only",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 1, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 1}))},
				{rows: fakeRowsFromValues()},
			},
			wantExec: 2,
		},
		{
			name: "refund error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 1, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 1}))},
				{rows: fakeRowsFromValues([]any{8})},
			},
			execErrs: []error{errors.New("refund failed")},
			wantErr:  "refund failed",
			wantExec: 1,
		},
		{
			name: "remove global error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 1, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 1}))},
				{rows: fakeRowsFromValues([]any{8})},
			},
			execErrs: []error{nil, errors.New("remove global failed")},
			wantErr:  "remove global failed",
			wantExec: 2,
		},
		{
			name: "shift error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 1, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 1}))},
				{rows: fakeRowsFromValues()},
			},
			execErrs: []error{errors.New("shift failed")},
			wantErr:  "shift failed",
			wantExec: 1,
		},
		{
			name: "delete row error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 1, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 1}))},
				{rows: fakeRowsFromValues()},
			},
			execErrs: []error{nil, errors.New("delete row failed")},
			wantErr:  "delete row failed",
			wantExec: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeBuildingsRunner{
				fakeQueryer: fakeQueryer{results: append(shipyardOverviewResults(), tt.results...)},
				results:     append([]sql.Result(nil), tt.sqlResult...),
				execErrs:    append([]error(nil), tt.execErrs...),
			}
			repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_005, 0) })
			_, err := repository.MutateBuildings(context.Background(), appgame.BuildingsMutationQuery{PlayerID: 42, PlanetID: 99, Action: domaingame.BuildingsMutationRemove, ListID: 1})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected %q error, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if len(runner.execs) != tt.wantExec {
				t.Fatalf("expected %d execs, got %+v", tt.wantExec, runner.execs)
			}
		})
	}
}

func TestBuildingsRepositoryStartNextBuildQueueBranches(t *testing.T) {
	tests := []struct {
		name      string
		results   []fakeQueryResult
		sqlResult []sql.Result
		execErrs  []error
		wantErr   string
		wantExec  int
	}{
		{
			name:    "user query",
			results: []fakeQueryResult{{err: errors.New("start user failed")}},
			wantErr: "start user failed",
		},
		{
			name: "config query",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildingMutationUserRow(0, 0, nil))},
				{err: errors.New("start config failed")},
			},
			wantErr: "start config failed",
		},
		{
			name: "vacation",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildingMutationUserRow(1, 0, nil))},
				{rows: fakeRowsFromValues([]any{1.0, 0})},
			},
		},
		{
			name: "frozen",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildingMutationUserRow(0, 0, nil))},
				{rows: fakeRowsFromValues([]any{1.0, 1})},
			},
		},
		{
			name: "queue rows",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildingMutationUserRow(0, 0, nil))},
				{rows: fakeRowsFromValues([]any{1.0, 0})},
				{err: errors.New("start rows failed")},
			},
			wantErr: "start rows failed",
		},
		{
			name: "empty queue",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildingMutationUserRow(0, 0, nil))},
				{rows: fakeRowsFromValues([]any{1.0, 0})},
				{rows: fakeRowsFromValues()},
			},
		},
		{
			name: "planet query",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildingMutationUserRow(0, 0, nil))},
				{rows: fakeRowsFromValues([]any{1.0, 0})},
				{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 2, OwnerID: 42, PlanetID: 99, ListID: 2, TechID: domaingame.BuildingMetalMine, Level: 1}))},
				{err: errors.New("start planet failed")},
			},
			wantErr: "start planet failed",
		},
		{
			name: "invalid queued row removed",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildingMutationUserRow(0, 0, nil))},
				{rows: fakeRowsFromValues([]any{1.0, 0})},
				{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 2, OwnerID: 42, PlanetID: 99, ListID: 2, TechID: 9999, Level: 1}))},
				{rows: fakeRowsFromValues(buildingMutationPlanetRow(map[int]int{}))},
				{rows: fakeRowsFromValues()},
			},
			wantExec: 2,
		},
		{
			name: "spend affected none",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildingMutationUserRow(0, 0, nil))},
				{rows: fakeRowsFromValues([]any{1.0, 0})},
				{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 2, OwnerID: 42, PlanetID: 99, ListID: 2, TechID: domaingame.BuildingMetalMine, Level: 1}))},
				{rows: fakeRowsFromValues(buildingMutationPlanetRow(map[int]int{}))},
			},
			sqlResult: []sql.Result{buildingSQLResult{affected: 0}},
			wantExec:  1,
		},
		{
			name: "timing update error",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(buildingMutationUserRow(0, 0, nil))},
				{rows: fakeRowsFromValues([]any{1.0, 0})},
				{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 2, OwnerID: 42, PlanetID: 99, ListID: 2, TechID: domaingame.BuildingMetalMine, Level: 1}))},
				{rows: fakeRowsFromValues(buildingMutationPlanetRow(map[int]int{}))},
			},
			sqlResult: []sql.Result{buildingSQLResult{affected: 1}, buildingSQLResult{id: 9}},
			execErrs:  []error{nil, nil, errors.New("timing failed")},
			wantErr:   "timing failed",
			wantExec:  3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeBuildingsRunner{
				fakeQueryer: fakeQueryer{results: tt.results},
				results:     append([]sql.Result(nil), tt.sqlResult...),
				execErrs:    append([]error(nil), tt.execErrs...),
			}
			repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_005, 0) })
			err := repository.startNextBuildQueue(context.Background(), "ogame_planets", "ogame_buildqueue", "ogame_queue", 42, 99, 2_005)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected %q error, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if len(runner.execs) != tt.wantExec {
				t.Fatalf("expected %d execs, got %+v", tt.wantExec, runner.execs)
			}
		})
	}
}

func TestBuildingsRepositoryStartNextBuildQueueUpdatesResourcesFirst(t *testing.T) {
	runner := &fakeBuildingsRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues(buildingMutationUserRow(0, 0, nil))},
			{rows: fakeRowsFromValues([]any{1.0, 0})},
			{rows: fakeRowsFromValues(resourceUpdatePlanetRow(resourceUpdatePlanetFixture{
				lastPeek: 1000,
				metal:    1000,
				crystal:  1000,
				deut:     1000,
				levels: map[int]int{
					domaingame.BuildingMetalMine:  1,
					domaingame.BuildingSolarPlant: 1,
				},
			}))},
			{rows: fakeRowsFromValues(resourceUpdateUserRow(42, 0, 0, 0))},
			{rows: fakeRowsFromValues([]any{1.0})},
			{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 2, OwnerID: 42, PlanetID: 99, ListID: 2, TechID: domaingame.BuildingMetalMine, Level: 1}))},
			{rows: fakeRowsFromValues(buildingMutationPlanetRow(map[int]int{}))},
			{rows: fakeRowsFromValues()},
		}},
		results: []sql.Result{
			buildingSQLResult{affected: 1},
			buildingSQLResult{affected: 1},
			buildingSQLResult{id: 9},
			buildingSQLResult{affected: 1},
		},
	}
	repository := NewBuildingsRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_005, 0) })
	repository.updateResources = true

	if err := repository.startNextBuildQueue(context.Background(), "`ogame_planets`", "`ogame_buildqueue`", "`ogame_queue`", 42, 99, 2_005); err != nil {
		t.Fatal(err)
	}

	if len(runner.execs) != 4 {
		t.Fatalf("expected resource, spend, global, and timing writes, got %+v", runner.execs)
	}
	if !strings.Contains(runner.execs[0].sql, "lastpeek = ? WHERE planet_id = ?") || runner.execs[0].args[3] != 2_005 {
		t.Fatalf("expected resource update first, got %+v", runner.execs)
	}
	if !strings.Contains(runner.execs[1].sql, "SET `700` = `700` - ?") {
		t.Fatalf("expected spend after resource update, got %+v", runner.execs)
	}
}

func TestBuildingsRepositoryLoadHelpersHandleFallbacks(t *testing.T) {
	repository := NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues()},
	}}, "ogame_")

	speed, err := repository.loadUniverseSpeed(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if speed != 1 {
		t.Fatalf("expected missing speed row to default to one, got %v", speed)
	}

	repository = NewBuildingsRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{0.0})},
	}}, "ogame_")
	speed, err = repository.loadUniverseSpeed(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if speed != 1 {
		t.Fatalf("expected non-positive speed to default to one, got %v", speed)
	}
}

func TestBuildingsRepositoryReturnsErrors(t *testing.T) {
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
			name:   "building levels",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{err: errors.New("building query failed")},
			}},
			want: "building query failed",
		},
		{
			name:   "missing building levels",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValues()},
			}},
			want: "building levels not found",
		},
		{
			name:   "building rows",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsError(errors.New("building rows failed"))},
			}},
			want: "building rows failed",
		},
		{
			name:   "building scan",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValues([]any{"bad"})},
			}},
			want: "unexpected scan destination count",
		},
		{
			name:   "research query",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValues(buildingLevelRow(nil))},
				{err: errors.New("research query failed")},
			}},
			want: "research query failed",
		},
		{
			name:   "research levels",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValues(buildingLevelRow(nil))},
				{rows: fakeRowsFromValues()},
			}},
			want: "research levels not found",
		},
		{
			name:   "research rows",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValues(buildingLevelRow(nil))},
				{rows: fakeRowsError(errors.New("research rows failed"))},
			}},
			want: "research rows failed",
		},
		{
			name:   "research scan",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValues(buildingLevelRow(nil))},
				{rows: fakeRowsFromValues([]any{"bad"})},
			}},
			want: "unexpected scan destination count",
		},
		{
			name:   "speed",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValues(buildingLevelRow(nil))},
				{rows: fakeRowsFromValues(researchLevelRow(nil))},
				{err: errors.New("speed query failed")},
			}},
			want: "speed query failed",
		},
		{
			name:   "speed rows",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValues(buildingLevelRow(nil))},
				{rows: fakeRowsFromValues(researchLevelRow(nil))},
				{rows: fakeRowsError(errors.New("speed rows failed"))},
			}},
			want: "speed rows failed",
		},
		{
			name:   "speed scan",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValues(buildingLevelRow(nil))},
				{rows: fakeRowsFromValues(researchLevelRow(nil))},
				{rows: fakeRowsFromValues([]any{"bad"})},
			}},
			want: "expected float64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewBuildingsRepositoryWithQueryer(tt.queryer, tt.prefix)
			_, err := repository.GetBuildings(context.Background(), appgame.BuildingsQuery{PlayerID: 42})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func buildingLevelRow(values map[int]int) []any {
	row := make([]any, 0, len(domaingame.BuildingIDs()))
	for _, id := range domaingame.BuildingIDs() {
		row = append(row, values[id])
	}
	return row
}

func researchLevelRow(values map[int]int) []any {
	row := make([]any, 0, len(domaingame.BuildingResearchIDs()))
	for _, id := range domaingame.BuildingResearchIDs() {
		row = append(row, values[id])
	}
	return row
}

func buildingMutationUserRow(vacation int, commanderUntil int64, research map[int]int) []any {
	row := []any{vacation, commanderUntil}
	return append(row, researchLevelRow(research)...)
}

func buildingMutationPlanetRow(values map[int]int) []any {
	row := []any{99, 42, domaingame.PlanetTypePlanet, 0, 163, 10_000.0, 10_000.0, 10_000.0}
	return append(row, buildingLevelRow(values)...)
}

func buildingMutationPlanetRowWithResources(values map[int]int, metal float64, crystal float64, deuterium float64) []any {
	row := []any{99, 42, domaingame.PlanetTypePlanet, 0, 163, metal, crystal, deuterium}
	return append(row, buildingLevelRow(values)...)
}

func buildingMutationPlanetRowWithFields(values map[int]int, planetType int, fields int, maxFields int) []any {
	row := []any{99, 42, planetType, fields, maxFields, 10_000.0, 10_000.0, 10_000.0}
	return append(row, buildingLevelRow(values)...)
}

func buildQueueRowValues(row buildQueueRow) []any {
	return []any{row.ID, row.OwnerID, row.PlanetID, row.ListID, row.TechID, row.Level, row.Destroy, row.Start, row.End}
}

func buildingQueueTaskValues(row buildingQueueTask) []any {
	return []any{row.TaskID, row.OwnerID, row.Type, row.SubID, row.ObjID, row.Level, row.Start, row.End, row.Prio, row.Freeze, row.Frozen}
}

type fakeBuildingsRunner struct {
	fakeQueryer
	execs    []fakeBuddyExec
	results  []sql.Result
	execErr  error
	execErrs []error
}

func (f *fakeBuildingsRunner) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.execs = append(f.execs, fakeBuddyExec{sql: query, args: args})
	if len(f.execErrs) > 0 {
		err := f.execErrs[0]
		f.execErrs = f.execErrs[1:]
		if err != nil {
			return nil, err
		}
	}
	if f.execErr != nil {
		return nil, f.execErr
	}
	if len(f.results) > 0 {
		result := f.results[0]
		f.results = f.results[1:]
		return result, nil
	}
	return buildingSQLResult{affected: 1, id: 1}, nil
}

type buildingSQLResult struct {
	id       int64
	affected int64
	lastErr  error
	rowsErr  error
}

func (r buildingSQLResult) LastInsertId() (int64, error) {
	if r.lastErr != nil {
		return 0, r.lastErr
	}
	return r.id, nil
}

func (r buildingSQLResult) RowsAffected() (int64, error) {
	if r.rowsErr != nil {
		return 0, r.rowsErr
	}
	return r.affected, nil
}

func buildingByID(t *testing.T, buildings domaingame.Buildings, id int) domaingame.BuildingItem {
	t.Helper()
	for _, item := range buildings.Items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("building %d not found in %+v", id, buildings.Items)
	return domaingame.BuildingItem{}
}

func containsBuilding(buildings domaingame.Buildings, id int) bool {
	for _, item := range buildings.Items {
		if item.ID == id {
			return true
		}
	}
	return false
}
