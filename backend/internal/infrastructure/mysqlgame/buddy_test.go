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

func TestBuddyRepositoryMapsMCPBuddyStatus(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(buddyRow(11, 43, "allymate", 7, "TAG", 0, 1, 2, 4, 1_000, "hello"))},
	)}
	repository := NewBuddyRepositoryWithQueryer(queryer, "ogame_")
	repository.now = func() time.Time { return time.Unix(2_000, 0) }

	status, err := repository.GetMCPBuddyStatus(context.Background(), 42, domainmcp.BuddyStatusCommand{
		Action: domaingame.BuddyActionIncoming,
	})
	if err != nil {
		t.Fatalf("GetMCPBuddyStatus returned error: %v", err)
	}
	if status.PlayerID != 42 || status.Planet.ID != 99 || status.Planet.TypeName != "planet" || status.Action != domaingame.BuddyActionIncoming || len(status.Rows) != 1 {
		t.Fatalf("unexpected mcp buddy status: %+v", status)
	}
	row := status.Rows[0]
	if row.BuddyID != 11 || row.Player.PlayerID != 43 || row.Player.Alliance == nil || row.Player.Alliance.Tag != "TAG" ||
		row.Status.Text != "16 min" || row.Status.Color != "yellow" {
		t.Fatalf("unexpected mcp buddy row: %+v", row)
	}
	if _, err := (BuddyRepository{}).GetMCPBuddyStatus(context.Background(), 42, domainmcp.BuddyStatusCommand{}); err == nil {
		t.Fatalf("expected unavailable reader error")
	}
}

func TestBuddyRepositoryMCPMutationPreviewAndExecute(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{43, "target", 7, "TAG", 0, 1, 2, 4})},
	)}
	repository := NewBuddyRepositoryWithQueryer(queryer, "ogame_")
	preview, err := repository.PreviewMCPBuddyMutation(context.Background(), 42, domainmcp.BuddyMutationCommand{
		PlanetID: 99,
		Action:   "add",
		BuddyID:  43,
		Text:     "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.PlayerID != 42 || preview.LegacyAction != domaingame.BuddyActionAdd || preview.Status.Target == nil || preview.TextChars != 5 {
		t.Fatalf("unexpected buddy preview: %+v", preview)
	}

	results := []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor"})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{0})},
	}
	results = append(results, shipyardOverviewResults()...)
	results = append(results, fakeQueryResult{rows: fakeRowsFromValues()})
	runner := &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: results}}
	repository = NewBuddyRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
	mutated, err := repository.MutateMCPBuddy(context.Background(), 42, domainmcp.BuddyMutationCommand{
		PlanetID: 99,
		Action:   "add",
		BuddyID:  43,
		Text:     "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if mutated.LegacyAction != domaingame.BuddyActionAdd || mutated.Issue != nil || mutated.Status.Action != domaingame.BuddyActionHome || len(runner.execs) != 2 {
		t.Fatalf("unexpected buddy mutation=%+v execs=%+v", mutated, runner.execs)
	}
}

func TestBuddyRepositoryMCPMutationHelpersAndErrors(t *testing.T) {
	for _, tt := range []struct {
		action  string
		legacy  int
		preview int
	}{
		{"add", domaingame.BuddyActionAdd, domaingame.BuddyActionRequest},
		{"ACCEPT", domaingame.BuddyActionAccept, domaingame.BuddyActionIncoming},
		{"decline", domaingame.BuddyActionDecline, domaingame.BuddyActionIncoming},
		{"withdraw", domaingame.BuddyActionWithdraw, domaingame.BuddyActionOutgoing},
		{"delete", domaingame.BuddyActionDelete, domaingame.BuddyActionHome},
		{"unknown", domaingame.BuddyActionHome, domaingame.BuddyActionHome},
	} {
		if got := mcpBuddyLegacyAction(tt.action); got != tt.legacy {
			t.Fatalf("legacy action %q got %d want %d", tt.action, got, tt.legacy)
		}
		if got := mcpBuddyPreviewAction(tt.legacy); got != tt.preview {
			t.Fatalf("preview action %q got %d want %d", tt.action, got, tt.preview)
		}
	}
	if issue := mcpBuddyActionIssue(&domaingame.BuddyActionIssue{Code: "already_sent", Message: "duplicate"}); issue == nil || issue.Code != "already_sent" {
		t.Fatalf("unexpected mapped issue: %+v", issue)
	}

	queryErr := errors.New("buddy preview failed")
	repository := NewBuddyRepositoryWithRunner(&fakeQueryer{results: []fakeQueryResult{{err: queryErr}}}, nil, "ogame_", time.Now)
	if _, err := repository.PreviewMCPBuddyMutation(context.Background(), 42, domainmcp.BuddyMutationCommand{Action: "add", BuddyID: 43}); !errors.Is(err, queryErr) {
		t.Fatalf("expected preview error, got %v", err)
	}

	execErr := errors.New("buddy exec failed")
	runner := &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"legor"})}, {rows: fakeRowsFromValues()}, {rows: fakeRowsFromValues([]any{0})}}}, execErr: execErr}
	repository = NewBuddyRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.MutateMCPBuddy(context.Background(), 42, domainmcp.BuddyMutationCommand{Action: "add", BuddyID: 43}); !errors.Is(err, execErr) {
		t.Fatalf("expected mutate exec error, got %v", err)
	}
}

