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

func TestBuddyRepositoryReadsLegacyHomeRows(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(buddyRow(11, 43, "allymate", 7, "TAG", 0, 1, 2, 4, 1_000, "hello"))},
	)}
	repository := NewBuddyRepositoryWithQueryer(queryer, "ogame_")
	repository.now = func() time.Time { return time.Unix(2_000, 0) }

	buddy, err := repository.GetBuddy(context.Background(), appgame.BuddyQuery{PlayerID: 42})
	if err != nil {
		t.Fatal(err)
	}
	if buddy.Commander != "legor" || buddy.Action != domaingame.BuddyActionHome || len(buddy.Rows) != 1 {
		t.Fatalf("unexpected buddy summary: %+v", buddy)
	}
	row := buddy.Rows[0]
	if row.BuddyID != 11 || row.Player.PlayerID != 43 || row.Player.Alliance == nil || !row.Player.Alliance.Founder ||
		row.Player.Coordinates.Position != 4 || row.Text != "hello" || row.Status.Text != "16 min" || row.Status.Color != "yellow" {
		t.Fatalf("unexpected buddy row: %+v", row)
	}
	if !strings.Contains(queryer.calls[4].sql, "accepted = 1") || !strings.Contains(queryer.calls[4].sql, "CASE WHEN b.request_from = ?") {
		t.Fatalf("expected accepted buddy CASE query, got %s", queryer.calls[4].sql)
	}
}

func TestBuddyRepositoryReadsIncomingRows(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(buddyRow(12, 44, "requester", 0, "", 1, 1, 2, 5, 1_900, "please"))},
	)}
	repository := NewBuddyRepositoryWithQueryer(queryer, "ogame_")
	repository.now = func() time.Time { return time.Unix(2_000, 0) }

	buddy, err := repository.GetBuddy(context.Background(), appgame.BuddyQuery{PlayerID: 42, Action: domaingame.BuddyActionIncoming})
	if err != nil {
		t.Fatal(err)
	}
	if buddy.Action != domaingame.BuddyActionIncoming || len(buddy.Rows) != 1 || buddy.Rows[0].Player.Alliance != nil || buddy.Rows[0].Status.Text != "On" {
		t.Fatalf("unexpected incoming buddy rows: %+v", buddy.Rows)
	}
	if !strings.Contains(queryer.calls[4].sql, "b.request_to = ? AND b.accepted = 0") || queryer.calls[4].args[0] != 42 {
		t.Fatalf("expected incoming request query, got %+v", queryer.calls[4])
	}
}

func TestBuddyRepositoryReadsOutgoingRows(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(buddyRow(13, 45, "target", 8, "FOO", 2, 1, 2, 6, 1_000, "out"))},
	)}
	repository := NewBuddyRepositoryWithQueryer(queryer, "ogame_")

	buddy, err := repository.GetBuddy(context.Background(), appgame.BuddyQuery{PlayerID: 42, Action: domaingame.BuddyActionOutgoing})
	if err != nil {
		t.Fatal(err)
	}
	if buddy.Action != domaingame.BuddyActionOutgoing || len(buddy.Rows) != 1 || buddy.Rows[0].Player.Alliance.Founder {
		t.Fatalf("unexpected outgoing buddy rows: %+v", buddy.Rows)
	}
	if !strings.Contains(queryer.calls[4].sql, "b.request_from = ? AND b.accepted = 0") {
		t.Fatalf("expected outgoing request query, got %s", queryer.calls[4].sql)
	}
}

func TestBuddyRepositoryReadsRequestTarget(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{43, "target", 7, "TAG", 0, 1, 2, 4})},
	)}
	repository := NewBuddyRepositoryWithQueryer(queryer, "ogame_")

	buddy, err := repository.GetBuddy(context.Background(), appgame.BuddyQuery{PlayerID: 42, Action: domaingame.BuddyActionRequest, BuddyID: 43})
	if err != nil {
		t.Fatal(err)
	}
	if buddy.Action != domaingame.BuddyActionRequest || buddy.Target == nil || buddy.Target.Name != "target" ||
		buddy.Target.Alliance == nil || !buddy.Target.Alliance.Founder {
		t.Fatalf("unexpected request target: %+v", buddy.Target)
	}
	if !strings.Contains(queryer.calls[4].sql, "WHERE u.player_id = ? LIMIT 1") || queryer.calls[4].args[0] != 43 {
		t.Fatalf("expected request target query, got %+v", queryer.calls[4])
	}
}

