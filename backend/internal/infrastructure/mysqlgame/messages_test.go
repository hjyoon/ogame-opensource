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

func TestMessagesRepositoryReadsLegacyInbox(t *testing.T) {
	now := time.Unix(1700000000, 0)
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix()})},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{11, domaingame.MessageTypePM, `Sender\\Name`, `Subject\"Line`, `Player Gophalaxtarget\'s fleet`, 0, int64(1700000000)},
			[]any{10, domaingame.MessageTypeSpyReport, "Spy", "<a>Report</a>", "<table></table>", 1, int64(1699999900)},
		)},
	)}
	repository := NewMessagesRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })

	messages, err := repository.GetMessages(context.Background(), appgame.MessagesQuery{PlayerID: 42, PlanetID: 99})
	if err != nil {
		t.Fatal(err)
	}
	if messages.Commander != "legor" || messages.Action != domaingame.MessagesActionInbox || len(messages.Rows) != 2 {
		t.Fatalf("unexpected messages summary: %+v", messages)
	}
	if !messages.Rows[0].Unread || !messages.Rows[0].Reportable || messages.Rows[1].Reportable {
		t.Fatalf("unexpected message flags: %+v", messages.Rows)
	}
	if messages.Rows[0].From != `Sender\Name` || messages.Rows[0].Subject != `Subject"Line` || messages.Rows[0].Text != "Player Gophalaxtarget's fleet" {
		t.Fatalf("expected legacy escaped message fields to be unescaped, got %+v", messages.Rows[0])
	}
	if !strings.Contains(queryer.calls[5].sql, "pm <> ? ORDER BY date DESC, msg_id DESC LIMIT ?") ||
		queryer.calls[5].args[1] != domaingame.MessageTypeBattleReportText ||
		queryer.calls[5].args[2] != domaingame.MessagesLimitCommander {
		t.Fatalf("expected legacy messages query, got %+v", queryer.calls[5])
	}
}

func TestMessagesRepositoryReadsComposeTarget(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{77, "Target", 2, 3, 4})},
	)}
	repository := NewMessagesRepositoryWithQueryer(queryer, "ogame_", time.Now)

	messages, err := repository.GetMessages(context.Background(), appgame.MessagesQuery{PlayerID: 42, TargetPlayerID: 77})
	if err != nil {
		t.Fatal(err)
	}
	if messages.Action != domaingame.MessagesActionCompose || messages.Compose == nil || messages.Compose.Target.Name != "Target" {
		t.Fatalf("unexpected compose messages: %+v", messages)
	}
	if !strings.Contains(queryer.calls[4].sql, "LEFT JOIN") || queryer.calls[4].args[0] != 77 {
		t.Fatalf("expected compose target query, got %+v", queryer.calls[4])
	}
}

func TestNewMessagesRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewMessagesRepository(nil, "ogame_")
	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
	if _, ok := repository.execer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL execer, got %T", repository.execer)
	}
	repository = NewMessagesRepositoryWithQueryer(&fakeQueryer{}, "ogame_", nil)
	if repository.now == nil {
		t.Fatal("nil clock should default to time.Now")
	}
	repository = NewMessagesRepositoryWithQueryer(&fakeMessagesRunner{}, "ogame_", nil)
	if repository.execer == nil {
		t.Fatal("queryer that implements Execer should be reused for writes")
	}
}

