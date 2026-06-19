package game

import (
	"context"
	"errors"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestDefenseServiceReturnsAuthenticatedDefense(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeDefenseRepository{result: domaingame.Defense{Commander: "legor"}}
	service := NewDefenseService(sessions, repository)

	result, err := service.GetDefense(context.Background(), DefenseCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "secret"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Defense.Commander != "legor" {
		t.Fatalf("unexpected defense result: %+v", result)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 99 {
		t.Fatalf("unexpected repository query: %+v", repository.query)
	}
	if sessions.command.PublicSession != "public" || sessions.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}
}

func TestDefenseServiceReturnsSessionIssuesWithoutRepository(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	repository := &fakeDefenseRepository{}
	service := NewDefenseService(sessions, repository)

	result, err := service.GetDefense(context.Background(), DefenseCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.called {
		t.Fatalf("expected unauthenticated result without repository call, got %+v", result)
	}
}

func TestDefenseServiceMutatesAuthenticatedOrders(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	issue := domaingame.BuildingActionIssue(domaingame.BuildingsIssueQueueFull)
	repository := &fakeDefenseRepository{
		result:          domaingame.Defense{Commander: "legor"},
		mutationOutcome: DefenseMutationOutcome{ActionIssue: issue},
	}
	service := NewDefenseService(sessions, repository)

	result, err := service.MutateDefense(context.Background(), DefenseMutationCommand{
		PublicSession: "public",
		PlanetID:      99,
		Orders:        map[int]int{domaingame.DefenseRocketLauncher: 4},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Defense.Commander != "legor" || result.ActionIssue != issue {
		t.Fatalf("unexpected mutation result: %+v", result)
	}
	if repository.mutation.PlayerID != 42 || repository.mutation.PlanetID != 99 || repository.mutation.Orders[domaingame.DefenseRocketLauncher] != 4 {
		t.Fatalf("unexpected mutation query: %+v", repository.mutation)
	}
	if !repository.called {
		t.Fatal("expected read model refresh after mutation")
	}
}

func TestDefenseServiceMutateHandlesSessionIssuesAndErrors(t *testing.T) {
	if _, err := (DefenseService{}).MutateDefense(context.Background(), DefenseMutationCommand{}); err == nil {
		t.Fatal("expected dependency error")
	}

	repository := &fakeDefenseRepository{}
	service := NewDefenseService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues:        []domainpublicsite.SessionIssue{{Code: domainpublicsite.SessionIssuePrivateInvalid}},
	}}, repository)
	result, err := service.MutateDefense(context.Background(), DefenseMutationCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.mutation.Orders != nil {
		t.Fatalf("expected unauthenticated mutation without repository call, got %+v", result)
	}

	sessionErr := errors.New("session failed")
	service = NewDefenseService(&fakeSessionLookup{err: sessionErr}, &fakeDefenseRepository{})
	if _, err := service.MutateDefense(context.Background(), DefenseMutationCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected session error, got %v", err)
	}

	repoErr := errors.New("mutation failed")
	service = NewDefenseService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeDefenseRepository{mutationErr: repoErr})
	if _, err := service.MutateDefense(context.Background(), DefenseMutationCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected mutation error, got %v", err)
	}

	readErr := errors.New("refresh failed")
	service = NewDefenseService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeDefenseRepository{err: readErr})
	if _, err := service.MutateDefense(context.Background(), DefenseMutationCommand{}); !errors.Is(err, readErr) {
		t.Fatalf("expected refresh error, got %v", err)
	}
}

func TestDefenseServicePropagatesErrors(t *testing.T) {
	sessionErr := errors.New("session failed")
	service := NewDefenseService(&fakeSessionLookup{err: sessionErr}, &fakeDefenseRepository{})
	if _, err := service.GetDefense(context.Background(), DefenseCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected session error, got %v", err)
	}

	repoErr := errors.New("repository failed")
	service = NewDefenseService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeDefenseRepository{err: repoErr})
	if _, err := service.GetDefense(context.Background(), DefenseCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestDefenseServiceRequiresDependencies(t *testing.T) {
	if _, err := (DefenseService{}).GetDefense(context.Background(), DefenseCommand{}); err == nil {
		t.Fatal("expected dependency error")
	}
}

type fakeDefenseRepository struct {
	result          domaingame.Defense
	mutationOutcome DefenseMutationOutcome
	err             error
	mutationErr     error
	query           DefenseQuery
	mutation        DefenseMutationQuery
	called          bool
}

func (f *fakeDefenseRepository) GetDefense(_ context.Context, query DefenseQuery) (domaingame.Defense, error) {
	f.query = query
	f.called = true
	return f.result, f.err
}

func (f *fakeDefenseRepository) MutateDefense(_ context.Context, query DefenseMutationQuery) (DefenseMutationOutcome, error) {
	f.mutation = query
	return f.mutationOutcome, f.mutationErr
}
