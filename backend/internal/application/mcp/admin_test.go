package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

type fakeMCPAdminService struct {
	level          int
	getErr         error
	mutationErr    error
	getIssue       *domaingame.AdminActionIssue
	getQuery       appgame.AdminQuery
	mutationPlayer int
	mutation       appgame.AdminMutationCommand
	botMutation    appgame.AdminBotEditMutationCommand
	mutated        bool
}

func (f *fakeMCPAdminService) MutateAdminBotEditForPlayer(_ context.Context, playerID int, command appgame.AdminBotEditMutationCommand) (appgame.AdminBotEditMutationResult, error) {
	f.mutationPlayer = playerID
	f.botMutation = command
	f.mutated = true
	if f.mutationErr != nil {
		return appgame.AdminBotEditMutationResult{}, f.mutationErr
	}
	return appgame.AdminBotEditMutationResult{
		Authenticated: true, Source: command.Source, Name: command.Name,
		SelectedStrategyID: command.StrategyID, Strategies: []domaingame.AdminBotStrategy{{ID: command.StrategyID, Name: command.Name}},
	}, nil
}

func (f *fakeMCPAdminService) GetAdminForPlayer(_ context.Context, query appgame.AdminQuery) (appgame.AdminResult, error) {
	f.getQuery = query
	if f.getErr != nil {
		return appgame.AdminResult{}, f.getErr
	}
	admin := domaingame.NewAdmin(
		domaingame.Overview{Commander: "staff", CurrentPlanet: domaingame.PlanetOverview{ID: 70, Name: "Staffworld"}},
		domaingame.AdminViewer{PlayerID: query.PlayerID, Name: "staff", Level: f.level},
		query.Mode,
	)
	return appgame.AdminResult{Authenticated: true, Admin: admin, ActionIssue: f.getIssue}, nil
}

func (f *fakeMCPAdminService) MutateAdminForPlayer(_ context.Context, playerID int, command appgame.AdminMutationCommand) (appgame.AdminResult, error) {
	f.mutationPlayer = playerID
	f.mutation = command
	f.mutated = true
	if f.mutationErr != nil {
		return appgame.AdminResult{}, f.mutationErr
	}
	admin := domaingame.NewAdmin(
		domaingame.Overview{Commander: "staff", CurrentPlanet: domaingame.PlanetOverview{ID: 70}},
		domaingame.AdminViewer{PlayerID: playerID, Name: "staff", Level: f.level},
		command.Mode,
	)
	return appgame.AdminResult{Authenticated: true, Admin: admin, ActionIssue: domaingame.AdminIssue(domaingame.AdminIssueActionSaved)}, nil
}

type fakeUserTypeLookup struct {
	level int
	err   error
}

func (f fakeUserTypeLookup) GetMCPUserType(context.Context, int) (int, error) {
	return f.level, f.err
}

func TestServiceListsStaffToolsByRoleAndScope(t *testing.T) {
	adminService := &fakeMCPAdminService{level: domaingame.AdminLevelAdmin}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{access: map[string]domainmcp.Access{
		"player":   {Authenticated: true, PlayerID: 40, UserType: domaingame.AdminLevelPlayer, Scopes: []string{domainmcp.ScopeOperator}},
		"operator": {Authenticated: true, PlayerID: 41, UserType: domaingame.AdminLevelOperator, Scopes: []string{domainmcp.ScopeOperator}},
		"limited":  {Authenticated: true, PlayerID: 42, UserType: domaingame.AdminLevelAdmin, Scopes: []string{domainmcp.ScopeOperator}},
		"admin":    {Authenticated: true, PlayerID: 43, UserType: domaingame.AdminLevelAdmin, Scopes: []string{domainmcp.ScopeAdmin}},
	}}).WithAdminService(adminService)

	for _, tt := range []struct {
		token string
		want  string
	}{
		{token: "player", want: "get_server_health"},
		{token: "operator", want: "get_server_health,get_mcp_access,get_admin_access,get_admin_panel,mutate_admin_panel"},
		{token: "limited", want: "get_server_health,get_mcp_access,get_admin_access,get_admin_panel,mutate_admin_panel"},
		{token: "admin", want: "get_server_health,get_mcp_access,get_admin_access,get_admin_panel,mutate_admin_panel"},
	} {
		tools, err := service.ListTools(context.Background(), domainmcp.ListToolsCommand{AccessToken: tt.token})
		if err != nil {
			t.Fatalf("ListTools(%s): %v", tt.token, err)
		}
		names := make([]string, 0, len(tools.Tools))
		for _, tool := range tools.Tools {
			names = append(names, tool.Name)
		}
		if strings.Join(names, ",") != tt.want {
			t.Fatalf("ListTools(%s)=%v want=%s", tt.token, names, tt.want)
		}
	}
}