func TestMessagesRepositorySendsPrivateMessage(t *testing.T) {
	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{42, "Sender", 1, 0, "", 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{77, "Recipient", 1, 0, "", 2, 3, 4})},
		{rows: fakeRowsFromValues([]any{127})},
	}}}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })

	outcome, err := repository.MutateMessages(context.Background(), appgame.MessagesMutationQuery{
		PlayerID:       42,
		Action:         domaingame.MessagesMutationActionSend,
		TargetPlayerID: 77,
		Subject:        "Hello",
		Text:           "Line 1\nLine 2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue == nil || outcome.ActionIssue.Code != domaingame.MessageIssueSent || outcome.NextTargetPlayerID != 77 {
		t.Fatalf("unexpected send outcome: %+v", outcome)
	}
	if len(runner.execs) != 2 ||
		!strings.Contains(runner.execs[0].sql, "DELETE FROM `ogame_messages` WHERE owner_id = ? ORDER BY date ASC LIMIT 1") ||
		!strings.Contains(runner.execs[1].sql, "INSERT INTO `ogame_messages`") ||
		runner.execs[1].args[0] != 77 || runner.execs[1].args[1] != domaingame.MessageTypePM ||
		!strings.Contains(runner.execs[1].args[2].(string), "page=galaxy") ||
		!strings.Contains(runner.execs[1].args[3].(string), "page=writemessages") ||
		runner.execs[1].args[4] != "Line 1<br />Line 2" {
		t.Fatalf("unexpected send execs: %+v", runner.execs)
	}
}

