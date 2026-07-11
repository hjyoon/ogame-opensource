package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
	domainsystem "github.com/hjyoon/ogame-opensource/backend/internal/domain/system"
)

var ErrInvalidTokenRequest = errors.New("invalid mcp token request")
var ErrInvalidOAuthRequest = errors.New("invalid mcp oauth request")
var ErrInvalidOAuthGrant = errors.New("invalid mcp oauth grant")

const (
	defaultMCPTokenTTL = 30 * 24 * time.Hour
	mcpOAuthCodeTTL    = 10 * time.Minute
)

type HealthProvider interface {
	Get(context.Context) domainsystem.Health
}

type TokenVerifier interface {
	VerifyMCPToken(context.Context, string) (domainmcp.Access, error)
}

type ToolCallAuditor interface {
	RecordMCPToolCall(context.Context, domainmcp.ToolCallAudit)
}

type OIDCSigner interface {
	Algorithm() string
	JWKS(context.Context) domainmcp.JSONWebKeySet
	SignIDToken(context.Context, domainmcp.IDTokenCommand) (string, error)
}

type SessionLookup interface {
	GetGameSession(context.Context, apppublicsite.GameSessionCommand) (domainpublicsite.SessionAuthentication, error)
}

type TokenRepository interface {
	ListMCPTokens(context.Context, int) ([]domainmcp.Token, error)
	CreateMCPToken(context.Context, domainmcp.Token, string) (domainmcp.Token, error)
	RevokeMCPToken(context.Context, int, int, int64) (bool, error)
	RevokeMCPTokenByHash(context.Context, string, int64) (bool, error)
}

type OAuthCodeRepository interface {
	CreateMCPOAuthCode(context.Context, domainmcp.OAuthAuthorizationCode) (domainmcp.OAuthAuthorizationCode, error)
	ConsumeMCPOAuthCode(context.Context, string, int64) (domainmcp.OAuthAuthorizationCode, error)
}

type ReadRepository interface {
	ListMCPPlanets(context.Context, int) ([]domainmcp.Planet, error)
	GetMCPAccountOverview(context.Context, int) (domainmcp.AccountOverview, error)
	GetMCPPlanetResources(context.Context, int, int) (domainmcp.PlanetResources, error)
	GetMCPBuildingQueue(context.Context, int, int) (domainmcp.BuildingQueue, error)
	ListMCPMessages(context.Context, int, domainmcp.MessageQuery) (domainmcp.MessageList, error)
	GetMCPMessage(context.Context, int, int) (domainmcp.MessageDetail, error)
	GetMCPFleetMovements(context.Context, int) (domainmcp.FleetMovements, error)
}

type WriteRepository interface {
	PreviewMCPSendMessage(context.Context, int, domainmcp.SendMessageCommand) (domainmcp.SendMessageResult, error)
	SendMCPMessage(context.Context, int, domainmcp.SendMessageCommand) (domainmcp.SendMessageResult, error)
	PreviewMCPDeleteMessages(context.Context, int, domainmcp.DeleteMessagesCommand) (domainmcp.DeleteMessagesResult, error)
	DeleteMCPMessages(context.Context, int, domainmcp.DeleteMessagesCommand) (domainmcp.DeleteMessagesResult, error)
	PreviewMCPReportMessage(context.Context, int, domainmcp.ReportMessageCommand) (domainmcp.ReportMessageResult, error)
	ReportMCPMessage(context.Context, int, domainmcp.ReportMessageCommand) (domainmcp.ReportMessageResult, error)
}

type FleetWriteRepository interface {
	PreviewMCPDispatchFleet(context.Context, int, domainmcp.DispatchFleetCommand) (domainmcp.DispatchFleetValidationResult, error)
	DispatchMCPFleet(context.Context, int, domainmcp.DispatchFleetCommand) (domainmcp.DispatchFleetValidationResult, error)
	PreviewMCPRecallFleet(context.Context, int, domainmcp.RecallFleetCommand) (domainmcp.RecallFleetResult, error)
	RecallMCPFleet(context.Context, int, domainmcp.RecallFleetCommand) (domainmcp.RecallFleetResult, error)
}

type QueueWriteRepository interface {
	PreviewMCPCancelBuildingQueue(context.Context, int, domainmcp.CancelBuildingQueueCommand) (domainmcp.CancelBuildingQueueResult, error)
	CancelMCPBuildingQueue(context.Context, int, domainmcp.CancelBuildingQueueCommand) (domainmcp.CancelBuildingQueueResult, error)
	PreviewMCPCancelResearchQueue(context.Context, int, domainmcp.CancelResearchQueueCommand) (domainmcp.CancelResearchQueueResult, error)
	CancelMCPResearchQueue(context.Context, int, domainmcp.CancelResearchQueueCommand) (domainmcp.CancelResearchQueueResult, error)
	PreviewMCPEnqueueShipyardOrder(context.Context, int, domainmcp.EnqueueShipyardOrderCommand) (domainmcp.EnqueueShipyardOrderResult, error)
	EnqueueMCPShipyardOrder(context.Context, int, domainmcp.EnqueueShipyardOrderCommand) (domainmcp.EnqueueShipyardOrderResult, error)
}

type ResourceWriteRepository interface {
	PreviewMCPUpdateResourceProduction(context.Context, int, domainmcp.UpdateResourceProductionCommand) (domainmcp.UpdateResourceProductionResult, error)
	UpdateMCPResourceProduction(context.Context, int, domainmcp.UpdateResourceProductionCommand) (domainmcp.UpdateResourceProductionResult, error)
}

type PremiumWriteRepository interface {
	PreviewMCPRecruitOfficer(context.Context, int, domainmcp.RecruitOfficerCommand) (domainmcp.RecruitOfficerResult, error)
	RecruitMCPOfficer(context.Context, int, domainmcp.RecruitOfficerCommand) (domainmcp.RecruitOfficerResult, error)
}

type TokenSecretGenerator interface {
	NewMCPToken() (string, error)
}

type OAuthCodeGenerator interface {
	NewMCPOAuthCode() (string, error)
}

type SecureTokenGenerator struct{}

func (SecureTokenGenerator) NewMCPToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "ogmcp_" + hex.EncodeToString(bytes), nil
}

func (SecureTokenGenerator) NewMCPOAuthCode() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "ogmcp_code_" + hex.EncodeToString(bytes), nil
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

type OAuthAuthorizeCommand struct {
	TokenManagementCommand
	ResponseType        string
	ClientID            string
	RedirectURI         string
	Resource            string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	ConsentApproved     bool
	Issuer              string
}

type OAuthAuthorizeResult struct {
	Authenticated   bool
	Issues          []domainpublicsite.SessionIssue
	RequiresConsent bool
	ClientID        string
	RedirectURI     string
	Scopes          []string
	RedirectTo      string
	Code            string
}

type OAuthClientRegistrationCommand struct {
	RedirectURIs            []string
	ClientName              string
	GrantTypes              []string
	ResponseTypes           []string
	TokenEndpointAuthMethod string
	Scope                   string
}

type OAuthClientRegistrationResult struct {
	ClientID                string
	ClientIDIssuedAt        int64
	ClientName              string
	RedirectURIs            []string
	GrantTypes              []string
	ResponseTypes           []string
	TokenEndpointAuthMethod string
	Scope                   string
}

type OAuthTokenCommand struct {
	GrantType    string
	Code         string
	RedirectURI  string
	Resource     string
	ClientID     string
	CodeVerifier string
	Issuer       string
}

type OAuthRevokeCommand struct {
	Token         string
	TokenTypeHint string
}

type OAuthTokenResult struct {
	AccessToken string
	TokenType   string
	Scope       string
	IDToken     string
	ExpiresIn   int64
	Token       domainmcp.Token
}

type OAuthRevokeResult struct {
	Revoked bool
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
	oauthRepository OAuthCodeRepository
	oauthRedirects  []string
	oidcSigner      OIDCSigner
	readRepository  ReadRepository
	writeRepository WriteRepository
	fleetWrite      FleetWriteRepository
	queueWrite      QueueWriteRepository
	resourceWrite   ResourceWriteRepository
	premiumWrite    PremiumWriteRepository
	sessions        SessionLookup
	tokenGenerator  TokenSecretGenerator
	codeGenerator   OAuthCodeGenerator
	auditor         ToolCallAuditor
	tokenTTL        time.Duration
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
	codeGenerator, ok := generator.(OAuthCodeGenerator)
	if !ok {
		codeGenerator = SecureTokenGenerator{}
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
		codeGenerator:   codeGenerator,
		tokenTTL:        defaultMCPTokenTTL,
		now:             now,
	}
}

func (s Service) WithOAuthCodeRepository(repository OAuthCodeRepository) Service {
	s.oauthRepository = repository
	return s
}

