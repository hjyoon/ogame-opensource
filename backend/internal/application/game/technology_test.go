package game

import (
	"context"
	"errors"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestTechnologyServiceReturnsAuthenticatedTechnology(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session: domainpublicsite.GameSession{
			PlayerID: 42,
		},
	}}
	repository := &fakeTechnologyRepository{result: domaingame.Technology{Commander: "legor"}}
	service := NewTechnologyService(sessions, repository)

	result, err := service.GetTechnology(context.Background(), TechnologyCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "secret"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
		TechnologyID:    206,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Technology.Commander != "legor" {
		t.Fatalf("unexpected technology result: %+v", result)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 99 || repository.query.TechnologyID != 206 {
		t.Fatalf("unexpected repository query: %+v", repository.query)
	}
	if sessions.command.PublicSession != "public" || sessions.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}
}

func TestTechnologyServiceReturnsSessionIssuesWithoutRepository(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	repository := &fakeTechnologyRepository{}
	service := NewTechnologyService(sessions, repository)

	result, err := service.GetTechnology(context.Background(), TechnologyCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.called {
		t.Fatalf("expected unauthenticated result without repository call, got %+v", result)
	}
}

func TestTechnologyServicePropagatesErrors(t *testing.T) {
	sessionErr := errors.New("session failed")
	service := NewTechnologyService(&fakeSessionLookup{err: sessionErr}, &fakeTechnologyRepository{})
	if _, err := service.GetTechnology(context.Background(), TechnologyCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected session error, got %v", err)
	}

	repoErr := errors.New("repository failed")
	service = NewTechnologyService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeTechnologyRepository{err: repoErr})
	if _, err := service.GetTechnology(context.Background(), TechnologyCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestTechnologyServiceRequiresDependencies(t *testing.T) {
	if _, err := (TechnologyService{}).GetTechnology(context.Background(), TechnologyCommand{}); err == nil {
		t.Fatal("expected dependency error")
	}
}

type fakeTechnologyRepository struct {
	result domaingame.Technology
	err    error
	query  TechnologyQuery
	called bool
}

func (f *fakeTechnologyRepository) GetTechnology(_ context.Context, query TechnologyQuery) (domaingame.Technology, error) {
	f.query = query
	f.called = true
	return f.result, f.err
}
