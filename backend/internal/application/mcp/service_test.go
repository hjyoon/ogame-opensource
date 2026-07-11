package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
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

func TestServiceBuildsOAuthAuthorizationServerMetadata(t *testing.T) {
	service := NewService(fakeHealthProvider{})

	metadata := service.OAuthAuthorizationServerMetadata(context.Background(), "https://game.example/")
	if metadata.Issuer != "https://game.example" || metadata.AuthorizationEndpoint != "https://game.example/oauth/authorize" || metadata.TokenEndpoint != "https://game.example/oauth/token" || metadata.RevocationEndpoint != "https://game.example/oauth/revoke" || metadata.RegistrationEndpoint != "https://game.example/oauth/register" {
		t.Fatalf("unexpected metadata endpoints: %+v", metadata)
	}
	if strings.Join(metadata.ResponseTypesSupported, ",") != "code" || strings.Join(metadata.GrantTypesSupported, ",") != "authorization_code" {
		t.Fatalf("expected authorization-code metadata, got %+v", metadata)
	}
	if strings.Join(metadata.CodeChallengeMethodsSupported, ",") != "S256" || strings.Join(metadata.TokenEndpointAuthMethodsSupported, ",") != "none" {
		t.Fatalf("expected PKCE public-client metadata, got %+v", metadata)
	}
	if strings.Join(metadata.RevocationEndpointAuthMethods, ",") != "none" {
		t.Fatalf("expected public-client revocation metadata, got %+v", metadata)
	}
	scopes := strings.Join(metadata.ScopesSupported, ",")
	if !strings.Contains(scopes, "openid") || !strings.Contains(scopes, domainmcp.ScopeRead) {
		t.Fatalf("expected OIDC and MCP read scopes, got %+v", metadata.ScopesSupported)
	}
	if jwks := service.OAuthJWKS(context.Background()); len(jwks.Keys) != 0 {
		t.Fatalf("expected empty JWKS without signer, got %+v", jwks)
	}
	resourceMetadata := service.OAuthProtectedResourceMetadata(context.Background(), "https://game.example/mcp", "https://game.example/")
	if resourceMetadata.Resource != "https://game.example/mcp" || strings.Join(resourceMetadata.AuthorizationServers, ",") != "https://game.example" || !strings.Contains(strings.Join(resourceMetadata.BearerMethods, ","), "header") {
		t.Fatalf("unexpected protected resource metadata: %+v", resourceMetadata)
	}

	signer := &fakeOIDCSigner{token: "id.token.signature"}
	service = service.WithOIDCSigner(signer)
	metadata = service.OAuthAuthorizationServerMetadata(context.Background(), "https://game.example/")
	if metadata.JWKSURI != "https://game.example/.well-known/jwks.json" || strings.Join(metadata.IDTokenSigningAlgValuesSupported, ",") != "EdDSA" {
		t.Fatalf("expected OIDC metadata, got %+v", metadata)
	}
	jwks := service.OAuthJWKS(context.Background())
	if len(jwks.Keys) != 1 || jwks.Keys[0].KeyID != "kid" {
		t.Fatalf("unexpected JWKS: %+v", jwks)
	}
}

func TestServiceRegistersOAuthClient(t *testing.T) {
	service := NewServiceWithTokenManagement(
		fakeHealthProvider{},
		nil,
		&fakeTokenRepository{},
		fakeSessionLookup{},
		fakeTokenGenerator{secret: "token"},
		func() time.Time { return time.Unix(1700, 0) },
	)

	result, err := service.RegisterOAuthClient(context.Background(), OAuthClientRegistrationCommand{
		RedirectURIs:            []string{" http://127.0.0.1:9911/callback ", "http://127.0.0.1:9911/callback"},
		ClientName:              "Claude Desktop",
		GrantTypes:              []string{"authorization_code", "authorization_code"},
		ResponseTypes:           []string{"code", "code"},
		TokenEndpointAuthMethod: "none",
		Scope:                   "openid mcp:read mcp:read",
	})
	if err != nil {
		t.Fatalf("RegisterOAuthClient returned error: %v", err)
	}
	if !strings.HasPrefix(result.ClientID, "ogmcp_client_") || result.ClientIDIssuedAt != 1700 || result.ClientName != "Claude Desktop" {
		t.Fatalf("unexpected client registration result: %+v", result)
	}
	if strings.Join(result.RedirectURIs, ",") != "http://127.0.0.1:9911/callback" || strings.Join(result.GrantTypes, ",") != "authorization_code" || result.TokenEndpointAuthMethod != "none" || result.Scope != "openid mcp:read" {
		t.Fatalf("unexpected normalized client registration result: %+v", result)
	}

	external, err := service.WithOAuthRedirectURIs([]string{"https://client.example/callback"}).RegisterOAuthClient(context.Background(), OAuthClientRegistrationCommand{
		RedirectURIs: []string{"https://client.example/callback"},
		Scope:        "mcp:read",
	})
	if err != nil {
		t.Fatalf("RegisterOAuthClient allow-listed redirect returned error: %v", err)
	}
	if strings.Join(external.RedirectURIs, ",") != "https://client.example/callback" {
		t.Fatalf("unexpected external registration result: %+v", external)
	}

	result, err = service.RegisterOAuthClient(context.Background(), OAuthClientRegistrationCommand{
		RedirectURIs: []string{"http://localhost:9911/callback"},
	})
	if err != nil {
		t.Fatalf("RegisterOAuthClient defaults returned error: %v", err)
	}
	if strings.Join(result.GrantTypes, ",") != "authorization_code" || strings.Join(result.ResponseTypes, ",") != "code" || result.TokenEndpointAuthMethod != "none" || result.Scope != domainmcp.ScopeRead {
		t.Fatalf("unexpected default registration result: %+v", result)
	}

	for _, command := range []OAuthClientRegistrationCommand{
		{},
		{RedirectURIs: []string{"https://client.example/callback"}},
		{RedirectURIs: []string{":"}},
		{RedirectURIs: []string{"http://127.0.0.1:9911/callback"}, GrantTypes: []string{"client_credentials"}},
		{RedirectURIs: []string{"http://127.0.0.1:9911/callback"}, GrantTypes: []string{" "}},
		{RedirectURIs: []string{"http://127.0.0.1:9911/callback"}, ResponseTypes: []string{"token"}},
		{RedirectURIs: []string{"http://127.0.0.1:9911/callback"}, ResponseTypes: []string{" "}},
		{RedirectURIs: []string{"http://127.0.0.1:9911/callback"}, TokenEndpointAuthMethod: "client_secret_basic"},
		{RedirectURIs: []string{"http://127.0.0.1:9911/callback"}, Scope: "mcp:admin"},
	} {
		if _, err := service.RegisterOAuthClient(context.Background(), command); !errors.Is(err, ErrInvalidOAuthRequest) {
			t.Fatalf("expected invalid registration request for %+v, got %v", command, err)
		}
	}

	failing := NewServiceWithTokenManagement(fakeHealthProvider{}, nil, &fakeTokenRepository{}, fakeSessionLookup{}, fakeTokenGenerator{err: errors.New("random down")}, time.Now)
	if _, err := failing.RegisterOAuthClient(context.Background(), OAuthClientRegistrationCommand{RedirectURIs: []string{"http://127.0.0.1:9911/callback"}}); err == nil {
		t.Fatalf("expected client id generator error")
	}
}

func TestServiceOAuthRevokeToken(t *testing.T) {
	repository := &fakeTokenRepository{}
	service := NewServiceWithTokenManagement(
		fakeHealthProvider{},
		nil,
		repository,
		fakeSessionLookup{},
		fakeTokenGenerator{secret: "token"},
		func() time.Time { return time.Unix(1700, 0) },
	)

	result, err := service.RevokeOAuthToken(context.Background(), OAuthRevokeCommand{
		Token:         " ogmcp_access ",
		TokenTypeHint: "access_token",
	})
	if err != nil {
		t.Fatalf("RevokeOAuthToken returned error: %v", err)
	}
	if !result.Revoked || repository.revokedHash != HashToken("ogmcp_access") || repository.revokedAt != 1700 {
		t.Fatalf("unexpected revoke result=%+v repository=%+v", result, repository)
	}

	repository.revokedHash = ""
	repository.revokeByHashResult = false
	result, err = service.RevokeOAuthToken(context.Background(), OAuthRevokeCommand{Token: "missing"})
	if err != nil {
		t.Fatalf("RevokeOAuthToken missing token returned error: %v", err)
	}
	if result.Revoked || repository.revokedHash != HashToken("missing") {
		t.Fatalf("expected missing token to be a non-error miss, result=%+v repository=%+v", result, repository)
	}

	_, err = service.RevokeOAuthToken(context.Background(), OAuthRevokeCommand{})
	if !errors.Is(err, ErrInvalidOAuthRequest) {
		t.Fatalf("expected invalid revoke request, got %v", err)
	}

	_, err = NewService(fakeHealthProvider{}).RevokeOAuthToken(context.Background(), OAuthRevokeCommand{Token: "token"})
	if err == nil {
		t.Fatalf("expected dependency error")
	}

	repository.revokeErr = errors.New("update down")
	_, err = service.RevokeOAuthToken(context.Background(), OAuthRevokeCommand{Token: "token"})
	if !errors.Is(err, repository.revokeErr) {
		t.Fatalf("expected revoke repository error, got %v", err)
	}
}

func TestServiceOAuthAuthorizeConsentAndTokenExchange(t *testing.T) {
	now := time.Unix(1700, 0)
	repository := &fakeTokenRepository{}
	signer := &fakeOIDCSigner{token: "id.token.signature"}
	service := NewServiceWithTokenManagement(
		fakeHealthProvider{},
		nil,
		repository,
		fakeSessionLookup{auth: authenticatedSession(42)},
		fakeTokenGenerator{secret: "ogmcp_access", code: "ogmcp_code_authorized"},
		func() time.Time { return now },
	).WithOAuthCodeRepository(repository).WithOIDCSigner(signer)
	verifier := strings.Repeat("a", 43)
	challenge := testPKCEChallenge(verifier)
	command := OAuthAuthorizeCommand{
		TokenManagementCommand: TokenManagementCommand{PublicSession: "pub"},
		ResponseType:           "code",
		ClientID:               "desktop-client",
		RedirectURI:            "http://127.0.0.1:9911/callback",
		Resource:               "https://game.example/mcp",
		Scope:                  "openid mcp:read mcp:fleet",
		State:                  "state-1",
		CodeChallenge:          challenge,
		CodeChallengeMethod:    "S256",
		Issuer:                 "https://game.example",
	}

	consent, err := service.AuthorizeOAuth(context.Background(), command)
	if err != nil {
		t.Fatalf("AuthorizeOAuth consent returned error: %v", err)
	}
	if !consent.Authenticated || !consent.RequiresConsent || strings.Join(consent.Scopes, " ") != "openid mcp:read mcp:fleet" {
		t.Fatalf("unexpected consent result: %+v", consent)
	}

	command.ConsentApproved = true
	authorized, err := service.AuthorizeOAuth(context.Background(), command)
	if err != nil {
		t.Fatalf("AuthorizeOAuth approve returned error: %v", err)
	}
	if !strings.Contains(authorized.RedirectTo, "code=ogmcp_code_authorized") || !strings.Contains(authorized.RedirectTo, "state=state-1") {
		t.Fatalf("expected redirect with code and state, got %+v", authorized)
	}
	if repository.oauthCode.PlayerID != 42 || repository.oauthCode.CodeHash != HashToken("ogmcp_code_authorized") || repository.oauthCode.Resource != "https://game.example/mcp" || repository.oauthCode.ExpiresAt != now.Add(mcpOAuthCodeTTL).Unix() {
		t.Fatalf("unexpected stored OAuth code: %+v", repository.oauthCode)
	}

	token, err := service.ExchangeOAuthCode(context.Background(), OAuthTokenCommand{
		GrantType:    "authorization_code",
		Code:         "ogmcp_code_authorized",
		RedirectURI:  "http://127.0.0.1:9911/callback",
		Resource:     "https://game.example/mcp",
		ClientID:     "desktop-client",
		CodeVerifier: verifier,
		Issuer:       "https://game.example",
	})
	if err != nil {
		t.Fatalf("ExchangeOAuthCode returned error: %v", err)
	}
	if token.AccessToken != "ogmcp_access" || token.TokenType != "Bearer" || token.Scope != "openid mcp:read mcp:fleet" || token.IDToken != "id.token.signature" || token.ExpiresIn != int64(defaultMCPTokenTTL.Seconds()) {
		t.Fatalf("unexpected token result: %+v", token)
	}
	if signer.command.Issuer != "https://game.example" || signer.command.Audience != "desktop-client" || signer.command.PlayerID != 42 {
		t.Fatalf("unexpected id token command: %+v", signer.command)
	}
	if repository.created.Name != "OAuth desktop-client" || repository.created.Scopes[2] != domainmcp.ScopeFleet || repository.created.ExpiresAt != now.Add(defaultMCPTokenTTL).Unix() || repository.hash != HashToken("ogmcp_access") {
		t.Fatalf("unexpected persisted OAuth token: token=%+v hash=%q", repository.created, repository.hash)
	}
}

func TestServiceOAuthRejectsInvalidRequestsAndGrants(t *testing.T) {
	repository := &fakeTokenRepository{oauthCode: domainmcp.OAuthAuthorizationCode{
		PlayerID:            42,
		ClientID:            "desktop-client",
		RedirectURI:         "http://127.0.0.1:9911/callback",
		Resource:            "https://game.example/mcp",
		Scopes:              []string{domainmcp.ScopeRead},
		CodeHash:            HashToken("code"),
		CodeChallenge:       testPKCEChallenge(strings.Repeat("a", 43)),
		CodeChallengeMethod: "S256",
		ExpiresAt:           2000,
	}}
	service := NewServiceWithTokenManagement(
		fakeHealthProvider{},
		nil,
		repository,
		fakeSessionLookup{auth: authenticatedSession(42)},
		fakeTokenGenerator{secret: "token", code: "code"},
		func() time.Time { return time.Unix(1700, 0) },
	).WithOAuthCodeRepository(repository)

	_, err := service.AuthorizeOAuth(context.Background(), OAuthAuthorizeCommand{
		TokenManagementCommand: TokenManagementCommand{PublicSession: "pub"},
		ResponseType:           "code",
		ClientID:               "desktop-client",
		RedirectURI:            "http://evil.example/callback",
		CodeChallenge:          testPKCEChallenge(strings.Repeat("a", 43)),
		CodeChallengeMethod:    "S256",
		ConsentApproved:        true,
	})
	if !errors.Is(err, ErrInvalidOAuthRequest) {
		t.Fatalf("expected invalid insecure redirect request, got %v", err)
	}

	_, err = service.AuthorizeOAuth(context.Background(), OAuthAuthorizeCommand{
		TokenManagementCommand: TokenManagementCommand{PublicSession: "pub"},
		ResponseType:           "code",
		ClientID:               "desktop-client",
		RedirectURI:            "http://127.0.0.1:9911/callback",
		Resource:               "https://other.example/mcp",
		CodeChallenge:          testPKCEChallenge(strings.Repeat("a", 43)),
		CodeChallengeMethod:    "S256",
		ConsentApproved:        true,
		Issuer:                 "https://game.example",
	})
	if !errors.Is(err, ErrInvalidOAuthRequest) {
		t.Fatalf("expected invalid OAuth resource request, got %v", err)
	}

	_, err = service.ExchangeOAuthCode(context.Background(), OAuthTokenCommand{
		GrantType:    "authorization_code",
		Code:         "code",
		RedirectURI:  "http://127.0.0.1:9911/callback",
		Resource:     "https://game.example/mcp",
		ClientID:     "desktop-client",
		CodeVerifier: strings.Repeat("b", 43),
		Issuer:       "https://game.example",
	})
	if !errors.Is(err, ErrInvalidOAuthGrant) {
		t.Fatalf("expected PKCE invalid grant, got %v", err)
	}

	resourceMismatch := validStoredOAuthCode(strings.Repeat("a", 43), "desktop-client")
	resourceMismatch.Resource = "https://game.example/other"
	service = oauthExchangeService(&fakeTokenRepository{oauthCode: resourceMismatch}, fakeTokenGenerator{secret: "token"})
	_, err = service.ExchangeOAuthCode(context.Background(), OAuthTokenCommand{
		GrantType:    "authorization_code",
		Code:         "code",
		RedirectURI:  "http://127.0.0.1:9911/callback",
		Resource:     "https://game.example/mcp",
		ClientID:     "desktop-client",
		CodeVerifier: strings.Repeat("a", 43),
		Issuer:       "https://game.example",
	})
	if !errors.Is(err, ErrInvalidOAuthGrant) {
		t.Fatalf("expected resource mismatch invalid grant, got %v", err)
	}
}

