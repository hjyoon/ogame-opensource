package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type ResearchRepository interface {
	GetResearch(context.Context, ResearchQuery) (domaingame.Research, error)
}

type ResearchQuery struct {
	PlayerID int
	PlanetID int
}

type ResearchCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
}

type ResearchResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Research      domaingame.Research
}

type ResearchService struct {
	sessions   SessionLookup
	repository ResearchRepository
}

func NewResearchService(sessions SessionLookup, repository ResearchRepository) ResearchService {
	return ResearchService{sessions: sessions, repository: repository}
}

func (s ResearchService) GetResearch(ctx context.Context, command ResearchCommand) (ResearchResult, error) {
	if s.sessions == nil || s.repository == nil {
		return ResearchResult{}, errors.New("research dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return ResearchResult{}, err
	}
	if !session.Authenticated {
		return ResearchResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	research, err := s.repository.GetResearch(ctx, ResearchQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
	})
	if err != nil {
		return ResearchResult{}, err
	}
	return ResearchResult{
		Authenticated: true,
		Research:      research,
	}, nil
}
