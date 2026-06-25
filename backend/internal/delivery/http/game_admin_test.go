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

func TestGameAdminHandlerMutatesBans(t *testing.T) {
	usecase := &fakeGameAdminUseCase{result: appgame.AdminResult{
		Authenticated: true,
		Admin: domaingame.NewAdmin(
			domaingame.Overview{Commander: "legor", CurrentPlanet: domaingame.PlanetOverview{ID: 99}},
			domaingame.AdminViewer{PlayerID: 42, Name: "legor", Level: domaingame.AdminLevelAdmin},
			"Bans",
		),
		ActionIssue: domaingame.AdminIssue(domaingame.AdminIssueActionSaved),
	}}
	request := httptest.NewRequest(http.MethodPost, "/api/game/admin?session=pub&cp=99&mode=Bans", strings.NewReader(`{"action":"ban","taskId":1001,"targetIds":[77],"banMode":1,"days":0,"hours":2,"reason":"test"}`))
	request.RemoteAddr = "203.0.113.10:4321"
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	app{deps: Dependencies{GameAdmin: usecase}}.handleGameAdmin(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected response status=%d body=%s", response.Code, response.Body.String())
	}
	if usecase.mutation.PlanetID != 99 || usecase.mutation.Mode != "Bans" || usecase.mutation.Action != "ban" ||
		usecase.mutation.TaskID != 1001 || len(usecase.mutation.TargetIDs) != 1 || usecase.mutation.TargetIDs[0] != 77 || usecase.mutation.BanMode != 1 ||
		usecase.mutation.Hours != 2 || usecase.mutation.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected mutation command: %+v", usecase.mutation)
	}
}

