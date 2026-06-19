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
	MutateFleetTemplate(context.Context, FleetTemplateMutationQuery) error
	RecallFleet(context.Context, FleetRecallQuery) error
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

type FleetTemplateMutationQuery struct {
	PlayerID   int
	TemplateID int
	Action     string
	Name       string
	Ships      map[int]int
}

type FleetTemplateMutationCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	TemplateID      int
	Action          string
	Name            string
	Ships           map[int]int
}

type FleetDispatchPrepareCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Ships           map[int]int
	Target          domaingame.Coordinates
	TargetType      int
	Mission         int
	Speed           int
}

type FleetRecallQuery struct {
	PlayerID int
	FleetID  int
}

type FleetRecallCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	FleetID         int
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

func (s FleetService) MutateFleetTemplate(ctx context.Context, command FleetTemplateMutationCommand) (FleetResult, error) {
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

	if err := s.repository.MutateFleetTemplate(ctx, FleetTemplateMutationQuery{
		PlayerID:   session.Session.PlayerID,
		TemplateID: command.TemplateID,
		Action:     command.Action,
		Name:       command.Name,
		Ships:      command.Ships,
	}); err != nil {
		return FleetResult{}, err
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

func (s FleetService) PrepareFleetDispatch(ctx context.Context, command FleetDispatchPrepareCommand) (FleetResult, error) {
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
	draft := domaingame.BuildFleetDispatchDraft(fleet, domaingame.FleetDispatchDraftInput{
		Ships:      command.Ships,
		Target:     command.Target,
		TargetType: command.TargetType,
		Mission:    command.Mission,
		Speed:      command.Speed,
	})
	if draft.HasSelection {
		fleet.DispatchDraft = &draft
	}
	return FleetResult{
		Authenticated: true,
		Fleet:         fleet,
	}, nil
}

func (s FleetService) RecallFleet(ctx context.Context, command FleetRecallCommand) (FleetResult, error) {
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

	if err := s.repository.RecallFleet(ctx, FleetRecallQuery{
		PlayerID: session.Session.PlayerID,
		FleetID:  command.FleetID,
	}); err != nil {
		return FleetResult{}, err
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
