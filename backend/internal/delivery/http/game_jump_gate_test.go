package httpdelivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/configcatalog"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/filesystem"
)

func TestGameJumpGateEndpointReturnsScreen(t *testing.T) {
	usecase := &fakeGameJumpGate{result: appgame.JumpGateResult{Authenticated: true, JumpGate: jumpGateFixture(nil)}}
	server := testServerWithGameJumpGate(t, usecase, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/game/jump-gate?session=public&cp=10", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameJumpGateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.JumpGate == nil || response.JumpGate.Source.ID != 10 || len(response.JumpGate.Targets) != 1 || response.JumpGate.Ships[0].ID != domaingame.FleetSmallCargo {
		t.Fatalf("unexpected jump gate response: %+v", response)
	}
	if usecase.command.PublicSession != "public" || usecase.command.PlanetID != 10 || usecase.command.PrivateSessions["prsess_42_1"] != "private" || usecase.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected command: %+v", usecase.command)
	}
}

func TestGameJumpGateEndpointPostsMutation(t *testing.T) {
	usecase := &fakeGameJumpGate{result: appgame.JumpGateResult{Authenticated: true, JumpGate: jumpGateFixture(domaingame.JumpGateMovedIssue())}}
	server := testServerWithGameJumpGate(t, usecase, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/game/jump-gate?session=public&cp=10", strings.NewReader(`{"sourceMoonId":10,"targetMoonId":20,"ships":{"202":2}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if usecase.mutation.SourceMoonID != 10 || usecase.mutation.TargetMoonID != 20 || usecase.mutation.Ships[domaingame.FleetSmallCargo] != 2 {
		t.Fatalf("unexpected mutation command: %+v", usecase.mutation)
	}
}

