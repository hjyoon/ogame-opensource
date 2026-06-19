package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type BuddyRepository interface {
	GetBuddy(context.Context, BuddyQuery) (domaingame.Buddy, error)
	MutateBuddy(context.Context, BuddyMutationQuery) (BuddyMutationOutcome, error)
}

type BuddyQuery struct {
	PlayerID int
	PlanetID int
	Action   int
	BuddyID  int
}

type BuddyCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Action          int
	BuddyID         int
}

type BuddyResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	ActionIssue   *domaingame.BuddyActionIssue
	Buddy         domaingame.Buddy
}

type BuddyMutationQuery struct {
	PlayerID int
	PlanetID int
	Action   int
	BuddyID  int
	Text     string
}

type BuddyMutationCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Action          int
	BuddyID         int
	Text            string
}

type BuddyMutationOutcome struct {
	NextAction  int
	ActionIssue *domaingame.BuddyActionIssue
}

type BuddyService struct {
	sessions   SessionLookup
	repository BuddyRepository
}

func NewBuddyService(sessions SessionLookup, repository BuddyRepository) BuddyService {
	return BuddyService{sessions: sessions, repository: repository}
}

func (s BuddyService) GetBuddy(ctx context.Context, command BuddyCommand) (BuddyResult, error) {
	if s.sessions == nil || s.repository == nil {
		return BuddyResult{}, errors.New("buddy dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return BuddyResult{}, err
	}
	if !session.Authenticated {
		return BuddyResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	buddy, err := s.repository.GetBuddy(ctx, BuddyQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		Action:   command.Action,
		BuddyID:  command.BuddyID,
	})
	if err != nil {
		return BuddyResult{}, err
	}
	return BuddyResult{
		Authenticated: true,
		Buddy:         buddy,
	}, nil
}

func (s BuddyService) MutateBuddy(ctx context.Context, command BuddyMutationCommand) (BuddyResult, error) {
	if s.sessions == nil || s.repository == nil {
		return BuddyResult{}, errors.New("buddy dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return BuddyResult{}, err
	}
	if !session.Authenticated {
		return BuddyResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	outcome, err := s.repository.MutateBuddy(ctx, BuddyMutationQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		Action:   command.Action,
		BuddyID:  command.BuddyID,
		Text:     command.Text,
	})
	if err != nil {
		return BuddyResult{}, err
	}
	buddy, err := s.repository.GetBuddy(ctx, BuddyQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		Action:   outcome.NextAction,
	})
	if err != nil {
		return BuddyResult{}, err
	}
	return BuddyResult{
		Authenticated: true,
		ActionIssue:   outcome.ActionIssue,
		Buddy:         buddy,
	}, nil
}