func TestMCPBuddyStatusMapsTargetAndNilBranches(t *testing.T) {
	status := mcpBuddyStatus(42, domaingame.Buddy{
		Commander: "legor",
		CurrentPlanet: domaingame.PlanetOverview{
			ID:   99,
			Name: "Moon",
			Type: domaingame.PlanetTypeMoon,
			Coordinates: domaingame.Coordinates{
				Galaxy:   1,
				System:   2,
				Position: 3,
			},
		},
		Action: domaingame.BuddyActionRequest,
		Rows: []domaingame.BuddyRow{{
			BuddyID: 12,
			Player: domaingame.BuddyPlayer{
				PlayerID: 44,
				Name:     "without_alliance",
			},
			Status: domaingame.BuddyStatus{Text: "Off", Color: "red"},
		}},
		Target: &domaingame.BuddyPlayer{
			PlayerID: 43,
			Name:     "target",
			Alliance: &domaingame.BuddyAlliance{ID: 7, Tag: "TAG", Founder: true},
			Coordinates: domaingame.Coordinates{
				Galaxy:   4,
				System:   5,
				Position: 6,
			},
		},
	})
	if status.Planet.TypeName != "moon" || status.Target == nil || status.Target.Alliance == nil || !status.Target.Alliance.Founder ||
		len(status.Rows) != 1 || status.Rows[0].Player.Alliance != nil {
		t.Fatalf("unexpected mapped buddy status: %+v", status)
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

func TestBuddyRepositoryReadsRequestActionWithoutTarget(t *testing.T) {
	queryer := &fakeQueryer{results: shipyardOverviewResults()}
	repository := NewBuddyRepositoryWithQueryer(queryer, "ogame_")

	buddy, err := repository.GetBuddy(context.Background(), appgame.BuddyQuery{PlayerID: 42, Action: domaingame.BuddyActionRequest})
	if err != nil {
		t.Fatal(err)
	}
	if buddy.Action != domaingame.BuddyActionRequest || buddy.Target != nil || len(queryer.calls) != len(shipyardOverviewResults()) {
		t.Fatalf("expected request form without target lookup, buddy=%+v calls=%d", buddy, len(queryer.calls))
	}
}

func TestBuddyRepositoryAddsBuddyRequest(t *testing.T) {
	runner := &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor"})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{0})},
	}}}
	repository := NewBuddyRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })

	outcome, err := repository.MutateBuddy(context.Background(), appgame.BuddyMutationQuery{PlayerID: 42, Action: domaingame.BuddyActionAdd, BuddyID: 43, Text: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.NextAction != domaingame.BuddyActionHome || outcome.ActionIssue != nil {
		t.Fatalf("unexpected add outcome: %+v", outcome)
	}
	if len(runner.execs) != 2 || !strings.Contains(runner.execs[0].sql, "INSERT INTO `ogame_buddy`") ||
		runner.execs[0].args[0] != 42 || runner.execs[0].args[1] != 43 || runner.execs[0].args[2] != "hello" ||
		!strings.Contains(runner.execs[1].sql, "INSERT INTO `ogame_messages`") || runner.execs[1].args[1] != "legor" {
		t.Fatalf("unexpected add execs: %+v", runner.execs)
	}
}

func TestBuddyRepositoryKeepsLegacyEmptyRequestMessage(t *testing.T) {
	runner := &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor"})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{0})},
	}}}
	repository := NewBuddyRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.MutateBuddy(context.Background(), appgame.BuddyMutationQuery{PlayerID: 42, Action: domaingame.BuddyActionAdd, BuddyID: 43}); err != nil {
		t.Fatal(err)
	}
	if len(runner.execs) != 2 || runner.execs[0].args[2] != "пусто" || runner.execs[1].args[3] != "" {
		t.Fatalf("legacy stores a placeholder in the relation but sends the original empty PM: %+v", runner.execs)
	}
}

