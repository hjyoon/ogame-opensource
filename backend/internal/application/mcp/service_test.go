package mcp

import (
	"context"
	"errors"
	"testing"

	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
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

func TestServiceRejectsUnknownTools(t *testing.T) {
	service := NewService(fakeHealthProvider{})

	_, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "start_building"})
	if !errors.Is(err, domainmcp.ErrToolNotFound) {
		t.Fatalf("expected ErrToolNotFound, got %v", err)
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
