package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
	domainsystem "github.com/hjyoon/ogame-opensource/backend/internal/domain/system"
)

func TestServiceInitializesAndListsReadOnlyTools(t *testing.T) {
	service := NewService(fakeHealthProvider{})

	init := service.Initialize(context.Background())
	if init.ProtocolVersion != domainmcp.ProtocolVersion || init.Capabilities.Tools == nil || init.Capabilities.Tools.ListChanged {
		t.Fatalf("unexpected initialize result: %+v", init)
	}

	tools, err := service.ListTools(context.Background(), domainmcp.ListToolsCommand{})
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "get_server_health" {
		t.Fatalf("unexpected tools: %+v", tools)
	}
	if tools.Tools[0].InputSchema["additionalProperties"] != false {
		t.Fatalf("expected closed input schema: %+v", tools.Tools[0].InputSchema)
	}
}

func TestServiceListsScopedToolsOnlyForReadableTokens(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read":  {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
			"write": {Authenticated: true, PlayerID: 43, Scopes: []string{domainmcp.ScopeWrite}},
		},
	})

	tools, err := service.ListTools(context.Background(), domainmcp.ListToolsCommand{AccessToken: "read"})
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	if len(tools.Tools) != 2 || tools.Tools[1].Name != "get_mcp_access" {
		t.Fatalf("expected protected access tool for read token, got %+v", tools)
	}

	tools, err = service.ListTools(context.Background(), domainmcp.ListToolsCommand{AccessToken: "write"})
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	if len(tools.Tools) != 1 {
		t.Fatalf("expected only public tools for token without read scope, got %+v", tools)
	}

	if _, err := service.ListTools(context.Background(), domainmcp.ListToolsCommand{AccessToken: "missing"}); !errors.Is(err, domainmcp.ErrUnauthorized) {
		t.Fatalf("expected unauthorized invalid token, got %v", err)
	}
}

func TestServiceListsPlanetToolWhenReadRepositoryIsAvailable(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
		},
	}).WithReadRepository(fakeReadRepository{})

	tools, err := service.ListTools(context.Background(), domainmcp.ListToolsCommand{AccessToken: "read"})
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "get_server_health,get_mcp_access,list_planets,get_account_overview,get_planet_resources,get_building_queue" {
		t.Fatalf("unexpected tools: %v", names)
	}
}

func TestServiceCallsServerHealthTool(t *testing.T) {
	service := NewService(fakeHealthProvider{})

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_server_health"})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if result.IsError || len(result.Content) != 1 || result.Content[0].Type != "text" {
		t.Fatalf("unexpected health result: %+v", result)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok || structured["status"] != "ok" || structured["goTarget"] != "1.25" {
		t.Fatalf("unexpected structured content: %#v", result.StructuredContent)
	}
}

func TestServiceCallsScopedAccessTool(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read":  {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead, domainmcp.ScopeMessages}},
			"write": {Authenticated: true, PlayerID: 43, Scopes: []string{domainmcp.ScopeWrite}},
		},
	})

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_mcp_access", AccessToken: "read"})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	structured := result.StructuredContent.(map[string]any)
	if result.IsError || structured["playerId"] != 42 {
		t.Fatalf("unexpected access result: %+v", result)
	}

	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_mcp_access"}); !errors.Is(err, domainmcp.ErrUnauthorized) {
		t.Fatalf("expected unauthorized without token, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_mcp_access", AccessToken: "write"}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without read scope, got %v", err)
	}
}

func TestServiceCallsListPlanetsTool(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
		},
	}).WithReadRepository(fakeReadRepository{planets: []domainmcp.Planet{{
		ID:       99,
		Name:     "Homeworld",
		Type:     1,
		TypeName: "planet",
		Coordinates: domainmcp.Coordinates{
			Galaxy:   1,
			System:   2,
			Position: 3,
		},
		Current: true,
	}}})

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "list_planets", AccessToken: "read"})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	structured := result.StructuredContent.(map[string]any)
	if result.IsError || structured["playerId"] != 42 || structured["count"] != 1 {
		t.Fatalf("unexpected list planets result: %+v", result)
	}
	planets := structured["planets"].([]domainmcp.Planet)
	if planets[0].Name != "Homeworld" || !planets[0].Current {
		t.Fatalf("unexpected planets: %+v", planets)
	}
}