func (s Service) WithOAuthRedirectURIs(redirectURIs []string) Service {
	s.oauthRedirects = normalizeAllowedOAuthRedirectURIs(redirectURIs)
	return s
}

func (s Service) WithOIDCSigner(signer OIDCSigner) Service {
	s.oidcSigner = signer
	return s
}

func (s Service) WithReadRepository(repository ReadRepository) Service {
	s.readRepository = repository
	return s
}

func (s Service) WithWriteRepository(repository WriteRepository) Service {
	s.writeRepository = repository
	return s
}

func (s Service) WithFleetWriteRepository(repository FleetWriteRepository) Service {
	s.fleetWrite = repository
	return s
}

func (s Service) WithQueueWriteRepository(repository QueueWriteRepository) Service {
	s.queueWrite = repository
	return s
}

func (s Service) WithResourceWriteRepository(repository ResourceWriteRepository) Service {
	s.resourceWrite = repository
	return s
}

func (s Service) WithPremiumWriteRepository(repository PremiumWriteRepository) Service {
	s.premiumWrite = repository
	return s
}

func (s Service) WithToolCallAuditor(auditor ToolCallAuditor) Service {
	s.auditor = auditor
	return s
}

func (s Service) WithTokenTTL(ttl time.Duration) Service {
	if ttl < 0 {
		ttl = defaultMCPTokenTTL
	}
	s.tokenTTL = ttl
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
	metadata := domainmcp.OAuthAuthorizationServerMetadata{
		Issuer:                            issuer,
		AuthorizationEndpoint:             issuer + "/oauth/authorize",
		TokenEndpoint:                     issuer + "/oauth/token",
		RevocationEndpoint:                issuer + "/oauth/revoke",
		RegistrationEndpoint:              issuer + "/oauth/register",
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code"},
		CodeChallengeMethodsSupported:     []string{"S256"},
		TokenEndpointAuthMethodsSupported: []string{"none"},
		RevocationEndpointAuthMethods:     []string{"none"},
		ScopesSupported: []string{
			"openid",
			"profile",
			domainmcp.ScopeRead,
			domainmcp.ScopeMessages,
			domainmcp.ScopeMessageWrite,
			domainmcp.ScopeFleet,
			domainmcp.ScopeFleetWrite,
			domainmcp.ScopeQueueWrite,
			domainmcp.ScopeResourcesWrite,
			domainmcp.ScopePremiumWrite,
		},
	}
	if s.oidcSigner != nil {
		metadata.JWKSURI = issuer + "/.well-known/jwks.json"
		metadata.IDTokenSigningAlgValuesSupported = []string{s.oidcSigner.Algorithm()}
	}
	return metadata
}

func (s Service) OAuthProtectedResourceMetadata(ctx context.Context, resource string, issuer string) domainmcp.OAuthProtectedResourceMetadata {
	_ = ctx
	return domainmcp.OAuthProtectedResourceMetadata{
		Resource:             strings.TrimRight(strings.TrimSpace(resource), "/"),
		AuthorizationServers: []string{strings.TrimRight(strings.TrimSpace(issuer), "/")},
		ScopesSupported: []string{
			domainmcp.ScopeRead,
			domainmcp.ScopeMessages,
			domainmcp.ScopeMessageWrite,
			domainmcp.ScopeFleet,
			domainmcp.ScopeFleetWrite,
			domainmcp.ScopeQueueWrite,
			domainmcp.ScopeResourcesWrite,
			domainmcp.ScopePremiumWrite,
		},
		BearerMethods: []string{"header"},
	}
}

func (s Service) RegisterOAuthClient(ctx context.Context, command OAuthClientRegistrationCommand) (OAuthClientRegistrationResult, error) {
	_ = ctx
	redirectURIs, err := normalizeOAuthClientRedirectURIs(command.RedirectURIs, s.oauthRedirects)
	if err != nil {
		return OAuthClientRegistrationResult{}, err
	}
	grantTypes, err := normalizeOAuthClientValues(command.GrantTypes, []string{"authorization_code"}, "grant_types")
	if err != nil {
		return OAuthClientRegistrationResult{}, err
	}
	responseTypes, err := normalizeOAuthClientValues(command.ResponseTypes, []string{"code"}, "response_types")
	if err != nil {
		return OAuthClientRegistrationResult{}, err
	}
	authMethod := strings.TrimSpace(command.TokenEndpointAuthMethod)
	if authMethod == "" {
		authMethod = "none"
	}
	if authMethod != "none" {
		return OAuthClientRegistrationResult{}, fmt.Errorf("%w: token_endpoint_auth_method must be none", ErrInvalidOAuthRequest)
	}
	scopes, err := normalizeOAuthScopes(command.Scope)
	if err != nil {
		return OAuthClientRegistrationResult{}, err
	}
	generator := s.tokenGenerator
	if generator == nil {
		generator = SecureTokenGenerator{}
	}
	secret, err := generator.NewMCPToken()
	if err != nil {
		return OAuthClientRegistrationResult{}, err
	}
	clientID := "ogmcp_client_" + strings.TrimPrefix(secret, "ogmcp_")
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	return OAuthClientRegistrationResult{
		ClientID:                clientID,
		ClientIDIssuedAt:        now().Unix(),
		ClientName:              truncateOAuthTokenName(strings.TrimSpace(command.ClientName), 64),
		RedirectURIs:            redirectURIs,
		GrantTypes:              grantTypes,
		ResponseTypes:           responseTypes,
		TokenEndpointAuthMethod: authMethod,
		Scope:                   strings.Join(scopes, " "),
	}, nil
}

func (s Service) OAuthJWKS(ctx context.Context) domainmcp.JSONWebKeySet {
	if s.oidcSigner == nil {
		return domainmcp.JSONWebKeySet{}
	}
	return s.oidcSigner.JWKS(ctx)
}

func (s Service) AuthorizeOAuth(ctx context.Context, command OAuthAuthorizeCommand) (OAuthAuthorizeResult, error) {
	if s.sessions == nil || s.oauthRepository == nil || s.codeGenerator == nil {
		return OAuthAuthorizeResult{}, errors.New("mcp oauth dependencies unavailable")
	}
	request, err := normalizeOAuthAuthorizeRequest(command, s.oauthRedirects)
	if err != nil {
		return OAuthAuthorizeResult{}, err
	}
	session, err := s.authenticateSession(ctx, command.TokenManagementCommand)
	if err != nil {
		return OAuthAuthorizeResult{}, err
	}
	if !session.Authenticated {
		return OAuthAuthorizeResult{Authenticated: false, Issues: session.Issues}, nil
	}
	if !command.ConsentApproved {
		return OAuthAuthorizeResult{
			Authenticated:   true,
			RequiresConsent: true,
			ClientID:        request.ClientID,
			RedirectURI:     request.RedirectURI,
			Scopes:          request.Scopes,
		}, nil
	}
	code, err := s.codeGenerator.NewMCPOAuthCode()
	if err != nil {
		return OAuthAuthorizeResult{}, err
	}
	now := s.now()
	_, err = s.oauthRepository.CreateMCPOAuthCode(ctx, domainmcp.OAuthAuthorizationCode{
		PlayerID:            session.Session.PlayerID,
		ClientID:            request.ClientID,
		RedirectURI:         request.RedirectURI,
		Resource:            request.Resource,
		Scopes:              request.Scopes,
		CodeHash:            HashToken(code),
		CodeChallenge:       request.CodeChallenge,
		CodeChallengeMethod: request.CodeChallengeMethod,
		CreatedAt:           now.Unix(),
		ExpiresAt:           now.Add(mcpOAuthCodeTTL).Unix(),
	})
	if err != nil {
		return OAuthAuthorizeResult{}, err
	}
	redirectTo, err := oauthRedirectURI(request.RedirectURI, code, request.State)
	if err != nil {
		return OAuthAuthorizeResult{}, err
	}
	return OAuthAuthorizeResult{
		Authenticated: true,
		ClientID:      request.ClientID,
		RedirectURI:   request.RedirectURI,
		Scopes:        request.Scopes,
		RedirectTo:    redirectTo,
		Code:          code,
	}, nil
}