func TestBuddyRepositoryRollsBackRelationWhenNotificationFails(t *testing.T) {
	base := &fakeBuddyRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{"legor"})},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues([]any{0})},
		}},
		execErrs: []error{nil, errors.New("notification failed")},
	}
	runner := &fakeBuddyTransactionRunner{fakeBuddyRunner: base}
	repository := NewBuddyRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	_, err := repository.MutateBuddy(context.Background(), appgame.BuddyMutationQuery{PlayerID: 42, Action: domaingame.BuddyActionAdd, BuddyID: 43, Text: "hello"})
	if err == nil || !strings.Contains(err.Error(), "notification failed") {
		t.Fatalf("expected notification failure, got %v", err)
	}
	if !runner.rolledBack || runner.committed || len(runner.execs) != 2 {
		t.Fatalf("expected relationship and notification rollback, got %+v", runner)
	}
}

func TestBuddyRepositoryAddReturnsAlreadySentIssue(t *testing.T) {
	runner := &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor"})},
		{rows: fakeRowsFromValues([]any{11})},
	}}}
	repository := NewBuddyRepositoryWithRunner(runner, runner, "ogame_", time.Now)

	outcome, err := repository.MutateBuddy(context.Background(), appgame.BuddyMutationQuery{PlayerID: 42, Action: domaingame.BuddyActionAdd, BuddyID: 43})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ActionIssue == nil || outcome.ActionIssue.Code != domaingame.BuddyIssueAlreadySent || len(runner.execs) != 0 {
		t.Fatalf("expected duplicate issue without execs, got outcome=%+v execs=%+v", outcome, runner.execs)
	}
}

func TestBuddyRepositoryAcceptsBuddyRequest(t *testing.T) {
	runner := &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor"})},
		{rows: fakeRowsFromValues([]any{11, 43, 42, "hello", 0})},
		{rows: fakeRowsFromValues([]any{0})},
	}}}
	repository := NewBuddyRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })

	outcome, err := repository.MutateBuddy(context.Background(), appgame.BuddyMutationQuery{PlayerID: 42, Action: domaingame.BuddyActionAccept, BuddyID: 11})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.NextAction != domaingame.BuddyActionIncoming || len(runner.execs) != 2 ||
		!strings.Contains(runner.execs[0].sql, "UPDATE `ogame_buddy` SET accepted = 1") ||
		runner.execs[1].args[0] != 43 || runner.execs[1].args[1] != "Buddylist" || runner.execs[1].args[2] != "confirm" {
		t.Fatalf("unexpected accept outcome=%+v execs=%+v", outcome, runner.execs)
	}
}

func TestBuddyRepositoryDeletesRelationAndPrunesOldMessage(t *testing.T) {
	runner := &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor"})},
		{rows: fakeRowsFromValues([]any{11, 42, 43, "hello", 1})},
		{rows: fakeRowsFromValues([]any{127})},
	}}}
	repository := NewBuddyRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })

	outcome, err := repository.MutateBuddy(context.Background(), appgame.BuddyMutationQuery{PlayerID: 42, Action: domaingame.BuddyActionDelete, BuddyID: 11})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.NextAction != domaingame.BuddyActionHome || len(runner.execs) != 3 ||
		!strings.Contains(runner.execs[0].sql, "DELETE FROM `ogame_buddy` WHERE buddy_id = ?") ||
		!strings.Contains(runner.execs[1].sql, "DELETE FROM `ogame_messages` WHERE owner_id = ? ORDER BY date ASC LIMIT 1") ||
		!strings.Contains(runner.execs[2].sql, "INSERT INTO `ogame_messages`") {
		t.Fatalf("unexpected delete outcome=%+v execs=%+v", outcome, runner.execs)
	}
}

