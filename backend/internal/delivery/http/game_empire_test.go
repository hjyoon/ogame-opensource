package httpdelivery

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	"github.com/hjyoon/ogame-opensource/backend/internal/config"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/configcatalog"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/filesystem"
)

func TestGameEmpireEndpointReturnsEmpire(t *testing.T) {
	empireUseCase := &fakeGameEmpire{result: appgame.EmpireResult{
		Authenticated: true,
		Empire:        sampleGameEmpire(),
		ActionIssue:   domaingame.EmpireActionIssueFor(domaingame.EmpireIssueCommanderRequired),
	}}
	server := testServerWithGameEmpire(t, empireUseCase)
	req := httptest.NewRequest(http.MethodGet, "/api/game/empire?session=public&cp=99&planettype=3", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response gameEmpireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Empire == nil || response.Empire.Commander != "legor" ||
		len(response.Empire.Planets) != 1 || response.Empire.Resources[0].Total != 1234 ||
		len(response.Empire.Buildings[0].Values[0].Queue) != 1 || !response.Empire.Buildings[0].Values[0].Queue[0].Active ||
		response.ActionIssue == nil || response.ActionIssue.Code != domaingame.EmpireIssueCommanderRequired {
		t.Fatalf("unexpected empire response: %+v", response)
	}
	if empireUseCase.command.PublicSession != "public" || empireUseCase.command.PlanetID != 99 ||
		empireUseCase.command.PlanetType != domaingame.EmpirePlanetTypeMoons ||
		empireUseCase.command.RemoteAddr != "203.0.113.10" ||
		empireUseCase.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("unexpected empire command: %+v", empireUseCase.command)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/game/empire?session=public&planettype=999", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || empireUseCase.command.PlanetType != domaingame.EmpirePlanetTypePlanets {
		t.Fatalf("expected unknown planettype to normalize to planets, code=%d command=%+v", rec.Code, empireUseCase.command)
	}
}

func TestGameEmpireEndpointAppliesLegacyShortcut(t *testing.T) {
	empireUseCase := &fakeGameEmpire{mutationResult: appgame.EmpireResult{
		Authenticated: true,
		Empire:        sampleGameEmpire(),
		ActionIssue:   &domaingame.EmpireActionIssue{Code: domaingame.BuildingsIssueNoResources, Message: "Not enough resources."},
	}}
	server := testServerWithGameEmpire(t, empireUseCase)
	req := httptest.NewRequest(http.MethodGet, "/api/game/empire?session=public&cp=99&planettype=3&modus=add&planet=100&techid=1", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response gameEmpireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ActionIssue == nil || response.ActionIssue.Code != domaingame.BuildingsIssueNoResources {
		t.Fatalf("expected building issue to be returned, got %+v", response)
	}
	if empireUseCase.mutationCommand.PublicSession != "public" ||
		empireUseCase.mutationCommand.PlanetID != 99 ||
		empireUseCase.mutationCommand.PlanetType != domaingame.EmpirePlanetTypeMoons ||
		empireUseCase.mutationCommand.TargetPlanetID != 100 ||
		empireUseCase.mutationCommand.Action != domaingame.BuildingsMutationAdd ||
		empireUseCase.mutationCommand.TechID != domaingame.BuildingMetalMine ||
		empireUseCase.mutationCommand.RemoteAddr != "203.0.113.10" ||
		empireUseCase.mutationCommand.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("unexpected empire mutation command: %+v", empireUseCase.mutationCommand)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/game/empire?session=public&cp=99&modus=remove&listid=2", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK ||
		empireUseCase.mutationCommand.TargetPlanetID != 99 ||
		empireUseCase.mutationCommand.Action != domaingame.BuildingsMutationRemove ||
		empireUseCase.mutationCommand.ListID != 2 {
		t.Fatalf("expected missing shortcut planet to fall back to cp, code=%d command=%+v", rec.Code, empireUseCase.mutationCommand)
	}
}

func TestGameEmpireEndpointReturnsUnauthorizedAndErrors(t *testing.T) {
	empireUseCase := &fakeGameEmpire{result: appgame.EmpireResult{
		Authenticated: false,
		Issues:        []domainpublicsite.SessionIssue{{Code: "missing", Message: "missing session"}},
	}}
	server := testServerWithGameEmpire(t, empireUseCase)
	req := httptest.NewRequest(http.MethodGet, "/api/game/empire?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/game/empire?session=public&cp=bad", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid selected planet 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/game/empire?session=public&planettype=bad", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid planet type 400, got %d", rec.Code)
	}

	server = testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req = httptest.NewRequest(http.MethodGet, "/api/game/empire?session=public", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing use case 503, got %d", rec.Code)
	}

	server = testServerWithGameEmpire(t, &fakeGameEmpire{err: errors.New("empire failed")})
	req = httptest.NewRequest(http.MethodGet, "/api/game/empire?session=public", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected use case error 503, got %d", rec.Code)
	}
}

