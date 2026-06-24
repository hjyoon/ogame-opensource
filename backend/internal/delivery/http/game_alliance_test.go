package httpdelivery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestGameAllianceHandlerReadsLegacyApplyQuery(t *testing.T) {
	usecase := &fakeGameAllianceUseCase{result: appgame.AllianceResult{
		Authenticated: true,
		Alliance:      allianceHandlerFixture(),
	}}
	a := app{deps: Dependencies{GameAlliance: usecase}}
	request := httptest.NewRequest(http.MethodGet, "/api/game/alliance?session=pub&page=bewerben&allyid=7&show=11", nil)
	response := httptest.NewRecorder()

	a.handleGameAlliance(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", response.Code, response.Body.String())
	}
	if usecase.command.View != domaingame.AllianceViewApply || usecase.command.AllianceID != 7 || usecase.command.ApplicationID != 11 {
		t.Fatalf("unexpected command: %+v", usecase.command)
	}
	var payload gameAllianceResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Authenticated || payload.Alliance == nil || payload.Alliance.Own == nil ||
		payload.Alliance.Own.Tag != "TAG" || len(payload.Alliance.Members) != 1 ||
		payload.Alliance.Pending == nil || payload.Alliance.SelectedApp == nil {
		t.Fatalf("unexpected response: %+v", payload)
	}
}