func TestMessagesRepositorySendValidatesDraftAndActivation(t *testing.T) {
	repository := NewMessagesRepositoryWithRunner(&fakeMessagesRunner{}, &fakeMessagesRunner{}, "ogame_", time.Now)
	outcome, err := repository.MutateMessages(context.Background(), appgame.MessagesMutationQuery{
		Action:         domaingame.MessagesMutationActionSend,
		TargetPlayerID: 77,
		Text:           "body",
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue == nil || outcome.ActionIssue.Code != domaingame.MessageIssueMissingSubject {
		t.Fatalf("expected missing subject issue, got %+v", outcome)
	}

	outcome, err = repository.MutateMessages(context.Background(), appgame.MessagesMutationQuery{
		Action:         domaingame.MessagesMutationActionSend,
		TargetPlayerID: 77,
		Subject:        "Hi",
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue == nil || outcome.ActionIssue.Code != domaingame.MessageIssueMissingText {
		t.Fatalf("expected missing text issue, got %+v", outcome)
	}

	outcome, err = repository.MutateMessages(context.Background(), appgame.MessagesMutationQuery{
		Action:  domaingame.MessagesMutationActionSend,
		Subject: "Hi",
		Text:    "body",
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue != nil {
		t.Fatalf("expected missing target to no-op, got %+v", outcome)
	}

	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{42, "Sender", 0, 0, "", 1, 2, 3})},
	}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	outcome, err = repository.MutateMessages(context.Background(), appgame.MessagesMutationQuery{
		PlayerID:       42,
		Action:         domaingame.MessagesMutationActionSend,
		TargetPlayerID: 77,
		Subject:        "Hi",
		Text:           "body",
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue == nil || outcome.ActionIssue.Code != domaingame.MessageIssueNotActivated || len(runner.execs) != 0 {
		t.Fatalf("expected activation issue without execs, got outcome=%+v execs=%+v", outcome, runner.execs)
	}
}

func TestMessagesRepositorySendReturnsParticipantAndInsertErrors(t *testing.T) {
	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.MutateMessages(context.Background(), appgame.MessagesMutationQuery{
		PlayerID: 42, Action: domaingame.MessagesMutationActionSend, TargetPlayerID: 77, Subject: "Hi", Text: "body",
	}); err == nil || !strings.Contains(err.Error(), "message participant not found") {
		t.Fatalf("expected missing participant error, got %v", err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{42, "Sender", 1, 0, "", 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{77, "Recipient", 1, 1, "custom/", 2, 3, 4})},
		{rows: fakeRowsFromValues([]any{0})},
	}}, execErr: errors.New("insert failed")}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.MutateMessages(context.Background(), appgame.MessagesMutationQuery{
		PlayerID: 42, Action: domaingame.MessagesMutationActionSend, TargetPlayerID: 77, Subject: "Hi", Text: "body",
	}); err == nil || !strings.Contains(err.Error(), "insert failed") {
		t.Fatalf("expected insert error, got %v", err)
	}
	if len(runner.execs) != 1 || !strings.Contains(runner.execs[0].args[3].(string), "custom/img/m.gif") {
		t.Fatalf("expected custom skin reply icon in insert, got %+v", runner.execs)
	}
}

func TestMessagesRepositoryDeletesAndReportsInboxMessages(t *testing.T) {
	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{int64(0)})},
		{rows: fakeRowsFromValues(
			[]any{11, domaingame.MessageTypePM, "Sender", "Subject", "Body", 0, int64(1)},
			[]any{12, domaingame.MessageTypeSpyReport, "Spy", "Spy", "Text", 0, int64(2)},
		)},
		{rows: fakeRowsFromValues([]any{11, domaingame.MessageTypePM, "Sender", "Subject", "Body", 0, int64(1)})},
		{rows: fakeRowsFromValues()},
	}}}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)

	outcome, err := repository.MutateMessages(context.Background(), appgame.MessagesMutationQuery{
		PlayerID:   42,
		Action:     domaingame.MessagesMutationActionDelete,
		DeleteMode: domaingame.MessageDeleteModeMarked,
		MessageIDs: []int{12},
		ReportIDs:  []int{11, 12},
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue == nil || outcome.ActionIssue.Code != domaingame.MessageIssueReported {
		t.Fatalf("unexpected report outcome: %+v", outcome)
	}
	if len(runner.execs) != 2 ||
		!strings.Contains(runner.execs[0].sql, "INSERT INTO `ogame_reports`") ||
		!strings.Contains(runner.execs[1].sql, "DELETE FROM `ogame_messages` WHERE owner_id = ? AND msg_id = ?") ||
		runner.execs[1].args[1] != 12 {
		t.Fatalf("unexpected delete/report execs: %+v", runner.execs)
	}
}

func TestMessagesRepositoryDeleteModesSelectVisibleRows(t *testing.T) {
	repository := NewMessagesRepositoryWithRunner(&fakeMessagesRunner{}, &fakeMessagesRunner{}, "ogame_", time.Now)
	rows := []domaingame.Message{{ID: 1}, {ID: 2}, {ID: 3}}
	unmarked := repository.messageDeleteIDs(domaingame.MessageDeleteModeNonMarked, rows, []int{2})
	if len(unmarked) != 2 || unmarked[0] != 1 || unmarked[1] != 3 {
		t.Fatalf("unexpected unmarked delete ids: %+v", unmarked)
	}
	allShown := repository.messageDeleteIDs(domaingame.MessageDeleteModeAllShown, rows, []int{2})
	if len(allShown) != 3 {
		t.Fatalf("unexpected all-shown delete ids: %+v", allShown)
	}
	none := repository.messageDeleteIDs(domaingame.MessageDeleteModeNone, rows, []int{2})
	if len(none) != 0 {
		t.Fatalf("unexpected no-op delete ids: %+v", none)
	}
}

func TestMessagesRepositoryInboxMutationEdges(t *testing.T) {
	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("commander failed")}}}}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.mutateInboxMessages(context.Background(), "`ogame_messages`", "`ogame_users`", "`ogame_reports`", appgame.MessagesMutationQuery{PlayerID: 42}); err == nil || !strings.Contains(err.Error(), "commander failed") {
		t.Fatalf("expected commander error, got %v", err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{int64(0)})},
		{err: errors.New("inbox failed")},
	}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.mutateInboxMessages(context.Background(), "`ogame_messages`", "`ogame_users`", "`ogame_reports`", appgame.MessagesMutationQuery{PlayerID: 42}); err == nil || !strings.Contains(err.Error(), "inbox failed") {
		t.Fatalf("expected inbox error, got %v", err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{int64(0)})},
		{rows: fakeRowsFromValues([]any{11, domaingame.MessageTypePM, "Sender", "Subject", "Body", 0, int64(1)})},
	}}, execErr: errors.New("delete failed")}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.mutateInboxMessages(context.Background(), "`ogame_messages`", "`ogame_users`", "`ogame_reports`", appgame.MessagesMutationQuery{
		PlayerID:   42,
		DeleteMode: domaingame.MessageDeleteModeAllShown,
	}); err == nil || !strings.Contains(err.Error(), "delete failed") {
		t.Fatalf("expected delete error, got %v", err)
	}
}

