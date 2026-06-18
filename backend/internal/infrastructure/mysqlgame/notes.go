package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type NotesRepository struct {
	queryer Queryer
	execer  Execer
	prefix  string
	now     func() time.Time
}

func NewNotesRepository(db *sql.DB, prefix string) NotesRepository {
	runner := SQLQueryer{DB: db}
	return NotesRepository{queryer: runner, execer: runner, prefix: prefix, now: time.Now}
}

func NewNotesRepositoryWithQueryer(queryer Queryer, prefix string) NotesRepository {
	var execer Execer
	if runner, ok := queryer.(Execer); ok {
		execer = runner
	}
	return NewNotesRepositoryWithRunner(queryer, execer, prefix, time.Now)
}

func NewNotesRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, now func() time.Time) NotesRepository {
	if now == nil {
		now = time.Now
	}
	return NotesRepository{queryer: queryer, execer: execer, prefix: prefix, now: now}
}

func (r NotesRepository) GetNotes(ctx context.Context, query appgame.NotesQuery) (domaingame.Notes, error) {
	notesTable, err := tableName(r.prefix, "notes")
	if err != nil {
		return domaingame.Notes{}, err
	}
	overview, err := NewOverviewRepositoryWithQueryer(r.queryer, r.prefix).GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.Notes{}, err
	}

	action := domaingame.NormalizeNotesAction(query.Action)
	notes := domaingame.Notes{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		Action:         action,
	}

	if action == domaingame.NotesActionEdit {
		note, err := r.loadNote(ctx, notesTable, query.PlayerID, query.NoteID)
		if err != nil {
			return domaingame.Notes{}, err
		}
		notes.EditNote = &note
		return notes, nil
	}
	if action == domaingame.NotesActionCreate {
		return notes, nil
	}

	rows, err := r.loadNotes(ctx, notesTable, query.PlayerID)
	if err != nil {
		return domaingame.Notes{}, err
	}
	notes.Rows = rows
	return notes, nil
}

func (r NotesRepository) CreateNote(ctx context.Context, query appgame.NotesMutationQuery) (domaingame.Notes, error) {
	if r.execer == nil {
		return domaingame.Notes{}, errors.New("notes updater unavailable")
	}
	notesTable, err := tableName(r.prefix, "notes")
	if err != nil {
		return domaingame.Notes{}, err
	}
	if _, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, subj, text, textsize, prio, date) VALUES (?, ?, ?, ?, ?, ?)", notesTable),
		query.PlayerID,
		query.Draft.Subject,
		query.Draft.Text,
		query.Draft.TextSize,
		query.Draft.Priority,
		r.now().Unix(),
	); err != nil {
		return domaingame.Notes{}, err
	}
	return r.GetNotes(ctx, appgame.NotesQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
}

func (r NotesRepository) UpdateNote(ctx context.Context, query appgame.NotesMutationQuery) (domaingame.Notes, error) {
	if r.execer == nil {
		return domaingame.Notes{}, errors.New("notes updater unavailable")
	}
	notesTable, err := tableName(r.prefix, "notes")
	if err != nil {
		return domaingame.Notes{}, err
	}
	if query.NoteID > 0 {
		if _, err := r.execer.ExecContext(
			ctx,
			fmt.Sprintf("UPDATE %s SET subj = ?, text = ?, textsize = ?, prio = ?, date = ? WHERE owner_id = ? AND note_id = ?", notesTable),
			query.Draft.Subject,
			query.Draft.Text,
			query.Draft.TextSize,
			query.Draft.Priority,
			r.now().Unix(),
			query.PlayerID,
			query.NoteID,
		); err != nil {
			return domaingame.Notes{}, err
		}
	}
	return r.GetNotes(ctx, appgame.NotesQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
}

func (r NotesRepository) DeleteNotes(ctx context.Context, query appgame.NotesDeleteQuery) (domaingame.Notes, error) {
	if r.execer == nil {
		return domaingame.Notes{}, errors.New("notes updater unavailable")
	}
	notesTable, err := tableName(r.prefix, "notes")
	if err != nil {
		return domaingame.Notes{}, err
	}
	for _, noteID := range query.NoteIDs {
		if _, err := r.execer.ExecContext(
			ctx,
			fmt.Sprintf("DELETE FROM %s WHERE owner_id = ? AND note_id = ?", notesTable),
			query.PlayerID,
			noteID,
		); err != nil {
			return domaingame.Notes{}, err
		}
	}
	return r.GetNotes(ctx, appgame.NotesQuery{PlayerID: query.PlayerID, PlanetID: query.PlanetID})
}

func (r NotesRepository) loadNotes(ctx context.Context, notesTable string, playerID int) ([]domaingame.Note, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT note_id, subj, text, textsize, prio, date FROM %s WHERE owner_id = ? ORDER BY date DESC LIMIT ?", notesTable),
		playerID,
		domaingame.NotesLimit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notes := []domaingame.Note{}
	for rows.Next() {
		note, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return notes, nil
}

func (r NotesRepository) loadNote(ctx context.Context, notesTable string, playerID int, noteID int) (domaingame.Note, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT note_id, subj, text, textsize, prio, date FROM %s WHERE owner_id = ? AND note_id = ? LIMIT 1", notesTable),
		playerID,
		noteID,
	)
	if err != nil {
		return domaingame.Note{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domaingame.Note{}, err
		}
		return domaingame.Note{}, errors.New("note not found")
	}
	note, err := scanNote(rows)
	if err != nil {
		return domaingame.Note{}, err
	}
	if err := rows.Err(); err != nil {
		return domaingame.Note{}, err
	}
	return note, nil
}

func scanNote(rows Rows) (domaingame.Note, error) {
	var note domaingame.Note
	if err := rows.Scan(&note.ID, &note.Subject, &note.Text, &note.TextSize, &note.Priority, &note.Date); err != nil {
		return domaingame.Note{}, err
	}
	return note, nil
}
