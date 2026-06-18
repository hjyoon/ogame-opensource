package game

import (
	"context"
	"errors"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestFleetServiceReturnsAuthenticatedFleet(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeFleetRepository{result: domaingame.Fleet{Commander: "legor"}}
	service := NewFleetService(sessions, repository)

	result, err := service.GetFleet(context.Background(), FleetCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "secret"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Fleet.Commander != "legor" {
		t.Fatalf("unexpected fleet result: %+v", result)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 99 {
		t.Fatalf("unexpected repository query: %+v", repository.query)
	}
	if sessions.command.PublicSession != "public" || sessions.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}
}

func TestFleetServiceReturnsSessionIssuesWithoutRepository(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	repository := &fakeFleetRepository{}
	service := NewFleetService(sessions, repository)

	result, err := service.GetFleet(context.Background(), FleetCommand{PublicSession: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || repository.called {
		t.Fatalf("expected unauthenticated result without repository call, got %+v", result)
	}
}

func TestFleetServicePropagatesErrors(t *testing.T) {
	sessionErr := errors.New("session failed")
	service := NewFleetService(&fakeSessionLookup{err: sessionErr}, &fakeFleetRepository{})
	if _, err := service.GetFleet(context.Background(), FleetCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected session error, got %v", err)
	}

	repoErr := errors.New("repository failed")
	service = NewFleetService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeFleetRepository{err: repoErr})
	if _, err := service.GetFleet(context.Background(), FleetCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestFleetServiceRequiresDependencies(t *testing.T) {
	if _, err := (FleetService{}).GetFleet(context.Background(), FleetCommand{}); err == nil {
		t.Fatal("expected dependency error")
	}
}

type fakeFleetRepository struct {
	result domaingame.Fleet
	err    error
	query  FleetQuery
	called bool
}

func (f *fakeFleetRepository) GetFleet(_ context.Context, query FleetQuery) (domaingame.Fleet, error) {
	f.query = query
	f.called = true
	return f.result, f.err
}
