package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
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

func NewNotesReadRepository(db *sql.DB, prefix string) NotesRepository {
	return NewNotesRepositoryWithRunner(SQLQueryer{DB: db}, nil, prefix, time.Now)
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

func (r NotesRepository) GetMCPNotes(ctx context.Context, playerID int, command domainmcp.NotesStatusCommand) (domainmcp.NotesStatus, error) {
	if r.queryer == nil {
		return domainmcp.NotesStatus{}, errors.New("notes reader unavailable")
	}
	notes, err := r.GetNotes(ctx, appgame.NotesQuery{
		PlayerID: playerID,
		PlanetID: command.PlanetID,
		Action:   command.Action,
		NoteID:   command.NoteID,
	})
	if err != nil {
		return domainmcp.NotesStatus{}, err
	}
	return mcpNotesStatus(playerID, notes), nil
}

func (r NotesRepository) PreviewMCPCreateNote(ctx context.Context, playerID int, command domainmcp.NoteMutationCommand) (domainmcp.NoteMutationResult, error) {
	draft := domaingame.NormalizeNoteDraft(command.Subject, command.Text, command.Priority)
	notes, err := r.GetNotes(ctx, appgame.NotesQuery{PlayerID: playerID, PlanetID: command.PlanetID, Action: 1})
	if err != nil {
		return domainmcp.NoteMutationResult{}, err
	}
	return mcpNoteMutationResult(playerID, command.PlanetID, 0, nil, draft, notes), nil
}

func (r NotesRepository) CreateMCPNote(ctx context.Context, playerID int, command domainmcp.NoteMutationCommand) (domainmcp.NoteMutationResult, error) {
	draft := domaingame.NormalizeNoteDraft(command.Subject, command.Text, command.Priority)
	notes, err := r.CreateNote(ctx, appgame.NotesMutationQuery{
		PlayerID: playerID,
		PlanetID: command.PlanetID,
		Draft:    draft,
	})
	if err != nil {
		return domainmcp.NoteMutationResult{}, err
	}
	noteID := 0
	if len(notes.Rows) > 0 {
		noteID = notes.Rows[0].ID
	}
	return mcpNoteMutationResult(playerID, command.PlanetID, noteID, nil, draft, notes), nil
}

func (r NotesRepository) PreviewMCPUpdateNote(ctx context.Context, playerID int, command domainmcp.NoteMutationCommand) (domainmcp.NoteMutationResult, error) {
	draft := domaingame.NormalizeNoteDraft(command.Subject, command.Text, command.Priority)
	notes, err := r.GetNotes(ctx, appgame.NotesQuery{PlayerID: playerID, PlanetID: command.PlanetID, Action: 2, NoteID: command.NoteID})
	if err != nil {
		return domainmcp.NoteMutationResult{}, err
	}
	return mcpNoteMutationResult(playerID, command.PlanetID, command.NoteID, nil, draft, notes), nil
}

func (r NotesRepository) UpdateMCPNote(ctx context.Context, playerID int, command domainmcp.NoteMutationCommand) (domainmcp.NoteMutationResult, error) {
	draft := domaingame.NormalizeNoteDraft(command.Subject, command.Text, command.Priority)
	notes, err := r.UpdateNote(ctx, appgame.NotesMutationQuery{
		PlayerID: playerID,
		PlanetID: command.PlanetID,
		NoteID:   command.NoteID,
		Draft:    draft,
	})
	if err != nil {
		return domainmcp.NoteMutationResult{}, err
	}
	return mcpNoteMutationResult(playerID, command.PlanetID, command.NoteID, nil, draft, notes), nil
}

func (r NotesRepository) PreviewMCPDeleteNotes(ctx context.Context, playerID int, command domainmcp.NoteMutationCommand) (domainmcp.NoteMutationResult, error) {
	noteIDs := domaingame.NormalizeNoteIDs(command.NoteIDs)
	notes, err := r.GetNotes(ctx, appgame.NotesQuery{PlayerID: playerID, PlanetID: command.PlanetID})
	if err != nil {
		return domainmcp.NoteMutationResult{}, err
	}
	result := mcpNoteMutationResult(playerID, command.PlanetID, 0, noteIDs, domaingame.NoteDraft{}, notes)
	result.DeleteCount = countMatchingNoteIDs(notes.Rows, noteIDs)
	return result, nil
}

func (r NotesRepository) DeleteMCPNotes(ctx context.Context, playerID int, command domainmcp.NoteMutationCommand) (domainmcp.NoteMutationResult, error) {
	preview, err := r.PreviewMCPDeleteNotes(ctx, playerID, command)
	if err != nil {
		return domainmcp.NoteMutationResult{}, err
	}
	notes, err := r.DeleteNotes(ctx, appgame.NotesDeleteQuery{
		PlayerID: playerID,
		PlanetID: command.PlanetID,
		NoteIDs:  preview.NoteIDs,
	})
	if err != nil {
		return domainmcp.NoteMutationResult{}, err
	}
	result := mcpNoteMutationResult(playerID, command.PlanetID, 0, preview.NoteIDs, domaingame.NoteDraft{}, notes)
	result.DeleteCount = preview.DeleteCount
	return result, nil
}

func mcpNotesStatus(playerID int, notes domaingame.Notes) domainmcp.NotesStatus {
	return domainmcp.NotesStatus{
		PlayerID: playerID,
		Planet: domainmcp.Planet{
			ID:       notes.CurrentPlanet.ID,
			Name:     notes.CurrentPlanet.Name,
			Type:     notes.CurrentPlanet.Type,
			TypeName: mcpPlanetTypeName(notes.CurrentPlanet.Type),
			Coordinates: domainmcp.Coordinates{
				Galaxy:   notes.CurrentPlanet.Coordinates.Galaxy,
				System:   notes.CurrentPlanet.Coordinates.System,
				Position: notes.CurrentPlanet.Coordinates.Position,
			},
			Current: true,
		},
		Commander: notes.Commander,
		Action:    notes.Action,
		Rows:      mcpNotes(notes.Rows),
		EditNote:  mcpNote(notes.EditNote),
	}
}

func mcpNoteMutationResult(playerID int, planetID int, noteID int, noteIDs []int, draft domaingame.NoteDraft, notes domaingame.Notes) domainmcp.NoteMutationResult {
	return domainmcp.NoteMutationResult{
		PlayerID: playerID,
		PlanetID: planetID,
		NoteID:   noteID,
		NoteIDs:  append([]int(nil), noteIDs...),
		Subject:  draft.Subject,
		TextSize: draft.TextSize,
		Priority: draft.Priority,
		Notes:    mcpNotesStatus(playerID, notes),
		Executed: false,
		DryRun:   true,
	}
}

func countMatchingNoteIDs(notes []domaingame.Note, noteIDs []int) int {
	wanted := map[int]struct{}{}
	for _, id := range noteIDs {
		wanted[id] = struct{}{}
	}
	count := 0
	for _, note := range notes {
		if _, ok := wanted[note.ID]; ok {
			count++
		}
	}
	return count
}

func mcpNotes(notes []domaingame.Note) []domainmcp.Note {
	result := make([]domainmcp.Note, 0, len(notes))
	for _, note := range notes {
		result = append(result, mcpNoteValue(note))
	}
	return result
}

func mcpNote(note *domaingame.Note) *domainmcp.Note {
	if note == nil {
		return nil
	}
	result := mcpNoteValue(*note)
	return &result
}

func mcpNoteValue(note domaingame.Note) domainmcp.Note {
	return domainmcp.Note{
		ID:            note.ID,
		Subject:       note.Subject,
		Text:          note.Text,
		TextSize:      note.TextSize,
		Priority:      note.Priority,
		PriorityColor: note.PriorityColor(),
		Date:          note.Date,
	}
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
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT n.note_id, n.subj, n.text, n.textsize, n.prio, n.date, COALESCE(u.admin, 0) FROM %s n LEFT JOIN %s u ON u.player_id = n.owner_id WHERE n.owner_id = ? ORDER BY n.date DESC LIMIT ?", notesTable, usersTable),
		playerID,
		domaingame.AdminNotesLimit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notes := []domaingame.Note{}
	adminLevel := domaingame.AdminLevelPlayer
	for rows.Next() {
		note, level, err := scanListedNote(rows)
		if err != nil {
			return nil, err
		}
		adminLevel = level
		notes = append(notes, note)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if adminLevel <= domaingame.AdminLevelPlayer && len(notes) > domaingame.NotesLimit {
		notes = notes[:domaingame.NotesLimit]
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

func scanListedNote(rows Rows) (domaingame.Note, int, error) {
	var note domaingame.Note
	adminLevel := domaingame.AdminLevelPlayer
	if err := rows.Scan(&note.ID, &note.Subject, &note.Text, &note.TextSize, &note.Priority, &note.Date, &adminLevel); err != nil {
		return domaingame.Note{}, domaingame.AdminLevelPlayer, err
	}
	return note, adminLevel, nil
}
