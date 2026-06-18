package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type NotesRepository struct {
	queryer Queryer
	prefix  string
}

func NewNotesRepository(db *sql.DB, prefix string) NotesRepository {
	return NotesRepository{queryer: SQLQueryer{DB: db}, prefix: prefix}
}

func NewNotesRepositoryWithQueryer(queryer Queryer, prefix string) NotesRepository {
	return NotesRepository{queryer: queryer, prefix: prefix}
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
