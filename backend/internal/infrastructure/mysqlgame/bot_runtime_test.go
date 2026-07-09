package mysqlgame

import (
	"context"
	"strings"
	"testing"
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
