package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestNotesRepositoryReadsLegacyList(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{11, "Important", "Remember this", 13, 2, int64(1700000000)},
			[]any{10, "Normal", "Older note", 10, 1, int64(1699999900)},
		)},
	)}
	repository := NewNotesRepositoryWithQueryer(queryer, "ogame_")

	notes, err := repository.GetNotes(context.Background(), appgame.NotesQuery{PlayerID: 42, PlanetID: 99})
	if err != nil {
		t.Fatal(err)
	}
	if notes.Commander != "legor" || notes.Action != domaingame.NotesActionList || len(notes.Rows) != 2 {
		t.Fatalf("unexpected notes summary: %+v", notes)
	}
	if notes.Rows[0].ID != 11 || notes.Rows[0].Subject != "Important" || notes.Rows[0].PriorityColor() != "red" {
		t.Fatalf("unexpected first note: %+v", notes.Rows[0])
	}
	if !strings.Contains(queryer.calls[4].sql, "WHERE owner_id = ? ORDER BY date DESC LIMIT ?") ||
		queryer.calls[4].args[0] != 42 || queryer.calls[4].args[1] != domaingame.NotesLimit {
		t.Fatalf("expected legacy notes list query, got %+v", queryer.calls[4])
	}
}

func TestNotesRepositoryRendersCreateWithoutLoadingRows(t *testing.T) {
	queryer := &fakeQueryer{results: shipyardOverviewResults()}
	repository := NewNotesRepositoryWithQueryer(queryer, "ogame_")

	notes, err := repository.GetNotes(context.Background(), appgame.NotesQuery{PlayerID: 42, Action: 1})
	if err != nil {
		t.Fatal(err)
	}
	if notes.Action != domaingame.NotesActionCreate || len(notes.Rows) != 0 || notes.EditNote != nil || len(queryer.calls) != 4 {
		t.Fatalf("create should only load overview, got notes=%+v calls=%d", notes, len(queryer.calls))
	}
}

func TestNotesRepositoryReadsEditNote(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{11, "Subject", "Body", 4, 0, int64(1700000000)})},
	)}
	repository := NewNotesRepositoryWithQueryer(queryer, "ogame_")

	notes, err := repository.GetNotes(context.Background(), appgame.NotesQuery{PlayerID: 42, Action: 2, NoteID: 11})
	if err != nil {
		t.Fatal(err)
	}
	if notes.Action != domaingame.NotesActionEdit || notes.EditNote == nil || notes.EditNote.ID != 11 || notes.EditNote.PriorityColor() != "lime" {
		t.Fatalf("unexpected edit note: %+v", notes)
	}
	if !strings.Contains(queryer.calls[4].sql, "WHERE owner_id = ? AND note_id = ? LIMIT 1") ||
		queryer.calls[4].args[0] != 42 || queryer.calls[4].args[1] != 11 {
		t.Fatalf("expected legacy edit query, got %+v", queryer.calls[4])
	}
}

func TestNewNotesRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewNotesRepository(nil, "ogame_")
	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
}

func TestNotesRepositoryReturnsErrors(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		queryer *fakeQueryer
		query   appgame.NotesQuery
		want    string
	}{
		{name: "unsafe prefix", prefix: "bad-prefix_", queryer: &fakeQueryer{}, want: "invalid database table prefix"},
		{name: "overview", prefix: "ogame_", queryer: &fakeQueryer{results: []fakeQueryResult{{err: errors.New("overview failed")}}}, want: "overview failed"},
		{name: "list query", prefix: "ogame_", queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{err: errors.New("notes failed")})}, want: "notes failed"},
		{name: "missing edit", prefix: "ogame_", queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues()})}, query: appgame.NotesQuery{Action: 2, NoteID: 1}, want: "note not found"},
		{name: "edit query", prefix: "ogame_", queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{err: errors.New("edit failed")})}, query: appgame.NotesQuery{Action: 2, NoteID: 1}, want: "edit failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewNotesRepositoryWithQueryer(tt.queryer, tt.prefix)
			_, err := repository.GetNotes(context.Background(), tt.query)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestNotesRepositoryScanEdges(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", "Subject", "Body", 4, 0, int64(1700000000)})}}}
	repository := NewNotesRepositoryWithQueryer(queryer, "ogame_")
	if _, err := repository.loadNotes(context.Background(), "ogame_notes", 42); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected note scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("notes rows failed"), []any{11, "Subject", "Body", 4, 0, int64(1700000000)})}}}
	repository = NewNotesRepositoryWithQueryer(queryer, "ogame_")
	if _, err := repository.loadNotes(context.Background(), "ogame_notes", 42); err == nil || !strings.Contains(err.Error(), "notes rows failed") {
		t.Fatalf("expected note rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("edit empty rows failed"))}}}
	repository = NewNotesRepositoryWithQueryer(queryer, "ogame_")
	if _, err := repository.loadNote(context.Background(), "ogame_notes", 42, 11); err == nil || !strings.Contains(err.Error(), "edit empty rows failed") {
		t.Fatalf("expected edit empty rows error, got %v", err)
	}
}
