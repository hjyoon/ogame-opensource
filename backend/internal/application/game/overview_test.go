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
	overview domaingame.Overview
	err      error
	query    OverviewQuery
}

func (f *fakeOverviewRepository) GetOverview(_ context.Context, query OverviewQuery) (domaingame.Overview, error) {
	f.query = query
	return f.overview, f.err
}
