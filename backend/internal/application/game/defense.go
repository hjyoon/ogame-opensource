package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type DefenseRepository interface {
	GetDefense(context.Context, DefenseQuery) (domaingame.Defense, error)
	MutateDefense(context.Context, DefenseMutationQuery) (DefenseMutationOutcome, error)
}

type DefenseQuery struct {
	PlayerID int
	PlanetID int
}

type DefenseMutationQuery struct {
	PlayerID int
	PlanetID int
	Orders   map[int]int
}

type DefenseCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
}

type DefenseMutationCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Orders          map[int]int
}

type DefenseMutationOutcome struct {
	ActionIssue *domaingame.BuildingsActionIssue
}

type DefenseResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	ActionIssue   *domaingame.BuildingsActionIssue
	Defense       domaingame.Defense
}

type DefenseService struct {
	sessions   SessionLookup
	repository DefenseRepository
}

func NewDefenseService(sessions SessionLookup, repository DefenseRepository) DefenseService {
	return DefenseService{sessions: sessions, repository: repository}
}

func (s DefenseService) MutateDefense(ctx context.Context, command DefenseMutationCommand) (DefenseResult, error) {
	if s.sessions == nil || s.repository == nil {
		return DefenseResult{}, errors.New("defense dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return DefenseResult{}, err
	}
	if !session.Authenticated {
		return DefenseResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	outcome, err := s.repository.MutateDefense(ctx, DefenseMutationQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		Orders:   command.Orders,
	})
	if err != nil {
		return DefenseResult{}, err
	}
	defense, err := s.repository.GetDefense(ctx, DefenseQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
	})
	if err != nil {
		return DefenseResult{}, err
	}
	return DefenseResult{
		Authenticated: true,
		ActionIssue:   outcome.ActionIssue,
		Defense:       defense,
	}, nil
}

func (s DefenseService) GetDefense(ctx context.Context, command DefenseCommand) (DefenseResult, error) {
	if s.sessions == nil || s.repository == nil {
		return DefenseResult{}, errors.New("defense dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return DefenseResult{}, err
	}
	if !session.Authenticated {
		return DefenseResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	defense, err := s.repository.GetDefense(ctx, DefenseQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
	})
	if err != nil {
		return DefenseResult{}, err
	}
	return DefenseResult{
		Authenticated: true,
		Defense:       defense,
	}, nil
}
