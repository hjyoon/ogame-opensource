package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type SearchRepository interface {
	GetSearch(context.Context, SearchQuery) (domaingame.Search, error)
}

type SearchQuery struct {
	PlayerID int
	PlanetID int
	Type     string
	Text     string
}

type SearchCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Type            string
	Text            string
}

type SearchResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Search        domaingame.Search
}

type SearchService struct {
	sessions   SessionLookup
	repository SearchRepository
}

func NewSearchService(sessions SessionLookup, repository SearchRepository) SearchService {
	return SearchService{sessions: sessions, repository: repository}
}

func (s SearchService) GetSearch(ctx context.Context, command SearchCommand) (SearchResult, error) {
	if s.sessions == nil || s.repository == nil {
		return SearchResult{}, errors.New("search dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return SearchResult{}, err
	}
	if !session.Authenticated {
		return SearchResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	search, err := s.repository.GetSearch(ctx, SearchQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		Type:     command.Type,
		Text:     command.Text,
	})
	if err != nil {
		return SearchResult{}, err
	}
	return SearchResult{
		Authenticated: true,
		Search:        search,
	}, nil
}
