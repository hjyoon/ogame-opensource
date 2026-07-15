package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestBotRuntimeFinishesStartBlock(t *testing.T) {
	source := `{"nodeDataArray":[{"key":1,"category":"Start","text":"Start"},{"key":2,"category":"End","text":"End"}],"linkDataArray":[{"from":1,"to":2}]}`
	task := buildingQueueTask{TaskID: 9, OwnerID: 42, Type: queueTypeAI, SubID: 7, ObjID: 1, Start: 100, End: 150, Prio: botQueuePriority}
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1.0, 0})},
		{rows: fakeRowsFromValues(buildingQueueTaskValues(task))},
		{rows: fakeRowsFromValues([]any{source})},
	}}}
	repository := BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}

	if err := repository.FinishDueBotQueues(context.Background(), 200); err != nil {
		t.Fatalf("FinishDueBotQueues returned error: %v", err)
	}
	if len(runner.execCalls) != 2 {
		t.Fatalf("expected insert and delete, got %+v", runner.execCalls)
	}
	insert := runner.execCalls[0]
	if !strings.Contains(insert.sql, "INSERT INTO `ogame_queue`") ||
		insert.args[0] != 42 || insert.args[1] != queueTypeAI || insert.args[2] != 7 ||
		insert.args[3] != 2 || insert.args[5] != 150 || insert.args[6] != 150 || insert.args[7] != botQueuePriority {
		t.Fatalf("unexpected bot child insert: %+v", insert)
	}
	if !strings.Contains(runner.execCalls[1].sql, "DELETE FROM `ogame_queue` WHERE task_id = ?") || runner.execCalls[1].args[0] != 9 {
		t.Fatalf("unexpected current task delete: %+v", runner.execCalls[1])
	}
}

func TestBotRuntimeConditionalUsesBotVars(t *testing.T) {
	source := `{"nodeDataArray":[{"key":1,"category":"Cond","text":"BotGetVar(\"state\", \"idle\") == \"ready\""},{"key":2,"category":"End","text":"yes"},{"key":3,"category":"End","text":"no"}],"linkDataArray":[{"from":1,"to":2,"text":"yes"},{"from":1,"to":3,"text":"no"}]}`
	task := buildingQueueTask{TaskID: 10, OwnerID: 42, Type: queueTypeAI, SubID: 7, ObjID: 1, Start: 100, End: 200, Prio: botQueuePriority}
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1.0, 0})},
		{rows: fakeRowsFromValues(buildingQueueTaskValues(task))},
		{rows: fakeRowsFromValues([]any{source})},
		{rows: fakeRowsFromValues([]any{"ready"})},
	}}}
	repository := BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}

	if err := repository.FinishDueBotQueues(context.Background(), 250); err != nil {
		t.Fatalf("FinishDueBotQueues returned error: %v", err)
	}
	if len(runner.execCalls) != 2 || runner.execCalls[0].args[3] != 2 {
		t.Fatalf("expected yes branch to be queued, execs=%+v", runner.execCalls)
	}
}

func TestBotRuntimeRegularBlockSupportsSetVarAndSleep(t *testing.T) {
	source := `{"nodeDataArray":[{"key":1,"category":"","text":"BotSetVar(\"state\", \"ready\"); return 7"},{"key":2,"category":"End","text":"End"}],"linkDataArray":[{"from":1,"to":2}]}`
	task := buildingQueueTask{TaskID: 11, OwnerID: 42, Type: queueTypeAI, SubID: 7, ObjID: 1, Start: 100, End: 300, Prio: botQueuePriority}
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1.0, 0})},
		{rows: fakeRowsFromValues(buildingQueueTaskValues(task))},
		{rows: fakeRowsFromValues([]any{source})},
		{rows: fakeRowsFromValues()},
	}}}
	repository := BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}

	if err := repository.FinishDueBotQueues(context.Background(), 350); err != nil {
		t.Fatalf("FinishDueBotQueues returned error: %v", err)
	}
	if len(runner.execCalls) != 3 {
		t.Fatalf("expected botvar insert, queue insert, and delete, got %+v", runner.execCalls)
	}
	if !strings.Contains(runner.execCalls[0].sql, "INSERT INTO `ogame_botvars`") || runner.execCalls[0].args[0] != 42 || runner.execCalls[0].args[1] != "state" || runner.execCalls[0].args[2] != "ready" {
		t.Fatalf("unexpected bot var insert: %+v", runner.execCalls[0])
	}
	if runner.execCalls[1].args[5] != 300 || runner.execCalls[1].args[6] != 307 {
		t.Fatalf("expected child queue sleep to be applied, got %+v", runner.execCalls[1])
	}
}

func TestBotRuntimeRegularBlockSupportsBotExec(t *testing.T) {
	source := `{"nodeDataArray":[{"key":1,"category":"","text":"BotExec(\"worker\"); return 3"},{"key":2,"category":"End","text":"End"}],"linkDataArray":[{"from":1,"to":2}]}`
	worker := `{"nodeDataArray":[{"key":8,"category":"Start","text":"Start"}],"linkDataArray":[]}`
	task := buildingQueueTask{TaskID: 12, OwnerID: 42, Type: queueTypeAI, SubID: 7, ObjID: 1, Start: 100, End: 400, Prio: botQueuePriority}
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1.0, 0})},
		{rows: fakeRowsFromValues(buildingQueueTaskValues(task))},
		{rows: fakeRowsFromValues([]any{source})},
		{rows: fakeRowsFromValues([]any{9, worker})},
	}}}
	repository := BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}

	if err := repository.FinishDueBotQueues(context.Background(), 450); err != nil {
		t.Fatalf("FinishDueBotQueues returned error: %v", err)
	}
	if len(runner.execCalls) != 3 {
		t.Fatalf("expected worker queue, child queue, and delete, got %+v", runner.execCalls)
	}
	if runner.execCalls[0].args[2] != 9 || runner.execCalls[0].args[3] != 8 || runner.execCalls[0].args[5] != 400 || runner.execCalls[0].args[6] != 400 {
		t.Fatalf("unexpected BotExec queue insert: %+v", runner.execCalls[0])
	}
	if runner.execCalls[1].args[2] != 7 || runner.execCalls[1].args[3] != 2 || runner.execCalls[1].args[6] != 403 {
		t.Fatalf("unexpected current strategy child insert: %+v", runner.execCalls[1])
	}
}

func TestBotRuntimeLabelUsesBottomPort(t *testing.T) {
	source := `{"nodeDataArray":[{"key":1,"category":"Label","text":"Loop"},{"key":2,"category":"End","text":"top"},{"key":3,"category":"End","text":"bottom"}],"linkDataArray":[{"from":1,"to":2,"fromPort":"T"},{"from":1,"to":3,"fromPort":"B"}]}`
	task := buildingQueueTask{TaskID: 13, OwnerID: 42, Type: queueTypeAI, SubID: 7, ObjID: 1, Start: 100, End: 500, Prio: botQueuePriority}
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1.0, 0})},
		{rows: fakeRowsFromValues(buildingQueueTaskValues(task))},
		{rows: fakeRowsFromValues([]any{source})},
	}}}
	repository := BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}

	if err := repository.FinishDueBotQueues(context.Background(), 550); err != nil {
		t.Fatalf("FinishDueBotQueues returned error: %v", err)
	}
	if len(runner.execCalls) != 2 || runner.execCalls[0].args[3] != 3 {
		t.Fatalf("expected label bottom port target to be queued, execs=%+v", runner.execCalls)
	}
}

func TestBotRuntimeBranchJumpsToMatchingLabel(t *testing.T) {
	source := `{"nodeDataArray":[{"key":1,"category":"Branch","text":"Loop"},{"key":2,"category":"Label","text":"Loop"},{"key":3,"category":"Label","text":"Other"}],"linkDataArray":[]}`
	task := buildingQueueTask{TaskID: 14, OwnerID: 42, Type: queueTypeAI, SubID: 7, ObjID: 1, Start: 100, End: 600, Prio: botQueuePriority}
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1.0, 0})},
		{rows: fakeRowsFromValues(buildingQueueTaskValues(task))},
		{rows: fakeRowsFromValues([]any{source})},
	}}}
	repository := BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}

	if err := repository.FinishDueBotQueues(context.Background(), 650); err != nil {
		t.Fatalf("FinishDueBotQueues returned error: %v", err)
	}
	if len(runner.execCalls) != 2 || runner.execCalls[0].args[3] != 2 {
		t.Fatalf("expected matching label target to be queued, execs=%+v", runner.execCalls)
	}
}

func TestBotRuntimeChoosesDeterministicPercent(t *testing.T) {
	repository := BotRuntimeRepository{randInt: func(int) int { return 40 }}
	if got := repository.chooseBotConditionChild(true, []botStrategyLink{
		{To: 2, Text: "50%"},
		{To: 3, Text: "no"},
	}); got != 2 {
		t.Fatalf("expected percent branch, got %d", got)
	}
	repository.randInt = func(int) int { return 80 }
	if got := repository.chooseBotConditionChild(true, []botStrategyLink{
		{To: 2, Text: "50%"},
		{To: 3, Text: "no"},
	}); got != 3 {
		t.Fatalf("expected failed percent branch to fall back to no, got %d", got)
	}
}

func TestBotRuntimeBlockExecutionBranches(t *testing.T) {
	task := buildingQueueTask{TaskID: 91, OwnerID: 42, Type: queueTypeAI, SubID: 7, ObjID: 1, Start: 100, End: 200, Prio: botQueuePriority}
	for _, tt := range []struct {
		name      string
		block     botStrategyNode
		graph     botStrategyGraph
		children  []botStrategyLink
		results   []fakeQueryResult
		wantExecs int
		wantTo    any
	}{
		{
			name:      "end removes current",
			block:     botStrategyNode{Key: 1, Category: "End"},
			wantExecs: 1,
		},
		{
			name:      "start without child only removes current",
			block:     botStrategyNode{Key: 1, Category: "Start"},
			wantExecs: 1,
		},
		{
			name:      "label falls back to first child",
			block:     botStrategyNode{Key: 1, Category: "Label"},
			children:  []botStrategyLink{{From: 1, To: 4, FromPort: "T"}},
			wantExecs: 2,
			wantTo:    4,
		},
		{
			name:      "branch without matching label only removes current",
			block:     botStrategyNode{Key: 1, Category: "Branch", Text: "Missing"},
			graph:     botStrategyGraph{Nodes: []botStrategyNode{{Key: 2, Category: "Label", Text: "Other"}}},
			wantExecs: 1,
		},
		{
			name:      "condition false chooses no branch",
			block:     botStrategyNode{Key: 1, Category: "Cond", Text: "false"},
			children:  []botStrategyLink{{From: 1, To: 2, Text: "yes"}, {From: 1, To: 3, Text: "no"}},
			wantExecs: 2,
			wantTo:    3,
		},
		{
			name:      "default without child only removes current",
			block:     botStrategyNode{Key: 1, Category: "", Text: "BotIdle()"},
			wantExecs: 1,
		},
		{
			name:     "default bad expression still queues child with zero sleep",
			block:    botStrategyNode{Key: 1, Category: "", Text: "BotGetVar(\"missing\")"},
			children: []botStrategyLink{{From: 1, To: 6}},
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues()},
			},
			wantExecs: 3,
			wantTo:    6,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: tt.results}}
			err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).executeBotBlock(
				context.Background(),
				"`ogame_queue`",
				"`ogame_botstrat`",
				"`ogame_botvars`",
				task,
				tt.block,
				tt.graph,
				tt.children,
			)
			if err != nil {
				t.Fatalf("executeBotBlock returned error: %v", err)
			}
			if len(runner.execCalls) != tt.wantExecs {
				t.Fatalf("expected %d execs, got %+v", tt.wantExecs, runner.execCalls)
			}
			if tt.wantTo != nil {
				found := false
				for _, call := range runner.execCalls {
					if strings.Contains(call.sql, "INSERT INTO `ogame_queue`") && len(call.args) > 3 && call.args[3] == tt.wantTo {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected child target %v, got %+v", tt.wantTo, runner.execCalls)
				}
			}
		})
	}
}

