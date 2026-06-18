package game

import (
	"context"
	"errors"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestResearchServiceReturnsAuthenticatedResearch(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session: domainpublicsite.GameSession{
			PlayerID: 42,
		},
	}}
	repository := &fakeResearchRepository{result: domaingame.Research{Commander: "legor"}}
	service := NewResearchService(sessions, repository)

	result, err := service.GetResearch(context.Background(), ResearchCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "secret"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Research.Commander != "legor" {
		t.Fatalf("unexpected research result: %+v", result)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 99 {
		t.Fatalf("unexpected repository query: %+v", repository.query)
	}
	if sessions.command.PublicSession != "public" || sessions.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}
}

func TestResearchServiceReturnsSessionIssuesWithoutRepository(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	repository := &fakeResearchRepository{}
	service := NewResearchService(sessions, repository)

	result, err := service.GetResearch(context.Background(), ResearchCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.called {
		t.Fatalf("expected unauthenticated result without repository call, got %+v", result)
	}
}

func TestResearchServicePropagatesErrors(t *testing.T) {
	sessionErr := errors.New("session failed")
	service := NewResearchService(&fakeSessionLookup{err: sessionErr}, &fakeResearchRepository{})
	if _, err := service.GetResearch(context.Background(), ResearchCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected session error, got %v", err)
	}

	repoErr := errors.New("repository failed")
	service = NewResearchService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeResearchRepository{err: repoErr})
	if _, err := service.GetResearch(context.Background(), ResearchCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestResearchServiceRequiresDependencies(t *testing.T) {
	if _, err := (ResearchService{}).GetResearch(context.Background(), ResearchCommand{}); err == nil {
		t.Fatal("expected dependency error")
	}
}

type fakeResearchRepository struct {
	result domaingame.Research
	err    error
	query  ResearchQuery
	called bool
}

func (f *fakeResearchRepository) GetResearch(_ context.Context, query ResearchQuery) (domaingame.Research, error) {
	f.query = query
	f.called = true
	return f.result, f.err
}