func TestLegacyJumpGatePostRedirectsOnMovedAndRendersErrors(t *testing.T) {
	usecase := &fakeGameJumpGate{result: appgame.JumpGateResult{Authenticated: true, JumpGate: jumpGateFixture(domaingame.JumpGateMovedIssue())}}
	server := testServerWithGameJumpGate(t, usecase, nil)
	form := "qm=10&zm=20&c202=2&c212=3"
	req := httptest.NewRequest(http.MethodPost, "/game/index.php?page=sprungtor&session=public&cp=10", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || !strings.Contains(rec.Header().Get("Location"), "page=infos") || !strings.Contains(rec.Header().Get("Location"), "cp=20") || usecase.mutation.Ships[domaingame.FleetSmallCargo] != 2 {
		t.Fatalf("unexpected legacy redirect code=%d location=%s mutation=%+v", rec.Code, rec.Header().Get("Location"), usecase.mutation)
	}

	usecase = &fakeGameJumpGate{result: appgame.JumpGateResult{Authenticated: true, JumpGate: jumpGateFixture(&domaingame.JumpGateActionIssue{Code: domaingame.JumpGateIssueNoShips, Message: "no ships selected"})}}
	server = testServerWithGameJumpGate(t, usecase, nil)
	req = httptest.NewRequest(http.MethodPost, "/game/index.php?page=sprungtor&session=public&cp=10", strings.NewReader("qm=10&zm=20"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "no ships selected") {
		t.Fatalf("expected legacy error body, code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGameJumpGateEndpointUnauthorizedInvalidAndUnavailable(t *testing.T) {
	server := testServerWithGameJumpGate(t, &fakeGameJumpGate{result: appgame.JumpGateResult{
		Authenticated: false,
		Issues:        []domainpublicsite.SessionIssue{{Code: "missing", Message: "missing session"}},
	}}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/game/jump-gate?session=public&cp=10", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/game/jump-gate?session=public&cp=bad", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid cp 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/jump-gate?session=public&cp=10", strings.NewReader("{"))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid JSON 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/jump-gate?session=public&cp=bad", strings.NewReader(`{"sourceMoonId":10}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid POST cp 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/game/jump-gate?session=public&cp=10", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected method 405, got %d", rec.Code)
	}

	server = testServerWithGameJumpGate(t, nil, nil)
	req = httptest.NewRequest(http.MethodGet, "/api/game/jump-gate?session=public&cp=10", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing usecase 503, got %d", rec.Code)
	}

	logs := bytes.Buffer{}
	server = testServerWithGameJumpGate(t, &fakeGameJumpGate{err: errors.New("repository failed")}, slog.New(slog.NewJSONHandler(&logs, nil)))
	req = httptest.NewRequest(http.MethodGet, "/api/game/jump-gate?session=public&cp=10", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || !bytes.Contains(logs.Bytes(), []byte("repository failed")) {
		t.Fatalf("expected logged 503, code=%d logs=%s", rec.Code, logs.String())
	}

	logs.Reset()
	req = httptest.NewRequest(http.MethodPost, "/api/game/jump-gate?session=public&cp=10", strings.NewReader(`{"sourceMoonId":10,"targetMoonId":20}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || !bytes.Contains(logs.Bytes(), []byte("repository failed")) {
		t.Fatalf("expected logged POST 503, code=%d logs=%s", rec.Code, logs.String())
	}
}

func TestLegacyJumpGatePostRejectsUnavailableUnauthenticatedAndErrors(t *testing.T) {
	server := testServerWithGameJumpGate(t, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/game/index.php?page=sprungtor&session=public&cp=10", strings.NewReader("qm=10&zm=20"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing legacy usecase 503, got %d", rec.Code)
	}

	server = testServerWithGameJumpGate(t, &fakeGameJumpGate{result: appgame.JumpGateResult{Authenticated: false}}, nil)
	req = httptest.NewRequest(http.MethodPost, "/game/index.php?page=sprungtor&session=public&cp=10", strings.NewReader("qm=10&zm=20"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected unauthenticated legacy 403, got %d", rec.Code)
	}

	logs := bytes.Buffer{}
	server = testServerWithGameJumpGate(t, &fakeGameJumpGate{err: errors.New("legacy failed")}, slog.New(slog.NewJSONHandler(&logs, nil)))
	req = httptest.NewRequest(http.MethodPost, "/game/index.php?page=sprungtor&session=public&cp=10", strings.NewReader("qm=bad&zm=20&c202=bad"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || !bytes.Contains(logs.Bytes(), []byte("legacy failed")) {
		t.Fatalf("expected legacy logged 503, code=%d logs=%s", rec.Code, logs.String())
	}
}

func jumpGateFixture(issue *domaingame.JumpGateActionIssue) domaingame.JumpGate {
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
	return domaingame.BuildJumpGate(
		overview,
		domaingame.JumpGateMoon{ID: 10, OwnerID: 42, Name: "Moon", Type: domaingame.PlanetTypeMoon, Coordinates: overview.CurrentPlanet.Coordinates, GateLevel: 1, Ships: domaingame.FleetCounts{domaingame.FleetSmallCargo: 4}},
		[]domaingame.JumpGateMoon{{ID: 20, OwnerID: 42, Name: "Target", Type: domaingame.PlanetTypeMoon, Coordinates: domaingame.Coordinates{Galaxy: 1, System: 3, Position: 4}, GateLevel: 1}},
		issue,
	)
}

func testServerWithGameJumpGate(t *testing.T, jumpGate GameJumpGateUseCase, logger *slog.Logger) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GameJumpGate:       jumpGate,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
		Logger:             logger,
	})
}

type fakeGameJumpGate struct {
	result   appgame.JumpGateResult
	command  appgame.JumpGateCommand
	mutation appgame.JumpGateMutationCommand
	err      error
}

func (f *fakeGameJumpGate) GetJumpGate(_ context.Context, command appgame.JumpGateCommand) (appgame.JumpGateResult, error) {
	f.command = command
	if f.err != nil {
		return appgame.JumpGateResult{}, f.err
	}
	return f.result, nil
}

func (f *fakeGameJumpGate) Jump(_ context.Context, command appgame.JumpGateMutationCommand) (appgame.JumpGateResult, error) {
	f.mutation = command
	if f.err != nil {
		return appgame.JumpGateResult{}, f.err
	}
	return f.result, nil
}