func TestServiceListPlanetsToolRequiresRepositoryAndReadScope(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read":  {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
			"write": {Authenticated: true, PlayerID: 43, Scopes: []string{domainmcp.ScopeWrite}},
		},
	})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "list_planets", AccessToken: "read"}); err == nil {
		t.Fatalf("expected missing read repository error")
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "list_planets", AccessToken: "write"}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without read scope, got %v", err)
	}

	service = service.WithReadRepository(fakeReadRepository{err: errors.New("planets down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "list_planets", AccessToken: "read"}); err == nil {
		t.Fatalf("expected read repository error")
	}
}

func TestServiceCallsAccountOverviewTool(t *testing.T) {
	overview := domainmcp.AccountOverview{
		PlayerID:  42,
		Commander: "legor",
		Score:     domainmcp.Score{Raw: 123456, Display: 123, Rank: 1},
		CurrentPlanet: domainmcp.Planet{
			ID:          99,
			Name:        "Homeworld",
			Type:        1,
			TypeName:    "planet",
			Coordinates: domainmcp.Coordinates{Galaxy: 1, System: 2, Position: 3},
			Current:     true,
		},
		PlanetCount:    2,
		UnreadMessages: 5,
	}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
		},
	}).WithReadRepository(fakeReadRepository{overview: overview})

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_account_overview", AccessToken: "read"})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	structured := result.StructuredContent.(map[string]any)
	got := structured["overview"].(domainmcp.AccountOverview)
	if result.IsError || got.Commander != "legor" || got.UnreadMessages != 5 {
		t.Fatalf("unexpected account overview result: %+v", result)
	}
}

func TestServiceAccountOverviewToolRequiresRepositoryAndReadScope(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read":  {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
			"write": {Authenticated: true, PlayerID: 43, Scopes: []string{domainmcp.ScopeWrite}},
		},
	})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_account_overview", AccessToken: "read"}); err == nil {
		t.Fatalf("expected missing read repository error")
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_account_overview", AccessToken: "write"}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without read scope, got %v", err)
	}

	service = service.WithReadRepository(fakeReadRepository{err: errors.New("overview down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_account_overview", AccessToken: "read"}); err == nil {
		t.Fatalf("expected read repository error")
	}
}

func TestServiceCallsPlanetResourcesTool(t *testing.T) {
	resources := domainmcp.PlanetResources{
		PlayerID: 42,
		Planet: domainmcp.Planet{
			ID:          99,
			Name:        "Homeworld",
			Type:        1,
			TypeName:    "planet",
			Coordinates: domainmcp.Coordinates{Galaxy: 1, System: 2, Position: 3},
			Current:     true,
		},
		Resources:         domainmcp.ResourceAmounts{Metal: 100, Crystal: 200, Deuterium: 300, DarkMatter: 400},
		Capacity:          domainmcp.ResourceCapacity{Metal: 100000, Crystal: 100000, Deuterium: 100000},
		Energy:            domainmcp.Energy{Available: 17, Capacity: 42},
		ProductionPerHour: domainmcp.ResourceRates{Metal: 20, Crystal: 10},
	}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
		},
	}).WithReadRepository(fakeReadRepository{resources: resources, resourcesPlanetID: 99})

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "get_planet_resources",
		AccessToken: "read",
		Arguments:   map[string]any{"planetId": float64(99)},
	})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	structured := result.StructuredContent.(map[string]any)
	got := structured["resources"].(domainmcp.PlanetResources)
	if result.IsError || got.Planet.Name != "Homeworld" || got.Energy.Available != 17 || got.Resources.DarkMatter != 400 {
		t.Fatalf("unexpected planet resources result: %+v", result)
	}
}