func TestServiceCallsRoleFilteredAdminTools(t *testing.T) {
	adminService := &fakeMCPAdminService{level: domaingame.AdminLevelAdmin}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{access: map[string]domainmcp.Access{
		"operator": {Authenticated: true, PlayerID: 42, UserType: domaingame.AdminLevelOperator, Scopes: []string{domainmcp.ScopeOperator}},
		"limited":  {Authenticated: true, PlayerID: 42, UserType: domaingame.AdminLevelAdmin, Scopes: []string{domainmcp.ScopeOperator}},
		"admin":    {Authenticated: true, PlayerID: 42, UserType: domaingame.AdminLevelAdmin, Scopes: []string{domainmcp.ScopeAdmin}},
	}}).WithAdminService(adminService)

	access, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_admin_access", AccessToken: "admin"})
	if err != nil || access.StructuredContent.(map[string]any)["adminAccess"] == nil {
		t.Fatalf("get_admin_access result=%+v err=%v", access, err)
	}

	panel, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name: "get_admin_panel", AccessToken: "admin",
		Arguments: map[string]any{"mode": "Bots", "planetId": 70},
	})
	if err != nil || panel.StructuredContent.(map[string]any)["adminPanel"] == nil || adminService.getQuery.Mode != "Bots" {
		t.Fatalf("get_admin_panel result=%+v query=%+v err=%v", panel, adminService.getQuery, err)
	}
	for _, token := range []string{"operator", "limited"} {
		_, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_admin_panel", AccessToken: token, Arguments: map[string]any{"mode": "Bots"}})
		if !errors.Is(err, domainmcp.ErrForbidden) {
			t.Fatalf("expected %s token to reject admin-only mode, got %v", token, err)
		}
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_admin_panel", AccessToken: "admin", Arguments: map[string]any{"mode": "missing"}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid mode, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_admin_access", AccessToken: "missing"}); !errors.Is(err, domainmcp.ErrUnauthorized) {
		t.Fatalf("expected missing staff token to be unauthorized, got %v", err)
	}
}

func TestServiceAdminMutationRequiresRoleAndConfirmation(t *testing.T) {
	adminService := &fakeMCPAdminService{level: domaingame.AdminLevelAdmin}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{access: map[string]domainmcp.Access{
		"operator": {Authenticated: true, PlayerID: 42, UserType: domaingame.AdminLevelOperator, Scopes: []string{domainmcp.ScopeOperator}},
		"admin":    {Authenticated: true, PlayerID: 42, UserType: domaingame.AdminLevelAdmin, Scopes: []string{domainmcp.ScopeAdmin}},
	}}).WithAdminService(adminService)

	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name: "mutate_admin_panel", AccessToken: "operator",
		Arguments: map[string]any{"mode": "Queue", "action": domaingame.AdminActionQueueFreeze},
	}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected operator to reject admin-only queue mutation, got %v", err)
	}

	arguments := map[string]any{
		"mode": "Queue", "action": domaingame.AdminActionQueueFreeze, "planetId": 70,
		"taskId": 9, "targetIds": []any{7.0}, "values": map[string]any{"speed": 8.0},
		"userSettings":   map[string]any{"email": "staff@example.test", "research": map[string]any{"106": 12.0}},
		"planetSettings": map[string]any{"diameter": 12000.0, "buildings": map[string]any{"1": 20.0}},
	}
	preview, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "mutate_admin_panel", AccessToken: "admin", Arguments: arguments})
	if err != nil || adminService.mutated {
		t.Fatalf("preview result=%+v mutated=%v err=%v", preview, adminService.mutated, err)
	}
	mutation := preview.StructuredContent.(map[string]any)["adminMutation"].(map[string]any)
	confirmation, _ := mutation["confirmation"].(string)
	if confirmation == "" || mutation["executed"] != false {
		t.Fatalf("unexpected preview payload: %+v", mutation)
	}
	execute := make(map[string]any, len(arguments)+2)
	for key, value := range arguments {
		execute[key] = value
	}
	execute["dryRun"] = false
	execute["confirm"] = confirmation
	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "mutate_admin_panel", AccessToken: "admin", Arguments: execute})
	if err != nil || !adminService.mutated || adminService.mutationPlayer != 42 || adminService.mutation.TaskID != 9 || adminService.mutation.User == nil || adminService.mutation.User.Research[106] != 12 || adminService.mutation.Planet == nil || adminService.mutation.Planet.Buildings[1] != 20 {
		t.Fatalf("execution result=%+v mutation=%+v err=%v", result, adminService.mutation, err)
	}

	execute["confirm"] = "wrong"
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "mutate_admin_panel", AccessToken: "admin", Arguments: execute}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid confirmation, got %v", err)
	}
}

