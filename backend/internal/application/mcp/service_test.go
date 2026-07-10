package mcp

import (
	"context"
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