func TestServicePlanetResourcesToolRequiresRepositoryReadScopeAndValidArguments(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read":  {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
			"write": {Authenticated: true, PlayerID: 43, Scopes: []string{domainmcp.ScopeWrite}},
		},
	})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_planet_resources", AccessToken: "read"}); err == nil {
		t.Fatalf("expected missing read repository error")
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_planet_resources", AccessToken: "write"}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without read scope, got %v", err)
	}

	service = service.WithReadRepository(fakeReadRepository{})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "get_planet_resources",
		AccessToken: "read",
		Arguments:   map[string]any{"planetId": 1.5},
	}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid planet id error, got %v", err)
	}

	service = service.WithReadRepository(fakeReadRepository{err: errors.New("resources down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_planet_resources", AccessToken: "read"}); err == nil {
		t.Fatalf("expected read repository error")
	}
}

func TestServiceCallsBuildingQueueTool(t *testing.T) {
	queue := domainmcp.BuildingQueue{
		PlayerID: 42,
		Planet: domainmcp.Planet{
			ID:          99,
			Name:        "Homeworld",
			Type:        1,
			TypeName:    "planet",
			Coordinates: domainmcp.Coordinates{Galaxy: 1, System: 2, Position: 3},
			Current:     true,
		},
		Count: 1,
		Entries: []domainmcp.BuildingQueueEntry{{
			ListID:           1,
			TechID:           1,
			Name:             "Metal Mine",
			Level:            3,
			End:              200,
			RemainingSeconds: 80,
		}},
	}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
		},
	}).WithReadRepository(fakeReadRepository{buildingQueue: queue, buildingQueuePlanetID: 99})

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "get_building_queue",
		AccessToken: "read",
		Arguments:   map[string]any{"planetId": "99"},
	})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	structured := result.StructuredContent.(map[string]any)
	got := structured["buildingQueue"].(domainmcp.BuildingQueue)
	if result.IsError || got.Count != 1 || got.Entries[0].Name != "Metal Mine" || got.Entries[0].RemainingSeconds != 80 {
		t.Fatalf("unexpected building queue result: %+v", result)
	}
}

func TestServiceBuildingQueueToolRequiresRepositoryReadScopeAndValidArguments(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read":  {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
			"write": {Authenticated: true, PlayerID: 43, Scopes: []string{domainmcp.ScopeWrite}},
		},
	})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_building_queue", AccessToken: "read"}); err == nil {
		t.Fatalf("expected missing read repository error")
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_building_queue", AccessToken: "write"}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without read scope, got %v", err)
	}

	service = service.WithReadRepository(fakeReadRepository{})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "get_building_queue",
		AccessToken: "read",
		Arguments:   map[string]any{"planetId": true},
	}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid planet id error, got %v", err)
	}

	service = service.WithReadRepository(fakeReadRepository{err: errors.New("queue down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_building_queue", AccessToken: "read"}); err == nil {
		t.Fatalf("expected read repository error")
	}
}

func TestOptionalNonNegativeIntArgument(t *testing.T) {
	for _, tt := range []struct {
		name    string
		args    map[string]any
		want    int
		wantErr bool
	}{
		{name: "nil args"},
		{name: "missing", args: map[string]any{"other": 1}},
		{name: "nil value", args: map[string]any{"planetId": nil}},
		{name: "int", args: map[string]any{"planetId": 7}, want: 7},
		{name: "int64", args: map[string]any{"planetId": int64(8)}, want: 8},
		{name: "float64", args: map[string]any{"planetId": float64(9)}, want: 9},
		{name: "json number", args: map[string]any{"planetId": json.Number("10")}, want: 10},
		{name: "string", args: map[string]any{"planetId": "11"}, want: 11},
		{name: "negative", args: map[string]any{"planetId": -1}, wantErr: true},
		{name: "fraction", args: map[string]any{"planetId": 1.5}, wantErr: true},
		{name: "bad json number", args: map[string]any{"planetId": json.Number("bad")}, wantErr: true},
		{name: "bad string", args: map[string]any{"planetId": "bad"}, wantErr: true},
		{name: "bad type", args: map[string]any{"planetId": true}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := optionalNonNegativeIntArgument(tt.args, "planetId")
			if tt.wantErr {
				if !errors.Is(err, domainmcp.ErrInvalidParams) {
					t.Fatalf("expected invalid params error, got %v", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("expected %d without error, got %d err=%v", tt.want, got, err)
			}
		})
	}
}

func TestServiceRejectsUnknownTools(t *testing.T) {
	service := NewService(fakeHealthProvider{})

	_, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "start_building"})
	if !errors.Is(err, domainmcp.ErrToolNotFound) {
		t.Fatalf("expected ErrToolNotFound, got %v", err)
	}
}