func TestBotRuntimeLoadersAndRandomBranches(t *testing.T) {
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{`{bad`})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{8, `{bad`})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues()},
	}}}
	repository := BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}

	if _, found, err := repository.loadBotStrategyByID(context.Background(), "`ogame_botstrat`", 7); err != nil || found {
		t.Fatalf("missing strategy by id should not be found, found=%v err=%v", found, err)
	}
	if graph, found, err := repository.loadBotStrategyByID(context.Background(), "`ogame_botstrat`", 8); err != nil || !found || len(graph.Nodes) != 0 {
		t.Fatalf("invalid strategy by id should be found as empty graph, graph=%+v found=%v err=%v", graph, found, err)
	}
	if _, _, found, err := repository.loadBotStrategyByName(context.Background(), "`ogame_botstrat`", "missing"); err != nil || found {
		t.Fatalf("missing strategy by name should not be found, found=%v err=%v", found, err)
	}
	if _, graph, found, err := repository.loadBotStrategyByName(context.Background(), "`ogame_botstrat`", "bad"); err != nil || !found || len(graph.Nodes) != 0 {
		t.Fatalf("invalid strategy by name should be found as empty graph, graph=%+v found=%v err=%v", graph, found, err)
	}
	if id, err := repository.loadBotActivePlanetID(context.Background(), 42); err != nil || id != 0 {
		t.Fatalf("missing active planet should return zero, id=%d err=%v", id, err)
	}
	if planet, err := repository.loadBotActivePlanet(context.Background(), 42); err != nil || planet.ID != 0 {
		t.Fatalf("missing active planet row should return empty planet, planet=%+v err=%v", planet, err)
	}

	repository.randInt = func(int) int { return -4 }
	if got := repository.rollBotPercent(); got != 1 {
		t.Fatalf("expected low random clamp to 1, got %d", got)
	}
	repository.randInt = func(int) int { return 123 }
	if got := repository.rollBotPercent(); got != 100 {
		t.Fatalf("expected high random clamp to 100, got %d", got)
	}
	if got := (BotRuntimeRepository{}).rollBotPercent(); got < 1 || got > 100 {
		t.Fatalf("default random roll out of range: %d", got)
	}
}

func TestBotRuntimeDatabaseErrorBranches(t *testing.T) {
	ctx := context.Background()
	queryErr := errors.New("query failed")
	execErr := errors.New("exec failed")
	task := buildingQueueTask{TaskID: 1, OwnerID: 42, Type: queueTypeAI, SubID: 7, ObjID: 1, Start: 10, End: 20, Prio: botQueuePriority}
	graph := `{"nodeDataArray":[{"key":1,"category":"Start","text":"Start"},{"key":2,"category":"End","text":"End"}],"linkDataArray":[{"from":1,"to":2}]}`

	t.Run("finish due config query error", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: queryErr}}}}
		err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).FinishDueBotQueues(ctx, 100)
		if err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected config query error, got %v", err)
		}
	})

	t.Run("finish due task query error", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{1.0, 0})},
			{err: queryErr},
		}}}
		err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).FinishDueBotQueues(ctx, 100)
		if err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected task query error, got %v", err)
		}
	})

	t.Run("finish due strategy query error", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{1.0, 0})},
			{rows: fakeRowsFromValues(buildingQueueTaskValues(task))},
			{err: queryErr},
		}}}
		err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).FinishDueBotQueues(ctx, 100)
		if err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected strategy query error, got %v", err)
		}
	})

	t.Run("due task scan and rows errors", func(t *testing.T) {
		repository := BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, prefix: "ogame_"}
		if _, err := repository.loadDueBotQueueTasks(ctx, "`ogame_queue`", 100, 1); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
			t.Fatalf("expected scan error, got %v", err)
		}

		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("task rows failed"), buildingQueueTaskValues(task))}}}, prefix: "ogame_"}
		if _, err := repository.loadDueBotQueueTasks(ctx, "`ogame_queue`", 100, 1); err == nil || !strings.Contains(err.Error(), "task rows failed") {
			t.Fatalf("expected rows error, got %v", err)
		}
	})

	t.Run("strategy loader scan and rows errors", func(t *testing.T) {
		repository := BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"too", "many"})}}}, prefix: "ogame_"}
		if _, _, err := repository.loadBotStrategyByID(ctx, "`ogame_botstrat`", 7); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
			t.Fatalf("expected id scan error, got %v", err)
		}

		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("id rows failed"), []any{graph})}}}, prefix: "ogame_"}
		if _, _, err := repository.loadBotStrategyByID(ctx, "`ogame_botstrat`", 7); err == nil || !strings.Contains(err.Error(), "id rows failed") {
			t.Fatalf("expected id rows error, got %v", err)
		}

		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{7})}}}, prefix: "ogame_"}
		if _, _, _, err := repository.loadBotStrategyByName(ctx, "`ogame_botstrat`", "worker"); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
			t.Fatalf("expected name scan error, got %v", err)
		}

		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("name rows failed"), []any{7, graph})}}}, prefix: "ogame_"}
		if _, _, _, err := repository.loadBotStrategyByName(ctx, "`ogame_botstrat`", "worker"); err == nil || !strings.Contains(err.Error(), "name rows failed") {
			t.Fatalf("expected name rows error, got %v", err)
		}
	})

	t.Run("strategy vars and exec error branches", func(t *testing.T) {
		repository := BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{err: queryErr}}}, prefix: "ogame_"}
		if _, err := repository.botStrategyExists(ctx, "`ogame_botstrat`", "worker"); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected strategy exists error, got %v", err)
		}

		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("exists rows failed"), []any{7})}}}, prefix: "ogame_"}
		if _, err := repository.botStrategyExists(ctx, "`ogame_botstrat`", "worker"); err == nil || !strings.Contains(err.Error(), "exists rows failed") {
			t.Fatalf("expected strategy rows error, got %v", err)
		}

		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{err: queryErr}}}, prefix: "ogame_"}
		if _, err := repository.botGetVar(ctx, "`ogame_botvars`", 42, "state", nil); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected get var query error, got %v", err)
		}

		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{7})}}}, prefix: "ogame_"}
		if _, err := repository.botGetVar(ctx, "`ogame_botvars`", 42, "state", nil); err == nil || !strings.Contains(err.Error(), "expected string") {
			t.Fatalf("expected get var scan error, got %v", err)
		}

		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, execErr: execErr}
		repository = BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}
		def := "idle"
		if _, err := repository.botGetVar(ctx, "`ogame_botvars`", 42, "state", &def); err == nil || !strings.Contains(err.Error(), "exec failed") {
			t.Fatalf("expected get var default insert error, got %v", err)
		}

		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{err: queryErr}}}, prefix: "ogame_"}
		if err := repository.botSetVar(ctx, "`ogame_botvars`", 42, "state", "ready"); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected set var query error, got %v", err)
		}

		runner = &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}, execErr: execErr}
		repository = BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}
		if err := repository.botSetVar(ctx, "`ogame_botvars`", 42, "state", "ready"); err == nil || !strings.Contains(err.Error(), "exec failed") {
			t.Fatalf("expected set var update error, got %v", err)
		}
	})

	t.Run("bot exec non-start and insert errors", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues([]any{7, `{"nodeDataArray":[{"key":3,"category":"End","text":"End"}],"linkDataArray":[]}`})},
			{rows: fakeRowsFromValues([]any{8, graph})},
		}}, execErr: execErr}
		repository := BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}
		if ok, err := repository.botExec(ctx, "`ogame_botstrat`", task, "missing"); err != nil || ok {
			t.Fatalf("missing strategy should be false without error, ok=%v err=%v", ok, err)
		}
		if ok, err := repository.botExec(ctx, "`ogame_botstrat`", task, "nostart"); err != nil || ok {
			t.Fatalf("strategy without Start should be false without error, ok=%v err=%v", ok, err)
		}
		if ok, err := repository.botExec(ctx, "`ogame_botstrat`", task, "worker"); err == nil || !strings.Contains(err.Error(), "exec failed") || !ok {
			t.Fatalf("expected bot exec insert error after Start, ok=%v err=%v", ok, err)
		}
	})
}

func TestBotRuntimeBotBuildQueuesBuilding(t *testing.T) {
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{99, 99})},
		{rows: fakeRowsFromValues([]any{99, 99})},
		{rows: fakeRowsFromValues(botBuildingUserRow(nil))},
		{rows: fakeRowsFromValues([]any{128.0, 0})},
		{rows: fakeRowsFromValues(botBuildingPlanetRow(map[int]int{}, 0, 200))},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues(botBuildingUserRow(nil))},
		{rows: fakeRowsFromValues([]any{128.0, 0})},
		{rows: fakeRowsFromValues(botBuildingPlanetRow(map[int]int{}, 0, 200))},
		{rows: fakeRowsFromValues()},
	}}}
	repository := BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}

	duration, err := repository.botBuild(context.Background(), 42, domaingame.BuildingMetalMine, 2000)
	if err != nil {
		t.Fatalf("botBuild returned error: %v", err)
	}
	if duration <= 0 {
		t.Fatalf("expected positive build duration, got %d", duration)
	}
	if len(runner.execCalls) != 3 {
		t.Fatalf("expected spend, buildqueue insert, and queue insert, got %+v", runner.execCalls)
	}
	if !strings.Contains(runner.execCalls[0].sql, "UPDATE `ogame_planets` SET `700` = `700` - ?") ||
		runner.execCalls[0].args[4] != 99 || runner.execCalls[0].args[5] != 42 {
		t.Fatalf("unexpected build spend exec: %+v", runner.execCalls[0])
	}
	if !strings.Contains(runner.execCalls[1].sql, "INSERT INTO `ogame_buildqueue`") ||
		runner.execCalls[1].args[2] != 1 ||
		runner.execCalls[1].args[3] != domaingame.BuildingMetalMine ||
		runner.execCalls[1].args[4] != 1 {
		t.Fatalf("unexpected buildqueue insert: %+v", runner.execCalls[1])
	}
	if !strings.Contains(runner.execCalls[2].sql, "INSERT INTO `ogame_queue`") ||
		runner.execCalls[2].args[1] != queueTypeBuild ||
		runner.execCalls[2].args[3] != domaingame.BuildingMetalMine {
		t.Fatalf("unexpected global build queue insert: %+v", runner.execCalls[2])
	}
}

