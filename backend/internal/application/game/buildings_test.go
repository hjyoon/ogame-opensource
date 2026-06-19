package game

import (
	"context"
	"errors"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestBuildingsServiceReturnsAuthenticatedBuildings(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session: domainpublicsite.GameSession{
			PlayerID: 42,
		},
	}}
	repository := &fakeBuildingsRepository{result: domaingame.Buildings{Commander: "legor"}}
	service := NewBuildingsService(sessions, repository)

	result, err := service.GetBuildings(context.Background(), BuildingsCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "secret"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Buildings.Commander != "legor" {
		t.Fatalf("unexpected buildings result: %+v", result)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 99 {
		t.Fatalf("unexpected repository query: %+v", repository.query)
	}
	if sessions.command.PublicSession != "public" || sessions.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}
}

func TestBuildingsServiceMutatesAuthenticatedBuildings(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session: domainpublicsite.GameSession{
			PlayerID: 42,
		},
	}}
	issue := domaingame.BuildingActionIssue(domaingame.BuildingsIssueNoResources)
	repository := &fakeBuildingsRepository{
		result:  domaingame.Buildings{Commander: "legor"},
		outcome: BuildingsMutationOutcome{ActionIssue: issue},
	}
	service := NewBuildingsService(sessions, repository)

	result, err := service.MutateBuildings(context.Background(), BuildingsMutationCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "secret"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
		Action:          "add",
		TechID:          domaingame.BuildingMetalMine,
		ListID:          1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.ActionIssue != issue || result.Buildings.Commander != "legor" {
		t.Fatalf("unexpected buildings mutation result: %+v", result)
	}
	if repository.mutation.PlayerID != 42 || repository.mutation.PlanetID != 99 || repository.mutation.Action != "add" ||
		repository.mutation.TechID != domaingame.BuildingMetalMine || repository.mutation.ListID != 1 {
		t.Fatalf("unexpected repository mutation: %+v", repository.mutation)
	}
}

func TestBuildingsServiceReturnsSessionIssuesWithoutRepository(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	repository := &fakeBuildingsRepository{}
	service := NewBuildingsService(sessions, repository)

	result, err := service.GetBuildings(context.Background(), BuildingsCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.called {
		t.Fatalf("expected unauthenticated result without repository call, got %+v", result)
	}

	result, err = service.MutateBuildings(context.Background(), BuildingsMutationCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.mutated {
		t.Fatalf("expected unauthenticated mutation without repository call, got %+v", result)
	}
}

func TestBuildingsServicePropagatesErrors(t *testing.T) {
	sessionErr := errors.New("session failed")
	service := NewBuildingsService(&fakeSessionLookup{err: sessionErr}, &fakeBuildingsRepository{})
	if _, err := service.GetBuildings(context.Background(), BuildingsCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected session error, got %v", err)
	}
	if _, err := service.MutateBuildings(context.Background(), BuildingsMutationCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected mutation session error, got %v", err)
	}

	repoErr := errors.New("repository failed")
	service = NewBuildingsService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeBuildingsRepository{err: repoErr})
	if _, err := service.GetBuildings(context.Background(), BuildingsCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
	if _, err := service.MutateBuildings(context.Background(), BuildingsMutationCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected mutation repository error, got %v", err)
	}

	readErr := errors.New("read after mutation failed")
	service = NewBuildingsService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeBuildingsRepository{getErr: readErr})
	if _, err := service.MutateBuildings(context.Background(), BuildingsMutationCommand{}); !errors.Is(err, readErr) {
		t.Fatalf("expected mutation read error, got %v", err)
	}
}

func TestBuildingsServiceRequiresDependencies(t *testing.T) {
	if _, err := (BuildingsService{}).GetBuildings(context.Background(), BuildingsCommand{}); err == nil {
		t.Fatal("expected dependency error")
	}
	if _, err := (BuildingsService{}).MutateBuildings(context.Background(), BuildingsMutationCommand{}); err == nil {
		t.Fatal("expected mutation dependency error")
	}
}

type fakeBuildingsRepository struct {
	result   domaingame.Buildings
	outcome  BuildingsMutationOutcome
	err      error
	getErr   error
	query    BuildingsQuery
	mutation BuildingsMutationQuery
	called   bool
	mutated  bool
}

func (f *fakeBuildingsRepository) GetBuildings(_ context.Context, query BuildingsQuery) (domaingame.Buildings, error) {
	f.query = query
	f.called = true
	if f.getErr != nil {
		return domaingame.Buildings{}, f.getErr
	}
	return f.result, f.err
}

func (f *fakeBuildingsRepository) MutateBuildings(_ context.Context, query BuildingsMutationQuery) (BuildingsMutationOutcome, error) {
	f.mutation = query
	f.mutated = true
	return f.outcome, f.err
}