func TestServiceAuditsToolCalls(t *testing.T) {
	auditor := &fakeToolCallAuditor{}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
		},
	}).WithToolCallAuditor(auditor)

	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_server_health"}); err != nil {
		t.Fatalf("CallTool health returned error: %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_mcp_access", AccessToken: "read"}); err != nil {
		t.Fatalf("CallTool access returned error: %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "missing"}); !errors.Is(err, domainmcp.ErrToolNotFound) {
		t.Fatalf("expected missing tool error, got %v", err)
	}

	if len(auditor.events) != 3 {
		t.Fatalf("expected three audit events, got %+v", auditor.events)
	}
	if !auditor.events[0].Authorized || auditor.events[0].ToolName != "get_server_health" {
		t.Fatalf("unexpected public audit event: %+v", auditor.events[0])
	}
	if auditor.events[1].PlayerID != 42 || !auditor.events[1].Authorized || len(auditor.events[1].Scopes) != 1 {
		t.Fatalf("unexpected access audit event: %+v", auditor.events[1])
	}
	if auditor.events[2].Error == "" || auditor.events[2].Authorized {
		t.Fatalf("unexpected missing tool audit event: %+v", auditor.events[2])
	}
}

func TestServiceManagesUserOwnedTokens(t *testing.T) {
	repository := &fakeTokenRepository{}
	service := NewServiceWithTokenManagement(
		fakeHealthProvider{},
		fakeTokenVerifier{},
		repository,
		fakeSessionLookup{auth: authenticatedSession(42)},
		fakeTokenGenerator{secret: "secret-token"},
		func() time.Time { return time.Unix(1700000000, 0) },
	)

	created, err := service.CreateToken(context.Background(), CreateTokenCommand{
		TokenManagementCommand: TokenManagementCommand{PublicSession: "pub", PrivateSessions: map[string]string{"prsess_42_1": "private"}},
		Name:                   "Claude desktop",
		Scopes:                 []string{domainmcp.ScopeRead, domainmcp.ScopeRead, domainmcp.ScopeMessages},
	})
	if err != nil {
		t.Fatalf("CreateToken returned error: %v", err)
	}
	if !created.Authenticated || created.Creation.Secret != "secret-token" {
		t.Fatalf("unexpected creation: %+v", created)
	}
	if created.Creation.Token.ID != 7 || created.Creation.Token.PlayerID != 0 {
		t.Fatalf("expected returned token id without player id, got %+v", created.Creation.Token)
	}
	if repository.created.PlayerID != 42 || repository.created.Name != "Claude desktop" || repository.created.CreatedAt != 1700000000 {
		t.Fatalf("unexpected repository token: %+v", repository.created)
	}
	if repository.hash != HashToken("secret-token") {
		t.Fatalf("expected hashed token to be stored, got %q", repository.hash)
	}

	listed, err := service.ListTokens(context.Background(), TokenManagementCommand{PublicSession: "pub"})
	if err != nil {
		t.Fatalf("ListTokens returned error: %v", err)
	}
	if !listed.Authenticated || len(listed.Tokens) != 1 || listed.Tokens[0].ID != 7 {
		t.Fatalf("unexpected token list: %+v", listed)
	}

	revoked, err := service.RevokeToken(context.Background(), RevokeTokenCommand{
		TokenManagementCommand: TokenManagementCommand{PublicSession: "pub"},
		TokenID:                7,
	})
	if err != nil {
		t.Fatalf("RevokeToken returned error: %v", err)
	}
	if !revoked.Authenticated || !revoked.Revoked || repository.revokedAt != 1700000000 {
		t.Fatalf("unexpected revoke result=%+v repository=%+v", revoked, repository)
	}
}

