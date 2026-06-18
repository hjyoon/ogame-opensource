package game

import (
	"context"
	"errors"
	"testing"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestOverviewServiceReturnsOverviewForAuthenticatedSession(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeOverviewRepository{overview: domaingame.Overview{
		Commander:     "legor",
		CurrentPlanet: domaingame.PlanetOverview{ID: 99, Name: "Arakis"},
	}}
	service := NewOverviewService(sessions, repository)

	result, err := service.GetOverview(context.Background(), OverviewCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"prsess_42_1": "private"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
	})

	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Overview.CurrentPlanet.Name != "Arakis" {
		t.Fatalf("expected overview result, got %+v", result)
	}
	if sessions.command.PublicSession != "public" || sessions.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}
	if repository.query.PlayerID != 42 || repository.query.PlanetID != 99 {
		t.Fatalf("unexpected repository query: %+v", repository.query)
	}
}

func TestOverviewServiceReturnsSessionIssues(t *testing.T) {
	service := NewOverviewService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}, &fakeOverviewRepository{})

	result, err := service.GetOverview(context.Background(), OverviewCommand{PublicSession: "public"})

	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || result.Issues[0].Code != domainpublicsite.SessionIssuePrivateInvalid {
		t.Fatalf("expected session issue result, got %+v", result)
	}
}

func TestOverviewServiceReturnsSessionError(t *testing.T) {
	wantErr := errors.New("session failed")
	service := NewOverviewService(&fakeSessionLookup{err: wantErr}, &fakeOverviewRepository{})

	_, err := service.GetOverview(context.Background(), OverviewCommand{PublicSession: "public"})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected session error, got %v", err)
	}
}

func TestOverviewServiceReturnsRepositoryError(t *testing.T) {
	wantErr := errors.New("overview failed")
	service := NewOverviewService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeOverviewRepository{err: wantErr})

	_, err := service.GetOverview(context.Background(), OverviewCommand{PublicSession: "public"})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestOverviewServiceRequiresDependencies(t *testing.T) {
	_, err := (OverviewService{}).GetOverview(context.Background(), OverviewCommand{PublicSession: "public"})

	if err == nil {
		t.Fatal("expected missing dependency error")
	}
}

func TestOverviewServiceRenamesPlanetForAuthenticatedSession(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeOverviewRepository{overview: domaingame.Overview{
		Commander:     "legor",
		CurrentPlanet: domaingame.PlanetOverview{ID: 99, Name: "New Colony"},
	}}
	service := NewOverviewService(sessions, repository)

	result, err := service.RenamePlanet(context.Background(), OverviewRenameCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"prsess_42_1": "private"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
		Name:            "New Colony",
	})

	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Overview.CurrentPlanet.Name != "New Colony" {
		t.Fatalf("expected renamed overview result, got %+v", result)
	}
	if repository.renameQuery.PlayerID != 42 || repository.renameQuery.PlanetID != 99 || repository.renameQuery.Name != "New Colony" {
		t.Fatalf("unexpected rename query: %+v", repository.renameQuery)
	}
	if sessions.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}
}

func TestOverviewServiceRenameReturnsSessionIssues(t *testing.T) {
	service := NewOverviewService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}, &fakeOverviewRepository{})

	result, err := service.RenamePlanet(context.Background(), OverviewRenameCommand{PublicSession: "public"})

	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || result.Issues[0].Code != domainpublicsite.SessionIssuePrivateInvalid {
		t.Fatalf("expected session issue result, got %+v", result)
	}
}

