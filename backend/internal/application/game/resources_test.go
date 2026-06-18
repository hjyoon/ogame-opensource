package game

import (
	"context"
	"errors"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestResourcesServiceReturnsAuthenticatedResources(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeResourcesRepository{result: domaingame.ResourceProduction{Commander: "legor"}}
	service := NewResourcesService(sessions, repository)

	result, err := service.GetResources(context.Background(), ResourcesCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "secret"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Resources.Commander != "legor" {
		t.Fatalf("unexpected resources result: %+v", result)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 99 {
		t.Fatalf("unexpected repository query: %+v", repository.query)
	}
	if sessions.command.PublicSession != "public" || sessions.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}
}

func TestResourcesServiceReturnsSessionIssuesWithoutRepository(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	repository := &fakeResourcesRepository{}
	service := NewResourcesService(sessions, repository)

	result, err := service.GetResources(context.Background(), ResourcesCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.called {
		t.Fatalf("expected unauthenticated result without repository call, got %+v", result)
	}
}

func TestResourcesServicePropagatesErrors(t *testing.T) {
	sessionErr := errors.New("session failed")
	service := NewResourcesService(&fakeSessionLookup{err: sessionErr}, &fakeResourcesRepository{})
	if _, err := service.GetResources(context.Background(), ResourcesCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected session error, got %v", err)
	}

	repoErr := errors.New("repository failed")
	service = NewResourcesService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeResourcesRepository{err: repoErr})
	if _, err := service.GetResources(context.Background(), ResourcesCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestResourcesServiceRequiresDependencies(t *testing.T) {
	if _, err := (ResourcesService{}).GetResources(context.Background(), ResourcesCommand{}); err == nil {
		t.Fatal("expected dependency error")
	}
}

type fakeResourcesRepository struct {
	result domaingame.ResourceProduction
	err    error
	query  ResourcesQuery
	called bool
}

func (f *fakeResourcesRepository) GetResources(_ context.Context, query ResourcesQuery) (domaingame.ResourceProduction, error) {
	f.query = query
	f.called = true
	return f.result, f.err
}