func TestServiceTokenManagementRejectsUnauthenticatedAndPrivilegedScopes(t *testing.T) {
	service := NewServiceWithTokenManagement(
		fakeHealthProvider{},
		nil,
		&fakeTokenRepository{},
		fakeSessionLookup{auth: domainpublicsite.SessionAuthentication{
			Authenticated: false,
			Issues:        []domainpublicsite.SessionIssue{{Code: domainpublicsite.SessionIssueInvalid, Message: "bad"}},
		}},
		fakeTokenGenerator{secret: "secret-token"},
		func() time.Time { return time.Unix(1, 0) },
	)

	result, err := service.CreateToken(context.Background(), CreateTokenCommand{})
	if err != nil {
		t.Fatalf("CreateToken returned error for unauthenticated session: %v", err)
	}
	if result.Authenticated || len(result.Issues) != 1 {
		t.Fatalf("expected unauthenticated token creation result, got %+v", result)
	}

	service.sessions = fakeSessionLookup{auth: authenticatedSession(42)}
	_, err = service.CreateToken(context.Background(), CreateTokenCommand{Scopes: []string{domainmcp.ScopeAdmin}})
	if !errors.Is(err, ErrInvalidTokenRequest) {
		t.Fatalf("expected invalid scope request, got %v", err)
	}
	_, err = service.RevokeToken(context.Background(), RevokeTokenCommand{})
	if !errors.Is(err, ErrInvalidTokenRequest) {
		t.Fatalf("expected missing token id request error, got %v", err)
	}
}