func TestBuddyRepositoryRejectsAndWithdrawsRequests(t *testing.T) {
	tests := []struct {
		name       string
		action     int
		record     []any
		nextAction int
		wantText   string
	}{
		{
			name:       "reject",
			action:     domaingame.BuddyActionDecline,
			record:     []any{11, 43, 42, "hello", 0},
			nextAction: domaingame.BuddyActionIncoming,
			wantText:   "has declined your buddy request",
		},
		{
			name:       "withdraw",
			action:     domaingame.BuddyActionWithdraw,
			record:     []any{11, 42, 43, "hello", 0},
			nextAction: domaingame.BuddyActionOutgoing,
			wantText:   "Buddy request cancelled",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor"})},
				{rows: fakeRowsFromValues(tt.record)},
				{rows: fakeRowsFromValues([]any{0})},
			}}}
			repository := NewBuddyRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })

			outcome, err := repository.MutateBuddy(context.Background(), appgame.BuddyMutationQuery{PlayerID: 42, Action: tt.action, BuddyID: 11})
			if err != nil {
				t.Fatal(err)
			}
			if outcome.NextAction != tt.nextAction || len(runner.execs) != 2 ||
				!strings.Contains(runner.execs[0].sql, "DELETE FROM `ogame_buddy`") ||
				!strings.Contains(runner.execs[1].args[3].(string), tt.wantText) {
				t.Fatalf("unexpected %s outcome=%+v execs=%+v", tt.name, outcome, runner.execs)
			}
		})
	}
}

func TestBuddyRepositoryIgnoresUnauthorizedMutations(t *testing.T) {
	tests := []struct {
		name   string
		action int
		record []any
	}{
		{name: "accept wrong recipient", action: domaingame.BuddyActionAccept, record: []any{11, 43, 44, "hello", 0}},
		{name: "reject wrong recipient", action: domaingame.BuddyActionDecline, record: []any{11, 43, 44, "hello", 0}},
		{name: "withdraw wrong sender", action: domaingame.BuddyActionWithdraw, record: []any{11, 43, 42, "hello", 0}},
		{name: "delete unrelated", action: domaingame.BuddyActionDelete, record: []any{11, 43, 44, "hello", 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor"})},
				{rows: fakeRowsFromValues(tt.record)},
			}}}
			repository := NewBuddyRepositoryWithRunner(runner, runner, "ogame_", time.Now)
			if _, err := repository.MutateBuddy(context.Background(), appgame.BuddyMutationQuery{PlayerID: 42, Action: tt.action, BuddyID: 11}); err != nil {
				t.Fatal(err)
			}
			if len(runner.execs) != 0 {
				t.Fatalf("unauthorized mutation should not write, got %+v", runner.execs)
			}
		})
	}
}

func TestBuddyRepositoryMutationNoopBranches(t *testing.T) {
	tests := []struct {
		name    string
		query   appgame.BuddyMutationQuery
		results []fakeQueryResult
	}{
		{name: "self request", query: appgame.BuddyMutationQuery{PlayerID: 42, Action: domaingame.BuddyActionAdd, BuddyID: 42}, results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"legor"})}}},
		{name: "missing target request", query: appgame.BuddyMutationQuery{PlayerID: 42, Action: domaingame.BuddyActionAdd}, results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"legor"})}}},
		{name: "unknown action", query: appgame.BuddyMutationQuery{PlayerID: 42, Action: 99}, results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"legor"})}}},
		{name: "missing buddy record", query: appgame.BuddyMutationQuery{PlayerID: 42, Action: domaingame.BuddyActionDelete, BuddyID: 11}, results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"legor"})}, {rows: fakeRowsFromValues()}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: tt.results}}
			repository := NewBuddyRepositoryWithRunner(runner, runner, "ogame_", time.Now)
			if _, err := repository.MutateBuddy(context.Background(), tt.query); err != nil {
				t.Fatal(err)
			}
			if len(runner.execs) != 0 {
				t.Fatalf("noop mutation should not write, got %+v", runner.execs)
			}
		})
	}
}

