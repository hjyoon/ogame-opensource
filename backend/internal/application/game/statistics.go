package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type StatisticsRepository interface {
	GetStatistics(context.Context, StatisticsQuery) (domaingame.Statistics, error)
}

type StatisticsQuery struct {
	PlayerID int
	PlanetID int
	Who      string
	Type     string
	Start    int
}

type StatisticsCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Who             string
	Type            string
	Start           int
}

type StatisticsResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Statistics    domaingame.Statistics
}

type StatisticsService struct {
	sessions   SessionLookup
	repository StatisticsRepository
}

func NewStatisticsService(sessions SessionLookup, repository StatisticsRepository) StatisticsService {
	return StatisticsService{sessions: sessions, repository: repository}
}

func (s StatisticsService) GetStatistics(ctx context.Context, command StatisticsCommand) (StatisticsResult, error) {
	if s.sessions == nil || s.repository == nil {
		return StatisticsResult{}, errors.New("statistics dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return StatisticsResult{}, err
	}
	if !session.Authenticated {
		return StatisticsResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	statistics, err := s.repository.GetStatistics(ctx, StatisticsQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		Who:      command.Who,
		Type:     command.Type,
		Start:    command.Start,
	})
	if err != nil {
		return StatisticsResult{}, err
	}
	return StatisticsResult{
		Authenticated: true,
		Statistics:    statistics,
	}, nil
}