func (s Service) ExchangeOAuthCode(ctx context.Context, command OAuthTokenCommand) (OAuthTokenResult, error) {
	if s.oauthRepository == nil || s.tokenRepository == nil || s.tokenGenerator == nil {
		return OAuthTokenResult{}, errors.New("mcp oauth dependencies unavailable")
	}
	if strings.TrimSpace(command.GrantType) != "authorization_code" {
		return OAuthTokenResult{}, fmt.Errorf("%w: grant_type must be authorization_code", ErrInvalidOAuthRequest)
	}
	code := strings.TrimSpace(command.Code)
	if code == "" {
		return OAuthTokenResult{}, fmt.Errorf("%w: code is required", ErrInvalidOAuthRequest)
	}
	redirectURI := strings.TrimSpace(command.RedirectURI)
	if !validOAuthRedirectURI(redirectURI, s.oauthRedirects) {
		return OAuthTokenResult{}, fmt.Errorf("%w: redirect_uri is invalid", ErrInvalidOAuthRequest)
	}
	if !validOAuthResource(command.Resource, command.Issuer) {
		return OAuthTokenResult{}, fmt.Errorf("%w: resource is invalid", ErrInvalidOAuthRequest)
	}
	clientID := strings.TrimSpace(command.ClientID)
	if !validOAuthClientID(clientID) {
		return OAuthTokenResult{}, fmt.Errorf("%w: client_id is required", ErrInvalidOAuthRequest)
	}
	verifier := strings.TrimSpace(command.CodeVerifier)
	if !validPKCEValue(verifier) {
		return OAuthTokenResult{}, fmt.Errorf("%w: code_verifier is invalid", ErrInvalidOAuthRequest)
	}
	now := s.now().Unix()
	stored, err := s.oauthRepository.ConsumeMCPOAuthCode(ctx, HashToken(code), now)
	if err != nil {
		if errors.Is(err, domainmcp.ErrUnauthorized) {
			return OAuthTokenResult{}, ErrInvalidOAuthGrant
		}
		return OAuthTokenResult{}, err
	}
	resource := strings.TrimRight(strings.TrimSpace(command.Resource), "/")
	if stored.PlayerID <= 0 || stored.ClientID != clientID || stored.RedirectURI != redirectURI || stored.Resource != resource || stored.CodeChallengeMethod != "S256" || !pkceS256Matches(verifier, stored.CodeChallenge) {
		return OAuthTokenResult{}, ErrInvalidOAuthGrant
	}
	scopes, err := normalizeOAuthScopes(strings.Join(stored.Scopes, " "))
	if err != nil {
		return OAuthTokenResult{}, ErrInvalidOAuthGrant
	}
	secret, err := s.tokenGenerator.NewMCPToken()
	if err != nil {
		return OAuthTokenResult{}, err
	}
	token, err := s.tokenRepository.CreateMCPToken(ctx, domainmcp.Token{
		PlayerID:  stored.PlayerID,
		Name:      truncateOAuthTokenName("OAuth "+clientID, 64),
		Scopes:    scopes,
		CreatedAt: now,
		ExpiresAt: s.tokenExpiresAt(time.Unix(now, 0)),
	}, HashToken(secret))
	if err != nil {
		return OAuthTokenResult{}, err
	}
	token.PlayerID = 0
	idToken := ""
	if scopeAllowed(scopes, "openid") && s.oidcSigner != nil {
		idToken, err = s.oidcSigner.SignIDToken(ctx, domainmcp.IDTokenCommand{
			Issuer:   command.Issuer,
			Audience: clientID,
			PlayerID: stored.PlayerID,
			Scopes:   scopes,
		})
		if err != nil {
			return OAuthTokenResult{}, err
		}
	}
	return OAuthTokenResult{
		AccessToken: secret,
		TokenType:   "Bearer",
		Scope:       strings.Join(scopes, " "),
		IDToken:     idToken,
		ExpiresIn:   s.tokenExpiresIn(),
		Token:       token,
	}, nil
}

func (s Service) RevokeOAuthToken(ctx context.Context, command OAuthRevokeCommand) (OAuthRevokeResult, error) {
	if s.tokenRepository == nil {
		return OAuthRevokeResult{}, errors.New("mcp oauth token revocation dependencies unavailable")
	}
	token := strings.TrimSpace(command.Token)
	if token == "" {
		return OAuthRevokeResult{}, fmt.Errorf("%w: token is required", ErrInvalidOAuthRequest)
	}
	revoked, err := s.tokenRepository.RevokeMCPTokenByHash(ctx, HashToken(token), s.now().Unix())
	if err != nil {
		return OAuthRevokeResult{}, err
	}
	return OAuthRevokeResult{Revoked: revoked}, nil
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
	if access.HasScope(domainmcp.ScopeMessages) && s.readRepository != nil {
		tools = append(tools, listMessagesTool(), getMessageTool())
	}
	if access.HasScope(domainmcp.ScopeMessageWrite) && s.writeRepository != nil {
		tools = append(tools, sendMessageTool(), deleteMessagesTool(), reportMessageTool())
	}
	if access.HasScope(domainmcp.ScopeFleetWrite) && s.fleetWrite != nil {
		tools = append(tools, validateFleetDispatchTool(), dispatchFleetTool(), recallFleetTool())
	}
	if access.HasScope(domainmcp.ScopeQueueWrite) && s.queueWrite != nil {
		tools = append(tools, cancelBuildingQueueTool(), cancelResearchQueueTool(), enqueueShipyardOrderTool())
	}
	if access.HasScope(domainmcp.ScopeResourcesWrite) && s.resourceWrite != nil {
		tools = append(tools, updateResourceProductionTool())
	}
	if access.HasScope(domainmcp.ScopePremiumWrite) && s.premiumWrite != nil {
		tools = append(tools, recruitOfficerTool())
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
	case "list_messages":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeMessages)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callListMessages(ctx, access, command.Arguments)
	case "get_message":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeMessages)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callGetMessage(ctx, access, command.Arguments)
	case "send_message":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeMessageWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callSendMessage(ctx, access, command.Arguments)
	case "delete_messages":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeMessageWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callDeleteMessages(ctx, access, command.Arguments)
	case "report_message":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeMessageWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callReportMessage(ctx, access, command.Arguments)
	case "recall_fleet":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeFleetWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callRecallFleet(ctx, access, command.Arguments)
	case "validate_fleet_dispatch":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeFleetWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callValidateFleetDispatch(ctx, access, command.Arguments)
	case "dispatch_fleet":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeFleetWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callDispatchFleet(ctx, access, command.Arguments)
	case "cancel_building_queue":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeQueueWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callCancelBuildingQueue(ctx, access, command.Arguments)
	case "cancel_research_queue":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeQueueWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callCancelResearchQueue(ctx, access, command.Arguments)
	case "enqueue_shipyard_order":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeQueueWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callEnqueueShipyardOrder(ctx, access, command.Arguments)
	case "update_resource_production":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeResourcesWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callUpdateResourceProduction(ctx, access, command.Arguments)
	case "recruit_officer":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopePremiumWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callRecruitOfficer(ctx, access, command.Arguments)
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
		ExpiresAt: s.tokenExpiresAt(time.Unix(now, 0)),
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

func (s Service) tokenExpiresAt(now time.Time) int64 {
	if s.tokenTTL <= 0 {
		return 0
	}
	return now.Add(s.tokenTTL).Unix()
}