func TestServiceAdminBotEditUsesDedicatedUseCase(t *testing.T) {
	adminService := &fakeMCPAdminService{level: domaingame.AdminLevelAdmin}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{access: map[string]domainmcp.Access{
		"admin": {Authenticated: true, PlayerID: 42, UserType: domaingame.AdminLevelAdmin, Scopes: []string{domainmcp.ScopeAdmin}},
	}}).WithAdminService(adminService)
	arguments := map[string]any{
		"mode": "BotEdit", "action": domaingame.AdminActionBotEditSave,
		"strategyId": 7, "name": "Raider", "source": "return true;",
	}
	preview, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "mutate_admin_panel", AccessToken: "admin", Arguments: arguments})
	if err != nil {
		t.Fatalf("bot preview: %v", err)
	}
	confirmation := preview.StructuredContent.(map[string]any)["adminMutation"].(map[string]any)["confirmation"].(string)
	arguments["dryRun"] = false
	arguments["confirm"] = confirmation
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "mutate_admin_panel", AccessToken: "admin", Arguments: arguments}); err != nil || adminService.botMutation.StrategyID != 7 || adminService.botMutation.Source != "return true;" {
		t.Fatalf("bot mutation=%+v err=%v", adminService.botMutation, err)
	}
}

func TestStaffTokenCreationAndOAuthHonorCurrentRole(t *testing.T) {
	repository := &fakeTokenRepository{}
	service := NewServiceWithTokenManagement(
		fakeHealthProvider{}, nil, repository, fakeSessionLookup{auth: authenticatedSession(42)},
		fakeTokenGenerator{secret: "token", code: "code"}, func() time.Time { return time.Unix(1700, 0) },
	).WithUserTypeLookup(fakeUserTypeLookup{level: domaingame.AdminLevelOperator}).WithOAuthCodeRepository(repository)

	created, err := service.CreateToken(context.Background(), CreateTokenCommand{Scopes: []string{domainmcp.ScopeOperator}})
	if err != nil || created.Creation.Token.Scopes[0] != domainmcp.ScopeOperator {
		t.Fatalf("operator token result=%+v err=%v", created, err)
	}
	if _, err := service.CreateToken(context.Background(), CreateTokenCommand{Scopes: []string{domainmcp.ScopeAdmin}}); !errors.Is(err, ErrInvalidTokenRequest) {
		t.Fatalf("expected operator admin scope rejection, got %v", err)
	}
	oauthCommand := OAuthAuthorizeCommand{
		ResponseType: "code", ClientID: "staff-client", RedirectURI: "http://127.0.0.1:9000/callback",
		Resource: "https://game.example/mcp", Scope: domainmcp.ScopeAdmin,
		CodeChallenge: testPKCEChallenge(strings.Repeat("a", 43)), CodeChallengeMethod: "S256",
		Issuer: "https://game.example",
	}
	if _, err := service.AuthorizeOAuth(context.Background(), oauthCommand); !errors.Is(err, ErrInvalidOAuthRequest) {
		t.Fatalf("expected operator OAuth admin scope rejection, got %v", err)
	}

	service = service.WithUserTypeLookup(fakeUserTypeLookup{level: domaingame.AdminLevelAdmin})
	created, err = service.CreateToken(context.Background(), CreateTokenCommand{Scopes: []string{domainmcp.ScopeOperator, domainmcp.ScopeAdmin}})
	if err != nil || len(created.Creation.Token.Scopes) != 2 {
		t.Fatalf("admin token result=%+v err=%v", created, err)
	}
	oauth, err := service.AuthorizeOAuth(context.Background(), oauthCommand)
	if err != nil || !oauth.Authenticated || !oauth.RequiresConsent || oauth.Scopes[0] != domainmcp.ScopeAdmin {
		t.Fatalf("admin OAuth result=%+v err=%v", oauth, err)
	}
	listed, err := service.ListTokens(context.Background(), TokenManagementCommand{})
	if err != nil || listed.Role != "admin" || listed.UserType != domaingame.AdminLevelAdmin || listed.AvailableScopes[len(listed.AvailableScopes)-1] != domainmcp.ScopeAdmin {
		t.Fatalf("admin token list=%+v err=%v", listed, err)
	}

	roleErr := errors.New("role down")
	service = service.WithUserTypeLookup(fakeUserTypeLookup{err: roleErr})
	if _, err := service.CreateToken(context.Background(), CreateTokenCommand{}); !errors.Is(err, roleErr) {
		t.Fatal("expected role lookup error")
	}
	if _, err := service.ListTokens(context.Background(), TokenManagementCommand{}); !errors.Is(err, roleErr) {
		t.Fatalf("expected token list role lookup error, got %v", err)
	}
	if _, err := service.AuthorizeOAuth(context.Background(), oauthCommand); !errors.Is(err, roleErr) {
		t.Fatalf("expected OAuth role lookup error, got %v", err)
	}
}

