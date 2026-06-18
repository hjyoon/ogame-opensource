package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type GalaxyRepository interface {
	GetGalaxy(context.Context, GalaxyQuery) (domaingame.Galaxy, error)
}

type GalaxyQuery struct {
	PlayerID    int
	PlanetID    int
	Coordinates domaingame.Coordinates
}

type GalaxyCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Coordinates     domaingame.Coordinates
}

type GalaxyResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Galaxy        domaingame.Galaxy
}

type GalaxyService struct {
	sessions   SessionLookup
	repository GalaxyRepository
}

func NewGalaxyService(sessions SessionLookup, repository GalaxyRepository) GalaxyService {
	return GalaxyService{sessions: sessions, repository: repository}
}

func (s GalaxyService) GetGalaxy(ctx context.Context, command GalaxyCommand) (GalaxyResult, error) {
	if s.sessions == nil || s.repository == nil {
		return GalaxyResult{}, errors.New("galaxy dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return GalaxyResult{}, err
	}
	if !session.Authenticated {
		return GalaxyResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	galaxy, err := s.repository.GetGalaxy(ctx, GalaxyQuery{
		PlayerID:    session.Session.PlayerID,
		PlanetID:    command.PlanetID,
		Coordinates: command.Coordinates,
	})
	if err != nil {
		return GalaxyResult{}, err
	}
	return GalaxyResult{
		Authenticated: true,
		Galaxy:        galaxy,
	}, nil
}