func TestBotRuntimeBotResearchQueuesResearch(t *testing.T) {
	buildings := map[int]int{domaingame.BuildingResearchLab: 3}
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{99, 99})},
		{rows: fakeRowsFromValues([]any{99, 99})},
		{rows: fakeRowsFromValues(botResearchUserRow(nil))},
		{rows: fakeRowsFromValues([]any{128.0, 0})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues(botBuildingPlanetRow(buildings, 0, 200))},
		{rows: fakeRowsFromValues([]any{99, 3})},
		{rows: fakeRowsFromValues(botResearchUserRow(nil))},
		{rows: fakeRowsFromValues([]any{128.0, 0})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues(botBuildingPlanetRow(buildings, 0, 200))},
		{rows: fakeRowsFromValues([]any{99, 3})},
	}}}
	repository := BotRuntimeRepository{
		queryer: runner,
		execer:  runner,
		prefix:  "ogame_",
		now:     func() time.Time { return time.Unix(1000, 0) },
	}

	duration, err := repository.botResearch(context.Background(), 42, domaingame.ResearchEnergy, 2000)
	if err != nil {
		t.Fatalf("botResearch returned error: %v", err)
	}
	if duration <= 0 {
		t.Fatalf("expected positive research duration, got %d", duration)
	}
	if len(runner.execCalls) != 2 {
		t.Fatalf("expected resource spend and research queue insert, got %+v", runner.execCalls)
	}
	if !strings.Contains(runner.execCalls[0].sql, "UPDATE `ogame_planets` SET `700` = `700` - ?") ||
		runner.execCalls[0].args[4] != 99 ||
		runner.execCalls[0].args[5] != 42 {
		t.Fatalf("unexpected research spend exec: %+v", runner.execCalls[0])
	}
	if !strings.Contains(runner.execCalls[1].sql, "INSERT INTO `ogame_queue`") ||
		runner.execCalls[1].args[1] != queueTypeResearch ||
		runner.execCalls[1].args[2] != 99 ||
		runner.execCalls[1].args[3] != domaingame.ResearchEnergy ||
		runner.execCalls[1].args[4] != 1 {
		t.Fatalf("unexpected research queue insert: %+v", runner.execCalls[1])
	}
}

func TestBotRuntimeBotBuildFleetQueuesShips(t *testing.T) {
	research := map[int]int{domaingame.ResearchCombustionDrive: 2}
	buildings := map[int]int{domaingame.BuildingShipyard: 2}
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{99, 99})},
		{rows: fakeRowsFromValues(botResearchUserRow(research))},
		{rows: fakeRowsFromValues([]any{1.0, 1000, 0})},
		{rows: fakeRowsFromValues([]any{99, 99})},
		{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 30, 1_000_000.0, 1_000_000.0, 1_000_000.0})},
		{rows: fakeRowsFromValues(botBuildingLevelsRow(buildings))},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues(botDefenseCountsRow(nil))},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues(botFleetCountsRow(nil))},
		{rows: fakeRowsFromValues()},
	}}}
	repository := BotRuntimeRepository{
		queryer: runner,
		execer:  runner,
		prefix:  "ogame_",
		now:     func() time.Time { return time.Unix(1000, 0) },
	}

	duration, err := repository.botBuildFleet(context.Background(), 42, domaingame.FleetSmallCargo, 2, 2000)
	if err != nil {
		t.Fatalf("botBuildFleet returned error: %v", err)
	}
	if duration <= 0 {
		t.Fatalf("expected positive shipyard duration, got %d", duration)
	}
	if len(runner.execCalls) != 2 {
		t.Fatalf("expected resource spend and shipyard queue insert, got %+v", runner.execCalls)
	}
	if !strings.Contains(runner.execCalls[0].sql, "UPDATE `ogame_planets` SET `700` = `700` - ?") ||
		runner.execCalls[0].args[4] != 99 ||
		runner.execCalls[0].args[5] != 42 {
		t.Fatalf("unexpected shipyard spend exec: %+v", runner.execCalls[0])
	}
	if !strings.Contains(runner.execCalls[1].sql, "INSERT INTO `ogame_queue`") ||
		runner.execCalls[1].args[1] != queueTypeShipyard ||
		runner.execCalls[1].args[2] != 99 ||
		runner.execCalls[1].args[3] != domaingame.FleetSmallCargo ||
		runner.execCalls[1].args[4] != 2 {
		t.Fatalf("unexpected shipyard queue insert: %+v", runner.execCalls[1])
	}
}

func TestBotRuntimeResourceSettingsAndStateHelpers(t *testing.T) {
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(botResearchUserRow(map[int]int{domaingame.ResearchEnergy: 4}))},
		{rows: fakeRowsFromValues([]any{0, 99})},
	}}}
	repository := BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}

	level, err := repository.botGetResearch(context.Background(), 42, domaingame.ResearchEnergy)
	if err != nil || level != 4 {
		t.Fatalf("expected research level 4, got level=%d err=%v", level, err)
	}
	if err := repository.botResourceSettings(context.Background(), 42, []int{-10, 66, 104, 100, 100, 83}); err != nil {
		t.Fatalf("botResourceSettings returned error: %v", err)
	}
	if len(runner.execCalls) != 1 {
		t.Fatalf("expected production update, got %+v", runner.execCalls)
	}
	call := runner.execCalls[0]
	if !strings.Contains(call.sql, "UPDATE `ogame_planets` SET prod1 = ?") ||
		call.args[0] != 0.0 ||
		call.args[1] != 0.7 ||
		call.args[2] != 1.0 ||
		call.args[5] != 0.8 ||
		call.args[6] != 99 ||
		call.args[7] != 42 {
		t.Fatalf("unexpected production update: %+v", call)
	}
}

func TestBotRuntimeValidationHelpers(t *testing.T) {
	t.Run("get build", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botBuildingPlanetRow(map[int]int{domaingame.BuildingShipyard: 2}, 0, 200))},
		}}}
		level, err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).botGetBuild(context.Background(), 42, domaingame.BuildingShipyard)
		if err != nil || level != 2 {
			t.Fatalf("expected shipyard level 2, level=%d err=%v", level, err)
		}
	})

	t.Run("can build", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botBuildingUserRow(nil))},
			{rows: fakeRowsFromValues([]any{128.0, 0})},
			{rows: fakeRowsFromValues(botBuildingPlanetRow(map[int]int{}, 0, 200))},
			{rows: fakeRowsFromValues()},
		}}}
		ok, err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).botCanBuild(context.Background(), 42, domaingame.BuildingMetalMine, 2000)
		if err != nil || !ok {
			t.Fatalf("expected metal mine to be buildable, ok=%v err=%v", ok, err)
		}
	})

	t.Run("can research", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botResearchUserRow(nil))},
			{rows: fakeRowsFromValues([]any{128.0, 0})},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues(botBuildingPlanetRow(map[int]int{domaingame.BuildingResearchLab: 3}, 0, 200))},
			{rows: fakeRowsFromValues([]any{99, 3})},
		}}}
		ok, err := (BotRuntimeRepository{
			queryer: runner,
			execer:  runner,
			prefix:  "ogame_",
			now:     func() time.Time { return time.Unix(1000, 0) },
		}).botCanResearch(context.Background(), 42, domaingame.ResearchEnergy, 2000)
		if err != nil || !ok {
			t.Fatalf("expected energy research to be available, ok=%v err=%v", ok, err)
		}
	})
}

func TestBotRuntimeValidationFailureBranches(t *testing.T) {
	t.Run("build invalid active planet", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
		issue, _, duration, err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).botValidateBuild(context.Background(), 42, domaingame.BuildingMetalMine, 2000)
		if err != nil || issue == nil || issue.Code != domaingame.BuildingsIssueInvalid || duration != 0 {
			t.Fatalf("expected invalid build issue, issue=%+v duration=%d err=%v", issue, duration, err)
		}
	})

	t.Run("build vacation", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botBuildingUserStateRow(nil, 1, 0))},
		}}}
		issue, _, _, err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).botValidateBuild(context.Background(), 42, domaingame.BuildingMetalMine, 2000)
		if err != nil || issue == nil || issue.Code != domaingame.BuildingsIssueVacation {
			t.Fatalf("expected vacation build issue, issue=%+v err=%v", issue, err)
		}
	})

	t.Run("build frozen", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botBuildingUserRow(nil))},
			{rows: fakeRowsFromValues([]any{128.0, 1})},
		}}}
		issue, _, _, err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).botValidateBuild(context.Background(), 42, domaingame.BuildingMetalMine, 2000)
		if err != nil || issue == nil || issue.Code != domaingame.BuildingsIssueUniversePause {
			t.Fatalf("expected frozen build issue, issue=%+v err=%v", issue, err)
		}
	})

	t.Run("build busy", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botBuildingUserRow(nil))},
			{rows: fakeRowsFromValues([]any{128.0, 0})},
			{rows: fakeRowsFromValues(botBuildingPlanetRow(map[int]int{}, 0, 200))},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues([]any{123})},
		}}}
		issue, _, _, err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).botValidateBuild(context.Background(), 42, domaingame.BuildingResearchLab, 2000)
		if err != nil || issue == nil || issue.Code != domaingame.BuildingsIssueBusy {
			t.Fatalf("expected busy build issue, issue=%+v err=%v", issue, err)
		}
	})

	t.Run("build no resources", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botBuildingUserRow(nil))},
			{rows: fakeRowsFromValues([]any{128.0, 0})},
			{rows: fakeRowsFromValues(botBuildingPlanetRowWithResources(map[int]int{}, 0, 200, 0, 0, 0))},
			{rows: fakeRowsFromValues()},
		}}}
		issue, _, _, err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).botValidateBuild(context.Background(), 42, domaingame.BuildingMetalMine, 2000)
		if err != nil || issue == nil || issue.Code != domaingame.BuildingsIssueNoResources {
			t.Fatalf("expected no resource build issue, issue=%+v err=%v", issue, err)
		}
	})

	t.Run("research active queue busy", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botResearchUserRow(nil))},
			{rows: fakeRowsFromValues([]any{128.0, 0})},
			{rows: fakeRowsFromValues([]any{7001, 99, domaingame.ResearchEnergy, 1, 1000, 2000, 0, 0})},
		}}}
		issue, _, _, err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_", now: func() time.Time { return time.Unix(1000, 0) }}).botValidateResearch(context.Background(), 42, domaingame.ResearchEnergy, 2000)
		if err != nil || issue == nil || issue.Code != domaingame.BuildingsIssueBusy {
			t.Fatalf("expected active research busy issue, issue=%+v err=%v", issue, err)
		}
	})

	t.Run("research lab busy", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botResearchUserRow(nil))},
			{rows: fakeRowsFromValues([]any{128.0, 0})},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues([]any{123})},
		}}}
		issue, _, _, err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_", now: func() time.Time { return time.Unix(1000, 0) }}).botValidateResearch(context.Background(), 42, domaingame.ResearchEnergy, 2000)
		if err != nil || issue == nil || issue.Code != domaingame.BuildingsIssueBusy {
			t.Fatalf("expected lab busy research issue, issue=%+v err=%v", issue, err)
		}
	})

	t.Run("research no resources", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botResearchUserRow(nil))},
			{rows: fakeRowsFromValues([]any{128.0, 0})},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues(botBuildingPlanetRowWithResources(map[int]int{domaingame.BuildingResearchLab: 3}, 0, 200, 0, 0, 0))},
		}}}
		issue, _, _, err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_", now: func() time.Time { return time.Unix(1000, 0) }}).botValidateResearch(context.Background(), 42, domaingame.ResearchEnergy, 2000)
		if err != nil || issue == nil || issue.Code != domaingame.BuildingsIssueNoResources {
			t.Fatalf("expected no resource research issue, issue=%+v err=%v", issue, err)
		}
	})
}