func TestServiceOAuthCoversDependencyAndFailureBranches(t *testing.T) {
	verifier := strings.Repeat("a", 43)
	validCommand := OAuthAuthorizeCommand{
		TokenManagementCommand: TokenManagementCommand{PublicSession: "pub"},
		ResponseType:           "code",
		ClientID:               "desktop-client",
		RedirectURI:            "http://localhost:9911/callback",
		Resource:               "http://localhost:8080/mcp",
		Scope:                  "mcp:read",
		State:                  "state-1",
		CodeChallenge:          testPKCEChallenge(verifier),
		CodeChallengeMethod:    "S256",
		ConsentApproved:        true,
		Issuer:                 "http://localhost:8080",
	}
	if _, err := (Service{}).AuthorizeOAuth(context.Background(), validCommand); err == nil {
		t.Fatalf("expected authorize dependency error")
	}

	sessionErr := errors.New("session down")
	service := NewServiceWithTokenManagement(fakeHealthProvider{}, nil, &fakeTokenRepository{}, fakeSessionLookup{err: sessionErr}, fakeTokenGenerator{secret: "token", code: "code"}, time.Now).WithOAuthCodeRepository(&fakeTokenRepository{})
	if _, err := service.AuthorizeOAuth(context.Background(), validCommand); !errors.Is(err, sessionErr) {
		t.Fatalf("expected authorize session error, got %v", err)
	}

	service = NewServiceWithTokenManagement(fakeHealthProvider{}, nil, &fakeTokenRepository{}, fakeSessionLookup{auth: domainpublicsite.SessionAuthentication{Authenticated: false, Issues: []domainpublicsite.SessionIssue{{Code: domainpublicsite.SessionIssueInvalid}}}}, fakeTokenGenerator{secret: "token", code: "code"}, time.Now).WithOAuthCodeRepository(&fakeTokenRepository{})
	result, err := service.AuthorizeOAuth(context.Background(), validCommand)
	if err != nil || result.Authenticated || len(result.Issues) != 1 {
		t.Fatalf("expected unauthenticated authorize result, got result=%+v err=%v", result, err)
	}

	service = NewServiceWithTokenManagement(fakeHealthProvider{}, nil, &fakeTokenRepository{}, fakeSessionLookup{auth: authenticatedSession(42)}, fakeTokenGenerator{err: errors.New("random down")}, time.Now).WithOAuthCodeRepository(&fakeTokenRepository{})
	if _, err := service.AuthorizeOAuth(context.Background(), validCommand); err == nil {
		t.Fatalf("expected authorize code generator error")
	}

	service = NewServiceWithTokenManagement(fakeHealthProvider{}, nil, &fakeTokenRepository{}, fakeSessionLookup{auth: authenticatedSession(42)}, fakeTokenGenerator{secret: "token", code: "code"}, time.Now).WithOAuthCodeRepository(&fakeTokenRepository{oauthCreateErr: errors.New("insert down")})
	if _, err := service.AuthorizeOAuth(context.Background(), validCommand); err == nil {
		t.Fatalf("expected authorize repository error")
	}

	repository := &fakeTokenRepository{}
	service = NewServiceWithTokenManagement(fakeHealthProvider{}, nil, repository, fakeSessionLookup{auth: authenticatedSession(42)}, fakeTokenGenerator{secret: "token", code: "code"}, time.Now).
		WithOAuthCodeRepository(repository).
		WithOAuthRedirectURIs([]string{"https://client.example/callback", "bad"})
	externalCommand := validCommand
	externalCommand.RedirectURI = "https://client.example/callback"
	external, err := service.AuthorizeOAuth(context.Background(), externalCommand)
	if err != nil || external.RedirectTo == "" {
		t.Fatalf("expected allow-listed external redirect to authorize, result=%+v err=%v", external, err)
	}
}

func TestServiceOAuthTokenExchangeCoversValidationAndFailureBranches(t *testing.T) {
	verifier := strings.Repeat("a", 43)
	validTokenCommand := OAuthTokenCommand{
		GrantType:    "authorization_code",
		Code:         "code",
		RedirectURI:  "http://127.0.0.1:9911/callback",
		Resource:     "https://game.example/mcp",
		ClientID:     "desktop-client",
		CodeVerifier: verifier,
		Issuer:       "https://game.example",
	}
	if _, err := (Service{}).ExchangeOAuthCode(context.Background(), validTokenCommand); err == nil {
		t.Fatalf("expected exchange dependency error")
	}

	for _, tt := range []struct {
		name    string
		command OAuthTokenCommand
	}{
		{name: "grant", command: OAuthTokenCommand{GrantType: "client_credentials"}},
		{name: "code", command: OAuthTokenCommand{GrantType: "authorization_code"}},
		{name: "redirect", command: OAuthTokenCommand{GrantType: "authorization_code", Code: "code", RedirectURI: "ftp://127.0.0.1/callback"}},
		{name: "resource", command: OAuthTokenCommand{GrantType: "authorization_code", Code: "code", RedirectURI: "http://127.0.0.1/callback", Resource: "https://other.example/mcp", Issuer: "https://game.example"}},
		{name: "client", command: OAuthTokenCommand{GrantType: "authorization_code", Code: "code", RedirectURI: "http://127.0.0.1/callback", Resource: "https://game.example/mcp", ClientID: "bad client", Issuer: "https://game.example"}},
		{name: "verifier", command: OAuthTokenCommand{GrantType: "authorization_code", Code: "code", RedirectURI: "http://127.0.0.1/callback", Resource: "https://game.example/mcp", ClientID: "desktop", Issuer: "https://game.example"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			service := oauthExchangeService(&fakeTokenRepository{oauthCode: validStoredOAuthCode(verifier, "desktop-client")}, fakeTokenGenerator{secret: "token"})
			if _, err := service.ExchangeOAuthCode(context.Background(), tt.command); !errors.Is(err, ErrInvalidOAuthRequest) {
				t.Fatalf("expected invalid request, got %v", err)
			}
		})
	}

	service := oauthExchangeService(&fakeTokenRepository{oauthConsumeErr: errors.New("db down")}, fakeTokenGenerator{secret: "token"})
	if _, err := service.ExchangeOAuthCode(context.Background(), validTokenCommand); err == nil {
		t.Fatalf("expected exchange repository consume error")
	}

	service = oauthExchangeService(&fakeTokenRepository{oauthCode: validStoredOAuthCode(verifier, "other-client")}, fakeTokenGenerator{secret: "token"})
	if _, err := service.ExchangeOAuthCode(context.Background(), validTokenCommand); !errors.Is(err, ErrInvalidOAuthGrant) {
		t.Fatalf("expected client mismatch invalid grant, got %v", err)
	}

	badMethod := validStoredOAuthCode(verifier, "desktop-client")
	badMethod.CodeChallengeMethod = "plain"
	service = oauthExchangeService(&fakeTokenRepository{oauthCode: badMethod}, fakeTokenGenerator{secret: "token"})
	if _, err := service.ExchangeOAuthCode(context.Background(), validTokenCommand); !errors.Is(err, ErrInvalidOAuthGrant) {
		t.Fatalf("expected method mismatch invalid grant, got %v", err)
	}

	badScopes := validStoredOAuthCode(verifier, "desktop-client")
	badScopes.Scopes = []string{"mcp:admin"}
	service = oauthExchangeService(&fakeTokenRepository{oauthCode: badScopes}, fakeTokenGenerator{secret: "token"})
	if _, err := service.ExchangeOAuthCode(context.Background(), validTokenCommand); !errors.Is(err, ErrInvalidOAuthGrant) {
		t.Fatalf("expected invalid stored scope grant, got %v", err)
	}

	service = oauthExchangeService(&fakeTokenRepository{oauthCode: validStoredOAuthCode(verifier, "desktop-client")}, fakeTokenGenerator{err: errors.New("random down")})
	if _, err := service.ExchangeOAuthCode(context.Background(), validTokenCommand); err == nil {
		t.Fatalf("expected exchange token generator error")
	}

	service = oauthExchangeService(&fakeTokenRepository{oauthCode: validStoredOAuthCode(verifier, "desktop-client"), createErr: errors.New("insert down")}, fakeTokenGenerator{secret: "token"})
	if _, err := service.ExchangeOAuthCode(context.Background(), validTokenCommand); err == nil {
		t.Fatalf("expected exchange token repository error")
	}

	openIDCode := validStoredOAuthCode(verifier, "desktop-client")
	openIDCode.Scopes = []string{"openid", domainmcp.ScopeRead}
	service = oauthExchangeService(&fakeTokenRepository{oauthCode: openIDCode}, fakeTokenGenerator{secret: "token"}).WithOIDCSigner(&fakeOIDCSigner{err: errors.New("sign down")})
	if _, err := service.ExchangeOAuthCode(context.Background(), validTokenCommand); err == nil {
		t.Fatalf("expected id token signing error")
	}

	longClient := strings.Repeat("c", 70)
	longCommand := validTokenCommand
	longCommand.ClientID = longClient
	longCode := validStoredOAuthCode(verifier, longClient)
	repository := &fakeTokenRepository{oauthCode: longCode}
	service = oauthExchangeService(repository, fakeTokenGenerator{secret: "token"})
	if _, err := service.ExchangeOAuthCode(context.Background(), longCommand); err != nil {
		t.Fatalf("expected long client exchange success, got %v", err)
	}
	if len([]rune(repository.created.Name)) != 64 {
		t.Fatalf("expected oauth token name truncation, got %q", repository.created.Name)
	}
}

func TestOAuthValidationHelpers(t *testing.T) {
	if code, err := (SecureTokenGenerator{}).NewMCPOAuthCode(); err != nil || !strings.HasPrefix(code, "ogmcp_code_") {
		t.Fatalf("unexpected secure oauth code: code=%q err=%v", code, err)
	}
	service := NewServiceWithTokenManagement(fakeHealthProvider{}, nil, &fakeTokenRepository{}, fakeSessionLookup{}, nil, nil)
	if service.tokenGenerator == nil || service.codeGenerator == nil || service.now == nil {
		t.Fatalf("expected default token management dependencies: %+v", service)
	}
	if !validOAuthRedirectURI("https://client.example/callback", []string{"https://client.example/callback"}) || !validOAuthRedirectURI("http://[::1]:9000/callback", nil) {
		t.Fatalf("expected allow-listed https and loopback redirect URIs to be valid")
	}
	if validOAuthRedirectURI("https://client.example/callback", nil) {
		t.Fatalf("expected external https redirect URI to require allow-list")
	}
	for _, raw := range []string{"http://example.com/callback", "http://127.0.0.1/callback#fragment", ":", "custom://callback"} {
		if validOAuthRedirectURI(raw, nil) {
			t.Fatalf("expected redirect URI %q to be invalid", raw)
		}
	}
	allowed := ParseOAuthRedirectURIs("https://client.example/callback, invalid ; https://client.example/callback")
	if strings.Join(allowed, ",") != "https://client.example/callback" {
		t.Fatalf("unexpected parsed allow-list: %v", allowed)
	}
	for _, value := range []string{strings.Repeat("a", 42), strings.Repeat("a", 129), strings.Repeat("!", 43)} {
		if validPKCEValue(value) {
			t.Fatalf("expected PKCE value %q to be invalid", value)
		}
	}
	scopes, err := normalizeOAuthScopes("profile mcp:read profile")
	if err != nil || strings.Join(scopes, " ") != "profile mcp:read" {
		t.Fatalf("unexpected normalized scopes=%v err=%v", scopes, err)
	}
	scopes, err = normalizeOAuthScopes(domainmcp.ScopeFleetWrite + " " + domainmcp.ScopeQueueWrite + " " + domainmcp.ScopeResourcesWrite + " " + domainmcp.ScopePremiumWrite)
	if err != nil || strings.Join(scopes, " ") != domainmcp.ScopeFleetWrite+" "+domainmcp.ScopeQueueWrite+" "+domainmcp.ScopeResourcesWrite+" "+domainmcp.ScopePremiumWrite {
		t.Fatalf("expected fleet, queue, resources, and premium write scopes to be allowed, got scopes=%v err=%v", scopes, err)
	}
	scopes, err = normalizeOAuthScopes("")
	if err != nil || strings.Join(scopes, " ") != domainmcp.ScopeRead {
		t.Fatalf("expected default read scope, got scopes=%v err=%v", scopes, err)
	}
	if _, err := normalizeOAuthScopes("mcp:admin"); !errors.Is(err, ErrInvalidOAuthRequest) {
		t.Fatalf("expected invalid oauth scope, got %v", err)
	}
	if _, err := oauthRedirectURI("%", "code", ""); err == nil {
		t.Fatalf("expected redirect URI parse error")
	}

	validChallenge := testPKCEChallenge(strings.Repeat("a", 43))
	validCommand := OAuthAuthorizeCommand{
		ResponseType:        "code",
		ClientID:            "desktop",
		RedirectURI:         "http://127.0.0.1:9000/callback",
		Resource:            "https://game.example/mcp",
		Scope:               domainmcp.ScopeRead,
		CodeChallenge:       validChallenge,
		CodeChallengeMethod: "S256",
		Issuer:              "https://game.example",
	}
	for _, tt := range []struct {
		name   string
		mutate func(*OAuthAuthorizeCommand)
	}{
		{name: "response type", mutate: func(command *OAuthAuthorizeCommand) { command.ResponseType = "token" }},
		{name: "client id", mutate: func(command *OAuthAuthorizeCommand) { command.ClientID = "" }},
		{name: "redirect", mutate: func(command *OAuthAuthorizeCommand) { command.RedirectURI = "http://example.com/callback" }},
		{name: "resource", mutate: func(command *OAuthAuthorizeCommand) { command.Resource = "" }},
		{name: "challenge method", mutate: func(command *OAuthAuthorizeCommand) { command.CodeChallengeMethod = "plain" }},
		{name: "challenge", mutate: func(command *OAuthAuthorizeCommand) { command.CodeChallenge = "short" }},
		{name: "scope", mutate: func(command *OAuthAuthorizeCommand) { command.Scope = domainmcp.ScopeAdmin }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			command := validCommand
			tt.mutate(&command)
			if _, err := normalizeOAuthAuthorizeRequest(command, nil); !errors.Is(err, ErrInvalidOAuthRequest) {
				t.Fatalf("expected invalid authorize request, got %v", err)
			}
		})
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
	if strings.Join(names, ",") != "get_server_health,get_mcp_access,list_planets,get_account_overview,get_planet_resources,get_building_queue,get_fleet_movements" {
		t.Fatalf("unexpected tools: %v", names)
	}
}

func TestServiceListsOfficerStatusToolForReadScope(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
		},
	}).WithPremiumReadRepository(&fakePremiumWriteRepository{})

	tools, err := service.ListTools(context.Background(), domainmcp.ListToolsCommand{AccessToken: "read"})
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "get_server_health,get_mcp_access,get_officer_status" {
		t.Fatalf("unexpected read officer tools: %v", names)
	}
}

func TestServiceListsSearchGameToolForReadScope(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
		},
	}).WithSearchReadRepository(&fakeSearchReadRepository{})

	tools, err := service.ListTools(context.Background(), domainmcp.ListToolsCommand{AccessToken: "read"})
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "get_server_health,get_mcp_access,search_game" {
		t.Fatalf("unexpected read search tools: %v", names)
	}
}

func TestServiceListsGalaxySystemToolForReadScope(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
		},
	}).WithGalaxyReadRepository(&fakeGalaxyReadRepository{})

	tools, err := service.ListTools(context.Background(), domainmcp.ListToolsCommand{AccessToken: "read"})
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "get_server_health,get_mcp_access,get_galaxy_system" {
		t.Fatalf("unexpected read galaxy tools: %v", names)
	}
}

