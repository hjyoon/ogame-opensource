package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type ShipyardRepository interface {
	GetShipyard(context.Context, ShipyardQuery) (domaingame.Shipyard, error)
	MutateShipyard(context.Context, ShipyardMutationQuery) (ShipyardMutationOutcome, error)
}

type ShipyardQuery struct {
	PlayerID int
	PlanetID int
}

type ShipyardMutationQuery struct {
	PlayerID int
	PlanetID int
	Orders   map[int]int
}

type ShipyardCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
}

type ShipyardMutationCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Orders          map[int]int
}

type ShipyardMutationOutcome struct {
	ActionIssue *domaingame.BuildingsActionIssue
}

type ShipyardResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	ActionIssue   *domaingame.BuildingsActionIssue
	Shipyard      domaingame.Shipyard
}

type ShipyardService struct {
	sessions   SessionLookup
	repository ShipyardRepository
}

func NewShipyardService(sessions SessionLookup, repository ShipyardRepository) ShipyardService {
	return ShipyardService{sessions: sessions, repository: repository}
}

func (s ShipyardService) MutateShipyard(ctx context.Context, command ShipyardMutationCommand) (ShipyardResult, error) {
	if s.sessions == nil || s.repository == nil {
		return ShipyardResult{}, errors.New("shipyard dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return ShipyardResult{}, err
	}
	if !session.Authenticated {
		return ShipyardResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	outcome, err := s.repository.MutateShipyard(ctx, ShipyardMutationQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		Orders:   command.Orders,
	})
	if err != nil {
		return ShipyardResult{}, err
	}
	shipyard, err := s.repository.GetShipyard(ctx, ShipyardQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
	})
	if err != nil {
		return ShipyardResult{}, err
	}
	return ShipyardResult{
		Authenticated: true,
		ActionIssue:   outcome.ActionIssue,
		Shipyard:      shipyard,
	}, nil
}

func (s ShipyardService) GetShipyard(ctx context.Context, command ShipyardCommand) (ShipyardResult, error) {
	if s.sessions == nil || s.repository == nil {
		return ShipyardResult{}, errors.New("shipyard dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return ShipyardResult{}, err
	}
	if !session.Authenticated {
		return ShipyardResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	shipyard, err := s.repository.GetShipyard(ctx, ShipyardQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
	})
	if err != nil {
		return ShipyardResult{}, err
	}
	return ShipyardResult{
		Authenticated: true,
		Shipyard:      shipyard,
	}, nil
}
