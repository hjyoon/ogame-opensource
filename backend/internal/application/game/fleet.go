package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type FleetRepository interface {
	GetFleet(context.Context, FleetQuery) (domaingame.Fleet, error)
}

type FleetQuery struct {
	PlayerID int
	PlanetID int
}

type FleetCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
}

type FleetResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Fleet         domaingame.Fleet
}

type FleetService struct {
	sessions   SessionLookup
	repository FleetRepository
}

func NewFleetService(sessions SessionLookup, repository FleetRepository) FleetService {
	return FleetService{sessions: sessions, repository: repository}
}

func (s FleetService) GetFleet(ctx context.Context, command FleetCommand) (FleetResult, error) {
	if s.sessions == nil || s.repository == nil {
		return FleetResult{}, errors.New("fleet dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return FleetResult{}, err
	}
	if !session.Authenticated {
		return FleetResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	fleet, err := s.repository.GetFleet(ctx, FleetQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
	})
	if err != nil {
		return FleetResult{}, err
	}
	return FleetResult{
		Authenticated: true,
		Fleet:         fleet,
	}, nil
}