func TestBotRuntimeShipyardDefenseAndFailureBranches(t *testing.T) {
	repository := BotRuntimeRepository{prefix: "ogame_"}
	if duration, err := repository.botBuildFleet(context.Background(), 42, domaingame.FleetSmallCargo, 0, 2000); err != nil || duration != 0 {
		t.Fatalf("zero ship amount should no-op, duration=%d err=%v", duration, err)
	}

	emptyActive := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
	duration, err := (BotRuntimeRepository{queryer: emptyActive, execer: emptyActive, prefix: "ogame_"}).botBuildFleet(context.Background(), 42, domaingame.FleetSmallCargo, 1, 2000)
	if err != nil || duration != 0 {
		t.Fatalf("missing active planet should no-op shipyard, duration=%d err=%v", duration, err)
	}

	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{99, 99})},
		{rows: fakeRowsFromValues(botResearchUserRow(nil))},
		{rows: fakeRowsFromValues([]any{1.0, 1000, 0})},
		{rows: fakeRowsFromValues([]any{99, 99})},
		{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 30, 1_000_000.0, 1_000_000.0, 1_000_000.0})},
		{rows: fakeRowsFromValues(botBuildingLevelsRow(map[int]int{domaingame.BuildingShipyard: 1}))},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues(botDefenseCountsRow(nil))},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues()},
	}}}
	duration, err = (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_", now: func() time.Time { return time.Unix(1000, 0) }}).botBuildFleet(context.Background(), 42, domaingame.DefenseRocketLauncher, 1, 2000)
	if err != nil || duration <= 0 || len(runner.execCalls) != 2 {
		t.Fatalf("expected defense shipyard queue, duration=%d err=%v execs=%+v", duration, err, runner.execCalls)
	}
	if runner.execCalls[1].args[3] != domaingame.DefenseRocketLauncher {
		t.Fatalf("expected rocket launcher queue, got %+v", runner.execCalls[1])
	}
}

func TestBotRuntimeActionFailureBranches(t *testing.T) {
	t.Run("bot build missing active planet", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
		duration, err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).botBuild(context.Background(), 42, domaingame.BuildingMetalMine, 2000)
		if err != nil || duration != 0 || len(runner.execCalls) != 0 {
			t.Fatalf("expected build no-op, duration=%d err=%v execs=%+v", duration, err, runner.execCalls)
		}
	})

	t.Run("bot build validation issue", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botBuildingUserRow(nil))},
			{rows: fakeRowsFromValues([]any{128.0, 0})},
			{rows: fakeRowsFromValues(botBuildingPlanetRowWithResources(map[int]int{}, 0, 200, 0, 0, 0))},
			{rows: fakeRowsFromValues()},
		}}}
		duration, err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).botBuild(context.Background(), 42, domaingame.BuildingMetalMine, 2000)
		if err != nil || duration != 0 || len(runner.execCalls) != 0 {
			t.Fatalf("expected build validation no-op, duration=%d err=%v execs=%+v", duration, err, runner.execCalls)
		}
	})

	t.Run("bot research missing active planet", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
		duration, err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).botResearch(context.Background(), 42, domaingame.ResearchEnergy, 2000)
		if err != nil || duration != 0 || len(runner.execCalls) != 0 {
			t.Fatalf("expected research no-op, duration=%d err=%v execs=%+v", duration, err, runner.execCalls)
		}
	})

	t.Run("bot research validation issue", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botResearchUserRow(nil))},
			{rows: fakeRowsFromValues([]any{128.0, 0})},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues(botBuildingPlanetRowWithResources(map[int]int{domaingame.BuildingResearchLab: 3}, 0, 200, 0, 0, 0))},
		}}}
		duration, err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_", now: func() time.Time { return time.Unix(1000, 0) }}).botResearch(context.Background(), 42, domaingame.ResearchEnergy, 2000)
		if err != nil || duration != 0 || len(runner.execCalls) != 0 {
			t.Fatalf("expected research validation no-op, duration=%d err=%v execs=%+v", duration, err, runner.execCalls)
		}
	})

	t.Run("bot shipyard requirements issue", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botResearchUserRow(nil))},
			{rows: fakeRowsFromValues([]any{1.0, 1000, 0})},
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 30, 1_000_000.0, 1_000_000.0, 1_000_000.0})},
			{rows: fakeRowsFromValues(botBuildingLevelsRow(map[int]int{domaingame.BuildingShipyard: 2}))},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues(botDefenseCountsRow(nil))},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues(botFleetCountsRow(nil))},
		}}}
		duration, err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_", now: func() time.Time { return time.Unix(1000, 0) }}).botBuildFleet(context.Background(), 42, domaingame.FleetSmallCargo, 1, 2000)
		if err != nil || duration != 0 || len(runner.execCalls) != 0 {
			t.Fatalf("expected shipyard requirements no-op, duration=%d err=%v execs=%+v", duration, err, runner.execCalls)
		}
	})

	t.Run("active planet row missing", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues()},
		}}}
		planet, err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).loadBotActivePlanet(context.Background(), 42)
		if err != nil || planet.ID != 0 {
			t.Fatalf("expected empty active planet, planet=%+v err=%v", planet, err)
		}
	})

	if _, _, _, _, err := (BotRuntimeRepository{prefix: "bad-prefix_"}).buildingTables(); err == nil {
		t.Fatalf("expected bad building table prefix error")
	}
	if _, _, _, err := (BotRuntimeRepository{prefix: "bad-prefix_"}).researchTables(); err == nil {
		t.Fatalf("expected bad research table prefix error")
	}
	if now := (BotRuntimeRepository{}).clock()(); now.IsZero() {
		t.Fatalf("default bot clock should return current time")
	}
	if !compareBotValues(numberBotValue(4), numberBotValue(4), "<=") ||
		!compareBotValues(numberBotValue(4), numberBotValue(4), ">=") ||
		!compareBotValues(numberBotValue(4), numberBotValue(5), "!=") ||
		!compareBotValues(stringBotValue("a"), stringBotValue("b"), "!==") ||
		!compareBotValues(botValue{}, botValue{}, "==") {
		t.Fatalf("additional bot comparisons returned unexpected values")
	}
	if numberBotValue(4).asString() != "4" || boolBotValue(true).asString() != "1" || (botValue{}).asString() != "" {
		t.Fatalf("bot asString helpers returned unexpected values")
	}
	if stringBotValue("bad").asInt() != 0 || boolBotValue(true).asInt() != 1 {
		t.Fatalf("bot asInt helpers returned unexpected values")
	}
	if boolBotValue(false).truthy() {
		t.Fatalf("false bot bool should not be truthy")
	}
}

func TestBotRuntimeStrategyVarAndEvalHelpers(t *testing.T) {
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{7})},
		{rows: fakeRowsFromValues([]any{"ready"})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{1})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{8})},
		{rows: fakeRowsFromValues([]any{"warm"})},
		{rows: fakeRowsFromValues()},
	}}}
	repository := BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}

	exists, err := repository.botStrategyExists(context.Background(), "`ogame_botstrat`", "worker")
	if err != nil || !exists {
		t.Fatalf("expected worker strategy to exist, exists=%v err=%v", exists, err)
	}
	value, err := repository.botGetVar(context.Background(), "`ogame_botvars`", 42, "state", nil)
	if err != nil || value == nil || *value != "ready" {
		t.Fatalf("expected existing bot var, value=%v err=%v", value, err)
	}
	def := "idle"
	value, err = repository.botGetVar(context.Background(), "`ogame_botvars`", 42, "missing", &def)
	if err != nil || value == nil || *value != "idle" {
		t.Fatalf("expected default bot var insert, value=%v err=%v", value, err)
	}
	if err := repository.botSetVar(context.Background(), "`ogame_botvars`", 42, "state", "done"); err != nil {
		t.Fatalf("botSetVar update returned error: %v", err)
	}
	if err := repository.botSetVar(context.Background(), "`ogame_botvars`", 42, "fresh", "new"); err != nil {
		t.Fatalf("botSetVar insert returned error: %v", err)
	}
	if len(runner.execCalls) != 3 {
		t.Fatalf("expected default insert, update, and insert execs, got %+v", runner.execCalls)
	}

	task := buildingQueueTask{OwnerID: 42}
	if value, err := repository.evalBotValue(context.Background(), "`ogame_botstrat`", "`ogame_botvars`", task, `BotStrategyExists("other")`); err != nil || !value.truthy() {
		t.Fatalf("expected BotStrategyExists value to be truthy, value=%+v err=%v", value, err)
	}
	if value, err := repository.evalBotValue(context.Background(), "`ogame_botstrat`", "`ogame_botvars`", task, `BotGetVar("mode", "cold")`); err != nil || value.asString() != "warm" {
		t.Fatalf("expected BotGetVar value, value=%+v err=%v", value, err)
	}
	if value, err := repository.evalBotValue(context.Background(), "`ogame_botstrat`", "`ogame_botvars`", task, `BotSetVar("mode", "hot")`); err != nil || value.truthy() {
		t.Fatalf("expected BotSetVar empty value, value=%+v err=%v", value, err)
	}
	for _, expression := range []string{"BotIdle()", "null", "true", "false", `"text"`, "7.5", "Unsupported(1)"} {
		if _, err := repository.evalBotValue(context.Background(), "`ogame_botstrat`", "`ogame_botvars`", task, expression); err != nil {
			t.Fatalf("evalBotValue(%s) returned error: %v", expression, err)
		}
	}
}

