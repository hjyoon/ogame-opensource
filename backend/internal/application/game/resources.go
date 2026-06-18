package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type ResourcesRepository interface {
	GetResources(context.Context, ResourcesQuery) (domaingame.ResourceProduction, error)
	UpdateProduction(context.Context, ResourcesUpdateQuery) (domaingame.ResourceProduction, error)
}

type ResourcesQuery struct {
	PlayerID int
	PlanetID int
}

type ResourcesCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
}

type ResourcesUpdateQuery struct {
	PlayerID   int
	PlanetID   int
	Production domaingame.ProductionFactors
}

type ResourcesUpdateCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Production      domaingame.ProductionPercents
}

type ResourcesResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Resources     domaingame.ResourceProduction
}

type ResourcesService struct {
	sessions   SessionLookup
	repository ResourcesRepository
}

func NewResourcesService(sessions SessionLookup, repository ResourcesRepository) ResourcesService {
	return ResourcesService{sessions: sessions, repository: repository}
}

func (s ResourcesService) GetResources(ctx context.Context, command ResourcesCommand) (ResourcesResult, error) {
	if s.sessions == nil || s.repository == nil {
		return ResourcesResult{}, errors.New("resources dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return ResourcesResult{}, err
	}
	if !session.Authenticated {
		return ResourcesResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	resources, err := s.repository.GetResources(ctx, ResourcesQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
	})
	if err != nil {
		return ResourcesResult{}, err
	}
	return ResourcesResult{
		Authenticated: true,
		Resources:     resources,
	}, nil
}

func (s ResourcesService) UpdateResources(ctx context.Context, command ResourcesUpdateCommand) (ResourcesResult, error) {
	if s.sessions == nil || s.repository == nil {
		return ResourcesResult{}, errors.New("resources dependencies unavailable")
	}

	production, err := domaingame.NormalizeProductionSettings(command.Production)
	if err != nil {
		return ResourcesResult{}, err
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return ResourcesResult{}, err
	}
	if !session.Authenticated {
		return ResourcesResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	resources, err := s.repository.UpdateProduction(ctx, ResourcesUpdateQuery{
		PlayerID:   session.Session.PlayerID,
		PlanetID:   command.PlanetID,
		Production: production,
	})
	if err != nil {
		return ResourcesResult{}, err
	}
	return ResourcesResult{
		Authenticated: true,
		Resources:     resources,
	}, nil
}