func TestServiceTokenManagementCoversErrorBranches(t *testing.T) {
	if token, err := (SecureTokenGenerator{}).NewMCPToken(); err != nil || len(token) != len("ogmcp_")+64 || token[:6] != "ogmcp_" {
		t.Fatalf("unexpected secure token secret=%q err=%v", token, err)
	}

	defaulted := NewServiceWithTokenManagement(fakeHealthProvider{}, fakeTokenVerifier{}, &fakeTokenRepository{}, fakeSessionLookup{auth: authenticatedSession(42)}, nil, nil)
	if defaulted.tokenGenerator == nil || defaulted.now == nil {
		t.Fatalf("expected default generator and clock")
	}

	_, err := (Service{}).ListTokens(context.Background(), TokenManagementCommand{})
	if err == nil {
		t.Fatalf("expected dependency error")
	}

	sessionErr := errors.New("session down")
	service := NewServiceWithTokenManagement(fakeHealthProvider{}, nil, &fakeTokenRepository{}, fakeSessionLookup{err: sessionErr}, fakeTokenGenerator{secret: "token"}, time.Now)
	if _, err := service.ListTokens(context.Background(), TokenManagementCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected session error, got %v", err)
	}

	repository := &fakeTokenRepository{listErr: errors.New("list down")}
	service = NewServiceWithTokenManagement(fakeHealthProvider{}, nil, repository, fakeSessionLookup{auth: authenticatedSession(42)}, fakeTokenGenerator{secret: "token"}, time.Now)
	if _, err := service.ListTokens(context.Background(), TokenManagementCommand{}); !errors.Is(err, repository.listErr) {
		t.Fatalf("expected list error, got %v", err)
	}

	if _, err := (Service{}).CreateToken(context.Background(), CreateTokenCommand{}); err == nil {
		t.Fatalf("expected create dependency error")
	}
	service = NewServiceWithTokenManagement(fakeHealthProvider{}, nil, &fakeTokenRepository{}, fakeSessionLookup{err: sessionErr}, fakeTokenGenerator{secret: "token"}, time.Now)
	if _, err := service.CreateToken(context.Background(), CreateTokenCommand{}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected create session error, got %v", err)
	}

	service = NewServiceWithTokenManagement(fakeHealthProvider{}, nil, &fakeTokenRepository{}, fakeSessionLookup{auth: authenticatedSession(42)}, fakeTokenGenerator{err: errors.New("random down")}, time.Now)
	if _, err := service.CreateToken(context.Background(), CreateTokenCommand{}); err == nil {
		t.Fatalf("expected generator error")
	}

	service = NewServiceWithTokenManagement(fakeHealthProvider{}, nil, &fakeTokenRepository{createErr: errors.New("insert down")}, fakeSessionLookup{auth: authenticatedSession(42)}, fakeTokenGenerator{secret: "token"}, time.Now)
	if _, err := service.CreateToken(context.Background(), CreateTokenCommand{}); err == nil {
		t.Fatalf("expected create repository error")
	}

	service = NewServiceWithTokenManagement(fakeHealthProvider{}, nil, &fakeTokenRepository{}, fakeSessionLookup{auth: authenticatedSession(42)}, fakeTokenGenerator{secret: "token"}, time.Now)
	if _, err := service.CreateToken(context.Background(), CreateTokenCommand{Name: strings.Repeat("x", 65)}); !errors.Is(err, ErrInvalidTokenRequest) {
		t.Fatalf("expected long name validation error, got %v", err)
	}

	if _, err := (Service{}).RevokeToken(context.Background(), RevokeTokenCommand{TokenID: 7}); err == nil {
		t.Fatalf("expected revoke dependency error")
	}
	service = NewServiceWithTokenManagement(fakeHealthProvider{}, nil, &fakeTokenRepository{}, fakeSessionLookup{err: sessionErr}, fakeTokenGenerator{secret: "token"}, time.Now)
	if _, err := service.RevokeToken(context.Background(), RevokeTokenCommand{TokenID: 7}); !errors.Is(err, sessionErr) {
		t.Fatalf("expected revoke session error, got %v", err)
	}

	service = NewServiceWithTokenManagement(fakeHealthProvider{}, nil, &fakeTokenRepository{revokeErr: errors.New("update down")}, fakeSessionLookup{auth: authenticatedSession(42)}, fakeTokenGenerator{secret: "token"}, time.Now)
	if _, err := service.RevokeToken(context.Background(), RevokeTokenCommand{TokenID: 7}); err == nil {
		t.Fatalf("expected revoke repository error")
	}

	service = NewServiceWithTokenManagement(fakeHealthProvider{}, nil, &fakeTokenRepository{}, fakeSessionLookup{auth: domainpublicsite.SessionAuthentication{Authenticated: false, Issues: []domainpublicsite.SessionIssue{{Code: domainpublicsite.SessionIssueInvalid}}}}, fakeTokenGenerator{secret: "token"}, time.Now)
	revoke, err := service.RevokeToken(context.Background(), RevokeTokenCommand{TokenID: 7})
	if err != nil || revoke.Authenticated || len(revoke.Issues) != 1 {
		t.Fatalf("expected unauthenticated revoke result, got result=%+v err=%v", revoke, err)
	}

	service = NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{access: map[string]domainmcp.Access{"noauth": {Authenticated: false}}})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_mcp_access", AccessToken: "noauth"}); !errors.Is(err, domainmcp.ErrUnauthorized) {
		t.Fatalf("expected inactive verifier access to be unauthorized, got %v", err)
	}
}

type fakeHealthProvider struct{}

func (fakeHealthProvider) Get(context.Context) domainsystem.Health {
	return domainsystem.Health{
		Status:      "ok",
		Service:     "ogame-go",
		Environment: "test",
		Runtime:     "go-test",
		Targets: domainsystem.RuntimeTargets{
			Go:    "1.25",
			Bun:   "1.3",
			React: "19",
		},
		StaticReady:       true,
		LegacyAssetsReady: true,
		LegacyBaseURL:     "http://legacy.local",
	}
}

type fakeTokenVerifier struct {
	access map[string]domainmcp.Access
}

func (f fakeTokenVerifier) VerifyMCPToken(_ context.Context, token string) (domainmcp.Access, error) {
	access, ok := f.access[token]
	if !ok {
		return domainmcp.Access{}, domainmcp.ErrUnauthorized
	}
	return access, nil
}

