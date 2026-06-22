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

func TestHandleGameMerchantGetWritesAuthenticatedSummary(t *testing.T) {
	useCase := &fakeGameMerchantUseCase{
		getResult: appgame.MerchantResult{Authenticated: true, Merchant: sampleGameMerchant()},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/game/merchant?session=public&cp=99", nil)
	request.RemoteAddr = "203.0.113.1:7000"
	request.AddCookie(&http.Cookie{Name: "private", Value: "token"})

	app{deps: Dependencies{GameMerchant: useCase}}.handleGameMerchant(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response gameMerchantResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Authenticated || response.Merchant == nil {
		t.Fatalf("expected authenticated merchant response, got %+v", response)
	}
	if response.Merchant.Commander != "legor" || response.Merchant.CurrentPlanet.ID != 99 ||
		response.Merchant.ActiveOfferID != domaingame.MerchantResourceMetal || response.Merchant.Rows[0].Name != "Metal" {
		t.Fatalf("unexpected mapped merchant: %+v", response.Merchant)
	}
	if useCase.getCommand.PublicSession != "public" || useCase.getCommand.PlanetID != 99 ||
		useCase.getCommand.RemoteAddr != "203.0.113.1" || useCase.getCommand.PrivateSessions["private"] != "token" {
		t.Fatalf("unexpected command: %+v", useCase.getCommand)
	}
}

func TestHandleGameMerchantPostAcceptsJSONAndLegacyForm(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		issue := domaingame.MerchantNotEnoughResourceIssue()
		useCase := &fakeGameMerchantUseCase{
			mutateResult: appgame.MerchantResult{Authenticated: true, Merchant: sampleGameMerchant(), ActionIssue: issue},
		}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/game/merchant?cp=101", strings.NewReader(`{"action":"trade","offerId":1,"values":{"metal":0,"crystal":100,"deuterium":50}}`))
		request.Header.Set("Content-Type", "application/json")

		app{deps: Dependencies{GameMerchant: useCase}}.handleGameMerchant(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if useCase.mutateCommand.PlanetID != 101 ||
			useCase.mutateCommand.Mutation.Action != "trade" ||
			useCase.mutateCommand.Mutation.OfferID != domaingame.MerchantResourceMetal ||
			useCase.mutateCommand.Mutation.Values.Crystal != 100 {
			t.Fatalf("unexpected JSON mutation command: %+v", useCase.mutateCommand)
		}
		var response gameMerchantResponse
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if response.ActionIssue == nil || response.ActionIssue.Code != domaingame.MerchantIssueNotEnoughResource {
			t.Fatalf("expected action issue mapping, got %+v", response.ActionIssue)
		}
	})

	t.Run("legacy form", func(t *testing.T) {
		useCase := &fakeGameMerchantUseCase{
			mutateResult: appgame.MerchantResult{Authenticated: true, Merchant: sampleGameMerchant()},
		}
		form := url.Values{}
		form.Set("trade", "1")
		form.Set("offer_id", "2")
		form.Set("1_value", "-1.500")
		form.Set("2_value", "0")
		form.Set("3_value", "750")
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/game/merchant", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		app{deps: Dependencies{GameMerchant: useCase}}.handleGameMerchant(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if useCase.mutateCommand.Mutation.Action != "trade" ||
			useCase.mutateCommand.Mutation.OfferID != domaingame.MerchantResourceCrystal ||
			useCase.mutateCommand.Mutation.Values.Metal != 1500 ||
			useCase.mutateCommand.Mutation.Values.Deuterium != 750 {
			t.Fatalf("unexpected legacy mutation command: %+v", useCase.mutateCommand)
		}
	})

	t.Run("legacy form defaults to call", func(t *testing.T) {
		useCase := &fakeGameMerchantUseCase{
			mutateResult: appgame.MerchantResult{Authenticated: true, Merchant: sampleGameMerchant()},
		}
		form := url.Values{}
		form.Set("offer_id", "3")
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/game/merchant", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		app{deps: Dependencies{GameMerchant: useCase}}.handleGameMerchant(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if useCase.mutateCommand.Mutation.Action != "call" ||
			useCase.mutateCommand.Mutation.OfferID != domaingame.MerchantResourceDeuterium {
			t.Fatalf("unexpected default call mutation command: %+v", useCase.mutateCommand)
		}
	})
}

func TestHandleGameMerchantReportsAuthAndRequestErrors(t *testing.T) {
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
			request:  httptest.NewRequest(http.MethodGet, "/api/game/merchant", nil),
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game merchant unavailable",
		},
		{
			name:     "post missing dependency",
			app:      app{},
			request:  jsonMerchantRequest(`{"action":"call","offerId":1}`),
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game merchant unavailable",
		},
		{
			name:     "invalid planet",
			app:      app{deps: Dependencies{GameMerchant: &fakeGameMerchantUseCase{}}},
			request:  httptest.NewRequest(http.MethodGet, "/api/game/merchant?cp=bad", nil),
			wantCode: http.StatusBadRequest,
			wantBody: "invalid selected planet",
		},
		{
			name:     "invalid json",
			app:      app{deps: Dependencies{GameMerchant: &fakeGameMerchantUseCase{}}},
			request:  jsonMerchantRequest("{"),
			wantCode: http.StatusBadRequest,
			wantBody: "invalid merchant request",
		},
		{
			name:     "post invalid planet",
			app:      app{deps: Dependencies{GameMerchant: &fakeGameMerchantUseCase{}}},
			request:  httptest.NewRequest(http.MethodPost, "/api/game/merchant?cp=bad", strings.NewReader("offer_id=1")),
			wantCode: http.StatusBadRequest,
			wantBody: "invalid selected planet",
		},
		{
			name: "unauthenticated",
			app: app{deps: Dependencies{GameMerchant: &fakeGameMerchantUseCase{
				getResult: appgame.MerchantResult{Authenticated: false, Issues: []domainpublicsite.SessionIssue{issue}},
			}}},
			request:  httptest.NewRequest(http.MethodGet, "/api/game/merchant", nil),
			wantCode: http.StatusUnauthorized,
			wantBody: "missing_session",
		},
		{
			name:     "use case error",
			app:      app{deps: Dependencies{GameMerchant: &fakeGameMerchantUseCase{getErr: errors.New("database down")}}},
			request:  httptest.NewRequest(http.MethodGet, "/api/game/merchant", nil),
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game merchant unavailable",
		},
		{
			name:     "post use case error",
			app:      app{deps: Dependencies{GameMerchant: &fakeGameMerchantUseCase{mutateErr: errors.New("database down")}}},
			request:  jsonMerchantRequest(`{"action":"call","offerId":1}`),
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game merchant unavailable",
		},
		{
			name:     "method not allowed",
			app:      app{deps: Dependencies{GameMerchant: &fakeGameMerchantUseCase{}}},
			request:  httptest.NewRequest(http.MethodPut, "/api/game/merchant", nil),
			wantCode: http.StatusMethodNotAllowed,
			wantBody: "method not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			tt.app.handleGameMerchant(recorder, tt.request)
			if recorder.Code != tt.wantCode {
				t.Fatalf("expected %d, got %d: %s", tt.wantCode, recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), tt.wantBody) {
				t.Fatalf("expected body to contain %q, got %q", tt.wantBody, recorder.Body.String())
			}
		})
	}
}