func TestBotRuntimeEvalValueBotAPISwitchCases(t *testing.T) {
	for _, tt := range []struct {
		name       string
		expression string
		results    []fakeQueryResult
		wantTruthy bool
		wantNumber bool
		wantExecs  int
	}{
		{
			name:       "BotGetBuild",
			expression: "BotGetBuild(21)",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{99, 99})},
				{rows: fakeRowsFromValues(botBuildingPlanetRow(map[int]int{domaingame.BuildingShipyard: 2}, 0, 200))},
			},
			wantNumber: true,
		},
		{
			name:       "BotCanBuild",
			expression: "BotCanBuild(1)",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{99, 99})},
				{rows: fakeRowsFromValues(botBuildingUserRow(nil))},
				{rows: fakeRowsFromValues([]any{128.0, 0})},
				{rows: fakeRowsFromValues(botBuildingPlanetRow(map[int]int{}, 0, 200))},
				{rows: fakeRowsFromValues()},
			},
			wantTruthy: true,
		},
		{
			name:       "BotBuild",
			expression: "BotBuild(1)",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{99, 99})},
				{rows: fakeRowsFromValues([]any{99, 99})},
				{rows: fakeRowsFromValues(botBuildingUserRow(nil))},
				{rows: fakeRowsFromValues([]any{128.0, 0})},
				{rows: fakeRowsFromValues(botBuildingPlanetRow(map[int]int{}, 0, 200))},
				{rows: fakeRowsFromValues()},
				{rows: fakeRowsFromValues(botBuildingUserRow(nil))},
				{rows: fakeRowsFromValues([]any{128.0, 0})},
				{rows: fakeRowsFromValues(botBuildingPlanetRow(map[int]int{}, 0, 200))},
				{rows: fakeRowsFromValues()},
			},
			wantNumber: true,
			wantExecs:  3,
		},
		{
			name:       "BotResourceSettings",
			expression: "BotResourceSettings(70, 60, 50, 100, 100, 80)",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{99, 99})},
			},
			wantExecs: 1,
		},
		{
			name:       "BotEnergyAbove",
			expression: "BotEnergyAbove(1)",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{99, 99})},
				{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 30, 1_000_000.0, 1_000_000.0, 1_000_000.0})},
				{rows: fakeRowsFromValues([]any{0, 0, 0, 20, 0, 0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0})},
				{rows: fakeRowsFromValues([]any{0, int64(0), int64(0)})},
				{rows: fakeRowsFromValues([]any{128.0})},
			},
			wantTruthy: true,
		},
		{
			name:       "BotGetResearch",
			expression: "BotGetResearch(113)",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(botResearchUserRow(map[int]int{domaingame.ResearchEnergy: 4}))},
			},
			wantNumber: true,
		},
		{
			name:       "BotCanResearch",
			expression: "BotCanResearch(113)",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{99, 99})},
				{rows: fakeRowsFromValues(botResearchUserRow(nil))},
				{rows: fakeRowsFromValues([]any{128.0, 0})},
				{rows: fakeRowsFromValues()},
				{rows: fakeRowsFromValues()},
				{rows: fakeRowsFromValues(botBuildingPlanetRow(map[int]int{domaingame.BuildingResearchLab: 3}, 0, 200))},
				{rows: fakeRowsFromValues([]any{99, 3})},
			},
			wantTruthy: true,
		},
		{
			name:       "BotResearch",
			expression: "BotResearch(113)",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{99, 99})},
				{rows: fakeRowsFromValues([]any{99, 99})},
				{rows: fakeRowsFromValues(botResearchUserRow(nil))},
				{rows: fakeRowsFromValues([]any{128.0, 0})},
				{rows: fakeRowsFromValues()},
				{rows: fakeRowsFromValues()},
				{rows: fakeRowsFromValues(botBuildingPlanetRow(map[int]int{domaingame.BuildingResearchLab: 3}, 0, 200))},
				{rows: fakeRowsFromValues([]any{99, 3})},
				{rows: fakeRowsFromValues(botResearchUserRow(nil))},
				{rows: fakeRowsFromValues([]any{128.0, 0})},
				{rows: fakeRowsFromValues()},
				{rows: fakeRowsFromValues()},
				{rows: fakeRowsFromValues(botBuildingPlanetRow(map[int]int{domaingame.BuildingResearchLab: 3}, 0, 200))},
				{rows: fakeRowsFromValues([]any{99, 3})},
			},
			wantNumber: true,
			wantExecs:  2,
		},
		{
			name:       "BotBuildFleet",
			expression: "BotBuildFleet(202, 2)",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{99, 99})},
				{rows: fakeRowsFromValues(botResearchUserRow(map[int]int{domaingame.ResearchCombustionDrive: 2}))},
				{rows: fakeRowsFromValues([]any{1.0, 1000, 0})},
				{rows: fakeRowsFromValues([]any{99, 99})},
				{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 30, 1_000_000.0, 1_000_000.0, 1_000_000.0})},
				{rows: fakeRowsFromValues(botBuildingLevelsRow(map[int]int{domaingame.BuildingShipyard: 2}))},
				{rows: fakeRowsFromValues()},
				{rows: fakeRowsFromValues(botDefenseCountsRow(nil))},
				{rows: fakeRowsFromValues()},
				{rows: fakeRowsFromValues(botFleetCountsRow(nil))},
				{rows: fakeRowsFromValues()},
			},
			wantNumber: true,
			wantExecs:  2,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: tt.results}}
			repository := BotRuntimeRepository{
				queryer: runner,
				execer:  runner,
				prefix:  "ogame_",
				now:     func() time.Time { return time.Unix(1000, 0) },
			}
			value, err := repository.evalBotValue(context.Background(), "`ogame_botstrat`", "`ogame_botvars`", buildingQueueTask{OwnerID: 42, End: 2000}, tt.expression)
			if err != nil {
				t.Fatalf("evalBotValue returned error: %v", err)
			}
			if tt.wantTruthy && !value.truthy() {
				t.Fatalf("expected truthy value, got %+v", value)
			}
			if tt.wantNumber && value.asInt() <= 0 {
				t.Fatalf("expected positive number value, got %+v", value)
			}
			if len(runner.execCalls) != tt.wantExecs {
				t.Fatalf("expected %d exec calls, got %+v", tt.wantExecs, runner.execCalls)
			}
		})
	}
}

func TestBotRuntimeFinishDueBranches(t *testing.T) {
	if err := (BotRuntimeRepository{queryer: &fakeQueryer{}, prefix: "ogame_"}).FinishDueBotQueues(context.Background(), 1); err == nil || !strings.Contains(err.Error(), "updater unavailable") {
		t.Fatalf("expected missing updater error, got %v", err)
	}
	if err := (BotRuntimeRepository{queryer: &fakeQueryer{}, execer: &fakeOverviewRunner{}, prefix: "bad-prefix_"}).FinishDueBotQueues(context.Background(), 1); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected unsafe prefix error, got %v", err)
	}
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1.0, 1})},
	}}}
	if err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).FinishDueBotQueues(context.Background(), 2000); err != nil {
		t.Fatalf("frozen universe should skip bot queues, got %v", err)
	}
	if len(runner.calls) != 1 || len(runner.execCalls) != 0 {
		t.Fatalf("expected frozen universe to stop after config, calls=%+v execs=%+v", runner.calls, runner.execCalls)
	}
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(buildingQueueTaskValues(buildingQueueTask{TaskID: 1, OwnerID: 42, Type: queueTypeAI, SubID: 7, ObjID: 1, Start: 10, End: 20, Prio: botQueuePriority}))},
	}}
	tasks, err := (BotRuntimeRepository{queryer: queryer, prefix: "ogame_"}).loadDueBotQueueTasks(context.Background(), "`ogame_queue`", 30, 0)
	if err != nil || len(tasks) != 1 || tasks[0].TaskID != 1 || queryer.calls[0].args[2] != buildQueueBatch {
		t.Fatalf("expected default bot queue batch, tasks=%+v calls=%+v err=%v", tasks, queryer.calls, err)
	}
	if graph, err := parseBotStrategyGraph(""); err != nil || len(graph.Nodes) != 0 {
		t.Fatalf("empty bot strategy should parse to an empty graph, graph=%+v err=%v", graph, err)
	}
	if _, err := parseBotStrategyGraph("{bad"); err == nil {
		t.Fatalf("expected invalid bot strategy JSON to fail")
	}
}

func TestBotRuntimeEnergyAndExpressionHelpers(t *testing.T) {
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{99, 99})},
		{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 30, 1_000_000.0, 1_000_000.0, 1_000_000.0})},
		{rows: fakeRowsFromValues([]any{0, 0, 0, 20, 0, 0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0})},
		{rows: fakeRowsFromValues([]any{0, int64(0), int64(0)})},
		{rows: fakeRowsFromValues([]any{128.0})},
	}}}
	repository := BotRuntimeRepository{
		queryer: runner,
		execer:  runner,
		prefix:  "ogame_",
		now:     func() time.Time { return time.Unix(1000, 0) },
	}

	ok, err := repository.botEnergyAbove(context.Background(), 42, 1)
	if err != nil || !ok {
		t.Fatalf("expected bot energy to be above threshold, ok=%v err=%v", ok, err)
	}

	if got, err := repository.evalBotCondition(context.Background(), "`ogame_botstrat`", "`ogame_botvars`", buildingQueueTask{}, `(("7" === "7") && !(3 < 2)) || false`); err != nil || !got {
		t.Fatalf("expected nested bot condition to be true, got=%v err=%v", got, err)
	}
	if !compareBotValues(numberBotValue(3), numberBotValue(4), "<") ||
		compareBotValues(numberBotValue(3), numberBotValue(4), ">=") ||
		!compareBotValues(stringBotValue("same"), stringBotValue("same"), "===") ||
		compareBotValues(stringBotValue("same"), stringBotValue("other"), "===") {
		t.Fatalf("bot value comparison helpers returned unexpected results")
	}
	if got := botArgInt([]string{`"5"`, "bad"}, 0, 1); got != 5 {
		t.Fatalf("expected quoted integer argument, got %d", got)
	}
	if got := botArgInt([]string{"bad"}, 0, 7); got != 7 {
		t.Fatalf("expected default integer argument, got %d", got)
	}
	if !boolBotValue(true).truthy() || numberBotValue(0).truthy() || stringBotValue("").truthy() {
		t.Fatalf("bot truthiness helpers returned unexpected results")
	}
}

