package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestOptionsServiceReturnsOptionsForAuthenticatedSession(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeOptionsRepository{options: domaingame.Options{User: domaingame.OptionsUser{Name: "Legor"}}}
	service := NewOptionsService(sessions, repository)

	result, err := service.GetOptions(context.Background(), OptionsCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "token"},
		RemoteAddr:      "203.0.113.9",
		PlanetID:        99,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Options.User.Name != "Legor" ||
		repository.query.PlayerID != 42 || repository.query.PlanetID != 99 ||
		sessions.command.PublicSession != "public" {
		t.Fatalf("unexpected options result/query: result=%+v query=%+v session=%+v", result, repository.query, sessions.command)
	}
}

func TestOptionsServiceUpdatesOptionsForAuthenticatedSession(t *testing.T) {
	issue := domaingame.OptionsSavedIssue()
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeOptionsRepository{options: domaingame.Options{Settings: domaingame.OptionsSettings{MaxSpy: 7}}, issue: issue}
	service := NewOptionsService(sessions, repository)

	result, err := service.UpdateOptions(context.Background(), OptionsUpdateCommand{
		PublicSession: "public",
		PlanetID:      99,
		Mutation:      domaingame.OptionsMutation{MaxSpy: 7},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.ActionIssue.Code != domaingame.OptionsIssueSaved ||
		repository.updateQuery.PlayerID != 42 || repository.updateQuery.Mutation.MaxSpy != 7 {
		t.Fatalf("unexpected update result/query: result=%+v query=%+v", result, repository.updateQuery)
	}
}

func TestOptionsServiceReturnsUnauthenticatedAndErrors(t *testing.T) {
	issue := domainpublicsite.SessionIssue{Code: "missing", Message: "missing session"}
	service := NewOptionsService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Issues: []domainpublicsite.SessionIssue{issue}}}, &fakeOptionsRepository{})
	result, err := service.GetOptions(context.Background(), OptionsCommand{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 {
		t.Fatalf("expected unauthenticated result, got %+v", result)
	}
	result, err = service.UpdateOptions(context.Background(), OptionsUpdateCommand{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 {
		t.Fatalf("expected unauthenticated update result, got %+v", result)
	}

	if _, err := (OptionsService{}).GetOptions(context.Background(), OptionsCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected dependency error, got %v", err)
	}
	if _, err := (OptionsService{}).UpdateOptions(context.Background(), OptionsUpdateCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected update dependency error, got %v", err)
	}
	if _, err := NewOptionsService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeOptionsRepository{}).GetOptions(context.Background(), OptionsCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected get session error, got %v", err)
	}
	if _, err := NewOptionsService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeOptionsRepository{}).UpdateOptions(context.Background(), OptionsUpdateCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected session error, got %v", err)
	}
	if _, err := NewOptionsService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeOptionsRepository{err: errors.New("options failed")}).GetOptions(context.Background(), OptionsCommand{}); err == nil || !strings.Contains(err.Error(), "options failed") {
		t.Fatalf("expected repository error, got %v", err)
	}
	if _, err := NewOptionsService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeOptionsRepository{err: errors.New("update failed")}).UpdateOptions(context.Background(), OptionsUpdateCommand{}); err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("expected update repository error, got %v", err)
	}
}

type fakeOptionsRepository struct {
	options     domaingame.Options
	issue       *domaingame.OptionsActionIssue
	err         error
	query       OptionsQuery
	updateQuery OptionsUpdateQuery
}

func (f *fakeOptionsRepository) GetOptions(_ context.Context, query OptionsQuery) (domaingame.Options, error) {
	f.query = query
	return f.options, f.err
}

func (f *fakeOptionsRepository) UpdateOptions(_ context.Context, query OptionsUpdateQuery) (domaingame.Options, *domaingame.OptionsActionIssue, error) {
	f.updateQuery = query
	return f.options, f.issue, f.err
}
