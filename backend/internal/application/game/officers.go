package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type OfficersRepository interface {
	GetOfficers(context.Context, OfficersQuery) (domaingame.Officers, error)
	RecruitOfficer(context.Context, OfficersMutationQuery) (domaingame.Officers, *domaingame.OfficerActionIssue, error)
}

type OfficersQuery struct {
	PlayerID int
	PlanetID int
}

type OfficersCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
}

type OfficersMutationQuery struct {
	PlayerID int
	PlanetID int
	Mutation domaingame.OfficerMutation
}

type OfficersMutationCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Mutation        domaingame.OfficerMutation
}

type OfficersResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Officers      domaingame.Officers
	ActionIssue   *domaingame.OfficerActionIssue
}

type OfficersService struct {
	sessions   SessionLookup
	repository OfficersRepository
}

func NewOfficersService(sessions SessionLookup, repository OfficersRepository) OfficersService {
	return OfficersService{sessions: sessions, repository: repository}
}

func (s OfficersService) GetOfficers(ctx context.Context, command OfficersCommand) (OfficersResult, error) {
	if s.sessions == nil || s.repository == nil {
		return OfficersResult{}, errors.New("officers dependencies unavailable")
	}
	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return OfficersResult{}, err
	}
	if !session.Authenticated {
		return OfficersResult{Authenticated: false, Issues: session.Issues}, nil
	}
	officers, err := s.repository.GetOfficers(ctx, OfficersQuery{PlayerID: session.Session.PlayerID, PlanetID: command.PlanetID})
	if err != nil {
		return OfficersResult{}, err
	}
	return OfficersResult{Authenticated: true, Officers: officers}, nil
}

func (s OfficersService) RecruitOfficer(ctx context.Context, command OfficersMutationCommand) (OfficersResult, error) {
	if s.sessions == nil || s.repository == nil {
		return OfficersResult{}, errors.New("officers dependencies unavailable")
	}
	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return OfficersResult{}, err
	}
	if !session.Authenticated {
		return OfficersResult{Authenticated: false, Issues: session.Issues}, nil
	}
	officers, issue, err := s.repository.RecruitOfficer(ctx, OfficersMutationQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		Mutation: command.Mutation,
	})
	if err != nil {
		return OfficersResult{}, err
	}
	return OfficersResult{Authenticated: true, Officers: officers, ActionIssue: issue}, nil
}
