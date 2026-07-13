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
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

func TestMessagesRepositoryReadsLegacyInbox(t *testing.T) {
	now := time.Unix(1700000000, 0)
	queryer := &fakeQueryer{results: messageInboxResults(
		fakeQueryResult{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), domaingame.AdminLevelPlayer, int64(0)})},
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

func TestMessagesRepositoryFiltersLegacyInboxByMessageType(t *testing.T) {
	now := time.Unix(1700000000, 0)
	queryer := &fakeQueryer{results: messageInboxResults(
		fakeQueryResult{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), domaingame.AdminLevelPlayer, int64(0)})},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{12, domaingame.MessageTypeMisc, "System", "Notice", "Text", 1, int64(1700000001)},
		)},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{domaingame.MessageTypeMisc, 1, 0},
		)},
	)}
	repository := NewMessagesRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })

	messages, err := repository.GetMessages(context.Background(), appgame.MessagesQuery{
		PlayerID:             42,
		PlanetID:             99,
		LegacyFolderDisplay:  true,
		MessageTypeFilter:    domaingame.MessageTypeMisc,
		HasMessageTypeFilter: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if messages.Action != domaingame.MessagesActionInbox || len(messages.Rows) != 1 || messages.Rows[0].Type != domaingame.MessageTypeMisc ||
		len(messages.Summary) != 6 || messages.Summary[5].Total != 1 {
		t.Fatalf("unexpected filtered inbox payload: %+v", messages)
	}
	if !strings.Contains(queryer.calls[5].sql, "pm <> ? AND pm = ? ORDER BY date DESC, msg_id DESC LIMIT ?") ||
		queryer.calls[5].args[1] != domaingame.MessageTypeBattleReportText ||
		queryer.calls[5].args[2] != domaingame.MessageTypeMisc ||
		queryer.calls[5].args[3] != domaingame.MessagesLimitCommander {
		t.Fatalf("expected filtered legacy messages query, got %+v", queryer.calls[5])
	}
}

func TestMessagesRepositoryLoadInboxRowsFilterBranch(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(
		[]any{12, domaingame.MessageTypePM, "Sender", "Subject", "Body", 1, int64(1700000001)},
	)}}}
	repository := NewMessagesRepositoryWithQueryer(queryer, "ogame_", time.Now)

	rows, err := repository.loadInboxRows(context.Background(), "`ogame_messages`", 42, 25, domaingame.MessageTypePM, true)
	if err != nil || len(rows) != 1 || rows[0].Type != domaingame.MessageTypePM {
		t.Fatalf("expected filtered inbox row, rows=%+v err=%v", rows, err)
	}
	if !strings.Contains(queryer.calls[0].sql, "pm <> ? AND pm = ? ORDER BY date DESC, msg_id DESC LIMIT ?") ||
		queryer.calls[0].args[1] != domaingame.MessageTypeBattleReportText ||
		queryer.calls[0].args[2] != domaingame.MessageTypePM ||
		queryer.calls[0].args[3] != 25 {
		t.Fatalf("expected typed inbox query, got %+v", queryer.calls[0])
	}
}

func TestMessagesRepositoryLoadsMessageCategoryCounts(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(
		[]any{domaingame.MessageTypeSpyReport, 2, 1},
		[]any{domaingame.MessageTypeBattleReportLink, 3, 0},
		[]any{domaingame.MessageTypeExpedition, 4, 2},
		[]any{domaingame.MessageTypeAlliance, 5, 3},
		[]any{domaingame.MessageTypePM, 6, 4},
		[]any{domaingame.MessageTypeMisc, 7, 5},
		[]any{domaingame.MessageTypeMisc, 8, 6},
	)}}}
	repository := NewMessagesRepositoryWithQueryer(queryer, "ogame_", time.Now)

	counts, err := repository.loadMessageCategoryCounts(context.Background(), "`ogame_messages`", 42)

	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		key    string
		total  int
		unread int
	}{
		{key: "spy", total: 2, unread: 1},
		{key: "battle", total: 3, unread: 0},
		{key: "expedition", total: 4, unread: 2},
		{key: "alliance", total: 5, unread: 3},
		{key: "personal", total: 6, unread: 4},
		{key: "other", total: 15, unread: 11},
	}
	if len(counts) != len(want) {
		t.Fatalf("unexpected category count length: %+v", counts)
	}
	for index, expected := range want {
		if counts[index].Key != expected.key || counts[index].Total != expected.total || counts[index].Unread != expected.unread {
			t.Fatalf("unexpected category at %d: got %+v want %+v", index, counts[index], expected)
		}
	}
	if len(queryer.calls) != 1 || !strings.Contains(queryer.calls[0].sql, "GROUP BY pm") ||
		queryer.calls[0].args[0] != 42 || queryer.calls[0].args[1] != domaingame.MessageTypeBattleReportText {
		t.Fatalf("unexpected category query: %+v", queryer.calls)
	}
}

