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
	queryer := &fakeQueryer{results: messageInboxResults(
		fakeQueryResult{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), domaingame.AdminLevelPlayer})},
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
	if len(messages.Operators) != 1 || messages.Operators[0].Name != "QA Type Operator" || messages.Operators[0].Subject != "Question from Legor of the 1 universe" {
		t.Fatalf("unexpected operators: %+v", messages.Operators)
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

func TestMessagesRepositoryDeletesExpiredInboxMessagesOnRead(t *testing.T) {
	now := time.Unix(1700000000, 0)
	tests := []struct {
		name           string
		commanderUntil int64
		adminLevel     int
		wantDelete     bool
		wantThreshold  int64
	}{
		{
			name:          "regular",
			wantDelete:    true,
			wantThreshold: now.Add(-24 * time.Hour).Unix(),
		},
		{
			name:           "commander",
			commanderUntil: now.Add(time.Hour).Unix(),
			wantDelete:     true,
			wantThreshold:  now.Add(-7 * 24 * time.Hour).Unix(),
		},
		{
			name:       "admin",
			adminLevel: domaingame.AdminLevelOperator,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: messageInboxResults(
				fakeQueryResult{rows: fakeRowsFromValues([]any{tt.commanderUntil, tt.adminLevel})},
				fakeQueryResult{rows: fakeRowsFromValues()},
			)}}
			repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
			if _, err := repository.GetMessages(context.Background(), appgame.MessagesQuery{PlayerID: 42, PlanetID: 99}); err != nil {
				t.Fatal(err)
			}
			if !tt.wantDelete {
				if len(runner.execs) != 0 {
					t.Fatalf("expected no expiry cleanup, got %+v", runner.execs)
				}
				return
			}
			if len(runner.execs) != 1 ||
				!strings.Contains(runner.execs[0].sql, "DELETE FROM `ogame_messages` WHERE owner_id = ? AND date <= ?") ||
				runner.execs[0].args[0] != 42 ||
				runner.execs[0].args[1] != tt.wantThreshold {
				t.Fatalf("unexpected expiry cleanup execs: %+v", runner.execs)
			}
		})
	}
}

func TestMessagesRepositoryMarksVisibleInboxMessagesReadOnRead(t *testing.T) {
	now := time.Unix(1700000000, 0)
	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: messageInboxResults(
		fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0), domaingame.AdminLevelPlayer})},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{11, domaingame.MessageTypePM, "Sender", "Subject", "Body", 0, int64(1)},
			[]any{12, domaingame.MessageTypeMisc, "System", "Notice", "Text", 1, int64(2)},
		)},
	)}}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	messages, err := repository.GetMessages(context.Background(), appgame.MessagesQuery{PlayerID: 42, PlanetID: 99})
	if err != nil {
		t.Fatal(err)
	}
	if len(messages.Rows) != 2 || !messages.Rows[0].Unread || messages.Rows[1].Unread {
		t.Fatalf("expected response to keep pre-mark unread flags, got %+v", messages.Rows)
	}
	if len(runner.execs) != 2 ||
		!strings.Contains(runner.execs[0].sql, "DELETE FROM `ogame_messages` WHERE owner_id = ? AND date <= ?") ||
		!strings.Contains(runner.execs[1].sql, "UPDATE `ogame_messages` SET shown = 1 WHERE owner_id = ? AND msg_id IN (?, ?)") ||
		runner.execs[1].args[0] != 42 ||
		runner.execs[1].args[1] != 11 ||
		runner.execs[1].args[2] != 12 {
		t.Fatalf("unexpected mark-read execs: %+v", runner.execs)
	}
}