func TestServiceListsMessageToolsForMessageScopedTokens(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"messages": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeMessages}},
			"both":     {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead, domainmcp.ScopeMessages}},
		},
	}).WithReadRepository(fakeReadRepository{})

	tools, err := service.ListTools(context.Background(), domainmcp.ListToolsCommand{AccessToken: "messages"})
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "get_server_health,list_messages,get_message" {
		t.Fatalf("unexpected message-only tools: %v", names)
	}

	tools, err = service.ListTools(context.Background(), domainmcp.ListToolsCommand{AccessToken: "both"})
	if err != nil {
		t.Fatalf("ListTools both returned error: %v", err)
	}
	names = names[:0]
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	if !strings.Contains(strings.Join(names, ","), "get_fleet_movements,list_messages,get_message") {
		t.Fatalf("expected read and message tools, got %v", names)
	}
}

func TestServiceListsSendMessageToolForMessageWriteScope(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeMessageWrite}},
		},
	}).WithWriteRepository(&fakeWriteRepository{})

	tools, err := service.ListTools(context.Background(), domainmcp.ListToolsCommand{AccessToken: "write"})
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "get_server_health,send_message,delete_messages,report_message" {
		t.Fatalf("unexpected message-write tools: %v", names)
	}
}

func TestServiceListsRecallFleetToolForFleetWriteScope(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"fleet-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeFleetWrite}},
		},
	}).WithFleetWriteRepository(&fakeFleetWriteRepository{})

	tools, err := service.ListTools(context.Background(), domainmcp.ListToolsCommand{AccessToken: "fleet-write"})
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "get_server_health,validate_fleet_dispatch,dispatch_fleet,recall_fleet" {
		t.Fatalf("unexpected fleet-write tools: %v", names)
	}
}

func TestServiceListsCancelBuildingQueueToolForQueueWriteScope(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"queue-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeQueueWrite}},
		},
	}).WithQueueWriteRepository(&fakeQueueWriteRepository{})

	tools, err := service.ListTools(context.Background(), domainmcp.ListToolsCommand{AccessToken: "queue-write"})
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "get_server_health,cancel_building_queue,cancel_research_queue,enqueue_shipyard_order" {
		t.Fatalf("unexpected queue-write tools: %v", names)
	}
}

func TestServiceListsUpdateResourceProductionToolForResourcesWriteScope(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"resources-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeResourcesWrite}},
		},
	}).WithResourceWriteRepository(&fakeResourceWriteRepository{})

	tools, err := service.ListTools(context.Background(), domainmcp.ListToolsCommand{AccessToken: "resources-write"})
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "get_server_health,update_resource_production" {
		t.Fatalf("unexpected resources-write tools: %v", names)
	}
}

func TestServiceListsRecruitOfficerToolForPremiumWriteScope(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"premium-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopePremiumWrite}},
		},
	}).WithPremiumWriteRepository(&fakePremiumWriteRepository{})

	tools, err := service.ListTools(context.Background(), domainmcp.ListToolsCommand{AccessToken: "premium-write"})
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "get_server_health,recruit_officer" {
		t.Fatalf("unexpected premium-write tools: %v", names)
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

func TestServiceCallsFleetMovementsTool(t *testing.T) {
	movements := domainmcp.FleetMovements{
		PlayerID: 42,
		Now:      100,
		Count:    1,
		Events: []domainmcp.FleetMovement{{
			ID:               7,
			Mission:          3,
			MissionName:      "Transport",
			StateShort:       "(G)",
			RemainingSeconds: 50,
		}},
	}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
		},
	}).WithReadRepository(fakeReadRepository{fleetMovements: movements})

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "get_fleet_movements",
		AccessToken: "read",
	})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	structured := result.StructuredContent.(map[string]any)
	got := structured["fleetMovements"].(domainmcp.FleetMovements)
	if result.IsError || got.Count != 1 || got.Events[0].MissionName != "Transport" || got.Events[0].RemainingSeconds != 50 {
		t.Fatalf("unexpected fleet movements result: %+v", result)
	}
}

func TestServiceFleetMovementsToolRequiresRepositoryAndReadScope(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read":  {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
			"write": {Authenticated: true, PlayerID: 43, Scopes: []string{domainmcp.ScopeWrite}},
		},
	})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_fleet_movements", AccessToken: "read"}); err == nil {
		t.Fatalf("expected missing read repository error")
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_fleet_movements", AccessToken: "write"}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without read scope, got %v", err)
	}

	service = service.WithReadRepository(fakeReadRepository{err: errors.New("fleet down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_fleet_movements", AccessToken: "read"}); err == nil {
		t.Fatalf("expected read repository error")
	}
}

func TestServiceCallsOfficerStatusTool(t *testing.T) {
	status := domainmcp.OfficerStatus{
		PlayerID:       42,
		PlanetID:       99,
		PaidDarkMatter: 1000,
		FreeDarkMatter: 2000,
		Officers: []domainmcp.OfficerStatusRow{{
			ID:       1,
			Key:      "commander",
			Name:     "Commander",
			Active:   true,
			DaysLeft: 3,
			WeekCost: 10000,
		}},
	}
	repository := &fakePremiumWriteRepository{status: status}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
		},
	}).WithPremiumReadRepository(repository)

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "get_officer_status",
		AccessToken: "read",
		Arguments:   map[string]any{"planetId": "99"},
	})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	got := result.StructuredContent.(map[string]any)["officerStatus"].(domainmcp.OfficerStatus)
	if result.IsError || got.PlayerID != 42 || got.PaidDarkMatter != 1000 || len(got.Officers) != 1 || got.Officers[0].Name != "Commander" {
		t.Fatalf("unexpected officer status result: %+v", result)
	}
	if repository.statusPlayerID != 42 || repository.statusPlanetID != 99 {
		t.Fatalf("unexpected officer status command: player=%d planet=%d", repository.statusPlayerID, repository.statusPlanetID)
	}
}

func TestServiceOfficerStatusToolRequiresRepositoryReadScopeAndValidArguments(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read":  {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
			"write": {Authenticated: true, PlayerID: 43, Scopes: []string{domainmcp.ScopeWrite}},
		},
	})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_officer_status", AccessToken: "read"}); err == nil {
		t.Fatalf("expected missing premium read repository error")
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_officer_status", AccessToken: "write"}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without read scope, got %v", err)
	}

	service = service.WithPremiumReadRepository(&fakePremiumWriteRepository{})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "get_officer_status",
		AccessToken: "read",
		Arguments:   map[string]any{"planetId": true},
	}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid planet id error, got %v", err)
	}

	service = service.WithPremiumReadRepository(&fakePremiumWriteRepository{err: errors.New("premium read down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_officer_status", AccessToken: "read"}); err == nil || !strings.Contains(err.Error(), "premium read down") {
		t.Fatalf("expected premium read repository error, got %v", err)
	}
}

func TestServiceCallsSearchGameTool(t *testing.T) {
	search := domainmcp.SearchResult{
		PlayerID: 42,
		PlanetID: 99,
		Type:     "playername",
		Text:     "leg",
		Players: []domainmcp.SearchPlayerRow{{
			PlayerID:    42,
			PlayerName:  "legor",
			PlanetName:  "Homeworld",
			Coordinates: domainmcp.Coordinates{Galaxy: 1, System: 2, Position: 3},
			Own:         true,
		}},
		Alliances: []domainmcp.SearchAllianceRow{},
	}
	repository := &fakeSearchReadRepository{result: search}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
		},
	}).WithSearchReadRepository(repository)

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "search_game",
		AccessToken: "read",
		Arguments:   map[string]any{"planetId": "99", "type": "playername", "text": " leg "},
	})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	got := result.StructuredContent.(map[string]any)["search"].(domainmcp.SearchResult)
	if result.IsError || got.PlayerID != 42 || len(got.Players) != 1 || got.Players[0].PlayerName != "legor" {
		t.Fatalf("unexpected search result: %+v", result)
	}
	if repository.playerID != 42 || repository.command.PlanetID != 99 || repository.command.Type != "playername" || repository.command.Text != "leg" {
		t.Fatalf("unexpected search command: player=%d command=%+v", repository.playerID, repository.command)
	}
}

func TestServiceSearchGameToolRequiresRepositoryReadScopeAndValidArguments(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read":  {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
			"write": {Authenticated: true, PlayerID: 43, Scopes: []string{domainmcp.ScopeWrite}},
		},
	})
	valid := map[string]any{"text": "leg"}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "search_game", AccessToken: "read", Arguments: valid}); err == nil {
		t.Fatalf("expected missing search read repository error")
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "search_game", AccessToken: "write", Arguments: valid}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without read scope, got %v", err)
	}

	service = service.WithSearchReadRepository(&fakeSearchReadRepository{})
	for _, arguments := range []map[string]any{
		{"planetId": true, "text": "leg"},
		{"type": true, "text": "leg"},
		{"type": "bad", "text": "leg"},
		{"type": "playername"},
		{"type": "playername", "text": " "},
	} {
		if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "search_game", AccessToken: "read", Arguments: arguments}); !errors.Is(err, domainmcp.ErrInvalidParams) {
			t.Fatalf("expected invalid params for %+v, got %v", arguments, err)
		}
	}

	service = service.WithSearchReadRepository(&fakeSearchReadRepository{err: errors.New("search down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "search_game", AccessToken: "read", Arguments: valid}); err == nil || !strings.Contains(err.Error(), "search down") {
		t.Fatalf("expected search repository error, got %v", err)
	}

	command, err := mcpSearchCommand(map[string]any{"text": "leg"})
	if err != nil || command.Type != "playername" || command.Text != "leg" {
		t.Fatalf("unexpected default search command=%+v err=%v", command, err)
	}
}

func TestServiceCallsGalaxySystemTool(t *testing.T) {
	galaxy := domainmcp.GalaxySystem{
		PlayerID:    42,
		PlanetID:    99,
		Coordinates: domainmcp.Coordinates{Galaxy: 1, System: 2},
		Bounds:      domainmcp.GalaxyBounds{Galaxies: 9, Systems: 499},
		Rows: []domainmcp.GalaxySystemRow{{
			Position: 4,
			Planet: &domainmcp.GalaxySystemObject{
				ID:   200,
				Name: "Target",
				Player: &domainmcp.GalaxySystemPlayer{
					ID:   43,
					Name: "enemy",
				},
			},
		}},
	}
	repository := &fakeGalaxyReadRepository{result: galaxy}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
		},
	}).WithGalaxyReadRepository(repository)

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "get_galaxy_system",
		AccessToken: "read",
		Arguments:   map[string]any{"planetId": "99", "galaxy": 1, "system": 2},
	})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	got := result.StructuredContent.(map[string]any)["galaxySystem"].(domainmcp.GalaxySystem)
	if result.IsError || got.PlayerID != 42 || len(got.Rows) != 1 || got.Rows[0].Planet.Player.Name != "enemy" {
		t.Fatalf("unexpected galaxy system result: %+v", result)
	}
	if repository.playerID != 42 || repository.command.PlanetID != 99 || repository.command.Galaxy != 1 || repository.command.System != 2 {
		t.Fatalf("unexpected galaxy command: player=%d command=%+v", repository.playerID, repository.command)
	}
}

func TestServiceGalaxySystemToolRequiresRepositoryReadScopeAndValidArguments(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read":  {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
			"write": {Authenticated: true, PlayerID: 43, Scopes: []string{domainmcp.ScopeWrite}},
		},
	})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_galaxy_system", AccessToken: "read"}); err == nil {
		t.Fatalf("expected missing galaxy read repository error")
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_galaxy_system", AccessToken: "write"}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without read scope, got %v", err)
	}

	service = service.WithGalaxyReadRepository(&fakeGalaxyReadRepository{})
	for _, arguments := range []map[string]any{
		{"planetId": true},
		{"galaxy": -1},
		{"system": "bad"},
	} {
		if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_galaxy_system", AccessToken: "read", Arguments: arguments}); !errors.Is(err, domainmcp.ErrInvalidParams) {
			t.Fatalf("expected invalid params for %+v, got %v", arguments, err)
		}
	}

	service = service.WithGalaxyReadRepository(&fakeGalaxyReadRepository{err: errors.New("galaxy down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_galaxy_system", AccessToken: "read"}); err == nil || !strings.Contains(err.Error(), "galaxy down") {
		t.Fatalf("expected galaxy repository error, got %v", err)
	}
}

func TestServiceCallsMessageTools(t *testing.T) {
	messageList := domainmcp.MessageList{
		PlayerID: 42,
		Count:    1,
		Limit:    3,
		Messages: []domainmcp.PlayerMessage{{
			ID:       11,
			Type:     0,
			TypeName: "personal",
			From:     "Admin",
			Subject:  "Hello",
			Text:     "Body",
			Date:     1000,
			Unread:   true,
		}},
	}
	detail := domainmcp.MessageDetail{PlayerID: 42, Message: messageList.Messages[0]}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"messages": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeMessages}},
		},
	}).WithReadRepository(fakeReadRepository{
		messageList:   messageList,
		messageQuery:  domainmcp.MessageQuery{Limit: 3, MessageType: 0, HasMessageType: true, IncludeText: true},
		messageDetail: detail,
		messageID:     11,
	})

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "list_messages",
		AccessToken: "messages",
		Arguments:   map[string]any{"limit": float64(3), "messageType": float64(0), "includeText": true},
	})
	if err != nil {
		t.Fatalf("list_messages returned error: %v", err)
	}
	listed := result.StructuredContent.(map[string]any)["messages"].(domainmcp.MessageList)
	if result.IsError || listed.Count != 1 || listed.Messages[0].Text != "Body" {
		t.Fatalf("unexpected message list result: %+v", result)
	}

	result, err = service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "get_message",
		AccessToken: "messages",
		Arguments:   map[string]any{"messageId": "11"},
	})
	if err != nil {
		t.Fatalf("get_message returned error: %v", err)
	}
	got := result.StructuredContent.(map[string]any)["message"].(domainmcp.MessageDetail)
	if got.Message.Subject != "Hello" || !got.Message.Unread {
		t.Fatalf("unexpected message detail: %+v", got)
	}
}

func TestServiceMessageToolsRequireMessageScopeRepositoryAndValidArguments(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read":     {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
			"messages": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeMessages}},
		},
	})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "list_messages", AccessToken: "messages"}); err == nil {
		t.Fatalf("expected missing repository error")
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_message", AccessToken: "messages", Arguments: map[string]any{"messageId": 7}}); err == nil {
		t.Fatalf("expected missing repository error")
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "list_messages", AccessToken: "read"}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without messages scope, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_message", AccessToken: "read", Arguments: map[string]any{"messageId": 7}}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden detail without messages scope, got %v", err)
	}

	service = service.WithReadRepository(fakeReadRepository{})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "list_messages", AccessToken: "messages", Arguments: map[string]any{"limit": true}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid limit error, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "list_messages", AccessToken: "messages", Arguments: map[string]any{"includeText": "yes"}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid includeText error, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_message", AccessToken: "messages"}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected missing message id error, got %v", err)
	}

	service = service.WithReadRepository(fakeReadRepository{err: errors.New("messages down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "get_message", AccessToken: "messages", Arguments: map[string]any{"messageId": 7}}); err == nil {
		t.Fatalf("expected repository error")
	}
}

func TestServiceCallsSendMessageWithDryRunAndConfirmation(t *testing.T) {
	repository := &fakeWriteRepository{
		preview: domainmcp.SendMessageResult{PlayerID: 42, TargetPlayerID: 77, Subject: "Hello", TextChars: 4},
		sent:    domainmcp.SendMessageResult{PlayerID: 42, TargetPlayerID: 77, Subject: "Hello", TextChars: 4, Executed: true},
	}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeMessageWrite}},
		},
	}).WithWriteRepository(repository)
	arguments := map[string]any{"targetPlayerId": float64(77), "subject": "Hello", "text": "Body"}

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "send_message",
		AccessToken: "write",
		Arguments:   arguments,
	})
	if err != nil {
		t.Fatalf("send_message dry-run returned error: %v", err)
	}
	preview := result.StructuredContent.(map[string]any)["sendMessage"].(domainmcp.SendMessageResult)
	if !preview.DryRun || preview.Executed || !preview.RequiresConfirmation || !strings.HasPrefix(preview.Confirmation, "send_message:77:") {
		t.Fatalf("unexpected dry-run result: %+v", preview)
	}
	if repository.previewPlayerID != 42 || repository.previewCommand.TargetPlayerID != 77 || !repository.previewCommand.DryRun {
		t.Fatalf("unexpected preview command: player=%d command=%+v", repository.previewPlayerID, repository.previewCommand)
	}

	executeArgs := map[string]any{"targetPlayerId": 77, "subject": "Hello", "text": "Body", "dryRun": false, "confirm": preview.Confirmation}
	result, err = service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "send_message",
		AccessToken: "write",
		Arguments:   executeArgs,
	})
	if err != nil {
		t.Fatalf("send_message execute returned error: %v", err)
	}
	sent := result.StructuredContent.(map[string]any)["sendMessage"].(domainmcp.SendMessageResult)
	if sent.DryRun || !sent.Executed || sent.RequiresConfirmation || sent.Confirmation != preview.Confirmation {
		t.Fatalf("unexpected sent result: %+v", sent)
	}
	if repository.sendPlayerID != 42 || repository.sendCommand.Confirm != preview.Confirmation || repository.sendCommand.DryRun {
		t.Fatalf("unexpected send command: player=%d command=%+v", repository.sendPlayerID, repository.sendCommand)
	}
}