func TestBuddyRepositoryMissingRecordsAreNoops(t *testing.T) {
	for _, action := range []int{domaingame.BuddyActionAccept, domaingame.BuddyActionDecline, domaingame.BuddyActionWithdraw, domaingame.BuddyActionDelete} {
		runner := &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{"legor"})},
			{rows: fakeRowsFromValues()},
		}}}
		repository := NewBuddyRepositoryWithRunner(runner, runner, "ogame_", time.Now)
		if _, err := repository.MutateBuddy(context.Background(), appgame.BuddyMutationQuery{PlayerID: 42, Action: action, BuddyID: 11}); err != nil {
			t.Fatalf("action %d should ignore missing record, got %v", action, err)
		}
		if len(runner.execs) != 0 {
			t.Fatalf("action %d should not write for missing record, got %+v", action, runner.execs)
		}
	}
}

func TestBuddyRepositoryConstructorBranches(t *testing.T) {
	runner := &fakeBuddyRunner{}
	repository := NewBuddyRepositoryWithQueryer(runner, "ogame_")
	if repository.execer == nil {
		t.Fatalf("queryer implementing Execer should be reused as execer")
	}
	repository = NewBuddyRepositoryWithRunner(runner, runner, "ogame_", nil)
	if repository.now == nil {
		t.Fatalf("nil clock should default")
	}
}

func TestBuddyRepositoryTextHelpers(t *testing.T) {
	if normalizeBuddyRequestText("") != "пусто" {
		t.Fatalf("empty buddy request should match legacy placeholder")
	}
	if truncateRunes("abcdef", 3) != "abc" || truncateRunes("abcdef", 0) != "abcdef" {
		t.Fatalf("unexpected truncate result")
	}
	long := strings.Repeat("a", 5001)
	if len(normalizeBuddyRequestText(long)) != 5000 {
		t.Fatalf("expected buddy request text to be truncated to 5000")
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
	if _, ok := repository.execer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL execer, got %T", repository.execer)
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

func TestBuddyRepositoryMutationReturnsErrors(t *testing.T) {
	if _, err := NewBuddyRepositoryWithQueryer(&fakeQueryer{}, "ogame_").MutateBuddy(context.Background(), appgame.BuddyMutationQuery{}); err == nil || !strings.Contains(err.Error(), "updater") {
		t.Fatalf("expected missing updater error, got %v", err)
	}

	runner := &fakeBuddyRunner{}
	repository := NewBuddyRepositoryWithRunner(runner, runner, "bad-prefix_", time.Now)
	if _, err := repository.MutateBuddy(context.Background(), appgame.BuddyMutationQuery{}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected mutation table prefix error, got %v", err)
	}

	runner = &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("user failed")}}}}
	repository = NewBuddyRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.MutateBuddy(context.Background(), appgame.BuddyMutationQuery{PlayerID: 42}); err == nil || !strings.Contains(err.Error(), "user failed") {
		t.Fatalf("expected user load error, got %v", err)
	}

	runner = &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor"})},
		{err: errors.New("exists failed")},
	}}}
	repository = NewBuddyRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.MutateBuddy(context.Background(), appgame.BuddyMutationQuery{PlayerID: 42, Action: domaingame.BuddyActionAdd, BuddyID: 43}); err == nil || !strings.Contains(err.Error(), "exists failed") {
		t.Fatalf("expected relationship error, got %v", err)
	}

	runner = &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor"})},
		{rows: fakeRowsFromValues()},
	}}, execErr: errors.New("insert failed")}
	repository = NewBuddyRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.MutateBuddy(context.Background(), appgame.BuddyMutationQuery{PlayerID: 42, Action: domaingame.BuddyActionAdd, BuddyID: 43}); err == nil || !strings.Contains(err.Error(), "insert failed") {
		t.Fatalf("expected insert error, got %v", err)
	}

	runner = &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor"})},
		{rows: fakeRowsFromValues([]any{11, 43, 42, "hello", 0})},
	}}, execErr: errors.New("update failed")}
	repository = NewBuddyRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.MutateBuddy(context.Background(), appgame.BuddyMutationQuery{PlayerID: 42, Action: domaingame.BuddyActionAccept, BuddyID: 11}); err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("expected update error, got %v", err)
	}

	runner = &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor"})},
		{rows: fakeRowsFromValues([]any{11, 43, 42, "hello", 0})},
		{rows: fakeRowsFromValues([]any{0})},
	}}, execErrs: []error{nil, errors.New("accept message failed")}}
	repository = NewBuddyRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.MutateBuddy(context.Background(), appgame.BuddyMutationQuery{PlayerID: 42, Action: domaingame.BuddyActionAccept, BuddyID: 11}); err == nil || !strings.Contains(err.Error(), "accept message failed") {
		t.Fatalf("expected accept message error, got %v", err)
	}
}