func TestMessagesRepositoryReadsComposeTarget(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{77, "Target", 2, 3, 4})},
	)}
	repository := NewMessagesRepositoryWithQueryer(queryer, "ogame_", time.Now)

	messages, err := repository.GetMessages(context.Background(), appgame.MessagesQuery{PlayerID: 42, TargetPlayerID: 77, Subject: "Re: Target"})
	if err != nil {
		t.Fatal(err)
	}
	if messages.Action != domaingame.MessagesActionCompose || messages.Compose == nil ||
		messages.Compose.Target.Name != "Target" || messages.Compose.Subject != "Re: Target" {
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

func TestMessagesRepositoryMutationReachableErrorEdges(t *testing.T) {
	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{int64(0)})},
		{rows: fakeRowsFromValues([]any{11, domaingame.MessageTypePM, "Sender", "Subject", "Body", 0, int64(1)})},
		{err: errors.New("owned reload failed")},
	}}}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.MutateMessages(context.Background(), appgame.MessagesMutationQuery{
		PlayerID:  42,
		ReportIDs: []int{11},
	}); err == nil || !strings.Contains(err.Error(), "owned reload failed") {
		t.Fatalf("expected report reload error, got %v", err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{42, "Sender", 1, 0, "", 1, 2, 3})},
		{err: errors.New("recipient lookup failed")},
	}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.MutateMessages(context.Background(), appgame.MessagesMutationQuery{
		PlayerID:       42,
		Action:         domaingame.MessagesMutationActionSend,
		TargetPlayerID: 77,
		Subject:        "Hi",
		Text:           "body",
	}); err == nil || !strings.Contains(err.Error(), "recipient lookup failed") {
		t.Fatalf("expected recipient lookup error, got %v", err)
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

func TestMessagesRepositoryPostRowErrorEdges(t *testing.T) {
	repository := NewMessagesRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("count empty rows failed"))}}}, "ogame_", time.Now)
	if _, err := repository.countMessages(context.Background(), "`ogame_messages`", 42); err == nil || !strings.Contains(err.Error(), "count empty rows failed") {
		t.Fatalf("expected empty count rows error, got %v", err)
	}

	repository = NewMessagesRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("commander post rows failed"), []any{int64(1700000000)})}}}, "ogame_", time.Now)
	if _, err := repository.loadCommanderActive(context.Background(), "`ogame_users`", 42); err == nil || !strings.Contains(err.Error(), "commander post rows failed") {
		t.Fatalf("expected commander post rows error, got %v", err)
	}

	repository = NewMessagesRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("retention post rows failed"), []any{int64(1700000000), domaingame.AdminLevelPlayer})}}}, "ogame_", time.Now)
	if _, err := repository.loadMessageRetentionState(context.Background(), "`ogame_users`", 42); err == nil || !strings.Contains(err.Error(), "retention post rows failed") {
		t.Fatalf("expected retention post rows error, got %v", err)
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
		{name: "inbox query", prefix: "ogame_", queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0), domaingame.AdminLevelPlayer})}, fakeQueryResult{err: errors.New("inbox failed")})}, want: "inbox failed"},
		{name: "operators query", prefix: "ogame_", queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0), domaingame.AdminLevelPlayer})}, fakeQueryResult{rows: fakeRowsFromValues()}, fakeQueryResult{err: errors.New("operator subject failed")})}, want: "operator subject failed"},
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

func TestMessagesRepositoryGetMessagesWriteErrorEdges(t *testing.T) {
	runner := &fakeMessagesRunner{
		fakeQueryer: fakeQueryer{results: append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0), domaingame.AdminLevelPlayer})},
		)},
		execErr: errors.New("delete expired failed"),
	}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(1700000000, 0) })
	if _, err := repository.GetMessages(context.Background(), appgame.MessagesQuery{PlayerID: 42}); err == nil || !strings.Contains(err.Error(), "delete expired failed") {
		t.Fatalf("expected delete expired error, got %v", err)
	}

	runner = &fakeMessagesRunner{
		fakeQueryer: fakeQueryer{results: append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0), domaingame.AdminLevelPlayer})},
			fakeQueryResult{rows: fakeRowsFromValues([]any{11, domaingame.MessageTypePM, "Sender", "Subject", "Body", 0, int64(1)})},
		)},
		execErrs: []error{nil, errors.New("mark read failed")},
	}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(1700000000, 0) })
	if _, err := repository.GetMessages(context.Background(), appgame.MessagesQuery{PlayerID: 42}); err == nil || !strings.Contains(err.Error(), "mark read failed") {
		t.Fatalf("expected mark read error, got %v", err)
	}
}