func TestMessagesRepositoryHandlesReportDuplicatesAndDeleteAll(t *testing.T) {
	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{int64(0)})},
		{rows: fakeRowsFromValues([]any{11, domaingame.MessageTypePM, "Sender", "Subject", "Body", 0, int64(1)})},
		{rows: fakeRowsFromValues([]any{11, domaingame.MessageTypePM, "Sender", "Subject", "Body", 0, int64(1)})},
		{rows: fakeRowsFromValues([]any{99})},
	}}}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	outcome, err := repository.MutateMessages(context.Background(), appgame.MessagesMutationQuery{
		PlayerID:  42,
		ReportIDs: []int{11},
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue == nil || outcome.ActionIssue.Code != domaingame.MessageIssueReportExists || len(runner.execs) != 0 {
		t.Fatalf("expected duplicate report issue without execs, got outcome=%+v execs=%+v", outcome, runner.execs)
	}

	runner = &fakeMessagesRunner{}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.MutateMessages(context.Background(), appgame.MessagesMutationQuery{PlayerID: 42, DeleteMode: domaingame.MessageDeleteModeAllMessages}); err != nil {
		t.Fatal(err)
	}
	if len(runner.execs) != 1 || !strings.Contains(runner.execs[0].sql, "DELETE FROM `ogame_messages` WHERE owner_id = ?") {
		t.Fatalf("expected delete-all exec, got %+v", runner.execs)
	}
}

func TestMessagesRepositoryMutationErrors(t *testing.T) {
	repository := NewMessagesRepositoryWithQueryer(&fakeQueryer{}, "ogame_", time.Now)
	if _, err := repository.MutateMessages(context.Background(), appgame.MessagesMutationQuery{}); err == nil || !strings.Contains(err.Error(), "updater unavailable") {
		t.Fatalf("expected updater error, got %v", err)
	}
	runner := &fakeMessagesRunner{}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "bad-prefix_", time.Now)
	if _, err := repository.MutateMessages(context.Background(), appgame.MessagesMutationQuery{}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected unsafe prefix error, got %v", err)
	}
}

func TestMessagesRepositoryReportAndCountEdges(t *testing.T) {
	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("owned query failed")}}}}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.reportMessage(context.Background(), "`ogame_messages`", "`ogame_reports`", 42, 11); err == nil || !strings.Contains(err.Error(), "owned query failed") {
		t.Fatalf("expected owned query error, got %v", err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{11, domaingame.MessageTypePM, "Sender", "Subject", "Body", 0, int64(1)})},
		{err: errors.New("report exists failed")},
	}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.reportMessage(context.Background(), "`ogame_messages`", "`ogame_reports`", 42, 11); err == nil || !strings.Contains(err.Error(), "report exists failed") {
		t.Fatalf("expected report exists error, got %v", err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{11, domaingame.MessageTypePM, "Sender", "Subject", "Body", 0, int64(1)})},
		{rows: fakeRowsFromValues()},
	}}, execErr: errors.New("report insert failed")}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.reportMessage(context.Background(), "`ogame_messages`", "`ogame_reports`", 42, 11); err == nil || !strings.Contains(err.Error(), "report insert failed") {
		t.Fatalf("expected report insert error, got %v", err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.countMessages(context.Background(), "`ogame_messages`", 42); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected count scan error, got %v", err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("count rows failed"), []any{0})}}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.countMessages(context.Background(), "`ogame_messages`", 42); err == nil || !strings.Contains(err.Error(), "count rows failed") {
		t.Fatalf("expected count rows error, got %v", err)
	}
}

