package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type AllianceRepository interface {
	GetAlliance(context.Context, AllianceQuery) (domaingame.Alliance, error)
	MutateAlliance(context.Context, AllianceMutationQuery) (domaingame.Alliance, *domaingame.AllianceActionIssue, error)
}

type AllianceQuery struct {
	PlayerID      int
	PlanetID      int
	View          domaingame.AllianceView
	SearchText    string
	TextKind      int
	AllianceID    int
	ApplicationID int
}

type AllianceCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	View            domaingame.AllianceView
	SearchText      string
	TextKind        int
	AllianceID      int
	ApplicationID   int
}

type AllianceMutationQuery struct {
	PlayerID int
	PlanetID int
	Query    AllianceQuery
	Mutation domaingame.AllianceMutation
}

type AllianceMutationCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Query           AllianceQuery
	Mutation        domaingame.AllianceMutation
}

type AllianceResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Alliance      domaingame.Alliance
	ActionIssue   *domaingame.AllianceActionIssue
}

type AllianceService struct {
	sessions   SessionLookup
	repository AllianceRepository
}

func NewAllianceService(sessions SessionLookup, repository AllianceRepository) AllianceService {
	return AllianceService{sessions: sessions, repository: repository}
}

func (s AllianceService) GetAlliance(ctx context.Context, command AllianceCommand) (AllianceResult, error) {
	if s.sessions == nil || s.repository == nil {
		return AllianceResult{}, errors.New("alliance dependencies unavailable")
	}
	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return AllianceResult{}, err
	}
	if !session.Authenticated {
		return AllianceResult{Authenticated: false, Issues: session.Issues}, nil
	}
	alliance, err := s.repository.GetAlliance(ctx, AllianceQuery{
		PlayerID:      session.Session.PlayerID,
		PlanetID:      command.PlanetID,
		View:          command.View,
		SearchText:    command.SearchText,
		TextKind:      command.TextKind,
		AllianceID:    command.AllianceID,
		ApplicationID: command.ApplicationID,
	})
	if err != nil {
		return AllianceResult{}, err
	}
	return AllianceResult{Authenticated: true, Alliance: alliance}, nil
}

func (s AllianceService) MutateAlliance(ctx context.Context, command AllianceMutationCommand) (AllianceResult, error) {
	if s.sessions == nil || s.repository == nil {
		return AllianceResult{}, errors.New("alliance dependencies unavailable")
	}
	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return AllianceResult{}, err
	}
	if !session.Authenticated {
		return AllianceResult{Authenticated: false, Issues: session.Issues}, nil
	}
	query := command.Query
	query.PlayerID = session.Session.PlayerID
	query.PlanetID = command.PlanetID
	alliance, issue, err := s.repository.MutateAlliance(ctx, AllianceMutationQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		Query:    query,
		Mutation: command.Mutation,
	})
	if err != nil {
		return AllianceResult{}, err
	}
	return AllianceResult{Authenticated: true, Alliance: alliance, ActionIssue: issue}, nil
}