func TestOverviewServiceRenameErrors(t *testing.T) {
	if _, err := (OverviewService{}).RenamePlanet(context.Background(), OverviewRenameCommand{}); err == nil {
		t.Fatal("expected missing dependency error")
	}
	wantErr := errors.New("session failed")
	if _, err := NewOverviewService(&fakeSessionLookup{err: wantErr}, &fakeOverviewRepository{}).RenamePlanet(context.Background(), OverviewRenameCommand{}); !errors.Is(err, wantErr) {
		t.Fatalf("expected session error, got %v", err)
	}
	wantErr = errors.New("rename failed")
	service := NewOverviewService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeOverviewRepository{err: wantErr})
	if _, err := service.RenamePlanet(context.Background(), OverviewRenameCommand{}); !errors.Is(err, wantErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestOverviewServiceDeletesPlanetForAuthenticatedSession(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeOverviewRepository{overview: domaingame.Overview{
		Commander:     "legor",
		CurrentPlanet: domaingame.PlanetOverview{ID: 1, Name: "Homeworld"},
	}}
	service := NewOverviewService(sessions, repository)

	result, err := service.DeletePlanet(context.Background(), OverviewDeleteCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"prsess_42_1": "private"},
		RemoteAddr:      "203.0.113.10",
		PlanetID:        99,
		DeleteID:        99,
		Password:        "admin",
	})

	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Overview.CurrentPlanet.Name != "Homeworld" {
		t.Fatalf("expected delete overview result, got %+v", result)
	}
	if repository.deleteQuery.PlayerID != 42 || repository.deleteQuery.PlanetID != 99 ||
		repository.deleteQuery.DeleteID != 99 || repository.deleteQuery.Password != "admin" {
		t.Fatalf("unexpected delete query: %+v", repository.deleteQuery)
	}
	if sessions.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected session command: %+v", sessions.command)
	}
}

func TestOverviewServiceDeleteReturnsActionIssue(t *testing.T) {
	wantIssue := &domaingame.OverviewActionIssue{Code: domaingame.OverviewIssueHomePlanet, Message: "You can't abandon the home planet!"}
	service := NewOverviewService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeOverviewRepository{overview: domaingame.Overview{CurrentPlanet: domaingame.PlanetOverview{ID: 1}}, actionIssue: wantIssue})

	result, err := service.DeletePlanet(context.Background(), OverviewDeleteCommand{PublicSession: "public", DeleteID: 1})

	if err != nil {
		t.Fatal(err)
	}
	if result.ActionIssue == nil || result.ActionIssue.Code != domaingame.OverviewIssueHomePlanet {
		t.Fatalf("expected action issue result, got %+v", result)
	}
}

func TestOverviewServiceDeleteReturnsSessionIssues(t *testing.T) {
	service := NewOverviewService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}, &fakeOverviewRepository{})

	result, err := service.DeletePlanet(context.Background(), OverviewDeleteCommand{PublicSession: "public"})

	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || result.Issues[0].Code != domainpublicsite.SessionIssuePrivateInvalid {
		t.Fatalf("expected session issue result, got %+v", result)
	}
}

func TestOverviewServiceDeleteErrors(t *testing.T) {
	if _, err := (OverviewService{}).DeletePlanet(context.Background(), OverviewDeleteCommand{}); err == nil {
		t.Fatal("expected missing dependency error")
	}
	wantErr := errors.New("session failed")
	if _, err := NewOverviewService(&fakeSessionLookup{err: wantErr}, &fakeOverviewRepository{}).DeletePlanet(context.Background(), OverviewDeleteCommand{}); !errors.Is(err, wantErr) {
		t.Fatalf("expected session error, got %v", err)
	}
	wantErr = errors.New("delete failed")
	service := NewOverviewService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeOverviewRepository{err: wantErr})
	if _, err := service.DeletePlanet(context.Background(), OverviewDeleteCommand{}); !errors.Is(err, wantErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

type fakeSessionLookup struct {
	result  domainpublicsite.SessionAuthentication
	err     error
	command apppublicsite.GameSessionCommand
}

func (f *fakeSessionLookup) GetGameSession(_ context.Context, command apppublicsite.GameSessionCommand) (domainpublicsite.SessionAuthentication, error) {
	f.command = command
	return f.result, f.err
}

type fakeOverviewRepository struct {
	overview    domaingame.Overview
	err         error
	actionIssue *domaingame.OverviewActionIssue
	query       OverviewQuery
	renameQuery OverviewRenameQuery
	deleteQuery OverviewDeleteQuery
}

func (f *fakeOverviewRepository) GetOverview(_ context.Context, query OverviewQuery) (domaingame.Overview, error) {
	f.query = query
	return f.overview, f.err
}

func (f *fakeOverviewRepository) RenamePlanet(_ context.Context, query OverviewRenameQuery) (domaingame.Overview, error) {
	f.renameQuery = query
	return f.overview, f.err
}

func (f *fakeOverviewRepository) DeletePlanet(_ context.Context, query OverviewDeleteQuery) (domaingame.Overview, *domaingame.OverviewActionIssue, error) {
	f.deleteQuery = query
	return f.overview, f.actionIssue, f.err
}