func TestAdminHelpersAndModePayloads(t *testing.T) {
	if staffAccessLevel(domainmcp.Access{UserType: 2, Scopes: []string{domainmcp.ScopeOperator}}) != 1 ||
		staffAccessLevel(domainmcp.Access{UserType: 2, Scopes: []string{domainmcp.ScopeAdmin}}) != 2 ||
		staffAccessLevel(domainmcp.Access{UserType: 0, Scopes: []string{domainmcp.ScopeAdmin}}) != 0 {
		t.Fatal("staff scope ceiling mismatch")
	}
	if adminMutationConfirmation(map[string]any{"mode": "Bans", "dryRun": true}) != adminMutationConfirmation(map[string]any{"mode": "Bans", "dryRun": false, "confirm": "ignored"}) {
		t.Fatal("confirmation should ignore transport confirmation fields")
	}
	if got := stringIntMap(map[string]int{"7": 8, "bad": 9}); len(got) != 1 || got[7] != 8 {
		t.Fatalf("unexpected integer map: %+v", got)
	}
	if got := stringFloatMap(map[string]float64{"7": 1.5, "bad": 2}); len(got) != 1 || got[7] != 1.5 {
		t.Fatalf("unexpected float map: %+v", got)
	}
	for _, mode := range []string{"Fleetlogs", "Browse", "Reports", "Bans", "Users", "Planets", "Queue", "Uni", "BattleSim", "Errors", "Debug", "Expedition", "Logins", "Checksum", "Bots", "BattleReport", "UserLogs", "BotEdit", "Coupons", "DB", "ColonySettings", "Loca", "Mods", "Home"} {
		if adminModeData(domaingame.Admin{Mode: mode}) == nil {
			t.Fatalf("mode %s returned nil data", mode)
		}
	}
	for _, tool := range []domainmcp.Tool{adminAccessTool(), adminPanelTool(), mutateAdminPanelTool()} {
		if tool.Name == "" || tool.InputSchema["additionalProperties"] != false || tool.Annotations == nil {
			t.Fatalf("invalid admin tool schema: %+v", tool)
		}
	}
}

