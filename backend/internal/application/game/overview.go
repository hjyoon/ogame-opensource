package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type SessionLookup interface {
	GetGameSession(context.Context, apppublicsite.GameSessionCommand) (domainpublicsite.SessionAuthentication, error)
}

type OverviewRepository interface {
	GetOverview(context.Context, OverviewQuery) (domaingame.Overview, error)
	RenamePlanet(context.Context, OverviewRenameQuery) (domaingame.Overview, error)
	DeletePlanet(context.Context, OverviewDeleteQuery) (domaingame.Overview, *domaingame.OverviewActionIssue, error)
}

type OverviewQuery struct {
	PlayerID int
	PlanetID int
	Login    bool
}

type OverviewRenameQuery struct {
	PlayerID int
	PlanetID int
	Name     string
}

type OverviewDeleteQuery struct {
	PlayerID int
	PlanetID int
	DeleteID int
	Password string
}

type OverviewCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Login           bool
}

type OverviewRenameCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Name            string
}

type OverviewDeleteCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	DeleteID        int
	Password        string
}

type OverviewResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Overview      domaingame.Overview
	ActionIssue   *domaingame.OverviewActionIssue
}

type OverviewService struct {
	sessions   SessionLookup
	repository OverviewRepository
}

func NewOverviewService(sessions SessionLookup, repository OverviewRepository) OverviewService {
	return OverviewService{sessions: sessions, repository: repository}
}

func (s OverviewService) GetOverview(ctx context.Context, command OverviewCommand) (OverviewResult, error) {
	if s.sessions == nil || s.repository == nil {
		return OverviewResult{}, errors.New("overview dependencies unavailable")
	}

	session, err := s.authenticatedSession(ctx, command.PublicSession, command.PrivateSessions, command.RemoteAddr)
	if err != nil {
		return OverviewResult{}, err
	}
	if !session.Authenticated {
		return OverviewResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	overview, err := s.repository.GetOverview(ctx, OverviewQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		Login:    command.Login,
	})
	if err != nil {
		return OverviewResult{}, err
	}
	return OverviewResult{
		Authenticated: true,
		Overview:      overview,
	}, nil
}

func (s OverviewService) RenamePlanet(ctx context.Context, command OverviewRenameCommand) (OverviewResult, error) {
	if s.sessions == nil || s.repository == nil {
		return OverviewResult{}, errors.New("overview dependencies unavailable")
	}

	session, err := s.authenticatedSession(ctx, command.PublicSession, command.PrivateSessions, command.RemoteAddr)
	if err != nil {
		return OverviewResult{}, err
	}
	if !session.Authenticated {
		return OverviewResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	overview, err := s.repository.RenamePlanet(ctx, OverviewRenameQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		Name:     command.Name,
	})
	if err != nil {
		return OverviewResult{}, err
	}
	return OverviewResult{
		Authenticated: true,
		Overview:      overview,
	}, nil
}

func (s OverviewService) DeletePlanet(ctx context.Context, command OverviewDeleteCommand) (OverviewResult, error) {
	if s.sessions == nil || s.repository == nil {
		return OverviewResult{}, errors.New("overview dependencies unavailable")
	}

	session, err := s.authenticatedSession(ctx, command.PublicSession, command.PrivateSessions, command.RemoteAddr)
	if err != nil {
		return OverviewResult{}, err
	}
	if !session.Authenticated {
		return OverviewResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	overview, actionIssue, err := s.repository.DeletePlanet(ctx, OverviewDeleteQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		DeleteID: command.DeleteID,
		Password: command.Password,
	})
	if err != nil {
		return OverviewResult{}, err
	}
	return OverviewResult{
		Authenticated: true,
		Overview:      overview,
		ActionIssue:   actionIssue,
	}, nil
}

func (s OverviewService) authenticatedSession(ctx context.Context, publicSession string, privateSessions map[string]string, remoteAddr string) (domainpublicsite.SessionAuthentication, error) {
	return s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   publicSession,
		PrivateSessions: privateSessions,
		RemoteAddr:      remoteAddr,
	})
}