func TestBuddyRepositoryRemovalMutationReturnsErrors(t *testing.T) {
	tests := []struct {
		name   string
		action int
		record []any
		want   string
	}{
		{name: "reject remove", action: domaingame.BuddyActionDecline, record: []any{11, 43, 42, "hello", 0}, want: "reject remove failed"},
		{name: "withdraw remove", action: domaingame.BuddyActionWithdraw, record: []any{11, 42, 43, "hello", 0}, want: "withdraw remove failed"},
		{name: "delete remove", action: domaingame.BuddyActionDelete, record: []any{11, 43, 42, "hello", 1}, want: "delete remove failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor"})},
				{rows: fakeRowsFromValues(tt.record)},
			}}, execErr: errors.New(tt.want)}
			repository := NewBuddyRepositoryWithRunner(runner, runner, "ogame_", time.Now)
			if _, err := repository.MutateBuddy(context.Background(), appgame.BuddyMutationQuery{PlayerID: 42, Action: tt.action, BuddyID: 11}); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestBuddyRepositoryRemovalMutationReturnsMessageErrors(t *testing.T) {
	tests := []struct {
		name   string
		action int
		record []any
		want   string
	}{
		{name: "reject message", action: domaingame.BuddyActionDecline, record: []any{11, 43, 42, "hello", 0}, want: "reject message failed"},
		{name: "withdraw message", action: domaingame.BuddyActionWithdraw, record: []any{11, 42, 43, "hello", 0}, want: "withdraw message failed"},
		{name: "delete message", action: domaingame.BuddyActionDelete, record: []any{11, 43, 42, "hello", 1}, want: "delete message failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor"})},
				{rows: fakeRowsFromValues(tt.record)},
				{rows: fakeRowsFromValues([]any{0})},
			}}, execErrs: []error{nil, errors.New(tt.want)}}
			repository := NewBuddyRepositoryWithRunner(runner, runner, "ogame_", time.Now)
			if _, err := repository.MutateBuddy(context.Background(), appgame.BuddyMutationQuery{PlayerID: 42, Action: tt.action, BuddyID: 11}); err == nil || !strings.Contains(err.Error(), tt.want) {
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

	repository = NewBuddyRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("target empty rows failed"))}}}, "ogame_")
	if _, err := repository.loadBuddyTarget(context.Background(), "ogame_users", "ogame_planets", "ogame_ally", 43); err == nil || !strings.Contains(err.Error(), "target empty rows failed") {
		t.Fatalf("expected empty target rows error, got %v", err)
	}
}

func TestBuddyRepositoryHelperEdges(t *testing.T) {
	repository := NewBuddyRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_")
	if name, err := repository.loadBuddyUserName(context.Background(), "ogame_users", 42); err == nil || name != "" || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected missing user error, got name=%q err=%v", name, err)
	}

	repository = NewBuddyRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{42})}}}, "ogame_")
	if _, err := repository.loadBuddyUserName(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "expected string") {
		t.Fatalf("expected user scan error, got %v", err)
	}

	repository = NewBuddyRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("user rows failed"), []any{"legor"})}}}, "ogame_")
	if _, err := repository.loadBuddyUserName(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "user rows failed") {
		t.Fatalf("expected user rows error, got %v", err)
	}

	repository = NewBuddyRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("missing user rows failed"))}}}, "ogame_")
	if _, err := repository.loadBuddyUserName(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "missing user rows failed") {
		t.Fatalf("expected missing user rows error, got %v", err)
	}

	repository = NewBuddyRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("record query failed")}}}, "ogame_")
	if _, err := repository.loadBuddyRecord(context.Background(), "ogame_buddy", 11); err == nil || !strings.Contains(err.Error(), "record query failed") {
		t.Fatalf("expected buddy record query error, got %v", err)
	}

	repository = NewBuddyRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 43, 42, "", 0})}}}, "ogame_")
	if _, err := repository.loadBuddyRecord(context.Background(), "ogame_buddy", 11); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected buddy record scan error, got %v", err)
	}

	repository = NewBuddyRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("record empty rows failed"))}}}, "ogame_")
	if _, err := repository.loadBuddyRecord(context.Background(), "ogame_buddy", 11); err == nil || !strings.Contains(err.Error(), "record empty rows failed") {
		t.Fatalf("expected buddy record empty rows error, got %v", err)
	}

	repository = NewBuddyRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("record rows failed"), []any{11, 43, 42, "", 0})}}}, "ogame_")
	if _, err := repository.loadBuddyRecord(context.Background(), "ogame_buddy", 11); err == nil || !strings.Contains(err.Error(), "record rows failed") {
		t.Fatalf("expected buddy record rows error, got %v", err)
	}

	repository = NewBuddyRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("exists rows failed"), []any{11})}}}, "ogame_")
	if _, err := repository.buddyRelationshipExists(context.Background(), "ogame_buddy", 42, 43); err == nil || !strings.Contains(err.Error(), "exists rows failed") {
		t.Fatalf("expected exists rows error, got %v", err)
	}
}

