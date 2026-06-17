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
}

func TestBuildingsServicePropagatesErrors(t *testing.T) {
	sessionErr := errors.New("session failed")
	service := NewBuildingsService(&fakeSessionLookup{err: sessionErr}, &fakeBuildingsRepository{})
	if _, err := service.GetBuildings(context.Background(), BuildingsCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected session error, got %v", err)
	}

	repoErr := errors.New("repository failed")
	service = NewBuildingsService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeBuildingsRepository{err: repoErr})
	if _, err := service.GetBuildings(context.Background(), BuildingsCommand{}); !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestBuildingsServiceRequiresDependencies(t *testing.T) {
	if _, err := (BuildingsService{}).GetBuildings(context.Background(), BuildingsCommand{}); err == nil {
		t.Fatal("expected dependency error")
	}
}

type fakeBuildingsRepository struct {
	result domaingame.Buildings
	err    error
	query  BuildingsQuery
	called bool
}

func (f *fakeBuildingsRepository) GetBuildings(_ context.Context, query BuildingsQuery) (domaingame.Buildings, error) {
	f.query = query
	f.called = true
	return f.result, f.err
}