func TestMessagesRepositoryReadsSummaryCategories(t *testing.T) {
	now := time.Unix(1700000000, 0)
	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: messageInboxResults(
		fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0), domaingame.AdminLevelPlayer, int64(0)})},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{domaingame.MessageTypeSpyReport, 2, 1},
			[]any{domaingame.MessageTypePM, 3, 2},
		)},
	)}}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	messages, err := repository.GetMessages(context.Background(), appgame.MessagesQuery{
		PlayerID:    42,
		PlanetID:    99,
		ShowSummary: true,
	})

	if err != nil {
		t.Fatal(err)
	}
	if messages.Action != domaingame.MessagesActionSummary || len(messages.Summary) != 6 ||
		messages.Summary[0].Total != 2 || messages.Summary[0].Unread != 1 ||
		messages.Summary[4].Total != 3 || messages.Summary[4].Unread != 2 {
		t.Fatalf("unexpected summary payload: %+v", messages)
	}
	if len(messages.Operators) != 1 || !messages.Operators[0].HideEmail ||
		messages.Operators[0].Subject != "Question from Legor of the 1 universe" {
		t.Fatalf("unexpected summary operators: %+v", messages.Operators)
	}
	if len(runner.execs) != 1 || !strings.Contains(runner.execs[0].sql, "DELETE FROM `ogame_messages` WHERE owner_id = ? AND date <= ?") ||
		runner.execs[0].args[0] != 42 ||
		runner.execs[0].args[1] != now.Add(-24*time.Hour).Unix() {
		t.Fatalf("unexpected summary expiry cleanup: %+v", runner.execs)
	}
}

func TestMessagesRepositoryUsesLegacyCommanderFolderSummary(t *testing.T) {
	now := time.Unix(1700000000, 0)
	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: messageInboxResults(
		fakeQueryResult{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), domaingame.AdminLevelPlayer, int64(0)})},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{domaingame.MessageTypeSpyReport, 2, 1},
			[]any{domaingame.MessageTypePM, 3, 2},
		)},
	)}}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	messages, err := repository.GetMessages(context.Background(), appgame.MessagesQuery{
		PlayerID:            42,
		PlanetID:            99,
		LegacyFolderDisplay: true,
	})

	if err != nil {
		t.Fatal(err)
	}
	if messages.Action != domaingame.MessagesActionSummary || len(messages.Rows) != 0 ||
		len(messages.Summary) != 6 || messages.Summary[0].Total != 2 || messages.Summary[4].Total != 3 {
		t.Fatalf("unexpected legacy folder summary payload: %+v", messages)
	}
	if len(runner.execs) != 1 || !strings.Contains(runner.execs[0].sql, "DELETE FROM `ogame_messages` WHERE owner_id = ? AND date <= ?") {
		t.Fatalf("expected expiry cleanup before folder summary, got %+v", runner.execs)
	}
}

func TestMessagesRepositoryMessageCategoryCountEdges(t *testing.T) {
	repository := NewMessagesRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("category query failed")}}}, "ogame_", time.Now)
	if _, err := repository.loadMessageCategoryCounts(context.Background(), "`ogame_messages`", 42); err == nil || !strings.Contains(err.Error(), "category query failed") {
		t.Fatalf("expected category query error, got %v", err)
	}

	repository = NewMessagesRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 1, 0})}}}, "ogame_", time.Now)
	if _, err := repository.loadMessageCategoryCounts(context.Background(), "`ogame_messages`", 42); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected category scan error, got %v", err)
	}

	repository = NewMessagesRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("category rows failed"), []any{domaingame.MessageTypePM, 1, 0})}}}, "ogame_", time.Now)
	if _, err := repository.loadMessageCategoryCounts(context.Background(), "`ogame_messages`", 42); err == nil || !strings.Contains(err.Error(), "category rows failed") {
		t.Fatalf("expected category rows error, got %v", err)
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
				fakeQueryResult{rows: fakeRowsFromValues([]any{tt.commanderUntil, tt.adminLevel, int64(0)})},
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
		fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0), domaingame.AdminLevelPlayer, int64(0)})},
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