func TestBotRuntimeConditionAndLoaderEdges(t *testing.T) {
	ctx := context.Background()
	task := buildingQueueTask{TaskID: 33, OwnerID: 42, Type: queueTypeAI, SubID: 7, ObjID: 1, Start: 100, End: 200, Prio: botQueuePriority}
	queryErr := errors.New("query failed")

	t.Run("strategy loader query and empty row errors", func(t *testing.T) {
		repository := BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{err: queryErr}}}, prefix: "ogame_"}
		if _, _, _, err := repository.loadBotStrategyByName(ctx, "`ogame_botstrat`", "worker"); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected name query error, got %v", err)
		}

		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("name empty rows failed"))}}}, prefix: "ogame_"}
		if _, _, _, err := repository.loadBotStrategyByName(ctx, "`ogame_botstrat`", "worker"); err == nil || !strings.Contains(err.Error(), "name empty rows failed") {
			t.Fatalf("expected empty rows error, got %v", err)
		}
	})

	t.Run("condition truth table edges", func(t *testing.T) {
		repository := BotRuntimeRepository{}
		for _, tt := range []struct {
			expression string
			want       bool
		}{
			{expression: "", want: false},
			{expression: "false || 0", want: false},
			{expression: "true && false", want: false},
			{expression: "true && 1", want: true},
			{expression: `("a" != "b")`, want: true},
			{expression: `!("")`, want: true},
		} {
			got, err := repository.evalBotCondition(ctx, "`ogame_botstrat`", "`ogame_botvars`", task, tt.expression)
			if err != nil || got != tt.want {
				t.Fatalf("condition %q got=%v err=%v, want %v", tt.expression, got, err, tt.want)
			}
		}
	})

	t.Run("condition value errors bubble from each side", func(t *testing.T) {
		for _, tt := range []struct {
			name       string
			expression string
		}{
			{name: "or part", expression: `BotGetVar("state") || true`},
			{name: "and part", expression: `true && BotGetVar("state")`},
			{name: "comparison left", expression: `BotGetVar("state") == "ready"`},
			{name: "comparison right", expression: `"ready" == BotGetVar("state")`},
			{name: "truthy value", expression: `BotGetVar("state")`},
		} {
			t.Run(tt.name, func(t *testing.T) {
				repository := BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{err: queryErr}}}, prefix: "ogame_"}
				if _, err := repository.evalBotCondition(ctx, "`ogame_botstrat`", "`ogame_botvars`", task, tt.expression); err == nil || !strings.Contains(err.Error(), "query failed") {
					t.Fatalf("expected condition query error for %q, got %v", tt.expression, err)
				}
			})
		}
	})

	t.Run("bot variable rows and insert errors", func(t *testing.T) {
		repository := BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("get empty rows failed"))}}}, prefix: "ogame_"}
		if _, err := repository.botGetVar(ctx, "`ogame_botvars`", 42, "state", nil); err == nil || !strings.Contains(err.Error(), "get empty rows failed") {
			t.Fatalf("expected get var empty rows error, got %v", err)
		}

		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("get post rows failed"), []any{"ready"})}}}, prefix: "ogame_"}
		if _, err := repository.botGetVar(ctx, "`ogame_botvars`", 42, "state", nil); err == nil || !strings.Contains(err.Error(), "get post rows failed") {
			t.Fatalf("expected get var post rows error, got %v", err)
		}

		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("set rows failed"))}}}, prefix: "ogame_"}
		if err := repository.botSetVar(ctx, "`ogame_botvars`", 42, "state", "ready"); err == nil || !strings.Contains(err.Error(), "set rows failed") {
			t.Fatalf("expected set var rows error, got %v", err)
		}

		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, execErr: errors.New("insert var failed")}
		repository = BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}
		if err := repository.botSetVar(ctx, "`ogame_botvars`", 42, "fresh", "ready"); err == nil || !strings.Contains(err.Error(), "insert var failed") {
			t.Fatalf("expected set var insert error, got %v", err)
		}
	})

	t.Run("finish and resource settings no-op edges", func(t *testing.T) {
		if err := (BotRuntimeRepository{prefix: "ogame_"}).FinishDueBotQueues(ctx, 1000); err == nil || !strings.Contains(err.Error(), "bot queue updater unavailable") {
			t.Fatalf("expected missing updater error, got %v", err)
		}

		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1.0, 1})}}}}
		if err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).FinishDueBotQueues(ctx, 1000); err != nil || len(runner.execCalls) != 0 {
			t.Fatalf("frozen universe should skip bot queues, err=%v execs=%+v", err, runner.execCalls)
		}

		if duration, err := (BotRuntimeRepository{}).botBuildFleet(ctx, 42, domaingame.FleetSmallCargo, 0, 1000); err != nil || duration != 0 {
			t.Fatalf("zero fleet amount should no-op, duration=%d err=%v", duration, err)
		}

		repository := BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, prefix: "ogame_"}
		if duration, err := repository.botBuildFleet(ctx, 42, domaingame.FleetSmallCargo, 1, 1000); err != nil || duration != 0 {
			t.Fatalf("missing active planet should no-op for fleet build, duration=%d err=%v", duration, err)
		}

		runner = &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{99, 99})}}}, execErr: errors.New("resource settings failed")}
		repository = BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}
		if err := repository.botResourceSettings(ctx, 42, []int{-5, 15, 55, 105, 100, 0}); err == nil || !strings.Contains(err.Error(), "resource settings failed") {
			t.Fatalf("expected resource settings exec error, got %v", err)
		}
	})
}

func TestBotRuntimeAdditionalBranchEdges(t *testing.T) {
	ctx := context.Background()
	task := buildingQueueTask{TaskID: 22, OwnerID: 42, Type: queueTypeAI, SubID: 7, ObjID: 99, Start: 100, End: 200, Prio: botQueuePriority}
	graph := `{"nodeDataArray":[{"key":1,"category":"Start","text":"Start"}],"linkDataArray":[]}`

	t.Run("finish skips missing block", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{graph})}}}}
		err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).finishBotQueueTask(ctx, "`ogame_queue`", "`ogame_botstrat`", "`ogame_botvars`", task)
		if err != nil || len(runner.execCalls) != 0 {
			t.Fatalf("missing strategy block should no-op, err=%v execs=%+v", err, runner.execCalls)
		}
	})

	t.Run("child insert errors bubble from each block kind", func(t *testing.T) {
		for _, tt := range []struct {
			name     string
			block    botStrategyNode
			children []botStrategyLink
			graph    botStrategyGraph
			results  []fakeQueryResult
		}{
			{name: "start", block: botStrategyNode{Key: 1, Category: "Start"}, children: []botStrategyLink{{From: 1, To: 2}}},
			{name: "label", block: botStrategyNode{Key: 1, Category: "Label"}, children: []botStrategyLink{{From: 1, To: 2}}},
			{name: "branch", block: botStrategyNode{Key: 1, Category: "Branch", Text: "Loop"}, graph: botStrategyGraph{Nodes: []botStrategyNode{{Key: 2, Category: "Label", Text: "Loop"}}}},
			{name: "condition", block: botStrategyNode{Key: 1, Category: "Cond", Text: "true"}, children: []botStrategyLink{{From: 1, To: 2, Text: "yes"}}},
			{name: "default", block: botStrategyNode{Key: 1, Text: "BotIdle()"}, children: []botStrategyLink{{From: 1, To: 2}}},
		} {
			t.Run(tt.name, func(t *testing.T) {
				runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: tt.results}, execErr: errors.New("insert failed"), execErrAt: 1}
				err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).executeBotBlock(
					ctx,
					"`ogame_queue`",
					"`ogame_botstrat`",
					"`ogame_botvars`",
					task,
					tt.block,
					tt.graph,
					tt.children,
				)
				if err == nil || !strings.Contains(err.Error(), "insert failed") {
					t.Fatalf("expected child insert error, got %v", err)
				}
			})
		}
	})

	t.Run("condition and statement eval errors fall back before remove", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("var failed")}}}}
		err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).executeBotBlock(
			ctx,
			"`ogame_queue`",
			"`ogame_botstrat`",
			"`ogame_botvars`",
			task,
			botStrategyNode{Key: 1, Category: "Cond", Text: `BotGetVar("missing")`},
			botStrategyGraph{},
			[]botStrategyLink{{From: 1, To: 2, Text: "no"}},
		)
		if err != nil || len(runner.execCalls) != 2 || runner.execCalls[0].args[3] != 2 {
			t.Fatalf("condition error should choose no branch, err=%v execs=%+v", err, runner.execCalls)
		}

		runner = &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("var failed")}}}}
		err = (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).executeBotBlock(
			ctx,
			"`ogame_queue`",
			"`ogame_botstrat`",
			"`ogame_botvars`",
			task,
			botStrategyNode{Key: 1, Text: `BotGetVar("missing")`},
			botStrategyGraph{},
			[]botStrategyLink{{From: 1, To: 3}},
		)
		if err != nil || len(runner.execCalls) != 2 || runner.execCalls[0].args[3] != 3 || runner.execCalls[0].args[6] != task.End {
			t.Fatalf("statement error should queue child without sleep, err=%v execs=%+v", err, runner.execCalls)
		}
	})

	t.Run("condition child selection edges", func(t *testing.T) {
		repository := BotRuntimeRepository{randInt: func(int) int { return 90 }}
		if got := repository.chooseBotConditionChild(true, []botStrategyLink{{To: 2, Text: "no"}, {To: 3, Text: "50%"}}); got != 2 {
			t.Fatalf("expected failed percent to fall back to no branch, got %d", got)
		}
		if got := repository.chooseBotConditionChild(false, []botStrategyLink{{To: 4, Text: "maybe"}}); got != 0 {
			t.Fatalf("false condition should skip default branch, got %d", got)
		}
		if got := repository.chooseBotConditionChild(true, []botStrategyLink{{To: 5, Text: "maybe"}}); got != 0 {
			t.Fatalf("unmatched true branch should fall through, got %d", got)
		}
		if got := repository.chooseBotConditionChild(true, nil); got != 0 {
			t.Fatalf("empty children should return zero, got %d", got)
		}
	})

	t.Run("parser and statement edges", func(t *testing.T) {
		runner := &fakeOverviewRunner{}
		repository := BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}
		if err := repository.insertBotQueue(ctx, "`ogame_queue`", 42, 7, 2, 100, -9); err != nil {
			t.Fatalf("negative sleep insert returned error: %v", err)
		}
		if runner.execCalls[0].args[6] != 91 {
			t.Fatalf("negative sleep should be relative to the block time, got %+v", runner.execCalls[0])
		}
		if sleep, err := repository.executeBotStatements(ctx, "`ogame_botstrat`", "`ogame_botvars`", task, `; 5`); err != nil || sleep != 5 {
			t.Fatalf("expected numeric statement sleep, sleep=%d err=%v", sleep, err)
		}
		errRunner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("value failed")}}}}
		if _, err := (BotRuntimeRepository{queryer: errRunner, execer: errRunner, prefix: "ogame_"}).executeBotStatements(ctx, "`ogame_botstrat`", "`ogame_botvars`", task, `BotGetVar("state")`); err == nil || !strings.Contains(err.Error(), "value failed") {
			t.Fatalf("expected statement value error, got %v", err)
		}
		if parts := splitBotStatements(`BotSetVar("a\";b", "c");BotIdle()`, ";"); len(parts) != 2 {
			t.Fatalf("quoted semicolon should not split, got %+v", parts)
		}
		if got := splitBotStatements("raw", ""); len(got) != 1 || got[0] != "raw" {
			t.Fatalf("empty separator should return source, got %+v", got)
		}
		if left, right, ok := splitTopLevelComparison(`"a\" > b" > 1`, ">"); !ok || left != `"a\" > b"` || right != "1" {
			t.Fatalf("comparison should ignore escaped operator in string, left=%q right=%q ok=%v", left, right, ok)
		}
		if got := botArgString(nil, 0, "fallback"); got != "fallback" {
			t.Fatalf("missing string arg should use default, got %q", got)
		}
		if got := botArgNullableString([]string{"null"}, 0); got != nil {
			t.Fatalf("null arg should return nil, got %v", *got)
		}
		if got := botArgInt(nil, 2, 17); got != 17 {
			t.Fatalf("missing int arg should use default, got %d", got)
		}
		if got := trimBotExpression(" return 4; "); got != "4" {
			t.Fatalf("return expression should trim to 4, got %q", got)
		}
		if !botOuterParensBalanced(`("a\"b")`) {
			t.Fatalf("escaped quote expression should remain balanced")
		}
		if value, ok := unquoteBotString(`'\x'`); !ok || value != `\x` {
			t.Fatalf("invalid escape should use raw fallback, value=%q ok=%v", value, ok)
		}
		if !compareBotValues(numberBotValue(5), numberBotValue(4), ">") || compareBotValues(numberBotValue(1), numberBotValue(1), "??") {
			t.Fatalf("comparison edge cases returned unexpected values")
		}
	})
}

