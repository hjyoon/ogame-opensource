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
	result domaingame.Shipyard
	err    error
	query  ShipyardQuery
	called bool
}

func (f *fakeShipyardRepository) GetShipyard(_ context.Context, query ShipyardQuery) (domaingame.Shipyard, error) {
	f.query = query
	f.called = true
	return f.result, f.err
}