func TestMessagesRepositoryMCPSendMessagePreviewAndExecute(t *testing.T) {
	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{42, "Sender", 1, 0, "", 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{77, "Recipient", 1, 0, "", 2, 3, 4})},
		{rows: fakeRowsFromValues([]any{42, "Sender", 1, 0, "", 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{77, "Recipient", 1, 0, "", 2, 3, 4})},
		{rows: fakeRowsFromValues([]any{0})},
	}}}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })

	preview, err := repository.PreviewMCPSendMessage(context.Background(), 42, domainmcp.SendMessageCommand{
		TargetPlayerID: 77,
		Subject:        "Hello",
		Text:           "Line 1\nLine 2",
	})
	if err != nil {
		t.Fatalf("PreviewMCPSendMessage returned error: %v", err)
	}
	if !preview.DryRun || preview.Executed || preview.Issue != nil || preview.TargetPlayerID != 77 || preview.TextChars != len([]rune("Line 1\nLine 2")) || len(runner.execs) != 0 {
		t.Fatalf("unexpected preview=%+v execs=%+v", preview, runner.execs)
	}

	sent, err := repository.SendMCPMessage(context.Background(), 42, domainmcp.SendMessageCommand{
		TargetPlayerID: 77,
		Subject:        "Hello",
		Text:           "Line 1\nLine 2",
	})
	if err != nil {
		t.Fatalf("SendMCPMessage returned error: %v", err)
	}
	if sent.DryRun || !sent.Executed || sent.Issue == nil || sent.Issue.Code != domaingame.MessageIssueSent || len(runner.execs) != 1 {
		t.Fatalf("unexpected send=%+v execs=%+v", sent, runner.execs)
	}
	if !strings.Contains(runner.execs[0].sql, "INSERT INTO `ogame_messages`") || runner.execs[0].args[0] != 77 {
		t.Fatalf("expected legacy insert, got %+v", runner.execs)
	}
}

func TestMessagesRepositoryMCPDeleteMessagesPreviewAndExecute(t *testing.T) {
	messageRows := func() *fakeRows {
		return fakeRowsFromValues(
			[]any{11, domaingame.MessageTypePM, "Sender", "Subject", "Body", 0, int64(1)},
			[]any{12, domaingame.MessageTypeSpyReport, "Spy", "Spy", "Text", 0, int64(2)},
		)
	}
	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{int64(0)})},
		{rows: messageRows()},
		{rows: fakeRowsFromValues([]any{int64(0)})},
		{rows: messageRows()},
		{rows: fakeRowsFromValues([]any{int64(0)})},
		{rows: messageRows()},
	}}}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)

	preview, err := repository.PreviewMCPDeleteMessages(context.Background(), 42, domainmcp.DeleteMessagesCommand{MessageIDs: []int{12, 99}})
	if err != nil {
		t.Fatalf("PreviewMCPDeleteMessages returned error: %v", err)
	}
	if !preview.DryRun || preview.Executed || preview.DeleteCount != 1 || len(preview.MessageIDs) != 1 || preview.MessageIDs[0] != 12 || len(runner.execs) != 0 {
		t.Fatalf("unexpected delete preview=%+v execs=%+v", preview, runner.execs)
	}

	deleted, err := repository.DeleteMCPMessages(context.Background(), 42, domainmcp.DeleteMessagesCommand{MessageIDs: []int{12, 99}})
	if err != nil {
		t.Fatalf("DeleteMCPMessages returned error: %v", err)
	}
	if deleted.DryRun || !deleted.Executed || deleted.DeleteCount != 1 || len(deleted.MessageIDs) != 1 || deleted.MessageIDs[0] != 12 {
		t.Fatalf("unexpected delete execute: %+v", deleted)
	}
	if len(runner.execs) != 1 || !strings.Contains(runner.execs[0].sql, "DELETE FROM `ogame_messages` WHERE owner_id = ? AND msg_id = ?") ||
		runner.execs[0].args[0] != 42 || runner.execs[0].args[1] != 12 {
		t.Fatalf("expected legacy marked delete exec, got %+v", runner.execs)
	}
}

