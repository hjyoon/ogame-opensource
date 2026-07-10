package httpdelivery

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appmcp "github.com/hjyoon/ogame-opensource/backend/internal/application/mcp"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

func TestMCPOAuthAuthorizationServerMetadata(t *testing.T) {
	server := New(Dependencies{MCPOAuth: &fakeMCPOAuthUseCase{}})
	req := httptest.NewRequest(http.MethodGet, "http://game.local/.well-known/oauth-authorization-server", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected metadata success, got status=%d body=%q", rec.Code, rec.Body.String())
	}
	var body domainmcp.OAuthAuthorizationServerMetadata
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if body.Issuer != "https://game.local" || body.AuthorizationEndpoint != "https://game.local/oauth/authorize" || body.TokenEndpoint != "https://game.local/oauth/token" {
		t.Fatalf("unexpected metadata: %+v", body)
	}
	if body.ResponseTypesSupported[0] != "code" || body.CodeChallengeMethodsSupported[0] != "S256" || body.TokenEndpointAuthMethodsSupported[0] != "none" {
		t.Fatalf("unexpected OAuth 2.1 metadata: %+v", body)
	}

	server = New(Dependencies{MCPOAuth: &fakeMCPOAuthUseCase{jwksResult: domainmcp.JSONWebKeySet{Keys: []domainmcp.JSONWebKey{{KeyType: "OKP", KeyID: "kid"}}}}})
	req = httptest.NewRequest(http.MethodGet, "http://game.local/.well-known/jwks.json", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "public, max-age=300" {
		t.Fatalf("unexpected JWKS response status=%d headers=%v body=%q", rec.Code, rec.Header(), rec.Body.String())
	}
	var jwks domainmcp.JSONWebKeySet
	if err := json.Unmarshal(rec.Body.Bytes(), &jwks); err != nil {
		t.Fatalf("decode JWKS: %v", err)
	}
	if len(jwks.Keys) != 1 || jwks.Keys[0].KeyID != "kid" {
		t.Fatalf("unexpected JWKS: %+v", jwks)
	}
}

func TestMCPOAuthAuthorizeConsentRedirectAndToken(t *testing.T) {
	oauth := &fakeMCPOAuthUseCase{
		authorizeResult: appmcp.OAuthAuthorizeResult{
			Authenticated:   true,
			RequiresConsent: true,
			ClientID:        "desktop",
			Scopes:          []string{domainmcp.ScopeRead, domainmcp.ScopeFleet},
		},
		exchangeResult: appmcp.OAuthTokenResult{
			AccessToken: "ogmcp_access",
			TokenType:   "Bearer",
			Scope:       "mcp:read mcp:fleet",
			IDToken:     "id.token.signature",
		},
		revokeResult: appmcp.OAuthRevokeResult{Revoked: true},
	}
	server := New(Dependencies{MCPOAuth: oauth})

	req := httptest.NewRequest(http.MethodGet, "http://game.local/oauth/authorize?response_type=code&client_id=desktop&redirect_uri=http://127.0.0.1:9000/callback&scope=mcp:read+mcp:fleet&state=s1&code_challenge="+strings.Repeat("a", 43)+"&code_challenge_method=S256&session=pub", nil)
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Authorize MCP access") || oauth.authorizeCommand.PublicSession != "pub" || oauth.authorizeCommand.ClientID != "desktop" || oauth.authorizeCommand.ConsentApproved {
		t.Fatalf("unexpected consent response status=%d body=%q command=%+v", rec.Code, rec.Body.String(), oauth.authorizeCommand)
	}

	oauth.authorizeResult = appmcp.OAuthAuthorizeResult{Authenticated: true, RedirectTo: "http://127.0.0.1:9000/callback?code=abc&state=s1"}
	req = httptest.NewRequest(http.MethodGet, "http://game.local/oauth/authorize?response_type=code&client_id=desktop&redirect_uri=http://127.0.0.1:9000/callback&code_challenge="+strings.Repeat("a", 43)+"&code_challenge_method=S256&consent=approve", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "http://127.0.0.1:9000/callback?code=abc&state=s1" || !oauth.authorizeCommand.ConsentApproved {
		t.Fatalf("unexpected authorize redirect status=%d location=%q command=%+v", rec.Code, rec.Header().Get("Location"), oauth.authorizeCommand)
	}

	req = httptest.NewRequest(http.MethodPost, "http://game.local/oauth/token", strings.NewReader("grant_type=authorization_code&code=abc&redirect_uri=http%3A%2F%2F127.0.0.1%3A9000%2Fcallback&client_id=desktop&code_verifier="+strings.Repeat("b", 43)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || oauth.tokenCommand.Code != "abc" || oauth.tokenCommand.ClientID != "desktop" {
		t.Fatalf("unexpected token response status=%d body=%q command=%+v", rec.Code, rec.Body.String(), oauth.tokenCommand)
	}
	var tokenBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &tokenBody); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if tokenBody["access_token"] != "ogmcp_access" || tokenBody["token_type"] != "Bearer" || tokenBody["id_token"] != "id.token.signature" {
		t.Fatalf("unexpected token response: %+v", tokenBody)
	}

	req = httptest.NewRequest(http.MethodPost, "http://game.local/oauth/revoke", strings.NewReader("token=ogmcp_access&token_type_hint=access_token"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || oauth.revokeCommand.Token != "ogmcp_access" || oauth.revokeCommand.TokenTypeHint != "access_token" {
		t.Fatalf("unexpected revoke response status=%d body=%q command=%+v", rec.Code, rec.Body.String(), oauth.revokeCommand)
	}
	var revokeBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &revokeBody); err != nil {
		t.Fatalf("decode revoke response: %v", err)
	}
	if revokeBody["revoked"] != true {
		t.Fatalf("unexpected revoke body: %+v", revokeBody)
	}
}

func TestMCPOAuthErrors(t *testing.T) {
	server := New(Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "http://game.local/oauth/authorize", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected unavailable authorize, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "http://game.local/oauth/token", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected unavailable token, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "http://game.local/oauth/revoke", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected unavailable revoke, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	server = New(Dependencies{MCPOAuth: &fakeMCPOAuthUseCase{}})
	req = httptest.NewRequest(http.MethodPost, "http://game.local/oauth/revoke", strings.NewReader("%"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_request") {
		t.Fatalf("expected malformed revoke request, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	server = New(Dependencies{MCPOAuth: &fakeMCPOAuthUseCase{authorizeResult: appmcp.OAuthAuthorizeResult{Authenticated: false}}})
	req = httptest.NewRequest(http.MethodGet, "http://game.local/oauth/authorize?response_type=code", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "Login is required") {
		t.Fatalf("expected login-required authorize, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	server = New(Dependencies{MCPOAuth: &fakeMCPOAuthUseCase{authorizeErr: appmcp.ErrInvalidOAuthRequest}})
	req = httptest.NewRequest(http.MethodGet, "http://game.local/oauth/authorize", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_request") {
		t.Fatalf("expected invalid authorize, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	server = New(Dependencies{MCPOAuth: &fakeMCPOAuthUseCase{authorizeErr: errors.New("down")}})
	req = httptest.NewRequest(http.MethodGet, "http://game.local/oauth/authorize", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "temporarily_unavailable") {
		t.Fatalf("expected unavailable authorize error, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	server = New(Dependencies{MCPOAuth: &fakeMCPOAuthUseCase{exchangeErr: appmcp.ErrInvalidOAuthGrant}})
	req = httptest.NewRequest(http.MethodPost, "http://game.local/oauth/token", strings.NewReader("grant_type=authorization_code"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_grant") {
		t.Fatalf("expected invalid grant, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	server = New(Dependencies{MCPOAuth: &fakeMCPOAuthUseCase{exchangeErr: errors.New("down")}})
	req = httptest.NewRequest(http.MethodPost, "http://game.local/oauth/token", strings.NewReader("%"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_request") {
		t.Fatalf("expected malformed token request, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "http://game.local/oauth/token", strings.NewReader("grant_type=authorization_code"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "temporarily_unavailable") {
		t.Fatalf("expected unavailable token error, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	server = New(Dependencies{MCPOAuth: &fakeMCPOAuthUseCase{revokeErr: appmcp.ErrInvalidOAuthRequest}})
	req = httptest.NewRequest(http.MethodPost, "http://game.local/oauth/revoke", strings.NewReader("token="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_request") {
		t.Fatalf("expected invalid revoke request, got status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestMCPOAuthMetadataUnavailableAndMethodGuards(t *testing.T) {
	server := New(Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "http://game.local/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected metadata unavailable, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "http://game.local/.well-known/jwks.json", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected JWKS unavailable, got status=%d body=%q", rec.Code, rec.Body.String())
	}

	server = New(Dependencies{MCPOAuth: &fakeMCPOAuthUseCase{}})
	req = httptest.NewRequest(http.MethodPost, "http://game.local/.well-known/oauth-authorization-server", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("expected metadata method guard, got status=%d allow=%q", rec.Code, rec.Header().Get("Allow"))
	}
}

type fakeMCPOAuthUseCase struct {
	authorizeResult appmcp.OAuthAuthorizeResult
	exchangeResult  appmcp.OAuthTokenResult
	revokeResult    appmcp.OAuthRevokeResult
	jwksResult      domainmcp.JSONWebKeySet
	authorizeErr    error
	exchangeErr     error
	revokeErr       error

	authorizeCommand appmcp.OAuthAuthorizeCommand
	tokenCommand     appmcp.OAuthTokenCommand
	revokeCommand    appmcp.OAuthRevokeCommand
}

func (f *fakeMCPOAuthUseCase) OAuthAuthorizationServerMetadata(_ context.Context, issuer string) domainmcp.OAuthAuthorizationServerMetadata {
	return (appmcp.Service{}).OAuthAuthorizationServerMetadata(context.Background(), issuer)
}

func (f *fakeMCPOAuthUseCase) OAuthJWKS(context.Context) domainmcp.JSONWebKeySet {
	return f.jwksResult
}

func (f *fakeMCPOAuthUseCase) AuthorizeOAuth(_ context.Context, command appmcp.OAuthAuthorizeCommand) (appmcp.OAuthAuthorizeResult, error) {
	f.authorizeCommand = command
	if f.authorizeErr != nil {
		return appmcp.OAuthAuthorizeResult{}, f.authorizeErr
	}
	return f.authorizeResult, nil
}

func (f *fakeMCPOAuthUseCase) ExchangeOAuthCode(_ context.Context, command appmcp.OAuthTokenCommand) (appmcp.OAuthTokenResult, error) {
	f.tokenCommand = command
	if f.exchangeErr != nil {
		return appmcp.OAuthTokenResult{}, f.exchangeErr
	}
	return f.exchangeResult, nil
}

func (f *fakeMCPOAuthUseCase) RevokeOAuthToken(_ context.Context, command appmcp.OAuthRevokeCommand) (appmcp.OAuthRevokeResult, error) {
	f.revokeCommand = command
	if f.revokeErr != nil {
		return appmcp.OAuthRevokeResult{}, f.revokeErr
	}
	return f.revokeResult, nil
}
