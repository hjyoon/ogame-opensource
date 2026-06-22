package httpdelivery

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestHandleGameOfficersGetWritesAuthenticatedSummary(t *testing.T) {
	useCase := &fakeGameOfficersUseCase{
		getResult: appgame.OfficersResult{Authenticated: true, Officers: sampleGameOfficers()},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/game/officers?session=public&cp=99", nil)
	request.RemoteAddr = "203.0.113.1:7000"
	request.AddCookie(&http.Cookie{Name: "private", Value: "token"})

	app{deps: Dependencies{GameOfficers: useCase}}.handleGameOfficers(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response gameOfficersResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Authenticated || response.Officers == nil {
		t.Fatalf("expected authenticated officers response, got %+v", response)
	}
	if response.Officers.Commander != "legor" || response.Officers.CurrentPlanet.ID != 99 ||
		response.Officers.User.PaidDarkMatter != 4000 || response.Officers.Rows[0].Name != "Commander" {
		t.Fatalf("unexpected mapped officers: %+v", response.Officers)
	}
	if useCase.getCommand.PublicSession != "public" || useCase.getCommand.PlanetID != 99 ||
		useCase.getCommand.RemoteAddr != "203.0.113.1" || useCase.getCommand.PrivateSessions["private"] != "token" {
		t.Fatalf("unexpected command: %+v", useCase.getCommand)
	}
}

func TestHandleGameOfficersPostAcceptsJSONAndLegacyForm(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		issue := domaingame.OfficerRecruitedIssue()
		useCase := &fakeGameOfficersUseCase{
			recruitResult: appgame.OfficersResult{Authenticated: true, Officers: sampleGameOfficers(), ActionIssue: issue},
		}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/game/officers?cp=101", strings.NewReader(`{"officerId":3,"days":7}`))
		request.Header.Set("Content-Type", "application/json")

		app{deps: Dependencies{GameOfficers: useCase}}.handleGameOfficers(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if useCase.recruitCommand.PlanetID != 101 ||
			useCase.recruitCommand.Mutation.OfficerID != domaingame.OfficerEngineer ||
			useCase.recruitCommand.Mutation.Days != domaingame.OfficerWeekDays {
			t.Fatalf("unexpected JSON mutation command: %+v", useCase.recruitCommand)
		}
		var response gameOfficersResponse
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if response.ActionIssue == nil || response.ActionIssue.Code != domaingame.OfficerIssueRecruited {
			t.Fatalf("expected action issue mapping, got %+v", response.ActionIssue)
		}
	})

	t.Run("legacy form", func(t *testing.T) {
		useCase := &fakeGameOfficersUseCase{
			recruitResult: appgame.OfficersResult{Authenticated: true, Officers: sampleGameOfficers()},
		}
		form := url.Values{}
		form.Set("type", "-5")
		form.Set("days", "90")
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/game/officers", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		app{deps: Dependencies{GameOfficers: useCase}}.handleGameOfficers(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if useCase.recruitCommand.Mutation.OfficerID != domaingame.OfficerTechnocrat ||
			useCase.recruitCommand.Mutation.Days != domaingame.OfficerThreeMonthDays {
			t.Fatalf("unexpected legacy mutation command: %+v", useCase.recruitCommand)
		}
	})

	t.Run("json legacy type fallback", func(t *testing.T) {
		useCase := &fakeGameOfficersUseCase{
			recruitResult: appgame.OfficersResult{Authenticated: true, Officers: sampleGameOfficers()},
		}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/game/officers", strings.NewReader(`{"type":2,"days":90}`))
		request.Header.Set("Content-Type", "application/json")

		app{deps: Dependencies{GameOfficers: useCase}}.handleGameOfficers(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if useCase.recruitCommand.Mutation.OfficerID != domaingame.OfficerAdmiral ||
			useCase.recruitCommand.Mutation.Days != domaingame.OfficerThreeMonthDays {
			t.Fatalf("unexpected JSON fallback mutation command: %+v", useCase.recruitCommand)
		}
	})
}

func TestHandleGameOfficersReportsAuthAndRequestErrors(t *testing.T) {
	issue := domainpublicsite.SessionIssue{Code: "missing_session", Message: "Session is invalid."}
	tests := []struct {
		name     string
		app      app
		request  *http.Request
		wantCode int
		wantBody string
	}{
		{
			name:     "missing dependency",
			app:      app{},
			request:  httptest.NewRequest(http.MethodGet, "/api/game/officers", nil),
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game officers unavailable",
		},
		{
			name:     "post missing dependency",
			app:      app{},
			request:  jsonOfficersRequest(`{"officerId":1,"days":7}`),
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game officers unavailable",
		},
		{
			name:     "invalid planet",
			app:      app{deps: Dependencies{GameOfficers: &fakeGameOfficersUseCase{}}},
			request:  httptest.NewRequest(http.MethodGet, "/api/game/officers?cp=bad", nil),
			wantCode: http.StatusBadRequest,
			wantBody: "invalid selected planet",
		},
		{
			name:     "invalid json",
			app:      app{deps: Dependencies{GameOfficers: &fakeGameOfficersUseCase{}}},
			request:  jsonOfficersRequest("{"),
			wantCode: http.StatusBadRequest,
			wantBody: "invalid officers request",
		},
		{
			name:     "post invalid planet",
			app:      app{deps: Dependencies{GameOfficers: &fakeGameOfficersUseCase{}}},
			request:  httptest.NewRequest(http.MethodPost, "/api/game/officers?cp=bad", strings.NewReader("type=1")),
			wantCode: http.StatusBadRequest,
			wantBody: "invalid selected planet",
		},
		{
			name: "unauthenticated",
			app: app{deps: Dependencies{GameOfficers: &fakeGameOfficersUseCase{
				getResult: appgame.OfficersResult{Authenticated: false, Issues: []domainpublicsite.SessionIssue{issue}},
			}}},
			request:  httptest.NewRequest(http.MethodGet, "/api/game/officers", nil),
			wantCode: http.StatusUnauthorized,
			wantBody: "missing_session",
		},
		{
			name:     "use case error",
			app:      app{deps: Dependencies{GameOfficers: &fakeGameOfficersUseCase{getErr: errors.New("database down")}}},
			request:  httptest.NewRequest(http.MethodGet, "/api/game/officers", nil),
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game officers unavailable",
		},
		{
			name:     "post use case error",
			app:      app{deps: Dependencies{GameOfficers: &fakeGameOfficersUseCase{recruitErr: errors.New("database down")}}},
			request:  jsonOfficersRequest(`{"officerId":1,"days":7}`),
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game officers unavailable",
		},
		{
			name:     "method not allowed",
			app:      app{deps: Dependencies{GameOfficers: &fakeGameOfficersUseCase{}}},
			request:  httptest.NewRequest(http.MethodPut, "/api/game/officers", nil),
			wantCode: http.StatusMethodNotAllowed,
			wantBody: "method not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			tt.app.handleGameOfficers(recorder, tt.request)
			if recorder.Code != tt.wantCode {
				t.Fatalf("expected %d, got %d: %s", tt.wantCode, recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), tt.wantBody) {
				t.Fatalf("expected body to contain %q, got %q", tt.wantBody, recorder.Body.String())
			}
		})
	}
}

func TestLegacyOfficerInt(t *testing.T) {
	for _, test := range []struct {
		value string
		want  int
	}{
		{"", 0},
		{"abc", 0},
		{"1.500", 1500},
		{"-7", 7},
	} {
		if got := legacyOfficerInt(test.value); got != test.want {
			t.Fatalf("legacyOfficerInt(%q)=%d want %d", test.value, got, test.want)
		}
	}
}

func jsonOfficersRequest(body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/game/officers", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

func sampleGameOfficers() domaingame.Officers {
	return domaingame.Officers{
		Commander: "legor",
		CurrentPlanet: domaingame.PlanetOverview{
			ID:          99,
			Name:        "Homeworld",
			Type:        domaingame.PlanetTypePlanet,
			Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
			Resources:   domaingame.Resources{Metal: 100, Crystal: 50, Deuterium: 25, DarkMatter: 11000, Energy: 7},
		},
		PlanetSwitcher: []domaingame.PlanetSummary{
			{ID: 99, Name: "Homeworld", Type: domaingame.PlanetTypePlanet, Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3}, Current: true},
		},
		User: domaingame.OfficersUser{PaidDarkMatter: 4000, FreeDarkMatter: 7000},
		Rows: []domaingame.OfficerRow{
			{ID: domaingame.OfficerCommander, Key: "commander", Name: "Commander", Description: "desc", Note: "note", Image: "commander_stern_gross.jpg", Icon: "commander_ikon.gif", Active: true, Until: 1700604800, DaysLeft: 7, WeekCost: 10000, ThreeMonthCost: 100000},
		},
	}
}

type fakeGameOfficersUseCase struct {
	getResult      appgame.OfficersResult
	recruitResult  appgame.OfficersResult
	getErr         error
	recruitErr     error
	getCommand     appgame.OfficersCommand
	recruitCommand appgame.OfficersMutationCommand
}

func (f *fakeGameOfficersUseCase) GetOfficers(_ context.Context, command appgame.OfficersCommand) (appgame.OfficersResult, error) {
	f.getCommand = command
	return f.getResult, f.getErr
}

func (f *fakeGameOfficersUseCase) RecruitOfficer(_ context.Context, command appgame.OfficersMutationCommand) (appgame.OfficersResult, error) {
	f.recruitCommand = command
	return f.recruitResult, f.recruitErr
}