func TestMessagesRepositoryMCPDeleteMessagesValidationAndErrors(t *testing.T) {
	repository := NewMessagesRepositoryWithRunner(nil, nil, "ogame_", time.Now)
	if _, err := repository.PreviewMCPDeleteMessages(context.Background(), 42, domainmcp.DeleteMessagesCommand{MessageIDs: []int{7}}); err == nil {
		t.Fatalf("expected preview queryer error")
	}

	repository = NewMessagesRepositoryWithRunner(&fakeMessagesRunner{}, nil, "ogame_", time.Now)
	if _, err := repository.DeleteMCPMessages(context.Background(), 42, domainmcp.DeleteMessagesCommand{MessageIDs: []int{7}}); err == nil {
		t.Fatalf("expected execute updater error")
	}

	repository = NewMessagesRepositoryWithRunner(&fakeMessagesRunner{}, &fakeMessagesRunner{}, "bad-prefix_", time.Now)
	if _, err := repository.PreviewMCPDeleteMessages(context.Background(), 42, domainmcp.DeleteMessagesCommand{MessageIDs: []int{7}}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected unsafe prefix error, got %v", err)
	}

	runner := &fakeMessagesRunner{}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	preview, err := repository.PreviewMCPDeleteMessages(context.Background(), 42, domainmcp.DeleteMessagesCommand{})
	if err != nil || !preview.DryRun || preview.DeleteCount != 0 || len(runner.calls) != 0 || len(runner.execs) != 0 {
		t.Fatalf("expected empty delete no-op, preview=%+v err=%v calls=%+v execs=%+v", preview, err, runner.calls, runner.execs)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("commander failed")}}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.PreviewMCPDeleteMessages(context.Background(), 42, domainmcp.DeleteMessagesCommand{MessageIDs: []int{7}}); err == nil || !strings.Contains(err.Error(), "commander failed") {
		t.Fatalf("expected commander error, got %v", err)
	}
}

func TestMessagesRepositoryMCPReportMessagePreviewAndExecute(t *testing.T) {
	messageRows := func() *fakeRows {
		return fakeRowsFromValues(
			[]any{11, domaingame.MessageTypePM, "Sender", "Subject", "Body", 0, int64(1)},
			[]any{12, domaingame.MessageTypeSpyReport, "Spy", "Spy", "Text", 0, int64(2)},
		)
	}
	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{int64(0)})},
		{rows: messageRows()},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{int64(0)})},
		{rows: messageRows()},
		{rows: fakeRowsFromValues([]any{int64(0)})},
		{rows: messageRows()},
		{rows: fakeRowsFromValues([]any{11, domaingame.MessageTypePM, "Sender", "Subject", "Body", 0, int64(1)})},
		{rows: fakeRowsFromValues()},
	}}}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)

	preview, err := repository.PreviewMCPReportMessage(context.Background(), 42, domainmcp.ReportMessageCommand{MessageID: 11})
	if err != nil {
		t.Fatalf("PreviewMCPReportMessage returned error: %v", err)
	}
	if !preview.DryRun || preview.Executed || !preview.Reportable || preview.Issue != nil || len(runner.execs) != 0 {
		t.Fatalf("unexpected report preview=%+v execs=%+v", preview, runner.execs)
	}

	reported, err := repository.ReportMCPMessage(context.Background(), 42, domainmcp.ReportMessageCommand{MessageID: 11})
	if err != nil {
		t.Fatalf("ReportMCPMessage returned error: %v", err)
	}
	if reported.DryRun || !reported.Executed || !reported.Reportable || reported.Issue == nil || reported.Issue.Code != domaingame.MessageIssueReported {
		t.Fatalf("unexpected report execute: %+v", reported)
	}
	if len(runner.execs) != 1 || !strings.Contains(runner.execs[0].sql, "INSERT INTO `ogame_reports`") ||
		runner.execs[0].args[0] != 42 || runner.execs[0].args[1] != 11 {
		t.Fatalf("expected legacy report insert exec, got %+v", runner.execs)
	}
}