func TestServiceSendMessageRequiresScopeRepositoryAndValidConfirmation(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"messages": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeMessages}},
			"write":    {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeMessageWrite}},
		},
	})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "send_message", AccessToken: "write", Arguments: map[string]any{"targetPlayerId": 77, "subject": "Hello", "text": "Body"}}); err == nil {
		t.Fatalf("expected missing repository error")
	}

	service = service.WithWriteRepository(&fakeWriteRepository{})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "send_message", AccessToken: "messages", Arguments: map[string]any{"targetPlayerId": 77, "subject": "Hello", "text": "Body"}}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without message write scope, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "send_message", AccessToken: "write", Arguments: map[string]any{"targetPlayerId": true, "subject": "Hello", "text": "Body"}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid target error, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "send_message", AccessToken: "write", Arguments: map[string]any{"targetPlayerId": 77, "subject": "Hello", "text": "Body", "dryRun": false}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected missing confirmation error, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "send_message", AccessToken: "write", Arguments: map[string]any{"targetPlayerId": 77, "subject": "Hello", "text": "Body", "dryRun": false, "confirm": "wrong"}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected wrong confirmation error, got %v", err)
	}

	service = service.WithWriteRepository(&fakeWriteRepository{err: errors.New("send down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "send_message", AccessToken: "write", Arguments: map[string]any{"targetPlayerId": 77, "subject": "Hello", "text": "Body"}}); err == nil {
		t.Fatalf("expected repository error")
	}
}

func TestServiceCallsDeleteMessagesWithDryRunAndConfirmation(t *testing.T) {
	repository := &fakeWriteRepository{
		deletePreview: domainmcp.DeleteMessagesResult{PlayerID: 42, MessageIDs: []int{5, 7}, DeleteCount: 2},
		deleted:       domainmcp.DeleteMessagesResult{PlayerID: 42, MessageIDs: []int{5, 7}, DeleteCount: 2, Executed: true},
	}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeMessageWrite}},
		},
	}).WithWriteRepository(repository)

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "delete_messages",
		AccessToken: "write",
		Arguments:   map[string]any{"messageIds": []any{float64(7), float64(5), float64(5)}},
	})
	if err != nil {
		t.Fatalf("delete_messages dry-run returned error: %v", err)
	}
	preview := result.StructuredContent.(map[string]any)["deleteMessages"].(domainmcp.DeleteMessagesResult)
	if !preview.DryRun || preview.Executed || !preview.RequiresConfirmation || !strings.HasPrefix(preview.Confirmation, "delete_messages:5,7:") {
		t.Fatalf("unexpected delete preview: %+v", preview)
	}
	if repository.deletePreviewPlayerID != 42 || len(repository.deletePreviewCommand.MessageIDs) != 2 || repository.deletePreviewCommand.MessageIDs[0] != 5 || repository.deletePreviewCommand.MessageIDs[1] != 7 {
		t.Fatalf("unexpected delete preview command: player=%d command=%+v", repository.deletePreviewPlayerID, repository.deletePreviewCommand)
	}

	result, err = service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "delete_messages",
		AccessToken: "write",
		Arguments:   map[string]any{"messageIds": []any{5, 7}, "dryRun": false, "confirm": preview.Confirmation},
	})
	if err != nil {
		t.Fatalf("delete_messages execute returned error: %v", err)
	}
	deleted := result.StructuredContent.(map[string]any)["deleteMessages"].(domainmcp.DeleteMessagesResult)
	if deleted.DryRun || !deleted.Executed || deleted.RequiresConfirmation || deleted.Confirmation != preview.Confirmation {
		t.Fatalf("unexpected delete execute: %+v", deleted)
	}
	if repository.deletePlayerID != 42 || repository.deleteCommand.Confirm != preview.Confirmation || repository.deleteCommand.DryRun {
		t.Fatalf("unexpected delete command: player=%d command=%+v", repository.deletePlayerID, repository.deleteCommand)
	}
}

func TestServiceDeleteMessagesRequiresScopeRepositoryAndValidConfirmation(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"messages": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeMessages}},
			"write":    {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeMessageWrite}},
		},
	})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "delete_messages", AccessToken: "write", Arguments: map[string]any{"messageIds": []any{7}}}); err == nil {
		t.Fatalf("expected missing repository error")
	}

	service = service.WithWriteRepository(&fakeWriteRepository{})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "delete_messages", AccessToken: "messages", Arguments: map[string]any{"messageIds": []any{7}}}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without message write scope, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "delete_messages", AccessToken: "write", Arguments: map[string]any{"messageIds": true}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid ids error, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "delete_messages", AccessToken: "write", Arguments: map[string]any{"messageIds": []any{7}, "dryRun": false}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected missing confirmation error, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "delete_messages", AccessToken: "write", Arguments: map[string]any{"messageIds": []any{7}, "dryRun": false, "confirm": "wrong"}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected wrong confirmation error, got %v", err)
	}

	service = service.WithWriteRepository(&fakeWriteRepository{err: errors.New("delete down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "delete_messages", AccessToken: "write", Arguments: map[string]any{"messageIds": []any{7}}}); err == nil {
		t.Fatalf("expected repository error")
	}
}

func TestServiceCallsReportMessageWithDryRunAndConfirmation(t *testing.T) {
	repository := &fakeWriteRepository{
		reportPreview: domainmcp.ReportMessageResult{PlayerID: 42, MessageID: 7, Reportable: true},
		reported:      domainmcp.ReportMessageResult{PlayerID: 42, MessageID: 7, Reportable: true, Executed: true},
	}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeMessageWrite}},
		},
	}).WithWriteRepository(repository)

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "report_message",
		AccessToken: "write",
		Arguments:   map[string]any{"messageId": float64(7)},
	})
	if err != nil {
		t.Fatalf("report_message dry-run returned error: %v", err)
	}
	preview := result.StructuredContent.(map[string]any)["reportMessage"].(domainmcp.ReportMessageResult)
	if !preview.DryRun || preview.Executed || !preview.Reportable || !preview.RequiresConfirmation || !strings.HasPrefix(preview.Confirmation, "report_message:7:") {
		t.Fatalf("unexpected report preview: %+v", preview)
	}
	if repository.reportPreviewPlayerID != 42 || repository.reportPreviewCommand.MessageID != 7 || !repository.reportPreviewCommand.DryRun {
		t.Fatalf("unexpected report preview command: player=%d command=%+v", repository.reportPreviewPlayerID, repository.reportPreviewCommand)
	}

	result, err = service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "report_message",
		AccessToken: "write",
		Arguments:   map[string]any{"messageId": 7, "dryRun": false, "confirm": preview.Confirmation},
	})
	if err != nil {
		t.Fatalf("report_message execute returned error: %v", err)
	}
	reported := result.StructuredContent.(map[string]any)["reportMessage"].(domainmcp.ReportMessageResult)
	if reported.DryRun || !reported.Executed || reported.RequiresConfirmation || reported.Confirmation != preview.Confirmation {
		t.Fatalf("unexpected report execute: %+v", reported)
	}
	if repository.reportPlayerID != 42 || repository.reportCommand.Confirm != preview.Confirmation || repository.reportCommand.DryRun {
		t.Fatalf("unexpected report command: player=%d command=%+v", repository.reportPlayerID, repository.reportCommand)
	}
}

func TestServiceReportMessageDryRunIssueDoesNotRequireConfirmation(t *testing.T) {
	repository := &fakeWriteRepository{
		reportPreview: domainmcp.ReportMessageResult{
			PlayerID:   42,
			MessageID:  7,
			Reportable: true,
			Issue:      &domainmcp.ActionIssue{Code: "report_exists", Message: "already reported"},
		},
	}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeMessageWrite}},
		},
	}).WithWriteRepository(repository)

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "report_message",
		AccessToken: "write",
		Arguments:   map[string]any{"messageId": 7},
	})
	if err != nil {
		t.Fatalf("report_message dry-run issue returned error: %v", err)
	}
	preview := result.StructuredContent.(map[string]any)["reportMessage"].(domainmcp.ReportMessageResult)
	if !preview.DryRun || preview.RequiresConfirmation || preview.Confirmation != "" || preview.Issue == nil {
		t.Fatalf("expected issue dry-run without confirmation, got %+v", preview)
	}
}

func TestServiceReportMessageRequiresScopeRepositoryAndValidConfirmation(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"messages": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeMessages}},
			"write":    {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeMessageWrite}},
		},
	})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "report_message", AccessToken: "write", Arguments: map[string]any{"messageId": 7}}); err == nil {
		t.Fatalf("expected missing repository error")
	}

	service = service.WithWriteRepository(&fakeWriteRepository{})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "report_message", AccessToken: "messages", Arguments: map[string]any{"messageId": 7}}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without message write scope, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "report_message", AccessToken: "write", Arguments: map[string]any{"messageId": true}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid id error, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "report_message", AccessToken: "write", Arguments: map[string]any{"messageId": 7, "dryRun": false}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected missing confirmation error, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "report_message", AccessToken: "write", Arguments: map[string]any{"messageId": 7, "dryRun": false, "confirm": "wrong"}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected wrong confirmation error, got %v", err)
	}

	service = service.WithWriteRepository(&fakeWriteRepository{err: errors.New("report down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "report_message", AccessToken: "write", Arguments: map[string]any{"messageId": 7}}); err == nil {
		t.Fatalf("expected repository error")
	}
}

func TestServiceValidatesFleetDispatchWithConfirmation(t *testing.T) {
	repository := &fakeFleetWriteRepository{
		dispatchPreview: domainmcp.DispatchFleetValidationResult{
			PlayerID:        42,
			PlanetID:        99,
			Ready:           true,
			TotalShips:      1,
			Mission:         3,
			Target:          domainmcp.Coordinates{Galaxy: 2, System: 3, Position: 4},
			TargetType:      1,
			FuelConsumption: 12,
			Cargo:           5000,
		},
	}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"fleet-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeFleetWrite}},
		},
	}).WithFleetWriteRepository(repository)
	arguments := map[string]any{
		"planetId":       99,
		"ships":          map[string]any{"202": float64(1)},
		"resources":      map[string]any{"metal": float64(10), "crystal": float64(0), "deuterium": float64(0)},
		"targetGalaxy":   2,
		"targetSystem":   3,
		"targetPosition": 4,
		"targetType":     1,
		"mission":        3,
		"speed":          10,
	}

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "validate_fleet_dispatch",
		AccessToken: "fleet-write",
		Arguments:   arguments,
	})
	if err != nil {
		t.Fatalf("validate_fleet_dispatch returned error: %v", err)
	}
	preview := result.StructuredContent.(map[string]any)["fleetDispatchValidation"].(domainmcp.DispatchFleetValidationResult)
	if !preview.DryRun || !preview.Ready || !preview.RequiresConfirmation || !strings.HasPrefix(preview.Confirmation, "dispatch_fleet:") {
		t.Fatalf("unexpected dispatch validation: %+v", preview)
	}
	if repository.dispatchPreviewPlayerID != 42 || repository.dispatchPreviewCommand.Ships[202] != 1 || repository.dispatchPreviewCommand.Resources.Metal != 10 {
		t.Fatalf("unexpected dispatch command: player=%d command=%+v", repository.dispatchPreviewPlayerID, repository.dispatchPreviewCommand)
	}
}

func TestServiceValidateFleetDispatchIssueDoesNotRequireConfirmation(t *testing.T) {
	repository := &fakeFleetWriteRepository{
		dispatchPreview: domainmcp.DispatchFleetValidationResult{
			PlayerID: 42,
			PlanetID: 99,
			Issue: &domainmcp.ActionIssue{
				Code:    "no_ships",
				Message: "No ships have been selected.",
			},
		},
	}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"fleet-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeFleetWrite}},
		},
	}).WithFleetWriteRepository(repository)

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "validate_fleet_dispatch",
		AccessToken: "fleet-write",
		Arguments: map[string]any{
			"ships":          map[string]any{"202": 1},
			"targetGalaxy":   2,
			"targetSystem":   3,
			"targetPosition": 4,
			"targetType":     1,
			"mission":        3,
		},
	})
	if err != nil {
		t.Fatalf("validate_fleet_dispatch issue returned error: %v", err)
	}
	preview := result.StructuredContent.(map[string]any)["fleetDispatchValidation"].(domainmcp.DispatchFleetValidationResult)
	if !preview.DryRun || preview.RequiresConfirmation || preview.Confirmation != "" || preview.Issue == nil {
		t.Fatalf("expected issue validation without confirmation, got %+v", preview)
	}
}

func TestServiceValidateFleetDispatchRequiresScopeRepositoryAndValidParams(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"fleet":       {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeFleet}},
			"fleet-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeFleetWrite}},
		},
	})
	valid := map[string]any{"ships": map[string]any{"202": 1}, "targetGalaxy": 2, "targetSystem": 3, "targetPosition": 4, "targetType": 1, "mission": 3}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "validate_fleet_dispatch", AccessToken: "fleet-write", Arguments: valid}); err == nil {
		t.Fatalf("expected missing repository error")
	}

	service = service.WithFleetWriteRepository(&fakeFleetWriteRepository{})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "validate_fleet_dispatch", AccessToken: "fleet", Arguments: valid}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without fleet write scope, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "validate_fleet_dispatch", AccessToken: "fleet-write", Arguments: map[string]any{"ships": true}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid ships error, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "validate_fleet_dispatch", AccessToken: "fleet-write", Arguments: map[string]any{"ships": map[string]any{"bad": 1}, "targetGalaxy": 2, "targetSystem": 3, "targetPosition": 4, "targetType": 1, "mission": 3}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid ship id error, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "validate_fleet_dispatch", AccessToken: "fleet-write", Arguments: map[string]any{"ships": map[string]any{"202": 1}, "resources": true, "targetGalaxy": 2, "targetSystem": 3, "targetPosition": 4, "targetType": 1, "mission": 3}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid resources error, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "validate_fleet_dispatch", AccessToken: "fleet-write", Arguments: map[string]any{"ships": map[string]any{"202": 1}, "targetGalaxy": 0, "targetSystem": 3, "targetPosition": 4, "targetType": 1, "mission": 3}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid target error, got %v", err)
	}

	service = service.WithFleetWriteRepository(&fakeFleetWriteRepository{err: errors.New("dispatch down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "validate_fleet_dispatch", AccessToken: "fleet-write", Arguments: valid}); err == nil {
		t.Fatalf("expected repository error")
	}
}

func TestServiceDispatchFleetWithConfirmation(t *testing.T) {
	repository := &fakeFleetWriteRepository{
		dispatched: domainmcp.DispatchFleetValidationResult{
			PlayerID:   42,
			PlanetID:   99,
			Ready:      true,
			Executed:   true,
			TotalShips: 1,
			Mission:    3,
			Target:     domainmcp.Coordinates{Galaxy: 2, System: 3, Position: 4},
			TargetType: 1,
		},
	}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"fleet-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeFleetWrite}},
		},
	}).WithFleetWriteRepository(repository)
	arguments := map[string]any{
		"planetId":       99,
		"ships":          map[string]any{"202": 1},
		"targetGalaxy":   2,
		"targetSystem":   3,
		"targetPosition": 4,
		"targetType":     1,
		"mission":        3,
		"speed":          10,
	}
	command, err := mcpDispatchFleetCommand(arguments)
	if err != nil {
		t.Fatalf("unexpected command error: %v", err)
	}
	arguments["confirm"] = mcpDispatchFleetConfirmation(command)

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "dispatch_fleet",
		AccessToken: "fleet-write",
		Arguments:   arguments,
	})
	if err != nil {
		t.Fatalf("dispatch_fleet returned error: %v", err)
	}
	dispatched := result.StructuredContent.(map[string]any)["fleetDispatch"].(domainmcp.DispatchFleetValidationResult)
	if dispatched.DryRun || !dispatched.Executed || dispatched.RequiresConfirmation || !strings.HasPrefix(dispatched.Confirmation, "dispatch_fleet:") {
		t.Fatalf("unexpected dispatch result: %+v", dispatched)
	}
	if repository.dispatchPlayerID != 42 || repository.dispatchCommand.Ships[202] != 1 {
		t.Fatalf("unexpected dispatch command: player=%d command=%+v", repository.dispatchPlayerID, repository.dispatchCommand)
	}

	arguments["confirm"] = "wrong"
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "dispatch_fleet", AccessToken: "fleet-write", Arguments: arguments}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected wrong confirmation error, got %v", err)
	}
	arguments["confirm"] = true
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "dispatch_fleet", AccessToken: "fleet-write", Arguments: arguments}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid confirmation type, got %v", err)
	}
}

