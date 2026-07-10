package mcp

import (
	"context"
	"encoding/json"
	"strings"

	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
	domainsystem "github.com/hjyoon/ogame-opensource/backend/internal/domain/system"
)

type HealthProvider interface {
	Get(context.Context) domainsystem.Health
}

type TokenVerifier interface {
	VerifyMCPToken(context.Context, string) (domainmcp.Access, error)
}

type Service struct {
	health   HealthProvider
	verifier TokenVerifier
}

func NewService(health HealthProvider) Service {
	return Service{health: health}
}

func NewServiceWithTokenVerifier(health HealthProvider, verifier TokenVerifier) Service {
	return Service{health: health, verifier: verifier}
}

func (s Service) Initialize(ctx context.Context) domainmcp.InitializeResult {
	_ = ctx
	return domainmcp.InitializeResult{
		ProtocolVersion: domainmcp.ProtocolVersion,
		Capabilities: domainmcp.Capabilities{
			Tools: &domainmcp.ToolsCapability{ListChanged: false},
		},
		ServerInfo: domainmcp.ServerInfo{
			Name:    "ogame-opensource",
			Title:   "OGame Open Source",
			Version: domainmcp.ProtocolVersion,
		},
	}
}

func (s Service) ListTools(ctx context.Context, command domainmcp.ListToolsCommand) (domainmcp.ListToolsResult, error) {
	tools := []domainmcp.Tool{serverHealthTool()}
	if strings.TrimSpace(command.AccessToken) == "" {
		return domainmcp.ListToolsResult{Tools: tools}, nil
	}
	access, err := s.verify(ctx, command.AccessToken)
	if err != nil {
		return domainmcp.ListToolsResult{}, err
	}
	if access.HasScope(domainmcp.ScopeRead) {
		tools = append(tools, accessTool())
	}
	return domainmcp.ListToolsResult{Tools: tools}, nil
}

func (s Service) CallTool(ctx context.Context, command domainmcp.CallToolCommand) (domainmcp.ToolCallResult, error) {
	switch command.Name {
	case "get_server_health":
		return s.callServerHealth(ctx)
	case "get_mcp_access":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		return callMCPAccess(access), nil
	default:
		return domainmcp.ToolCallResult{}, domainmcp.ErrToolNotFound
	}
}

func (s Service) callServerHealth(ctx context.Context) (domainmcp.ToolCallResult, error) {
	health := s.health.Get(ctx)
	structured := map[string]any{
		"status":            health.Status,
		"service":           health.Service,
		"environment":       health.Environment,
		"runtime":           health.Runtime,
		"goTarget":          health.Targets.Go,
		"bunTarget":         health.Targets.Bun,
		"reactTarget":       health.Targets.React,
		"staticReady":       health.StaticReady,
		"legacyAssetsReady": health.LegacyAssetsReady,
		"legacyBaseUrl":     health.LegacyBaseURL,
	}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) authorize(ctx context.Context, token string, scope string) (domainmcp.Access, error) {
	access, err := s.verify(ctx, token)
	if err != nil {
		return domainmcp.Access{}, err
	}
	if !access.HasScope(scope) {
		return domainmcp.Access{}, domainmcp.ErrForbidden
	}
	return access, nil
}

func (s Service) verify(ctx context.Context, token string) (domainmcp.Access, error) {
	token = strings.TrimSpace(token)
	if token == "" || s.verifier == nil {
		return domainmcp.Access{}, domainmcp.ErrUnauthorized
	}
	access, err := s.verifier.VerifyMCPToken(ctx, token)
	if err != nil {
		return domainmcp.Access{}, err
	}
	if !access.Authenticated {
		return domainmcp.Access{}, domainmcp.ErrUnauthorized
	}
	return access, nil
}

func callMCPAccess(access domainmcp.Access) domainmcp.ToolCallResult {
	structured := map[string]any{
		"authenticated": access.Authenticated,
		"playerId":      access.PlayerID,
		"scopes":        access.Scopes,
	}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}
}

func serverHealthTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_server_health",
		Title:       "Get Server Health",
		Description: "Return public Go migration runtime and readiness information for this OGame server.",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status":            map[string]any{"type": "string"},
				"service":           map[string]any{"type": "string"},
				"environment":       map[string]any{"type": "string"},
				"runtime":           map[string]any{"type": "string"},
				"goTarget":          map[string]any{"type": "string"},
				"bunTarget":         map[string]any{"type": "string"},
				"reactTarget":       map[string]any{"type": "string"},
				"staticReady":       map[string]any{"type": "boolean"},
				"legacyAssetsReady": map[string]any{"type": "boolean"},
				"legacyBaseUrl":     map[string]any{"type": "string"},
			},
			"required": []string{"status", "service", "environment", "runtime"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func accessTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_mcp_access",
		Title:       "Get MCP Access",
		Description: "Return the authenticated MCP player id and granted scopes for the current bearer token.",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"authenticated": map[string]any{"type": "boolean"},
				"playerId":      map[string]any{"type": "integer"},
				"scopes": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
			},
			"required": []string{"authenticated", "playerId", "scopes"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}