func TestMessagesRepositoryMCPReportMessageValidationAndErrors(t *testing.T) {
	repository := NewMessagesRepositoryWithRunner(nil, nil, "ogame_", time.Now)
	if _, err := repository.PreviewMCPReportMessage(context.Background(), 42, domainmcp.ReportMessageCommand{MessageID: 7}); err == nil {
		t.Fatalf("expected preview queryer error")
	}

	repository = NewMessagesRepositoryWithRunner(&fakeMessagesRunner{}, nil, "ogame_", time.Now)
	if _, err := repository.ReportMCPMessage(context.Background(), 42, domainmcp.ReportMessageCommand{MessageID: 7}); err == nil {
		t.Fatalf("expected execute updater error")
	}

	repository = NewMessagesRepositoryWithRunner(&fakeMessagesRunner{}, &fakeMessagesRunner{}, "bad-prefix_", time.Now)
	if _, err := repository.PreviewMCPReportMessage(context.Background(), 42, domainmcp.ReportMessageCommand{MessageID: 7}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected unsafe prefix error, got %v", err)
	}

	runner := &fakeMessagesRunner{}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	preview, err := repository.PreviewMCPReportMessage(context.Background(), 42, domainmcp.ReportMessageCommand{})
	if err != nil || !preview.DryRun || preview.Reportable || len(runner.calls) != 0 || len(runner.execs) != 0 {
		t.Fatalf("expected empty report no-op, preview=%+v err=%v calls=%+v execs=%+v", preview, err, runner.calls, runner.execs)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{int64(0)})},
		{rows: fakeRowsFromValues([]any{12, domaingame.MessageTypeSpyReport, "Spy", "Spy", "Text", 0, int64(2)})},
	}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	preview, err = repository.PreviewMCPReportMessage(context.Background(), 42, domainmcp.ReportMessageCommand{MessageID: 12})
	if err != nil || preview.Reportable || preview.RequiresConfirmation || len(runner.execs) != 0 {
		t.Fatalf("expected non-PM report no-op, preview=%+v err=%v execs=%+v", preview, err, runner.execs)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{int64(0)})},
		{rows: fakeRowsFromValues([]any{12, domaingame.MessageTypeSpyReport, "Spy", "Spy", "Text", 0, int64(2)})},
	}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	reported, err := repository.ReportMCPMessage(context.Background(), 42, domainmcp.ReportMessageCommand{MessageID: 12})
	if err != nil || reported.Reportable || reported.Executed || len(runner.execs) != 0 {
		t.Fatalf("expected execute non-PM report no-op, reported=%+v err=%v execs=%+v", reported, err, runner.execs)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{int64(0)})},
		{rows: fakeRowsFromValues([]any{11, domaingame.MessageTypePM, "Sender", "Subject", "Body", 0, int64(1)})},
		{rows: fakeRowsFromValues([]any{99})},
	}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	preview, err = repository.PreviewMCPReportMessage(context.Background(), 42, domainmcp.ReportMessageCommand{MessageID: 11})
	if err != nil || !preview.Reportable || preview.Issue == nil || preview.Issue.Code != domaingame.MessageIssueReportExists || len(runner.execs) != 0 {
		t.Fatalf("expected duplicate report issue, preview=%+v err=%v execs=%+v", preview, err, runner.execs)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{int64(0)})},
		{err: errors.New("inbox failed")},
	}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.PreviewMCPReportMessage(context.Background(), 42, domainmcp.ReportMessageCommand{MessageID: 11}); err == nil || !strings.Contains(err.Error(), "inbox failed") {
		t.Fatalf("expected inbox error, got %v", err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{int64(0)})},
		{rows: fakeRowsFromValues([]any{11, domaingame.MessageTypePM, "Sender", "Subject", "Body", 0, int64(1)})},
		{err: errors.New("mutate commander failed")},
	}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.ReportMCPMessage(context.Background(), 42, domainmcp.ReportMessageCommand{MessageID: 11}); err == nil || !strings.Contains(err.Error(), "mutate commander failed") {
		t.Fatalf("expected mutate error, got %v", err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("commander failed")}}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.PreviewMCPReportMessage(context.Background(), 42, domainmcp.ReportMessageCommand{MessageID: 7}); err == nil || !strings.Contains(err.Error(), "commander failed") {
		t.Fatalf("expected commander error, got %v", err)
	}
}

