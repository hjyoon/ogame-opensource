package game

import (
	"context"
	"errors"
	"html"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type NotesRepository interface {
	GetNotes(context.Context, NotesQuery) (domaingame.Notes, error)
	CreateNote(context.Context, NotesMutationQuery) (domaingame.Notes, error)
	UpdateNote(context.Context, NotesMutationQuery) (domaingame.Notes, error)
	DeleteNotes(context.Context, NotesDeleteQuery) (domaingame.Notes, error)
}

type NotesQuery struct {
	PlayerID int
	PlanetID int
	Action   int
	NoteID   int
}

type NotesCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Action          int
	NoteID          int
}

type NotesMutationQuery struct {
	PlayerID int
	PlanetID int
	NoteID   int
	Draft    domaingame.NoteDraft
}

type NotesDeleteQuery struct {
	PlayerID int
	PlanetID int
	NoteIDs  []int
}

type NotesMutationCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	NoteID          int
	Subject         string
	Text            string
	Priority        int
	NoteIDs         []int
}

type NotesResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Notes         domaingame.Notes
}

type NotesService struct {
	sessions   SessionLookup
	repository NotesRepository
}

func NewNotesService(sessions SessionLookup, repository NotesRepository) NotesService {
	return NotesService{sessions: sessions, repository: repository}
}

func (s NotesService) GetNotes(ctx context.Context, command NotesCommand) (NotesResult, error) {
	if s.sessions == nil || s.repository == nil {
		return NotesResult{}, errors.New("notes dependencies unavailable")
	}

	session, err := s.authenticatedSession(ctx, command.PublicSession, command.PrivateSessions, command.RemoteAddr)
	if err != nil {
		return NotesResult{}, err
	}
	if !session.Authenticated {
		return NotesResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	notes, err := s.repository.GetNotes(ctx, NotesQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		Action:   command.Action,
		NoteID:   command.NoteID,
	})
	if err != nil {
		return NotesResult{}, err
	}
	return NotesResult{
		Authenticated: true,
		Notes:         notes,
	}, nil
}

func (s NotesService) CreateNote(ctx context.Context, command NotesMutationCommand) (NotesResult, error) {
	if s.sessions == nil || s.repository == nil {
		return NotesResult{}, errors.New("notes dependencies unavailable")
	}
	session, err := s.authenticatedSession(ctx, command.PublicSession, command.PrivateSessions, command.RemoteAddr)
	if err != nil {
		return NotesResult{}, err
	}
	if !session.Authenticated {
		return NotesResult{Authenticated: false, Issues: session.Issues}, nil
	}
	notes, err := s.repository.CreateNote(ctx, NotesMutationQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		Draft:    normalizeNoteMutationDraft(command),
	})
	if err != nil {
		return NotesResult{}, err
	}
	return NotesResult{Authenticated: true, Notes: notes}, nil
}

func (s NotesService) UpdateNote(ctx context.Context, command NotesMutationCommand) (NotesResult, error) {
	if s.sessions == nil || s.repository == nil {
		return NotesResult{}, errors.New("notes dependencies unavailable")
	}
	session, err := s.authenticatedSession(ctx, command.PublicSession, command.PrivateSessions, command.RemoteAddr)
	if err != nil {
		return NotesResult{}, err
	}
	if !session.Authenticated {
		return NotesResult{Authenticated: false, Issues: session.Issues}, nil
	}
	notes, err := s.repository.UpdateNote(ctx, NotesMutationQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		NoteID:   command.NoteID,
		Draft:    normalizeNoteMutationDraft(command),
	})
	if err != nil {
		return NotesResult{}, err
	}
	return NotesResult{Authenticated: true, Notes: notes}, nil
}

func (s NotesService) DeleteNotes(ctx context.Context, command NotesMutationCommand) (NotesResult, error) {
	if s.sessions == nil || s.repository == nil {
		return NotesResult{}, errors.New("notes dependencies unavailable")
	}
	session, err := s.authenticatedSession(ctx, command.PublicSession, command.PrivateSessions, command.RemoteAddr)
	if err != nil {
		return NotesResult{}, err
	}
	if !session.Authenticated {
		return NotesResult{Authenticated: false, Issues: session.Issues}, nil
	}
	notes, err := s.repository.DeleteNotes(ctx, NotesDeleteQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		NoteIDs:  domaingame.NormalizeNoteIDs(command.NoteIDs),
	})
	if err != nil {
		return NotesResult{}, err
	}
	return NotesResult{Authenticated: true, Notes: notes}, nil
}

func (s NotesService) authenticatedSession(ctx context.Context, publicSession string, privateSessions map[string]string, remoteAddr string) (domainpublicsite.SessionAuthentication, error) {
	return s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   publicSession,
		PrivateSessions: privateSessions,
		RemoteAddr:      remoteAddr,
	})
}

func normalizeNoteMutationDraft(command NotesMutationCommand) domaingame.NoteDraft {
	return domaingame.NormalizeNoteDraft(html.EscapeString(command.Subject), command.Text, command.Priority)
}
