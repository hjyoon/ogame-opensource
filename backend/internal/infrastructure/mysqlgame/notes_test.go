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

	runner := &fakeNotesRunner{}
	repository = NewNotesRepositoryWithQueryer(runner, "ogame_")
	if repository.execer == nil {
		t.Fatal("queryer that also implements Execer should be kept for mutations")
	}

	repository = NewNotesRepositoryWithRunner(runner, runner, "ogame_", nil)
	if repository.now == nil {
		t.Fatal("nil clock should default to time.Now")
	}
}

func TestNotesRepositoryMutationsWriteLegacyRowsAndReturnList(t *testing.T) {
	now := time.Unix(1700000000, 0)
	runner := &fakeNotesRunner{fakeQueryer: fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{11, "Subject", "Body", 4, 1, now.Unix()})},
	)}}
	repository := NewNotesRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	notes, err := repository.CreateNote(context.Background(), appgame.NotesMutationQuery{
		PlayerID: 42,
		PlanetID: 99,
		Draft:    domaingame.NoteDraft{Subject: "Subject", Text: "Body", TextSize: 4, Priority: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(notes.Rows) != 1 || !strings.Contains(runner.execs[0].sql, "INSERT INTO `ogame_notes`") ||
		runner.execs[0].args[0] != 42 || runner.execs[0].args[5] != now.Unix() {
		t.Fatalf("unexpected create mutation notes=%+v execs=%+v", notes, runner.execs)
	}

	runner = &fakeNotesRunner{fakeQueryer: fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues()})}}
	repository = NewNotesRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if _, err := repository.UpdateNote(context.Background(), appgame.NotesMutationQuery{
		PlayerID: 42,
		NoteID:   11,
		Draft:    domaingame.NoteDraft{Subject: "Updated", Text: "Text", TextSize: 4, Priority: 0},
	}); err != nil {
		t.Fatal(err)
	}
	if len(runner.execs) != 1 || !strings.Contains(runner.execs[0].sql, "UPDATE `ogame_notes` SET") ||
		runner.execs[0].args[5] != 42 || runner.execs[0].args[6] != 11 {
		t.Fatalf("unexpected update execs: %+v", runner.execs)
	}

	runner = &fakeNotesRunner{fakeQueryer: fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues()})}}
	repository = NewNotesRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if _, err := repository.DeleteNotes(context.Background(), appgame.NotesDeleteQuery{PlayerID: 42, NoteIDs: []int{11, 12}}); err != nil {
		t.Fatal(err)
	}
	if len(runner.execs) != 2 || !strings.Contains(runner.execs[0].sql, "DELETE FROM `ogame_notes` WHERE owner_id = ? AND note_id = ?") ||
		runner.execs[1].args[1] != 12 {
		t.Fatalf("unexpected delete execs: %+v", runner.execs)
	}

	runner = &fakeNotesRunner{fakeQueryer: fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues()})}}
	repository = NewNotesRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if _, err := repository.UpdateNote(context.Background(), appgame.NotesMutationQuery{PlayerID: 42}); err != nil {
		t.Fatal(err)
	}
	if len(runner.execs) != 0 {
		t.Fatalf("update without note id should not write, got %+v", runner.execs)
	}

	runner = &fakeNotesRunner{fakeQueryer: fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues()})}}
	repository = NewNotesRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if _, err := repository.DeleteNotes(context.Background(), appgame.NotesDeleteQuery{PlayerID: 42}); err != nil {
		t.Fatal(err)
	}
	if len(runner.execs) != 0 {
		t.Fatalf("delete without ids should not write, got %+v", runner.execs)
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

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", "Subject", "Body", 4, 0, int64(1700000000)})}}}
	repository = NewNotesRepositoryWithQueryer(queryer, "ogame_")
	if _, err := repository.loadNote(context.Background(), "ogame_notes", 42, 11); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected edit scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("edit rows failed"), []any{11, "Subject", "Body", 4, 0, int64(1700000000)})}}}
	repository = NewNotesRepositoryWithQueryer(queryer, "ogame_")
	if _, err := repository.loadNote(context.Background(), "ogame_notes", 42, 11); err == nil || !strings.Contains(err.Error(), "edit rows failed") {
		t.Fatalf("expected edit rows error, got %v", err)
	}
}

func TestNotesRepositoryMutationErrors(t *testing.T) {
	repository := NewNotesRepositoryWithRunner(&fakeQueryer{}, nil, "ogame_", time.Now)
	if _, err := repository.CreateNote(context.Background(), appgame.NotesMutationQuery{}); err == nil || !strings.Contains(err.Error(), "updater unavailable") {
		t.Fatalf("expected missing updater error, got %v", err)
	}
	if _, err := repository.UpdateNote(context.Background(), appgame.NotesMutationQuery{}); err == nil || !strings.Contains(err.Error(), "updater unavailable") {
		t.Fatalf("expected update missing updater error, got %v", err)
	}
	if _, err := repository.DeleteNotes(context.Background(), appgame.NotesDeleteQuery{}); err == nil || !strings.Contains(err.Error(), "updater unavailable") {
		t.Fatalf("expected delete missing updater error, got %v", err)
	}

	runner := &fakeNotesRunner{}
	for _, tt := range []struct {
		name string
		call func(NotesRepository) error
	}{
		{"create", func(repository NotesRepository) error {
			_, err := repository.CreateNote(context.Background(), appgame.NotesMutationQuery{})
			return err
		}},
		{"update", func(repository NotesRepository) error {
			_, err := repository.UpdateNote(context.Background(), appgame.NotesMutationQuery{})
			return err
		}},
		{"delete", func(repository NotesRepository) error {
			_, err := repository.DeleteNotes(context.Background(), appgame.NotesDeleteQuery{})
			return err
		}},
	} {
		t.Run("unsafe prefix "+tt.name, func(t *testing.T) {
			err := tt.call(NewNotesRepositoryWithRunner(runner, runner, "bad-prefix_", time.Now))
			if err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
				t.Fatalf("expected unsafe prefix error, got %v", err)
			}
		})
	}

	runner = &fakeNotesRunner{execErr: errors.New("exec failed")}
	repository = NewNotesRepositoryWithRunner(runner, runner, "ogame_", time.Now)
	if _, err := repository.CreateNote(context.Background(), appgame.NotesMutationQuery{}); err == nil || !strings.Contains(err.Error(), "exec failed") {
		t.Fatalf("expected create exec error, got %v", err)
	}
	if _, err := repository.UpdateNote(context.Background(), appgame.NotesMutationQuery{NoteID: 1}); err == nil || !strings.Contains(err.Error(), "exec failed") {
		t.Fatalf("expected update exec error, got %v", err)
	}
	if _, err := repository.DeleteNotes(context.Background(), appgame.NotesDeleteQuery{NoteIDs: []int{1}}); err == nil || !strings.Contains(err.Error(), "exec failed") {
		t.Fatalf("expected delete exec error, got %v", err)
	}
}

type fakeNotesExec struct {
	sql  string
	args []any
}

type fakeNotesRunner struct {
	fakeQueryer
	execs   []fakeNotesExec
	execErr error
}

func (f *fakeNotesRunner) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.execs = append(f.execs, fakeNotesExec{sql: query, args: args})
	if f.execErr != nil {
		return nil, f.execErr
	}
	return fakeSQLResult(1), nil
}