func TestMessagesRepositoryMCPSendMessageValidationAndErrors(t *testing.T) {
	repository := NewMessagesRepositoryWithRunner(&fakeMessagesRunner{}, &fakeMessagesRunner{}, "ogame_", time.Now)
	preview, err := repository.PreviewMCPSendMessage(context.Background(), 42, domainmcp.SendMessageCommand{Subject: "Hi", Text: "body"})
	if err != nil {
		t.Fatalf("missing target preview returned error: %v", err)
	}
	if preview.Issue != nil || preview.TargetPlayerID != 0 || !preview.DryRun {
		t.Fatalf("expected missing target no-op preview, got %+v", preview)
	}
	if mcpActionIssue(nil) != nil {
		t.Fatalf("nil game issue should map to nil mcp issue")
	}

	preview, err = repository.PreviewMCPSendMessage(context.Background(), 42, domainmcp.SendMessageCommand{TargetPlayerID: 77, Text: "body"})
	if err != nil {
		t.Fatalf("missing subject preview returned error: %v", err)
	}
	if preview.Issue == nil || preview.Issue.Code != domaingame.MessageIssueMissingSubject {
		t.Fatalf("expected missing subject issue, got %+v", preview)
	}
	preview, err = repository.PreviewMCPSendMessage(context.Background(), 42, domainmcp.SendMessageCommand{TargetPlayerID: 77, Subject: "Hi"})
	if err != nil {
		t.Fatalf("missing text preview returned error: %v", err)
	}
	if preview.Issue == nil || preview.Issue.Code != domaingame.MessageIssueMissingText {
		t.Fatalf("expected missing text issue, got %+v", preview)
	}

	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{42, "Sender", 0, 0, "", 1, 2, 3})},
	}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	preview, err = repository.PreviewMCPSendMessage(context.Background(), 42, domainmcp.SendMessageCommand{TargetPlayerID: 77, Subject: "Hi", Text: "body"})
	if err != nil {
		t.Fatalf("not activated preview returned error: %v", err)
	}
	if preview.Issue == nil || preview.Issue.Code != domaingame.MessageIssueNotActivated || len(runner.execs) != 0 {
		t.Fatalf("expected activation issue without execs, got preview=%+v execs=%+v", preview, runner.execs)
	}

	repository = NewMessagesRepositoryWithRunner(nil, nil, "ogame_", time.Now)
	if _, err := repository.PreviewMCPSendMessage(context.Background(), 42, domainmcp.SendMessageCommand{TargetPlayerID: 77, Subject: "Hi", Text: "body"}); err == nil {
		t.Fatalf("expected nil queryer preview error")
	}
	repository = NewMessagesRepositoryWithRunner(&fakeMessagesRunner{}, nil, "ogame_", time.Now)
	if _, err := repository.SendMCPMessage(context.Background(), 42, domainmcp.SendMessageCommand{TargetPlayerID: 77, Subject: "Hi", Text: "body"}); err == nil {
		t.Fatalf("expected nil updater send error")
	}
	repository = NewMessagesRepositoryWithRunner(&fakeMessagesRunner{}, &fakeMessagesRunner{}, "bad-prefix_", time.Now)
	if _, err := repository.PreviewMCPSendMessage(context.Background(), 42, domainmcp.SendMessageCommand{TargetPlayerID: 77, Subject: "Hi", Text: "body"}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected invalid prefix preview error, got %v", err)
	}

	runner = &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{42, "Sender", 1, 0, "", 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{77, "Recipient", 1, 0, "", 2, 3, 4})},
		{rows: fakeRowsFromValues([]any{0})},
	}}, execErr: errors.New("mcp insert failed")}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.SendMCPMessage(context.Background(), 42, domainmcp.SendMessageCommand{TargetPlayerID: 77, Subject: "Hi", Text: "body"}); err == nil || !strings.Contains(err.Error(), "mcp insert failed") {
		t.Fatalf("expected send insert error, got %v", err)
	}
}

