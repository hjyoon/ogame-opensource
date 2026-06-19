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

func TestResearchServiceMutatesAuthenticatedResearch(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeResearchRepository{
		result:       domaingame.Research{Commander: "legor"},
		mutateResult: ResearchMutationOutcome{ActionIssue: domaingame.BuildingActionIssue(domaingame.BuildingsIssueNoResources)},
	}
	service := NewResearchService(sessions, repository)

	result, err := service.MutateResearch(context.Background(), ResearchMutationCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "secret"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
		Action:          "start",
		TechID:          domaingame.ResearchComputer,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Research.Commander != "legor" || result.ActionIssue == nil || result.ActionIssue.Code != domaingame.BuildingsIssueNoResources {
		t.Fatalf("unexpected mutate result: %+v", result)
	}
	if repository.mutateQuery.PlayerID != 42 || repository.mutateQuery.PlanetID != 99 || repository.mutateQuery.Action != "start" || repository.mutateQuery.TechID != domaingame.ResearchComputer {
		t.Fatalf("unexpected mutate query: %+v", repository.mutateQuery)
	}
}

func TestResearchServiceMutationReturnsSessionIssuesWithoutRepository(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	repository := &fakeResearchRepository{}
	service := NewResearchService(sessions, repository)

	result, err := service.MutateResearch(context.Background(), ResearchMutationCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.mutateCalled || repository.called {
		t.Fatalf("expected unauthenticated mutation without repository call, got %+v", result)
	}
}

func TestResearchServiceMutationPropagatesSessionErrors(t *testing.T) {
	sessionErr := errors.New("session failed")
	service := NewResearchService(&fakeSessionLookup{err: sessionErr}, &fakeResearchRepository{})

	if _, err := service.MutateResearch(context.Background(), ResearchMutationCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected mutation session error, got %v", err)
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

	service = NewResearchService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeResearchRepository{mutateErr: repoErr})
	if _, err := service.MutateResearch(context.Background(), ResearchMutationCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected mutation repository error, got %v", err)
	}

	service = NewResearchService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeResearchRepository{err: repoErr})
	if _, err := service.MutateResearch(context.Background(), ResearchMutationCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected post-mutation research load error, got %v", err)
	}
}

func TestResearchServiceRequiresDependencies(t *testing.T) {
	if _, err := (ResearchService{}).GetResearch(context.Background(), ResearchCommand{}); err == nil {
		t.Fatal("expected dependency error")
	}
	if _, err := (ResearchService{}).MutateResearch(context.Background(), ResearchMutationCommand{}); err == nil {
		t.Fatal("expected mutation dependency error")
	}
}

type fakeResearchRepository struct {
	result       domaingame.Research
	mutateResult ResearchMutationOutcome
	err          error
	mutateErr    error
	query        ResearchQuery
	mutateQuery  ResearchMutationQuery
	called       bool
	mutateCalled bool
}

func (f *fakeResearchRepository) GetResearch(_ context.Context, query ResearchQuery) (domaingame.Research, error) {
	f.query = query
	f.called = true
	return f.result, f.err
}

func (f *fakeResearchRepository) MutateResearch(_ context.Context, query ResearchMutationQuery) (ResearchMutationOutcome, error) {
	f.mutateQuery = query
	f.mutateCalled = true
	return f.mutateResult, f.mutateErr
}