func TestServiceDispatchFleetRequiresScopeRepositoryAndValidParams(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"fleet":       {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeFleet}},
			"fleet-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeFleetWrite}},
		},
	})
	valid := map[string]any{"ships": map[string]any{"202": 1}, "targetGalaxy": 2, "targetSystem": 3, "targetPosition": 4, "targetType": 1, "mission": 3}
	command, err := mcpDispatchFleetCommand(valid)
	if err != nil {
		t.Fatalf("unexpected command error: %v", err)
	}
	valid["confirm"] = mcpDispatchFleetConfirmation(command)

	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "dispatch_fleet", AccessToken: "fleet-write", Arguments: valid}); err == nil {
		t.Fatalf("expected missing repository error")
	}

	service = service.WithFleetWriteRepository(&fakeFleetWriteRepository{})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "dispatch_fleet", AccessToken: "fleet", Arguments: valid}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without fleet write scope, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "dispatch_fleet", AccessToken: "fleet-write", Arguments: map[string]any{"ships": true, "confirm": "wrong"}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid ships error, got %v", err)
	}

	service = service.WithFleetWriteRepository(&fakeFleetWriteRepository{err: errors.New("dispatch down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "dispatch_fleet", AccessToken: "fleet-write", Arguments: valid}); err == nil || !strings.Contains(err.Error(), "dispatch down") {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestServiceCallsCancelBuildingQueueWithDryRunAndConfirmation(t *testing.T) {
	repository := &fakeQueueWriteRepository{
		preview:  domainmcp.CancelBuildingQueueResult{PlayerID: 42, PlanetID: 99, ListID: 2, TechID: 1, Name: "Metal Mine", Level: 3, Cancelable: true},
		canceled: domainmcp.CancelBuildingQueueResult{PlayerID: 42, PlanetID: 99, ListID: 2, TechID: 1, Name: "Metal Mine", Level: 3, Cancelable: true, Executed: true},
	}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"queue-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeQueueWrite}},
		},
	}).WithQueueWriteRepository(repository)

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "cancel_building_queue",
		AccessToken: "queue-write",
		Arguments:   map[string]any{"planetId": 99, "listId": 2},
	})
	if err != nil {
		t.Fatalf("cancel_building_queue dry-run returned error: %v", err)
	}
	dryRun := result.StructuredContent.(map[string]any)["cancelBuildingQueue"].(domainmcp.CancelBuildingQueueResult)
	if !dryRun.DryRun || dryRun.Executed || !dryRun.RequiresConfirmation || !strings.HasPrefix(dryRun.Confirmation, "cancel_building_queue:") {
		t.Fatalf("unexpected dry-run result: %+v", dryRun)
	}
	if repository.previewPlayerID != 42 || repository.previewCommand.PlanetID != 99 || repository.previewCommand.ListID != 2 {
		t.Fatalf("unexpected preview command: player=%d command=%+v", repository.previewPlayerID, repository.previewCommand)
	}

	result, err = service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "cancel_building_queue",
		AccessToken: "queue-write",
		Arguments:   map[string]any{"planetId": 99, "listId": 2, "dryRun": false, "confirm": dryRun.Confirmation},
	})
	if err != nil {
		t.Fatalf("cancel_building_queue execute returned error: %v", err)
	}
	canceled := result.StructuredContent.(map[string]any)["cancelBuildingQueue"].(domainmcp.CancelBuildingQueueResult)
	if canceled.DryRun || !canceled.Executed || canceled.RequiresConfirmation {
		t.Fatalf("unexpected execute result: %+v", canceled)
	}
	if repository.cancelPlayerID != 42 || repository.cancelCommand.ListID != 2 {
		t.Fatalf("unexpected cancel command: player=%d command=%+v", repository.cancelPlayerID, repository.cancelCommand)
	}

	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "cancel_building_queue", AccessToken: "queue-write", Arguments: map[string]any{"listId": 2, "dryRun": false, "confirm": "wrong"}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected wrong confirmation error, got %v", err)
	}
}

func TestServiceCancelBuildingQueueRequiresScopeRepositoryAndValidParams(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read":        {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
			"queue-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeQueueWrite}},
		},
	})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "cancel_building_queue", AccessToken: "queue-write", Arguments: map[string]any{"listId": 1}}); err == nil {
		t.Fatalf("expected missing repository error")
	}

	service = service.WithQueueWriteRepository(&fakeQueueWriteRepository{})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "cancel_building_queue", AccessToken: "read", Arguments: map[string]any{"listId": 1}}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without queue write scope, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "cancel_building_queue", AccessToken: "queue-write", Arguments: map[string]any{"listId": 0}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid list id error, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "cancel_building_queue", AccessToken: "queue-write", Arguments: map[string]any{"listId": 1, "dryRun": "no"}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid dry-run error, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "cancel_building_queue", AccessToken: "queue-write", Arguments: map[string]any{"listId": 1, "confirm": true}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid confirm error, got %v", err)
	}

	service = service.WithQueueWriteRepository(&fakeQueueWriteRepository{err: errors.New("queue down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "cancel_building_queue", AccessToken: "queue-write", Arguments: map[string]any{"listId": 1}}); err == nil || !strings.Contains(err.Error(), "queue down") {
		t.Fatalf("expected repository error, got %v", err)
	}

	confirm := mcpCancelBuildingQueueConfirmation(domainmcp.CancelBuildingQueueCommand{ListID: 1})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "cancel_building_queue", AccessToken: "queue-write", Arguments: map[string]any{"listId": 1, "dryRun": false, "confirm": confirm}}); err == nil || !strings.Contains(err.Error(), "queue down") {
		t.Fatalf("expected execute repository error, got %v", err)
	}
}

func TestServiceCallsCancelResearchQueueWithDryRunAndConfirmation(t *testing.T) {
	repository := &fakeQueueWriteRepository{
		researchPreview:  domainmcp.CancelResearchQueueResult{PlayerID: 42, PlanetID: 99, TaskID: 7, TechID: 113, Name: "Energy Technology", Level: 2, Cancelable: true},
		researchCanceled: domainmcp.CancelResearchQueueResult{PlayerID: 42, PlanetID: 99, TaskID: 7, TechID: 113, Name: "Energy Technology", Level: 2, Cancelable: true, Executed: true},
	}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"queue-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeQueueWrite}},
		},
	}).WithQueueWriteRepository(repository)

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "cancel_research_queue",
		AccessToken: "queue-write",
	})
	if err != nil {
		t.Fatalf("cancel_research_queue dry-run returned error: %v", err)
	}
	dryRun := result.StructuredContent.(map[string]any)["cancelResearchQueue"].(domainmcp.CancelResearchQueueResult)
	if !dryRun.DryRun || dryRun.Executed || !dryRun.RequiresConfirmation || !strings.HasPrefix(dryRun.Confirmation, "cancel_research_queue:") {
		t.Fatalf("unexpected dry-run result: %+v", dryRun)
	}
	if repository.researchPreviewPlayerID != 42 {
		t.Fatalf("unexpected preview player: %d", repository.researchPreviewPlayerID)
	}

	result, err = service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "cancel_research_queue",
		AccessToken: "queue-write",
		Arguments:   map[string]any{"dryRun": false, "confirm": dryRun.Confirmation},
	})
	if err != nil {
		t.Fatalf("cancel_research_queue execute returned error: %v", err)
	}
	canceled := result.StructuredContent.(map[string]any)["cancelResearchQueue"].(domainmcp.CancelResearchQueueResult)
	if canceled.DryRun || !canceled.Executed || canceled.RequiresConfirmation {
		t.Fatalf("unexpected execute result: %+v", canceled)
	}
	if repository.researchCancelPlayerID != 42 || repository.researchCancelCommand.Confirm != dryRun.Confirmation {
		t.Fatalf("unexpected cancel command: player=%d command=%+v", repository.researchCancelPlayerID, repository.researchCancelCommand)
	}

	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "cancel_research_queue", AccessToken: "queue-write", Arguments: map[string]any{"dryRun": false, "confirm": "wrong"}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected wrong confirmation error, got %v", err)
	}
}

func TestServiceCancelResearchQueueRequiresScopeRepositoryAndValidParams(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read":        {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
			"queue-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeQueueWrite}},
		},
	})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "cancel_research_queue", AccessToken: "queue-write"}); err == nil {
		t.Fatalf("expected missing repository error")
	}

	service = service.WithQueueWriteRepository(&fakeQueueWriteRepository{})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "cancel_research_queue", AccessToken: "read"}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without queue write scope, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "cancel_research_queue", AccessToken: "queue-write", Arguments: map[string]any{"dryRun": "no"}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid dry-run error, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "cancel_research_queue", AccessToken: "queue-write", Arguments: map[string]any{"confirm": true}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid confirm error, got %v", err)
	}

	service = service.WithQueueWriteRepository(&fakeQueueWriteRepository{err: errors.New("queue down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "cancel_research_queue", AccessToken: "queue-write"}); err == nil || !strings.Contains(err.Error(), "queue down") {
		t.Fatalf("expected repository error, got %v", err)
	}

	confirm := mcpCancelResearchQueueConfirmation()
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "cancel_research_queue", AccessToken: "queue-write", Arguments: map[string]any{"dryRun": false, "confirm": confirm}}); err == nil || !strings.Contains(err.Error(), "queue down") {
		t.Fatalf("expected execute repository error, got %v", err)
	}
}

func TestServiceCallsEnqueueShipyardOrderWithDryRunAndConfirmation(t *testing.T) {
	repository := &fakeQueueWriteRepository{
		shipyardPreview:  domainmcp.EnqueueShipyardOrderResult{PlayerID: 42, PlanetID: 99, Kind: "fleet", ItemID: 204, Name: "Light Fighter", Requested: 2, Amount: 2, MaxBuild: 10, DurationSeconds: 5},
		shipyardEnqueued: domainmcp.EnqueueShipyardOrderResult{PlayerID: 42, PlanetID: 99, Kind: "fleet", ItemID: 204, Name: "Light Fighter", Requested: 2, Amount: 2, MaxBuild: 10, DurationSeconds: 5, Executed: true},
	}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"queue-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeQueueWrite}},
		},
	}).WithQueueWriteRepository(repository)

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "enqueue_shipyard_order",
		AccessToken: "queue-write",
		Arguments:   map[string]any{"planetId": 99, "kind": "fleet", "itemId": 204, "amount": 2},
	})
	if err != nil {
		t.Fatalf("enqueue_shipyard_order dry-run returned error: %v", err)
	}
	dryRun := result.StructuredContent.(map[string]any)["enqueueShipyardOrder"].(domainmcp.EnqueueShipyardOrderResult)
	if !dryRun.DryRun || dryRun.Executed || !dryRun.RequiresConfirmation || !strings.HasPrefix(dryRun.Confirmation, "enqueue_shipyard_order:99:fleet:204:2:") {
		t.Fatalf("unexpected dry-run result: %+v", dryRun)
	}
	if repository.shipyardPreviewPlayerID != 42 || repository.shipyardPreviewCommand.PlanetID != 99 || repository.shipyardPreviewCommand.Kind != "fleet" || repository.shipyardPreviewCommand.ItemID != 204 || repository.shipyardPreviewCommand.Amount != 2 {
		t.Fatalf("unexpected preview command: player=%d command=%+v", repository.shipyardPreviewPlayerID, repository.shipyardPreviewCommand)
	}

	result, err = service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "enqueue_shipyard_order",
		AccessToken: "queue-write",
		Arguments:   map[string]any{"planetId": 99, "kind": "fleet", "itemId": 204, "amount": 2, "dryRun": false, "confirm": dryRun.Confirmation},
	})
	if err != nil {
		t.Fatalf("enqueue_shipyard_order execute returned error: %v", err)
	}
	enqueued := result.StructuredContent.(map[string]any)["enqueueShipyardOrder"].(domainmcp.EnqueueShipyardOrderResult)
	if enqueued.DryRun || !enqueued.Executed || enqueued.RequiresConfirmation {
		t.Fatalf("unexpected execute result: %+v", enqueued)
	}
	if repository.shipyardEnqueuePlayerID != 42 || repository.shipyardEnqueueCommand.Confirm != dryRun.Confirmation {
		t.Fatalf("unexpected enqueue command: player=%d command=%+v", repository.shipyardEnqueuePlayerID, repository.shipyardEnqueueCommand)
	}

	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "enqueue_shipyard_order", AccessToken: "queue-write", Arguments: map[string]any{"planetId": 99, "kind": "fleet", "itemId": 204, "amount": 2, "dryRun": false, "confirm": "wrong"}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected wrong confirmation error, got %v", err)
	}
}

func TestServiceEnqueueShipyardOrderRequiresScopeRepositoryAndValidParams(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read":        {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
			"queue-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeQueueWrite}},
		},
	})
	valid := map[string]any{"kind": "defense", "itemId": 401, "amount": 1}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "enqueue_shipyard_order", AccessToken: "queue-write", Arguments: valid}); err == nil {
		t.Fatalf("expected missing repository error")
	}

	service = service.WithQueueWriteRepository(&fakeQueueWriteRepository{})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "enqueue_shipyard_order", AccessToken: "read", Arguments: valid}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without queue write scope, got %v", err)
	}
	for _, arguments := range []map[string]any{
		{"planetId": "bad", "kind": "fleet", "itemId": 204, "amount": 1},
		{"kind": true, "itemId": 401, "amount": 1},
		{"kind": "bad", "itemId": 401, "amount": 1},
		{"kind": "fleet", "itemId": 0, "amount": 1},
		{"kind": "fleet", "itemId": 204, "amount": 0},
		{"kind": "fleet", "itemId": 204, "amount": 1, "dryRun": "no"},
		{"kind": "fleet", "itemId": 204, "amount": 1, "confirm": true},
	} {
		if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "enqueue_shipyard_order", AccessToken: "queue-write", Arguments: arguments}); !errors.Is(err, domainmcp.ErrInvalidParams) {
			t.Fatalf("expected invalid params for %+v, got %v", arguments, err)
		}
	}

	service = service.WithQueueWriteRepository(&fakeQueueWriteRepository{err: errors.New("queue down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "enqueue_shipyard_order", AccessToken: "queue-write", Arguments: valid}); err == nil || !strings.Contains(err.Error(), "queue down") {
		t.Fatalf("expected repository error, got %v", err)
	}

	command, err := mcpEnqueueShipyardOrderCommand(valid)
	if err != nil {
		t.Fatalf("valid command error: %v", err)
	}
	defaultCommand, err := mcpEnqueueShipyardOrderCommand(map[string]any{"itemId": "204", "amount": 2})
	if err != nil || defaultCommand.Kind != "fleet" || !defaultCommand.DryRun || defaultCommand.ItemID != 204 || defaultCommand.Amount != 2 {
		t.Fatalf("unexpected default command=%+v err=%v", defaultCommand, err)
	}
	confirm := mcpEnqueueShipyardOrderConfirmation(command)
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "enqueue_shipyard_order", AccessToken: "queue-write", Arguments: map[string]any{"kind": "defense", "itemId": 401, "amount": 1, "dryRun": false, "confirm": confirm}}); err == nil || !strings.Contains(err.Error(), "queue down") {
		t.Fatalf("expected execute repository error, got %v", err)
	}
}

