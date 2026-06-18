package game

import (
	"context"
	"errors"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestGalaxyServiceReturnsAuthenticatedGalaxy(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeGalaxyRepository{result: domaingame.Galaxy{Commander: "legor"}}
	service := NewGalaxyService(sessions, repository)

	result, err := service.GetGalaxy(context.Background(), GalaxyCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "secret"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
		Coordinates:     domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Galaxy.Commander != "legor" {
		t.Fatalf("unexpected galaxy result: %+v", result)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 99 || repository.query.Coordinates.System != 2 {
		t.Fatalf("unexpected repository query: %+v", repository.query)
	}
	if sessions.command.PublicSession != "public" || sessions.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}
}

func TestGalaxyServiceReturnsSessionIssuesWithoutRepository(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	repository := &fakeGalaxyRepository{}
	service := NewGalaxyService(sessions, repository)

	result, err := service.GetGalaxy(context.Background(), GalaxyCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.called {
		t.Fatalf("expected unauthenticated result without repository call, got %+v", result)
	}
}

func TestGalaxyServicePropagatesErrors(t *testing.T) {
	sessionErr := errors.New("session failed")
	service := NewGalaxyService(&fakeSessionLookup{err: sessionErr}, &fakeGalaxyRepository{})
	if _, err := service.GetGalaxy(context.Background(), GalaxyCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected session error, got %v", err)
	}

	repoErr := errors.New("repository failed")
	service = NewGalaxyService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeGalaxyRepository{err: repoErr})
	if _, err := service.GetGalaxy(context.Background(), GalaxyCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestGalaxyServiceRequiresDependencies(t *testing.T) {
	if _, err := (GalaxyService{}).GetGalaxy(context.Background(), GalaxyCommand{}); err == nil {
		t.Fatal("expected dependency error")
	}
}

type fakeGalaxyRepository struct {
	result domaingame.Galaxy
	err    error
	query  GalaxyQuery
	called bool
}

func (f *fakeGalaxyRepository) GetGalaxy(_ context.Context, query GalaxyQuery) (domaingame.Galaxy, error) {
	f.query = query
	f.called = true
	return f.result, f.err
}