func TestGameAllianceHandlerMutatesJSONAndLegacyForms(t *testing.T) {
	usecase := &fakeGameAllianceUseCase{result: appgame.AllianceResult{
		Authenticated: true,
		Alliance:      allianceHandlerFixture(),
		ActionIssue:   domaingame.AllianceIssue(domaingame.AllianceIssueCreated),
	}}
	a := app{deps: Dependencies{GameAlliance: usecase}}
	request := httptest.NewRequest(http.MethodPost, "/api/game/alliance?session=pub&a=1", strings.NewReader(`{"action":"create","tag":"TAG","name":"Alliance"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	a.handleGameAlliance(response, request)

	if response.Code != http.StatusOK || usecase.mutation.Mutation.Action != "create" || usecase.mutation.Mutation.Tag != "TAG" {
		t.Fatalf("unexpected json mutation status=%d query=%+v body=%s", response.Code, usecase.mutation, response.Body.String())
	}

	form := strings.NewReader("aktion=Accept&text=")
	request = httptest.NewRequest(http.MethodPost, "/api/game/alliance?session=pub&page=bewerbungen&show=15", form)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response = httptest.NewRecorder()
	a.handleGameAlliance(response, request)
	if response.Code != http.StatusOK || usecase.mutation.Mutation.Action != "accept" || usecase.mutation.Mutation.ApplicationID != 15 {
		t.Fatalf("unexpected form mutation status=%d query=%+v", response.Code, usecase.mutation)
	}
}

func TestGameAllianceHandlerRejectsInvalidAndUnauthenticatedRequests(t *testing.T) {
	a := app{}
	response := httptest.NewRecorder()
	a.handleGameAlliance(response, httptest.NewRequest(http.MethodGet, "/api/game/alliance?session=pub", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected unavailable without dependency, got %d", response.Code)
	}

	response = httptest.NewRecorder()
	a.handleGameAlliance(response, httptest.NewRequest(http.MethodPost, "/api/game/alliance?session=pub", strings.NewReader("{}")))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected post unavailable without dependency, got %d", response.Code)
	}

	usecase := &fakeGameAllianceUseCase{result: appgame.AllianceResult{
		Authenticated: false,
		Issues:        []domainpublicsite.SessionIssue{{Code: "missing", Message: "Session is invalid."}},
	}}
	a = app{deps: Dependencies{GameAlliance: usecase}}
	response = httptest.NewRecorder()
	a.handleGameAlliance(response, httptest.NewRequest(http.MethodGet, "/api/game/alliance?session=bad", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", response.Code)
	}

	response = httptest.NewRecorder()
	a.handleGameAlliance(response, httptest.NewRequest(http.MethodPut, "/api/game/alliance?session=pub", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected method not allowed, got %d", response.Code)
	}

	usecase.err = context.Canceled
	response = httptest.NewRecorder()
	a.handleGameAlliance(response, httptest.NewRequest(http.MethodGet, "/api/game/alliance?session=pub", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected get usecase error as unavailable, got %d", response.Code)
	}

	usecase.err = nil
	response = httptest.NewRecorder()
	a.handleGameAlliance(response, httptest.NewRequest(http.MethodGet, "/api/game/alliance?session=pub&cp=bad", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid cp bad request, got %d", response.Code)
	}

	response = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/game/alliance?session=pub&cp=bad", strings.NewReader("{}"))
	request.Header.Set("Content-Type", "application/json")
	a.handleGameAlliance(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid post cp bad request, got %d", response.Code)
	}

	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/game/alliance?session=pub", strings.NewReader("{"))
	request.Header.Set("Content-Type", "application/json")
	a.handleGameAlliance(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid json bad request, got %d", response.Code)
	}

	usecase.err = context.Canceled
	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/game/alliance?session=pub", strings.NewReader(`{"action":"search"}`))
	request.Header.Set("Content-Type", "application/json")
	a.handleGameAlliance(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected post usecase error as unavailable, got %d", response.Code)
	}
}

func TestSelectedAllianceQueryAndLegacyMutationParsing(t *testing.T) {
	tests := []struct {
		url  string
		view domaingame.AllianceView
	}{
		{"/api/game/alliance?page=bewerben&allyid=7", domaingame.AllianceViewApply},
		{"/api/game/alliance?page=bewerbungen&show=3", domaingame.AllianceViewApplications},
		{"/api/game/alliance?a=1", domaingame.AllianceViewCreate},
		{"/api/game/alliance?a=2&suchtext=TAG", domaingame.AllianceViewSearch},
		{"/api/game/alliance?a=4", domaingame.AllianceViewMembers},
		{"/api/game/alliance?a=5&t=3", domaingame.AllianceViewManagement},
		{"/api/game/alliance?a=11&d=2", domaingame.AllianceViewManagement},
		{"/api/game/alliance", domaingame.AllianceViewHome},
	}
	for _, tt := range tests {
		query := selectedAllianceQuery(httptest.NewRequest(http.MethodGet, tt.url, nil))
		if query.View != tt.view {
			t.Fatalf("expected %s for %s, got %+v", tt.view, tt.url, query)
		}
	}

	formRequest := httptest.NewRequest(http.MethodPost, "/api/game/alliance?page=bewerben&allyid=9", strings.NewReader("weiter=Submit&text=hello"))
	formRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mutation, err := decodeGameAllianceMutation(formRequest)
	if err != nil || mutation.Action != "apply" || mutation.AllianceID != 9 || mutation.Text != "hello" {
		t.Fatalf("unexpected apply mutation=%+v err=%v", mutation, err)
	}
	for _, tt := range []struct {
		url  string
		form string
		want string
	}{
		{"/api/game/alliance?page=bewerbungen&show=9", "aktion=Reject&text=no", "reject"},
		{"/api/game/alliance?a=1&weiter=1", "tag=TAG&name=Alliance", "create"},
		{"/api/game/alliance?a=3", "", "leave"},
		{"/api/game/alliance?a=11&d=1&t=3", "text=hello&bewforce=1", "save_text"},
		{"/api/game/alliance?a=11&d=2", "hp=https%3A%2F%2Fexample.com&logo=&bew=1&fname=Right+Hand", "save_settings"},
		{"/api/game/alliance", "bcancel=Withdraw+application", "withdraw"},
		{"/api/game/alliance?a=15", "newrangname=Bad", "15"},
	} {
		request := httptest.NewRequest(http.MethodPost, tt.url, strings.NewReader(tt.form))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		mutation, err := decodeGameAllianceMutation(request)
		if err != nil || mutation.Action != tt.want {
			t.Fatalf("expected %q mutation for %s, got %+v err=%v", tt.want, tt.url, mutation, err)
		}
	}
	saveTextRequest := httptest.NewRequest(http.MethodPost, "/api/game/alliance?a=11&d=1&t=3", strings.NewReader("text=hello&bewforce=1"))
	saveTextRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if mutation, err := decodeGameAllianceMutation(saveTextRequest); err != nil || mutation.TextKind != 3 || !mutation.InsertApp {
		t.Fatalf("unexpected save text mutation=%+v err=%v", mutation, err)
	}
	saveSettingsRequest := httptest.NewRequest(http.MethodPost, "/api/game/alliance?a=11&d=2", strings.NewReader("hp=https%3A%2F%2Fexample.com&logo=https%3A%2F%2Fexample.com%2Flogo.png&bew=1&fname=Right+Hand"))
	saveSettingsRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if mutation, err := decodeGameAllianceMutation(saveSettingsRequest); err != nil || mutation.Open || mutation.Homepage != "https://example.com" || mutation.FounderRankName != "Right Hand" {
		t.Fatalf("unexpected save settings mutation=%+v err=%v", mutation, err)
	}
	if legacyAllianceInt("-12") != 12 || legacyAllianceInt("bad") != 0 {
		t.Fatal("legacy int parser mismatch")
	}
	if toGameAllianceInfo(nil) != nil || toGameAllianceApplicationPtr(nil) != nil || toGameAllianceActionIssue(nil) != nil {
		t.Fatal("expected nil conversion helpers")
	}
}

func allianceHandlerFixture() domaingame.Alliance {
	app := domaingame.AllianceApplication{ID: 11, AllianceID: 7, PlayerID: 43, PlayerName: "newcomer", Text: "hello", Date: 123}
	return domaingame.Alliance{
		Commander:      "legor",
		CurrentPlanet:  domaingame.PlanetOverview{ID: 99},
		PlanetSwitcher: []domaingame.PlanetSummary{{ID: 99, Name: "Arakis"}},
		View:           domaingame.AllianceViewApplications,
		Viewer:         domaingame.AllianceViewer{PlayerID: 42, Name: "legor", Validated: true, AllianceID: 7, RankName: "Founder", RankRights: domaingame.AllianceFounderRights, Founder: true},
		Own:            &domaingame.AllianceInfo{ID: 7, Tag: "TAG", Name: "Alliance", Open: true, MemberCount: 2, ApplicationCount: 1},
		Target:         &domaingame.AllianceInfo{ID: 7, Tag: "TAG", Name: "Alliance", Open: true},
		Pending:        &app,
		SearchText:     "TA",
		SearchResults:  []domaingame.AllianceSearchResult{{ID: 7, Tag: "TAG", Name: "Alliance", MemberCount: 2}},
		Applications:   []domaingame.AllianceApplication{app},
		SelectedApp:    &app,
		Members:        []domaingame.AllianceMember{{PlayerID: 42, Name: "legor", RankName: "Founder", Score: 1000, Galaxy: 1, System: 2, Position: 3}},
		Ranks:          []domaingame.AllianceRank{{ID: 0, Name: "Founder", Rights: domaingame.AllianceFounderRights}},
	}
}

type fakeGameAllianceUseCase struct {
	result   appgame.AllianceResult
	err      error
	command  appgame.AllianceCommand
	mutation appgame.AllianceMutationCommand
}

func (f *fakeGameAllianceUseCase) GetAlliance(_ context.Context, command appgame.AllianceCommand) (appgame.AllianceResult, error) {
	f.command = command
	return f.result, f.err
}

func (f *fakeGameAllianceUseCase) MutateAlliance(_ context.Context, command appgame.AllianceMutationCommand) (appgame.AllianceResult, error) {
	f.mutation = command
	return f.result, f.err
}