func TestMessagesRepositoryOperatorSubjectAndRowsEdges(t *testing.T) {
	repository := NewMessagesRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues()},
	}}, "ogame_", time.Now)
	subject, err := repository.loadOperatorSubject(context.Background(), "`ogame_users`", "`ogame_uni`", 42)
	if err != nil || subject != "Question from  of the 1 universe" {
		t.Fatalf("expected fallback operator subject, got subject=%q err=%v", subject, err)
	}

	cases := []struct {
		name    string
		results []fakeQueryResult
		run     func(MessagesRepository) error
		want    string
	}{
		{
			name:    "subject user query",
			results: []fakeQueryResult{{err: errors.New("user subject failed")}},
			run: func(repository MessagesRepository) error {
				_, err := repository.loadOperatorSubject(context.Background(), "`ogame_users`", "`ogame_uni`", 42)
				return err
			},
			want: "user subject failed",
		},
		{
			name:    "subject user scan",
			results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}},
			run: func(repository MessagesRepository) error {
				_, err := repository.loadOperatorSubject(context.Background(), "`ogame_users`", "`ogame_uni`", 42)
				return err
			},
			want: "expected string",
		},
		{
			name:    "subject user rows",
			results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("user rows failed"))}},
			run: func(repository MessagesRepository) error {
				_, err := repository.loadOperatorSubject(context.Background(), "`ogame_users`", "`ogame_uni`", 42)
				return err
			},
			want: "user rows failed",
		},
		{
			name: "subject universe query",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"Legor"})},
				{err: errors.New("uni subject failed")},
			},
			run: func(repository MessagesRepository) error {
				_, err := repository.loadOperatorSubject(context.Background(), "`ogame_users`", "`ogame_uni`", 42)
				return err
			},
			want: "uni subject failed",
		},
		{
			name: "subject universe scan",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"Legor"})},
				{rows: fakeRowsFromValues([]any{"bad"})},
			},
			run: func(repository MessagesRepository) error {
				_, err := repository.loadOperatorSubject(context.Background(), "`ogame_users`", "`ogame_uni`", 42)
				return err
			},
			want: "expected int",
		},
		{
			name: "subject universe rows",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"Legor"})},
				{rows: fakeRowsFromValuesWithErr(errors.New("uni rows failed"))},
			},
			run: func(repository MessagesRepository) error {
				_, err := repository.loadOperatorSubject(context.Background(), "`ogame_users`", "`ogame_uni`", 42)
				return err
			},
			want: "uni rows failed",
		},
		{
			name: "operators query",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"Legor"})},
				{rows: fakeRowsFromValues([]any{1})},
				{err: errors.New("operators failed")},
			},
			run: func(repository MessagesRepository) error {
				_, err := repository.loadOperators(context.Background(), "`ogame_users`", "`ogame_uni`", 42)
				return err
			},
			want: "operators failed",
		},
		{
			name: "operators scan",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"Legor"})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValues([]any{101})},
			},
			run: func(repository MessagesRepository) error {
				_, err := repository.loadOperators(context.Background(), "`ogame_users`", "`ogame_uni`", 42)
				return err
			},
			want: "unexpected scan destination count",
		},
		{
			name: "operators rows",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"Legor"})},
				{rows: fakeRowsFromValues([]any{1})},
				{rows: fakeRowsFromValuesWithErr(errors.New("operators rows failed"), []any{101, "operator", "email", int64(0)})},
			},
			run: func(repository MessagesRepository) error {
				_, err := repository.loadOperators(context.Background(), "`ogame_users`", "`ogame_uni`", 42)
				return err
			},
			want: "operators rows failed",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewMessagesRepositoryWithQueryer(&fakeQueryer{results: tt.results}, "ogame_", time.Now)
			if err := tt.run(repository); err == nil || !strings.Contains(err.Error(), tt.want) {
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
	execs    []fakeMessagesExec
	execErr  error
	execErrs []error
}

func messageInboxResults(results ...fakeQueryResult) []fakeQueryResult {
	all := append(shipyardOverviewResults(), results...)
	return append(all, messageOperatorResults()...)
}

func messageOperatorResults() []fakeQueryResult {
	return []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"Legor"})},
		{rows: fakeRowsFromValues([]any{1})},
		{rows: fakeRowsFromValues([]any{101, "QA Type Operator", "operator@example.local", int64(domaingame.UserFlagHideGOEmail)})},
	}
}

func (f *fakeMessagesRunner) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.execs = append(f.execs, fakeMessagesExec{sql: query, args: args})
	if len(f.execErrs) > 0 {
		err := f.execErrs[0]
		f.execErrs = f.execErrs[1:]
		if err != nil {
			return nil, err
		}
		return fakeSQLResult(1), nil
	}
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

func TestMessagesRepositoryRetentionAndLegacySlashEdges(t *testing.T) {
	now := time.Unix(1_000, 0)
	tests := []struct {
		name  string
		rows  *fakeRows
		err   error
		want  messageRetentionState
		match string
	}{
		{name: "query error", err: errors.New("retention query failed"), match: "retention query failed"},
		{name: "missing row", rows: fakeRowsFromValues(), match: "message retention state not found"},
		{name: "rows error", rows: fakeRowsError(errors.New("retention rows failed")), match: "retention rows failed"},
		{name: "scan error", rows: fakeRowsFromValues([]any{"bad", domaingame.AdminLevelPlayer}), match: "expected int64"},
		{name: "player", rows: fakeRowsFromValues([]any{now.Add(-time.Hour).Unix(), domaingame.AdminLevelPlayer}), want: messageRetentionState{}},
		{name: "commander admin", rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), domaingame.AdminLevelOperator}), want: messageRetentionState{CommanderActive: true, Admin: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryer := &fakeQueryer{results: []fakeQueryResult{{rows: tt.rows, err: tt.err}}}
			repository := NewMessagesRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
			got, err := repository.loadMessageRetentionState(context.Background(), "ogame_users", 42)
			if tt.match != "" {
				if err == nil || !strings.Contains(err.Error(), tt.match) {
					t.Fatalf("expected %q error, got state=%+v err=%v", tt.match, got, err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("unexpected retention state=%+v err=%v", got, err)
			}
		})
	}

	slashed := legacyStripSlashes(`a\\b\'c\"d\0e\q\`)
	if slashed != "a\\b'c\"d\x00e\\q\\" {
		t.Fatalf("unexpected stripped slash value %q", slashed)
	}
	if legacyStripSlashes("plain") != "plain" {
		t.Fatal("plain value should not be rewritten")
	}
}
