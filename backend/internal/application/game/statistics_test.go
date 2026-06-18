package game

import (
	"context"
	"errors"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestStatisticsServiceReturnsAuthenticatedStatistics(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeStatisticsRepository{result: domaingame.Statistics{Commander: "legor", Type: domaingame.StatisticsTypeResources}}
	service := NewStatisticsService(sessions, repository)

	result, err := service.GetStatistics(context.Background(), StatisticsCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "secret"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
		Who:             "player",
		Type:            "fleet",
		Start:           101,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Statistics.Commander != "legor" {
		t.Fatalf("unexpected statistics result: %+v", result)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 99 || repository.query.Type != "fleet" || repository.query.Start != 101 {
		t.Fatalf("unexpected repository query: %+v", repository.query)
	}
	if sessions.command.PublicSession != "public" || sessions.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}
}

func TestStatisticsServiceReturnsSessionIssuesWithoutRepository(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	repository := &fakeStatisticsRepository{}
	service := NewStatisticsService(sessions, repository)

	result, err := service.GetStatistics(context.Background(), StatisticsCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.called {
		t.Fatalf("expected unauthenticated result without repository call, got %+v", result)
	}
}

func TestStatisticsServicePropagatesErrors(t *testing.T) {
	sessionErr := errors.New("session failed")
	service := NewStatisticsService(&fakeSessionLookup{err: sessionErr}, &fakeStatisticsRepository{})
	if _, err := service.GetStatistics(context.Background(), StatisticsCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected session error, got %v", err)
	}

	repoErr := errors.New("repository failed")
	service = NewStatisticsService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeStatisticsRepository{err: repoErr})
	if _, err := service.GetStatistics(context.Background(), StatisticsCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestStatisticsServiceRequiresDependencies(t *testing.T) {
	if _, err := (StatisticsService{}).GetStatistics(context.Background(), StatisticsCommand{}); err == nil {
		t.Fatal("expected dependency error")
	}
}

type fakeStatisticsRepository struct {
	result domaingame.Statistics
	err    error
	query  StatisticsQuery
	called bool
}

func (f *fakeStatisticsRepository) GetStatistics(_ context.Context, query StatisticsQuery) (domaingame.Statistics, error) {
	f.query = query
	f.called = true
	return f.result, f.err
}
