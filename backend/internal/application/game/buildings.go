package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type BuildingsRepository interface {
	GetBuildings(context.Context, BuildingsQuery) (domaingame.Buildings, error)
	MutateBuildings(context.Context, BuildingsMutationQuery) (BuildingsMutationOutcome, error)
}

type BuildingsQuery struct {
	PlayerID int
	PlanetID int
}

type BuildingsCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
}

type BuildingsMutationQuery struct {
	PlayerID int
	PlanetID int
	Action   string
	TechID   int
	ListID   int
}

type BuildingsMutationCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Action          string
	TechID          int
	ListID          int
}

type BuildingsMutationOutcome struct {
	ActionIssue *domaingame.BuildingsActionIssue
}

type BuildingsResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	ActionIssue   *domaingame.BuildingsActionIssue
	Buildings     domaingame.Buildings
}

type BuildingsService struct {
	sessions   SessionLookup
	repository BuildingsRepository
}

func NewBuildingsService(sessions SessionLookup, repository BuildingsRepository) BuildingsService {
	return BuildingsService{sessions: sessions, repository: repository}
}

func (s BuildingsService) GetBuildings(ctx context.Context, command BuildingsCommand) (BuildingsResult, error) {
	if s.sessions == nil || s.repository == nil {
		return BuildingsResult{}, errors.New("buildings dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return BuildingsResult{}, err
	}
	if !session.Authenticated {
		return BuildingsResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	buildings, err := s.repository.GetBuildings(ctx, BuildingsQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
	})
	if err != nil {
		return BuildingsResult{}, err
	}
	return BuildingsResult{
		Authenticated: true,
		Buildings:     buildings,
	}, nil
}

func (s BuildingsService) MutateBuildings(ctx context.Context, command BuildingsMutationCommand) (BuildingsResult, error) {
	if s.sessions == nil || s.repository == nil {
		return BuildingsResult{}, errors.New("buildings dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return BuildingsResult{}, err
	}
	if !session.Authenticated {
		return BuildingsResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	outcome, err := s.repository.MutateBuildings(ctx, BuildingsMutationQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		Action:   command.Action,
		TechID:   command.TechID,
		ListID:   command.ListID,
	})
	if err != nil {
		return BuildingsResult{}, err
	}
	buildings, err := s.repository.GetBuildings(ctx, BuildingsQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
	})
	if err != nil {
		return BuildingsResult{}, err
	}
	return BuildingsResult{
		Authenticated: true,
		ActionIssue:   outcome.ActionIssue,
		Buildings:     buildings,
	}, nil
}