func TestBotRuntimeLoaderAndValidationErrorEdges(t *testing.T) {
	ctx := context.Background()
	queryErr := errors.New("query failed")

	t.Run("active planet id errors", func(t *testing.T) {
		if _, err := (BotRuntimeRepository{prefix: "bad-prefix_"}).loadBotActivePlanetID(ctx, 42); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
			t.Fatalf("expected bad prefix error, got %v", err)
		}
		if _, err := (BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{err: queryErr}}}, prefix: "ogame_"}).loadBotActivePlanetID(ctx, 42); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected active id query error, got %v", err)
		}
		if _, err := (BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("active rows failed"))}}}, prefix: "ogame_"}).loadBotActivePlanetID(ctx, 42); err == nil || !strings.Contains(err.Error(), "active rows failed") {
			t.Fatalf("expected active id rows error, got %v", err)
		}
		if _, err := (BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 99})}}}, prefix: "ogame_"}).loadBotActivePlanetID(ctx, 42); err == nil || !strings.Contains(err.Error(), "expected int") {
			t.Fatalf("expected active id scan error, got %v", err)
		}
		if _, err := (BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("active post rows failed"), []any{99, 99})}}}, prefix: "ogame_"}).loadBotActivePlanetID(ctx, 42); err == nil || !strings.Contains(err.Error(), "active post rows failed") {
			t.Fatalf("expected active id post rows error, got %v", err)
		}
	})

	t.Run("active planet row errors", func(t *testing.T) {
		repository := BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{err: queryErr},
		}}, prefix: "ogame_"}
		if _, err := repository.loadBotActivePlanet(ctx, 42); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected active planet query error, got %v", err)
		}
		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsError(errors.New("planet rows failed"))},
		}}, prefix: "ogame_"}
		if _, err := repository.loadBotActivePlanet(ctx, 42); err == nil || !strings.Contains(err.Error(), "planet rows failed") {
			t.Fatalf("expected active planet rows error, got %v", err)
		}
		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues([]any{"bad"})},
		}}, prefix: "ogame_"}
		if _, err := repository.loadBotActivePlanet(ctx, 42); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
			t.Fatalf("expected active planet scan error, got %v", err)
		}
	})

	t.Run("build validation errors and queued levels", func(t *testing.T) {
		repository := BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{err: queryErr},
		}}, prefix: "ogame_"}
		if _, _, _, err := repository.botValidateBuild(ctx, 42, domaingame.BuildingMetalMine, 1000); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected build user query error, got %v", err)
		}

		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botBuildingUserRow(nil))},
			{err: queryErr},
		}}, prefix: "ogame_"}
		if _, _, _, err := repository.botValidateBuild(ctx, 42, domaingame.BuildingMetalMine, 1000); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected build config query error, got %v", err)
		}

		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botBuildingUserRow(nil))},
			{rows: fakeRowsFromValues([]any{128.0, 0})},
			{err: queryErr},
		}}, prefix: "ogame_"}
		if _, _, _, err := repository.botValidateBuild(ctx, 42, domaingame.BuildingMetalMine, 1000); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected build planet query error, got %v", err)
		}

		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botBuildingUserRow(nil))},
			{rows: fakeRowsFromValues([]any{128.0, 0})},
			{rows: fakeRowsFromValues(botBuildingPlanetRow(map[int]int{}, 0, 200))},
			{err: queryErr},
		}}, prefix: "ogame_"}
		if _, _, _, err := repository.botValidateBuild(ctx, 42, domaingame.BuildingMetalMine, 1000); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected build queue query error, got %v", err)
		}

		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botBuildingUserRow(nil))},
			{rows: fakeRowsFromValues([]any{128.0, 0})},
			{rows: fakeRowsFromValues(botBuildingPlanetRow(map[int]int{}, 0, 200))},
			{rows: fakeRowsFromValues(buildQueueRowValues(buildQueueRow{ID: 1, OwnerID: 42, PlanetID: 99, ListID: 2, TechID: domaingame.BuildingMetalMine, Level: 2}))},
		}}, prefix: "ogame_"}
		issue, _, duration, err := repository.botValidateBuild(ctx, 42, domaingame.BuildingMetalMine, 1000)
		if err != nil || issue != nil || duration <= 0 {
			t.Fatalf("expected queued build level to validate, issue=%+v duration=%d err=%v", issue, duration, err)
		}

		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, prefix: "ogame_"}
		if ok, err := repository.botCanBuild(ctx, 42, domaingame.BuildingMetalMine, 1000); err != nil || ok {
			t.Fatalf("invalid active planet should not be buildable, ok=%v err=%v", ok, err)
		}
	})

	t.Run("research validation errors", func(t *testing.T) {
		vacationUser := botResearchUserRow(nil)
		vacationUser[0] = 1
		for _, tt := range []struct {
			name    string
			results []fakeQueryResult
			wantErr string
			want    string
		}{
			{name: "missing active", results: []fakeQueryResult{{rows: fakeRowsFromValues()}}, want: domaingame.BuildingsIssueInvalid},
			{name: "user query", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{99, 99})}, {err: queryErr}}, wantErr: "query failed"},
			{name: "vacation", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{99, 99})}, {rows: fakeRowsFromValues(vacationUser)}}, want: domaingame.BuildingsIssueVacation},
			{name: "config query", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{99, 99})}, {rows: fakeRowsFromValues(botResearchUserRow(nil))}, {err: queryErr}}, wantErr: "query failed"},
			{name: "frozen", results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{99, 99})}, {rows: fakeRowsFromValues(botResearchUserRow(nil))}, {rows: fakeRowsFromValues([]any{128.0, 1})}}, want: domaingame.BuildingsIssueUniversePause},
			{name: "planet query", results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{99, 99})},
				{rows: fakeRowsFromValues(botResearchUserRow(nil))},
				{rows: fakeRowsFromValues([]any{128.0, 0})},
				{rows: fakeRowsFromValues()},
				{rows: fakeRowsFromValues()},
				{err: queryErr},
			}, wantErr: "query failed"},
		} {
			t.Run(tt.name, func(t *testing.T) {
				repository := BotRuntimeRepository{queryer: &fakeQueryer{results: tt.results}, prefix: "ogame_", now: func() time.Time { return time.Unix(1000, 0) }}
				issue, _, _, err := repository.botValidateResearch(ctx, 42, domaingame.ResearchEnergy, 1000)
				if tt.wantErr != "" {
					if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
						t.Fatalf("expected error %q, got %v", tt.wantErr, err)
					}
					return
				}
				if err != nil || issue == nil || issue.Code != tt.want {
					t.Fatalf("expected issue %q, issue=%+v err=%v", tt.want, issue, err)
				}
			})
		}
		repository := BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, prefix: "ogame_"}
		if ok, err := repository.botCanResearch(ctx, 42, domaingame.ResearchEnergy, 1000); err != nil || ok {
			t.Fatalf("invalid active planet should not be researchable, ok=%v err=%v", ok, err)
		}
	})

	t.Run("energy and research loader errors", func(t *testing.T) {
		repository := BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, prefix: "ogame_"}
		if ok, err := repository.botEnergyAbove(ctx, 42, 1); err != nil || ok {
			t.Fatalf("missing active planet should not pass energy check, ok=%v err=%v", ok, err)
		}
		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 30, 1_000_000.0, 1_000_000.0, 1_000_000.0})},
			{err: queryErr},
		}}, prefix: "ogame_"}
		if _, err := repository.botEnergyAbove(ctx, 42, 1); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected production settings error, got %v", err)
		}
		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 30, 1_000_000.0, 1_000_000.0, 1_000_000.0})},
			{rows: fakeRowsFromValues([]any{0, 0, 0, 20, 0, 0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0})},
			{err: queryErr},
		}}, prefix: "ogame_"}
		if _, err := repository.botEnergyAbove(ctx, 42, 1); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected resource user error, got %v", err)
		}
		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 30, 1_000_000.0, 1_000_000.0, 1_000_000.0})},
			{rows: fakeRowsFromValues([]any{0, 0, 0, 20, 0, 0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0})},
			{rows: fakeRowsFromValues([]any{0, int64(0), int64(0)})},
			{err: queryErr},
		}}, prefix: "ogame_"}
		if _, err := repository.botEnergyAbove(ctx, 42, 1); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected universe speed error, got %v", err)
		}
		if _, err := (BotRuntimeRepository{prefix: "bad-prefix_"}).botGetResearch(ctx, 42, domaingame.ResearchEnergy); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
			t.Fatalf("expected get research prefix error, got %v", err)
		}
		if _, err := (BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{err: queryErr}}}, prefix: "ogame_"}).botGetResearch(ctx, 42, domaingame.ResearchEnergy); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected get research query error, got %v", err)
		}
	})

	t.Run("shipyard state errors", func(t *testing.T) {
		if _, err := (BotRuntimeRepository{prefix: "bad-prefix_"}).loadBotShipyardState(ctx, 42, 99, domaingame.FleetSmallCargo); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
			t.Fatalf("expected shipyard prefix error, got %v", err)
		}
		repository := BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{err: queryErr}}}, prefix: "ogame_"}
		if _, err := repository.loadBotShipyardState(ctx, 42, 99, domaingame.FleetSmallCargo); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected shipyard user error, got %v", err)
		}
		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues(botResearchUserRow(nil))},
			{err: queryErr},
		}}, prefix: "ogame_"}
		if _, err := repository.loadBotShipyardState(ctx, 42, 99, domaingame.FleetSmallCargo); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected shipyard config error, got %v", err)
		}
		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues(botResearchUserRow(nil))},
			{rows: fakeRowsFromValues([]any{128.0, 1000, 0})},
			{err: queryErr},
		}}, prefix: "ogame_"}
		if _, err := repository.loadBotShipyardState(ctx, 42, 99, domaingame.FleetSmallCargo); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected shipyard active planet error, got %v", err)
		}
		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{err: queryErr},
		}}, prefix: "ogame_"}
		if _, err := repository.botBuildFleet(ctx, 42, domaingame.FleetSmallCargo, 1, 1000); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected botBuildFleet state error, got %v", err)
		}

		for _, tt := range []struct {
			name    string
			results []fakeQueryResult
		}{
			{
				name: "building levels",
				results: []fakeQueryResult{
					{rows: fakeRowsFromValues(botResearchUserRow(nil))},
					{rows: fakeRowsFromValues([]any{128.0, 1000, 0})},
					{rows: fakeRowsFromValues([]any{99, 99})},
					{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 30, 1_000_000.0, 1_000_000.0, 1_000_000.0})},
					{err: queryErr},
				},
			},
			{
				name: "busy",
				results: []fakeQueryResult{
					{rows: fakeRowsFromValues(botResearchUserRow(nil))},
					{rows: fakeRowsFromValues([]any{128.0, 1000, 0})},
					{rows: fakeRowsFromValues([]any{99, 99})},
					{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 30, 1_000_000.0, 1_000_000.0, 1_000_000.0})},
					{rows: fakeRowsFromValues(botBuildingLevelsRow(map[int]int{domaingame.BuildingShipyard: 2}))},
					{err: queryErr},
				},
			},
			{
				name: "defense",
				results: []fakeQueryResult{
					{rows: fakeRowsFromValues(botResearchUserRow(nil))},
					{rows: fakeRowsFromValues([]any{128.0, 1000, 0})},
					{rows: fakeRowsFromValues([]any{99, 99})},
					{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 30, 1_000_000.0, 1_000_000.0, 1_000_000.0})},
					{rows: fakeRowsFromValues(botBuildingLevelsRow(map[int]int{domaingame.BuildingShipyard: 2}))},
					{rows: fakeRowsFromValues()},
					{err: queryErr},
				},
			},
			{
				name: "queue",
				results: []fakeQueryResult{
					{rows: fakeRowsFromValues(botResearchUserRow(nil))},
					{rows: fakeRowsFromValues([]any{128.0, 1000, 0})},
					{rows: fakeRowsFromValues([]any{99, 99})},
					{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 30, 1_000_000.0, 1_000_000.0, 1_000_000.0})},
					{rows: fakeRowsFromValues(botBuildingLevelsRow(map[int]int{domaingame.BuildingShipyard: 2}))},
					{rows: fakeRowsFromValues()},
					{rows: fakeRowsFromValues(botDefenseCountsRow(nil))},
					{err: queryErr},
				},
			},
			{
				name: "fleet counts",
				results: []fakeQueryResult{
					{rows: fakeRowsFromValues(botResearchUserRow(nil))},
					{rows: fakeRowsFromValues([]any{128.0, 1000, 0})},
					{rows: fakeRowsFromValues([]any{99, 99})},
					{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 30, 1_000_000.0, 1_000_000.0, 1_000_000.0})},
					{rows: fakeRowsFromValues(botBuildingLevelsRow(map[int]int{domaingame.BuildingShipyard: 2}))},
					{rows: fakeRowsFromValues()},
					{rows: fakeRowsFromValues(botDefenseCountsRow(nil))},
					{rows: fakeRowsFromValues()},
					{err: queryErr},
				},
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				repository := BotRuntimeRepository{queryer: &fakeQueryer{results: tt.results}, prefix: "ogame_"}
				if _, err := repository.loadBotShipyardState(ctx, 42, 99, domaingame.FleetSmallCargo); err == nil || !strings.Contains(err.Error(), "query failed") {
					t.Fatalf("expected shipyard %s error, got %v", tt.name, err)
				}
			})
		}

		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botResearchUserRow(nil))},
			{rows: fakeRowsFromValues([]any{128.0, 1000, 0})},
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 30, 1_000_000.0, 1_000_000.0, 1_000_000.0})},
			{rows: fakeRowsFromValues(botBuildingLevelsRow(map[int]int{domaingame.BuildingShipyard: 2}))},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues(botDefenseCountsRow(nil))},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues(botFleetCountsRow(nil))},
		}}}
		duration, err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).botBuildFleet(ctx, 42, domaingame.FleetSmallCargo, 1, 1000)
		if err != nil || duration != 0 || len(runner.execCalls) != 0 {
			t.Fatalf("unmet ship requirements should no-op, duration=%d err=%v execs=%+v", duration, err, runner.execCalls)
		}
	})

	t.Run("bot action error propagation", func(t *testing.T) {
		if _, err := (BotRuntimeRepository{prefix: "bad-prefix_"}).botBuild(ctx, 42, domaingame.BuildingMetalMine, 1000); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
			t.Fatalf("expected botBuild prefix error, got %v", err)
		}
		if _, err := (BotRuntimeRepository{prefix: "bad-prefix_"}).botResearch(ctx, 42, domaingame.ResearchEnergy, 1000); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
			t.Fatalf("expected botResearch prefix error, got %v", err)
		}
		repository := BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{err: queryErr}}}, prefix: "ogame_"}
		if _, err := repository.botGetBuild(ctx, 42, domaingame.BuildingMetalMine); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected botGetBuild active id error, got %v", err)
		}
		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{err: queryErr},
		}}, prefix: "ogame_"}
		if _, err := repository.botGetBuild(ctx, 42, domaingame.BuildingMetalMine); err == nil || !strings.Contains(err.Error(), "query failed") {
			t.Fatalf("expected botGetBuild planet error, got %v", err)
		}
		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{err: queryErr}}}, prefix: "ogame_"}
		if ok, err := repository.botCanBuild(ctx, 42, domaingame.BuildingMetalMine, 1000); err == nil || !strings.Contains(err.Error(), "query failed") || ok {
			t.Fatalf("expected botCanBuild error, ok=%v err=%v", ok, err)
		}
		repository = BotRuntimeRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, prefix: "ogame_"}
		if err := repository.botResourceSettings(ctx, 42, []int{100, 100, 100, 100, 100, 100}); err != nil {
			t.Fatalf("missing active planet resource settings should no-op, got %v", err)
		}
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botBuildingUserRow(nil))},
			{rows: fakeRowsFromValues([]any{128.0, 0})},
			{rows: fakeRowsFromValues(botBuildingPlanetRow(map[int]int{}, 0, 200))},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues(botBuildingUserRow(nil))},
			{rows: fakeRowsFromValues([]any{128.0, 0})},
			{rows: fakeRowsFromValues(botBuildingPlanetRow(map[int]int{}, 0, 200))},
			{rows: fakeRowsFromValues()},
		}}, execErr: errors.New("spend failed"), execErrAt: 1}
		if _, err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_"}).botBuild(ctx, 42, domaingame.BuildingMetalMine, 1000); err == nil || !strings.Contains(err.Error(), "spend failed") {
			t.Fatalf("expected botBuild enqueue error, got %v", err)
		}

		buildings := map[int]int{domaingame.BuildingResearchLab: 3}
		runner = &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues([]any{99, 99})},
			{rows: fakeRowsFromValues(botResearchUserRow(nil))},
			{rows: fakeRowsFromValues([]any{128.0, 0})},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues(botBuildingPlanetRow(buildings, 0, 200))},
			{rows: fakeRowsFromValues([]any{99, 3})},
			{rows: fakeRowsFromValues(botResearchUserRow(nil))},
			{rows: fakeRowsFromValues([]any{128.0, 0})},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues(botBuildingPlanetRow(buildings, 0, 200))},
			{rows: fakeRowsFromValues([]any{99, 3})},
		}}, execErr: errors.New("research spend failed"), execErrAt: 1}
		if _, err := (BotRuntimeRepository{queryer: runner, execer: runner, prefix: "ogame_", now: func() time.Time { return time.Unix(1000, 0) }}).botResearch(ctx, 42, domaingame.ResearchEnergy, 1000); err == nil || !strings.Contains(err.Error(), "research spend failed") {
			t.Fatalf("expected botResearch start error, got %v", err)
		}
	})
}