func testServerWithGameEmpire(t *testing.T, empire GameEmpireUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GameEmpire:         empire,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

type fakeGameEmpire struct {
	result          appgame.EmpireResult
	mutationResult  appgame.EmpireResult
	err             error
	mutationErr     error
	command         appgame.EmpireCommand
	mutationCommand appgame.EmpireMutationCommand
}

func (f *fakeGameEmpire) GetEmpire(_ context.Context, command appgame.EmpireCommand) (appgame.EmpireResult, error) {
	f.command = command
	return f.result, f.err
}

func (f *fakeGameEmpire) MutateEmpire(_ context.Context, command appgame.EmpireMutationCommand) (appgame.EmpireResult, error) {
	f.mutationCommand = command
	if f.mutationResult.Authenticated || f.mutationResult.Empire.Commander != "" || len(f.mutationResult.Issues) > 0 || f.mutationResult.ActionIssue != nil {
		return f.mutationResult, f.mutationErr
	}
	return f.result, f.mutationErr
}

func sampleGameEmpire() domaingame.Empire {
	return domaingame.Empire{
		Commander:       "legor",
		CommanderActive: true,
		CurrentPlanet: domaingame.PlanetOverview{
			ID:          99,
			Name:        "Arakis",
			Type:        domaingame.PlanetTypePlanet,
			Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
		},
		PlanetSwitcher: []domaingame.PlanetSummary{{
			ID:          99,
			Name:        "Arakis",
			Type:        domaingame.PlanetTypePlanet,
			Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
			Current:     true,
		}},
		PlanetType:  domaingame.EmpirePlanetTypePlanets,
		MoonEnabled: true,
		HasMoons:    true,
		Planets: []domaingame.EmpirePlanet{{
			ID:          99,
			Name:        "Arakis",
			Type:        domaingame.PlanetTypePlanet,
			Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
			Fields:      4,
			MaxFields:   163,
		}},
		Resources: []domaingame.EmpireResourceRow{{
			ID:     domaingame.ResourceMetal,
			Name:   "Metal",
			Values: []domaingame.EmpireResourceValue{{PlanetID: 99, Amount: 1234, Production: 42}},
			Total:  1234,
		}},
		Buildings: []domaingame.EmpireLevelRow{{ID: domaingame.BuildingMetalMine, Name: "Metal Mine", Values: []domaingame.EmpireLevelValue{{PlanetID: 99, Level: 12, Queue: []domaingame.EmpireBuildQueueEntry{{ListID: 1, Level: 13, Active: true}}}}, Total: 12, Average: 12}},
		Research:  []domaingame.EmpireLevelRow{{ID: domaingame.ResearchComputer, Name: "Computer Technology", Values: []domaingame.EmpireLevelValue{{PlanetID: 99, Level: 3}}, Total: 3, Average: 3}},
		Fleet:     []domaingame.EmpireCountRow{{ID: domaingame.FleetSmallCargo, Name: "Small Cargo", Values: []domaingame.EmpireCountValue{{PlanetID: 99, Count: 5}}, Total: 5}},
		Defense:   []domaingame.EmpireCountRow{{ID: domaingame.DefenseRocketLauncher, Name: "Rocket Launcher", Values: []domaingame.EmpireCountValue{{PlanetID: 99, Count: 7}}, Total: 7}},
	}
}
