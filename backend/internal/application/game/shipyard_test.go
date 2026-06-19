package game

import (
	"context"
	"errors"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestShipyardServiceReturnsAuthenticatedShipyard(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session: domainpublicsite.GameSession{
			PlayerID: 42,
		},
	}}
	repository := &fakeShipyardRepository{result: domaingame.Shipyard{Commander: "legor"}}
	service := NewShipyardService(sessions, repository)

	result, err := service.GetShipyard(context.Background(), ShipyardCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "secret"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Shipyard.Commander != "legor" {
		t.Fatalf("unexpected shipyard result: %+v", result)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 99 {
		t.Fatalf("unexpected repository query: %+v", repository.query)
	}
	if sessions.command.PublicSession != "public" || sessions.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}
}

func TestShipyardServiceReturnsSessionIssuesWithoutRepository(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	repository := &fakeShipyardRepository{}
	service := NewShipyardService(sessions, repository)

	result, err := service.GetShipyard(context.Background(), ShipyardCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.called {
		t.Fatalf("expected unauthenticated result without repository call, got %+v", result)
	}
}

func TestShipyardServiceMutatesAuthenticatedOrders(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	issue := domaingame.BuildingActionIssue(domaingame.BuildingsIssueNoResources)
	repository := &fakeShipyardRepository{
		result:          domaingame.Shipyard{Commander: "legor"},
		mutationOutcome: ShipyardMutationOutcome{ActionIssue: issue},
	}
	service := NewShipyardService(sessions, repository)

	result, err := service.MutateShipyard(context.Background(), ShipyardMutationCommand{
		PublicSession: "public",
		PlanetID:      99,
		Orders:        map[int]int{domaingame.FleetLightFighter: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Shipyard.Commander != "legor" || result.ActionIssue != issue {
		t.Fatalf("unexpected mutation result: %+v", result)
	}
	if repository.mutation.PlayerID != 42 || repository.mutation.PlanetID != 99 || repository.mutation.Orders[domaingame.FleetLightFighter] != 3 {
		t.Fatalf("unexpected mutation query: %+v", repository.mutation)
	}
	if !repository.called {
		t.Fatal("expected read model refresh after mutation")
	}
}

func TestShipyardServiceMutateHandlesSessionIssuesAndErrors(t *testing.T) {
	if _, err := (ShipyardService{}).MutateShipyard(context.Background(), ShipyardMutationCommand{}); err == nil {
		t.Fatal("expected dependency error")
	}

	repository := &fakeShipyardRepository{}
	service := NewShipyardService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues:        []domainpublicsite.SessionIssue{{Code: domainpublicsite.SessionIssuePrivateInvalid}},
	}}, repository)
	result, err := service.MutateShipyard(context.Background(), ShipyardMutationCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.mutation.Orders != nil {
		t.Fatalf("expected unauthenticated mutation without repository call, got %+v", result)
	}

	sessionErr := errors.New("session failed")
	service = NewShipyardService(&fakeSessionLookup{err: sessionErr}, &fakeShipyardRepository{})
	if _, err := service.MutateShipyard(context.Background(), ShipyardMutationCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected session error, got %v", err)
	}

	repoErr := errors.New("mutation failed")
	service = NewShipyardService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeShipyardRepository{mutationErr: repoErr})
	if _, err := service.MutateShipyard(context.Background(), ShipyardMutationCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected mutation error, got %v", err)
	}

	readErr := errors.New("refresh failed")
	service = NewShipyardService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeShipyardRepository{err: readErr})
	if _, err := service.MutateShipyard(context.Background(), ShipyardMutationCommand{}); !errors.Is(err, readErr) {
		t.Fatalf("expected refresh error, got %v", err)
	}
}

func TestShipyardServicePropagatesErrors(t *testing.T) {
	sessionErr := errors.New("session failed")
	service := NewShipyardService(&fakeSessionLookup{err: sessionErr}, &fakeShipyardRepository{})
	if _, err := service.GetShipyard(context.Background(), ShipyardCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected session error, got %v", err)
	}

	repoErr := errors.New("repository failed")
	service = NewShipyardService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeShipyardRepository{err: repoErr})
	if _, err := service.GetShipyard(context.Background(), ShipyardCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestShipyardServiceRequiresDependencies(t *testing.T) {
	if _, err := (ShipyardService{}).GetShipyard(context.Background(), ShipyardCommand{}); err == nil {
		t.Fatal("expected dependency error")
	}
}

type fakeShipyardRepository struct {
	result          domaingame.Shipyard
	mutationOutcome ShipyardMutationOutcome
	err             error
	mutationErr     error
	query           ShipyardQuery
	mutation        ShipyardMutationQuery
	called          bool
}

func (f *fakeShipyardRepository) GetShipyard(_ context.Context, query ShipyardQuery) (domaingame.Shipyard, error) {
	f.query = query
	f.called = true
	return f.result, f.err
}

func (f *fakeShipyardRepository) MutateShipyard(_ context.Context, query ShipyardMutationQuery) (ShipyardMutationOutcome, error) {
	f.mutation = query
	return f.mutationOutcome, f.mutationErr
}
