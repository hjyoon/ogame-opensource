package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type TechnologyRepository interface {
	GetTechnology(context.Context, TechnologyQuery) (domaingame.Technology, error)
}

type TechnologyQuery struct {
	PlayerID     int
	PlanetID     int
	TechnologyID int
}

type TechnologyCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	TechnologyID    int
}

type TechnologyResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Technology    domaingame.Technology
}

type TechnologyService struct {
	sessions   SessionLookup
	repository TechnologyRepository
}

func NewTechnologyService(sessions SessionLookup, repository TechnologyRepository) TechnologyService {
	return TechnologyService{sessions: sessions, repository: repository}
}

func (s TechnologyService) GetTechnology(ctx context.Context, command TechnologyCommand) (TechnologyResult, error) {
	if s.sessions == nil || s.repository == nil {
		return TechnologyResult{}, errors.New("technology dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return TechnologyResult{}, err
	}
	if !session.Authenticated {
		return TechnologyResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	technology, err := s.repository.GetTechnology(ctx, TechnologyQuery{
		PlayerID:     session.Session.PlayerID,
		PlanetID:     command.PlanetID,
		TechnologyID: command.TechnologyID,
	})
	if err != nil {
		return TechnologyResult{}, err
	}
	return TechnologyResult{
		Authenticated: true,
		Technology:    technology,
	}, nil
}
