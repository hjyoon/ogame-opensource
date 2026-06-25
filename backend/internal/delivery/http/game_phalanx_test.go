package httpdelivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/configcatalog"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/filesystem"
)

func TestGamePhalanxEndpointReturnsReport(t *testing.T) {
	overview := domaingame.Overview{
		Commander:     "Legor",
		CurrentPlanet: domaingame.PlanetOverview{ID: 10, Name: "Moon", Type: domaingame.PlanetTypeMoon, Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3}},
		PlanetSwitcher: []domaingame.PlanetSummary{{
			ID:          10,
			Name:        "Moon",
			Type:        domaingame.PlanetTypeMoon,
			Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
			Current:     true,
		}},
	}
	phalanx := domaingame.NewPhalanx(
		overview,
		domaingame.PhalanxPlanet{ID: 10, OwnerID: 42, Name: "Moon", Type: domaingame.PlanetTypeMoon, Coordinates: overview.CurrentPlanet.Coordinates, PhalanxLevel: 3, Deuterium: 20000},
		domaingame.PhalanxPlanet{ID: 20, OwnerID: 77, Name: "Target", Type: domaingame.PlanetTypePlanet, Coordinates: domaingame.Coordinates{Galaxy: 1, System: 4, Position: 5}},
		[]domaingame.FleetMission{
			domaingame.BuildFleetMission(99, domaingame.FleetMissionTransport, domaingame.FleetCounts{domaingame.FleetSmallCargo: 1}, domaingame.Coordinates{Galaxy: 1, System: 4, Position: 5}, domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3}, domaingame.PlanetTypePlanet, "Legor", 1000, 2000),
		},
		nil,
	)
	usecase := &fakeGamePhalanx{result: appgame.PhalanxResult{Authenticated: true, Phalanx: phalanx}}
	server := testServerWithGamePhalanx(t, usecase, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/game/phalanx?session=public&cp=10&spid=20", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gamePhalanxResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Phalanx == nil || response.Phalanx.Source.ID != 10 || response.Phalanx.Target.ID != 20 {
		t.Fatalf("unexpected phalanx response: %+v", response)
	}
	if response.Phalanx.Cost != domaingame.PhalanxCost || response.Phalanx.RemainingDeuterium != 15000 || len(response.Phalanx.Events) != 1 {
		t.Fatalf("unexpected phalanx summary: %+v", response.Phalanx)
	}
	if usecase.command.PublicSession != "public" ||
		usecase.command.PlanetID != 10 ||
		usecase.command.TargetPlanetID != 20 ||
		usecase.command.RemoteAddr != "203.0.113.10" ||
		usecase.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("unexpected phalanx command: %+v", usecase.command)
	}
}

func TestGamePhalanxEndpointReturnsUnauthorized(t *testing.T) {
	usecase := &fakeGamePhalanx{result: appgame.PhalanxResult{
		Authenticated: false,
		Issues:        []domainpublicsite.SessionIssue{{Code: "missing", Message: "missing session"}},
	}}
	server := testServerWithGamePhalanx(t, usecase, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/game/phalanx?session=public&cp=10&targetPlanetId=20", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gamePhalanxResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Phalanx != nil || len(response.Issues) != 1 {
		t.Fatalf("unexpected unauthenticated phalanx response: %+v", response)
	}
}

func TestGamePhalanxActionIssueMapping(t *testing.T) {
	issue := toGamePhalanxActionIssue(&domaingame.PhalanxActionIssue{Code: "missing_sensor", Message: "No cheating!"})
	if issue == nil || issue.Code != "missing_sensor" || issue.Message != "No cheating!" {
		t.Fatalf("unexpected action issue mapping: %+v", issue)
	}
}

func TestGamePhalanxEndpointRejectsInvalidSelection(t *testing.T) {
	for _, target := range []string{
		"/api/game/phalanx?session=public&cp=bad&spid=20",
		"/api/game/phalanx?session=public&cp=10&spid=bad",
		"/api/game/phalanx?session=public&cp=10",
	} {
		server := testServerWithGamePhalanx(t, &fakeGamePhalanx{}, nil)
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", target, rec.Code)
		}
	}
}

func TestGamePhalanxEndpointReturnsUnavailableAndLogsErrors(t *testing.T) {
	server := testServerWithGamePhalanx(t, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/game/phalanx?session=public&cp=10&spid=20", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing usecase to return 503, got %d", rec.Code)
	}

	logs := bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	server = testServerWithGamePhalanx(t, &fakeGamePhalanx{err: errors.New("repository failed")}, logger)
	req = httptest.NewRequest(http.MethodGet, "/api/game/phalanx?session=public&cp=10&spid=20", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || !bytes.Contains(logs.Bytes(), []byte("repository failed")) {
		t.Fatalf("expected logged 503, code=%d logs=%s", rec.Code, logs.String())
	}
}

func testServerWithGamePhalanx(t *testing.T, phalanx GamePhalanxUseCase, logger *slog.Logger) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GamePhalanx:        phalanx,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
		Logger:             logger,
	})
}

type fakeGamePhalanx struct {
	result  appgame.PhalanxResult
	command appgame.PhalanxCommand
	err     error
}

func (f *fakeGamePhalanx) GetPhalanx(_ context.Context, command appgame.PhalanxCommand) (appgame.PhalanxResult, error) {
	f.command = command
	if f.err != nil {
		return appgame.PhalanxResult{}, f.err
	}
	return f.result, nil
}
