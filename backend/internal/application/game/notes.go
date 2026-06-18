package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type NotesRepository interface {
	GetNotes(context.Context, NotesQuery) (domaingame.Notes, error)
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

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
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