func TestMessagesRepositoryMessageRowEdges(t *testing.T) {
	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("owned rows failed"))}}}}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.loadOwnedMessage(context.Background(), "`ogame_messages`", 42, 11); err == nil || !strings.Contains(err.Error(), "owned rows failed") {
		t.Fatalf("expected owned rows error, got %v", err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 0, "", "", "", 0, int64(1)})}}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.loadOwnedMessage(context.Background(), "`ogame_messages`", 42, 11); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected owned scan error, got %v", err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("owned post rows failed"), []any{11, domaingame.MessageTypePM, "Sender", "Subject", "Body", 0, int64(1)})}}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.loadOwnedMessage(context.Background(), "`ogame_messages`", 42, 11); err == nil || !strings.Contains(err.Error(), "owned post rows failed") {
		t.Fatalf("expected owned post rows error, got %v", err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	record, err := repository.loadOwnedMessage(context.Background(), "`ogame_messages`", 42, 11)
	if err != nil || record != nil {
		t.Fatalf("expected missing owned message to return nil, got record=%+v err=%v", record, err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("participant rows failed"))}}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.loadMessageParticipant(context.Background(), "`ogame_users`", "`ogame_planets`", 42); err == nil || !strings.Contains(err.Error(), "participant rows failed") {
		t.Fatalf("expected participant rows error, got %v", err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("participant query failed")}}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.loadMessageParticipant(context.Background(), "`ogame_users`", "`ogame_planets`", 42); err == nil || !strings.Contains(err.Error(), "participant query failed") {
		t.Fatalf("expected participant query error, got %v", err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", "Sender", 1, 0, "", 1, 2, 3})}}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.loadMessageParticipant(context.Background(), "`ogame_users`", "`ogame_planets`", 42); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected participant scan error, got %v", err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("participant post rows failed"), []any{42, "Sender", 1, 0, "", 1, 2, 3})}}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.loadMessageParticipant(context.Background(), "`ogame_users`", "`ogame_planets`", 42); err == nil || !strings.Contains(err.Error(), "participant post rows failed") {
		t.Fatalf("expected participant post rows error, got %v", err)
	}
}

func TestMessagesRepositoryReportCountAndInsertNoOps(t *testing.T) {
	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{11, domaingame.MessageTypeSpyReport, "Spy", "Subject", "Body", 0, int64(1)})},
	}}}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	issue, err := repository.reportMessage(context.Background(), "`ogame_messages`", "`ogame_reports`", 42, 11)
	if err != nil || issue != nil || len(runner.execs) != 0 {
		t.Fatalf("expected non-PM report to no-op, issue=%+v err=%v execs=%+v", issue, err, runner.execs)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	count, err := repository.countMessages(context.Background(), "`ogame_messages`", 42)
	if err != nil || count != 0 {
		t.Fatalf("expected missing count row to return zero, count=%d err=%v", count, err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("count query failed")}}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if err := repository.insertPrivateMessage(context.Background(), "`ogame_messages`", 42, "from", "subject", "text"); err == nil || !strings.Contains(err.Error(), "count query failed") {
		t.Fatalf("expected count query error, got %v", err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{127})}}}, execErr: errors.New("delete oldest failed")}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if err := repository.insertPrivateMessage(context.Background(), "`ogame_messages`", 42, "from", "subject", "text"); err == nil || !strings.Contains(err.Error(), "delete oldest failed") {
		t.Fatalf("expected delete oldest error, got %v", err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("report rows failed"), []any{1})}}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.reportExists(context.Background(), "`ogame_reports`", 11); err == nil || !strings.Contains(err.Error(), "report rows failed") {
		t.Fatalf("expected report rows error, got %v", err)
	}
}

