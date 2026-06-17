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
}

type OverviewQuery struct {
	PlayerID int
	PlanetID int
}

type OverviewCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
}

type OverviewResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Overview      domaingame.Overview
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

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
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
	})
	if err != nil {
		return OverviewResult{}, err
	}
	return OverviewResult{
		Authenticated: true,
		Overview:      overview,
	}, nil
}