func TestBuddyRepositoryMessageEdges(t *testing.T) {
	runner := &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("count failed")}}}}
	repository := NewBuddyRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if err := repository.sendBuddyMessage(context.Background(), "ogame_messages", 42, "from", "subject", "text"); err == nil || !strings.Contains(err.Error(), "count failed") {
		t.Fatalf("expected count error, got %v", err)
	}

	if err := repository.sendBuddyMessage(context.Background(), "ogame_messages", 0, "from", "subject", "text"); err != nil {
		t.Fatalf("zero owner message should be ignored, got %v", err)
	}

	repository = NewBuddyRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_")
	if count, err := repository.countMessages(context.Background(), "ogame_messages", 42); err != nil || count != 0 {
		t.Fatalf("missing count should return zero, got count=%d err=%v", count, err)
	}

	repository = NewBuddyRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("missing count rows failed"))}}}, "ogame_")
	if _, err := repository.countMessages(context.Background(), "ogame_messages", 42); err == nil || !strings.Contains(err.Error(), "missing count rows failed") {
		t.Fatalf("expected missing count rows error, got %v", err)
	}

	repository = NewBuddyRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "ogame_")
	if _, err := repository.countMessages(context.Background(), "ogame_messages", 42); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected count scan error, got %v", err)
	}

	repository = NewBuddyRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("count rows failed"), []any{0})}}}, "ogame_")
	if _, err := repository.countMessages(context.Background(), "ogame_messages", 42); err == nil || !strings.Contains(err.Error(), "count rows failed") {
		t.Fatalf("expected count rows error, got %v", err)
	}

	runner = &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{127})}}}, execErr: errors.New("delete oldest failed")}
	repository = NewBuddyRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if err := repository.sendBuddyMessage(context.Background(), "ogame_messages", 42, "from", "subject", "text"); err == nil || !strings.Contains(err.Error(), "delete oldest failed") {
		t.Fatalf("expected delete oldest error, got %v", err)
	}

	runner = &fakeBuddyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0})}}}, execErr: errors.New("message insert failed")}
	repository = NewBuddyRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if err := repository.sendBuddyMessage(context.Background(), "ogame_messages", 42, "from", "subject", "text"); err == nil || !strings.Contains(err.Error(), "message insert failed") {
		t.Fatalf("expected message insert error, got %v", err)
	}
}

func buddyRow(buddyID int, playerID int, name string, allianceID int, allianceTag string, allianceRank int, galaxy int, system int, position int, lastClick int64, text string) []any {
	return []any{buddyID, playerID, name, allianceID, allianceTag, allianceRank, galaxy, system, position, lastClick, text}
}

type fakeBuddyExec struct {
	sql  string
	args []any
}

type fakeBuddyRunner struct {
	fakeQueryer
	execs    []fakeBuddyExec
	execErr  error
	execErrs []error
}

type fakeBuddyTransactionRunner struct {
	*fakeBuddyRunner
	committed  bool
	rolledBack bool
}

func (r *fakeBuddyTransactionRunner) WithTransaction(ctx context.Context, run func(Queryer, Execer) error) error {
	if err := run(r, r); err != nil {
		r.rolledBack = true
		return err
	}
	r.committed = true
	return nil
}

func (f *fakeBuddyRunner) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
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
	return fakeSQLResult(1), nil
}