func TestMessagesRepositoryReturnsErrors(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		queryer *fakeQueryer
		query   appgame.MessagesQuery
		want    string
	}{
		{name: "unsafe prefix", prefix: "bad-prefix_", queryer: &fakeQueryer{}, want: "invalid database table prefix"},
		{name: "overview", prefix: "ogame_", queryer: &fakeQueryer{results: []fakeQueryResult{{err: errors.New("overview failed")}}}, want: "overview failed"},
		{name: "commander query", prefix: "ogame_", queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{err: errors.New("commander failed")})}, want: "commander failed"},
		{name: "inbox query", prefix: "ogame_", queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0)})}, fakeQueryResult{err: errors.New("inbox failed")})}, want: "inbox failed"},
		{name: "compose query", prefix: "ogame_", queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{err: errors.New("compose failed")})}, query: appgame.MessagesQuery{TargetPlayerID: 77}, want: "compose failed"},
		{name: "missing compose target", prefix: "ogame_", queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues()})}, query: appgame.MessagesQuery{TargetPlayerID: 77}, want: "message target not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewMessagesRepositoryWithQueryer(tt.queryer, tt.prefix, time.Now)
			_, err := repository.GetMessages(context.Background(), tt.query)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

type fakeMessagesExec struct {
	sql  string
	args []any
}

type fakeMessagesRunner struct {
	fakeQueryer
	execs   []fakeMessagesExec
	execErr error
}

func (f *fakeMessagesRunner) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.execs = append(f.execs, fakeMessagesExec{sql: query, args: args})
	if f.execErr != nil {
		return nil, f.execErr
	}
	return fakeSQLResult(1), nil
}

func TestMessagesRepositoryScanEdges(t *testing.T) {
	repository := NewMessagesRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 0, "from", "subj", "text", 0, int64(1)})}}}, "ogame_", time.Now)
	if _, err := repository.loadInboxRows(context.Background(), "ogame_messages", 42, 25); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected inbox scan error, got %v", err)
	}

	repository = NewMessagesRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("inbox rows failed"), []any{11, 0, "from", "subj", "text", 0, int64(1)})}}}, "ogame_", time.Now)
	if _, err := repository.loadInboxRows(context.Background(), "ogame_messages", 42, 25); err == nil || !strings.Contains(err.Error(), "inbox rows failed") {
		t.Fatalf("expected inbox rows error, got %v", err)
	}

	repository = NewMessagesRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_", time.Now)
	if _, err := repository.loadCommanderActive(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "expected int64") {
		t.Fatalf("expected commander scan error, got %v", err)
	}

	repository = NewMessagesRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_", time.Now)
	if _, err := repository.loadCommanderActive(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "message commander state not found") {
		t.Fatalf("expected missing commander error, got %v", err)
	}

	repository = NewMessagesRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("commander rows failed"))}}}, "ogame_", time.Now)
	if _, err := repository.loadCommanderActive(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "commander rows failed") {
		t.Fatalf("expected commander rows error, got %v", err)
	}

	repository = NewMessagesRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("target rows failed"))}}}, "ogame_", time.Now)
	if _, err := repository.loadComposeTarget(context.Background(), "ogame_users", "ogame_planets", 77); err == nil || !strings.Contains(err.Error(), "target rows failed") {
		t.Fatalf("expected compose rows error, got %v", err)
	}

	repository = NewMessagesRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", "Target", 1, 2, 3})}}}, "ogame_", time.Now)
	if _, err := repository.loadComposeTarget(context.Background(), "ogame_users", "ogame_planets", 77); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected compose scan error, got %v", err)
	}

	repository = NewMessagesRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("target post scan failed"), []any{77, "Target", 1, 2, 3})}}}, "ogame_", time.Now)
	if _, err := repository.loadComposeTarget(context.Background(), "ogame_users", "ogame_planets", 77); err == nil || !strings.Contains(err.Error(), "target post scan failed") {
		t.Fatalf("expected compose post scan error, got %v", err)
	}
}