func TestServiceCallsUpdateResourceProductionWithDryRunAndConfirmation(t *testing.T) {
	repository := &fakeResourceWriteRepository{
		preview: domainmcp.UpdateResourceProductionResult{
			PlayerID: 42,
			PlanetID: 99,
			Settings: []domainmcp.ResourceProductionSetting{
				{ID: 1, Name: "Metal Mine", Percent: 80},
				{ID: 2, Name: "Crystal Mine", Percent: 70},
			},
		},
		updated: domainmcp.UpdateResourceProductionResult{
			PlayerID: 42,
			PlanetID: 99,
			Settings: []domainmcp.ResourceProductionSetting{
				{ID: 1, Name: "Metal Mine", Percent: 80},
				{ID: 2, Name: "Crystal Mine", Percent: 70},
			},
			Executed: true,
		},
	}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"resources-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeResourcesWrite}},
		},
	}).WithResourceWriteRepository(repository)
	arguments := map[string]any{"planetId": 99, "production": map[string]any{"1": 80, "2": 70}}

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "update_resource_production",
		AccessToken: "resources-write",
		Arguments:   arguments,
	})
	if err != nil {
		t.Fatalf("update_resource_production dry-run returned error: %v", err)
	}
	dryRun := result.StructuredContent.(map[string]any)["updateResourceProduction"].(domainmcp.UpdateResourceProductionResult)
	if !dryRun.DryRun || dryRun.Executed || !dryRun.RequiresConfirmation || !strings.HasPrefix(dryRun.Confirmation, "update_resource_production:99:1=80,2=70:") {
		t.Fatalf("unexpected dry-run result: %+v", dryRun)
	}
	if repository.previewPlayerID != 42 || repository.previewCommand.PlanetID != 99 || repository.previewCommand.Production[1] != 80 || repository.previewCommand.Production[2] != 70 {
		t.Fatalf("unexpected preview command: player=%d command=%+v", repository.previewPlayerID, repository.previewCommand)
	}

	result, err = service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "update_resource_production",
		AccessToken: "resources-write",
		Arguments:   map[string]any{"planetId": 99, "production": map[string]any{"1": 80, "2": 70}, "dryRun": false, "confirm": dryRun.Confirmation},
	})
	if err != nil {
		t.Fatalf("update_resource_production execute returned error: %v", err)
	}
	updated := result.StructuredContent.(map[string]any)["updateResourceProduction"].(domainmcp.UpdateResourceProductionResult)
	if updated.DryRun || !updated.Executed || updated.RequiresConfirmation {
		t.Fatalf("unexpected execute result: %+v", updated)
	}
	if repository.updatePlayerID != 42 || repository.updateCommand.Confirm != dryRun.Confirmation {
		t.Fatalf("unexpected update command: player=%d command=%+v", repository.updatePlayerID, repository.updateCommand)
	}

	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "update_resource_production", AccessToken: "resources-write", Arguments: map[string]any{"planetId": 99, "production": map[string]any{"1": 80}, "dryRun": false, "confirm": "wrong"}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected wrong confirmation error, got %v", err)
	}
}

func TestServiceUpdateResourceProductionRequiresScopeRepositoryAndValidParams(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read":            {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
			"resources-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeResourcesWrite}},
		},
	})
	valid := map[string]any{"production": map[string]any{"1": 80}}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "update_resource_production", AccessToken: "resources-write", Arguments: valid}); err == nil {
		t.Fatalf("expected missing repository error")
	}

	service = service.WithResourceWriteRepository(&fakeResourceWriteRepository{})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "update_resource_production", AccessToken: "read", Arguments: valid}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without resources write scope, got %v", err)
	}
	for _, arguments := range []map[string]any{
		{"planetId": "bad", "production": map[string]any{"1": 80}},
		{"production": true},
		{"production": map[string]any{}},
		{"production": map[string]any{"x": 80}},
		{"production": map[string]any{"1": 101}},
		{"production": map[string]any{"1": 80}, "dryRun": "no"},
		{"production": map[string]any{"1": 80}, "confirm": true},
	} {
		if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "update_resource_production", AccessToken: "resources-write", Arguments: arguments}); !errors.Is(err, domainmcp.ErrInvalidParams) {
			t.Fatalf("expected invalid params for %+v, got %v", arguments, err)
		}
	}

	service = service.WithResourceWriteRepository(&fakeResourceWriteRepository{err: errors.New("resources down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "update_resource_production", AccessToken: "resources-write", Arguments: valid}); err == nil || !strings.Contains(err.Error(), "resources down") {
		t.Fatalf("expected repository error, got %v", err)
	}

	command, err := mcpUpdateResourceProductionCommand(valid)
	if err != nil {
		t.Fatalf("valid command error: %v", err)
	}
	confirm := mcpUpdateResourceProductionConfirmation(command)
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "update_resource_production", AccessToken: "resources-write", Arguments: map[string]any{"production": map[string]any{"1": 80}, "dryRun": false, "confirm": confirm}}); err == nil || !strings.Contains(err.Error(), "resources down") {
		t.Fatalf("expected execute repository error, got %v", err)
	}
}

func TestServiceCallsRecruitOfficerWithDryRunAndConfirmation(t *testing.T) {
	repository := &fakePremiumWriteRepository{
		preview: domainmcp.RecruitOfficerResult{
			PlayerID:       42,
			PlanetID:       99,
			OfficerID:      1,
			Name:           "Commander",
			Days:           7,
			Cost:           10000,
			PaidDarkMatter: 5000,
			FreeDarkMatter: 2000,
			Until:          1700604800,
			DaysLeft:       7,
			Active:         true,
		},
		recruited: domainmcp.RecruitOfficerResult{
			PlayerID:       42,
			PlanetID:       99,
			OfficerID:      1,
			Name:           "Commander",
			Days:           7,
			Cost:           10000,
			PaidDarkMatter: 5000,
			FreeDarkMatter: 2000,
			Until:          1700604800,
			DaysLeft:       7,
			Active:         true,
			Executed:       true,
		},
	}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"premium-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopePremiumWrite}},
		},
	}).WithPremiumWriteRepository(repository)
	arguments := map[string]any{"planetId": 99, "officerId": 1, "days": 7}

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "recruit_officer",
		AccessToken: "premium-write",
		Arguments:   arguments,
	})
	if err != nil {
		t.Fatalf("recruit_officer dry-run returned error: %v", err)
	}
	dryRun := result.StructuredContent.(map[string]any)["recruitOfficer"].(domainmcp.RecruitOfficerResult)
	if !dryRun.DryRun || dryRun.Executed || !dryRun.RequiresConfirmation || !strings.HasPrefix(dryRun.Confirmation, "recruit_officer:99:1:7:") {
		t.Fatalf("unexpected dry-run result: %+v", dryRun)
	}
	if repository.previewPlayerID != 42 || repository.previewCommand.PlanetID != 99 || repository.previewCommand.OfficerID != 1 || repository.previewCommand.Days != 7 {
		t.Fatalf("unexpected preview command: player=%d command=%+v", repository.previewPlayerID, repository.previewCommand)
	}

	result, err = service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "recruit_officer",
		AccessToken: "premium-write",
		Arguments:   map[string]any{"planetId": 99, "officerId": 1, "days": 7, "dryRun": false, "confirm": dryRun.Confirmation},
	})
	if err != nil {
		t.Fatalf("recruit_officer execute returned error: %v", err)
	}
	recruited := result.StructuredContent.(map[string]any)["recruitOfficer"].(domainmcp.RecruitOfficerResult)
	if recruited.DryRun || !recruited.Executed || recruited.RequiresConfirmation {
		t.Fatalf("unexpected execute result: %+v", recruited)
	}
	if repository.recruitPlayerID != 42 || repository.recruitCommand.Confirm != dryRun.Confirmation {
		t.Fatalf("unexpected recruit command: player=%d command=%+v", repository.recruitPlayerID, repository.recruitCommand)
	}

	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "recruit_officer", AccessToken: "premium-write", Arguments: map[string]any{"officerId": 1, "dryRun": false, "confirm": "wrong"}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected wrong confirmation error, got %v", err)
	}
}

func TestServiceRecruitOfficerRequiresScopeRepositoryAndValidParams(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"read":          {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeRead}},
			"premium-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopePremiumWrite}},
		},
	})
	valid := map[string]any{"officerId": 1}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "recruit_officer", AccessToken: "premium-write", Arguments: valid}); err == nil {
		t.Fatalf("expected missing repository error")
	}

	service = service.WithPremiumWriteRepository(&fakePremiumWriteRepository{})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "recruit_officer", AccessToken: "read", Arguments: valid}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without premium write scope, got %v", err)
	}
	for _, arguments := range []map[string]any{
		{"planetId": "bad", "officerId": 1},
		{"officerId": 0},
		{"officerId": 6},
		{"officerId": 1, "days": 1},
		{"officerId": 1, "dryRun": "no"},
		{"officerId": 1, "confirm": true},
	} {
		if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "recruit_officer", AccessToken: "premium-write", Arguments: arguments}); !errors.Is(err, domainmcp.ErrInvalidParams) {
			t.Fatalf("expected invalid params for %+v, got %v", arguments, err)
		}
	}

	service = service.WithPremiumWriteRepository(&fakePremiumWriteRepository{err: errors.New("premium down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "recruit_officer", AccessToken: "premium-write", Arguments: valid}); err == nil || !strings.Contains(err.Error(), "premium down") {
		t.Fatalf("expected repository error, got %v", err)
	}

	command, err := mcpRecruitOfficerCommand(valid)
	if err != nil || command.Days != 7 || !command.DryRun {
		t.Fatalf("unexpected default command=%+v err=%v", command, err)
	}
	confirm := mcpRecruitOfficerConfirmation(command)
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "recruit_officer", AccessToken: "premium-write", Arguments: map[string]any{"officerId": 1, "dryRun": false, "confirm": confirm}}); err == nil || !strings.Contains(err.Error(), "premium down") {
		t.Fatalf("expected execute repository error, got %v", err)
	}
}

func TestServiceCallsRecallFleetWithDryRunAndConfirmation(t *testing.T) {
	repository := &fakeFleetWriteRepository{
		preview:  domainmcp.RecallFleetResult{PlayerID: 42, FleetID: 55, OwnerID: 42, Mission: 3, TotalShips: 2, Recallable: true},
		recalled: domainmcp.RecallFleetResult{PlayerID: 42, FleetID: 55, OwnerID: 42, Mission: 3, TotalShips: 2, Recallable: true, Executed: true},
	}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"fleet-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeFleetWrite}},
		},
	}).WithFleetWriteRepository(repository)

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "recall_fleet",
		AccessToken: "fleet-write",
		Arguments:   map[string]any{"fleetId": float64(55)},
	})
	if err != nil {
		t.Fatalf("recall_fleet dry-run returned error: %v", err)
	}
	preview := result.StructuredContent.(map[string]any)["recallFleet"].(domainmcp.RecallFleetResult)
	if !preview.DryRun || preview.Executed || !preview.Recallable || !preview.RequiresConfirmation || !strings.HasPrefix(preview.Confirmation, "recall_fleet:55:") {
		t.Fatalf("unexpected recall preview: %+v", preview)
	}
	if repository.previewPlayerID != 42 || repository.previewCommand.FleetID != 55 || !repository.previewCommand.DryRun {
		t.Fatalf("unexpected recall preview command: player=%d command=%+v", repository.previewPlayerID, repository.previewCommand)
	}

	result, err = service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "recall_fleet",
		AccessToken: "fleet-write",
		Arguments:   map[string]any{"fleetId": 55, "dryRun": false, "confirm": preview.Confirmation},
	})
	if err != nil {
		t.Fatalf("recall_fleet execute returned error: %v", err)
	}
	recalled := result.StructuredContent.(map[string]any)["recallFleet"].(domainmcp.RecallFleetResult)
	if recalled.DryRun || !recalled.Executed || recalled.RequiresConfirmation || recalled.Confirmation != preview.Confirmation {
		t.Fatalf("unexpected recall execute: %+v", recalled)
	}
	if repository.recallPlayerID != 42 || repository.recallCommand.Confirm != preview.Confirmation || repository.recallCommand.DryRun {
		t.Fatalf("unexpected recall command: player=%d command=%+v", repository.recallPlayerID, repository.recallCommand)
	}
}

func TestServiceRecallFleetDryRunIssueDoesNotRequireConfirmation(t *testing.T) {
	repository := &fakeFleetWriteRepository{
		preview: domainmcp.RecallFleetResult{
			PlayerID: 42,
			FleetID:  55,
			Issue:    &domainmcp.ActionIssue{Code: "fleet_not_found", Message: "missing"},
		},
	}
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"fleet-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeFleetWrite}},
		},
	}).WithFleetWriteRepository(repository)

	result, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{
		Name:        "recall_fleet",
		AccessToken: "fleet-write",
		Arguments:   map[string]any{"fleetId": 55},
	})
	if err != nil {
		t.Fatalf("recall_fleet dry-run issue returned error: %v", err)
	}
	preview := result.StructuredContent.(map[string]any)["recallFleet"].(domainmcp.RecallFleetResult)
	if !preview.DryRun || preview.RequiresConfirmation || preview.Confirmation != "" || preview.Issue == nil {
		t.Fatalf("expected issue dry-run without confirmation, got %+v", preview)
	}
}

func TestServiceRecallFleetRequiresScopeRepositoryAndValidConfirmation(t *testing.T) {
	service := NewServiceWithTokenVerifier(fakeHealthProvider{}, fakeTokenVerifier{
		access: map[string]domainmcp.Access{
			"fleet":       {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeFleet}},
			"fleet-write": {Authenticated: true, PlayerID: 42, Scopes: []string{domainmcp.ScopeFleetWrite}},
		},
	})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "recall_fleet", AccessToken: "fleet-write", Arguments: map[string]any{"fleetId": 55}}); err == nil {
		t.Fatalf("expected missing repository error")
	}

	service = service.WithFleetWriteRepository(&fakeFleetWriteRepository{})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "recall_fleet", AccessToken: "fleet", Arguments: map[string]any{"fleetId": 55}}); !errors.Is(err, domainmcp.ErrForbidden) {
		t.Fatalf("expected forbidden without fleet write scope, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "recall_fleet", AccessToken: "fleet-write", Arguments: map[string]any{"fleetId": true}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid id error, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "recall_fleet", AccessToken: "fleet-write", Arguments: map[string]any{"fleetId": 55, "dryRun": "no"}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid dryRun error, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "recall_fleet", AccessToken: "fleet-write", Arguments: map[string]any{"fleetId": 55, "confirm": true}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid confirm error, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "recall_fleet", AccessToken: "fleet-write", Arguments: map[string]any{"fleetId": 55, "dryRun": false}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected missing confirmation error, got %v", err)
	}
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "recall_fleet", AccessToken: "fleet-write", Arguments: map[string]any{"fleetId": 55, "dryRun": false, "confirm": "wrong"}}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected wrong confirmation error, got %v", err)
	}

	service = service.WithFleetWriteRepository(&fakeFleetWriteRepository{err: errors.New("recall down")})
	if _, err := service.CallTool(context.Background(), domainmcp.CallToolCommand{Name: "recall_fleet", AccessToken: "fleet-write", Arguments: map[string]any{"fleetId": 55}}); err == nil {
		t.Fatalf("expected repository error")
	}
}