func botBuildingUserRow(research map[int]int) []any {
	return botBuildingUserStateRow(research, 0, 0)
}

func botBuildingUserStateRow(research map[int]int, vacation int, commanderUntil int64) []any {
	row := []any{vacation, commanderUntil}
	for _, id := range domaingame.BuildingResearchIDs() {
		row = append(row, research[id])
	}
	return row
}

func botResearchUserRow(research map[int]int) []any {
	row := []any{0, int64(0)}
	for _, id := range domaingame.ResearchIDs() {
		row = append(row, research[id])
	}
	return row
}

func botBuildingPlanetRow(levels map[int]int, fields int, maxFields int) []any {
	return botBuildingPlanetRowWithResources(levels, fields, maxFields, 1_000_000, 1_000_000, 1_000_000)
}

func botBuildingPlanetRowWithResources(levels map[int]int, fields int, maxFields int, metal float64, crystal float64, deuterium float64) []any {
	row := []any{99, 42, domaingame.PlanetTypePlanet, fields, maxFields, metal, crystal, deuterium}
	for _, id := range domaingame.BuildingIDs() {
		row = append(row, levels[id])
	}
	return row
}

func botBuildingLevelsRow(levels map[int]int) []any {
	row := []any{}
	for _, id := range domaingame.BuildingIDs() {
		row = append(row, levels[id])
	}
	return row
}

func botFleetCountsRow(fleet map[int]int) []any {
	row := []any{}
	for _, id := range domaingame.FleetIDs() {
		row = append(row, fleet[id])
	}
	return row
}

func botDefenseCountsRow(defense map[int]int) []any {
	row := []any{}
	for _, id := range domaingame.DefenseIDs() {
		row = append(row, defense[id])
	}
	return row
}