func TestAdminToolFailurePaths(t *testing.T) {
	verifier := fakeTokenVerifier{access: map[string]domainmcp.Access{
		"player":   {Authenticated: true, PlayerID: 40, UserType: domaingame.AdminLevelPlayer, Scopes: []string{domainmcp.ScopeOperator}},
		"operator": {Authenticated: true, PlayerID: 41, UserType: domaingame.AdminLevelOperator, Scopes: []string{domainmcp.ScopeOperator}},
		"admin":    {Authenticated: true, PlayerID: 42, UserType: domaingame.AdminLevelAdmin, Scopes: []string{domainmcp.ScopeAdmin}},
	}}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, verifier)
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_admin_access", AccessToken: "player"}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("player staff access error=%v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_admin_panel", AccessToken: "operator", Arguments: map[string]any{"mode": "Home"}}); err == nil {
		t.Fatal("expected unavailable admin service")
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "mutate_admin_panel", AccessToken: "operator", Arguments: map[string]any{"mode": "Bans", "action": "ban"}}); err == nil {
		t.Fatal("expected unavailable admin mutation service")
	}

	adminService := &fakeMCPAdminService{level: domaingame.AdminLevelAdmin}
	service = service.WithAdminService(adminService)
	operatorPreview, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name: "mutate_admin_panel", AccessToken: "operator", Arguments: map[string]any{"mode": "Bans", "action": "ban"},
	})
	if err != nil || operatorPreview.StructuredContent.(map[string]any)["adminMutation"] == nil {
		t.Fatalf("operator preview result=%+v error=%v", operatorPreview, err)
	}
	for _, arguments := range []map[string]any{
		{"mode": "Home", "planetId": -1},
		{"mode": "Home", "planetSearch": "invalid"},
	} {
		if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_admin_panel", AccessToken: "admin", Arguments: arguments}); !errors.Is(err, domainmcp.ErrInvalidParams) {
			t.Fatalf("invalid panel arguments=%+v error=%v", arguments, err)
		}
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "mutate_admin_panel", AccessToken: "admin", Arguments: map[string]any{"mode": "Bans"}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("missing action error=%v", err)
	}

	adminService.getErr = errors.New("admin read failed")
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_admin_panel", AccessToken: "admin", Arguments: map[string]any{"mode": "Home"}}); err == nil {
		t.Fatal("expected panel read error")
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "mutate_admin_panel", AccessToken: "admin", Arguments: map[string]any{"mode": "Bans", "action": "ban"}}); err == nil {
		t.Fatal("expected mutation preview read error")
	}
	adminService.getErr = nil
	adminService.getIssue = domaingame.AdminIssue(domaingame.AdminIssueAccessDenied)
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_admin_panel", AccessToken: "admin", Arguments: map[string]any{"mode": "Home"}}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("access issue error=%v", err)
	}
	adminService.getIssue = nil

	playerAdminService := &fakeMCPAdminService{level: domaingame.AdminLevelPlayer}
	service = service.WithAdminService(playerAdminService)
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "mutate_admin_panel", AccessToken: "admin", Arguments: map[string]any{"mode": "Bans", "action": "ban"}}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("service role downgrade error=%v", err)
	}

	adminService.mutationErr = errors.New("admin mutation failed")
	service = service.WithAdminService(adminService)
	arguments := map[string]any{"mode": "Bans", "action": "ban"}
	preview, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "mutate_admin_panel", AccessToken: "admin", Arguments: arguments})
	if err != nil {
		t.Fatalf("mutation preview error=%v", err)
	}
	confirmation := preview.StructuredContent.(map[string]any)["adminMutation"].(map[string]any)["confirmation"].(string)
	arguments["dryRun"], arguments["confirm"] = false, confirmation
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "mutate_admin_panel", AccessToken: "admin", Arguments: arguments}); err == nil {
		t.Fatal("expected mutation execution error")
	}
	botArguments := map[string]any{"mode": "BotEdit", "action": "save"}
	preview, err = service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "mutate_admin_panel", AccessToken: "admin", Arguments: botArguments})
	if err != nil {
		t.Fatalf("bot mutation preview error=%v", err)
	}
	botArguments["dryRun"] = false
	botArguments["confirm"] = preview.StructuredContent.(map[string]any)["adminMutation"].(map[string]any)["confirmation"].(string)
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "mutate_admin_panel", AccessToken: "admin", Arguments: botArguments}); err == nil {
		t.Fatal("expected bot mutation execution error")
	}

	if adminUserMutation(nil) != nil || adminPlanetMutation(nil) != nil {
		t.Fatal("nil admin mutations must remain nil")
	}
	if err := decodeAdminArguments(map[string]any{"invalid": make(chan int)}, &adminReadArguments{}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("marshal error=%v", err)
	}
	if _, err := adminToolResult("invalid", make(chan int)); err == nil {
		t.Fatal("expected admin tool JSON marshal error")
	}
}
