package httpdelivery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestGameAdminHandlerReturnsAdminHome(t *testing.T) {
	usecase := &fakeGameAdminUseCase{result: appgame.AdminResult{
		Authenticated: true,
		Admin: domaingame.NewAdmin(
			domaingame.Overview{Commander: "legor", CurrentPlanet: domaingame.PlanetOverview{ID: 99}, PlanetSwitcher: []domaingame.PlanetSummary{{ID: 99}}},
			domaingame.AdminViewer{PlayerID: 42, Name: "legor", Level: domaingame.AdminLevelAdmin},
			"Users",
		),
	}}
	request := httptest.NewRequest(http.MethodGet, "/api/game/admin?session=pub&cp=99&mode=Users", nil)
	response := httptest.NewRecorder()

	app{deps: Dependencies{GameAdmin: usecase}}.handleGameAdmin(response, request)

	if response.Code != http.StatusOK || usecase.command.PlanetID != 99 || usecase.command.Mode != "Users" {
		t.Fatalf("unexpected response status=%d command=%+v body=%s", response.Code, usecase.command, response.Body.String())
	}
	var payload gameAdminResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Authenticated || payload.Admin == nil || payload.Admin.Viewer.Level != domaingame.AdminLevelAdmin ||
		len(payload.Admin.Menu) != 25 || payload.Admin.Menu[0].Label != "Fleet Logs" {
		t.Fatalf("unexpected admin payload: %+v", payload)
	}
	if toGameAdminActionIssue(nil) != nil {
		t.Fatal("expected nil admin issue conversion")
	}
}

func TestGameAdminHandlerRejectsInvalidAndUnauthenticatedRequests(t *testing.T) {
	response := httptest.NewRecorder()
	app{}.handleGameAdmin(response, httptest.NewRequest(http.MethodGet, "/api/game/admin?session=pub", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected unavailable without dependency, got %d", response.Code)
	}

	usecase := &fakeGameAdminUseCase{result: appgame.AdminResult{
		Authenticated: false,
		Issues:        []domainpublicsite.SessionIssue{{Code: "missing", Message: "Session is invalid."}},
	}}
	response = httptest.NewRecorder()
	app{deps: Dependencies{GameAdmin: usecase}}.handleGameAdmin(response, httptest.NewRequest(http.MethodGet, "/api/game/admin?session=bad", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", response.Code)
	}

	usecase = &fakeGameAdminUseCase{err: context.Canceled}
	response = httptest.NewRecorder()
	app{deps: Dependencies{GameAdmin: usecase}}.handleGameAdmin(response, httptest.NewRequest(http.MethodGet, "/api/game/admin?session=pub", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected usecase error as unavailable, got %d", response.Code)
	}

	response = httptest.NewRecorder()
	app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{}}}.handleGameAdmin(response, httptest.NewRequest(http.MethodGet, "/api/game/admin?session=pub&cp=bad", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid cp bad request, got %d", response.Code)
	}
}

type fakeGameAdminUseCase struct {
	result  appgame.AdminResult
	err     error
	command appgame.AdminCommand
}

func (f *fakeGameAdminUseCase) GetAdmin(_ context.Context, command appgame.AdminCommand) (appgame.AdminResult, error) {
	f.command = command
	return f.result, f.err
}
