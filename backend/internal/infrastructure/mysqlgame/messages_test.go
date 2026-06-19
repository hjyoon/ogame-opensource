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

func TestMessagesRepositoryReadsLegacyInbox(t *testing.T) {
	now := time.Unix(1700000000, 0)
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix()})},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{11, domaingame.MessageTypePM, "Sender", "Subject", "Body", 0, int64(1700000000)},
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
	repository = NewMessagesRepositoryWithQueryer(&fakeQueryer{}, "ogame_", nil)
	if repository.now == nil {
		t.Fatal("nil clock should default to time.Now")
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