func (s Service) tokenExpiresIn() int64 {
	if s.tokenTTL <= 0 {
		return 0
	}
	return int64(s.tokenTTL.Seconds())
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

func (s Service) callListMessages(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.readRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp read repository unavailable")
	}
	query, err := mcpMessageQuery(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	messages, err := s.readRepository.ListMCPMessages(ctx, access.PlayerID, query)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"messages": messages}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callGetMessage(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.readRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp read repository unavailable")
	}
	messageID, err := optionalNonNegativeIntArgument(arguments, "messageId")
	if err != nil || messageID <= 0 {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	message, err := s.readRepository.GetMCPMessage(ctx, access.PlayerID, messageID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"message": message}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callSendMessage(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.writeRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp write repository unavailable")
	}
	command, err := mcpSendMessageCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpSendMessageConfirmation(command)
	var result domainmcp.SendMessageResult
	if command.DryRun {
		result, err = s.writeRepository.PreviewMCPSendMessage(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Issue == nil {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.writeRepository.SendMCPMessage(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"sendMessage": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callDeleteMessages(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.writeRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp write repository unavailable")
	}
	command, err := mcpDeleteMessagesCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpDeleteMessagesConfirmation(command)
	var result domainmcp.DeleteMessagesResult
	if command.DryRun {
		result, err = s.writeRepository.PreviewMCPDeleteMessages(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.DeleteCount > 0 {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.writeRepository.DeleteMCPMessages(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"deleteMessages": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callReportMessage(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.writeRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp write repository unavailable")
	}
	command, err := mcpReportMessageCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpReportMessageConfirmation(command)
	var result domainmcp.ReportMessageResult
	if command.DryRun {
		result, err = s.writeRepository.PreviewMCPReportMessage(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Reportable && result.Issue == nil {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.writeRepository.ReportMCPMessage(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"reportMessage": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callValidateFleetDispatch(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.fleetWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp fleet write repository unavailable")
	}
	command, err := mcpDispatchFleetCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result, err := s.fleetWrite.PreviewMCPDispatchFleet(ctx, access.PlayerID, command)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result.DryRun = true
	if result.Ready && result.Issue == nil {
		result.RequiresConfirmation = true
		result.Confirmation = mcpDispatchFleetConfirmation(command)
	}
	structured := map[string]any{"fleetDispatchValidation": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callDispatchFleet(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.fleetWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp fleet write repository unavailable")
	}
	command, err := mcpDispatchFleetCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpDispatchFleetConfirmation(command)
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	if strings.TrimSpace(confirm) != confirmation {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	result, err := s.fleetWrite.DispatchMCPFleet(ctx, access.PlayerID, command)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result.DryRun = false
	result.RequiresConfirmation = false
	result.Confirmation = confirmation
	structured := map[string]any{"fleetDispatch": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callRecallFleet(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.fleetWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp fleet write repository unavailable")
	}
	command, err := mcpRecallFleetCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpRecallFleetConfirmation(command)
	var result domainmcp.RecallFleetResult
	if command.DryRun {
		result, err = s.fleetWrite.PreviewMCPRecallFleet(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Recallable && result.Issue == nil {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.fleetWrite.RecallMCPFleet(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"recallFleet": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callCancelBuildingQueue(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.queueWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp queue write repository unavailable")
	}
	command, err := mcpCancelBuildingQueueCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpCancelBuildingQueueConfirmation(command)
	var result domainmcp.CancelBuildingQueueResult
	if command.DryRun {
		result, err = s.queueWrite.PreviewMCPCancelBuildingQueue(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Cancelable && result.Issue == nil {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.queueWrite.CancelMCPBuildingQueue(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"cancelBuildingQueue": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callCancelResearchQueue(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.queueWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp queue write repository unavailable")
	}
	command, err := mcpCancelResearchQueueCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpCancelResearchQueueConfirmation()
	var result domainmcp.CancelResearchQueueResult
	if command.DryRun {
		result, err = s.queueWrite.PreviewMCPCancelResearchQueue(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Cancelable && result.Issue == nil {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.queueWrite.CancelMCPResearchQueue(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"cancelResearchQueue": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callEnqueueShipyardOrder(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.queueWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp queue write repository unavailable")
	}
	command, err := mcpEnqueueShipyardOrderCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpEnqueueShipyardOrderConfirmation(command)
	var result domainmcp.EnqueueShipyardOrderResult
	if command.DryRun {
		result, err = s.queueWrite.PreviewMCPEnqueueShipyardOrder(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Issue == nil && result.Amount > 0 {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.queueWrite.EnqueueMCPShipyardOrder(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"enqueueShipyardOrder": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callUpdateResourceProduction(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.resourceWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp resource write repository unavailable")
	}
	command, err := mcpUpdateResourceProductionCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpUpdateResourceProductionConfirmation(command)
	var result domainmcp.UpdateResourceProductionResult
	if command.DryRun {
		result, err = s.resourceWrite.PreviewMCPUpdateResourceProduction(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Issue == nil && len(result.Settings) > 0 {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.resourceWrite.UpdateMCPResourceProduction(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"updateResourceProduction": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callRecruitOfficer(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.premiumWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp premium write repository unavailable")
	}
	command, err := mcpRecruitOfficerCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpRecruitOfficerConfirmation(command)
	var result domainmcp.RecruitOfficerResult
	if command.DryRun {
		result, err = s.premiumWrite.PreviewMCPRecruitOfficer(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Issue == nil && result.OfficerID > 0 {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.premiumWrite.RecruitMCPOfficer(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"recruitOfficer": result}
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
	case domainmcp.ScopeRead, domainmcp.ScopeMessages, domainmcp.ScopeMessageWrite, domainmcp.ScopeFleet, domainmcp.ScopeFleetWrite, domainmcp.ScopeQueueWrite, domainmcp.ScopeResourcesWrite, domainmcp.ScopePremiumWrite:
		return true
	default:
		return false
	}
}

type normalizedOAuthAuthorizeRequest struct {
	ClientID            string
	RedirectURI         string
	Resource            string
	State               string
	Scopes              []string
	CodeChallenge       string
	CodeChallengeMethod string
}

func normalizeOAuthAuthorizeRequest(command OAuthAuthorizeCommand, allowedRedirectURIs []string) (normalizedOAuthAuthorizeRequest, error) {
	if strings.TrimSpace(command.ResponseType) != "code" {
		return normalizedOAuthAuthorizeRequest{}, fmt.Errorf("%w: response_type must be code", ErrInvalidOAuthRequest)
	}
	clientID := strings.TrimSpace(command.ClientID)
	if !validOAuthClientID(clientID) {
		return normalizedOAuthAuthorizeRequest{}, fmt.Errorf("%w: client_id is required", ErrInvalidOAuthRequest)
	}
	redirectURI := strings.TrimSpace(command.RedirectURI)
	if !validOAuthRedirectURI(redirectURI, allowedRedirectURIs) {
		return normalizedOAuthAuthorizeRequest{}, fmt.Errorf("%w: redirect_uri is invalid", ErrInvalidOAuthRequest)
	}
	if !validOAuthResource(command.Resource, command.Issuer) {
		return normalizedOAuthAuthorizeRequest{}, fmt.Errorf("%w: resource is invalid", ErrInvalidOAuthRequest)
	}
	method := strings.TrimSpace(command.CodeChallengeMethod)
	if method != "S256" {
		return normalizedOAuthAuthorizeRequest{}, fmt.Errorf("%w: code_challenge_method must be S256", ErrInvalidOAuthRequest)
	}
	challenge := strings.TrimSpace(command.CodeChallenge)
	if !validPKCEValue(challenge) {
		return normalizedOAuthAuthorizeRequest{}, fmt.Errorf("%w: code_challenge is invalid", ErrInvalidOAuthRequest)
	}
	scopes, err := normalizeOAuthScopes(command.Scope)
	if err != nil {
		return normalizedOAuthAuthorizeRequest{}, err
	}
	return normalizedOAuthAuthorizeRequest{
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		Resource:            strings.TrimRight(strings.TrimSpace(command.Resource), "/"),
		State:               strings.TrimSpace(command.State),
		Scopes:              scopes,
		CodeChallenge:       challenge,
		CodeChallengeMethod: method,
	}, nil
}

func normalizeOAuthScopes(raw string) ([]string, error) {
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		parts = []string{domainmcp.ScopeRead}
	}
	seen := map[string]bool{}
	scopes := make([]string, 0, len(parts))
	for _, scope := range parts {
		scope = strings.TrimSpace(scope)
		if scope == "" || seen[scope] {
			continue
		}
		if !oauthScopeAllowed(scope) {
			return nil, fmt.Errorf("%w: scope %q is not available", ErrInvalidOAuthRequest, scope)
		}
		seen[scope] = true
		scopes = append(scopes, scope)
	}
	if len(scopes) == 0 {
		scopes = append(scopes, domainmcp.ScopeRead)
	}
	return scopes, nil
}

func normalizeOAuthClientRedirectURIs(raw []string, allowedRedirectURIs []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: redirect_uris are required", ErrInvalidOAuthRequest)
	}
	redirects := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, value := range raw {
		parsed := strings.TrimSpace(value)
		if !validOAuthRedirectURI(parsed, allowedRedirectURIs) {
			return nil, fmt.Errorf("%w: redirect_uri is invalid", ErrInvalidOAuthRequest)
		}
		normalized, ok := normalizedOAuthResource(parsed)
		if !ok {
			return nil, fmt.Errorf("%w: redirect_uri is invalid", ErrInvalidOAuthRequest)
		}
		if !seen[normalized] {
			seen[normalized] = true
			redirects = append(redirects, normalized)
		}
	}
	return redirects, nil
}

func normalizeOAuthClientValues(raw []string, allowed []string, field string) ([]string, error) {
	if len(raw) == 0 {
		return append([]string(nil), allowed...), nil
	}
	allowedSet := map[string]bool{}
	for _, value := range allowed {
		allowedSet[value] = true
	}
	values := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, value := range raw {
		value = strings.TrimSpace(value)
		if !allowedSet[value] {
			return nil, fmt.Errorf("%w: %s is unsupported", ErrInvalidOAuthRequest, field)
		}
		if !seen[value] {
			seen[value] = true
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("%w: %s is required", ErrInvalidOAuthRequest, field)
	}
	return values, nil
}

func oauthScopeAllowed(scope string) bool {
	return scope == "openid" || scope == "profile" || userScopeAllowed(scope)
}

func scopeAllowed(scopes []string, want string) bool {
	for _, scope := range scopes {
		if scope == want {
			return true
		}
	}
	return false
}

func validOAuthClientID(clientID string) bool {
	return clientID != "" && len(clientID) <= 128 && !strings.ContainsAny(clientID, " \t\r\n")
}

func validOAuthRedirectURI(raw string, allowedRedirectURIs []string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.Fragment != "" {
		return false
	}
	if oauthRedirectAllowedByConfig(parsed.String(), allowedRedirectURIs) {
		return true
	}
	switch parsed.Scheme {
	case "http", "https":
		return isLoopbackHost(parsed.Hostname())
	default:
		return false
	}
}

func validOAuthResource(raw string, issuer string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	resource, ok := normalizedOAuthResource(raw)
	if !ok {
		return false
	}
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	for _, candidate := range []string{issuer, issuer + "/mcp"} {
		normalized, ok := normalizedOAuthResource(candidate)
		if ok && resource == normalized {
			return true
		}
	}
	return false
}

func normalizedOAuthResource(raw string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.Fragment != "" {
		return "", false
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), true
}

func normalizeAllowedOAuthRedirectURIs(raw []string) []string {
	allowed := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, value := range raw {
		parsed, err := url.Parse(strings.TrimSpace(value))
		if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.Fragment != "" {
			continue
		}
		normalized := parsed.String()
		if !seen[normalized] {
			seen[normalized] = true
			allowed = append(allowed, normalized)
		}
	}
	return allowed
}

func ParseOAuthRedirectURIs(raw string) []string {
	return normalizeAllowedOAuthRedirectURIs(strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\t' || r == ' '
	}))
}

func oauthRedirectAllowedByConfig(raw string, allowedRedirectURIs []string) bool {
	for _, allowed := range allowedRedirectURIs {
		if raw == allowed {
			return true
		}
	}
	return false
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validPKCEValue(value string) bool {
	if len(value) < 43 || len(value) > 128 {
		return false
	}
	for _, ch := range value {
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '.' || ch == '_' || ch == '~' {
			continue
		}
		return false
	}
	return true
}

func pkceS256Matches(verifier string, challenge string) bool {
	sum := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return computed == strings.TrimSpace(challenge)
}

func oauthRedirectURI(raw string, code string, state string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("code", code)
	if strings.TrimSpace(state) != "" {
		query.Set("state", strings.TrimSpace(state))
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func truncateOAuthTokenName(name string, limit int) string {
	runes := []rune(strings.TrimSpace(name))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}

func optionalNonNegativeIntArgument(arguments map[string]any, name string) (int, error) {
	if arguments == nil {
		return 0, nil
	}
	raw, ok := arguments[name]
	if !ok || raw == nil {
		return 0, nil
	}
	return nonNegativeIntValue(raw, name)
}

func nonNegativeIntValue(raw any, name string) (int, error) {
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

func mcpSendMessageCommand(arguments map[string]any) (domainmcp.SendMessageCommand, error) {
	targetPlayerID, err := optionalNonNegativeIntArgument(arguments, "targetPlayerId")
	if err != nil || targetPlayerID <= 0 {
		return domainmcp.SendMessageCommand{}, domainmcp.ErrInvalidParams
	}
	subject, err := optionalStringArgument(arguments, "subject")
	if err != nil {
		return domainmcp.SendMessageCommand{}, err
	}
	text, err := optionalStringArgument(arguments, "text")
	if err != nil {
		return domainmcp.SendMessageCommand{}, err
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.SendMessageCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.SendMessageCommand{}, err
	}
	return domainmcp.SendMessageCommand{
		TargetPlayerID: targetPlayerID,
		Subject:        subject,
		Text:           text,
		DryRun:         dryRun,
		Confirm:        confirm,
	}, nil
}

func mcpSendMessageConfirmation(command domainmcp.SendMessageCommand) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d\n%s\n%s", command.TargetPlayerID, command.Subject, command.Text)))
	return fmt.Sprintf("send_message:%d:%s", command.TargetPlayerID, hex.EncodeToString(sum[:])[:12])
}

func mcpDeleteMessagesCommand(arguments map[string]any) (domainmcp.DeleteMessagesCommand, error) {
	messageIDs, err := positiveIntSliceArgument(arguments, "messageIds")
	if err != nil || len(messageIDs) == 0 {
		return domainmcp.DeleteMessagesCommand{}, domainmcp.ErrInvalidParams
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.DeleteMessagesCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.DeleteMessagesCommand{}, err
	}
	return domainmcp.DeleteMessagesCommand{MessageIDs: messageIDs, DryRun: dryRun, Confirm: confirm}, nil
}

func mcpDeleteMessagesConfirmation(command domainmcp.DeleteMessagesCommand) string {
	ids := make([]string, 0, len(command.MessageIDs))
	for _, id := range command.MessageIDs {
		ids = append(ids, strconv.Itoa(id))
	}
	payload := strings.Join(ids, ",")
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("delete_messages:%s:%s", payload, hex.EncodeToString(sum[:])[:12])
}

func mcpReportMessageCommand(arguments map[string]any) (domainmcp.ReportMessageCommand, error) {
	messageID, err := optionalNonNegativeIntArgument(arguments, "messageId")
	if err != nil || messageID <= 0 {
		return domainmcp.ReportMessageCommand{}, domainmcp.ErrInvalidParams
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.ReportMessageCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.ReportMessageCommand{}, err
	}
	return domainmcp.ReportMessageCommand{MessageID: messageID, DryRun: dryRun, Confirm: confirm}, nil
}

func mcpReportMessageConfirmation(command domainmcp.ReportMessageCommand) string {
	payload := strconv.Itoa(command.MessageID)
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("report_message:%s:%s", payload, hex.EncodeToString(sum[:])[:12])
}

func mcpDispatchFleetCommand(arguments map[string]any) (domainmcp.DispatchFleetCommand, error) {
	ships, err := intObjectArgument(arguments, "ships")
	if err != nil || len(ships) == 0 {
		return domainmcp.DispatchFleetCommand{}, domainmcp.ErrInvalidParams
	}
	targetGalaxy, err := optionalNonNegativeIntArgument(arguments, "targetGalaxy")
	if err != nil || targetGalaxy <= 0 {
		return domainmcp.DispatchFleetCommand{}, domainmcp.ErrInvalidParams
	}
	targetSystem, err := optionalNonNegativeIntArgument(arguments, "targetSystem")
	if err != nil || targetSystem <= 0 {
		return domainmcp.DispatchFleetCommand{}, domainmcp.ErrInvalidParams
	}
	targetPosition, err := optionalNonNegativeIntArgument(arguments, "targetPosition")
	if err != nil || targetPosition <= 0 {
		return domainmcp.DispatchFleetCommand{}, domainmcp.ErrInvalidParams
	}
	targetType, err := optionalNonNegativeIntArgument(arguments, "targetType")
	if err != nil || targetType <= 0 {
		return domainmcp.DispatchFleetCommand{}, domainmcp.ErrInvalidParams
	}
	mission, err := optionalNonNegativeIntArgument(arguments, "mission")
	if err != nil || mission <= 0 {
		return domainmcp.DispatchFleetCommand{}, domainmcp.ErrInvalidParams
	}
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.DispatchFleetCommand{}, err
	}
	speed, err := optionalNonNegativeIntArgument(arguments, "speed")
	if err != nil {
		return domainmcp.DispatchFleetCommand{}, err
	}
	holdHours, err := optionalNonNegativeIntArgument(arguments, "holdHours")
	if err != nil {
		return domainmcp.DispatchFleetCommand{}, err
	}
	expeditionHours, err := optionalNonNegativeIntArgument(arguments, "expeditionHours")
	if err != nil {
		return domainmcp.DispatchFleetCommand{}, err
	}
	unionID, err := optionalNonNegativeIntArgument(arguments, "unionId")
	if err != nil {
		return domainmcp.DispatchFleetCommand{}, err
	}
	resources, err := fleetResourcesArgument(arguments, "resources")
	if err != nil {
		return domainmcp.DispatchFleetCommand{}, err
	}
	return domainmcp.DispatchFleetCommand{
		PlanetID:        planetID,
		Ships:           ships,
		Resources:       resources,
		Target:          domainmcp.Coordinates{Galaxy: targetGalaxy, System: targetSystem, Position: targetPosition},
		TargetType:      targetType,
		Mission:         mission,
		Speed:           speed,
		HoldHours:       holdHours,
		ExpeditionHours: expeditionHours,
		UnionID:         unionID,
	}, nil
}

func mcpDispatchFleetConfirmation(command domainmcp.DispatchFleetCommand) string {
	ships := stableIntMapPayload(command.Ships)
	payload := fmt.Sprintf("%d|%s|%d,%d,%d,%d|%d|%d|%d|%d|%d|%d,%d,%d",
		command.PlanetID,
		ships,
		command.Target.Galaxy,
		command.Target.System,
		command.Target.Position,
		command.TargetType,
		command.Mission,
		command.Speed,
		command.HoldHours,
		command.ExpeditionHours,
		command.UnionID,
		command.Resources.Metal,
		command.Resources.Crystal,
		command.Resources.Deuterium,
	)
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("dispatch_fleet:%s", hex.EncodeToString(sum[:])[:16])
}

func mcpRecallFleetCommand(arguments map[string]any) (domainmcp.RecallFleetCommand, error) {
	fleetID, err := optionalNonNegativeIntArgument(arguments, "fleetId")
	if err != nil || fleetID <= 0 {
		return domainmcp.RecallFleetCommand{}, domainmcp.ErrInvalidParams
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.RecallFleetCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.RecallFleetCommand{}, err
	}
	return domainmcp.RecallFleetCommand{FleetID: fleetID, DryRun: dryRun, Confirm: confirm}, nil
}

func mcpRecallFleetConfirmation(command domainmcp.RecallFleetCommand) string {
	payload := strconv.Itoa(command.FleetID)
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("recall_fleet:%s:%s", payload, hex.EncodeToString(sum[:])[:12])
}

func mcpCancelBuildingQueueCommand(arguments map[string]any) (domainmcp.CancelBuildingQueueCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.CancelBuildingQueueCommand{}, err
	}
	listID, err := optionalNonNegativeIntArgument(arguments, "listId")
	if err != nil || listID <= 0 {
		return domainmcp.CancelBuildingQueueCommand{}, domainmcp.ErrInvalidParams
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.CancelBuildingQueueCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.CancelBuildingQueueCommand{}, err
	}
	return domainmcp.CancelBuildingQueueCommand{PlanetID: planetID, ListID: listID, DryRun: dryRun, Confirm: confirm}, nil
}

func mcpCancelBuildingQueueConfirmation(command domainmcp.CancelBuildingQueueCommand) string {
	payload := fmt.Sprintf("%d:%d", command.PlanetID, command.ListID)
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("cancel_building_queue:%s:%s", payload, hex.EncodeToString(sum[:])[:12])
}

func mcpCancelResearchQueueCommand(arguments map[string]any) (domainmcp.CancelResearchQueueCommand, error) {
	dryRun := true
	var err error
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.CancelResearchQueueCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.CancelResearchQueueCommand{}, err
	}
	return domainmcp.CancelResearchQueueCommand{DryRun: dryRun, Confirm: confirm}, nil
}

func mcpCancelResearchQueueConfirmation() string {
	payload := "active"
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("cancel_research_queue:%s:%s", payload, hex.EncodeToString(sum[:])[:12])
}

func mcpEnqueueShipyardOrderCommand(arguments map[string]any) (domainmcp.EnqueueShipyardOrderCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.EnqueueShipyardOrderCommand{}, err
	}
	kind, err := optionalStringArgument(arguments, "kind")
	if err != nil {
		return domainmcp.EnqueueShipyardOrderCommand{}, err
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = "fleet"
	}
	if kind != "fleet" && kind != "defense" {
		return domainmcp.EnqueueShipyardOrderCommand{}, domainmcp.ErrInvalidParams
	}
	itemID, err := optionalNonNegativeIntArgument(arguments, "itemId")
	if err != nil || itemID <= 0 {
		return domainmcp.EnqueueShipyardOrderCommand{}, domainmcp.ErrInvalidParams
	}
	amount, err := optionalNonNegativeIntArgument(arguments, "amount")
	if err != nil || amount <= 0 {
		return domainmcp.EnqueueShipyardOrderCommand{}, domainmcp.ErrInvalidParams
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.EnqueueShipyardOrderCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.EnqueueShipyardOrderCommand{}, err
	}
	return domainmcp.EnqueueShipyardOrderCommand{
		PlanetID: planetID,
		Kind:     kind,
		ItemID:   itemID,
		Amount:   amount,
		DryRun:   dryRun,
		Confirm:  confirm,
	}, nil
}

func mcpEnqueueShipyardOrderConfirmation(command domainmcp.EnqueueShipyardOrderCommand) string {
	payload := fmt.Sprintf("%d:%s:%d:%d", command.PlanetID, command.Kind, command.ItemID, command.Amount)
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("enqueue_shipyard_order:%s:%s", payload, hex.EncodeToString(sum[:])[:12])
}

func mcpUpdateResourceProductionCommand(arguments map[string]any) (domainmcp.UpdateResourceProductionCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.UpdateResourceProductionCommand{}, err
	}
	production, err := intObjectArgument(arguments, "production")
	if err != nil {
		return domainmcp.UpdateResourceProductionCommand{}, err
	}
	if len(production) == 0 {
		return domainmcp.UpdateResourceProductionCommand{}, domainmcp.ErrInvalidParams
	}
	for _, percent := range production {
		if percent > 100 {
			return domainmcp.UpdateResourceProductionCommand{}, domainmcp.ErrInvalidParams
		}
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.UpdateResourceProductionCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.UpdateResourceProductionCommand{}, err
	}
	return domainmcp.UpdateResourceProductionCommand{
		PlanetID:   planetID,
		Production: production,
		DryRun:     dryRun,
		Confirm:    confirm,
	}, nil
}

func mcpUpdateResourceProductionConfirmation(command domainmcp.UpdateResourceProductionCommand) string {
	payload := fmt.Sprintf("%d:%s", command.PlanetID, stableIntMapPayload(command.Production))
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("update_resource_production:%s:%s", payload, hex.EncodeToString(sum[:])[:12])
}

func mcpRecruitOfficerCommand(arguments map[string]any) (domainmcp.RecruitOfficerCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.RecruitOfficerCommand{}, err
	}
	officerID, err := optionalNonNegativeIntArgument(arguments, "officerId")
	if err != nil || officerID < 1 || officerID > 5 {
		return domainmcp.RecruitOfficerCommand{}, domainmcp.ErrInvalidParams
	}
	days, err := optionalNonNegativeIntArgument(arguments, "days")
	if err != nil {
		return domainmcp.RecruitOfficerCommand{}, err
	}
	if days == 0 {
		days = 7
	}
	if days != 7 && days != 90 {
		return domainmcp.RecruitOfficerCommand{}, domainmcp.ErrInvalidParams
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.RecruitOfficerCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.RecruitOfficerCommand{}, err
	}
	return domainmcp.RecruitOfficerCommand{
		PlanetID:  planetID,
		OfficerID: officerID,
		Days:      days,
		DryRun:    dryRun,
		Confirm:   confirm,
	}, nil
}

func mcpRecruitOfficerConfirmation(command domainmcp.RecruitOfficerCommand) string {
	payload := fmt.Sprintf("%d:%d:%d", command.PlanetID, command.OfficerID, command.Days)
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("recruit_officer:%s:%s", payload, hex.EncodeToString(sum[:])[:12])
}

func mcpMessageQuery(arguments map[string]any) (domainmcp.MessageQuery, error) {
	limit, err := optionalNonNegativeIntArgument(arguments, "limit")
	if err != nil {
		return domainmcp.MessageQuery{}, err
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 50 {
		limit = 50
	}
	messageType, err := optionalNonNegativeIntArgument(arguments, "messageType")
	if err != nil {
		return domainmcp.MessageQuery{}, err
	}
	includeText, err := optionalBoolArgument(arguments, "includeText")
	if err != nil {
		return domainmcp.MessageQuery{}, err
	}
	return domainmcp.MessageQuery{
		Limit:          limit,
		MessageType:    messageType,
		HasMessageType: arguments != nil && arguments["messageType"] != nil,
		IncludeText:    includeText,
	}, nil
}

func optionalStringArgument(arguments map[string]any, name string) (string, error) {
	if arguments == nil {
		return "", nil
	}
	raw, ok := arguments[name]
	if !ok || raw == nil {
		return "", nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%w: %s must be a string", domainmcp.ErrInvalidParams, name)
	}
	return value, nil
}

func positiveIntSliceArgument(arguments map[string]any, name string) ([]int, error) {
	if arguments == nil {
		return nil, nil
	}
	raw, ok := arguments[name]
	if !ok || raw == nil {
		return nil, nil
	}
	values, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: %s must be an array", domainmcp.ErrInvalidParams, name)
	}
	seen := map[int]struct{}{}
	result := make([]int, 0, len(values))
	for _, value := range values {
		id, err := nonNegativeIntValue(value, name)
		if err != nil {
			return nil, err
		}
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Ints(result)
	return result, nil
}

func intObjectArgument(arguments map[string]any, name string) (map[int]int, error) {
	if arguments == nil {
		return nil, nil
	}
	raw, ok := arguments[name]
	if !ok || raw == nil {
		return nil, nil
	}
	object, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: %s must be an object", domainmcp.ErrInvalidParams, name)
	}
	values := map[int]int{}
	for key, rawValue := range object {
		id, err := strconv.Atoi(strings.TrimSpace(key))
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("%w: %s contains an invalid id", domainmcp.ErrInvalidParams, name)
		}
		count, err := nonNegativeIntValue(rawValue, name+"."+key)
		if err != nil {
			return nil, err
		}
		if count > 0 {
			values[id] = count
		}
	}
	return values, nil
}

func fleetResourcesArgument(arguments map[string]any, name string) (domainmcp.FleetResources, error) {
	if arguments == nil || arguments[name] == nil {
		return domainmcp.FleetResources{}, nil
	}
	object, ok := arguments[name].(map[string]any)
	if !ok {
		return domainmcp.FleetResources{}, fmt.Errorf("%w: %s must be an object", domainmcp.ErrInvalidParams, name)
	}
	metal, err := optionalObjectNonNegativeInt(object, "metal")
	if err != nil {
		return domainmcp.FleetResources{}, err
	}
	crystal, err := optionalObjectNonNegativeInt(object, "crystal")
	if err != nil {
		return domainmcp.FleetResources{}, err
	}
	deuterium, err := optionalObjectNonNegativeInt(object, "deuterium")
	if err != nil {
		return domainmcp.FleetResources{}, err
	}
	return domainmcp.FleetResources{Metal: metal, Crystal: crystal, Deuterium: deuterium}, nil
}

func optionalObjectNonNegativeInt(object map[string]any, name string) (int, error) {
	raw, ok := object[name]
	if !ok || raw == nil {
		return 0, nil
	}
	return nonNegativeIntValue(raw, name)
}

func stableIntMapPayload(values map[int]int) string {
	keys := make([]int, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%d=%d", key, values[key]))
	}
	return strings.Join(parts, ",")
}

func optionalBoolArgument(arguments map[string]any, name string) (bool, error) {
	if arguments == nil {
		return false, nil
	}
	raw, ok := arguments[name]
	if !ok || raw == nil {
		return false, nil
	}
	value, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("%w: %s must be a boolean", domainmcp.ErrInvalidParams, name)
	}
	return value, nil
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

func listMessagesTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "list_messages",
		Title:       "List Messages",
		Description: "Return the authenticated player's message inbox rows without marking messages read or deleting expired rows.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit": map[string]any{
					"type":        "integer",
					"minimum":     0,
					"maximum":     50,
					"description": "Optional row limit. Defaults to 25 and caps at 50.",
				},
				"messageType": map[string]any{
					"type":        "integer",
					"minimum":     0,
					"description": "Optional legacy message type filter.",
				},
				"includeText": map[string]any{
					"type":        "boolean",
					"description": "Include message body text when true. Defaults to false.",
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"messages": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId": map[string]any{"type": "integer"},
						"count":    map[string]any{"type": "integer"},
						"limit":    map[string]any{"type": "integer"},
						"messages": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "object"},
						},
					},
					"required": []string{"playerId", "count", "limit", "messages"},
				},
			},
			"required": []string{"messages"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func getMessageTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_message",
		Title:       "Get Message",
		Description: "Return one owned message by id without marking it read.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"messageId": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Owned message id.",
				},
			},
			"required":             []string{"messageId"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId": map[string]any{"type": "integer"},
						"message":  map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "message"},
				},
			},
			"required": []string{"message"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func sendMessageTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "send_message",
		Title:       "Send Message",
		Description: "Dry-run or explicitly confirm sending an in-game private message. Defaults to dry-run; confirmed execution requires the exact confirmation string returned by dry-run.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"targetPlayerId": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Recipient player id.",
				},
				"subject": map[string]any{
					"type":        "string",
					"description": "Message subject. Legacy rules truncate to 40 characters.",
				},
				"text": map[string]any{
					"type":        "string",
					"description": "Message body. Legacy rules truncate to 2000 characters.",
				},
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "Defaults to true. Set false only with a matching confirmation value.",
				},
				"confirm": map[string]any{
					"type":        "string",
					"description": "Exact confirmation string returned by a dry-run for the same payload.",
				},
			},
			"required":             []string{"targetPlayerId", "subject", "text"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"sendMessage": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"targetPlayerId":       map[string]any{"type": "integer"},
						"subject":              map[string]any{"type": "string"},
						"textChars":            map[string]any{"type": "integer"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "targetPlayerId", "subject", "textChars", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"sendMessage"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": false,
			"idempotentHint":  false,
		},
	}
}

func deleteMessagesTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "delete_messages",
		Title:       "Delete Messages",
		Description: "Dry-run or explicitly confirm deleting selected owned inbox messages. Only positive messageIds are accepted; broad delete-all modes are not exposed.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"messageIds": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "integer", "minimum": 1},
					"description": "Owned inbox message ids to delete.",
				},
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "Defaults to true. Set false only with a matching confirmation value.",
				},
				"confirm": map[string]any{
					"type":        "string",
					"description": "Exact confirmation string returned by a dry-run for the same messageIds.",
				},
			},
			"required":             []string{"messageIds"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"deleteMessages": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"messageIds":           map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
						"deleteCount":          map[string]any{"type": "integer"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
					},
					"required": []string{"playerId", "messageIds", "deleteCount", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"deleteMessages"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}

func reportMessageTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "report_message",
		Title:       "Report Message",
		Description: "Dry-run or explicitly confirm reporting one owned inbox private message. Non-PM and non-visible messages are not reported.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"messageId": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Owned visible private-message id to report.",
				},
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "Defaults to true. Set false only with a matching confirmation value.",
				},
				"confirm": map[string]any{
					"type":        "string",
					"description": "Exact confirmation string returned by a dry-run for the same messageId.",
				},
			},
			"required":             []string{"messageId"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"reportMessage": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"messageId":            map[string]any{"type": "integer"},
						"reportable":           map[string]any{"type": "boolean"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "messageId", "reportable", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"reportMessage"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
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

func validateFleetDispatchTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "validate_fleet_dispatch",
		Title:       "Validate Fleet Dispatch",
		Description: "Dry-run a fleet dispatch against the authenticated player's current fleet screen and return readiness, issue, fuel, cargo, and confirmation data.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId":       map[string]any{"type": "integer", "minimum": 1},
				"ships":          map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "integer", "minimum": 0}},
				"resources":      map[string]any{"type": "object", "properties": map[string]any{"metal": map[string]any{"type": "integer", "minimum": 0}, "crystal": map[string]any{"type": "integer", "minimum": 0}, "deuterium": map[string]any{"type": "integer", "minimum": 0}}, "additionalProperties": false},
				"targetGalaxy":   map[string]any{"type": "integer", "minimum": 1},
				"targetSystem":   map[string]any{"type": "integer", "minimum": 1},
				"targetPosition": map[string]any{"type": "integer", "minimum": 1},
				"targetType":     map[string]any{"type": "integer", "minimum": 1},
				"mission":        map[string]any{"type": "integer", "minimum": 1},
				"speed":          map[string]any{"type": "integer", "minimum": 0},
				"holdHours":      map[string]any{"type": "integer", "minimum": 0},
				"expeditionHours": map[string]any{
					"type":    "integer",
					"minimum": 0,
				},
				"unionId": map[string]any{"type": "integer", "minimum": 0},
			},
			"required":             []string{"ships", "targetGalaxy", "targetSystem", "targetPosition", "targetType", "mission"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"fleetDispatchValidation": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"planetId":             map[string]any{"type": "integer"},
						"ready":                map[string]any{"type": "boolean"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"totalShips":           map[string]any{"type": "integer"},
						"mission":              map[string]any{"type": "integer"},
						"target":               map[string]any{"type": "object"},
						"targetType":           map[string]any{"type": "integer"},
						"speed":                map[string]any{"type": "integer"},
						"fuelConsumption":      map[string]any{"type": "integer"},
						"cargo":                map[string]any{"type": "integer"},
						"remainingCargo":       map[string]any{"type": "integer"},
						"durationSeconds":      map[string]any{"type": "integer"},
						"distance":             map[string]any{"type": "integer"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planetId", "ready", "dryRun", "requiresConfirmation", "totalShips", "mission", "target", "targetType"},
				},
			},
			"required": []string{"fleetDispatchValidation"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func dispatchFleetTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "dispatch_fleet",
		Title:       "Dispatch Fleet",
		Description: "Execute a fleet dispatch only with the exact confirmation returned by validate_fleet_dispatch for the same arguments.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId":        map[string]any{"type": "integer", "minimum": 1},
				"ships":           map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "integer", "minimum": 0}},
				"resources":       map[string]any{"type": "object", "properties": map[string]any{"metal": map[string]any{"type": "integer", "minimum": 0}, "crystal": map[string]any{"type": "integer", "minimum": 0}, "deuterium": map[string]any{"type": "integer", "minimum": 0}}, "additionalProperties": false},
				"targetGalaxy":    map[string]any{"type": "integer", "minimum": 1},
				"targetSystem":    map[string]any{"type": "integer", "minimum": 1},
				"targetPosition":  map[string]any{"type": "integer", "minimum": 1},
				"targetType":      map[string]any{"type": "integer", "minimum": 1},
				"mission":         map[string]any{"type": "integer", "minimum": 1},
				"speed":           map[string]any{"type": "integer", "minimum": 0},
				"holdHours":       map[string]any{"type": "integer", "minimum": 0},
				"expeditionHours": map[string]any{"type": "integer", "minimum": 0},
				"unionId":         map[string]any{"type": "integer", "minimum": 0},
				"confirm":         map[string]any{"type": "string"},
			},
			"required":             []string{"ships", "targetGalaxy", "targetSystem", "targetPosition", "targetType", "mission", "confirm"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"fleetDispatch": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":        map[string]any{"type": "integer"},
						"planetId":        map[string]any{"type": "integer"},
						"ready":           map[string]any{"type": "boolean"},
						"dryRun":          map[string]any{"type": "boolean"},
						"executed":        map[string]any{"type": "boolean"},
						"totalShips":      map[string]any{"type": "integer"},
						"mission":         map[string]any{"type": "integer"},
						"target":          map[string]any{"type": "object"},
						"targetType":      map[string]any{"type": "integer"},
						"confirmation":    map[string]any{"type": "string"},
						"fuelConsumption": map[string]any{"type": "integer"},
						"issue":           map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planetId", "ready", "dryRun", "executed", "totalShips", "mission", "target", "targetType"},
				},
			},
			"required": []string{"fleetDispatch"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}

func recallFleetTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "recall_fleet",
		Title:       "Recall Fleet",
		Description: "Recall one owned outgoing fleet. Defaults to dry-run and requires the returned confirmation string before execution.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"fleetId": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Owned fleet id to recall.",
				},
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "Defaults to true. Set false only with a matching confirmation value.",
				},
				"confirm": map[string]any{
					"type":        "string",
					"description": "Exact confirmation string returned by a dry-run for the same fleetId.",
				},
			},
			"required":             []string{"fleetId"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"recallFleet": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"fleetId":              map[string]any{"type": "integer"},
						"ownerId":              map[string]any{"type": "integer"},
						"mission":              map[string]any{"type": "integer"},
						"totalShips":           map[string]any{"type": "integer"},
						"recallable":           map[string]any{"type": "boolean"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "fleetId", "recallable", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"recallFleet"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}

func cancelBuildingQueueTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "cancel_building_queue",
		Title:       "Cancel Building Queue",
		Description: "Cancel one owned building queue row. Defaults to dry-run and requires the returned confirmation string before execution.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Planet id. Omit to use the active planet.",
				},
				"listId": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Building queue list id to cancel.",
				},
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "Defaults to true. Set false only with a matching confirmation value.",
				},
				"confirm": map[string]any{
					"type":        "string",
					"description": "Exact confirmation string returned by a dry-run for the same planetId/listId.",
				},
			},
			"required":             []string{"listId"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cancelBuildingQueue": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"planetId":             map[string]any{"type": "integer"},
						"listId":               map[string]any{"type": "integer"},
						"techId":               map[string]any{"type": "integer"},
						"name":                 map[string]any{"type": "string"},
						"level":                map[string]any{"type": "integer"},
						"destroy":              map[string]any{"type": "boolean"},
						"cancelable":           map[string]any{"type": "boolean"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planetId", "listId", "cancelable", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"cancelBuildingQueue"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}

func cancelResearchQueueTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "cancel_research_queue",
		Title:       "Cancel Research Queue",
		Description: "Cancel the active owned research queue. Defaults to dry-run and requires the returned confirmation string before execution.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "Defaults to true. Set false only with a matching confirmation value.",
				},
				"confirm": map[string]any{
					"type":        "string",
					"description": "Exact confirmation string returned by a dry-run for the active research queue.",
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cancelResearchQueue": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"planetId":             map[string]any{"type": "integer"},
						"taskId":               map[string]any{"type": "integer"},
						"techId":               map[string]any{"type": "integer"},
						"name":                 map[string]any{"type": "string"},
						"level":                map[string]any{"type": "integer"},
						"cancelable":           map[string]any{"type": "boolean"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "cancelable", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"cancelResearchQueue"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}

func enqueueShipyardOrderTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "enqueue_shipyard_order",
		Title:       "Enqueue Shipyard Order",
		Description: "Queue fleet or defense construction for an owned planet. Defaults to dry-run and requires the returned confirmation string before execution.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet.",
				},
				"kind": map[string]any{
					"type":        "string",
					"enum":        []string{"fleet", "defense"},
					"description": "Order catalog. Defaults to fleet.",
				},
				"itemId": map[string]any{
					"type":        "integer",
					"description": "Fleet or defense technology id to queue.",
				},
				"amount": map[string]any{
					"type":        "integer",
					"description": "Requested unit count. The game may clamp to order cap/resources.",
				},
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "Defaults to true. Set false only with a matching confirmation value.",
				},
				"confirm": map[string]any{
					"type":        "string",
					"description": "Exact confirmation string returned by a dry-run for the same planet, kind, item, and amount.",
				},
			},
			"required":             []string{"itemId", "amount"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"enqueueShipyardOrder": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"planetId":             map[string]any{"type": "integer"},
						"kind":                 map[string]any{"type": "string"},
						"itemId":               map[string]any{"type": "integer"},
						"name":                 map[string]any{"type": "string"},
						"requested":            map[string]any{"type": "integer"},
						"amount":               map[string]any{"type": "integer"},
						"maxBuild":             map[string]any{"type": "integer"},
						"durationSeconds":      map[string]any{"type": "integer"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planetId", "kind", "itemId", "requested", "amount", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"enqueueShipyardOrder"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}

func updateResourceProductionTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "update_resource_production",
		Title:       "Update Resource Production",
		Description: "Update owned planet resource production percentages. Defaults to dry-run and requires the returned confirmation string before execution.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet.",
				},
				"production": map[string]any{
					"type":        "object",
					"description": "Producer id to percent, for example {\"1\":80,\"2\":70}. Supported ids are legacy resource producer ids.",
					"additionalProperties": map[string]any{
						"type":    "integer",
						"minimum": 0,
						"maximum": 100,
					},
				},
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "Defaults to true. Set false only with a matching confirmation value.",
				},
				"confirm": map[string]any{
					"type":        "string",
					"description": "Exact confirmation string returned by a dry-run for the same planet and production map.",
				},
			},
			"required":             []string{"production"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"updateResourceProduction": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"planetId":             map[string]any{"type": "integer"},
						"settings":             map[string]any{"type": "array"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planetId", "settings", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"updateResourceProduction"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}

func recruitOfficerTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "recruit_officer",
		Title:       "Recruit Officer",
		Description: "Recruit or extend a commander/officer using Dark Matter. Defaults to dry-run and requires the returned confirmation string before execution.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet.",
				},
				"officerId": map[string]any{
					"type":        "integer",
					"description": "Legacy officer id: 1 commander, 2 admiral, 3 engineer, 4 geologist, 5 technocrat.",
					"minimum":     1,
					"maximum":     5,
				},
				"days": map[string]any{
					"type":        "integer",
					"description": "Recruitment duration in days. Defaults to 7; allowed values are 7 and 90.",
					"enum":        []int{7, 90},
				},
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "Defaults to true. Set false only with a matching confirmation value.",
				},
				"confirm": map[string]any{
					"type":        "string",
					"description": "Exact confirmation string returned by a dry-run for the same planet, officer, and duration.",
				},
			},
			"required":             []string{"officerId"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"recruitOfficer": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"planetId":             map[string]any{"type": "integer"},
						"officerId":            map[string]any{"type": "integer"},
						"name":                 map[string]any{"type": "string"},
						"days":                 map[string]any{"type": "integer"},
						"cost":                 map[string]any{"type": "integer"},
						"paidDarkMatter":       map[string]any{"type": "integer"},
						"freeDarkMatter":       map[string]any{"type": "integer"},
						"until":                map[string]any{"type": "integer"},
						"daysLeft":             map[string]any{"type": "integer"},
						"active":               map[string]any{"type": "boolean"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planetId", "officerId", "days", "cost", "paidDarkMatter", "freeDarkMatter", "active", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"recruitOfficer"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}