func TestLegacyMerchantInt(t *testing.T) {
	for _, test := range []struct {
		value string
		want  int
	}{
		{"", 0},
		{"abc", 0},
		{"1.500", 1500},
		{"-7", 7},
	} {
		if got := legacyMerchantInt(test.value); got != test.want {
			t.Fatalf("legacyMerchantInt(%q)=%d want %d", test.value, got, test.want)
		}
	}
}

func jsonMerchantRequest(body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/game/merchant", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

func sampleGameMerchant() domaingame.Merchant {
	return domaingame.Merchant{
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
		User:          domaingame.MerchantUser{PaidDarkMatter: 4000, FreeDarkMatter: 7000},
		ActiveOfferID: domaingame.MerchantResourceMetal,
		Rates:         domaingame.MerchantRates{Metal: 3, Crystal: 2, Deuterium: 1},
		Rows: []domaingame.MerchantResourceRow{
			{ID: domaingame.MerchantResourceMetal, Name: "Metal", Offered: true, Value: 100, FreeStorage: 900, Rate: 3},
		},
	}
}

type fakeGameMerchantUseCase struct {
	getResult     appgame.MerchantResult
	mutateResult  appgame.MerchantResult
	getErr        error
	mutateErr     error
	getCommand    appgame.MerchantCommand
	mutateCommand appgame.MerchantMutationCommand
}

func (f *fakeGameMerchantUseCase) GetMerchant(_ context.Context, command appgame.MerchantCommand) (appgame.MerchantResult, error) {
	f.getCommand = command
	return f.getResult, f.getErr
}

func (f *fakeGameMerchantUseCase) MutateMerchant(_ context.Context, command appgame.MerchantMutationCommand) (appgame.MerchantResult, error) {
	f.mutateCommand = command
	return f.mutateResult, f.mutateErr
}
