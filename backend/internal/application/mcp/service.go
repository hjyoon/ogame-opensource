package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
	domainsystem "github.com/hjyoon/ogame-opensource/backend/internal/domain/system"
)

var ErrInvalidTokenRequest = errors.New("invalid mcp token request")

type HealthProvider interface {
	Get(context.Context) domainsystem.Health
}

type TokenVerifier interface {
	VerifyMCPToken(context.Context, string) (domainmcp.Access, error)
}

type ToolCallAuditor interface {
	RecordMCPToolCall(context.Context, domainmcp.ToolCallAudit)
}

type SessionLookup interface {
	GetGameSession(context.Context, apppublicsite.GameSessionCommand) (domainpublicsite.SessionAuthentication, error)
}

type TokenRepository interface {
	ListMCPTokens(context.Context, int) ([]domainmcp.Token, error)
	CreateMCPToken(context.Context, domainmcp.Token, string) (domainmcp.Token, error)
	RevokeMCPToken(context.Context, int, int, int64) (bool, error)
}

type ReadRepository interface {
	ListMCPPlanets(context.Context, int) ([]domainmcp.Planet, error)
	GetMCPAccountOverview(context.Context, int) (domainmcp.AccountOverview, error)
	GetMCPPlanetResources(context.Context, int, int) (domainmcp.PlanetResources, error)
	GetMCPBuildingQueue(context.Context, int, int) (domainmcp.BuildingQueue, error)
	GetMCPFleetMovements(context.Context, int) (domainmcp.FleetMovements, error)
}

type TokenSecretGenerator interface {
	NewMCPToken() (string, error)
}

type SecureTokenGenerator struct{}

func (SecureTokenGenerator) NewMCPToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "ogmcp_" + hex.EncodeToString(bytes), nil
}

type TokenManagementCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
}

type CreateTokenCommand struct {
	TokenManagementCommand
	Name   string
	Scopes []string
}

type RevokeTokenCommand struct {
	TokenManagementCommand
	TokenID int
}

type TokenListResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Tokens        []domainmcp.Token
}

type TokenCreationResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Creation      domainmcp.TokenCreation
}

type TokenRevokeResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Revoked       bool
}

type Service struct {
	health          HealthProvider
	verifier        TokenVerifier
	tokenRepository TokenRepository
	readRepository  ReadRepository
	sessions        SessionLookup
	tokenGenerator  TokenSecretGenerator
	auditor         ToolCallAuditor
	now             func() time.Time
}

func NewService(health HealthProvider) Service {
	return Service{health: health, now: time.Now}
}

func NewServiceWithTokenVerifier(health HealthProvider, verifier TokenVerifier) Service {
	return Service{health: health, verifier: verifier, now: time.Now}
}

func NewServiceWithTokenManagement(health HealthProvider, verifier TokenVerifier, repository TokenRepository, sessions SessionLookup, generator TokenSecretGenerator, now func() time.Time) Service {
	if generator == nil {
		generator = SecureTokenGenerator{}
	}
	if now == nil {
		now = time.Now
	}
	return Service{
		health:          health,
		verifier:        verifier,
		tokenRepository: repository,
		sessions:        sessions,
		tokenGenerator:  generator,
		now:             now,
	}
}

func (s Service) WithReadRepository(repository ReadRepository) Service {
	s.readRepository = repository
	return s
}