type fakeToolCallAuditor struct {
	events []domainmcp.ToolCallAudit
}

func (f *fakeToolCallAuditor) RecordMCPToolCall(_ context.Context, event domainmcp.ToolCallAudit) {
	f.events = append(f.events, event)
}

type fakeReadRepository struct {
	planets               []domainmcp.Planet
	overview              domainmcp.AccountOverview
	resources             domainmcp.PlanetResources
	resourcesPlanetID     int
	buildingQueue         domainmcp.BuildingQueue
	buildingQueuePlanetID int
	err                   error
}

func (f fakeReadRepository) ListMCPPlanets(_ context.Context, playerID int) ([]domainmcp.Planet, error) {
	if f.err != nil {
		return nil, f.err
	}
	if playerID != 42 {
		return nil, errors.New("unexpected player")
	}
	return f.planets, nil
}

func (f fakeReadRepository) GetMCPAccountOverview(_ context.Context, playerID int) (domainmcp.AccountOverview, error) {
	if f.err != nil {
		return domainmcp.AccountOverview{}, f.err
	}
	if playerID != 42 {
		return domainmcp.AccountOverview{}, errors.New("unexpected player")
	}
	return f.overview, nil
}

func (f fakeReadRepository) GetMCPPlanetResources(_ context.Context, playerID int, planetID int) (domainmcp.PlanetResources, error) {
	if f.err != nil {
		return domainmcp.PlanetResources{}, f.err
	}
	if playerID != 42 {
		return domainmcp.PlanetResources{}, errors.New("unexpected player")
	}
	if planetID != f.resourcesPlanetID {
		return domainmcp.PlanetResources{}, errors.New("unexpected planet")
	}
	return f.resources, nil
}

func (f fakeReadRepository) GetMCPBuildingQueue(_ context.Context, playerID int, planetID int) (domainmcp.BuildingQueue, error) {
	if f.err != nil {
		return domainmcp.BuildingQueue{}, f.err
	}
	if playerID != 42 {
		return domainmcp.BuildingQueue{}, errors.New("unexpected player")
	}
	if planetID != f.buildingQueuePlanetID {
		return domainmcp.BuildingQueue{}, errors.New("unexpected planet")
	}
	return f.buildingQueue, nil
}

type fakeSessionLookup struct {
	auth domainpublicsite.SessionAuthentication
	err  error
}

func (f fakeSessionLookup) GetGameSession(context.Context, apppublicsite.GameSessionCommand) (domainpublicsite.SessionAuthentication, error) {
	return f.auth, f.err
}

type fakeTokenGenerator struct {
	secret string
	err    error
}

func (f fakeTokenGenerator) NewMCPToken() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.secret, nil
}

type fakeTokenRepository struct {
	created   domainmcp.Token
	hash      string
	revokedAt int64
	listErr   error
	createErr error
	revokeErr error
}

func (f *fakeTokenRepository) ListMCPTokens(context.Context, int) ([]domainmcp.Token, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return []domainmcp.Token{{ID: 7, Name: "Claude desktop", Scopes: []string{domainmcp.ScopeRead}}}, nil
}

func (f *fakeTokenRepository) CreateMCPToken(_ context.Context, token domainmcp.Token, hash string) (domainmcp.Token, error) {
	if f.createErr != nil {
		return domainmcp.Token{}, f.createErr
	}
	f.created = token
	f.hash = hash
	token.ID = 7
	return token, nil
}

func (f *fakeTokenRepository) RevokeMCPToken(_ context.Context, playerID int, tokenID int, revokedAt int64) (bool, error) {
	if f.revokeErr != nil {
		return false, f.revokeErr
	}
	if playerID != 42 || tokenID != 7 {
		return false, nil
	}
	f.revokedAt = revokedAt
	return true, nil
}

func authenticatedSession(playerID int) domainpublicsite.SessionAuthentication {
	return domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session: domainpublicsite.GameSession{
			Found:          true,
			PlayerID:       playerID,
			PublicID:       "pub",
			PrivateID:      "private",
			UniverseNumber: 1,
		},
	}
}