func TestMessagesRepositoryMCPSendMessageExecuteValidationIssues(t *testing.T) {
	repository := NewMessagesRepositoryWithRunner(&fakeMessagesRunner{}, &fakeMessagesRunner{}, "ogame_", time.Now)
	sent, err := repository.SendMCPMessage(context.Background(), 42, domainmcp.SendMessageCommand{Subject: "Hi", Text: "body"})
	if err != nil {
		t.Fatalf("missing target send returned error: %v", err)
	}
	if sent.Issue != nil || sent.TargetPlayerID != 0 || sent.DryRun {
		t.Fatalf("expected missing target no-op send, got %+v", sent)
	}

	sent, err = repository.SendMCPMessage(context.Background(), 42, domainmcp.SendMessageCommand{TargetPlayerID: 77, Text: "body"})
	if err != nil {
		t.Fatalf("missing subject send returned error: %v", err)
	}
	if sent.Issue == nil || sent.Issue.Code != domaingame.MessageIssueMissingSubject || sent.DryRun {
		t.Fatalf("expected missing subject send issue, got %+v", sent)
	}

	sent, err = repository.SendMCPMessage(context.Background(), 42, domainmcp.SendMessageCommand{TargetPlayerID: 77, Subject: "Hi"})
	if err != nil {
		t.Fatalf("missing text send returned error: %v", err)
	}
	if sent.Issue == nil || sent.Issue.Code != domaingame.MessageIssueMissingText || sent.DryRun {
		t.Fatalf("expected missing text send issue, got %+v", sent)
	}

	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{42, "Sender", 0, 0, "", 1, 2, 3})},
	}}}
	repository = NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	sent, err = repository.SendMCPMessage(context.Background(), 42, domainmcp.SendMessageCommand{TargetPlayerID: 77, Subject: "Hi", Text: "body"})
	if err != nil {
		t.Fatalf("not activated send returned error: %v", err)
	}
	if sent.Issue == nil || sent.Issue.Code != domaingame.MessageIssueNotActivated || sent.Executed || len(runner.execs) != 0 {
		t.Fatalf("expected activation issue without execution, got sent=%+v execs=%+v", sent, runner.execs)
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

func TestMessagesRepositoryMutatesMessageDisplayFlags(t *testing.T) {
	now := time.Unix(1700000000, 0)
	oldFlags := messageUserFlagPartialReports | messageUserFlagFolderCategoryAll
	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix()})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), domaingame.AdminLevelPlayer, oldFlags})},
	}}}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	partialReports := false
	_, err := repository.MutateMessages(context.Background(), appgame.MessagesMutationQuery{
		PlayerID:       42,
		DeleteMode:     domaingame.MessageDeleteModeNone,
		PartialReports: &partialReports,
		FolderSelection: &appgame.MessageFolderSelection{
			Spy:        true,
			Battle:     false,
			Expedition: true,
			Alliance:   false,
			Personal:   true,
			Other:      false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantFlags := (oldFlags &^ messageUserFlagPartialReports &^ messageUserFlagFolderCategoryAll) |
		messageUserFlagFolderEspionage |
		messageUserFlagFolderExpedition |
		messageUserFlagFolderPlayer
	if len(runner.execs) != 1 ||
		!strings.Contains(runner.execs[0].sql, "UPDATE `ogame_users` SET flags = ? WHERE player_id = ?") ||
		runner.execs[0].args[0] != wantFlags ||
		runner.execs[0].args[1] != 42 {
		t.Fatalf("unexpected flag update execs: %+v", runner.execs)
	}
}

func TestMessagesRepositoryAppliesMessageDisplayFlagsOnRead(t *testing.T) {
	now := time.Unix(1700000000, 0)
	flags := messageUserFlagPartialReports | messageUserFlagFolderEspionage | messageUserFlagFolderPlayer
	runner := &fakeMessagesRunner{fakeQueryer: fakeQueryer{results: messageInboxResults(
		fakeQueryResult{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), domaingame.AdminLevelPlayer, flags})},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{11, domaingame.MessageTypeSpyReport, "Visual Control", "Visual Spy Report", "Spy body", 0, int64(1)},
			[]any{12, domaingame.MessageTypePM, "Sender", "Personal", "Body", 0, int64(2)},
		)},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{domaingame.MessageTypeSpyReport, 1, 1},
			[]any{domaingame.MessageTypePM, 1, 1},
		)},
	)}}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	messages, err := repository.GetMessages(context.Background(), appgame.MessagesQuery{
		PlayerID:            42,
		PlanetID:            99,
		LegacyFolderDisplay: true,
	})

	if err != nil {
		t.Fatal(err)
	}
	if !messages.PartialReports || len(messages.Rows) != 2 ||
		messages.Rows[0].Text != "" || !strings.Contains(messages.Rows[0].Subject, "page=bericht") ||
		!messages.Summary[0].Checked || messages.Summary[1].Checked || !messages.Summary[4].Checked {
		t.Fatalf("unexpected messages payload: %+v", messages)
	}
	foundFolderFilter := false
	for _, call := range runner.calls {
		if strings.Contains(call.sql, "pm IN") {
			foundFolderFilter = true
			break
		}
	}
	if !foundFolderFilter {
		t.Fatalf("expected folder flag inbox SQL filter, calls=%+v", runner.calls)
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

func TestMessagesRepositoryTransactionsRollBackMailboxReplacement(t *testing.T) {
	want := errors.New("message insert failed")
	runner := &fakeMessagesTransactionRunner{fakeMessagesRunner: &fakeMessagesRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{42, "Sender", 1, 1, "/evolution/", 1, 2, 3})},
			{rows: fakeRowsFromValues([]any{77, "Recipient", 1, 1, "/evolution/", 1, 2, 4})},
			{rows: fakeRowsFromValues([]any{127})},
		}},
		execErrs: []error{nil, want},
	}}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", time.Now)

	_, err := repository.MutateMessages(context.Background(), appgame.MessagesMutationQuery{
		PlayerID:       42,
		Action:         domaingame.MessagesMutationActionSend,
		TargetPlayerID: 77,
		Subject:        "Subject",
		Text:           "Body",
	})
	if !errors.Is(err, want) || !runner.rolledBack || runner.committed {
		t.Fatalf("expected failed insert to roll back oldest-message deletion, committed=%t rolledBack=%t err=%v", runner.committed, runner.rolledBack, err)
	}
	if len(runner.execs) != 2 || !strings.Contains(runner.execs[0].sql, "ORDER BY date ASC LIMIT 1") || !strings.Contains(runner.execs[1].sql, "INSERT INTO") {
		t.Fatalf("unexpected mailbox replacement statements: %+v", runner.execs)
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

	repository = NewMessagesRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("retention post rows failed"), []any{int64(1700000000), domaingame.AdminLevelPlayer, int64(0)})}}}, "ogame_", time.Now)
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
		{name: "inbox query", prefix: "ogame_", queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0), domaingame.AdminLevelPlayer, int64(0)})}, fakeQueryResult{err: errors.New("inbox failed")})}, want: "inbox failed"},
		{name: "operators query", prefix: "ogame_", queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0), domaingame.AdminLevelPlayer, int64(0)})}, fakeQueryResult{rows: fakeRowsFromValues()}, fakeQueryResult{err: errors.New("operator subject failed")})}, want: "operator subject failed"},
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
			fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0), domaingame.AdminLevelPlayer, int64(0)})},
		)},
		execErr: errors.New("delete expired failed"),
	}
	repository := NewMessagesRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(1700000000, 0) })
	if _, err := repository.GetMessages(context.Background(), appgame.MessagesQuery{PlayerID: 42}); err == nil || !strings.Contains(err.Error(), "delete expired failed") {
		t.Fatalf("expected delete expired error, got %v", err)
	}

	runner = &fakeMessagesRunner{
		fakeQueryer: fakeQueryer{results: append(shipyardOverviewResults(),
			fakeQueryResult{rows: fakeRowsFromValues([]any{int64(0), domaingame.AdminLevelPlayer, int64(0)})},
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

type fakeMessagesTransactionRunner struct {
	*fakeMessagesRunner
	committed  bool
	rolledBack bool
}

func (r *fakeMessagesTransactionRunner) WithTransaction(ctx context.Context, run func(Queryer, Execer) error) error {
	if err := run(r, r); err != nil {
		r.rolledBack = true
		return err
	}
	r.committed = true
	return nil
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
	if _, err := repository.loadInboxRows(context.Background(), "ogame_messages", 42, 25, 0, false); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected inbox scan error, got %v", err)
	}

	repository = NewMessagesRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("inbox rows failed"), []any{11, 0, "from", "subj", "text", 0, int64(1)})}}}, "ogame_", time.Now)
	if _, err := repository.loadInboxRows(context.Background(), "ogame_messages", 42, 25, 0, false); err == nil || !strings.Contains(err.Error(), "inbox rows failed") {
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
		{name: "scan error", rows: fakeRowsFromValues([]any{"bad", domaingame.AdminLevelPlayer, int64(0)}), match: "expected int64"},
		{name: "player", rows: fakeRowsFromValues([]any{now.Add(-time.Hour).Unix(), domaingame.AdminLevelPlayer, int64(0)}), want: messageRetentionState{}},
		{name: "commander admin", rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), domaingame.AdminLevelOperator, int64(messageUserFlagDontUseFolders)}), want: messageRetentionState{CommanderActive: true, Admin: true, Flags: messageUserFlagDontUseFolders}},
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