func (s Service) WithToolCallAuditor(auditor ToolCallAuditor) Service {
	s.auditor = auditor
	return s
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

func (s Service) OAuthAuthorizationServerMetadata(ctx context.Context, issuer string) domainmcp.OAuthAuthorizationServerMetadata {
	_ = ctx
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	return domainmcp.OAuthAuthorizationServerMetadata{
		Issuer:                            issuer,
		AuthorizationEndpoint:             issuer + "/oauth/authorize",
		TokenEndpoint:                     issuer + "/oauth/token",
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code"},
		CodeChallengeMethodsSupported:     []string{"S256"},
		TokenEndpointAuthMethodsSupported: []string{"none"},
		ScopesSupported: []string{
			"openid",
			"profile",
			domainmcp.ScopeRead,
			domainmcp.ScopeMessages,
			domainmcp.ScopeFleet,
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
		if s.readRepository != nil {
			tools = append(tools, listPlanetsTool(), accountOverviewTool(), planetResourcesTool(), buildingQueueTool(), fleetMovementsTool())
		}
	}
	return domainmcp.ListToolsResult{Tools: tools}, nil
}

func (s Service) CallTool(ctx context.Context, command domainmcp.CallToolCommand) (result domainmcp.ToolCallResult, err error) {
	started := s.now()
	audit := domainmcp.ToolCallAudit{ToolName: command.Name, At: started.Unix()}
	defer func() {
		audit.DurationMS = s.now().Sub(started).Milliseconds()
		if err != nil {
			audit.Error = err.Error()
		}
		s.auditToolCall(ctx, audit)
	}()

	switch command.Name {
	case "get_server_health":
		audit.Authorized = true
		return s.callServerHealth(ctx)
	case "get_mcp_access":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return callMCPAccess(access), nil
	case "list_planets":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callListPlanets(ctx, access)
	case "get_account_overview":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callAccountOverview(ctx, access)
	case "get_planet_resources":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callPlanetResources(ctx, access, command.Arguments)
	case "get_building_queue":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callBuildingQueue(ctx, access, command.Arguments)
	case "get_fleet_movements":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callFleetMovements(ctx, access)
	default:
		return domainmcp.ToolCallResult{}, domainmcp.ErrToolNotFound
	}
}

func (s Service) ListTokens(ctx context.Context, command TokenManagementCommand) (TokenListResult, error) {
	if s.sessions == nil || s.tokenRepository == nil {
		return TokenListResult{}, errors.New("mcp token management dependencies unavailable")
	}
	session, err := s.authenticateSession(ctx, command)
	if err != nil {
		return TokenListResult{}, err
	}
	if !session.Authenticated {
		return TokenListResult{Authenticated: false, Issues: session.Issues}, nil
	}
	tokens, err := s.tokenRepository.ListMCPTokens(ctx, session.Session.PlayerID)
	if err != nil {
		return TokenListResult{}, err
	}
	return TokenListResult{Authenticated: true, Tokens: tokens}, nil
}

func (s Service) CreateToken(ctx context.Context, command CreateTokenCommand) (TokenCreationResult, error) {
	if s.sessions == nil || s.tokenRepository == nil || s.tokenGenerator == nil {
		return TokenCreationResult{}, errors.New("mcp token management dependencies unavailable")
	}
	session, err := s.authenticateSession(ctx, command.TokenManagementCommand)
	if err != nil {
		return TokenCreationResult{}, err
	}
	if !session.Authenticated {
		return TokenCreationResult{Authenticated: false, Issues: session.Issues}, nil
	}
	name, scopes, err := normalizeTokenRequest(command.Name, command.Scopes)
	if err != nil {
		return TokenCreationResult{}, err
	}
	secret, err := s.tokenGenerator.NewMCPToken()
	if err != nil {
		return TokenCreationResult{}, err
	}
	now := s.now().Unix()
	token, err := s.tokenRepository.CreateMCPToken(ctx, domainmcp.Token{
		PlayerID:  session.Session.PlayerID,
		Name:      name,
		Scopes:    scopes,
		CreatedAt: now,
	}, HashToken(secret))
	if err != nil {
		return TokenCreationResult{}, err
	}
	token.PlayerID = 0
	return TokenCreationResult{
		Authenticated: true,
		Creation:      domainmcp.TokenCreation{Token: token, Secret: secret},
	}, nil
}

func (s Service) RevokeToken(ctx context.Context, command RevokeTokenCommand) (TokenRevokeResult, error) {
	if s.sessions == nil || s.tokenRepository == nil {
		return TokenRevokeResult{}, errors.New("mcp token management dependencies unavailable")
	}
	if command.TokenID <= 0 {
		return TokenRevokeResult{}, fmt.Errorf("%w: token id is required", ErrInvalidTokenRequest)
	}
	session, err := s.authenticateSession(ctx, command.TokenManagementCommand)
	if err != nil {
		return TokenRevokeResult{}, err
	}
	if !session.Authenticated {
		return TokenRevokeResult{Authenticated: false, Issues: session.Issues}, nil
	}
	revoked, err := s.tokenRepository.RevokeMCPToken(ctx, session.Session.PlayerID, command.TokenID, s.now().Unix())
	if err != nil {
		return TokenRevokeResult{}, err
	}
	return TokenRevokeResult{Authenticated: true, Revoked: revoked}, nil
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

func (s Service) callListPlanets(ctx context.Context, access domainmcp.Access) (domainmcp.ToolCallResult, error) {
	if s.readRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp read repository unavailable")
	}
	planets, err := s.readRepository.ListMCPPlanets(ctx, access.PlayerID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{
		"playerId": access.PlayerID,
		"count":    len(planets),
		"planets":  planets,
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

func (s Service) callAccountOverview(ctx context.Context, access domainmcp.Access) (domainmcp.ToolCallResult, error) {
	if s.readRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp read repository unavailable")
	}
	overview, err := s.readRepository.GetMCPAccountOverview(ctx, access.PlayerID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"overview": overview}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callPlanetResources(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.readRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp read repository unavailable")
	}
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	resources, err := s.readRepository.GetMCPPlanetResources(ctx, access.PlayerID, planetID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"resources": resources}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callBuildingQueue(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.readRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp read repository unavailable")
	}
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	queue, err := s.readRepository.GetMCPBuildingQueue(ctx, access.PlayerID, planetID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"buildingQueue": queue}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callFleetMovements(ctx context.Context, access domainmcp.Access) (domainmcp.ToolCallResult, error) {
	if s.readRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp read repository unavailable")
	}
	movements, err := s.readRepository.GetMCPFleetMovements(ctx, access.PlayerID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"fleetMovements": movements}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) authenticateSession(ctx context.Context, command TokenManagementCommand) (domainpublicsite.SessionAuthentication, error) {
	return s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
}

func (s Service) auditToolCall(ctx context.Context, audit domainmcp.ToolCallAudit) {
	if s.auditor == nil {
		return
	}
	s.auditor.RecordMCPToolCall(ctx, audit)
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

func HashToken(secret string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(sum[:])
}

func normalizeTokenRequest(name string, requested []string) (string, []string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "MCP token"
	}
	if len(name) > 64 {
		return "", nil, fmt.Errorf("%w: name is too long", ErrInvalidTokenRequest)
	}
	scopes := normalizeUserScopes(requested)
	if len(scopes) == 0 {
		scopes = []string{domainmcp.ScopeRead}
	}
	for _, scope := range scopes {
		if !userScopeAllowed(scope) {
			return "", nil, fmt.Errorf("%w: scope %q is not available for user tokens", ErrInvalidTokenRequest, scope)
		}
	}
	return name, scopes, nil
}

func normalizeUserScopes(requested []string) []string {
	seen := map[string]bool{}
	scopes := make([]string, 0, len(requested))
	for _, raw := range requested {
		scope := strings.TrimSpace(raw)
		if scope == "" || seen[scope] {
			continue
		}
		seen[scope] = true
		scopes = append(scopes, scope)
	}
	return scopes
}

func userScopeAllowed(scope string) bool {
	switch scope {
	case domainmcp.ScopeRead, domainmcp.ScopeMessages, domainmcp.ScopeFleet:
		return true
	default:
		return false
	}
}

func optionalNonNegativeIntArgument(arguments map[string]any, name string) (int, error) {
	if arguments == nil {
		return 0, nil
	}
	raw, ok := arguments[name]
	if !ok || raw == nil {
		return 0, nil
	}
	maxIntValue := int64(^uint(0) >> 1)
	var value int64
	switch typed := raw.(type) {
	case int:
		value = int64(typed)
	case int64:
		value = typed
	case float64:
		if math.Trunc(typed) != typed {
			return 0, fmt.Errorf("%w: %s must be an integer", domainmcp.ErrInvalidParams, name)
		}
		if typed < 0 || typed > float64(maxIntValue) {
			return 0, fmt.Errorf("%w: %s must be a non-negative integer", domainmcp.ErrInvalidParams, name)
		}
		value = int64(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, fmt.Errorf("%w: %s must be an integer", domainmcp.ErrInvalidParams, name)
		}
		value = parsed
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: %s must be an integer", domainmcp.ErrInvalidParams, name)
		}
		value = parsed
	default:
		return 0, fmt.Errorf("%w: %s must be an integer", domainmcp.ErrInvalidParams, name)
	}
	if value < 0 || value > maxIntValue {
		return 0, fmt.Errorf("%w: %s must be a non-negative integer", domainmcp.ErrInvalidParams, name)
	}
	return int(value), nil
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

func listPlanetsTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "list_planets",
		Title:       "List Planets",
		Description: "Return the authenticated player's selectable planets and moons using the legacy planet switcher ordering.",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"playerId": map[string]any{"type": "integer"},
				"count":    map[string]any{"type": "integer"},
				"planets": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":       map[string]any{"type": "integer"},
							"name":     map[string]any{"type": "string"},
							"type":     map[string]any{"type": "integer"},
							"typeName": map[string]any{"type": "string"},
							"coordinates": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"galaxy":   map[string]any{"type": "integer"},
									"system":   map[string]any{"type": "integer"},
									"position": map[string]any{"type": "integer"},
								},
								"required": []string{"galaxy", "system", "position"},
							},
							"current": map[string]any{"type": "boolean"},
						},
						"required": []string{"id", "name", "type", "typeName", "coordinates", "current"},
					},
				},
			},
			"required": []string{"playerId", "count", "planets"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func accountOverviewTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_account_overview",
		Title:       "Get Account Overview",
		Description: "Return read-only account summary data for the authenticated player without triggering legacy overview mutations.",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"overview": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":       map[string]any{"type": "integer"},
						"commander":      map[string]any{"type": "string"},
						"planetCount":    map[string]any{"type": "integer"},
						"unreadMessages": map[string]any{"type": "integer"},
						"score": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"raw":     map[string]any{"type": "integer"},
								"display": map[string]any{"type": "integer"},
								"rank":    map[string]any{"type": "integer"},
							},
							"required": []string{"raw", "display", "rank"},
						},
						"currentPlanet": map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "commander", "score", "currentPlanet", "planetCount", "unreadMessages"},
				},
			},
			"required": []string{"overview"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func planetResourcesTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_planet_resources",
		Title:       "Get Planet Resources",
		Description: "Return read-only resource, storage, energy, and hourly production values for the current or requested owned planet.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"minimum":     0,
					"description": "Optional owned planet id. Omit or pass 0 to use the current active planet.",
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"resources": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":          map[string]any{"type": "integer"},
						"planet":            map[string]any{"type": "object"},
						"resources":         map[string]any{"type": "object"},
						"capacity":          map[string]any{"type": "object"},
						"energy":            map[string]any{"type": "object"},
						"productionPerHour": map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planet", "resources", "capacity", "energy", "productionPerHour"},
				},
			},
			"required": []string{"resources"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func buildingQueueTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_building_queue",
		Title:       "Get Building Queue",
		Description: "Return read-only building queue entries for the current or requested owned planet without finishing or mutating queued work.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"minimum":     0,
					"description": "Optional owned planet id. Omit or pass 0 to use the current active planet.",
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"buildingQueue": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId": map[string]any{"type": "integer"},
						"planet":   map[string]any{"type": "object"},
						"count":    map[string]any{"type": "integer"},
						"entries": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"listId":           map[string]any{"type": "integer"},
									"techId":           map[string]any{"type": "integer"},
									"name":             map[string]any{"type": "string"},
									"level":            map[string]any{"type": "integer"},
									"destroy":          map[string]any{"type": "boolean"},
									"start":            map[string]any{"type": "integer"},
									"end":              map[string]any{"type": "integer"},
									"remainingSeconds": map[string]any{"type": "integer"},
								},
								"required": []string{"listId", "techId", "name", "level", "destroy", "start", "end", "remainingSeconds"},
							},
						},
					},
					"required": []string{"playerId", "planet", "count", "entries"},
				},
			},
			"required": []string{"buildingQueue"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func fleetMovementsTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_fleet_movements",
		Title:       "Get Fleet Movements",
		Description: "Return read-only overview-style fleet movements for the authenticated player, including incoming, outgoing, return, hold, and ACS grouped events.",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"fleetMovements": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId": map[string]any{"type": "integer"},
						"now":      map[string]any{"type": "integer"},
						"count":    map[string]any{"type": "integer"},
						"events": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "object"},
						},
					},
					"required": []string{"playerId", "now", "count", "events"},
				},
			},
			"required": []string{"fleetMovements"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}
