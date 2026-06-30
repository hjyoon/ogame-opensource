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

func TestGameAdminHandlerMutatesBroadcastAndReports(t *testing.T) {
	usecase := &fakeGameAdminUseCase{result: appgame.AdminResult{
		Authenticated: true,
		Admin: domaingame.NewAdmin(
			domaingame.Overview{Commander: "legor", CurrentPlanet: domaingame.PlanetOverview{ID: 99}},
			domaingame.AdminViewer{PlayerID: 42, Name: "legor", Level: domaingame.AdminLevelOperator},
			"Broadcast",
		),
		ActionIssue: domaingame.AdminIssue(domaingame.AdminIssueActionSaved),
	}}
	request := httptest.NewRequest(http.MethodPost, "/api/game/admin?session=pub&cp=99&mode=Broadcast", strings.NewReader(`{"action":"broadcast_send","category":3,"subject":"subject","text":"text"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	app{deps: Dependencies{GameAdmin: usecase}}.handleGameAdmin(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected broadcast response status=%d body=%s", response.Code, response.Body.String())
	}
	if usecase.mutation.Action != "broadcast_send" || usecase.mutation.Category != 3 ||
		usecase.mutation.Subject != "subject" || usecase.mutation.Text != "text" {
		t.Fatalf("unexpected broadcast mutation command: %+v", usecase.mutation)
	}

	usecase = &fakeGameAdminUseCase{result: appgame.AdminResult{
		Authenticated: true,
		Admin: domaingame.NewAdmin(
			domaingame.Overview{Commander: "legor", CurrentPlanet: domaingame.PlanetOverview{ID: 99}},
			domaingame.AdminViewer{PlayerID: 42, Name: "legor", Level: domaingame.AdminLevelOperator},
			"Reports",
		),
		ActionIssue: domaingame.AdminIssue(domaingame.AdminIssueActionSaved),
	}}
	request = httptest.NewRequest(http.MethodPost, "/api/game/admin?session=pub&cp=99&mode=Reports", strings.NewReader(`{"action":"reports_delete","reportIds":[701,702],"deleteMode":"deletemarked","fileName":"backup_test.json"}`))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()

	app{deps: Dependencies{GameAdmin: usecase}}.handleGameAdmin(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected reports response status=%d body=%s", response.Code, response.Body.String())
	}
	if usecase.mutation.Action != "reports_delete" || len(usecase.mutation.ReportIDs) != 2 ||
		usecase.mutation.ReportIDs[0] != 701 || usecase.mutation.DeleteMode != "deletemarked" ||
		usecase.mutation.FileName != "backup_test.json" {
		t.Fatalf("unexpected reports mutation command: %+v", usecase.mutation)
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

func TestSelectedAdminPlayerIDHandlesLegacyQuery(t *testing.T) {
	playerID, err := selectedAdminPlayerID(httptest.NewRequest(http.MethodGet, "/api/game/admin?player_id=77", nil))
	if err != nil || playerID != 77 {
		t.Fatalf("expected player id 77, got id=%d err=%v", playerID, err)
	}
	playerID, err = selectedAdminPlayerID(httptest.NewRequest(http.MethodGet, "/api/game/admin", nil))
	if err != nil || playerID != 0 {
		t.Fatalf("expected missing player id to be zero, got id=%d err=%v", playerID, err)
	}
	if _, err := selectedAdminPlayerID(httptest.NewRequest(http.MethodGet, "/api/game/admin?player_id=bad", nil)); err == nil {
		t.Fatal("expected invalid player id to fail")
	}
	if _, err := selectedAdminPlayerID(httptest.NewRequest(http.MethodGet, "/api/game/admin?player_id=-1", nil)); err == nil {
		t.Fatal("expected negative player id to fail")
	}
	if toGameAdminPlanetRowPointer(nil) != nil {
		t.Fatal("expected nil planet row pointer conversion")
	}
	if toGameAdminUserPlanetPointer(nil) != nil {
		t.Fatal("expected nil user planet pointer conversion")
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
		LastClick: 99,
		Vacation:  true,
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
	admin.SelectedUser = &domaingame.AdminUserDetail{
		AdminUserRow:   owner,
		PermanentEmail: "permanent@example.test",
		Email:          "active@example.test",
		Alliance:       "[TAG] Test Alliance",
		JoinDate:       4100,
		DisableUntil:   4200,
		VacationUntil:  4300,
		BannedUntil:    4400,
		NoAttackUntil:  4500,
		LastLogin:      4600,
		IPAddress:      "203.0.113.77",
		Validated:      true,
		AdminLevel:     domaingame.AdminLevelOperator,
		Sniff:          true,
		Debug:          true,
		SortBy:         1,
		SortOrder:      2,
		Skin:           "legacy",
		UseSkin:        true,
		DeactivateIP:   true,
		MaxSpy:         7,
		MaxFleetMsg:    8,
		OldScore1:      100,
		OldPlace1:      10,
		OldScore2:      200,
		OldPlace2:      20,
		OldScore3:      300,
		OldPlace3:      30,
		Score1:         400,
		Place1:         40,
		Score2:         500,
		Place2:         50,
		Score3:         600,
		Place3:         60,
		ScoreDate:      4700,
		DarkMatterFree: 900,
		DarkMatter:     1000,
		Research:       domaingame.ResearchLevels{domaingame.ResearchEnergy: 3},
		ActivePlanet: &domaingame.AdminUserPlanet{
			ID:   403,
			Name: "active planet",
			Coordinates: domaingame.Coordinates{
				Galaxy:   3,
				System:   4,
				Position: 5,
			},
		},
		Planets: []domaingame.AdminPlanetRow{{ID: 404, Name: "detail colony"}},
	}
	admin.SelectedPlanet = &domaingame.AdminPlanetDetail{
		AdminPlanetRow: domaingame.AdminPlanetRow{
			ID:    405,
			Name:  "detail planet",
			Date:  4800,
			Owner: &owner,
		},
		Type:             domaingame.PlanetTypePlanet,
		Diameter:         12800,
		Temperature:      40,
		Fields:           20,
		MaxFields:        180,
		RemoveDate:       4900,
		LastActivity:     5000,
		LastUpdate:       5100,
		GateUntil:        5200,
		Score:            domaingame.PlanetScore{Points: 11, FleetPoints: 22, DefensePoints: 33},
		Resources:        domaingame.Resources{Metal: 100, Crystal: 200, Deuterium: 300, DarkMatter: 400},
		EnergyBalance:    50,
		EnergyCapacity:   60,
		ProductionFactor: 0.75,
		Buildings:        []domaingame.AdminTechnologyValue{{ID: domaingame.BuildingMetalMine, Name: "Metal Mine", Value: 12, Percent: 100}},
		Fleet:            []domaingame.AdminTechnologyValue{{ID: domaingame.FleetSmallCargo, Name: "Small Cargo", Value: 3}},
		Defense:          []domaingame.AdminTechnologyValue{{ID: domaingame.DefenseRocketLauncher, Name: "Rocket Launcher", Value: 4}},
		BuildQueue:       []domaingame.BuildingQueueEntry{{TechID: domaingame.BuildingMetalMine, Name: "Metal Mine", Level: 13, Destroy: true, End: 5300}},
		Moon:             &domaingame.AdminPlanetRow{ID: 406, Name: "detail moon"},
		Debris:           &domaingame.AdminPlanetRow{ID: 407, Name: "detail debris"},
	}
	admin.ReportRows = []domaingame.AdminReportRow{{
		ID:        451,
		OwnerID:   7,
		OwnerName: "owner",
		MessageID: 12,
		From:      "reporter",
		Subject:   "subject",
		Text:      "body",
		Date:      4500,
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
		len(payload.UserLogRows) != 1 || payload.UserLogRows[0].Type != "ADMIN" ||
		payload.UserLogRows[0].LastClick != 99 || !payload.UserLogRows[0].Vacation {
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
	if payload.SelectedUser == nil || payload.SelectedUser.PermanentEmail != "permanent@example.test" ||
		payload.SelectedUser.ActivePlanet == nil || payload.SelectedUser.ActivePlanet.Coordinates.System != 4 ||
		payload.SelectedUser.Research[domaingame.ResearchEnergy] != 3 || len(payload.SelectedUser.Planets) != 1 {
		t.Fatalf("expected selected user detail to map: %+v", payload.SelectedUser)
	}
	if payload.SelectedPlanet == nil || payload.SelectedPlanet.Owner == nil || payload.SelectedPlanet.Owner.Name != "owner" ||
		payload.SelectedPlanet.Resources.DarkMatter != 400 || payload.SelectedPlanet.Score.DefensePoints != 33 ||
		len(payload.SelectedPlanet.Buildings) != 1 || payload.SelectedPlanet.Buildings[0].Percent != 100 ||
		len(payload.SelectedPlanet.Fleet) != 1 || len(payload.SelectedPlanet.Defense) != 1 ||
		len(payload.SelectedPlanet.BuildQueue) != 1 || !payload.SelectedPlanet.BuildQueue[0].Destroy ||
		payload.SelectedPlanet.Moon == nil || payload.SelectedPlanet.Moon.Name != "detail moon" ||
		payload.SelectedPlanet.Debris == nil || payload.SelectedPlanet.Debris.Name != "detail debris" {
		t.Fatalf("expected selected planet detail to map: %+v", payload.SelectedPlanet)
	}
	if len(payload.ReportRows) != 1 || payload.ReportRows[0].ID != 451 || payload.ReportRows[0].Subject != "subject" {
		t.Fatalf("expected report rows to map: %+v", payload.ReportRows)
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

func TestLogGameAdminError(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/game/admin?mode=Users", nil)
	logGameAdminError(nil, request, "ignored", errors.New("ignored"))
	logGameAdminError(slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)), request, "ignored", nil)

	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, nil))
	logGameAdminError(logger, request, "game admin mutation failed", errors.New("boom"))

	output := buffer.String()
	if !strings.Contains(output, "game admin mutation failed") ||
		!strings.Contains(output, `"mode":"Users"`) ||
		!strings.Contains(output, `"path":"/api/game/admin"`) {
		t.Fatalf("expected structured admin error log, got %s", output)
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
