package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestSearchServiceReturnsSearchForAuthenticatedSession(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeSearchRepository{search: domaingame.Search{Commander: "legor", Type: domaingame.SearchTypePlayerName, Text: "legor"}}
	service := NewSearchService(sessions, repository)

	result, err := service.GetSearch(context.Background(), SearchCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "token"},
		RemoteAddr:      "203.0.113.9",
		PlanetID:        99,
		Type:            "playername",
		Text:            "legor",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Search.Commander != "legor" {
		t.Fatalf("unexpected search result: %+v", result)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 99 || repository.query.Type != "playername" || repository.query.Text != "legor" {
		t.Fatalf("unexpected search query: %+v", repository.query)
	}
	if sessions.command.PublicSession != "public" || sessions.command.RemoteAddr != "203.0.113.9" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}
}

func TestSearchServiceReturnsUnauthenticatedSessionIssues(t *testing.T) {
	issue := domainpublicsite.SessionIssue{Code: "missing", Message: "missing session"}
	service := NewSearchService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Issues: []domainpublicsite.SessionIssue{issue}}}, &fakeSearchRepository{})

	result, err := service.GetSearch(context.Background(), SearchCommand{PublicSession: "bad"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || result.Issues[0].Code != "missing" {
		t.Fatalf("expected unauthenticated issue, got %+v", result)
	}
}

func TestSearchServiceReturnsDependencyAndRepositoryErrors(t *testing.T) {
	if _, err := (SearchService{}).GetSearch(context.Background(), SearchCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected dependency error, got %v", err)
	}
	if _, err := NewSearchService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeSearchRepository{}).GetSearch(context.Background(), SearchCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected session error, got %v", err)
	}
	if _, err := NewSearchService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeSearchRepository{err: errors.New("search failed")}).GetSearch(context.Background(), SearchCommand{}); err == nil || !strings.Contains(err.Error(), "search failed") {
		t.Fatalf("expected repository error, got %v", err)
	}
}

type fakeSearchRepository struct {
	search domaingame.Search
	query  SearchQuery
	err    error
}

func (f *fakeSearchRepository) GetSearch(_ context.Context, query SearchQuery) (domaingame.Search, error) {
	f.query = query
	if f.err != nil {
		return domaingame.Search{}, f.err
	}
	return f.search, nil
}
