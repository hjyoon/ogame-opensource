package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type EmpireRepository interface {
	GetEmpire(context.Context, EmpireQuery) (domaingame.Empire, *domaingame.EmpireActionIssue, error)
	MutateEmpire(context.Context, EmpireMutationQuery) (EmpireMutationOutcome, error)
}

type EmpireQuery struct {
	PlayerID   int
	PlanetID   int
	PlanetType int
}

type EmpireCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	PlanetType      int
}

type EmpireMutationQuery struct {
	PlayerID int
	PlanetID int
	Action   string
	TechID   int
	ListID   int
}

type EmpireMutationCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	PlanetType      int
	Action          string
	TargetPlanetID  int
	TechID          int
	ListID          int
}

type EmpireMutationOutcome struct {
	ActionIssue *domaingame.EmpireActionIssue
}

type EmpireResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Empire        domaingame.Empire
	ActionIssue   *domaingame.EmpireActionIssue
}

type EmpireService struct {
	sessions   SessionLookup
	repository EmpireRepository
}

func NewEmpireService(sessions SessionLookup, repository EmpireRepository) EmpireService {
	return EmpireService{sessions: sessions, repository: repository}
}

func (s EmpireService) GetEmpire(ctx context.Context, command EmpireCommand) (EmpireResult, error) {
	if s.sessions == nil || s.repository == nil {
		return EmpireResult{}, errors.New("empire dependencies unavailable")
	}
	session, err := s.authenticatedSession(ctx, command.PublicSession, command.PrivateSessions, command.RemoteAddr)
	if err != nil {
		return EmpireResult{}, err
	}
	if !session.Authenticated {
		return EmpireResult{Authenticated: false, Issues: session.Issues}, nil
	}
	empire, issue, err := s.repository.GetEmpire(ctx, EmpireQuery{
		PlayerID:   session.Session.PlayerID,
		PlanetID:   command.PlanetID,
		PlanetType: command.PlanetType,
	})
	if err != nil {
		return EmpireResult{}, err
	}
	return EmpireResult{Authenticated: true, Empire: empire, ActionIssue: issue}, nil
}

func (s EmpireService) MutateEmpire(ctx context.Context, command EmpireMutationCommand) (EmpireResult, error) {
	if s.sessions == nil || s.repository == nil {
		return EmpireResult{}, errors.New("empire dependencies unavailable")
	}
	session, err := s.authenticatedSession(ctx, command.PublicSession, command.PrivateSessions, command.RemoteAddr)
	if err != nil {
		return EmpireResult{}, err
	}
	if !session.Authenticated {
		return EmpireResult{Authenticated: false, Issues: session.Issues}, nil
	}
	targetPlanetID := command.TargetPlanetID
	if targetPlanetID == 0 {
		targetPlanetID = command.PlanetID
	}
	outcome, err := s.repository.MutateEmpire(ctx, EmpireMutationQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: targetPlanetID,
		Action:   command.Action,
		TechID:   command.TechID,
		ListID:   command.ListID,
	})
	if err != nil {
		return EmpireResult{}, err
	}
	empire, issue, err := s.repository.GetEmpire(ctx, EmpireQuery{
		PlayerID:   session.Session.PlayerID,
		PlanetID:   command.PlanetID,
		PlanetType: command.PlanetType,
	})
	if err != nil {
		return EmpireResult{}, err
	}
	if outcome.ActionIssue != nil {
		issue = outcome.ActionIssue
	}
	return EmpireResult{Authenticated: true, Empire: empire, ActionIssue: issue}, nil
}

func (s EmpireService) authenticatedSession(ctx context.Context, publicSession string, privateSessions map[string]string, remoteAddr string) (domainpublicsite.SessionAuthentication, error) {
	return s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   publicSession,
		PrivateSessions: privateSessions,
		RemoteAddr:      remoteAddr,
	})
}
