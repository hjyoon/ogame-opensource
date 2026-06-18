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
	result domaingame.Defense
	err    error
	query  DefenseQuery
	called bool
}

func (f *fakeDefenseRepository) GetDefense(_ context.Context, query DefenseQuery) (domaingame.Defense, error) {
	f.query = query
	f.called = true
	return f.result, f.err
}