func TestBuddyRepositoryHandlesMissingRequestTarget(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues()})}
	repository := NewBuddyRepositoryWithQueryer(queryer, "ogame_")

	buddy, err := repository.GetBuddy(context.Background(), appgame.BuddyQuery{PlayerID: 42, Action: domaingame.BuddyActionRequest, BuddyID: 43})
	if err != nil {
		t.Fatal(err)
	}
	if buddy.Target != nil {
		t.Fatalf("expected missing target, got %+v", buddy.Target)
	}
}

func TestNewBuddyRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewBuddyRepository(nil, "ogame_")
	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
}

func TestBuddyRepositoryReturnsErrors(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		queryer *fakeQueryer
		query   appgame.BuddyQuery
		want    string
	}{
		{name: "unsafe prefix", prefix: "bad-prefix_", queryer: &fakeQueryer{}, want: "invalid database table prefix"},
		{name: "overview", prefix: "ogame_", queryer: &fakeQueryer{results: []fakeQueryResult{{err: errors.New("overview failed")}}}, want: "overview failed"},
		{name: "home rows", prefix: "ogame_", queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{err: errors.New("buddy rows failed")})}, want: "buddy rows failed"},
		{name: "target rows", prefix: "ogame_", queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{err: errors.New("target failed")})}, query: appgame.BuddyQuery{Action: domaingame.BuddyActionRequest, BuddyID: 1}, want: "target failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewBuddyRepositoryWithQueryer(tt.queryer, tt.prefix)
			_, err := repository.GetBuddy(context.Background(), tt.query)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestBuddyRepositoryScanEdges(t *testing.T) {
	repository := NewBuddyRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 43, "target", 0, "", 1, 1, 2, 4, int64(1), ""})}}}, "ogame_")
	if _, err := repository.loadBuddyRows(context.Background(), "ogame_buddy", "ogame_users", "ogame_planets", "ogame_ally", "b.request_to = ?", "b.request_from", 42); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected buddy row scan error, got %v", err)
	}

	repository = NewBuddyRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("buddy rows post failed"), buddyRow(1, 43, "target", 0, "", 1, 1, 2, 4, 1, ""))}}}, "ogame_")
	if _, err := repository.loadBuddyRows(context.Background(), "ogame_buddy", "ogame_users", "ogame_planets", "ogame_ally", "b.request_to = ?", "b.request_from", 42); err == nil || !strings.Contains(err.Error(), "buddy rows post failed") {
		t.Fatalf("expected buddy rows post error, got %v", err)
	}

	repository = NewBuddyRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", "target", 0, "", 1, 1, 2, 4})}}}, "ogame_")
	if _, err := repository.loadBuddyTarget(context.Background(), "ogame_users", "ogame_planets", "ogame_ally", 43); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected target scan error, got %v", err)
	}

	repository = NewBuddyRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("target rows post failed"), []any{43, "target", 0, "", 1, 1, 2, 4})}}}, "ogame_")
	if _, err := repository.loadBuddyTarget(context.Background(), "ogame_users", "ogame_planets", "ogame_ally", 43); err == nil || !strings.Contains(err.Error(), "target rows post failed") {
		t.Fatalf("expected target post rows error, got %v", err)
	}
}

func buddyRow(buddyID int, playerID int, name string, allianceID int, allianceTag string, allianceRank int, galaxy int, system int, position int, lastClick int64, text string) []any {
	return []any{buddyID, playerID, name, allianceID, allianceTag, allianceRank, galaxy, system, position, lastClick, text}
}