func TestMCPMessageQueryDefaultsCapsAndValidation(t *testing.T) {
	query, err := mcpMessageQuery(nil)
	if err != nil || query.Limit != 25 || query.HasMessageType || query.IncludeText {
		t.Fatalf("unexpected default message query: %+v err=%v", query, err)
	}

	query, err = mcpMessageQuery(map[string]any{"limit": float64(99), "includeText": false})
	if err != nil || query.Limit != 50 || query.IncludeText {
		t.Fatalf("unexpected capped message query: %+v err=%v", query, err)
	}

	query, err = mcpMessageQuery(map[string]any{"limit": float64(0), "includeText": nil})
	if err != nil || query.Limit != 25 || query.IncludeText {
		t.Fatalf("unexpected zero-limit message query: %+v err=%v", query, err)
	}

	query, err = mcpMessageQuery(map[string]any{"messageType": float64(3)})
	if err != nil || !query.HasMessageType || query.MessageType != 3 {
		t.Fatalf("unexpected message type query: %+v err=%v", query, err)
	}

	if _, err := mcpMessageQuery(map[string]any{"limit": true}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid message limit error, got %v", err)
	}
	if _, err := mcpMessageQuery(map[string]any{"messageType": true}); !errors.Is(err, domainmcp.ErrInvalidParams) {
		t.Fatalf("expected invalid message type error, got %v", err)
	}
}

func TestMCPDispatchFleetCommandValidationEdges(t *testing.T) {
	valid := func() map[string]any {
		return map[string]any{
			"planetId":        99,
			"ships":           map[string]any{"202": 1},
			"resources":       map[string]any{"metal": 1, "crystal": 2, "deuterium": 3},
			"targetGalaxy":    2,
			"targetSystem":    3,
			"targetPosition":  4,
			"targetType":      1,
			"mission":         3,
			"speed":           10,
			"holdHours":       0,
			"expeditionHours": 0,
			"unionId":         0,
		}
	}
	command, err := mcpDispatchFleetCommand(valid())
	if err != nil || command.PlanetID != 99 || command.Ships[202] != 1 || command.Resources.Deuterium != 3 || command.Target.Position != 4 {
		t.Fatalf("unexpected valid dispatch command: %+v err=%v", command, err)
	}

	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "missing ships", mutate: func(arguments map[string]any) { delete(arguments, "ships") }},
		{name: "zero ships", mutate: func(arguments map[string]any) { arguments["ships"] = map[string]any{"202": 0} }},
		{name: "bad ship count", mutate: func(arguments map[string]any) { arguments["ships"] = map[string]any{"202": "x"} }},
		{name: "bad target galaxy", mutate: func(arguments map[string]any) { arguments["targetGalaxy"] = 0 }},
		{name: "bad target system", mutate: func(arguments map[string]any) { arguments["targetSystem"] = 0 }},
		{name: "bad target position", mutate: func(arguments map[string]any) { arguments["targetPosition"] = 0 }},
		{name: "bad target type", mutate: func(arguments map[string]any) { arguments["targetType"] = 0 }},
		{name: "bad mission", mutate: func(arguments map[string]any) { arguments["mission"] = 0 }},
		{name: "bad planet", mutate: func(arguments map[string]any) { arguments["planetId"] = "x" }},
		{name: "bad speed", mutate: func(arguments map[string]any) { arguments["speed"] = "x" }},
		{name: "bad hold", mutate: func(arguments map[string]any) { arguments["holdHours"] = "x" }},
		{name: "bad expedition", mutate: func(arguments map[string]any) { arguments["expeditionHours"] = "x" }},
		{name: "bad union", mutate: func(arguments map[string]any) { arguments["unionId"] = "x" }},
		{name: "bad resource value", mutate: func(arguments map[string]any) { arguments["resources"] = map[string]any{"metal": "x"} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arguments := valid()
			tt.mutate(arguments)
			if _, err := mcpDispatchFleetCommand(arguments); !errors.Is(err, domainmcp.ErrInvalidParams) {
				t.Fatalf("expected invalid params, got %v", err)
			}
		})
	}
}

func TestMCPSendMessageCommandDefaultsAndValidation(t *testing.T) {
	command, err := mcpSendMessageCommand(map[string]any{"targetPlayerId": float64(77), "subject": "Hello", "text": "Body"})
	if err != nil || command.TargetPlayerID != 77 || command.Subject != "Hello" || command.Text != "Body" || !command.DryRun {
		t.Fatalf("unexpected default send command: %+v err=%v", command, err)
	}

	command, err = mcpSendMessageCommand(map[string]any{"targetPlayerId": "77", "subject": "", "text": "", "dryRun": false, "confirm": "send_message:77:test"})
	if err != nil || command.TargetPlayerID != 77 || command.DryRun || command.Confirm != "send_message:77:test" {
		t.Fatalf("unexpected explicit send command: %+v err=%v", command, err)
	}

	confirmation := mcpSendMessageConfirmation(domainmcp.SendMessageCommand{TargetPlayerID: 77, Subject: "Hello", Text: "Body"})
	if !strings.HasPrefix(confirmation, "send_message:77:") || confirmation != mcpSendMessageConfirmation(domainmcp.SendMessageCommand{TargetPlayerID: 77, Subject: "Hello", Text: "Body"}) {
		t.Fatalf("unexpected confirmation: %q", confirmation)
	}
	if confirmation == mcpSendMessageConfirmation(domainmcp.SendMessageCommand{TargetPlayerID: 78, Subject: "Hello", Text: "Body"}) {
		t.Fatalf("confirmation should bind target")
	}

	for _, args := range []map[string]any{
		nil,
		{"targetPlayerId": 0, "subject": "Hello", "text": "Body"},
		{"targetPlayerId": 77, "subject": true, "text": "Body"},
		{"targetPlayerId": 77, "subject": "Hello", "text": true},
		{"targetPlayerId": 77, "subject": "Hello", "text": "Body", "dryRun": "false"},
		{"targetPlayerId": 77, "subject": "Hello", "text": "Body", "confirm": false},
	} {
		if _, err := mcpSendMessageCommand(args); !errors.Is(err, domainmcp.ErrInvalidParams) {
			t.Fatalf("expected invalid params for %+v, got %v", args, err)
		}
	}
}

func TestMCPDeleteMessagesCommandDefaultsAndValidation(t *testing.T) {
	command, err := mcpDeleteMessagesCommand(map[string]any{"messageIds": []any{float64(7), 5, "7", 0}})
	if err != nil || len(command.MessageIDs) != 2 || command.MessageIDs[0] != 5 || command.MessageIDs[1] != 7 || !command.DryRun {
		t.Fatalf("unexpected default delete command: %+v err=%v", command, err)
	}

	command, err = mcpDeleteMessagesCommand(map[string]any{"messageIds": []any{json.Number("8")}, "dryRun": false, "confirm": "delete_messages:8:test"})
	if err != nil || len(command.MessageIDs) != 1 || command.MessageIDs[0] != 8 || command.DryRun || command.Confirm != "delete_messages:8:test" {
		t.Fatalf("unexpected explicit delete command: %+v err=%v", command, err)
	}

	confirmation := mcpDeleteMessagesConfirmation(domainmcp.DeleteMessagesCommand{MessageIDs: []int{5, 7}})
	if !strings.HasPrefix(confirmation, "delete_messages:5,7:") || confirmation != mcpDeleteMessagesConfirmation(domainmcp.DeleteMessagesCommand{MessageIDs: []int{5, 7}}) {
		t.Fatalf("unexpected confirmation: %q", confirmation)
	}
	if confirmation == mcpDeleteMessagesConfirmation(domainmcp.DeleteMessagesCommand{MessageIDs: []int{5, 8}}) {
		t.Fatalf("confirmation should bind message ids")
	}

	for _, args := range []map[string]any{
		nil,
		{"messageIds": []any{}},
		{"messageIds": []any{0}},
		{"messageIds": true},
		{"messageIds": []any{true}},
		{"messageIds": []any{-1}},
		{"messageIds": []any{1.5}},
		{"messageIds": []any{7}, "dryRun": "false"},
		{"messageIds": []any{7}, "confirm": false},
	} {
		if _, err := mcpDeleteMessagesCommand(args); !errors.Is(err, domainmcp.ErrInvalidParams) {
			t.Fatalf("expected invalid params for %+v, got %v", args, err)
		}
	}
}

func TestMCPReportMessageCommandDefaultsAndValidation(t *testing.T) {
	command, err := mcpReportMessageCommand(map[string]any{"messageId": float64(7)})
	if err != nil || command.MessageID != 7 || !command.DryRun {
		t.Fatalf("unexpected default report command: %+v err=%v", command, err)
	}

	command, err = mcpReportMessageCommand(map[string]any{"messageId": "8", "dryRun": false, "confirm": "report_message:8:test"})
	if err != nil || command.MessageID != 8 || command.DryRun || command.Confirm != "report_message:8:test" {
		t.Fatalf("unexpected explicit report command: %+v err=%v", command, err)
	}

	confirmation := mcpReportMessageConfirmation(domainmcp.ReportMessageCommand{MessageID: 7})
	if !strings.HasPrefix(confirmation, "report_message:7:") || confirmation != mcpReportMessageConfirmation(domainmcp.ReportMessageCommand{MessageID: 7}) {
		t.Fatalf("unexpected confirmation: %q", confirmation)
	}
	if confirmation == mcpReportMessageConfirmation(domainmcp.ReportMessageCommand{MessageID: 8}) {
		t.Fatalf("confirmation should bind message id")
	}

	for _, args := range []map[string]any{
		nil,
		{"messageId": 0},
		{"messageId": true},
		{"messageId": -1},
		{"messageId": 1.5},
		{"messageId": 7, "dryRun": "false"},
		{"messageId": 7, "confirm": false},
	} {
		if _, err := mcpReportMessageCommand(args); !errors.Is(err, domainmcp.ErrInvalidParams) {
			t.Fatalf("expected invalid params for %+v, got %v", args, err)
		}
	}
}