func TestGameAdminHandlerMutatesExpeditionSettings(t *testing.T) {
	usecase := &fakeGameAdminUseCase{result: appgame.AdminResult{
		Authenticated: true,
		Admin: domaingame.NewAdmin(
			domaingame.Overview{Commander: "legor", CurrentPlanet: domaingame.PlanetOverview{ID: 99}},
			domaingame.AdminViewer{PlayerID: 42, Name: "legor", Level: domaingame.AdminLevelAdmin},
			"Expedition",
		),
		ActionIssue: domaingame.AdminIssue(domaingame.AdminIssueActionSaved),
	}}
	request := httptest.NewRequest(http.MethodPost, "/api/game/admin?session=pub&cp=99&mode=Expedition", strings.NewReader(`{"action":"settings","values":{"dm_factor":9,"chance_success":77}}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	app{deps: Dependencies{GameAdmin: usecase}}.handleGameAdmin(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected response status=%d body=%s", response.Code, response.Body.String())
	}
	if usecase.mutation.Mode != "Expedition" || usecase.mutation.Action != "settings" ||
		usecase.mutation.Values["dm_factor"] != 9 || usecase.mutation.Values["chance_success"] != 77 {
		t.Fatalf("unexpected mutation command: %+v", usecase.mutation)
	}
}

func TestGameAdminHandlerRejectsInvalidAndUnauthenticatedRequests(t *testing.T) {
	response := httptest.NewRecorder()
	app{}.handleGameAdmin(response, httptest.NewRequest(http.MethodGet, "/api/game/admin?session=pub", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected unavailable without dependency, got %d", response.Code)
	}

	response = httptest.NewRecorder()
	app{}.handleGameAdmin(response, httptest.NewRequest(http.MethodPost, "/api/game/admin?session=pub", strings.NewReader(`{}`)))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected post unavailable without dependency, got %d", response.Code)
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

	response = httptest.NewRecorder()
	app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{}}}.handleGameAdmin(response, httptest.NewRequest(http.MethodPost, "/api/game/admin?session=pub&cp=bad", strings.NewReader(`{}`)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid post cp bad request, got %d", response.Code)
	}

	response = httptest.NewRecorder()
	app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{}}}.handleGameAdmin(response, httptest.NewRequest(http.MethodPost, "/api/game/admin?session=pub", strings.NewReader(`{`)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid post bad request, got %d", response.Code)
	}

	response = httptest.NewRecorder()
	app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{err: context.Canceled}}}.handleGameAdmin(response, httptest.NewRequest(http.MethodPost, "/api/game/admin?session=pub", strings.NewReader(`{}`)))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected mutation error as unavailable, got %d", response.Code)
	}

	response = httptest.NewRecorder()
	app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{}}}.handleGameAdmin(response, httptest.NewRequest(http.MethodPut, "/api/game/admin?session=pub", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected method not allowed, got %d", response.Code)
	}
}

func TestGameAdminSummaryMapsFullPayload(t *testing.T) {
	home := &domaingame.AdminUserPlanet{
		ID:   301,
		Name: "Homeworld",
		Coordinates: domaingame.Coordinates{
			Galaxy:   1,
			System:   42,
			Position: 7,
		},
	}
	owner := domaingame.AdminUserRow{
		PlayerID:   7,
		Name:       "owner",
		RegDate:    1000,
		LastClick:  2000,
		Vacation:   true,
		Banned:     true,
		NoAttack:   true,
		Disable:    true,
		HomePlanet: home,
	}
	admin := domaingame.NewAdmin(
		domaingame.Overview{
			Commander: "legor",
			CurrentPlanet: domaingame.PlanetOverview{
				ID:   99,
				Name: "planet",
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
			},
			PlanetSwitcher: []domaingame.PlanetSummary{{
				ID:      99,
				Name:    "planet",
				Current: true,
			}},
		},
		domaingame.AdminViewer{PlayerID: 1, Name: "legor", Level: domaingame.AdminLevelAdmin},
		"Queue",
	)
	admin.MessageRows = []domaingame.AdminMessageRow{{
		ID:        10,
		OwnerID:   7,
		OwnerName: "owner",
		IP:        "127.0.0.1",
		Agent:     "e2e",
		Text:      "message",
		Date:      111,
	}}
	admin.UserLogRows = []domaingame.AdminUserLogRow{{
		ID:        11,
		OwnerID:   7,
		OwnerName: "owner",
		Type:      "ADMIN",
		Text:      "log",
		Date:      112,
	}}
	admin.UserRows = []domaingame.AdminUserRow{owner, {PlayerID: 8, Name: "plain"}}
	admin.ActiveUsers = []domaingame.AdminUserRow{{PlayerID: 9, Name: "active", LastClick: 3000}}
	admin.PlanetRows = []domaingame.AdminPlanetRow{{
		ID:   401,
		Name: "colony",
		Date: 4000,
		Coordinates: domaingame.Coordinates{
			Galaxy:   2,
			System:   3,
			Position: 4,
		},
		Owner: &owner,
	}, {
		ID:   402,
		Name: "unowned",
	}}
	admin.Universe = &domaingame.AdminUniverseSettings{
		Number:          1,
		Speed:           128,
		FleetSpeed:      64,
		Galaxies:        9,
		Systems:         499,
		MaxUsers:        1000,
		ACS:             1,
		FleetDebris:     30,
		DefenseDebris:   0,
		RapidFire:       true,
		Moons:           true,
		DefenseRepair:   70,
		DefenseDelta:    10,
		UserCount:       55,
		Freeze:          true,
		News1:           "news-one",
		News2:           "news-two",
		NewsUntil:       12345,
		StartDate:       54321,
		BattleEngine:    "/game/battle",
		Language:        "en",
		Hacks:           2,
		ExtBoard:        "board",
		ExtDiscord:      "discord",
		ExtTutorial:     "tutorial",
		ExtRules:        "rules",
		ExtImpressum:    "imprint",
		PHPBattle:       true,
		BattleMax:       250000,
		ForceLanguage:   true,
		StartDarkMatter: 8000,
		MaxShipyard:     5,
		FeedAge:         30,
	}
	admin.Expedition = map[string]int{"maxDarkMatter": 100}
	admin.QueueRows = []domaingame.AdminQueueRow{{
		ID:          501,
		OwnerID:     7,
		OwnerName:   "owner",
		Type:        "Build",
		Description: "Metal Mine 2",
		Priority:    10,
		Start:       1,
		End:         2,
		Freeze:      true,
		Frozen:      3,
	}}
	admin.FleetLogRows = []domaingame.AdminFleetLogRow{{
		TaskID:     502,
		Number:     1,
		Mission:    3,
		Start:      10,
		End:        20,
		FlightTime: 7200,
		Fuel:       5,
		UnionID:    6,
		Origin: domaingame.AdminFleetLogPlanet{
			ID:        601,
			Name:      "origin",
			OwnerID:   7,
			OwnerName: "owner",
			Coordinates: domaingame.Coordinates{
				Galaxy:   1,
				System:   470,
				Position: 4,
			},
			Type: 1,
		},
		Target: domaingame.AdminFleetLogPlanet{
			ID:        602,
			Name:      "target",
			OwnerID:   8,
			OwnerName: "target-owner",
			Coordinates: domaingame.Coordinates{
				Galaxy:   1,
				System:   470,
				Position: 5,
			},
			Type: 1,
		},
		Ships: []domaingame.FleetShipCount{{ID: 202, Name: "Small Cargo", Count: 2}},
		Cargo: []domaingame.FleetResourceLoad{{ID: 901, Name: "Metal", Loaded: 123}},
	}}
	admin.BattleReports = []domaingame.AdminBattleReportRow{{ID: 601, Date: 6000, Title: "battle"}}
	admin.ChecksumGroups = []domaingame.AdminChecksumGroup{{
		Title: "core",
		Rows:  []domaingame.AdminChecksumRow{{Path: "core.php", Checksum: "abc", Status: "ok"}},
	}}
	admin.BotStrategies = []domaingame.AdminBotStrategy{{ID: 701, Name: "bot"}}

	payload := toGameAdminSummary(admin)

	if payload.Mode != "Queue" || payload.Viewer.Name != "legor" || payload.Commander != "legor" {
		t.Fatalf("unexpected admin identity mapping: %+v", payload)
	}
	if len(payload.MessageRows) != 1 || payload.MessageRows[0].Text != "message" ||
		len(payload.UserLogRows) != 1 || payload.UserLogRows[0].Type != "ADMIN" {
		t.Fatalf("expected message and user log rows to map: %+v", payload)
	}
	if len(payload.UserRows) != 2 || payload.UserRows[0].HomePlanet == nil ||
		payload.UserRows[0].HomePlanet.Coordinates.System != 42 || payload.UserRows[1].HomePlanet != nil {
		t.Fatalf("expected user rows and optional home planets to map: %+v", payload.UserRows)
	}
	if len(payload.ActiveUsers) != 1 || payload.ActiveUsers[0].Name != "active" {
		t.Fatalf("expected active users to map: %+v", payload.ActiveUsers)
	}
	if len(payload.PlanetRows) != 2 || payload.PlanetRows[0].Owner == nil ||
		payload.PlanetRows[0].Coordinates.Position != 4 || payload.PlanetRows[1].Owner != nil {
		t.Fatalf("expected planet rows and optional owners to map: %+v", payload.PlanetRows)
	}
	if payload.Universe == nil || payload.Universe.Speed != 128 || !payload.Universe.PHPBattle ||
		payload.Universe.ExtRules != "rules" || payload.Expedition["maxDarkMatter"] != 100 {
		t.Fatalf("expected universe and expedition settings to map: %+v", payload)
	}
	if len(payload.QueueRows) != 1 || !payload.QueueRows[0].Freeze ||
		len(payload.FleetLogRows) != 1 || payload.FleetLogRows[0].Origin.OwnerID != 7 || payload.FleetLogRows[0].Cargo[0].Loaded != 123 ||
		len(payload.BattleReports) != 1 || len(payload.ChecksumGroups) != 1 ||
		len(payload.BotStrategies) != 1 {
		t.Fatalf("expected admin detail rows to map: %+v", payload)
	}
	if issue := toGameAdminActionIssue(&domaingame.AdminActionIssue{Code: "blocked", Message: "Blocked"}); issue == nil || issue.Code != "blocked" || issue.Message != "Blocked" {
		t.Fatalf("expected non-nil action issue conversion, got %+v", issue)
	}
}

type fakeGameAdminUseCase struct {
	result   appgame.AdminResult
	err      error
	command  appgame.AdminCommand
	mutation appgame.AdminMutationCommand
}

func (f *fakeGameAdminUseCase) GetAdmin(_ context.Context, command appgame.AdminCommand) (appgame.AdminResult, error) {
	f.command = command
	return f.result, f.err
}

func (f *fakeGameAdminUseCase) MutateAdmin(_ context.Context, command appgame.AdminMutationCommand) (appgame.AdminResult, error) {
	f.mutation = command
	return f.result, f.err
}
