package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestBuddyServiceReturnsBuddyForAuthenticatedSession(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeBuddyRepository{buddy: domaingame.Buddy{Commander: "legor", Action: domaingame.BuddyActionIncoming}}
	service := NewBuddyService(sessions, repository)

	result, err := service.GetBuddy(context.Background(), BuddyCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "token"},
		RemoteAddr:      "203.0.113.9",
		PlanetID:        99,
		Action:          5,
		BuddyID:         7,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Buddy.Commander != "legor" || result.Buddy.Action != domaingame.BuddyActionIncoming {
		t.Fatalf("unexpected buddy result: %+v", result)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 99 || repository.query.Action != 5 || repository.query.BuddyID != 7 {
		t.Fatalf("unexpected buddy query: %+v", repository.query)
	}
	if sessions.command.PublicSession != "public" || sessions.command.RemoteAddr != "203.0.113.9" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}
}

func TestBuddyServiceReturnsUnauthenticatedSessionIssues(t *testing.T) {
	issue := domainpublicsite.SessionIssue{Code: "missing", Message: "missing session"}
	service := NewBuddyService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Issues: []domainpublicsite.SessionIssue{issue}}}, &fakeBuddyRepository{})

	result, err := service.GetBuddy(context.Background(), BuddyCommand{PublicSession: "bad"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || result.Issues[0].Code != "missing" {
		t.Fatalf("expected unauthenticated issue, got %+v", result)
	}
}

func TestBuddyServiceReturnsDependencyAndRepositoryErrors(t *testing.T) {
	if _, err := (BuddyService{}).GetBuddy(context.Background(), BuddyCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected dependency error, got %v", err)
	}
	if _, err := NewBuddyService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeBuddyRepository{}).GetBuddy(context.Background(), BuddyCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected session error, got %v", err)
	}
	if _, err := NewBuddyService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeBuddyRepository{err: errors.New("buddy failed")}).GetBuddy(context.Background(), BuddyCommand{}); err == nil || !strings.Contains(err.Error(), "buddy failed") {
		t.Fatalf("expected repository error, got %v", err)
	}
}

type fakeBuddyRepository struct {
	buddy domaingame.Buddy
	query BuddyQuery
	err   error
}

func (f *fakeBuddyRepository) GetBuddy(_ context.Context, query BuddyQuery) (domaingame.Buddy, error) {
	f.query = query
	if f.err != nil {
		return domaingame.Buddy{}, f.err
	}
	return f.buddy, nil
}