func TestOptionalBoolArgument(t *testing.T) {
	for _, tt := range []struct {
		name    string
		args    map[string]any
		want    bool
		wantErr bool
	}{
		{name: "nil args"},
		{name: "missing", args: map[string]any{"other": true}},
		{name: "nil value", args: map[string]any{"includeText": nil}},
		{name: "true", args: map[string]any{"includeText": true}, want: true},
		{name: "false", args: map[string]any{"includeText": false}},
		{name: "invalid", args: map[string]any{"includeText": "true"}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := optionalBoolArgument(tt.args, "includeText")
			if tt.wantErr {
				if !errors.Is(err, domainmcp.ErrInvalidParams) {
					t.Fatalf("expected invalid params error, got %v", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("optionalBoolArgument got=%v err=%v want=%v", got, err, tt.want)
			}
		})
	}
}

func TestOptionalStringArgument(t *testing.T) {
	for _, tt := range []struct {
		name    string
		args    map[string]any
		want    string
		wantErr bool
	}{
		{name: "nil args"},
		{name: "missing", args: map[string]any{"other": "value"}},
		{name: "nil value", args: map[string]any{"subject": nil}},
		{name: "string", args: map[string]any{"subject": "Hello"}, want: "Hello"},
		{name: "invalid", args: map[string]any{"subject": 7}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := optionalStringArgument(tt.args, "subject")
			if tt.wantErr {
				if !errors.Is(err, domainmcp.ErrInvalidParams) {
					t.Fatalf("expected invalid params error, got %v", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("optionalStringArgument got=%q err=%v want=%q", got, err, tt.want)
			}
		})
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
	if repository.created.PlayerID != 42 || repository.created.Name != "Claude desktop" || repository.created.CreatedAt != 1700000000 || repository.created.ExpiresAt != time.Unix(1700000000, 0).Add(defaultMCPTokenTTL).Unix() {
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
	created, err := service.CreateToken(context.Background(), CreateTokenCommand{Scopes: []string{domainmcp.ScopeFleetWrite, domainmcp.ScopeQueueWrite, domainmcp.ScopeResourcesWrite, domainmcp.ScopePremiumWrite}})
	if err != nil || strings.Join(created.Creation.Token.Scopes, " ") != domainmcp.ScopeFleetWrite+" "+domainmcp.ScopeQueueWrite+" "+domainmcp.ScopeResourcesWrite+" "+domainmcp.ScopePremiumWrite {
		t.Fatalf("expected fleet, queue, resources, and premium write user token scopes to be allowed, created=%+v err=%v", created, err)
	}
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
	if defaulted.tokenGenerator == nil || defaulted.now == nil || defaulted.tokenTTL != defaultMCPTokenTTL {
		t.Fatalf("expected default generator and clock")
	}
	if defaulted.WithTokenTTL(-time.Second).tokenTTL != defaultMCPTokenTTL {
		t.Fatalf("expected negative token ttl to reset to default")
	}
	if defaulted.WithTokenTTL(0).tokenExpiresIn() != 0 {
		t.Fatalf("expected disabled token ttl to report no expires_in")
	}

	unlimitedRepository := &fakeTokenRepository{}
	unlimited := NewServiceWithTokenManagement(fakeHealthProvider{}, nil, unlimitedRepository, fakeSessionLookup{auth: authenticatedSession(42)}, fakeTokenGenerator{secret: "token"}, func() time.Time { return time.Unix(1700, 0) }).WithTokenTTL(0)
	created, err := unlimited.CreateToken(context.Background(), CreateTokenCommand{Name: "Unlimited", Scopes: []string{domainmcp.ScopeRead}})
	if err != nil || created.Creation.Token.ExpiresAt != 0 || unlimitedRepository.created.ExpiresAt != 0 {
		t.Fatalf("expected disabled token ttl to be accepted, got created=%+v err=%v", created, err)
	}

	_, err = (Service{}).ListTokens(context.Background(), TokenManagementCommand{})
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

type fakeOIDCSigner struct {
	token   string
	err     error
	command domainmcp.IDTokenCommand
}

func (f *fakeOIDCSigner) Algorithm() string {
	return "EdDSA"
}

func (f *fakeOIDCSigner) JWKS(context.Context) domainmcp.JSONWebKeySet {
	return domainmcp.JSONWebKeySet{Keys: []domainmcp.JSONWebKey{{KeyType: "OKP", KeyID: "kid", Alg: "EdDSA"}}}
}

func (f *fakeOIDCSigner) SignIDToken(_ context.Context, command domainmcp.IDTokenCommand) (string, error) {
	f.command = command
	if f.err != nil {
		return "", f.err
	}
	return f.token, nil
}

type fakeReadRepository struct {
	planets               []domainmcp.Planet
	overview              domainmcp.AccountOverview
	resources             domainmcp.PlanetResources
	resourcesPlanetID     int
	buildingQueue         domainmcp.BuildingQueue
	buildingQueuePlanetID int
	messageList           domainmcp.MessageList
	messageQuery          domainmcp.MessageQuery
	messageDetail         domainmcp.MessageDetail
	messageID             int
	fleetMovements        domainmcp.FleetMovements
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

func (f fakeReadRepository) ListMCPMessages(_ context.Context, playerID int, query domainmcp.MessageQuery) (domainmcp.MessageList, error) {
	if f.err != nil {
		return domainmcp.MessageList{}, f.err
	}
	if playerID != 42 {
		return domainmcp.MessageList{}, errors.New("unexpected player")
	}
	if query.Limit != f.messageQuery.Limit || query.MessageType != f.messageQuery.MessageType || query.HasMessageType != f.messageQuery.HasMessageType || query.IncludeText != f.messageQuery.IncludeText {
		return domainmcp.MessageList{}, errors.New("unexpected message query")
	}
	return f.messageList, nil
}

func (f fakeReadRepository) GetMCPMessage(_ context.Context, playerID int, messageID int) (domainmcp.MessageDetail, error) {
	if f.err != nil {
		return domainmcp.MessageDetail{}, f.err
	}
	if playerID != 42 {
		return domainmcp.MessageDetail{}, errors.New("unexpected player")
	}
	if messageID != f.messageID {
		return domainmcp.MessageDetail{}, errors.New("unexpected message")
	}
	return f.messageDetail, nil
}

func (f fakeReadRepository) GetMCPFleetMovements(_ context.Context, playerID int) (domainmcp.FleetMovements, error) {
	if f.err != nil {
		return domainmcp.FleetMovements{}, f.err
	}
	if playerID != 42 {
		return domainmcp.FleetMovements{}, errors.New("unexpected player")
	}
	return f.fleetMovements, nil
}

type fakeWriteRepository struct {
	preview               domainmcp.SendMessageResult
	sent                  domainmcp.SendMessageResult
	deletePreview         domainmcp.DeleteMessagesResult
	deleted               domainmcp.DeleteMessagesResult
	reportPreview         domainmcp.ReportMessageResult
	reported              domainmcp.ReportMessageResult
	previewPlayerID       int
	sendPlayerID          int
	deletePreviewPlayerID int
	deletePlayerID        int
	reportPreviewPlayerID int
	reportPlayerID        int
	previewCommand        domainmcp.SendMessageCommand
	sendCommand           domainmcp.SendMessageCommand
	deletePreviewCommand  domainmcp.DeleteMessagesCommand
	deleteCommand         domainmcp.DeleteMessagesCommand
	reportPreviewCommand  domainmcp.ReportMessageCommand
	reportCommand         domainmcp.ReportMessageCommand
	err                   error
}

func (f *fakeWriteRepository) PreviewMCPSendMessage(_ context.Context, playerID int, command domainmcp.SendMessageCommand) (domainmcp.SendMessageResult, error) {
	f.previewPlayerID = playerID
	f.previewCommand = command
	if f.err != nil {
		return domainmcp.SendMessageResult{}, f.err
	}
	return f.preview, nil
}

func (f *fakeWriteRepository) SendMCPMessage(_ context.Context, playerID int, command domainmcp.SendMessageCommand) (domainmcp.SendMessageResult, error) {
	f.sendPlayerID = playerID
	f.sendCommand = command
	if f.err != nil {
		return domainmcp.SendMessageResult{}, f.err
	}
	return f.sent, nil
}

func (f *fakeWriteRepository) PreviewMCPDeleteMessages(_ context.Context, playerID int, command domainmcp.DeleteMessagesCommand) (domainmcp.DeleteMessagesResult, error) {
	f.deletePreviewPlayerID = playerID
	f.deletePreviewCommand = command
	if f.err != nil {
		return domainmcp.DeleteMessagesResult{}, f.err
	}
	return f.deletePreview, nil
}

func (f *fakeWriteRepository) DeleteMCPMessages(_ context.Context, playerID int, command domainmcp.DeleteMessagesCommand) (domainmcp.DeleteMessagesResult, error) {
	f.deletePlayerID = playerID
	f.deleteCommand = command
	if f.err != nil {
		return domainmcp.DeleteMessagesResult{}, f.err
	}
	return f.deleted, nil
}

func (f *fakeWriteRepository) PreviewMCPReportMessage(_ context.Context, playerID int, command domainmcp.ReportMessageCommand) (domainmcp.ReportMessageResult, error) {
	f.reportPreviewPlayerID = playerID
	f.reportPreviewCommand = command
	if f.err != nil {
		return domainmcp.ReportMessageResult{}, f.err
	}
	return f.reportPreview, nil
}

func (f *fakeWriteRepository) ReportMCPMessage(_ context.Context, playerID int, command domainmcp.ReportMessageCommand) (domainmcp.ReportMessageResult, error) {
	f.reportPlayerID = playerID
	f.reportCommand = command
	if f.err != nil {
		return domainmcp.ReportMessageResult{}, f.err
	}
	return f.reported, nil
}

type fakeFleetWriteRepository struct {
	dispatchPreview         domainmcp.DispatchFleetValidationResult
	dispatched              domainmcp.DispatchFleetValidationResult
	preview                 domainmcp.RecallFleetResult
	recalled                domainmcp.RecallFleetResult
	dispatchPreviewPlayerID int
	dispatchPlayerID        int
	previewPlayerID         int
	recallPlayerID          int
	dispatchPreviewCommand  domainmcp.DispatchFleetCommand
	dispatchCommand         domainmcp.DispatchFleetCommand
	previewCommand          domainmcp.RecallFleetCommand
	recallCommand           domainmcp.RecallFleetCommand
	err                     error
}

func (f *fakeFleetWriteRepository) PreviewMCPDispatchFleet(_ context.Context, playerID int, command domainmcp.DispatchFleetCommand) (domainmcp.DispatchFleetValidationResult, error) {
	f.dispatchPreviewPlayerID = playerID
	f.dispatchPreviewCommand = command
	if f.err != nil {
		return domainmcp.DispatchFleetValidationResult{}, f.err
	}
	return f.dispatchPreview, nil
}

func (f *fakeFleetWriteRepository) DispatchMCPFleet(_ context.Context, playerID int, command domainmcp.DispatchFleetCommand) (domainmcp.DispatchFleetValidationResult, error) {
	f.dispatchPlayerID = playerID
	f.dispatchCommand = command
	if f.err != nil {
		return domainmcp.DispatchFleetValidationResult{}, f.err
	}
	return f.dispatched, nil
}

func (f *fakeFleetWriteRepository) PreviewMCPRecallFleet(_ context.Context, playerID int, command domainmcp.RecallFleetCommand) (domainmcp.RecallFleetResult, error) {
	f.previewPlayerID = playerID
	f.previewCommand = command
	if f.err != nil {
		return domainmcp.RecallFleetResult{}, f.err
	}
	return f.preview, nil
}

func (f *fakeFleetWriteRepository) RecallMCPFleet(_ context.Context, playerID int, command domainmcp.RecallFleetCommand) (domainmcp.RecallFleetResult, error) {
	f.recallPlayerID = playerID
	f.recallCommand = command
	if f.err != nil {
		return domainmcp.RecallFleetResult{}, f.err
	}
	return f.recalled, nil
}

type fakeQueueWriteRepository struct {
	preview                 domainmcp.CancelBuildingQueueResult
	canceled                domainmcp.CancelBuildingQueueResult
	researchPreview         domainmcp.CancelResearchQueueResult
	researchCanceled        domainmcp.CancelResearchQueueResult
	shipyardPreview         domainmcp.EnqueueShipyardOrderResult
	shipyardEnqueued        domainmcp.EnqueueShipyardOrderResult
	previewPlayerID         int
	cancelPlayerID          int
	researchPreviewPlayerID int
	researchCancelPlayerID  int
	shipyardPreviewPlayerID int
	shipyardEnqueuePlayerID int
	previewCommand          domainmcp.CancelBuildingQueueCommand
	cancelCommand           domainmcp.CancelBuildingQueueCommand
	researchPreviewCommand  domainmcp.CancelResearchQueueCommand
	researchCancelCommand   domainmcp.CancelResearchQueueCommand
	shipyardPreviewCommand  domainmcp.EnqueueShipyardOrderCommand
	shipyardEnqueueCommand  domainmcp.EnqueueShipyardOrderCommand
	err                     error
}

func (f *fakeQueueWriteRepository) PreviewMCPCancelBuildingQueue(_ context.Context, playerID int, command domainmcp.CancelBuildingQueueCommand) (domainmcp.CancelBuildingQueueResult, error) {
	f.previewPlayerID = playerID
	f.previewCommand = command
	if f.err != nil {
		return domainmcp.CancelBuildingQueueResult{}, f.err
	}
	return f.preview, nil
}

func (f *fakeQueueWriteRepository) CancelMCPBuildingQueue(_ context.Context, playerID int, command domainmcp.CancelBuildingQueueCommand) (domainmcp.CancelBuildingQueueResult, error) {
	f.cancelPlayerID = playerID
	f.cancelCommand = command
	if f.err != nil {
		return domainmcp.CancelBuildingQueueResult{}, f.err
	}
	return f.canceled, nil
}

func (f *fakeQueueWriteRepository) PreviewMCPCancelResearchQueue(_ context.Context, playerID int, command domainmcp.CancelResearchQueueCommand) (domainmcp.CancelResearchQueueResult, error) {
	f.researchPreviewPlayerID = playerID
	f.researchPreviewCommand = command
	if f.err != nil {
		return domainmcp.CancelResearchQueueResult{}, f.err
	}
	return f.researchPreview, nil
}

func (f *fakeQueueWriteRepository) CancelMCPResearchQueue(_ context.Context, playerID int, command domainmcp.CancelResearchQueueCommand) (domainmcp.CancelResearchQueueResult, error) {
	f.researchCancelPlayerID = playerID
	f.researchCancelCommand = command
	if f.err != nil {
		return domainmcp.CancelResearchQueueResult{}, f.err
	}
	return f.researchCanceled, nil
}

func (f *fakeQueueWriteRepository) PreviewMCPEnqueueShipyardOrder(_ context.Context, playerID int, command domainmcp.EnqueueShipyardOrderCommand) (domainmcp.EnqueueShipyardOrderResult, error) {
	f.shipyardPreviewPlayerID = playerID
	f.shipyardPreviewCommand = command
	if f.err != nil {
		return domainmcp.EnqueueShipyardOrderResult{}, f.err
	}
	return f.shipyardPreview, nil
}

func (f *fakeQueueWriteRepository) EnqueueMCPShipyardOrder(_ context.Context, playerID int, command domainmcp.EnqueueShipyardOrderCommand) (domainmcp.EnqueueShipyardOrderResult, error) {
	f.shipyardEnqueuePlayerID = playerID
	f.shipyardEnqueueCommand = command
	if f.err != nil {
		return domainmcp.EnqueueShipyardOrderResult{}, f.err
	}
	return f.shipyardEnqueued, nil
}

type fakeResourceWriteRepository struct {
	preview         domainmcp.UpdateResourceProductionResult
	updated         domainmcp.UpdateResourceProductionResult
	previewPlayerID int
	updatePlayerID  int
	previewCommand  domainmcp.UpdateResourceProductionCommand
	updateCommand   domainmcp.UpdateResourceProductionCommand
	err             error
}

func (f *fakeResourceWriteRepository) PreviewMCPUpdateResourceProduction(_ context.Context, playerID int, command domainmcp.UpdateResourceProductionCommand) (domainmcp.UpdateResourceProductionResult, error) {
	f.previewPlayerID = playerID
	f.previewCommand = command
	if f.err != nil {
		return domainmcp.UpdateResourceProductionResult{}, f.err
	}
	return f.preview, nil
}

func (f *fakeResourceWriteRepository) UpdateMCPResourceProduction(_ context.Context, playerID int, command domainmcp.UpdateResourceProductionCommand) (domainmcp.UpdateResourceProductionResult, error) {
	f.updatePlayerID = playerID
	f.updateCommand = command
	if f.err != nil {
		return domainmcp.UpdateResourceProductionResult{}, f.err
	}
	return f.updated, nil
}

type fakePremiumWriteRepository struct {
	preview         domainmcp.RecruitOfficerResult
	recruited       domainmcp.RecruitOfficerResult
	status          domainmcp.OfficerStatus
	previewPlayerID int
	recruitPlayerID int
	statusPlayerID  int
	statusPlanetID  int
	previewCommand  domainmcp.RecruitOfficerCommand
	recruitCommand  domainmcp.RecruitOfficerCommand
	err             error
}

func (f *fakePremiumWriteRepository) GetMCPOfficerStatus(_ context.Context, playerID int, planetID int) (domainmcp.OfficerStatus, error) {
	f.statusPlayerID = playerID
	f.statusPlanetID = planetID
	if f.err != nil {
		return domainmcp.OfficerStatus{}, f.err
	}
	return f.status, nil
}

func (f *fakePremiumWriteRepository) PreviewMCPRecruitOfficer(_ context.Context, playerID int, command domainmcp.RecruitOfficerCommand) (domainmcp.RecruitOfficerResult, error) {
	f.previewPlayerID = playerID
	f.previewCommand = command
	if f.err != nil {
		return domainmcp.RecruitOfficerResult{}, f.err
	}
	return f.preview, nil
}

func (f *fakePremiumWriteRepository) RecruitMCPOfficer(_ context.Context, playerID int, command domainmcp.RecruitOfficerCommand) (domainmcp.RecruitOfficerResult, error) {
	f.recruitPlayerID = playerID
	f.recruitCommand = command
	if f.err != nil {
		return domainmcp.RecruitOfficerResult{}, f.err
	}
	return f.recruited, nil
}

type fakeSearchReadRepository struct {
	result   domainmcp.SearchResult
	playerID int
	command  domainmcp.SearchCommand
	err      error
}

func (f *fakeSearchReadRepository) SearchMCP(_ context.Context, playerID int, command domainmcp.SearchCommand) (domainmcp.SearchResult, error) {
	f.playerID = playerID
	f.command = command
	if f.err != nil {
		return domainmcp.SearchResult{}, f.err
	}
	return f.result, nil
}

type fakeGalaxyReadRepository struct {
	result   domainmcp.GalaxySystem
	playerID int
	command  domainmcp.GalaxySystemCommand
	err      error
}

func (f *fakeGalaxyReadRepository) GetMCPGalaxySystem(_ context.Context, playerID int, command domainmcp.GalaxySystemCommand) (domainmcp.GalaxySystem, error) {
	f.playerID = playerID
	f.command = command
	if f.err != nil {
		return domainmcp.GalaxySystem{}, f.err
	}
	return f.result, nil
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
	code   string
	err    error
}

func (f fakeTokenGenerator) NewMCPToken() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.secret, nil
}

func (f fakeTokenGenerator) NewMCPOAuthCode() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if f.code != "" {
		return f.code, nil
	}
	return f.secret, nil
}

type fakeTokenRepository struct {
	created            domainmcp.Token
	hash               string
	revokedHash        string
	revokedAt          int64
	oauthCode          domainmcp.OAuthAuthorizationCode
	listErr            error
	createErr          error
	revokeErr          error
	revokeByHashResult bool
	oauthCreateErr     error
	oauthConsumeErr    error
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

func (f *fakeTokenRepository) RevokeMCPTokenByHash(_ context.Context, tokenHash string, revokedAt int64) (bool, error) {
	if f.revokeErr != nil {
		return false, f.revokeErr
	}
	f.revokedHash = tokenHash
	f.revokedAt = revokedAt
	if f.revokeByHashResult {
		return true, nil
	}
	return tokenHash == HashToken("ogmcp_access"), nil
}

func (f *fakeTokenRepository) CreateMCPOAuthCode(_ context.Context, code domainmcp.OAuthAuthorizationCode) (domainmcp.OAuthAuthorizationCode, error) {
	if f.oauthCreateErr != nil {
		return domainmcp.OAuthAuthorizationCode{}, f.oauthCreateErr
	}
	code.ID = 9
	f.oauthCode = code
	return code, nil
}

func (f *fakeTokenRepository) ConsumeMCPOAuthCode(_ context.Context, codeHash string, now int64) (domainmcp.OAuthAuthorizationCode, error) {
	if f.oauthConsumeErr != nil {
		return domainmcp.OAuthAuthorizationCode{}, f.oauthConsumeErr
	}
	if f.oauthCode.CodeHash != codeHash || f.oauthCode.ConsumedAt != 0 || f.oauthCode.ExpiresAt < now {
		return domainmcp.OAuthAuthorizationCode{}, domainmcp.ErrUnauthorized
	}
	f.oauthCode.ConsumedAt = now
	return f.oauthCode, nil
}

func testPKCEChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func validStoredOAuthCode(verifier string, clientID string) domainmcp.OAuthAuthorizationCode {
	return domainmcp.OAuthAuthorizationCode{
		PlayerID:            42,
		ClientID:            clientID,
		RedirectURI:         "http://127.0.0.1:9911/callback",
		Resource:            "https://game.example/mcp",
		Scopes:              []string{domainmcp.ScopeRead},
		CodeHash:            HashToken("code"),
		CodeChallenge:       testPKCEChallenge(verifier),
		CodeChallengeMethod: "S256",
		ExpiresAt:           2000,
	}
}

func oauthExchangeService(repository *fakeTokenRepository, generator fakeTokenGenerator) Service {
	return NewServiceWithTokenManagement(
		fakeHealthProvider{},
		nil,
		repository,
		fakeSessionLookup{auth: authenticatedSession(42)},
		generator,
		func() time.Time { return time.Unix(1700, 0) },
	).WithOAuthCodeRepository(repository)
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
